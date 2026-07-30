// Package github is a driven adapter POSTing a GitHub repository_dispatch
// event to trigger the site rebuild (design D-2). It resolves no
// configuration of its own -- the caller (app/cmd/concontexto) reads the
// repository slug and fine-grained token from env and passes them to
// NewClient, mirroring adapters/ine|eurostat|xlsx's own convention that a
// source-adapter package never resolves its own configuration.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/useragent"
)

const defaultBaseURL = "https://api.github.com"
const defaultTimeout = 10 * time.Second

// Client dispatches a rebuild event for one repository. BaseURL is
// exported (not a functional option) purely so a test can point it at an
// httptest.Server -- production code never sets it, leaving
// defaultBaseURL in place (NewClient's own zero-value contract).
type Client struct {
	Repo       string // "owner/repo"
	Token      string
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient builds a Client for repo, authenticating with token (a
// fine-grained personal access token scoped to Contents: read and
// Actions: write on this one repository -- design D-2, "a fine-grained
// token from env"). A nil httpClient gets a bounded-timeout default, the
// same convention every other source adapter's NewClient already
// establishes (verify-report WARNING W6).
func NewClient(repo, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{Repo: repo, Token: token, BaseURL: defaultBaseURL, HTTPClient: httpClient}
}

type dispatchPayload struct {
	EventType     string            `json:"event_type"`
	ClientPayload map[string]string `json:"client_payload"`
}

// Dispatch POSTs /repos/{repo}/dispatches with event_type "rebuild" and a
// client_payload naming generatedAt (the artifact's own manifest.
// generated_at, RFC3339) and manifestDigest (a stable digest over the
// whole manifest's per-file digests) -- satisfies publishing.Dispatcher.
// A non-204 response is a returned error, never a panic and never a
// retry of its own (design D-2: "Dispatch failure is an alert ...,
// never a retry loop" -- the retry decision belongs to the caller,
// which per that same design NEVER retries either).
func (c *Client) Dispatch(ctx context.Context, generatedAt time.Time, manifestDigest string) error {
	body, err := json.Marshal(dispatchPayload{
		EventType: "rebuild",
		ClientPayload: map[string]string{
			"generated_at":    generatedAt.UTC().Format(time.RFC3339),
			"manifest_digest": manifestDigest,
		},
	})
	if err != nil {
		return fmt.Errorf("github: marshalling dispatch payload: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/dispatches", c.BaseURL, c.Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("github: building dispatch request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	// The GitHub REST API documents a User-Agent as REQUIRED and answers a
	// request without one with HTTP 403 ("Request forbidden by
	// administrative rules"). That is a hard blocker here, not a warning:
	// this dispatch is what triggers the site rebuild, and design D-2 makes
	// a dispatch failure an alert and NEVER a retry loop -- so a request
	// refused on a header technicality means the published artifact simply
	// never reaches the site, with no second attempt. The same constant
	// also fixes INE, whose edge blackholes Go's default token outright;
	// app/internal/useragent carries the measured evidence for both.
	req.Header.Set("User-Agent", useragent.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: dispatch request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("github: dispatch returned status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

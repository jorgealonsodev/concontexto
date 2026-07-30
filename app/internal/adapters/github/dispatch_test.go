package github_test

// Task 4.7 (RED) / 4.8 (GREEN): the repository_dispatch adapter (design
// D-2: "adapters/github/ POSTs a repository_dispatch (event_type:
// rebuild, payload: generated_at, manifest digest) with a fine-grained
// token from env").

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/github"
)

func TestClient_DispatchPostsARepositoryDispatchEventWithTheExpectedShape(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotAccept string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := github.NewClient("jorgealonsodev/concontexto", "test-token", nil)
	client.BaseURL = server.URL

	generatedAt := time.Date(2026, 7, 29, 6, 0, 0, 0, time.UTC)
	if err := client.Dispatch(context.Background(), generatedAt, "digest123"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/repos/jorgealonsodev/concontexto/dispatches" {
		t.Errorf("expected the dispatches endpoint, got %s", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("expected a Bearer token header, got %q", gotAuth)
	}
	if gotAccept != "application/vnd.github+json" {
		t.Errorf("expected the GitHub JSON accept header, got %q", gotAccept)
	}
	if gotBody["event_type"] != "rebuild" {
		t.Errorf("expected event_type=rebuild, got %+v", gotBody)
	}
	payload, ok := gotBody["client_payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected a client_payload object, got %+v", gotBody)
	}
	if payload["generated_at"] != "2026-07-29T06:00:00Z" {
		t.Errorf("expected the RFC3339 generated_at, got %+v", payload["generated_at"])
	}
	if payload["manifest_digest"] != "digest123" {
		t.Errorf("expected the manifest digest, got %+v", payload["manifest_digest"])
	}
}

func TestClient_DispatchReturnsAnErrorOnANonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer server.Close()

	client := github.NewClient("jorgealonsodev/concontexto", "bad-token", nil)
	client.BaseURL = server.URL

	err := client.Dispatch(context.Background(), time.Now(), "digest123")
	if err == nil {
		t.Fatal("expected an error on a 403 response")
	}
}

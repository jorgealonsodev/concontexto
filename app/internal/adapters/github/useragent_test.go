package github_test

// Companion to adapters/ine's own useragent_test.go, which carries the
// full evidence for why an explicit User-Agent is load-bearing (INE's
// edge blackholes Go's default "Go-http-client/1.1" token outright --
// see app/internal/useragent for the measured table).
//
// This one is not merely good manners: the GitHub REST API DOCUMENTS a
// User-Agent as required and answers a request without one with HTTP 403
// ("Request forbidden by administrative rules"). Dispatch is what triggers
// the site rebuild (design D-2), and design D-2 says a dispatch failure is
// an alert and NEVER a retry loop -- so a request GitHub refuses on a
// header technicality means the published artifact silently never reaches
// the site, with no second chance. The assertion is on the header AS
// RECEIVED by the server, through a real request the Client issues.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/github"
	"github.com/jorgealonsodev/concontexto/app/internal/useragent"
)

func TestDispatch_SendsTheSharedUserAgentOnTheWire(t *testing.T) {
	var got string
	var seen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		seen = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := github.NewClient("owner/repo", "token", srv.Client())
	client.BaseURL = srv.URL
	if err := client.Dispatch(context.Background(), time.Unix(0, 0).UTC(), "digest"); err != nil {
		t.Fatalf("Dispatch against a healthy stub returned an error: %v", err)
	}

	if !seen {
		t.Fatal("the stub server was never called, so no header was observed")
	}
	if got != useragent.UserAgent {
		t.Fatalf("User-Agent on the wire = %q, want %q", got, useragent.UserAgent)
	}
}

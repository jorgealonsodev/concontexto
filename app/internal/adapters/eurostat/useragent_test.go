package eurostat_test

// Companion to adapters/ine's own useragent_test.go, which carries the
// full evidence for why an explicit User-Agent is load-bearing (INE's
// edge blackholes Go's default "Go-http-client/1.1" token outright --
// see app/internal/useragent for the measured table).
//
// Eurostat has NOT been observed to reject the default token, so this is
// not a fix for an observed Eurostat outage. It is here because a public
// open-data API is entitled to know who is calling it, and because
// "every outbound client identifies identically" is only true if it is
// checked everywhere -- a source operator correlating traffic must see
// one client, not one identified adapter and two anonymous ones. The
// assertion is on the header AS RECEIVED by the server, through a real
// request the Client issues.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/eurostat"
	"github.com/jorgealonsodev/concontexto/app/internal/useragent"
)

func TestFetchRaw_SendsTheSharedUserAgentOnTheWire(t *testing.T) {
	var got string
	var seen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		seen = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := eurostat.NewClient(srv.URL, map[string]string{"unit": "PC"}, srv.Client())
	if _, err := client.FetchRaw(context.Background(), "dataset"); err != nil {
		t.Fatalf("FetchRaw against a healthy stub returned an error: %v", err)
	}

	if !seen {
		t.Fatal("the stub server was never called, so no header was observed")
	}
	if got != useragent.UserAgent {
		t.Fatalf("User-Agent on the wire = %q, want %q", got, useragent.UserAgent)
	}
}

// The synthetic daily probe must be indistinguishable from ingestion at
// the source's edge, or it cannot detect what would block ingestion.
func TestFetchProbe_SendsTheSameUserAgentAsIngestion(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := eurostat.NewClient(srv.URL, nil, srv.Client())
	if _, err := client.FetchProbe(context.Background(), "dataset", 1); err != nil {
		t.Fatalf("FetchProbe against a healthy stub returned an error: %v", err)
	}
	if got != useragent.UserAgent {
		t.Fatalf("probe User-Agent = %q, want %q", got, useragent.UserAgent)
	}
}

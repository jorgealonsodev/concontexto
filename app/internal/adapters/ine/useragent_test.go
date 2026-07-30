package ine_test

// Production blocker (measured live 2026-07-30): INE's edge silently
// blackholes Go's DEFAULT User-Agent. Every ingest of every series failed
// with "context deadline exceeded (Client.Timeout exceeded while awaiting
// headers)" -- not a slow response, NO response at all.
//
// The bisection that produced this test is recorded in full at
// app/internal/useragent/useragent.go, where the header value lives. The
// short version: over one identical raw Go TLS connection to
// servicios.ine.es:443 requesting the same DATOS_SERIE URL, sending
// "Go-http-client/1.1" hangs past a 10s deadline with zero bytes back,
// while sending "Go-http-client/2.0", sending nothing at all, or sending
// this project's own token all return HTTP 200 in ~210ms.
// "Go-http-client/1.1" is EXACTLY what net/http puts on the wire over
// HTTP/1.1 when no User-Agent is set, and this client set none.
//
// This test therefore asserts on the header as it ARRIVES AT THE SERVER,
// through a real request the Client issues -- not on the existence of a
// constant. A constant nobody attaches to the request would have left
// production exactly as broken as it was.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/useragent"
)

// goDefaultUserAgentPrefix is what net/http sends when a request carries
// no explicit User-Agent ("Go-http-client/1.1" over HTTP/1.1,
// "Go-http-client/2.0" over HTTP/2). Only the /1.1 form is blackholed by
// INE, but asserting against the whole family is the correct guard: which
// protocol version net/http negotiates is not this adapter's decision, so
// "we happened to get HTTP/2 today" must never be what keeps ingestion
// alive.
const goDefaultUserAgentPrefix = "Go-http-client/"

// contactURL is the repository the User-Agent must point a source
// operator at. Public open-data APIs expect a client to be reachable; INE
// has no other channel to tell us we are misbehaving.
const contactURL = "https://github.com/jorgealonsodev/concontexto"

func TestFetchRaw_SendsAnExplicitHonestUserAgentOnTheWire(t *testing.T) {
	cod, fixture := loadDatosSerieFixture(t)

	var got string
	var seen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		seen = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	client := ine.NewClient(srv.URL, srv.Client())
	if _, err := client.FetchRaw(context.Background(), cod); err != nil {
		t.Fatalf("FetchRaw against a healthy stub returned an error: %v", err)
	}

	if !seen {
		t.Fatal("the stub server was never called, so no header was observed")
	}
	if got == "" {
		t.Fatal("the request arrived with no User-Agent at all; INE's edge tolerates that today, but an unidentified client is exactly what this project's honesty-to-sources principle forbids")
	}
	if strings.HasPrefix(got, goDefaultUserAgentPrefix) {
		t.Fatalf("the request arrived carrying Go's DEFAULT User-Agent %q -- INE's edge blackholes the /1.1 form of it, which is the production blocker this test exists to prevent regressing", got)
	}
	if !strings.HasPrefix(got, "concontexto") {
		t.Fatalf("User-Agent %q does not name this project; a source operator must be able to tell who is calling", got)
	}
	if !strings.Contains(got, contactURL) {
		t.Fatalf("User-Agent %q carries no contact URL; %s must be reachable from the header", got, contactURL)
	}
	for _, impersonation := range []string{"Mozilla", "Chrome", "Safari", "curl", "Wget"} {
		if strings.Contains(got, impersonation) {
			t.Fatalf("User-Agent %q impersonates %s; identifying as software we are not is dishonest to the source and is not how this blocker gets fixed", got, impersonation)
		}
	}
	if got != useragent.UserAgent {
		t.Fatalf("User-Agent on the wire = %q, want the shared constant %q -- every outbound client in this project identifies identically", got, useragent.UserAgent)
	}
}

// TestFetchProbe_SendsTheSameUserAgentAsIngestion guards the synthetic
// daily probe (spec pipeline-operations, "Synthetic daily probe against
// every endpoint"). The probe exists to detect a broken endpoint BEFORE
// the ingestion window; a probe that reached INE while ingestion was
// blackholed would have reported green through the entire outage, because
// it would have been a different client to INE's edge. It is not: it
// shares doRequest, and this test pins that.
func TestFetchProbe_SendsTheSameUserAgentAsIngestion(t *testing.T) {
	cod, fixture := loadDatosSerieFixture(t)

	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	client := ine.NewClient(srv.URL, srv.Client())
	if _, err := client.FetchProbe(context.Background(), cod, 1); err != nil {
		t.Fatalf("FetchProbe against a healthy stub returned an error: %v", err)
	}
	if got != useragent.UserAgent {
		t.Fatalf("probe User-Agent = %q, want %q -- the probe must be indistinguishable from ingestion at the source's edge, or it cannot detect what blocks ingestion", got, useragent.UserAgent)
	}
}

// TestFetchSeries_RetriesKeepTheUserAgent proves the header survives the
// retry path, not just the first attempt. A 5xx-then-200 sequence walks
// fetchWithRetry's retry branch; if the header were attached anywhere
// other than doRequest, the retry would be the request that silently lost
// it -- and a retry that hangs forever is precisely the failure mode this
// whole change removes.
func TestFetchSeries_RetriesKeepTheUserAgent(t *testing.T) {
	cod, fixture := loadDatosSerieFixture(t)

	var observed []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed = append(observed, r.Header.Get("User-Agent"))
		if len(observed) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	client := ine.NewClient(srv.URL, srv.Client(), ine.WithSleep(func(time.Duration) {}))
	if _, err := client.FetchSeries(context.Background(), cod, indicators.FrequencyQuarterly); err != nil {
		t.Fatalf("FetchSeries across a 503-then-200 sequence returned an error: %v", err)
	}
	if len(observed) != 2 {
		t.Fatalf("expected exactly 2 attempts (503 then 200), got %d", len(observed))
	}
	for attempt, ua := range observed {
		if ua != useragent.UserAgent {
			t.Fatalf("attempt %d sent User-Agent %q, want %q", attempt+1, ua, useragent.UserAgent)
		}
	}
}

package ine_test

// Task 5b.6 (RED/GREEN): the ingestion orchestrator must archive the
// exact raw bytes BEFORE any parsing happens (spec raw-file-archive's
// ordering guarantee), so FetchRaw and DecodeSeries split FetchSeries's
// one-call fetch+decode into two composable steps -- no behaviour change
// to FetchSeries itself (still exercised, unmodified, by every test
// above).
//
// This also RED/GREEN-covers a live discovery (verified 2026-07-28): a
// bare "?tip=A" query with no "nult" 404s against the real INE endpoint
// (https://servicios.ine.es/wstempus/js/ES/DATOS_SERIE/EPA453100?tip=A
// returns HTTP 404 -- confirmed against the live service, not a fixture
// artifact). Every one of milestone 0.2's six pinned series needs its
// FULL history (spec source-ingestion-ine, "each MUST load its full
// history"), so the client must always request an explicit, generous
// "nult" -- verified live: the longest of the six series (IPC, monthly,
// 2002-2026) is 294 periods, so 9999 leaves comfortable headroom for
// decades of future growth without ever needing to raise it.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

func TestFetchRaw_RequestsFullHistoryWithAnExplicitGenerousNult(t *testing.T) {
	cod, fixture := loadDatosSerieFixture(t)
	var gotQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())

	raw, err := client.FetchRaw(context.Background(), cod)
	if err != nil {
		t.Fatalf("FetchRaw: %v", err)
	}
	if string(raw) != string(fixture) {
		t.Error("expected FetchRaw to return the exact undecoded response bytes")
	}

	nult := gotQuery.Get("nult")
	if nult == "" {
		t.Fatal("expected the request to carry an explicit nult -- a bare tip=A with no nult 404s against the real INE endpoint, verified live 2026-07-28")
	}
	n, err := strconv.Atoi(nult)
	if err != nil {
		t.Fatalf("nult=%q is not numeric: %v", nult, err)
	}
	if n < 999 {
		t.Errorf("expected a generous nult (the longest real pinned series holds 294 periods) covering full history, got nult=%d", n)
	}
}

func TestFetchRaw_IssuesRequestOnlyToDatosSerie(t *testing.T) {
	cod, fixture := loadDatosSerieFixture(t)
	counter := &requestCounter{respond: func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}}
	server := httptest.NewServer(counter.handler())
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())
	if _, err := client.FetchRaw(context.Background(), cod); err != nil {
		t.Fatalf("FetchRaw: %v", err)
	}
	if counter.datosSerie != 1 || counter.datosTabla != 0 {
		t.Errorf("expected exactly 1 DATOS_SERIE request and 0 DATOS_TABLA, got serie=%d tabla=%d", counter.datosSerie, counter.datosTabla)
	}
}

func TestDecodeSeries_PureDecodeOfAlreadyFetchedBytes(t *testing.T) {
	cod, fixture := loadDatosSerieFixture(t)

	result, err := ine.DecodeSeries(fixture, cod, indicators.FrequencyQuarterly)
	if err != nil {
		t.Fatalf("DecodeSeries: %v", err)
	}
	if result.COD != cod {
		t.Errorf("expected COD %s, got %s", cod, result.COD)
	}
	if len(result.Observations) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(result.Observations))
	}
}

func TestDecodeSeries_PeriodicityMismatchStillFails(t *testing.T) {
	_, err := ine.DecodeSeries([]byte(monthlyFixtureBody), "test-decode-mismatch", indicators.FrequencyQuarterly)
	if err == nil {
		t.Fatal("expected DecodeSeries to still enforce the periodicity assertion with no HTTP involved")
	}
	if !strings.Contains(err.Error(), "periodicity mismatch") {
		t.Errorf("expected a periodicity-mismatch error, got %v", err)
	}
}

// TestFetchRaw_ResponseOverCeilingFailsWithoutRetryingAndNamesTheCeiling
// mirrors adapters/eurostat's identical test (remediation batch 4, W12
// follow-up: adapters/ine had no response-ceiling mechanism at all). A
// response-too-large classification must return immediately, exactly
// one request issued, never retried the way a RetryableTransport
// failure would be -- a body over the ceiling will not become smaller
// on a second attempt.
func TestFetchRaw_ResponseOverCeilingFailsWithoutRetryingAndNamesTheCeiling(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes.Repeat([]byte("x"), 500))
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client(),
		ine.WithMaxResponseBytes(100),
		ine.WithSleep(func(time.Duration) {}))
	_, err := client.FetchRaw(context.Background(), "any-cod")
	if err == nil {
		t.Fatal("expected a response-too-large error")
	}
	if !strings.Contains(err.Error(), "100") {
		t.Errorf("expected the error to name the configured 100-byte ceiling, got %v", err)
	}
	if requestCount != 1 {
		t.Errorf("expected exactly 1 request (a permanent classification must not be retried), got %d", requestCount)
	}
}

// TestFetchRaw_ResponseUnderCeilingSucceeds triangulates the ceiling
// check's other branch: a real fixture body, comfortably under a
// realistic ceiling, must decode successfully.
func TestFetchRaw_ResponseUnderCeilingSucceeds(t *testing.T) {
	cod, fixture := loadDatosSerieFixture(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client(),
		ine.WithMaxResponseBytes(int64(len(fixture))+1))
	raw, err := client.FetchRaw(context.Background(), cod)
	if err != nil {
		t.Fatalf("FetchRaw: %v", err)
	}
	if len(raw) != len(fixture) {
		t.Errorf("expected the full %d-byte fixture body, got %d bytes", len(fixture), len(raw))
	}
}

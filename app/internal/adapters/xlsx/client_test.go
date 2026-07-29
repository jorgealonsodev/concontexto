package xlsx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/xlsx"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

// TestClient_RequestURLIsTheRefItself proves the xlsx-specific shape
// (unlike ine/eurostat, which build a URL from a baseURL + a per-call
// code): for an xlsx-url source_ref, ref already IS the full download
// URL.
func TestClient_RequestURLIsTheRefItself(t *testing.T) {
	c := xlsx.NewClient(realFixtureSchema(), nil)
	const ref = "http://example.invalid/afiliacion.xlsx"
	if got := c.RequestURL(ref); got != ref {
		t.Errorf("RequestURL(%q) = %q, want the ref unchanged", ref, got)
	}
}

// TestClient_FetchRawThenDecode is task 8.9's end-to-end client-level
// proof: fetch the real fixture bytes over HTTP, then decode them through
// the same Client that carries the series' configured schema -- the
// exact round trip IngestSeries drives.
func TestClient_FetchRawThenDecode(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "afiliacion-ss", "afiliacion-ss.xlsx"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	client := xlsx.NewClient(realFixtureSchema(), server.Client())
	raw, err := client.FetchRaw(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("FetchRaw: %v", err)
	}
	if len(raw) != len(body) {
		t.Fatalf("expected the exact undecoded fixture bytes (%d bytes), got %d", len(body), len(raw))
	}
	if requestCount != 1 {
		t.Errorf("expected exactly 1 request, got %d", requestCount)
	}

	result, err := client.Decode(raw, server.URL, indicators.FrequencyMonthly)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(result.Observations) != 306 {
		t.Errorf("expected 306 observations from the full real fixture, got %d", len(result.Observations))
	}
}

// TestClient_TransportErrorRetriesWithBackoffAndSucceeds mirrors
// adapters/ine and adapters/eurostat's own resilience contract: a
// 503-then-200 sequence eventually succeeds.
func TestClient_TransportErrorRetriesWithBackoffAndSucceeds(t *testing.T) {
	body := []byte("PK\x03\x04fake-but-nonempty-zip-bytes")
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	client := xlsx.NewClient(realFixtureSchema(), server.Client(), xlsx.WithSleep(func(time.Duration) {}))
	raw, err := client.FetchRaw(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("FetchRaw: %v", err)
	}
	if string(raw) != string(body) {
		t.Error("expected the eventual successful response body")
	}
	if attempts != 2 {
		t.Errorf("expected exactly 2 attempts (1 failure + 1 success), got %d", attempts)
	}
}

// TestClient_ResponseOverCeilingFailsWithoutRetrying mirrors
// adapters/eurostat's own ceiling proof: a body over the configured
// ceiling fails permanently (ResponseTooLarge is not retryable), exactly
// one request issued.
func TestClient_ResponseOverCeilingFailsWithoutRetrying(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write(make([]byte, 500))
	}))
	defer server.Close()

	client := xlsx.NewClient(realFixtureSchema(), server.Client(),
		xlsx.WithMaxResponseBytes(100), xlsx.WithSleep(func(time.Duration) {}))
	_, err := client.FetchRaw(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected a response-too-large error")
	}
	if requestCount != 1 {
		t.Errorf("expected exactly 1 request (a permanent classification must not be retried), got %d", requestCount)
	}
}

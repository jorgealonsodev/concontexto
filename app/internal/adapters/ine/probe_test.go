package ine_test

// Task 9.3/9.4 (RED): the INE analogue of adapters/eurostat's own
// ProbeURL/FetchProbe (task 6.14/6.15) -- a probe request carries an
// explicit, small nult (spec pipeline-operations, "Synthetic daily probe
// against every endpoint": "INE nult=1"), issuing exactly one request,
// through the SAME fetchWithRetry/classification path FetchRaw already
// uses (this package's own client.go doc comment: "the clients already
// classify correctly").

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

func TestFetchProbe_IssuesOneRequestCarryingNult1(t *testing.T) {
	_, fixture := loadDatosSerieFixture(t)
	var requests int32
	var gotQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())
	raw, err := client.FetchProbe(context.Background(), "any-cod", 1)
	if err != nil {
		t.Fatalf("FetchProbe: %v", err)
	}
	if string(raw) != string(fixture) {
		t.Error("expected FetchProbe to return the exact undecoded response bytes")
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Errorf("expected exactly 1 request, got %d", got)
	}
	if got := gotQuery.Get("nult"); got != "1" {
		t.Errorf("expected nult=1, got %q", got)
	}
}

// TestProbeURL_NamesNultInTheURLItself triangulates FetchProbe's own test
// with the string-level shape ProbeURL builds, independent of a real
// round trip -- mirroring adapters/eurostat's own
// TestProbeURL_NamesLastTimePeriodInTheURLItself.
func TestProbeURL_NamesNultInTheURLItself(t *testing.T) {
	client := ine.NewClient("http://example.invalid", nil)
	got := client.ProbeURL("any-cod", 1)
	if !strings.Contains(got, "nult=1") {
		t.Errorf("expected ProbeURL to carry nult=1, got %q", got)
	}
	if !strings.Contains(got, "any-cod") {
		t.Errorf("expected ProbeURL to still carry the COD, got %q", got)
	}
}

// TestFetchProbe_RefusalEnvelopeDecodesToANamedNonRetryableErrorAfterExactlyOneRequest
// proves the probe primitive respects the SAME classification as
// FetchRaw+DecodeSeries (spec "the clients already classify correctly")
// rather than adding a second, looser retry policy for probe traffic:
// FetchProbe itself only issues the transport request (mirroring
// FetchRaw exactly, no body inspection); the volume-restriction envelope
// is only detected once DecodeSeries inspects the 200 body -- and even
// so, exactly one request was ever issued to get there.
func TestFetchProbe_RefusalEnvelopeDecodesToANamedNonRetryableErrorAfterExactlyOneRequest(t *testing.T) {
	body := loadRefusalFixture(t)
	var requests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())
	raw, err := client.FetchProbe(context.Background(), "refused-series", 1)
	if err != nil {
		t.Fatalf("expected FetchProbe itself (transport only) to succeed, got: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Errorf("expected exactly 1 request, got %d", got)
	}

	_, decodeErr := ine.DecodeSeries(raw, "refused-series", indicators.FrequencyQuarterly)
	if decodeErr == nil {
		t.Fatal("expected the volume-restriction envelope to fail decode")
	}
	var classified *sourceerr.Error
	if !errors.As(decodeErr, &classified) || classified.Class != sourceerr.SourceRefusal {
		t.Fatalf("expected a classified sourceerr.SourceRefusal, got %v", decodeErr)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Errorf("expected decode to issue no further requests, still 1, got %d", got)
	}
}

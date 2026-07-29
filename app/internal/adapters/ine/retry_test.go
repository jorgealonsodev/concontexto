package ine_test

// Task 5a.13 (RED): the volume-restriction envelope surfaces as a named
// non-retryable error, and a genuine transport error is retried with
// backoff (spec source-ingestion-ine, "Volume-restriction envelope
// surfaces as a named non-retryable error" + "Recorded fixtures back the
// offline test loop"). This is Finding A from Engram #4690: INE's
// refusal is HTTP 200, so if the client did not classify it as a
// non-retryable sourceerr.SourceRefusal, backoff would loop forever on a
// permanent condition instead of failing fast.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

func TestFetchSeries_RefusalEnvelopeDecodesToANamedNonRetryableError(t *testing.T) {
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

	_, err := client.FetchSeries(context.Background(), "refused-series", indicators.FrequencyQuarterly)
	if err == nil {
		t.Fatal("expected the volume-restriction envelope to fail, got nil")
	}

	var classified *sourceerr.Error
	if !errors.As(err, &classified) {
		t.Fatalf("expected a NAMED *sourceerr.Error, not an opaque decode error, got %T: %v", err, err)
	}
	if classified.Class != sourceerr.SourceRefusal {
		t.Errorf("expected class %s, got %s", sourceerr.SourceRefusal, classified.Class)
	}
	if classified.Message != "No puede mostrarse por restricciones de volumen" {
		t.Errorf("expected the Spanish envelope text preserved verbatim, got %q", classified.Message)
	}
	if classified.Class.Retryable() {
		t.Error("expected SourceRefusal to be non-retryable")
	}
}

func TestFetchSeries_FiveAttemptRetryPolicyIssuesExactlyOneRequestOnRefusal(t *testing.T) {
	body := loadRefusalFixture(t)
	var requests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client(),
		ine.WithMaxAttempts(5),
		ine.WithSleep(func(time.Duration) {}),
	)

	_, err := client.FetchSeries(context.Background(), "refused-series", indicators.FrequencyQuarterly)
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("expected exactly 1 request under a 5-attempt policy on a non-retryable refusal, got %d", got)
	}
}

func TestFetchSeries_TransportErrorRetriesWithBackoffAndSucceeds(t *testing.T) {
	cod, fixture := loadDatosSerieFixture(t)
	var requests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requests, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer server.Close()

	var backoffCalls []int
	client := ine.NewClient(server.URL, server.Client(),
		ine.WithSleep(func(time.Duration) {}),
		ine.WithBackoff(func(attempt int) time.Duration {
			backoffCalls = append(backoffCalls, attempt)
			return time.Millisecond
		}),
	)

	result, err := client.FetchSeries(context.Background(), cod, indicators.FrequencyQuarterly)
	if err != nil {
		t.Fatalf("expected the 503-then-success sequence to eventually succeed, got: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 3 {
		t.Fatalf("expected exactly 3 requests (2 failures + 1 success), got %d", got)
	}
	if len(result.Observations) != 3 {
		t.Fatalf("expected the successful response to decode normally, got %d observations", len(result.Observations))
	}
	if len(backoffCalls) != 2 {
		t.Fatalf("expected backoff to be consulted exactly twice (once per retryable failure), got %v", backoffCalls)
	}
}

// silentEmptyFixture proves a structurally valid, zero-observation
// success body is classified distinctly from both ordinary success and
// SourceRefusal (spec source-ingestion-ine's neighbouring
// data-validation rule 1 backstop only ever sees an EMPTY write because
// this adapter never publishes a silent empty as if it were ordinary
// data).
func TestFetchSeries_ZeroObservationSuccessIsClassifiedSilentEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Nombre": "Some series", "Data": []}`))
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())

	_, err := client.FetchSeries(context.Background(), "empty-series", indicators.FrequencyQuarterly)
	if err == nil {
		t.Fatal("expected a zero-observation success to fail as SilentEmpty, got nil")
	}
	var classified *sourceerr.Error
	if !errors.As(err, &classified) {
		t.Fatalf("expected a classified *sourceerr.Error, got %T: %v", err, err)
	}
	if classified.Class != sourceerr.SilentEmpty {
		t.Errorf("expected class %s, got %s", sourceerr.SilentEmpty, classified.Class)
	}
}

// TestOfflineFixturesExist guards 5a.17's offline requirement structurally:
// both checked-in fixture directories must exist with their source.txt,
// so a future edit cannot silently delete the evidence this suite's
// offline claim depends on.
func TestOfflineFixturesExist(t *testing.T) {
	for _, dir := range []string{
		filepath.Join("testdata", "datos_serie"),
		filepath.Join("testdata", "volume_restriction"),
	} {
		if _, err := os.Stat(filepath.Join(dir, "source.txt")); err != nil {
			t.Errorf("expected %s/source.txt to exist: %v", dir, err)
		}
	}
}

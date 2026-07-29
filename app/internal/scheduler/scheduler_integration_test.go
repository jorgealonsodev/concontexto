package scheduler_test

// Runtime harness for Unit 15 (PR 9a): runs the scheduler one real cycle
// against a real *ine.Client and the checked-in volume-restriction
// fixture (task 5a.13's own testdata) instead of a fake op, proving the
// classification-respecting property end-to-end -- op here is fetch THEN
// decode, the same two-step shape app/internal/ingestion.IngestSeries
// itself uses, because (per probe_test.go's own finding this batch) the
// INE volume-restriction envelope is HTTP 200 and is only classified as
// non-retryable once the body is decoded, not at the transport level
// alone.

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
	"github.com/jorgealonsodev/concontexto/app/internal/scheduler"
)

func TestRun_RealINEClientRefusalEnvelopeIssuesExactlyOneRequestThroughTheScheduler(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "adapters", "ine", "testdata", "volume_restriction", "refusal.json"))
	if err != nil {
		t.Fatalf("reading the checked-in volume-restriction fixture: %v", err)
	}
	var requests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	client := ine.NewClient(server.URL, server.Client())
	op := func(ctx context.Context) error {
		raw, err := client.FetchRaw(ctx, "refused-series")
		if err != nil {
			return err
		}
		_, err = ine.DecodeSeries(raw, "refused-series", indicators.FrequencyQuarterly)
		return err
	}

	r := &scheduler.Runner{SourceID: "ine", Backoff: func(int) time.Duration { return time.Hour }}
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	lastSuccess := now.Add(-1 * time.Hour)

	attempt := r.Run(context.Background(), &lastSuccess, now, op)
	if attempt.Err == nil {
		t.Fatal("expected the real client's classified refusal to fail the scheduled attempt")
	}
	var classified *sourceerr.Error
	if !errors.As(attempt.Err, &classified) || classified.Class != sourceerr.SourceRefusal {
		t.Fatalf("expected the scheduler to surface the adapter's own sourceerr.SourceRefusal classification unmodified, got %v", attempt.Err)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("expected exactly 1 HTTP request through the whole scheduler cycle, got %d", got)
	}
	if attempt.Incident {
		t.Error("expected no incident: only 1h since last success")
	}
}

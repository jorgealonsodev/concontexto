package scheduler_test

// Task 4.11 (RED): the publish-latency watchdog decision (spec
// pipeline-operations, "An ingestion not followed by a rebuild alerts
// operators" / "A within-budget cycle raises nothing"). Pure, offline: now,
// lastIngestionSuccess and manifestGeneratedAt are all explicit parameters,
// matching freshness.Resolve and Runner.Run's own established convention
// (this package's own doc comment) -- PublishLatencyBreached never reads
// the system clock itself.

import (
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/scheduler"
)

func TestPublishLatencyBreached(t *testing.T) {
	budget := 30 * time.Minute
	success := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		now             time.Time
		manifestAt      time.Time
		wantBreached    bool
		wantElapsedMins float64
	}{
		{
			name:            "within budget raises nothing, regardless of the local export",
			now:             success.Add(10 * time.Minute),
			manifestAt:      time.Time{}, // no export has ever run
			wantBreached:    false,
			wantElapsedMins: 10,
		},
		{
			name:            "budget elapsed but the local export already reflects this success",
			now:             success.Add(45 * time.Minute),
			manifestAt:      success.Add(1 * time.Minute), // export ran shortly after ingestion succeeded
			wantBreached:    false,
			wantElapsedMins: 45,
		},
		{
			name:            "budget elapsed and the local export is still older than the success",
			now:             success.Add(45 * time.Minute),
			manifestAt:      success.Add(-2 * time.Hour), // stale export, predates this success
			wantBreached:    true,
			wantElapsedMins: 45,
		},
		{
			name:            "exactly at the budget boundary with a stale export breaches",
			now:             success.Add(budget),
			manifestAt:      time.Time{},
			wantBreached:    true,
			wantElapsedMins: 30,
		},
		{
			name:            "the local export generated at the exact success instant is not stale",
			now:             success.Add(45 * time.Minute),
			manifestAt:      success,
			wantBreached:    false,
			wantElapsedMins: 45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			breached, elapsed := scheduler.PublishLatencyBreached(tt.now, success, tt.manifestAt, budget)
			if breached != tt.wantBreached {
				t.Errorf("PublishLatencyBreached() breached = %v, want %v", breached, tt.wantBreached)
			}
			if got := elapsed.Minutes(); got != tt.wantElapsedMins {
				t.Errorf("PublishLatencyBreached() elapsed = %v minutes, want %v", got, tt.wantElapsedMins)
			}
		})
	}
}

func TestDefaultPublishLatencyBudget(t *testing.T) {
	if scheduler.DefaultPublishLatencyBudget != 30*time.Minute {
		t.Errorf("expected the documented default of 30 minutes (spec pipeline-operations), got %s", scheduler.DefaultPublishLatencyBudget)
	}
}

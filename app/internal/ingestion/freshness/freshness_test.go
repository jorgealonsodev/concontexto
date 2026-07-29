package freshness_test

// Task 5a.7 (RED, pure half): the freshness decision itself is a pure
// function of "when did this source last succeed" and "as of when are
// we asking" — no clock read internally, so the 24h window is testable
// without sleeping or faking time.Now() (same pattern PR 4a established
// for the validation rules). Spec raw-file-archive, "Freshness state is
// tracked per source".

import (
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness"
)

func TestResolve(t *testing.T) {
	asOf := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	twentyFiveHoursAgo := asOf.Add(-25 * time.Hour)
	threeHoursAgo := asOf.Add(-3 * time.Hour)
	exactlyAtTheWindowBoundary := asOf.Add(-freshness.Window)

	tests := []struct {
		name        string
		lastSuccess *time.Time
		want        freshness.State
	}{
		{"25 hours without success marks the source stale", &twentyFiveHoursAgo, freshness.StateFailed},
		{"3 hours ago stays fresh", &threeHoursAgo, freshness.StateFresh},
		{"never succeeded is failed", nil, freshness.StateFailed},
		{"exactly at the window boundary is still fresh (>window, not >=)", &exactlyAtTheWindowBoundary, freshness.StateFresh},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := freshness.Resolve(tt.lastSuccess, asOf)
			if got != tt.want {
				t.Errorf("Resolve(%v, %v) = %v, want %v", tt.lastSuccess, asOf, got, tt.want)
			}
		})
	}
}

// TestResolve_EurostatMaintenanceWindowRetriesWithoutIncident is task
// 6.12/6.13's own real-world scenario: Eurostat publishes scheduled
// maintenance (one was observed for 28 July, 20:00-23:55). A source
// whose last success was 2 hours ago, currently unavailable (mid
// maintenance window), must resolve StateFresh -- i.e. "keep retrying
// on the next scheduled attempt, raise no incident" -- using the SAME
// freshness.Resolve every other source already goes through (design.md
// "extend PR 5a-i's freshness handling, do not fork it": this package
// takes no source identity at all, so "Eurostat maintenance" is not a
// special case in the code, only in this test's own real-world
// grounding of the existing 24h rule).
func TestResolve_EurostatMaintenanceWindowRetriesWithoutIncident(t *testing.T) {
	// 28 July, 22:00 UTC: squarely inside the observed 20:00-23:55
	// maintenance window, with the prior successful ingest at 20:00.
	asOf := time.Date(2026, 7, 28, 22, 0, 0, 0, time.UTC)
	lastSuccess := time.Date(2026, 7, 28, 20, 0, 0, 0, time.UTC)

	state := freshness.Resolve(&lastSuccess, asOf)
	if state != freshness.StateFresh {
		t.Fatalf("expected a 2-hour-old success during a maintenance window to resolve StateFresh, got %v", state)
	}
	if state.RaisesIncident() {
		t.Error("expected StateFresh not to raise an incident -- retry on the next scheduled attempt, no alert")
	}
}

// TestResolve_UnavailabilityBeyondTheWindowEscalatesToIncident is
// 6.12/6.13's other half: unavailability outstripping the 24h window
// (spec raw-file-archive's own escalation point) resolves StateFailed
// AND is incident-worthy -- the two facts a future alert-emission step
// (task 9.7/9.8, not built yet) will need to decide whether to page
// anyone.
func TestResolve_UnavailabilityBeyondTheWindowEscalatesToIncident(t *testing.T) {
	asOf := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	lastSuccess := time.Date(2026, 7, 28, 20, 0, 0, 0, time.UTC) // 40 hours ago

	state := freshness.Resolve(&lastSuccess, asOf)
	if state != freshness.StateFailed {
		t.Fatalf("expected unavailability beyond the 24h window to resolve StateFailed, got %v", state)
	}
	if !state.RaisesIncident() {
		t.Error("expected StateFailed to raise an incident")
	}
}

func TestResolve_OneFailingSourceNeverAffectsAnother(t *testing.T) {
	asOf := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	staleA := asOf.Add(-25 * time.Hour)
	freshB := asOf.Add(-1 * time.Hour)

	gotA := freshness.Resolve(&staleA, asOf)
	gotB := freshness.Resolve(&freshB, asOf)

	if gotA != freshness.StateFailed {
		t.Fatalf("expected source A to resolve failed, got %v", gotA)
	}
	if gotB != freshness.StateFresh {
		t.Fatalf("expected source B to stay fresh despite A being stale, got %v", gotB)
	}
}

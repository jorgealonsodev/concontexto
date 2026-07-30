package main

// Remediation batch (verify-report CRITICAL C6): before this batch,
// runScheduler seeded lastSuccess from an empty in-memory map on every
// process start, so freshness.Resolve(nil, asOf) always returned
// StateFailed for a source's very first (this-process) evaluation --
// meaning the first FAILED cycle after any restart raised a source-down
// incident, even for a source that had succeeded moments before the
// restart. Verified at runtime by the verifier: "source ine has been
// down for over 24h0m0s" fired on a process started milliseconds
// earlier.
//
// These two tests exercise runScheduler's cold-start path directly: a
// FRESH invocation (empty in-memory lastSuccess, exactly what every
// process restart produces) whose only knowledge of a source's true
// history comes from seedLastSuccess -- the same seam startScheduler
// wires to postgres.LastSuccessfulDownloadAttempt in production. Two
// cases distinguish "the persisted history is recent" from "the
// persisted history is stale/absent", exactly the distinction the 24h
// rule exists to make.

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/alerting"
	"github.com/jorgealonsodev/concontexto/app/internal/scheduler"
)

// spyAlertSink records every alert it receives, so a test can assert
// exactly how many source-down incidents a scheduled cycle raised
// without a real transport.
//
// It is mutex-guarded for the same reason schedule_test.go's callCounter
// already is: every test that uses this sink runs runScheduler (or
// startSchedulerLoop) in its OWN goroutine and asserts from the test
// goroutine, so Alert writes and the assertions read the same slice
// concurrently. The channel handshakes those tests use (`failed`) only
// order the OP's invocation against the test body -- runner.Run records
// the alert AFTER the op returns, i.e. strictly after that handshake, so
// nothing synchronises the append. `go test -race` reported exactly that,
// on the slice header and on its backing array. Callers therefore never
// touch `alerts` directly; count/snapshot are the only accessors.
type spyAlertSink struct {
	mu     sync.Mutex
	alerts []alerting.Alert
}

func (s *spyAlertSink) Alert(_ context.Context, a alerting.Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alerts = append(s.alerts, a)
	return nil
}

// count is the race-safe len(alerts) every polling loop needs.
func (s *spyAlertSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.alerts)
}

// snapshot copies the alerts recorded so far, so an assertion can index
// and format them without holding the lock -- and without handing the
// test goroutine a slice the scheduler goroutine may still append to.
func (s *spyAlertSink) snapshot() []alerting.Alert {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]alerting.Alert, len(s.alerts))
	copy(out, s.alerts)
	return out
}

func TestRunScheduler_ColdStartWithARecentPersistedSuccessRaisesNoIncidentOnFailure(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	recentSuccess := base.Add(-2 * time.Hour) // well inside the 24h window

	sink := &spyAlertSink{}
	runner := &scheduler.Runner{SourceID: "ine", Backoff: func(int) time.Duration { return time.Hour }, AlertSink: sink}
	runners := map[string]*scheduler.Runner{"ine": runner}

	// Atomic for the same reason spyAlertSink is mutex-guarded: this
	// closure runs on runScheduler's goroutine while the poll loop below
	// reads the counter from the test goroutine. Today the `failed`
	// handshake happens to order the two (seeding precedes the op), but
	// that ordering is precisely what this test exists to prove -- relying
	// on it to make the counter safe would be circular, and would silently
	// become a genuine race the moment the seeding call moved.
	var seedCalls atomic.Int64
	seedLastSuccess := func(_ context.Context, sourceID string) *time.Time {
		seedCalls.Add(1)
		if sourceID != "ine" {
			// Errorf, not Fatalf: this runs on the scheduler's goroutine,
			// where Fatalf's runtime.Goexit would kill the scheduler loop
			// instead of failing the test.
			t.Errorf("seedLastSuccess called for unexpected source %q", sourceID)
		}
		success := recentSuccess
		return &success
	}

	failed := make(chan struct{}, 1)
	newOp := func(sourceID string) func(context.Context) error {
		return func(context.Context) error {
			failed <- struct{}{}
			return context.Canceled // any non-nil error; classification is scheduler.Runner's concern
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tick := make(chan time.Time)
	// A FRESH invocation: runScheduler's own lastSuccess map starts
	// empty here, exactly as it does after every process restart --
	// this is the cold-start path production always takes.
	go runScheduler(ctx, runners, newOp, 24*time.Hour, tick, seedLastSuccess, nil)

	tick <- base
	select {
	case <-failed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first scheduled cycle")
	}

	// Give runScheduler's goroutine a moment to finish processing the
	// tick (record the alert, if any) before asserting.
	deadline := time.Now().Add(2 * time.Second)
	for seedCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := seedCalls.Load(); got != 1 {
		t.Fatalf("expected seedLastSuccess to be called exactly once for the cold-start source, got %d", got)
	}

	// Poll briefly: the alert (if any) is recorded synchronously inside
	// runner.Run, which already returned by the time <-failed unblocked
	// (failed is sent from inside newOp, called by Run before Run
	// returns), so no alert should ever appear.
	time.Sleep(50 * time.Millisecond)
	if got := sink.snapshot(); len(got) != 0 {
		t.Fatalf("expected NO incident when the persisted last success is recent (cold-start, seeded %s ago), got %d alert(s): %+v",
			base.Sub(recentSuccess), len(got), got)
	}
}

func TestRunScheduler_ColdStartWithAPersistedSuccessOlderThan24hRaisesAnIncidentOnFailure(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	staleSuccess := base.Add(-25 * time.Hour) // outside the 24h window

	sink := &spyAlertSink{}
	runner := &scheduler.Runner{SourceID: "ine", Backoff: func(int) time.Duration { return time.Hour }, AlertSink: sink}
	runners := map[string]*scheduler.Runner{"ine": runner}

	var seedCalls atomic.Int64
	seedLastSuccess := func(context.Context, string) *time.Time {
		seedCalls.Add(1)
		success := staleSuccess
		return &success
	}

	failed := make(chan struct{}, 1)
	newOp := func(sourceID string) func(context.Context) error {
		return func(context.Context) error {
			failed <- struct{}{}
			return context.Canceled
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tick := make(chan time.Time)
	go runScheduler(ctx, runners, newOp, 24*time.Hour, tick, seedLastSuccess, nil)

	tick <- base
	select {
	case <-failed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first scheduled cycle")
	}

	// This is the discriminating assertion: an incident alone does NOT
	// prove the seeding wiring works, because the pre-fix bug (an
	// always-nil lastSuccess) ALSO raises an incident on every cold
	// failure -- for the wrong reason. Requiring seedLastSuccess to have
	// actually been consulted makes this test fail if the seeding call
	// is ever removed, not merely tolerate its absence.
	deadline := time.Now().Add(2 * time.Second)
	for seedCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := seedCalls.Load(); got != 1 {
		t.Fatalf("expected seedLastSuccess to be called exactly once for the cold-start source, got %d", got)
	}

	deadline = time.Now().Add(2 * time.Second)
	for sink.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	alerts := sink.snapshot()
	if len(alerts) != 1 {
		t.Fatalf("expected exactly ONE source-down incident when the persisted last success is %s old (past the 24h window), got %d: %+v",
			base.Sub(staleSuccess), len(alerts), alerts)
	}
	if alerts[0].Kind != alerting.KindSourceDown {
		t.Fatalf("expected a KindSourceDown alert, got %+v", alerts[0])
	}
	if alerts[0].Source != "ine" {
		t.Fatalf("expected the alert to name source ine, got %q", alerts[0].Source)
	}
}

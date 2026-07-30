package main

// Remediation batch (verify-report CRITICAL C8/M2): startSchedulerLoop
// builds seedLastSuccess (postgres.LastSuccessfulDownloadAttempt, the C6
// fix) and hands it to runScheduler. Its pre-extraction predecessor,
// startScheduler, had 0.0% coverage: nothing called it, so a mutation
// replacing that argument with nil left the whole suite green, silently
// restoring the pass-2 C6 defect.
//
// This test drives the REAL composition against a real Postgres pool
// (not a hand-built closure, unlike schedule_freshness_test.go), with a
// source deliberately given an unsupported source_ref kind so its
// scheduled op fails INSTANTLY inside runIngest -- no network, no retry
// backoff, a fast deterministic failure for the cold-start path to react
// to.

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/alerting"
)

func TestStartSchedulerLoop_ColdStartWithARecentPersistedSuccessRaisesNoIncidentOnFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires Docker (testcontainers), disabled by -short")
	}
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("concontexto_schedule_composition_test"),
		tcpostgres.WithUsername("concontexto_schedule_composition_test"),
		tcpostgres.WithPassword("concontexto_schedule_composition_test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("starting postgres container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	var migrateOut, migrateErr bytes.Buffer
	t.Setenv("DATABASE_URL", dsn)
	if code := cmdMigrate([]string{"up"}, &migrateOut, &migrateErr); code != 0 {
		t.Fatalf("migrate up: exit %d, stderr=%q", code, migrateErr.String())
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()

	const sourceID = "test-comp-src"
	cfg := &config.Config{
		Sources: map[string]config.SourceConfig{
			sourceID: {ID: sourceID, Name: "Test", URL: "https://example.test", AccessType: "api-json",
				Licence: config.LicenceConfig{Name: "lic", AttributionText: "attr"}},
		},
		Series: []config.SeriesConfig{
			{Slug: "test-comp-series", Name: "Test", Source: sourceID, Dataset: "test-comp-dataset",
				Unit: "index", Frequency: "M", Decimals: 1, Geo: "ES",
				SourceRefs: []config.SourceRef{{Kind: "unsupported-source-ref-kind", Ref: "X", ValidFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}}},
		},
	}

	if err := postgres.ReconcileDimensions(ctx, pool, cfg, time.Now().UTC()); err != nil {
		t.Fatalf("seeding dimensions: %v", err)
	}

	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	recentSuccess := base.Add(-2 * time.Hour) // well inside the 24h window

	rawFile, _, err := postgres.RecordRawFile(ctx, pool, postgres.RawFile{
		Hash: "test-comp-hash", SourceID: sourceID, URL: "https://example.test/seed",
		DownloadedAt: recentSuccess, StoragePath: "unused", SizeBytes: 1,
	})
	if err != nil {
		t.Fatalf("seeding raw_file: %v", err)
	}
	if _, err := postgres.RecordDownloadAttempt(ctx, pool, postgres.DownloadAttempt{
		SourceID: sourceID, URL: "https://example.test/seed", AttemptedAt: recentSuccess,
		ResultingHash: &rawFile.Hash, Outcome: postgres.OutcomeNewFile,
	}); err != nil {
		t.Fatalf("seeding download_attempt: %v", err)
	}

	// scheduler.NewRunner captures alerting.DefaultSink() AT CONSTRUCTION
	// TIME, and startSchedulerLoop builds its runners internally -- the
	// process-wide default sink is the only seam available here (pass-3
	// verify-report already accepted SetDefaultSink as "a legitimate test
	// seam"). SetDefaultSink happens-before the `go` statement below in
	// program order, which the Go memory model guarantees happens-before
	// the new goroutine's first action -- no race with scheduler.NewRunner
	// reading it inside startSchedulerLoop.
	prevSink := alerting.DefaultSink()
	spy := &spyAlertSink{}
	alerting.SetDefaultSink(spy)
	t.Cleanup(func() { alerting.SetDefaultSink(prevSink) })

	root := t.TempDir()
	staticRoot := t.TempDir()

	loopCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tick := make(chan time.Time)
	var logs bytes.Buffer
	go startSchedulerLoop(loopCtx, pool, cfg, root, staticRoot, tick, &logs)

	tick <- base

	// The op fails synchronously (buildSourceClient errors on the
	// unsupported ref kind before any network call), so a short, generous
	// poll window covers the whole cycle (seed -> run -> alert-or-not).
	deadline := time.Now().Add(2 * time.Second)
	for spy.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	// count/snapshot, never spy.alerts: startSchedulerLoop runs on its own
	// goroutine and records alerts from there (see spyAlertSink's own doc
	// comment for the race `go test -race` caught).
	if got := spy.snapshot(); len(got) != 0 {
		t.Fatalf("expected NO incident when a recent persisted success genuinely exists in postgres "+
			"(cold-start, seeded %s before the tick) -- got %d alert(s): %+v. "+
			"A nil seedLastSuccess reaching runScheduler (verify-report CRITICAL C8/M2) produces exactly this failure.",
			base.Sub(recentSuccess), len(got), got)
	}
}

// TestStartSchedulerLoop_APublishLatencyBreachAlertsWhenTheLocalExportNeverCaughtUp
// pins task 4.11/4.12's REAL production wiring end to end:
// startSchedulerLoop -> publishLatencyWatchdog -> publishing.ReadManifest
// -> scheduler.PublishLatencyBreached -> alerting.PublishLatencyBreach.
// A source's last successful ingestion (real download_attempt row) is well
// past the default 30-minute budget, and STATIC_ROOT points at a directory
// where no export has EVER run (no data-derived/manifest.json at all) --
// the watchdog must raise exactly one KindPublishLatencyBreach alert on
// the tick where the breach first becomes observable.
func TestStartSchedulerLoop_APublishLatencyBreachAlertsWhenTheLocalExportNeverCaughtUp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires Docker (testcontainers), disabled by -short")
	}
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("concontexto_watchdog_test"),
		tcpostgres.WithUsername("concontexto_watchdog_test"),
		tcpostgres.WithPassword("concontexto_watchdog_test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("starting postgres container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	var migrateOut, migrateErr bytes.Buffer
	t.Setenv("DATABASE_URL", dsn)
	if code := cmdMigrate([]string{"up"}, &migrateOut, &migrateErr); code != 0 {
		t.Fatalf("migrate up: exit %d, stderr=%q", code, migrateErr.String())
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()

	const sourceID = "test-watchdog-src"
	cfg := &config.Config{
		Sources: map[string]config.SourceConfig{
			sourceID: {ID: sourceID, Name: "Test", URL: "https://example.test", AccessType: "api-json",
				Licence: config.LicenceConfig{Name: "lic", AttributionText: "attr"}},
		},
		Series: []config.SeriesConfig{
			{Slug: "test-watchdog-series", Name: "Test", Source: sourceID, Dataset: "test-watchdog-dataset",
				Unit: "index", Frequency: "M", Decimals: 1, Geo: "ES",
				SourceRefs: []config.SourceRef{{Kind: "unsupported-source-ref-kind", Ref: "X", ValidFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}}},
		},
	}

	if err := postgres.ReconcileDimensions(ctx, pool, cfg, time.Now().UTC()); err != nil {
		t.Fatalf("seeding dimensions: %v", err)
	}

	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	// One hour before base -- well past the default 30-minute publish-
	// latency budget by the time the watchdog first evaluates it below.
	staleSuccess := base.Add(-1 * time.Hour)

	rawFile, _, err := postgres.RecordRawFile(ctx, pool, postgres.RawFile{
		Hash: "test-watchdog-hash", SourceID: sourceID, URL: "https://example.test/seed",
		DownloadedAt: staleSuccess, StoragePath: "unused", SizeBytes: 1,
	})
	if err != nil {
		t.Fatalf("seeding raw_file: %v", err)
	}
	if _, err := postgres.RecordDownloadAttempt(ctx, pool, postgres.DownloadAttempt{
		SourceID: sourceID, URL: "https://example.test/seed", AttemptedAt: staleSuccess,
		ResultingHash: &rawFile.Hash, Outcome: postgres.OutcomeNewFile,
	}); err != nil {
		t.Fatalf("seeding download_attempt: %v", err)
	}

	prevSink := alerting.DefaultSink()
	spy := &spyAlertSink{}
	alerting.SetDefaultSink(spy)
	t.Cleanup(func() { alerting.SetDefaultSink(prevSink) })

	root := t.TempDir()
	staticRoot := t.TempDir()
	// STATIC_ROOT resolves exportOutputDir's own data-derived path (see
	// export_cmd.go's exportOutputDir/staticAssetRoot) -- no manifest.json
	// is ever written under it in this test, so ReadManifest resolves the
	// "no export has ever run" case.
	t.Setenv("STATIC_ROOT", staticRoot)

	loopCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tick := make(chan time.Time)
	var logs bytes.Buffer
	go startSchedulerLoop(loopCtx, pool, cfg, root, staticRoot, tick, &logs)

	// First tick: seeds lastSuccess from the real, persisted
	// download_attempt above (the op itself fails fast on the unsupported
	// ref kind, which does not matter here -- seeding happens before the
	// op runs). The watchdog cannot fire yet: nothing was known before
	// this tick.
	tick <- base

	// Give the first tick's own cycle (seed -> op) time to complete before
	// asserting nothing fired -- same generous polling window the
	// pre-existing cold-start test above uses for the same reason.
	time.Sleep(300 * time.Millisecond)
	if got := spy.snapshot(); len(got) != 0 {
		t.Fatalf("expected no alert on the very first tick (nothing known yet), got %d: %+v", len(got), got)
	}

	// Second tick: lastSuccess is now known and is already 61+ minutes
	// old -- comfortably past the 30-minute default budget -- with no
	// local export ever written. The watchdog must raise exactly one
	// KindPublishLatencyBreach alert.
	tick <- base.Add(1 * time.Minute)

	deadline := time.Now().Add(2 * time.Second)
	for spy.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	alerts := spy.snapshot()
	if len(alerts) != 1 {
		t.Fatalf("expected exactly 1 alert, got %d: %+v", len(alerts), alerts)
	}
	if alerts[0].Kind != alerting.KindPublishLatencyBreach {
		t.Errorf("expected KindPublishLatencyBreach, got %v", alerts[0].Kind)
	}
	if alerts[0].Source != sourceID {
		t.Errorf("expected the alert to name source %q, got %q", sourceID, alerts[0].Source)
	}
}

// TestStartScheduler_ScheduleDisabledSkipsTheSchedulerEvenWithADatabaseURL
// covers the opt-out that the immediate first tick (verify-report
// WARNING-24, schedulerTicks) makes necessary.
//
// Before that change the scheduler's first cycle landed 15 minutes after
// boot, so any short-lived stack -- CI's container smoke test above all --
// exited long before the pipeline ran and never touched a third-party API.
// Now the first cycle starts at boot, which is exactly the point for a
// real deployment and exactly wrong for a hermetic test stack. This flag
// is how such a stack keeps `serve` serving without also making it fetch
// from INE/Eurostat.
//
// Note the branch order: the flag is honoured BEFORE DATABASE_URL is
// even read, so a disabled scheduler needs no credentials to stay quiet.
func TestStartScheduler_ScheduleDisabledSkipsTheSchedulerEvenWithADatabaseURL(t *testing.T) {
	t.Setenv("APP_SCHEDULE_DISABLED", "true")
	// A syntactically valid DSN pointing at nothing: reaching the connect
	// step at all would be the failure this test exists to catch.
	t.Setenv("DATABASE_URL", "postgres://nobody@127.0.0.1:1/none?sslmode=disable")
	var logs bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startScheduler(ctx, &logs)

	if !bytes.Contains(logs.Bytes(), []byte("APP_SCHEDULE_DISABLED")) {
		t.Fatalf("expected a log line naming APP_SCHEDULE_DISABLED, got %q", logs.String())
	}
	if bytes.Contains(logs.Bytes(), []byte("DATABASE_URL not set")) {
		t.Fatalf("expected the disable check to short-circuit before DATABASE_URL, got %q", logs.String())
	}
}

// TestScheduleDisabled_ParsesTheFlagAndFailsOpen triangulates the flag's
// own parsing. "Fails open" is deliberate and matches every other
// resolver in schedule.go (scheduleInterval, publishLatencyBudget): an
// unparsable value falls back to ENABLED and is logged, because silently
// disabling the entire pipeline over a typo is the worse failure.
func TestScheduleDisabled_ParsesTheFlagAndFailsOpen(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		{name: "unset leaves the scheduler enabled", env: "", want: false},
		{name: "true disables it", env: "true", want: true},
		{name: "1 disables it", env: "1", want: true},
		{name: "false leaves it enabled", env: "false", want: false},
		{name: "an unparsable value fails open", env: "yes-please", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_SCHEDULE_DISABLED", tt.env)
			var logs bytes.Buffer
			if got := scheduleDisabled(&logs); got != tt.want {
				t.Fatalf("scheduleDisabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestStartScheduler_DatabaseURLUnsetDisablesTheScheduler covers
// startScheduler's own early-return branch cheaply (no Docker): a
// missing DATABASE_URL must leave the scheduler disabled and log why,
// never panic -- the same resilience choice runServe makes for a missing
// STATIC_ROOT directory.
func TestStartScheduler_DatabaseURLUnsetDisablesTheScheduler(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	var logs bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startScheduler(ctx, &logs)

	if got := logs.String(); got == "" || !bytes.Contains(logs.Bytes(), []byte("DATABASE_URL not set")) {
		t.Fatalf("expected a DATABASE_URL-not-set log line, got %q", got)
	}
}

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
	for len(spy.alerts) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	if len(spy.alerts) != 0 {
		t.Fatalf("expected NO incident when a recent persisted success genuinely exists in postgres "+
			"(cold-start, seeded %s before the tick) -- got %d alert(s): %+v. "+
			"A nil seedLastSuccess reaching runScheduler (verify-report CRITICAL C8/M2) produces exactly this failure.",
			base.Sub(recentSuccess), len(spy.alerts), spy.alerts)
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

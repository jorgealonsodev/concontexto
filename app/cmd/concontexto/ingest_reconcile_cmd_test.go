package main

// Task 7.15: wire ingestion.ReconcileEditorialConfig into the `ingest`
// subcommand as its own step ("ingest --reconcile"), separate from the
// per-series ingest pipeline (IngestSeries), which is not yet wired to
// the CLI (later phase territory — see the unchanged placeholder
// fallback in ingest_cmd.go).
//
// runIngestReconcile is the testable core (mirrors runValidateConfig's
// pattern in validate_config_cmd.go / runServe's in serve.go): it takes
// an already-open db and an already-loaded *config.Config instead of
// resolving DATABASE_URL / the embedded tree itself, so this test can
// inject both.

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

func TestRunIngestReconcile_ProjectsConfirmedEntriesAndReportsPendingOnes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires Docker (testcontainers), disabled by -short")
	}

	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("concontexto_ingest_reconcile_test"),
		tcpostgres.WithUsername("concontexto_ingest_reconcile_test"),
		tcpostgres.WithPassword("concontexto_ingest_reconcile_test"),
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
	t.Setenv("DATABASE_URL", dsn)

	var stdout, stderr bytes.Buffer
	if code := cmdMigrate([]string{"up"}, &stdout, &stderr); code != 0 {
		t.Fatalf("migrate up: exit %d, stderr=%q", code, stderr.String())
	}

	// Real embedded config, real rupturas.yaml/eventos.yaml/gobiernos.yaml
	// -- the runtime harness for task 7.15.
	stdout.Reset()
	stderr.Reset()
	if code := cmdIngest([]string{"--reconcile"}, &stdout, &stderr); code != 0 {
		t.Fatalf("ingest --reconcile (1st run): exit %d, stderr=%q", code, stderr.String())
	}
	firstOut := stdout.String()
	if !strings.Contains(firstOut, "inserted=") {
		t.Errorf("expected reconcile counts in output, got %q", firstOut)
	}
	for _, pendingID := range []string{
		"epa-cnae2025-doble-codificacion", "cn-revision-base-sept-2025",
		"sec-cambios-deuda-deficit", "ss-cnae2025-afiliacion",
	} {
		if !strings.Contains(firstOut, pendingID) {
			t.Errorf("expected still-unconfirmed break %q listed as pending, got %q", pendingID, firstOut)
		}
	}
	for _, confirmedID := range []string{"ecoicop-v2-2026-ine-ipc", "ecoicop-v2-2026-eurostat-hicp"} {
		if strings.Contains(firstOut, confirmedID) {
			t.Errorf("expected the now-confirmed break %q to NOT be reported as pending, got %q", confirmedID, firstOut)
		}
	}

	// Idempotence (task table's own runtime harness: "ingest --reconcile
	// run twice"): a second run against the identical config changes
	// zero rows.
	stdout.Reset()
	stderr.Reset()
	if code := cmdIngest([]string{"--reconcile"}, &stdout, &stderr); code != 0 {
		t.Fatalf("ingest --reconcile (2nd run): exit %d, stderr=%q", code, stderr.String())
	}
	secondOut := stdout.String()
	if !strings.Contains(secondOut, "inserted=0 updated=0 retired=0") {
		t.Errorf("expected the second run to change zero break rows, got %q", secondOut)
	}
}

func TestCmdIngest_ReconcileFlagRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	var stdout, stderr bytes.Buffer
	code := cmdIngest([]string{"--reconcile"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit 1 when DATABASE_URL is unset, got %d (stdout=%q stderr=%q)", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "DATABASE_URL") {
		t.Errorf("expected stderr to mention DATABASE_URL, got %q", stderr.String())
	}
}

// TestCmdIngest_WithoutAnyTargetFlagPrintsUsage superseded the former
// "not yet implemented" placeholder once ingest --series/--source were
// wired (remediation batch, verify-report CRITICAL C2): a bare `ingest`
// invocation is now genuinely ambiguous (which series? which source?)
// rather than a stub, so it prints usage and fails rather than silently
// exiting 0. See TestCmdIngest_NoTargetFlagPrintsUsageNamingSeriesAndSource
// in ingest_run_cmd_test.go for the fuller assertion on usage content.
func TestCmdIngest_WithoutAnyTargetFlagPrintsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cmdIngest(nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit 1 with no target flag, got %d", code)
	}
	if !strings.Contains(stderr.String(), "ingest:") {
		t.Errorf("expected a usage message on stderr, got %q", stderr.String())
	}
}

// TestRunIngestReconcile_TestableCoreReportsPendingAndInsertedCounts is a
// narrower proof of runIngestReconcile's output formatting against an
// injected config.Config (not the real embedded tree), guarded like
// every other Docker-dependent test in this repository.
func TestRunIngestReconcile_TestableCoreReportsPendingAndInsertedCounts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires Docker (testcontainers), disabled by -short")
	}
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("concontexto_run_ingest_reconcile_test"),
		tcpostgres.WithUsername("concontexto_run_ingest_reconcile_test"),
		tcpostgres.WithPassword("concontexto_run_ingest_reconcile_test"),
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
	t.Setenv("DATABASE_URL", dsn)

	var migrateOut, migrateErr bytes.Buffer
	if code := cmdMigrate([]string{"up"}, &migrateOut, &migrateErr); code != 0 {
		t.Fatalf("migrate up: exit %d, stderr=%q", code, migrateErr.String())
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()

	date := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := &config.Config{
		Breaks: []config.BreakConfig{
			{ID: "epa-metodologia-2021", Kind: "methodology",
				Scope:  config.BreakScopeConfig{Kind: "dataset", Ref: "ine-epa"},
				NoteMD: "test break", Date: &date},
			{ID: "unconfirmed-break", Kind: "methodology", DateStatus: "unconfirmed", Todo: "confirmar",
				Scope: config.BreakScopeConfig{Kind: "dataset", Ref: "ine-epa"}, NoteMD: "pending"},
		},
	}

	var stdout, stderr bytes.Buffer
	if code := runIngestReconcile(ctx, pool, cfg, &stdout, &stderr); code != 0 {
		t.Fatalf("runIngestReconcile: exit %d, stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "inserted=1") {
		t.Errorf("expected exactly 1 break inserted, got %q", out)
	}
	if !strings.Contains(out, "unconfirmed-break") {
		t.Errorf("expected the unconfirmed break to be reported as pending, got %q", out)
	}
}

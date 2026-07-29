package main

// Task 2.18: wire `migrate up|down|status` (task 1.13's skeleton) to the
// postgres adapter's embedded migration files. TestCmdMigrate_RunsAgainstARealPostgresContainer
// is this task's real proof: an end-to-end run against a genuine
// postgres:17-alpine container, guarded by testing.Short() like every
// other Docker-dependent test in this change.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestCmdMigrate_UsesNotConfiguredRunnerWhenDatabaseURLUnset(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	var stdout, stderr bytes.Buffer
	code := cmdMigrate([]string{"status"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit 1 when DATABASE_URL is unset, got %d (stdout=%q stderr=%q)",
			code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "no database configured") {
		t.Errorf("expected the NotConfiguredRunner error, got stderr=%q", stderr.String())
	}
}

func TestCmdMigrate_RunsAgainstARealPostgresContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires Docker (testcontainers), disabled by -short")
	}

	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("concontexto_cmd_test"),
		tcpostgres.WithUsername("concontexto_cmd_test"),
		tcpostgres.WithPassword("concontexto_cmd_test"),
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

	stdout.Reset()
	if code := cmdMigrate([]string{"status"}, &stdout, &stderr); code != 0 {
		t.Fatalf("migrate status: exit %d, stderr=%q", code, stderr.String())
	}
	// Deliberately not hardcoding the exact migration count: this test
	// proves the CLI-to-database wiring, not the current migration set's
	// size (which PR 2b's 0002_observation_rollback_reason legitimately
	// grew). "migration(s) applied" alone, without "no migrations
	// applied", proves at least one migration ran.
	if strings.Contains(stdout.String(), "no migrations applied") || !strings.Contains(stdout.String(), "migration(s) applied") {
		t.Errorf("expected status to report at least one migration applied, got %q", stdout.String())
	}

	// Roll back every applied migration, one step at a time (mirrors
	// postgres.TestMigrationReversibility_RollingBackToZeroLeavesEmptySchema),
	// so this test stays correct regardless of how many migrations exist.
	for steps := 0; ; steps++ {
		if steps > 100 {
			t.Fatal("migrate down did not converge to zero after 100 steps — possible infinite loop")
		}
		stdout.Reset()
		if code := cmdMigrate([]string{"status"}, &stdout, &stderr); code != 0 {
			t.Fatalf("migrate status (checking before down): exit %d, stderr=%q", code, stderr.String())
		}
		if strings.Contains(stdout.String(), "no migrations applied") {
			break
		}
		stdout.Reset()
		if code := cmdMigrate([]string{"down"}, &stdout, &stderr); code != 0 {
			t.Fatalf("migrate down (step %d): exit %d, stderr=%q", steps+1, code, stderr.String())
		}
	}

	stdout.Reset()
	if code := cmdMigrate([]string{"status"}, &stdout, &stderr); code != 0 {
		t.Fatalf("migrate status (after down): exit %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no migrations applied") {
		t.Errorf("expected status to report zero migrations applied after rolling back to zero, got %q", stdout.String())
	}
}

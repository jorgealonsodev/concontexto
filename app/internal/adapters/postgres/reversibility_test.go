package postgres_test

// Task 2.4 (RED) / 2.5 (GREEN): every migration is reversible (spec
// data-model-vintages, "Every migration is reversible"). Rolling back
// one step at a time, down to zero, must leave an empty public schema.

import (
	"context"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

func TestMigrationReversibility_RollingBackToZeroLeavesEmptySchema(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	runner := postgres.NewRunner(tx)

	if err := runner.Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	if names := publicTableNames(t, ctx, tx); len(names) == 0 {
		t.Fatal("expected tables to exist after Up, found none — test setup is broken")
	}

	steps := 0
	for {
		status, err := runner.Status(ctx)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if status == "no migrations applied" {
			break
		}
		if err := runner.Down(ctx); err != nil {
			t.Fatalf("Down (step %d): %v", steps+1, err)
		}
		steps++
		if steps > 100 {
			t.Fatal("Down did not converge to zero after 100 steps — possible infinite loop")
		}
	}

	if steps == 0 {
		t.Fatal("expected at least one Down step to roll back the applied migration")
	}

	names := publicTableNames(t, ctx, tx)
	if len(names) != 0 {
		t.Errorf("expected an empty public schema after rolling back to zero, found: %v", names)
	}
}

func TestMigrationReversibility_DownWithNothingAppliedReturnsANamedError(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	runner := postgres.NewRunner(tx)

	// Nothing has been applied in this fresh transaction: Down must fail
	// clearly rather than silently doing nothing or panicking.
	if err := runner.Down(ctx); err == nil {
		t.Fatal("expected Down to fail when no migration has been applied yet")
	}
}

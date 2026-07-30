package postgres_test

// Task 4.1 (RED) / 4.2 (GREEN): ListActiveEvents is the events half of
// design D-2's read list (the breaks half, ResolveActiveBreaksForSeries,
// already existed -- editorial.go). Unlike series_break, migration 0001's
// event table carries no scope columns at all (id, event_group, name,
// date_start, date_end, note_md, config_digest, retired_at) -- events
// (exogenous shocks, government terms, milestones) are global annotations
// the config schema (config.EventConfig) never scopes to a series,
// dataset or source. ListActiveEvents therefore returns every currently
// active (non-retired) event regardless of seriesID -- an honest,
// disclosed reading of "the editorial events ... that apply to it" given
// the schema this change inherited, not a per-series filter this schema
// has no column to express. seriesID is accepted (matching
// ResolveActiveBreaksForSeries's own signature and leaving room for a
// real per-series scope in a future slice) but is not read.
//
// "An unconfirmed editorial entry is skipped without error" is already
// guaranteed structurally: ingestion.ReconcileEditorialConfig (reconcile.go)
// never projects an entry whose DateStatus is "unconfirmed" into the event
// table at all -- it records the id in PendingEventIDs instead (reported
// on stdout by runIngestReconcile, "counted for operators"). ListActiveEvents
// therefore never has to skip one itself: the event table structurally
// cannot hold an unconfirmed entry.

import (
	"context"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

func TestListActiveEvents_ReturnsEveryNonRetiredConfirmedEvent(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	mustExec(t, ctx, tx, `INSERT INTO event (id, event_group, name, date_start, config_digest)
		VALUES ('crisis-2008', 'exogenous', 'Crisis financiera', '2008-09-15', 'digest1')`)
	mustExec(t, ctx, tx, `INSERT INTO event (id, event_group, name, date_start, config_digest, retired_at)
		VALUES ('retired-event', 'milestones', 'Retired', '2010-01-01', 'digest2', now())`)

	got, err := postgres.ListActiveEvents(ctx, tx, "tasa-de-paro-epa")
	if err != nil {
		t.Fatalf("ListActiveEvents: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly the one non-retired event, got %d: %+v", len(got), got)
	}
	if got[0].ID != "crisis-2008" || got[0].Group != "exogenous" {
		t.Fatalf("unexpected event: %+v", got[0])
	}
}

func TestListActiveEvents_EmptyWhenNoEventsConfigured(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	got, err := postgres.ListActiveEvents(ctx, tx, "tasa-de-paro-epa")
	if err != nil {
		t.Fatalf("ListActiveEvents: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no events, got %d", len(got))
	}
}

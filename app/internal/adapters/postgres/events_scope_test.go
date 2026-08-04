package postgres_test

// The scope the event registry gained, and the per-series filter it makes
// possible.
//
// WHAT THIS REPLACES. events_read_test.go's own package comment records the
// prior state exactly: "migration 0001's event table carries no scope
// columns at all ... ListActiveEvents therefore returns every currently
// active (non-retired) event regardless of seriesID -- an honest, disclosed
// reading ... not a per-series filter this schema has no column to express",
// and ListActiveEvents accepted seriesID "leaving room for a real
// per-series scope in a future slice". This is that slice.
//
// EVERY ENTRY THE REGISTRY HOLDS TODAY IS GLOBAL, and these tests are what
// keep the other three kinds correct while nothing in config/ uses them.
// The read path is not dead either way: the predicate below runs on every
// export, and 'global' is the branch it takes.
//
// THE WIDENING RULE IS THE BREAK REGISTRY'S, deliberately unchanged:
// series ⊂ dataset ⊂ source, resolved in ONE query with no row ever
// duplicated in storage (postgres.ResolveActiveBreaksForSeries, spec "it is
// stored once, not once per series"). What the event registry adds on top is
// a fourth kind, "global", which is what every entry written before this
// change means and therefore what the column defaults to.

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

// seedScopedSeries gives the scope resolver a real series → dataset →
// source chain to widen against, the same shape
// ResolveActiveBreaksForSeries' own tests use.
func seedScopedSeries(t *testing.T, ctx context.Context, tx pgx.Tx) {
	t.Helper()
	mustExec(t, ctx, tx, `INSERT INTO source (id, name, url, license, attribution_text, access_type, config_digest)
		VALUES ('ine', 'INE', 'https://www.ine.es', 'CC BY 4.0', 'INE', 'api-json', 'd')`)
	mustExec(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest) VALUES ('ine-epa', 'ine', 'EPA', 'd')`)
	mustExec(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest) VALUES ('ine-ipc', 'ine', 'IPC', 'd')`)
	mustExec(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('tasa-de-paro-epa', 'ine-epa', 'Tasa de paro', '%', 'Q', 'ES', 2, false, 'd')`)
	mustExec(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('ipc-general', 'ine-ipc', 'IPC general', 'indice', 'M', 'ES', 1, false, 'd')`)
}

func TestListActiveEvents_ResolvesScopeForTheSeriesAsked(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedScopedSeries(t, ctx, tx)

	mustExec(t, ctx, tx, `INSERT INTO event (id, event_group, name, date_start, config_digest, scope_kind, scope_ref)
		VALUES ('pandemia', 'exogenous', 'Pandemia', '2020-03-14', 'd1', 'global', '')`)
	mustExec(t, ctx, tx, `INSERT INTO event (id, event_group, name, date_start, config_digest, scope_kind, scope_ref, source_url)
		VALUES ('hito-epa', 'milestones', 'Hito EPA', '2021-12-31', 'd2', 'dataset', 'ine-epa', 'https://www.ine.es/x')`)
	mustExec(t, ctx, tx, `INSERT INTO event (id, event_group, name, date_start, config_digest, scope_kind, scope_ref)
		VALUES ('hito-ipc', 'milestones', 'Hito IPC', '2022-03-31', 'd3', 'dataset', 'ine-ipc')`)
	mustExec(t, ctx, tx, `INSERT INTO event (id, event_group, name, date_start, config_digest, scope_kind, scope_ref)
		VALUES ('solo-paro', 'milestones', 'Sólo paro', '2019-01-01', 'd4', 'series', 'tasa-de-paro-epa')`)
	mustExec(t, ctx, tx, `INSERT INTO event (id, event_group, name, date_start, config_digest, scope_kind, scope_ref)
		VALUES ('todo-ine', 'milestones', 'Todo INE', '2018-01-01', 'd5', 'source', 'ine')`)

	got, err := postgres.ListActiveEvents(ctx, tx, "tasa-de-paro-epa")
	if err != nil {
		t.Fatalf("ListActiveEvents: %v", err)
	}
	ids := map[string]bool{}
	for _, ev := range got {
		ids[ev.ID] = true
	}
	for _, want := range []string{"pandemia", "hito-epa", "solo-paro", "todo-ine"} {
		if !ids[want] {
			t.Errorf("expected %q to resolve for tasa-de-paro-epa, got %v", want, ids)
		}
	}
	// The whole point: an IPC-scoped entry is noise on an EPA chart, and
	// this is the layer that keeps it off.
	if ids["hito-ipc"] {
		t.Errorf("an ine-ipc-scoped event must NOT resolve for tasa-de-paro-epa, got %v", ids)
	}

	// And the citation survives the read, because it is what makes the
	// entry checkable by the reader rather than merely by the reviewer.
	for _, ev := range got {
		if ev.ID != "hito-epa" {
			continue
		}
		if ev.SourceURL == nil || *ev.SourceURL != "https://www.ine.es/x" {
			t.Errorf("event %q lost its source_url: %+v", ev.ID, ev.SourceURL)
		}
	}
}

// A series this database has never seen must not make the read FAIL. Export
// only ever asks about published series, but the scope resolution is a join
// and a join that errors would take the whole export down for a
// configuration mistake. Global and slug-scoped entries still resolve,
// because neither needs the dataset/source chain to be known.
func TestListActiveEvents_UnknownSeriesStillResolvesGlobalEntries(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	mustExec(t, ctx, tx, `INSERT INTO event (id, event_group, name, date_start, config_digest)
		VALUES ('pandemia', 'exogenous', 'Pandemia', '2020-03-14', 'd1')`)

	got, err := postgres.ListActiveEvents(ctx, tx, "series-que-no-existe")
	if err != nil {
		t.Fatalf("ListActiveEvents: %v", err)
	}
	if len(got) != 1 || got[0].ID != "pandemia" {
		t.Fatalf("expected the global event, got %+v", got)
	}
	// An entry written before the scope columns existed reads back as
	// global rather than as an empty string nobody downstream can branch on.
	if got[0].ScopeKind != "global" {
		t.Errorf("ScopeKind = %q, want global (the column's own default)", got[0].ScopeKind)
	}
}

// Reconcile carries the three columns end to end. Without this, an edit to
// an entry's scope or citation would leave the database holding the old
// value while the YAML claimed the new one — the drift config_digest exists
// to make impossible.
func TestReconcileEvents_PersistsScopeAndSourceURL(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	scoped := postgres.EventInput{
		ID: "hito-epa", Group: "milestones", Name: "Hito EPA",
		DateStart: time.Date(2021, 12, 31, 0, 0, 0, 0, time.UTC), ScopeKind: "dataset", ScopeRef: "ine-epa",
		SourceURL: "https://www.ine.es/x", ConfigDigest: "d1",
	}
	if _, err := postgres.ReconcileEvents(ctx, tx, []postgres.EventInput{scoped}); err != nil {
		t.Fatalf("ReconcileEvents: %v", err)
	}

	rows, err := postgres.ListEvents(ctx, tx)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 event row, got %d", len(rows))
	}
	if rows[0].ScopeKind != "dataset" || rows[0].ScopeRef != "ine-epa" {
		t.Errorf("scope = %s/%s, want dataset/ine-epa", rows[0].ScopeKind, rows[0].ScopeRef)
	}
	if rows[0].SourceURL == nil || *rows[0].SourceURL != "https://www.ine.es/x" {
		t.Errorf("SourceURL = %v, want the methodology url", rows[0].SourceURL)
	}

	// An in-place edit of the scope must UPDATE the same row, never insert
	// a second one: event's natural key is its bare id, so an entry that
	// moves from one dataset to another is the same entry re-scoped.
	moved := scoped
	moved.ScopeRef = "ine-ipc"
	moved.ConfigDigest = "d2"
	counts, err := postgres.ReconcileEvents(ctx, tx, []postgres.EventInput{moved})
	if err != nil {
		t.Fatalf("ReconcileEvents (moved): %v", err)
	}
	if counts.Updated != 1 || counts.Inserted != 0 || counts.Retired != 0 {
		t.Fatalf("expected exactly one in-place update, got %+v", counts)
	}
	rows, err = postgres.ListEvents(ctx, tx)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(rows) != 1 || rows[0].ScopeRef != "ine-ipc" {
		t.Fatalf("expected the one row re-scoped to ine-ipc, got %+v", rows)
	}
}

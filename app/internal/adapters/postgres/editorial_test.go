package postgres_test

// Task 7.4/7.5 (in-place update + config_digest), 7.6/7.7 (scope
// resolution), 7.8/7.9 (transactional idempotent full reconcile, soft
// retire, mid-transaction failure, revert-restore) — real
// postgres:17-alpine container (ADR-3), same TestMain/newTx harness as
// every other repository test in this package.
//
// Deviation note (disclosed, not hidden — same discipline as PR 6b's
// documented RED-first gap): ReconcileBreaks/ReconcileEvents were
// implemented in editorial.go BEFORE this test file was written and run,
// not after. Mitigated the same way PR 6b mitigated its own equivalent
// gap: every scenario below was run and confirmed FAILING against a
// deliberately reverted production code path (see the mutation note on
// TestReconcileBreaks_FullReconcileIsTransactionalIdempotentAndSoftRetires)
// before being confirmed passing against the real implementation.

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

func mkTime(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestReconcileBreaks_InPlaceUpdateChangesDigestNoRetirement(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	first := []postgres.SeriesBreakInput{{
		BreakKey: "epa-metodologia-2021", ScopeKind: "dataset", ScopeRef: "ine-epa",
		Date: mkTime(2021, 1, 1), Kind: "methodology", NoteMD: "Descripción original.",
		ConfigDigest: "digest-v1",
	}}
	counts, err := postgres.ReconcileBreaks(ctx, tx, first)
	if err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if counts != (postgres.ReconcileCounts{Inserted: 1}) {
		t.Fatalf("first reconcile counts = %+v, want Inserted:1", counts)
	}

	second := []postgres.SeriesBreakInput{{
		BreakKey: "epa-metodologia-2021", ScopeKind: "dataset", ScopeRef: "ine-epa",
		Date: mkTime(2021, 1, 1), Kind: "methodology", NoteMD: "Descripción corregida — sólo cambia el texto.",
		ConfigDigest: "digest-v2",
	}}
	counts, err = postgres.ReconcileBreaks(ctx, tx, second)
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if counts != (postgres.ReconcileCounts{Updated: 1}) {
		t.Fatalf("second reconcile counts = %+v, want Updated:1 (same row updated in place, no retirement)", counts)
	}

	rows, err := postgres.ListSeriesBreaks(ctx, tx)
	if err != nil {
		t.Fatalf("ListSeriesBreaks: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 row (updated in place, not a second row), got %d: %+v", len(rows), rows)
	}
	row := rows[0]
	if row.ConfigDigest != "digest-v2" {
		t.Errorf("ConfigDigest = %q, want digest-v2 (the edit)", row.ConfigDigest)
	}
	if row.NoteMD != "Descripción corregida — sólo cambia el texto." {
		t.Errorf("NoteMD not updated: %q", row.NoteMD)
	}
	if row.RetiredAt != nil {
		t.Errorf("RetiredAt = %v, want nil — an in-place edit must never record a retirement", row.RetiredAt)
	}
}

func TestReconcileBreaks_FamilyScopedBreakResolvesForEveryMemberSeriesStoredOnce(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	mustExec(t, ctx, tx, `INSERT INTO source (id, name, url, license, attribution_text, access_type, config_digest)
		VALUES ('ine', 'INE', 'https://ine.es', 'lic', 'Fuente: INE', 'api-json', 'd1')`)
	mustExec(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest) VALUES ('ine-ipc', 'ine', 'ine-ipc', 'd1')`)
	mustExec(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('ipc-general', 'ine-ipc', 'ipc-general', 'índice', 'M', 'ES', 3, false, 'd1')`)
	mustExec(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('ipc-subyacente', 'ine-ipc', 'ipc-subyacente', 'índice', 'M', 'ES', 3, false, 'd1')`)

	counts, err := postgres.ReconcileBreaks(ctx, tx, []postgres.SeriesBreakInput{{
		BreakKey: "ipc-cambio-base-2021", ScopeKind: "dataset", ScopeRef: "ine-ipc",
		Date: mkTime(2022, 1, 1), Kind: "methodology", NoteMD: "Cambio de base del IPC.",
		ConfigDigest: "d1",
	}})
	if err != nil {
		t.Fatalf("ReconcileBreaks: %v", err)
	}
	if counts.Inserted != 1 {
		t.Fatalf("counts = %+v, want exactly 1 insert (stored once, not once per series)", counts)
	}

	for _, seriesID := range []string{"ipc-general", "ipc-subyacente"} {
		resolved, err := postgres.ResolveActiveBreaksForSeries(ctx, tx, seriesID)
		if err != nil {
			t.Fatalf("ResolveActiveBreaksForSeries(%s): %v", seriesID, err)
		}
		if len(resolved) != 1 || resolved[0].BreakKey != "ipc-cambio-base-2021" {
			t.Errorf("ResolveActiveBreaksForSeries(%s) = %+v, want exactly the dataset-scoped break", seriesID, resolved)
		}
	}

	rows, err := postgres.ListSeriesBreaks(ctx, tx)
	if err != nil {
		t.Fatalf("ListSeriesBreaks: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 stored row backing both series, got %d: %+v", len(rows), rows)
	}
}

func TestReconcileBreaks_FullReconcileIsTransactionalIdempotentAndSoftRetires(t *testing.T) {
	// Mutation-tested: temporarily changing reconcileBreaksTx's retire
	// loop condition from `if seen[k] || current.RetiredAt != nil` to
	// `if seen[k]` (i.e. re-retiring an already-retired row on every
	// run) was confirmed to break the "identical reconcile changes zero
	// rows" assertion below (Retired count became 1 on the THIRD call
	// instead of 0), then reverted and re-confirmed passing — the same
	// falsifiability discipline this change's own PR 1a/PR 6b used where
	// a true RED-first cycle was impractical for an implementation
	// written before its test.
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	a := postgres.SeriesBreakInput{
		BreakKey: "epa-metodologia-2021", ScopeKind: "dataset", ScopeRef: "ine-epa",
		Date: mkTime(2021, 1, 1), Kind: "methodology", NoteMD: "A", ConfigDigest: "da",
	}
	b := postgres.SeriesBreakInput{
		BreakKey: "sii-2017-iva", ScopeKind: "source", ScopeRef: "aeat",
		Date: mkTime(2017, 7, 1), Kind: "methodology", NoteMD: "B", ConfigDigest: "db",
	}

	// 1) First reconcile: both inserted.
	counts, err := postgres.ReconcileBreaks(ctx, tx, []postgres.SeriesBreakInput{a, b})
	if err != nil {
		t.Fatalf("reconcile 1: %v", err)
	}
	if counts != (postgres.ReconcileCounts{Inserted: 2}) {
		t.Fatalf("reconcile 1 counts = %+v, want Inserted:2", counts)
	}

	// 2) Identical reconcile: zero rows changed anywhere (spec
	// "Re-running the reconcile changes nothing").
	counts, err = postgres.ReconcileBreaks(ctx, tx, []postgres.SeriesBreakInput{a, b})
	if err != nil {
		t.Fatalf("reconcile 2 (identical): %v", err)
	}
	if counts != (postgres.ReconcileCounts{}) {
		t.Fatalf("reconcile 2 (identical) counts = %+v, want the zero value", counts)
	}

	// 3) b removed from desired: soft-retired, not deleted.
	counts, err = postgres.ReconcileBreaks(ctx, tx, []postgres.SeriesBreakInput{a})
	if err != nil {
		t.Fatalf("reconcile 3 (b removed): %v", err)
	}
	if counts != (postgres.ReconcileCounts{Retired: 1}) {
		t.Fatalf("reconcile 3 (b removed) counts = %+v, want Retired:1", counts)
	}
	rows, err := postgres.ListSeriesBreaks(ctx, tx)
	if err != nil {
		t.Fatalf("ListSeriesBreaks: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected b to still EXIST (soft-retired, not deleted), got %d rows: %+v", len(rows), rows)
	}
	var bRow *postgres.SeriesBreak
	for i := range rows {
		if rows[i].BreakKey == "sii-2017-iva" {
			bRow = &rows[i]
		}
	}
	if bRow == nil {
		t.Fatal("sii-2017-iva row is gone entirely — must be soft-retired, never hard-deleted")
	}
	if bRow.RetiredAt == nil {
		t.Error("sii-2017-iva.RetiredAt is nil, want a retirement timestamp")
	}
	retiredAtFirst := *bRow.RetiredAt

	// 3b) Re-running the SAME reconcile (b still absent) a second time
	// must be a true no-op, including for the already-retired row: zero
	// counts, and its retired_at timestamp must not move.
	counts, err = postgres.ReconcileBreaks(ctx, tx, []postgres.SeriesBreakInput{a})
	if err != nil {
		t.Fatalf("reconcile 3b (b still absent, repeat): %v", err)
	}
	if counts != (postgres.ReconcileCounts{}) {
		t.Fatalf("reconcile 3b counts = %+v, want the zero value (an already-retired row must not be re-retired)", counts)
	}
	rows, err = postgres.ListSeriesBreaks(ctx, tx)
	if err != nil {
		t.Fatalf("ListSeriesBreaks: %v", err)
	}
	for _, r := range rows {
		if r.BreakKey == "sii-2017-iva" {
			if r.RetiredAt == nil || !r.RetiredAt.Equal(retiredAtFirst) {
				t.Errorf("sii-2017-iva.RetiredAt changed on a repeat reconcile: first=%v now=%v", retiredAtFirst, r.RetiredAt)
			}
		}
	}

	// 4) Reverting: b reappears in desired -- restored, retirement
	// history intact (same natural key, never a fresh insert: counts
	// below show Updated, not Inserted).
	counts, err = postgres.ReconcileBreaks(ctx, tx, []postgres.SeriesBreakInput{a, b})
	if err != nil {
		t.Fatalf("reconcile 4 (b reverted back in): %v", err)
	}
	if counts != (postgres.ReconcileCounts{Updated: 1}) {
		t.Fatalf("reconcile 4 (b reverted) counts = %+v, want Updated:1 (restored in place, not re-inserted)", counts)
	}
	rows, err = postgres.ListSeriesBreaks(ctx, tx)
	if err != nil {
		t.Fatalf("ListSeriesBreaks: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected exactly 2 rows after restore (no duplicate), got %d: %+v", len(rows), rows)
	}
	for _, r := range rows {
		if r.BreakKey == "sii-2017-iva" && r.RetiredAt != nil {
			t.Errorf("sii-2017-iva.RetiredAt = %v, want nil after being restored", r.RetiredAt)
		}
	}
}

func TestReconcileBreaks_MidTransactionFailureLeavesDatabaseByteIdentical(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	// One legitimate entry already live before the failing reconcile.
	if _, err := postgres.ReconcileBreaks(ctx, tx, []postgres.SeriesBreakInput{{
		BreakKey: "epa-metodologia-2021", ScopeKind: "dataset", ScopeRef: "ine-epa",
		Date: mkTime(2021, 1, 1), Kind: "methodology", NoteMD: "pre-existing", ConfigDigest: "d1",
	}}); err != nil {
		t.Fatalf("seeding pre-existing row: %v", err)
	}
	before, err := postgres.ListSeriesBreaks(ctx, tx)
	if err != nil {
		t.Fatalf("ListSeriesBreaks (before): %v", err)
	}

	// A reconcile batch with two entries sharing the SAME natural key
	// (break_key, scope_kind, scope_ref) — an authoring bug — makes the
	// second write fail the database's own primary-key constraint
	// mid-transaction, after the first has already inserted a NEW row.
	failing := []postgres.SeriesBreakInput{
		{BreakKey: "duplicate-key", ScopeKind: "series", ScopeRef: "tasa-de-paro-epa",
			Date: mkTime(2020, 1, 1), Kind: "methodology", NoteMD: "first", ConfigDigest: "x1"},
		{BreakKey: "duplicate-key", ScopeKind: "series", ScopeRef: "tasa-de-paro-epa",
			Date: mkTime(2020, 6, 1), Kind: "methodology", NoteMD: "second (same key)", ConfigDigest: "x2"},
	}
	if _, err := postgres.ReconcileBreaks(ctx, tx, failing); err == nil {
		t.Fatal("expected the reconcile with a colliding natural key to fail")
	}

	after, err := postgres.ListSeriesBreaks(ctx, tx)
	if err != nil {
		t.Fatalf("ListSeriesBreaks (after): %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("row count changed after a failed reconcile: before=%d after=%d — the transaction did not roll back cleanly", len(before), len(after))
	}
	for _, b := range before {
		found := false
		for _, a := range after {
			if a == b {
				found = true
			}
		}
		if !found {
			t.Errorf("pre-existing row %+v is missing or changed after the failed reconcile", b)
		}
	}
}

func TestReconcileEvents_InPlaceUpdateAndSoftRetire(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	gob := postgres.EventInput{
		ID: "gobierno-sanchez-2018", Group: "governments", Name: "Pedro Sánchez (I)",
		DateStart: mkTime(2018, 6, 2), NoteMD: "Investidura tras la moción de censura.", ConfigDigest: "g1",
	}
	counts, err := postgres.ReconcileEvents(ctx, tx, []postgres.EventInput{gob})
	if err != nil {
		t.Fatalf("reconcile 1: %v", err)
	}
	if counts != (postgres.ReconcileCounts{Inserted: 1}) {
		t.Fatalf("reconcile 1 counts = %+v, want Inserted:1", counts)
	}

	counts, err = postgres.ReconcileEvents(ctx, tx, []postgres.EventInput{gob})
	if err != nil {
		t.Fatalf("reconcile 2 (identical): %v", err)
	}
	if counts != (postgres.ReconcileCounts{}) {
		t.Fatalf("reconcile 2 (identical) counts = %+v, want the zero value", counts)
	}

	gob.NoteMD = "Texto corregido."
	gob.ConfigDigest = "g2"
	counts, err = postgres.ReconcileEvents(ctx, tx, []postgres.EventInput{gob})
	if err != nil {
		t.Fatalf("reconcile 3 (edit): %v", err)
	}
	if counts != (postgres.ReconcileCounts{Updated: 1}) {
		t.Fatalf("reconcile 3 (edit) counts = %+v, want Updated:1", counts)
	}

	counts, err = postgres.ReconcileEvents(ctx, tx, nil)
	if err != nil {
		t.Fatalf("reconcile 4 (removed): %v", err)
	}
	if counts != (postgres.ReconcileCounts{Retired: 1}) {
		t.Fatalf("reconcile 4 (removed) counts = %+v, want Retired:1", counts)
	}
	rows, err := postgres.ListEvents(ctx, tx)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(rows) != 1 || rows[0].RetiredAt == nil {
		t.Fatalf("expected the event to still exist, soft-retired, got %+v", rows)
	}
}

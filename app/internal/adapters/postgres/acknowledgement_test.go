package postgres_test

// RED for the acknowledgement registry's DATABASE half (spec
// data-validation, "Acknowledged findings") — real postgres:17-alpine
// container (ADR-3), same TestMain/newTx harness as every other repository
// test in this package.
//
// The reconcile discipline asserted here is deliberately the SAME one
// series_break and event already follow (editorial.go's package doc
// comment): insert, update in place, soft-retire, and zero writes on an
// identical re-run. An approvals registry is exactly the kind of record
// that must never be hard-deleted (principle P7), so it inherits that
// discipline rather than inventing a new one.

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

func ackInput() postgres.AcknowledgementInput {
	return postgres.AcknowledgementInput{
		AckKey: "ocupados-epa-2020-q2-covid", SeriesID: "ocupados-epa", Period: "2020-Q2",
		Rule: "rule3-plausibility", Value: 18607.2,
		AcknowledgedBy: "Jorge Alonso", AcknowledgedOn: mkTime(2026, time.July, 30),
		NoteMD: "Confinamiento por la COVID-19.", SourceURL: "https://www.ine.es/daco/daco42/daco4211/epa0220.pdf",
		ConfigDigest: "digest-v1",
	}
}

// TestReconcileAcknowledgements_InsertsUpdatesInPlaceAndSoftRetires pins
// the whole lifecycle in one pass, mirroring
// TestReconcileBreaks_FullReconcileIsTransactionalIdempotentAndSoftRetires.
func TestReconcileAcknowledgements_InsertsUpdatesInPlaceAndSoftRetires(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	counts, err := postgres.ReconcileAcknowledgements(ctx, tx, []postgres.AcknowledgementInput{ackInput()})
	if err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if counts != (postgres.ReconcileCounts{Inserted: 1}) {
		t.Fatalf("first reconcile counts = %+v, want Inserted:1", counts)
	}

	// An identical re-run must issue ZERO writes (spec editorial-config,
	// "Re-running the reconcile changes nothing").
	counts, err = postgres.ReconcileAcknowledgements(ctx, tx, []postgres.AcknowledgementInput{ackInput()})
	if err != nil {
		t.Fatalf("identical reconcile: %v", err)
	}
	if counts != (postgres.ReconcileCounts{}) {
		t.Fatalf("an identical reconcile must change zero rows, got %+v", counts)
	}

	// Editing the prose updates the same row in place; no retirement.
	edited := ackInput()
	edited.NoteMD = "Confinamiento por la COVID-19; redacción corregida."
	edited.ConfigDigest = "digest-v2"
	counts, err = postgres.ReconcileAcknowledgements(ctx, tx, []postgres.AcknowledgementInput{edited})
	if err != nil {
		t.Fatalf("edited reconcile: %v", err)
	}
	if counts != (postgres.ReconcileCounts{Updated: 1}) {
		t.Fatalf("editing the note must update in place, got %+v", counts)
	}

	// Removing it from the YAML soft-retires it — an approval is audit
	// history and must never be hard-deleted (principle P7).
	counts, err = postgres.ReconcileAcknowledgements(ctx, tx, nil)
	if err != nil {
		t.Fatalf("empty reconcile: %v", err)
	}
	if counts != (postgres.ReconcileCounts{Retired: 1}) {
		t.Fatalf("removing the entry must soft-retire it, got %+v", counts)
	}
	rows, err := postgres.ListAcknowledgements(ctx, tx)
	if err != nil {
		t.Fatalf("ListAcknowledgements: %v", err)
	}
	if len(rows) != 1 || rows[0].RetiredAt == nil {
		t.Fatalf("expected exactly one soft-retired row, got %+v", rows)
	}

	// Restoring the YAML un-retires the SAME row, never a fresh insert.
	counts, err = postgres.ReconcileAcknowledgements(ctx, tx, []postgres.AcknowledgementInput{ackInput()})
	if err != nil {
		t.Fatalf("restoring reconcile: %v", err)
	}
	if counts != (postgres.ReconcileCounts{Updated: 1}) {
		t.Fatalf("restoring must un-retire the same row, got %+v", counts)
	}
	rows, err = postgres.ListAcknowledgements(ctx, tx)
	if err != nil {
		t.Fatalf("ListAcknowledgements: %v", err)
	}
	if len(rows) != 1 || rows[0].RetiredAt != nil {
		t.Fatalf("expected exactly one live row after restore, got %+v", rows)
	}
}

// TestResolveActiveAcknowledgementsForSeries_IsScopedToOneSeriesAndSkipsRetired
// is the read side ingestion uses. It must never leak another series'
// approvals: the resolver is the last line of defence behind the config
// gate's scope rules.
func TestResolveActiveAcknowledgementsForSeries_IsScopedToOneSeriesAndSkipsRetired(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	other := ackInput()
	other.AckKey, other.SeriesID = "parados-epa-2020-q2-covid", "parados-epa"
	if _, err := postgres.ReconcileAcknowledgements(ctx, tx, []postgres.AcknowledgementInput{ackInput(), other}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	resolved, err := postgres.ResolveActiveAcknowledgementsForSeries(ctx, tx, "ocupados-epa")
	if err != nil {
		t.Fatalf("ResolveActiveAcknowledgementsForSeries: %v", err)
	}
	if len(resolved) != 1 || resolved[0].SeriesID != "ocupados-epa" {
		t.Fatalf("expected exactly ocupados-epa's own acknowledgement, got %+v", resolved)
	}
	if resolved[0].Value != 18607.2 {
		t.Errorf("the pinned value must survive the numeric round trip exactly, got %v", resolved[0].Value)
	}
	if resolved[0].AcknowledgedBy != "Jorge Alonso" {
		t.Errorf("expected the acknowledging human to be carried through, got %q", resolved[0].AcknowledgedBy)
	}

	// A retired acknowledgement resolves for nobody.
	if _, err := postgres.ReconcileAcknowledgements(ctx, tx, []postgres.AcknowledgementInput{other}); err != nil {
		t.Fatalf("retiring reconcile: %v", err)
	}
	resolved, err = postgres.ResolveActiveAcknowledgementsForSeries(ctx, tx, "ocupados-epa")
	if err != nil {
		t.Fatalf("ResolveActiveAcknowledgementsForSeries after retirement: %v", err)
	}
	if len(resolved) != 0 {
		t.Fatalf("a retired acknowledgement must resolve for nobody, got %+v", resolved)
	}
}

// TestAcknowledgementSchema_RefusesTwoLiveRecordsForOneFinding: the same
// invariant validate-config enforces, enforced again by PostgreSQL itself,
// so a hand-edited database cannot hold two conflicting approvals for one
// finding — the same "the database enforces it, not application code"
// discipline the one_current_row index already established (ADR-4).
func TestAcknowledgementSchema_RefusesTwoLiveRecordsForOneFinding(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	duplicate := ackInput()
	duplicate.AckKey = "ocupados-epa-2020-q2-covid-again"
	_, err := postgres.ReconcileAcknowledgements(ctx, tx,
		[]postgres.AcknowledgementInput{ackInput(), duplicate})
	if err == nil {
		t.Fatalf("expected PostgreSQL to refuse two live acknowledgements covering the same (series, period, rule)")
	}
}

package ingestion_test

// Task 7.4/7.5, 7.8/7.9 (orchestration level): ReconcileEditorialConfig
// wires config.Config into postgres.ReconcileBreaks/ReconcileEvents,
// computes each entry's config_digest, and deliberately skips any
// date-unconfirmed entry rather than guessing its date (see reconcile.go's
// own doc comment). Task 7.6/7.7's scope-resolution proof and 7.8's full
// transactional/soft-retire/mid-failure/revert-restore proofs live at the
// postgres-package level (app/internal/adapters/postgres/editorial_test.go)
// — this file proves the ingestion-level orchestration on top of them.

import (
	"context"
	"io/fs"
	"slices"
	"sort"
	"testing"
	"time"

	configdata "github.com/jorgealonsodev/concontexto"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion"
)

func TestReconcileEditorialConfig_ProjectsConfirmedEntriesAndSkipsUnconfirmedDates(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	confirmed := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := config.Config{
		Breaks: []config.BreakConfig{
			{ID: "epa-metodologia-2021", Kind: "methodology",
				Scope:  config.BreakScopeConfig{Kind: "dataset", Ref: "ine-epa"},
				NoteMD: "Cambio metodológico de la EPA (base 2021).", Date: &confirmed},
			{ID: "epa-cnae2025-doble-codificacion", DateStatus: "unconfirmed", Todo: "confirmar",
				Kind: "methodology", Scope: config.BreakScopeConfig{Kind: "dataset", Ref: "ine-epa"},
				NoteMD: "Doble codificación CNAE 2025 en la EPA; fecha pendiente."},
		},
		Events: []config.EventConfig{
			{ID: "crisis-financiera-2008-2013", Group: "exogenous", Name: "Crisis financiera global",
				DateStart: &confirmed, NoteMD: "..."},
		},
	}

	result, err := ingestion.ReconcileEditorialConfig(ctx, tx, cfg)
	if err != nil {
		t.Fatalf("ReconcileEditorialConfig: %v", err)
	}
	if result.Breaks.Inserted != 1 {
		t.Errorf("Breaks.Inserted = %d, want 1 (only the confirmed entry)", result.Breaks.Inserted)
	}
	if len(result.PendingBreakIDs) != 1 || result.PendingBreakIDs[0] != "epa-cnae2025-doble-codificacion" {
		t.Errorf("PendingBreakIDs = %v, want [epa-cnae2025-doble-codificacion]", result.PendingBreakIDs)
	}
	if result.Events.Inserted != 1 {
		t.Errorf("Events.Inserted = %d, want 1", result.Events.Inserted)
	}

	rows, err := postgres.ListSeriesBreaks(ctx, tx)
	if err != nil {
		t.Fatalf("ListSeriesBreaks: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 series_break row — the unconfirmed entry must NEVER be written with a guessed date, got %d: %+v", len(rows), rows)
	}

	// Idempotence at the orchestration level too (spec "Re-running the
	// reconcile changes nothing").
	result2, err := ingestion.ReconcileEditorialConfig(ctx, tx, cfg)
	if err != nil {
		t.Fatalf("ReconcileEditorialConfig (repeat): %v", err)
	}
	if result2.Breaks != (postgres.ReconcileCounts{}) || result2.Events != (postgres.ReconcileCounts{}) {
		t.Fatalf("repeat reconcile counts = breaks:%+v events:%+v, want both the zero value", result2.Breaks, result2.Events)
	}
	if len(result2.PendingBreakIDs) != 1 {
		t.Errorf("PendingBreakIDs on repeat = %v, want the unconfirmed entry still listed as pending", result2.PendingBreakIDs)
	}
}

func TestReconcileEditorialConfig_EditingOnlyADescriptionUpdatesInPlaceAndChangesTheDigest(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	date := time.Date(2017, 7, 1, 0, 0, 0, 0, time.UTC)
	cfg := config.Config{Breaks: []config.BreakConfig{
		{ID: "sii-2017-iva", Kind: "methodology",
			Scope:  config.BreakScopeConfig{Kind: "source", Ref: "aeat"},
			NoteMD: "Texto original.", Date: &date},
	}}
	if _, err := ingestion.ReconcileEditorialConfig(ctx, tx, cfg); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	before, err := postgres.ListSeriesBreaks(ctx, tx)
	if err != nil {
		t.Fatalf("ListSeriesBreaks (before): %v", err)
	}

	cfg.Breaks[0].NoteMD = "Texto corregido — sólo cambia la descripción."
	result, err := ingestion.ReconcileEditorialConfig(ctx, tx, cfg)
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if result.Breaks.Updated != 1 || result.Breaks.Inserted != 0 {
		t.Fatalf("counts = %+v, want exactly Updated:1, Inserted:0", result.Breaks)
	}

	after, err := postgres.ListSeriesBreaks(ctx, tx)
	if err != nil {
		t.Fatalf("ListSeriesBreaks (after): %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("row count changed on an in-place edit: before=%d after=%d", len(before), len(after))
	}
	if after[0].ConfigDigest == before[0].ConfigDigest {
		t.Error("ConfigDigest did not change after editing note_md")
	}
	if after[0].RetiredAt != nil {
		t.Error("editing a description must never record a retirement")
	}
}

// TestReconcileEditorialConfig_ECOICOPv2BreakIsConfirmedAndScopedToDisaggregationsNotHeadlineAggregates
// is task 7.12 (RED) / 7.13 (GREEN)'s end-to-end proof, against the REAL
// embedded config/rupturas.yaml (not a hand-typed fixture): the ECOICOP
// ver.2 break's effective date is now confirmed (2026-01, Commission
// Delegated Regulation (EU) 2024/3159 — see rupturas.yaml's own header
// comment for citations), so it must be PROJECTED, not left pending.
//
// Scope narrowing (orchestrator-confirmed, re-scoping PR 7a's original
// "every CPI-derived series" framing): INE states explicitly that
// aggregates surviving the reclassification are LINKED — "se realizaron
// los enlaces necesarios para tener series históricas completas, sin
// modificar las tasas de variación previamente publicadas". Fase 0's
// three configured IPC-family series (ipc-general, ipc-subyacente,
// ipc-armonizado-eurostat) are exactly such surviving aggregates, so the
// break must NOT resolve for any of them — asserted below alongside a
// positive proof that the SAME break correctly resolves for a genuine
// COICOP-subclass-level disaggregation series, so the exclusion is a
// deliberate scope choice, not an accident of nothing matching.
func TestReconcileEditorialConfig_ECOICOPv2BreakIsConfirmedAndScopedToDisaggregationsNotHeadlineAggregates(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	mustExecT(t, ctx, tx, `INSERT INTO source (id, name, url, license, attribution_text, access_type, config_digest)
		VALUES ('ine', 'INE', 'https://ine.es', 'lic', 'Fuente: INE', 'api-json', 'd1')`)
	mustExecT(t, ctx, tx, `INSERT INTO source (id, name, url, license, attribution_text, access_type, config_digest)
		VALUES ('eurostat', 'Eurostat', 'https://ec.europa.eu/eurostat', 'lic', 'Fuente: Eurostat', 'api-json', 'd1')`)
	mustExecT(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest) VALUES ('ine-ipc', 'ine', 'ine-ipc', 'd1')`)
	mustExecT(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest) VALUES ('ine-ipc-subclases', 'ine', 'ine-ipc-subclases', 'd1')`)
	mustExecT(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest) VALUES ('eurostat-hicp', 'eurostat', 'eurostat-hicp', 'd1')`)
	mustExecT(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest) VALUES ('eurostat-hicp-subclases', 'eurostat', 'eurostat-hicp-subclases', 'd1')`)
	// The three IPC-family series ALREADY configured in Fase 0 (headline
	// aggregates, expected LINKED — no break).
	mustExecT(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('ipc-general', 'ine-ipc', 'ipc-general', 'índice', 'M', 'ES', 3, false, 'd1')`)
	mustExecT(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('ipc-subyacente', 'ine-ipc', 'ipc-subyacente', 'índice', 'M', 'ES', 3, false, 'd1')`)
	mustExecT(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('ipc-armonizado-eurostat', 'eurostat-hicp', 'ipc-armonizado-eurostat', '% variación interanual', 'M', 'ES', 1, true, 'd1')`)
	// Representative COICOP-subclass-level disaggregation series — NOT
	// part of Fase 0's six configured series, seeded here only to prove
	// the break's scope resolution mechanism actually fires for its
	// intended scope, not just that it happens to match nothing.
	mustExecT(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('ipc-alimentos-no-elaborados-coicop', 'ine-ipc-subclases', 'ipc-alimentos-no-elaborados-coicop', 'índice', 'M', 'ES', 3, false, 'd1')`)
	mustExecT(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('hicp-alimentos-no-elaborados-coicop', 'eurostat-hicp-subclases', 'hicp-alimentos-no-elaborados-coicop', '% variación interanual', 'M', 'ES', 1, true, 'd1')`)

	sub, err := fs.Sub(configdata.FS, "config")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	cfg, err := config.Load(sub)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	result, err := ingestion.ReconcileEditorialConfig(ctx, tx, *cfg)
	if err != nil {
		t.Fatalf("ReconcileEditorialConfig: %v", err)
	}

	for _, pending := range result.PendingBreakIDs {
		if pending == "ecoicop-v2-2026-ine-ipc" || pending == "ecoicop-v2-2026-eurostat-hicp" {
			t.Errorf("ECOICOP v2 break %q is still listed as pending (unconfirmed date) — its effective date (2026-01) is now confirmed and must be projected", pending)
		}
	}

	rows, err := postgres.ListSeriesBreaks(ctx, tx)
	if err != nil {
		t.Fatalf("ListSeriesBreaks: %v", err)
	}
	byKey := map[string]postgres.SeriesBreak{}
	for _, r := range rows {
		byKey[r.BreakKey] = r
	}

	wantJan2026 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, id := range []string{"ecoicop-v2-2026-ine-ipc", "ecoicop-v2-2026-eurostat-hicp"} {
		row, ok := byKey[id]
		if !ok {
			t.Fatalf("expected a reconciled series_break row for %q, found none among %+v", id, rows)
		}
		if !row.Date.Equal(wantJan2026) {
			t.Errorf("%s.Date = %v, want 2026-01-01 (confirmed)", id, row.Date)
		}
		if row.SourceURL == nil || *row.SourceURL == "" {
			t.Errorf("%s.SourceURL is empty, want a link to the source methodological note", id)
		}
		if row.RetiredAt != nil {
			t.Errorf("%s.RetiredAt = %v, want nil", id, row.RetiredAt)
		}
	}

	// Scope narrowing: neither ECOICOP v2 break may resolve for a
	// headline aggregate that INE/Eurostat treat as linked.
	for _, headline := range []string{"ipc-general", "ipc-subyacente"} {
		resolved, err := postgres.ResolveActiveBreaksForSeries(ctx, tx, headline)
		if err != nil {
			t.Fatalf("ResolveActiveBreaksForSeries(%s): %v", headline, err)
		}
		for _, b := range resolved {
			if b.BreakKey == "ecoicop-v2-2026-ine-ipc" {
				t.Errorf("ECOICOP v2 break resolved for headline series %q — INE states this aggregate is linked, its previously published rates unchanged; this must NOT be flagged as a break", headline)
			}
		}
	}
	resolvedHICP, err := postgres.ResolveActiveBreaksForSeries(ctx, tx, "ipc-armonizado-eurostat")
	if err != nil {
		t.Fatalf("ResolveActiveBreaksForSeries(ipc-armonizado-eurostat): %v", err)
	}
	for _, b := range resolvedHICP {
		if b.BreakKey == "ecoicop-v2-2026-eurostat-hicp" {
			t.Error("ECOICOP v2 break resolved for the headline harmonised series ipc-armonizado-eurostat — this entry's scope must exclude it")
		}
	}

	// Positive proof: the SAME break DOES resolve for a genuine
	// COICOP-subclass-level disaggregation series.
	resolvedDisagg, err := postgres.ResolveActiveBreaksForSeries(ctx, tx, "ipc-alimentos-no-elaborados-coicop")
	if err != nil {
		t.Fatalf("ResolveActiveBreaksForSeries(disaggregation): %v", err)
	}
	foundINE := false
	for _, b := range resolvedDisagg {
		if b.BreakKey == "ecoicop-v2-2026-ine-ipc" {
			foundINE = true
		}
	}
	if !foundINE {
		t.Error("expected the ECOICOP v2 INE break to resolve for a COICOP-subclass-level disaggregation series, found none")
	}

	resolvedHICPDisagg, err := postgres.ResolveActiveBreaksForSeries(ctx, tx, "hicp-alimentos-no-elaborados-coicop")
	if err != nil {
		t.Fatalf("ResolveActiveBreaksForSeries(HICP disaggregation): %v", err)
	}
	foundHICP := false
	for _, b := range resolvedHICPDisagg {
		if b.BreakKey == "ecoicop-v2-2026-eurostat-hicp" {
			foundHICP = true
		}
	}
	if !foundHICP {
		t.Error("expected the ECOICOP v2 Eurostat break to resolve for a COICOP-subclass-level HICP disaggregation series, found none")
	}
}

// TestReconcileEditorialConfig_MidTransactionFailureAcrossBreaksAndEventsLeavesDatabaseByteIdentical
// closes PR 7a's disclosed gap (apply-progress Deviation 3): breaks and
// events used to reconcile as TWO separate top-level transactions, so a
// failure in the events half could leave an already-committed breaks
// change durable — violating spec editorial-config's "A failed reconcile
// leaves no partial state" at the ReconcileEditorialConfig level (the
// spec scenario is written against ONE reconcile, not "the breaks half
// of one reconcile"). ReconcileEditorialConfig now delegates to
// postgres.ReconcileEditorial, which reconciles both tables inside ONE
// transaction.
func TestReconcileEditorialConfig_MidTransactionFailureAcrossBreaksAndEventsLeavesDatabaseByteIdentical(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	date := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	pre := config.Config{
		Breaks: []config.BreakConfig{
			{ID: "epa-metodologia-2021", Kind: "methodology",
				Scope:  config.BreakScopeConfig{Kind: "dataset", Ref: "ine-epa"},
				NoteMD: "pre-existing break", Date: &date},
		},
		Events: []config.EventConfig{
			{ID: "crisis-financiera-2008-2013", Group: "exogenous", Name: "Crisis",
				DateStart: &date, NoteMD: "pre-existing event"},
		},
	}
	if _, err := ingestion.ReconcileEditorialConfig(ctx, tx, pre); err != nil {
		t.Fatalf("seeding pre-existing state: %v", err)
	}
	breaksBefore, err := postgres.ListSeriesBreaks(ctx, tx)
	if err != nil {
		t.Fatalf("ListSeriesBreaks (before): %v", err)
	}
	eventsBefore, err := postgres.ListEvents(ctx, tx)
	if err != nil {
		t.Fatalf("ListEvents (before): %v", err)
	}

	// Next desired state: a NEW break that would insert cleanly on its
	// own, PLUS two events sharing the same id -- an authoring bug that
	// makes the events half fail its own primary-key constraint
	// mid-transaction. If breaks/events reconcile as ONE transaction,
	// the new break must NOT be committed either.
	failing := config.Config{
		Breaks: []config.BreakConfig{
			pre.Breaks[0],
			{ID: "sii-2017-iva", Kind: "methodology",
				Scope:  config.BreakScopeConfig{Kind: "source", Ref: "aeat"},
				NoteMD: "would insert cleanly on its own", Date: &date},
		},
		Events: []config.EventConfig{
			pre.Events[0],
			{ID: "duplicate-event-id", Group: "exogenous", Name: "First", DateStart: &date, NoteMD: "first"},
			{ID: "duplicate-event-id", Group: "exogenous", Name: "Second", DateStart: &date, NoteMD: "second (same id)"},
		},
	}
	if _, err := ingestion.ReconcileEditorialConfig(ctx, tx, failing); err == nil {
		t.Fatal("expected the reconcile to fail (duplicate event id in one batch)")
	}

	breaksAfter, err := postgres.ListSeriesBreaks(ctx, tx)
	if err != nil {
		t.Fatalf("ListSeriesBreaks (after): %v", err)
	}
	if len(breaksAfter) != len(breaksBefore) {
		t.Fatalf("series_break row count changed after a failed reconcile: before=%d after=%d — the new break was committed even though the SAME reconcile's events half failed", len(breaksBefore), len(breaksAfter))
	}
	for _, b := range breaksAfter {
		if b.BreakKey == "sii-2017-iva" {
			t.Error("break \"sii-2017-iva\" was persisted despite the events half of the SAME reconcile failing — breaks and events are not reconciling as one transaction")
		}
	}

	eventsAfter, err := postgres.ListEvents(ctx, tx)
	if err != nil {
		t.Fatalf("ListEvents (after): %v", err)
	}
	if len(eventsAfter) != len(eventsBefore) {
		t.Fatalf("event row count changed after a failed reconcile: before=%d after=%d", len(eventsBefore), len(eventsAfter))
	}
}

// Re-scoping an event is an EDIT of that event, and must register as one: a
// new digest, an in-place update, a visible count.
//
// The digest is what makes a reconcile idempotent, so anything reaching the
// database that the digest does NOT cover is a field that can drift silently
// — the YAML claiming one thing while the row holds another, with a repeat
// run reporting zero changes. The scope decides WHICH CHARTS the entry
// appears on and the citation is what makes it checkable by a reader, so a
// silent drift in either is a reader-facing defect rather than a bookkeeping
// one.
func TestReconcileEditorialConfig_ReScopingAnEventIsAnInPlaceEditWithANewDigest(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	start := time.Date(2021, 12, 31, 0, 0, 0, 0, time.UTC)
	cfg := config.Config{Events: []config.EventConfig{
		{
			ID: "cambio-metodologico-ecoicop", Group: "milestones",
			Name:      "Cambio metodológico ECOICOP",
			Scope:     config.EventScopeConfig{Kind: config.EventScopeDataset, Ref: "ine-epa"},
			SourceURL: "https://www.ine.es/metodologia",
			NoteMD:    "Nota metodológica publicada por la fuente.", DateStart: &start,
		},
	}}
	if _, err := ingestion.ReconcileEditorialConfig(ctx, tx, cfg); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	before, err := postgres.ListEvents(ctx, tx)
	if err != nil {
		t.Fatalf("ListEvents (before): %v", err)
	}
	if len(before) != 1 || before[0].ScopeRef != "ine-epa" {
		t.Fatalf("expected one ine-epa-scoped row, got %+v", before)
	}

	cfg.Events[0].Scope.Ref = "ine-ipc"
	result, err := ingestion.ReconcileEditorialConfig(ctx, tx, cfg)
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if result.Events.Updated != 1 || result.Events.Inserted != 0 || result.Events.Retired != 0 {
		t.Fatalf("counts = %+v, want exactly Updated:1", result.Events)
	}
	after, err := postgres.ListEvents(ctx, tx)
	if err != nil {
		t.Fatalf("ListEvents (after): %v", err)
	}
	if after[0].ConfigDigest == before[0].ConfigDigest {
		t.Error("ConfigDigest did not change after re-scoping the event")
	}
	if after[0].ScopeRef != "ine-ipc" {
		t.Errorf("ScopeRef = %q, want ine-ipc", after[0].ScopeRef)
	}

	// The same argument, one field over: correcting a mis-typed citation
	// must not be a silent no-op that leaves the row pointing at a document
	// nobody approved.
	cfg.Events[0].SourceURL = "https://www.ine.es/metodologia#ecoicop"
	if _, err := ingestion.ReconcileEditorialConfig(ctx, tx, cfg); err != nil {
		t.Fatalf("third reconcile: %v", err)
	}
	final, err := postgres.ListEvents(ctx, tx)
	if err != nil {
		t.Fatalf("ListEvents (final): %v", err)
	}
	if final[0].ConfigDigest == after[0].ConfigDigest {
		t.Error("ConfigDigest did not change after correcting source_url")
	}
	if final[0].SourceURL == nil || *final[0].SourceURL != cfg.Events[0].SourceURL {
		t.Errorf("SourceURL = %v, want the corrected citation", final[0].SourceURL)
	}
}

// TestReconcileEditorialConfig_AProvisionalDateOnAnUnconfirmedEntryIsStillHeldBack
// is CRITICAL-54's regression proof, and it is written for the CLASS rather
// than the one entry that exhibited it.
//
// The editorial YAML may legitimately carry a PROVISIONAL date alongside
// date_status: unconfirmed — the date records the author's best current
// reading and the todo records what must be consulted to confirm it, which
// is strictly more useful to the next editor than an empty field, and
// validate-config allows exactly that shape. What must never happen is that
// the provisional date reaches series_break/event, because from the
// database down (export artifact, chart annotation, reader) nothing carries
// the "unconfirmed" qualifier: the date arrives looking exactly as
// authoritative as a confirmed one.
//
// Both registries are asserted together on purpose. Breaks and events carry
// the same Date/DateStatus/Todo triple and were guarded by the same
// nil-date test, so a fix applied to one and not the other would leave the
// defect alive in the half nobody happened to look at.
func TestReconcileEditorialConfig_AProvisionalDateOnAnUnconfirmedEntryIsStillHeldBack(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	provisional := time.Date(2021, 8, 1, 0, 0, 0, 0, time.UTC)
	cfg := config.Config{
		Breaks: []config.BreakConfig{
			{ID: "ruptura-con-fecha-provisional", Kind: "methodology",
				Scope:      config.BreakScopeConfig{Kind: "dataset", Ref: "ine-epa"},
				NoteMD:     "Ruptura cuya fecha efectiva sigue sin confirmar.",
				Date:       &provisional,
				DateStatus: "unconfirmed",
				Todo:       "Confirmar la fecha efectiva en la nota metodológica de la fuente."},
		},
		Events: []config.EventConfig{
			{ID: "evento-con-fecha-provisional", Group: "exogenous",
				Name:       "Evento cuya fecha de inicio sigue sin confirmar",
				NoteMD:     "Fecha pendiente de confirmar contra el calendario oficial.",
				DateStart:  &provisional,
				DateStatus: "unconfirmed",
				Todo:       "Confirmar la fecha exacta en el calendario oficial."},
		},
	}

	result, err := ingestion.ReconcileEditorialConfig(ctx, tx, cfg)
	if err != nil {
		t.Fatalf("ReconcileEditorialConfig: %v", err)
	}

	if got := result.PendingBreakIDs; len(got) != 1 || got[0] != "ruptura-con-fecha-provisional" {
		t.Errorf("PendingBreakIDs = %v, want [ruptura-con-fecha-provisional] — a break declared unconfirmed is pending whatever date it happens to carry", got)
	}
	if got := result.PendingEventIDs; len(got) != 1 || got[0] != "evento-con-fecha-provisional" {
		t.Errorf("PendingEventIDs = %v, want [evento-con-fecha-provisional] — an event declared unconfirmed is pending whatever date it happens to carry", got)
	}
	if result.Breaks.Inserted != 0 || result.Events.Inserted != 0 {
		t.Errorf("counts = breaks:%+v events:%+v, want nothing inserted", result.Breaks, result.Events)
	}

	breakRows, err := postgres.ListSeriesBreaks(ctx, tx)
	if err != nil {
		t.Fatalf("ListSeriesBreaks: %v", err)
	}
	if len(breakRows) != 0 {
		t.Errorf("series_break holds %d row(s) after reconciling only unconfirmed entries: %+v", len(breakRows), breakRows)
	}
	eventRows, err := postgres.ListEvents(ctx, tx)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(eventRows) != 0 {
		t.Errorf("event holds %d row(s) after reconciling only unconfirmed entries: %+v", len(eventRows), eventRows)
	}
}

// TestReconcileEditorialConfig_ShippedConfigPendingListsAreExactlyItsUnconfirmedEntries
// runs the REAL, EMBEDDED config/ tree through ReconcileEditorialConfig and
// asserts the pending lists the operator is shown — count and identifiers —
// against what the YAML itself declares.
//
// This test exists because the suite was structurally blind to CRITICAL-54.
// Every other unconfirmed-entry test here builds its fixture in Go, and
// every one of those fixtures happened to pair date_status: unconfirmed
// with a nil date. A hand-built fixture cannot exhibit a state the real
// configuration reaches unless somebody first thinks to write it, and
// nobody had: the shipped eventos.yaml carried an unconfirmed entry WITH a
// date for as long as the guard was wrong, and the whole suite stayed
// green. Reconciling the real tree removes the need to predict the shape in
// advance — whatever the editors actually write is what gets asserted.
//
// The expectation is derived from DateStatus ALONE, deliberately not from
// the "unconfirmed OR no date" disjunction the reconcile applies. Stating
// the rule the way an editor states it ("this entry says its date is not
// confirmed") rather than the way the code happens to implement it keeps
// this from degenerating into a restatement of the implementation: were the
// guard to drop either half of its condition, the two would disagree here.
func TestReconcileEditorialConfig_ShippedConfigPendingListsAreExactlyItsUnconfirmedEntries(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	sub, err := fs.Sub(configdata.FS, "config")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	cfg, err := config.Load(sub)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	var wantBreaks, wantEvents []string
	for _, b := range cfg.Breaks {
		if b.DateStatus == "unconfirmed" {
			wantBreaks = append(wantBreaks, b.ID)
		}
	}
	for _, e := range cfg.Events {
		if e.DateStatus == "unconfirmed" {
			wantEvents = append(wantEvents, e.ID)
		}
	}

	result, err := ingestion.ReconcileEditorialConfig(ctx, tx, *cfg)
	if err != nil {
		t.Fatalf("ReconcileEditorialConfig: %v", err)
	}

	assertSameIDs(t, "PendingBreakIDs", result.PendingBreakIDs, wantBreaks)
	assertSameIDs(t, "PendingEventIDs", result.PendingEventIDs, wantEvents)

	// The count is asserted as well as the identifiers because the spec
	// asks for both, and because the two failed apart in CRITICAL-54: the
	// operator was shown six pending entries while the configuration
	// declared seven, and the missing one was invisible in the identifier
	// list precisely because the count did not say one was absent.
	if got, want := len(result.PendingBreakIDs)+len(result.PendingEventIDs), len(wantBreaks)+len(wantEvents); got != want {
		t.Errorf("reconciliation reports %d pending entries, the configuration declares %d unconfirmed", got, want)
	}
	t.Logf("shipped configuration: %d pending breaks, %d pending events, %d total",
		len(wantBreaks), len(wantEvents), len(wantBreaks)+len(wantEvents))

	// The pending lists are what the operator READS; the tables are what
	// the reader eventually sees. Asserting only the first would let a
	// future change report an entry as pending and project it anyway.
	pending := map[string]bool{}
	for _, id := range append(append([]string{}, wantBreaks...), wantEvents...) {
		pending[id] = true
	}
	breakRows, err := postgres.ListSeriesBreaks(ctx, tx)
	if err != nil {
		t.Fatalf("ListSeriesBreaks: %v", err)
	}
	for _, r := range breakRows {
		if pending[r.BreakKey] {
			t.Errorf("series_break holds a row for %q, an entry config/rupturas.yaml declares unconfirmed: %+v", r.BreakKey, r)
		}
	}
	eventRows, err := postgres.ListEvents(ctx, tx)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	for _, r := range eventRows {
		if pending[r.ID] {
			t.Errorf("event holds a row for %q, an entry the editorial YAML declares unconfirmed: %+v", r.ID, r)
		}
	}
}

// assertSameIDs compares two id lists as SETS: the reconcile builds its
// pending lists in configuration order, which is an implementation detail
// no consumer depends on, and an order-sensitive comparison here would fail
// on a harmless reordering of the YAML while saying nothing about the
// invariant that matters.
func assertSameIDs(t *testing.T, label string, got, want []string) {
	t.Helper()
	gotSorted := append([]string{}, got...)
	wantSorted := append([]string{}, want...)
	sort.Strings(gotSorted)
	sort.Strings(wantSorted)
	if !slices.Equal(gotSorted, wantSorted) {
		t.Errorf("%s = %v, want %v", label, gotSorted, wantSorted)
	}
}

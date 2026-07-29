package postgres_test

// Task 2.12 (RED) / 2.13 (GREEN): an origin identifier change closes the
// old series_source_mapping (valid_to) and adds a new one, instead of
// overwriting it — both rows remain queryable (spec data-model-vintages,
// "Source identifiers are validity-ranged mappings", verified risk R1).

import (
	"context"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

func TestSourceMappingRepo_IdentifierChangeAddsMappingInsteadOfReplacing(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	if err := postgres.NewRunner(tx).Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}
	seedSeries(t, ctx, tx)

	mustExec(t, ctx, tx, `INSERT INTO series_source_mapping (series_id, ref_kind, ref, valid_from, config_digest)
		VALUES ('tasa-de-paro-epa', 'ine-series-cod', 'IPC251852', '2020-01-01', 'digest1')`)

	repo := postgres.NewSourceMappingRepo(tx)
	closedAt := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	next, err := repo.ReplaceActiveMapping(ctx, "tasa-de-paro-epa", closedAt, postgres.SourceMappingInput{
		SeriesID:     "tasa-de-paro-epa",
		RefKind:      "ine-series-cod",
		Ref:          "TESTCOD002",
		ValidFrom:    closedAt,
		ConfigDigest: "digest2",
	})
	if err != nil {
		t.Fatalf("ReplaceActiveMapping: %v", err)
	}
	if next.Ref != "TESTCOD002" {
		t.Errorf("expected the new mapping's ref to be TESTCOD002, got %q", next.Ref)
	}
	if next.ValidTo != nil {
		t.Error("expected the new mapping to be open-ended (valid_to NULL)")
	}

	all, err := postgres.ListMappings(ctx, tx, "tasa-de-paro-epa")
	if err != nil {
		t.Fatalf("ListMappings: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected both the closed and the new mapping to remain queryable, got %d rows", len(all))
	}

	var closed, open *postgres.SourceMapping
	for i := range all {
		if all[i].Ref == "IPC251852" {
			closed = &all[i]
		}
		if all[i].Ref == "TESTCOD002" {
			open = &all[i]
		}
	}
	if closed == nil {
		t.Fatal("expected the original mapping (IPC251852) to still be present")
	}
	if closed.ValidTo == nil {
		t.Error("expected the original mapping to be closed with a valid_to date")
	}
	if open == nil {
		t.Fatal("expected the replacement mapping (TESTCOD002) to be present")
	}
	if open.ValidTo != nil {
		t.Error("expected the replacement mapping to have an open validity range")
	}
}

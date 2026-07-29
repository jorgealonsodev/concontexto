package postgres_test

// Task 2.2 (RED) / 2.3 (GREEN): the Fase 0 migration set, plus the
// database-enforced is_current invariant (ADR-4) that belongs with the
// DDL rather than the writer (see spec data-model-vintages, "The
// database enforces exactly one current row" — the schema-only half of
// that requirement; the writer-owned promotion half is PR 2b).

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

// fase0Tables is the exact, closed set of tables the Fase 0 schema must
// contain (spec data-model-vintages, "Fase 0 migration set" — settled
// decision D3). No more, no fewer.
var fase0Tables = []string{
	"source", "dataset", "series", "series_source_mapping", "observation",
	"ingestion_run", "raw_file", "download_attempt", "series_break", "event",
}

// excludedTables are the four capabilities explicitly cut from Fase 0:
// no Fase 0 consumer references them.
var excludedTables = []string{"indicator_page", "verification", "glossary", "correction"}

func publicTableNames(t *testing.T, ctx context.Context, tx pgx.Tx) map[string]bool {
	t.Helper()
	rows, err := tx.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'`)
	if err != nil {
		t.Fatalf("querying public tables: %v", err)
	}
	defer rows.Close()

	names := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scanning table name: %v", err)
		}
		names[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating public tables: %v", err)
	}
	return names
}

func TestMigrationUp_CreatesExactlyTheFase0TableSet(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	runner := postgres.NewRunner(tx)

	if err := runner.Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	names := publicTableNames(t, ctx, tx)

	for _, want := range fase0Tables {
		if !names[want] {
			t.Errorf("expected table %q to exist after migrate up, it does not", want)
		}
	}
	for _, forbidden := range excludedTables {
		if names[forbidden] {
			t.Errorf("table %q must NOT exist (excluded by decision D3), but it does", forbidden)
		}
	}
	if len(names) != len(fase0Tables) {
		t.Errorf("expected exactly %d public tables, got %d: %v", len(fase0Tables), len(names), names)
	}
}

func TestMigrationUp_EventTableUsesEventGroupNotReservedWord(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	runner := postgres.NewRunner(tx)

	if err := runner.Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	rows, err := tx.Query(ctx, `
		SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'event'`)
	if err != nil {
		t.Fatalf("querying event columns: %v", err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scanning column name: %v", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating event columns: %v", err)
	}

	if !columns["event_group"] {
		t.Error("expected event.event_group to exist, it does not")
	}
	if columns["group"] {
		t.Error("event.group must not exist — 'group' is a SQL reserved word (design.md)")
	}
}

// TestMigrationUp_DatabaseRejectsSecondCurrentRow proves the partial
// unique index one_current_row (ADR-4) is a real, database-enforced
// invariant: PostgreSQL itself rejects a second is_current=true row for
// the same (series_id, period), not merely application-level discipline.
// This is a pure schema assertion — it does not exercise the writer's
// one-transaction promotion path (that is PR 2b's task 2.8/2.9).
func TestMigrationUp_DatabaseRejectsSecondCurrentRow(t *testing.T) {
	ctx := context.Background()
	tx := newTx(t)
	runner := postgres.NewRunner(tx)

	if err := runner.Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	mustExec(t, ctx, tx, `INSERT INTO source (id, name, url, license, attribution_text, access_type, config_digest)
		VALUES ('ine', 'INE', 'https://ine.es', 'lic', 'attr', 'api-json', 'digest1')`)
	mustExec(t, ctx, tx, `INSERT INTO dataset (id, source_id, name, config_digest)
		VALUES ('ine-epa', 'ine', 'EPA', 'digest1')`)
	mustExec(t, ctx, tx, `INSERT INTO series (id, dataset_id, name, unit, frequency, geo, decimals, is_harmonized, config_digest)
		VALUES ('tasa-de-paro-epa', 'ine-epa', 'Tasa de paro', '%', 'Q', 'ES', 2, false, 'digest1')`)
	mustExec(t, ctx, tx, `INSERT INTO raw_file (hash, source_id, url, downloaded_at, storage_path, size_bytes)
		VALUES ('deadbeef', 'ine', 'https://ine.es/data', now(), '/app_data/deadbeef', 100)`)
	mustExec(t, ctx, tx, `INSERT INTO ingestion_run (dataset_id, series_id, started_at, finished_at, raw_file_hash, outcome)
		VALUES ('ine-epa', 'tasa-de-paro-epa', now(), now(), 'deadbeef', 'succeeded')`)

	mustExec(t, ctx, tx, `INSERT INTO observation (series_id, period, version, value, status, extracted_at, ingestion_run_id, is_current)
		VALUES ('tasa-de-paro-epa', '2026-Q1', 1, 11.5, 'D', now(), 1, true)`)

	_, err := tx.Exec(ctx, `INSERT INTO observation (series_id, period, version, value, status, extracted_at, ingestion_run_id, is_current)
		VALUES ('tasa-de-paro-epa', '2026-Q1', 2, 11.6, 'D', now(), 1, true)`)

	if err == nil {
		t.Fatal("expected the second is_current=true row for the same (series_id, period) to be rejected, it was accepted")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected a *pgconn.PgError, got: %v", err)
	}
	if pgErr.Code != "23505" { // unique_violation
		t.Errorf("expected SQLSTATE 23505 (unique_violation), got %q", pgErr.Code)
	}
	if pgErr.ConstraintName != "one_current_row" {
		t.Errorf("expected the violated constraint to be one_current_row, got %q", pgErr.ConstraintName)
	}
}

func mustExec(t *testing.T, ctx context.Context, tx pgx.Tx, sql string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

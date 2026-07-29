package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jorgealonsodev/concontexto/app/migrations"
)

// Runner implements migrate.Runner (app/internal/migrate) against a real
// PostgreSQL database using the embedded Fase 0 migration files
// (app/migrations). It is wired into the `migrate` subcommand — the ONLY
// code path allowed to mutate schema state (task 2.18).
type Runner struct {
	db DBTX
}

// NewRunner builds a Runner against db. db may be a *pgxpool.Pool in
// production or a pgx.Tx in tests (see DBTX).
func NewRunner(db DBTX) *Runner {
	return &Runner{db: db}
}

// migrationFile pairs one migration's up/down SQL under a shared,
// lexically-ordered version name (e.g. "0001_fase0_schema").
type migrationFile struct {
	version  string
	up, down string
}

// loadMigrations reads every {version}.up.sql/{version}.down.sql pair
// from the embedded migrations.FS, in ascending version order.
func loadMigrations() ([]migrationFile, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("migrate: reading embedded migrations: %w", err)
	}

	byVersion := map[string]*migrationFile{}
	var order []string
	addPart := func(name, suffix string, set func(*migrationFile, string)) error {
		if !strings.HasSuffix(name, suffix) {
			return nil
		}
		version := strings.TrimSuffix(name, suffix)
		content, err := migrations.FS.ReadFile(name)
		if err != nil {
			return err
		}
		mf, ok := byVersion[version]
		if !ok {
			mf = &migrationFile{version: version}
			byVersion[version] = mf
			order = append(order, version)
		}
		set(mf, string(content))
		return nil
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if err := addPart(name, ".up.sql", func(mf *migrationFile, c string) { mf.up = c }); err != nil {
			return nil, err
		}
		if err := addPart(name, ".down.sql", func(mf *migrationFile, c string) { mf.down = c }); err != nil {
			return nil, err
		}
	}

	sort.Strings(order)
	result := make([]migrationFile, 0, len(order))
	for _, v := range order {
		mf := byVersion[v]
		if mf.up == "" {
			return nil, fmt.Errorf("migrate: %s is missing its .up.sql file", v)
		}
		if mf.down == "" {
			return nil, fmt.Errorf("migrate: %s is missing its .down.sql file", v)
		}
		result = append(result, *mf)
	}
	return result, nil
}

// ensureTrackingTableSQL creates the migration bookkeeping table in its
// own schema, deliberately kept out of `public` so the Fase 0 schema
// contains exactly the ten tables the spec requires — no eleventh,
// infrastructure-only table leaking into that count.
const ensureTrackingTableSQL = `
CREATE SCHEMA IF NOT EXISTS migration_state;
CREATE TABLE IF NOT EXISTS migration_state.schema_migrations (
	version text PRIMARY KEY,
	applied_at timestamptz NOT NULL DEFAULT now()
);`

func (r *Runner) ensureTrackingTable(ctx context.Context) error {
	if _, err := r.db.Exec(ctx, ensureTrackingTableSQL); err != nil {
		return fmt.Errorf("migrate: preparing tracking table: %w", err)
	}
	return nil
}

// appliedVersions returns applied migration versions in ascending order.
func (r *Runner) appliedVersions(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT version FROM migration_state.schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("migrate: listing applied migrations: %w", err)
	}
	defer rows.Close()

	var applied []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("migrate: scanning applied migration: %w", err)
		}
		applied = append(applied, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migrate: iterating applied migrations: %w", err)
	}
	return applied, nil
}

// Up applies every pending migration, in ascending version order.
func (r *Runner) Up(ctx context.Context) error {
	if err := r.ensureTrackingTable(ctx); err != nil {
		return err
	}
	all, err := loadMigrations()
	if err != nil {
		return err
	}
	applied, err := r.appliedVersions(ctx)
	if err != nil {
		return err
	}
	appliedSet := make(map[string]bool, len(applied))
	for _, v := range applied {
		appliedSet[v] = true
	}

	for _, m := range all {
		if appliedSet[m.version] {
			continue
		}
		if _, err := r.db.Exec(ctx, m.up); err != nil {
			return fmt.Errorf("migrate: applying %s: %w", m.version, err)
		}
		if _, err := r.db.Exec(ctx,
			`INSERT INTO migration_state.schema_migrations (version) VALUES ($1)`, m.version); err != nil {
			return fmt.Errorf("migrate: recording %s: %w", m.version, err)
		}
	}
	return nil
}

// ErrNoMigrationsApplied is returned by Down when there is nothing left
// to roll back, instead of silently doing nothing.
var ErrNoMigrationsApplied = errors.New("migrate: no migrations applied to roll back")

// Down rolls back exactly one migration step: the most recently applied
// version. Rolling back to zero means calling Down repeatedly until
// Status reports no migrations applied (spec data-model-vintages, "Every
// migration is reversible" — "rolled back one step at a time").
func (r *Runner) Down(ctx context.Context) error {
	if err := r.ensureTrackingTable(ctx); err != nil {
		return err
	}
	applied, err := r.appliedVersions(ctx)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		return ErrNoMigrationsApplied
	}
	last := applied[len(applied)-1]

	all, err := loadMigrations()
	if err != nil {
		return err
	}
	var target *migrationFile
	for i := range all {
		if all[i].version == last {
			target = &all[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("migrate: applied version %s has no matching migration file", last)
	}

	if _, err := r.db.Exec(ctx, target.down); err != nil {
		return fmt.Errorf("migrate: rolling back %s: %w", last, err)
	}
	if _, err := r.db.Exec(ctx,
		`DELETE FROM migration_state.schema_migrations WHERE version = $1`, last); err != nil {
		return fmt.Errorf("migrate: un-recording %s: %w", last, err)
	}
	return nil
}

// Status reports how many migrations are applied and which is head.
func (r *Runner) Status(ctx context.Context) (string, error) {
	if err := r.ensureTrackingTable(ctx); err != nil {
		return "", err
	}
	applied, err := r.appliedVersions(ctx)
	if err != nil {
		return "", err
	}
	if len(applied) == 0 {
		return "no migrations applied", nil
	}
	return fmt.Sprintf("%d migration(s) applied; head: %s", len(applied), applied[len(applied)-1]), nil
}

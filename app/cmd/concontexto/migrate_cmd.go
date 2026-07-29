package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
	"github.com/jorgealonsodev/concontexto/app/internal/migrate"
)

// cmdMigrate is the only subcommand allowed to touch schema state (spec
// platform-runtime, "Migrations run only on explicit command"). It wires
// to the real postgres adapter (task 2.18) when DATABASE_URL is set;
// otherwise every operation reports migrate.ErrNotConfigured, exactly as
// in PR 1a.
func cmdMigrate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: concontexto migrate <up|down|status>")
		return 1
	}

	ctx := context.Background()
	runner, cleanup, err := resolveMigrateRunner(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "migrate:", err)
		return 1
	}
	defer cleanup()

	msg, err := migrate.Execute(ctx, args[0], runner)
	if err != nil {
		fmt.Fprintln(stderr, "migrate:", err)
		return 1
	}
	fmt.Fprintln(stdout, msg)
	return 0
}

// resolveMigrateRunner builds the Postgres-backed migrate.Runner from
// DATABASE_URL, or falls back to migrate.NotConfiguredRunner when it is
// unset. The returned cleanup func closes any opened connection pool and
// must always be called, even on error paths that return a nil runner.
func resolveMigrateRunner(ctx context.Context) (migrate.Runner, func(), error) {
	noop := func() {}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return migrate.NotConfiguredRunner{}, noop, nil
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, noop, fmt.Errorf("connecting to DATABASE_URL: %w", err)
	}
	return postgres.NewRunner(pool), pool.Close, nil
}

package postgres_test

// Task 2.1 — testcontainers-go harness. TestMain starts one
// postgres:17-alpine container per package (the production image, per
// ADR-3), guarded by testing.Short(), and exposes a shared pool that
// individual tests isolate against with newTx (transaction rollback).
//
// `go test -short ./...` never touches Docker: TestMain exits before
// starting the container when testing.Short() is true, so short mode
// stays hermetic even though this package requires Docker otherwise.

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// testPool is shared by every test in this package once TestMain starts
// the container. It stays nil in -short mode; tests that need it must be
// skipped via testing.Short() themselves.
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	// testing.Short() reads the -test.short flag, which the testing
	// package registers but does not parse before calling TestMain.
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("concontexto_test"),
		tcpostgres.WithUsername("concontexto_test"),
		tcpostgres.WithPassword("concontexto_test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "postgres testcontainer: failed to start:", err)
		os.Exit(1)
	}
	defer func() { _ = container.Terminate(context.Background()) }()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, "postgres testcontainer: connection string:", err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "postgres testcontainer: pool:", err)
		os.Exit(1)
	}
	defer pool.Close()

	testPool = pool
	os.Exit(m.Run())
}

// newTx begins a transaction against the shared container and rolls it
// back via t.Cleanup, giving each test full isolation without paying for
// a fresh container or database per test. Migration-level tests in this
// package intentionally do NOT use newTx (schema mutation and its
// database-enforced constraints are exactly what they assert), but this
// helper is part of the harness required by task 2.1 for row-level tests
// (PR 2b's writer/repository suite).
func newTx(t *testing.T) pgx.Tx {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping: requires Docker (testcontainers), disabled by -short")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("newTx: begin: %v", err)
	}
	t.Cleanup(func() {
		_ = tx.Rollback(ctx)
	})
	return tx
}

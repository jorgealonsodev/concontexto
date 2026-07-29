package ingestion_test

// Task 5b.6 -- this package's own testcontainers-go harness (design.md
// "Repository/SQL ... TestMain container per package"): IngestSeries
// composes the postgres adapter, so its end-to-end proof needs a real
// Postgres exactly like app/internal/adapters/postgres's own suite does.
// Duplicated here, not imported, because Go test harnesses in
// TestMain/_test.go files are not importable across packages -- the same
// reason app/internal/adapters/postgres's own testmain_test.go is not
// shared with any other package either.

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

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
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

// Package postgres is the pgx-backed adapter for the indicators domain
// (design.md "Go package layout": "pgx repos; owns migrate SQL via
// embed"). It owns the concrete migrate.Runner implementation and, in
// PR 2b, the observation writer and other repository methods.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is the minimal query surface this adapter needs. Both
// *pgxpool.Pool (production) and pgx.Tx (test isolation) satisfy it
// structurally, so the same adapter code runs unmodified against a real
// connection pool or a per-test transaction that gets rolled back — the
// tx-rollback isolation the harness (task 2.1) requires.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxBeginner extends DBTX with Begin, letting a writer open its own
// atomic promotion transaction (design.md "postgres.ObservationWriter
// [Tx2, atomic]") regardless of whether it is constructed against the
// production pool or a test's rollback-isolated outer transaction:
// *pgxpool.Pool.Begin opens a real transaction, while pgx.Tx.Begin opens
// a nested transaction via SAVEPOINT — both satisfy this interface
// structurally, so the one-transaction promotion invariant (ADR-4) holds
// identically in production and in the tx-rollback test harness (2.1).
type TxBeginner interface {
	DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

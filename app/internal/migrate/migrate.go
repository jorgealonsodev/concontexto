// Package migrate defines the schema-migration contract and dispatch
// skeleton for the "migrate" subcommand. It is the ONLY code path allowed
// to mutate schema state (spec platform-runtime, "Migrations run only on
// explicit command") — never invoked automatically on boot, because
// auto-migrate races across replicas and fires during rollbacks
// (design.md).
//
// The concrete Postgres-backed Runner is wired in PR 2a once the
// postgres adapter exists (task 2.18); this package only owns the
// contract and CLI dispatch for PR 1a.
package migrate

import (
	"context"
	"errors"
	"sync/atomic"
)

// Runner performs schema migrations against a concrete backend.
type Runner interface {
	Up(ctx context.Context) error
	Down(ctx context.Context) error
	Status(ctx context.Context) (string, error)
}

var executionCount atomic.Int64

// ExecutionCount reports how many times Execute has run in this process.
// It exists so tests can prove that no other code path — in particular,
// serve's boot sequence — triggers a migration implicitly (see the
// boot-safety scenario in spec platform-runtime).
func ExecutionCount() int64 { return executionCount.Load() }

// ErrUnknownCommand is returned when cmd is not "up", "down" or "status".
var ErrUnknownCommand = errors.New("migrate: unknown command")

// Execute dispatches cmd ("up", "down" or "status") to runner. It is the
// only function in this binary that may cause schema mutation.
func Execute(ctx context.Context, cmd string, runner Runner) (string, error) {
	executionCount.Add(1)
	switch cmd {
	case "up":
		if err := runner.Up(ctx); err != nil {
			return "", err
		}
		return "migrated up", nil
	case "down":
		if err := runner.Down(ctx); err != nil {
			return "", err
		}
		return "migrated down", nil
	case "status":
		return runner.Status(ctx)
	default:
		return "", ErrUnknownCommand
	}
}

// ErrNotConfigured is returned by NotConfiguredRunner for every
// operation: no database connection exists yet in PR 1a.
var ErrNotConfigured = errors.New("migrate: no database configured yet (postgres adapter arrives in PR 2a)")

// NotConfiguredRunner is the PR-1a placeholder Runner. Every operation
// fails clearly instead of silently doing nothing, until the
// Postgres-backed Runner is wired in PR 2a.
type NotConfiguredRunner struct{}

func (NotConfiguredRunner) Up(context.Context) error   { return ErrNotConfigured }
func (NotConfiguredRunner) Down(context.Context) error { return ErrNotConfigured }
func (NotConfiguredRunner) Status(context.Context) (string, error) {
	return "", ErrNotConfigured
}

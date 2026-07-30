// Command concontexto is the single Go binary for the ConContexto data
// pipeline and server (PRD §14). It dispatches six subcommands: serve,
// ingest, migrate, validate-config, healthcheck and export.
//
// `export` is new in phase-1-indicator-page slice 3 (design D-2, task
// 3.11) -- the merged spec platform-runtime still documents "five
// subcommands" (openspec/specs/platform-runtime/spec.md, from Fase 0)
// and this change carries no platform-runtime delta spec of its own.
// design.md/tasks.md both explicitly commit to this sixth subcommand
// (ADR-7's "the publishing layer... Fase 0's design reserved at
// app/internal/publishing/ and never built"), so it is added here with
// TestRealCommands_ExposesExactlySixRequiredSubcommands (main_test.go)
// updated accordingly -- disclosed as a spec-maintenance gap (a
// platform-runtime delta spec documenting the sixth subcommand is still
// owed) rather than silently left unwired or silently left untested.
package main

import (
	"os"

	"github.com/jorgealonsodev/concontexto/app/internal/healthcheck"
)

// realCommands wires the six subcommands this binary exposes -- five
// required by spec platform-runtime ("Single binary with five
// subcommands") plus `export` (phase-1-indicator-page D-2, see this
// file's own package doc comment for why the live spec has not yet
// caught up).
func realCommands() []commandEntry {
	return []commandEntry{
		{name: "serve", run: cmdServe},
		{name: "ingest", run: cmdIngest},
		{name: "migrate", run: cmdMigrate},
		{name: "validate-config", run: cmdValidateConfig},
		{name: "healthcheck", run: healthcheck.Run},
		{name: "export", run: cmdExport},
	}
}

func main() {
	os.Exit(dispatch(os.Args[1:], realCommands(), os.Stdout, os.Stderr))
}

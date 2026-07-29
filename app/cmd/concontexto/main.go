// Command concontexto is the single Go binary for the ConContexto data
// pipeline and server (PRD §14). It dispatches exactly five subcommands:
// serve, ingest, migrate, validate-config and healthcheck.
package main

import (
	"os"

	"github.com/jorgealonsodev/concontexto/app/internal/healthcheck"
)

// realCommands wires the five subcommands required by spec
// platform-runtime ("Single binary with five subcommands").
func realCommands() []commandEntry {
	return []commandEntry{
		{name: "serve", run: cmdServe},
		{name: "ingest", run: cmdIngest},
		{name: "migrate", run: cmdMigrate},
		{name: "validate-config", run: cmdValidateConfig},
		{name: "healthcheck", run: healthcheck.Run},
	}
}

func main() {
	os.Exit(dispatch(os.Args[1:], realCommands(), os.Stdout, os.Stderr))
}

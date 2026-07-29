package main

// Task 3.6/3.8 (GREEN): the real `validate-config` subcommand, replacing
// the phase-1 stub. It schema-validates the embedded /config tree (ADR-1)
// through app/internal/adapters/config and is CI's blocking gate (spec
// editorial-config, "validate-config subcommand").

import (
	"fmt"
	"io"
	"io/fs"
	"sort"

	configdata "github.com/jorgealonsodev/concontexto"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

// runValidateConfig is the testable core: it takes an already-rooted
// config tree (see config.Load's doc comment on fs.Sub) instead of the
// real embedded configdata.FS, mirroring runServe's pattern in serve.go.
func runValidateConfig(fsys fs.FS, stdout, stderr io.Writer) int {
	cfg, err := config.Load(fsys)
	if err != nil {
		fmt.Fprintf(stderr, "validate-config: %v\n", err)
		return 1
	}
	violations := config.Validate(cfg)
	if len(violations) == 0 {
		fmt.Fprintln(stdout, "validate-config: ok")
		return 0
	}
	// Deterministic order: violations are gathered from a map (Sources)
	// and a slice (Series), so output order is not naturally stable
	// without an explicit sort.
	msgs := make([]string, len(violations))
	for i, v := range violations {
		msgs[i] = v.String()
	}
	sort.Strings(msgs)
	for _, m := range msgs {
		fmt.Fprintln(stderr, "validate-config:", m)
	}
	return 1
}

// cmdValidateConfig wires runValidateConfig to the real embedded /config
// tree (ADR-1's //go:embed shim at the repository root).
func cmdValidateConfig(args []string, stdout, stderr io.Writer) int {
	sub, err := fs.Sub(configdata.FS, "config")
	if err != nil {
		fmt.Fprintf(stderr, "validate-config: %v\n", err)
		return 1
	}
	return runValidateConfig(sub, stdout, stderr)
}

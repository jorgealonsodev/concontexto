package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestDispatch_RecognisesEachRequiredSubcommand covers the "Every
// subcommand is dispatchable" scenario in spec platform-runtime: each of
// serve/ingest/migrate/validate-config/healthcheck is recognised and
// executes its own entry point.
func TestDispatch_RecognisesEachRequiredSubcommand(t *testing.T) {
	required := []string{"serve", "ingest", "migrate", "validate-config", "healthcheck"}

	for _, name := range required {
		t.Run(name, func(t *testing.T) {
			called := false
			var gotArgs []string
			cmds := []commandEntry{
				{name: name, run: func(args []string, stdout, stderr io.Writer) int {
					called = true
					gotArgs = args
					return 0
				}},
			}
			var stdout, stderr bytes.Buffer

			code := dispatch([]string{name, "extra-arg"}, cmds, &stdout, &stderr)

			if !called {
				t.Fatalf("expected the %q handler to be invoked", name)
			}
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d", code)
			}
			if len(gotArgs) != 1 || gotArgs[0] != "extra-arg" {
				t.Fatalf("expected remaining args to be forwarded to the handler, got %v", gotArgs)
			}
		})
	}
}

// TestDispatch_UnknownSubcommandExitsNonZeroWithUsage covers the second
// half of the "Every subcommand is dispatchable" scenario: an unknown
// subcommand exits non-zero with a usage message.
func TestDispatch_UnknownSubcommandExitsNonZeroWithUsage(t *testing.T) {
	cmds := []commandEntry{
		{name: "serve", run: func(args []string, stdout, stderr io.Writer) int { return 0 }},
	}
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"bogus"}, cmds, &stdout, &stderr)

	if code == 0 {
		t.Fatal("expected a non-zero exit code for an unknown subcommand")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("expected a usage message on stderr, got %q", stderr.String())
	}
}

// TestDispatch_NoArgsExitsNonZeroWithUsage covers invocation with no
// subcommand at all — must behave the same as an unknown subcommand.
func TestDispatch_NoArgsExitsNonZeroWithUsage(t *testing.T) {
	cmds := []commandEntry{
		{name: "serve", run: func(args []string, stdout, stderr io.Writer) int { return 0 }},
	}
	var stdout, stderr bytes.Buffer

	code := dispatch(nil, cmds, &stdout, &stderr)

	if code == 0 {
		t.Fatal("expected a non-zero exit code when no subcommand is given")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("expected a usage message on stderr, got %q", stderr.String())
	}
}

// TestRealCommands_ExposesExactlySixRequiredSubcommands proves the
// production wiring (not just the dispatch mechanism) exposes exactly
// the five subcommands spec platform-runtime requires PLUS `export`
// (phase-1-indicator-page D-2, task 3.11 -- see main.go's own package
// doc comment: the merged platform-runtime spec has not yet been given
// a delta spec documenting this sixth subcommand, a disclosed
// spec-maintenance gap, not an oversight in this test).
func TestRealCommands_ExposesExactlySixRequiredSubcommands(t *testing.T) {
	want := map[string]bool{
		"serve": true, "ingest": true, "migrate": true,
		"validate-config": true, "healthcheck": true, "export": true,
	}

	cmds := realCommands()

	if len(cmds) != len(want) {
		t.Fatalf("expected exactly %d subcommands, got %d: %v", len(want), len(cmds), cmds)
	}
	for _, c := range cmds {
		if !want[c.name] {
			t.Errorf("unexpected subcommand %q", c.name)
		}
		if c.run == nil {
			t.Errorf("subcommand %q has a nil entry point", c.name)
		}
	}
}

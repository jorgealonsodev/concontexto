package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// cmdFunc is the shape every subcommand entry point must satisfy.
type cmdFunc func(args []string, stdout, stderr io.Writer) int

type commandEntry struct {
	name string
	run  cmdFunc
}

// dispatch resolves args[0] against cmds and runs its handler, forwarding
// the remaining args. An empty arg list or an unrecognised subcommand
// exits non-zero with a usage message (spec platform-runtime, "Every
// subcommand is dispatchable").
func dispatch(args []string, cmds []commandEntry, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(cmds, stderr)
		return 1
	}
	for _, c := range cmds {
		if c.name == args[0] {
			return c.run(args[1:], stdout, stderr)
		}
	}
	fmt.Fprintf(stderr, "unknown subcommand %q\n", args[0])
	printUsage(cmds, stderr)
	return 1
}

func printUsage(cmds []commandEntry, w io.Writer) {
	names := make([]string, 0, len(cmds))
	for _, c := range cmds {
		names = append(names, c.name)
	}
	sort.Strings(names)
	fmt.Fprintf(w, "usage: concontexto <%s>\n", strings.Join(names, "|"))
}

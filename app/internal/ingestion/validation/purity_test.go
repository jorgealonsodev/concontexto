package validation_test

// Task 4.1 (RED): a validation rule MUST be a pure function over domain
// types and its own configuration — no I/O, no clock, deterministic for
// identical inputs (spec data-validation, "Validation rules are pure
// functions" / "A rule is deterministic and side-effect free"). This
// file makes both halves of that contract falsifiable, not a
// convention:
//
//  1. TestRule_TwoEvaluationsOfIdenticalInputsAgree proves the Rule /
//     SeriesContext / Finding plumbing is deterministic, using a small
//     fixture rule defined only in this test (no production rule exists
//     yet — that arrives with 4.3-4.8).
//  2. TestValidationPackage_NeverImportsDBAdaptersOrNetworking makes "no
//     I/O" a compile-time-visible invariant via a go-list-deps guard,
//     exactly like httpserver's golden-rule import guard (task 1.6) —
//     if this package ever transitively imports a real I/O boundary
//     (os/exec, net, database/sql, pgx, the postgres/filestore/ine/
//     eurostat adapters), this test fails and CI blocks the merge. It
//     deliberately does NOT forbid transitively importing "os" or
//     "time" outright: config.SourceRef legitimately carries
//     ValidFrom/ValidTo as time.Time DATA (validity ranges, not a clock
//     read), and Go's own io/fs package transitively pulls in "os" —
//     unavoidable once validation reuses config's types per explicit
//     instruction, and neither is an I/O call by itself.
//  3. TestValidationSource_NeverCallsTheClockOrOSDirectly scans this
//     package's OWN .go files (not its dependencies) with go/scanner —
//     the same technique as guard/originidentifiers_test.go — for the
//     literal call time.Now(, and for direct "os"/"net"/"net/http"/
//     "database/sql" imports. This is the precise, falsifiable form of
//     "no clock access": it is about what THIS package's code does, not
//     what a legitimately-reused type happens to be shaped like.

import (
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

// fixtureRule is a throwaway Rule used only to prove the plumbing is
// deterministic — it is not one of the six production rules.
func fixtureRule(ctx validation.SeriesContext, incoming []indicators.Observation) []validation.Finding {
	var findings []validation.Finding
	for _, o := range incoming {
		if o.Value != nil && *o.Value < 0 {
			findings = append(findings, validation.Finding{
				Rule: "fixture", Severity: validation.SeverityBlock,
				Period: o.Period.String(), Message: "negative value",
			})
		}
	}
	return findings
}

func TestRule_TwoEvaluationsOfIdenticalInputsAgree(t *testing.T) {
	ctx := validation.SeriesContext{
		Series: indicators.Series{Slug: "test-series", Frequency: indicators.FrequencyQuarterly},
	}
	negative := -1.0
	incoming := []indicators.Observation{
		{Period: indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 1}, Value: &negative},
	}

	first := fixtureRule(ctx, incoming)
	second := fixtureRule(ctx, incoming)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("rule evaluated twice on identical inputs disagreed:\nfirst:  %+v\nsecond: %+v", first, second)
	}
	if len(first) != 1 {
		t.Fatalf("expected exactly one finding, got %d: %+v", len(first), first)
	}
}

func TestValidationPackage_NeverImportsDBAdaptersOrNetworking(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps",
		"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps failed: %v\n%s", err, out)
	}

	deps := strings.Fields(string(out))
	forbiddenExact := map[string]bool{
		"net":          true,
		"net/http":     true,
		"database/sql": true,
		"os/exec":      true,
	}
	forbiddenPrefixes := []string{
		"github.com/jackc/pgx",
		"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres",
		"github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore",
		"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine",
		"github.com/jorgealonsodev/concontexto/app/internal/adapters/eurostat",
	}

	for _, dep := range deps {
		if forbiddenExact[dep] {
			t.Errorf("purity violated: validation transitively imports %q (I/O boundary)", dep)
		}
		for _, prefix := range forbiddenPrefixes {
			if strings.HasPrefix(dep, prefix) {
				t.Errorf("purity violated: validation transitively imports %q (forbidden prefix %q)", dep, prefix)
			}
		}
	}
}

// TestValidationSource_NeverCallsTheClockOrOSDirectly scans every
// non-test .go file directly inside this package for a literal
// time.Now( call, or a direct import of "os", "net", "net/http" or
// "database/sql". Using go/scanner rather than a substring search means
// a comment mentioning "time.Now()" in prose (like this doc comment) is
// never mistaken for a real call. Test files are excluded on purpose:
// this file itself legitimately uses "os" to read its sibling source
// files for the scan — the contract under test is that the RULE
// engine's production code is I/O/clock-free, not that its test
// harnesses are.
func TestValidationSource_NeverCallsTheClockOrOSDirectly(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	dir := filepath.Dir(thisFile)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	var violations []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		if hasClockCall(e.Name(), src) {
			violations = append(violations, e.Name()+": calls time.Now(")
		}
		for _, forbidden := range []string{`"os"`, `"net"`, `"net/http"`, `"database/sql"`} {
			if hasImport(e.Name(), src, forbidden) {
				violations = append(violations, e.Name()+": imports "+forbidden)
			}
		}
	}

	if len(violations) > 0 {
		t.Fatalf("clock/OS access found directly in package validation (spec \"A rule is deterministic and side-effect free\"):\n%s", strings.Join(violations, "\n"))
	}
}

func hasClockCall(filename string, src []byte) bool {
	toks := tokenize(filename, src)
	for i := 0; i+3 < len(toks); i++ {
		if toks[i].tok == token.IDENT && toks[i].lit == "time" &&
			toks[i+1].tok == token.PERIOD &&
			toks[i+2].tok == token.IDENT && toks[i+2].lit == "Now" &&
			toks[i+3].tok == token.LPAREN {
			return true
		}
	}
	return false
}

func hasImport(filename string, src []byte, quotedPkg string) bool {
	toks := tokenize(filename, src)
	for _, tk := range toks {
		if tk.tok == token.STRING && tk.lit == quotedPkg {
			return true
		}
	}
	return false
}

type scannedToken struct {
	tok token.Token
	lit string
}

func tokenize(filename string, src []byte) []scannedToken {
	fset := token.NewFileSet()
	file := fset.AddFile(filename, fset.Base(), len(src))

	var s scanner.Scanner
	s.Init(file, src, nil, 0)

	var out []scannedToken
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		out = append(out, scannedToken{tok: tok, lit: lit})
	}
	return out
}

// TestValidationPackage_ImportGuardIsFalsifiable proves the guard above
// is not vacuous: config.ValidationConfig and indicators.Observation
// are real, currently-allowed dependencies the package needs for its
// own SeriesContext — confirming the guard runs against a non-empty
// dependency list rather than an empty/broken "go list" invocation.
func TestValidationPackage_ImportGuardIsFalsifiable(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps",
		"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps failed: %v\n%s", err, out)
	}
	deps := string(out)
	if !strings.Contains(deps, "app/internal/indicators") {
		t.Error("expected validation to depend on app/internal/indicators — the guard's dependency list looks empty or broken")
	}
	if !strings.Contains(deps, "app/internal/adapters/config") {
		t.Error("expected validation to depend on app/internal/adapters/config (reused threshold/schema types) — the guard's dependency list looks empty or broken")
	}
	var _ config.ValidationConfig // keep the config import live even if the assertions above are skipped
}

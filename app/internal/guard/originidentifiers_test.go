package guard

// Task 3.3 (RED) / 3.4 (GREEN): origin identifiers (INE series CODs and
// table Ids, Eurostat dataset codes and dimension names, XLSX download
// URLs) MUST live only in versioned /config YAML and MUST NOT appear as
// Go source literals (PRD §9.4; spec editorial-config, "Source
// identifiers live only in configuration"). This guard scans the Go
// source tree — excluding `testdata/` fixtures and this package's own
// config-loading exemption (`app/internal/adapters/config`, which
// legitimately needs to *describe* the shape of an origin reference,
// never a specific value) — for a deny-list of known identifiers,
// covering both the currently pinned ones (Engram #4690/#4692,
// verified live 2026-07-28) and identifiers verified retired/dead, so
// reintroducing a stale identifier is caught exactly like hard-coding a
// live one.
//
// This package (app/internal/guard) is itself excluded from the walk:
// its own denylist below necessarily contains these strings as Go
// literals, the same way httpserver's import-graph guard
// (importguard_test.go) necessarily contains the literal strings
// "adapters/postgres" and "github.com/jackc/pgx" it forbids elsewhere.

import (
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// forbiddenOriginIdentifiers is the deny-list of INE/Eurostat origin
// identifiers PRD §9.4 requires to live only in configuration.
//
// Deliberately NOT included: the bare INE operation Id "72" — a two-digit
// numeric literal has an unworkable false-positive rate in a Go codebase
// (buffer sizes, HTTP-adjacent constants, slice capacities...). The
// dimension name "time" is likewise never included — validate-config's
// Eurostat pinning check (spec source-attribution-licensing / task 3.9)
// legitimately names "time" as the one exempt dimension, and "time" is
// an extremely common Go token besides. Both exclusions are intentional,
// not oversights: they are documented, not silently missing.
var forbiddenOriginIdentifiers = []string{
	// INE Tempus3 — pinned series CODs (milestone 0.2, Engram #4690)
	"EPA453100", "EPA387796", "IPC290751", "IPC292511", "CNTR6721", "ECP320",
	// INE Tempus3 — pinned table Ids (discovery-only per ADR-2, never the
	// ingest identifier, but still forbidden as a Go literal)
	"65349", "65109", "76125", "76130", "67822", "59238",
	// INE Tempus3 — verified retired/dead identifiers
	"4247", "50902", "CP",
	// Eurostat — pinned dataset codes (milestone 0.3, Engram #4692)
	"prc_hicp_minr", "une_rt_q", "nama_10_gdp",
	// Eurostat — verified retired/dead dataset codes
	"prc_hicp_manr", "prc_hicp_midx",
	// Eurostat — dead v1 dimension name (live one is "coicop18") and the
	// live dimension name itself (also config-only, never a Go literal)
	"coicop", "coicop18",
}

// excludedDirs are directories the scan never descends into: fixtures
// (test data on disk) and the one package allowed to describe the shape
// of an origin reference (this package, "test fixtures and configuration
// loading" per the spec scenario).
var excludedDirSuffixes = []string{
	string(filepath.Separator) + "testdata",
	string(filepath.Separator) + "app" + string(filepath.Separator) + "internal" + string(filepath.Separator) + "adapters" + string(filepath.Separator) + "config",
	string(filepath.Separator) + "app" + string(filepath.Separator) + "internal" + string(filepath.Separator) + "guard",
	string(filepath.Separator) + ".git",
	string(filepath.Separator) + "web" + string(filepath.Separator) + "node_modules",
}

func TestNoOriginIdentifierLiteralsInGoSource(t *testing.T) {
	root := repoRoot(t)
	var violations []string

	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			for _, suffix := range excludedDirSuffixes {
				if strings.HasSuffix(p, suffix) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		src, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		rel, _ := filepath.Rel(root, p)
		for _, lit := range scanStringLiterals(rel, src) {
			if isForbidden(lit) {
				violations = append(violations, rel+": literal "+strconv.Quote(lit))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("origin identifier literal(s) found in Go source outside config-loading code and testdata/ (spec editorial-config, \"Source identifiers live only in configuration\"):\n%s",
			strings.Join(violations, "\n"))
	}
}

// TestScanStringLiterals_DetectsForbiddenLiteralAndIgnoresLookalikes
// proves the guard is falsifiable in-process (no on-disk mutation
// required): a synthetic snippet containing a forbidden literal must be
// flagged, and a lookalike that is NOT an exact match (coicop18 is a
// distinct literal from coicop, not a substring occurrence) must not be.
func TestScanStringLiterals_DetectsForbiddenLiteralAndIgnoresLookalikes(t *testing.T) {
	src := []byte(`package fake

const bad = "EPA453100"
const alsoBad = "prc_hicp_manr"
const fine = "not-an-origin-identifier"
const dimensionNameAlone = "coicop18" // forbidden too, but distinct from "coicop"

func comment() {
	// EPA453100 mentioned only in a comment must NOT be scanned as a
	// string literal — comments are prose, not Go values.
}
`)
	found := scanStringLiterals("fake.go", src)

	assertContains(t, found, "EPA453100")
	assertContains(t, found, "prc_hicp_manr")
	assertContains(t, found, "coicop18")
	assertContains(t, found, "not-an-origin-identifier") // scanner must not silently drop benign literals either

	if count := countOccurrences(found, "EPA453100"); count != 1 {
		t.Errorf("expected exactly one EPA453100 string-literal occurrence (the const, not the comment), got %d in %v", count, found)
	}

	if isForbidden("not-an-origin-identifier") {
		t.Error("isForbidden matched a benign literal that is not in the deny-list")
	}
	if isForbidden("coicop18x") {
		t.Error("isForbidden matched a superstring of a denied literal — must be an exact match, not a substring test")
	}
	if !isForbidden("coicop18") {
		t.Error("isForbidden did not match the exact denied literal coicop18")
	}
}

func countOccurrences(haystack []string, want string) int {
	n := 0
	for _, s := range haystack {
		if s == want {
			n++
		}
	}
	return n
}

func assertContains(t *testing.T, haystack []string, want string) {
	t.Helper()
	for _, s := range haystack {
		if s == want {
			return
		}
	}
	t.Errorf("expected %q among scanned string literals, got %v", want, haystack)
}

func isForbidden(lit string) bool {
	for _, f := range forbiddenOriginIdentifiers {
		if lit == f {
			return true
		}
	}
	return false
}

// scanStringLiterals tokenizes src with go/scanner and returns every
// unquoted STRING token's value. Using the tokenizer (rather than a
// regex over raw bytes) means comments are never scanned as values —
// exactly the property TestScanStringLiterals_.../"comment" case checks.
func scanStringLiterals(filename string, src []byte) []string {
	fset := token.NewFileSet()
	file := fset.AddFile(filename, fset.Base(), len(src))

	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)

	var out []string
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok != token.STRING {
			continue
		}
		unquoted, err := strconv.Unquote(lit)
		if err != nil {
			// Raw string literals (backtick-quoted) unquote fine via
			// strconv.Unquote too; anything that still fails is not a
			// literal we can meaningfully compare, so skip it rather
			// than fail the whole scan.
			continue
		}
		out = append(out, unquoted)
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	// This file lives at <root>/app/internal/guard/originidentifiers_test.go.
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

package guard

// Task 5b.2 (RED) / 5b.3 (GREEN, confirmation only): the retired INE
// identifiers -- table 4247 (EPA paro, frozen 2023-Q4), table 50902
// (IPC ECOICOP v1, frozen 2025-12 on the old base), operation 72 / code
// CP (Cifras de Población, empty table list) -- were verified dead
// (Engram #4690) and must never resurface in /config, the only place
// origin identifiers are allowed to live at all (spec
// source-ingestion-ine, "No retired identifier remains in
// configuration"). This complements originidentifiers_test.go's Go-
// source scan (which deliberately EXCLUDES /config, since that package
// legitimately contains real identifier VALUES) with the one check that
// must run INSIDE config content: a retired value is exactly as wrong
// there as a live one hard-coded in Go.

import (
	"io/fs"
	"testing"
	"testing/fstest"

	configdata "github.com/jorgealonsodev/concontexto"
)

func TestScanConfigForRetiredIdentifiers_DetectsEachRetiredTokenAndIgnoresLookalikes(t *testing.T) {
	fsys := fstest.MapFS{
		"series/bad-4247.yaml":  &fstest.MapFile{Data: []byte("table_hint: \"4247\"\n")},
		"series/bad-50902.yaml": &fstest.MapFile{Data: []byte("table_hint: \"50902\"\n")},
		"series/bad-72.yaml":    &fstest.MapFile{Data: []byte("operation: \"72\"\n")},
		"series/bad-cp.yaml":    &fstest.MapFile{Data: []byte("code: CP\n")},
		// Task 6.4: the two discontinued Eurostat HICP dataset codes and
		// the dead v1 dimension name "coicop" (spec
		// source-ingestion-eurostat, "prc_hicp_manr and prc_hicp_midx are
		// discontinued and MUST NOT be used ... the replacement
		// prc_hicp_minr uses dimension coicop18, not coicop").
		"series/bad-manr.yaml":   &fstest.MapFile{Data: []byte("ref: prc_hicp_manr\n")},
		"series/bad-midx.yaml":   &fstest.MapFile{Data: []byte("ref: prc_hicp_midx\n")},
		"series/bad-coicop.yaml": &fstest.MapFile{Data: []byte("dimensions: [coicop]\n")},
		// Lookalikes that must NOT be flagged: CNTR6721's digits contain
		// "72" as a substring, "CPI" contains "CP" as a substring, and
		// "coicop18" is the LIVE dimension name (a distinct identifier
		// from "coicop", not a substring occurrence of the retired one) --
		// none is the exact, word-bounded retired token.
		"series/pib-cvi.yaml":        &fstest.MapFile{Data: []byte("ref: CNTR6721\n")},
		"series/lookalike.yaml":      &fstest.MapFile{Data: []byte("note: CPI is not the retired code CP alone\ncode: CP\n")},
		"series/ipc-armonizado.yaml": &fstest.MapFile{Data: []byte("ref: prc_hicp_minr\ndimensions: [coicop18]\n")},
		"sources/ine.yaml":           &fstest.MapFile{Data: []byte("id: ine\n")},
	}

	violations, err := scanConfigForRetiredIdentifiers(fsys)
	if err != nil {
		t.Fatalf("scanConfigForRetiredIdentifiers: %v", err)
	}

	wantFiles := map[string]bool{
		"series/bad-4247.yaml":   false,
		"series/bad-50902.yaml":  false,
		"series/bad-72.yaml":     false,
		"series/bad-cp.yaml":     false,
		"series/lookalike.yaml":  false, // the exact "CP" token on its own line must still be caught
		"series/bad-manr.yaml":   false,
		"series/bad-midx.yaml":   false,
		"series/bad-coicop.yaml": false,
	}
	for _, v := range violations {
		for want := range wantFiles {
			if hasPrefix(v, want) {
				wantFiles[want] = true
			}
		}
		if hasPrefix(v, "series/pib-cvi.yaml") {
			t.Errorf("expected CNTR6721 (contains \"72\" as a substring, not a bounded token) to NOT be flagged, got violation %q", v)
		}
		if hasPrefix(v, "series/ipc-armonizado.yaml") {
			t.Errorf("expected prc_hicp_minr/coicop18 (the live, non-retired identifiers) to NOT be flagged, got violation %q", v)
		}
		if hasPrefix(v, "sources/ine.yaml") {
			t.Errorf("expected sources/ine.yaml to have no violations, got %q", v)
		}
	}
	for file, found := range wantFiles {
		if !found {
			t.Errorf("expected a violation naming %s, got %v", file, violations)
		}
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// TestNoRetiredIdentifiersInEmbeddedConfig is task 5b.2's actual
// acceptance test (spec source-ingestion-ine, "No retired identifier
// remains in configuration"): the REAL embedded /config tree, including
// the six milestone-0.2 series added in this batch, must carry none of
// the four retired tokens.
func TestNoRetiredIdentifiersInEmbeddedConfig(t *testing.T) {
	sub, err := fs.Sub(configdata.FS, "config")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	violations, err := scanConfigForRetiredIdentifiers(sub)
	if err != nil {
		t.Fatalf("scanConfigForRetiredIdentifiers: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected zero retired-identifier references in /config, got:\n%v", violations)
	}
}

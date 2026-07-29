package main

// Task 3.7 (RED) / 3.8 (GREEN): the `validate-config` subcommand wires
// app/internal/adapters/config's Load+Validate to the CLI (spec
// editorial-config, "validate-config subcommand"): a complete tree
// exits 0, a malformed one exits non-zero naming file+field. runValidateConfig
// is the testable core (mirrors runServe's pattern in serve.go): it
// takes an already fs.Sub'd tree instead of the real embedded
// configdata.FS, so this test can inject both a valid and a malformed
// tree without touching the real /config.

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"
)

func TestRunValidateConfig_CompleteTreeExitsZero(t *testing.T) {
	fsys := fstest.MapFS{
		"sources/ine.yaml": &fstest.MapFile{Data: []byte(`
id: ine
name: "INE"
url: https://www.ine.es
access_type: api-json
licence:
  name: "Reutilización con atribución"
  attribution_text: "Fuente: INE"
  redistribution: { allowed: true, conditions_md: "cita la fuente", commercial_restrictions_md: "ninguna" }
`)},
		"series/tasa-de-paro-epa.yaml": &fstest.MapFile{Data: []byte(`
slug: tasa-de-paro-epa
source: ine
dataset: ine-epa
unit: "% población activa"
frequency: Q
decimals: 2
source_refs:
  - kind: ine-series-cod
    ref: EPA453100
    valid_from: 2026-07-28
`)},
	}
	var stdout, stderr bytes.Buffer
	code := runValidateConfig(fsys, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
}

func TestRunValidateConfig_MissingUnitFieldExitsNonZeroNamingFileAndField(t *testing.T) {
	fsys := fstest.MapFS{
		"series/broken.yaml": &fstest.MapFile{Data: []byte(`
slug: broken
source: ine
dataset: ine-epa
frequency: Q
decimals: 2
source_refs:
  - kind: ine-series-cod
    ref: EPA453100
    valid_from: 2026-07-28
`)},
	}
	var stdout, stderr bytes.Buffer
	code := runValidateConfig(fsys, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected a non-zero exit code for a series config missing unit")
	}
	if !strings.Contains(stderr.String(), "series/broken.yaml") || !strings.Contains(stderr.String(), "unit") {
		t.Errorf("expected stderr to name the file and the missing field, got %q", stderr.String())
	}
}

func TestCmdValidateConfig_RunsAgainstTheRealEmbeddedConfig(t *testing.T) {
	// The real, checked-in /config tree (sources/ine.yaml,
	// sources/eurostat.yaml — no series/ yet, Phase 5b/6 territory) must
	// itself pass, proving the wiring end to end (task 3.6 "wire the
	// config adapter to consume it").
	var stdout, stderr bytes.Buffer
	code := cmdValidateConfig(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("validate-config against the real embedded config: exit %d, stderr=%s", code, stderr.String())
	}
}

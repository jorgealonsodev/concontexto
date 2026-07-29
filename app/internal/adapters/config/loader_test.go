package config_test

// Task 3.1 (RED) / 3.2 (GREEN): a well-formed series/{slug}.yaml
// resolves slug, at least one validity-ranged source reference, unit,
// frequency and decimals (spec editorial-config, "A series config
// declares its full identity"). fstest.MapFS stands in for
// configdata.FS: both are fs.FS, and Phase 3 has no real series/ files
// checked in yet (they arrive in Phase 5b/6) — the loader must not
// require them either, since Phase 3 ships sources/ only.

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

func TestLoad_SeriesConfigResolvesFullIdentity(t *testing.T) {
	fsys := fstest.MapFS{
		"series/tasa-de-paro-epa.yaml": &fstest.MapFile{Data: []byte(`
slug: tasa-de-paro-epa
name: "Tasa de paro (EPA)"
source: ine
dataset: ine-epa
unit: "% población activa"
frequency: Q
decimals: 2
geo: ES
harmonized: false
source_refs:
  - kind: ine-series-cod
    ref: EPA453100
    table_hint: "65349"
    valid_from: 2026-07-28
    valid_to: null
validation:
  plausibility: { min: 0, max: 40, max_delta_abs: 5 }
  continuity: { documented_gaps: [] }
  revision: { max_backward_periods: 4 }
`)},
	}

	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Series) != 1 {
		t.Fatalf("expected exactly one series, got %d", len(cfg.Series))
	}
	s := cfg.Series[0]

	if s.Slug != "tasa-de-paro-epa" {
		t.Errorf("Slug = %q, want tasa-de-paro-epa", s.Slug)
	}
	if s.Unit != "% población activa" {
		t.Errorf("Unit = %q, want the configured unit", s.Unit)
	}
	if s.Frequency != "Q" {
		t.Errorf("Frequency = %q, want Q", s.Frequency)
	}
	if s.Decimals != 2 {
		t.Errorf("Decimals = %d, want 2", s.Decimals)
	}
	if len(s.SourceRefs) < 1 {
		t.Fatalf("expected at least one source ref, got %d", len(s.SourceRefs))
	}
	ref := s.SourceRefs[0]
	if ref.Kind != "ine-series-cod" || ref.Ref != "EPA453100" {
		t.Errorf("SourceRefs[0] = %+v, want kind=ine-series-cod ref=EPA453100", ref)
	}
	wantFrom := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	if !ref.ValidFrom.Equal(wantFrom) {
		t.Errorf("ValidFrom = %v, want %v", ref.ValidFrom, wantFrom)
	}
	if ref.ValidTo != nil {
		t.Errorf("ValidTo = %v, want nil (still active)", ref.ValidTo)
	}
	if s.FilePath != "series/tasa-de-paro-epa.yaml" {
		t.Errorf("FilePath = %q, want series/tasa-de-paro-epa.yaml", s.FilePath)
	}
}

func TestLoad_SourcesKeyedByDeclaredID(t *testing.T) {
	fsys := fstest.MapFS{
		"sources/ine.yaml": &fstest.MapFile{Data: []byte(`
id: ine
name: "INE"
url: https://www.ine.es
access_type: api-json
licence:
  name: "Reutilización con atribución"
  attribution_text: "Fuente: INE"
  redistribution: { allowed: true, conditions_md: "cita", commercial_restrictions_md: "ninguna" }
`)},
	}

	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	src, ok := cfg.Sources["ine"]
	if !ok {
		t.Fatalf("expected source %q to be present, got keys %v", "ine", cfg.Sources)
	}
	if src.Name != "INE" || src.AccessType != "api-json" {
		t.Errorf("unexpected source contents: %+v", src)
	}
	if src.FilePath != "sources/ine.yaml" {
		t.Errorf("FilePath = %q, want sources/ine.yaml", src.FilePath)
	}
}

func TestLoad_EmptyTreeIsNotAnError(t *testing.T) {
	// Phase 3 ships sources/ only; series/ arrives in Phase 5b/6. An
	// entirely empty tree (neither directory present) must not error —
	// only validate-config decides whether an empty tree is acceptable.
	cfg, err := config.Load(fstest.MapFS{})
	if err != nil {
		t.Fatalf("Load on an empty tree returned an error: %v", err)
	}
	if len(cfg.Series) != 0 || len(cfg.Sources) != 0 {
		t.Errorf("expected an empty Config, got %+v", cfg)
	}
}

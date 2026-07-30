package config_test

// Task 1.3 (RED) / 1.4 (GREEN): cadence_segments config parse/validate
// (spec editorial-config, "A series configuration expresses a cadence
// that changes over its life"): ordered, non-overlapping, no gap between
// segments, a uniform (no cadence_segments) series still validates.

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

func segmentedSeries(t *testing.T, cadenceSegmentsYAML string) *config.Config {
	t.Helper()
	yaml := completeSeriesYAML() + cadenceSegmentsYAML
	fsys := fstest.MapFS{
		"sources/ine.yaml":             &fstest.MapFile{Data: []byte(completeSourceYAML())},
		"series/tasa-de-paro-epa.yaml": &fstest.MapFile{Data: []byte(yaml)},
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func TestValidate_SegmentedCadenceLoadsAndValidates(t *testing.T) {
	cfg := segmentedSeries(t, `
cadence_segments:
  - { from: "1977-Q1", to: "2023-Q2", cadence: semiannual, present: [1, 3] }
  - { from: "2023-Q3", cadence: quarterly }
`)
	if len(cfg.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(cfg.Series))
	}
	segs := cfg.Series[0].CadenceSegments
	if len(segs) != 2 {
		t.Fatalf("expected 2 cadence segments, got %d", len(segs))
	}
	if segs[0].Cadence != "semiannual" || segs[1].Cadence != "quarterly" {
		t.Errorf("expected cadences [semiannual quarterly], got [%s %s]", segs[0].Cadence, segs[1].Cadence)
	}
	if got := config.Validate(cfg); len(got) != 0 {
		t.Fatalf("expected a correctly segmented cadence to validate, got %v", got)
	}
}

func TestValidate_OverlappingSegmentsRejected(t *testing.T) {
	cfg := segmentedSeries(t, `
cadence_segments:
  - { from: "1977-Q1", to: "2023-Q3", cadence: semiannual, present: [1, 3] }
  - { from: "2023-Q2", cadence: quarterly }
`)
	violations := config.Validate(cfg)
	if len(violations) == 0 {
		t.Fatal("expected overlapping cadence segments to be rejected")
	}
	found := false
	for _, v := range violations {
		if strings.Contains(v.Message, "overlap") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a violation naming the overlap, got %v", violations)
	}
}

func TestValidate_GapBetweenSegmentsRejected(t *testing.T) {
	cfg := segmentedSeries(t, `
cadence_segments:
  - { from: "1977-Q1", to: "2020-Q1", cadence: semiannual, present: [1, 3] }
  - { from: "2023-Q3", cadence: quarterly }
`)
	violations := config.Validate(cfg)
	if len(violations) == 0 {
		t.Fatal("expected a gap between cadence segments to be rejected")
	}
	found := false
	for _, v := range violations {
		if strings.Contains(v.Message, "gap") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a violation naming the gap, got %v", violations)
	}
}

func TestValidate_UniformCadenceStillValidates(t *testing.T) {
	cfg := segmentedSeries(t, "")
	if got := config.Validate(cfg); len(got) != 0 {
		t.Fatalf("expected a series with no cadence_segments (uniform cadence) to validate, got %v", got)
	}
}

func TestValidate_LastSegmentMustBeOpenEnded(t *testing.T) {
	cfg := segmentedSeries(t, `
cadence_segments:
  - { from: "1977-Q1", to: "2023-Q2", cadence: semiannual, present: [1, 3] }
  - { from: "2023-Q3", to: "2026-Q2", cadence: quarterly }
`)
	violations := config.Validate(cfg)
	if len(violations) == 0 {
		t.Fatal("expected a closed last segment to be rejected (it must stay open-ended to cover ongoing history)")
	}
}

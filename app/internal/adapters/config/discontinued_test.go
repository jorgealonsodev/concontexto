package config_test

// Remediation B (verify-report CRITICAL-4), config half. indicator-page
// spec, "The three page states of PRD §6.1.3", scenario "A discontinued
// series shows a permanent banner": "GIVEN a series marked discontinued
// by its source ... AND a successor link is shown when a successor is
// configured".
//
// Before this slice nothing anywhere in the project could express that
// marking: `grep -rn "discontinued\|successor"` over config/, app/ and
// the migrations returned two unrelated prose comments and no field, so
// the page state was a hand-maintained web-side constant. This is the
// configuration surface the spec's word "configured" refers to.

import (
	"testing"
	"testing/fstest"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

// subjectSeriesFile is the file every case here puts the `discontinued`
// block in. Named as a constant because the rejection cases assert on
// the violation's File as well as its Field: a message that names the
// field but not the offending file is exactly what spec
// editorial-config's "the message names the file and the missing field"
// forbids, so both halves are pinned.
const subjectSeriesFile = "series/tasa-de-paro-epa.yaml"

// successorSeriesYAML is a second, ordinary live series. It exists so a
// `successor:` reference has something real to resolve against: the
// successor check is a cross-file resolution against the configured
// series set (validate.go), exactly like SeriesConfig.Source's, and a
// single-series tree could only ever exercise its failure branch.
func successorSeriesYAML() string {
	return `
slug: ocupados-epa
name: "Ocupados (EPA)"
source: ine
dataset: ine-epa
unit: "personas"
frequency: Q
decimals: 0
geo: ES
source_refs:
  - kind: ine-series-cod
    ref: EPA453200
    valid_from: 2026-07-28
`
}

// discontinuedConfig parses a two-series tree whose subject series
// carries discontinuedYAML. It deliberately only PARSES: config.Load
// runs no schema validation at all (loader.go decodes YAML and nothing
// else), so a case asserting a rejection must go through
// config.Validate, the package's actual validation entry point and the
// one validate-config's exit code is driven by. This mirrors
// cadence_segments_test.go's segmentedSeries helper exactly.
func discontinuedConfig(t *testing.T, discontinuedYAML string) *config.Config {
	t.Helper()
	fsys := fstest.MapFS{
		"sources/ine.yaml":         &fstest.MapFile{Data: []byte(completeSourceYAML())},
		subjectSeriesFile:          &fstest.MapFile{Data: []byte(completeSeriesYAML() + discontinuedYAML)},
		"series/ocupados-epa.yaml": &fstest.MapFile{Data: []byte(successorSeriesYAML())},
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

// subjectSeries returns the series carrying the `discontinued` block by
// SLUG, never by index: loadSeries walks the directory in lexical file
// order, so "series/ocupados-epa.yaml" sorts ahead of the subject and
// cfg.Series[0] would silently be the wrong series.
func subjectSeries(t *testing.T, cfg *config.Config) config.SeriesConfig {
	t.Helper()
	for _, s := range cfg.Series {
		if s.Slug == "tasa-de-paro-epa" {
			return s
		}
	}
	t.Fatalf("the subject series is not in the loaded config: %+v", cfg.Series)
	return config.SeriesConfig{}
}

// The ordinary case: no `discontinued` block at all. Every series in the
// project is live today, so this is what all six real config files look
// like and it must stay valid and zero-valued.
func TestValidate_ASeriesWithNoDiscontinuedBlockIsLive(t *testing.T) {
	cfg := discontinuedConfig(t, "")
	if d := subjectSeries(t, cfg).Discontinued; d != nil {
		t.Fatalf("expected no discontinued block, got %+v", d)
	}
	if got := config.Validate(cfg); len(got) != 0 {
		t.Fatalf("expected a live series to validate, got %v", got)
	}
}

func TestValidate_ADiscontinuedSeriesCarriesItsDateAndSuccessor(t *testing.T) {
	cfg := discontinuedConfig(t, `
discontinued:
  since: 2026-03-31
  successor: ocupados-epa
`)
	d := subjectSeries(t, cfg).Discontinued
	if d == nil {
		t.Fatal("expected a discontinued block")
	}
	if d.Since != "2026-03-31" {
		t.Errorf("expected since 2026-03-31, got %q", d.Since)
	}
	if d.Successor != "ocupados-epa" {
		t.Errorf("expected successor ocupados-epa, got %q", d.Successor)
	}
	if got := config.Validate(cfg); len(got) != 0 {
		t.Fatalf("expected a well-formed discontinued block to validate, got %v", got)
	}
}

// "where one exists" (spec) — the successor is genuinely optional. A
// source can retire a series without publishing a replacement, and the
// banner must still be expressible for it.
func TestValidate_ADiscontinuedSeriesNeedsNoSuccessor(t *testing.T) {
	cfg := discontinuedConfig(t, `
discontinued:
  since: 2026-03-31
`)
	d := subjectSeries(t, cfg).Discontinued
	if d == nil {
		t.Fatal("expected a discontinued block")
	}
	if d.Successor != "" {
		t.Errorf("expected no successor, got %q", d.Successor)
	}
	if got := config.Validate(cfg); len(got) != 0 {
		t.Fatalf("expected a successor-less discontinued block to validate, got %v", got)
	}
}

func TestValidate_ADiscontinuedBlockWithoutADateIsRejected(t *testing.T) {
	violations := config.Validate(discontinuedConfig(t, `
discontinued:
  successor: ocupados-epa
`))
	if !hasViolation(violations, subjectSeriesFile, "discontinued.since") {
		t.Fatalf("expected a violation naming %s + discontinued.since, got %v", subjectSeriesFile, violations)
	}
}

func TestValidate_ADiscontinuedDateMustBeAnISODate(t *testing.T) {
	violations := config.Validate(discontinuedConfig(t, `
discontinued:
  since: "marzo de 2026"
`))
	if !hasViolation(violations, subjectSeriesFile, "discontinued.since") {
		t.Fatalf("expected a violation naming %s + discontinued.since, got %v", subjectSeriesFile, violations)
	}
}

// "2026-02-31" matches the YYYY-MM-DD shape and is not a date. A shape
// check alone would let it through and the banner would name a day that
// never happened (P4: never fabricate).
func TestValidate_ADiscontinuedDateMustBeARealCalendarDate(t *testing.T) {
	violations := config.Validate(discontinuedConfig(t, `
discontinued:
  since: "2026-02-31"
`))
	if !hasViolation(violations, subjectSeriesFile, "discontinued.since") {
		t.Fatalf("expected a violation naming %s + discontinued.since, got %v", subjectSeriesFile, violations)
	}
}

// A successor link that 404s is worse than no link: it tells a reader
// the data continues somewhere and then strands them. The reference is
// resolved against the configured series set at validate time, exactly
// like SeriesConfig.Source already is.
func TestValidate_AnUnknownSuccessorIsRejected(t *testing.T) {
	violations := config.Validate(discontinuedConfig(t, `
discontinued:
  since: 2026-03-31
  successor: no-existe
`))
	if !hasViolation(violations, subjectSeriesFile, "discontinued.successor") {
		t.Fatalf("expected a violation naming %s + discontinued.successor, got %v", subjectSeriesFile, violations)
	}
}

// A series pointing at itself would render a banner whose "continue
// here" link returns the reader to the page they are already on.
func TestValidate_ASeriesCannotBeItsOwnSuccessor(t *testing.T) {
	violations := config.Validate(discontinuedConfig(t, `
discontinued:
  since: 2026-03-31
  successor: tasa-de-paro-epa
`))
	if !hasViolation(violations, subjectSeriesFile, "discontinued.successor") {
		t.Fatalf("expected a violation naming %s + discontinued.successor, got %v", subjectSeriesFile, violations)
	}
}

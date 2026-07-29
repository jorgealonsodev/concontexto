package config_test

// Task 3.7 (RED) / 3.8 (GREEN): validate-config's schema rules — a
// missing `unit` field exits non-zero naming file+field, a complete tree
// passes, and a reference to an unknown source is rejected naming it
// (spec editorial-config, "validate-config subcommand").
//
// Task 3.9 (RED) / 3.10 (GREEN): every sources/{source}.yaml must carry
// licence, attribution text, access type and redistribution terms (spec
// source-attribution-licensing, "Per-source licensing terms are
// authoritative"), and any Eurostat series ref must pin every dimension
// except `time` (spec source-attribution-licensing / design.md's
// Eurostat guard — Eurostat served 157 MB unfiltered against a 256 MB
// container, Engram #4692).

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

func completeSourceYAML() string {
	return `
id: ine
name: "INE"
url: https://www.ine.es
access_type: api-json
licence:
  name: "Reutilización con atribución"
  attribution_text: "Fuente: INE"
  redistribution: { allowed: true, conditions_md: "cita la fuente", commercial_restrictions_md: "ninguna" }
`
}

func completeSeriesYAML() string {
	return `
slug: tasa-de-paro-epa
name: "Tasa de paro (EPA)"
source: ine
dataset: ine-epa
unit: "% población activa"
frequency: Q
decimals: 2
geo: ES
source_refs:
  - kind: ine-series-cod
    ref: EPA453100
    valid_from: 2026-07-28
`
}

func TestValidate_CompleteTreePasses(t *testing.T) {
	fsys := fstest.MapFS{
		"sources/ine.yaml":             &fstest.MapFile{Data: []byte(completeSourceYAML())},
		"series/tasa-de-paro-epa.yaml": &fstest.MapFile{Data: []byte(completeSeriesYAML())},
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if violations := config.Validate(cfg); len(violations) != 0 {
		t.Fatalf("expected a complete tree to pass, got violations: %v", violations)
	}
}

func TestValidate_MissingUnitFieldFailsNamingFileAndField(t *testing.T) {
	fsys := fstest.MapFS{
		"sources/ine.yaml": &fstest.MapFile{Data: []byte(completeSourceYAML())},
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
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	violations := config.Validate(cfg)
	if !hasViolation(violations, "series/broken.yaml", "unit") {
		t.Fatalf("expected a violation naming series/broken.yaml + unit, got %v", violations)
	}
}

func TestValidate_UnknownSourceReferenceIsRejectedNamingIt(t *testing.T) {
	fsys := fstest.MapFS{
		"series/orphan.yaml": &fstest.MapFile{Data: []byte(`
slug: orphan
source: does-not-exist
dataset: whatever
unit: "%"
frequency: Q
decimals: 1
source_refs:
  - kind: ine-series-cod
    ref: EPA453100
    valid_from: 2026-07-28
`)},
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	violations := config.Validate(cfg)
	found := false
	for _, v := range violations {
		if v.File == "series/orphan.yaml" && v.Field == "source" {
			found = true
			if !strings.Contains(v.Message, "does-not-exist") {
				t.Errorf("expected the violation message to name the unresolved source, got %q", v.Message)
			}
		}
	}
	if !found {
		t.Fatalf("expected a source-reference violation for series/orphan.yaml, got %v", violations)
	}
}

func TestValidate_SourceMissingLicenceFieldFails(t *testing.T) {
	fsys := fstest.MapFS{
		"sources/incomplete.yaml": &fstest.MapFile{Data: []byte(`
id: incomplete
name: "Incomplete Source"
url: https://example.org
access_type: api-json
licence:
  name: "Some licence"
  attribution_text: "Fuente: Incomplete Source"
`)}, // redistribution block entirely absent
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	violations := config.Validate(cfg)
	if !hasViolationInFile(violations, "sources/incomplete.yaml") {
		t.Fatalf("expected sources/incomplete.yaml to fail validation for its missing redistribution terms, got %v", violations)
	}
}

func TestValidate_EurostatSeriesMissingDimensionPinFailsNamingIt(t *testing.T) {
	fsys := fstest.MapFS{
		"sources/eurostat.yaml": &fstest.MapFile{Data: []byte(`
id: eurostat
name: "Eurostat"
url: https://ec.europa.eu/eurostat
access_type: api-json
licence:
  name: "Commission Decision 2011/833/EU"
  attribution_text: "Source: Eurostat"
  redistribution: { allowed: true, conditions_md: "acknowledgement required", commercial_restrictions_md: "some restrictions apply" }
`)},
		"series/ipc-general.yaml": &fstest.MapFile{Data: []byte(`
slug: ipc-general
source: eurostat
dataset: eurostat-hicp
unit: index
frequency: M
decimals: 3
source_refs:
  - kind: eurostat-dataset
    ref: prc_hicp_minr
    dimensions: [freq, unit, coicop18, geo, time]
    filters: { freq: M, unit: RCH_A, geo: ES }
    valid_from: 2026-07-28
`)}, // coicop18 declared but NOT pinned in filters
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	violations := config.Validate(cfg)
	found := false
	for _, v := range violations {
		if v.File == "series/ipc-general.yaml" && strings.Contains(v.Field, "coicop18") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a violation naming the unpinned coicop18 dimension, got %v", violations)
	}
}

func TestValidate_EurostatSeriesFullyPinnedPasses(t *testing.T) {
	fsys := fstest.MapFS{
		"sources/eurostat.yaml": &fstest.MapFile{Data: []byte(`
id: eurostat
name: "Eurostat"
url: https://ec.europa.eu/eurostat
access_type: api-json
licence:
  name: "Commission Decision 2011/833/EU"
  attribution_text: "Source: Eurostat"
  redistribution: { allowed: true, conditions_md: "acknowledgement required", commercial_restrictions_md: "some restrictions apply" }
`)},
		"series/ipc-general.yaml": &fstest.MapFile{Data: []byte(`
slug: ipc-general
source: eurostat
dataset: eurostat-hicp
unit: index
frequency: M
decimals: 3
source_refs:
  - kind: eurostat-dataset
    ref: prc_hicp_minr
    dimensions: [freq, unit, coicop18, geo, time]
    filters: { freq: M, unit: RCH_A, coicop18: TOTAL, geo: ES }
    valid_from: 2026-07-28
`)}, // every dimension except time is pinned
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if violations := config.Validate(cfg); len(violations) != 0 {
		t.Fatalf("expected a fully-pinned Eurostat series to pass, got violations: %v", violations)
	}
}

// TestValidate_UneRtQFullyPinnedPasses is task 6.6/6.7's own literal
// scenario ("a fully-pinned une_rt_q passes"): a SECOND, distinct
// Eurostat dataset shape (7 dimensions, not 5) proves the guard's
// dataset-agnostic property -- it is not merely correct for the
// prc_hicp_minr/coicop18 case already covered above.
// validateEurostatPinning itself was already built in PR 3 (task
// 3.9/3.10) and this exact mechanism already passes for the real,
// checked-in config/series/paro-armonizado-eurostat.yaml (proven by
// TestCmdValidateConfig_RunsAgainstTheRealEmbeddedConfig in
// app/cmd/concontexto/validate_config_cmd_test.go); this test adds the
// task's literally-named scenario as its own standalone, synthetic
// case.
func TestValidate_UneRtQFullyPinnedPasses(t *testing.T) {
	fsys := fstest.MapFS{
		"sources/eurostat.yaml": &fstest.MapFile{Data: []byte(`
id: eurostat
name: "Eurostat"
url: https://ec.europa.eu/eurostat
access_type: api-json
licence:
  name: "Commission Decision 2011/833/EU"
  attribution_text: "Source: Eurostat"
  redistribution: { allowed: true, conditions_md: "acknowledgement required", commercial_restrictions_md: "some restrictions apply" }
`)},
		"series/paro-armonizado.yaml": &fstest.MapFile{Data: []byte(`
slug: paro-armonizado
source: eurostat
dataset: eurostat-une
unit: "% población activa 15-74 años"
frequency: Q
decimals: 1
source_refs:
  - kind: eurostat-dataset
    ref: une_rt_q
    dimensions: [freq, s_adj, age, unit, sex, geo, time]
    filters: { freq: Q, s_adj: SA, age: Y15-74, unit: PC_ACT, sex: T, geo: ES }
    valid_from: 2026-07-28
`)}, // every one of the 7 declared dimensions except time is pinned
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if violations := config.Validate(cfg); len(violations) != 0 {
		t.Fatalf("expected a fully-pinned une_rt_q series to pass, got violations: %v", violations)
	}
}

// TestValidate_UneRtQMissingOneOfSevenDimensionsFailsNamingIt proves the
// guard scales past a 5-dimension dataset: une_rt_q declares 7
// dimensions, and dropping just ONE non-time pin (age) must still be
// caught, not lost among the other 6 correctly-pinned ones.
func TestValidate_UneRtQMissingOneOfSevenDimensionsFailsNamingIt(t *testing.T) {
	fsys := fstest.MapFS{
		"sources/eurostat.yaml": &fstest.MapFile{Data: []byte(`
id: eurostat
name: "Eurostat"
url: https://ec.europa.eu/eurostat
access_type: api-json
licence:
  name: "Commission Decision 2011/833/EU"
  attribution_text: "Source: Eurostat"
  redistribution: { allowed: true, conditions_md: "acknowledgement required", commercial_restrictions_md: "some restrictions apply" }
`)},
		"series/paro-armonizado.yaml": &fstest.MapFile{Data: []byte(`
slug: paro-armonizado
source: eurostat
dataset: eurostat-une
unit: "% población activa 15-74 años"
frequency: Q
decimals: 1
source_refs:
  - kind: eurostat-dataset
    ref: une_rt_q
    dimensions: [freq, s_adj, age, unit, sex, geo, time]
    filters: { freq: Q, s_adj: SA, unit: PC_ACT, sex: T, geo: ES }
    valid_from: 2026-07-28
`)}, // age declared but NOT pinned
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	violations := config.Validate(cfg)
	found := false
	for _, v := range violations {
		if v.File == "series/paro-armonizado.yaml" && strings.Contains(v.Field, "age") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a violation naming the unpinned age dimension, got %v", violations)
	}
}

// Task 8.2 (RED) / 8.3 (GREEN): an xlsx-url series with no declared
// workbook structure fails validate-config naming the missing fields
// (spec source-ingestion-xlsx, "Workbook structure is declared in
// configuration, never in code" / "A configuration missing workbook
// structure fails validate-config").
func TestValidate_XLSXSeriesWithNoSheetNameOrHeaderFingerprintFailsNamingTheMissingFields(t *testing.T) {
	fsys := fstest.MapFS{
		"sources/seg-social.yaml": &fstest.MapFile{Data: []byte(completeSourceYAMLFor("seg-social"))},
		"series/afiliacion-ss.yaml": &fstest.MapFile{Data: []byte(`
slug: afiliacion-ss
name: "Afiliación media a la Seguridad Social"
source: seg-social
dataset: seg-social-afiliacion
unit: personas
frequency: M
decimals: 2
geo: ES
source_refs:
  - kind: xlsx-url
    ref: https://example.invalid/afiliacion.xlsx
    valid_from: 2026-07-28
`)},
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	violations := config.Validate(cfg)
	if !hasViolation(violations, "series/afiliacion-ss.yaml", "schema.xlsx.sheet_name") {
		t.Errorf("expected a violation naming the missing sheet_name, got %v", violations)
	}
	if !hasViolation(violations, "series/afiliacion-ss.yaml", "schema.xlsx.header_fingerprint") {
		t.Errorf("expected a violation naming the missing header_fingerprint, got %v", violations)
	}
	if !hasViolation(violations, "series/afiliacion-ss.yaml", "schema.xlsx.total_column") {
		t.Errorf("expected a violation naming the missing total_column (the arithmetic-invariant guard), got %v", violations)
	}
	if !hasViolation(violations, "series/afiliacion-ss.yaml", "schema.xlsx.component_columns") {
		t.Errorf("expected a violation naming the missing component_columns (the arithmetic-invariant guard), got %v", violations)
	}
}

// TestValidate_CompleteXLSXSeriesPasses is the happy-path counterpart:
// every structural field declared, no violation.
func TestValidate_CompleteXLSXSeriesPasses(t *testing.T) {
	fsys := fstest.MapFS{
		"sources/seg-social.yaml": &fstest.MapFile{Data: []byte(completeSourceYAMLFor("seg-social"))},
		"series/afiliacion-ss.yaml": &fstest.MapFile{Data: []byte(`
slug: afiliacion-ss
name: "Afiliación media a la Seguridad Social"
source: seg-social
dataset: seg-social-afiliacion
unit: personas
frequency: M
decimals: 2
geo: ES
source_refs:
  - kind: xlsx-url
    ref: https://example.invalid/afiliacion.xlsx
    valid_from: 2026-07-28
schema:
  xlsx:
    sheet_name: Hoja1
    header_row: 3
    column_anchors: { period: A, total: N }
    header_fingerprint: deadbeef
    total_column: N
    component_columns: [B, C, D, E, F, G, H, I, J, K, L, M]
    tolerance: 1.0
`)},
	}
	cfg, err := config.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if violations := config.Validate(cfg); hasViolationInFile(violations, "series/afiliacion-ss.yaml") {
		t.Fatalf("expected a complete xlsx series to pass, got violations: %v", violations)
	}
}

// completeSourceYAMLFor mirrors completeSourceYAML but for an arbitrary
// source id, so the XLSX tests above do not collide with completeSourceYAML's
// hard-coded "ine" id.
func completeSourceYAMLFor(id string) string {
	return `
id: ` + id + `
name: "Seguridad Social"
url: https://www.seg-social.es
access_type: xlsx-download
licence:
  name: "Reutilización con atribución"
  attribution_text: "Fuente: Seguridad Social"
  redistribution: { allowed: true, conditions_md: "cita la fuente", commercial_restrictions_md: "ninguna" }
`
}

func hasViolation(violations []config.Violation, file, field string) bool {
	for _, v := range violations {
		if v.File == file && v.Field == field {
			return true
		}
	}
	return false
}

func hasViolationInFile(violations []config.Violation, file string) bool {
	for _, v := range violations {
		if v.File == file {
			return true
		}
	}
	return false
}

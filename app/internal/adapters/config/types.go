// Package config parses the embedded /config tree (configdata.FS,
// ADR-1) into a typed Config and schema-validates it. It is the ONLY
// place in the Go source tree allowed to know the shape of an origin
// identifier reference — every adapter reads pinned series/source
// identifiers through this package, never as a Go literal (PRD §9.4,
// spec editorial-config "Source identifiers live only in
// configuration").
package config

import "time"

// Config is the fully-parsed, typed representation of everything under
// /config. Breaks and Events (Phase 7) are reconciled into series_break
// and event by ingestion.ReconcileEditorialConfig — this type only
// parses and schema-validates the YAML, it never touches the database.
type Config struct {
	// Sources is keyed by SourceConfig.ID (config/sources/{source}.yaml's
	// own `id` field, not the file name) so a series's `source:` reference
	// resolves by a single map lookup.
	Sources map[string]SourceConfig
	Series  []SeriesConfig

	// Breaks is every entry in config/rupturas.yaml (spec editorial-config,
	// "Editorial entries carry stable identifiers and a digest").
	Breaks []BreakConfig

	// Events is every entry in config/eventos.yaml (Group "exogenous" or
	// "milestones", read from the YAML) PLUS config/gobiernos.yaml (Group
	// always "governments", assigned by the loader — PRD §9.6 keeps
	// government changes in their own file with no per-entry group field
	// since the whole file is exactly one group).
	Events []EventConfig
}

// BreakConfig is one config/rupturas.yaml entry (PRD §9.6, filename fixed
// and kept in Spanish; field names stay English). It maps to one
// series_break row, scoped to a series, dataset or source so one break
// can apply to a whole family without a row per member series (spec
// "Breaks are scoped, not duplicated per series").
type BreakConfig struct {
	ID        string           `yaml:"id"`
	Kind      string           `yaml:"kind"`
	Scope     BreakScopeConfig `yaml:"scope"`
	NoteMD    string           `yaml:"note_md"`
	SourceURL string           `yaml:"source_url,omitempty"`

	// Date is the break's effective calendar date. It is a pointer
	// because an entry whose EXISTENCE is confirmed but whose EFFECTIVE
	// DATE is not yet confirmed against the source's own methodological
	// note MUST omit it rather than carry a guessed value (see
	// DateStatus below) — a wrong break date silently corrupts every
	// comparison across it (PRD's own stated worst failure mode for this
	// portal, principle P4).
	Date *time.Time `yaml:"date,omitempty"`

	// DateStatus is "" (confirmed — the default; Date MUST be set) or
	// "unconfirmed" (Date MAY be omitted; Todo MUST name the document to
	// consult). validate-config rejects any other value and rejects a
	// confirmed entry with no Date.
	DateStatus string `yaml:"date_status,omitempty"`

	// Todo is required when DateStatus is "unconfirmed": it names
	// exactly which source document must be consulted to confirm the
	// effective date. ReconcileEditorialConfig never projects an
	// unconfirmed entry's guessed date into series_break; it reconciles
	// the entry as pending-confirmation instead (ingestion package).
	Todo string `yaml:"todo,omitempty"`

	// FilePath is set by the loader, same rationale as SourceConfig.FilePath.
	FilePath string `yaml:"-"`
}

// BreakScopeConfig is a break's applicability: one series, one dataset
// (every series in it) or one source (every dataset in it) — design.md
// "series_break scope" decision.
type BreakScopeConfig struct {
	Kind string `yaml:"kind"` // series | dataset | source
	Ref  string `yaml:"ref"`

	// RefStatus mirrors BreakConfig.DateStatus's escape hatch, applied to
	// Ref instead of Date (remediation batch, verify-report WARNING W2):
	// "" (the default) means Ref MUST resolve against a configured
	// source/dataset/series, checked by validateScopeRef. "pending" means
	// Ref intentionally does not resolve YET — the same shape as the
	// ECOICOP v2 entries that resolve for zero configured series, made
	// explicit instead of silently shipping a typo-shaped dangling
	// reference (BreakConfig.Todo is then required, naming what is
	// missing, exactly as it already is for an unconfirmed date).
	RefStatus string `yaml:"ref_status,omitempty"`
}

// EventConfig is one config/eventos.yaml or config/gobiernos.yaml entry
// (PRD §9.6). It maps to one event row. Government entries carry NO
// party-colour field anywhere in this type (PRD §12.1 forbids colours
// readable as partisan) — only id/name/dates/note.
type EventConfig struct {
	ID     string `yaml:"id"`
	Group  string `yaml:"group,omitempty"` // exogenous | milestones | governments
	Name   string `yaml:"name"`
	NoteMD string `yaml:"note_md,omitempty"`

	DateStart *time.Time `yaml:"date_start,omitempty"`
	DateEnd   *time.Time `yaml:"date_end,omitempty"`

	// DateStatus/Todo mirror BreakConfig's: an event whose date is not
	// yet confirmed against its own source must say so explicitly rather
	// than carry a guessed value.
	DateStatus string `yaml:"date_status,omitempty"`
	Todo       string `yaml:"todo,omitempty"`

	// FilePath is set by the loader, same rationale as SourceConfig.FilePath.
	FilePath string `yaml:"-"`
}

// SourceConfig is one config/sources/{source}.yaml file (spec
// source-attribution-licensing, "Per-source licensing terms are
// authoritative").
type SourceConfig struct {
	ID         string        `yaml:"id"`
	Name       string        `yaml:"name"`
	URL        string        `yaml:"url"`
	AccessType string        `yaml:"access_type"`
	API        *APIConfig    `yaml:"api,omitempty"`
	Licence    LicenceConfig `yaml:"licence"`

	// FilePath is set by the loader (not the YAML content) so
	// validate-config's error messages can name the offending file, per
	// spec editorial-config's "the message names the file and the missing
	// field".
	FilePath string `yaml:"-"`
}

// APIConfig configures an api-json source's endpoint and response
// ceiling (design.md "Response ceiling" decision: io.LimitReader per
// source, default 8 MiB — Eurostat served 157 MB unfiltered, Engram
// #4692).
type APIConfig struct {
	BaseURL          string `yaml:"base_url"`
	MaxResponseBytes int64  `yaml:"max_response_bytes"`
}

// LicenceConfig is a source's licence, attribution and redistribution
// terms. These per-source terms are authoritative over any site-wide
// statement (settled decision D2; spec "No blanket data-licence claim
// exists in the repository").
type LicenceConfig struct {
	Name            string               `yaml:"name"`
	URL             string               `yaml:"url"`
	AttributionText string               `yaml:"attribution_text"`
	Redistribution  RedistributionConfig `yaml:"redistribution"`
}

// RedistributionConfig records redistribution terms as structured,
// testable fields rather than flattening them into free text, so a fact
// like Eurostat's Commission Decision 2011/833/EU scope
// (acknowledgement-only, no third-party coverage, some commercial
// redissemination restricted) is a fact validate-config and its tests
// can check, not prose nobody verifies (spec "Eurostat restrictions are
// recorded, not flattened").
type RedistributionConfig struct {
	Allowed                  bool   `yaml:"allowed"`
	ConditionsMD             string `yaml:"conditions_md"`
	CommercialRestrictionsMD string `yaml:"commercial_restrictions_md"`
	AcknowledgementRequired  bool   `yaml:"acknowledgement_required"`
	ThirdPartyExcluded       bool   `yaml:"third_party_excluded"`
}

// SeriesConfig is one config/series/{slug}.yaml file (spec
// editorial-config, "A series config declares its full identity").
type SeriesConfig struct {
	Slug       string           `yaml:"slug"`
	Name       string           `yaml:"name"`
	Source     string           `yaml:"source"`
	Dataset    string           `yaml:"dataset"`
	Unit       string           `yaml:"unit"`
	Frequency  string           `yaml:"frequency"`
	Decimals   int              `yaml:"decimals"`
	Geo        string           `yaml:"geo"`
	Harmonized bool             `yaml:"harmonized"`
	SourceRefs []SourceRef      `yaml:"source_refs"`
	Validation ValidationConfig `yaml:"validation"`

	// Schema declares the fields validation rule 1 expects this
	// series' payload to contain (Phase 4, task 4.3/4.4). Optional:
	// zero value means rule 1 has nothing to check for that series.
	Schema SchemaConfig `yaml:"schema,omitempty"`

	// FilePath is set by the loader, same rationale as SourceConfig.FilePath.
	FilePath string `yaml:"-"`
}

// SchemaConfig declares what validation rule 1 (schema) expects a
// series' payload to contain (spec data-validation, "Rule 1 —
// schema"). ExpectedFields applies to every source kind; XLSX is the
// only kind that also needs sheet/header/anchor/fingerprint checks
// (design.md "For workbook sources the check MUST include sheet name,
// header row index, column anchors and a header fingerprint").
type SchemaConfig struct {
	ExpectedFields []string          `yaml:"expected_fields,omitempty"`
	XLSX           *XLSXSchemaConfig `yaml:"xlsx,omitempty"`
}

// XLSXSchemaConfig is the workbook-specific half of SchemaConfig.
//
// TotalColumn/ComponentColumns/Tolerance were added in slice 8 (task
// 8.1's resolution, Engram #4699 as corrected during implementation --
// see the doc comment on package xlsx's Decode): the real Social
// Security affiliation workbook's header row does NOT align with its
// data columns (merged cells in rows 2-3 offset the labels), so header
// text can never be trusted to identify the total column. The declared
// TotalColumn MUST equal the sum of ComponentColumns within Tolerance on
// every parsed row; a mismatch is schema drift, not a data error, and
// blocks publication (spec source-ingestion-xlsx, "Column mapping is
// pinned by position and guarded by an arithmetic invariant"). This is
// exactly the additive extension PR 4a's own doc comment anticipated:
// "adding it later only means adding a new field here, never
// restructuring the existing ones."
type XLSXSchemaConfig struct {
	SheetName         string            `yaml:"sheet_name,omitempty"`
	HeaderRow         int               `yaml:"header_row,omitempty"`
	ColumnAnchors     map[string]string `yaml:"column_anchors,omitempty"` // declared field -> expected column reference
	HeaderFingerprint string            `yaml:"header_fingerprint,omitempty"`

	// TotalColumn is the column letter carrying the aggregate value a
	// row's ComponentColumns must sum to.
	TotalColumn string `yaml:"total_column,omitempty"`
	// ComponentColumns are the column letters whose values (blank == 0)
	// must sum to TotalColumn within Tolerance on every data row.
	ComponentColumns []string `yaml:"component_columns,omitempty"`
	// Tolerance is the maximum absolute |total - sum(components)|
	// difference still accepted as the source's own floating-point
	// rounding, not schema drift. Declared per series, never hard-coded
	// in Go, because how much rounding is "normal" is a fact about the
	// specific published workbook.
	Tolerance float64 `yaml:"tolerance,omitempty"`
}

// SourceRef is one validity-ranged origin reference — the config-only
// home for INE series CODs/table Ids, Eurostat dataset codes and
// dimension filters, and XLSX URLs (design.md series_source_mapping;
// spec "Source identifiers live only in configuration").
type SourceRef struct {
	Kind      string     `yaml:"kind"` // ine-series-cod | eurostat-dataset | xlsx-url
	Ref       string     `yaml:"ref"`
	TableHint string     `yaml:"table_hint,omitempty"` // INE discovery-only hint; never the ingest identifier (ADR-2)
	ValidFrom time.Time  `yaml:"valid_from"`
	ValidTo   *time.Time `yaml:"valid_to"`

	// Eurostat-only: the full declared dimension list and the pinned
	// filter for every dimension except time (design.md "/config file
	// schemas", the Eurostat pinning example; validated by
	// validateEurostatPinning).
	//
	// Design choice (task 6.6/6.7): validateEurostatPinning needs a
	// dataset's COMPLETE dimension list to know which pins are missing --
	// it cannot just check "are the filters we happen to have declared
	// self-consistent", it must check "is anything left un-pinned at
	// all". There were three ways to give it that list:
	//   1. Hard-code each dataset's dimension list as a Go literal --
	//      rejected: fragile (Eurostat can add a dimension to a dataset
	//      without notice) AND directly forbidden by
	//      app/internal/guard's origin-identifier deny-list, which
	//      already treats "prc_hicp_minr"/"une_rt_q"/"nama_10_gdp" and
	//      their dimension names as config-only values, never Go source.
	//   2. Declare it here, in config, alongside the pins themselves --
	//      CHOSEN. It is inert data (no code to keep in sync), it is
	//      reviewable in the same four-eyes PR that adds the pins, and it
	//      needs no network access to validate in CI or offline.
	//   3. Fetch each dataset's live structure at validate-config time --
	//      rejected: validate-config is a CI gate (task 3.8) that MUST
	//      run offline and deterministically; a live HTTP dependency
	//      would make the gate flaky and unusable in an offline
	//      environment, and would silently change behaviour if Eurostat
	//      altered a dataset between two otherwise-identical CI runs.
	// The cost of option 2 is that Dimensions and Filters can, in
	// principle, drift apart by hand-editing error (e.g. a typo'd
	// dimension name in one but not the other) -- validateEurostatPinning
	// catches exactly that: any name present in Dimensions but absent
	// from Filters (other than "time") fails naming it.
	Dimensions []string          `yaml:"dimensions,omitempty"`
	Filters    map[string]string `yaml:"filters,omitempty"`
}

// ValidationConfig carries the per-series thresholds consumed by
// validation rules 2-4 (design.md "five pure rules + gate", Phase 4).
type ValidationConfig struct {
	Plausibility PlausibilityConfig `yaml:"plausibility"`
	Continuity   ContinuityConfig   `yaml:"continuity"`
	Revision     RevisionConfig     `yaml:"revision"`
}

// PlausibilityConfig backs validation rule 3 (out of range / delta
// breach unless a resolved break covers the period).
type PlausibilityConfig struct {
	Min         *float64 `yaml:"min"`
	Max         *float64 `yaml:"max"`
	MaxDeltaAbs *float64 `yaml:"max_delta_abs"`
}

// ContinuityConfig backs validation rule 2 (documented-gap allowlist).
type ContinuityConfig struct {
	DocumentedGaps []string `yaml:"documented_gaps"`
}

// RevisionConfig backs validation rule 4. MaxBackwardPeriods defaults to
// 4 when omitted (design.md); the zero value here means "use the
// package default", resolved by the validation rule itself in Phase 4,
// not by this loader.
type RevisionConfig struct {
	MaxBackwardPeriods int `yaml:"max_backward_periods"`
}

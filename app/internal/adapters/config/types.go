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

	// Acknowledgements is every entry in config/reconocimientos.yaml (spec
	// data-validation, "Acknowledged findings") — the editorial record
	// that a named human reviewed one specific blocking validation finding
	// and confirmed the underlying datum. Reconciled into
	// validation_acknowledgement by ingestion.ReconcileEditorialConfig
	// alongside Breaks and Events, in the same transaction. See
	// acknowledgement.go for the whole rationale, including why this is a
	// registry of its own and not a break.
	Acknowledgements []AcknowledgementConfig
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

	// Date is the break's effective calendar date. It is a pointer because
	// an entry whose EXISTENCE is confirmed but whose EFFECTIVE DATE is
	// not yet confirmed against the source's own methodological note may
	// leave it unset — a wrong break date silently corrupts every
	// comparison across it (PRD's own stated worst failure mode for this
	// portal, principle P4).
	//
	// A PROVISIONAL date alongside DateStatus below is allowed, and is
	// usually the better entry: the guess plus the Todo tells the next
	// editor what to check and what the current best reading is, where an
	// empty field tells them only that somebody stopped. What makes it safe
	// is DateStatus, not the emptiness of this field — see DateStatus.
	Date *time.Time `yaml:"date,omitempty"`

	// DateStatus is "" (confirmed — the default; Date MUST be set) or
	// DateStatusUnconfirmed (Todo MUST name the document to consult; Date
	// may be set to a provisional value or left unset). validate-config
	// rejects any other value and rejects a confirmed entry with no Date.
	//
	// THIS FIELD, NOT A NIL Date, IS WHAT KEEPS AN UNGUARANTEED DATE OUT OF
	// THE DATABASE. ingestion.ReconcileEditorialConfig reads it directly
	// (isDatePending); nothing downstream of series_break carries the
	// qualifier, so a provisional date that got projected would look
	// exactly as authoritative as a confirmed one.
	DateStatus string `yaml:"date_status,omitempty"`

	// Todo is required when DateStatus is DateStatusUnconfirmed: it names
	// exactly which source document must be consulted to confirm the
	// effective date. ReconcileEditorialConfig never projects an
	// unconfirmed entry into series_break; it reconciles the entry as
	// pending-confirmation instead (ingestion package).
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
// readable as partisan) — only id/name/dates/note/scope.
type EventConfig struct {
	ID     string `yaml:"id"`
	Group  string `yaml:"group,omitempty"` // exogenous | milestones | governments
	Name   string `yaml:"name"`
	NoteMD string `yaml:"note_md,omitempty"`

	// Scope is the entry's applicability, mirroring BreakScopeConfig's
	// shape and widened by one kind (design.md "series_break scope";
	// postgres.ResolveActiveBreaksForSeries).
	//
	// It closes a gap the code used to admit in its own words: before this
	// field, event carried no scope columns at all and
	// postgres.ListActiveEvents said so in its doc comment ("every
	// currently active event is, by the schema this change inherited,
	// global"), accepting a seriesID it did not read. Every entry authored
	// today is still global — a change of government and a worldwide shock
	// are facts about the calendar and apply wherever the calendar does —
	// but "global" is now a value the registry states rather than a
	// property of the schema, and an entry that applies to one series,
	// dataset or source has somewhere to say so.
	//
	// The loader NORMALISES an omitted scope to Kind "global" (loader.go),
	// so no consumer downstream ever has to decide what "" means.
	Scope EventScopeConfig `yaml:"scope,omitempty"`

	// SourceURL is the document the entry was verified against — the same
	// field, for the same reason, BreakConfig has carried since Phase 7.
	// Optional: an event registry entry that HAS a document to point at
	// should point at it, and a change of government has none.
	SourceURL string `yaml:"source_url,omitempty"`

	DateStart *time.Time `yaml:"date_start,omitempty"`
	DateEnd   *time.Time `yaml:"date_end,omitempty"`

	// DateStatus/Todo mirror BreakConfig's, field for field and rule for
	// rule: an event whose date is not yet confirmed against its own source
	// must SAY SO here, and saying so is what holds it back from the event
	// table — a provisional DateStart alongside it is allowed and does not
	// weaken that.
	DateStatus string `yaml:"date_status,omitempty"`
	Todo       string `yaml:"todo,omitempty"`

	// FilePath is set by the loader, same rationale as SourceConfig.FilePath.
	FilePath string `yaml:"-"`
}

// EventScopeConfig is an event's applicability. It is deliberately its OWN
// type rather than a reuse of BreakScopeConfig, on two counts:
//
//   - It admits a fourth kind, "global", which a break cannot have. Every
//     break is a fact about a specific series, dataset or source; a change
//     of government or a worldwide shock is a fact about the calendar and
//     applies wherever the calendar does. Widening BreakScopeConfig instead
//     would have made "global" expressible in rupturas.yaml, where it means
//     nothing.
//   - It carries no RefStatus. That escape hatch exists for breaks because
//     a real methodological rupture can be documented before the series it
//     touches is configured (the ECOICOP v2 entries). An event's scope
//     names series this portal already publishes — a scope pointing at
//     nothing would simply mean the entry is invisible, which is a reason
//     not to write it yet rather than a state to encode.
type EventScopeConfig struct {
	Kind string `yaml:"kind"` // global | series | dataset | source
	Ref  string `yaml:"ref,omitempty"`
}

// The four scope kinds an event may declare. Named constants rather than
// bare literals because each value crosses three layers unchanged — YAML,
// the `event` table's own columns, and the SQL predicate that reads them —
// so a typo in any one of them would otherwise be a silently-invisible
// entry rather than a compile error.
const (
	EventScopeGlobal  = "global"
	EventScopeSeries  = "series"
	EventScopeDataset = "dataset"
	EventScopeSource  = "source"
)

// DateStatusUnconfirmed is the one non-empty value BreakConfig.DateStatus
// and EventConfig.DateStatus may take: the editor declaring, in the YAML,
// that this entry's effective date is not yet confirmed against its source
// document.
//
// Named for the same reason the scope kinds above are, and with a sharper
// edge. This literal is what validate-config accepts and what the reconcile
// tests before deciding whether to project a row; a typo in either place
// does not fail loudly, it silently reclassifies an unconfirmed entry as
// confirmed and lets a guessed date reach the database.
const DateStatusUnconfirmed = "unconfirmed"

// eventGroups is every group an entry may declare. `governments` is never
// written by hand (the loader assigns it from the file), but it is listed
// here because Validate sees the loaded value, not the YAML.
var eventGroups = map[string]bool{
	"exogenous":   true,
	"milestones":  true,
	"governments": true,
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

	// CadenceSegments declares a cadence that changes over the series'
	// life (spec editorial-config, "A series configuration expresses a
	// cadence that changes over its life"; design D-4). Empty means the
	// ordinary, common case: Frequency alone describes the whole series
	// uniformly. Deliberately plain strings/ints here, not an
	// indicators.Period/Frequency -- this package stays decoupled from
	// the domain layer exactly like Frequency itself already does (see
	// that field's own doc comment); adapters/config's ONLY caller-facing
	// job is parsing and schema-validating the YAML shape.
	CadenceSegments []CadenceSegmentConfig `yaml:"cadence_segments,omitempty"`

	// Discontinued marks a series the SOURCE has stopped publishing
	// (spec indicator-page, "The three page states of PRD §6.1.3",
	// scenario "A discontinued series shows a permanent banner": "GIVEN
	// a series marked discontinued by its source"). A pointer, not a
	// value: nil means "live", which is every series configured today,
	// and a zero-valued struct would be indistinguishable from a
	// discontinued series with a missing date.
	//
	// This is editorial configuration, not an observable fact of the
	// payload -- no source in this project announces its own retirement
	// in-band. It follows the same config -> reconcile -> database ->
	// export path the break and event registries already take, so the
	// artifact's page state stays reproducible from the database alone
	// (publishing.Export reads exclusively through postgres ports).
	Discontinued *DiscontinuedConfig `yaml:"discontinued,omitempty"`

	// FilePath is set by the loader, same rationale as SourceConfig.FilePath.
	FilePath string `yaml:"-"`
}

// DiscontinuedConfig is one config/series/{slug}.yaml `discontinued`
// block.
//
// Since is an ISO date (YYYY-MM-DD), kept as a plain string for the same
// reason SeriesConfig.Frequency is: this package's only job is parsing
// and schema-validating the YAML shape, and it stays decoupled from the
// domain and from time.Time.
//
// Successor is optional -- the spec says "where one exists", and a
// source can retire a series without publishing a replacement. When
// present it MUST name another configured series (validate.go), because
// a successor link that 404s strands the reader worse than no link does.
type DiscontinuedConfig struct {
	Since     string `yaml:"since"`
	Successor string `yaml:"successor,omitempty"`
}

// CadenceSegmentConfig is one config/series/{slug}.yaml cadence_segments
// entry (spec editorial-config, "A series configuration expresses a
// cadence that changes over its life"; D3 adjudication, orchestrator
// settled). From/To are period labels on the series' own base Frequency
// grid ("1977-Q1", "2023-Q2" for a quarterly series); To empty means
// open-ended (must be the series' last segment). Cadence is the source's
// own descriptive word for this segment's real-world cadence
// ("semiannual", "quarterly") -- diagnostic and legibility only, never a
// domain Frequency. Present lists the grid ordinals (1-4 for quarterly,
// 1-12 for monthly) this segment expects populated; empty means every
// ordinal is expected (an ordinary dense segment).
type CadenceSegmentConfig struct {
	From    string `yaml:"from"`
	To      string `yaml:"to,omitempty"`
	Cadence string `yaml:"cadence"`
	Present []int  `yaml:"present,omitempty"`
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

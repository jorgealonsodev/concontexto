package config

// Task 3.7 (RED) / 3.8 (GREEN): schema validation for the `validate-config`
// subcommand (spec editorial-config, "validate-config subcommand").
// Task 3.9 (RED) / 3.10 (GREEN): every source needs complete licensing
// terms (spec source-attribution-licensing, "Per-source licensing terms
// are authoritative"; "The attribution table is closed before anything
// is published").
// Task 3.9's Eurostat guard: any eurostat-dataset source ref must pin
// every declared dimension except "time" (design.md's Eurostat guard —
// an unfiltered prc_hicp_minr request served 157 MB against a 256 MB
// container, Engram #4692).

import "fmt"

// Violation is one schema violation. Its String form is exactly the
// "names the file and field" shape validate-config's CLI output prints.
type Violation struct {
	File    string
	Field   string
	Message string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s: %s: %s", v.File, v.Field, v.Message)
}

// Validate schema-validates a loaded Config and returns every violation
// found. An empty (nil) slice means the tree is valid (spec "A valid
// configuration set passes"); Validate has no side effects and never
// panics, so validate-config's exit code is driven entirely by
// len(Validate(cfg)).
func Validate(cfg *Config) []Violation {
	var violations []Violation
	for _, src := range cfg.Sources {
		violations = append(violations, validateSource(src)...)
	}
	for _, series := range cfg.Series {
		violations = append(violations, validateSeries(cfg, series)...)
	}
	for _, brk := range cfg.Breaks {
		violations = append(violations, validateBreak(brk)...)
		violations = append(violations, validateScopeRef(cfg, brk)...)
	}
	violations = append(violations, validateDuplicateBreakIDs(cfg.Breaks)...)
	for _, ev := range cfg.Events {
		violations = append(violations, validateEvent(ev)...)
	}
	violations = append(violations, validateDuplicateEventIDs(cfg.Events)...)
	return violations
}

// validateSource enforces spec source-attribution-licensing's "A source
// declares complete terms": licence, attribution text, access type and
// redistribution restrictions must all be present.
func validateSource(src SourceConfig) []Violation {
	var out []Violation
	require := func(field, value string) {
		if value == "" {
			out = append(out, Violation{File: src.FilePath, Field: field, Message: "required field is missing"})
		}
	}
	require("id", src.ID)
	require("name", src.Name)
	require("url", src.URL)
	require("access_type", src.AccessType)
	require("licence.name", src.Licence.Name)
	require("licence.attribution_text", src.Licence.AttributionText)
	require("licence.redistribution.conditions_md", src.Licence.Redistribution.ConditionsMD)
	require("licence.redistribution.commercial_restrictions_md", src.Licence.Redistribution.CommercialRestrictionsMD)
	return out
}

// validateSeries enforces spec editorial-config's "A series config
// declares its full identity" (unit, frequency, >=1 source ref) and "A
// reference to an unknown source is rejected".
func validateSeries(cfg *Config, s SeriesConfig) []Violation {
	var out []Violation
	if s.Unit == "" {
		out = append(out, Violation{File: s.FilePath, Field: "unit", Message: "required field is missing"})
	}
	if s.Frequency == "" {
		out = append(out, Violation{File: s.FilePath, Field: "frequency", Message: "required field is missing"})
	}
	if len(s.SourceRefs) == 0 {
		out = append(out, Violation{File: s.FilePath, Field: "source_refs", Message: "at least one validity-ranged source reference is required"})
	}
	if s.Source != "" {
		if _, ok := cfg.Sources[s.Source]; !ok {
			out = append(out, Violation{
				File:    s.FilePath,
				Field:   "source",
				Message: fmt.Sprintf("references unknown source %q (no sources/%s.yaml with that id)", s.Source, s.Source),
			})
		}
	}
	var hasXLSXRef bool
	for _, ref := range s.SourceRefs {
		if ref.Kind == "eurostat-dataset" {
			out = append(out, validateEurostatPinning(s.FilePath, ref)...)
		}
		if ref.Kind == "xlsx-url" {
			hasXLSXRef = true
		}
	}
	if hasXLSXRef {
		out = append(out, validateXLSXSchema(s.FilePath, s.Schema.XLSX)...)
	}
	return out
}

// validateXLSXSchema enforces spec source-ingestion-xlsx's "Workbook
// structure is declared in configuration, never in code": any series
// referencing an xlsx-url source MUST declare the sheet name, header
// fingerprint, and the arithmetic-invariant pair (total_column +
// component_columns) that guards against the header/column misalignment
// verified in the real Social Security workbook (task 8.1's resolution —
// row 3 labels column K "Discontínuos (7)" while the real total lives
// elsewhere; header text can never be trusted, so the parser is pinned by
// column letter and cross-checked arithmetically instead). xlsx == nil
// (the whole schema.xlsx block omitted) is reported as every field
// missing, not a separate error shape, so the message always names each
// concrete missing field.
func validateXLSXSchema(filePath string, xlsx *XLSXSchemaConfig) []Violation {
	var out []Violation
	require := func(field string, present bool) {
		if !present {
			out = append(out, Violation{File: filePath, Field: "schema.xlsx." + field, Message: "required for an xlsx-url series (workbook structure MUST live in configuration, never in code)"})
		}
	}
	require("sheet_name", xlsx != nil && xlsx.SheetName != "")
	require("header_fingerprint", xlsx != nil && xlsx.HeaderFingerprint != "")
	require("total_column", xlsx != nil && xlsx.TotalColumn != "")
	require("component_columns", xlsx != nil && len(xlsx.ComponentColumns) > 0)
	return out
}

// validateEurostatPinning enforces design.md's Eurostat guard: every
// dimension the series ref declares, except "time", must have a pinned
// value in filters. Without this, an unfiltered request can return HTTP
// 200 with a response far larger than the container's memory limit
// (verified live: prc_hicp_minr served 157 MB unfiltered against a
// 256 MB container, Engram #4692).
func validateEurostatPinning(filePath string, ref SourceRef) []Violation {
	var out []Violation
	for _, dim := range ref.Dimensions {
		if dim == "time" {
			continue
		}
		if _, pinned := ref.Filters[dim]; !pinned {
			out = append(out, Violation{
				File:  filePath,
				Field: fmt.Sprintf("source_refs[%s].filters.%s", ref.Ref, dim),
				Message: "every Eurostat dimension except time must be pinned in filters " +
					"(an unfiltered request can return HTTP 200 with a response far larger than the container's memory limit)",
			})
		}
	}
	return out
}

// validateBreak enforces spec editorial-config's break requirements: a
// stable id (checked for uniqueness separately, validateDuplicateBreakIDs),
// kind, scope, note_md, and EITHER a confirmed Date OR an explicit
// DateStatus "unconfirmed" plus a Todo naming the document to consult
// (PRD's own worst failure mode is a silently wrong break date — the
// schema refuses to let one exist without a date OR an honest admission
// that the date is not yet known).
func validateBreak(b BreakConfig) []Violation {
	var out []Violation
	require := func(field, value string) {
		if value == "" {
			out = append(out, Violation{File: b.FilePath, Field: field, Message: fmt.Sprintf("break %q: required field is missing", b.ID)})
		}
	}
	require("id", b.ID)
	require("kind", b.Kind)
	require("scope.kind", b.Scope.Kind)
	require("scope.ref", b.Scope.Ref)
	require("note_md", b.NoteMD)

	switch b.DateStatus {
	case "":
		if b.Date == nil {
			out = append(out, Violation{
				File: b.FilePath, Field: "date",
				Message: fmt.Sprintf("break %q: date is required unless date_status is \"unconfirmed\" "+
					"(a break's effective date must never be guessed — PRD principle P4)", b.ID),
			})
		}
	case "unconfirmed":
		if b.Todo == "" {
			out = append(out, Violation{
				File: b.FilePath, Field: "todo",
				Message: fmt.Sprintf("break %q: date_status is \"unconfirmed\" but todo does not name the document to consult", b.ID),
			})
		}
	default:
		out = append(out, Violation{
			File: b.FilePath, Field: "date_status",
			Message: fmt.Sprintf("break %q: date_status %q is not \"unconfirmed\" (leave it empty when the date is confirmed)", b.ID, b.DateStatus),
		})
	}
	return out
}

// validateScopeRef enforces design.md's "validate-config checks: ...
// scope refs resolve" (remediation batch, verify-report WARNING W2): a
// break's scope.ref must resolve against a configured source
// (scope.kind=="source"), a configured dataset — i.e. some series names
// it (scope.kind=="dataset") — or a configured series
// (scope.kind=="series"), UNLESS the entry declares
// scope.ref_status: "pending", the same escape hatch DateStatus already
// gives an unconfirmed date: a scope that is not yet configured (Fase 1
// work, per the ECOICOP v2 entries' own disclosure) must say so
// explicitly rather than silently ship a dangling reference
// indistinguishable from a typo.
func validateScopeRef(cfg *Config, b BreakConfig) []Violation {
	switch b.Scope.RefStatus {
	case "":
		// falls through to the resolution check below
	case "pending":
		if b.Todo == "" {
			return []Violation{{
				File: b.FilePath, Field: "todo",
				Message: fmt.Sprintf("break %q: scope.ref_status is \"pending\" but todo does not name what would need to be configured", b.ID),
			}}
		}
		return nil
	default:
		return []Violation{{
			File: b.FilePath, Field: "scope.ref_status",
			Message: fmt.Sprintf("break %q: scope.ref_status %q is not \"pending\" (leave it empty when the ref is expected to resolve)", b.ID, b.Scope.RefStatus),
		}}
	}

	switch b.Scope.Kind {
	case "source":
		if _, ok := cfg.Sources[b.Scope.Ref]; !ok {
			return []Violation{{
				File: b.FilePath, Field: "scope.ref",
				Message: fmt.Sprintf("break %q: scope.ref %q does not resolve to any configured source (no sources/%s.yaml with that id)", b.ID, b.Scope.Ref, b.Scope.Ref),
			}}
		}
	case "dataset":
		for _, s := range cfg.Series {
			if s.Dataset == b.Scope.Ref {
				return nil
			}
		}
		return []Violation{{
			File: b.FilePath, Field: "scope.ref",
			Message: fmt.Sprintf("break %q: scope.ref %q does not resolve to any configured series' dataset", b.ID, b.Scope.Ref),
		}}
	case "series":
		for _, s := range cfg.Series {
			if s.Slug == b.Scope.Ref {
				return nil
			}
		}
		return []Violation{{
			File: b.FilePath, Field: "scope.ref",
			Message: fmt.Sprintf("break %q: scope.ref %q does not resolve to any configured series", b.ID, b.Scope.Ref),
		}}
	}
	// An empty/unrecognised scope.kind is already reported by
	// validateBreak's own "scope.kind" required-field check — never
	// double-reported here.
	return nil
}

// validateDuplicateBreakIDs enforces spec editorial-config's "Duplicate
// ids are rejected" for rupturas.yaml (all Breaks come from that one
// file today, but the check is not file-scoped in case a future file
// ever contributes Breaks too).
func validateDuplicateBreakIDs(breaks []BreakConfig) []Violation {
	var out []Violation
	seen := map[string]bool{}
	for _, b := range breaks {
		if b.ID == "" {
			continue // already reported by validateBreak's own "id" check
		}
		if seen[b.ID] {
			out = append(out, Violation{
				File: b.FilePath, Field: "id",
				Message: fmt.Sprintf("duplicated id %q — every break/event id must be unique", b.ID),
			})
			continue
		}
		seen[b.ID] = true
	}
	return out
}

// validateEvent enforces spec editorial-config's requirements for
// eventos.yaml/gobiernos.yaml entries: a stable id, group, name, a
// DateStart, and the same confirmed-or-explicitly-unconfirmed date
// discipline validateBreak applies.
func validateEvent(ev EventConfig) []Violation {
	var out []Violation
	require := func(field, value string) {
		if value == "" {
			out = append(out, Violation{File: ev.FilePath, Field: field, Message: fmt.Sprintf("event %q: required field is missing", ev.ID)})
		}
	}
	require("id", ev.ID)
	require("group", ev.Group)
	require("name", ev.Name)

	switch ev.DateStatus {
	case "":
		if ev.DateStart == nil {
			out = append(out, Violation{
				File: ev.FilePath, Field: "date_start",
				Message: fmt.Sprintf("event %q: date_start is required unless date_status is \"unconfirmed\"", ev.ID),
			})
		}
	case "unconfirmed":
		if ev.Todo == "" {
			out = append(out, Violation{
				File: ev.FilePath, Field: "todo",
				Message: fmt.Sprintf("event %q: date_status is \"unconfirmed\" but todo does not name the document to consult", ev.ID),
			})
		}
	default:
		out = append(out, Violation{
			File: ev.FilePath, Field: "date_status",
			Message: fmt.Sprintf("event %q: date_status %q is not \"unconfirmed\" (leave it empty when the date is confirmed)", ev.ID, ev.DateStatus),
		})
	}
	return out
}

// validateDuplicateEventIDs enforces id uniqueness ACROSS eventos.yaml
// AND gobiernos.yaml together, not per file: event.id is a single
// PRIMARY KEY (migration 0001) shared by every row reconciled from
// either file, so an id reused across the two files would silently
// collide at reconcile time — a per-file check alone would miss it.
func validateDuplicateEventIDs(events []EventConfig) []Violation {
	var out []Violation
	seen := map[string]bool{}
	for _, ev := range events {
		if ev.ID == "" {
			continue // already reported by validateEvent's own "id" check
		}
		if seen[ev.ID] {
			out = append(out, Violation{
				File: ev.FilePath, Field: "id",
				Message: fmt.Sprintf("duplicated id %q — every break/event id must be unique", ev.ID),
			})
			continue
		}
		seen[ev.ID] = true
	}
	return out
}

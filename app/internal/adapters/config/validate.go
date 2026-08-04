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

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"
)

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
		violations = append(violations, validateEventScope(cfg, ev)...)
	}
	violations = append(violations, validateDuplicateEventIDs(cfg.Events)...)
	for _, ack := range cfg.Acknowledgements {
		violations = append(violations, validateAcknowledgement(cfg, ack)...)
	}
	violations = append(violations, validateDuplicateAcknowledgementIDs(cfg.Acknowledgements)...)
	violations = append(violations, validateDuplicateAcknowledgementScopes(cfg.Acknowledgements)...)
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
	out = append(out, validateCadenceSegments(s)...)
	out = append(out, validateDiscontinued(cfg, s)...)
	return out
}

// isoDatePattern is the YYYY-MM-DD shape `discontinued.since` must take.
// Deliberately a shape check plus a real calendar parse, not a shape
// check alone: "2026-02-31" matches the pattern and is not a date.
var isoDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// validateDiscontinued enforces the shape of the `discontinued` block
// (spec indicator-page, "A discontinued series shows a permanent
// banner"). A nil block is the ordinary live case and is always valid.
func validateDiscontinued(cfg *Config, s SeriesConfig) []Violation {
	d := s.Discontinued
	if d == nil {
		return nil
	}
	var out []Violation

	// The banner is permanent and states WHEN the source stopped
	// publishing. Without a date there is nothing truthful to render.
	if d.Since == "" {
		out = append(out, Violation{File: s.FilePath, Field: "discontinued.since", Message: "required field is missing"})
	} else if !isoDatePattern.MatchString(d.Since) {
		out = append(out, Violation{
			File: s.FilePath, Field: "discontinued.since",
			Message: fmt.Sprintf("must be an ISO date (YYYY-MM-DD), got %q", d.Since),
		})
	} else if _, err := time.Parse("2006-01-02", d.Since); err != nil {
		out = append(out, Violation{
			File: s.FilePath, Field: "discontinued.since",
			Message: fmt.Sprintf("is not a real calendar date: %q", d.Since),
		})
	}

	if d.Successor == "" {
		return out // optional -- the spec says "where one exists"
	}
	if d.Successor == s.Slug {
		out = append(out, Violation{
			File: s.FilePath, Field: "discontinued.successor",
			Message: "names the series itself; a successor link must lead somewhere else",
		})
		return out
	}
	var known bool
	for _, other := range cfg.Series {
		if other.Slug == d.Successor {
			known = true
			break
		}
	}
	if !known {
		out = append(out, Violation{
			File: s.FilePath, Field: "discontinued.successor",
			Message: fmt.Sprintf("references unknown series %q (no series/%s.yaml with that slug)", d.Successor, d.Successor),
		})
	}
	return out
}

// cadencePeriodPattern recognises the same "YYYY-Qn" / "YYYY-MM" label
// shapes indicators.NormalizePeriodLabel does (indicators/period.go).
// Deliberately re-implemented here, not imported, to keep this package
// decoupled from the domain layer -- SeriesConfig.Frequency has followed
// that same "plain string, no domain type" convention since Load was
// first written; a cadence_segments boundary is validated the same way.
var (
	cadenceQuarterlyPattern = regexp.MustCompile(`^(\d{4})-Q([1-4])$`)
	cadenceMonthlyPattern   = regexp.MustCompile(`^(\d{4})-(0[1-9]|1[0-2])$`)
)

// cadenceOrdinal is a package-local (year, ordinal) period value on a
// series' own base-grid Frequency, used only to order and check the
// continuity of cadence_segments boundaries.
type cadenceOrdinal struct {
	year, ordinal, stepsPerYear int
}

func (o cadenceOrdinal) before(other cadenceOrdinal) bool {
	if o.year != other.year {
		return o.year < other.year
	}
	return o.ordinal < other.ordinal
}

func (o cadenceOrdinal) next() cadenceOrdinal {
	if o.ordinal >= o.stepsPerYear {
		return cadenceOrdinal{year: o.year + 1, ordinal: 1, stepsPerYear: o.stepsPerYear}
	}
	return cadenceOrdinal{year: o.year, ordinal: o.ordinal + 1, stepsPerYear: o.stepsPerYear}
}

func (o cadenceOrdinal) label(freq string) string {
	if freq == "M" {
		return fmt.Sprintf("%04d-%02d", o.year, o.ordinal)
	}
	return fmt.Sprintf("%04d-Q%d", o.year, o.ordinal)
}

func parseCadenceOrdinal(raw, freq string) (cadenceOrdinal, error) {
	switch freq {
	case "M":
		if m := cadenceMonthlyPattern.FindStringSubmatch(raw); m != nil {
			year, _ := strconv.Atoi(m[1])
			month, _ := strconv.Atoi(m[2])
			return cadenceOrdinal{year: year, ordinal: month, stepsPerYear: 12}, nil
		}
		return cadenceOrdinal{}, fmt.Errorf("%q is not a valid monthly period label (expected YYYY-MM)", raw)
	default: // "Q" and any other frequency validate as quarterly-shaped labels
		if m := cadenceQuarterlyPattern.FindStringSubmatch(raw); m != nil {
			year, _ := strconv.Atoi(m[1])
			quarter, _ := strconv.Atoi(m[2])
			return cadenceOrdinal{year: year, ordinal: quarter, stepsPerYear: 4}, nil
		}
		return cadenceOrdinal{}, fmt.Errorf("%q is not a valid quarterly period label (expected YYYY-Qn)", raw)
	}
}

// validateCadenceSegments enforces spec editorial-config's "A series
// configuration expresses a cadence that changes over its life":
// segments must be ordered, non-overlapping, leave no gap, and the last
// segment must stay open-ended (no "to") so it keeps covering ongoing
// history. A series declaring no cadence_segments (the common, uniform
// case) is not touched at all (spec "A uniform cadence still validates").
func validateCadenceSegments(s SeriesConfig) []Violation {
	if len(s.CadenceSegments) == 0 {
		return nil
	}
	var out []Violation

	type resolved struct {
		seg       CadenceSegmentConfig
		from, to  cadenceOrdinal
		openEnded bool
	}
	var segs []resolved
	for _, seg := range s.CadenceSegments {
		if seg.Cadence == "" {
			out = append(out, Violation{File: s.FilePath, Field: "cadence_segments.cadence",
				Message: fmt.Sprintf("segment starting %q: cadence is required", seg.From)})
		}
		from, err := parseCadenceOrdinal(seg.From, s.Frequency)
		if err != nil {
			out = append(out, Violation{File: s.FilePath, Field: "cadence_segments.from", Message: err.Error()})
			continue
		}
		r := resolved{seg: seg, from: from}
		if seg.To == "" {
			r.openEnded = true
		} else {
			to, err := parseCadenceOrdinal(seg.To, s.Frequency)
			if err != nil {
				out = append(out, Violation{File: s.FilePath, Field: "cadence_segments.to", Message: err.Error()})
				continue
			}
			if to.before(from) {
				out = append(out, Violation{File: s.FilePath, Field: "cadence_segments.to",
					Message: fmt.Sprintf("segment %s..%s: \"to\" is before \"from\"", seg.From, seg.To)})
				continue
			}
			r.to = to
		}
		for _, p := range seg.Present {
			if p < 1 || p > from.stepsPerYear {
				out = append(out, Violation{File: s.FilePath, Field: "cadence_segments.present",
					Message: fmt.Sprintf("segment starting %s: present ordinal %d is out of range for frequency %s (1..%d)",
						seg.From, p, s.Frequency, from.stepsPerYear)})
			}
		}
		segs = append(segs, r)
	}
	if len(out) > 0 {
		return out // unparseable segments -- ordering/continuity checks below would be meaningless
	}

	sort.Slice(segs, func(i, j int) bool { return segs[i].from.before(segs[j].from) })

	for i, r := range segs {
		if i == 0 {
			continue
		}
		prev := segs[i-1]
		if prev.openEnded {
			out = append(out, Violation{File: s.FilePath, Field: "cadence_segments",
				Message: fmt.Sprintf("segment starting %s is open-ended but is followed by another segment starting %s",
					prev.seg.From, r.seg.From)})
			continue
		}
		expectedNext := prev.to.next()
		switch {
		case r.from.before(expectedNext):
			out = append(out, Violation{File: s.FilePath, Field: "cadence_segments",
				Message: fmt.Sprintf("segments %s..%s and %s..%s overlap", prev.seg.From, prev.seg.To, r.seg.From, r.seg.To)})
		case expectedNext.before(r.from):
			out = append(out, Violation{File: s.FilePath, Field: "cadence_segments",
				Message: fmt.Sprintf("gap between segment ending %s and segment starting %s (expected %s to start immediately)",
					prev.seg.To, r.seg.From, expectedNext.label(s.Frequency))})
		}
	}

	if last := segs[len(segs)-1]; !last.openEnded {
		out = append(out, Violation{File: s.FilePath, Field: "cadence_segments",
			Message: "the last cadence segment must be open-ended (no \"to\") to cover the series' ongoing history"})
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
	case EventScopeSource:
		if !scopeRefResolves(cfg, EventScopeSource, b.Scope.Ref) {
			return []Violation{{
				File: b.FilePath, Field: "scope.ref",
				Message: fmt.Sprintf("break %q: scope.ref %q does not resolve to any configured source (no sources/%s.yaml with that id)", b.ID, b.Scope.Ref, b.Scope.Ref),
			}}
		}
	case EventScopeDataset:
		if !scopeRefResolves(cfg, EventScopeDataset, b.Scope.Ref) {
			return []Violation{{
				File: b.FilePath, Field: "scope.ref",
				Message: fmt.Sprintf("break %q: scope.ref %q does not resolve to any configured series' dataset", b.ID, b.Scope.Ref),
			}}
		}
	case EventScopeSeries:
		if !scopeRefResolves(cfg, EventScopeSeries, b.Scope.Ref) {
			return []Violation{{
				File: b.FilePath, Field: "scope.ref",
				Message: fmt.Sprintf("break %q: scope.ref %q does not resolve to any configured series", b.ID, b.Scope.Ref),
			}}
		}
	}
	// An empty/unrecognised scope.kind is already reported by
	// validateBreak's own "scope.kind" required-field check — never
	// double-reported here.
	return nil
}

// scopeRefResolves is the resolution rule itself, shared by the break and
// the event registries so the two can never disagree about what "dataset:
// ine-epa" means. The MESSAGES stay with each caller, because a break's
// wording and an event's are read by the same person in different files and
// each should name what it actually is.
func scopeRefResolves(cfg *Config, kind, ref string) bool {
	switch kind {
	case EventScopeSource:
		_, ok := cfg.Sources[ref]
		return ok
	case EventScopeDataset:
		for _, s := range cfg.Series {
			if s.Dataset == ref {
				return true
			}
		}
		return false
	case EventScopeSeries:
		for _, s := range cfg.Series {
			if s.Slug == ref {
				return true
			}
		}
		return false
	}
	return false
}

// validateEventScope enforces the event registry's scope rules, using the
// break registry's own resolution rule (scopeRefResolves) so the two can
// never disagree about what "dataset: ine-epa" means.
//
// "global" is the value every entry authored today carries, and it is a
// legitimate one rather than a default nobody chose: a change of government
// and a worldwide shock are facts about the calendar and apply wherever the
// calendar does. What this function refuses is an entry that is scoped
// INCOHERENTLY — a global scope carrying a ref that names something it
// cannot apply to, a narrow scope with nothing named, a ref resolving to no
// configured series/dataset/source, or a kind outside the four.
func validateEventScope(cfg *Config, ev EventConfig) []Violation {
	var out []Violation

	switch ev.Scope.Kind {
	case EventScopeGlobal:
		if ev.Scope.Ref != "" {
			out = append(out, Violation{
				File: ev.FilePath, Field: "scope.ref",
				Message: fmt.Sprintf("event %q: scope.kind is \"global\", which names nothing, so scope.ref %q cannot be resolved against anything", ev.ID, ev.Scope.Ref),
			})
		}
	case EventScopeSeries, EventScopeDataset, EventScopeSource:
		if ev.Scope.Ref == "" {
			out = append(out, Violation{
				File: ev.FilePath, Field: "scope.ref",
				Message: fmt.Sprintf("event %q: scope.kind %q requires a scope.ref naming what it applies to", ev.ID, ev.Scope.Kind),
			})
			break
		}
		if !scopeRefResolves(cfg, ev.Scope.Kind, ev.Scope.Ref) {
			out = append(out, Violation{
				File: ev.FilePath, Field: "scope.ref",
				Message: fmt.Sprintf("event %q: scope.ref %q does not resolve to any configured %s", ev.ID, ev.Scope.Ref, ev.Scope.Kind),
			})
		}
	default:
		out = append(out, Violation{
			File: ev.FilePath, Field: "scope.kind",
			Message: fmt.Sprintf("event %q: scope.kind %q is not one of global, series, dataset, source", ev.ID, ev.Scope.Kind),
		})
	}

	return out
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

	// The group is an ENUM in every consumer downstream — the export
	// artifact's own Zod schema types it as a closed union, and an entry
	// carrying an unlisted value matches no group there, renders nowhere and
	// reports nothing. That silence is the failure mode this check exists
	// to convert into a named violation at the gate, which is the one place
	// a person is looking.
	if ev.Group != "" && !eventGroups[ev.Group] {
		out = append(out, Violation{
			File: ev.FilePath, Field: "group",
			Message: fmt.Sprintf("event %q: group %q is not one of exogenous, milestones, governments", ev.ID, ev.Group),
		})
	}

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

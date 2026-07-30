package config

// The acknowledgement registry's CONFIG half (spec data-validation,
// "Acknowledged findings"): parsing, the acknowledgeable-rule allowlist,
// and schema validation for config/reconocimientos.yaml.
//
// Why this registry exists at all. validation/types.go has declared
// SeverityBlockRequiresSignoff since PR 4b and rule4_revision.go documents
// its intent at length -- "a deep revision is either legitimate (a
// national-accounts methodology revision) or a parser bug silently
// rewriting history -- those two cases are indistinguishable to the
// machine, so SeverityBlockRequiresSignoff hands the decision to a human
// rather than guessing either way" -- but nothing anywhere resolved it.
// The comment described handing a decision to a human and gave the human
// no way to hand it back, so every blocked series stayed blocked forever.
//
// Why the registry is a separate file and not a field on
// series/{slug}.yaml. An acknowledgement is an EDITORIAL act with
// provenance (a named person, a date, a reason, a source), exactly like a
// rupturas.yaml break -- not a threshold. Keeping it beside the other
// editorial registries puts it under the same four-eyes review discipline
// those files declare in their own headers (PRD §9.6, §15.2), and keeps
// series/{slug}.yaml purely about the series' identity and thresholds.
//
// Why it is NOT a break. config/rupturas.yaml's own header declares it a
// "Registro de rupturas metodológicas". Recording the COVID-19 employment
// collapse there to silence rule 3 would falsify that registry: the fall
// was real economics, not a change of method, and rule 3's break
// exemption exists precisely to stop a methodology change from being
// published as if it were real movement. Widening a break's meaning to
// "anything that trips a threshold" would destroy the one distinction the
// rule is built on.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// AcknowledgementConfig is one config/reconocimientos.yaml entry: a named
// human's record that they reviewed one specific blocking validation
// finding and confirmed the underlying datum is correct.
//
// Every scope field is REQUIRED and matched by exact equality. There is
// deliberately no wildcard, no range, no "all rules" and no series-wide
// form: an acknowledgement is scoped exactly as narrowly as the finding it
// resolves, because a mechanism that can blanket-disable a guard is worse
// than the gap it fills. The three fields together are also the registry's
// uniqueness key (validateDuplicateAcknowledgementScopes).
type AcknowledgementConfig struct {
	ID     string `yaml:"id"`
	Series string `yaml:"series"`
	Period string `yaml:"period"`
	Rule   string `yaml:"rule"`

	// Value is the observed value at Period that the acknowledging human
	// actually looked at, pinned so the acknowledgement cannot silently
	// outlive it. A pointer, not a bare float64: 0 is a legal observed
	// value for most series in this portal, so a zero value could not
	// otherwise be told apart from an omitted field, and an omitted pin
	// would silently degrade the registry into an unconditional override
	// -- the one thing this design must never become.
	//
	// The staleness rule itself (exact equality, fail closed on any
	// difference) lives in validation.GateWithAcknowledgements; see its
	// doc comment for why the pinned VALUE, rather than an expiry date or
	// a hash of the finding's message, is the right thing to pin.
	Value *float64 `yaml:"value"`

	// SignatureStatus is "" (SIGNED — AcknowledgedBy and AcknowledgedOn are
	// both required) or "unsigned" (a DRAFT — DraftedBy and Todo are
	// required, and a signature MUST be absent). validate-config rejects
	// any other value, and rejects every mixture of the two states.
	//
	// This is BreakConfig.DateStatus's discipline applied to the fact that
	// actually matters here. rupturas.yaml never projects a guessed break
	// date because a wrong date silently corrupts every comparison across
	// it; this registry never projects an unsigned record because an
	// acknowledgement's entire authority IS the human signature. A record
	// nobody signed is a proposal, however good its research, and
	// rule4_revision.go's whole point is that the decision belongs to a
	// human rather than to whoever wrote the argument for it. An unsigned
	// record therefore never reaches the database and never resolves any
	// finding (ingestion.ReconcileEditorialConfig; validation.
	// GateWithAcknowledgements refuses one again as defence in depth).
	//
	// A draft is a legitimate thing to check in: the research, the pinned
	// figure and the citation are exactly what a reviewer needs in front of
	// them. What must be impossible is mistaking one for a signature, which
	// is why the two states are mutually exclusive at the schema level
	// rather than merely conventional.
	SignatureStatus string `yaml:"signature_status,omitempty"`

	// DraftedBy records who WROTE the record, which is not who approved it.
	// Required on an unsigned draft and permitted to remain after signing,
	// so "drafted by an agent, signed by a named human" stays legible as
	// the two distinct acts it is.
	DraftedBy string `yaml:"drafted_by,omitempty"`

	// Todo is required on an unsigned draft and names exactly what the
	// reviewer must do, mirroring BreakConfig.Todo naming the document to
	// consult. It MUST be absent once signed: a record that is approved and
	// still says something is pending reads as both at once.
	Todo string `yaml:"todo,omitempty"`

	// AcknowledgedBy/AcknowledgedOn/NoteMD/SourceURL are the provenance
	// every editorial record in this project carries (config/rupturas.yaml,
	// config/eventos.yaml): who, when, why in prose, and the source
	// document where one exists.
	//
	// AcknowledgedOn carries no "unconfirmed" form of its own, unlike
	// BreakConfig.Date. A break's effective date is a fact about a THIRD
	// PARTY's methodological note which we may not yet have read. The date
	// a reviewer signed is a fact about OUR OWN process: it can never be
	// legitimately unknown, because a reviewer who cannot say when they
	// reviewed something did not review it. The uncertainty this registry
	// genuinely has is "has anyone signed at all", and that is what
	// SignatureStatus expresses.
	AcknowledgedBy string     `yaml:"acknowledged_by,omitempty"`
	AcknowledgedOn *time.Time `yaml:"acknowledged_on,omitempty"`
	NoteMD         string     `yaml:"note_md"`
	SourceURL      string     `yaml:"source_url,omitempty"`

	// FilePath is set by the loader, same rationale as SourceConfig.FilePath.
	FilePath string `yaml:"-"`
}

// acknowledgeableRules is the closed set of validation rules whose
// blocking findings a human may acknowledge.
//
// The set is closed, and small, on purpose. An acknowledgement is only an
// honest statement where the finding is about a DATUM BEING SURPRISING --
// where the machine genuinely cannot separate a legitimate cause from a
// broken one and a human can. That is true of exactly two rules:
//
//   - rule3-plausibility: a value or a period-over-period change breached
//     a configured threshold with no covering break. Either the world
//     really moved that much, or the parser is wrong. Only a human knows.
//   - rule4-revision: a period older than the revision window changed.
//     Either a genuine national-accounts revision, or a parser silently
//     rewriting history -- rule4_revision.go's own words.
//
// Every other blocking finding this pipeline can raise is a defect in the
// machinery or in our own configuration, and each already has a correct,
// narrower remedy that this registry must not be allowed to bypass:
//
//   - rule1-schema, source-decode, source-status: the pipeline or its
//     pinned schema is broken. "A human confirmed the data is correct" is
//     not a true statement about a header-fingerprint mismatch, and
//     source-status's own message calls itself "an internal defect, not a
//     source failure". Acknowledging these would paper over a broken
//     parser -- the exact failure mode rule 4's comment exists to prevent.
//   - rule2-continuity: already has a period-scoped editorial remedy,
//     validation.continuity.documented_gaps, and spec data-validation says
//     in terms that "the correct remedy is to correct the series
//     configuration, never to relax the rule". A second way to silence one
//     rule would put the same fact in two registries.
//   - rule5-metadata: our own metadata is incomplete. Confirming the data
//     is correct says nothing about a missing unit or licence; the remedy
//     is to complete series/{slug}.yaml.
//   - rule6-nonempty: the payload carried nothing. There is no datum for a
//     human to have reviewed.
//
// The allowlist lives in this package, not in package validation, only
// because validation imports config and not the other way round. Its
// correctness is not left to that comment: the validation package's
// TestAcknowledgeableRules_AreExactlyRule3AndRule4 runs the REAL rules,
// collects the rule names they actually emit, and asserts this set matches
// them exactly -- so a renamed rule or a new judgement-call rule fails a
// test rather than silently making an acknowledgement unmatchable.
var acknowledgeableRules = []string{"rule3-plausibility", "rule4-revision"}

// AcknowledgeableRules returns the acknowledgeable-rule allowlist as a
// fresh copy, so a caller cannot mutate the registry's policy in place.
func AcknowledgeableRules() []string {
	out := make([]string, len(acknowledgeableRules))
	copy(out, acknowledgeableRules)
	return out
}

// IsAcknowledgeableRule reports whether rule admits an acknowledgement.
// Both validate-config (the config gate) and validation.
// GateWithAcknowledgements (defence in depth at run time) consult it, so a
// row that somehow reached the database naming a non-acknowledgeable rule
// still resolves nothing.
func IsAcknowledgeableRule(rule string) bool {
	for _, r := range acknowledgeableRules {
		if r == rule {
			return true
		}
	}
	return false
}

// validateAcknowledgement enforces spec data-validation's "Acknowledged
// findings": full provenance, a pinned value, and a scope that is exactly
// one series, one period and one rule.
//
// The three scope checks are what make "an acknowledgement can never be
// widened" a schema property rather than a convention. `series: "*"`
// resolves to no configured series; `period: "*"` matches no label on the
// series' own frequency grid; `rule: "*"` is not in the allowlist. There
// is no syntax in this schema that expresses "every period" or "every
// rule", so there is nothing for a reviewer to have to notice.
func validateAcknowledgement(cfg *Config, a AcknowledgementConfig) []Violation {
	var out []Violation
	require := func(field, value string) {
		if value == "" {
			out = append(out, Violation{File: a.FilePath, Field: field,
				Message: fmt.Sprintf("acknowledgement %q: required field is missing", a.ID)})
		}
	}
	require("id", a.ID)
	require("series", a.Series)
	require("period", a.Period)
	require("rule", a.Rule)
	require("note_md", a.NoteMD)

	out = append(out, validateAcknowledgementSignature(a)...)

	if a.Value == nil {
		out = append(out, Violation{File: a.FilePath, Field: "value",
			Message: fmt.Sprintf("acknowledgement %q: value is required — it pins the exact number the reviewer confirmed, so a later revision of that datum invalidates the acknowledgement instead of silently inheriting it", a.ID)})
	}

	if a.Rule != "" && !IsAcknowledgeableRule(a.Rule) {
		out = append(out, Violation{File: a.FilePath, Field: "rule",
			Message: fmt.Sprintf("acknowledgement %q: rule %q is not acknowledgeable (only %v are). A finding from any other rule reports a broken pipeline or an incomplete configuration, not a surprising datum, and its correct remedy is to fix that — never to acknowledge it",
				a.ID, a.Rule, acknowledgeableRules)})
	}

	series, found := findSeriesBySlug(cfg, a.Series)
	if a.Series != "" && !found {
		out = append(out, Violation{File: a.FilePath, Field: "series",
			Message: fmt.Sprintf("acknowledgement %q: series %q does not resolve to any configured series (no series/%s.yaml with that slug)", a.ID, a.Series, a.Series)})
		return out
	}
	if a.Period != "" && found && !isPeriodOnFrequencyGrid(a.Period, series.Frequency) {
		out = append(out, Violation{File: a.FilePath, Field: "period",
			Message: fmt.Sprintf("acknowledgement %q: period %q is not a single period on series %q's declared %q frequency grid (expected %s) — an acknowledgement covers exactly one period, never a whole year, a range or a wildcard",
				a.ID, a.Period, a.Series, series.Frequency, periodShapeFor(series.Frequency))})
	}
	return out
}

// Period label shapes, one per declared frequency. Deliberately
// re-implemented here rather than reusing parseCadenceOrdinal: that helper
// folds every non-monthly frequency into a quarterly-shaped label, which
// is correct for a cadence_segments boundary (segments only describe
// sub-annual grids) and WRONG here -- it would reject `2020`, the only
// real label an annual series such as pib-eurostat has, while accepting
// `2020-Q1` for it. This check exists to make widening impossible, so it
// must be exact about which single label a given frequency admits. The
// shapes match indicators.Period.String() exactly (indicators/period.go);
// this package stays decoupled from the domain layer by convention, the
// same way SeriesConfig.Frequency has been a plain string since Load was
// first written.
var (
	acknowledgementQuarterlyPeriod = regexp.MustCompile(`^\d{4}-Q[1-4]$`)
	acknowledgementMonthlyPeriod   = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)
	acknowledgementAnnualPeriod    = regexp.MustCompile(`^\d{4}$`)
)

func isPeriodOnFrequencyGrid(period, frequency string) bool {
	switch frequency {
	case "Q":
		return acknowledgementQuarterlyPeriod.MatchString(period)
	case "M":
		return acknowledgementMonthlyPeriod.MatchString(period)
	case "A":
		return acknowledgementAnnualPeriod.MatchString(period)
	default:
		// An unrecognised frequency is validateSeries' violation to report,
		// not this one's; failing closed here keeps an acknowledgement from
		// riding on a frequency nothing else accepted either.
		return false
	}
}

func periodShapeFor(frequency string) string {
	switch frequency {
	case "Q":
		return "YYYY-Qn"
	case "M":
		return "YYYY-MM"
	case "A":
		return "YYYY"
	default:
		return "a canonical period label for the series' declared frequency"
	}
}

// placeholderSignatures are tokens that satisfy a "the field is non-empty"
// check while asserting nothing at all. A vacant signature is WORSE than a
// missing one: a missing one is obviously missing, whereas "acknowledged_by:
// TODO" passes a careless review and then authorises a guard override
// forever. validateBreak refuses a confirmed break with no date for exactly
// this reason; this refuses a signed acknowledgement with no signer.
//
// The list is a closed set of tokens, compared case-insensitively after
// trimming. It deliberately does NOT try to decide whether a given string
// names a real person — that is undecidable, and a check that pretended
// otherwise would manufacture confidence rather than provide it. What
// stops a fabricated signature is the four-eyes review this file's own
// header demands, and SignatureStatus making "nobody has signed" an
// expressible, checkable state rather than something a drafter has to
// paper over.
var placeholderSignatures = map[string]bool{
	"todo": true, "tbd": true, "fixme": true, "xxx": true, "name": true,
	"n/a": true, "na": true, "-": true, "--": true, "?": true, "???": true,
	"unknown": true, "pending": true, "someone": true, "anon": true,
	"anonymous": true, "none": true, "nobody": true, "tbc": true,
}

// isPlaceholderSignature reports whether signer is empty, whitespace-only,
// implausibly short, or one of the known vacant tokens.
func isPlaceholderSignature(signer string) bool {
	trimmed := strings.TrimSpace(signer)
	if len(trimmed) < 2 {
		return true
	}
	return placeholderSignatures[strings.ToLower(trimmed)]
}

// validateAcknowledgementSignature enforces the two mutually exclusive
// states a record may be in, and refuses every mixture of them. See
// AcknowledgementConfig.SignatureStatus for why an unsigned record is a
// first-class, expressible state rather than something to be avoided.
func validateAcknowledgementSignature(a AcknowledgementConfig) []Violation {
	var out []Violation
	violation := func(field, message string) {
		out = append(out, Violation{File: a.FilePath, Field: field,
			Message: fmt.Sprintf("acknowledgement %q: %s", a.ID, message)})
	}

	switch a.SignatureStatus {
	case "":
		// SIGNED: a named human has put their name to this record, so it
		// carries authority and the publish gate may act on it.
		if isPlaceholderSignature(a.AcknowledgedBy) {
			violation("acknowledged_by", fmt.Sprintf(
				"acknowledged_by %q is not a signature. Name the human who reviewed the datum, "+
					"or declare signature_status: \"unsigned\" with drafted_by and todo — a record nobody "+
					"signed carries no authority and must say so rather than borrow a placeholder", a.AcknowledgedBy))
		}
		if a.AcknowledgedOn == nil {
			violation("acknowledged_on",
				"acknowledged_on is required on a signed record — a reviewer who cannot say when they reviewed the datum did not review it")
		}
		if a.Todo != "" {
			violation("todo",
				"a signed record must not still carry a todo; it reads as approved and pending at the same time. "+
					"Either the review happened (drop the todo) or it did not (declare signature_status: \"unsigned\")")
		}
	case "unsigned":
		// A DRAFT: research checked in for a human to review. It is inert.
		if a.DraftedBy == "" {
			violation("drafted_by",
				"signature_status is \"unsigned\" but drafted_by does not say who wrote this record")
		}
		if a.Todo == "" {
			violation("todo",
				"signature_status is \"unsigned\" but todo does not state what the reviewer must do to sign it")
		}
		if a.AcknowledgedBy != "" {
			violation("acknowledged_by", fmt.Sprintf(
				"signature_status is \"unsigned\" but acknowledged_by is set to %q. A record cannot be "+
					"awaiting a signature and signed at once — that shape reads as approved at a glance, "+
					"which is exactly the confusion this field exists to prevent", a.AcknowledgedBy))
		}
		if a.AcknowledgedOn != nil {
			violation("acknowledged_on",
				"signature_status is \"unsigned\" but acknowledged_on is set; a record cannot be awaiting a signature and signed at once")
		}
	default:
		violation("signature_status", fmt.Sprintf(
			"signature_status %q is not \"unsigned\" (omit the field entirely when a human has signed the record)", a.SignatureStatus))
	}
	return out
}

func findSeriesBySlug(cfg *Config, slug string) (SeriesConfig, bool) {
	for _, s := range cfg.Series {
		if s.Slug == slug {
			return s, true
		}
	}
	return SeriesConfig{}, false
}

// validateDuplicateAcknowledgementIDs mirrors validateDuplicateBreakIDs:
// two entries sharing an id would collide on the table's primary key and
// make the reconcile's "an edit is distinguishable from a delete plus
// insert" digest meaningless.
func validateDuplicateAcknowledgementIDs(acks []AcknowledgementConfig) []Violation {
	seen := map[string][]string{}
	for _, a := range acks {
		if a.ID == "" {
			continue
		}
		seen[a.ID] = append(seen[a.ID], a.FilePath)
	}
	var out []Violation
	for id, files := range seen {
		if len(files) < 2 {
			continue
		}
		sort.Strings(files)
		out = append(out, Violation{File: files[0], Field: "id",
			Message: fmt.Sprintf("acknowledgement id %q is declared %d times", id, len(files))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Message < out[j].Message })
	return out
}

// validateDuplicateAcknowledgementScopes rejects two acknowledgements
// covering the SAME finding. Which of two conflicting approvals applies is
// not a question this pipeline should ever have to answer at run time, and
// the database enforces the same thing (migration 0006's partial unique
// index) -- this check is what turns that constraint violation into a
// message naming the file and the offending entries.
func validateDuplicateAcknowledgementScopes(acks []AcknowledgementConfig) []Violation {
	type scope struct{ series, period, rule string }
	seen := map[scope][]string{}
	for _, a := range acks {
		if a.Series == "" || a.Period == "" || a.Rule == "" {
			continue
		}
		k := scope{a.Series, a.Period, a.Rule}
		seen[k] = append(seen[k], a.ID)
	}
	var out []Violation
	for k, ids := range seen {
		if len(ids) < 2 {
			continue
		}
		sort.Strings(ids)
		out = append(out, Violation{File: acknowledgementsFile, Field: "series/period/rule",
			Message: fmt.Sprintf("acknowledgements %v all cover series %q period %q rule %q; exactly one acknowledgement may cover one finding",
				ids, k.series, k.period, k.rule)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Message < out[j].Message })
	return out
}

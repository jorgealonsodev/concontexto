package config_test

// RED for the acknowledgement registry's CONFIG gate (spec
// data-validation, "Acknowledged findings"). validate-config is a blocking
// CI gate, so it is the place where "an acknowledgement can never be
// widened beyond one finding" stops being a convention and becomes a
// schema property: there is no syntax in reconocimientos.yaml that
// expresses "every period", "every rule" or "every series", and each of
// the tests below proves one of those doors is actually shut rather than
// merely undocumented.
//
// The last test reads the REAL embedded config/reconocimientos.yaml, the
// same discipline licensing_test.go applies to the real source files: a
// registry that carries human approvals is worth asserting the content of,
// not only the shape.

import (
	"strings"
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
)

func ackFloat(v float64) *float64 { return &v }

// ackFixtureConfig is a minimal Config carrying one quarterly series, one
// monthly series and one annual series, so a period-shape check can be
// exercised against every frequency this portal configures.
func ackFixtureConfig(acks ...config.AcknowledgementConfig) *config.Config {
	return &config.Config{
		Sources: map[string]config.SourceConfig{"ine": {ID: "ine"}},
		Series: []config.SeriesConfig{
			{Slug: "ocupados-epa", Frequency: "Q", Dataset: "ine-epa", Source: "ine"},
			{Slug: "ipc-general", Frequency: "M", Dataset: "ine-ipc", Source: "ine"},
			{Slug: "pib-eurostat", Frequency: "A", Dataset: "eurostat-nama", Source: "ine"},
		},
		Acknowledgements: acks,
	}
}

// validAck is a SIGNED acknowledgement: a named human has put their name to
// it, so it carries authority and the gate may act on it.
func validAck() config.AcknowledgementConfig {
	on := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	return config.AcknowledgementConfig{
		ID: "ocupados-epa-2020-q2-covid", Series: "ocupados-epa", Period: "2020-Q2",
		Rule: "rule3-plausibility", Value: ackFloat(18607.2),
		AcknowledgedBy: "Ada Lovelace", AcknowledgedOn: &on,
		NoteMD:   "COVID-19 lockdown quarter.",
		FilePath: "reconocimientos.yaml",
	}
}

// unsignedAck is the same record as a DRAFT: researched and written up, but
// nobody has signed it yet. It must validate (a draft is a legitimate state
// to check in) while carrying no authority whatsoever.
func unsignedAck() config.AcknowledgementConfig {
	a := validAck()
	a.SignatureStatus = "unsigned"
	a.AcknowledgedBy, a.AcknowledgedOn = "", nil
	a.DraftedBy = "Claude (agent), under delegated authority"
	a.Todo = "Review the cited INE publication and sign this record."
	return a
}

// ackViolations runs the acknowledgement half of Validate and returns only
// the violations that concern reconocimientos.yaml, so an unrelated
// fixture gap in the surrounding series cannot mask or fake a result.
func ackViolations(cfg *config.Config) []config.Violation {
	var out []config.Violation
	for _, v := range config.Validate(cfg) {
		if v.File == "reconocimientos.yaml" {
			out = append(out, v)
		}
	}
	return out
}

func TestValidateAcknowledgement_AFullyProvenancedEntryPasses(t *testing.T) {
	if got := ackViolations(ackFixtureConfig(validAck())); len(got) != 0 {
		t.Fatalf("expected a complete acknowledgement to validate, got %v", got)
	}
}

// TestValidateAcknowledgement_AnUnsignedDraftValidatesButMustDeclareItself:
// a draft is a legitimate thing to check in -- the research, the citation
// and the pinned figure are worth reviewing before anyone signs -- but it
// must be unmistakable AS a draft. It therefore has to name who drafted it
// and state what is pending, exactly as an unconfirmed break in
// rupturas.yaml must carry a `todo` naming the document to consult.
func TestValidateAcknowledgement_AnUnsignedDraftValidatesButMustDeclareItself(t *testing.T) {
	if got := ackViolations(ackFixtureConfig(unsignedAck())); len(got) != 0 {
		t.Fatalf("expected a well-formed unsigned draft to validate, got %v", got)
	}

	for _, tc := range []struct {
		name, field string
		mutate      func(*config.AcknowledgementConfig)
	}{
		{"no drafter", "drafted_by", func(a *config.AcknowledgementConfig) { a.DraftedBy = "" }},
		{"nothing pending stated", "todo", func(a *config.AcknowledgementConfig) { a.Todo = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := unsignedAck()
			tc.mutate(&a)
			if got := ackViolations(ackFixtureConfig(a)); !violatesField(got, tc.field) {
				t.Fatalf("expected a violation naming %q, got %v", tc.field, got)
			}
		})
	}
}

// TestValidateAcknowledgement_AnUnsignedDraftMustNotCarryASignature is the
// check that makes "a reader cannot mistake a draft for a signature" a
// schema property rather than a hope. A record cannot be both awaiting a
// signature and signed; a half-edited entry carrying both is the single
// most dangerous shape this file can hold, because it reads as approved at
// a glance.
func TestValidateAcknowledgement_AnUnsignedDraftMustNotCarryASignature(t *testing.T) {
	on := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)

	withSigner := unsignedAck()
	withSigner.AcknowledgedBy = "Ada Lovelace"
	if !violatesField(ackViolations(ackFixtureConfig(withSigner)), "acknowledged_by") {
		t.Errorf("an unsigned record carrying acknowledged_by must be rejected")
	}

	withDate := unsignedAck()
	withDate.AcknowledgedOn = &on
	if !violatesField(ackViolations(ackFixtureConfig(withDate)), "acknowledged_on") {
		t.Errorf("an unsigned record carrying acknowledged_on must be rejected")
	}

	// The converse: a SIGNED record must not still say something is pending.
	signedButPending := validAck()
	signedButPending.Todo = "Somebody should really check this."
	if !violatesField(ackViolations(ackFixtureConfig(signedButPending)), "todo") {
		t.Errorf("a signed record must not still carry a todo — it reads as approved and pending at once")
	}
}

// TestValidateAcknowledgement_RejectsAnUnknownSignatureStatus mirrors
// validateBreak's treatment of date_status: the only two states are
// "signed" (the field omitted) and "unsigned". Anything else is a typo, and
// a typo in THIS field would silently decide whether a guard can be
// overridden.
func TestValidateAcknowledgement_RejectsAnUnknownSignatureStatus(t *testing.T) {
	for _, status := range []string{"signed", "pending", "yes", "true", "UNSIGNED"} {
		a := unsignedAck()
		a.SignatureStatus = status
		if !violatesField(ackViolations(ackFixtureConfig(a)), "signature_status") {
			t.Errorf("signature_status %q must be rejected (only \"unsigned\", or omitted when signed)", status)
		}
	}
}

// TestValidateAcknowledgement_RejectsAPlaceholderShapedSignature: a
// signature that is present but vacant is worse than an absent one, because
// it satisfies a "field is non-empty" check while asserting nothing. The
// break registry refuses a confirmed entry with no date for the same
// reason; this refuses a signed entry with no signer.
func TestValidateAcknowledgement_RejectsAPlaceholderShapedSignature(t *testing.T) {
	for _, signer := range []string{"", " ", "\t\n", "TODO", "todo", "TBD", "FIXME", "XXX", "name", "N/A", "-", "?", "unknown", "pending", "someone", "anon", "anonymous"} {
		a := validAck()
		a.AcknowledgedBy = signer
		if !violatesField(ackViolations(ackFixtureConfig(a)), "acknowledged_by") {
			t.Errorf("acknowledged_by %q is not a signature and must be rejected", signer)
		}
	}
	// A real name, including one with surrounding whitespace to trim, passes.
	trimmed := validAck()
	trimmed.AcknowledgedBy = "  Ada Lovelace  "
	if got := ackViolations(ackFixtureConfig(trimmed)); len(got) != 0 {
		t.Errorf("a real name must validate, got %v", got)
	}
}

// TestValidateAcknowledgement_RequiresFullProvenance: a SIGNED
// acknowledgement with no named human, no date or no stated reason is an
// anonymous override, which is the thing this registry exists NOT to be.
func TestValidateAcknowledgement_RequiresFullProvenance(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		mutate      func(*config.AcknowledgementConfig)
	}{
		{"no acknowledger", "acknowledged_by", func(a *config.AcknowledgementConfig) { a.AcknowledgedBy = "" }},
		{"no date", "acknowledged_on", func(a *config.AcknowledgementConfig) { a.AcknowledgedOn = nil }},
		{"no reason", "note_md", func(a *config.AcknowledgementConfig) { a.NoteMD = "" }},
		{"no id", "id", func(a *config.AcknowledgementConfig) { a.ID = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := validAck()
			tc.mutate(&a)
			got := ackViolations(ackFixtureConfig(a))
			if !violatesField(got, tc.field) {
				t.Fatalf("expected a violation naming %q, got %v", tc.field, got)
			}
		})
	}
}

// TestValidateAcknowledgement_RequiresThePinnedValue is the staleness
// guard's config half. An acknowledgement with no pinned value would be an
// unconditional, permanent override -- exactly what the design must never
// degrade into -- and because 0 is a legal observed value the field has to
// be a pointer for "omitted" to be distinguishable at all.
func TestValidateAcknowledgement_RequiresThePinnedValue(t *testing.T) {
	a := validAck()
	a.Value = nil
	got := ackViolations(ackFixtureConfig(a))
	if !violatesField(got, "value") {
		t.Fatalf("expected a violation naming the missing pinned value, got %v", got)
	}

	zero := validAck()
	zero.Value = ackFloat(0)
	if v := ackViolations(ackFixtureConfig(zero)); len(v) != 0 {
		t.Fatalf("a pinned value of exactly 0 is legal and must not be read as omitted, got %v", v)
	}
}

// TestValidateAcknowledgement_CannotBeWidenedBeyondOneSeries.
func TestValidateAcknowledgement_CannotBeWidenedBeyondOneSeries(t *testing.T) {
	for _, series := range []string{"*", "", "all", "ine-epa"} {
		a := validAck()
		a.Series = series
		if !violatesField(ackViolations(ackFixtureConfig(a)), "series") {
			t.Errorf("series %q must not resolve — an acknowledgement covers exactly one configured series", series)
		}
	}
}

// TestValidateAcknowledgement_CannotBeWidenedBeyondOnePeriod is the single
// most important schema check here: it is what stops "ignore everything in
// 2020". A bare year, a wildcard and a range are all rejected for a
// quarterly series, and the annual series proves the check is grid-aware
// rather than a blanket "must contain a Q".
func TestValidateAcknowledgement_CannotBeWidenedBeyondOnePeriod(t *testing.T) {
	for _, period := range []string{"*", "2020", "2020-Q*", "2020-Q1..2020-Q4", "2020-Q5", "", "2020-06"} {
		a := validAck()
		a.Period = period
		if !violatesField(ackViolations(ackFixtureConfig(a)), "period") {
			t.Errorf("period %q must not validate for a quarterly series — an acknowledgement covers exactly one period", period)
		}
	}

	annual := validAck()
	annual.Series, annual.Period = "pib-eurostat", "2020"
	if v := ackViolations(ackFixtureConfig(annual)); len(v) != 0 {
		t.Errorf("`2020` is the only real period label an annual series has and must validate for one, got %v", v)
	}
	annualWrong := validAck()
	annualWrong.Series, annualWrong.Period = "pib-eurostat", "2020-Q2"
	if !violatesField(ackViolations(ackFixtureConfig(annualWrong)), "period") {
		t.Errorf("a quarterly label must not validate against an annual series' grid")
	}

	monthly := validAck()
	monthly.Series, monthly.Period = "ipc-general", "2020-06"
	if v := ackViolations(ackFixtureConfig(monthly)); len(v) != 0 {
		t.Errorf("`2020-06` must validate for a monthly series, got %v", v)
	}
}

// TestValidateAcknowledgement_CannotBeWidenedBeyondOneAcknowledgeableRule:
// no wildcard, and no rule outside the closed allowlist. The excluded
// rules are excluded on purpose: each reports a broken pipeline or an
// incomplete configuration, whose correct remedy is to fix it.
func TestValidateAcknowledgement_CannotBeWidenedBeyondOneAcknowledgeableRule(t *testing.T) {
	for _, rule := range []string{"*", "", "all", "rule1-schema", "rule2-continuity", "rule5-metadata", "rule6-nonempty", "source-decode", "source-status"} {
		a := validAck()
		a.Rule = rule
		if !violatesField(ackViolations(ackFixtureConfig(a)), "rule") {
			t.Errorf("rule %q must not be acknowledgeable", rule)
		}
	}
	for _, rule := range config.AcknowledgeableRules() {
		a := validAck()
		a.Rule = rule
		if v := ackViolations(ackFixtureConfig(a)); len(v) != 0 {
			t.Errorf("rule %q is on the allowlist and must validate, got %v", rule, v)
		}
	}
}

// TestValidateAcknowledgement_RejectsTwoRecordsCoveringTheSameFinding:
// which of two conflicting human approvals applies is not a question the
// pipeline should answer at run time.
func TestValidateAcknowledgement_RejectsTwoRecordsCoveringTheSameFinding(t *testing.T) {
	first := validAck()
	second := validAck()
	second.ID = "ocupados-epa-2020-q2-covid-again"
	got := ackViolations(ackFixtureConfig(first, second))
	if len(got) == 0 {
		t.Fatalf("expected two acknowledgements of the same (series, period, rule) to be rejected")
	}

	duplicateID := validAck()
	duplicateID.Period = "2020-Q3"
	if v := ackViolations(ackFixtureConfig(validAck(), duplicateID)); !violatesField(v, "id") {
		t.Fatalf("expected a duplicate acknowledgement id to be rejected, got %v", v)
	}
}

// TestRealAcknowledgementRegistry_ShipsExactlyOneUNSIGNEDDraft reads the
// REAL embedded registry (licensing_test.go's discipline).
//
// This test asserts the OPPOSITE of what a reader might expect, and that is
// the point. The portal ships exactly one acknowledgement record today, and
// it MUST be unsigned. The research behind it -- the measured delta
// distribution, the pinned figure, INE's own press release -- was done by
// an agent under delegated authority. No human has reviewed and signed it.
// Writing a person's name into that record would fabricate the one fact the
// whole mechanism rests on, and would defeat rule4_revision.go's stated
// purpose of handing the decision to a HUMAN rather than guessing.
//
// So this test is the guard against exactly that regression: if anyone --
// agent or human -- ever adds a signature to a shipped record without a
// real review, or ships a record whose drafted/signed status is ambiguous,
// this fails. `ocupados-epa` consequently stays blocked and absent from the
// artifact. That is the correct state: a human decision genuinely IS
// pending, and the pipeline says so instead of pretending otherwise.
func TestRealAcknowledgementRegistry_ShipsExactlyOneUNSIGNEDDraft(t *testing.T) {
	cfg := realConfig(t)

	if len(cfg.Acknowledgements) != 1 {
		t.Fatalf("expected exactly one shipped acknowledgement record, got %d: %+v", len(cfg.Acknowledgements), cfg.Acknowledgements)
	}
	a := cfg.Acknowledgements[0]

	if a.SignatureStatus != "unsigned" {
		t.Errorf("the shipped record MUST be unsigned — no human has reviewed it; got signature_status=%q", a.SignatureStatus)
	}
	if a.AcknowledgedBy != "" || a.AcknowledgedOn != nil {
		t.Errorf("the shipped record must carry NO signature: by=%q on=%v", a.AcknowledgedBy, a.AcknowledgedOn)
	}
	if a.DraftedBy == "" {
		t.Error("an unsigned draft must name who drafted it")
	}
	if a.Todo == "" {
		t.Error("an unsigned draft must state what is pending")
	}

	// The research it carries must still be the real, verified research.
	if a.Series != "ocupados-epa" || a.Period != "2020-Q2" || a.Rule != "rule3-plausibility" {
		t.Errorf("expected the record scoped to ocupados-epa/2020-Q2/rule3-plausibility, got %s/%s/%s", a.Series, a.Period, a.Rule)
	}
	if a.Value == nil || *a.Value != 18607.2 {
		t.Errorf("expected the pinned value to be INE's published 2020-Q2 figure 18607.2, got %v", a.Value)
	}
	if !strings.Contains(strings.ToUpper(a.NoteMD), "COVID") {
		t.Errorf("expected the note to name the COVID-19 lockdown as the real cause, got %q", a.NoteMD)
	}
	if !strings.Contains(a.SourceURL, "ine.es") {
		t.Errorf("expected the record to cite INE's own publication, got %q", a.SourceURL)
	}
	// The note must not assert a review that did not happen.
	for _, forbidden := range []string{"Revisado y confirmado", "revisado y confirmado"} {
		if strings.Contains(a.NoteMD, forbidden) {
			t.Errorf("the note claims a human review that has not happened: %q", a.NoteMD)
		}
	}

	if v := config.Validate(cfg); len(v) != 0 {
		t.Errorf("the real configuration tree must validate, got %v", v)
	}
}

func violatesField(violations []config.Violation, field string) bool {
	for _, v := range violations {
		if v.Field == field {
			return true
		}
	}
	return false
}

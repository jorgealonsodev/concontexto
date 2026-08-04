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

// TestValidateAcknowledgement_RejectsAnAgentAsTheSigner: an agent is not a
// person, so it can never be the human rule4_revision.go hands the decision
// to. Before this check the ONLY thing standing between an agent and a
// signature was the agent choosing not to write one, which is not a
// protection at all -- and the exact-token placeholder list could not have
// helped, because an agent signing would not write a bare "Claude". It would
// write a sentence. The first case below is the literal `drafted_by` string
// this repository's own registry carried while the record was a draft.
func TestValidateAcknowledgement_RejectsAnAgentAsTheSigner(t *testing.T) {
	for _, signer := range []string{
		"Claude (agente), bajo autoridad delegada — no es una firma",
		"Claude (agent), under delegated authority",
		"Claude", "claude", "Anthropic", "GPT-5", "ChatGPT", "Copilot",
		"Gemini", "an autonomous agent", "el agente de turno",
		"ingest-bot", "AI", "IA", "LLM", "assistant", "asistente editorial",
	} {
		a := validAck()
		a.AcknowledgedBy = signer
		if !violatesField(ackViolations(ackFixtureConfig(a)), "acknowledged_by") {
			t.Errorf("acknowledged_by %q names an agent, not a person, and must be rejected", signer)
		}
	}

	// The converse, and the reason the agent words are matched as WHOLE
	// words: a real person whose name merely contains those letters must
	// still be able to sign. A check that cried wolf here would be worked
	// around, and a worked-around check protects nothing.
	for _, signer := range []string{"Ada Lovelace", "Alberto Botella", "Agnès Bota", "Ai Weiwei", "Iago Bottino"} {
		a := validAck()
		a.AcknowledgedBy = signer
		if got := ackViolations(ackFixtureConfig(a)); len(got) != 0 {
			t.Errorf("%q is a person's name and must validate, got %v", signer, got)
		}
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

// TestRealAcknowledgementRegistry_ShipsExactlyOneRecordSignedByARealHuman
// reads the REAL embedded registry (licensing_test.go's discipline).
//
// WHY THIS TEST INVERTED, AND WHAT IT USED TO ASSERT. It was
// TestRealAcknowledgementRegistry_ShipsExactlyOneUNSIGNEDDraft, and it
// required the shipped record to carry `signature_status: unsigned`, no
// `acknowledged_by`, no `acknowledged_on`, a `drafted_by` and a `todo`. That
// was correct for as long as it was true: the research behind the record --
// the measured delta distribution, the pinned figure, INE's own press
// release -- was written by an agent under delegated authority, and no human
// had reviewed it. The guard existed because of a REAL failure that had
// already happened once: an earlier version of this record shipped SIGNED,
// with this same person's name, for a review he had never performed, and
// only a human reading a diff caught it.
//
// The record has since been reviewed and signed by the repository's
// maintainer, so "unsigned" is no longer the correct state. The failure mode
// the guard was built for is unchanged, though: a signature that is PRESENT
// BUT NOT REAL. So the test now asserts the signed shape and, more
// importantly, keeps interrogating the signature itself -- it must be a
// genuine human handle, never an agent, never a placeholder, never blank --
// and asserts that the two draft-only fields are gone, so a half-edited
// record that reads as approved at a glance cannot ship either.
//
// What is deliberately NOT asserted is the identity of the signer. Pinning a
// specific name would make the test a test of who happens to maintain the
// repository rather than of whether the mechanism was honoured.
func TestRealAcknowledgementRegistry_ShipsExactlyOneRecordSignedByARealHuman(t *testing.T) {
	cfg := realConfig(t)

	if len(cfg.Acknowledgements) != 1 {
		t.Fatalf("expected exactly one shipped acknowledgement record, got %d: %+v", len(cfg.Acknowledgements), cfg.Acknowledgements)
	}
	a := cfg.Acknowledgements[0]

	// THE SIGNED SHAPE. signature_status and drafted_by must both be gone:
	// the first because the record is no longer awaiting anyone, the second
	// because leaving "drafted by an agent" beside a human signature invites
	// exactly the confusion about who decided that this registry exists to
	// prevent. A todo would say the record is still pending; it is not.
	if a.SignatureStatus != "" {
		t.Errorf("the shipped record is signed, so signature_status must be absent entirely; got %q", a.SignatureStatus)
	}
	if a.DraftedBy != "" {
		t.Errorf("drafted_by must be gone from a signed record — the signature, not the drafting, is what carries authority; got %q", a.DraftedBy)
	}
	if a.Todo != "" {
		t.Errorf("a signed record must not still state something pending; got %q", a.Todo)
	}
	if a.AcknowledgedOn == nil {
		t.Error("a signed record must record WHEN the review happened — a reviewer who cannot say when they reviewed the datum did not review it")
	}

	// THE HALF THIS GUARD HAS ALWAYS BEEN FOR: is the signature real? An
	// empty string, a placeholder, or an agent's name in acknowledged_by is
	// the regression that already happened once, and it must fail here rather
	// than in a diff somebody happens to read carefully.
	signer := strings.TrimSpace(a.AcknowledgedBy)
	if signer == "" {
		t.Fatal("acknowledged_by is empty: a record with no signer carries no authority and must never ship signed")
	}
	for _, forbidden := range []string{
		// Agents. An agent cannot review a datum on a human's behalf; the
		// whole mechanism (rule4_revision.go) exists to hand the decision to
		// a person, and an agent-signed record hands it back to whoever
		// wrote the argument for it.
		"claude", "anthropic", "gpt", "chatgpt", "openai", "copilot",
		"gemini", "agent", "agente", "bot", "llm", "assistant", "asistente",
		// Placeholders. A vacant signature passes a careless review and then
		// authorises a guard override forever.
		"todo", "tbd", "fixme", "xxx", "n/a", "unknown", "pending",
		"someone", "anon", "nobody", "unsigned", "sin firmar",
	} {
		if strings.Contains(strings.ToLower(signer), forbidden) {
			t.Errorf("acknowledged_by %q contains %q: that is not a human signature, and a signature that is present but not real is the exact regression this test guards", signer, forbidden)
		}
	}
	// The same judgement the validator makes, applied to the shipped bytes:
	// whatever the list above misses, the config gate must still refuse.
	if v := ackViolations(cfg); len(v) != 0 {
		t.Errorf("the shipped registry must satisfy the acknowledgement validator, got %v", v)
	}

	// The research it carries must still be the real, verified research: the
	// signature changed, the evidence behind it did not.
	if a.Series != "ocupados-epa" || a.Period != "2020-Q2" || a.Rule != "rule3-plausibility" {
		t.Errorf("expected the record scoped to ocupados-epa/2020-Q2/rule3-plausibility, got %s/%s/%s", a.Series, a.Period, a.Rule)
	}
	if a.Value == nil || *a.Value != 18607.2 {
		t.Errorf("expected the pinned value to be INE's published 2020-Q2 figure 18607.2, got %v", a.Value)
	}
	if !strings.Contains(strings.ToUpper(a.NoteMD), "COVID") {
		t.Errorf("expected the note to name the COVID-19 lockdown as the real cause, got %q", a.NoteMD)
	}
	if !strings.Contains(a.NoteMD, "1074,1") || !strings.Contains(a.NoteMD, "18607,2") {
		t.Errorf("expected the note to keep the measured fall and the pinned figure it was signed against, got %q", a.NoteMD)
	}
	if !strings.Contains(a.SourceURL, "ine.es") {
		t.Errorf("expected the record to cite INE's own publication, got %q", a.SourceURL)
	}
	// The converse of the old assertion: a signed note must no longer say the
	// record is waiting for anybody.
	for _, forbidden := range []string{"PENDIENTE DE REVISIÓN", "pendiente de revisión", "no surte ningún efecto"} {
		if strings.Contains(a.NoteMD, forbidden) {
			t.Errorf("the note still declares the record pending while the record is signed: %q", a.NoteMD)
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

package ingestion

// Task 7.4/7.5, 7.8/7.9: ReconcileEditorialConfig is design.md's third
// named ingestion/ function (package doc comment on ingest.go), built in
// this batch. It turns the parsed config.Config's Breaks/Events into
// digested inputs and delegates the actual transactional full reconcile
// to postgres.ReconcileBreaks/ReconcileEvents (see that file's package
// doc comment for the insert/update-in-place/soft-retire discipline).
//
// A break or event whose DateStatus is "unconfirmed" (Date/DateStart is
// nil) is NEVER projected into series_break/event: this function records
// its id in PendingBreakIDs/PendingEventIDs instead of guessing a date.
// A wrong break date silently corrupts every comparison across it — the
// portal's own worst failure mode — so an entry whose effective date is
// not yet confirmed against its source's methodological note stays
// documented in rupturas.yaml/eventos.yaml, visible as "pending", and
// reconciled once its date is confirmed and the YAML is edited
// accordingly (which itself goes through the normal in-place-update path,
// task 7.4/7.5, once Date stops being nil).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres"
)

// ReconcileResult is one ReconcileEditorialConfig call's combined effect.
type ReconcileResult struct {
	Breaks postgres.ReconcileCounts
	Events postgres.ReconcileCounts

	// Acknowledgements is config/reconocimientos.yaml's effect on
	// validation_acknowledgement (spec data-validation, "Acknowledged
	// findings").
	Acknowledgements postgres.ReconcileCounts

	// PendingBreakIDs/PendingEventIDs list every entry this run declined
	// to project because its date is not yet confirmed (see the package
	// doc comment above) — never nil-but-silently-dropped.
	PendingBreakIDs []string
	PendingEventIDs []string

	// PendingAcknowledgementIDs lists every acknowledgement this run
	// declined to project because NO HUMAN HAS SIGNED IT YET. Reported for
	// the same reason the two lists above are: a record waiting on a person
	// must be visible as waiting, not silently absent. An operator seeing a
	// series blocked can then tell "nobody has looked at this yet" apart
	// from "there is no record at all".
	PendingAcknowledgementIDs []string
}

// ReconcileEditorialConfig reconciles cfg.Breaks/cfg.Events into
// series_break/event inside ONE transaction (spec editorial-config,
// "Editorial YAML is authoritative and reconciled transactionally" — "A
// failed reconcile leaves no partial state" is written against ONE
// reconcile call, not "the breaks half of one reconcile". Earlier PR 7a
// ran postgres.ReconcileBreaks/ReconcileEvents as two independent
// top-level transactions, disclosed as a gap: a failure in the events
// half could leave an already-committed breaks change durable. This now
// delegates to postgres.ReconcileEditorial, which shares one tx across
// both tables — see TestReconcileEditorialConfig_MidTransactionFailureAcrossBreaksAndEventsLeavesDatabaseByteIdentical).
func ReconcileEditorialConfig(ctx context.Context, db postgres.TxBeginner, cfg config.Config) (ReconcileResult, error) {
	var result ReconcileResult

	var breakInputs []postgres.SeriesBreakInput
	for _, b := range cfg.Breaks {
		if b.Date == nil {
			result.PendingBreakIDs = append(result.PendingBreakIDs, b.ID)
			continue
		}
		breakInputs = append(breakInputs, postgres.SeriesBreakInput{
			BreakKey: b.ID, ScopeKind: b.Scope.Kind, ScopeRef: b.Scope.Ref,
			Date: *b.Date, Kind: b.Kind, NoteMD: b.NoteMD, SourceURL: b.SourceURL,
			ConfigDigest: breakDigest(b),
		})
	}

	var eventInputs []postgres.EventInput
	for _, e := range cfg.Events {
		if e.DateStart == nil {
			result.PendingEventIDs = append(result.PendingEventIDs, e.ID)
			continue
		}
		eventInputs = append(eventInputs, postgres.EventInput{
			ID: e.ID, Group: e.Group, Name: e.Name,
			DateStart: *e.DateStart, DateEnd: e.DateEnd, NoteMD: e.NoteMD,
			ScopeKind: eventScopeKind(e), ScopeRef: e.Scope.Ref, SourceURL: e.SourceURL,
			ConfigDigest: eventDigest(e),
		})
	}

	// An UNSIGNED acknowledgement is never projected, for exactly the reason
	// an unconfirmed break date is never projected (this file's own package
	// doc comment). There, the refusal is because a guessed date silently
	// corrupts every comparison across it. Here, it is because an
	// acknowledgement's entire authority is the human signature: a record
	// nobody has signed is a PROPOSAL, however good its research and however
	// well cited, and letting it resolve a finding would hand the override
	// to whoever wrote the argument rather than to the human
	// rule4_revision.go says must decide. The draft stays documented in
	// reconocimientos.yaml, visible here as pending, and reconciles the
	// moment a person signs it and the YAML is edited accordingly — the same
	// lifecycle an unconfirmed break date already follows.
	//
	// A nil Value or a nil AcknowledgedOn on a supposedly signed record
	// cannot occur past validate-config; skipping rather than dereferencing
	// keeps a bypass of that gate inert instead of turning it into a panic
	// or, worse, an unconditional override.
	var ackInputs []postgres.AcknowledgementInput
	for _, a := range cfg.Acknowledgements {
		if a.SignatureStatus == "unsigned" || a.AcknowledgedBy == "" {
			result.PendingAcknowledgementIDs = append(result.PendingAcknowledgementIDs, a.ID)
			continue
		}
		if a.Value == nil || a.AcknowledgedOn == nil {
			result.PendingAcknowledgementIDs = append(result.PendingAcknowledgementIDs, a.ID)
			continue
		}
		ackInputs = append(ackInputs, postgres.AcknowledgementInput{
			AckKey: a.ID, SeriesID: a.Series, Period: a.Period, Rule: a.Rule,
			Value: *a.Value, AcknowledgedBy: a.AcknowledgedBy, AcknowledgedOn: *a.AcknowledgedOn,
			NoteMD: a.NoteMD, SourceURL: a.SourceURL,
			ConfigDigest: acknowledgementDigest(a),
		})
	}

	counts, err := postgres.ReconcileEditorial(ctx, db, breakInputs, eventInputs, ackInputs)
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("ingestion: reconciling editorial config: %w", err)
	}
	result.Breaks = counts.Breaks
	result.Events = counts.Events
	result.Acknowledgements = counts.Acknowledgements

	return result, nil
}

// breakDigest/eventDigest compute the per-entry SHA-256 config_digest
// (spec "Every reconciled database row MUST record a config_digest
// (SHA-256 of its source YAML) so an edit is distinguishable from a
// delete plus insert") over exactly the fields that reach the database
// row. Scoped per entry, not per file: editing one break's description
// must change ONLY that break's digest (task 7.4), not every other
// entry's in the same rupturas.yaml file.
func breakDigest(b config.BreakConfig) string {
	h := sha256.New()
	fmt.Fprintf(h, "id=%s\nkind=%s\nscope.kind=%s\nscope.ref=%s\ndate=%s\nnote_md=%s\nsource_url=%s\n",
		b.ID, b.Kind, b.Scope.Kind, b.Scope.Ref, dateDigestString(b.Date), b.NoteMD, b.SourceURL)
	return hex.EncodeToString(h.Sum(nil))
}

// eventDigest covers the SCOPE and the CITATION as well as the dates and
// the prose, and both inclusions are load-bearing rather than completeness
// for its own sake. The scope decides which charts an entry appears on and
// the citation is what makes it checkable by a reader; a field that reaches
// the row without reaching the digest can drift silently, with the YAML
// claiming one thing, the database holding another, and a repeat reconcile
// reporting zero changes. That is the same reasoning acknowledgementDigest
// already applies to its pinned value.
func eventDigest(e config.EventConfig) string {
	h := sha256.New()
	fmt.Fprintf(h, "id=%s\ngroup=%s\nname=%s\ndate_start=%s\ndate_end=%s\nnote_md=%s\nscope.kind=%s\nscope.ref=%s\nsource_url=%s\n",
		e.ID, e.Group, e.Name, dateDigestString(e.DateStart), dateDigestString(e.DateEnd), e.NoteMD,
		eventScopeKind(e), e.Scope.Ref, e.SourceURL)
	return hex.EncodeToString(h.Sum(nil))
}

// eventScopeKind reads an entry's scope kind, treating an unset one as
// "global".
//
// config's own loader already normalises this while parsing the YAML, so in
// production the branch below never fires. It exists for a Config assembled
// in Go — a test, or any future caller that does not come through Load — and
// it resolves the SAME way the loader does rather than a second way. Writing
// the empty string through instead would produce a row matching no scope
// predicate at all: an entry that reconciles cleanly, reports success and
// renders on no chart, which is the quietest failure available here.
//
// validate-config, not this function, is what refuses an INCOHERENT scope —
// a narrow kind with no ref, a ref resolving to nothing, a kind outside the
// four. That belongs at the gate, where the message can name the file and
// the field.
func eventScopeKind(e config.EventConfig) string {
	if e.Scope.Kind == "" {
		return config.EventScopeGlobal
	}
	return e.Scope.Kind
}

// acknowledgementDigest is breakDigest/eventDigest's counterpart for the
// acknowledgement registry, over exactly the fields that reach the
// database row -- INCLUDING the pinned value. That inclusion is
// load-bearing: correcting a mis-typed pin must register as an EDIT of
// that acknowledgement (a new digest, an in-place update, a visible
// reconcile count), never as a silent no-op that leaves the database
// holding a number no reviewer approved.
func acknowledgementDigest(a config.AcknowledgementConfig) string {
	h := sha256.New()
	fmt.Fprintf(h, "id=%s\nseries=%s\nperiod=%s\nrule=%s\nvalue=%v\nacknowledged_by=%s\nacknowledged_on=%s\nnote_md=%s\nsource_url=%s\n",
		a.ID, a.Series, a.Period, a.Rule, valueDigestString(a.Value), a.AcknowledgedBy,
		dateDigestString(a.AcknowledgedOn), a.NoteMD, a.SourceURL)
	return hex.EncodeToString(h.Sum(nil))
}

// valueDigestString formats the pinned value for the digest with full
// float64 precision ('g' with -1), so two pins that differ in any digit
// that survives the round trip produce different digests.
func valueDigestString(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'g', -1, 64)
}

func dateDigestString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

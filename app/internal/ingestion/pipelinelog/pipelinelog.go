// Package pipelinelog builds the structured log record for one completed
// ingestion run (spec pipeline-operations, "Structured pipeline logging":
// "Every ingestion run MUST emit structured logs carrying at least run
// id, source, dataset, series, outcome, validation verdicts, raw-file
// hash and duration"; "a failed run additionally records which rules
// failed"). Entry/Attrs is a pure builder -- no I/O, no clock access, the
// same "pure builder, effect stays at the call site" split
// validation.Gate/postgres.ApplyGate already established (task 4.16's own
// package-boundary rationale) -- so the exact field set a completed run
// logs is testable without a real slog.Handler, a database, or a network
// call. app/internal/ingestion.IngestSeries is the one caller that turns
// an Entry into an actual slog record, via slog.Default() (see that
// package's own doc comment for why a package-level default logger, not
// an added IngestSeries parameter, is this batch's chosen wiring).
package pipelinelog

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation"
)

// Entry is everything one completed (or failed) ingestion run's log line
// carries.
type Entry struct {
	RunID       int64
	Source      string
	Dataset     string
	Series      string
	Outcome     string
	RawFileHash string
	Duration    time.Duration

	// Verdicts is every finding produced by this run's validation rules,
	// formatted "rule: severity: message", regardless of whether the run
	// ultimately published or blocked (spec "A run is reconstructible
	// from logs alone" -- an operator reading only the log line, with no
	// database access, must see the full verdict set that decided the
	// outcome).
	Verdicts []string

	// FailedRules is the distinct set of rule names that produced a
	// blocking finding the run did NOT resolve -- empty on a Publish
	// outcome, by construction (spec "a failed run additionally records
	// which rules failed"). A finding a human acknowledged is not a failed
	// rule of a run that published; it appears in Acknowledgements below
	// and, verbatim, in Verdicts.
	FailedRules []string

	// Acknowledgements names every finding a human acknowledgement
	// resolved, with the record and the person that resolved it (spec
	// data-validation, "An acknowledged publish is distinguishable from a
	// clean one").
	//
	// This attribute, together with an `outcome` of "publish-overridden"
	// rather than "publish", is what makes an override impossible to
	// mistake for a pass in the structured log: a clean run has neither,
	// and no run can have one without the other. An operator grepping
	// their pipeline logs for `acknowledgements` gets exactly the set of
	// runs that proceeded because a person said so.
	Acknowledgements []string
}

// Attrs converts e into a fixed-order []slog.Attr for
// slog.Logger.LogAttrs, so a completed run's structured fields are
// testable (and greppable in real log output) without depending on
// slog's own key ordering guarantees.
func Attrs(e Entry) []slog.Attr {
	attrs := []slog.Attr{
		slog.Int64("run_id", e.RunID),
		slog.String("source", e.Source),
		slog.String("dataset", e.Dataset),
		slog.String("series", e.Series),
		slog.String("outcome", e.Outcome),
		slog.String("raw_file_hash", e.RawFileHash),
		slog.Duration("duration", e.Duration),
	}
	if len(e.Verdicts) > 0 {
		attrs = append(attrs, slog.Any("verdicts", e.Verdicts))
	}
	if len(e.FailedRules) > 0 {
		attrs = append(attrs, slog.Any("failed_rules", e.FailedRules))
	}
	if len(e.Acknowledgements) > 0 {
		attrs = append(attrs, slog.Any("acknowledgements", e.Acknowledgements))
	}
	return attrs
}

// Acknowledgements formats every override as
// "rule at period: overridden by acknowledgement <id>, acknowledged by
// <person>", in the order the gate recorded them.
//
// The wording is deliberately the word "overridden", not "passed",
// "waived" or "approved": an operator reading this line must be left in no
// doubt that a guard fired and a named human decided to proceed anyway.
func Acknowledgements(overrides []validation.Override) []string {
	if len(overrides) == 0 {
		return nil
	}
	out := make([]string, 0, len(overrides))
	for _, o := range overrides {
		out = append(out, fmt.Sprintf("%s at %s: overridden by acknowledgement %q, acknowledged by %s",
			o.Finding.Rule, o.Finding.Period, o.AcknowledgementID, o.AcknowledgedBy))
	}
	return out
}

// Verdicts formats every finding as "rule: severity: message", in the
// same fixed order Gate received them (spec "The gate reports every
// failure, not only the first" -- the log line preserves that same
// completeness).
func Verdicts(findings []validation.Finding) []string {
	if len(findings) == 0 {
		return nil
	}
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, string(f.Severity)+": "+f.Rule+": "+f.Message)
	}
	return out
}

// FailedRules returns the distinct rule names among findings whose
// severity blocks the publish gate (SeverityBlock or
// SeverityBlockRequiresSignoff -- the same two severities
// validation.Finding.blocks() treats as blocking; that method is
// unexported, so this mirrors it rather than reaching into package
// validation's internals), in first-seen order.
func FailedRules(findings []validation.Finding) []string {
	var out []string
	seen := map[string]bool{}
	for _, f := range findings {
		if f.Severity != validation.SeverityBlock && f.Severity != validation.SeverityBlockRequiresSignoff {
			continue
		}
		if seen[f.Rule] {
			continue
		}
		seen[f.Rule] = true
		out = append(out, f.Rule)
	}
	return out
}

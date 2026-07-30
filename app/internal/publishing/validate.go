package publishing

// ValidateArtifact is publishing-export's write-side gate (spec "The
// artifact is validated on write": "The export MUST validate the
// artifact against its declared schema before writing it. A validation
// failure MUST abort the export, MUST NOT write a partial artifact, and
// MUST leave the previously exported artifact intact"). Export calls
// this over the fully-built in-memory Artifact BEFORE writing a single
// byte (see export.go) -- ValidateArtifact itself never touches a
// filesystem or a database; it is a pure function of its one argument,
// the same purity convention app/internal/ingestion/validation's rules
// already establish.

import (
	"fmt"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

// ValidationError names the exact constraint an artifact violated (spec
// "the export fails with an error naming the violated constraint").
// Series/Period are empty for a manifest-level violation.
type ValidationError struct {
	Constraint string
	Series     string
	Period     string
	Detail     string
}

func (e *ValidationError) Error() string {
	switch {
	case e.Series == "":
		return fmt.Sprintf("publishing: invalid artifact: %s: %s", e.Constraint, e.Detail)
	case e.Period == "":
		return fmt.Sprintf("publishing: invalid artifact: series %s: %s: %s", e.Series, e.Constraint, e.Detail)
	default:
		return fmt.Sprintf("publishing: invalid artifact: series %s period %s: %s: %s", e.Series, e.Period, e.Constraint, e.Detail)
	}
}

// ValidateArtifact checks a fully-built Artifact against its declared
// schema, returning the FIRST violation found (manifest checks first,
// then each series in order). It never returns an aggregate: one
// violation is enough to abort the whole export (spec "A validation
// failure MUST abort the export"), and naming the first one keeps the
// error message unambiguous rather than a wall of unrelated findings.
func ValidateArtifact(a Artifact) error {
	if a.Manifest.SchemaVersion != SchemaVersion {
		return &ValidationError{Constraint: "schema_version", Detail: fmt.Sprintf("manifest declares %d, the writer's own version is %d", a.Manifest.SchemaVersion, SchemaVersion)}
	}
	if a.Manifest.GeneratedAt.IsZero() {
		return &ValidationError{Constraint: "generated_at", Detail: "manifest.generated_at is unset"}
	}

	for _, doc := range a.Series {
		if err := validateSeriesDoc(doc); err != nil {
			return err
		}
	}
	return nil
}

func validateSeriesDoc(doc SeriesDoc) error {
	if doc.Slug == "" {
		return &ValidationError{Constraint: "slug", Detail: "series doc has no slug"}
	}
	if doc.SchemaVersion != SchemaVersion {
		return &ValidationError{Series: doc.Slug, Constraint: "schema_version", Detail: fmt.Sprintf("declares %d, the writer's own version is %d", doc.SchemaVersion, SchemaVersion)}
	}
	if doc.Name == "" {
		return &ValidationError{Series: doc.Slug, Constraint: "name", Detail: "empty"}
	}
	if doc.Unit == "" {
		return &ValidationError{Series: doc.Slug, Constraint: "unit", Detail: "empty"}
	}
	freq := indicators.Frequency(doc.Frequency)
	if freq != indicators.FrequencyQuarterly && freq != indicators.FrequencyMonthly && freq != indicators.FrequencyAnnual {
		return &ValidationError{Series: doc.Slug, Constraint: "frequency", Detail: fmt.Sprintf("unrecognised frequency %q", doc.Frequency)}
	}
	if doc.Decimals < 0 {
		return &ValidationError{Series: doc.Slug, Constraint: "decimals", Detail: "negative"}
	}
	if doc.Source.ID == "" || doc.Source.Name == "" || doc.Source.Attribution == "" {
		return &ValidationError{Series: doc.Slug, Constraint: "source", Detail: "id, name and attribution must all be non-empty"}
	}
	if doc.Origin.Kind == "" || doc.Origin.Ref == "" {
		return &ValidationError{Series: doc.Slug, Constraint: "origin", Detail: "kind and ref must both be non-empty"}
	}
	if doc.Freshness != FreshnessFresh && doc.Freshness != FreshnessSourcePending {
		return &ValidationError{Series: doc.Slug, Constraint: "freshness", Detail: fmt.Sprintf("unrecognised state %q", doc.Freshness)}
	}

	if len(doc.Points) > 0 || len(doc.Withdrawn) > 0 {
		if doc.Vintage.IngestionRunID <= 0 || doc.Vintage.ExtractedAt.IsZero() {
			return &ValidationError{Series: doc.Slug, Constraint: "vintage", Detail: "a series with observations must carry a resolved ingestion_run_id and extracted_at"}
		}
	}

	if err := validatePageState(doc); err != nil {
		return err
	}

	if err := validatePoints(doc, freq); err != nil {
		return err
	}
	if err := validateBreaks(doc); err != nil {
		return err
	}
	if err := validateEvents(doc); err != nil {
		return err
	}
	return nil
}

// validatePoints enforces the two RED-scenario constraints task 3.2
// names directly: status must be P or D (a withdrawn row belongs in
// Withdrawn, never Points -- design D-1, "Current W tombstones are
// excluded from points"), and every period must parse under the
// series' OWN declared frequency, in strictly increasing, non-
// duplicated order ("cadence-inconsistent period"). It also confirms
// every point's ingestion_run_id resolves inside Vintages -- D1's own
// resolution would be silently unreachable-in-practice if a writer bug
// ever let a point's run id drift from the map it is supposed to key
// into.
func validatePoints(doc SeriesDoc, freq indicators.Frequency) error {
	var prior *indicators.Period
	for _, p := range doc.Points {
		if p.Status != "P" && p.Status != "D" {
			return &ValidationError{Series: doc.Slug, Period: p.Period, Constraint: "status", Detail: fmt.Sprintf("published points must be P or D, got %q", p.Status)}
		}
		parsed, err := indicators.NormalizePeriodLabel(p.Period)
		if err != nil {
			return &ValidationError{Series: doc.Slug, Period: p.Period, Constraint: "period", Detail: err.Error()}
		}
		if parsed.Frequency != freq {
			return &ValidationError{Series: doc.Slug, Period: p.Period, Constraint: "cadence", Detail: fmt.Sprintf("period frequency %q does not match the series' declared frequency %q", parsed.Frequency, freq)}
		}
		if prior != nil && !parsed.After(*prior) {
			return &ValidationError{Series: doc.Slug, Period: p.Period, Constraint: "cadence", Detail: "points must be strictly increasing with no duplicate period"}
		}
		prior = &parsed
		if _, ok := doc.Vintages[runKey(p.IngestionRunID)]; !ok {
			return &ValidationError{Series: doc.Slug, Period: p.Period, Constraint: "vintages", Detail: fmt.Sprintf("point references ingestion_run_id %d with no matching entry in vintages", p.IngestionRunID)}
		}
	}
	return nil
}

// validatePageState enforces PageStateRef's whole contract (see its own
// doc comment in artifact.go): a closed three-value enum, and two
// fields each populated ONLY on the one state that consumes it.
//
// The conditional-population checks are not pedantry. A lastCorrectUpdate
// on a "fresh" page is a date with no sentence to appear in, and the
// only way the web layer could use it would be to invent copy the spec
// does not describe; a successorSlug on a page that is not discontinued
// is a "continue here" link offered for a series that has not stopped.
// Both are shapes a writer bug can produce and a reader cannot make
// sense of, so they are rejected here -- before the artifact replaces a
// good one on disk -- rather than at the Astro loader, after.
//
// A validation-failure with NO lastCorrectUpdate is deliberately VALID:
// a series whose very first run failed has a real failure and no date to
// name (SeriesPageState's own doc comment records why that is emitted
// rather than substituted or hidden).
func validatePageState(doc SeriesDoc) error {
	reject := func(detail string) error {
		return &ValidationError{Series: doc.Slug, Constraint: "page-state", Detail: detail}
	}

	switch doc.PageState.Kind {
	case PageStateFresh, PageStateValidationFailure, PageStateDiscontinued:
	default:
		return reject(fmt.Sprintf("unrecognised kind %q (expected %q, %q or %q)",
			doc.PageState.Kind, PageStateFresh, PageStateValidationFailure, PageStateDiscontinued))
	}

	if doc.PageState.LastCorrectUpdate != nil {
		if doc.PageState.Kind != PageStateValidationFailure {
			return reject(fmt.Sprintf("lastCorrectUpdate is set on a %q page; it belongs only to %q",
				doc.PageState.Kind, PageStateValidationFailure))
		}
		if _, err := time.Parse("2006-01-02", *doc.PageState.LastCorrectUpdate); err != nil {
			return reject(fmt.Sprintf("lastCorrectUpdate %q is not an ISO calendar date (YYYY-MM-DD)",
				*doc.PageState.LastCorrectUpdate))
		}
	}

	if doc.PageState.SuccessorSlug != nil {
		if doc.PageState.Kind != PageStateDiscontinued {
			return reject(fmt.Sprintf("successorSlug is set on a %q page; it belongs only to %q",
				doc.PageState.Kind, PageStateDiscontinued))
		}
		if *doc.PageState.SuccessorSlug == "" {
			return reject("successorSlug is present but empty; omit it (null) when no successor is configured")
		}
	}
	return nil
}

func validateBreaks(doc SeriesDoc) error {
	seen := map[string]bool{}
	for _, b := range doc.Breaks {
		if b.Key == "" {
			return &ValidationError{Series: doc.Slug, Constraint: "break-reference", Detail: "a break entry with no key does not resolve to any single, identifiable break"}
		}
		if seen[b.Key] {
			return &ValidationError{Series: doc.Slug, Constraint: "break-reference", Detail: fmt.Sprintf("break key %q is duplicated -- an unresolved (ambiguous) reference", b.Key)}
		}
		seen[b.Key] = true
	}
	return nil
}

func validateEvents(doc SeriesDoc) error {
	seen := map[string]bool{}
	for _, e := range doc.Events {
		if e.ID == "" {
			return &ValidationError{Series: doc.Slug, Constraint: "event-reference", Detail: "an event entry with no id does not resolve to any single, identifiable event"}
		}
		if seen[e.ID] {
			return &ValidationError{Series: doc.Slug, Constraint: "event-reference", Detail: fmt.Sprintf("event id %q is duplicated -- an unresolved (ambiguous) reference", e.ID)}
		}
		seen[e.ID] = true
	}
	return nil
}

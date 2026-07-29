package ine

// Task 5a.11/5a.12 (GREEN): periodicity is asserted when resolving an
// identifier (spec source-ingestion-ine). Table name alone cannot
// identify a series (tables 65962/72982 share 65109's Nombre but hold
// annual averages), so this file classifies the response's actual
// cadence from its raw T3_Periodo code and fails fast -- as a named,
// non-retryable sourceerr.SchemaDrift -- when it disagrees with the
// caller's configured expectation, per design.md's error-taxonomy
// decision ("SchemaDrift covers ... periodicity mismatch").
//
// Task 5b.6 (GREEN): INE's annual cadence code IS now live-verified --
// the single letter "A" (GET .../DATOS_SERIE/EPA634676?nult=2&tip=A,
// verified 2026-07-28, returns T3_Periodo="A" for both 2024 and 2025).
// It is still classified as an unrecognised (empty Frequency) shape here
// on purpose: none of the six milestone-0.2 series is annual, and adding
// indicators.FrequencyAnnual is genuinely out of Fase 0 scope (it would
// also require changes to Frequency.stepsPerYear, Period.String, Next,
// Prev and NormalizePeriodLabel -- a domain change, not an adapter one).
// Returning an empty Frequency for "A" remains correct and fail-safe: it
// can never equal a configured Q/M expectation, so periodicity assertion
// still fails rather than silently passing. What DID change: the
// diagnostic message envelope.go builds now surfaces the raw code ("A")
// as the actual periodicity instead of an empty string -- see
// envelope.go's periodicityLabel.

import (
	"regexp"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

var (
	reQuarterCode = regexp.MustCompile(`^T[1-4]$`)
	reMonthCode   = regexp.MustCompile(`^M(0[1-9]|1[0-2])$`)
)

// quarterStartLabels maps INE's population-series (ECP -- Estadística
// Continua de Población) quarter-start-date labels to their ordinal
// position within the year. Verified live 2026-07-28 against
// DATOS_SERIE/ECP320: its T3_Periodo carries "1 de enero de"/"1 de abril
// de"/"1 de julio de"/"1 de octubre de" instead of EPA/CNTR's "T1".."T4",
// even though SERIE/ECP320's own metadata (Periodicidad.Codigo="Q")
// confirms it is quarterly exactly like them -- INE encodes the same
// cadence with a second, source-specific label shape for this one
// operation. Without recognising it, ECP320 -- one of the six
// milestone-0.2 pinned series -- could never pass the periodicity
// assertion at all.
var quarterStartLabels = map[string]int{
	"1 de enero de":   1,
	"1 de abril de":   2,
	"1 de julio de":   3,
	"1 de octubre de": 4,
}

// detectPeriodicity classifies a raw INE T3_Periodo code by its shape:
// "T1".."T4" or one of quarterStartLabels' population-series phrases is
// quarterly, "M01".."M12" is monthly. Any other shape (including INE's
// own annual code "A" -- see the package doc comment above) is reported
// as an empty Frequency, which can never equal a configured Q/M
// expectation and therefore always fails the periodicity assertion
// rather than silently passing.
func detectPeriodicity(periodo string) indicators.Frequency {
	switch {
	case reQuarterCode.MatchString(periodo):
		return indicators.FrequencyQuarterly
	case reMonthCode.MatchString(periodo):
		return indicators.FrequencyMonthly
	case isQuarterStartLabel(periodo):
		return indicators.FrequencyQuarterly
	default:
		return ""
	}
}

func isQuarterStartLabel(periodo string) bool {
	_, ok := quarterStartLabels[periodo]
	return ok
}

// periodicityLabel renders detectPeriodicity's result for a diagnostic
// message: the classified Frequency when recognised, or the raw
// T3_Periodo code itself when detectPeriodicity returned the empty
// Frequency for an unrecognised cadence (e.g. INE's annual code "A" --
// see the package doc comment). Before this existed, an operator saw
// "expected Q, got " with the actual cadence invisible; the spec's own
// scenario requires the failure to name BOTH the expected and the actual
// periodicity (spec source-ingestion-ine, "A periodicity mismatch fails
// the run").
func periodicityLabel(freq indicators.Frequency, raw string) string {
	if freq != "" {
		return string(freq)
	}
	return raw
}

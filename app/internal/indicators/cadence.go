package indicators

// Task 1.2/1.6 (GREEN): the shared cadence-audit mechanism. Design D-4
// ("Periodicity over the full payload; cadence segments in config; Rule 2
// audits the interior") calls for one consistent check reused by the INE
// adapter (envelope.go, this run's own payload) and Rule2Continuity
// (validation/rule2_continuity.go, the whole prior+incoming history) --
// AssertCadence is that one mechanism, so the two call sites can never
// silently diverge.
//
// D3 adjudication (orchestrator, settled on divergence between spec and
// design): a CadenceSegment DECLARES its own real-world cadence (Cadence,
// a source-descriptive string -- "semiannual" for ECP320's historical
// span) while storage and period arithmetic stay on the series' base grid
// Frequency (Present is a filter of grid ordinals, never a new Frequency
// value). No FrequencySemiannual is added anywhere in this file or
// Period's own arithmetic.

import (
	"fmt"
	"sort"
	"strings"
)

// minPointsForDensityCheck is the fewest observed points AssertCadence
// needs, within one segment's clipped range, before it will attempt to
// infer that segment's OWN observed density pattern at all. Below this,
// there are too few consecutive-pair steps to distinguish "one isolated
// gap" from "this segment's real cadence" -- Rule2Continuity's ordinary
// per-period walk (with its documented-gap allowlist) is the layer that
// judges a short span, not this coarser structural check.
const minPointsForDensityCheck = 3

// AssertCadence's density check is a MAJORITY-VOTE over the step size
// between consecutive observed periods within a segment's own span, not
// a raw count ratio: a segment genuinely declared dense (Present empty)
// should have a step-1 majority regardless of how many isolated,
// individually-tolerated gaps it also has (Rule2Continuity, not this
// function, names and blocks an individual missing period, exempting the
// per-series documented-gap allowlist -- spec data-validation, "A
// skipped period fails continuity" / "An allowlisted gap passes"). A
// segment whose MAJORITY step does not match what its own declared
// cadence implies (1 for a dense segment; stepsPerYear/len(Present) for a
// restricted one, e.g. 4/2=2 for a quarterly grid's semiannual segment)
// means the declared cadence itself -- not one period -- is wrong (spec
// "A single quarterly declaration fails against a mixed history").
func densityStep(seg CadenceSegment, stepsPerYear int) int {
	if len(seg.Present) == 0 {
		return 1
	}
	// Exact for the evenly-spaced Present sets every configured segment
	// uses today (e.g. [1,3] on a 4-step quarterly grid); an unevenly
	// spaced Present would make this an approximation, disclosed here
	// rather than solved, since no series configures one.
	return stepsPerYear / len(seg.Present)
}

// CadenceSegment is one span of a series' declared real-world publication
// cadence. Storage and period arithmetic never leave the series' base
// grid Frequency (Period.Next/Previous/stepsPerYear are untouched) --
// Cadence is a source-descriptive label only, and Present is the
// mechanism that encodes a lower-density cadence (e.g. semiannual) as a
// filter of which grid ordinals are expected within the segment.
type CadenceSegment struct {
	From      Period
	To        Period // meaningful only when OpenEnded is false
	OpenEnded bool

	// Cadence is the source's own word for this segment's real-world
	// cadence (e.g. "semiannual", "quarterly") -- diagnostic and
	// config-legibility only; nothing in this package switches behaviour
	// on its exact text (behaviour is driven entirely by Present).
	Cadence string

	// Present lists the base-grid ordinals (1..Frequency.stepsPerYear())
	// this segment expects to be populated. Empty means every ordinal is
	// expected -- an ordinary dense/uniform segment.
	Present []int
}

func (s CadenceSegment) contains(p Period) bool {
	if p.Before(s.From) {
		return false
	}
	if s.OpenEnded {
		return true
	}
	return !p.After(s.To)
}

func (s CadenceSegment) expectsOrdinal(ordinal, stepsPerYear int) bool {
	if len(s.Present) == 0 {
		return ordinal >= 1 && ordinal <= stepsPerYear
	}
	for _, o := range s.Present {
		if o == ordinal {
			return true
		}
	}
	return false
}

// ParseCadenceSegment parses a config-level cadence_segments entry (plain
// strings/ints, config.CadenceSegmentConfig's own shape) into a domain
// CadenceSegment. to == "" means open-ended (must be the series' last
// segment -- validated by config.Validate, not here).
func ParseCadenceSegment(from, to, cadence string, present []int) (CadenceSegment, error) {
	fromPeriod, err := NormalizePeriodLabel(from)
	if err != nil {
		return CadenceSegment{}, fmt.Errorf("indicators: cadence segment \"from\" %q: %w", from, err)
	}
	seg := CadenceSegment{From: fromPeriod, Cadence: cadence, Present: present}
	if to == "" {
		seg.OpenEnded = true
		return seg, nil
	}
	toPeriod, err := NormalizePeriodLabel(to)
	if err != nil {
		return CadenceSegment{}, fmt.Errorf("indicators: cadence segment \"to\" %q: %w", to, err)
	}
	seg.To = toPeriod
	return seg, nil
}

// AssertCadence audits periods (a decoded payload, or the union of a
// series' stored history plus an incoming run) against segments -- the
// series' declared cadence. An empty segments means the series declares
// one uniform, dense cadence at freq (the ordinary five-of-six-series
// case): every grid ordinal is expected throughout.
//
// Two independent failure modes, both fail closed (spec source-
// ingestion-ine "An unrecognised token" sibling: never silently coerced):
//  1. an observed period falls outside every declared segment's range,
//     or its ordinal is not one that segment's Present declares -- this
//     catches an invented period (e.g. a Q2 inside a segment declared
//     semiannual).
//  2. a declared segment's observed MAJORITY step size, across the span
//     the payload actually covers, does not match what its declared
//     cadence implies -- this catches a segment (or, with no
//     cadence_segments declared, the whole series) whose declared
//     cadence does not match its real history at all (a single
//     quarterly declaration against a mixed semiannual/quarterly
//     history). See densityStep's doc comment for the majority-vote
//     mechanism.
func AssertCadence(periods []Period, freq Frequency, segments []CadenceSegment) error {
	if len(periods) == 0 {
		return nil
	}
	sorted := sortedUniquePeriods(periods)

	effective := segments
	if len(effective) == 0 {
		effective = []CadenceSegment{{From: sorted[0], OpenEnded: true, Cadence: frequencyCadenceLabel(freq)}}
	}
	stepsPerYear := freq.stepsPerYear()

	for _, p := range sorted {
		idx := segmentIndexFor(effective, p)
		if idx < 0 {
			return fmt.Errorf("period %s is not covered by any declared cadence segment (observed segmentation: %s)",
				p, describeObservedCadence(sorted))
		}
		seg := effective[idx]
		if !seg.expectsOrdinal(p.Ordinal, stepsPerYear) {
			return fmt.Errorf("period %s falls in a segment declared %q but its ordinal is not expected there (observed segmentation: %s)",
				p, seg.Cadence, describeObservedCadence(sorted))
		}
	}

	payloadMin, payloadMax := sorted[0], sorted[len(sorted)-1]
	for _, seg := range effective {
		rangeFrom := seg.From
		if rangeFrom.Before(payloadMin) {
			rangeFrom = payloadMin
		}
		rangeTo := payloadMax
		if !seg.OpenEnded && seg.To.Before(payloadMax) {
			rangeTo = seg.To
		}
		if rangeFrom.After(rangeTo) {
			continue // this segment's range does not overlap the observed payload at all
		}
		obsInRange := periodsInRange(sorted, rangeFrom, rangeTo)
		if len(obsInRange) < minPointsForDensityCheck {
			continue // too few points to infer a density pattern -- Rule2Continuity's job
		}
		stepCounts := map[int]int{}
		for i := 1; i < len(obsInRange); i++ {
			stepCounts[ordinalDistance(obsInRange[i-1], obsInRange[i])]++
		}
		majorityStep, majorityCount := 0, -1
		for _, step := range sortedIntKeys(stepCounts) {
			if stepCounts[step] > majorityCount {
				majorityStep, majorityCount = step, stepCounts[step]
			}
		}
		if want := densityStep(seg, stepsPerYear); majorityStep != want {
			return fmt.Errorf(
				"declared cadence %q for %s..%s does not match the observed history's majority spacing (observed step %d, expected step %d; observed segmentation: %s)",
				seg.Cadence, rangeFrom, rangeTo, majorityStep, want, describeObservedCadence(sorted))
		}
	}
	return nil
}

// frequencyCadenceLabel renders freq as the human-readable cadence word an
// implicit (no cadence_segments declared) uniform segment carries in
// diagnostics, so "A single quarterly declaration fails against a mixed
// history" names the declared cadence in words an editor recognises, not
// only the one-letter config code.
func frequencyCadenceLabel(freq Frequency) string {
	switch freq {
	case FrequencyQuarterly:
		return "quarterly"
	case FrequencyMonthly:
		return "monthly"
	case FrequencyAnnual:
		return "annual"
	default:
		return string(freq)
	}
}

// CadenceExpects reports whether p is a period the declared cadence
// (segments, or freq's ordinary dense/uniform behaviour when segments is
// empty) expects to be populated. Rule2Continuity (validation package)
// uses this to skip periods no declared segment expects, so continuity
// is audited "under its own cadence" per segment (spec data-validation,
// "each segment MUST be audited under its own cadence") rather than
// flagging every historically-absent Q2/Q4 as a missing period. A period
// outside every declared segment's range is reported as NOT expected
// here (Rule2 is not the layer that reports a cadence-coverage gap --
// AssertCadence already fails closed on that, earlier, in the same
// caller).
func CadenceExpects(p Period, freq Frequency, segments []CadenceSegment) bool {
	effective := segments
	if len(effective) == 0 {
		effective = []CadenceSegment{{From: p, OpenEnded: true, Cadence: frequencyCadenceLabel(freq)}}
	}
	idx := segmentIndexFor(effective, p)
	if idx < 0 {
		return false
	}
	return effective[idx].expectsOrdinal(p.Ordinal, freq.stepsPerYear())
}

func segmentIndexFor(segments []CadenceSegment, p Period) int {
	for i, seg := range segments {
		if seg.contains(p) {
			return i
		}
	}
	return -1
}

func sortedUniquePeriods(periods []Period) []Period {
	out := append([]Period(nil), periods...)
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	deduped := out[:0]
	for i, p := range out {
		if i == 0 || !p.Equal(out[i-1]) {
			deduped = append(deduped, p)
		}
	}
	return deduped
}

// periodsInRange returns the subset of sorted falling within [from,to]
// (both inclusive). sorted is assumed already sorted (sortedUniquePeriods's
// own postcondition), so the result is sorted too.
func periodsInRange(sorted []Period, from, to Period) []Period {
	var out []Period
	for _, p := range sorted {
		if !p.Before(from) && !p.After(to) {
			out = append(out, p)
		}
	}
	return out
}

// sortedIntKeys returns m's keys in ascending order, so a majority-vote
// tie between two step sizes resolves deterministically (smallest step
// wins) instead of depending on Go's randomised map iteration order.
func sortedIntKeys(m map[int]int) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

// describeObservedCadence renders periods' actual segmentation -- runs of
// consecutive periods sharing the same step size -- so a failure message
// gives the editor exactly what the payload contains, letting them
// correct config/series/{slug}.yaml from the message itself (spec
// source-ingestion-ine, "the diagnostic prints the observed
// segmentation").
func describeObservedCadence(sorted []Period) string {
	if len(sorted) == 0 {
		return ""
	}
	if len(sorted) == 1 {
		return sorted[0].String()
	}

	type run struct {
		from, to Period
		step     int
	}
	var runs []run
	runFrom, runTo := sorted[0], sorted[0]
	runStep := ordinalDistance(sorted[0], sorted[1])
	for i := 1; i < len(sorted); i++ {
		step := ordinalDistance(sorted[i-1], sorted[i])
		if step != runStep {
			runs = append(runs, run{from: runFrom, to: runTo, step: runStep})
			runFrom = sorted[i-1]
			runStep = step
		}
		runTo = sorted[i]
	}
	runs = append(runs, run{from: runFrom, to: runTo, step: runStep})

	parts := make([]string, 0, len(runs))
	for _, r := range runs {
		if r.from.Equal(r.to) {
			parts = append(parts, r.from.String())
			continue
		}
		parts = append(parts, fmt.Sprintf("%s..%s (step %d)", r.from, r.to, r.step))
	}
	return strings.Join(parts, "; ")
}

func ordinalDistance(a, b Period) int {
	steps := a.Frequency.stepsPerYear()
	return (b.Year-a.Year)*steps + (b.Ordinal - a.Ordinal)
}

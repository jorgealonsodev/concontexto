// Package indicators is the domain: pure value types for series,
// observations, periods and editorial breaks. It imports nothing but
// the standard library (design.md package layout, "indicators/ #
// DOMAIN — stdlib only"), so anything built on top of it — starting
// with app/internal/ingestion/validation — inherits that same freedom
// from I/O and the clock for free.
package indicators

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// Frequency is a series' reporting cadence, using the same single-letter
// codes as config/series/{slug}.yaml's frequency field (design.md
// "frequency: Q") so no translation layer exists between the parsed
// config and the domain.
type Frequency string

const (
	FrequencyQuarterly Frequency = "Q"
	FrequencyMonthly   Frequency = "M"

	// FrequencyAnnual was added for slice 6 (Eurostat, milestone 0.3):
	// nama_10_gdp is annual data (spec source-ingestion-eurostat's
	// verified filter table). Eurostat's own native annual label ("2025")
	// needs no adapter-side join the way INE's Anyo/Periodo pair does, so
	// this is purely a domain arithmetic addition (stepsPerYear/String/
	// NormalizePeriodLabel below) -- it does NOT change INE's periodicity
	// detection (adapters/ine/periodicity.go still classifies INE's own
	// annual code "A" as an unrecognised, empty Frequency on purpose:
	// none of the six milestone-0.2 INE series is annual, and that
	// decision is independently disclosed there).
	FrequencyAnnual Frequency = "A"
)

// stepsPerYear returns how many ordinal positions one year holds for f,
// or 0 for an unrecognised frequency.
func (f Frequency) stepsPerYear() int {
	switch f {
	case FrequencyQuarterly:
		return 4
	case FrequencyMonthly:
		return 12
	case FrequencyAnnual:
		return 1
	default:
		return 0
	}
}

// Period is one reporting period for a series: a year plus an ordinal
// position within it (quarter 1-4, month 1-12, or always 1 for Annual),
// always tied to an explicit Frequency. Period is a pure value type —
// comparing or advancing one never touches the system clock (spec
// data-validation, "A rule is deterministic and side-effect free"; the
// reference point a rule needs is always an explicit argument, e.g.
// Prior observations, never time.Now()).
type Period struct {
	Frequency Frequency
	Year      int
	Ordinal   int // 1-4 for Quarterly, 1-12 for Monthly, always 1 for Annual
}

// String renders the canonical form every source-specific label
// normalises INTO (spec data-validation, "Period-format conventions
// differ per source ... and MUST be normalised before the check"):
// Eurostat's own native shapes, "2026-Q1" / "2026-06", which is why
// Eurostat labels round-trip through NormalizePeriodLabel unchanged.
func (p Period) String() string {
	switch p.Frequency {
	case FrequencyQuarterly:
		return fmt.Sprintf("%04d-Q%d", p.Year, p.Ordinal)
	case FrequencyMonthly:
		return fmt.Sprintf("%04d-%02d", p.Year, p.Ordinal)
	case FrequencyAnnual:
		return fmt.Sprintf("%04d", p.Year)
	default:
		return fmt.Sprintf("%04d-%s%d", p.Year, p.Frequency, p.Ordinal)
	}
}

// Next returns the period immediately following p, wrapping the ordinal
// into the next year exactly once every stepsPerYear() steps.
func (p Period) Next() Period {
	steps := p.Frequency.stepsPerYear()
	if p.Ordinal >= steps {
		return Period{Frequency: p.Frequency, Year: p.Year + 1, Ordinal: 1}
	}
	return Period{Frequency: p.Frequency, Year: p.Year, Ordinal: p.Ordinal + 1}
}

// Previous returns the period immediately preceding p — the exact
// inverse of Next, used by rule 3 (plausibility) to find the prior
// value a period-over-period delta is measured against.
func (p Period) Previous() Period {
	steps := p.Frequency.stepsPerYear()
	if p.Ordinal <= 1 {
		return Period{Frequency: p.Frequency, Year: p.Year - 1, Ordinal: steps}
	}
	return Period{Frequency: p.Frequency, Year: p.Year, Ordinal: p.Ordinal - 1}
}

// Compare returns -1, 0, or 1 as p is before, equal to, or after other,
// ordering first by Year then by Ordinal. Callers never compare periods
// of different frequencies within one series, so Compare does not
// special-case a Frequency mismatch.
func (p Period) Compare(other Period) int {
	switch {
	case p.Year != other.Year:
		if p.Year < other.Year {
			return -1
		}
		return 1
	case p.Ordinal != other.Ordinal:
		if p.Ordinal < other.Ordinal {
			return -1
		}
		return 1
	default:
		return 0
	}
}

func (p Period) Equal(other Period) bool  { return p.Compare(other) == 0 }
func (p Period) Before(other Period) bool { return p.Compare(other) < 0 }
func (p Period) After(other Period) bool  { return p.Compare(other) > 0 }

// Label-shape patterns recognised by NormalizePeriodLabel. Eurostat's
// own shapes are already canonical; INE's shapes join its "Periodo"
// code (T1-T4 / M01-M12) with its "Anyo" year field into one string —
// see the doc comment on NormalizePeriodLabel for why the monthly INE
// shape includes an explicit year rather than the spec's bare "M06"
// example.
var (
	reEurostatQuarterly = regexp.MustCompile(`^(\d{4})-Q([1-4])$`)
	reEurostatMonthly   = regexp.MustCompile(`^(\d{4})-(0[1-9]|1[0-2])$`)
	reEurostatAnnual    = regexp.MustCompile(`^(\d{4})$`)
	reINEQuarterly      = regexp.MustCompile(`^T([1-4]) (\d{4})$`)
	reINEMonthly        = regexp.MustCompile(`^M(0[1-9]|1[0-2]) (\d{4})$`)
)

// NormalizePeriodLabel parses a source-specific period label into the
// canonical Period (spec data-validation, "Rule 2 — continuity ...
// Period-format conventions differ per source ... INE T1 2026 and M06,
// Eurostat 2026-Q1 and 2026-06 ... MUST be normalised before the
// check"). The parser recognises label SHAPES, not "this came from
// INE" — by the time a rule normalises a label it must not care which
// adapter produced it (design.md, "rules never see source formats"
// once normalised).
//
// Note on the INE monthly shape: the spec's illustrative example gives
// the bare code "M06" with no year. INE's real Tempus API carries year
// and period as SEPARATE fields (Anyo / Periodo) that the adapter
// joins before this function ever sees the label — exactly like the
// quarterly example "T1 2026" already joins Periodo="T1" with
// Anyo="2026". This function therefore accepts the same "<code>
// <year>" shape for both INE cadences ("T1 2026", "M06 2026"), keeping
// one uniform signature across every recognised label shape rather than
// adding a second, cadence-only parameter. Slice 5a's real INE adapter
// (not built here) is expected to produce exactly this joined shape.
func NormalizePeriodLabel(raw string) (Period, error) {
	if m := reEurostatQuarterly.FindStringSubmatch(raw); m != nil {
		return Period{Frequency: FrequencyQuarterly, Year: atoi(m[1]), Ordinal: atoi(m[2])}, nil
	}
	if m := reEurostatMonthly.FindStringSubmatch(raw); m != nil {
		return Period{Frequency: FrequencyMonthly, Year: atoi(m[1]), Ordinal: atoi(m[2])}, nil
	}
	if m := reEurostatAnnual.FindStringSubmatch(raw); m != nil {
		return Period{Frequency: FrequencyAnnual, Year: atoi(m[1]), Ordinal: 1}, nil
	}
	if m := reINEQuarterly.FindStringSubmatch(raw); m != nil {
		return Period{Frequency: FrequencyQuarterly, Year: atoi(m[2]), Ordinal: atoi(m[1])}, nil
	}
	if m := reINEMonthly.FindStringSubmatch(raw); m != nil {
		return Period{Frequency: FrequencyMonthly, Year: atoi(m[2]), Ordinal: atoi(m[1])}, nil
	}
	return Period{}, fmt.Errorf("indicators: unrecognised period label %q", raw)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s) // regexp already constrained s to digits
	return n
}

// PeriodFromDate converts a calendar date (e.g. series_break.date, which
// PostgreSQL always returns at UTC midnight) into the Period it falls
// within at freq — the one join point between an editorial break's
// calendar date and a series' own period arithmetic (rule 3's exemption
// check compares Period, not raw dates: see breakAt in
// rule3_plausibility.go). Quarterly buckets the date's month into
// quarters 1-4; Annual always yields ordinal 1; any other Frequency
// (including the zero value) falls back to Monthly's month-number
// ordinal, matching stepsPerYear's own "unrecognised frequency" default
// rather than panicking on a Break scoped to a series whose frequency
// this package cannot yet classify.
func PeriodFromDate(t time.Time, freq Frequency) Period {
	switch freq {
	case FrequencyQuarterly:
		return Period{Frequency: freq, Year: t.Year(), Ordinal: (int(t.Month())-1)/3 + 1}
	case FrequencyAnnual:
		return Period{Frequency: freq, Year: t.Year(), Ordinal: 1}
	default:
		return Period{Frequency: freq, Year: t.Year(), Ordinal: int(t.Month())}
	}
}

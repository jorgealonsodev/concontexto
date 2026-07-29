package indicators_test

// Task 4.5 (RED, period-normalisation half): Period is the pure domain
// value every validation rule reasons about (design.md package layout,
// "indicators/ ... Period"). Rule 2 (continuity) needs two properties
// proven here in isolation before it can be trusted: (1) advancing and
// comparing periods never touches the clock or any external state
// (spec data-validation, "A rule is deterministic and side-effect
// free"), and (2) source-specific labels normalise into ONE canonical
// form regardless of which source produced them (spec, "Period-format
// conventions differ per source ... and MUST be normalised before the
// check").

import (
	"testing"
	"time"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

func TestNormalizePeriodLabel_SourceIndependentQuarterly(t *testing.T) {
	want := indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 1}

	for _, raw := range []string{"2026-Q1", "T1 2026"} {
		got, err := indicators.NormalizePeriodLabel(raw)
		if err != nil {
			t.Fatalf("NormalizePeriodLabel(%q): unexpected error: %v", raw, err)
		}
		if got != want {
			t.Errorf("NormalizePeriodLabel(%q) = %+v, want %+v", raw, got, want)
		}
	}
}

func TestNormalizePeriodLabel_SourceIndependentMonthly(t *testing.T) {
	want := indicators.Period{Frequency: indicators.FrequencyMonthly, Year: 2026, Ordinal: 6}

	for _, raw := range []string{"2026-06", "M06 2026"} {
		got, err := indicators.NormalizePeriodLabel(raw)
		if err != nil {
			t.Fatalf("NormalizePeriodLabel(%q): unexpected error: %v", raw, err)
		}
		if got != want {
			t.Errorf("NormalizePeriodLabel(%q) = %+v, want %+v", raw, got, want)
		}
	}
}

func TestNormalizePeriodLabel_RejectsUnrecognisedShape(t *testing.T) {
	if _, err := indicators.NormalizePeriodLabel("garbage"); err == nil {
		t.Fatal("expected an error for an unrecognised period label, got nil")
	}
}

// TestNormalizePeriodLabel_Annual (task 6.5's design decision): Eurostat's
// nama_10_gdp dataset is annual data (spec source-ingestion-eurostat's
// verified filter table, "nama_10_gdp"), so the domain needs a THIRD
// Frequency alongside Quarterly/Monthly -- Eurostat's own bare-year shape
// ("2025") is already canonical, needing no join step the way INE's
// separate Anyo/Periodo fields do (unlike INE, no live-verified INE
// annual label shape is added here; INE's "A" code stays deliberately
// unrecognised per adapters/ine/periodicity.go's own disclosed decision,
// since none of the six milestone-0.2 INE series is annual -- adding
// FrequencyAnnual to the domain does not, by itself, change that).
func TestNormalizePeriodLabel_Annual(t *testing.T) {
	want := indicators.Period{Frequency: indicators.FrequencyAnnual, Year: 2025, Ordinal: 1}
	got, err := indicators.NormalizePeriodLabel("2025")
	if err != nil {
		t.Fatalf("NormalizePeriodLabel(\"2025\"): unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("NormalizePeriodLabel(\"2025\") = %+v, want %+v", got, want)
	}
}

func TestPeriod_String(t *testing.T) {
	cases := []struct {
		period indicators.Period
		want   string
	}{
		{indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 1}, "2026-Q1"},
		{indicators.Period{Frequency: indicators.FrequencyMonthly, Year: 2026, Ordinal: 6}, "2026-06"},
		{indicators.Period{Frequency: indicators.FrequencyAnnual, Year: 2025, Ordinal: 1}, "2025"},
	}
	for _, c := range cases {
		if got := c.period.String(); got != c.want {
			t.Errorf("Period{%+v}.String() = %q, want %q", c.period, got, c.want)
		}
	}
}

func TestPeriod_NextAdvancesAndWrapsYear(t *testing.T) {
	q := indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 1}
	if next := q.Next(); next != (indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 2}) {
		t.Errorf("Q1 2026 .Next() = %+v, want Q2 2026", next)
	}
	q4 := indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 4}
	if next := q4.Next(); next != (indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2027, Ordinal: 1}) {
		t.Errorf("Q4 2026 .Next() = %+v, want Q1 2027", next)
	}

	m := indicators.Period{Frequency: indicators.FrequencyMonthly, Year: 2026, Ordinal: 12}
	if next := m.Next(); next != (indicators.Period{Frequency: indicators.FrequencyMonthly, Year: 2027, Ordinal: 1}) {
		t.Errorf("M12 2026 .Next() = %+v, want M01 2027", next)
	}
}

func TestPeriod_PreviousIsTheInverseOfNext(t *testing.T) {
	p := indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 1}
	if got := p.Next().Previous(); got != p {
		t.Errorf("p.Next().Previous() = %+v, want %+v", got, p)
	}
}

func TestPeriod_AnnualNextAdvancesYear(t *testing.T) {
	a := indicators.Period{Frequency: indicators.FrequencyAnnual, Year: 2025, Ordinal: 1}
	if next := a.Next(); next != (indicators.Period{Frequency: indicators.FrequencyAnnual, Year: 2026, Ordinal: 1}) {
		t.Errorf("2025 (annual) .Next() = %+v, want 2026", next)
	}
	if got := a.Next().Previous(); got != a {
		t.Errorf("annual Next().Previous() = %+v, want %+v", got, a)
	}
}

func TestPeriod_CompareOrdering(t *testing.T) {
	q1 := indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 1}
	q2 := indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 2}
	q1NextYear := indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2027, Ordinal: 1}

	if !q1.Before(q2) {
		t.Error("Q1 2026 should be Before Q2 2026")
	}
	if !q2.After(q1) {
		t.Error("Q2 2026 should be After Q1 2026")
	}
	if !q2.Before(q1NextYear) {
		t.Error("Q2 2026 should be Before Q1 2027")
	}
	if !q1.Equal(q1) {
		t.Error("a period should Equal itself")
	}
}

// TestPeriodFromDate is the RED half of C1's remediation (verify-report
// CRITICAL C1): a resolved postgres.SeriesBreak carries a calendar Date,
// but Rule3Plausibility's breakAt() compares indicators.Period — this is
// the one pure conversion function that lets ingest.go join the two
// without duplicating period arithmetic outside this package.
func TestPeriodFromDate(t *testing.T) {
	cases := []struct {
		name string
		date time.Time
		freq indicators.Frequency
		want indicators.Period
	}{
		{"monthly", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), indicators.FrequencyMonthly,
			indicators.Period{Frequency: indicators.FrequencyMonthly, Year: 2026, Ordinal: 5}},
		{"quarterly-Q2", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), indicators.FrequencyQuarterly,
			indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 2}},
		{"quarterly-Q1-boundary", time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), indicators.FrequencyQuarterly,
			indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 1}},
		{"annual", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), indicators.FrequencyAnnual,
			indicators.Period{Frequency: indicators.FrequencyAnnual, Year: 2026, Ordinal: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := indicators.PeriodFromDate(tc.date, tc.freq); got != tc.want {
				t.Errorf("PeriodFromDate(%v, %s) = %+v, want %+v", tc.date, tc.freq, got, tc.want)
			}
		})
	}
}

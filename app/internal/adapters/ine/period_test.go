package ine

// Task 5a.15/5a.16: direct, isolated triangulation over
// normalizeINEPeriod -- the pure join-then-normalize function that makes
// INE period normalisation source-independent (spec source-ingestion-ine,
// "INE period labels ... MUST be normalised to the canonical domain
// period before validation, so that continuity checks are
// source-independent").
//
// Disclosed honestly: normalizeINEPeriod already existed before this
// test was written -- it was required from task 5a.9/5a.10's very first
// GREEN step to decode ANY observation at all (every DATOS_SERIE
// response, success or not, carries Anyo/T3_Periodo pairs that must
// become an indicators.Period). This is therefore not a first-fail RED
// for the function's existence; it is dedicated unit-level triangulation
// isolating the join logic from the HTTP/decode machinery already
// covered end-to-end by client_test.go and periodicity_test.go (whose
// TestFetchSeries_MatchingPeriodicityProceeds already exercises the
// monthly case, and whose success-path test exercises the quarterly
// case). White-box (package ine, not ine_test) because
// normalizeINEPeriod is unexported.

import (
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

func TestNormalizeINEPeriod(t *testing.T) {
	cases := []struct {
		name    string
		anyo    int
		periodo string
		want    indicators.Period
		wantErr bool
	}{
		{
			name:    "quarterly T1 2026",
			anyo:    2026,
			periodo: "T1",
			want:    indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 1},
		},
		{
			name:    "quarterly T4 2025",
			anyo:    2025,
			periodo: "T4",
			want:    indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2025, Ordinal: 4},
		},
		{
			name:    "monthly M06 2026",
			anyo:    2026,
			periodo: "M06",
			want:    indicators.Period{Frequency: indicators.FrequencyMonthly, Year: 2026, Ordinal: 6},
		},
		{
			name:    "monthly M01 2026",
			anyo:    2026,
			periodo: "M01",
			want:    indicators.Period{Frequency: indicators.FrequencyMonthly, Year: 2026, Ordinal: 1},
		},
		{
			name:    "unrecognised shape errors rather than silently normalising",
			anyo:    2026,
			periodo: "T5",
			wantErr: true,
		},
		// Task 5b.6 (RED/GREEN): verified live 2026-07-28 against
		// DATOS_SERIE/ECP320 -- INE's population series (Estadística
		// Continua de Población) reports its quarterly period as a
		// Spanish quarter-start-date phrase ("1 de enero de", "1 de abril
		// de", "1 de julio de", "1 de octubre de") instead of EPA/CNTR's
		// "T1".."T4", even though SERIE/ECP320's own metadata confirms it
		// is quarterly (Periodicidad.Codigo="Q") like them. Without this
		// case, ECP320 -- one of the six milestone-0.2 pinned series --
		// could never pass the periodicity assertion at all.
		{
			name:    "population quarter-start label Q1 (1 de enero de)",
			anyo:    2026,
			periodo: "1 de enero de",
			want:    indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 1},
		},
		{
			name:    "population quarter-start label Q2 (1 de abril de)",
			anyo:    2026,
			periodo: "1 de abril de",
			want:    indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2026, Ordinal: 2},
		},
		{
			name:    "population quarter-start label Q3 (1 de julio de)",
			anyo:    2025,
			periodo: "1 de julio de",
			want:    indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2025, Ordinal: 3},
		},
		{
			name:    "population quarter-start label Q4 (1 de octubre de)",
			anyo:    2025,
			periodo: "1 de octubre de",
			want:    indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 2025, Ordinal: 4},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeINEPeriod(tc.anyo, tc.periodo)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error for Anyo=%d T3_Periodo=%q, got %+v", tc.anyo, tc.periodo, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeINEPeriod(%d, %q): %v", tc.anyo, tc.periodo, err)
			}
			if got != tc.want {
				t.Fatalf("normalizeINEPeriod(%d, %q) = %+v, want %+v", tc.anyo, tc.periodo, got, tc.want)
			}
			if got.String() == "" {
				t.Fatal("expected a non-empty canonical String() rendering")
			}
		})
	}
}

func TestDetectPeriodicity(t *testing.T) {
	cases := []struct {
		periodo string
		want    indicators.Frequency
	}{
		{"T1", indicators.FrequencyQuarterly},
		{"T4", indicators.FrequencyQuarterly},
		{"M01", indicators.FrequencyMonthly},
		{"M12", indicators.FrequencyMonthly},
		{"", ""},
		{"A", ""},
		{"1 de enero de", indicators.FrequencyQuarterly},
		{"1 de abril de", indicators.FrequencyQuarterly},
		{"1 de julio de", indicators.FrequencyQuarterly},
		{"1 de octubre de", indicators.FrequencyQuarterly},
	}
	for _, tc := range cases {
		t.Run(tc.periodo, func(t *testing.T) {
			if got := detectPeriodicity(tc.periodo); got != tc.want {
				t.Errorf("detectPeriodicity(%q) = %q, want %q", tc.periodo, got, tc.want)
			}
		})
	}
}

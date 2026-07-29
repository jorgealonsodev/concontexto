package xlsx

// Internal (white-box) test: same package as period.go so it can exercise
// the unexported parser directly, mirroring adapters/ine's own
// period_test.go pattern for its own unexported normalizeINEPeriod.

import (
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

func TestParsePeriodoLabel(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    indicators.Period
		wantErr bool
	}{
		{name: "plain january", raw: "Enero 2001", want: indicators.Period{Frequency: indicators.FrequencyMonthly, Year: 2001, Ordinal: 1}},
		{name: "june 2026", raw: "Junio 2026", want: indicators.Period{Frequency: indicators.FrequencyMonthly, Year: 2026, Ordinal: 6}},
		{
			name: "verified double-space trap (task 8.1)",
			raw:  "Febrero  2001", // two spaces, exactly as the real workbook carries it
			want: indicators.Period{Frequency: indicators.FrequencyMonthly, Year: 2001, Ordinal: 2},
		},
		{name: "leading/trailing whitespace", raw: "  Marzo 2010  ", want: indicators.Period{Frequency: indicators.FrequencyMonthly, Year: 2010, Ordinal: 3}},
		{name: "case-insensitive month name", raw: "DICIEMBRE 2011", want: indicators.Period{Frequency: indicators.FrequencyMonthly, Year: 2011, Ordinal: 12}},
		{name: "unrecognised month name", raw: "Marzoo 2010", wantErr: true},
		{name: "footnote-shaped row is not a period", raw: "(1) No incluye los Sistemas Especiales", wantErr: true},
		{name: "empty string", raw: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePeriodoLabel(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error parsing %q, got %v", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePeriodoLabel(%q): %v", tc.raw, err)
			}
			if got != tc.want {
				t.Errorf("parsePeriodoLabel(%q) = %+v, want %+v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestIsFootnoteRow(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"(1) No incluye los Sistemas Especiales Agrario y de Empleados de Hogar", true},
		{"(10) Vigente desde 1-enero-2008", true},
		{"Enero 2001", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isFootnoteRow(tc.raw); got != tc.want {
			t.Errorf("isFootnoteRow(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestCollapseInternalWhitespace(t *testing.T) {
	if got := collapseInternalWhitespace("Febrero  2001"); got != "Febrero 2001" {
		t.Errorf("collapseInternalWhitespace = %q, want %q", got, "Febrero 2001")
	}
	if got := collapseInternalWhitespace("  Enero   2001  "); got != "Enero 2001" {
		t.Errorf("collapseInternalWhitespace = %q, want %q", got, "Enero 2001")
	}
}

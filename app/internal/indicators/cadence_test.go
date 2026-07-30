package indicators_test

// Task 1.1/1.5 (RED): the shared cadence-audit mechanism both the INE
// adapter (whole-payload periodicity, envelope.go) and Rule2Continuity
// (interior audit) delegate to (spec source-ingestion-ine "Periodicity is
// asserted when resolving an identifier" / data-validation "Rule 2 —
// continuity"; design.md D-4 "same cadence-audit mechanism").
//
// D3 adjudication (orchestrator, settled): a cadence segment DECLARES its
// own cadence (Cadence field, a source-descriptive string -- "semiannual"
// for the historical population span, never a domain Frequency), while
// storage and period arithmetic stay on the series' base quarterly grid,
// filtered by Present. No indicators.Frequency variant is added; Period's
// arithmetic (stepsPerYear, Next, Prev, the label parser) is unchanged.

import (
	"strings"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
)

func mustCadencePeriod(t *testing.T, raw string) indicators.Period {
	t.Helper()
	p, err := indicators.NormalizePeriodLabel(raw)
	if err != nil {
		t.Fatalf("NormalizePeriodLabel(%q): %v", raw, err)
	}
	return p
}

func quarterlyRun(t *testing.T, fromYear, fromQ, toYear, toQ int) []indicators.Period {
	t.Helper()
	var out []indicators.Period
	cur := indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: fromYear, Ordinal: fromQ}
	end := indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: toYear, Ordinal: toQ}
	for {
		out = append(out, cur)
		if cur.Equal(end) {
			break
		}
		cur = cur.Next()
	}
	return out
}

// semiannualRun returns only Q1/Q3 (ordinals 1 and 3) for every year in
// [fromYear, toYear] -- the exact real-world shape verified for ECP320's
// historical span (1 de enero de / 1 de julio de).
func semiannualRun(fromYear, toYear int) []indicators.Period {
	var out []indicators.Period
	for y := fromYear; y <= toYear; y++ {
		out = append(out,
			indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: y, Ordinal: 1},
			indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: y, Ordinal: 3},
		)
	}
	return out
}

func TestAssertCadence_DenseUniformSeriesPasses(t *testing.T) {
	periods := quarterlyRun(t, 2018, 1, 2026, 2)
	if err := indicators.AssertCadence(periods, indicators.FrequencyQuarterly, nil); err != nil {
		t.Fatalf("expected a dense, uninterrupted quarterly series to pass, got: %v", err)
	}
}

func TestAssertCadence_SingleIsolatedGapIsTolerated(t *testing.T) {
	// A long, otherwise dense run with exactly one period removed from the
	// interior -- this is Rule2Continuity's job to flag (data-validation
	// "A skipped period fails continuity"), not a cadence-declaration
	// mismatch. AssertCadence must not fail the whole run over one gap.
	full := quarterlyRun(t, 2015, 1, 2026, 2)
	var withGap []indicators.Period
	for _, p := range full {
		if p.Year == 2020 && p.Ordinal == 3 {
			continue // remove one interior period
		}
		withGap = append(withGap, p)
	}
	if err := indicators.AssertCadence(withGap, indicators.FrequencyQuarterly, nil); err != nil {
		t.Fatalf("expected a single isolated gap to be tolerated by the cadence audit, got: %v", err)
	}
}

func TestAssertCadence_UniformDeclarationFailsAgainstMixedHistory(t *testing.T) {
	// spec source-ingestion-ine: "A single quarterly declaration fails
	// against a mixed history" -- a long semiannual historical span
	// followed by a recent quarterly span, declared as uniformly quarterly
	// (no cadence_segments), must fail.
	var periods []indicators.Period
	periods = append(periods, semiannualRun(1977, 2022)...)
	periods = append(periods, quarterlyRun(t, 2023, 3, 2026, 2)...)

	err := indicators.AssertCadence(periods, indicators.FrequencyQuarterly, nil)
	if err == nil {
		t.Fatal("expected the uniform quarterly declaration to fail against a mixed semiannual/quarterly history")
	}
	if !strings.Contains(err.Error(), "quarterly") {
		t.Errorf("expected the failure to name the declared cadence, got: %v", err)
	}
}

func TestAssertCadence_DeclaredSegmentedCadenceMatchingPayloadProceeds(t *testing.T) {
	// spec source-ingestion-ine: "A declared segmented cadence matching
	// the payload proceeds".
	var periods []indicators.Period
	periods = append(periods, semiannualRun(1977, 2022)...)
	periods = append(periods, quarterlyRun(t, 2023, 3, 2026, 2)...)

	segments := []indicators.CadenceSegment{
		{
			From: mustCadencePeriod(t, "1977-Q1"), To: mustCadencePeriod(t, "2023-Q2"),
			Cadence: "semiannual", Present: []int{1, 3},
		},
		{
			From: mustCadencePeriod(t, "2023-Q3"), OpenEnded: true,
			Cadence: "quarterly",
		},
	}

	if err := indicators.AssertCadence(periods, indicators.FrequencyQuarterly, segments); err != nil {
		t.Fatalf("expected a declared segmented cadence matching the payload to proceed, got: %v", err)
	}
}

func TestAssertCadence_InventedOrdinalInsideRestrictedSegmentFails(t *testing.T) {
	// spec source-ingestion-ine "The population series stores its real
	// cadence": "no invented Q2 or Q4 period exists" in the semiannual
	// span. A Q2 observation inside a segment declaring Present [Q1,Q3]
	// must fail -- it violates the segment's own declared cadence, not
	// merely its density.
	periods := semiannualRun(1977, 1990)
	periods = append(periods, indicators.Period{Frequency: indicators.FrequencyQuarterly, Year: 1985, Ordinal: 2})

	segments := []indicators.CadenceSegment{
		{From: mustCadencePeriod(t, "1977-Q1"), OpenEnded: true, Cadence: "semiannual", Present: []int{1, 3}},
	}

	err := indicators.AssertCadence(periods, indicators.FrequencyQuarterly, segments)
	if err == nil {
		t.Fatal("expected an invented Q2 inside a semiannual segment to fail")
	}
	if !strings.Contains(err.Error(), "1985-Q2") {
		t.Errorf("expected the failure to name the offending period 1985-Q2, got: %v", err)
	}
}

func TestAssertCadence_PeriodOutsideAnyDeclaredSegmentFails(t *testing.T) {
	segments := []indicators.CadenceSegment{
		{From: mustCadencePeriod(t, "2023-Q3"), OpenEnded: true, Cadence: "quarterly"},
	}
	periods := []indicators.Period{mustCadencePeriod(t, "2020-Q1")}

	err := indicators.AssertCadence(periods, indicators.FrequencyQuarterly, segments)
	if err == nil {
		t.Fatal("expected a period before every declared segment's range to fail")
	}
	if !strings.Contains(err.Error(), "2020-Q1") {
		t.Errorf("expected the failure to name the uncovered period 2020-Q1, got: %v", err)
	}
}

func TestAssertCadence_EmptyPayloadPasses(t *testing.T) {
	if err := indicators.AssertCadence(nil, indicators.FrequencyQuarterly, nil); err != nil {
		t.Fatalf("expected an empty payload to pass (SilentEmpty is a different, earlier check), got: %v", err)
	}
}

func TestParseCadenceSegment(t *testing.T) {
	seg, err := indicators.ParseCadenceSegment("1977-Q1", "2023-Q2", "semiannual", []int{1, 3})
	if err != nil {
		t.Fatalf("ParseCadenceSegment: %v", err)
	}
	if seg.OpenEnded {
		t.Error("expected a segment with a non-empty \"to\" to not be open-ended")
	}
	if seg.From.String() != "1977-Q1" || seg.To.String() != "2023-Q2" {
		t.Errorf("expected From=1977-Q1 To=2023-Q2, got From=%s To=%s", seg.From, seg.To)
	}
	if seg.Cadence != "semiannual" {
		t.Errorf("expected Cadence=semiannual, got %q", seg.Cadence)
	}

	open, err := indicators.ParseCadenceSegment("2023-Q3", "", "quarterly", nil)
	if err != nil {
		t.Fatalf("ParseCadenceSegment (open-ended): %v", err)
	}
	if !open.OpenEnded {
		t.Error("expected a segment with an empty \"to\" to be open-ended")
	}

	if _, err := indicators.ParseCadenceSegment("not-a-period", "", "quarterly", nil); err == nil {
		t.Fatal("expected an unparseable \"from\" label to error")
	}
}

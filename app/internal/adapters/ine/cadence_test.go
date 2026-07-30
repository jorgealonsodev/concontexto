package ine_test

// Task 1.1 (RED) / 1.2 (GREEN): periodicity classification MUST read the
// WHOLE payload, not Data[0] alone (spec source-ingestion-ine,
// "Periodicity MUST be detected over the whole payload, not from a
// single observation"), and a series MAY declare cadence_segments so a
// mixed-cadence history (ECP320's real shape: semiannual historically,
// quarterly recently) can be represented and audited honestly instead of
// being silently declared uniform.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/ine"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

func assertSchemaDrift(t *testing.T, err error) {
	t.Helper()
	var classified *sourceerr.Error
	if !errors.As(err, &classified) {
		t.Fatalf("expected a classified *sourceerr.Error, got %T: %v", err, err)
	}
	if classified.Class != sourceerr.SchemaDrift {
		t.Errorf("expected class %s, got %s", sourceerr.SchemaDrift, classified.Class)
	}
}

// wireRow renders one DATOS_SERIE Data[] entry.
func wireRow(anyo int, periodo string, valor float64) string {
	return fmt.Sprintf(`{"Anyo": %d, "T3_Periodo": %q, "Valor": %v}`, anyo, periodo, valor)
}

func wireBody(rows []string) string {
	return fmt.Sprintf(`{"Nombre": "Total Nacional", "Data": [%s]}`, strings.Join(rows, ","))
}

// semiannualWireRows renders "1 de enero de"/"1 de julio de" rows
// (ECP320's own live-verified quarter-start-date shape) for every year in
// [fromYear, toYear].
func semiannualWireRows(fromYear, toYear int) []string {
	var out []string
	for y := fromYear; y <= toYear; y++ {
		out = append(out, wireRow(y, "1 de enero de", 40000000+float64(y)),
			wireRow(y, "1 de julio de", 40000000+float64(y)))
	}
	return out
}

// quarterlyWireRows renders ordinary "T1".."T4" rows from (fromYear,fromQ)
// to (toYear,toQ) inclusive.
func quarterlyWireRows(fromYear, fromQ, toYear, toQ int) []string {
	var out []string
	y, q := fromYear, fromQ
	for {
		out = append(out, wireRow(y, fmt.Sprintf("T%d", q), 49000000+float64(y)))
		if y == toYear && q == toQ {
			break
		}
		q++
		if q > 4 {
			q = 1
			y++
		}
	}
	return out
}

func TestDecodeSeries_PeriodicityReadsWholePayloadNotJustFirstRow(t *testing.T) {
	// The FIRST row is quarterly-shaped (matches expectedFrequency), but a
	// later row is annual-shaped -- a Data[0]-only check would miss this
	// entirely (the historical Fase 0 defect this task fixes).
	rows := []string{
		wireRow(2024, "T1", 10.0),
		wireRow(2024, "T2", 10.1),
		wireRow(2025, "A", 22221.1), // shape mismatch buried past index 0
	}
	body := wireBody(rows)

	_, err := ine.DecodeSeries([]byte(body), "test-whole-payload", indicators.FrequencyQuarterly)
	if err == nil {
		t.Fatal("expected a periodicity mismatch buried past Data[0] to still be caught")
	}
	assertSchemaDrift(t, err)
}

func TestDecodeSeries_UniformDeclarationFailsAgainstMixedHistory(t *testing.T) {
	var rows []string
	rows = append(rows, semiannualWireRows(1977, 2022)...)
	rows = append(rows, quarterlyWireRows(2023, 3, 2026, 2)...)
	body := wireBody(rows)

	// No cadence segments passed: a uniformly-quarterly declaration.
	_, err := ine.DecodeSeries([]byte(body), "test-mixed-cadence-series", indicators.FrequencyQuarterly)
	if err == nil {
		t.Fatal("expected a uniform quarterly declaration to fail against a mixed semiannual/quarterly history")
	}
	assertSchemaDrift(t, err)
}

func TestDecodeSeries_DeclaredSegmentedCadenceMatchingPayloadProceeds(t *testing.T) {
	var rows []string
	rows = append(rows, semiannualWireRows(1977, 2022)...)
	rows = append(rows, quarterlyWireRows(2023, 3, 2026, 2)...)
	body := wireBody(rows)

	segments := mustSegments(t,
		segSpec{from: "1977-Q1", to: "2023-Q2", cadence: "semiannual", present: []int{1, 3}},
		segSpec{from: "2023-Q3", cadence: "quarterly"},
	)

	result, err := ine.DecodeSeries([]byte(body), "test-mixed-cadence-series", indicators.FrequencyQuarterly, segments...)
	if err != nil {
		t.Fatalf("expected a declared segmented cadence matching the payload to proceed, got: %v", err)
	}
	if len(result.Observations) != len(rows) {
		t.Fatalf("expected %d observations, got %d", len(rows), len(result.Observations))
	}
}

func TestDecodeSeries_InventedOrdinalInsideSemiannualSegmentFails(t *testing.T) {
	rows := semiannualWireRows(1977, 1990)
	rows = append(rows, wireRow(1985, "T2", 40000000)) // invented Q2 inside a semiannual segment
	body := wireBody(rows)

	segments := mustSegments(t, segSpec{from: "1977-Q1", cadence: "semiannual", present: []int{1, 3}})

	_, err := ine.DecodeSeries([]byte(body), "test-mixed-cadence-series", indicators.FrequencyQuarterly, segments...)
	if err == nil {
		t.Fatal("expected an invented Q2 inside a declared semiannual segment to fail")
	}
	assertSchemaDrift(t, err)
}

type segSpec struct {
	from, to, cadence string
	present           []int
}

func mustSegments(t *testing.T, specs ...segSpec) []indicators.CadenceSegment {
	t.Helper()
	out := make([]indicators.CadenceSegment, 0, len(specs))
	for _, s := range specs {
		seg, err := indicators.ParseCadenceSegment(s.from, s.to, s.cadence, s.present)
		if err != nil {
			t.Fatalf("ParseCadenceSegment(%+v): %v", s, err)
		}
		out = append(out, seg)
	}
	return out
}

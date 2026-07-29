package xlsx_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/xlsx"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

// realFixtureSchema is the exact structural declaration
// config/series/afiliacion-ss.yaml carries for the checked-in real
// fixture (task 8.1's verified anchors, corrected total column -- see
// decode.go's package doc comment). Building it here as a Go value
// (rather than loading the YAML) keeps this test focused on Decode's own
// contract; loader_test.go/validate_test.go already prove the YAML
// parses into this same shape.
func realFixtureSchema() config.XLSXSchemaConfig {
	return config.XLSXSchemaConfig{
		SheetName:         "Hoja1",
		HeaderRow:         3,
		ColumnAnchors:     map[string]string{"period": "A", "total": "N"},
		HeaderFingerprint: "ignored-by-decode-itself", // Decode COMPUTES this; Rule1Schema compares it
		TotalColumn:       "N",
		ComponentColumns:  []string{"B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M"},
		// Tolerance 5.0, not the package's own defaultTolerance (1.0):
		// the exhaustive 306-row sweep (this correction batch) found one
		// row -- Julio 2013 -- whose declared total and component sum
		// differ by 3.03 (source rounding on a 16.4-million-unit figure).
		// 5.0 comfortably absorbs that verified maximum (every other row
		// is within 0.06) while staying orders of magnitude below the
		// effect of a genuinely shifted or missing column.
		Tolerance: 5.0,
	}
}

func readRealFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "afiliacion-ss", "afiliacion-ss.xlsx"))
	if err != nil {
		t.Fatalf("reading real fixture: %v", err)
	}
	return raw
}

// TestDecode_HappyPath is task 8.8/8.9's proof: the real, trimmed
// workbook fixture parses into canonical observations with full
// provenance, and every applicable rule 1 fact (sheet, header row,
// fingerprint) is observed. Values are asserted with a small epsilon
// because the source itself carries float64 rounding noise (task 8.1's
// verification: up to ~0.05 across twelve summed columns).
func TestDecode_HappyPath(t *testing.T) {
	raw := readRealFixture(t)
	result, err := xlsx.Decode(raw, realFixtureSchema(), indicators.FrequencyMonthly)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if len(result.Observations) != 306 {
		t.Fatalf("expected 306 observations (the full real history; the footnote rows must not produce any), got %d: %+v", len(result.Observations), result.Observations)
	}

	for i := 0; i < len(result.Observations)-1; i++ {
		if !result.Observations[i].Period.Before(result.Observations[i+1].Period) {
			t.Errorf("observation %d (%s) is not strictly before observation %d (%s)",
				i, result.Observations[i].Period, i+1, result.Observations[i+1].Period)
		}
	}

	first := result.Observations[0]
	if first.Period.String() != "2001-01" {
		t.Errorf("expected the first observation to be 2001-01, got %s", first.Period)
	}
	assertApprox(t, "2001-01", first.Value, 15194299.22, 0.1)

	last := result.Observations[len(result.Observations)-1]
	if last.Period.String() != "2026-06" {
		t.Errorf("expected the last observation to be 2026-06, got %s", last.Period)
	}
	// Verified live 2026-07-28 (task 8.1): 22,466,338.86 is the real
	// Junio 2026 system total, independently reconfirmed here via the
	// arithmetic invariant this same Decode call already enforced.
	assertApprox(t, "2026-06", last.Value, 22466338.86, 0.1)

	if result.ObservedSchema.SheetName != "Hoja1" {
		t.Errorf("ObservedSchema.SheetName = %q, want Hoja1", result.ObservedSchema.SheetName)
	}
	if result.ObservedSchema.HeaderRow != 3 {
		t.Errorf("ObservedSchema.HeaderRow = %d, want 3", result.ObservedSchema.HeaderRow)
	}
	if result.ObservedSchema.HeaderFingerprint == "" {
		t.Error("expected a non-empty computed header fingerprint")
	}
}

// TestDecode_FullHistoryEveryRowSatisfiesArithmeticInvariant is this
// correction batch's mandated regression test: "ingest the FULL 306-row
// history and assert every row's invariant holds — not a sampled row."
//
// Background (disclosed at length in decode.go's package doc comment):
// two prior investigations (Engram #4699, then PR 8a's own correction)
// each sampled ONE row, generalised, and were each wrong about which
// column carries the total. A THIRD claim (Engram #4709, forwarded by
// this batch's own prompt) asserted the total column itself moves across
// five overlapping eras (N/M/L/J/K). Direct, exhaustive re-verification
// against this exact sha256-matched fixture (every one of the 306 real
// data rows, not a sample) REFUTES that claim: column N is the maximum
// of B..N, and therefore the total, in literally all 306 rows — matching
// the workbook's OWN header row 2, which labels N2 "TOTAL SISTEMA" for
// the whole table. Columns J/K/L/M/C/D are not alternating TOTAL pins;
// they are régimen-membership sub-columns that come and go across the
// 2008/2012 restructures (footnotes "(9) Extinguido 1-enero-2008", "(2)
// Vigente desde 1-enero-2012") — a COMPONENT-set change, not a
// total-column change. Decode already handles this correctly because
// blank component cells contribute zero to the sum regardless of era.
//
// The one genuine defect this sweep caught: Julio 2013 (row 154)'s
// declared total and the sum of its components differ by 3.03 (source
// rounding on a 16.4-million-unit figure), which the previously
// committed Tolerance=1.0 rejected as schema drift. Confirmed FAILING
// against Tolerance=1.0 before this batch widened it — see apply-progress
// for the RED transcript. This test is what would have caught BOTH the
// original K/B-I error and this tolerance defect, because it checks
// every row instead of one.
func TestDecode_FullHistoryEveryRowSatisfiesArithmeticInvariant(t *testing.T) {
	raw := readRealFixture(t)
	result, err := xlsx.Decode(raw, realFixtureSchema(), indicators.FrequencyMonthly)
	if err != nil {
		t.Fatalf("Decode over the full 306-row real history: %v (every row's arithmetic invariant must hold within the declared tolerance)", err)
	}
	if len(result.Observations) != 306 {
		t.Fatalf("expected all 306 monthly rows (Enero 2001 - Junio 2026) to decode, got %d", len(result.Observations))
	}
	if got := result.Observations[0].Period.String(); got != "2001-01" {
		t.Errorf("first observation = %s, want 2001-01", got)
	}
	if got := result.Observations[len(result.Observations)-1].Period.String(); got != "2026-06" {
		t.Errorf("last observation = %s, want 2026-06", got)
	}
	for i := 1; i < len(result.Observations); i++ {
		if result.Observations[i-1].Period.Next() != result.Observations[i].Period {
			t.Errorf("expected a continuous unbroken monthly run: %s is not immediately followed by %s",
				result.Observations[i-1].Period, result.Observations[i].Period)
		}
	}
}

// TestDecode_HeaderFingerprintStableAcrossTwoRuns is task 8.1's trap #3
// (trailing CRLF/LF in header cells, e.g. "REGIMEN GENERAL\n"): decoding
// the exact same unchanged bytes twice MUST yield the exact same
// fingerprint, or every unchanged run would falsely alert as drift.
func TestDecode_HeaderFingerprintStableAcrossTwoRuns(t *testing.T) {
	raw := readRealFixture(t)
	r1, err := xlsx.Decode(raw, realFixtureSchema(), indicators.FrequencyMonthly)
	if err != nil {
		t.Fatalf("Decode (run 1): %v", err)
	}
	r2, err := xlsx.Decode(raw, realFixtureSchema(), indicators.FrequencyMonthly)
	if err != nil {
		t.Fatalf("Decode (run 2): %v", err)
	}
	if r1.ObservedSchema.HeaderFingerprint != r2.ObservedSchema.HeaderFingerprint {
		t.Errorf("fingerprint not stable across runs: %q vs %q", r1.ObservedSchema.HeaderFingerprint, r2.ObservedSchema.HeaderFingerprint)
	}
}

// TestDecode_SelectorIndependenceRealFixtureHasNoFormulaCells is task
// 8.6/8.7's proof, per the orchestrator's explicit framing: the
// selector-dependent-cell hazard is real (it is the OTHER Social
// Security workbook, the 25-sheet SAS-generated one with an INDICE
// sheet), but it is avoided by SOURCE SELECTION, not defended against in
// code. This test makes that a falsifiable property of the checked-in
// fixture rather than an assumption: every cell this package actually
// reads (the period column and every total/component column, across
// every data row) carries no formula at all, so there is no cached,
// selector-dependent value Decode could possibly be fooled by.
func TestDecode_SelectorIndependenceRealFixtureHasNoFormulaCells(t *testing.T) {
	raw := readRealFixture(t)
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()

	schema := realFixtureSchema()
	columns := append([]string{schema.ColumnAnchors["period"], schema.TotalColumn}, schema.ComponentColumns...)
	rows, err := f.GetRows(schema.SheetName)
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}

	var formulaCells []string
	for row := schema.HeaderRow + 1; row <= len(rows); row++ {
		for _, col := range columns {
			ref := col + strconv.Itoa(row)
			formula, err := f.GetCellFormula(schema.SheetName, ref)
			if err != nil {
				t.Fatalf("GetCellFormula(%s): %v", ref, err)
			}
			if formula != "" {
				formulaCells = append(formulaCells, ref)
			}
		}
	}
	if len(formulaCells) != 0 {
		t.Fatalf("expected ZERO formula cells among the columns this package reads (that is what makes selector-independence true for THIS source), found: %v", formulaCells)
	}
}

// TestDecode_UnresolvablePeriodFailsAndWritesNothing is the other half
// of the selector-independence requirement: "if the requested period
// cannot be resolved from underlying data, the run fails and writes
// nothing" (spec source-ingestion-xlsx). A synthetic workbook with one
// garbage period label mid-table proves Decode returns an error and a
// zero-value result (nothing a caller could mistake for a partial
// success).
func TestDecode_UnresolvablePeriodFailsAndWritesNothing(t *testing.T) {
	raw := buildWorkbook(t, "Hoja1", [][]cell{
		{str("Periodo"), str("Total")}, // header row 1 for this synthetic sheet
		{str("Enero 2001"), num(100)},
		{str("not a period at all"), num(200)}, // unresolvable, and NOT footnote-shaped
	})
	schema := config.XLSXSchemaConfig{
		SheetName:        "Hoja1",
		HeaderRow:        1,
		ColumnAnchors:    map[string]string{"period": "A"},
		TotalColumn:      "B",
		ComponentColumns: []string{"B"},
		Tolerance:        0.01,
	}
	result, err := xlsx.Decode(raw, schema, indicators.FrequencyMonthly)
	if err == nil {
		t.Fatal("expected an error for the unresolvable period row")
	}
	if result.Observations != nil {
		t.Errorf("expected zero observations on failure, got %+v", result.Observations)
	}
}

// TestDecode_ArithmeticInvariantFailureIsSchemaDrift is the central
// guard task 8.1 requires: a shifted/missing component column breaks the
// invariant, and the failure is classified as schema drift, not an
// ordinary data error, and blocks the whole decode (spec
// source-ingestion-xlsx, "A shifted column breaks the invariant and
// blocks publication").
func TestDecode_ArithmeticInvariantFailureIsSchemaDrift(t *testing.T) {
	raw := buildWorkbook(t, "Hoja1", [][]cell{
		{str("Periodo"), str("A"), str("B"), str("Total")},
		{str("Enero 2001"), num(10), num(20), num(999)}, // 10+20=30, declared total column says 999
	})
	schema := config.XLSXSchemaConfig{
		SheetName:        "Hoja1",
		HeaderRow:        1,
		ColumnAnchors:    map[string]string{"period": "A"},
		TotalColumn:      "D",
		ComponentColumns: []string{"B", "C"},
		Tolerance:        0.01,
	}
	_, err := xlsx.Decode(raw, schema, indicators.FrequencyMonthly)
	if err == nil {
		t.Fatal("expected the arithmetic invariant to fail")
	}
	var classified *sourceerr.Error
	if !errors.As(err, &classified) {
		t.Fatalf("expected a classified *sourceerr.Error, got %T: %v", err, err)
	}
	if classified.Class != sourceerr.SchemaDrift {
		t.Errorf("expected sourceerr.SchemaDrift (a shifted column is schema drift, not a data error), got %s", classified.Class)
	}
}

// TestDecode_ConfigOnlyColumnAnchorChange is task 8.4's own scenario,
// word for word: "GIVEN a workbook whose value column moves to a
// different position WHEN only the column anchor in the series
// configuration is updated THEN ingestion succeeds with no change to Go
// source." Two synthetic workbooks place the SAME logical total in
// DIFFERENT columns (C vs D); only the config's TotalColumn/
// ComponentColumns differ between the two Decode calls -- the Go source
// under test (decode.go) is identical for both.
func TestDecode_ConfigOnlyColumnAnchorChange(t *testing.T) {
	totalInC := buildWorkbook(t, "Hoja1", [][]cell{
		{str("Periodo"), str("Componente"), str("Total")},
		{str("Enero 2001"), num(40), num(40)},
	})
	schemaC := config.XLSXSchemaConfig{
		SheetName: "Hoja1", HeaderRow: 1,
		ColumnAnchors:    map[string]string{"period": "A"},
		TotalColumn:      "C",
		ComponentColumns: []string{"B"},
		Tolerance:        0.01,
	}

	totalInD := buildWorkbook(t, "Hoja1", [][]cell{
		{str("Periodo"), str("Componente"), str("Relleno"), str("Total")},
		{str("Enero 2001"), num(40), str(""), num(40)}, // the SAME total, one column further right
	})
	schemaD := config.XLSXSchemaConfig{
		SheetName: "Hoja1", HeaderRow: 1,
		ColumnAnchors:    map[string]string{"period": "A"},
		TotalColumn:      "D", // ONLY this configuration changed
		ComponentColumns: []string{"B"},
		Tolerance:        0.01,
	}

	resultC, err := xlsx.Decode(totalInC, schemaC, indicators.FrequencyMonthly)
	if err != nil {
		t.Fatalf("Decode(total in column C): %v", err)
	}
	resultD, err := xlsx.Decode(totalInD, schemaD, indicators.FrequencyMonthly)
	if err != nil {
		t.Fatalf("Decode(total moved to column D, config-only change): %v", err)
	}

	if len(resultC.Observations) != 1 || len(resultD.Observations) != 1 {
		t.Fatalf("expected exactly 1 observation from each workbook, got %d and %d", len(resultC.Observations), len(resultD.Observations))
	}
	if *resultC.Observations[0].Value != *resultD.Observations[0].Value {
		t.Errorf("expected both workbooks to yield the same value (40) once configuration follows the moved column: got %v vs %v",
			*resultC.Observations[0].Value, *resultD.Observations[0].Value)
	}
}

// --- test helpers -----------------------------------------------------

func assertApprox(t *testing.T, label string, got *float64, want, epsilon float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: expected a non-nil value", label)
	}
	diff := *got - want
	if diff < 0 {
		diff = -diff
	}
	if diff > epsilon {
		t.Errorf("%s: got %.4f, want %.4f (±%.4f)", label, *got, want, epsilon)
	}
}

type cell struct {
	s     string
	f     float64
	isStr bool
}

func str(s string) cell  { return cell{s: s, isStr: true} }
func num(f float64) cell { return cell{f: f} }

// buildWorkbook builds a minimal in-memory xlsx file for a synthetic
// test case (never a real published identifier -- app/internal/guard's
// origin-identifier guard only forbids specific real INE/Eurostat/XLSX
// identifiers, and this data is neither), so tests 8.4 and the
// arithmetic-invariant failure do not depend on the checked-in real
// fixture at all.
func buildWorkbook(t *testing.T, sheet string, rows [][]cell) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", sheet)
	for r, row := range rows {
		for c, v := range row {
			ref, err := excelize.CoordinatesToCellName(c+1, r+1)
			if err != nil {
				t.Fatalf("CoordinatesToCellName: %v", err)
			}
			if v.isStr {
				if v.s == "" {
					continue // leave the cell genuinely blank
				}
				if err := f.SetCellStr(sheet, ref, v.s); err != nil {
					t.Fatalf("SetCellStr(%s): %v", ref, err)
				}
				continue
			}
			if err := f.SetCellFloat(sheet, ref, v.f, -1, 64); err != nil {
				t.Fatalf("SetCellFloat(%s): %v", ref, err)
			}
		}
	}
	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		t.Fatalf("writing synthetic workbook: %v", err)
	}
	return buf.Bytes()
}

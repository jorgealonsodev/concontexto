package xlsx_test

// Task 8.10/8.11: the malformed-file suite (spec source-ingestion-xlsx,
// "Every malformed workbook writes nothing and preserves the published
// datum"). Every case below proves Decode itself rejects the malformed
// payload -- validate-before-write, the FIRST half of the all-or-nothing
// contract: a caller (app/internal/ingestion.IngestSeries) can only
// reach its decode-failure branch (a synthetic all-Block finding, nil
// candidates -- see ingest.go) when Decode returns a non-nil error, so
// every case proven to fail HERE is, by construction, a case that writes
// zero observations and leaves the previously published datum untouched
// once run through the real pipeline.
//
// The remaining half of the required assertion -- "the repository
// receives zero write calls" and "the published datum is unchanged" --
// is proven generically, once, for every case that reaches Decode
// failure or a validation Block (both funnel through the exact same
// ApplyGate code path): app/internal/adapters/postgres/gate_spy_test.go
// (TestApplyGate_BlockNeverCallsTheObservationWriter, a spy proving the
// INTERACTION property) and app/internal/ingestion/malformed_xlsx_test.go
// (ONE real-Postgres, real-fixture, end-to-end test for the sharpest
// case -- partial success within one workbook -- proving the STATE
// property against the real schema, per the orchestrator's explicit
// design decision: "use both, because the spec asks for two different
// things"). Re-running all nine cases against a full testcontainers
// stack here would only re-prove that same structural fact nine times.
//
// Fixtures are built in Go (buildWorkbook, already established by
// decode_test.go for exactly this purpose), not checked in as opaque
// binary files: the Go source IS the fixture recipe, which is MORE
// legible to a reviewer than a binary blob plus prose describing it, and
// matches this package's own existing convention (see
// TestDecode_ArithmeticInvariantFailureIsSchemaDrift,
// TestDecode_UnresolvablePeriodFailsAndWritesNothing). testdata/README.md
// documents the whole suite as an index for reviewers, per this batch's
// scope.

import (
	"bytes"
	"errors"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/adapters/xlsx"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

// malformedSuiteSchema is the small, fixed schema every malformed-suite
// fixture in this file is built against: period column A, two component
// columns B/C, total column D. Deliberately NOT the real 306-row/12-
// component afiliacion-ss schema (decode_test.go's realFixtureSchema) --
// the malformed suite's fixtures are synthetic and minimal by design
// (batch instruction: "keep the checked-in fixtures small"), and using a
// small schema keeps every fixture's construction obvious by inspection.
func malformedSuiteSchema() config.XLSXSchemaConfig {
	return config.XLSXSchemaConfig{
		SheetName:         "Hoja1",
		HeaderRow:         1,
		ColumnAnchors:     map[string]string{"period": "A", "total": "D"},
		HeaderFingerprint: "ignored-by-decode-itself",
		TotalColumn:       "D",
		ComponentColumns:  []string{"B", "C"},
		Tolerance:         0.01,
	}
}

// assertSchemaDriftError requires err to be a classified
// *sourceerr.Error with Class==SchemaDrift -- every malformed case in
// this suite is exactly that: a shape problem, not an ordinary data
// value out of range (spec's own framing, "the failure is classified as
// schema drift, not a data error").
func assertSchemaDriftError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	var classified *sourceerr.Error
	if !errors.As(err, &classified) {
		t.Fatalf("expected a classified *sourceerr.Error, got %T: %v", err, err)
	}
	if classified.Class != sourceerr.SchemaDrift {
		t.Errorf("expected sourceerr.SchemaDrift, got %s", classified.Class)
	}
}

// assertNoPanic runs fn and fails the test (rather than crashing the
// whole test binary) if it panics -- the required assertion for the two
// unreadable-file cases (spec "the failure is reported without
// panicking").
func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked: %v", r)
		}
	}()
	fn()
}

// TestDecode_MalformedFileSuite is the table-driven core of task 8.10:
// every listed malformed-file case (spec's own minimum list) makes
// Decode fail, never partially succeed. See the package doc comment
// above for how this connects to the "zero writes / unchanged datum"
// requirement.
func TestDecode_MalformedFileSuite(t *testing.T) {
	cases := []struct {
		name        string
		raw         []byte
		wantNoPanic bool
	}{
		{
			// The publisher renamed the total column's header text AND
			// repurposed the cell (its value no longer sums to the
			// components). Decode never consults header text for column
			// mapping (spec "Header text is never consulted for column
			// mapping") -- header renames are only visible to Decode
			// through their effect on the arithmetic invariant, which is
			// exactly what this fixture exercises, per the orchestrator's
			// explicit steer for this batch.
			name: "header renamed (value column relabelled and repurposed, breaking the arithmetic invariant)",
			raw: buildWorkbook(t, "Hoja1", [][]cell{
				{str("Periodo"), str("ComponenteUno"), str("ComponenteDos"), str("Total (renombrado)")},
				{str("Enero 2001"), num(10), num(20), num(999)}, // 10+20=30, declared total says 999
			}),
		},
		{
			// A column was inserted immediately before the declared total
			// column, shifting every column from the insertion point
			// rightward -- literally the spec's own scenario ("one
			// component column has been inserted before the total
			// column"). The value now sitting at D no longer holds the
			// true total.
			name: "header moved (a column inserted before the total column shifts the layout)",
			raw:  buildColumnInsertedBeforeTotal(t),
		},
		{
			name: "sheet missing (workbook has no sheet named Hoja1)",
			raw: buildWorkbook(t, "SheetOtroNombre", [][]cell{
				{str("Periodo"), str("ComponenteUno"), str("ComponenteDos"), str("Total")},
				{str("Enero 2001"), num(10), num(20), num(30)},
			}),
		},
		{
			name: "text in a value cell (the total column holds free text, not a number)",
			raw: buildWorkbook(t, "Hoja1", [][]cell{
				{str("Periodo"), str("ComponenteUno"), str("ComponenteDos"), str("Total")},
				{str("Enero 2001"), num(10), num(20), str("N/D")},
			}),
		},
		{
			name: "#REF! in a value cell (a component column holds a broken-reference error value)",
			raw: buildWorkbook(t, "Hoja1", [][]cell{
				{str("Periodo"), str("ComponenteUno"), str("ComponenteDos"), str("Total")},
				{str("Enero 2001"), str("#REF!"), num(20), num(30)},
			}),
		},
		{
			name: "formula cell with no cached value (total column is a live formula, never opened in Excel to compute it)",
			raw:  buildTotalAsUncachedFormula(t),
		},
		{
			name:        "truncated file (a valid workbook's byte stream is cut short mid-archive)",
			raw:         truncatedWorkbook(t),
			wantNoPanic: true,
		},
		{
			name:        "not a valid zip archive (plain bytes, no zip structure at all)",
			raw:         []byte("this is not a workbook, just plain garbage bytes with no zip signature"),
			wantNoPanic: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var result indicators.SourceResult
			var err error
			run := func() { result, err = xlsx.Decode(tc.raw, malformedSuiteSchema(), indicators.FrequencyMonthly) }
			if tc.wantNoPanic {
				assertNoPanic(t, run)
			} else {
				run()
			}

			if err == nil {
				t.Fatalf("expected Decode to fail, got a result with %d observation(s)", len(result.Observations))
			}
			if len(result.Observations) != 0 {
				t.Errorf("expected zero observations on failure, got %d", len(result.Observations))
			}

			var classified *sourceerr.Error
			if errors.As(err, &classified) {
				if classified.Class != sourceerr.SchemaDrift {
					t.Errorf("expected sourceerr.SchemaDrift, got %s", classified.Class)
				}
			} else {
				t.Errorf("expected a classified *sourceerr.Error, got %T: %v", err, err)
			}
		})
	}
}

// TestDecode_PartialSuccessWithinOneWorkbookWritesNothing is task 8.10's
// "sharpest case" (batch instructions): two rows parse perfectly fine,
// a third is malformed (text in the total column) -- proving Decode
// discards the two good rows too, rather than returning a partial
// result. A caller that published the two good rows anyway would look
// successful while quietly shipping an incomplete month -- "worse than a
// failed one, because it looks successful" (batch instructions).
func TestDecode_PartialSuccessWithinOneWorkbookWritesNothing(t *testing.T) {
	raw := buildWorkbook(t, "Hoja1", [][]cell{
		{str("Periodo"), str("ComponenteUno"), str("ComponenteDos"), str("Total")},
		{str("Enero 2001"), num(10), num(20), num(30)},    // valid
		{str("Febrero 2001"), num(11), num(21), num(32)},  // valid
		{str("Marzo 2001"), num(12), num(22), str("N/D")}, // malformed: text in the total column
	})

	result, err := xlsx.Decode(raw, malformedSuiteSchema(), indicators.FrequencyMonthly)
	if err == nil {
		t.Fatal("expected Decode to fail on the malformed third row")
	}
	if len(result.Observations) != 0 {
		t.Fatalf("expected ZERO observations -- not the two valid rows that parsed before the malformed one -- got %d: %+v",
			len(result.Observations), result.Observations)
	}
	assertSchemaDriftError(t, err)
}

// --- fixture builders ---------------------------------------------------

// buildColumnInsertedBeforeTotal builds a valid small workbook (period,
// two components, total in D), then inserts a new column immediately
// before D -- exactly the spec's own "shifted column" scenario. The
// value that used to sum correctly at D now sits at E; D holds whatever
// InsertCols leaves behind (a blank cell here), so the declared total
// column no longer equals the sum of the declared component columns.
func buildColumnInsertedBeforeTotal(t *testing.T) []byte {
	t.Helper()
	raw := buildWorkbook(t, "Hoja1", [][]cell{
		{str("Periodo"), str("ComponenteUno"), str("ComponenteDos"), str("Total")},
		{str("Enero 2001"), num(10), num(20), num(30)},
	})
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("opening workbook to insert a column: %v", err)
	}
	defer f.Close()
	if err := f.InsertCols("Hoja1", "D", 1); err != nil {
		t.Fatalf("InsertCols: %v", err)
	}
	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		t.Fatalf("writing workbook after column insertion: %v", err)
	}
	return buf.Bytes()
}

// buildTotalAsUncachedFormula builds a workbook whose total column is a
// live SUM formula with no cached <v> value -- exactly what a workbook
// looks like when the publisher edited it with a tool that never opened
// it in a spreadsheet application to compute the formula (or a
// programmatic export that only ever writes formulas). readCellFloat's
// RawCellValue read sees an empty string for such a cell.
func buildTotalAsUncachedFormula(t *testing.T) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", "Hoja1")
	if err := f.SetCellStr("Hoja1", "A1", "Periodo"); err != nil {
		t.Fatalf("SetCellStr: %v", err)
	}
	if err := f.SetCellStr("Hoja1", "B1", "ComponenteUno"); err != nil {
		t.Fatalf("SetCellStr: %v", err)
	}
	if err := f.SetCellStr("Hoja1", "C1", "ComponenteDos"); err != nil {
		t.Fatalf("SetCellStr: %v", err)
	}
	if err := f.SetCellStr("Hoja1", "D1", "Total"); err != nil {
		t.Fatalf("SetCellStr: %v", err)
	}
	if err := f.SetCellStr("Hoja1", "A2", "Enero 2001"); err != nil {
		t.Fatalf("SetCellStr: %v", err)
	}
	if err := f.SetCellFloat("Hoja1", "B2", 10, -1, 64); err != nil {
		t.Fatalf("SetCellFloat: %v", err)
	}
	if err := f.SetCellFloat("Hoja1", "C2", 20, -1, 64); err != nil {
		t.Fatalf("SetCellFloat: %v", err)
	}
	// SetCellFormula ONLY sets the formula string, never a cached <v> --
	// that is precisely the malformed condition under test.
	if err := f.SetCellFormula("Hoja1", "D2", "SUM(B2:C2)"); err != nil {
		t.Fatalf("SetCellFormula: %v", err)
	}
	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		t.Fatalf("writing workbook: %v", err)
	}
	return buf.Bytes()
}

// truncatedWorkbook builds a valid small workbook, then cuts it roughly
// in half -- destroying the zip's central directory so excelize.OpenReader
// fails cleanly instead of reading a structurally broken file.
func truncatedWorkbook(t *testing.T) []byte {
	t.Helper()
	raw := buildWorkbook(t, "Hoja1", [][]cell{
		{str("Periodo"), str("ComponenteUno"), str("ComponenteDos"), str("Total")},
		{str("Enero 2001"), num(10), num(20), num(30)},
	})
	return raw[:len(raw)/2]
}

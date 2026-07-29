// Package xlsx is the SourceClient (indicators.SourceClient) for
// spreadsheet-published sources, first built for the Social Security
// affiliation series (milestone 0.6, spec source-ingestion-xlsx). It
// reuses the same domain types, observation writer and validation
// harness as adapters/ine and adapters/eurostat — only the transport and
// decode differ, exactly like eurostat's own package doc comment states
// for itself.
//
// Task 8.1's resolution (Engram #4699, corrected during implementation —
// see the note below): the Social Security publishes two very different
// artifacts side by side. The annual workbook is a 25-sheet, SAS-generated
// file with an INDICE selector sheet and 12,826 formula cells — the
// selector-dependent hazard the spec anticipated, and it is real. This
// package's decode path is built and tested ONLY against the OTHER
// artifact: the flat monthly series workbook, one sheet, ZERO formula
// cells, no external links. That choice — made in configuration
// (series/afiliacion-ss.yaml's source_refs, never in Go) — is what makes
// "the parser targets underlying data, not a selector-dependent view"
// true for this source: there is no cached selector state to be fooled
// by, because the ingested file has no formulas at all (see decode_test.go's
// TestDecode_SelectorIndependence... for the falsifiable proof).
//
// CORRECTION during implementation (disclosed, not silent — see this
// batch's apply-progress): the investigation record (Engram #4699) stated
// the workbook's total column is K. Direct verification with excelize
// against the real, sha256-matched fixture (both via GetRows and,
// independently, via GetCellValue on explicit cell references) instead
// found the total in column N; K's row-3 label ("Discontínuos (7)") is a
// genuine, separate régimen sub-category whose data is blank in every row
// sampled. The arithmetic invariant only holds when TotalColumn=N and
// ComponentColumns spans the FULL régimen breakdown, columns B through M
// — not merely B-I, which is exact only by coincidence for rows where the
// pre-2012 régimen columns (J-M) happen to be empty. Verified across two
// widely separated rows (Enero 2001 and Junio 2026): both sum to their
// row's column-N value within floating-point rounding. See
// series/afiliacion-ss.yaml for the corrected, checked-in values.
//
// SECOND CORRECTION, this batch (disclosed, not silent): a follow-up
// investigation record (Engram #4709) claimed the ABOVE correction was
// itself incomplete — that the total column does not stay pinned at N,
// but moves across five overlapping eras (N/M/L/J/K), and proposed a
// validity-ranged (total_column, component_columns) mapping structurally
// like series_source_mapping to handle the churn.
//
// That claim was investigated exhaustively against this exact
// sha256-matched fixture — all 306 real data rows, not a sampled row —
// and is REFUTED by direct evidence:
//   - Header row 2 (one header, shared by the whole table) labels N2
//     "TOTAL SISTEMA" outright. There is no per-era header; one label
//     covers 2001-2026.
//   - Computing max(B..N) per row independently, with no assumption
//     about which column "should" be the total, finds column N is the
//     maximum in literally all 306 rows.
//   - The specific rows Engram #4709's era table would require to have a
//     populated K/J/L/M total contradict it directly: e.g. Mayo 2017
//     (claimed inside the "K era") has K, J, L and M ALL blank that row;
//     N alone carries the value.
//   - What actually varies across eras is the COMPONENT set, not the
//     total: J/K/L/M (pre-2012 régimen split) and C/D (post-2012 Sistema
//     Especial split) are populated in different, non-overlapping date
//     ranges (footnotes "(9) Extinguido 1-enero-2008", "(2) Vigente desde
//     1-enero-2012") while N stays the constant total throughout. Decode
//     already handles this correctly with NO era logic at all, because a
//     blank component cell contributes zero to the sum regardless of
//     which era it belongs to.
//
// The one genuine defect the exhaustive sweep DID find: Julio 2013 (row
// 154) differs from its declared total by 3.03 (rounding on a
// 16.4-million-unit figure), which the previously committed Tolerance=1.0
// rejected as schema drift. That is the actual fix this batch makes --
// widening Tolerance to 5.0 (series/afiliacion-ss.yaml) -- not a
// validity-ranged column mapping, which would add real complexity to
// solve a churn problem this workbook does not have. See
// decode_test.go's TestDecode_FullHistoryEveryRowSatisfiesArithmeticInvariant,
// which checks every row instead of a sample, so this class of error is
// caught by CI rather than by a third round of manual re-investigation.
package xlsx

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/jorgealonsodev/concontexto/app/internal/adapters/config"
	"github.com/jorgealonsodev/concontexto/app/internal/indicators"
	"github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr"
)

// defaultTolerance backs config.XLSXSchemaConfig.Tolerance when a series
// leaves it at the zero value: the real workbook's own displayed
// precision (2 decimals) accumulated over up to twelve summed columns
// showed at most a few hundredths of a unit of rounding drift (task 8.1's
// verification); 1.0 comfortably absorbs that while still catching a
// genuinely shifted or missing column, whose effect is orders of
// magnitude larger (a whole régimen's worth of affiliates).
const defaultTolerance = 1.0

// anchorPeriod/anchorTotal are the two config.ColumnAnchors keys this
// package reads. They are declared here, not as Go literals scattered
// through the file, so decode.go and its tests share one spelling.
const (
	anchorPeriod = "period"
	anchorTotal  = "total"
)

// Decode parses raw XLSX bytes into canonical observations according to
// schema (structure lives in configuration, never in Go — PRD §9.4, spec
// source-ingestion-xlsx "Workbook structure is declared in configuration,
// never in code"). expectedFrequency is asserted against the first parsed
// observation's period, mirroring adapters/ine and adapters/eurostat.
//
// Every column this function reads (the period column, the total column,
// every component column) is addressed purely by the LETTER declared in
// schema — nothing here special-cases "column N" or any other position
// as a Go literal, which is what makes task 8.4's config-only-structure
// scenario true (see decode_test.go).
func Decode(raw []byte, schema config.XLSXSchemaConfig, expectedFrequency indicators.Frequency) (indicators.SourceResult, error) {
	periodAnchor := schema.ColumnAnchors[anchorPeriod]
	if periodAnchor == "" {
		return indicators.SourceResult{}, fmt.Errorf("xlsx: schema.column_anchors has no %q entry", anchorPeriod)
	}
	if schema.TotalColumn == "" || len(schema.ComponentColumns) == 0 {
		return indicators.SourceResult{}, fmt.Errorf("xlsx: schema has no total_column/component_columns declared (the arithmetic-invariant guard cannot run)")
	}
	if schema.HeaderRow <= 0 {
		return indicators.SourceResult{}, fmt.Errorf("xlsx: schema has no header_row declared")
	}

	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return indicators.SourceResult{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf("xlsx: opening workbook: %v", err))
	}
	defer f.Close()

	sheet := schema.SheetName
	if !hasSheet(f, sheet) {
		return indicators.SourceResult{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf("xlsx: workbook has no sheet %q", sheet))
	}

	headerColumns := append([]string{periodAnchor, schema.TotalColumn}, schema.ComponentColumns...)
	headerCells, err := readRowCells(f, sheet, schema.HeaderRow, headerColumns)
	if err != nil {
		return indicators.SourceResult{}, fmt.Errorf("xlsx: reading header row %d: %w", schema.HeaderRow, err)
	}

	tolerance := schema.Tolerance
	if tolerance <= 0 {
		tolerance = defaultTolerance
	}

	var observations []indicators.Observation
	for row := schema.HeaderRow + 1; ; row++ {
		periodCell, err := f.GetCellValue(sheet, cellRef(periodAnchor, row))
		if err != nil {
			return indicators.SourceResult{}, fmt.Errorf("xlsx: reading %s: %w", cellRef(periodAnchor, row), err)
		}
		trimmed := strings.TrimSpace(periodCell)
		if trimmed == "" {
			break // ran off the end of the data range
		}
		if isFootnoteRow(trimmed) {
			break // spec's verified terminator: footnote rows follow the last data row
		}

		period, err := parsePeriodoLabel(trimmed)
		if err != nil {
			// Column A no longer matches a recognised period label and is
			// not a footnote either -- an unresolvable period. The run
			// fails and writes nothing rather than guess (spec "The
			// parser targets underlying data ... if the requested period
			// cannot be resolved ... the run fails and writes nothing").
			return indicators.SourceResult{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf("xlsx: row %d: %v", row, err))
		}

		total, present, err := readCellFloat(f, sheet, schema.TotalColumn, row)
		if err != nil {
			return indicators.SourceResult{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf("xlsx: row %d: reading total column %s: %v", row, schema.TotalColumn, err))
		}
		if !present {
			return indicators.SourceResult{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf("xlsx: row %d (%s): total column %s is empty", row, trimmed, schema.TotalColumn))
		}

		sum := 0.0
		for _, col := range schema.ComponentColumns {
			v, componentPresent, err := readCellFloat(f, sheet, col, row)
			if err != nil {
				return indicators.SourceResult{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf("xlsx: row %d: reading component column %s: %v", row, col, err))
			}
			if componentPresent {
				sum += v
			}
		}

		if diff := math.Abs(total - sum); diff > tolerance {
			return indicators.SourceResult{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf(
				"xlsx: row %d (%s): arithmetic invariant failed: total column %s=%.4f, sum of component columns %v=%.4f (diff %.4f exceeds tolerance %.4f) -- the workbook's column layout may have shifted",
				row, trimmed, schema.TotalColumn, total, schema.ComponentColumns, sum, diff, tolerance))
		}

		value := total
		observations = append(observations, indicators.Observation{Period: period, Value: &value})
	}

	if len(observations) == 0 {
		return indicators.SourceResult{}, sourceerr.New(sourceerr.SilentEmpty, "xlsx: workbook has zero data rows")
	}
	if actual := observations[0].Period.Frequency; actual != expectedFrequency {
		return indicators.SourceResult{}, sourceerr.New(sourceerr.SchemaDrift, fmt.Sprintf("xlsx: periodicity mismatch: expected %s, got %s", expectedFrequency, actual))
	}

	observedAnchors := map[string]string{anchorPeriod: periodAnchor, "total": schema.TotalColumn}
	return indicators.SourceResult{
		Name:         sheet,
		Observations: observations,
		ObservedSchema: indicators.ObservedSchema{
			SheetName:         sheet,
			HeaderRow:         schema.HeaderRow,
			HeaderFingerprint: computeFingerprint(headerCells),
			ColumnAnchors:     observedAnchors,
		},
	}, nil
}

// hasSheet reports whether sheet is one of f's sheets.
func hasSheet(f *excelize.File, sheet string) bool {
	for _, name := range f.GetSheetList() {
		if name == sheet {
			return true
		}
	}
	return false
}

// cellRef builds an A1-style cell reference from a column letter and a
// 1-indexed row number, e.g. cellRef("N", 309) == "N309".
func cellRef(col string, row int) string {
	return fmt.Sprintf("%s%d", col, row)
}

// readRowCells reads the given columns' values at row row, in the
// caller's order -- used to build the header fingerprint over exactly
// the columns this package consumes (period, total, every component),
// not the whole header row.
func readRowCells(f *excelize.File, sheet string, row int, columns []string) ([]string, error) {
	out := make([]string, len(columns))
	for i, col := range columns {
		v, err := f.GetCellValue(sheet, cellRef(col, row))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", cellRef(col, row), err)
		}
		out[i] = v
	}
	return out, nil
}

// readCellFloat reads sheet!col{row} with excelize's RawCellValue option
// (the underlying numeric value, e.g. "22466338.863636", never the
// display-formatted "22,466,338.86" GetCellValue's default would return)
// and parses it. present is false for a blank cell -- callers decide
// whether that means "treat as zero" (a component column) or "this row
// is unreadable" (the total column).
func readCellFloat(f *excelize.File, sheet, col string, row int) (value float64, present bool, err error) {
	raw, err := f.GetCellValue(sheet, cellRef(col, row), excelize.Options{RawCellValue: true})
	if err != nil {
		return 0, false, err
	}
	if strings.TrimSpace(raw) == "" {
		return 0, false, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false, fmt.Errorf("value %q is not numeric: %w", raw, err)
	}
	return v, true, nil
}

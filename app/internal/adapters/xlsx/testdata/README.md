# testdata — package xlsx

## `afiliacion-ss/` — the real happy-path fixture

`afiliacion-ss.xlsx` is the real, unmodified Social Security workbook
(`19_Serie afiliación media por regímenes (Total Sistema).xlsx`), checked in
byte-identical to the published source. `source.txt` in that directory
records its origin URL, fetch date and sha256, and the full structural
history (including the two corrected investigation records) is in
`app/internal/adapters/xlsx/decode.go`'s package doc comment and
`config/series/afiliacion-ss.yaml`'s own comments.

## The malformed-file suite (tasks 8.10/8.11)

The malformed-file suite (spec `source-ingestion-xlsx`, "Every malformed
workbook writes nothing and preserves the published datum") does **not**
check in binary fixture files. Every malformed workbook is built as Go
source, in `malformed_test.go` (package `xlsx`) and
`app/internal/ingestion/malformed_xlsx_test.go`, using the same
`excelize`-based construction the package already used for synthetic
fixtures before this batch (see `decode_test.go`'s `buildWorkbook`,
`TestDecode_ArithmeticInvariantFailureIsSchemaDrift`,
`TestDecode_UnresolvablePeriodFailsAndWritesNothing`).

This is a deliberate choice, not an oversight: the instruction behind
this file is "a reviewer must be able to read what each file is broken
in and why without opening it in Excel." A few lines of Go that build a
2x4 workbook cell-by-cell satisfy that more directly than a checked-in
binary blob plus prose describing it — the Go source **is** the fixture
recipe, and it is reviewed the same way as any other test change (`git
diff`), with no extra tooling. This index exists so a reviewer can find
each case without reading the whole test file line by line.

Every case's required assertion (per spec) is not merely "an error is
returned": the repository must receive **zero write calls**, and the
**previously published datum for `(series, period)` must be unchanged at
its original version**. How that split across three test files is
explained in `malformed_test.go`'s own package doc comment; in short:

- **Decode-level failure** (this file's cases): proven for each case
  individually, offline, no Docker.
- **Interaction property** ("the writer is never called"): proven once,
  generically, in `app/internal/adapters/postgres/gate_spy_test.go`
  (`TestApplyGate_BlockNeverCallsTheObservationWriter`) — every case
  below reaches the exact same `ApplyGate` Block branch, so this is
  complete proof for all of them, not just the one case that also gets
  an end-to-end test.
- **State property** ("the published datum is unchanged, at its original
  version"): proven once, end-to-end, against a real Postgres, for the
  sharpest case (partial success) in
  `app/internal/ingestion/malformed_xlsx_test.go`.

### Cases (all in `malformed_test.go`'s `TestDecode_MalformedFileSuite`
### table, unless noted)

| Case | What is broken | How it is built | Why `Decode` rejects it |
|---|---|---|---|
| Header renamed | The total column's header TEXT is renamed, and — because `Decode` never trusts header text for column mapping (spec: "Header text is never consulted for column mapping") — the fixture ALSO repurposes that cell's numeric value so it no longer sums to the components. | `buildWorkbook`: 1 header row + 1 data row; `D1` header text changed; `D2` set to `999` while `B2+C2=30`. | Arithmetic invariant: declared total (999) ≠ sum of declared components (30) → `sourceerr.SchemaDrift`. |
| Header moved | A column is inserted immediately before the declared total column (spec's own scenario: "one component column has been inserted before the total column"), shifting the true total one column to the right. | `buildColumnInsertedBeforeTotal`: builds a valid workbook, then calls `excelize.InsertCols("Hoja1", "D", 1)`. | The declared total column (`D`) is now empty/wrong after the shift → arithmetic invariant fails → `SchemaDrift`. |
| Sheet missing | The workbook has no sheet named `Hoja1` (only a differently-named sheet). | `buildWorkbook("SheetOtroNombre", ...)`. | `Decode`'s own `hasSheet` check fails first, naming the missing sheet → `SchemaDrift`. |
| Text in a value cell | The total column holds free text (`"N/D"`) instead of a number. | `buildWorkbook` with `str("N/D")` in the total cell. | `readCellFloat`'s `strconv.ParseFloat` fails on the total column → `SchemaDrift`. |
| `#REF!` in a value cell | A component column holds the literal Excel broken-reference error text `#REF!`. | `buildWorkbook` with `str("#REF!")` in a component cell. | Same numeric-parse failure path, on a component column this time → `SchemaDrift`. |
| Formula cell with no cached value | The total column is a live formula (`SUM(B2:C2)`) with no cached `<v>` result — exactly what a workbook looks like when it was generated/edited by a tool that never opened it in a spreadsheet application to compute the formula. | `buildTotalAsUncachedFormula`: `SetCellFormula` only, deliberately never `SetCellValue`. | `RawCellValue` reads an empty string for the cell → treated as "total column is empty" → `SchemaDrift`. |
| Truncated file | A valid workbook's byte stream is cut roughly in half, destroying the zip's central directory. | `truncatedWorkbook`: `raw[:len(raw)/2]` on a valid built workbook. | `excelize.OpenReader` fails to open a broken zip → `SchemaDrift`; test also asserts no panic. |
| Not a valid zip archive | Plain ASCII bytes with no zip signature at all. | A literal `[]byte(...)` string constant. | Same `excelize.OpenReader` failure path; test also asserts no panic. |
| Partial success within one workbook | Two data rows parse perfectly (valid period, valid arithmetic); a third row has text in its total column. | `TestDecode_PartialSuccessWithinOneWorkbookWritesNothing` (its own test function, not the shared table): 3 data rows, third malformed. | `Decode`'s row loop returns the error immediately on the malformed row; the two already-accumulated valid observations are discarded, never returned — this is what makes the whole decode all-or-nothing, "not partially written" (the sharpest case, batch instructions). Reproduced end-to-end against a real Postgres in `app/internal/ingestion/malformed_xlsx_test.go`. |

None of these fixtures use a real, forbidden origin identifier (see
`app/internal/guard/originidentifiers_test.go`) — every period label,
value and identifier is synthetic test data.

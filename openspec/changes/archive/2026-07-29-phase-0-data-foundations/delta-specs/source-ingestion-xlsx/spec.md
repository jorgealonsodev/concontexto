# Delta for source-ingestion-xlsx

Slice 8 · Milestone 0.6 (Social Security affiliation). Greenfield capability — no existing spec to modify.

**Open item RESOLVED 2026-07-28.** The real files were downloaded and inspected. The Social Security publishes two very different artifacts side by side on the "Afiliados medios TOTALES" page, and the choice between them is the central decision of this capability.

| | `19_Serie afiliación media por regímenes (Total Sistema).xlsx` | `Afiliación 2026_CNAE25.xlsx` |
|---|---|---|
| Size | 54 KB | 3.3 MB |
| Sheets | 1 (`Hoja1`) | 25, including an `INDICE` selector sheet |
| Formula cells | **0** | **12,826** |
| Producer | — | SAS Add-in for Microsoft Office |
| Coverage | 306 monthly observations, Enero 2001 → Junio 2026 | one year |

The INDEX-selector risk this specification originally anticipated is REAL — it is the annual workbook. It is also entirely avoidable: the series file is a flat table with zero formulas, so no cached-selector value can be read by mistake.

**DECIDED: milestone 0.6 ingests the series file**, `19_Serie afiliación media por regímenes (Total Sistema).xlsx`. The annual workbook MUST NOT be the ingestion source for the headline affiliation series.

Verified structure, to be recorded in `series/afiliacion-ss.yaml`:

- Sheet `Hoja1` (the only sheet). Row 1 title, row 2 régimen group header, **row 3 column headers, data rows 4–309**.
- Period column `A`, format `<MesEspañol> <YYYY>` (`Enero 2001` … `Junio 2026`).
- Data terminator: footnote rows begin at row 310, matching `^\(\d+\)` in column A. Parsing MUST stop when column A no longer matches the month-year pattern.
- Column `N` carries the system total — in every one of the 306 data rows, not a subset — matching the workbook's own row-2 header, which labels `N2` "TOTAL SISTEMA". Columns `B` through `M` are the component columns; a blank component cell is treated as zero. Which of `B`–`M` are actually populated changes across two restructures (footnotes "(9) Extinguido 1-enero-2008", "(2) Vigente desde 1-enero-2012"), but this is a component-set change, not a total-column change — `N` never moves.
- Row 3 mislabels column `K` "Discontínuos (7)"; it is a genuine, separate régimen sub-category, not the total, despite the header/data-column misalignment described below.
- Defined name `NumDias` = `Hoja1!$Q$2`, an auxiliary cell outside the print area `Hoja1!$A$4:$O$319`.

**Investigation history (disclosed in full because two earlier records were wrong):** an early investigation record claimed the total column is `K` with components `B`–`I`; a later one corrected this to `N` with components `B`–`M`; a further record claimed `N` itself is not stable and that the total column moves across five overlapping eras (`N`/`M`/`L`/`J`/`K`), proposing a validity-ranged column mapping. That third claim was investigated exhaustively against the real, sha256-verified fixture — all 306 data rows, not a sampled row — and is refuted: `N` is the maximum of `B`..`N` (and therefore the total) in literally every row, matching the workbook's own uniform header label. `total_column: N` / `component_columns: B`–`M` is a single, fixed, config-declared pin for the entire history, guarded by the arithmetic invariant below. The one genuine defect the exhaustive sweep found is a tolerance value, not a column mapping: Julio 2013 differs from its declared total by 3.03 (source rounding on a 16.4-million-unit figure), which requires `tolerance: 5.0`, not 1.0.

The download URL embeds a per-file UUID and a `CACHEID` token (a WebSphere content-management URL), so PRD §8.6's claim of a "URL estable" does NOT hold. The URL MUST live in `sources/seg-social.yaml` with a validity range, and the §9.4 synthetic probe is the early warning when it rotates.

## ADDED Requirements

### Requirement: Workbook structure is declared in configuration, never in code

Sheet name, header row index, column anchors, header fingerprint and the download URL MUST live in `series/{slug}.yaml` and MUST NOT be hard-coded in Go (PRD §9.4). The parser MUST read structure from configuration alone.

#### Scenario: Structure changes without touching code

- GIVEN a workbook whose value column moves to a different position
- WHEN only the column anchor in the series configuration is updated
- THEN ingestion succeeds with no change to Go source

#### Scenario: A configuration missing workbook structure fails validate-config

- GIVEN an XLSX-backed series configuration with no sheet name or header fingerprint
- WHEN `validate-config` runs
- THEN it exits non-zero naming the missing structure fields

### Requirement: The parser targets underlying data, not a selector-dependent view

The parser MUST read the period it was asked to ingest, independently of any interactive selector state cached in the workbook. It MUST NOT rely on cached formula results whose value depends on which month the publisher last selected.

#### Scenario: A selector-dependent cell is not the source of truth

- GIVEN a workbook whose visible summary cells reflect a month other than the requested one
- WHEN the requested month is ingested
- THEN the observation written corresponds to the requested month
- AND if the requested month cannot be resolved from underlying data, the run fails and writes nothing

### Requirement: Column mapping is pinned by position and guarded by an arithmetic invariant

Header text MUST NOT be used to map columns to series. In the verified workbook the header row does not align with the data columns: row 3 labels column `K` as `Discontínuos (7)`, a genuine, separate régimen sub-category — not the system total, which lives in column `N` for every data row. The offset is caused by merged cells in rows 2–3. A parser that mapped columns by header text could publish the wrong régimen's figures at a value plausible enough that no range check would catch it.

Therefore column mapping MUST be pinned by column letter in `series/afiliacion-ss.yaml` (a single, fixed pin for the whole history — the total column does not move across eras; only which component columns are populated changes, and blank components are treated as zero), and every ingestion MUST verify an arithmetic invariant declared alongside it: the value in the declared total column MUST equal the sum of the values in the declared component columns, within a declared tolerance. Failing the invariant MUST block publication and MUST be reported as schema drift, not as a data error.

This invariant doubles as the schema-drift detector for this source: if the publisher inserts, removes or reorders a column, the sum stops matching. It also MUST be verified against the source's full history, not a sampled row, before being relied on — see "A real monthly affiliation ingest passes validation" below.

#### Scenario: The declared total equals the sum of its components

- GIVEN `afiliacion-ss.yaml` declaring total column `N` and component columns `B` through `M`
- WHEN the row for `Junio 2026` is parsed
- THEN the value in `N` equals the sum of `B` through `M` within the declared tolerance
- AND the observation is accepted

#### Scenario: A shifted column breaks the invariant and blocks publication

- GIVEN a published current observation for `(series, period)` with value `V`
- AND a workbook in which one component column has been inserted before the total column
- WHEN ingestion runs
- THEN the arithmetic invariant fails
- AND the failure is classified as schema drift
- AND the repository receives zero write calls
- AND the current observation for `(series, period)` is still `V` at its original version

#### Scenario: Header text is never consulted for column mapping

- GIVEN a workbook whose header row labels the total column with an unrelated name
- WHEN ingestion runs against a configuration pinning the total column by letter
- THEN the correct column is read
- AND the run succeeds provided the arithmetic invariant holds

### Requirement: Period labels and header fingerprints are normalised before matching

The workbook contains inconsistent whitespace and embedded line endings that would defeat naive string comparison. Both were verified in the real file: twelve data rows carry a double space in the period label (`Febrero  2001` through `Febrero  2006`), and header cells carry trailing CRLF (`Régimen General (1) \r\n`, `REGIMEN GENERAL\r\n`).

Period parsing MUST collapse internal whitespace and trim before matching the month name. Header fingerprinting MUST normalise line endings and trim trailing whitespace, so that an unchanged workbook never reports a schema change.

#### Scenario: A double-spaced period label parses

- GIVEN a period cell containing `Febrero  2001` with two spaces
- WHEN it is parsed
- THEN it yields the canonical monthly period for February 2001

#### Scenario: A header fingerprint is stable across unchanged runs

- GIVEN header cells containing trailing CRLF
- WHEN the header fingerprint is computed on two consecutive runs of the same unchanged workbook
- THEN both fingerprints are equal
- AND no schema-change alert is raised

### Requirement: A real monthly affiliation ingest passes validation

A real monthly affiliation workbook MUST parse into canonical observations and pass every applicable validation rule, using the same domain types, writer and validation harness as the API-backed sources.

#### Scenario: The happy-path workbook ingests and validates

- GIVEN the real series workbook fixture `19_Serie afiliación media por regímenes (Total Sistema).xlsx`
- WHEN ingestion runs
- THEN observations are written with source, unit, frequency, extraction timestamp and raw-file hash
- AND every applicable validation rule passes

#### Scenario: The full monthly history loads

- GIVEN the same fixture
- WHEN a full historical ingestion runs
- THEN 306 monthly observations are written, spanning January 2001 through June 2026 inclusive
- AND no gap is reported by the continuity rule
- AND the footnote rows from row 310 onwards produce no observations

#### Scenario: The arithmetic invariant holds for every row in the full history, not a sample

- GIVEN the real series workbook fixture, checked in unmodified (not trimmed)
- WHEN ingestion parses all 306 data rows
- THEN every row's declared total equals the sum of its declared component columns within the declared tolerance
- AND this is asserted by decoding the full fixture and checking every resulting observation, not by inspecting one or a handful of sampled rows

### Requirement: Every malformed workbook writes nothing and preserves the published datum

For each malformed-file case the required assertion is NOT merely that an error is returned, but that **nothing was written AND the previously published datum is unchanged**. The suite MUST cover at least: header renamed, header moved, sheet missing, text in a value cell, `#REF!` in a value cell, formula cell with no cached value, truncated file, and a file that is not a valid zip archive.

Each crafted malformed fixture MUST be checked into the repository and described in `testdata/README.md`, because binary fixtures are opaque to reviewers.

#### Scenario: Header renamed — nothing written, published datum unchanged

- GIVEN a published current observation for `(series, period)` with value `V`
- WHEN a workbook whose value-column header has been renamed is ingested
- THEN the repository receives zero write calls
- AND the current observation for `(series, period)` is still `V` at its original version
- AND the run is recorded with a failed outcome and its raw-file hash

#### Scenario: Sheet missing — nothing written, published datum unchanged

- GIVEN the same published state
- WHEN a workbook missing the configured sheet is ingested
- THEN the repository receives zero write calls
- AND the current observation is unchanged

#### Scenario: Corrupt value cell — nothing written, published datum unchanged

- GIVEN the same published state
- WHEN a workbook containing text or `#REF!` in a value cell is ingested
- THEN the repository receives zero write calls
- AND the current observation is unchanged

#### Scenario: Unreadable file — nothing written, published datum unchanged

- GIVEN the same published state
- WHEN a truncated file or a file that is not a valid zip archive is ingested
- THEN the repository receives zero write calls
- AND the current observation is unchanged
- AND the failure is reported without panicking

#### Scenario: Partial success is not partially written

- GIVEN a workbook where some rows parse and one row is malformed
- WHEN ingestion runs
- THEN no observation from that workbook is written
- AND the current observations for every affected series are unchanged

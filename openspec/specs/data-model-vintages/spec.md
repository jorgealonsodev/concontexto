# Spec: data-model-vintages

Baseline capability specification — the source of truth for `data-model-vintages`.

Sources: PRD §9.5, §10; settled decisions D3 (Fase 0) and D3 (Fase 1).
Contributing changes: `phase-0-data-foundations` (archived 2026-07-29) — established this capability;
`phase-1-indicator-page` (archived 2026-08-05) — added the verbatim `source_status` annotation and its
version semantics.

`observation.status` stays P/D/W because PRD §6.1.1's reader-facing contract is binary and Eurostat's
`b`/`d` are not statuses; a nullable `source_status` preserves the source's own word because principle P2
wants the verbatim token.

## Requirements

### Requirement: Fase 0 migration set

The Fase 0 schema MUST contain exactly these tables: `source`, `dataset`, `series`, `series_source_mapping`, `observation`, `ingestion_run`, `raw_file`, `download_attempt`, `series_break`, `event`. It MUST NOT contain `indicator_page`, `verification`, `glossary` or `correction` (settled decision D3). The `event` group column MUST be named `event_group` because `group` is a SQL reserved word.

#### Scenario: Migrated schema matches the Fase 0 set

- GIVEN an empty PostgreSQL 17 database
- WHEN `migrate` runs to head
- THEN the ten tables above exist
- AND none of `indicator_page`, `verification`, `glossary`, `correction` exists

#### Scenario: Every migration is reversible

- GIVEN a database migrated to head
- WHEN every migration is rolled back one step at a time to zero
- THEN each `down` step succeeds
- AND the resulting schema is empty

### Requirement: Observations are immutable per version

An `observation` row MUST NOT be updated in place for its value. A revision from the source MUST insert a new row for the same `(series_id, period)` with `version = prior version + 1`. The current value is `MAX(version)` for that pair (PRD §9.5, §10).

#### Scenario: Simulated revision creates a new version without overwriting

- GIVEN an observation for `(series, period)` with `version = 1` and value `0.6`
- WHEN a later ingestion run reports value `0.7` for the same `(series, period)`
- THEN the row with `version = 1` still exists and still holds `0.6`
- AND a new row exists with `version = 2` and value `0.7`
- AND the current value resolved for that pair is `0.7`

#### Scenario: Unchanged value does not create a new version

- GIVEN an observation for `(series, period)` at `version = 1` with value `0.6`
- WHEN a later run reports the identical value and status
- THEN no new `observation` row is created
- AND the run is still recorded in `ingestion_run`

### Requirement: The database enforces exactly one current row

`observation` MUST carry `is_current boolean`, constrained by a partial unique index on `(series_id, period) WHERE is_current`. The invariant MUST be enforced by the database, not by application code.

#### Scenario: The database rejects a second current row

- GIVEN an existing current observation for `(series, period)`
- WHEN a second row for the same `(series, period)` is inserted with `is_current = true` without clearing the first
- THEN PostgreSQL rejects the insert with a unique-violation error

#### Scenario: Promotion moves the current flag atomically

- GIVEN a current observation at `version = 1`
- WHEN `version = 2` is written and promoted in one transaction
- THEN exactly one row for that pair has `is_current = true`
- AND that row is `version = 2`

### Requirement: Run-level vintage separated from per-period version

`ingestion_run(id, dataset_id, started_at, raw_file_hash, outcome)` MUST record each run. Every `observation` MUST reference its `ingestion_run_id`. The raw-file hash MUST be reachable through the run (`observation → ingestion_run.raw_file_hash → raw_file.hash`) rather than duplicated per observation row.

#### Scenario: Provenance is traceable from an observation to a raw file

- GIVEN a published observation
- WHEN its provenance is resolved
- THEN it yields source, origin series identifier, extraction timestamp, ingestion run and raw-file SHA-256
- AND every one of those fields is non-null

#### Scenario: Vintage as-of a date resolves to the right version

- GIVEN two runs for a series, one before and one after date `D`, each producing a version for the same period
- WHEN the value as of `D` is resolved
- THEN it is the maximum version among runs started at or before `D`

### Requirement: Source identifiers are validity-ranged mappings

`series_source_mapping` MUST map a canonical `series_id` to an origin reference with `ref_kind`, `ref`, `valid_from` and `valid_to`, so that an identifier change is recorded rather than overwritten (verified risk R1: both table Ids and series CODs churn).

#### Scenario: An identifier change adds a mapping instead of replacing one

- GIVEN a series mapped to origin ref `IPC251852` with an open validity range
- WHEN the source replaces it with `IPC290751`
- THEN the original mapping is closed with a `valid_to` date
- AND a new mapping row for `IPC290751` is added with a `valid_from` date
- AND both rows remain queryable

### Requirement: Source withdrawal is representable

`observation.status` MUST represent at least provisional, definitive and withdrawn. A source withdrawing a period MUST be recorded as a new version with withdrawn status, never as a delete.

#### Scenario: Withdrawn period is tombstoned, not deleted

- GIVEN a current observation for `(series, period)`
- WHEN the source stops publishing that period and the withdrawal is ingested
- THEN a new version is inserted with withdrawn status and becomes current
- AND the prior version row still exists with its original value

### Requirement: Bad-run rollback never deletes

Rolling back a bad ingestion run MUST be performed by re-pointing the current row to the prior version and recording the reason. It MUST NOT delete observations and MUST NOT delete or alter the raw file or its hash (principle P7).

#### Scenario: Rollback restores the prior current version

- GIVEN `version = 2` is current and was produced by a run later judged bad
- WHEN the run is rolled back
- THEN `version = 1` becomes current again
- AND `version = 2` still exists with its original value and a recorded rollback reason
- AND the raw file and its hash are unchanged

### Requirement: The source's verbatim status token is preserved

`observation` MUST carry a nullable `source_status text` column holding the source's own token verbatim —
INE's `T3_TipoDato` string, Eurostat's JSON-stat flag character — alongside the domain `status`. The
column MUST be nullable because Eurostat expresses definitive by the absence of a flag. `observation.status`
MUST remain the binary-plus-withdrawn domain enum and MUST NOT be widened to hold source vocabulary.

#### Scenario: The verbatim token round-trips

- GIVEN an INE observation whose source token is `"Provisional"`
- WHEN it is written and read back
- THEN its `source_status` is `"Provisional"`
- AND its domain `status` is provisional

#### Scenario: A null token is valid and means definitive for Eurostat

- GIVEN a Eurostat observation with no flag
- WHEN it is written and read back
- THEN its `source_status` is null
- AND its domain `status` is definitive

#### Scenario: The domain enum is unchanged

- GIVEN the `observation` schema
- WHEN the `status` domain is inspected
- THEN it still represents exactly provisional, definitive and withdrawn

### Requirement: The source_status migration is additive and reversible

The migration adding `source_status` MUST be additive and nullable, so existing rows remain valid, and
MUST ship a reversible `down` step. Reverting MUST lose only the annotation, never an observation.

#### Scenario: The migration applies to a populated database

- GIVEN a database populated with Fase 0 observations
- WHEN the migration runs
- THEN it succeeds
- AND every pre-existing observation row still exists with its original value and status
- AND `source_status` is null on those rows until the next ingestion

#### Scenario: The migration reverses without data loss

- GIVEN a database migrated to head
- WHEN the `source_status` migration is rolled back
- THEN the `down` step succeeds
- AND every observation row still exists with its original value, status and version

### Requirement: A status-only change appends a version

An ingestion that reports the same value with a different status for an existing `(series, period)` MUST
append a new version rather than update in place, and MUST NOT trip the deep-revision human-signoff block,
because no value changed.

#### Scenario: A provisional-to-definitive transition appends a version

- GIVEN a current observation at `version = 1` with value `0.6` and provisional status
- WHEN a later run reports value `0.6` with definitive status
- THEN a new row exists at `version = 2` with definitive status and value `0.6`
- AND `version = 1` still exists with its provisional status
- AND `version = 2` is current

#### Scenario: A status-only transition does not require human signoff

- GIVEN a status-only transition on a period older than the revision window
- WHEN validation runs
- THEN the deep-revision rule does not fail, because it compares values
- AND the run publishes without a human-review block

### Requirement: A backfill caused by our own defect is disclosed, not erased

Backfilling `source_status` over history that Fase 0 stored with a hardcoded status MUST create ordinary
new versions and MUST record, in the ingestion run, that the cause was a defect in this system rather
than a source revision (principle P7).

#### Scenario: The backfill is attributable

- GIVEN a backfill run correcting statuses that were hardcoded
- WHEN the resulting versions are inspected
- THEN each references an ingestion run whose recorded reason names the internal defect
- AND no prior version is deleted or altered

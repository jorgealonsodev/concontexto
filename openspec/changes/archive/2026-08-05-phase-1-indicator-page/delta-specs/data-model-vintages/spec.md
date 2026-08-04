# Delta for data-model-vintages

Slice **2** (prerequisite). Delta against `openspec/specs/data-model-vintages/spec.md`.

Settled decision D3: `observation.status` stays P/D/W because §6.1.1's reader-facing contract is binary
and Eurostat's `b`/`d` are not statuses; a nullable `source_status` preserves the source's own word
because principle P2 wants the verbatim token.

## ADDED Requirements

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

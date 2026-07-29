# Delta for raw-file-archive

Slice 5a · PRD §9.1, §14.2, principle P5. Greenfield capability — no existing spec to modify.

## ADDED Requirements

### Requirement: Raw downloads are stored immutably keyed by SHA-256

Every downloaded payload MUST be persisted to the `app_data` volume before parsing, keyed by its SHA-256, with the source, the request URL and a download timestamp. A stored raw file MUST NOT be modified or deleted by any pipeline path, including rollback.

#### Scenario: A download is archived before parsing

- GIVEN a successful download from a source
- WHEN ingestion proceeds
- THEN a `raw_file` row exists with the payload SHA-256, source, URL, `downloaded_at` and storage path
- AND the archive write happens before any parsing

#### Scenario: An identical redownload does not duplicate storage

- GIVEN a raw file already archived under hash `H`
- WHEN the same bytes are downloaded again
- THEN no second copy is stored
- AND a new `download_attempt` row records the attempt resolving to `H`

#### Scenario: Rollback preserves the raw file

- GIVEN a run rolled back as bad
- WHEN the rollback completes
- THEN the raw file and its hash are unchanged and still resolvable

### Requirement: Raw-file hashes are listed in the repository

The repository MUST publish a listing of archived raw-file hashes (PRD §14.2: raw files live in the `app_data` volume with their hash listed in the repo). The listing MUST be refreshed as part of the ingestion pipeline and MUST be sufficient to verify an archived file independently.

#### Scenario: The listing matches the archive

- GIVEN a completed ingestion run that archived a new raw file
- WHEN the published hash listing is compared with the archive contents
- THEN every archived file appears in the listing with its SHA-256, source and download timestamp
- AND recomputing the SHA-256 of the archived bytes reproduces the listed hash

Note: publication of derived CSV under `/data-derived` is deferred to Fase 1. Principle P5's public commitment binds at launch and Fase 0 does not launch; the raw-file hash listing above is the Fase 0 obligation.

### Requirement: Download attempts are logged independently of content

`download_attempt(source_id, url, attempted_at, resulting_hash, outcome)` MUST record every attempt, including attempts that succeed with unchanged content and attempts that fail, so that "we checked and nothing changed" is distinguishable from "we could not check".

#### Scenario: An unchanged check is recorded

- GIVEN a source whose payload is byte-identical to the last download
- WHEN the scheduled attempt completes
- THEN a `download_attempt` row is recorded with a success outcome and the existing hash
- AND no new `raw_file` row is created

#### Scenario: A failed attempt is recorded with no hash

- GIVEN a source that returns a transport error
- WHEN the attempt completes
- THEN a `download_attempt` row is recorded with a failure outcome and a null `resulting_hash`

### Requirement: Freshness state is tracked per source

The freshness state that drives the amber semaphore MUST be computed per source, not per series, because the ingestion job is per source. A source with no successful `download_attempt` for 24 hours MUST be recorded as failed at source level, and that state MUST propagate to every series belonging to that source's datasets (PRD §9.2, §6.1.1).

#### Scenario: Twenty-four hours without success marks the source stale

- GIVEN a source whose last successful `download_attempt` was 25 hours ago
- WHEN freshness is evaluated
- THEN the source is in the failed freshness state
- AND every series under that source's datasets resolves to amber

#### Scenario: Within the window the source stays fresh

- GIVEN a source with a successful attempt 3 hours ago
- WHEN freshness is evaluated
- THEN the source is fresh and no series resolves to amber

#### Scenario: One failing source does not affect another

- GIVEN source A stale for 25 hours and source B successful 1 hour ago
- WHEN freshness is evaluated
- THEN only series under source A resolve to amber

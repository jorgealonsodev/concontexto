# Spec for publishing-export

New capability. Slices **3** and **4**. Sources: ADR-7, PRD §14.2, §9.2, §19.3, principle P5.

The export artifact is a **versioned contract between two ecosystems**: the Go pipeline writes it, the
Astro build reads it. Fase 0 produced eight components that were built, tested and never connected. An
unvalidated hand-off between Go and Node is exactly that shape, so validation on both sides is a
requirement of this capability, not a quality-of-implementation detail.

## Requirements

### Requirement: Versioned build-time export artifact

The pipeline MUST produce a build-time export artifact from `app/internal/publishing`, reading published
data through existing driven ports. The artifact MUST declare an explicit `schema_version` and MUST
record the instant it was generated.

For every published series the artifact MUST carry: canonical slug, series metadata (source, statistical
operation, origin series identifier, unit, base, decimals, cadence, licence and attribution), the full
observation history (period, value, `status`, `source_status`, `version`, `ingestion_run_id`), the series
breaks that resolve for it, the editorial events and government entries that apply to it, its
source-relative freshness state, and the vintage identifier the artifact represents.

#### Scenario: The artifact declares its version and contents

- GIVEN a database holding the six published series
- WHEN the export runs
- THEN the artifact declares a `schema_version`
- AND it records the generation timestamp
- AND every field listed above is present and non-null for each of the six series

#### Scenario: Provenance survives the export

- GIVEN a published observation whose provenance resolves to source, origin identifier, extraction
  timestamp, ingestion run and raw-file SHA-256
- WHEN it is exported
- THEN every one of those fields is reachable from the artifact without a further database query

### Requirement: The artifact is validated on write

The export MUST validate the artifact against its declared schema before writing it. A validation failure
MUST abort the export, MUST NOT write a partial artifact, and MUST leave the previously exported artifact
intact and usable.

#### Scenario: An invalid artifact is never written

- GIVEN an export whose in-memory artifact violates the declared schema
- WHEN write-side validation runs
- THEN the export fails with an error naming the violated constraint
- AND no new artifact file exists
- AND the previous artifact is byte-identical to its pre-run state

### Requirement: The artifact is validated on read by the build

The Astro build MUST validate `schema_version` and artifact shape before rendering anything. An
unsupported version or a shape mismatch MUST fail the build loudly, naming the expected and the received
version or field. The build MUST NOT emit a partial site and MUST NOT silently skip a series it cannot
parse.

#### Scenario: An unsupported schema version fails the build

- GIVEN an artifact declaring a `schema_version` the build does not support
- WHEN the build starts
- THEN it exits non-zero naming both the expected and the received version
- AND no page output is produced

#### Scenario: A missing required field fails the build

- GIVEN an artifact whose series omits `source_status` on one observation
- WHEN read-side validation runs
- THEN the build fails naming the series, the period and the missing field
- AND no page output is produced

#### Scenario: A matching artifact builds

- GIVEN an artifact whose version and shape the build supports
- WHEN the build runs
- THEN validation passes and every configured page is rendered

### Requirement: Only published data enters the artifact

The artifact MUST contain, for each `(series, period)`, the current published value — the maximum version
among published runs. An observation produced by a run that failed validation MUST NOT appear. A series
whose latest run failed MUST still appear, carrying its last valid data.

#### Scenario: A failed run's suspect datum is absent

- GIVEN a series whose latest ingestion run failed validation
- WHEN the export runs
- THEN the suspect value is absent from the artifact
- AND the previously published value for that period is present
- AND the series carries the state that drives PRD §6.1.3's validation banner

### Requirement: Breaks and events have a read path

A read path resolving `series_break` and `event` for a series MUST exist. It MUST resolve
family-scoped breaks for every member series, MUST exclude soft-retired entries, and MUST exclude
editorial entries that carry no confirmed date.

#### Scenario: A family-scoped break resolves for a member series

- GIVEN a break scoped to a series family and a member series
- WHEN breaks are read for that member
- THEN the break is returned once
- AND a retired break is not returned

#### Scenario: An unconfirmed editorial entry is skipped without error

- GIVEN an editorial event entry whose date is unconfirmed and therefore not projected
- WHEN events are read for an affected series
- THEN the entry is absent from the result
- AND the read succeeds
- AND the export run records the count of unprojected entries for operators

### Requirement: `/data-derived` is generated from the same artifact

The export MUST write `/data-derived` as CSV in the same run and from the same in-memory artifact, so the
published CSV and the built site can never disagree (principle P5).

#### Scenario: CSV and artifact agree value by value

- GIVEN a completed export
- WHEN each `/data-derived` CSV row is compared with the artifact
- THEN every series, period, value and status matches exactly
- AND the CSV is regenerated whenever the artifact is

### Requirement: Freshness in the artifact is source-relative

The per-series freshness state carried by the artifact MUST be derived only from the source's expected
publication calendar and the newest observation held at export time. It MUST NOT encode any comparison
between the artifact and a later database state, because no such state exists when the artifact is
written.

#### Scenario: Freshness derives only from source-side facts

- GIVEN a series whose source has published its latest expected period and whose newest held observation
  is that period
- WHEN freshness is computed for the artifact
- THEN the state is fresh
- AND the computation reads only the source calendar and the held observations

#### Scenario: A pending source period yields the pending state

- GIVEN a series whose expected next period is past due at the source and is not held
- WHEN freshness is computed
- THEN the state is source-pending
- AND no field describing build staleness is emitted

### Requirement: The build has no database dependency

The Astro build MUST be able to complete with no database reachable and no database credentials present.
The artifact MUST be the build's only data input.

#### Scenario: The build completes with no database

- GIVEN a build environment with no database host, port or credentials configured
- WHEN the build runs against a valid artifact
- THEN it completes successfully and renders every page

### Requirement: An ingestion cycle exports what it learned, and only what it learned

A successful ingestion run MUST produce a new export artifact and MUST dispatch the site rebuild. The
dispatch MUST record the ingestion-success instant so end-to-end publish latency is measurable.

**A run that fails validation MUST ALSO produce a new artifact and dispatch a rebuild**, carrying the
series' last valid data and its recorded failure state. **A cycle that learned nothing — a fetch that
returned no payload, a run that never reached validation, a `nothing-new` outcome — MUST NOT export and
MUST NOT dispatch**, so the currently published artifact stays exactly as it is.

(Previously: the failed-run scenario read "THEN no new artifact is exported / AND no rebuild is
dispatched / AND the currently published site continues to be served", with no distinction between a
validation failure and a cycle that learned nothing. That phrasing was too broad and is **narrowed
here**. It contradicted this capability's own "A series whose latest run failed MUST still appear,
carrying its last valid data" and "AND the series carries the state that drives PRD §6.1.3's validation
banner" — neither of which is reachable unless the export runs — and it made `indicator-page`'s "The
three page states of PRD §6.1.3" undeliverable, because a new artifact is a reader's only route to that
banner. The requirement's intent is that **a suspect datum must never be published**, not that the
pipeline must go silent; the publish gate already guarantees the former by writing no observation at all
on a blocked run, so an export after a validation failure publishes exactly the last valid data plus the
honest fact that the latest run failed. Suppressing it instead served a stale artifact that silently
claimed everything was fine. See design.md's D-2 resolution for the full reasoning. Requirement renamed
from "A successful ingestion produces an export and dispatches a rebuild", which described only half of
what the gate must decide.)

#### Scenario: Ingestion success chains to a rebuild

- GIVEN an ingestion run that passes every validation rule
- WHEN it completes
- THEN a new artifact is exported
- AND a rebuild is dispatched
- AND the ingestion-success instant and the dispatch instant are both recorded

#### Scenario: A failed ingestion exports the failure state, never the suspect datum

- GIVEN a series with a previously published value, whose latest ingestion run fails validation
- WHEN it completes
- THEN a new artifact is exported and a rebuild is dispatched
- AND the suspect value is absent from the artifact
- AND the previously published value is the latest value the series carries
- AND the series' page state names the validation failure and the date of the last correct update
- AND the currently published site continues to be served until the rebuild replaces it

#### Scenario: A cycle that learned nothing exports nothing

- GIVEN an ingestion cycle in which no run published an observation and no run recorded a validation
  failure — every target failed to fetch, failed to resolve its source, or had nothing new to deliver
- WHEN it completes
- THEN no new artifact is exported
- AND no rebuild is dispatched
- AND the currently published artifact is unchanged

### Requirement: Artifacts are retained for rollback

The last N artifacts MUST be retained, with N configurable and at least 5. Rebuilding from a retained
artifact MUST reproduce the site that artifact originally produced.

#### Scenario: Rebuilding from a retained artifact reproduces the prior site

- GIVEN a retained artifact from a previous build and the same site source revision
- WHEN the site is rebuilt from it
- THEN the rendered output is byte-identical to the output that artifact originally produced
- AND no database action is required

### Requirement: One end-to-end ingest-then-build test exists

An automated test MUST ingest from recorded fixtures, export the artifact and build the site from it,
proving the two ecosystems actually meet.

#### Scenario: Ingest, export and build run as one test

- GIVEN recorded source fixtures for at least one series
- WHEN the end-to-end test runs
- THEN ingestion writes observations, the export produces a valid artifact, and the build renders that
  series' page from it
- AND the rendered page shows the value that was ingested

# Delta for editorial-config

Slices **1** and **7**. Delta against `openspec/specs/editorial-config/spec.md`.

File names `rupturas.yaml`, `eventos.yaml` and `gobiernos.yaml` are fixed by PRD §9.6 and stay Spanish, as
do editorial YAML keys and series slugs.

**Four-eyes disclosure, carried unchanged.** Correcting `config/series/poblacion-residente.yaml` is a
`/config/**` change. `platform-runtime`'s four-eyes requirement stands as written and is **not** modified,
weakened or substituted here. It remains **documented but unenforced on the remote**, because the second
code owner is still a placeholder and branch protection is not enabled. This delta adds no substitute
control; the gap stays disclosed until a second maintainer exists.

## ADDED Requirements

### Requirement: A series configuration expresses a cadence that changes over its life

`series/{slug}.yaml` MUST be able to declare a cadence as an ordered sequence of segments, each with its
own cadence and validity range, so a series whose real publication cadence changes is representable. A
single uniform cadence MUST remain expressible and MUST remain the common case. `validate-config` MUST
reject overlapping segments, gaps between segments, and a segment boundary that does not align to a valid
period of both adjoining cadences.

#### Scenario: A segmented cadence loads

- GIVEN a `series/{slug}.yaml` declaring a semiannual segment followed by a quarterly segment
- WHEN it is loaded
- THEN both segments resolve with their cadence and validity range
- AND the cadence applicable to any given period is unambiguous

#### Scenario: Overlapping segments are rejected

- GIVEN a configuration whose two cadence segments overlap
- WHEN `validate-config` runs
- THEN it exits non-zero naming the file and the overlapping segments
- AND CI fails

#### Scenario: A gap between segments is rejected

- GIVEN a configuration whose segments leave an uncovered span
- WHEN `validate-config` runs
- THEN it exits non-zero naming the uncovered span

#### Scenario: A uniform cadence still validates

- GIVEN a `series/{slug}.yaml` declaring a single quarterly cadence
- WHEN `validate-config` runs
- THEN it exits zero

### Requirement: The population series declares its real cadence

`config/series/poblacion-residente.yaml` MUST declare the cadence `ECP320` actually publishes:
semiannual across its historical span, quarterly from its verified quarterly-onset period. It MUST NOT
declare a single uniform quarterly cadence.

#### Scenario: The corrected configuration matches the source

- GIVEN the corrected `poblacion-residente` configuration
- WHEN it is validated against the recorded `ECP320` payload
- THEN the declared segments match the observed cadence in both spans
- AND the boundary period is the verified quarterly-onset period

#### Scenario: The uncorrected configuration now fails

- GIVEN the previous configuration declaring a uniform quarterly cadence
- WHEN ingestion and validation run against the recorded payload
- THEN the run fails
- AND that failure is the correct outcome, not a regression to be worked around

### Requirement: Unconfirmed editorial dates are operator-visible, never reader-visible

An editorial entry whose date is unconfirmed MUST NOT be projected into the database and MUST NOT reach
any rendered page. Reconciliation MUST report the count and identifiers of unprojected entries to
operators through the run's structured output, so the backlog is visible to the people who can close it.

#### Scenario: An unconfirmed entry is not projected and is not an error

- GIVEN an editorial entry with no confirmed date
- WHEN reconciliation runs
- THEN no row is projected for it
- AND reconciliation succeeds

#### Scenario: Operators see the pending count

- GIVEN seven editorial entries with unconfirmed dates
- WHEN reconciliation completes
- THEN its structured output records the count seven and the identifiers of those entries

#### Scenario: Readers see nothing about pending entries

- GIVEN the same unconfirmed entries
- WHEN any indicator page is rendered
- THEN no count, badge, warning or placeholder about them appears on the page

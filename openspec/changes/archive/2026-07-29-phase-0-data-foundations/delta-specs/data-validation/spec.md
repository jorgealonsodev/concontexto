# Delta for data-validation

Slice 4 · PRD §9.3. Greenfield capability — no existing spec to modify.

## ADDED Requirements

### Requirement: Validation rules are pure functions

Each validation rule MUST be a pure function over domain types and its own configuration. A rule MUST NOT perform I/O, MUST NOT read the clock, and MUST be deterministic for identical inputs.

#### Scenario: A rule is deterministic and side-effect free

- GIVEN a rule, a candidate observation set and a configuration
- WHEN the rule is evaluated twice
- THEN both evaluations return the identical verdict
- AND no database, filesystem or network access occurs

### Requirement: Rule 1 — schema

The system MUST verify that expected fields are present with correct types before normalisation. For workbook sources the check MUST include sheet name, header row index, column anchors and a header fingerprint declared in configuration.

#### Scenario: A renamed column fails schema validation

- GIVEN a payload whose expected value column is renamed
- WHEN rule 1 evaluates it
- THEN the verdict is fail
- AND the failure names the missing field

### Requirement: Rule 2 — continuity

The system MUST verify that the newest period is the next expected period for the series frequency and that no new undocumented gap appears. Period-format conventions differ per source (INE `T1 2026` / `M06`, Eurostat `2026-Q1` / `2026-06`) and MUST be normalised before the check. Documented gaps MUST come from a per-series allowlist.

#### Scenario: A skipped period fails continuity

- GIVEN a quarterly series whose latest stored period is `2026-Q1`
- WHEN a run delivers `2026-Q3` with no `2026-Q2`
- THEN rule 2 fails naming the missing period

#### Scenario: An allowlisted gap passes

- GIVEN a series whose configuration documents a known gap
- WHEN a run reproduces exactly that gap
- THEN rule 2 passes

### Requirement: Rule 3 — plausibility

The system MUST verify each value against a per-series absolute min/max and verify the period-over-period change against a per-series threshold. The change threshold MUST be exempted at dates recorded as breaks in `rupturas.yaml`, which makes editorial reconciliation a hard dependency of this rule.

#### Scenario: An out-of-range value fails

- GIVEN an unemployment-rate series configured with range 0–40
- WHEN a run delivers `412.0`
- THEN rule 3 fails naming the range violation

#### Scenario: A large jump at a recorded break passes

- GIVEN a series with a break recorded at period `P`
- WHEN the change between `P-1` and `P` exceeds the configured threshold
- THEN rule 3 passes because the break exempts that boundary

#### Scenario: The same jump away from a break fails

- GIVEN the same series and threshold
- WHEN an equally large change occurs at a period with no recorded break
- THEN rule 3 fails

### Requirement: Rule 4 — revision consistency

The system MUST flag a run that revises any period older than the last `N` periods. `N` MUST default to `4` and MUST be overridable per series in `series/{slug}.yaml`. A breach MUST raise a human-review alert and MUST block publication of that run until a human signs it off.

#### Scenario: A revision within the default window passes

- GIVEN a series with no override, so `N = 4`
- WHEN a run revises a value in the third-most-recent period
- THEN rule 4 passes

#### Scenario: A deep revision blocks publication

- GIVEN a series with no override, so `N = 4`
- WHEN a run revises a value nine periods back
- THEN rule 4 fails, a human-review alert is raised, and the run is not published

#### Scenario: A per-series override widens the window

- GIVEN a national-accounts series configured with `N = 12`
- WHEN a run revises a value nine periods back
- THEN rule 4 passes

### Requirement: Rule 5 — metadata completeness

A series MUST NOT be published unless source, unit, frequency and licence are all present (PRD §9.3.5). This rule MUST be evaluated before anything is published.

#### Scenario: A series missing licence cannot be published

- GIVEN a series whose configuration omits licence
- WHEN a run for it reaches the publish gate
- THEN rule 5 fails
- AND nothing is published for that series

### Requirement: Rule 6 — non-empty result

A run that yields zero observations MUST fail validation and MUST NOT be published. This is a distinct failure class from a transport error: a stale or renamed dimension code can return HTTP 200 with a structurally valid but empty payload.

#### Scenario: A structurally valid empty payload fails

- GIVEN a source returning HTTP 200 with a well-formed payload containing zero observations
- WHEN validation runs
- THEN rule 6 fails, classified distinctly from a transport error
- AND nothing is written and nothing is published

### Requirement: Publish gate

Validation failure MUST NOT publish. On failure the previously published valid datum MUST continue to be served, the amber banner state MUST be recorded, the chart MUST NOT be hidden and the suspect datum MUST NOT be published (PRD §6.1.3, §9.3). The raw file and the failed `ingestion_run` MUST still be retained for audit.

#### Scenario: A failed run leaves the published datum untouched

- GIVEN a published current observation for `(series, period)`
- WHEN a later run for the same series fails any validation rule
- THEN the current observation is unchanged
- AND no observation row is written for that run
- AND the run is recorded with a failed outcome and its raw file hash

#### Scenario: All rules passing publishes

- GIVEN a run that satisfies every applicable rule
- WHEN the publish gate evaluates it
- THEN the run is published and its observations become current

#### Scenario: The gate reports every failure, not only the first

- GIVEN a run violating rules 2 and 3
- WHEN the publish gate evaluates it
- THEN the recorded outcome lists both violations

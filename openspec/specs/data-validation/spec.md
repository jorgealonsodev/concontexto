# Spec: data-validation

Baseline capability specification — the source of truth for `data-validation`.

Sources: PRD §9.3, §6.1.3.
Contributing changes: `phase-0-data-foundations` (archived 2026-07-29) — established this capability;
`phase-1-indicator-page` (archived 2026-08-05) — widened rule 2 to audit the historical interior and added
the editorial acknowledgement registry.

Amendment history is deliberately not restated below. `(Previously: …)` annotations are delta-relative and
live in the archived change records under `openspec/changes/archive/`.

## Requirements

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

The system MUST verify that the newest period is the next expected period for the series cadence, that no
new undocumented gap appears, **and that the historical interior of the delivered series is itself
continuous under the series' declared cadence**. Rule 2 MUST audit the whole delivered series, not only
the span after `priorLatest`, and **MUST NOT return a passing verdict merely because no prior data exists**.
A declared cadence that contradicts the observed history MUST fail closed.

Period-format conventions differ per source (INE `T1 2026` / `M06`, Eurostat `2026-Q1` / `2026-06`) and
MUST be normalised before the check. Documented gaps MUST come from a per-series allowlist. Where a series
declares a segmented cadence, each segment MUST be audited under its own cadence and the segment
boundaries MUST be continuous with each other.

#### Scenario: A skipped period fails continuity

- GIVEN a quarterly series whose latest stored period is `2026-Q1`
- WHEN a run delivers `2026-Q3` with no `2026-Q2`
- THEN rule 2 fails naming the missing period

#### Scenario: An allowlisted gap passes

- GIVEN a series whose configuration documents a known gap
- WHEN a run reproduces exactly that gap
- THEN rule 2 passes

#### Scenario: The first run audits the interior rather than passing by default

- GIVEN a series with no previously stored observations
- WHEN the first run delivers a history containing an interior gap
- THEN rule 2 fails naming the missing periods
- AND the run is not published

#### Scenario: A later run audits the interior, not only the tail

- GIVEN a series already ingested, whose stored history contains an interior gap
- WHEN a later run delivers the same history
- THEN rule 2 fails naming the interior gap
- AND it does not pass merely because the tail is contiguous

#### Scenario: A declared cadence contradicting the observed history fails closed

- GIVEN a series declared as uniformly quarterly
- WHEN the delivered history is semiannual across its historical span
- THEN rule 2 fails naming the declared cadence, the observed cadence and the span
- AND nothing is published

#### Scenario: A correctly declared segmented cadence passes

- GIVEN a series declaring a semiannual segment followed by a quarterly segment with a stated boundary
- WHEN the delivered history matches those segments and the boundary is continuous
- THEN rule 2 passes

#### Scenario: Fixing the guard may reject data that previously ingested

- GIVEN a series that passed the earlier continuity check and whose stored history contains an interior gap
- WHEN the corrected rule evaluates it
- THEN the run fails
- AND the correct remedy is to correct the series configuration, never to relax the rule

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

### Requirement: Acknowledged findings

The system MUST provide an editorial acknowledgement registry recording that a named human reviewed a
specific blocking validation finding and confirmed the underlying datum is correct, after which the
publish gate MUST treat that finding as informational rather than blocking.

An acknowledgement MUST be scoped to exactly one series, one period and one rule, matched by exact
equality on all three. The configuration schema MUST NOT admit any wildcard, range or "all rules" form,
and `validate-config` MUST reject an entry whose series does not resolve, whose period is not a single
label on that series' declared frequency grid, or whose rule is not acknowledgeable.

Only rules whose blocking finding is a genuine human judgement call — the machine cannot separate a
legitimate cause from a broken one — MUST be acknowledgeable. Every other blocking finding reports a
defect in the pipeline or in configuration and MUST retain its own narrower remedy; in particular an
acknowledgement MUST NOT be able to resolve a continuity finding, whose documented remedy is to correct
the series configuration. Acknowledgement MUST be severity-agnostic: it MUST resolve `block` and
`block-requires-signoff` findings alike.

An acknowledgement MUST pin the observed value the human reviewed. Where a later run delivers a
different value at that period, or no value at all, the acknowledgement MUST NOT apply, and the run MUST
raise a blocking finding naming the acknowledgement, the pinned value and the current one. An
acknowledgement resolving no finding MUST be reported as advisory and MUST NOT block.

An acknowledgement's authority comes from a human signature, and nothing else. A record that no named
human has signed is a proposal, not an acknowledgement, however sound its research or its citation: it
MUST NOT be projected into the database and MUST NOT resolve any finding. The configuration MUST be able
to express the unsigned state explicitly — a record awaiting a signature MUST declare who drafted it and
what the reviewer must do — and the signed and unsigned states MUST be mutually exclusive, so a reader
cannot mistake a draft for a signature. A run that declines to project an unsigned record MUST report it
as pending rather than dropping it silently.

`validate-config` MUST reject a signature that is present but empty or placeholder-shaped, exactly as it
rejects a break declared confirmed with no date, and MUST require both the signer and the date on a
signed record.

Each entry MUST carry provenance: a stable identifier, the acknowledging person, the date, the reason in
prose, and the source document where one exists. The registry MUST follow the same
editorial-YAML-to-database reconcile path as the break and event registries, including per-entry digests
and soft retirement.

A run that published only because an acknowledgement resolved a finding MUST be distinguishable from one
that simply passed, both in the recorded run outcome and in the structured log; the log MUST name the
acknowledgement and the person, and MUST NOT report the acknowledged rule as a failed rule of that run.

#### Scenario: An acknowledged finding publishes as an override

- GIVEN a series blocked by a plausibility finding at one period
- AND an acknowledgement naming that series, that period and that rule, pinning the observed value
- WHEN the run is validated
- THEN the finding no longer blocks and the observations are published
- AND the recorded run outcome and the structured log both state that the run was overridden, naming the
  acknowledgement and the acknowledging person

#### Scenario: A deep revision requiring sign-off is resolvable the same way

- GIVEN a run blocked by a `block-requires-signoff` revision finding
- AND an acknowledgement naming that series, that period and the revision rule
- WHEN the run is validated
- THEN the finding is resolved by the same mechanism a `block` finding is

#### Scenario: An acknowledgement never widens beyond the finding it names

- GIVEN an acknowledgement for one series, one period and one rule
- WHEN a finding differs in the series, the period or the rule
- THEN that finding still blocks
- AND no configuration expresses an acknowledgement covering more than one finding

#### Scenario: A revised value invalidates the acknowledgement

- GIVEN an acknowledgement pinning the value a human reviewed at that period
- WHEN a later run delivers a different value at that period
- THEN the acknowledgement does not apply and the run blocks
- AND the run reports a finding naming the acknowledgement, the pinned value and the current value

#### Scenario: An unsigned record resolves nothing

- GIVEN an acknowledgement record declared unsigned, naming its drafter and what review is pending
- WHEN an ingestion run evaluates the finding it describes
- THEN the finding still blocks and the series is not published
- AND the record is not projected into the database
- AND the reconcile reports it as pending a signature

#### Scenario: A record cannot be both a draft and signed

- GIVEN a record declared unsigned that also carries a signer or a signature date
- WHEN `validate-config` runs
- THEN it fails naming the contradicting field

#### Scenario: A placeholder signature is not a signature

- GIVEN a record declared signed whose signer is empty, whitespace or a placeholder token
- WHEN `validate-config` runs
- THEN it fails naming the field, and states that an unsigned record must declare itself as such rather
  than borrow a placeholder

#### Scenario: A non-acknowledgeable rule is rejected at the configuration gate

- GIVEN an acknowledgement naming a schema, decode, continuity, metadata or non-empty rule
- WHEN `validate-config` runs
- THEN it fails naming the file, the field and the rules that are acknowledgeable

#### Scenario: An acknowledgement without provenance is rejected

- GIVEN an acknowledgement missing the acknowledging person, the date, the reason or the pinned value
- WHEN `validate-config` runs
- THEN it fails naming the file and the missing field

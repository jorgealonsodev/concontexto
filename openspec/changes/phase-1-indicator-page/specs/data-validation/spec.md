# Delta for data-validation

Slice **1** (prerequisite). Delta against `openspec/specs/data-validation/spec.md`.

Fase 0's Rule 2 returned nil on the first run and otherwise walked forward from `priorLatest`, so it never
audited the historical interior. That gap, together with single-observation periodicity detection, is what
admitted the population series with decades of missing periods.

## MODIFIED Requirements

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
(Previously: the rule returned nil on the first run and otherwise walked forward from `priorLatest` alone,
so an interior gap was never detected.)

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

- GIVEN a series that passed Fase 0's continuity check and whose stored history contains an interior gap
- WHEN the corrected rule evaluates it
- THEN the run fails
- AND the correct remedy is to correct the series configuration, never to relax the rule

# Delta for data-validation

Slice **1** (prerequisite). Delta against `openspec/specs/data-validation/spec.md`.

Fase 0's Rule 2 returned nil on the first run and otherwise walked forward from `priorLatest`, so it never
audited the historical interior. That gap, together with single-observation periodicity detection, is what
admitted the population series with decades of missing periods.

Separately, `validation/types.go` has declared `SeverityBlockRequiresSignoff` since Fase 0 and
`rule4_revision.go` documents its purpose at length — the severity exists because a deep revision is
either a legitimate methodology revision or a parser silently rewriting history, and only a human can
tell — but nothing anywhere resolved it. `gate.go` treated it exactly like `SeverityBlock`, so every
blocking finding was permanently terminal (verify-report WARNING-31). The added requirement below closes
that: it gives the human the way back that the severity's own comment promised, without weakening any
guard.

## ADDED Requirements

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

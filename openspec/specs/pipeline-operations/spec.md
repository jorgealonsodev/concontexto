# Spec: pipeline-operations

Baseline capability specification — the source of truth for `pipeline-operations`.

Sources: PRD §9.2, §9.4, §13, §19.3; ADR-7.
Contributing changes: `phase-0-data-foundations` (archived 2026-07-29) — established this capability;
`phase-1-indicator-page` (archived 2026-08-05) — added the publish-latency budget, the stalled-rebuild
alert and the rebuild dispatch, and widened the operational alert set.

This capability owns the **operations half** of ADR-7's stale-build problem. The reader-facing half is
settled in `indicator-page`: an amber page indicator means only *the source has not published yet*. "We
ingested successfully but the rebuild did not follow" is an operations alert, never rendered to a reader.

Amendment history is deliberately not restated below. `(Previously: …)` annotations are delta-relative and
live in the archived change records under `openspec/changes/archive/`.

## Requirements

### Requirement: One scheduled ingestion job per source

The scheduler MUST run one job per source, scheduled against that source's publication calendar. Transport failures MUST be retried with backoff. A source that has not responded successfully for 24 hours MUST be recorded as failed at source level and raise an internal incident (PRD §9.2).

#### Scenario: A transient failure is retried

- GIVEN a source returning a transport error on the first two attempts
- WHEN the scheduled job runs
- THEN it retries with increasing backoff and succeeds on the third attempt
- AND no incident is raised

#### Scenario: Twenty-four hours of failure raises an incident

- GIVEN a source whose last successful attempt was more than 24 hours ago
- WHEN the scheduled job fails again
- THEN an internal incident is raised
- AND the source's freshness state is failed, propagating amber to its series

#### Scenario: A non-retryable error is not retried

- GIVEN a source returning a named non-retryable error such as the INE volume restriction
- WHEN the scheduled job runs
- THEN exactly one request is issued and the job fails immediately

### Requirement: Synthetic daily probe against every endpoint

A synthetic daily probe MUST issue a minimal request to every configured source endpoint (INE `nult=1`, Eurostat `lastTimePeriod=1`) and MUST assert response SHAPE only, never values, so identifier churn is detected before the ingestion window. The probe MUST be the same build-tagged contract test that CI runs on a schedule rather than per pull request.

#### Scenario: The probe detects a broken identifier

- GIVEN a configured series whose origin identifier no longer resolves
- WHEN the probe runs
- THEN it fails naming the series and the unresolvable identifier
- AND an alert is raised before the next ingestion window

#### Scenario: The probe asserts shape, not values

- GIVEN a live endpoint whose latest value has changed since the last run
- WHEN the probe runs
- THEN it passes, because only the response shape is asserted

#### Scenario: The probe never runs in the default test suite

- GIVEN the default `go test ./...` invocation
- WHEN the suite runs
- THEN no probe test executes and no network call is made

### Requirement: Structured pipeline logging

Every ingestion run MUST emit structured logs carrying at least run id, source, dataset, series, outcome, validation verdicts, raw-file hash and duration.

#### Scenario: A run is reconstructible from logs alone

- GIVEN a completed ingestion run
- WHEN its structured logs are read
- THEN run id, source, dataset, outcome, validation verdicts and raw-file hash are all present
- AND a failed run additionally records which rules failed

### Requirement: Operational alerts

The system MUST alert on failed ingestion, failed validation, a source recorded as down, **a failed or
undispatched site rebuild, and a publish-latency budget breach**. Alerts MUST name the source and the
affected series. Alerts are internal operational signals and MUST NOT be surfaced on any reader-facing
page.

#### Scenario: A validation failure alerts without publishing

- GIVEN a run that fails validation
- WHEN the run completes
- THEN an alert is raised naming the source, the series and the failing rules
- AND the previously published datum is still served

#### Scenario: A publish-latency breach alerts operators only

- GIVEN a successful ingestion whose rebuild has not completed within the budget
- WHEN the breach is detected
- THEN an alert is raised naming the source, the series and the elapsed time
- AND no reader-facing page changes as a result

### Requirement: Publish latency has a stated budget

The publish-latency budget — from ingestion success to a deployed page — MUST be a configured value, not
a discovered property. It MUST default to **30 minutes** and MUST remain well inside PRD §19.3's under-24-hour
freshness target for API-backed sources. Each publish cycle MUST record the ingestion-success instant, the
rebuild-dispatch instant and the deploy-completed instant so the elapsed time is measurable.

#### Scenario: A publish cycle is measured end to end

- GIVEN a successful ingestion that dispatches a rebuild which deploys
- WHEN the cycle completes
- THEN the ingestion-success, dispatch and deploy-completed instants are all recorded
- AND the elapsed time is computed and compared against the configured budget

#### Scenario: The budget is configuration, not a constant

- GIVEN the operations configuration
- WHEN the publish-latency budget is read
- THEN it resolves from configuration with a documented default of 30 minutes

### Requirement: An ingestion not followed by a rebuild alerts operators

When a successful ingestion is not followed by a completed rebuild and deploy inside the publish-latency
budget, the system MUST raise an operational alert through the existing alerting sink, naming the affected
series, the ingestion run and the elapsed time. The condition MUST NOT be rendered to a reader on any page,
and the page MUST NOT be given any mechanism to detect it.

#### Scenario: A stalled rebuild raises an alert

- GIVEN a successful ingestion whose rebuild has not completed after the configured budget elapses
- WHEN latency is evaluated
- THEN an alert is raised naming the series, the ingestion run and the elapsed time
- AND the alert is delivered through the same sink as ingestion and validation alerts

#### Scenario: A failed rebuild alerts once the publish-latency budget elapses

- GIVEN a rebuild dispatched by a successful ingestion that fails, so the pages still being served are the
  ones built from an artifact older than the one this publish cycle produced
- WHEN latency is evaluated after the configured publish-latency budget has elapsed
- THEN an alert is raised naming the source and the elapsed time, through the same sink as ingestion and
  validation alerts
- AND nothing is raised while the cycle is still inside that budget
- AND nothing is raised once the pages being served carry the artifact this cycle produced

#### Scenario: The condition never reaches a reader

- GIVEN an outstanding publish-latency breach
- WHEN any indicator page is rendered or served
- THEN no banner, badge, semaphore state or text discloses the breach
- AND the page's amber state continues to mean only that the source has not published yet

#### Scenario: A within-budget cycle raises nothing

- GIVEN a publish cycle completing inside the budget
- WHEN latency is evaluated
- THEN no alert is raised

### Requirement: A successful ingestion dispatches the rebuild

A successful ingestion run MUST dispatch the site rebuild automatically. A failed or unpublished run MUST
NOT dispatch one. Dispatch MUST be recorded in the run's structured log.

#### Scenario: Success dispatches, failure does not

- GIVEN two ingestion runs, one passing every validation rule and one failing
- WHEN each completes
- THEN the passing run dispatches a rebuild and records the dispatch
- AND the failing run dispatches nothing

#### Scenario: A dispatch failure is itself an alert

- GIVEN a successful ingestion whose rebuild dispatch cannot be delivered
- WHEN the dispatch fails
- THEN an alert is raised naming the run and the dispatch failure

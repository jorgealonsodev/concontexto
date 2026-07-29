# Delta for pipeline-operations

Slice 9 · PRD §9.2, §9.4, §13 (observability). Greenfield capability — no existing spec to modify.

## ADDED Requirements

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

The system MUST alert on failed ingestion, failed validation and a source recorded as down. Alerts MUST name the source and the affected series.

#### Scenario: A validation failure alerts without publishing

- GIVEN a run that fails validation
- WHEN the run completes
- THEN an alert is raised naming the source, the series and the failing rules
- AND the previously published datum is still served

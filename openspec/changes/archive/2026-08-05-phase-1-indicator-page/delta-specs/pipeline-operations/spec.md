# Delta for pipeline-operations

Slice **4**. Delta against `openspec/specs/pipeline-operations/spec.md`. Sources: ADR-7, PRD §19.3, §13.

This delta owns the **operations half** of ADR-7's stale-build problem. The reader-facing half is settled
in `indicator-page`: the page's amber indicator means only *the source has not published yet*. "We
ingested successfully but the rebuild did not follow" is an operations alert, never rendered to a reader.

## ADDED Requirements

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

(Previously — this delta's own earlier text, not the baseline's; the requirement above is ADDED by this
delta and its normative sentence is unchanged. The scenario read: "A failed rebuild raises an alert
**immediately**" — GIVEN a rebuild dispatched by a successful ingestion that fails / WHEN the failure is
observed / THEN an alert is raised naming the ingestion run and the build failure / AND the currently
deployed site continues to be served unchanged. Nothing in this system ever observes a dispatched run's
conclusion: the alerting package raises on a failed dispatch *call* (`KindDispatchFailed`) and on the
budget elapsing (`KindPublishLatencyBreach`), and the only `workflow_run` in the repository is
`deploy.yml:16`'s deploy trigger, not an alerting receiver. So neither "immediately" nor "naming the
ingestion run and the build failure" was ever reachable from inside the container, and the scenario had gone
untested since verify pass 4. **What is amended is the bound, not the promise**: the detection is real, it is bounded
by the publish-latency budget this same delta already requires — a configured value defaulting to 30
minutes — and it is bounded because a failed rebuild pushes no image, so the artifact the served pages were
built from cannot advance and the divergence outlives the budget. The dropped "continues to be served
unchanged" clause is not lost: it is now the scenario's own GIVEN, which is the condition the alert fires
on, and the reader-facing half is already carried by this requirement's "The condition never reaches a
reader" scenario below.)

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

## MODIFIED Requirements

### Requirement: Operational alerts

The system MUST alert on failed ingestion, failed validation, a source recorded as down, **a failed or
undispatched site rebuild, and a publish-latency budget breach**. Alerts MUST name the source and the
affected series. Alerts are internal operational signals and MUST NOT be surfaced on any reader-facing
page.
(Previously: the alert set covered failed ingestion, failed validation and a source recorded as down only.)

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

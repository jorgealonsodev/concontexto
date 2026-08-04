# Spec: source-ingestion-eurostat

Baseline capability specification — the source of truth for `source-ingestion-eurostat`.

Sources: PRD §9.2, §9.4.
Contributing changes: `phase-0-data-foundations` (archived 2026-07-29) — established this capability;
`phase-1-indicator-page` (archived 2026-08-05) — added JSON-stat status decoding and the break/definition
flag routing.

Endpoint behaviour below was verified live on 2026-07-28.

Rationale worth preserving: INE and Eurostat fail in opposite directions. INE *refuses* an oversized query with a non-array HTTP 200 envelope; Eurostat *serves* it — 157 MB in the verified case, against a 256 MB container limit. The two adapters therefore need different guards, and Eurostat's guard must sit at configuration-validation time, not only at runtime.

## Requirements

### Requirement: JSON-stat 2.0 adapter over the dissemination API

The Eurostat adapter MUST request `https://ec.europa.eu/eurostat/api/dissemination/statistics/1.0/data/{dataset}?format=JSON&lang=EN&{dim}={code}...` and MUST decode JSON-stat 2.0. It MUST reuse the same domain types, observation writer and validation harness as the INE adapter; only the adapter differs.

#### Scenario: A JSON-stat response becomes canonical observations

- GIVEN a recorded JSON-stat 2.0 response for a configured dataset
- WHEN the adapter parses it
- THEN it yields canonical observations with normalised periods
- AND those observations pass through the same writer and validation harness as INE observations

### Requirement: Three verified milestone-0.3 datasets

Configuration MUST pin these datasets with at least the filters shown. `prc_hicp_manr` and `prc_hicp_midx` are discontinued and MUST NOT be used; the replacement `prc_hicp_minr` uses dimension `coicop18`, not `coicop`.

| Dataset | Required pinned filters |
|---|---|
| `prc_hicp_minr` | `geo=ES`, `unit=RCH_A`, `coicop18=TOTAL` |
| `une_rt_q` | `geo=ES`, plus `s_adj`, `age`, `unit`, `sex` |
| `nama_10_gdp` | `geo=ES`, plus `unit`, `na_item` |

#### Scenario: The three datasets load and validate

- GIVEN the three pinned dataset configurations
- WHEN ingestion runs against recorded fixtures
- THEN each dataset loads observations and passes every applicable validation rule

#### Scenario: Discontinued dataset codes are absent from configuration

- GIVEN the `/config` tree
- WHEN it is scanned
- THEN neither `prc_hicp_manr` nor `prc_hicp_midx` is referenced
- AND no configuration uses dimension name `coicop` for `prc_hicp_minr`

### Requirement: Every dimension except time MUST be pinned in configuration

A Eurostat series configuration MUST declare a filter for every dimension of its dataset except `time`. `validate-config` MUST reject a configuration with an unpinned dimension, naming the dataset and the unpinned dimension. Enforcement MUST happen at configuration-validation time, not only at runtime, because an unpinned dimension yields a cartesian product with no server-side size guard.

#### Scenario: An unpinned dimension fails validate-config

- GIVEN a `prc_hicp_minr` series configuration pinning `geo` and `unit` but not `coicop18`
- WHEN `validate-config` runs
- THEN it exits non-zero naming `prc_hicp_minr` and the unpinned `coicop18` dimension
- AND CI fails

#### Scenario: A fully pinned configuration passes

- GIVEN a `une_rt_q` configuration pinning `freq`, `s_adj`, `age`, `unit`, `sex` and `geo`
- WHEN `validate-config` runs
- THEN it exits zero

### Requirement: Response size ceiling

The Eurostat HTTP client MUST impose a configured maximum response size and MUST abort the read and fail the run when the ceiling is exceeded, without buffering the full body. The ceiling MUST be small enough that decoding cannot approach the 256 MB container limit.

#### Scenario: An oversized response is aborted

- GIVEN a stubbed endpoint streaming a body larger than the configured ceiling
- WHEN the adapter reads it
- THEN the read is aborted before the body is fully buffered
- AND the run fails with a named response-too-large error
- AND process memory does not exceed the configured ceiling by more than the read buffer

### Requirement: A zero-observation result fails the run

Querying a dead dimension code returns HTTP 200 with valid JSON-stat and `"value": {}` — a silent empty result, not an error. In ECOICOP ver.2 the all-items code is `TOTAL`; `CP00` is the dead v1 code. An ingestion run yielding zero observations MUST fail validation and MUST NOT be published.

#### Scenario: A dead dimension code is caught as an empty result

- GIVEN a stubbed response with HTTP 200, valid JSON-stat and `"value": {}`
- WHEN the run is validated
- THEN it fails with the zero-observation failure class, distinct from a transport error
- AND nothing is written and nothing is published
- AND the run is recorded with a failed outcome

### Requirement: Scheduled maintenance is a retryable condition

Eurostat publishes scheduled maintenance windows. The scheduler MUST treat unavailability during such a window as a normal retryable condition handled by backoff per PRD §9.2, and MUST NOT raise it as an incident until the 24-hour source-level threshold is crossed.

#### Scenario: Maintenance unavailability retries rather than alerting

- GIVEN the endpoint is unavailable and the last successful attempt was 2 hours ago
- WHEN the scheduled job runs
- THEN the attempt is retried with backoff
- AND no incident alert is raised

#### Scenario: Prolonged unavailability escalates

- GIVEN the endpoint has been unavailable for more than 24 hours
- WHEN freshness is evaluated
- THEN the source is recorded as failed and an incident alert is raised

### Requirement: lastTimePeriod is the minimal-probe parameter

The adapter MUST support `lastTimePeriod=N` as the Eurostat analogue of INE's `nult=N`, and the synthetic daily probe MUST use it to issue a minimal request.

#### Scenario: The probe requests a single period

- GIVEN a probe against a configured Eurostat dataset
- WHEN the request is built
- THEN it carries `lastTimePeriod=1` together with every pinned dimension filter

### Requirement: JSON-stat status is read at the computed position

The decoder MUST read the JSON-stat `status` map at the same linear `pos` it already computes for each
value, and MUST carry the verbatim flag through to the observation's `source_status`.

#### Scenario: A flagged observation carries its verbatim flag

- GIVEN a recorded JSON-stat response whose `status` map carries `"p"` at the position of one observation
- WHEN it is decoded
- THEN that observation's `source_status` is `"p"`
- AND its domain status is provisional
- AND no other observation is affected

#### Scenario: Status is aligned with the value it belongs to

- GIVEN a response with flags at several non-contiguous positions
- WHEN it is decoded
- THEN each flag lands on the observation at the same linear position as the value it annotates

### Requirement: Absence of a flag means definitive

Eurostat publishes no definitive flag. An observation with no entry in the `status` map MUST be recorded
as definitive, with a null `source_status`. The absence MUST NOT be treated as unknown, missing or a
schema drift.

#### Scenario: An unflagged observation is definitive

- GIVEN a recorded response whose `status` map has no entry for an observation
- WHEN it is decoded
- THEN that observation's domain status is definitive
- AND its `source_status` is null

#### Scenario: An empty status map is valid

- GIVEN a recorded response carrying no `status` map at all
- WHEN it is decoded
- THEN every observation is definitive and ingestion proceeds

### Requirement: Break and definition flags are metadata, not statuses

The flags `b` (break in time series) and `d` (definition differs) MUST be routed to break and definition
metadata beside `series_break`, and MUST NOT be mapped into the observation status enum. An unrecognised
flag MUST fail the run as `sourceerr.SchemaDrift`.

#### Scenario: A break flag becomes break metadata

- GIVEN a recorded response with `"b"` at an observation's position
- WHEN it is decoded
- THEN a break annotation is recorded for that series and period beside `series_break`
- AND the observation's domain status is not set from the `b` flag

#### Scenario: A definition flag becomes definition metadata

- GIVEN a recorded response with `"d"` at an observation's position
- WHEN it is decoded
- THEN a definition-differs annotation is recorded for that series and period
- AND the observation's domain status is not set from the `d` flag

#### Scenario: An unrecognised flag fails closed

- GIVEN a recorded response carrying a flag outside the documented set
- WHEN it is decoded
- THEN the run fails with `sourceerr.SchemaDrift` naming the dataset and the unrecognised flag
- AND nothing is written

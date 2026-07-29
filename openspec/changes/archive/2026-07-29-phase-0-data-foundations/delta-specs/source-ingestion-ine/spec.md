# Delta for source-ingestion-ine

Slices 5a and 5b · Milestone 0.2. Greenfield capability — no existing spec to modify.

Identifiers below were verified live on 2026-07-28 against `https://servicios.ine.es/wstempus/js/ES/...`.

## ADDED Requirements

### Requirement: Ingestion uses DATOS_SERIE per canonical series

The INE adapter MUST ingest through `DATOS_SERIE/{COD}`, one request per canonical series (ADR D5). `DATOS_TABLA` and `SERIES_TABLA` MUST be used for discovery only and MUST NOT appear on the ingestion path. The series COD is the stable identifier; the table Id is only a container.

#### Scenario: A series is ingested through DATOS_SERIE

- GIVEN a configured INE series with COD `EPA453100`
- WHEN ingestion runs
- THEN exactly one request is issued to `DATOS_SERIE/EPA453100`
- AND no `DATOS_TABLA` request is issued

### Requirement: Six canonical series with pinned identifiers

Configuration MUST pin these six milestone-0.2 series, each with its expected periodicity, and each MUST load its full history and pass validation.

| Series | Table Id | Series COD | Expected periodicity |
|---|---|---|---|
| Tasa de paro (EPA) | 65349 | `EPA453100` | quarterly |
| Ocupados (EPA) | 65109 | `EPA387796` | quarterly |
| IPC general (index) | 76125 | `IPC290751` | monthly |
| IPC subyacente | 76130 | `IPC292511` | monthly |
| PIB (chained volume index) | 67822 | `CNTR6721` | quarterly |
| Población residente | 59238 | `ECP320` | quarterly |

The stale PRD identifiers table `4247` (frozen at 2023-Q4), table `50902` (frozen at 2025-12 on the old base) and operation `72`/`CP` (empty table list) MUST NOT be used; population comes from operation `450`/`ECP`.

#### Scenario: All six series load full history and validate

- GIVEN the six pinned series configurations
- WHEN a full historical ingestion runs against recorded fixtures
- THEN each series loads its complete observation history
- AND each passes every applicable validation rule
- AND each produces observations with source, origin COD, extraction timestamp and raw-file hash

#### Scenario: No retired identifier remains in configuration

- GIVEN the `/config` tree
- WHEN it is scanned
- THEN none of `4247`, `50902`, operation `72` or `CP` is referenced

### Requirement: Periodicity is asserted when resolving an identifier

Every pinned identifier MUST declare its expected periodicity, and ingestion MUST fail if the returned periodicity differs. Table name alone does not identify a series: tables 65962 and 72982 carry the same `Nombre` as 65109 but hold annual averages while 65109 is quarterly.

#### Scenario: A periodicity mismatch fails the run

- GIVEN a series configured as quarterly
- WHEN the response carries annual periods
- THEN ingestion fails with a periodicity-mismatch error naming the expected and actual periodicity
- AND nothing is written

#### Scenario: A matching periodicity proceeds

- GIVEN a series configured as quarterly
- WHEN the response carries quarterly periods
- THEN ingestion proceeds to validation

### Requirement: Volume-restriction envelope surfaces as a named non-retryable error

`DATOS_TABLA` on a wide table returns HTTP 200 with the body `{"status" : "No puede mostrarse por restricciones de volumen"}` — a JSON object where success is a JSON array. The adapter MUST decode into a discriminating type, MUST surface this envelope as a named error, and MUST NOT retry it. Retry and backoff MUST NOT loop on this condition.

#### Scenario: The restriction envelope produces a named error

- GIVEN a stubbed endpoint returning HTTP 200 with body `{"status" : "No puede mostrarse por restricciones de volumen"}`
- WHEN the adapter decodes the response
- THEN it returns the named volume-restriction error, not an opaque JSON type error
- AND the error is classified as non-retryable

#### Scenario: Backoff does not loop on the restriction envelope

- GIVEN the same stubbed endpoint and a retry policy allowing five attempts
- WHEN the call is made
- THEN exactly one request is issued
- AND the run fails immediately with the named error

#### Scenario: A genuine transport error is still retried

- GIVEN a stubbed endpoint returning HTTP 503 twice and then a valid array body
- WHEN the call is made under the same retry policy
- THEN the request is retried with backoff and eventually succeeds

### Requirement: INE period formats are normalised

INE period labels (for example `T1 2026` for quarters and `M06` for months) MUST be normalised to the canonical domain period before validation, so that continuity checks are source-independent.

#### Scenario: Quarterly and monthly labels normalise

- GIVEN INE responses carrying `T1 2026` and `M06 2026`
- WHEN they are normalised
- THEN they yield the canonical quarterly period for 2026 Q1 and the canonical monthly period for June 2026

### Requirement: Recorded fixtures back the offline test loop

Each INE fixture under `testdata/` MUST be a trimmed real response accompanied by a `source.txt` recording its URL and fetch date, and MUST be served through an in-process HTTP test server. The volume-restriction envelope MUST be a first-class fixture.

#### Scenario: The default test run needs no network

- GIVEN network access is unavailable
- WHEN the default test suite runs
- THEN every INE adapter test passes against recorded fixtures

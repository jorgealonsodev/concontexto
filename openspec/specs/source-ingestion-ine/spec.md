# Spec: source-ingestion-ine

Baseline capability specification — the source of truth for `source-ingestion-ine`.

Sources: PRD §9.4; ADR D5.
Contributing changes: `phase-0-data-foundations` (archived 2026-07-29) — established this capability;
`phase-1-indicator-page` (archived 2026-08-05) — added the `T3_TipoDato` carry-through, the null-`Valor`
refusal and the phantom-field rule, and amended the pinned-series and periodicity requirements.

Identifiers below were verified live on 2026-07-28 against `https://servicios.ine.es/wstempus/js/ES/...`.

Amendment history is deliberately not restated below. `(Previously: …)` annotations are delta-relative and
live in the archived change records under `openspec/changes/archive/`.

## Requirements

### Requirement: Ingestion uses DATOS_SERIE per canonical series

The INE adapter MUST ingest through `DATOS_SERIE/{COD}`, one request per canonical series (ADR D5). `DATOS_TABLA` and `SERIES_TABLA` MUST be used for discovery only and MUST NOT appear on the ingestion path. The series COD is the stable identifier; the table Id is only a container.

#### Scenario: A series is ingested through DATOS_SERIE

- GIVEN a configured INE series with COD `EPA453100`
- WHEN ingestion runs
- THEN exactly one request is issued to `DATOS_SERIE/EPA453100`
- AND no `DATOS_TABLA` request is issued

### Requirement: Six canonical series with pinned identifiers

Configuration MUST pin these six milestone-0.2 series, each with its expected cadence, and each MUST load
its full history and pass validation.

| Series | Table Id | Series COD | Expected cadence |
|---|---|---|---|
| Tasa de paro (EPA) | 65349 | `EPA453100` | quarterly |
| Ocupados (EPA) | 65109 | `EPA387796` | quarterly |
| IPC general (index) | 76125 | `IPC290751` | monthly |
| IPC subyacente | 76130 | `IPC292511` | monthly |
| PIB (chained volume index) | 67822 | `CNTR6721` | quarterly |
| Población residente | 59238 | `ECP320` | semiannual historically, quarterly from its verified quarterly-onset period |

`ECP320` is verified semiannual across its historical span (1977–1980 carry only `"1 de enero de"` and
`"1 de julio de"`) and verified continuously quarterly from 2023-Q3 to 2026-Q2. Its configuration MUST
declare a cadence that admits that real mixed history and MUST NOT declare a single quarterly cadence.

The stale PRD identifiers table `4247` (frozen at 2023-Q4), table `50902` (frozen at 2025-12 on the old
base) and operation `72`/`CP` (empty table list) MUST NOT be used; population comes from operation
`450`/`ECP`.

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

#### Scenario: The population series stores its real cadence

- GIVEN the corrected `poblacion-residente` configuration
- WHEN its full history is ingested
- THEN its historical span stores semiannual observations only
- AND no invented Q2 or Q4 period exists in that span
- AND its recent span stores quarterly observations

### Requirement: Periodicity is asserted when resolving an identifier

Every pinned identifier MUST declare its expected cadence, and ingestion MUST fail if the observed cadence
differs. **Periodicity MUST be detected over the whole payload, not from a single observation**, so that a
series whose cadence changes over its life is judged against its real history rather than against its
first row. Table name alone does not identify a series: tables 65962 and 72982 carry the same `Nombre` as
65109 but hold annual averages while 65109 is quarterly.

#### Scenario: A periodicity mismatch fails the run

- GIVEN a series configured as quarterly
- WHEN the response carries annual periods
- THEN ingestion fails with a periodicity-mismatch error naming the expected and actual periodicity
- AND nothing is written

#### Scenario: A matching periodicity proceeds

- GIVEN a series configured as quarterly
- WHEN the response carries quarterly periods
- THEN ingestion proceeds to validation

#### Scenario: A single quarterly declaration fails against a mixed history

- GIVEN a series declared as uniformly quarterly
- WHEN the payload carries semiannual periods across its historical span and quarterly periods recently
- THEN periodicity detection reads the whole payload
- AND ingestion fails naming the declared cadence and the observed historical cadence
- AND nothing is written

#### Scenario: A declared segmented cadence matching the payload proceeds

- GIVEN a series declaring a semiannual segment followed by a quarterly segment
- WHEN the payload matches those segments at their declared boundaries
- THEN periodicity detection passes
- AND ingestion proceeds to validation

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

### Requirement: The source's data-type token is carried through to the domain

`T3_TipoDato` MUST be decoded by the wire type, carried through the adapter's observation type and the
domain observation type, and persisted as the observation's verbatim `source_status`. The ingestion path
MUST NOT hardcode a status for INE observations.

#### Scenario: A definitive observation carries its token

- GIVEN a recorded `DATOS_SERIE` response whose observation carries `T3_TipoDato` of `"Definitivo"`
- WHEN it is ingested
- THEN the stored observation's `source_status` is `"Definitivo"`
- AND its domain status is definitive

#### Scenario: A provisional observation is not coerced to definitive

- GIVEN a recorded response whose observation carries `T3_TipoDato` of `"Provisional"`
- WHEN it is ingested
- THEN the stored observation's `source_status` is `"Provisional"`
- AND its domain status is provisional
- AND no code path assigns a definitive status without reading the token

#### Scenario: A mixed series preserves the boundary

- GIVEN a recorded `ECP320` response definitive through 2025-Q1 and provisional from 2025-Q2 onward
- WHEN it is ingested
- THEN each observation carries the token its own period declared
- AND the provisional/definitive boundary in the stored series matches the payload exactly

### Requirement: An unknown data-type token fails closed

INE publishes no enumeration for this field: the OpenAPI `TiposDatosJSON` schema declares Id, Nombre and
Codigo with no enum. An unrecognised token MUST therefore fail the run as `sourceerr.SchemaDrift`, naming
the series and the token. It MUST NOT be coerced to definitive, silently dropped, or stored with an empty
domain status.

#### Scenario: An unrecognised token fails as schema drift

- GIVEN a stubbed response carrying an unrecognised `T3_TipoDato` value
- WHEN ingestion runs
- THEN it fails with `sourceerr.SchemaDrift` naming the series and the unrecognised token
- AND nothing is written

#### Scenario: A missing token fails rather than defaulting

- GIVEN a stubbed `tip=A` response whose observation omits `T3_TipoDato`
- WHEN ingestion runs
- THEN it fails as schema drift
- AND no observation is stored with a defaulted status

### Requirement: A null Valor has no decided meaning and fails closed

A `DATOS_SERIE` row published with an explicit `"Valor": null` MUST fail the decode as
`sourceerr.SchemaDrift`, naming the series, the canonical period and the row as decoded. It MUST NOT
become an observation with a null value, MUST NOT be assigned a withdrawn, definitive or any other
domain status to satisfy migration 0001's `CHECK (value IS NOT NULL OR status = 'W')`, and MUST NOT be
silently dropped.

The reason is that the row carries no information about WHY it is null. A `DATOS_SERIE` row carries
exactly five fields — `Fecha`, `Anyo`, `T3_Periodo`, `T3_TipoDato`, `Valor` — with no secrecy marker, no
not-applicable flag and no annotation of any kind (verified live 2026-07-30 across all six configured
series, 1,032 rows, of which zero carry a null). Statistical secrecy, a not-applicable period and a
genuine gap are therefore indistinguishable, so every projection would be an invention. Marking the row
withdrawn would be the worst of them: `data-model-vintages` defines withdrawal as a source **stopping**
publication of a period it previously published, and a period the source is publishing right now, as a
null, was never withdrawn.

This is deliberately **not** the shape of the Eurostat sparse-position rule. A JSON-stat value map with
no entry at a position is the **absence** of a datum and is decidable, so no observation is emitted. An
INE row that exists and carries an explicit null is a positive act by the source, and discarding it would
throw away something INE chose to publish.

The refusal message MUST name what would have to be decided — which domain status a null `Valor` carries,
or that the row yields no observation — so whoever first encounters it can act rather than only diagnose.

#### Scenario: A null value with a definitive token fails closed

- GIVEN a stubbed `DATOS_SERIE` response whose row carries `"Valor": null` and `T3_TipoDato` of `"Definitivo"`
- WHEN ingestion runs
- THEN it fails with `sourceerr.SchemaDrift` naming the series, the canonical period and the row
- AND the message states that the project has no decided meaning for a null `Valor`
- AND the message names the spec where the projection must be decided
- AND no observation is written, neither for that period nor for the other rows in the payload

#### Scenario: The refusal does not depend on the accompanying status token

- GIVEN stubbed responses whose row carries `"Valor": null` with `"Definitivo"`, with `"Provisional"`, with an unrecognised token, and with `T3_TipoDato` omitted
- WHEN each is decoded
- THEN every one fails as schema drift naming the null `Valor`
- AND no token value causes the null to be accepted

#### Scenario: A published zero is a value, not a missing one

- GIVEN a response whose row carries `"Valor": 0`
- WHEN it is decoded
- THEN it produces an observation whose value is `0`
- AND the run is not refused

#### Scenario: Every decoded observation carries a value

- GIVEN any `DATOS_SERIE` response the adapter decodes successfully
- WHEN its observations are inspected
- THEN none carries a null value

### Requirement: Wire types declare only fields the requested response contains

An INE wire type MUST NOT declare a field absent from the response the adapter actually requests. The
`Secreto` field, which does not exist in the `tip=A` response and has therefore always decoded as false,
MUST be removed.

#### Scenario: No phantom field survives

- GIVEN the INE wire observation type
- WHEN its fields are compared against a recorded `tip=A` response
- THEN every declared field is present in the response
- AND no `Secreto` field is declared

# Delta for source-ingestion-ine

Slices **1** and **2** (prerequisites). Delta against `openspec/specs/source-ingestion-ine/spec.md`.

Verified live 2026-07-29: the adapter already requests `tip=A`, so `T3_TipoDato` is present in every raw
payload already archived. Nothing needs re-fetching.

## ADDED Requirements

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

### Requirement: Wire types declare only fields the requested response contains

An INE wire type MUST NOT declare a field absent from the response the adapter actually requests. The
`Secreto` field, which does not exist in the `tip=A` response and has therefore always decoded as false,
MUST be removed.

#### Scenario: No phantom field survives

- GIVEN the INE wire observation type
- WHEN its fields are compared against a recorded `tip=A` response
- THEN every declared field is present in the response
- AND no `Secreto` field is declared

## MODIFIED Requirements

### Requirement: Six canonical series with pinned identifiers

Configuration MUST pin these six milestone-0.2 series, each with its expected cadence, and each MUST load
its full history and pass validation.
(Previously: `Población residente` declared a single `quarterly` periodicity, which contradicts its
observed history.)

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
(Previously: periodicity was judged from `Data[0]` alone, which admitted a series whose declared cadence
contradicts its historical interior.)

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

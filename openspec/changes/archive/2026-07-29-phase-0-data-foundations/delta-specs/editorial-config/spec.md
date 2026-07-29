# Delta for editorial-config

Slices 3 and 7 · Milestones 0.5 and 0.7 (structure). Greenfield capability — no existing spec to modify.

File names `rupturas.yaml`, `eventos.yaml` and `gobiernos.yaml` are fixed by PRD §9.6 and are kept in Spanish.

## ADDED Requirements

### Requirement: Configuration layout

`/config` MUST contain `sources/{source}.yaml` (identity, licence, attribution, access type), `series/{slug}.yaml` (canonical slug, validity-ranged source references, unit, frequency, decimals, thresholds for validation rules 2–4, adapter-specific schema expectations) and the three editorial files `rupturas.yaml`, `eventos.yaml`, `gobiernos.yaml`.

#### Scenario: A series config declares its full identity

- GIVEN a `series/{slug}.yaml` file
- WHEN it is loaded
- THEN it resolves a canonical slug, at least one validity-ranged source reference, unit, frequency and decimals

### Requirement: Source identifiers live only in configuration

Origin identifiers (INE series CODs and table Ids, Eurostat dataset codes and dimension filters, XLSX download URLs) MUST live in versioned configuration and MUST NOT appear in Go source code (PRD §9.4).

#### Scenario: No origin identifier is hard-coded

- GIVEN the Go source tree excluding `testdata/`
- WHEN it is scanned for origin identifier literals
- THEN no origin identifier is found outside configuration loading and test fixtures

### Requirement: Configuration is compiled into the binary

Configuration MUST be embedded with `go:embed` and MUST NOT be mounted at runtime (ADR D4), so the effective configuration version is pinned to the binary version and is traceable per principle P2.

#### Scenario: Runtime configuration cannot drift from the binary

- GIVEN a running container
- WHEN configuration files on the host are modified
- THEN the running process continues to use the embedded configuration
- AND the effective configuration changes only after a rebuild and redeploy

### Requirement: validate-config subcommand

`validate-config` MUST schema-validate every file under `/config`, MUST exit non-zero on any violation with a message naming the offending file and field, and MUST run in CI as a blocking gate. Adapter-specific configuration rules contributed by each source capability MUST be enforced by the same command.

#### Scenario: A malformed series config fails the build

- GIVEN a `series/{slug}.yaml` missing its `unit` field
- WHEN `validate-config` runs
- THEN it exits non-zero
- AND the message names the file and the missing field
- AND CI fails

#### Scenario: A valid configuration set passes

- GIVEN a complete and well-formed `/config` tree
- WHEN `validate-config` runs
- THEN it exits zero

#### Scenario: A reference to an unknown source is rejected

- GIVEN a series config referencing a source with no `sources/{source}.yaml`
- WHEN `validate-config` runs
- THEN it exits non-zero naming the unresolved source reference

### Requirement: Editorial entries carry stable identifiers and a digest

Every entry in `rupturas.yaml`, `eventos.yaml` and `gobiernos.yaml` MUST carry a stable, human-authored `id`. Every reconciled database row MUST record a `config_digest` (SHA-256 of its source YAML) so an edit is distinguishable from a delete plus insert.

#### Scenario: Editing a description updates rather than replaces

- GIVEN a break entry with a stable `id` already reconciled
- WHEN only its description text changes and the reconcile runs
- THEN the same row is updated in place
- AND its `config_digest` changes
- AND no retirement is recorded

#### Scenario: Duplicate ids are rejected

- GIVEN two entries in the same editorial file sharing an `id`
- WHEN `validate-config` runs
- THEN it exits non-zero naming the duplicated `id`

### Requirement: Breaks are scoped, not duplicated per series

`series_break` MUST support a scope that applies one break to a family of series (for example every CPI-derived series) without duplicating one row per member series.

#### Scenario: A family-scoped break applies to every member

- GIVEN a break scoped to the CPI series family
- WHEN the reconcile runs
- THEN the break resolves for every member series
- AND it is stored once, not once per series

### Requirement: Editorial YAML is authoritative and reconciled transactionally

The database is a projection of the editorial YAML. Reconciliation to `series_break` and `event` MUST be a single transactional, idempotent FULL RECONCILE. It MUST NOT be upsert-only and MUST NOT hard-delete: entries absent from the YAML MUST be soft-retired.

#### Scenario: Removing an entry soft-retires it

- GIVEN a reconciled break entry
- WHEN the entry is deleted from `rupturas.yaml` and the reconcile runs
- THEN the database row still exists, marked retired with a retirement timestamp
- AND it is excluded from the live projection

#### Scenario: Re-running the reconcile changes nothing

- GIVEN a reconcile that has just completed
- WHEN the identical reconcile runs again
- THEN zero rows are inserted, updated or retired

#### Scenario: A failed reconcile leaves no partial state

- GIVEN a reconcile that fails part-way through
- WHEN the transaction aborts
- THEN the database contents are byte-identical to the pre-reconcile state

#### Scenario: Reverting the YAML restores the prior projection

- GIVEN a retired entry
- WHEN the YAML change is reverted and the reconcile runs
- THEN the row returns to the live projection
- AND the retirement history remains recorded

### Requirement: Minimum break list includes the ECOICOP v2 transition

`rupturas.yaml` MUST include, in addition to the PRD §9.6 minimum list, the ECOICOP ver.2 / January 2026 break. ECOICOP 1 is archived and frozen; from January 2026 data exists only under ECOICOP 2 and Eurostat warns of potential January 2026 breaks. The break MUST be scoped to every CPI-derived series that is genuinely affected by the reclassification — not to every CPI-derived series unconditionally. A headline aggregate that its source explicitly states was LINKED across the transition (variation rates unchanged) is correctly excluded from the break's scope; the break applies to the subclass-level disaggregations that are discontinued or newly introduced under ECOICOP v2.

Amendment (remediation 2026-07-29, sdd-verify W3): the original wording of this requirement ("scoped to every CPI-derived series") did not match the shipped entry, which correctly narrows scope to subclass-level datasets (`ine-ipc-subclases`, `eurostat-hicp-subclases`) because INE and Eurostat both document the headline aggregates as linked/prudently-excluded. As of this remediation, none of Fase 0's ten configured series belongs to either subclass dataset, so the break currently resolves for zero configured series — this is the correct, disclosed outcome of the scoping decision, not a defect; the entry remains reconciled and ready for the subclass-level series Fase 1 is expected to add.

#### Scenario: The ECOICOP v2 break is present, correctly scoped, and may resolve for zero series today

- GIVEN the reconciled database
- WHEN breaks for the subclass-level COICOP datasets are resolved
- THEN the ECOICOP ver.2 / January 2026 break is among them with a link to the source methodological note
- AND resolving breaks for any of Fase 0's ten currently configured (headline-aggregate) series MUST NOT return this break, because those series are documented as linked across the transition

### Requirement: Breaks are stored as non-dismissible

Series breaks MUST be persisted without any user-dismissible or default-off flag (principle P4). Fase 0 stores and reconciles them only; rendering belongs to Fase 1.

#### Scenario: The break model exposes no dismiss affordance

- GIVEN the `series_break` schema and its domain type
- WHEN they are inspected
- THEN neither carries a dismissible, optional or default-hidden attribute

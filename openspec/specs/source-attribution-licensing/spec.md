# Spec: source-attribution-licensing

Baseline capability specification — the source of truth for `source-attribution-licensing`.

Sources: PRD §9.3.5; settled decision D2.
Contributing changes: `phase-0-data-foundations` (archived 2026-07-29) — established this capability;
`phase-1-indicator-page` (archived 2026-08-05) — extended "No blanket data-licence claim" to reader-facing
surfaces.

Amendment history is deliberately not restated below. `(Previously: …)` annotations are delta-relative and
live in the archived change records under `openspec/changes/archive/`.

## Requirements

### Requirement: Per-source licensing terms are authoritative

Each `sources/{source}.yaml` MUST record the source identity, its licence, its required attribution text, its access type and any redistribution restriction. These per-source terms MUST be authoritative over any site-wide statement (settled decision D2).

#### Scenario: A source declares complete terms

- GIVEN a `sources/{source}.yaml` file
- WHEN it is validated
- THEN licence, attribution text, access type and redistribution restrictions are all present
- AND a file missing any of them fails `validate-config`

#### Scenario: Eurostat restrictions are recorded, not flattened

- GIVEN the Eurostat source configuration
- WHEN it is read
- THEN it records that reuse is permitted with acknowledgement under Commission Decision 2011/833/EU, that the permission does not extend to third-party material, and that some commercial redissemination is restricted

### Requirement: No blanket data-licence claim exists in the repository

The repository MUST NOT assert a single licence over all derived data. Code is licensed MIT. Derived data
and editorial text are offered under CC BY 4.0 with chained attribution only where per-source terms permit
it, and `LICENSE-DATA` MUST defer to `sources/{source}.yaml` rather than override it.

**Reader-facing surfaces are part of the repository for the purposes of this requirement.** Any site-level
statement about the reuse of published data MUST state that no single licence covers it and that each source
fixes its own conditions; it MUST NOT summarise, flatten or substitute for those per-source terms, and it
MUST route a reader to the authoritative per-source configuration rather than to the deferring document.
The CC BY 4.0 offer above is CONDITIONAL on each source permitting redistribution, so it MUST NOT appear
beside the data on a reader-facing surface, where a conditional claim is read as an unconditional one; it
belongs in `LICENSE-DATA`, where its condition travels with it.

#### Scenario: Repository licensing files are consistent

- GIVEN `LICENSE` and `LICENSE-DATA`
- WHEN they are read
- THEN `LICENSE` is MIT for code
- AND `LICENSE-DATA` states that per-source terms in `sources/{source}.yaml` govern source-derived values
- AND no statement claims one licence over all derived data

#### Scenario: The site defers on data licensing rather than asserting one

- GIVEN a rendered page of the site
- WHEN its site-level licensing statement is read
- THEN it states in Spanish that the published data is not covered by a single licence and that each source
  fixes its own reuse conditions
- AND it summarises no source's conditions
- AND it names MIT for the code and for nothing else
- AND it links the per-source configuration directory rather than the deferring `LICENSE-DATA`
- AND its rendered text and markup match none of `cc by`, `creative commons`, `todos los datos` or
  `licencia de los datos`, in any casing

#### Scenario: Every route entry point carries the statement

- GIVEN the set of route entry points the build emits, discovered by scanning the site sources for a
  document root rather than from a maintained list
- WHEN the build output is inspected
- THEN every one of them renders the site-level licensing statement
- AND no entry point is exempted by a skip list

### Requirement: Attribution chains from the original source

Attribution for a published value MUST be derivable from its provenance chain: series to source to that source's configured attribution text. The chain MUST resolve without manual lookup.

#### Scenario: Attribution resolves from an observation

- GIVEN a published observation
- WHEN its attribution is resolved
- THEN it yields the configured attribution text of its originating source
- AND the origin series identifier and extraction timestamp accompany it

### Requirement: The attribution table is closed before anything is published

Every source referenced by any configured series MUST have complete licensing configuration. `validate-config` MUST fail if any referenced source is missing licensing fields, and a series missing source, unit, frequency or licence MUST NOT be publishable.

#### Scenario: An incomplete source blocks the build

- GIVEN a series referencing a source whose configuration omits its licence
- WHEN `validate-config` runs
- THEN it exits non-zero naming the source and the missing field
- AND CI fails

#### Scenario: A series missing licence cannot be published

- GIVEN a configured series with no resolvable licence
- WHEN a run for it reaches the publish gate
- THEN the metadata-completeness rule fails and nothing is published for that series

# Delta for source-attribution-licensing

Slices 3 and 9 · Milestone 0.7. Greenfield capability — no existing spec to modify.

## ADDED Requirements

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

The repository MUST NOT assert a single licence over all derived data. Code is licensed MIT. Derived data and editorial text are offered under CC BY 4.0 with chained attribution only where per-source terms permit it, and `LICENSE-DATA` MUST defer to `sources/{source}.yaml` rather than override it.

#### Scenario: Repository licensing files are consistent

- GIVEN `LICENSE` and `LICENSE-DATA`
- WHEN they are read
- THEN `LICENSE` is MIT for code
- AND `LICENSE-DATA` states that per-source terms in `sources/{source}.yaml` govern source-derived values
- AND no statement claims one licence over all derived data

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

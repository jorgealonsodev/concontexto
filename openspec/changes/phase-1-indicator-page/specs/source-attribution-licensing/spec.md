# Delta for source-attribution-licensing

Slice **25** (`9af3f86`, the site footer). Delta against `openspec/specs/source-attribution-licensing/spec.md`.

Written in the record pass that closed verify-report pass-7 CRITICAL-46, resolving pass-7 **WARNING-47**'s
footer half. The finding as pass 7 stated it was that the footer's licensing claim is governed by no
requirement. That is nearly right and the correction matters: the baseline requirement "No blanket
data-licence claim exists in the repository" **already governs it by its own text** — the footer is part of
the repository — but its only scenario reads `LICENSE` and `LICENSE-DATA`, two files, neither of them a
rendered page. So a footer asserting CC BY over all derived data would violate the requirement's sentence
while passing its only scenario.

This delta therefore adds a scenario rather than a requirement. The requirement's text is restated unchanged
below; nothing about the repository's licensing position is altered by this change. What changes is that the
position is now pinned on the surface where the site makes it to every reader, on every page, rather than
only in two files a reader never opens.

Nothing else in this capability moves. The other three requirements — per-source terms authoritative,
attribution chains from the original source, the attribution table closed before publication — are untouched
by this change and are not restated here.

## MODIFIED Requirements

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
(Previously: the requirement's text already governed the whole repository, but its only scenario read
`LICENSE` and `LICENSE-DATA`. No scenario reached a rendered page, so the site could have asserted a blanket
licence to every reader without failing this requirement's only check.)

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

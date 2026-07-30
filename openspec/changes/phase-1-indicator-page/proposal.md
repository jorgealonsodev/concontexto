# Proposal — phase-1-indicator-page

Change: `phase-1-indicator-page` · Project: **ConContexto** · PRD Fase 1 (§17), milestones **1.1** and
**1.2** only, plus one blocking prerequisite pair.
Input: `openspec/changes/phase-1-indicator-page/exploration.md` (complete), `docs/adr/0007-pre-render-pipeline.md`
(accepted), `docs/adr/0006-tailwind-for-styling.md`, `docs/adr/0008-free-colour-palette.md` (accepted,
PRD §12.1 amended to v2.4). Artifact store: hybrid. Delivery: `auto-chain`,
stacked-to-main, 400-line review budget, Strict TDD.

## Intent

Fase 0 built a pipeline and rendered nothing. Every value in the database is versioned, provenanced and
validated, and **no human can see any of it**. This change turns the pipeline into the product's atom:
the indicator page at `/indicador/{slug}`.

PRD §2.1 names the problem precisely — Spanish economic debate misuses real figures through temporal
cherry-picking, ignored series breaks and nominal-for-real substitution. The answer is not a chart. It is
a chart that cannot be cropped without showing what was cropped. This milestone is where three principles
stop being schema and become visible for the first time:

| Principle | What this change makes visible |
|---|---|
| **P1 — data ≠ interpretation** | The chart and its methodology sheet form one block. No editorialised headline sits above a chart. Structural separation, not a style guideline. |
| **P2 — total traceability** | Every page links source, origin series identifier, extraction timestamp, ingestion script and the vintage shown. Every point discloses provisional or definitive. |
| **P4 — context by default** | Full range by default, per capita beside every aggregate, year-on-year beside every index, and series breaks that the reader **cannot** dismiss. |

Why now: ADR-7 settled the one architectural question that blocked all of Fase 1, and the archive report
lists "surface the P/D distinction" and "build the indicator-page capability" as the two items that block
the Fase 1 UI launch. Both are in this change.

## Scope

### In scope

**Prerequisites** (blocking, sequenced first)

- **Cadence guard defect.** `poblacion-residente` (ECP320) is semiannual across its historical span
  (verified 1977–1980) and quarterly only recently (verified continuous 2023-Q3 → 2026-Q2), yet its config
  declares `frequency: Q`. Two Fase 0 guard gaps let it through: `envelope.go:58` judges periodicity from
  `Data[0]` alone, and `Rule2Continuity` returns nil on the first run and otherwise walks forward from
  `priorLatest`, never auditing the historical interior. Both are fixed here; the series config is
  corrected to a cadence that admits the real mixed history.
- **`TipoDato` surfacing.** Verified live: the adapter already sends `tip=A`
  (`app/internal/adapters/ine/client.go:179`), so `T3_TipoDato` is present in every archived raw payload.
  Nothing needs re-fetching. The loss is three places: `wireObservation`, the `Observation` types, and
  `ingest.go:248`, which hardcodes `StatusDefinitive`. Also drop the phantom `Secreto bool`, which does
  not exist in the `tip=A` response. Eurostat's `status` is read at the same linear `pos` the decoder
  already computes.

**Milestone 1.1 — design system and component library**

- Tailwind CSS-first `@theme` block that **explicitly zeroes Tailwind's defaults**, so a stock utility
  class generates nothing at all. Dark mode as a second explicit token set, never derived.
- The project palette, chosen on ordinary design criteria (ADR-8), plus the two reserved semantics that
  carry meaning rather than identity: **amber = pending data, dotted grey = provisional**.
- The eight PRD §12.1 components: indicator card, interactive chart, methodology sheet, break band,
  annotation chip, action bar, freshness semaphore, accessible data table. Seven static, one hydrated.
- Documented component workbench, measured AA contrast in both themes, 44 px touch targets.

**Milestone 1.2 — complete indicator page for the first six indicators**

- `/indicador/{slug}` for `tasa-de-paro-epa`, `ocupados-epa`, `ipc-general`, `ipc-subyacente`, `pib` and
  `poblacion-residente`, with the full §6.1 anatomy and the three §6.1.3 page states.
- Range presets, year-on-year variation (mandatory, not optional, for the four index-backed series),
  per-capita over the span where the denominator genuinely exists, and the §6.1.2 coherence rules.
- The publishing layer at `app/internal/publishing/`, the versioned export contract, and the
  ingest → rebuild trigger. `/data-derived` falls out of the same artifact.
- Frontend test stack and blocking quality gates.

### Out of scope

- **Fase 1 milestones 1.3–1.7**: the full ~26-indicator catalogue and its written methodology sheets,
  search, permalinks/exports/OG images, methodology-about-corrections pages, audits and public launch.
- **Real / nominal deflation.** None of the six is a monetary magnitude — a rate, a headcount, two price
  indices, an already-real volume index and a population. The entire deflator apparatus is unnecessary
  here. **Median** likewise does not exist for any of the six.
- **Any colour-neutrality analysis.** ADR-8 lifted the party-colour prohibition; there is no vetoed hue
  list and no research to produce. Multi-series categorical encoding barely binds here anyway — milestone
  1.2 renders one series per page — and becomes real in the Fase 2 comparator.
- Fase 2–4: comparator, *Verificado*, glossary, embeds, public API, vintages in UI.
- Onboarding the second `/config` reviewer and provisioning the VPS deploy target: real blockers,
  user-side actions, not deliverables of this change.

## Capabilities

### New capabilities

| Capability | Covers | Slice |
|---|---|---|
| `publishing-export` | `app/internal/publishing/`: the versioned build-time export artifact (published series, metadata, breaks, events, freshness), its schema and two-sided validation, the breaks/events read path, `/data-derived` CSV, the ingest → rebuild trigger and the publish-latency budget. | 3, 4 |
| `design-system` | Tailwind `@theme` with zeroed defaults, the project palette and its reserved semantics, dual explicit light/dark token sets, typography with tabular figures, the eight-component library and its workbench, the no-component-kit guard. | 5, 6 |
| `indicator-page` | §6.1 anatomy for six slugs, §6.1.3 page states, freshness semaphore including the stale-build state, methodology sheet, non-dismissible break bands, annotation layers, related indicators, the 300 KB budget and the no-JS baseline. | 7, 8, 9 |
| `series-transformations` | Range presets, year-on-year variation, per-capita with the denominator-coverage constraint, and §6.1.2's rule that a toggle which does not apply does not exist. | 8 |
| `web-accessibility-gates` | Vitest with `experimental_AstroContainer`, Playwright with `@axe-core/playwright`, the `javaScriptEnabled: false` context proving §14.2's no-JS clause, and blocking Lighthouse CI with an explicit transferred-bytes assertion. | 9 |

### Modified capabilities

| Capability | Requirement change |
|---|---|
| `source-ingestion-ine` | `T3_TipoDato` is carried through to the domain; unknown tokens fail closed as `sourceerr.SchemaDrift` (INE publishes no enum); periodicity is detected over the whole payload, not from `Data[0]`; the phantom `Secreto` field is removed. |
| `source-ingestion-eurostat` | JSON-stat `status` is read at the computed `pos`; **absence means definitive** (there is no "definitive" flag); `b` and `d` are routed to break/definition metadata beside `series_break`, not into the status enum. |
| `data-model-vintages` | `observation.status` stays P/D/W; a nullable `source_status` column holds the source's verbatim token. Migration required. Status-only transitions already append a version and already bypass `Rule4Revision`. |
| `data-validation` | `Rule2Continuity` MUST audit the historical interior of a series, not only walk forward from `priorLatest`, and MUST NOT return nil on the first run. A declared cadence that contradicts the observed history fails closed. |
| `editorial-config` | Series configuration MUST express a cadence that changes over the life of a series; `poblacion-residente` is corrected accordingly. |
| `pipeline-operations` | A successful ingestion triggers the site rebuild; publish latency is measured against a stated budget and alerts on breach. |

## Settled decisions (do not re-open)

| # | Decision | Where |
|---|---|---|
| D1 | **ADR-7** — the pipeline exports a build-time artifact; Astro builds from it; a successful ingestion triggers the rebuild. Options A (Go renders everything) and C (Go splices fragments) were considered and rejected. | `docs/adr/0007-pre-render-pipeline.md` |
| D2 | **ADR-6** — Tailwind with a hand-written theme. Component kits (Flowbite, shadcn/ui, Bootstrap, DaisyUI, Material UI, Chakra) remain forbidden. Tailwind's default palette, type scale, radii and shadows are forbidden. | `docs/adr/0006-tailwind-for-styling.md` |
| D3 | `observation.status` stays P/D/W; a nullable `source_status` preserves the source's own word. P2 wants the verbatim token; §6.1.1's UI contract is binary; Eurostat's `b`/`d` are not statuses. | exploration §1 |
| D4 | Per capita is implemented **over the span where the denominator genuinely exists**, after the cadence defect is fixed, with the attribution rule stated in the methodology sheet and flagged for editorial sign-off. Dropping the toggle was rejected (P4 requires it); interpolating the missing semiannual periods was rejected (§6.1.2 forbids inventing the denominator). | exploration §9 |
| D5 | **ADR-8** — the party-colour prohibition is **lifted**. Palette choice is an ordinary design decision made on design grounds; there is no forbidden-hue list and no neutrality analysis, and hue is available as a categorical channel again. What is **not** lifted, because it is accessibility rather than preference (§12.5): colour is never the sole channel distinguishing a series — line pattern or marker shape always carries it too — and AA contrast is **measured** in both light and dark themes, including tooltips, never derived from the light palette. The reserved semantics stand because they carry meaning rather than identity: amber = pending data, dotted grey = provisional. | `docs/adr/0008-free-colour-palette.md` |
| D6 | The chart is **one hydrated Svelte island plus four static Astro components** (SVG renderer with break bands, accessible data table, generated textual description, break/annotation partials). Ship a pre-rendered SVG plus a compact series JSON (~2–4 KB gzipped for the longest 294-point series) and redraw client-side on toggle. §14.2's "cero cómputo" constrains the **server's request path**, not the browser — the import guard already encodes that distinction. | exploration §3 |

## Non-negotiable rules (must survive into the spec)

| Rule | Source |
|---|---|
| **Zero database queries and zero computation at page-request time.** All HTML and SVG is pre-rendered. | §14.2 golden rule |
| **No external source is ever called at page-request time. Ever.** | §9.2 |
| **Series breaks are never user-dismissible** — no affordance exists to hide them. | P4, §6.1.1 |
| **Data and interpretation are structurally separate.** No editorialised headline above a chart. | P1, §12.1 |
| **Under 300 KB transferred per indicator page**, including data for the default range, excluding the typeface. | §12.3 |
| **Colour is never the sole channel distinguishing one series from another** — line pattern or marker shape always carries the distinction too. | §12.5, ADR-8 |
| **AA contrast is measured in both light and dark themes, including tooltips**, never derived from the light palette. | §12.5, ADR-8 |
| **Reserved semantics stand: amber = pending data, dotted grey = provisional.** They carry meaning, not identity. | §12.1, ADR-8 |
| **Every chart carries an alternative data table, a textual description of the pattern, and keyboard navigation of its points.** | §12.5 |
| **Annotation groups (a) governments and (b) exogenous shocks are OFF by default**; the reader enables them consciously. | §6.1.1 |
| **Validation failure never publishes.** The last valid datum keeps being served, with a banner; the chart is never hidden. | §6.1.3, §9.3 |
| **A toggle that does not apply does not exist** — it is not shown disabled. | §6.1.2 |

## Approach

### 1 · Publishing layer — `app/internal/publishing/`

Fase 0's design reserved this package and never built it; the raw-hash listing went into
`postgres.PublishRawFileHashListing` instead. This is a **ninth instance** of the archive report's
recurring pattern — here "designed but never built" rather than "built but never wired".

- `Export` reads published series, their metadata, breaks, events and freshness through existing driven
  ports and writes a versioned artifact. `postgres.SourceFreshness` is **not** dead code (archive-report
  W16 is wrong): `SeriesFreshness` calls it at `freshness.go:65`, and this page is its first consumer.
- A **read path for breaks and events does not exist yet** and is part of this layer.
- The artifact carries an explicit `schema_version`. It is validated **on write in Go and on read in the
  Astro build**. A version or shape mismatch fails the build loudly. Without this the export becomes the
  next silent-drift surface, which is exactly the shape ADR-7 warns about.
- `/data-derived` is the same artifact in CSV form, discharging P5's public derived-data obligation.
- A successful ingestion dispatches the rebuild. **Publish latency is a stated budget, not a discovered
  property**, and it is instrumented and alerted.
- The amber freshness indicator gains a distinct state meaning **"we hold newer data than this page
  shows"**, separate from "the source has not published yet". Conflating them would tell a reader the
  source is late when our own publish step is.

### 2 · Frontend — `web/`

`web/` today is a three-file Astro hello-world with no integrations, no Svelte, no Tailwind, no components
and no CSS. This change adds:

- `@astrojs/svelte`, Tailwind via its Vite plugin, and a content layer that loads the export artifact.
- `src/styles/theme.css` — one `@theme` block that zeroes stock scales (`--color-*: initial;` and
  equivalents), so a stray stock palette class produces **no class at all**. The mistake becomes loud
  instead of shipped, which makes ADR-6's first constraint mechanically enforced rather than reviewed.
  The constraint is on Tailwind's *default values*, not on any hue — ADR-8 removed every hue restriction.
- Static Astro components for seven of the eight; one Svelte island for tooltips, keyboard navigation,
  range presets and transformation toggles.
- Chart geometry is generated at build time in the Astro build (D3 / Observable Plot), never in Go — that
  was the decisive argument against ADR-7 option A.

### 3 · Testing (Strict TDD is active, and applies honestly)

- `npm --prefix web test` — Vitest with Astro's `experimental_AstroContainer` for SVG output and
  design-token guards.
- `npm --prefix web run test:e2e` — Playwright with `@axe-core/playwright`, keyboard navigation, 44 px
  touch targets, and a `javaScriptEnabled: false` context proving §14.2's no-JS clause.
- Blocking Lighthouse CI with an explicit under-300-KB-transferred assertion, wired **from the first web
  slice, not at the end**.
- `openspec/config.yaml` lines 87 and 90 must be updated with the web test command (currently `TBD`).
- **Strict TDD applies red-first to the Go publishing layer and to pure render and transform functions.
  Playwright and Lighthouse are acceptance gates written alongside the work, not before it.** Forcing
  red-first onto a browser-rendered accessibility audit produces theatre, not proof.
- The archive report's eight-instance "built, tested, never connected" pattern argues for at least one
  end-to-end test that ingests and then builds, proving the two ecosystems actually meet.

### 4 · Language boundary

All technical artifacts — specs, design, tasks, code, comments, tests, ADRs, commit messages — are
**English**. This change introduces the project's first real **user-facing product copy, which is Spanish**
(PRD §13: "Español v1; arquitectura i18n preparada, cadenas externalizadas"). The boundary:

| Layer | Language |
|---|---|
| Specs, design, tasks, code, comments, tests, docs | English |
| UI strings, banner text, methodology-sheet prose, chart labels, textual chart descriptions | **Spanish**, externalised in a strings module from day one, never inlined in components |
| Indicator slugs, config filenames, source and operation names, editorial YAML keys | Spanish where the PRD fixes them as data |

## Exit criteria per milestone

### Prerequisites

- [ ] Periodicity is detected over the full payload; a series whose declared cadence contradicts its
      observed history **fails validation**.
- [ ] `Rule2Continuity` detects an interior gap in a series it has already ingested, on the first run and
      on later runs.
- [ ] `poblacion-residente` stores its real semiannual history without invented Q2/Q4 periods.
- [ ] Every INE observation carries `source_status`; an unknown token fails as `sourceerr.SchemaDrift`
      rather than being coerced to Definitivo.
- [ ] Eurostat observations with no flag are definitive; `b` and `d` reach break metadata, not status.

### Milestone 1.1

- [ ] A build using a Tailwind **stock palette or stock scale** class emits **no CSS rule** — asserted by
      a test. (ADR-6, a defaults constraint; ADR-8 removed every hue constraint.)
- [ ] Light and dark token sets are independently defined; every text-on-background, tooltip and chart
      pairing has **measured** AA contrast in both, asserted by a test.
- [ ] Two series in one chart remain distinguishable with colour removed — line pattern or marker shape
      carries the distinction (§12.5), asserted by a test even though 1.2 renders one series per page.
- [ ] `web/package.json` contains no component kit — asserted by a dependency guard test.
- [ ] All eight components render in a documented workbench; seven ship zero runtime JavaScript.
- [ ] The chart renders non-dismissible break bands and three separately toggleable annotation groups,
      with (a) and (b) off by default.
- [ ] Every chart point is reachable by keyboard; the data table and textual description are present.

### Milestone 1.2

- [ ] Six `/indicador/{slug}` pages build **purely from the export artifact**, with no database
      reachable from the build and none from the request path.
- [ ] Each page transfers **under 300 KB** including default-range data, excluding the typeface —
      asserted by a blocking CI gate, not measured by hand.
- [ ] With JavaScript disabled, the SVG chart, data table, textual description, methodology sheet and
      break bands are all present and legible.
- [ ] The freshness semaphore distinguishes three states: fresh, source pending, and **this page is older
      than the data we hold**.
- [ ] Every tooltip and every table row discloses provisional or definitive from `source_status`.
- [ ] The methodology sheet carries source, origin series identifier, extraction timestamp, vintage,
      break list and a link to the ingestion script (P2, end to end).
- [ ] The per-capita toggle exists only on pages where the denominator covers the rendered span, and is
      absent — not disabled — elsewhere.
- [ ] An ingestion success produces a rebuilt, deployed page within the stated latency budget.

## Affected areas

| Area | Impact | Description |
|---|---|---|
| `app/internal/publishing/` | New | Export artifact, schema, validation, `/data-derived` writer. |
| `app/internal/adapters/postgres/` | Modified | Breaks/events read path; `source_status` persistence. |
| `app/internal/adapters/ine/envelope.go` | Modified | `T3_TipoDato` wired; `Secreto` removed; periodicity over the full payload. |
| `app/internal/adapters/eurostat/envelope.go` | Modified | `status` read at `pos`; `b`/`d` routed to break metadata. |
| `app/internal/ingestion/ingest.go` | Modified | Line 248 stops hardcoding `StatusDefinitive`. |
| `app/internal/ingestion/validation/` | Modified | `Rule2Continuity` audits the historical interior. |
| `app/internal/indicators/` | Modified | `Observation` carries source status. |
| migrations | New | Nullable `source_status` column. |
| `config/series/` | Modified | `poblacion-residente` cadence correction (**four-eyes gated**). |
| `web/` | New | Tailwind theme, Svelte integration, components, island, six pages, Vitest, Playwright. |
| `.github/workflows/` | Modified | Web tests, Lighthouse CI, ingest-triggered rebuild dispatch. |
| `openspec/config.yaml` | Modified | Lines 87 and 90: the web test command, currently `TBD`. **Also stale after ADR-8**: `stack.styling_constraints_that_survive` and the `rules.apply` guideline "never use red, blue, green or purple where they could read as party colours" must be removed, or `sdd-apply` will enforce a withdrawn constraint. |
| `/data-derived` | New | Populated for the first time. |

## Delivery forecast

Fase 0's measured overrun was **3.4×** against a 400-line review budget (~5,300 estimated, ~18,000
delivered), and the archive report's revised Fase 1 projection is 12,000–15,000 lines. This change is a
design system, a component library, a chart, a pre-render layer, a schema migration and a new test stack.
**It is not one slice.**

| # | Slice | Milestone | Budget risk | Depends on |
|---|---|---|---|---|
| 1 | Periodicity over full payload + `Rule2Continuity` interior audit + population cadence config | prereq | Medium-High | — |
| 2 | `source_status` migration + INE `T3_TipoDato` + Eurostat flags — **split likely** | prereq | High | 1 |
| 3 | Export contract schema + `publishing.Export` core + write-side validation | 1.2 | High | 2 |
| 4 | Breaks/events read path + `/data-derived` + ingest → rebuild trigger + latency budget | 1.2 | Medium-High | 3 |
| 5 | Tailwind `@theme`, zeroed defaults, palette, dual token sets, contrast harness, Vitest bootstrap | 1.1 | Medium-High | — |
| 6 | The seven static components + workbench | 1.1 | High | 5 |
| 7 | Static chart: SVG renderer, break bands, annotation layers, data table, textual description | 1.1 | High | 6 |
| 8 | Chart island: tooltips, keyboard navigation, range presets, YoY and per-capita toggles | 1.1, 1.2 | High | 7 |
| 9 | Six indicator pages, §6.1.3 states, Spanish copy, Playwright/axe/Lighthouse gates — **split likely** | 1.2 | High | 4, 8 |

**Honest arithmetic**: 9 slices at a strict 400 lines is ~3,600 lines, against a 12,000–16,000-line
expectation. Either the slice count triples into micro-PRs or per-slice overrun is accepted, as it was in
Fase 0 (PR 3 delivered 3.9× and the orchestrator accepted it as a coherent unit). **Realistic outcome:
9–11 slices averaging 1,200–1,600 lines.** Any slice projected above 2,000 lines must split before apply.

`Decision needed before apply: Yes` · `Chained PRs recommended: Yes` · `400-line budget risk: High`

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Fixing the cadence guards rejects data that currently ingests — a green pipeline turns red. | **Confirmed** | That is the correct outcome, not a regression. Sequence it first, before anything depends on the population series. Rollback is correcting the config, never reverting the guard. |
| The export artifact becomes the next silent-drift surface — a ninth "built and never connected". | High | Versioned schema validated on **both** sides; the build fails loudly on mismatch; one end-to-end ingest-then-build test. |
| Milestone 1.2's annotation layers **are** the events UI, which depends on the **seven still-unconfirmed editorial dates** from Fase 0. | High | Confirm or exclude before slice 7. `ReconcileEditorialConfig` already refuses to project a nil date, so nothing wrong reaches the page — but the layer will be incomplete. |
| The population cadence correction is a `/config/**` change, and four-eyes review is **mathematically impossible** with one maintainer (`CODEOWNERS` line 17 is a placeholder). | High | Escalate before slice 1. Either onboard the second reviewer or record an explicit, dated, documented exception. Do not silently bypass. |
| Strict TDD does not map onto Playwright and Lighthouse. | High | Red-first for the Go publishing layer and pure render/transform functions; browser and budget gates written alongside. Stated in the spec so `sdd-apply` does not manufacture ceremony. |
| The page is a build-time snapshot, so it **cannot self-detect a failed rebuild** — yet it must show "we hold newer data than this page shows". | Medium | Open design question for `sdd-design`: operational alert only, or reader-visible, and by what mechanism that does not break the golden rule or add a second island. |
| 300 KB budget under Tailwind + island + SVG + series data. | Medium | Assert the budget from the first web slice, not at milestone 1.7's audit. |
| The `source_status` backfill produces version-2 rows caused by our own bug, publicly visible in the vintage history. | Medium | P7: disclose, do not erase. Record the reason in the ingestion run. |
| Review-budget overrun repeats Fase 0's 3.4×. | High | Forecast above; split any slice projected over 2,000 lines. |
| No VPS deploy target exists (`PORTAINER_WEBHOOK_URL` unset). | Medium | Does not block build or CI; blocks the ingest → rebuild → deploy loop end to end. Provision before slice 4 closes. |

## Rollback plan

1. **Per slice**: `git revert` the slice PR. Slices 5–8 are additive and unreferenced until slice 9 wires
   the routes, so reverting them has zero user impact.
2. **Schema**: the `source_status` migration ships a reversible `down`. The column is nullable and
   additive, so a revert loses annotation but never an observation.
3. **Publish**: the export artifact is versioned and retained for the last N builds. Rolling back the
   pipeline binary and rebuilding from the previous artifact restores the previous site byte-for-byte.
   A site rollback is a CI re-run of the last green commit — no database action.
4. **Deploy**: Portainer redeploy of the previous image, as in Fase 0.
5. **Cadence fix — explicit exception.** If the corrected guard rejects live data, the rollback is
   **not** reverting the guard. It is correcting the series configuration. Reverting would silently
   re-admit data known to be wrong, which is the defect this change exists to close.
6. **Point of no return**: none, because public launch (1.7) is out of scope. But `/indicador/{slug}`
   slugs become a permanent commitment the moment pages are public (Anexo E.1: permalinks never break).
   Freeze the six slugs deliberately in this change rather than by accident in 1.3.

## Dependencies

- **ADR-6**, **ADR-7** and **ADR-8**, all accepted. None is re-opened.
- Seven unconfirmed editorial dates in `config/gobiernos.yaml` and `config/eventos.yaml` — blocks the
  annotation layer's completeness.
- A second `/config/**` reviewer — blocks the population cadence correction.
- A VPS with Portainer and `PORTAINER_WEBHOOK_URL` — blocks end-to-end publish verification.
- Editorial sign-off on the per-capita attribution rule stated in the methodology sheet.

## Success criteria

- [ ] All prerequisite, 1.1 and 1.2 exit criteria above pass as automated tests, except the deploy loop,
      which is a CI job plus a smoke check.
- [ ] Six indicator pages are live in the build, each under 300 KB, each usable with JavaScript disabled.
- [ ] The golden rule is structurally provable: the request path has no database dependency and the site
      is built from a validated artifact alone.
- [ ] The population series stores its real cadence, and the guard that missed it now fails on it.
- [ ] Every published point discloses provisional or definitive from the source's own token.
- [ ] Breaks are non-dismissible, and no UI affordance exists to hide them.
- [ ] `openspec/config.yaml` no longer says `TBD` for the web test command, and no longer carries the
      party-colour guideline withdrawn by ADR-8.
- [ ] `/data-derived` is populated from the same artifact the site is built from.

## Proposal question round (unanswered — auto mode)

Execution mode is `auto`, so these were not asked interactively. Each carries the assumption adopted in
this proposal. **Correct any of these before `sdd-spec` runs**, since each changes spec-level behaviour.

| # | Question | Assumption adopted |
|---|---|---|
| Q1 | What publish-latency budget is acceptable from ingestion success to a live page? | A stated budget well inside §19.3's 24-hour freshness target; the exact number is set in the spec and instrumented. A budget breach alerts operators. |
| Q2 | Should the "this page is older than the data we hold" state be visible to readers, or is it an operational alert only? A build-time page cannot detect its own staleness without new machinery. | Reader-visible is assumed, because ADR-7 explicitly requires the distinct state — but the mechanism is deferred to `sdd-design`. If ops-only is acceptable, the design simplifies considerably. |
| Q3 | Are the six slugs frozen now? Permalinks are a permanent commitment (Anexo E.1) and 1.3 adds twenty more. | Frozen in this change, deliberately, rather than by accident later. |
| Q4 | The population cadence correction touches `/config/**`, where four-eyes review cannot currently be satisfied. Proceed with a documented exception, or block until a second reviewer exists? | Escalated as a blocker before slice 1. No silent bypass is assumed. |
| Q5 | If the corrected guards reject other live series beyond the population one, does that block the change or ship as disclosed known-open items? | Blocks the affected series only; the remaining pages proceed, with the rejection disclosed per P7. |

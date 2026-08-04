# Exploration — phase-1-indicator-page

Change: `phase-1-indicator-page` · PRD Fase 1, milestones **1.1** (design system and
component library) and **1.2** (complete indicator page for the first six indicators),
plus one blocking prerequisite.

Artifact store: hybrid — Engram `sdd/phase-1-indicator-page/explore` (observation #4721)
and this file.

Out of scope, for later Fase 1 changes: 1.3 (full ~26-indicator catalogue and its
methodology sheets), 1.4 (search), 1.5 (permalinks, exports, OG images), 1.6
(methodology / about / corrections pages), 1.7 (audits and launch).

## 0. Current state

- `web/` is a three-file Astro hello-world (`astro ^7.1.4`): no integrations, no Svelte,
  no Tailwind, no components, no CSS.
- Serving is `httpserver.NewServer(os.DirFS(staticAssetRoot()))` — `http.FileServer`
  plus `/healthz` and cache headers, rooted at `$STATIC_ROOT` or `web/dist`.
- The golden-rule guard (`app/internal/httpserver/importguard_test.go`) runs
  `go list -deps` and fails on `adapters/{postgres,ine,eurostat,xlsx}` and `pgx`.
- **`app/internal/publishing/` does not exist.** Fase 0's design reserved it for the
  Fase 1 pre-render layer; the raw-hash listing went into
  `postgres.PublishRawFileHashListing` instead. Fase 1 starts this layer from zero —
  a ninth instance of the archive report's recurring pattern, here "designed but never
  built" rather than "built but never wired".
- CI has three jobs: `go`, `web` (npm ci + astro build, **zero web tests**),
  `container-smoke-test`. No Lighthouse CI.

## 1. Prerequisite RESOLVED — INE `TipoDato`, verified live 2026-07-29

| Request | Field | Observed |
|---|---|---|
| `DATOS_SERIE/EPA453100?nult=4&tip=A` | `T3_TipoDato` | `"Definitivo"` |
| `DATOS_SERIE/CNTR6721?nult=4&tip=A` | `T3_TipoDato` | `"Provisional"` |
| `DATOS_SERIE/ECP320?nult=12&tip=A` | `T3_TipoDato` | `"Definitivo"` to 2025-Q1, `"Provisional"` 2025-Q2 → 2026-Q2 |
| same, no `tip` | `FK_TipoDato` | `1` = Definitivo, `2` = Provisional |
| `?tip=AM` | `MetaData[]` | no additional status information |

**The adapter already requests `tip=A`** (`app/internal/adapters/ine/client.go:179`), so
`T3_TipoDato` is present in every raw byte stream already archived. Nothing needs
re-fetching and the archived history is not lossy.

The loss is exactly three places:
1. `wireObservation` (`app/internal/adapters/ine/envelope.go:35-40`) has no TipoDato field.
2. `ine.Observation` / `indicators.Observation` carry only Period and Value.
3. `app/internal/ingestion/ingest.go:248` hardcodes `Status: postgres.StatusDefinitive`.

The write side is already prepared: `observation.status` has no CHECK constraining its
domain, `ObservationWriter.WriteRevision` already appends when "value **or status**
differs", and `Rule4Revision` compares values only — so a status-only P→D transition
will not trip the human-signoff block.

Two incidental findings:
- `wireObservation` declares `Secreto bool`, which does **not** exist in the `tip=A`
  response actually requested. A silently always-false field.
- INE does not enumerate the domain: the OpenAPI `TiposDatosJSON` schema declares
  Id/Nombre/Codigo with **no enum**. Unknown tokens must therefore fail closed as
  `sourceerr.SchemaDrift`, exactly as `detectPeriodicity` already does for unknown
  cadence codes.

**Eurostat**: `app/internal/adapters/eurostat/envelope.go:30-35` explicitly discloses the
identical omission. JSON-stat keys `status` by the same linear index the decoder already
computes as `pos`, so reading it is a one-line addition. But Eurostat's flags are not a
P/D binary — `p e b d f u c n :` — and there is **no "definitive" flag**; absence means
definitive.

**Recommended domain shape**: keep `observation.status` at P/D/W and add a nullable
`source_status text` holding the source's verbatim token. P2 wants the source's own word
preserved; §6.1.1's UI contract is binary; and Eurostat's `b`/`d` are break and
definition metadata that belong beside `series_break`, not inside a status enum.
Requires a schema migration.

## 2. Q3 — pre-rendering under the golden rule (the central architectural question)

Two events are decoupled today. Content changes when the pipeline ingests (VPS, Go, no
Node). The site is built when someone pushes (CI, Node, no database). Nothing bridges
them, and PRD §14.2 forbids closing the gap at request time.

| Approach | Pros | Cons | Effort |
|---|---|---|---|
| **A. Go renders everything at ingest** | Correct trigger by construction; no CI coupling; no export format to keep in sync; one renderer feeds both pages and OG images | Discards Astro/Svelte for the core page, contradicting §14.2 and ADR-6's premise; no Go equivalent of Observable Plot; the Svelte island still needs Node in CI anyway | High |
| **B. Astro builds from a build-time export; ingest fires a CI rebuild** | Keeps the decided stack exactly; D3/Observable Plot available; Tailwind works normally; `/data-derived` falls out for free, discharging the P5 obligation deferred in Fase 0; the site becomes reproducible from a public artifact, which is literally P5's promise | Publish path acquires a GitHub Actions dependency and multi-minute latency the PRD never contemplated; a failed CI run leaves stale pages against fresh database data, extending §6.1.3's banner semantics | Medium |
| **C. Astro builds the shell; Go splices data fragments at ingest** | Keeps Astro, Tailwind and Svelte; publish latency in seconds | Two renderers must agree on identical SVG geometry, or Go owns the chart — which is A plus fragile template splicing | High |

**Recommendation: B**, with a stated latency budget. It is the only option that preserves
the §14.2 architecture already decided, discharges P5, and avoids inventing a Go charting
stack. Publish latency measured in CI minutes is acceptable against §19.3's under-24-hour
freshness target.

Consequence to carry into the spec: under B the amber freshness semaphore is itself a
build-time snapshot, so it must be able to mean "we hold newer data than this page shows"
as well as "the source is pending".

**This warrants ADR-7, decided before `sdd-design`.**

## 3. Q1 — chart scoping

Not one component. One hydrated Svelte island (tooltips, keyboard navigation, range
presets, transformation toggles) plus four static Astro components: the SVG renderer with
break bands, the accessible data table, the generated textual description, and the
break/annotation partials.

Ship one pre-rendered SVG plus a compact series JSON (roughly 2–4 KB gzipped for the
longest 294-point series) and redraw client-side on toggle.

§14.2's "cero cómputo" constrains the **server's request path**, not the browser — the
import guard already encodes exactly that distinction.

## 4. Q2 — which transformations actually exist

- **"Real" does not exist for any of the six.** None is a monetary magnitude: a rate, a
  headcount, two price indices, an already-real volume index, and a population. The
  entire deflator apparatus is unnecessary for milestone 1.2.
- **"Median" does not exist** for any of the six either.
- **Year-on-year variation is mandatory, not optional**, for four of them: the configs
  store the IPC and PIB *indices*, while §7's headline indicators are the *annual rates*.

### Blocker found — the population series has the wrong cadence recorded

`poblacion-residente` (ECP320) is **semiannual across its historical span** — verified
1977–1980 carrying only `"1 de enero de"` and `"1 de julio de"` — and quarterly only
recently (verified continuous 2023-Q3 → 2026-Q2). The config declares `frequency: Q`.

Two Fase 0 guard gaps let this through:
1. `envelope.go:58` inspects `wr.Data[0].Periodo` only, so periodicity is judged from a
   single observation.
2. `Rule2Continuity` returns nil on the first run and otherwise walks forward from
   `priorLatest` alone, so it never audits the historical interior.

The stored population series therefore has Q2/Q4 missing for decades. §6.1.2 forbids
inventing the denominator, so per-capita cannot simply interpolate.

## 5. Q4 — Tailwind theme (ADR-6)

Put the theme in a single CSS-first `@theme` block and **explicitly zero Tailwind's
defaults** (`--color-*: initial;` and equivalents). That makes ADR-6's first constraint
mechanically enforced: `bg-blue-500` then generates no class at all, so the mistake is
loud instead of shipped.

Define dark mode as a second explicit token set, never derived, since §12.5 requires
measured AA in both.

## 6. Q6 — frontend testing

- `npm --prefix web test` — Vitest with Astro's `experimental_AstroContainer` for SVG
  output and design-token guards.
- `npm --prefix web run test:e2e` — Playwright with `@axe-core/playwright`, keyboard
  navigation, 44 px touch targets, and a `javaScriptEnabled: false` context proving
  §14.2's no-JS clause.
- Blocking Lighthouse CI with an explicit under-300-KB-transferred assertion (§12.3).
- `openspec/config.yaml` lines 87 and 90 must be updated with the web test command.

Strict TDD applies honestly to the Go publishing layer and to pure render and transform
functions. Playwright and Lighthouse are acceptance gates written alongside the work, not
before it; forcing red-first onto them produces theatre.

## 7. Q7 — Fase 0 leftovers this change touches

- The archive report's **W16 is wrong**: `postgres.SourceFreshness` is not dead code —
  `SeriesFreshness` calls it at `freshness.go:65`. Neither has a UI consumer, and this
  page is the first.
- `ReconcileEvents` / `ReconcileBreaks` should be deleted, but note that this change needs
  a **read** path for breaks and events which does not exist yet.

## 8. Risks

1. Q3 is unresolved architecture, not a detail. Needs ADR-7 before `sdd-design`.
2. The population cadence defect blocks per-capita and exposes two Fase 0 guard gaps.
   Fixing them may reject data that currently ingests, turning a green pipeline red.
3. The status backfill produces version-2 rows caused by our own bug, visible in the
   public vintage history. P7 says disclose, not erase.
4. Review budget: Fase 0's measured multiplier was 3.4× against a 400-line budget. A
   design system, component library, chart, pre-render layer and a new test stack need
   stacked slices — expect 6–10, not one.
5. Strict TDD does not map onto Playwright and Lighthouse; forcing it produces theatre.
6. Milestone 1.2's annotation layers *are* the events UI, which depends on the seven
   still-unconfirmed editorial dates recorded in the Fase 0 archive report.

## 9. Decisions taken by the orchestrator

**Pre-render architecture: B.** Astro builds from a build-time export; ingestion fires a
CI rebuild. Recorded as ADR-7 before design. It preserves the decided §14.2 stack,
discharges P5's public derived-data obligation, and avoids inventing a Go charting stack.
The latency budget and the extended amber-banner semantics must be specified.

**Per-capita: fix the cadence defect first, then implement over the span where the
denominator genuinely exists**, with the attribution rule stated in the methodology sheet
and flagged for editorial sign-off. Dropping the toggle entirely was rejected because
principle P4 requires per-capita alongside every aggregate; interpolating the missing
semiannual periods was rejected because §6.1.2 forbids inventing the denominator.

**Colour: the prohibition is LIFTED — see ADR-8 (`docs/adr/0008-free-colour-palette.md`),
PRD amended to v2.4.** The product owner directed that the party-colour restriction be
dropped entirely: palette choice is an ordinary design decision with no vetoed hues and
no neutrality analysis. Section 8's "colour space is narrower than §12.1 states" risk is
withdrawn, and the research this exploration did on Spanish party hues is superseded —
retained above only as the record of why the restriction existed.

Hue is available as a categorical channel again. Milestone 1.2 renders one series per
page, so this barely binds here; it becomes real in the international comparator (§6.3,
Fase 2).

What does NOT change, because it is accessibility rather than preference (§12.5): colour
is never the sole channel distinguishing a series — line pattern or marker shape always
carries it too — and AA contrast is measured in both light and dark themes rather than
derived. The reserved semantics also stand, because they carry meaning rather than
identity: amber = pending data, dotted grey = provisional.

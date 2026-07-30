# ADR-7 — Pre-rendering: Astro builds from a build-time export, ingestion triggers the rebuild

- **Status**: Accepted
- **Date**: 2026-07-29
- **Context**: PRD Fase 1, change `phase-1-indicator-page`
- **Required by**: PRD §14, "Cambios posteriores requieren ADR"

## Context

PRD §14.2 states the golden rule as non-negotiable: **zero database queries and zero
computation at page-request time**. All HTML and SVG is pre-rendered when the pipeline
ingests new data. The site's speed comes from serving static files from a CDN, not from
the database or a framework.

Fase 0 built the pipeline and Fase 0 built nothing that renders. The two halves of the
system are currently decoupled in a way nothing bridges:

- **Content changes when the pipeline ingests.** That happens on the VPS, in Go, with a
  database and no Node.
- **The site is built when someone pushes.** That happens in CI, in Node, with Astro and
  no database.

Nothing connects an ingestion to a rebuild, and §14.2 forbids closing the gap at request
time. This is the central architectural question of Fase 1 and no existing spec answers
it.

## Options considered

**A. Go renders everything at ingest.**
The trigger is correct by construction, there is no CI coupling and no export format to
keep in sync, and one renderer could feed both pages and OG images.
Rejected: it discards Astro and Svelte for the core page, contradicting §14.2's decided
frontend and the premise of ADR-6; there is no Go equivalent of D3 or Observable Plot, so
the charting stack would have to be invented; and the Svelte island still requires Node in
CI regardless, so the second ecosystem is not actually eliminated.

**B. Astro builds from a build-time export; ingestion fires a CI rebuild. CHOSEN.**

**C. Astro builds the shell; Go splices data fragments at ingest.**
Keeps the decided stack and gives publish latency in seconds.
Rejected: two renderers must then agree on identical SVG geometry, or Go owns the chart —
which is option A plus fragile template splicing.

## Decision

The pipeline exports the published series, their metadata, breaks and events as a
build-time artifact. Astro builds the site from that artifact. A successful ingestion
triggers the rebuild.

## Consequences

**Positive**

- The §14.2 architecture already decided is preserved exactly: Astro with Svelte islands,
  build-time only, no Node in production, one Go binary and PostgreSQL on the VPS.
- D3 and Observable Plot remain available for chart generation, as §14.2 anticipated.
- Tailwind works normally, as ADR-6 assumes.
- **`/data-derived` falls out for free.** PRD §14.2 lists it in the monorepo layout and
  Fase 0 deferred it; the export artifact IS that data. This discharges principle P5's
  public-derived-data obligation as a by-product rather than as extra work.
- The site becomes reproducible from a public artifact by anyone — which is literally
  what P5 promises when it says the answer to an accusation of manipulation is a link.

**Negative, and accepted**

- The publish path acquires a GitHub Actions dependency and a latency measured in CI
  minutes. The PRD never contemplated this. It is acceptable against §19.3's freshness
  target of under 24 hours for API-backed sources, but it MUST be stated as a budget
  rather than discovered.
- A failed CI run leaves stale pages against fresh database data. This extends §6.1.3's
  banner semantics: the amber freshness indicator is itself a build-time snapshot, so it
  must be able to express **"we hold newer data than this page shows"** as a distinct
  state from **"the source has not published yet"**. Conflating the two would tell a
  reader the source is late when in fact our own publish step is.

**Neutral**

- The export format becomes a versioned contract between the Go pipeline and the Astro
  build. It needs a schema and a validation step, or it becomes the next silent-drift
  surface — Fase 0 produced eight instances of components that were built, tested and
  never connected, and an unvalidated hand-off between two ecosystems is exactly that
  shape.

## Scope

This decision governs Fase 1 onward. It does not alter anything Fase 0 shipped; the
pipeline, its validation and its storage are unchanged. What it adds is the publishing
layer that Fase 0's design reserved at `app/internal/publishing/` and never built.

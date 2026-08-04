# Tasks: phase-1-indicator-page

## Spec-vs-Design Reconciliation (read before Slice 1)

Spec and design were authored in parallel. Per orchestrator instruction, the **spec is the contract**
wherever the two disagree; design.md must be corrected to match. Six areas were diffed.

| # | Area | Finding | Resolution required |
|---|---|---|---|
| D1 | Export artifact shape | Spec's "Provenance survives the export" and "artifact declares its version and contents" requirements make per-observation `ingestion_run_id` and raw-file SHA-256 mandatory-reachable fields. Design's illustrative `series/{slug}.json` schema carries `vintage.ingestionRunId` once per series doc and no raw-file hash at all — no per-observation provenance link. **Real gap.** | Task 3.1: add a version→{ingestion_run_id, extracted_at, raw_file_sha256} lookup (or equivalent) to the artifact model before the schema is finalized; update design.md's schema sketch. |
| D2 | `status`/`source_status` split | Spec (`data-model-vintages`) and design D-3 agree exactly: `status` stays P/D/W, nullable `source_status` holds the verbatim token. **No divergence.** | None — proceed as designed. |
| D3 | Periodicity-segment fix | Spec (`source-ingestion-ine`, `data-validation`, `editorial-config`) repeatedly calls the historical span's cadence "**semiannual**" and requires each segment to carry "**its own cadence**". Design's illustrative config instead keeps every segment at `frequency: Q` and represents semiannuality via a `present: [Q1, Q3]` filter on the quarterly grid, explicitly rejecting a distinct semiannual cadence value in its "Alternatives considered". **Real gap** — design's chosen encoding may not satisfy a literal reading of "each segment with its own cadence" / "declared segmented cadence" audited "under its own cadence". | Task 1.10: resolve before implementing `cadence_segments` — either the segment's `cadence` field must be able to hold a genuine `semiannual` label (minimal addition, not a full `FrequencySemiannual` domain type: a per-segment cadence tag is enough to satisfy "each segment MUST be audited under its own cadence" without re-labelling stored rows), or design.md must be corrected to state explicitly that "own cadence" is satisfied by the quarterly-grid-plus-presence-list shape and why that reading is intended. Do not silently pick one — record the decision in design.md before slice 1 lands. |
| D4 | Chart island/static split | `design-system` spec requires all eight named components (incl. `break band`, `annotation chip`, `accessible data table` as separate catalog entries) to render independently in a documented workbench with their own props/state variants. Design's D-5/D6 describe the chart as "one island + four static Astro components (SVG renderer with break bands, accessible data table, generated textual description, break/annotation partials)" — a decomposition that risks fusing `break band` and `annotation chip` into one internal partial not independently workbench-renderable, and adds "generated textual description" as a component with no home in the eight-item catalog. **Structural ambiguity, not a hard contradiction.** | Task 6-note and 7 tasks: confirm and document explicitly that `BreakBand.astro`, `AnnotationChip.astro` and `AccessibleDataTable.astro` are independently workbench-instantiable (slice 6) before slice 7 composes them into `IndicatorChart.astro`; textual description is implemented as a chart-internal generator with no standalone workbench entry, which is consistent with the spec as long as it is not miscounted as a ninth catalog component. **Resolved (slice 6, `sdd-apply`):** the spec's reading won — `BreakBand.astro`, `AnnotationChip.astro` and `AccessibleDataTable.astro` are each their own top-level component, each with its own workbench section, own props interface and own `experimental_AstroContainer` test; none is a chart-only internal partial. `BreakBand` has NO prop that could hide it (P4's non-dismissibility is unrepresentable by construction, verified by an executable test asserting its exact 4-key prop surface). The generated textual description is deliberately deferred to slice 7 (chart-internal, no standalone workbench entry) — not built this slice, not miscounted. |
| D5 | Tailwind theme structure | Spec requires exactly four stock scales zeroed: colours, type scale, border radii, shadows. Design's `@theme` code block zeros five properties (`--color-*`, `--font-*`, `--text-*`, `--radius-*`, `--shadow-*`) under a comment reading "the four forbidden stock scales" — an internal miscount, not a contract violation (zeroing more than the floor is permitted). | Task 5.3: zero all five as designed (permitted, non-blocking), but fix the comment/count mismatch and confirm the extra `--font-*` zeroing is intentional before merging. |
| D6 | Per-capita sub-span disclosure copy | Both spec and design require the covered-span disclosure to be shown "in Spanish" but neither fixes the literal string, unlike every other reader-facing string in this change (semaphore label, validation banner), which are quoted verbatim. **Real gap — copy does not exist yet.** | Task 8.4: draft and get editorial sign-off (settled decision D4) on the exact Spanish per-capita disclosure copy before slice 8 closes; externalise in `web/src/i18n/es.ts`. |

Orchestrator's five settled ambiguities (mandatory YoY set of four, 30-min publish budget as config, per-capita
sub-span toggle semantics, empty annotation group has no control, spec size) are treated as already resolved
in both spec and design and are not re-litigated below.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 12,000–16,000 (honest arithmetic per proposal; Fase 0 ran 1.5–3× low on similar estimates because tests, the larger half under Strict TDD, were under-counted) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | 11 work units, PR 1 → PR 11 (see below) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

```text
Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High
```

Target **1,200–1,500 authored lines per slice at coherent boundaries** (per orchestrator decision), not
30+ micro-PRs held to a strict 400. Any slice measured or forecast above ~2,000 lines during `sdd-apply`
must split further before merge.

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary | Est. lines |
|---|---|---|---|---|---|---|
| 1 | Periodicity-over-full-payload + Rule2 interior audit + `poblacion-residente` cadence correction | PR 1 | `go test ./app/internal/adapters/ine/... ./app/internal/ingestion/validation/... ./app/internal/adapters/config/...` | Full-history ingestion of all six series against recorded fixtures | Revert PR; config/guard changes are additive-strict (fail closed), no data written differently until re-ingested | ~900 |
| 2a | INE `TipoDato` + `source_status` migration + status mapping | PR 2 | `go test ./app/internal/adapters/ine/... ./app/internal/adapters/postgres/... ./app/internal/ingestion/...` | Migration up/down against testcontainers-go populated DB | `down` migration is reversible and additive; revert PR leaves prior status-only behavior | ~950 |
| 2b | Eurostat status/break-flag decoding | PR 3 | `go test ./app/internal/adapters/eurostat/...` | Fixture-based decode of a recorded JSON-stat response with mixed flags | Revert PR; no schema change, decode-only | ~500 |
| 3 | Export contract schema + `publishing.Export` core + write-side validation + golden fixture | PR 4 | `go test ./app/internal/publishing/... ./app/internal/adapters/postgres/... ./app/internal/cmd/concontexto/...` | `go run ./app/cmd/concontexto export --fixture` | Package is new and unreferenced by `ingest_cmd.go` until PR 5; revert removes only the new package | ~1,500 |
| 4 | Breaks/events read path + `/data-derived` CSV + rebuild dispatch + latency watchdog + e2e ingest→build CI job | PR 5 | `go test ./app/internal/publishing/... ./app/internal/adapters/github/... ./app/internal/scheduler/...` | New CI job: ingest fixtures → export → validate → astro build | Revert PR; `Publish()` wiring into `ingest_cmd.go` is the only production call site touched | ~1,400 |
| 5 | Tailwind `@theme`, zeroed defaults, palette, dual tokens, contrast harness, dependency guard, Vitest bootstrap, Zod schema for the artifact | PR 6 | `npm --prefix web test` | N/A — no user-facing runtime yet, pure build-output/token assertions | Additive, unreferenced by any route until PR 8; revert removes `web/src/styles/theme.css` and test files only | ~1,200 |
| 6 | Seven static components + documented workbench | PR 7 | `npm --prefix web test` | Workbench build under `WORKBENCH=1` | Additive, unreferenced by production pages until PR 9/10; revert removes `web/src/components/*` (7 files) | ~1,300 |
| 7 | Static chart: geometry/transform pure functions, `IndicatorChart.astro`, break bands, annotations (static/default state), data table wiring, textual description | PR 8 | `npm --prefix web test` | Vitest golden SVG comparison | Additive, unreferenced by production pages until PR 10; revert removes `web/src/lib/chart`, `web/src/lib/transform`, `IndicatorChart.astro` | ~1,700 |
| 8 | Chart island: tooltips, keyboard nav, range presets, YoY/QoQ/per-capita toggles, island-parity golden | PR 9 | `npm --prefix web test` | Manual `client:idle` hydration check in a local Astro preview | Additive, unreferenced until PR 10/11; revert removes `ChartIsland.svelte` and per-slug transform config | ~1,700 |
| 9a | Three indicator pages (`tasa-de-paro-epa`, `ocupados-epa`, `poblacion-residente`), export loader, page states, Playwright/axe scaffold for these three | PR 10 | `npm --prefix web test && npm --prefix web run test:e2e` | Local `astro build` + Playwright run against the 3 pages | Revert PR; these 3 routes are new, no cross-page coupling | ~1,900 |
| 9b | Remaining three pages, full axe/no-JS/keyboard/44px gate over all six, blocking Lighthouse budget gate, contrast-in-gate, redirect infra, `openspec/config.yaml` web commands | PR 11 | `npm --prefix web test && npm --prefix web run test:e2e` | Full CI run: axe + Lighthouse + no-JS context over all six built pages | Revert PR; last slice, closes all milestone 1.1/1.2 exit criteria | ~1,900 |

---

## Slice 1 — Prerequisite: periodicity over full payload + Rule 2 interior audit + population cadence

**Blocking, first.** Fixing these guards may reject series that currently ingest — the pipeline going red
is the correct outcome, not a regression (proposal risk, data-validation "Fixing the guard may reject data
that previously ingested").

Files: `app/internal/adapters/ine/{envelope,periodicity}.go`, `app/internal/ingestion/validation/rule2_continuity.go`,
`app/internal/adapters/config/`, `config/series/poblacion-residente.yaml`.

- [x] 1.1 RED — table-driven test asserting periodicity classification reads the **whole payload**, not `Data[0]` alone (`source-ingestion-ine`: "A periodicity mismatch fails the run", "A single quarterly declaration fails against a mixed history", "A declared segmented cadence matching the payload proceeds").
- [x] 1.2 GREEN — implement full-payload segmentation in `ine/periodicity.go`; diagnostic prints the observed segmentation. (Implemented as the shared `indicators.AssertCadence`, reused verbatim by `ine/envelope.go`, `eurostat/envelope.go` and `Rule2Continuity` — see design.md D-4.)
- [x] 1.3 RED — `cadence_segments` config parse/validate test: ordered, non-overlapping, no gap, boundary aligns to both adjoining cadences, uniform cadence still validates (`editorial-config` scenarios).
- [x] 1.4 GREEN — implement `cadence_segments` YAML schema + `validate-config` checks in `app/internal/adapters/config/`.
- [x] 1.5 RED — `Rule2Continuity` test: first-run interior gap fails (no more `!hadPrior → nil`); allowlisted gap passes; segment-aware expectation; declared-vs-observed cadence mismatch fails closed (`data-validation` scenarios).
- [x] 1.6 GREEN — rewrite `app/internal/ingestion/validation/rule2_continuity.go` to audit the whole span per cadence segment.
- [x] 1.7 Correct `config/series/poblacion-residente.yaml` per **D3 resolution** (see reconciliation table), reading the boundary off the guard's observed-segmentation output on the first corrected run. Document the disclosed, unenforced four-eyes gap on `/config/**` in the commit message — no workaround, per design's Migration/Rollout note. **Disclosed limitation**: this `sdd-apply` session had no live network access; the boundary (`2023-Q3`) is the earliest live-verified continuous-quarterly point recorded in exploration.md, not a fresh live-run confirmation — see design.md's Open Questions.
- [x] 1.8 Triage — run full-history ingestion against fixtures for all six pinned series with corrected guards; disclose (P7) any series beyond `poblacion-residente` that now fails; per proposal Q5, affected series block only, others proceed. (See apply-progress for the full triage result.)
- [x] 1.9 Verify: `go test ./app/internal/adapters/ine/... ./app/internal/ingestion/validation/... ./app/internal/adapters/config/...`.
- [x] 1.10 **Resolve D3** (periodicity-segment cadence representation) before/alongside 1.3–1.4; update design.md. (Orchestrator adjudication supplied pre-settled; implemented and recorded in design.md D-4.)

---

## Slice 2a — Prerequisite: INE `TipoDato` + `source_status` migration + status mapping

Files: `app/internal/adapters/ine/envelope.go`, `app/internal/indicators/{observation,ports}.go`,
`app/internal/ingestion/ingest.go`, `app/migrations/0003_observation_source_status.{up,down}.sql`,
`app/internal/adapters/postgres/observation.go`.

- [x] 2a.1 RED — `wireObservation`/`ine.Observation` test: `T3_TipoDato` decodes to a typed token; `Secreto` no longer declared (`source-ingestion-ine`: "No phantom field survives").
- [x] 2a.2 GREEN — wire `T3_TipoDato` through envelope → adapter type → domain type; delete `Secreto`.
- [x] 2a.3 RED — migration test: applies to a populated DB, all rows retain value/status, `source_status` null until next ingestion, `down` reverses without data loss (`data-model-vintages` scenarios).
- [x] 2a.4 GREEN — write `app/migrations/0003_observation_source_status.{up,down}.sql` (additive, nullable, reversible).
- [x] 2a.5 RED — `postgres` round-trip + `WriteRevision` test: status-only transition appends version 2, does not trip `Rule4Revision` (`data-model-vintages` scenarios).
- [x] 2a.6 GREEN — extend `postgres/observation.go` types, columns, `WriteRevision` diff for `source_status`.
- [x] 2a.7 RED — `ingest.go` mapping test: `"Definitivo"`→D, `"Provisional"`→P, unrecognised token → `sourceerr.SchemaDrift` naming series+token, missing token fails rather than defaults (`source-ingestion-ine` scenarios). (Classification itself lives in `ine.Client.Decode`, called from `ingest.go`; see apply-progress for why — the RED/GREEN test pair lives in `ingest_tipodato_test.go`, proving the pipeline end-to-end as the task names it.)
- [x] 2a.8 GREEN — replace `ingest.go:248`'s hardcoded `StatusDefinitive` with the mapping from 2a.7.
- [x] 2a.9 Confirm the next scheduled ingest backfills via ordinary version-2 rows and the run log records the internal-defect cause (`data-model-vintages`: "A backfill caused by our own defect is disclosed, not erased"). (Confirmed, no new code needed: `WriteRevision`'s existing value-or-status diff already appends version 2 for any backfilled status; `logAndAlertRun` already records every run. No bespoke backfill mechanism — matches design D-3 "Backfill: none bespoke".)
- [x] 2a.10 Verify: `go test ./app/internal/adapters/ine/... ./app/internal/adapters/postgres/... ./app/internal/ingestion/...`.

---

## Slice 2b — Prerequisite: Eurostat status/break-flag decoding

Files: `app/internal/adapters/eurostat/envelope.go`.

- [x] 2b.1 RED — decoder test: `status` read at computed `pos`, verbatim flag on `source_status`, alignment across non-contiguous positions.
- [x] 2b.2 GREEN — implement `status`-at-`pos` reading.
- [x] 2b.3 RED — absence-means-definitive test: no entry → D + null `source_status`; empty status map → all definitive.
- [x] 2b.4 GREEN — implement absence-as-definitive mapping.
- [x] 2b.5 RED — `b`/`d` routing test: routed to break/definition metadata, never into status enum; unrecognised flag → `SchemaDrift` naming dataset+flag.
- [x] 2b.6 GREEN — emit `SourceResult.BreakSignals []{Period, Flag}`; `ingest.go` logs and alerts when no active `series_break` covers the period (never writes `series_break` itself).
- [x] 2b.7 Verify: `go test ./app/internal/adapters/eurostat/...`.

---

## Slice 2c — Corrective: `adapters/xlsx` status classification (closes the Open Question slice 2b raised)

Not part of the original 11-unit plan — a small, contained corrective slice added after slice 2b
disclosed that closing `ingest.go`'s compatibility shim leaves `adapters/xlsx` (the `afiliacion-ss`
source) with no status classification of its own, so a real, successfully-decoded XLSX ingest would
BLOCK on `firstUnclassifiedStatus` rather than publish.

Files: `app/internal/adapters/xlsx/decode.go`, `app/internal/ingestion/xlsx_status_test.go` (new).

- [x] 2c.1 RED — `IngestSeries` test with a clean (non-malformed) XLSX workbook reaching the candidate
      loop: asserts the run publishes rather than blocking on `firstUnclassifiedStatus`
      (`TestIngestSeries_XLSXSuccessfulDecodePublishesDefinitiveWithNullSourceStatus`); confirmed
      failing against the pre-fix adapter.
- [x] 2c.2 GREEN — `xlsx.Decode` sets `Status: indicators.ObservationStatusDefinitive` on every parsed
      observation, `SourceStatus` left empty (maps to NULL); documented in `decode.go` as a positive
      statement about the source (no status concept), not a fallback.
- [x] 2c.3 Document the three-source status contrast (INE fail-closed / Eurostat absence-is-definitive /
      XLSX no-status-concept) in `decode.go` and `design.md` D-3.
- [x] 2c.4 Resolve design.md's Open Question slice 2b raised (`adapters/xlsx` does not classify `Status`).
- [x] 2c.5 Mutation check: revert 2c.2's fix, confirm 2c.1's test fails, restore.
- [x] 2c.6 Verify: `go test -count=1 ./...`, `go test -short -count=1 ./...`, `go vet ./...`, `gofmt -l .`,
      `./scripts/check-env-example.sh`, `go run ./app/cmd/concontexto validate-config`.

---

## Slice 3 — `publishing-export`: schema, `Export` core, write-side validation, golden fixture

Files: `app/internal/publishing/{export,artifact,validate}.go`, `app/internal/adapters/postgres/published_series.go`,
`app/cmd/concontexto/export_cmd.go`.

- [x] 3.1 **Resolve D1** (export artifact shape) — add a version→{ingestion_run_id, extracted_at, raw_file_sha256} lookup to the artifact model so per-observation provenance is reachable without a further DB query; update design.md's schema sketch. (Implemented as a per-run `vintages` map keyed by `ingestion_run_id`, an explicitly-permitted "equivalent" to a literal per-version lookup — see design.md's corrected "Export artifact schema (as-built ...)" section.)
- [x] 3.2 RED — `publishing.ValidateArtifact` table-driven tests: accepts well-formed artifact; rejects unresolved break/event reference, non-P/D status, cadence-inconsistent period (`publishing-export`: "An invalid artifact is never written").
- [x] 3.3 GREEN — implement `app/internal/publishing/artifact.go` + `validate.go`.
- [x] 3.4 RED — `Export()` test with fake ports: only max-version published data enters the artifact; a failed run's suspect datum is absent while the prior valid value remains; provenance resolves per observation.
- [x] 3.5 GREEN — implement `Export(ctx, deps, asOf)` via `ListPublishedSeries` (new), `ListPublishedObservations` (new — see apply-progress for why this port is NOT the existing `ListCurrentObservations`, D1's explicit resolution), `SeriesFreshness`.
- [x] 3.6 RED — freshness-in-artifact test: fresh vs source-pending derive only from source calendar + held observations; no build-staleness field emitted. (Reuses the existing `postgres.SeriesFreshness`/`freshness.State`, relabelled via `publishing.ArtifactFreshness` — design D-2's explicit mechanism; see apply-progress for the reasoned reading of this task's wording.)
- [x] 3.7 GREEN — wire `SeriesFreshness` into the writer.
- [x] 3.8 RED — atomic-write test (`t.TempDir()`): a validation failure aborts, writes nothing, previous artifact stays byte-identical.
- [x] 3.9 GREEN — implement tmp-write + atomic rename. (Per-file atomic rename, not a whole-directory swap — `os.Rename` cannot atomically replace a non-empty existing directory on POSIX; disclosed, narrower atomicity boundary documented in `export.go`'s own doc comment.)
- [x] 3.10 RED/GREEN — `app/internal/adapters/postgres/published_series.go`: `ListPublishedSeries` (metadata joined with active source mapping) + `ListPublishedObservations` (current vintage joined with provenance), testcontainers-go.
- [x] 3.11 RED/GREEN — `app/cmd/concontexto/export_cmd.go`: standalone `concontexto export` subcommand (recovery, boot self-heal, fixture generation). Wired into `main.go`'s `realCommands()` as a sixth subcommand; `main_test.go`'s guard test updated accordingly — disclosed as a `platform-runtime` spec-maintenance gap in design.md's Open Questions, not a silent deviation.
- [x] 3.12 Generate the golden artifact fixture (`web/test/fixtures/export/`) via `go run ./app/cmd/concontexto export --fixture`, following the repo's golden-update path; commit it. (Generated against a throwaway local Postgres container seeded with one realistic series, `tasa-de-paro-epa`, including a revision (version 1→2) and a status transition — not all six pinned series, kept small and reviewable; not committed per this session's explicit "do NOT commit" instruction, left as a tracked-on-disk file for the orchestrator/user to commit.)
- [x] 3.13 Verify: `go test ./app/internal/publishing/... ./app/internal/adapters/postgres/... ./app/cmd/concontexto/...`.

---

## Slice 4 — `publishing-export`: breaks/events read path, `/data-derived`, rebuild trigger, latency budget

Files: `app/internal/adapters/postgres/events_read.go`, `app/internal/publishing/csv.go`,
`app/internal/adapters/github/dispatch.go`, `app/internal/publishing/trigger.go`, `app/cmd/concontexto/ingest_cmd.go`,
`app/internal/scheduler/`, `.github/workflows/`.

- [x] 4.1 RED — `ListActiveEvents` test: family-scoped break resolves once per member, retired break excluded, unconfirmed editorial entry skipped without error and counted for operators. (Breaks half — family-scoping, retired exclusion — already existed via `ResolveActiveBreaksForSeries`/`ReconcileBreaks`, predating this change's own numbering; this slice's RED covers the new `ListActiveEvents` half — `events_read_test.go`.)
- [x] 4.2 GREEN — implement `app/internal/adapters/postgres/events_read.go`.
- [x] 4.3 RED — CSV projection test: every `/data-derived` row matches the artifact value-by-value, regenerated whenever the artifact is.
- [x] 4.4 GREEN — implement `app/internal/publishing/csv.go` (one file per series, licence/attribution header comment).
- [x] 4.5 RED — `publishing.Publish` test: ≥1 published series triggers `Export` + dispatch; instants recorded; a failed ingestion dispatches nothing. (`trigger_test.go`.)
- [x] 4.6 GREEN — implement `Publish()`; wire into `ingest_cmd.go` after a cycle with `GateApplyResult.Published` non-empty.
- [x] 4.7 RED — `adapters/github/dispatch.go`: `repository_dispatch` POST with fine-grained token; dispatch failure alerts, never retries, never fails the ingest.
- [x] 4.8 GREEN — implement the dispatcher adapter behind the `publishing.Dispatcher` port.
- [x] 4.9 RED — artifact-retention test: last N (configurable, ≥5) retained; rebuild from retained artifact reproduces byte-identical output. (`app/internal/publishing/retention_test.go`.)
- [x] 4.10 GREEN — implement retention. (`app/internal/publishing/retention.go`: `ArchiveArtifact`, called from `Publish` (trigger.go) right after a successful `Export`; wired into production via `runIngest`'s publish block, `retentionHistoryDir()`/`retainedArtifacts()` in `ingest_cmd.go`. See apply-progress for the D9 reasoning on where retained history lives.)
- [x] 4.11 RED — publish-latency watchdog test: budget from config, default 30 min; stalled rebuild alerts naming series/run/elapsed; within-budget cycle raises nothing; condition never reaches a reader field (`pipeline-operations` scenarios). (`app/internal/scheduler/watchdog_test.go` for the pure decision; `app/cmd/concontexto/schedule_test.go`'s `TestRunScheduler_InvokesWatchdogEveryTickOnceASuccessIsKnown` and `schedule_composition_test.go`'s `TestStartSchedulerLoop_APublishLatencyBreachAlertsWhenTheLocalExportNeverCaughtUp` for the real wiring, the latter mutation-checked.)
- [x] 4.12 GREEN — implement the scheduler watchdog (live `manifest.generated_at` vs newest local export). (`app/internal/scheduler/watchdog.go`: `PublishLatencyBreached` (pure decision); `app/cmd/concontexto/schedule.go`: `publishLatencyWatchdog`/`publishLatencyBudget`, wired into `startSchedulerLoop`'s `runScheduler` call. `alerting.PublishLatencyBreach` now has a real production call site — see apply-progress for the disclosed source-level-vs-series-level reading.)
- [x] 4.13 Write the one end-to-end CI job: ingest fixtures → export → validate → astro build succeeds; corrupted artifact fails the build (`publishing-export`: "Ingest, export and build run as one test") — the structural fix for the eight-instance "built, tested, never connected" pattern. New `.github/workflows/` file. (`.github/workflows/ingest-export-build.yml` + `app/internal/ingestion/e2e_export_test.go`'s `TestEndToEndIngestExportBuild`; "corrupted artifact fails the build" only partially closed — disclosed in design.md's Open Questions, the Astro-side loader does not exist until slice 9a.)
- [x] 4.14 Add an export-usage check: every new exported function under `app/internal/publishing/` has a production call site (`ingest_cmd.go`, `export_cmd.go`, or the scheduler) before this slice closes — verify via `go vet`/unused-export tooling or manual review; record the result in the PR description. (Confirmed by manual review — see apply-progress. `alerting.PublishLatencyBreach` now also has a real production call site, `app/cmd/concontexto/schedule.go`'s `publishLatencyWatchdog`, closed this batch — task 4.11/4.12.)
- [x] 4.15 Verify: `go test ./app/internal/publishing/... ./app/internal/adapters/github/... ./app/internal/scheduler/...`; run the new CI e2e job locally. (Ran the broader `go test -count=1 ./...`/`-short` per this session's own instructions — see apply-progress for verbatim output.)

---

## Slice 5 — `design-system`: Tailwind `@theme`, palette, dual tokens, contrast harness, Vitest bootstrap

Files: `web/` scaffold (`@astrojs/svelte`, Tailwind Vite plugin), `web/src/styles/theme.css`,
`web/src/lib/export/schema.ts`, `web/package.json`.

Strict-TDD note: the token guard, dependency guard and contrast harness assert compiled CSS / `package.json` /
computed contrast — a genuinely unit-testable surface. These ARE red-first; there is no ceremony exception here.

- [x] 5.1 Scaffold `web/`: `@astrojs/svelte`, Tailwind via Vite plugin, Vitest with `experimental_AstroContainer`. (`astro.config.mjs`, `svelte.config.js`, `vitest.config.ts`; smoke-tested by `test/smoke/home.container.test.ts`.)
- [x] 5.2 RED — token-guard test: a stock Tailwind class (e.g. `bg-blue-500`, `text-sm`) compiles to **no CSS rule** (`design-system`: "A stock palette class emits no rule", "Stock type scale, radii and shadows are equally zeroed"). (`test/design-system/token-guard.test.ts`; mutation-checked — disabling the zeroing directives flips 17/19 assertions red, restored.)
- [x] 5.3 GREEN — author `web/src/styles/theme.css`: one `@theme` block zeroing `--color-*`, `--text-*`, `--radius-*`, `--shadow-*` (and `--font-*` — resolve **D5** by fixing the "four forbidden stock scales" comment/count before merging) plus project tokens (`--color-bg`, `--color-surface`, `--color-ink`, `--color-accent`, `--color-pending` = amber, `--color-provisional` = grey, `--font-sans`, `--font-numeric`). (D5 resolved: zero all five, comment fixed — see design.md D-6.)
- [x] 5.4 RED — dark-token-parity test: every light semantic token has an explicit dark counterpart, no dark value computed from light. (`test/design-system/dark-token-parity.test.ts`, scoped to `--color-*` — see its own scope-note comment; mutation-checked, restored.)
- [x] 5.5 GREEN — author `[data-theme="dark"]`, re-stating every token explicitly.
- [x] 5.6 RED — contrast harness test: every text/background/tooltip/chart-stroke pairing meets 4.5:1 (body) / 3:1 (large text, non-text graphics) in both themes; a breaking token change fails naming pairing + theme. (`test/design-system/contrast.test.ts` + `src/lib/design-system/contrast.ts`; mutation-checked, failure message names pairing+theme, restored.)
- [x] 5.7 GREEN — implement the contrast harness; pass it for the initial palette. (27/27 pairings pass in both themes.)
- [x] 5.8 RED — reserved-semantics test: every amber usage is pending-data, every dotted-grey usage is provisional. (`test/design-system/reserved-semantics.test.ts`; mutation-checked on all 3 assertions — comment-only check bug found and fixed during this session, see apply-progress.)
- [x] 5.9 RED — dependency-guard test: `web/package.json`/lockfile fails on a denylisted kit (flowbite, shadcn, bootstrap, daisyui, @mui, @chakra-ui, …). (`test/design-system/dependency-guard.test.ts`; mutation-checked by adding `flowbite-svelte`, restored.)
- [x] 5.10 GREEN — keep 5.9 green against the current kit-free `package.json`.
- [x] 5.11 RED — tabular-figures test: computed `font-variant-numeric` of numeric elements resolves to tabular figures. (Scoped to the compiled-CSS mechanism level — see `test/design-system/tabular-figures.test.ts`'s own disclosed scope note; full rendered-page/browser-computed scenario deferred to slice 9 where real numeral-bearing pages exist.)
- [x] 5.12 GREEN — apply `--font-numeric`/`font-variant-numeric: tabular-nums`. (`tabular-nums` is Tailwind's built-in static utility, unaffected by the zeroing; `--font-numeric` re-declared as a project token.)
- [x] 5.13 RED/GREEN — `web/src/lib/export/schema.ts`: Zod schemas validating the slice-3 golden fixture (design's anti-drift device, wired as early as both sides exist rather than deferred to slice 9). (`test/export/schema.test.ts`, 6 tests including a schema_version exact-equality check and a provenance-resolution check; mutation-checked.)
- [x] 5.14 Verify: `npm --prefix web test`. (88/88 passing, 8 test files.)

---

## Slice 6 — `design-system`: seven static components + documented workbench

Files: `web/src/components/{IndicatorCard,MethodologySheet,BreakBand,AnnotationChip,ActionBar,FreshnessSemaphore,AccessibleDataTable}.astro`.

**Resolve D4** here: confirm `BreakBand`, `AnnotationChip` and `AccessibleDataTable` are independently
workbench-renderable (not chart-only partials) before slice 7 composes them.

- [x] 6.1 RED — workbench test: each of the seven renders standalone with representative props and ≥1 state variant (`experimental_AstroContainer`). (`web/test/workbench/components.container.test.ts`, 15 tests.)
- [x] 6.2 GREEN — implement the seven `.astro` components; wire workbench routes behind `WORKBENCH=1`. (`web/src/components/{IndicatorCard,MethodologySheet,BreakBand,AnnotationChip,ActionBar,FreshnessSemaphore,AccessibleDataTable}.astro`; `astro.config.mjs`'s `workbenchRoutes()` integration; `web/src/workbench/{fixtures.ts,WorkbenchShowcase.astro,pages/index.astro}`.)
- [x] 6.3 RED — zero-runtime-JS test: a page with the seven static components and no chart island ships no client-side script. (`web/test/workbench/zero-runtime-js.test.ts`.)
- [x] 6.4 GREEN — confirm no `client:*` directive on any of the seven. (Structurally true by construction — none imports Svelte or uses a `client:` directive; asserted mechanically by the same test.)
- [x] 6.5 RED — 44 px touch-target test at 375 px for every interactive control among the seven. (`web/tests/e2e/workbench/workbench.spec.ts`, real Playwright browser measurement — not a compiled-CSS mechanism-level stand-in, per this slice's own instruction to wire the workbench for real measurement.)
- [x] 6.6 GREEN — adjust hit-area sizing. (`min-h-11`/`min-w-11` added to every interactive control; two real undersized controls found and fixed by the RED run: `BreakBand`'s tooltip link and — after filtering legitimately-hidden `<details>`-closed controls out of the check — none remained undersized.)
- [x] 6.7 Verify: `npm --prefix web test`. (119/119 passing, 10 test files; `npm --prefix web run test:e2e` also run, 4/4 passing, including two new Playwright specs this slice added: 44 px measurement and real-rendered-component AA contrast in both themes.)

---

## Slice 7 — Static chart: geometry/transforms, SVG renderer, breaks, annotations, data table, textual description

Files: `web/src/lib/chart/*`, `web/src/lib/transform/{yoy,perCapita,sliceRange}.ts`, `web/src/components/IndicatorChart.astro`.

Dependency: confirm or exclude the seven unconfirmed editorial dates before this slice; `ReconcileEditorialConfig`
already refuses a nil date, but ship the annotation layer disclosed-incomplete if still unresolved.

- [x] 7.1 RED — pure unit tests for scales/geometry and `yoy`/`perCapita`/`sliceRange`: `perCapita` divides by the population value at the observation's own period (never latest-value retroprojection, never interpolated); YoY yields no point absent a same-period-prior-year observation (`series-transformations` scenarios). (`web/test/chart/{periods,geometry}.test.ts`, `web/test/transform/{yoy,perCapita,sliceRange}.test.ts`.)
- [x] 7.2 GREEN — implement `web/src/lib/chart/*.ts`, `web/src/lib/transform/*.ts` (pure, no DOM). (`periods.ts`, `geometry.ts`, `svg.ts`, `description.ts`; `transform/{yoy,perCapita,sliceRange}.ts`.)
- [x] 7.3 RED — `IndicatorChart.astro` container test: default render spans the whole series; two resolved breaks render two bands at the correct periods; no affordance hides a break; provisional points carry dotted-grey + "Provisional" disclosure (`indicator-page` scenarios). (`web/test/chart/indicator-chart.container.test.ts`.)
- [x] 7.4 GREEN — implement `IndicatorChart.astro` composing slice-6 components with the slice-7.2 geometry module.
- [x] 7.5 RED — annotation-default-state test: groups (a)/(b) off by default with visible enable controls; group (c) has no control when no entry applies; unconfirmed entries never render or count for readers. (Same container test file; unconfirmed-entry filtering verified structurally — see apply-progress: unconfirmed editorial entries are never projected to the DB per `editorial-config` spec, so they can never reach this component's `annotations` prop in the first place.)
- [x] 7.6 GREEN — wire the annotation layer to slice-4's `ListActiveEvents`-sourced artifact data. (Zero-JS CSS-only `peer-checked` disclosure: groups (a) governments/(b) exogenous off by default, (c) milestones on; absent, not disabled, when a group has zero applicable entries.)
- [x] 7.7 RED — textual-description test: build-time generated, names start/end values + periods + principal direction change in Spanish, present without JS, differs between series (`web-accessibility-gates` scenarios). (`web/test/chart/description.test.ts`, 9 tests.)
- [x] 7.8 GREEN — implement the build-time description generator; externalise Spanish in `web/src/i18n/es.ts`. **Produces user-facing Spanish copy.** (`web/src/lib/chart/description.ts` + `web/src/i18n/es.ts`, this project's first i18n module.)
- [x] 7.9 RED/GREEN — Vitest golden test: chart's build-time SVG matches a committed golden fixture (anti-divergence device ahead of slice 8's island). (`web/test/chart/svg.test.ts` using Vitest's `toMatchFileSnapshot`; golden at `web/test/fixtures/chart/golden-indicator-chart.svg`.)
- [x] 7.10 Verify: `npm --prefix web test`. (213/213 passing, 18 test files.)

---

## Slice 8 — Chart island: tooltips, keyboard nav, range presets, YoY/QoQ/per-capita toggles

Files: `web/src/components/ChartIsland.svelte`, `web/src/content/indicators/{slug}.ts`, `web/src/i18n/es.ts`.

- [x] 8.1 RED — control-visibility test per the six-series applicability matrix (`series-transformations`): `tasa-de-paro-epa` has no per-capita control; no page shows a disabled control anywhere; `pib` shows both YoY and QoQ; `ipc-general`/`ipc-subyacente`/`pib` show YoY as present. (`web/test/chart/applicability.test.ts`, pure `visibleTransformControls` — genuinely red-first, no DOM needed.)
- [x] 8.2 GREEN — implement per-page transformation config in `web/src/content/indicators/{slug}.ts`, driving `ChartIsland.svelte`. (Six per-slug files + `types.ts` + `index.ts` aggregator.)
- [x] 8.3 RED — per-capita coverage-disclosure test: control absent with no coverage; active view renders only the covered sub-span and discloses span + reason in Spanish; methodology sheet states the attribution rule. (Absence: `applicability.test.ts`, pure. Active-view disclosure: `chart-island.spec.ts`'s "the per-capita view discloses its covered span in Spanish" — real browser interaction, since it needs an actual toggle click.)
- [x] 8.4 **Resolve D6** — draft and get editorial sign-off on the exact Spanish per-capita disclosure copy (settled decision D4); externalise in `web/src/i18n/es.ts`. **Produces user-facing Spanish copy.** (`es.chart.perCapita.coverageDisclosure`/`attributionRule` — drafted this session, flagged for editorial sign-off per this project's own established convention for every other new reader-facing string, e.g. `FreshnessSemaphore`'s "Al día".)
- [x] 8.5 GREEN — implement per-capita rendering restricted to `coverage.from`–`coverage.to` with the 8.4 disclosure. (`ChartIsland.svelte`'s `perCapitaResult.points`, already restricted by `computePerCapita`'s own coverage-span filtering, slice 7.)
- [x] 8.6 RED — range-preset test: a preset preceding the series' first observation is absent (not disabled); selecting a preset updates the permalink; loading it reproduces the range. (Pure round-trip: `web/test/chart/permalink.test.ts`. Real interaction: `chart-island.spec.ts`'s "selecting a range preset updates the permalink; reloading it reproduces the same range".)
- [x] 8.7 GREEN — implement presets (5 años, 10 años, desde 2008, desde 2018) with permalink encoding. **Disclosed gap**: "personalizado" (a free-form custom date-range picker) is NOT built this slice — only the five other spec-named presets ship as real controls. Slice 7's `sliceCustomRange` primitive exists and is reusable, but a genuine custom-range UI (two date inputs, validation against the series' own span, permalink encoding for an arbitrary `from`/`to` pair) is materially larger scope than the five named presets and was not part of this session's `RangePreset` type. Recorded as an open item for a follow-up slice rather than silently claimed done — `series-transformations` spec's "Range presets" requirement lists "personalizado" explicitly and this is not yet satisfied. **SUPERSEDED — the disclosure above was accurate when written and is kept as the historical record; the gap it names is CLOSED by slice 13 (2026-07-30).** Every element it lists as missing now exists and was verified on disk: the date inputs, the validation against the series' own span, the permalink encoding for an arbitrary `from`/`to` pair, and a `CUSTOM_RANGE` selection alongside the preset enum. See slice 13.
- [x] 8.8 RED — no-network-request test on toggle activation. (`chart-island.spec.ts`, mutation-checked: a temporary `fetch()` call inside the handler was confirmed to fail the test, then reverted.)
- [x] 8.9 GREEN — ensure `ChartIsland.svelte` calls only the shared chart/transform module client-side. (No `fetch`/`XMLHttpRequest` anywhere in the component; confirmed structurally in `island-ssr.test.ts` and behaviourally in the Playwright test above.)
- [x] 8.10 RED — relabel test: YoY activation changes axis label/unit, methodology sheet states the derivation; legend + non-colour channel distinguish two encodings in one view. (`chart-island.spec.ts`'s "activating year-on-year relabels the unit and states the derivation..." — the derivation note is rendered by the island itself, not the static `MethodologySheet`, since only the island knows the live toggle state; see apply-progress for the reasoning.)
- [x] 8.11 GREEN — implement relabeling and the legend/pattern-channel logic. (`viewUnit`/`viewDecimals`/`transformLabel` derived state; legend/marker shape logic reused verbatim from the shared `renderChartSVG`.)
- [x] 8.12 RED/GREEN — island-parity golden test: `ChartIsland.svelte`'s initial client render equals the slice-7 build-time SVG golden. (`web/test/chart/island-ssr.test.ts`, using `svelte/server`'s `render()` — no DOM/browser needed; byte-identical match confirmed against the same committed golden fixture svg.test.ts uses.)
- [x] 8.13 Wire `client:idle` hydration. (`WorkbenchShowcase.astro`'s new ChartIsland section; production page wiring is slice 9a's own job — the directive itself is proven working here.)
- [x] 8.14 Verify: `npm --prefix web test`. (247/247 passing, 25 test files — 213 from slice 7 + 34 new. `npm --prefix web run test:e2e` also run: 13/13 passing, including `chart-no-js.spec.ts` UNCHANGED.)

---

## Slice 9a — Three indicator pages, export loader, page states, Playwright/axe scaffold

Slugs: `tasa-de-paro-epa`, `ocupados-epa`, `poblacion-residente`.
Files: `web/src/lib/export/loader.ts`, `web/src/pages/indicador/[slug].astro` (route infra for all six),
`web/src/i18n/es.ts`, `web/tests/e2e/`.

- [x] 9a.1 RED — loader test: fetches/reads the artifact, verifies every sha256 against the manifest, parses with the slice-5 Zod schemas, requires exact `schema_version` integer equality; any failure exits non-zero, previous deploy stays live. (`web/test/export/loader.test.ts`, 10 tests.)
- [x] 9a.2 GREEN — implement `web/src/lib/export/loader.ts` against the slice-3 golden fixture. (Regenerated the fixture this slice with real 3-series data — see apply-progress.)
- [x] 9a.3 RED — page-anatomy test (3 pages): header/chart/action-bar/methodology-sheet/related-indicators all present; header carries no interpretive prose beyond labelled fields; methodology sheet is its own landmark region; 3–5 related cards resolve to existing routes. (`web/test/pages/indicator-page.container.test.ts`; "existing routes" resolved against the full six-slug frozen catalog, a disclosed temporary narrowing — see apply-progress.)
- [x] 9a.4 GREEN — implement the page template for the 3 slugs, composing slices 6/7/8. (`web/src/templates/IndicatorPage.astro` + thin route `web/src/pages/indicador/[slug].astro`.)
- [x] 9a.5 RED — freshness-semaphore test: fresh → green; source-pending → amber with exact label "Pendiente de actualización por la fuente"; page never issues a network request for freshness and never renders a "page older than data" element.
- [x] 9a.6 GREEN — wire `FreshnessSemaphore` to the artifact's two-state field only.
- [x] 9a.7 RED — three-page-states test: validation-failure banner reads exactly "Última actualización correcta: {fecha}. La fuente ha publicado un dato que no ha superado nuestra validación automática; estamos revisándolo"; chart never hidden; discontinued-series permanent banner with successor link when configured. (`web/src/lib/indicator/pageState.ts` + its own test; mutation-checked in the container test — see apply-progress.)
- [x] 9a.8 GREEN — implement the banners in `web/src/i18n/es.ts` and the page template. **Produces user-facing Spanish copy.**
- [x] 9a.9 RED — methodology-sheet traceability test: source, operation, origin identifier + link, periodicity, next-publication calendar, extraction timestamp, unit/base, break list, vintage + revision-history access, ingestion-script link — all present and non-empty. (Next-publication/revision-history fields are rendered from honestly disclosed fallback copy, not fabricated data.) **The design.md Open Question this task promised was never written; it now exists** — added 2026-07-30 as the last entry under design.md's Open Questions (verify-report WARNING-11). Writing it up corrected this task's own claim in one respect, recorded here so the two agree: "no backing data source anywhere in this project" is exactly right for the next-publication calendar (no such field exists in `config/`, `adapters/config/types.go` or `publishing/artifact.go`, and all six series share one hard-coded INE calendar URL), but overstated for the revision history — the artifact does carry per-point `version`/`ingestionRunId` and the `vintages` lookup, and the sheet's "Vintage mostrado" value is computed from that real data. What is genuinely missing there is the superseded versions (`ListPublishedObservations` filters on `o.is_current`, so only the current row per period is exported) and any surface to link to: `revisionHistoryHref` resolves to `#vintage`, an anchor with no target element on the page. Full detail, including what would have to exist to close each half, in the design.md entry.
- [x] 9a.10 GREEN — complete methodology-sheet data wiring from the artifact.
- [x] 9a.11 RED — no-inlined-copy scan: every reader-facing string on these 3 pages resolves through `es.ts`; technical identifiers render verbatim, untranslated. (Scoped to this slice's own new files, per this session's explicit scope instruction — the seven pre-existing static components' own inlined Spanish from slices 6/7 is a disclosed, deferred gap, NOT closed this slice; `es.ts`'s own prior overclaiming comment corrected.)
- [x] 9a.12 GREEN — fix any remaining inlined strings. (Within this slice's own scoped files only — see 9a.11.)
- [x] 9a.13 Scaffold Playwright + axe-core for these 3 pages as acceptance gates written alongside this slice (not red-first, per the stated Strict-TDD boundary): keyboard point navigation, 44 px targets, `javaScriptEnabled: false` context. (`web/tests/e2e/indicator/{indicator-page.ts,indicator-pages.spec.ts,indicator-pages-no-js.spec.ts}`, 12 tests, all passing against the real build.)
- [x] 9a.14 Verify: `npm --prefix web test`; `npm --prefix web run test:e2e` (3 pages); local `export --fixture` → `astro build` end-to-end. (272/272 vitest, 25/25 Playwright, real `WORKBENCH=1 npm run build` produced all 3 pages — see apply-progress for verbatim evidence.)

---

## Slice 9b — Remaining three pages, full accessibility/budget gates, redirect infra, config wiring

Slugs: `ipc-general`, `ipc-subyacente`, `pib`.
Files: remaining page routes, `.github/workflows/` (Lighthouse, full axe/no-JS jobs), `openspec/config.yaml`.

- [x] 9b.1 Repeat 9a.3–9a.12 for the remaining 3 slugs (mandatory YoY applies; `pib` additionally shows QoQ). (`web/test/pages/indicator-page.container.test.ts`'s new "slice 9b" describe blocks; widened the methodology-traceability loop to all six slugs; `pib`'s QoQ assertion passes against real fixture data — see apply-progress for the intra-annual-variation check.)
- [x] 9b.2 RED/GREEN — redirect-infra test + implementation: a slug change ships a permanent redirect, no permalink ever 404s (exercised via a test fixture; all six slugs are frozen in this change, so the mechanism is proven but unused). (`web/src/lib/indicator/redirects.ts` + `web/test/indicator/redirects.test.ts`, wired into `astro.config.mjs`'s native `redirects` option; `SLUG_REDIRECTS` is empty in production, by design.)
- [x] 9b.3 RED/GREEN — data-table/chart association and row-count parity, all six pages (`web-accessibility-gates`). (`indicator-page.container.test.ts`'s new "data table / chart association and row-count parity" describe block — asserts `aria-describedby` on every page's SVG resolves to a real description id + table id, and every table has exactly one row per artifact point, for all six slugs.)
- [x] 9b.4 Acceptance gate (alongside, not red-first): full axe-core over all six pages in both themes, blocking CI. (`web/tests/e2e/indicator/indicator-pages.spec.ts` widened to all six slugs × both themes — dark theme exercised by setting `data-theme="dark"` post-load, since production ships no runtime toggle; 12 axe tests, all passing against the real build.)
- [x] 9b.5 Acceptance gate: keyboard traversal reaches every point with visible focus and no keyboard trap, all six pages. (Same file, widened SLUGS array; 6 tests passing.)
- [x] 9b.6 Acceptance gate: 44 px measured in-browser for every control, all six pages. (Same file, widened SLUGS array; 6 tests passing.)
- [x] 9b.7 Acceptance gate: `javaScriptEnabled: false` context asserting SVG/table/description/methodology/break-bands present, no loading/empty/error state, all six pages. (`indicator-pages-no-js.spec.ts` widened to all six slugs; 6 tests passing.)
- [x] 9b.8 Acceptance gate: Lighthouse CI with a named transferred-bytes assertion (<300 KB excl. typeface), blocking, all six pages — verify the gate was actually live from slice 5 (spec requires "wired from the first web slice, not at the end"); backfill the wiring if it was only exercised starting here. **Verified NOT live before this slice** (checked `.github/workflows/ci.yml` history across slices 5-8: no Lighthouse job existed) — a genuine, disclosed spec-compliance gap, backfilled this slice: `web/src/lib/budget/transferredBytes.ts` (pure, unit-tested) + `web/scripts/check-lighthouse-budget.mjs` (real `lighthouse` npm-package run over Playwright's own Chromium via CDP) + two new blocking CI steps. Measured real numbers: all six pages ~35.5-35.8 KB excl. typeface, well under 300 KB. See design.md's new disclosure entry. **Later change (slice 12, 2026-07-30)**: that `.mjs` script was deleted and replaced by `web/scripts/check-lighthouse-budget.ts`, and the two CI steps became one; the measured numbers above were taken against the 3-point fixture that shipped at the time, not the full-history fixture slice 12 installed. Both corrections are recorded in slice 12, not rewritten here.
- [x] 9b.9 Acceptance gate: contrast verification (slice 5 harness) included in the same blocking gate, both themes. (Already true structurally: `npm test` — which runs `contrast.test.ts` — is a step in the SAME blocking `web` CI job as the build/e2e/Lighthouse steps; no redundant re-invocation added.)
- [x] 9b.10 Update `openspec/config.yaml`: `testing.web.command` / apply `test_command` → `npm --prefix web test` and `npm --prefix web run test:e2e`, replacing `TBD`; verify no residual ADR-8-withdrawn party-colour guideline remains (already clean per design's supporting-decisions table — confirm and close). **Both already correct on disk** (a prior slice had already filled them in — confirmed by reading the file before editing); no residual party-colour mention found anywhere in the repo (grepped `openspec/`, `docs/`, `web/src`, `web/test`) — the only hits are the ADR-8-lifted disclosures themselves. Additionally closed `verify.build_command`'s stale `TBD (... once scaffolded)` placeholder, now genuinely scaffolded.
- [x] 9b.11 Verify full milestone exit criteria: `go test ./...` (repo root), `npm --prefix web test`, `npm --prefix web run test:e2e` all green; axe + Lighthouse CI pass on all six pages; end-to-end deploy loop marked `N/A` pending VPS/`PORTAINER_WEBHOOK_URL` provisioning (disclosed dependency, not a blocker for this slice). See apply-progress for verbatim exit-criteria output.

---

## Slice 10 — Remediation A (verify-report CRITICALs 1-3)

`sdd-verify` returned FAIL: 4 CRITICAL findings (Engram #4771/#4772, `verify-report.md`). CRITICAL-4 (export
artifact carries no page-state field) is explicitly OUT OF SCOPE for this slice — split into its own later
work unit because it crosses the Go/web boundary. Files: `web/src/templates/IndicatorPage.astro`,
`web/src/components/{ActionBar,AccessibleDataTable,AnnotationChip,BreakBand,ChartIsland,
FreshnessSemaphore,IndicatorCard,IndicatorChart,MethodologySheet,MethodologySheetFields}.{astro,svelte}`,
`web/src/i18n/es.ts`, `web/scripts/check-lighthouse-budget.mjs` (deleted in slice 12, replaced by
`check-lighthouse-budget.ts`), `web/src/lib/budget/transferredBytes.ts`,
`web/test/pages/indicator-page.container.test.ts`, `web/test/budget/transferredBytes.test.ts`,
`web/test/budget/check-lighthouse-budget-gate.test.ts` (new).

- [x] 10.1 RED — CRITICAL-1: added a test asserting `csvHref`/`jsonHref` against the REAL on-disk artifact
  layout (the golden fixture `export --fixture` actually writes: `csv/{slug}.csv`, `series/{slug}.json`),
  not a second hard-coded string, for all six pages. Confirmed it fails against the pre-fix code
  (`csvHref` = `/data-derived/{slug}.csv`, real file at `/data-derived/csv/{slug}.csv`) before touching
  production code. (`web/test/pages/indicator-page.container.test.ts`, new "action bar hrefs resolve to the
  artifact's REAL on-disk layout" describe block.)
- [x] 10.2 GREEN — fixed `IndicatorPage.astro`'s `csvHref` to `/data-derived/csv/${doc.slug}.csv`, matching
  `app/internal/publishing/csv.go`'s real writer path. `jsonHref` was independently re-verified against the
  real layout by the same test and required no change. Corrected `ActionBar.astro`'s doc comment, which
  claimed these links were "fully functional today" with no test backing that claim — now states the real
  responsibility split (this component renders whatever href it is given; the caller/test own correctness).
- [x] 10.3 RED — CRITICAL-2: added `assertRealPageLoad` unit tests (`web/test/budget/transferredBytes.test.ts`)
  — runtimeError present, empty network-record set, non-2xx document response all throw
  `LighthouseMeasurementError`; a real page load (non-empty items, 200 document, no runtimeError) does not
  throw (triangulated positive case); an unfamiliar report shape with no `statusCode` at all degrades
  gracefully. Also added an integration-level RED test
  (`web/test/budget/check-lighthouse-budget-gate.test.ts`) that spawns the REAL
  `check-lighthouse-budget.mjs` script (now `.ts`, slice 12) against an unreachable `PREVIEW_URL` and
  asserts a non-zero exit —
  confirmed this fails against the pre-fix script (exit 0, six `PASS ... 0.0 KB` lines, reproducing the
  verify report's own repro exactly) before touching the script.
- [x] 10.4 GREEN — implemented `assertRealPageLoad` in `web/src/lib/budget/transferredBytes.ts` (pure,
  unit-tested) and mirrored it in `web/scripts/check-lighthouse-budget.mjs` (same hand-duplication
  convention this file already used for `computeTransferredBytesExcludingFonts`, since plain `node` cannot
  import a TS module — SUGGESTION-12, a disclosed pre-existing drift risk, not resolved this slice; **slice
  12 later removed the duplication entirely** by rewriting the script as `check-lighthouse-budget.ts` and
  deleting the `.mjs`, after establishing that the premise no longer held — Node strips types from `.ts`
  natively). The gate
  now throws loudly (non-zero exit) on a runtime error, an empty network-request set, or a non-2xx document
  response, instead of silently treating a broken measurement as "0 bytes, PASS". The 300 KB budget itself
  was not touched.
- [x] 10.5 RED — CRITICAL-3: widened the no-inlined-copy scan from 1 file (`IndicatorPage.astro`) to all 11
  production component files (the 9 the report named, `IndicatorPage.astro` itself, plus
  `AnnotationChip.astro` — found inlining "Gobierno"/"Shock"/"Hito" during this remediation, not named in
  the report's own 37-string count but a real violation of the same requirement). Confirmed this test fails
  against the pre-fix components. Also fixed a real gap in the scan's own regex while widening it: the
  original single-line-only pattern could not see multi-line text nodes (the common case — most of these
  components format inline text on its own indented line), which is very likely WHY the original narrow
  scan of `IndicatorPage.astro` never caught anything either; the widened regex now spans newlines and
  strips frontmatter/`<script>`/`<style>` blocks before scanning to avoid false positives.
- [x] 10.6 GREEN — moved every found string to `web/src/i18n/es.ts`, preserving each byte-for-byte (the
  spec's own verbatim `FreshnessSemaphore` string included), reusing existing `es.page.methodologyHeading`/
  `ingestionScriptLinkLabel`/`es.chart.statusLabel`/`breaksHeading` where the SAME string already existed
  rather than duplicating it under a new key. New `es.ts` sections: `table`, `actionBar`, `freshness`,
  `methodologySheet`, `breakBand`, `annotationChip`, `indicatorCard`. Technical identifiers (slugs, origin
  refs, units-as-published) were left verbatim, untranslated, per instruction — none were moved.
- [x] 10.7 Verify: `go test -count=1 ./...` (391 Go test funcs, all packages ok); `npm --prefix web test`
  (304/304, up from 287 — 1 hrefs test + 1 gate-integration test + 5 `assertRealPageLoad` unit tests + 10 net
  from widening the scan to 11 files); `npm --prefix web run test:e2e` (43/43, unchanged, no regression);
  `npm --prefix web run build` (all six pages, real `csv/{slug}.csv` hrefs confirmed in built HTML);
  `npm --prefix web run budget:lighthouse` against a REAL preview server (PASS, all six ~36.0-36.3 KB, exit
  0) AND separately against a dead port (FAIL, `LighthouseMeasurementError: ... FAILED_DOCUMENT_REQUEST ...
  net::ERR_CONNECTION_REFUSED`, exit 1) — the gate now fails loudly exactly where it silently passed before.
  Port hygiene confirmed clear before and after every run; every preview server started was killed.
  CRITICAL-4 (export artifact page-state) intentionally NOT touched — out of scope, split to a future work
  unit per the product owner's own instruction.

---

## Slice 10a — Corrective: break-band tooltip painted at rest (found by browser inspection)

Not part of any planned work unit. Added here 2026-07-30 by the reconciliation pass that found this slice's
work fully recorded in `apply-progress.md` ("Post-slice defect — break-band tooltip painted at rest") but
carrying no task entries at all, so the task arithmetic under-counted it. The entries below are written from
that record and from the files as they stand on disk today; nothing was inferred.

Files: `web/src/styles/components.css` (new), `web/src/components/BreakBand.astro`,
`web/src/pages/index.astro`, `web/src/workbench/pages/index.astro`, `web/src/pages/indicador/[slug].astro`,
`web/tests/e2e/workbench/workbench.spec.ts`.

- [x] 10a.1 RED — two page-wide Playwright tests in `web/tests/e2e/workbench/workbench.spec.ts` asserting,
  over every `break-band-tooltip` the page emits regardless of which renderer produced it, that none is
  painted at rest and that focusing the island's own trigger reveals its own tooltip. Both confirmed failing
  against the as-built page (`expected "0", received "1"` on
  `break-tooltip-tasa-de-paro-epa-island-light-reform-2022`).
- [x] 10a.2 GREEN — moved the `.break-band__tooltip` reveal rules out of `BreakBand.astro`'s Astro-scoped
  `<style>` into a new shared `web/src/styles/components.css`, imported by the three page entry points that
  already import `theme.css`. Root cause: Astro scopes a component `<style>` to that component, and
  `ChartIsland.svelte` re-renders the same markup, so the island's copy never received the rules and kept the
  browser default `opacity: 1`. `theme.css` deliberately untouched — it stays the pure token file
  `theme-tokens.ts` parses and `token-guard.test.ts` polices.
- [x] 10a.3 Verify: `npm --prefix web test` (304/304, 31 files) and `npm --prefix web run test:e2e` (45/45,
  43 prior + the 2 new). **Disclosed and carried forward at the time**: a parity test asserting the two
  renderers' break-band markup cannot diverge in class or `data-testid` remained unwritten — closed later,
  by task 12.9.

---

## Slice 11 — Remediation B (verify-report CRITICAL-4)

Scope chosen by the product owner: **CRITICAL-4 only**. The seven WARNINGs and three SUGGESTIONs were left
open for a later unit (they are slice 12 below). Run as two delegated writers, Go half then web half,
sequential because the web contract depends on the regenerated fixture.

Task entries added 2026-07-30 by the reconciliation pass: the slice shipped and is fully recorded in
`apply-progress.md`, but `tasks.md` was never extended, so the task count under-reported it. Every entry
below was re-verified against the files on disk before being written.

Files: `app/internal/adapters/postgres/{page_state,dimensions,published_series}.go`,
`app/internal/adapters/config/{types,validate}.go`, `app/migrations/0005_series_discontinued.{up,down}.sql`,
`app/internal/publishing/{artifact,export,validate}.go`, `web/src/lib/export/schema.ts`,
`web/src/lib/indicator/pageState.ts`, `web/src/i18n/es.ts`, `web/src/content/indicators/methodology.ts`,
`web/src/templates/IndicatorPage.astro`, `web/test/fixtures/export/`.

- [x] 11.1 Fix the artifact contract on paper before either writer starts, so the two halves cannot disagree:
  `pageState: { kind, lastCorrectUpdate, successorSlug }`, `kind` ∈ `fresh` | `validation-failure` |
  `discontinued`, always present; `lastCorrectUpdate` non-null only on `validation-failure`, `successorSlug`
  non-null only on `discontinued`; precedence `discontinued` > `validation-failure` > `fresh`;
  `schema_version` stays 1. (Recorded verbatim in apply-progress.)
- [x] 11.2 RED/GREEN — `app/internal/adapters/postgres/page_state.go`: `SeriesValidationOutcome` reads back
  the `ingestion_run.outcome='validation-failed'` rows `ApplyGate` has written since task 4.16 and nothing
  ever read. A `pending` run is deliberately NOT reported as a failure — `CreateIngestionRun` inserts that
  placeholder before validation decides anything and a crash leaves it there, so reporting it would put a
  banner on the page asserting the SOURCE published a failing datum. 6 `Test*` functions on disk
  (`page_state_test.go`).
- [x] 11.3 RED/GREEN — `app/internal/adapters/config/{types,validate}.go`: `SeriesConfig.Discontinued`
  (`since` required, ISO and a real calendar day; `successor` optional, must name another configured series,
  never itself). 8 `Test*` functions on disk (`discontinued_test.go`).
- [x] 11.4 GREEN — `app/migrations/0005_series_discontinued.{up,down}.sql`, additive nullable columns.
  Deliberately **no FK** on the successor column: reconcile walks series in file order, so the successor row
  may not exist yet; `validate-config` is the referential gate and can name the offending file and field.
- [x] 11.5 RED/GREEN — `dimensions.go` persists AND clears both columns and `seriesIdentityDigest` now covers
  them, so a discontinuation actually triggers a reconcile rather than being silently ignored;
  `published_series.go` reads them back (`series_discontinued_test.go`).
- [x] 11.6 RED/GREEN — `artifact.go` / `export.go` / `validate.go`: `PageStateRef`, the
  `SeriesValidationOutcome` port, the pure `SeriesPageState` composition with the 11.1 precedence, and
  write-side artifact validation. Recorded as "12 tests" at the time; counted on disk 2026-07-30,
  `app/internal/publishing/page_state_test.go` carries **13** `Test*` functions (7 `TestExport_*`,
  6 `TestValidateArtifact_*`) — the discrepancy is recorded rather than silently reconciled, since neither
  number can now be shown to be the one that was true when the line was written.
- [x] 11.7 Resolve the edge case that shaped the design: `SeriesValidationOutcome` can report a failure with
  no prior success (the series' very first run failed), so there is no correct-update date to name. Pinned on
  both sides as `kind: "validation-failure"` with `lastCorrectUpdate: null`. Substituting any date makes the
  banner assert a provenance fact that never happened (P4 forbids it; `BreakConfig.DateStatus` already
  refuses the same thing for an unconfirmed break date), and falling back to `fresh` silently suppresses a
  recorded failure.
- [x] 11.8 GREEN — regenerate `web/test/fixtures/export/` through the real `export --fixture` path against a
  testcontainers Postgres, not hand-edited; all six manifest sha256 digests re-verified against `sha256sum`
  on disk.
- [x] 11.9 RED/GREEN — `web/src/lib/export/schema.ts`: `pageState` REQUIRED in the Zod schema, never
  `.optional()`/`.default()` — a fixture lacking it fails the build rather than being silently treated as
  fresh, which is CRITICAL-4 itself. No `schema_version` bump: the Go writer and this loader ship in the same
  change, so no already-published artifact lacking the field is ever read by a loader that requires it.
- [x] 11.10 RED/GREEN — `web/src/lib/indicator/pageState.ts`: `lastCorrectUpdate` widened to
  `string | null`, plus `pageStateFromArtifact()`. The stale doc paragraph claiming "The Go artifact carries
  no field for this today" deleted — it had become false. 11 `it()` cases on disk
  (`web/test/indicator/pageState.test.ts`), one carrying its own in-file `— mutation-checked` label.
- [x] 11.11 GREEN — `web/src/i18n/es.ts`: new dateless banner copy (`validationFailureBannerNoDate`,
  byte-for-byte the second half of `validationFailureBanner` so both state the identical fact),
  `discontinuedBanner`, `discontinuedSuccessorLink`. **Produces user-facing Spanish copy.**
- [x] 11.12 GREEN — `web/src/content/indicators/methodology.ts`'s six hard-coded `pageState` constants
  removed, leaving only a pointer comment; `web/src/templates/IndicatorPage.astro`'s banner now driven by the
  artifact. Mutation-checked: re-hard-coding `{kind:"fresh"}` in the template fails 6 tests — the one named,
  causal RED of this slice, and it reproduces CRITICAL-4 itself.
- [x] 11.13 Cross-boundary finding, found by the web writer while wiring the successor link:
  `successorSlug` is a PIPELINE series slug, not a route slug (`pib`'s artifact document is `pib-cvi` while
  its route is `/indicador/pib`), so linking the raw value would 404. Resolved web-side via
  `routeSlugForArtifactSlug` (`web/src/content/indicators/index.ts`); an unresolvable successor renders the
  banner with no link at all. The two identifier spaces differ and nothing previously said so.
- [x] 11.14 Verify: `go build ./...`, `go vet ./...`, `gofmt -l .` clean; `go test ./...` green across all 23
  packages; `validate-config: ok`; `npm --prefix web test` 337/337 across 31 files (baseline 304);
  `npm --prefix web run test:e2e` 45/45; real build 7 pages production / 8 with `WORKBENCH=1`, built against
  an `EXPORT_DIR` copy carrying all three states with recomputed digests (the dateless banner, the dated
  banner and a resolved successor link all render, the chart section is present in every state, and
  `grep -c 'correcta: null'` returns 0). 8 mutation checks in total across both halves, all restored.

---

## Slice 12 — Remediation C (verify-report WARNINGs 6/7/8, SUGGESTIONs 12/13/14)

The findings slice 11 explicitly left open. Run as three delegated work units — CI/tooling, fixture,
web-quality — plus the break-band parity test that slices 10a and 11 both carried forward as an open
disclosure. `verify-report.md` itself was not edited: it is the auditor's record of what was true when it was
written, and a finding that turns out to have been closed earlier is recorded here, not erased there.

Files: `.github/workflows/ci.yml`, `web/tsconfig.json` (new), `web/package.json`, `web/package-lock.json`,
`web/scripts/check-lighthouse-budget.{mjs → ts}`, `web/src/lib/budget/transferredBytes.ts`,
`web/test/budget/check-lighthouse-budget-gate.test.ts`, `web/test/fixtures/export/` (+ `source.txt`),
`web/src/lib/export/schema.ts`, `web/src/components/{BreakBand.astro,ChartIsland.svelte}`,
`web/src/templates/IndicatorPage.astro`, `web/test/design-system/break-band-parity.test.ts` (new),
`web/test/pages/indicator-page.container.test.ts`.

### CI / tooling unit (WARNING-8, SUGGESTION-12, SUGGESTION-14)

- [x] 12.1 WARNING-8 — reorder `.github/workflows/ci.yml`'s `web` job so the transferred-bytes gate measures
  the PRODUCTION build. `playwright.config.ts`'s `webServer.command` is `WORKBENCH=1 npm run build &&
  npm run preview` and under `CI` it sets `reuseExistingServer: false`, so the e2e step always overwrites
  `dist/` with a workbench build; the production `astro build` now runs AFTER e2e, and the preview server and
  budget gate that follow measure what actually ships. The two-step `nohup … & disown` pattern (verify-report
  scrutinised claim 6) was collapsed into ONE step with a `trap`, removing the cross-step process-survival
  assumption altogether, and execs `./node_modules/.bin/astro preview` directly rather than `npm run preview`
  so `$!` is the process that holds the port.
- [x] 12.2 RED/GREEN — `assertProductionBuild` in `web/src/lib/budget/transferredBytes.ts`: the gate probes
  `/workbench` (a route `astro.config.mjs` injects only under `WORKBENCH=1`) BEFORE Chromium is launched and
  refuses to measure a build that answers 2xx there. Covered end to end against a real in-process HTTP server
  by `check-lighthouse-budget-gate.test.ts`, which also asserts the probe costs exactly one request
  (`expect(requestedPaths).toEqual(["/workbench"])`) and that no page prints a budget verdict against the
  wrong build. The ordering in 12.1 is the fix; this assertion is the guard rail that keeps a future edit from
  silently undoing it.
- [x] 12.3 SUGGESTION-12 — delete `web/scripts/check-lighthouse-budget.mjs` and replace it with
  `web/scripts/check-lighthouse-budget.ts`, which IMPORTS `computeTransferredBytesExcludingFonts`,
  `assertRealPageLoad`, `assertProductionBuild`, `evaluatePageBudget` and `TRANSFERRED_BYTES_BUDGET` from the
  unit-tested `src/lib/budget/transferredBytes.ts` instead of hand-duplicating them. Slice 10's stated
  premise for the duplication ("plain `node` cannot import a TS module") had stopped being true: Node strips
  types from `.ts` natively. No build step, no `tsx`, no new dependency. Four new guards in
  `check-lighthouse-budget-gate.test.ts` fail if anyone copies the logic or the budget constant back, or
  leaves an orphan `.mjs` behind for CI to keep running.
- [x] 12.4 SUGGESTION-14 — add `web/tsconfig.json` (extends `astro/tsconfigs/strict`, `include` covers
  `src/`, `test/`, `tests/`, `scripts/` and not just `src/`) and wire `npm run check` (`astro check`) into
  `ci.yml` as a BLOCKING step, placed before the test steps so a type error stops the job in seconds rather
  than after a Chromium download. `@astrojs/check` and `typescript` added as devDependencies. The ADR-6
  dependency guard (`web/test/design-system/dependency-guard.test.ts`) was verified compatible with both
  additions and was NOT modified — slice 11 had flagged the guard as the reason `astro check` could not be
  installed, and that concern turned out not to apply (the denylist names component kits, not tooling).
  `strictest` was deliberately not chosen: `noUncheckedIndexedAccess` and `exactOptionalPropertyTypes` are
  real design decisions about how this codebase models optionality, not mechanical fixes.
- [x] 12.5 Re-check verify-report finding 1(b) (the dead preview server reporting six `PASS 0.0 KB` and
  exit 0) rather than assume it still stood. **Found ALREADY CLOSED, by slice 10's `assertRealPageLoad`** —
  slice 10.4 fixed exactly this and re-verified it against a dead port; the verify report predates that fix
  and is stale on this point. This slice's `assertMeasuringProductionBuild` probe adds a second, earlier
  closure of the same case (an unreachable server now fails on a plain `fetch` in milliseconds, before
  Chromium starts, instead of after six Lighthouse runs). Recorded rather than re-claimed as newly fixed.

### Fixture unit (WARNING-6)

- [x] 12.6 WARNING-6 — regenerate `web/test/fixtures/export/` through the real export pipeline so the
  page-level acceptance evidence no longer rests on a 3-observation-per-series stub. Counted on disk:
  `tasa-de-paro-epa` 98 (2002-Q1→2026-Q2), `ocupados-epa` 98 (2002-Q1→2026-Q2), `ipc-general` 294
  (2002-01→2026-06), `ipc-subyacente` 294 (2002-01→2026-06), `pib-cvi` 125 (1995-Q1→2026-Q1),
  `poblacion-residente` 122 (1971-Q1→2026-Q2) — the real live series lengths the verify report itself names.
  Real configured breaks (1 each on the three EPA/IPC series) and 10 real reconciled events per series now
  reach the pages. Generated by a temporary
  `app/cmd/concontexto/zzgen_fullhistory_fixture_test.go`, deleted immediately after running — the
  throwaway-generator convention slices 3, 9a, 9b and 11 all used.
- [x] 12.7 Document the fixture's provenance in `web/test/fixtures/export/source.txt`, stating exactly what
  is real and what is not. **Real**: every structural and metadata field (slugs, units, frequencies,
  decimals, source names, attribution, licence names/URLs, origin identifiers, schema shape) from the shipped
  `/config` tree through the real pipeline; the breaks and events, reconciled by the real
  `ingestion.ReconcileEditorialConfig` from `config/rupturas.yaml`/`eventos.yaml`/`gobiernos.yaml`; and the
  NEWEST THREE observations of every series, copied verbatim from the live-verified INE responses under
  `app/internal/ingestion/testdata/datos_serie/`. **Synthesised**: every other observation — a straight line
  plus a fixed seasonal offset, chosen deliberately so nobody can mistake the history for real data, with a
  published recipe, no randomness and byte-reproducible output. The file opens with an explicit "MOST OF THE
  NUMBERS IN THIS DIRECTORY ARE SYNTHESISED … MUST NOT be quoted, charted, exported or cited" warning.

### Web-quality unit (WARNING-7, SUGGESTION-13, plus the carried-forward parity test)

- [x] 12.8 GREEN — replace `z.array(z.unknown())` for `breaks` and `events` in `web/src/lib/export/schema.ts`
  with real `BreakRefSchema` / `EventRefSchema` object schemas, each with a `uniqueBy` refinement that
  rejects a duplicated `key`/`id` as an unresolved (ambiguous) reference. This cleared the last `astro check`
  errors — 12 of them per the writers' report, a count recorded as reported since the pre-fix state no longer
  exists to re-observe. (`withdrawn: z.array(z.unknown())` is left as-is and is now the only such member —
  the Go writer emits an empty array there and no shape has been fixed for it.)
- [x] 12.9 RED/GREEN — WARNING-7: convert the break-band trigger from `<span tabindex="0">` to
  `<button type="button">` in BOTH renderers (`BreakBand.astro` and `ChartIsland.svelte`), removing the
  `a11y_no_noninteractive_tabindex` warning that every build emitted and that would otherwise have masked the
  next real Svelte a11y warning.
- [x] 12.10 RED/GREEN — write the cross-renderer divergence guard that slices 10a and 11 both carried forward
  as an open disclosure: `web/test/design-system/break-band-parity.test.ts` asserts three things together —
  (1) both renderers emit the same class names and the same `data-testid` for the same three elements,
  (2) both emit the same element for the trigger and it is a real `<button>`, and (3)
  `src/styles/components.css` actually carries rules for those exact names and neither component has taken
  them back into a scoped `<style>`. (3) is not redundant with (1): a rename applied consistently to both
  components but not to the stylesheet keeps them in perfect parity while reproducing the original
  painted-at-rest defect exactly.
- [x] 12.11 GREEN — SUGGESTION-13: `IndicatorPage.astro` now passes `slug={content.slug}` (the ROUTE slug) to
  `MethodologySheet`, so the pib page no longer carries the DOM id `methodology-heading-pib-cvi`. `ActionBar`'s
  two hrefs deliberately still use `doc.slug`, because they must name real published FILES.
- [x] 12.12 RED/GREEN — heading-order defect exposed by the new fixture: with real breaks now present,
  `ChartIsland` rendered the break list under an `<h4>` directly after the page `<h1>`, skipping two levels
  and failing axe's `heading-order` rule. Fixed by a `breaksHeadingLevel?: 2 | 3 | 4` prop (defaulting to 4,
  which is correct in the workbench where the island sits under a section heading);
  `IndicatorPage.astro` passes `2`. Covered by a new describe block in
  `web/test/pages/indicator-page.container.test.ts` that asserts the level on a series that HAS breaks — this
  defect was structurally invisible while every fixture series had `breaks: []`.
- [x] 12.13 Verify: `npm --prefix web run check` (`astro check`) — 0 errors, 0 warnings, 2 hints across 93
  files; `npm --prefix web test` — 400/400 across 32 files; `npm --prefix web run test:e2e` — 51/51;
  `go test ./...` — green across all 23 packages (+ `app/migrations`, no test files);
  `npm --prefix web run build` then `npm --prefix web run budget:lighthouse` against a real preview server —
  all six pages PASS at 34.2-60.0 KB against the 300 KB budget, measured against the full-history fixture.
  Port hygiene confirmed clear before and after; the preview server was killed.
- [x] 12.14 Verify the state of verify-report's three assertion-quality WARNINGs rather than assume them.
  All three read as closed on disk 2026-07-30. (a) `sliceRange.test.ts`'s misleading title — see 12.15.
  (b) The 1-of-10-file inlined-copy scan — widened to 11 files by slice 10.5. (c)
  `expect(html).toContain('data-testid="page-yoy-variation"')`, which passed while the rendered figure was
  the placeholder dash: `indicator-page.container.test.ts` now has a `renderedText()` helper and asserts
  `not.toBe("—")` plus `toMatch(/^[+-]\d+(\.\d+)?%$/)` on both the YoY and intra-annual figures for every
  slug. That assertion is only meaningful BECAUSE of task 12.6 — against the old 3-point fixture the figure
  genuinely was a dash, so the strengthened assertion and the fixture had to land together.
- [x] 12.15 **Open at the moment this slice's record was written; CLOSED shortly afterwards by slice 13.**
  WARNING-5 / task 8.7's `personalizado` custom range preset was being built by a concurrent writer WHILE
  this record was written, with pieces landing between successive checks. At 11:57 on 2026-07-30 the
  supporting layers had landed (`CUSTOM_RANGE`, `RangeSelection`, permalink encoding, the Spanish copy) but
  `ChartIsland.svelte` still carried ZERO references to any of it, so no control a reader could operate
  existed and the disclosure was left standing rather than claimed. That snapshot was accurate when taken
  and is kept here as the record of what was verified at the time; **it was superseded within the hour —
  see slice 13, where the closure is recorded with its own verification.** The lesson is recorded rather
  than tidied away: a point-in-time observation of another writer's in-flight work is a statement about a
  moment, not about the change, and must be labelled as one.
  One related fault WAS closed inside this slice's own window, by the same writer and verified here:
  verify-report's assertion-quality finding that `sliceRange.test.ts:38` claimed spec conformance under the
  title "names exactly the five spec-mandated presets". The title now reads "RANGE_PRESETS names the
  full-series default plus the four fixed presets, and never the custom selection" and asserts
  `not.toContain(CUSTOM_RANGE)` — it no longer freezes the gap as green.

---

## Slice 13 — `personalizado` custom range preset (verify-report WARNING-5, task 8.7)

The last open requirement of this change. Built by a writer running concurrently with slice 12's
documentation pass, which is why slice 12's own record shows it open — that snapshot was taken before this
landed. Every entry below was verified on disk and by re-running the suites after it landed.

Files: `web/src/lib/transform/sliceRange.ts`, `web/src/lib/chart/permalink.ts`,
`web/src/components/ChartIsland.svelte`, `web/src/i18n/es.ts`, `web/test/transform/sliceRange.test.ts`,
`web/test/chart/permalink.test.ts`, `web/tests/e2e/{workbench/chart-island.spec.ts,
workbench/chart-no-js.spec.ts, workbench/workbench.spec.ts, workbench/workbench-page.ts,
indicator/indicator-pages.spec.ts, indicator/indicator-page.ts}`.

- [x] 13.1 RED/GREEN — pure layer in `web/src/lib/transform/sliceRange.ts`: `CUSTOM_RANGE = "custom"` and
  `type RangeSelection = RangePreset | typeof CUSTOM_RANGE` (a custom range carries its own `from`/`to`
  pair, so it is a selection ALONGSIDE the preset enum, never a bare member of it — `RANGE_PRESETS` is
  correctly unchanged); `resolveCustomRange()` returning a
  `{ status: "ok"; from; to; clamped } | { status: "rejected"; reason }` union; and
  `periodStartCalendarDate()`, the inverse of `periodFromCalendarDate`, used to prefill the date inputs and
  set their `min`/`max` when a custom permalink is reopened (deliberately lossy to a period's FIRST day, so
  a round-trip through `periodFromCalendarDate` is exactly stable and a reopened permalink does not move
  under the reader).
- [x] 13.2 RED/GREEN — out-of-span behaviour, three branches, and the reasoning is the point:
  (a) PARTLY outside the span → clamped to the span with `clamped: true`, and the caller DISCLOSES the
  narrowing in the status line, because a silent clamp would be the free-form equivalent of the disabled
  preset the spec forbids — a control that appears to honour the request while quietly showing something
  else; (b) containing not one observation → REFUSED, chart left exactly as it was, reason shown — the
  direct analogue of "absent, not disabled", since an empty chart is a view of nothing; (c) inverted, blank
  or unparseable → refused. The coverage check runs against the series' real observation list, not merely
  against `[first, last]`, which is what also catches a range landing inside a cadence gap and what makes
  "never render an empty chart" a guarantee rather than a hope.
- [x] 13.3 GREEN — `tryPeriodOrdinal`, a non-throwing wrapper over `periodOrdinalIndex`. `parsePeriod`
  throws by design on a label that does not match the frequency, which is right for the export artifact's
  own data (a mismatch there is a genuine data-contract defect, P4 fail-closed). A custom range arrives from
  two places that are NOT internal data — two date inputs and a query string a reader can hand-edit to
  anything — where a throw would take the whole island down and a rejection is the correct fail-closed
  response.
- [x] 13.4 RED/GREEN — permalink: `?range=custom&from=…&to=…` round-trips through
  `web/src/lib/chart/permalink.ts`. A hand-edited permalink takes the SAME judgement as the UI but degrades
  silently to the full range rather than showing an error — verified in `permalink.test.ts`: a missing
  bound, `range=custom` with no bounds at all, an unparseable bound, and `range=custom` offered to a caller
  that does not support a custom range all decode to the default range.
  **Disclosed territory deviation**: `permalink.ts` was just outside the file territory this writer was
  given. They extended it anyway and justified it — a custom range must encode into the same query string
  the presets already own, and splitting that across two independent owners would have been the worse
  outcome. Recorded here rather than left implicit.
- [x] 13.5 GREEN — UI in `ChartIsland.svelte`: two labelled `<input type="date">`, a commit button and a
  polite live-region status line (`data-testid` values `custom-range-from`, `custom-range-to`,
  `custom-range-from-label`, `custom-range-to-label`, `custom-range-apply`, `custom-range-status`). Draft
  state and committed state are kept apart deliberately, which is what makes a refusal non-destructive: a
  rejected range leaves the chart exactly as it was, with the reader's own input still in the fields to
  correct.
- [x] 13.6 RED/GREEN — the picker renders only after `onMount` (a `hydrated` flag), so it is **absent**
  without JavaScript rather than present-but-dead. A statically built page cannot make a free-form range
  work at all, and shipping it unconditionally would put two date inputs and a commit button in front of a
  no-JS reader that look exactly like every working control on the page and do nothing — the same
  discipline the spec states for a preset that cannot apply ("absent, not disabled"). Gated at SSR level and
  proven by a real `javaScriptEnabled: false` context
  (`chart-no-js.spec.ts`, "the custom range picker is absent — not present-but-dead — with JavaScript
  disabled").
- [x] 13.7 GREEN — new Spanish copy in `es.chart.customRange`: 10 strings (`groupLabel`, `fromLabel`,
  `toLabel`, `applyLabel`, `appliedNote`, `clampedNote`, and four `error.*` messages, one per
  `CustomRangeRejection`). **Produces user-facing Spanish copy — needs editorial sign-off**, flagged the
  same way as every other new reader-facing string in this change (task 8.4's per-capita disclosure, slice
  11's banner copy, `FreshnessSemaphore`'s "Al día").
- [x] 13.8 Widen the 44 px touch-target sweep, which was **blind to `<input>` before this change** — the
  custom range added the first real form control to this product, so the sweep would have shipped without
  ever looking at it. `interactiveControls()` in both
  `tests/e2e/workbench/workbench-page.ts` and `tests/e2e/indicator/indicator-page.ts` now reads
  `'a, button, summary, input:not(.sr-only), [tabindex="0"]'`. The `.sr-only` carve-out is deliberate and
  documented: `IndicatorChart.astro`'s annotation toggles are visually-hidden native checkboxes whose ENTIRE
  touch target is the associated `<label>`, which the selector already measures — measuring the 1×1 px
  checkbox itself would fail a control that is, in the reader's hands, 44 px tall. Both sweeps now also wait
  for hydration and for `custom-range-apply` before running, or they would pass by not looking.
- [x] 13.9 **Disclosed, NOT fixed here — the five FIXED preset buttons are server-rendered and equally
  inert without JavaScript.** That predates this change and is outside WARNING-5's scope. Recorded in
  `chart-no-js.spec.ts`'s own comment rather than silently swept into this slice. It is a real open item for
  a future slice: either render them after hydration too, or make them work without JS.
- [x] 13.10 Verify: 3 mutation checks, all reverted. Re-run independently for this record after the slice
  landed — `npm --prefix web run check` 0 errors / 0 warnings / 2 hints (93 files);
  `npm --prefix web test` **426/426 across 32 files** (+26 over slice 12's 400);
  `npm --prefix web run test:e2e` **64 passed** (+13 over slice 12's 51);
  `npm --prefix web run build` exit 0 with zero a11y warnings; `npm --prefix web run budget:lighthouse`
  against a real preview server measuring the production build — all six pages PASS at 34.2-62.1 KB against
  the 300 KB budget. **The writer did NOT run the budget gate and estimated ~1.5 KB of added markup per
  page; that estimate is superseded by the measurement above and is not recorded as measured.** Measured
  delta against the pre-slice run on the same machine: +2.1 to +2.2 KB on five of the six pages, with
  `ipc-subyacente` reporting 34.2 KB both before and after — unchanged to the tenth of a KB, which is not
  explained and is recorded as an anomaly rather than smoothed into a uniform per-page figure.

---

## Slice 14 — the ingest→export→build chain proves "corrupted artifact fails the build" (task 4.13's read half)

Closes design.md's slice-4 Open Question, the last item slice 13 had named as open. Task 4.13 shipped the
structural chain in slice 4 with its read half explicitly disclosed as unproven; this closes that half.

Files: `.github/workflows/ingest-export-build.yml`, `scripts/assert-corrupt-artifact-fails-build.sh` (new),
`app/internal/ingestion/e2e_export_test.go`, `.gitignore`.

- [x] 14.1 Confirm slice 12's diagnosis before acting on it, rather than trusting the record. Confirmed in
  full, element by element: the `astro build` step set neither `EXPORT_URL` nor `EXPORT_DIR` so the loader
  fell back to the checked-in fixture, while the Go step exported into a `t.TempDir()` discarded on exit —
  the two halves of the job never touched the same bytes. The workflow's header now records this superseded
  rationale rather than deleting it.
- [x] 14.2 **The finding slice 12's diagnosis did not have, and the non-obvious part of this slice**:
  `getStaticPaths` filters `INDICATOR_CONTENT` down to the slugs the artifact actually carries
  (`Object.keys(INDICATOR_CONTENT).filter((slug) => seriesBySlug.has(artifactSlugFor(slug)) && …)`), so
  merely pointing `EXPORT_DIR` at the old test's output would have built a site with ZERO indicator pages
  and STILL exited 0. Connecting the directories was necessary but not sufficient; a plausible one-line fix
  would have produced a green job proving less than nothing.
- [x] 14.3 GREEN — one job-level `EXPORT_ARTIFACT_DIR: ${{ github.workspace }}/.e2e-export-artifact`,
  declared once and read by both halves: the Go step passes it as `E2E_EXPORT_DIR`, the build step as
  `EXPORT_DIR`. Not two literals free to drift apart. `.gitignore` gained `/.e2e-export-artifact/`.
- [x] 14.4 GREEN — `exportOutputDir` in `app/internal/ingestion/e2e_export_test.go` reads `E2E_EXPORT_DIR`,
  resolves it to an absolute path, fatals if it cannot, and `MkdirAll`s it (`publishing.Export` creates
  `outDir/series` and `outDir/csv` but not `outDir`). Unset — every ordinary `go test ./...` — it returns
  `t.TempDir()`, so the test still leaves nothing behind and needs no writable project path.
  **Why an env-driven output directory rather than invoking `concontexto export` as its own step**: the
  test's rows live in a transaction rolled back on exit, so no separate process could ever see them. The
  export must happen in-process.
- [x] 14.5 GREEN — a hand-off guard fails the job if `manifest.json` is absent at that path. This is what
  makes the fix durable rather than merely correct today: a renamed variable or a skipped test can no longer
  degrade into a silent fixture fallback, which is the exact failure mode being closed.
- [x] 14.6 GREEN — consumption assertions, because a green build can mean "validated the artifact" or "never
  looked at it": all six frozen routes must exist in `dist/`, and the latest value the ARTIFACT carries must
  appear in the rendered HTML, read out of the artifact at run time rather than hard-coded so the assertion
  cannot drift from the data it checks. The ROUTE slugs are listed (`pib`, not `pib-cvi`), so the step also
  proves the alias resolves. The Open Question's own wording, "the rendered page shows the value that was
  ingested", is now mechanically true against `9.87` — a genuinely-real INE 2026-Q2 tail row.
- [x] 14.7 RED/GREEN — `scripts/assert-corrupt-artifact-fails-build.sh` (new, executable): three corruption
  scenarios, one per validation layer, each against its own throwaway copy of the honest artifact, which is
  never modified. (1) `sha256-mismatch` — a value edited as raw text with the manifest digest left stale,
  caught before parsing. (2) `schema-version` — `schema_version: 2` with the digest **REISSUED**, so the
  digest check passes and the exact-integer version check is demonstrably what rejects it. (3) `zod-shape` —
  required `pageState` deleted with the digest **REISSUED**, so Zod is demonstrably what rejects it.
- [x] 14.8 GREEN — each scenario must produce both the expected message AND attribution to the loader
  (`export loader:|parseSeriesDoc|parseManifest`). A build that dies in Vite, on a missing dependency or in
  an unrelated page matches none of those and is refused as proof. That second guard is what separates "the
  artifact validation worked" from "something happened to be broken".
- [x] 14.9 **Prove the assertion is not invertible, rather than asserting it — done twice, independently.**
  (a) Accidentally, by the guard catching its own author: the first version used `process.argv.slice(2)`,
  wrong for `node -e`, so the corruption silently no-op'd; the script reported "3 of 3 corruption scenarios
  did NOT fail the build as required" and exited 1. Arguments now travel through the environment with a
  comment explaining the offset, and the script additionally `diff`s the corrupted copy against the honest
  one before the build is allowed to mean anything. (b) Deliberately, as a negative control:
  `delete doc.pageState` was swapped for ADDING an unknown field with a reissued digest — Zod strips unknown
  keys, so the build legitimately succeeds, and the script correctly reported FAIL and exited 1. A guard
  that cannot be shown to fail is not a guard.
- [x] 14.10 Disclosed, deliberately NOT closed: `manifest.json` is itself not digest-verified and cannot be —
  it CARRIES the digests. It is Zod-validated, which is what catches a corrupted manifest, and the
  schema-version scenario deliberately targets a SERIES document so the two gaps are not conflated. Also
  disclosed: the corruption steps leave `web/dist` holding a failed build's output — harmless, nothing
  deploys from this job, and documented in the workflow comment as why the consumption assertions run first.
- [x] 14.11 Verify, with the proven/reasoned split recorded precisely and NOT upgraded. **No CI run has been
  observed and no claim is made about one.** Proven locally by the writer: the whole step sequence with real
  Docker, Postgres, ingest, export and build, plus all three corruption failures. Re-verified independently
  for this record: `shellcheck` clean (one annotated `SC2016`, correct — the embedded JavaScript owns its own
  `${…}` template literals and must not be shell-expanded); `bash -n` clean; `check-env-example.sh` OK (10
  variables, none new); `go build ./...` and `go vet ./...` clean; `go test ./...` **23 packages ok, 0 FAIL**,
  exit 0; `npm --prefix web test` **426/426 across 32 files**; `astro check` **0 errors, 0 warnings, 2
  hints**. Reported but NOT re-verified: `actionlint` 1.7.12 clean on all four workflows, sanity-checked
  against a deliberately broken copy that flagged both injected faults — `actionlint` is not installed on
  this machine. Only REASONED about, never observed: GitHub-hosted runners having Docker preinstalled
  (unchanged from this job's previous version, which already relied on it), job-level `env` expansion and
  `github.workspace` resolving at runtime (actionlint validates context availability, not runtime
  expansion), and `setup-go`/`setup-node`/`npm ci` behaviour on a clean runner.

---

## Slice 15 — the build refuses to drop a frozen indicator route (verify-report pass-4 CRITICAL-27)

Commit `4b20ca7`, `fix(web): refuse to build rather than silently drop a frozen indicator route`.
Files: `web/src/lib/indicator/routes.ts` (new, 201 lines), `web/src/pages/indicador/[slug].astro`,
`web/test/indicator/routes.test.ts` (new, 180 lines), `web/test/export/missing-slug-fails-build.test.ts`
(new, 186 lines), `web/.gitignore`.

- [x] 15.1 Name the defect precisely before fixing it. `getStaticPaths` read
  `Object.keys(INDICATOR_CONTENT).filter((slug) => seriesBySlug.has(artifactSlugFor(slug)) && …)`. Upstream,
  `export.go` skips a series with zero observations, so a never-published or gate-blocked series is absent
  from both `series/` and `manifest.series` — and the digest chain and the Zod loader then validate
  perfectly. The result was a green build, five indicator pages, and a 404 on a permalink this project
  promised to keep permanent, with nothing anywhere reporting it. Recorded as the **third** appearance of
  "a check placed where the failure it guards cannot occur": the only all-six assertion lives in
  `ingest-export-build.yml`, which builds from a fixture that always contains all six.
- [x] 15.2 GREEN — route resolution extracted out of the `.astro` file into
  `web/src/lib/indicator/routes.ts`. The move is not cosmetic: a `getStaticPaths` is reachable only through
  a real `astro build`, so the rule could not be asserted at unit level at all while it lived there.
- [x] 15.3 GREEN — `resolveIndicatorRouteSlugs` iterates `FROZEN_INDICATOR_SLUGS` (declared in that module,
  six entries) rather than whatever configuration happens to exist. Deriving the routes from
  `INDICATOR_CONTENT` is the same defect pointing the other way — deleting a content entry would silently
  shrink the site. It returns all six verbatim or throws; there is no filter and no partial return.
- [x] 15.4 GREEN — three failure classes worded separately, because they have different fixes. Missing from
  the artifact is a **pipeline** problem and says so ("THIS IS A PIPELINE PROBLEM, not a problem in the web
  tree"), naming the two causes and telling the reader not to edit the web tree to make the build pass.
  Missing from `METHODOLOGY_CONTENT` or from `INDICATOR_CONTENT` is a **content-authoring** problem and
  names the file and the fields. A fourth message covers a slug present in `INDICATOR_CONTENT` but not
  frozen.
- [x] 15.5 GREEN — `web/test/indicator/routes.test.ts`, 8 cases, including one asserting that the pipeline
  wording does not leak into the two authoring cases. Wording is load-bearing here: the whole value of the
  throw is that it tells the reader which of three unrelated remedies applies.
- [x] 15.6 RED/GREEN — `web/test/export/missing-slug-fails-build.test.ts` drives a **real `npm run build`**
  against a real artifact with one slug removed in the exact three-part shape `export.go` produces
  (`series/{slug}.json`, the `manifest.series` entry, and the digest), **paired with an unmutated control
  build** that must exit 0 and emit all six. The control is what makes case 1 falsifiable: without it, a
  build failing for an unrelated reason would read as proof.
- [x] 15.7 A fourth "no data yet" page state was considered and **rejected**: the spec separately requires
  that the chart MUST NOT be hidden in any state, and a page for a series with zero observations has no
  chart to show. Recorded because the rejected option is the one a later reader will propose.
- [x] 15.8 Consequence stated, not softened: a real production build now **FAILS** while `ocupados-epa` is
  held by the publish gate. The site was already missing that route and shipping anyway; the only change is
  that it now says so. No env var, no allowlist, no escape hatch. This is the direct cause of verify-report
  pass-5 CRITICAL-37, and that is the correct behaviour, not a regression.

---

## Slice 16 — a human, and only a human, can resolve a blocking finding (new capability); plus the Go half of the publish loop

Commit `bdb6cc8`, `feat(validation): let a human resolve a blocking finding, and only a human`.
**Recorded as it landed, not as its message describes it**: the commit body documents only the
acknowledgement registry, but the commit also carries the entire Go half of CRITICAL-28's remedy —
`app/cmd/concontexto/rebuild_dispatch.go` (+ 292 test lines), `app/internal/scheduler/watchdog.go`
(+ 89 test lines), `app/internal/publishing/trigger.go` (+ 55 test lines) and `schedule.go` (+111). Verified
with `git log --diff-filter=A -- app/cmd/concontexto/rebuild_dispatch.go` → `bdb6cc8`. Two work units in one
commit, and the undescribed one is the larger operational change.

- [x] 16.1 Name what was actually missing. `SeverityBlockRequiresSignoff` has existed since PR 4b, and
  `rule4_revision.go` explains its purpose at length — a deep revision is either a legitimate methodology
  revision or a parser silently rewriting history, the machine cannot tell, so the decision goes to a human.
  **Nothing in the codebase ever resolved it**: `gate.go` treated it identically to `SeverityBlock`
  (`blocks()` returns true for both, `gate.go:97`). The comment handed a decision to a human and gave the
  human no way to hand it back, so every blocking finding was permanently terminal.
- [x] 16.2 The live proof the gap was real, not theoretical: `ocupados-epa` is blocked on every real ingest
  by `rule3-plausibility` — a period-over-period change of 1074.1 at 2020-Q2 against a `max_delta_abs` of
  1000. Measured across the series' real history, **exactly one of 97 deltas** breaches the threshold, with
  median 152.4, p95 503.3 and second-largest 770.9 (2009). The threshold is well calibrated; raising it
  would blind the guard permanently to avoid looking at one number once.
- [x] 16.3 Two remedies considered and correctly rejected, recorded because they are the obvious ones.
  Raising the threshold: see 16.2. Recording the quarter in `config/rupturas.yaml`: that registry's own
  header declares it holds **methodological** ruptures, and the 2020-Q2 collapse was real economics —
  filing it there would falsify the registry and destroy the exact real-versus-methodological distinction
  rule 3 is built on.
- [x] 16.4 GREEN — `app/internal/ingestion/validation/acknowledgement.go` (243 lines): a record names one
  series, one period and one rule, pins the observed value, and carries provenance. Scope is compared by
  exact equality; there is no syntax in the schema for "all periods", "the whole series" or "all rules". A
  mechanism that can blanket-disable a guard is worse than the gap it fills.
- [x] 16.5 GREEN — only `rule3-plausibility` and `rule4-revision` are acknowledgeable
  (`config.AcknowledgeableRules`), and `TestAcknowledgeableRules_AreExactlyRule3AndRule4` runs the **real
  rules** to assert the allowlist matches what they actually emit, so it cannot drift. Those two are the
  ones whose finding means "this number is surprising and I cannot tell legitimate from broken". Rule 2 is
  deliberately excluded because its own requirement says the remedy is to correct the series configuration,
  never to relax the rule.
- [x] 16.6 GREEN — staleness is handled by **pinning the observed value**, not by an expiry date. The
  calendar is unrelated to whether the datum changed, and an expiry would re-block correct data while still
  covering revised data — wrong in both directions. A revised value raises its own blocking
  `acknowledgement-stale` finding naming the record and both values, rather than letting the original
  finding reappear unexplained.
- [x] 16.7 GREEN — an override is never mistakable for a pass. Outcome `publish-overridden`
  (`GatePublishOverridden`, `gate.go:47`), the log carries an `acknowledgements` attribute naming the record
  and the signer, the level is WARN not INFO, and `ingestion_run.outcome` reads
  `succeeded-with-acknowledgement` (`postgres/gate.go:69`).
- [x] 16.8 **THE AUTHORITY IS THE SIGNATURE — and the first version of this faked it.** See slice 16's
  section in `apply-progress.md`; recorded here as a task because the remedy is shipped code, not a note.
  A record now has two mutually exclusive states, modelled on `rupturas.yaml`'s `date_status: unconfirmed`
  discipline of never projecting an unverified fact.
- [x] 16.9 GREEN — an unsigned record is inert in **two independent layers**, because a mechanism whose
  safety rests on one layer never being bypassed is not safe. Layer 1, the reconcile never projects it:
  `reconcile.go:116` routes `SignatureStatus == "unsigned" || AcknowledgedBy == ""` to
  `PendingAcknowledgementIDs` and never to `ackInputs`. Layer 2, the pure gate refuses it again:
  `acknowledgement.go:168`, `if !a.signed() { continue }` inside the finding-matching loop, with a second
  `!a.signed()` guard suppressing the "unused, ready to retire" advisory — the right place to surface a
  draft is the reconcile's pending list, which says what is actually true.
- [x] 16.10 GREEN — `app/internal/adapters/config/acknowledgement.go` (446 lines) rejects at
  `validate-config`: an empty, too-short (< 2 characters) or placeholder signature across a vacant-token
  table; a signed record with no date; and a record that is both unsigned and signed at once. **Counted for
  this record: the table holds 19 tokens, not the 18 the commit body and the verify-report both state.**
  Small, and recorded rather than copied, because a record that repeats a figure it did not check is how the
  next wrong figure gets in.
- [x] 16.11 GREEN — supporting surfaces: `app/internal/adapters/postgres/acknowledgement.go` (216 lines),
  migration `0006_validation_acknowledgement` (up 78 / down 12), `config/reconocimientos.yaml` (108 lines,
  one record, unsigned), `config/README.md` (+25), and a new `data-validation` requirement
  ("Acknowledged findings") with **nine** scenarios in the delta spec.
- [x] 16.12 GREEN, undescribed in the commit message — the Go half of CRITICAL-28: `buildRebuildDispatcher`
  and the `APP_REBUILD_DISPATCH` three-state parse (`rebuild_dispatch.go`), `RebuildLatencyBreached`
  (`scheduler/watchdog.go:98`, consumed at `schedule.go:478`) and the `trigger.go` changes. Described under
  slice 17, where the rest of that work landed.

---

## Slice 17 — the publish loop's receiver, composition and deploy-completed signal (verify-report pass-4 CRITICAL-28)

Commit `1f856e2`, `fix(ops): close the publish loop, which was open at four links`.
Files: `.github/workflows/rebuild.yml` (new, 136 lines), `.github/workflows/deploy.yml`, `Dockerfile`,
`docker-compose.yml`, `docker-compose.override.yml.example`, `docs/deploy.md` (new, 148 lines),
`env.example`, `.gitignore`. The Go code this depends on landed in `bdb6cc8` (see 16.12).

- [x] 17.1 Name all four open links before fixing any. (L1) `design.md` planned a `repository_dispatch`
  rebuild job; `grep -rn repository_dispatch .github/` returned nothing, at HEAD and on `origin/main`.
  (L2) the compose `app` service passed no `GITHUB_DISPATCH_*`, so `buildDispatcher` returned nil and
  `trigger.go` returned silently. (L3/L4) the publish-latency watchdog compares the manifest `Publish` wrote
  seconds earlier in the same call, so it can only ever detect an export that did not run — never a dispatch
  never sent, a rebuild that failed, or a deploy that never landed.
- [x] 17.2 State the net effect in the deployed stack, so the severity is not inferred from the file count:
  pre-rendered pages frozen at whatever the image was built from, `/data-derived` refreshed every fifteen
  minutes, the two diverging silently, and the spec naming an alert — "a failed or undispatched site
  rebuild" — that nothing could raise.
- [x] 17.3 GREEN — L1: `.github/workflows/rebuild.yml` is the missing receiver, gated the way `deploy.yml`
  already is: unconfigured means a visible warning and no action, never a fabricated success. It verifies
  the origin actually serves the dispatched artifact before calling deploy, so a dispatch naming an artifact
  the site does not have fails rather than rebuilding something else. `deploy.yml` gained `workflow_call: {}`.
- [x] 17.4 GREEN — L2, the subtler half. The binary cannot infer whether it is deployed, but the compose
  file is exactly that difference, so the default lives there: `APP_REBUILD_DISPATCH` defaults to `required`
  for the `app` service and to `off` in the local override. Off means nil and one INFO record.
  **Required-but-unconfigured returns a dispatcher that fails immediately rather than nil**, routing the
  undispatched case down the already-tested dispatch-failure branch instead of the silent one. Unrecognised
  values fail loud — the inverse of `scheduleDisabled`'s fail-open, because here the quiet outcome is the
  unsafe one.
- [x] 17.5 GREEN — L3/L4, a deploy-completed signal that cannot be faked. The image records the artifact its
  pages were rendered from, at a path **outside `dist/`** so the export volume cannot mount over it. That
  stamp changes by exactly one mechanism — a new image being deployed — which cannot happen unless dispatch,
  rebuild and redeploy all succeeded, so one comparison inside the container covers the three remaining links
  with no call to GitHub, Portainer or the public site.
- [x] 17.6 GREEN — the watchdog declines to fire when dispatch is off, because divergence is then
  intentional, and when there is no stamp at all, because unknown is not stale. Start-up says which, so
  silence stays readable.
- [x] 17.7 Disclosed and not papered over: a real dispatch round trip **cannot be proven here**. There is no
  provisioned VPS and no `PORTAINER_WEBHOOK_URL`. The one live attempt returned a genuine 401 from a
  deliberately invalid token, which proves the request is well formed and reaches `api.github.com` and
  proves nothing about the receiver. `rebuild.yml` calling `deploy.yml` is actionlint-validated and never
  dispatched.
- [x] 17.8 Disclosed, and it is a spec gap rather than a disclosure: the `pipeline-operations` scenario "a
  failed rebuild raises an alert **immediately**" is **substituted, not implemented**. Nothing observes a
  failed CI rebuild; `alerting.DispatchFailed` fires when the POST fails, which is a different event. A
  failed `rebuild.yml` run produces no callback and is caught only by `RebuildLatencyBreached` once the
  30-minute budget elapses. That is a good substitution and it does close L3 — but "immediately" is not what
  happens, and until this entry the substitution was disclosed only in a comment inside `rebuild.yml`.
  Verify-report pass-5 WARNING-41; now recorded in the change's own record as well.

---

## Slice 18 — CI ingests with the real thresholds, so a green signal can go red (verify-report pass-5 CRITICAL-37, structural half)

Commit `27cc0c9`, `test(ingest): ingest with the real thresholds, so CI can go red for this`.
Files (`git show --numstat`): `.github/workflows/ingest-export-build.yml` (+99/−2),
`app/internal/ingestion/e2e_blocked_export_test.go` (new, 212),
`scripts/assert-blocked-series-fails-build.sh` (new, 197), `app/internal/ingestion/ingest_test.go` (+108/−7),
`app/internal/ingestion/e2e_export_test.go` (+36/−29), `app/internal/ingestion/acknowledgement_e2e_test.go`
(+18/−16), `app/internal/ingestion/population_cadence_triage_test.go` (+16/−3),
`app/internal/ingestion/logging_alerting_test.go` (+2/−2). 688 added, 59 removed across 8 files.

- [x] 18.1 Name the defect at the exact line before changing anything. `ineIngestConfig`
  (`app/internal/ingestion/ingest_test.go`) passed `Validation: config.ValidationConfig{}` — the zero value,
  no thresholds at all — so the one CI job that exercises the real Go→Astro hand-off ran the whole chain
  with rule 3 unable to fire. Every CI signal stayed green while a real production build could not ship.
  This is the **fifth** instance of the check-positioned-where-the-failure-cannot-occur pattern already
  recorded as a design.md Open Question, and the first that is not a bug in any one gate: each gate is
  correct, and no CI path was ever handed production's own artifact shape.
- [x] 18.2 GREEN — `ineIngestConfig` now reads the shipped `validation:` block instead of restating it.
  `shippedConfig` (`ingest_test.go:118`, a `sync.OnceValues`) walks `configdata.FS` → `fs.Sub` →
  `config.Load` — the same three calls `validate-config` and every `ingest` invocation make — and
  `realValidationConfig` (`ingest_test.go:152`) returns one slug's block from it. There is now no threshold
  literal on the test side of this path to drift away from the YAML.
- [x] 18.3 GREEN — the contract test that makes 18.2 non-vacuous:
  `TestIneIngestConfig_CarriesEveryShippedThresholdForTheSixFrozenSlugs` compares all six frozen slugs
  field-for-field against the YAML, **and checks its own fixture for vacuity first** — a slug declaring no
  plausibility bound at all is called out rather than silently passing. Verify-report pass 6 confirmed this
  by mutation: reverting `ineIngestConfig` to `config.ValidationConfig{}` fails it for all six slugs.
- [x] 18.4 GREEN — the same defect one level down, found while proving 18.2 and worse than the finding as
  written. `acknowledgement_e2e_test.go`'s `ocupadosCovidCase()` returned a hand-copied
  `config.ValidationConfig` (`maxDelta := 1000.0`, `maxValue := 30000.0`) that `runCovidIngest` then assigned
  over `icfg.Validation`. Raising `max_delta_abs` in `config/series/ocupados-epa.yaml` therefore left green
  the test whose entire subject is that file. `ocupadosCovidCase()` now returns the series identity only and
  carries no thresholds; the run reads them through `ineIngestConfig(t, sc, cod)`. A restated threshold is a
  second source of truth, and this file exists because of what that one threshold decides.
- [x] 18.5 GREEN — the new arm asserts the **failure** path, not a fully successful chain.
  `TestEndToEndBlockedSeriesIsAbsentFromTheExportedArtifact` (`e2e_blocked_export_test.go`, 212 lines)
  ingests the recorded COVID range through a real Postgres, then asserts the block by **rule name and
  period**, asserts the export omits exactly that slug, and asserts the other five are present — so a block
  for any other reason, or an empty export, fails it. **Why the failure path and not the success path**: a
  job that stays red until a human signs a YAML file is a job that gets ignored within a week. This one is
  green today for the right reason and goes red if the block stops happening.
- [x] 18.6 GREEN — `scripts/assert-blocked-series-fails-build.sh` (197 lines) builds the blocked and the
  honest artifact as a **matched pair**: the blocked one must exit non-zero naming `ocupados-epa`, and the
  honest one must exit 0 with all six routes. A guard that only ever sees the failing input cannot tell a
  real refusal from a build that was broken anyway — the same paired-control discipline slices 14 and 15
  used, and the generalisable rule the design.md Open Question already states.
- [x] 18.7 GREEN — the CI wiring, without which the Go half proves nothing on a runner.
  `ingest-export-build.yml` gained a `BLOCKED_ARTIFACT_DIR`, runs both e2e exports under
  `-run 'TestEndToEndIngestExportBuild|TestEndToEndBlockedSeriesIsAbsentFromTheExportedArtifact'`, fails
  the step if the blocked arm produced no `manifest.json` (so an export that did not happen cannot be read
  as a passing assertion), and invokes the script at line 262 with both artifact directories.
- [x] 18.8 Disclosed, confirmed by verify-report pass 6, and **not closed by this commit**: the *successful*
  arm still runs the recorded three-period fixtures, whose largest period-over-period ratio is 486.0 against
  a threshold of 1000, so they cannot breach any shipped threshold even with validation on. CI therefore
  proves the mechanism in **both directions** and does not prove that today's production artifact builds.
  The commit body's "goes red the day one bites there" is true for a *lowered* threshold, not for new data —
  the fixtures are frozen files. Recorded here as a narrowing, not as a false claim.

---

## Slice 19 — the published directory contains exactly what the manifest declares (found on the running stack)

Commit `5310586`, `fix(export): the published directory now contains exactly what the manifest declares`.
Files (`git show --numstat`): `app/internal/publishing/export.go` (+155),
`app/internal/publishing/export_prune_test.go` (new, 263), `app/internal/publishing/artifact.go` (+32),
`app/cmd/concontexto/export_cmd.go` (+34), `app/cmd/concontexto/export_cmd_test.go` (+96),
`app/cmd/concontexto/ingest_cmd.go` (+7), and the `publishing-export` delta spec (+51). 638 added, 0
removed across 7 files. **This is the only commit in the change to add a delta-spec requirement after the
spec phase closed**, which is why it carries its own task rows rather than a note.

- [x] 19.1 The defect, and how it was found — **by running the deployed stack and reading the served site,
  not by a test**. `ocupados-epa` was blocked by the publish gate, so `Export` skipped it and the manifest
  came out with nine series and nine digests, none of them that slug. The container nevertheless served
  `GET /data-derived/csv/ocupados-epa.csv` and `GET /data-derived/series/ocupados-epa.json` at 200, from
  bytes an earlier export wrote while the series was still published. `Export` only ever wrote;
  `writeFileAtomic` replaces and never deletes. Data reachable at a URL the indicator page still links to,
  carrying no digest, absent from the manifest, indistinguishable to a reader from current data — this
  project's founding principle inverted.
- [x] 19.2 Confirm no existing requirement covered it before writing a new one. The closest, "`/data-derived`
  is generated from the same artifact", governs how each written file is **derived** so the CSV and site
  projections cannot disagree; it says nothing about files the export stops writing, and nothing at all
  about the JSON side. Verify-report pass 6 independently re-checked this and agreed the gap was real. New
  requirement "The published directory contains exactly what the manifest declares", three scenarios, with
  a parenthetical in the spec itself stating exactly what it does and does not weaken.
- [x] 19.3 GREEN — `pruneUnpublishedFiles` (`export.go:411`) removes, from the two subdirectories the export
  owns, every regular file whose base name the current export's own document set does not declare. All three
  scenarios have tests in `export_prune_test.go` (263 lines):
  `TestExport_ASeriesThatDropsOutOfALaterExportLeavesNothingBehind` asserts the invariant as **one set
  comparison against the manifest**, not two hand-listed filenames, so a future third projection cannot
  satisfy it while going stale.
- [x] 19.4 GREEN — the prune runs **last**, after the manifest's rename, and the choice is reasoned in
  `export.go:226-247` (line numbers as of `5310586`; that comment is being rewritten — see 19.8) because
  neither ordering is atomic. Pruning first opens a window where the manifest a
  reader currently holds declares a series whose files are gone — a 404 on a declared path and a digest that
  can never verify. Pruning last leaves a window where the directory holds *more* than the manifest
  declares, which is this bug's own steady state. Second window degrades to an unreferenced file; first
  degrades to a live 404. **The reasoning is sound inside one `Export` call and does not survive the build
  boundary — see 19.8.**
- [x] 19.5 GREEN — an export declaring **no series at all** removes nothing and reports `Skipped`
  (`export.go:413`). "What should exist" is derived from one export's own output, so an export producing
  nothing would otherwise delete the entire published artifact — a recoverable stale-file bug converted into
  unrecoverable data loss. `TestExport_RefusesToPruneWhenTheExportDeclaresNoSeries` goes RED when the guard
  is removed (verify-report pass 6, by mutation). The **partial** case is deliberately unguarded: five docs
  where nine were expected is indistinguishable, from inside `Export`, from a legitimate retirement of four,
  and any ratio floor eventually errs in the direction that loses data.
- [x] 19.6 GREEN — scope cannot escape. Only `<outDir>/series/*.json` and `<outDir>/csv/*.csv`, only regular
  files, base names from `os.ReadDir` so no traversal is expressible, and every non-regular entry skipped —
  a symlink named `x.json` is not followed, let alone removed. `.tmp-*` is never matched, which is
  load-bearing because `writeFileAtomic` creates `os.CreateTemp(dir, ".tmp-*")` in the destination directory
  on every write. `TestExport_LeavesEveryFileItDoesNotOwnUntouched` goes RED on `series/README.md` when the
  extension filter is removed (verify-report pass 6, by mutation).
- [x] 19.7 GREEN — a removal is never silent. `PruneOutcome` (`artifact.go:115`) carries `Removed` and
  `Skipped` and is `json:"-"` — deliberately not artifact content, because it describes what the run did
  rather than what the artifact contains. `pruneOutcomeMessage` (`export_cmd.go`) names each removed path
  **individually rather than counting them**, because "removed 2 files" gives an operator investigating a
  missing page nothing to correlate against, and is shared by `runExport` and the in-cycle publish so the
  two paths cannot report the same fact differently. `Skipped` is reported loudly for the reason it exists:
  an empty `Removed` cannot discriminate "nothing stale" from "the guard refused".
- [x] 19.8 Disclosed here rather than left in the verify-report, because all three are corrections to this
  commit's **record** and two of them contradict claims made in its own comments. Verify-report pass 6:
  **WARNING-44** — the ordering rationale's "bounded to milliseconds and unreachable through any
  manifest-driven path" is false across the build boundary. Measured by the verifier on the running stack at
  `5310586`: `/indicador/ocupados-epa/` serves 200 while both `href`s it emits unconditionally from
  `doc.slug` (`IndicatorPage.astro:199-200`) return 404, and the frozen-route guard *guarantees* no rebuild
  while the series is blocked, so this is the steady state rather than a window. Judged defensible under P4
  (a 404 is honest, stale bytes presented as current are not) and not a spec violation, but undisclosed.
  **WARNING-45** — the ordering is untested and the stated reason for the gap is a category error: a
  read-only directory is what the *unlink-failure* path needs, which is a different claim. The verifier
  proved the ordering testable in 25 lines and showed that moving the prune before the writes leaves the
  whole `publishing` package green. **WARNING-46** — both outer defences cited for leaving the partial case
  unguarded fail to cover it: the ingest gate is batch-scoped (`ingest_cmd.go:487`, one series learning
  anything arms the export for all ten) and `concontexto export` bypasses it entirely, and retention's
  snapshot is taken *after* the prune (`trigger.go:128-135`), so the first bad export's own snapshot already
  lacks the removed files. Neither observation changes the decision; both change what the record claims
  about why it is safe.

---

## Slices 20–36 — the seventeen commits verify-report pass 7 found unrecorded (CRITICAL-46)

Written 2026-08-04 at `5af95c5`, after pass 7 escalated pass 5's WARNING-40 to a blocker. Every figure in
these sections was measured here with `git show --numstat` per commit and `git diff --shortstat
823311e..HEAD` for the window: **17 commits, 112 files, 14,631 insertions, 425 deletions**. Nothing is
taken from a commit body without saying so, and where a commit body and the repository disagree the
repository wins and the disagreement is recorded rather than reconciled away.

**Read the honesty note before using the TDD Cycle Evidence tables in `apply-progress.md` for these
slices.** Not one of these seventeen commits recorded literal failing-test output. What exists instead is
three different kinds of evidence, and they are labelled as what they are: a measured defect in the running
product against which the new test's own threshold provably fails; a mutation check; and, for six of them,
nothing at all.

---

## Slice 20 — a null `Valor` fails closed instead of violating a database constraint

Commit `026c7fa`. Files: `app/internal/adapters/ine/envelope.go` (+71), `app/internal/adapters/ine/client.go`
(+9), `app/internal/adapters/ine/nullvalue_test.go` (new, 205), and the `source-ingestion-ine` delta spec
(+54). 339 added, 0 removed across 4 files. **The second commit in this change to add a delta-spec
requirement after the spec phase closed** (slice 19's `5310586` was the first), which is why it carries
task rows rather than a note.

- [x] 20.1 The defect. `envelope.go` passed the wire pointer straight through, so a `DATOS_SERIE` row
  carrying `"Valor": null` with `"T3_TipoDato": "Definitivo"` produced a nil-valued *definitive*
  observation and hit `CHECK (value IS NOT NULL OR status = 'W')` — surfacing as SQLSTATE 23514 from inside
  the publish gate, naming a Postgres constraint rather than the source behaviour that caused it.
- [x] 20.2 Latent, not live, and measured rather than assumed: the commit body records a live full-history
  probe of all six configured series — 1,032 rows, zero nulls. Recorded here as the commit's own
  measurement, not re-run for this record.
- [x] 20.3 Why the Eurostat fix (`befa81f`) does not transfer, argued rather than copied. A sparse
  JSON-stat map has no entry at a position: that is the *absence* of a datum, it is decidable, and no
  observation is emitted. An INE row exists and carries an explicit null: that is a positive act by the
  source, and discarding it would throw away something INE chose to publish.
- [x] 20.4 Why every available projection would be an invention. A `DATOS_SERIE` row carries exactly five
  fields — `Anyo`, `Fecha`, `T3_Periodo`, `T3_TipoDato`, `Valor` — with no secrecy marker, no
  not-applicable flag and no annotation; the only field that could annotate is series-level `Notas`, which
  for two series is a bare link to the INEbase page and can never explain one period. Statistical secrecy,
  a not-applicable period and a genuine gap are therefore indistinguishable, so `withdrawn` fabricates a
  retraction, `absent` discards a deliberately emitted row, and anything numeric is unthinkable. The
  adapter refuses.
- [x] 20.5 Classified `schema-drift` after argument, not by default: this adapter already classifies a
  periodicity mismatch and an unrecognised `T3_TipoDato` that way, so the established meaning is "the
  payload's CONTRACT is not what the adapter assumes". A republished null is still null on retry, so it
  belongs with the non-retryable classes. A sixth class was rejected structurally — `sourceerr`'s taxonomy
  mirrors `postgres.DownloadOutcome` 1:1 by documented design, so adding one needs a migration and
  fractures an invariant two packages document.
- [x] 20.6 One deliberate asymmetry with `befa81f`, flagged in both the code and the test: that commit
  judges flag vocabulary before value presence so an undocumented flag keeps its own diagnostic; this one
  does the opposite, because `DecodeSeries` and `FetchSeries` are exported and never run
  `classifyTipoDato`, so a check one layer up would leave two entry points able to hand back a nil-valued
  observation. Containment of the invariant beats symmetry of the diagnostic.
- [x] 20.7 GREEN — four scenarios, four named tests, all listed as newly compliant by verify-report pass 7
  §G: `TestDecodeSeries_NullValorFailsClosedRatherThanEmittingANilValuedObservation`,
  `TestDecodeSeries_NullValorFailsClosedUnderEveryTipoDatoToken` (4 sub-cases),
  `TestDecodeSeries_ZeroIsAPublishedValueNotAMissingOne`,
  `TestDecodeSeries_EveryDecodedObservationCarriesANonNilValue`.
- [x] 20.8 What this does NOT decide, written into the refusal message itself: what a null `Valor` means.
  That needs INE documentation or a human ruling, and the message names which projection has to be decided,
  where to record it, and that the archived raw file holds the verbatim payload to decide against —
  mirroring the acknowledgement registry's `todo`. **This closes verify-report WARNING-30**, which had the
  crash class disclosed only in a commit message.

---

## Slice 21 — a homepage that lists the six indicators, and a way back from them

Commit `ac69a29`. 779 added, 30 removed across 13 files. New: `web/src/lib/indicator/homeListing.ts` (111),
`web/src/templates/HomePage.astro` (89), `web/test/indicator/homeListing.test.ts` (101),
`web/test/pages/home.container.test.ts` (144), `web/tests/e2e/home/home-page.ts` (22). Modified:
`IndicatorCard.astro` (+35/−4), `es.ts` (+28/−1), `pages/index.astro` (+26/−12), `IndicatorPage.astro`
(+24), `home.spec.ts` (+145/−11), and three others.

- [x] 21.1 The gap, in the state the site was actually in: `/` was a placeholder whose visible text ended
  "— deploy smoke target for milestone 0.1.", an English build note on the first page a Spanish reader
  sees, and the six indicator pages were reachable only by typing their URLs. An indicator page offered no
  way home.
- [x] 21.2 GREEN — the listing derives from `resolveIndicatorRouteSlugs`, the same all-or-nothing guard
  `/indicador/{slug}` uses, reached through `lib/indicator/homeListing.ts`. Six tests in
  `homeListing.test.ts`, counted here with `grep -c "  it("` → **6**, including "refuses to list anything
  when a frozen slug is absent from the artifact, naming the slug" and "refuses to list a series the
  artifact carries with no observations, rather than showing an empty card".
- [x] 21.3 The reasoning for all-or-nothing, which is the substance of this slice and lives in
  `homeListing.ts`'s own header: a homepage listing five of six is strictly worse than a missing page. A
  missing route 404s loudly for anyone holding the permalink; a missing ROW is invisible — the site looks
  complete, and the vanished indicator is precisely the one nothing else on the site mentions, because `/`
  is the only place a reader learns it exists.
- [x] 21.4 **Mutation-verified, and this is the strongest evidence in this slice.** The commit body records
  that replacing the guard with the exact `.filter()` shape verify-report CRITICAL-27 removed "kills one
  test and leaves thirteen green". Counted here at `ac69a29`: `homeListing.test.ts` has 6 `it(` and
  `home.container.test.ts` has 8 — **14 tests in the mutated scope**, so "one dies, thirteen stay green"
  is arithmetically exact. Recorded as a mutation check, not as observed RED output.
- [x] 21.5 GREEN — two minimal changes to `IndicatorCard.astro`, both argued. An optional `headingLevel`
  defaulting to the previous `<span>`, so the related-cards strip stays byte-identical and the level comes
  from the caller, because only the page knows what heading precedes the cards — which is exactly how the
  h1→h4 skip happened. And a missing space between value and unit: Astro strips whitespace around a lone
  expression, so every card rendered `22779personas`.
- [x] 21.6 A defect the existing assertions could not see, recorded because the lesson generalises. The
  `22779personas` fault was **live on all six indicator pages** and every existing assertion passed,
  because each looked for the number and the unit separately. It was found by reading the built page's
  TEXT rather than its markup.
- [x] 21.7 Scope held: no search, no filtering, no catalogue beyond the six, no category navigation — those
  are milestones 1.3–1.7 and stay there (`proposal.md:70-72`). Produces three new reader-facing Spanish
  strings in `es.ts`: a rewritten tagline claiming only what the product keeps, an "Indicadores" heading
  and `backToHomeLabel: "Volver al inicio"` (`es.ts:389`).
- [x] 21.8 **This slice makes verify-report pass-6 SUGGESTION-50 obsolete**, and the design.md Open Question
  written against it is corrected in this pass rather than left standing. That entry recorded, correctly at
  the time, that `index.astro` carried a false "replaced starting slice 9" comment, was 1,002 bytes, and
  linked none of the six permalinks. Re-measured here at `5af95c5`: the file is **2,713 bytes** (`wc -c`),
  the false comment is gone, and verify-report pass 7 §B.4 measured the built `dist/index.html` as linking
  exactly the six frozen slugs. See slice 21's spec decision below (WARNING-47).

---

## Slice 22 — a figure a thousand times too small, and numbers a Spanish reader can read

Commit `52b7abe`. 676 added, 39 removed across 20 files. New: `web/src/lib/format/number.ts` (115),
`web/test/format/number.test.ts` (118), `web/test/indicator/unit-agreement.test.ts` (148),
`web/test/design-system/tabular-figures.test.ts` (46). Modified: `lib/indicator/routes.ts` (+100/−5) and
fifteen others including the golden chart fixture.

- [x] 22.1 The defect, and how it surfaced. The `ocupados-epa` card and page header read "22779 personas".
  The figure means 22,779 THOUSAND people — roughly 22.8 million. Every source of truth agreed except the
  one the page used: `config/series/ocupados-epa.yaml` says "miles de personas", the export artifact says
  "miles de personas", and INE's own API returns `T3_Unidad` "Personas" with `T3_Escala` "Miles". Only
  `web/src/content/indicators/ocupados-epa.ts` said "personas". **Live since slice 9a on the indicator
  page**, and slice 21 had just put it on the front page.
- [x] 22.2 The value is NOT rescaled — the pipeline is right and INE's scale is real. What changed is the
  label, plus a guard so the two cannot contradict each other again.
- [x] 22.3 GREEN — the duplicated unit is KEPT rather than deleted, and the reason is the substance.
  Reading the artifact instead would have deleted the index base from two public pages: the artifact says
  "índice" for both IPC series while the page says "índice (base 2021=100)", because the artifact's own
  `base` field is a disclosed permanent-null gap. So the rule is not equality but **elaboration**:
  editorial copy may append after a space or a parenthesis, never replace, shorten or rescale.
  `unit-agreement.test.ts` (148 lines) is that guard.
- [x] 22.4 The guard lives inside `resolveIndicatorRouteSlugs` — the one function both `/` and
  `/indicador/{slug}` already reach — rather than in a second guard. CRITICAL-27's lesson, applied
  deliberately: a second guard is a second thing to forget to call.
- [x] 22.5 `decimals` is deliberately NOT guarded, and the asymmetry is argued: rounding a value for
  display never changes what it means, relabelling its unit does. It has drifted in three of six slugs
  (artifact vs content: 1/0, 3/2, 3/2) and is **flagged rather than changed**, since moving it moves
  published figures. Carried into design.md Open Questions by this pass.
- [x] 22.6 GREEN — one `Intl.NumberFormat` `es-ES` helper (`lib/format/number.ts`) applied to every
  reader-facing figure: the cards, the page header, both variation figures, both accessible data tables,
  the chart's point announcements, its textual description, and the y-axis tick labels. The CSV and JSON
  are untouched structurally — they are Go-generated machine projections carrying digests, and the web
  tree never writes them. SVG geometry keeps `toFixed`, because a grouping separator in a path `d`
  attribute is a syntax error.
- [x] 22.7 Why the axis labels mattered more than they look: before this, one screen showed the same number
  two ways — the accessible data table read "49.687.120" while the axis beside it read "49687120".
- [x] 22.8 One real behaviour change, found rather than assumed and measured rather than estimated. `Intl`
  and `toFixed` disagree on exact decimal halves because `toFixed` rounds the underlying binary double:
  `(70.865).toFixed(2)` is `"70.86"`, `Intl` gives `70,87`, which is what a person rounding the printed
  number gets. Measured across every observation at each route's rendered precision: **zero** figures
  change in the real artifact and **37 of 1031** in the synthetic fixture — all in its formula-generated
  straight line, which is why they sit on exact halves and real INE readings do not. The golden chart
  fixture moves four tick labels for this reason and the island/build-time parity device agrees again
  afterwards.

---

## Slice 23 — a favicon, readable dates, a real link, and a chart legible on a phone

Commit `6556f0e`. 1,556 added, 62 removed across 25 files. New: `web/public/favicon.svg` (77),
`web/src/lib/format/date.ts` (137), `web/test/format/date.test.ts` (131), `web/test/pages/favicon.test.ts`
(134). Modified: `lib/chart/geometry.ts` (+159/−4), `ChartIsland.svelte` (+98/−22), `lib/chart/svg.ts`
(+58/−10), and eighteen others.

- [x] 23.1 Every page load 404'd on `/favicon.ico` — the only console error on the site. There is now an SVG
  icon with a dark-scheme variant, wired into all three route entry points that own an `<html>`.
- [x] 23.2 A real bug found while writing it, recorded because the symptom was indistinguishable from
  having no favicon at all: **XML forbids `--` inside a comment**, and the first version's rationale named
  `--color-accent`, so Chromium silently refused the file. The rationale moved into a CSS comment and a
  test pins it. This is genuine observed-failure evidence, in a browser, before the fix.
- [x] 23.3 GREEN — `favicon.test.ts` finds route entry points by **scanning for `<html>`** rather than
  reading a hand-maintained list. That is the WARNING-18 failure mode applied prospectively, and slice 25's
  footer guard reuses the same discovery for the same reason.
- [x] 23.4 GREEN — the methodology sheet showed `Última extracción: 2026-07-29T12:00:00Z`, a machine
  instant on a Spanish page. It now reads "29 de julio de 2026 a las 14:00 (hora peninsular)", with the ISO
  value preserved in `<time datetime>` so machines and the existing assertion both still see it.
- [x] 23.5 Each part of that format argued rather than defaulted. `extractedAt` is provenance, not a
  freshness badge: the question it answers on the one day it matters is "INE published this morning, is
  this figure from before or after", which a date alone cannot answer. Seconds would advertise precision
  the schedule does not have. Printing `12:00` from `12:00:00Z` would be wrong by one or two hours
  invisibly, so the zone is converted to Europe/Madrid and named in prose, because CEST/GMT+2 change
  wording twice a year.
- [x] 23.6 GREEN — "Próxima publicación" pasted its URL into the visible copy, in parentheses. It is now a
  link with the same text, matching the pattern the neighbouring fields already use. The disclosure is
  unchanged: no per-series next-publication date exists anywhere in this project, and the copy still says
  to consult the source's calendar rather than inventing one.
- [x] 23.7 The chart was illegible on a phone, and the measurement is what decided the fix. At 375px its
  tick labels measured **3.25 CSS px**, because `font-size="10"` inside a 960-unit viewBox scales down with
  everything else. A bigger font could not fix it and the browser said why: `poblacion-residente`'s
  `49.477.903` measures **55.3 units** against the **48** the left margin leaves, so it was clipped at
  every viewport. Type and margins are one decision, and 2.7× separates a 312px phone column from an 848px
  desktop one, so no single viewBox serves both.
- [x] 23.8 GREEN — two geometries: 560×420 with a 20-unit tick font, three x-ticks and margins derived per
  series from the real label widths, toggled at the same breakpoint `MethodologySheet` already uses.
  Measured in a real browser at 375px: tick labels 3.25 → 11.15 CSS px, chart 312×117 → 312×234, clipped
  labels one → zero. Costs 3–4 KB gzip per page against a 300 KB budget the heaviest page uses 19% of.
  Verify-report pass 7 §A re-measured the budget against the **real artifact** with both geometries
  server-rendered: worst page 76.6 KB, a 3.9× margin.
- [x] 23.9 The golden chart fixture did not move: every new `renderChartSVG` input defaults to the prior
  value and the narrow variant suffixes its own test ids.
- [x] 23.10 Recorded and deliberately NOT fixed here, because this was a design pass and not bug-hunting:
  the WIDE variant clips `poblacion-residente`'s y-labels at every viewport and its last x-tick overruns by
  about 5.5 units. Only the narrow variant's derived margins solved it; the wide geometry's margins were
  frozen by the golden fixture. **Closed three commits later by slice 26 (`b5d7858`)** — carried here so
  the sequence is visible rather than looking like it was never noticed.

---

## Slice 24 — equal-height cards and a status pill that stopped looking like a button

Commit `f6949f9`. 401 added, 5 removed across 6 files, **all six of them either component or e2e** —
`FreshnessSemaphore.astro` (+26/−1), `IndicatorCard.astro` (+30/−2), and four Playwright files
(`home-page.ts` +76, `home.spec.ts` +137, `indicator-page.ts` +17, `indicator-pages.spec.ts` +115). No unit
test file changed, and the reason is task 24.6.

- [x] 24.1 Defect one, measured. The homepage grid stretches each `<li>` to its row height but the card
  inside carried no `h-full`, so it did not fill its cell. At 1280px row one rendered **156/156/188** —
  ragged, because "índice (base 2021=100)" wraps to two lines and its neighbours do not. Now **188/188/188**.
- [x] 24.2 Equalising heights alone would have left the badges at **319/319/351**, so the card also pins the
  badge to its bottom edge. The badge is the one element a reader compares ACROSS cards rather than within
  one: six freshness stamps on a common baseline read as a single horizontal scan, while a zigzag has to be
  read card by card. It also moves the slack whitespace above the badge rather than below it, where it made
  a taller card look like it had ended early.
- [x] 24.3 Defect two, measured. The freshness semaphore rendered as a full-width bordered box that looked
  exactly like a button — on the homepage cards, the related-indicators strip and every indicator page
  header. It is a non-interactive `<span>` (verified: no role, no tabindex, no handler, `cursor: auto`) but
  `inline-flex` inside a flex column is still stretched by the parent's default `align-items: stretch`.
  Measured **100.0%** of its container's width everywhere; the indicator page header badge was **848 px**
  wide. Now **31.1%** on a card and **8.7%** in the header.
- [x] 24.4 GREEN — the fix is `self-start` in the COMPONENT, not at the call sites, and the placement is
  argued. `inline-flex` already declares "sized to my own text", and cross-axis sizing is the one property
  a parent decides for its child, so the component asserting its own declared shape is the honest place for
  it; `align-self` is inert outside a flex/grid context, so it costs nothing in normal flow. Not `w-fit`,
  which would fix the width and leave the same defect on the height axis in a flex row.
- [x] 24.5 The `mt-auto` went the other way — into the card — because "the badge sits at my bottom edge" is
  a decision only the card is entitled to make: the page header renders the same component right after the
  variation figures and must not be pushed.
- [x] 24.6 GREEN — tested by measuring rendered `boundingBox()` geometry, never by grepping for a utility
  class. A test asserting `h-full` appears in the markup proves nothing about what a reader sees and passes
  forever once someone changes the mechanism. The badge-width gate is expressed as a **share of its
  container** — 70%, against a worst case of 31.1% after and exactly 100.0% before — so one threshold covers
  both viewports. A new test also asserts the badge does not match the 44px sweep's own selector, applied
  to the badge itself rather than as a descendant query, because the card is an `<a>` and a descendant
  query would match all six and prove nothing.
- [x] 24.7 **A mutation check with a NEGATIVE result, recorded rather than overclaimed.** Removing
  `self-start` alone leaves the homepage badge-width tests green, because inside the new `mt-auto` wrapper
  `inline-flex` is already shrink-to-fit. The card is protected by two independent mechanisms and
  `self-start` is proven load-bearing by the page header alone. Recorded because a mutation check that does
  not go red is evidence about the test suite, not an embarrassment to hide.

---

## Slice 25 — a site footer that defers on licensing instead of asserting one

Commit `9af3f86`. **669 added, 0 removed across 7 files** — purely additive. New:
`web/src/templates/SiteFooter.astro` (136), `web/test/pages/site-footer.test.ts` (304),
`web/tests/e2e/footer/site-footer.spec.ts` (153). Modified: `es.ts` (+51), `pages/index.astro` (+5),
`pages/indicador/[slug].astro` (+5), `workbench/pages/index.astro` (+15).

- [x] 25.1 The gap, stated as the measurement that found it. Per-series provenance lived in each methodology
  sheet, but a reader arriving at `/` had no route to the repository, to the code licence, or to the source
  terms — and the raw-file hash listing the container serves at `/transparencia/raw-files.sha256` was
  reachable by nobody: `grep -rn transparencia web/src/` returned nothing. For a project whose whole premise
  is that every published figure traces back to the bytes it came from, an unreachable provenance artifact
  is a real gap.
- [x] 25.2 The obvious footer would have violated a spec, and the spec is a BASELINE capability rather than
  one of this change's twelve deltas. `openspec/specs/source-attribution-licensing/spec.md:26` — "The
  repository MUST NOT assert a single licence over all derived data … `LICENSE-DATA` MUST defer to
  `sources/{source}.yaml` rather than override it." Eurostat's permission is acknowledgement-only, excludes
  third-party material and restricts some commercial redissemination; INE and Seguridad Social carry their
  own terms. There is no honest way to compress three sets of conditions into one line.
- [x] 25.3 GREEN — the sentence asserts an **absence** rather than a licence. `es.footer.dataTermsNote`:
  "Los datos publicados aquí no están cubiertos por una licencia única: cada fuente fija sus propias
  condiciones de reutilización." It summarises none of them.
- [x] 25.4 The link goes to `config/sources/`, not to `LICENSE-DATA`, and the reason is the spec's own
  ordering: the spec makes the per-source YAML authoritative and `LICENSE-DATA` the deferring document, so
  routing a reader through a deferring summary reinstates the hop the footer exists to remove. MIT appears
  exactly once, beside "código", where a single claim is true (`es.footer.codeLicenceLabel`).
- [x] 25.5 GREEN — the guard is written against what must NOT appear. `site-footer.test.ts` (304 lines)
  asserts the rendered text and the markup match none of `/cc\s*by/i`, `/creative\s*commons/i`,
  `/todos los datos/i` or `/licencia de los datos/i`.
- [x] 25.6 Deliberately left out, each with its reason. The CC BY 4.0 offer for this project's own editorial
  text — real, but CONDITIONAL on each source permitting redistribution, and printing a conditional claim
  in a footer beside the data is exactly how it gets read as covering the data; it stays in `LICENSE-DATA`
  where its condition travels with it. The manifest, because the action bar already offers each series' CSV
  and JSON one level down and a footer is not a download hub. Every per-series fact the methodology sheet
  owns — a test asserts the footer contains none of its source, origin or extraction labels.
- [x] 25.7 The hash listing's label says what a reader will actually find — "Hashes SHA-256 de los ficheros
  originales (texto plano)" — rather than "transparencia", which would promise a page this project does not
  have.
- [x] 25.8 The workbench gets the footer too, and that is a decision rather than a copy-paste: the guard
  finds route entry points by scanning for `<html>`, exactly as slice 23's favicon test does, so exempting
  one page would mean maintaining a skip list — the WARNING-18 failure mode. And it is the one page a
  reviewer opens with both themes side by side, so it is where the footer's contrast and 44px targets get
  looked at rather than only measured.
- [x] 25.9 Disclosed and compensated rather than hidden: the e2e suite asserts the footer EMITS the
  hash-listing href but cannot assert it resolves, because that file is written at runtime by the Go binary
  into the container's volume and never exists in `web/dist`. Compensated by pinning the href against the
  writer's own source — `test/pages/site-footer.test.ts:196` regex-reads `publicHashPath` out of
  `app/cmd/concontexto/ingest_cmd.go` and fails loudly if the regex stops matching ("this guard has gone
  blind") — and by verifying 200 against the running container. Verify-report pass 7 §B.4 independently
  re-read that guard and confirmed it. The same limitation already applied to the action bar's
  `/data-derived/**` links, which no e2e test has ever asserted resolve either.
- [x] 25.10 Placement argued in the component's own header: rendered as a sibling of `<main>`, never inside
  it, because a `<footer>` nested in `<main>` has NO landmark role. Carries no heading at all — it follows
  an `h3` on `/`, an `h2` on an indicator page and an `h2` on the workbench, so any fixed level would skip
  on at least one. Lives under `src/templates/`, not `src/components/`, because several slice 5/6 guards
  scan `src/components/` wholesale on the assumption that every `.astro` file there is one of the design
  system's eight catalog components. Ships zero runtime JavaScript.
- [x] 25.11 Produces five new reader-facing Spanish strings in a new `es.footer` block.
- [x] 25.12 **The spec decision this slice forces — verify-report pass-7 WARNING-47 — is taken in this
  pass**, and it is not "leave it". See the section "The WARNING-47 spec decision" at the end of these
  slices, and the delta spec it produced.

---

## Slice 26 — deriving the wide chart's margins so it stops clipping published figures

Commit `b5d7858`. 427 added, 74 removed across 7 files: `lib/chart/geometry.ts` (+135/−26),
`web/test/chart/geometry.test.ts` (+113/−11), `web/test/chart/svg.test.ts` (+110/−29),
`ChartIsland.svelte` (+12/−2), `lib/chart/svg.ts` (+9/−3), the golden fixture, and
`indicator-pages.spec.ts` (+47/−2).

- [x] 26.1 The defect, measured in a real browser on `/indicador/poblacion-residente` by reading `getBBox()`
  off the live DOM: **all four y-axis labels started LEFT of the viewBox origin** (−5.5, −6.5, −4.6, −3.6)
  and were therefore cut off, and the last x tick ended at **965.5 against a viewBox 960 wide**. **Five of
  the six pages** overran the right edge. A portal whose premise is publishing figures accurately was
  rendering them sliced. This is slice 23's task 23.10 being closed.
- [x] 26.2 The cause was never the grouping separators, which only made it visible. The margins were
  constants — `marginLeft` 56, `marginRight` 16 — and the labels are right-anchored at `marginLeft - 8`. A
  grouped eight-digit figure measures ~54 units and had 48 to live in. The right edge was worse by
  construction: the last tick is centred on `plotArea.x1`, so half its width ALWAYS overran, on every
  series, whatever the label.
- [x] 26.3 GREEN — the narrow variant had already solved exactly this by deriving its margins from the
  labels the series really prints. That derivation is now **one function both variants use**. Two copies of
  this rule would drift, and the narrow one was already correct.
- [x] 26.4 The asymmetry between the two gutters is argued, not accidental. The left gutter keeps 56 as a
  FLOOR; the right one has none. At 960 units the ~18 units a short-label series would win back is 1.9% of
  the drawing, invisible, and the y axis is where the eye enters the chart — nothing is bought by shrinking
  it, and five of six pages keep their coordinates. The narrow box is the opposite trade at 560 units,
  where a phone can see the loss, so it keeps no floor. No right-hand floor either: the derived value
  exceeds the old constant for any period label of four glyphs or more, so a floor there could never bind
  and would be dead code.
- [x] 26.5 GREEN — the mirror rule went in with it: the FIRST tick is centred on `x0`, so `marginLeft` is
  never allowed below `marginRight`. It never binds in practice; it is there so the contract is complete
  rather than lucky.
- [x] 26.6 The estimator is calibrated conservative and says so: 0.64 advance ratio against a measured
  ~0.545 for grouped numerals, so it errs towards a gutter a few units too wide, never a clipped label. And
  it is deliberately **not what the tests trust** — the new gate measures `getBBox()` in Chromium across all
  six slugs, because an estimate is precisely the thing that was wrong here.
- [x] 26.7 The golden fixture was **regenerated, not hand-edited**. Its tick labels are identical, every y
  coordinate is identical, and the file is the same size — only x moved, by the 11 units the plot area
  narrowed when the right margin went 16 → 27. The island/build-time parity test passes against it, which
  is the whole point of that device.
- [x] 26.8 Left in place deliberately: `DEFAULT_DIMENSIONS` still exists and still carries the old
  horizontal margins. It is now only "a box" for pure-scale and hit-test tests that do not care about
  labels, and **a test pins that production's default is the DERIVED box, not this constant**. The doc
  comment is what stops someone reaching for it in new drawing code.

---

## Slice 27 — showing periods the way Spanish statistics write them

Commit `0c40097`. 970 added, 46 removed across 21 files. New: `web/src/lib/format/period.ts` (191),
`web/test/format/period.test.ts` (139), `web/test/format/machine-surfaces.test.ts` (206). Modified:
`ChartIsland.svelte` (+43/−6), `IndicatorPage.astro` (+11/−2), `indicator-page.container.test.ts`
(+107/−1), `indicator-pages.spec.ts` (+79/−2), the golden fixture, and twelve others.

- [x] 27.1 The defect: the site rendered `2026-Q2` to readers, **twenty-two times on one indicator page**.
  That is the database's canonical storage format leaking to the screen, and `Q` is an English abbreviation
  for quarter. Spain does not use it in official statistics, and this project's own source proves it —
  INE's API returns `T3_Periodo` with values `T1`–`T4`, and its press releases write "el segundo trimestre
  de 2020". Every figure comes from a source that says T and the reader was shown Q.
- [x] 27.2 Confirmed against the requirement text rather than assumed: **not** covered by the
  verbatim-identifiers requirement, which enumerates four things — indicator slugs, configuration
  filenames, source names and origin series identifiers. A period label is none of them. It is a date, the
  same category as the extraction instant slice 23 reformatted for the same reason.
- [x] 27.3 GREEN — two registers, differing in exactly one place, **named in code rather than implied by
  call sites**. Compact where the label sits in a column or on an axis and its width is load-bearing:
  `T2 2026`, `jun 2026`, `2026`. Prose where it sits in a sentence: `T2 2026`, `junio de 2026`, `2026`.
- [x] 27.4 Why quarters and years have one form and months two, argued rather than defaulted. `T2 2026` is
  already the decision and inventing "el segundo trimestre de 2026" for prose would leave the site saying
  two things about one period. Months differ because `septiembre de 2026` is 18 glyphs against `2026-09`'s
  7 — it wraps the Periodo column at 375px and widens every monthly chart's gutter — while running prose
  has no column to save. The screen-reader announcement takes prose deliberately: speech has no column
  either.
- [x] 27.5 Only ONE authored Spanish string, in `es.ts`: the quarter form. Every other Spanish word here is
  CLDR's `es-ES` grammar. That is also the whole English-version seam — `es.periods.quarter` and the single
  `LOCALE` constant `format/number.ts` already exported. The letter T appears nowhere else in the code.
- [x] 27.6 GREEN — **the machine boundaries were found and each verified rather than assumed**, which is
  the substance of this slice. The published CSV and JSON still carry canonical periods and every
  recomputed sha256 still matches the manifest, over HTTP from the running stack as well as in the fixture;
  permalink `from`/`to` are byte-identical and a display-format bound is rejected; `<time datetime>` still
  carries ISO; and every sort, comparison and join — `periodOrdinalIndex`, `previousPeriod`, `sliceRange`,
  the YoY join, the `{#each}` keys — still runs on the canonical form. `machine-surfaces.test.ts` (206
  lines) is that guard.
- [x] 27.7 **Mutation-verified.** The commit body records three mutations confirming each boundary catches a
  formatter applied where it does not belong. Recorded as a mutation check by the writer, not as observed
  RED output. Verify-report pass 7 §A independently re-measured the machine surfaces at `5af95c5`:
  `data-derived/csv/ocupados-epa.csv` ends `2026-Q2,22779,D,Definitivo,1` and the JSON carries
  `['2025-Q4','2026-Q1','2026-Q2']`, while the reader surface says `T2 2026`.
- [x] 27.8 GREEN — the chart's derived margins now measure the DRAWN label rather than the stored one,
  which matters because monthly labels grew from 7 glyphs to 9. Quarterly geometry is byte-identical —
  `T2 2026` is the same seven glyphs as `2026-Q2` — so the golden fixture moved in four x-tick text nodes
  and **nothing else: no coordinate, no viewBox, no margin**.

---

## Slice 28 — reconciling the editorial registries in the deployed stack, where nothing ever did

Commit `a7c3739`. 546 added, 13 removed across 5 files: `app/cmd/concontexto/deploy_reconcile_composition_test.go`
(new, 255), `docker-compose.yml` (+80), `docs/deploy.md` (+99/−1), `app/cmd/concontexto/ingest_cmd.go`
(+43/−12), `app/internal/ingestion/e2e_export_test.go` (+69). **A production defect fix, and the seventh
instance in this change of something built, tested, and never connected to the pipeline that would make it
do anything.**

- [x] 28.1 The defect, measured on the running stack: `event = 0`, `series_break = 0`. The only production
  call site of `ReconcileEditorialConfig` was the manual `ingest --reconcile` flag; the scheduler's cycle
  never touched it. So in **any** deployed stack — which is every deployed stack, since they all run
  `serve` — `rupturas.yaml`, `eventos.yaml` and `gobiernos.yaml` never reached the database at all.
- [x] 28.2 What that cost, invisibly, since slice 7, recorded because none of it was visible as a failure:
  **no break band ever rendered** (the component, its non-dismissibility guarantee under PRD principle P4,
  and the chart's shaded band all drew nothing, because `ResolveActiveBreaksForSeries` had nothing to
  resolve); **no annotation ever rendered**, for the same reason; and **rule 3's break exemption could
  never fire** — `breakAt` always saw an empty slice, so a jump at a genuinely recorded methodological
  break blocked exactly as if no break existed.
- [x] 28.3 GREEN — a one-shot compose service, gated on `migrate` and gating `app`. Argued from ADR-1:
  config is embedded in the binary, so it cannot change without a new image and a new image cannot arrive
  without a container recreation — "reconcile when the config changes" and "run once on `up`" are therefore
  the same instant. It mirrors the `migrate` service the stack already has for exactly this shape of work,
  and a failed reconcile becomes a non-zero exit that gates the app rather than a line in a log nobody
  reads.
- [x] 28.4 Two alternatives rejected with their reasons. The scheduler's cycle: idempotent, but it redoes
  byte-identical work every fifteen minutes forever and couples editorial reconciliation to per-source
  scheduling, so a source in backoff would delay the registries for reasons unrelated to them. `serve` at
  boot: the spec argument does NOT transfer — a reconcile is not a schema migration — but the operational
  one does, because `runServe` is deliberately resilient to every missing prerequisite, so a failed
  reconcile there would have to be swallowed to preserve that resilience, and a silently swallowed
  reconcile is this defect again.
- [x] 28.5 Ordering is **declared rather than timed**, and it matters twice: rule 3 reads `series_break`
  DURING ingestion, and `Export` reads it back INTO the artifact. A reconcile landing after the app's first
  cycle would publish empty arrays for a day.
- [x] 28.6 GREEN — **the guard is not a grep**, which is the substance of this slice's test.
  `deploy_reconcile_composition_test.go` (255 lines) parses the committed compose file for the service and
  its gates, then takes that file's own `command:` array — never a literal — and runs it through the same
  dispatch table `main()` uses, against a real migrated Postgres, asserting the rows land and that the
  output names the pending entries.
- [x] 28.7 **Mutation-verified by the verifier, not by the writer.** Verify-report pass 7 §B.4 broke it two
  ways: deleting `reconcile: condition: service_completed_successfully` from `app.depends_on` turned
  `TestDockerComposeReconcile_RunsAfterMigrationsAndGatesTheApp` RED naming the exact condition and why it
  matters; changing `command: ["ingest","--reconcile"]` to `command: ["ingest"]` turned
  `TestDockerComposeReconcile_TheCommittedCommandProjectsEditorialRows` RED with the binary's own usage on
  stderr. Recorded as the auditor's measurement, cited not claimed.
- [x] 28.8 Reporting closed while here: the reconcile line now prints acknowledgement counts, and every
  pending list as a count AND its identifiers. An operator sees `acknowledgements pending=1 (no human
  signature), not projected: ocupados-epa-2020-q2-covid` rather than nothing.
- [x] 28.9 Proven on a clean slate — isolated project, `docker compose up -d` and nothing else: breaks
  inserted=5, events inserted=10, and the break `epa-metodologia-2021` now reaches the exported document
  and renders on the page as "Rupturas de la serie — T1 2021", which the page contained **zero** times
  before. Verify-report pass 7 §A re-measured the artifact independently: every series document carries
  `events: 10`, and `tasa-de-paro-epa`, `ocupados-epa`, `ipc-general` and `ipc-subyacente` each carry
  `breaks: 1`.
- [x] 28.10 Disclosed at the time and now closed by later events: the drawn SVG band was unproven end to
  end, because a full build from live full-history data failed while `ocupados-epa` was held by the publish
  gate awaiting a human signature. That signature landed two commits later (slice 30, `4e11378`) and pass 7
  §A measured a production build at exit 0 with all seven pages.

---

## Slice 29 — filtering a series by the government in office

Commit `ff2ea4f`. 1,122 added, 21 removed across 12 files. New: `web/src/lib/transform/governmentTerms.ts`
(224), `web/test/transform/governmentTerms.test.ts` (254). Modified: `ChartIsland.svelte` (+206/−6),
`indicator-pages.spec.ts` (+141/−6), `permalink.test.ts` (+99), `es.ts` (+43),
`indicator-pages-no-js.spec.ts` (+35), `permalink.ts` (+33/−3), and four others.

- [x] 29.1 One of the two requested filters was built and the other refused, with the reason recorded. A
  year IS a `[from, to]` pair, which the custom picker already expresses, so a year control would buy two
  field fills at the cost of a third encoding of the same concept in the permalink — and one year of a
  quarterly macro series is four points, a chart saying less than the table beside it. The government
  filter is different in kind: **its bounds are not on the page**. A reader cannot type "Rajoy's term" into
  a date picker without already knowing the dates.
- [x] 29.2 GREEN — the registry records a start for each government and an end for none, so the window has
  to be derived: term N runs until term N+1 takes office, half-open, with the last one open-ended. Correct
  for Spanish prime-ministerial succession, which is continuous — **but it is an inference, and this
  project does not let inferences pass unmarked**.
- [x] 29.3 GREEN — the inference is marked in the type itself. `endKind` distinguishes "configured" from
  "succession" from "open"; a configured end always wins over the derivation; `succeededById` names the
  entry each boundary was read off; and the reader is told: "El registro editorial no recoge la fecha de
  fin de este gobierno: el final del intervalo se deduce de la toma de posesión del gobierno siguiente."
- [x] 29.4 What would break it, **recorded rather than discovered later**: a real gap or caretaker period is
  absorbed into the preceding term silently; a government missing from the middle of the registry is
  absorbed by its predecessor invisibly; two governments inside one period on a coarse axis collapse the
  earlier one's window. The handover period goes to the incoming government, so no observation sits in two
  terms — a choice, not a fact.
- [x] 29.5 Adolfo Suárez is configured with an unconfirmed date and is never projected, so the earliest
  selectable term starts in 1981 and observations before it belong to no government. The earliest term is
  deliberately NOT stretched back to the series start: that would assert Calvo-Sotelo governed in 1971. The
  data stays fully reachable through the full range and the custom picker; it is simply not selectable by
  government.
- [x] 29.6 GREEN — a term that selects nothing is absent; so is a term that selects everything, which is
  the full range under a president's name. **This extends the spec's "absent, not disabled" rule by ANALOGY
  rather than applying it literally**, and the extension is stated in the module and tested: the clause
  names the five fixed presets, and a condition that is neither of those carries across by its reason, not
  its wording.
- [x] 29.7 GREEN — the permalink encodes the **editorial id, never the derived window**. Freezing the
  window into a link would freeze today's inference, so an old link would stop agreeing with the registry
  the day a real end date lands. An unknown or non-overlapping id degrades silently to the full range.
  `permalink.test.ts` +99.
- [x] 29.8 GREEN — absent without JavaScript rather than present and dead, gated at SSR and proven in a real
  `javaScriptEnabled: false` context (`indicator-pages-no-js.spec.ts` +35). The government annotation chips
  stay server-rendered; only the interactive filter is withheld.
- [x] 29.9 Two disclosures, both about what is NOT proven, and both still true at `5af95c5`. The control
  does not appear on the currently deployed stack, correctly: its artifact carried three quarters per
  series, entirely inside Sánchez's open term, so every government filter would be the full range renamed.
  And the "selects everything ⇒ absent" half is **proven only at unit level**, because no government term
  covers any of the six real series entirely. Carried into design.md Open Questions by this pass.

---

## Slice 30 — signing the COVID acknowledgement, and making an agent unable to sign the next one

Commit `4e11378`. 225 added, 89 removed across 5 files: `app/internal/adapters/config/acknowledgement.go`
(+58/−2), `acknowledgement_validate_test.go` (+116/−31), `config/reconocimientos.yaml` (+10/−34),
`app/internal/ingestion/acknowledgement_e2e_test.go` (+20/−18),
`deploy_reconcile_composition_test.go` (+21/−4). **This is the fix for pass 6's own blocker, and it is the
direct counterpart of this file's "an agent signed a human's name" process finding.**

- [x] 30.1 The repository owner reviewed the record and instructed that it be signed in his name.
  `acknowledged_by: "jorgealonsodev"`, `acknowledged_on: 2026-08-04`; `signature_status`, `drafted_by` and
  the `todo` are gone, and `note_md` states a reviewed conclusion instead of a reading. **The research it
  rests on survives verbatim**: the measured distribution, the pinned `18607.2`, and the INE press-release
  citation.
- [x] 30.2 `jorgealonsodev` rather than `concontexto`, which was the alternative offered, and the reason is
  the field's whole purpose: a project name would sign an attestation as an organisation, and the value of
  this field is that **someone can be asked about it in two years**. A handle resolves to a person; a
  project name resolves to itself.
- [x] 30.3 GREEN — **the guard INVERTS rather than being deleted**, and that is the decision worth
  recording. `TestRealAcknowledgementRegistry_ShipsExactlyOneUNSIGNEDDraft` had asserted that the shipped
  record MUST be unsigned. Its purpose is intact — what changed is which state is correct — so it now
  asserts the record is signed BY A REAL HUMAN, failing on an empty signer, on thirteen agent tokens, on
  twelve placeholder tokens, on a leftover draft field, on the note still declaring itself pending, and on
  the research drifting. Its comment records what it used to assert and the incident that produced it.
- [x] 30.4 GREEN — **and the hole the original incident went through is closed.** The validator's nineteen
  placeholder tokens contained not one agent-shaped name, so the exact string the fabricated draft carried
  — "Claude (agente), bajo autoridad delegada — no es una firma" — would have passed every one of them.
  Whole-string tokens were not enough either: an agent that decided to sign would write a sentence, not a
  token. So agent words are now matched **at word boundaries anywhere in the string**. `ai` and `ia` stay
  whole-string-only so "Ai Weiwei" still validates, and word-boundary rather than substring matching keeps
  "Alberto Botella" valid.
- [x] 30.5 The principle, stated in the commit and worth keeping in the record: **a mechanism whose only
  defence against an agent signing is an agent choosing not to is not a defence.**
- [x] 30.6 GREEN — the whole path proven end to end on a **disposable database rather than the running
  stack**: `validate-config` accepts it; the reconcile projects it, 0 rows → 1 with `acknowledgements
  inserted=1` and the pending line gone; a real ingest against live INE publishes 98 observations as
  `publish-overridden` at WARN, with the log naming the record and the signer and `ingestion_run.outcome =
  succeeded-with-acknowledgement`; and the artifact now carries `series/ocupados-epa.json`.
- [x] 30.7 Live INE corroborates the record's own figures: 2020-Q1 = 19681.3, 2020-Q2 = 18607.2 — exactly
  the two the note cites and exactly the pinned value. Had it drifted, the run would have raised
  `acknowledgement-stale` and blocked rather than inheriting an approval given for a different number.
- [x] 30.8 **Attacked by the verifier rather than read, and the attack is the citable evidence.**
  Verify-report pass 7 §D mutated the record three ways: the exact original fabrication string → exit 1;
  `"TODO"` → exit 1; `"Jorge Alonso"`, an ordinary human name → exit 0, so the guard is not simply
  rejecting everything. And replacing the signature with a properly-declared `signature_status: "unsigned"`
  leaves `validate-config: ok` while the inverted test fails with **five distinct assertions**. **Pass 7
  records CRITICAL-37 CLOSED, both halves.**
- [x] 30.9 What did NOT change, and should not: WARNING-39 stays open. `.github/CODEOWNERS:17` still reads
  `/config/** @jorgealonsodev @TODO-second-config-reviewer`, a placeholder GitHub cannot resolve, and
  `gh api .../collaborators` returns exactly one login. The mechanism now exists; the second pair of eyes
  does not, and cannot until a second person does. **Materially more relevant now that a human signature is
  the thing being protected.**

---

## Slice 31 — marking each change of government on the timeline

Commit `154824f`. 1,136 added, 6 removed across 18 files. New: `web/src/lib/chart/governmentMarkers.ts`
(163), `web/test/chart/governmentMarkers.test.ts` (159). Modified: `indicator-pages.spec.ts` (+106),
`island-ssr.test.ts` (+97), `lib/chart/svg.ts` (+93), `reserved-semantics.test.ts` (+71),
`description.test.ts` (+62/−1), `indicator-chart.container.test.ts` (+55), `es.ts` (+38), the golden
fixture, and eight others.

- [x] 31.1 The requested treatment was refused for a stated reason. The owner asked for a **dashed** vertical
  line; it could not be dashed, because a dotted stroke is a RESERVED semantic here — `theme.css`'s own
  header says dotted grey always means provisional data, `svg.ts` draws provisional observations that way,
  and `reserved-semantics.test.ts` enforces it. A dashed government line would have taught a reader two
  contradictory meanings for one visual code.
- [x] 31.2 GREEN — a thin solid rule across the full plot height in `--color-ink`, capped by a small
  downward triangle. A flag planted at the investiture.
- [x] 31.3 The separation from both existing marks is carried by **shape, not palette**. Against the
  provisional dash: solid, and running ACROSS the plot instead of ALONG the data path — one is a statement
  about an observation's status, the other about the calendar. Against the break band: a line has no width
  at all against a band one full period step wide; solid ink against a 0.35-opacity wash; an instant
  against an interval. The flag is a triangle, a third glyph beside the definitive circle and the
  provisional diamond, so the chart stays readable with colour discarded entirely.
- [x] 31.4 `--color-ink-muted` was rejected deliberately: read side by side against the reserved provisional
  grey they are the same grey to the eye, so a rule in the axis colour would have collided with the
  reserved semantic in everything but name.
- [x] 31.5 Colour carries no information in this mark — every marker on every chart is the same ink. That is
  also why no party reading is possible, which matters because the data could not support one anyway:
  `gobiernos.yaml` and `EventConfig` carry no party field at all, by PRD §12.1's design.
- [x] 31.6 GREEN — three parts do the labelling because no one of them is enough: a third legend entry with
  the glyph in miniature; a `<title>` per marker naming the government and its year; and a sentence
  appended to the generated description, which is what a screen reader gets, because the drawing is a
  single `role="img"` that prunes its own descendants so a marker title reaches a pointer and nobody else.
  **One sentence serves both readers** rather than a visible list plus a hidden duplicate that could drift.
- [x] 31.7 No text on the drawing itself, and that is measured rather than aesthetic: six four-digit years
  need about 26 units each at wide tick size while Calvo-Sotelo and González sit **21 units apart**.
  (Slice 33 revisits exactly this and solves it with vertical text.)
- [x] 31.8 GREEN — a marker is drawn only when the investiture's snapped period falls inside the periods
  handed in. Without that rule `nearestPeriodIndex` snaps Aznar's 1996 investiture onto 2002-Q1 and draws
  a change of government **that did not happen there**. The rule lives inside the one function both
  renderers call, so neither call site can forget it.
- [x] 31.9 Range interaction falls out by construction rather than needing more logic: the island passes the
  currently visible periods, so narrowing re-selects the markers. Under the government filter at most one
  survives — the term's own opening boundary — and that edge marker is the point, since it shows where the
  chosen window came from. "At most" and not "exactly": on `poblacion-residente`, semiannual before 2021,
  Rajoy's 2011-Q4 investiture falls in a gap in the cadence and no marker is drawn, which is correct
  because the change did not happen inside the span on screen.
- [x] 31.10 Nothing is drawn before 1981 and nothing is invented. Suárez is unconfirmed and never projected,
  so that decade is honestly unmarked — and the sentence says the changes REGISTERED in the period shown,
  never that these were the only ones, which would turn honest silence into a false claim.
- [x] 31.11 The golden fixture **moved on purpose**: a change of government now falls inside the golden
  span, because a feature absent from the golden is a feature the two renderers can silently disagree
  about.

---

## Slice 32 — projecting a selected event's period onto the plot

Commit `d5cfbed`. 1,610 added, 16 removed across 18 files. New: `web/src/lib/chart/eventSpans.ts` (264),
`web/tests/e2e/indicator/indicator-event-spans.spec.ts` (249), `web/test/chart/eventSpans.test.ts` (246).
Modified: `svg.test.ts` (+142), `island-ssr.test.ts` (+106), `lib/chart/svg.ts` (+100),
`indicator-chart.container.test.ts` (+79), `description.test.ts` (+78/−1), `es.ts` (+51), and eight others.

- [x] 32.1 The treatment had to earn its place on a channel none of the other three uses, because the chart
  already carried three visual languages and a fourth risked making it a hieroglyph. An event has
  **duration**, which none of the other marks do — so: a solid horizontal rail near the top of the plot,
  spanning first to last covered period, capped at each end by a short serif turning down into the plot.
- [x] 32.2 Four marks, four channels, **and not one of them is colour**: dotted grey ALONG the data path
  with diamonds (provisional observation); translucent FILLED column one period wide (methodology break);
  thin solid VERTICAL rule + triangle flag (a government took office); solid HORIZONTAL rail + serifs (the
  registry dates this event here to here).
- [x] 32.3 Each separation measured, not asserted. Rail against band is stroke versus fill. Rail against the
  government rule is **orientation**, the property read before any other — measured in the browser, the rule
  spans over **80%** of plot height and the rail under **15%**. Rail against the provisional dash is
  solid-versus-dashed and across-versus-along. No new glyph shape, so circle, diamond and triangle keep
  their meanings.
- [x] 32.4 A translucent fill was rejected: it would have left hue as the only channel separating it from
  the break band, and would wash out about a quarter of the plot on the real 2008–2013 case.
- [x] 32.5 GREEN — per group rather than per entry, and measured rather than assumed: the exogenous group on
  `tasa-de-paro-epa` is four chips and two rails, non-overlapping, one lane. Per-entry selection would need
  a 44px target per chip — pushing the chart off a 375px screen — and could only work with JavaScript,
  leaving a no-JS reader chips that look selectable and are not.
- [x] 32.6 GREEN — an event with no end date is **not drawn**, and each alternative is refused by name:
  running it to the last observation invents an end, capping it at the start asserts one quarter, and a
  vertical rule would steal the government code. The chip stays and the sentence discloses the rule in the
  same breath — the periods shown are those the registry bounds with a start AND an end date.
- [x] 32.7 The outside-the-window test is deliberately **weaker** than the government marker's: INTERSECTS,
  not contained, because a 2008–2013 crisis really does cover 2010–2013 of a series starting in 2010. A
  clamped end is drawn UNCAPPED so the plot's edge never reads as a boundary, and the sentence says it
  extends beyond the period shown.
- [x] 32.8 GREEN — the rail's `title` is pointer-only (the drawing is one `role="img"` and prunes its
  descendants), so the real route for a screen reader is a polite live region that re-narrates on every
  toggle, **always present rather than created on first change**. It states transient selection, which is
  why it is a live region and not part of `aria-describedby`, matching the government-range and custom-range
  precedent.
- [x] 32.9 Two real cross-layer overlaps exist in the data and both were checked to stay legible: the
  pandemic rail crosses the 2021 EPA break band, and the financial-crisis rail contains Rajoy's 2011
  government rule. Spans that overlap each other lane-pack; none do today.
- [x] 32.10 The golden fixture gained one event span, so the fourth layer sits **inside** the
  anti-divergence device rather than outside it.
- [x] 32.11 Two findings this commit reported rather than fixed. First: the island server-rendered its
  annotation group toggles as buttons with zero chips for closed groups, so without JavaScript that control
  was present and dead — the same defect the custom-range picker and the government select were fixed for
  by being absent. **Closed by slice 33 (`e1db0ea`), whose subject line is "draw nothing until it is asked
  for".**
- [x] 32.12 Second, and **still open at `5af95c5` — re-verified for this record rather than accepted from
  the commit body**: `ngeu-primer-desembolso` carries `date_status: unconfirmed` in `config/eventos.yaml`
  (line 56) and nevertheless reaches the published artifact. Verified here by reading
  `web/data-derived/series/tasa-de-paro-epa.json`, whose `events` array contains it. The mechanism is
  exact: `app/internal/ingestion/reconcile.go:86` reads `if e.DateStart == nil`, **never `e.DateStatus`**,
  and this entry has both a `date_start` and an `unconfirmed` status. Two doc comments in shipped source
  state the opposite — `reconcile.go:10-12` ("A break or event whose DateStatus is 'unconfirmed'
  (Date/DateStart is nil) is NEVER projected") and `events_read.go:47-49` ("An unconfirmed
  (date_status='unconfirmed') entry never reaches this table at all"). Recorded as a disagreement, not
  adjudicated here: see the design.md Open Question this pass opens for it.

---

## Slice 33 — naming every mark on the drawing, and drawing nothing until it is asked for

Commit `e1db0ea`. 1,573 added, 177 removed across 17 files. New: `web/src/lib/chart/annotationLabels.ts`
(309), `web/test/chart/annotationLabels.test.ts` (337),
`web/tests/e2e/indicator/indicator-annotation-labels.spec.ts` (328). Modified: `lib/chart/svg.ts`
(+130/−22), `svg.test.ts` (+114/−13), `ChartIsland.svelte` (+67/−14), `IndicatorChart.astro` (+44/−25),
`island-ssr.test.ts` (+48/−19), and eight others.

- [x] 33.1 The previous version was rejected by the owner on two counts, both fixed here: a mark that only
  identifies itself on hover or in a paragraph below is not readable, and government markers were drawn
  unconditionally on every chart whether or not anyone had asked for them. The second half **closes slice
  32's task 32.11 disclosure**.
- [x] 33.2 Labels had been left off before for a **measured** reason, not an aesthetic one (slice 31, task
  31.7): six four-digit years need about 26 user units each and Calvo-Sotelo and González sit 21 units
  apart. Vertical text dissolves that — a sideways label needs one LINE HEIGHT of horizontal room instead
  of one string length: **11.91 units against the 108 and 75** the horizontal names would have needed, so
  the 21-unit pair now clears by **9.34**.
- [x] 33.3 GREEN — narrow is where it stays hard, at **9.20 units apart**, and two things were both
  necessary. The label font is its own constant rather than the tick size (wide 10, narrow 14 — at the
  narrow tick's 20 the stacked pair overruns the plot and the second name is refused), and a **pairwise
  downward settle** drops only labels whose bands actually overlap, so a long name elsewhere on the axis
  does not eat its neighbour's room. A naive lane-row scheme fails at 16.
- [x] 33.4 Twelve combinations swept in a real browser — six pages, both variants, every group open: no label
  overlaps another, none escapes the viewBox, none measures zero.
- [x] 33.5 The label carries the registry's name **verbatim and nothing else**, and each omission is argued.
  Not the year: the axis under the mark is a calendar and the chip below already reads "1981: Leopoldo
  Calvo-Sotelo", so repeating it would spend the drawing's scarcest resource on the one fact already
  available twice. Not a surname either, tempting as "Zapatero" is at 8 glyphs against 28 — there is no
  surname field, so it could only be derived, and a last-token rule that handles "José Luis Rodríguez
  Zapatero" would turn a "Fernández de la Vega" into "Vega". **That is a rendering layer inventing an
  editorial fact.** The upshot is **zero new Spanish strings**: every label is data already in `config/`.
- [x] 33.6 GREEN — event rails take a horizontal label, centred, drawn only when it fits whole. The crisis
  name is 58 glyphs and 448 units against a 435-unit narrow plot, so there it is **WITHHELD rather than
  truncated** — an ellipsis renames the event on screen, and shrinking type below what the rest of the
  drawing sets trades an unreadable label for an illegible one. The sentence below still carries it in full.
- [x] 33.7 **The one recorded observed failure in this window, and it belongs in the record because the
  estimator did not catch it.** The first rule anchored a rail label on the rail's start with a fallback to
  its end, which on the narrow chart put "Pandemia de COVID-19" entirely left of its own rail and partly
  under the crisis rail above — **a name attached to the wrong mark**. Centring, a clamp, and an explicit
  "must still overlap its own rail" now hold it, and two unit tests name that failure:
  `annotationLabels.test.ts:224` ("centres a rail label on the rail it names, so it can never be read as
  belonging to the next one") and `:247` ("never lets a clamped rail label lose contact with the span it
  names"). Both verified present at `5af95c5`.
- [x] 33.8 The description sentences stayed and moved into their own polite live regions, for two reasons.
  The labels live inside a single `role="img"` that prunes its descendants, so a screen-reader user reaches
  not one glyph of them — deleting the sentence would hand sighted readers a feature and take it from
  everyone else. And the drawing does not promise to label everything: a name that cannot be drawn whole is
  refused, and the sentence is what keeps that omission **disclosed rather than silent**.
- [x] 33.9 Labels are painted OVER the data with a background-coloured halo — the one place the marks break
  their own under-the-data rule — because a 2-unit accent stroke through a 10-unit glyph erases the letter
  rather than dimming it. Four new pairings are declared at the **4.5:1 BODY-TEXT** threshold rather than
  the 3:1 the existing mark pairings use: this is text a reader must actually read.
- [x] 33.10 GREEN — without JavaScript a reader now sees **no government marks at all**, asserted rather than
  left to be discovered (`indicator-pages-no-js.spec.ts` +24/−12). That is the inconsistency being removed:
  exogenous has behaved this way since it shipped and the government select is absent on the same "absent,
  not disabled" principle. Nothing is hidden — the chips are server-rendered in a CSS-only disclosure and
  the data table is complete.
- [x] 33.11 Disclosed: 14 units renders at **7.81 CSS px** on a 375px viewport, the honest floor this design
  reaches — larger loses "Felipe González" on `poblacion-residente`. A shorter editorial `short_name` in
  `eventos.yaml` would let the crisis rail be labelled on phones; that is a four-eyes editorial file and
  not this change's to write. Carried into design.md Open Questions by this pass.

---

## Slice 34 — a policy-measures registry, scoped per series and citable (later reverted)

Commit `610290a`. 2,559 added, 95 removed across 35 files, including migration `0007_event_scope.{up,down}.sql`
(57/21), `config/medidas.yaml` (123), `web/src/lib/chart/measureMarks.ts` (253),
`app/internal/adapters/config/measures_test.go` (265), `app/internal/adapters/postgres/events_scope_test.go`
(178), and `web/tests/e2e/workbench/chart-policy-measures.spec.ts` (164). **Reverted two commits later by
slice 36 (`5af95c5`). Recorded in full anyway, because migration 0007 survived it and its retention is the
one decision pass 7 said had no durable home.**

- [x] 34.1 The prohibition drove the design more than the feature did: the owner asked to see which measures
  were taken and when, and agreed the site must not say whether they worked.
- [x] 34.2 **Enforced structurally, not by discipline.** `EventConfig` had no field for an effect, an
  outcome, a direction, a magnitude or an evaluation, so no layer downstream — digest, artifact, chart,
  generated sentence — could project one, and none of them had to be trusted not to. The same structural
  refusal `gobiernos.yaml` already applies to party colour.
- [x] 34.3 The geometry enforced it too, and this is the part that needs no words: a measure mark sat in the
  bottom margin, BELOW the x-axis tick labels, **outside the plot area entirely**, so it could never sit
  adjacent to the curve and could not assert an effect by adjacency — the failure mode where a mark at a
  point the curve then falls says "it worked" with no author and no citable source. Asserted three times: a
  unit test, a renderer test, and a real-browser bounding-box measurement.
- [x] 34.4 The copy stated instrument and date and stopped, and its test asserted the **ABSENCE** of
  `efecto`, `impacto`, `consecuencia`, `resultado`, `gracias`, `debido`, `logr`, `consigui`, `mejor`,
  `empeor`, `redu`, `aument`, "desde entonces" and "tras la medida". The sentence closed by refusing
  explicitly — "El gráfico no representa ninguna relación entre esas medidas y la evolución de la serie" —
  because silence is not neutrality when the layout poses the question. No derived figure was computed
  anywhere: no periods-after delta, no cross-government comparison, in any layer.
- [x] 34.5 GREEN — the registry reused `EventConfig` with a new `measures` group rather than getting its own
  table, and in doing so **closed a gap the code already admitted**: `events_read.go` carried a TODO saying
  `seriesID` "is accepted … to leave room for a real per-series scope in a future slice" while the function
  ignored it, and every event was global by schema. A second registry would have routed around that
  acknowledged gap while duplicating eight working layers. So `event` gained scope, widening
  series-in-dataset-in-source with the break registry's rule verbatim.
- [x] 34.6 GREEN — migration `0007_event_scope` defaults every pre-existing row to `global`, which is
  exactly what each already meant.
- [x] 34.7 Five measures seeded, **none written from memory**: each date is entry into force — not approval,
  not publication — read off the BOE consolidated text at the URL recorded in the entry. RDL 3/2012, RDL
  8/2020, RDL 12/2021, RDL 32/2021 and RDL 6/2022. RDL 32/2021's entry into force is staggered by its
  disposición final octava, so the recorded date is the norm's general one and the note said so. Every
  entry flagged for editorial review: `config/**` is a documented four-eyes path and this was a single
  reviewer.
- [x] 34.8 The mark reused the event rail's colour **on purpose**: a fifth vertical rule in a sixth hue
  would have left colour as the only channel separating it from the government marker, which PRD §12.5
  forbids. What separated it was REGION — the only annotation living outside the plot — plus
  stub-versus-full-height and a whole row of tick labels between it and the axis. Discard colour entirely
  and the five marks were still five.
- [x] 34.9 Disclosed and not fixed at the time: `measures` opened by default, unlike the other groups,
  widening the spec requirement titled "Three separately toggleable annotation groups, two off by default"
  to four groups, two off. Every scenario under it still passed; the title was under-descriptive. Flagged
  as the owner's delta to accept rather than silently amended. **Moot after slice 36** — the revert restores
  the code to what the spec already said, and pass 7 confirmed `git log -- openspec/` over both commits is
  empty.
- [x] 34.10 One structural limit recorded at the time and still true: one measure carries one scope. A
  measure touching both EPA and Social Security would need an `event_scope` child table.

---

## Slice 35 — listing only the annotations that are actually in the visible window

Commit `578bb86`. 1,028 added, 36 removed across 13 files. New: `web/src/lib/chart/annotationWindow.ts`
(180), `web/tests/e2e/indicator/indicator-annotation-range.spec.ts` (248),
`web/test/chart/island-annotation-chips.test.ts` (163), `web/test/chart/annotationWindow.test.ts` (140).
Modified: `indicator-chart.container.test.ts` (+72), `ChartIsland.svelte` (+51/−1), and six others.

- [x] 35.1 **The defect, measured in a browser, and this is the RED-equivalent evidence for this slice.**
  Narrowing `tasa-de-paro-epa` to "Desde 2018" took the series from **98 points to 34** and the measure
  marks from **12 to 8**, while the chip list beside it still named a 2012 reform. The drawing respected
  the window; the list did not.
- [x] 35.2 All three groups were affected, not the one that happened to be measured, and **`governments` was
  the worst: it lied at the DEFAULT view**, listing Calvo-Sotelo (1981), González (1982) and Aznar (1996)
  on a series that begins in 2002. No reader had to touch a control to see it.
- [x] 35.3 The cause was the shape of the code, not an oversight in one place: the in-range rule existed
  **three times** — once inside each selector — and the chips had none. GREEN — it now exists once, in its
  own module (`annotationWindow.ts`), and the three selectors read it instead of restating it. Divergence
  stops being possible rather than stopping by discipline.
- [x] 35.4 The rules per entry kind are the ones the marks already used, **reused rather than re-decided**.
  An instant is in range when its date falls inside the window. An interval **intersects** rather than
  being contained, because a 2008–2013 crisis legitimately covers a 2011–2018 window. An event with no end
  draws no rail at all, so the only honest test is the instant rule on the one date it has — which is why
  exogenous shows four chips over two rails at full range, and the sentence already accounted for it by
  naming only what the registry bounds with a start AND an end.
- [x] 35.5 GREEN — a group with nothing in the window is removed, control included, **and the cost is stated
  because it is real**: a reader who narrows far enough watches the group vanish under their own hand and
  nothing says "none in this period". What decided it is that the toggle gates the DRAWING, not just the
  list, so an empty group is a 44px focus stop that cannot change one pixel — exactly the dead control
  `availablePresets` and the government filter already refuse. A third precedent was already inside the
  same component: `visibleBreaks` vanishes when the window carries no rupture, and breaks are the more
  load-bearing layer.
- [x] 35.6 What a reader loses was **checked rather than assumed**. The 2012 reform survives in "Todo el
  periodo", which `isPresetAvailable` offers unconditionally on every series, and in the per-series JSON
  the action bar links, which carries all thirteen events regardless of window. It is NOT in the
  methodology sheet — that field is the indicator's definition, not policy — and not in the CSV. Two routes,
  not the one most people would guess.
- [x] 35.7 The three generated sentences were never part of the bug: they already derived from the same
  selectors. They now agree with the chips and the marks, verified layer by layer on the rebuilt stack at
  both ranges.
- [x] 35.8 Zero new Spanish strings. The empty-group decision is what avoided needing a "ninguna en este
  periodo" one.
- [x] 35.9 **This slice closes verify-report pass-6 SUGGESTION-21** — `indicator-annotation-range.spec.ts`
  now exercises range selection on real indicator routes rather than only in the workbench. Recorded as
  CLOSED by pass 7 §E.

---

## Slice 36 — removing the policy-measures layer, keeping migration 0007, and collapsing the data table

Commit `5af95c5`. 1,450 added, 2,151 removed across 45 files — **the only commit in this window with more
removals than additions**. New: `web/tests/e2e/indicator/indicator-data-table.spec.ts` (318),
`app/internal/adapters/config/event_scope_test.go` (241),
`web/test/design-system/data-table-disclosure-parity.test.ts` (180),
`web/test/chart/tableSummary.test.ts` (82), `web/src/lib/chart/tableSummary.ts` (61). Removed:
`config/medidas.yaml` (−123), `lib/chart/measureMarks.ts` (−256), `measures_test.go` (−265),
`chart-policy-measures.spec.ts` (−212), `measureMarks.test.ts` (−154), and the measures branches of
`svg.ts` (−123), `description.ts` (−49) and `es.ts` (−72).

- [x] 36.1 The owner decided against the measures layer. Gone: `config/medidas.yaml` and its five seeded
  entries, the `measures` group and its validation rules, the chart's gutter marks, its legend entry, its
  generated sentence and every Spanish string under `chart.measure`.
- [x] 36.2 What did NOT go with it, stated explicitly because a revert is where a good fix gets thrown out
  by accident: slice 35's fix. `governments` was wrong at the DEFAULT view, and the shared in-range
  predicate, the two remaining selectors reading it instead of restating it, and the empty-group-is-absent
  behaviour all stay — proved intact on the live stack at both ranges.
- [x] 36.3 **Migration 0007 stays, and that is a decision rather than an omission.** Verify-report pass 7
  named this the one decision with no durable home; it is recorded here in full. The scope columns are not
  dead schema: `scope_kind` is `NOT NULL`, all fifteen rows carry a real `'global'`, and `ListActiveEvents`
  reads it on **every export of every series** — it simply resolves to one branch today.
- [x] 36.4 Reverting would have **restored a defect the code had already documented**: `events_read.go`
  carried a TODO saying `seriesID` "is accepted … to leave room for a real per-series scope in a future
  slice" while the function ignored it, so dropping the columns means going back to a filter that lies
  about filtering.
- [x] 36.5 A `DROP COLUMN` on a live database is the strictly riskier of the two operations, for no
  functional gain. Keeping 0007 as the newest migration also leaves the hand-counted `Down()` step
  assertions untouched rather than adjusting the same fragile counts twice.
- [x] 36.6 **Verified by the auditor rather than asserted by the writer, and one clause of the writer's own
  claim turned out to be wrong in the PESSIMISTIC direction.** Verify-report pass 7 §B.3 confirmed
  `0007_event_scope.up.sql:52` (`scope_kind text NOT NULL DEFAULT 'global'`, `scope_ref text NOT NULL
  DEFAULT ''`, `source_url text`, plus a partial index) and `events_read.go:62-70`'s `WHERE` clause — and
  then established that **configuration CAN populate these columns**, which the revert commit had left
  ambiguous. Four `config/eventos.yaml` mutations run through `validate-config`: an unresolvable
  `scope.ref` → exit 1; an invalid `scope.kind` → exit 1; `kind: series` with no `ref` → exit 1; a valid
  `{ kind: series, ref: ocupados-epa }` → exit 0, accepted. Persistence and read-back are covered by
  `TestReconcileEvents_PersistsScopeAndSourceURL`, `TestListActiveEvents_ResolvesScopeForTheSeriesAsked` and
  `TestListActiveEvents_UnknownSeriesStillResolvesGlobalEntries`. Pass 7's verdict: keeping 0007 was
  correct — "not dead columns … a live, validated, exercised read path that today carries one value because
  one value is the truth."
- [x] 36.7 One residual on that path, recorded rather than closed (pass-7 SUGGESTION-52): `event.source_url`
  is wired end to end — YAML → reconcile → column → `EventRef.SourceURL` → Zod `EventRefSchema` — and
  unit-tested, but **no shipped config entry populates it**, so the key is `omitempty`-absent from every
  production artifact and the JSON leg has no production exercise. Carried into design.md Open Questions.
- [x] 36.8 The five rows already reconciled into the live database were removed by the existing soft-retire
  path with **no operator step**: `events inserted=0 updated=0 retired=5`. `ListActiveEvents` filters on
  `retired_at IS NULL`, so nothing reaches the artifact or the page.
- [x] 36.9 Three tests were **kept and rewritten rather than deleted**, because they guard something still
  true: the scope predicate is now the only thing holding series-in-dataset-in-source correct while no
  config populates it; the digest test stops scope or citation drifting silently between YAML and row; and
  the artifact tests pin the optional-never-nullable contract on `source_url`. One test was **added** — a
  group emptying removes only itself and leaves the section standing, which is exactly the default-view
  `governments` case and was previously only implicit.
- [x] 36.10 Nothing needed reverting in `openspec/`: `git log -- openspec/` over both commits is empty. The
  requirement still reads "Three separately toggleable annotation groups, two off by default" and
  enumerates exactly `gobiernos`, `shocks exógenos` and `hitos`, so removing measures restores the code to
  what the spec already said. Independently re-confirmed by pass 7 §B.1.
- [x] 36.11 **One observed failure, and it was the loader contract working.** The first rebuild after the
  removal FAILED, correctly: the locally generated artifact still carried `group: "measures"` and the
  tightened Zod enum refused it. It declined to build a site from data that no longer matches the code.
- [x] 36.12 The data-table collapse rides in this commit, and the reason is stated rather than left to be
  noticed: both changes edit `ChartIsland.svelte` and `es.ts`, and splitting them would mean staging hunks
  by hand. **Verify-report pass 7 raises exactly this as SUGGESTION-51** — a commit named "revert" adds a
  feature — and records that everything it adds is tested and green.
- [x] 36.13 The defect it fixes, measured: ninety-eight rows on `tasa-de-paro-epa` and two hundred and
  ninety-four on `ipc-general` arrived open, pushing everything below fifteen screens down. The document
  goes from **12,226px to 2,467px** on the monthly series.
- [x] 36.14 GREEN — a native `<details>`/`<summary>`, no JavaScript, copied from the closed-by-default
  disclosure `MethodologySheet` already uses and which already satisfies the same requirement. **Verified
  against the scenario text rather than assumed**: the spec's own scenario says the table must be PRESENT
  with JavaScript disabled and its single assertion is presence, not paint — checked along with the two
  `web-accessibility-gates` scenarios, which are about the DOM and the accessibility tree. Pass 7 §B.2
  independently re-adjudicated this as COMPLIANT and recorded that the test checks the
  `aria-describedby` reference is not dangling **while the disclosure is closed**, which is the failure
  mode a disclosure implemented by *removing* the table would have introduced.
- [x] 36.15 GREEN — both renderers were unguarded hand-duplicated markup, so the label is now one pure
  function (`tableSummary.ts`) both print, and a new parity test renders both and compares them
  (`data-table-disclosure-parity.test.ts`, 180 lines). **This project has already had one defect from two
  renderers of the same table drifting**, which is why the guard is a rendered comparison and not a
  code-shape assertion.
- [x] 36.16 The summary reads "Tabla de datos (98 periodos, de T1 2002 a T2 2026)" rather than "Ver la tabla
  de datos", following the rule `es.ts` already states for the annotation toggles: no script updates this
  text, so "Ver" becomes a lie the moment it is open. A noun phrase is true in both states and the
  browser's triangle carries the state.
- [x] 36.17 The caption stays, and a test asserts the two are not equal: the summary labels the disclosure
  widget; the caption is the table's accessible NAME, which is what a screen reader announces on landing
  and what the chart is described by. They say different things.
- [x] 36.18 Open state is deliberately **not** bound to component state: the `details` sits outside every
  `each` and `if` block, so narrowing the range updates the rows and the label while leaving open or closed
  as the reader set it.
- [x] 36.19 **Two no-JS assertions changed meaning, recorded as a change rather than as a fix.** They
  asserted the table was visible, which is now false by design, so they became presence plus a non-zero row
  count plus a visible summary plus proof it opens. Strictly stronger — the old assertion could not tell a
  rendered table from a rendered empty one — but changed, and a strengthened assertion that silently
  replaces a weaker one is exactly the kind of edit that needs saying out loud.
- [x] 36.20 Also added: an axe audit with the disclosure OPEN, because the existing sweep audits the page as
  it arrives, which since this change no longer includes the table's contents.

---

## The WARNING-47 spec decision — taken, argued, and written

Verify-report pass 7 raised **WARNING-47**: the homepage (slice 21) and the site footer (slice 25) ship
implemented, tested and reader-facing with no requirement in any of the twelve delta specs and none in the
ten baseline capabilities governing either. The decision was left to this record pass. **It is taken here:
both get requirements**, and the reasoning is below rather than in a commit body.

- [x] W47.1 **The footer's licence sentence is NOT unpinned today — but the pin does not reach the page, and
  that distinction is the finding.** Measured here: `openspec/specs/source-attribution-licensing/spec.md:26`
  already carries the requirement "No blanket data-licence claim exists in the repository", whose prose
  reads "The repository MUST NOT assert a single licence over all derived data". The footer is part of the
  repository, so the requirement's **text** governs it. Its only scenario does not:
  "GIVEN `LICENSE` and `LICENSE-DATA` / WHEN they are read" — two files, neither of them a rendered page.
  So a footer that asserted CC BY over all data would violate the requirement's sentence while passing its
  only scenario.
- [x] W47.2 What protects it in the meantime, verified rather than assumed: `web/test/pages/site-footer.test.ts`
  (304 lines) asserts the rendered text and markup match none of `/cc\s*by/i`, `/creative\s*commons/i`,
  `/todos los datos/i`, `/licencia de los datos/i`. That is a real guard and it is green — but it is a test
  with no requirement to adjudicate against, which is precisely the state this project's own record calls
  the class of decision that decays.
- [x] W47.3 GREEN — `specs/source-attribution-licensing/spec.md` is added to this change as a **MODIFIED**
  requirement, not a new one. It restates the existing requirement unchanged and adds one scenario reaching
  the surface where the claim is now made on every page. This is the smallest possible pin: no new claim,
  no new capability semantics, and it lands in the capability that already owns the subject.
- [x] W47.4 GREEN — `specs/indicator-page/spec.md` gains one ADDED requirement for the homepage: it lists
  exactly the six frozen slugs or the build fails, each row links its route, every indicator page links
  back, and the listing derives from the same guard the routes use. Bounded to milestone 1.2's own frozen
  set by construction — it names the six and asserts an all-or-nothing derivation, so it **cannot grow into
  the ~26-indicator catalogue, search or category navigation** that `proposal.md:70-72` puts out of scope in
  milestones 1.3–1.7.
- [x] W47.5 Why writing them rather than recording them as shipped-outside-scope, argued against the
  alternative. **The project's actual rule is not "the spec phase is closed".** Two requirements were added
  after it closed in this very change, both when a real gap was found on the running product: `5310586`
  (slice 19, `publishing-export`) and `026c7fa` (slice 20, `source-ingestion-ine`). A third, for a claim
  with legal weight printed on every page and for the only surface on which a reader learns an indicator
  exists, is the same move for the same reason.
- [x] W47.6 The cost, stated rather than glossed: this adds a **thirteenth** capability delta to the change
  and two requirements plus their scenarios to re-verify, so pass 8 has more to check than the record
  alone. Accepted, because pass 8 is required for the record update regardless and because the alternative
  leaves a legal-weight claim asserted by a test alone.
- [x] W47.7 What was deliberately NOT written. No requirement for the *content* of the four footer links,
  the tagline, the card layout, or the freshness badge geometry — those are design decisions with tests,
  not contract. No requirement obliging `/transparencia/raw-files.sha256` or `/data-derived/**` to resolve
  from a static build: slice 25 task 25.9 records why that is untestable in `dist/`, and
  `publishing-export/spec.md:146-155` already records the accepted consequence for the `/data-derived` half.
- [x] W47.8 One consequence pass 7 named and this pass does not close:
  `.github/workflows/ingest-export-build.yml`'s real-artifact assertion loop iterates the six indicator
  slugs only, so `/index.html` is never checked to exist in a real-artifact build. Pass 7 verified that it
  does exist; nothing enforces it. Recorded as an open item in design.md rather than fixed here, because
  this pass changes no workflow file.

---

## Record-pass verification (2026-08-04, at `5af95c5`)

- [x] V.1 Task count, **counted rather than asserted**: `grep -c "^- \[x\]" tasks.md` and
  `grep -c "^- \[ \]" tasks.md`, run after this section landed. Figures reported in `apply-progress.md`'s
  "Verification for this record" section, which also names what was deliberately not re-run.
- [x] V.2 `git log --oneline 823311e..HEAD | wc -l` → 17; `git diff --shortstat 823311e..HEAD` → 112 files
  changed, 14,631 insertions(+), 425 deletions(−). Both match verify-report pass 7's figures exactly.
- [x] V.3 Every per-commit file list and line count in slices 20–36 was read from `git show --numstat` for
  that commit, not from its body.
- [x] V.4 `go run ./app/cmd/concontexto validate-config` → `validate-config: ok`. Run because this pass
  touches `openspec/` only, to prove it touched nothing a Go test reads.
- [x] V.5 **No Go, Vitest or Playwright suite was re-run for this record.** This pass changed only
  `openspec/changes/phase-1-indicator-page/**`, which no suite reads, so a suite result would describe
  nothing this pass did. Where suite numbers appear above they are **verify-report pass 7's measurements at
  a clean `5af95c5`**, labelled as such: `go test -race -count=1 ./...` exit 0 over 22 packages, Vitest
  799/799, Playwright 191/191, `astro check` 0 errors over 131 files, `EXPORT_DIR=data-derived npm run
  build` exit 0 emitting 7 pages, and the transferred-bytes gate against the real artifact with a worst
  page of 76.6 KB.
- [x] V.6 **There is no structural validator for these spec files, and saying so is part of the record.**
  `which openspec` returns nothing and no repository script parses `openspec/**/spec.md`. The two spec
  files this pass writes were checked by hand against the established shape — `## ADDED Requirements` /
  `## MODIFIED Requirements`, `### Requirement:`, `#### Scenario:`, GIVEN/WHEN/THEN/AND bullets — and
  against the sibling deltas in this change. Nothing machine-checks them.

---

## Slice 37 — bringing the record to HEAD, and pinning two surfaces that shipped unspecced

Commit `374eb80` (2026-08-04 21:34 +0200). 2,605 added, 332 removed across 6 files, **all of them under
`openspec/changes/phase-1-indicator-page/`**: `tasks.md` (+1,042), `apply-progress.md` (+856), `design.md`
(+184/−…), `specs/source-attribution-licensing/spec.md` (+67), `specs/indicator-page/spec.md` (+57),
`verify-report.md` (sweeping in pass 8's report). Figures read from `git show --numstat 374eb80`, not from
the commit body.

- [x] 37.1 Closed verify-report pass-7 **CRITICAL-46**: `tasks.md` and `apply-progress.md` had last been
  written by `823311e` and seventeen commits had landed since. Task rows for slices 20–36 and the matching
  narrative plus TDD Cycle Evidence tables were written in this one commit.
- [x] 37.2 The **WARNING-47** spec decision was taken and written: the site footer gets a **MODIFIED**
  requirement in `specs/source-attribution-licensing/` and the homepage an **ADDED** requirement in
  `specs/indicator-page/`, both bounded to shipped behaviour. Reasoning in `apply-progress.md`'s
  "The WARNING-47 spec decision, and what it rests on".
- [x] 37.3 Three decisions that existed only in commit bodies were moved into the record: migration 0007's
  retention argument, slice 36's revert scope, and the unconfirmed-editorial-date disclosure.
- [x] 37.4 **This commit is the reason the third process finding exists.** Reconstructing seventeen commits
  after the fact left six of them with no reconstructible red state, permanently. Recorded in
  `apply-progress.md`'s "A third process finding".
- [x] 37.5 **Recorded late, and recording that is the point.** This slice was itself unrecorded for one
  commit — `9483d23` landed 31 minutes later and neither commit appeared in `tasks.md` or
  `apply-progress.md` until this pass. Verify-report pass 9 raised it as WARNING-59, the second staleness
  finding in two passes and the fifth occurrence in this change overall.

---

## Slice 38 — an entry the configuration declares unverified is held back

Commit `9483d23` (2026-08-04 22:05 +0200). 614 added, 433 removed across 7 files; excluding
`verify-report.md` (which this commit only swept pass 8's report into), the code change is **288 added, 34
removed across 6 files**: `reconcile_test.go` (+189), `types.go` (+39/−14), `reconcile.go` (+39/−12),
`events_read.go` (+14/−3), `fase0_closure_test.go` (+5/−3), `validate.go` (+2/−2). Figures read from
`git show --numstat 9483d23`.

- [x] 38.1 **The defect.** `config/eventos.yaml`'s `ngeu-primer-desembolso` carries `date_status:
  unconfirmed` **and** a provisional `date_start`, and its own `todo` says the month comes from unverified
  general knowledge. `reconcile.go`'s projection guard tested `e.DateStart == nil` and never read
  `e.DateStatus`, so the entry was projected, reached all ten published series documents and every rendered
  page with no disclosure of its pendingness, and the operator backlog reported **six** pending entries
  where the configuration declares **seven**. All three clauses of `editorial-config`'s "Unconfirmed
  editorial dates are operator-visible, never reader-visible" failed at once.
- [x] 38.2 **The fix, as a class fix.** Both registries now route through one predicate,
  `isDatePending(dateStatus string, date *time.Time) bool` (`app/internal/ingestion/reconcile.go:167-168`),
  called at `:74` (breaks) and `:87` (events). It reads the **declared status** first; the nil check stays
  as its second half, guarding the pointer dereference at the call site if `validate-config` is ever
  bypassed, rather than restating the rule.
- [x] 38.3 `BreakConfig` carried the identical latent defect — same triple, same nil-date guard — latent
  only because all four unconfirmed breaks in the shipped YAML happen to omit their dates. Fixed in the
  same commit, before any shipped break exercised it.
- [x] 38.4 **The provisional-date shape is now blessed, not merely tolerated.** `types.go:63-68` records
  that a provisional date alongside `DateStatus: unconfirmed` is allowed and is usually the better entry:
  the guess plus the `todo` tells the next editor the current best reading AND what to check, where an
  empty field tells them only that somebody stopped. Validation keeps allowing the shape; the reconcile
  holds it back.
- [x] 38.5 Three shipped comments that asserted this could not happen were rewritten rather than left
  accidentally true: `reconcile.go:10-12`, `events_read.go:47-49` (which gained the load-bearing point that
  the `event` table has **no `date_status` column at all**, so a projected unconfirmed entry would be
  indistinguishable there and that function could not filter it even if it tried — the guarantee holds
  upstream or not at all), and the `fase0_closure_test.go` comment that said "never projects a nil date",
  which was trivially true and told a reader nothing. `validate.go` switched to the named
  `config.DateStatusUnconfirmed` constant in both switch statements.
- [x] 38.6 **Two tests, complementary rather than redundant — and pass 9's mutation testing is what
  establishes which half each carries.** `TestReconcileEditorialConfig_AProvisionalDateOnAnUnconfirmedEntryIsStillHeldBack`
  (`reconcile_test.go:457`) is the class test, asserting a break and an event in one call so a half-applied
  fix fails. `TestReconcileEditorialConfig_ShippedConfigPendingListsAreExactlyItsUnconfirmedEntries`
  (`reconcile_test.go:536`) runs the **shipped** config through `ReconcileEditorialConfig` against a real
  database, deriving its expectation from `DateStatus` alone and asserting the identifiers and the total
  count **separately**, because those two failed apart here. See the TDD Cycle Evidence table for the
  mutation results.
- [x] 38.7 `config/**` was left untouched, deliberately. The date is checkable against the RRF disbursement
  calendar the `todo` names, but confirming it is an editorial act on a **four-eyes path** and this was a
  single reviewer. Seven remains the correct number: four breaks plus three events, counted from what the
  YAML **declares**, which is what both the shipped closure test and the spec scenario already counted; the
  product's six counted what the guard **inferred**.
- [x] 38.8 The two rejected fixes, rejected on the merits: confirming the date under four eyes fixes one
  instance and leaves the guard wrong, so the next provisional date ships just as silently; filtering in the
  web layer leaves a guessed date in the database, in the artifact and in every JSON consumer, correcting
  only what a browser happens to show.

---

## Slice 39 — the last open scenario, and the record brought to HEAD again

This pass. Changes `openspec/changes/phase-1-indicator-page/**` and two `config/**` prose headers; it
touches no `app/` or `web/` source and no `verify-report.md`.

- [x] 39.1 **WARNING-41 closed by amending the scenario, after establishing on the code that the amendment
  is available.** `pipeline-operations` / "A failed rebuild raises an alert **immediately**" had no covering
  test since pass 4 and held requirement coverage at 78/79 and scenario coverage at 175/176. Verified before
  writing: a rebuild that dispatched and then failed **is** genuinely caught, by
  `scheduler.RebuildLatencyBreached` (`app/internal/scheduler/watchdog.go:98-110`) via
  `publishLatencyWatchdog` (`app/cmd/concontexto/schedule.go:478-482`), which raises
  `alerting.KindPublishLatencyBreach` naming the source and the elapsed time. The bound is the configured
  publish-latency budget (`APP_PUBLISH_LATENCY_BUDGET`, defaulting to
  `scheduler.DefaultPublishLatencyBudget` = 30 minutes, `watchdog.go:20`) — the same budget this delta's own
  "Publish latency has a stated budget" requirement already declares.
- [x] 39.2 Why the detection is real and not a plausible story: a failed rebuild pushes no image
  (`deploy.yml:39`'s conclusion gate), and `/web/build-manifest.json` is written by exactly one mechanism —
  the Dockerfile stage that fails the build rather than shipping an unobservable image (`Dockerfile:162-194`).
  So `deployed.GeneratedAt` cannot advance, the divergence outlives the budget, and the watchdog fires.
- [x] 39.3 The amended scenario is falsifiable in three directions, each mapped to a passing test: it must
  fire after the budget with stale pages (`app/cmd/concontexto/schedule_rebuild_watchdog_test.go:79-107`),
  must stay silent inside the budget (`app/internal/scheduler/watchdog_test.go:106-113`), and must stay
  silent once the pages carry the current artifact
  (`app/cmd/concontexto/schedule_rebuild_watchdog_test.go:109-128`).
- [x] 39.4 The amendment is recorded **as an amendment**, with a `(Previously: …)` block carrying the
  original scenario text verbatim and stating explicitly that it supersedes this delta's own earlier text
  rather than a baseline requirement. Requirement and scenario counts are unchanged: **79 / 176**.
- [x] 39.5 **WARNING-59 closed**: slices 37 and 38 above, plus their TDD Cycle Evidence rows in
  `apply-progress.md`.
- [x] 39.6 **`design.md`'s open item for this defect corrected.** It was unchecked and asserted in the
  present tense that `reconcile.go:86` "never reads `e.DateStatus`" and that the entry "reaches the
  published artifact" — both false at HEAD. Ticked, with a resolution block appended and the original body
  left standing under an explicit header warning, per this change's supersede-don't-delete rule.
  `design.md`'s "(reconcile already refuses nil dates)" parenthetical corrected the same way; that item
  itself correctly stays open, because the seven dates still need confirming.
- [x] 39.7 **WARNING-57 closed**: `config/rupturas.yaml`'s header and `config/README.md`'s editorial-registry
  paragraph both said an unconfirmed entry omits its date / carries the status "instead of a guessed date".
  Both contradicted `types.go:63-68` and the shipped `eventos.yaml`. Corrected in Spanish and English
  respectively. **`config/**` is a four-eyes path and this is a single reviewer** — the change is prose
  only, inside comment and documentation text that no loader parses, and it *removes* a claim the shipped
  configuration already violates rather than authorising any new data shape.
- [x] 39.8 **WARNING-58 and SUGGESTION-60 recorded, not closed.** Both are pass-9 findings about test
  coverage in `app/`, which this pass may not touch. Written into `apply-progress.md`'s slice 38 TDD Cycle
  Evidence and its accompanying prose so archive freezes them as known open coverage gaps rather than losing
  them with the verify report.

---

## Record-pass verification (2026-08-05, at `9483d23` + this pass's working tree)

- [x] V37.1 Requirement and scenario totals **counted, not asserted**:
  `grep -c '^### Requirement:' specs/*/spec.md` and `grep -c '^#### Scenario:' specs/*/spec.md`, summed —
  **79** and **176**, unchanged by the amendment, which renamed one scenario and rewrote its bullets
  without adding or removing a heading.
- [x] V37.2 Task counts **counted, not asserted**: `grep -c "^- \[x\]" tasks.md` and
  `grep -c "^- \[ \]" tasks.md`. Figures reported in `apply-progress.md`'s verification section.
- [x] V37.3 Per-commit figures for slices 37 and 38 read from `git show --numstat <sha>` for each commit
  individually, never from a commit body.
- [x] V37.4 `go run ./app/cmd/concontexto validate-config` — run because this pass edits `config/**`.
- [x] V37.5 `go test -race -count=1 ./...` — exit **0**, **23 packages ok** + 2 with no test files, zero
  race reports. Run because `fase0_closure_test.go` reads the config tree and could assert on its prose or
  counts. Playwright deliberately **not** run concurrently.
- [x] V37.6 **Run from the repository ROOT, not from `app/`, and the difference is load-bearing.** `go.mod`
  is at the root, so `cd app && go test ./...` reports 22 packages and silently omits the **root** package —
  whose `config_embed_test.go` is the one test that reads `config/`, the tree this pass edits. From the root
  it is 23. Verify-report pass 9's own 23-package figure is the root form; the `app/`-relative form would
  have skipped the only package this pass could plausibly have broken.
- [x] V37.7 **There is still no structural validator for these spec files.** `which openspec` returns
  nothing and no repository script parses `openspec/**/spec.md`. The amended scenario was checked by hand
  against the shape every sibling delta uses and against the `(Previously: …)` convention in
  `publishing-export/spec.md:231`, `source-ingestion-ine/spec.md:132`, `platform-runtime/spec.md:18` and
  this same file's line 91. Nothing machine-checks it.

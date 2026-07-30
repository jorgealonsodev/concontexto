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

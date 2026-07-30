```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:c8704724496f70936a6201940bbd9e8084b8904144859514e087f2fd18690149
verdict: fail
blockers: 1
critical_findings: 1
requirements: 74/74
scenarios: 152/152
test_command: go test -count=1 ./... then npm --prefix web test then npm --prefix web run test:e2e (sequential)
test_exit_code: 0
test_output_hash: sha256:c8704724496f70936a6201940bbd9e8084b8904144859514e087f2fd18690149
build_command: npm --prefix web run build:fixture
build_exit_code: 0
build_output_hash: sha256:16d5d3b82a15d1cb8f55a99da777be4149e48da98deae9d01bf572fc35118d66
```

## Verification Report

**Change**: phase-1-indicator-page
**Version**: N/A (delta specs, 12 capability files)
**Mode**: Strict TDD
**Pass**: THIRD (re-verification). Pass 1 returned FAIL with 4 CRITICAL / 7 WARNING / 3 SUGGESTION at
68/74 requirements and 145/151 scenarios. Pass 2 returned FAIL with 2 CRITICAL / 2 WARNING / 4
SUGGESTION at 73/74 and 150/151. All prior findings are preserved below with a re-adjudicated status.
**Artifacts read**: proposal, exploration, design.md, tasks.md, apply-progress.md (3,303 lines,
including its "process finding" section), 12 delta spec files, the pass-2 report.

Every claim below was re-derived from the repository and from commands this verifier executed. The
records (`apply-progress.md`, `tasks.md`, `design.md`) and the three remediation writers' reports were
treated as artifacts under audit, never as evidence. Where a record and the repository disagreed, the
repository won.

### Completeness

| Metric | Value |
|--------|-------|
| Tasks total | 191 |
| Tasks complete (`[x]`) | 191 |
| Tasks incomplete (`[ ]`) | 0 |
| Requirements in delta specs | 74 (counted from the 12 spec files) |
| Scenarios in delta specs | 152 (counted from the 12 spec files) |

The scenario count moved from 151 to 152 because the `publishing-export` amendment described under
CRITICAL-16 replaced one scenario with two. Per-file counts: data-model-vintages 4/8,
data-validation 1/7, design-system 9/14, editorial-config 3/9, indicator-page 15/34,
pipeline-operations 4/10, platform-runtime 1/2, publishing-export 11/18, series-transformations 8/14,
source-ingestion-eurostat 3/7, source-ingestion-ine 5/13, web-accessibility-gates 10/16.

### Build & Tests Execution (all re-run by this verifier, sequentially)

**Go static**: `go build ./...` exit 0, `go vet ./...` exit 0, `gofmt -l .` lists 0 files.
`sha256(output) = a3bbca58996e4c1cf636cb91229b6044a5bbd6b3f106bd3d9d93ddb05a9ee25e`

**Go tests**: `go test -count=1 ./...` exit 0 — 23 packages `ok`, 0 `FAIL`.
`sha256(output) = 9a6bfd534f6e1333e5be3f65034e7520bfc56b79e6ceedaf215a03944d06faea`

**Type check**: `npm --prefix web run check` exit 0 — 0 errors, 0 warnings, 2 hints.
`sha256(output) = 2c6d82db996dab224c6f54c4d6a18f4178787c66cb6a1dfd8a7a78b429ce6460`

**Web unit**: `npm --prefix web test` exit 0 — **Test Files 33 passed (33), Tests 438 passed (438)**.
`sha256(output) = 4f6b04e4a30477b6a81c00b441d19c35bbb55eef3cfa363e8173e89470d5d4a7`
The three remediation writers reported 426, 431 and 438 because each measured at a different moment;
438 across 33 files is the value this verifier measured on the current tree.

**Web e2e**: `npm --prefix web run test:e2e` exit 0 — **64 passed (25.8s)**.
`sha256(output) = 5d7acc23feaa52776e53230ecf068ddc6b5565729fafab53ddea48e98d02b790`
Port 4321 was unbound before and after. The Go and Playwright suites were never run concurrently.

**Build**: `npm --prefix web run build:fixture` (the command `openspec/config.yaml` now declares) exit 0,
7 pages.
`sha256(output) = 16d5d3b82a15d1cb8f55a99da777be4149e48da98deae9d01bf572fc35118d66`

**Container build against real pipeline data — the evidence the writer could not supply, supplied here.**
See CRITICAL-15 below. `docker build --build-arg EXPORT_DIR=data-derived` exit 0 against an artifact the
Go pipeline itself wrote; two negative container builds exit 1.

**Coverage**: no coverage tool is configured (no Go cover threshold, no Vitest coverage provider).
Skipped, not a failure.

---

### Adjudication of the four handed-over items

**CRITICAL-15 — CLOSED. Defence in depth, and this verifier supplied the missing production-data proof
rather than accepting the writer's substitute.**

The writer disclosed that their passing `docker build` used an artifact they had derived by hand
(truncating each series to its three real INE rows and recomputing digests), so their proof established
plumbing-plus-no-fabricated-values, not the real production data path. That disclosure was correct and
the proof was insufficient on its own. This verifier ran the real chain instead:

```text
1. E2E_EXPORT_DIR=$PWD/web/data-derived go test -count=1 \
     -run TestEndToEndIngestExportBuild ./app/internal/ingestion/...      exit 0
   -> web/data-derived/{manifest.json, series/*.json, csv/}
      manifest schema_version 1, series: ipc-general, ipc-subyacente, ocupados-epa,
      pib-cvi, poblacion-residente, tasa-de-paro-epa, test-e2e-export
   This is publishing.Export writing to disk from a real Postgres transaction after a real
   ingestion.IngestSeries run over the recorded INE payloads. No hand-editing.

2. docker build -f Dockerfile --build-arg EXPORT_DIR=data-derived .        exit 0
   "[build] 7 page(s) built"

3. docker create + docker cp <cid>:/web/dist  ->  the bytes the image actually serves
   dist/index.html + all six dist/indicador/{slug}/index.html
   grep '2026-Q2' tasa-de-paro-epa/index.html  -> value 9.87, status "D"
     (9.87 is the real INE EPA453100 2026-Q2 figure, cross-checked in PRD 11.3 and in
      app/internal/ingestion/testdata/datos_serie/source.txt)
   grep -c '2002-Q1' tasa-de-paro-epa/index.html -> 0
     (2002-Q1 = 11.85 was the fabricated value pass 2 read out of the built HTML)
```

Both refusal layers were then exercised as negative controls:

```text
docker build -f Dockerfile .                                exit 1
  "Dockerfile: the web build has no export artifact source." + both variable names
  (the pre-check at Dockerfile:98 fires before npm run build)

docker build -f Dockerfile --build-arg EXPORT_DIR=test/fixtures/export .    exit 1
  'export loader: "/web/test/fixtures/export/manifest.json" does not exist'
  (.dockerignore removed web/test from the context, so a build aimed straight at the
   synthetic fixture cannot reach it — the second layer, proven independently of the first)
```

Source-side, the fallback is gone and cannot be re-entered by accident. `resolveLoadOptionsFromEnv`
(`web/src/lib/export/loader.ts:214-230`) is `EXPORT_URL` -> `EXPORT_DIR` ->
`BUILD_WITH_SYNTHETIC_FIXTURE === "1"` -> throw, with the opt-in ranked BELOW the two real sources so a
stale variable cannot downgrade a build that was handed real data, and compared against the exact string
`"1"` so `=0`/`=false` do not read as yes. `readBytes` also throws when neither field is set, so an empty
`opts` object cannot silently read some directory nobody chose.

This verifier enumerated every path by which a build could publish synthesised values attributed to a real
source, and found none:

| Path | Artifact source | Can it publish? |
|---|---|---|
| `npm run build` (bare — the old Dockerfile command) | none | No: throws. Asserted by a REAL spawned build in `web/test/export/unconfigured-build-fails.test.ts`, plus a matched positive case so the refusal is pinned to the guard and not to an unrelated breakage |
| `npm run dev`, `npm run build:fixture` | opt-in, in the script name | Local/dev only; the name says what it is |
| `playwright.config.ts` webServer | `WORKBENCH=1 BUILD_WITH_SYNTHETIC_FIXTURE=1` | Local `dist/`, publishes nothing |
| `ci.yml` web job | `BUILD_WITH_SYNTHETIC_FIXTURE=1` | Publishes nothing — no upload-artifact, no push, no deploy in that job. Verifies build SHAPE, and its own comment says so |
| `ci.yml` container-smoke-test | real artifact from `TestEndToEndIngestExportBuild`, staged to `web/data-derived`, forwarded as the `EXPORT_DIR` build arg via `docker-compose.yml` | Real data. That test asserts all six frozen slugs are on disk, so the image gets the production route set |
| `Dockerfile` / `docker compose build` | `--build-arg EXPORT_URL` or `EXPORT_DIR`, no default | Pre-check + loader + `.dockerignore`; all three exercised above |
| `deploy.yml` (the only path that pushes to ghcr.io) | `vars.EXPORT_URL` | Login, build-push and the Portainer redeploy are ALL gated on `steps.artifact.outputs.configured == 'true'`; unset produces a `::warning::` no-op, not a fake green |

`web/src/pages/indicador/[slug].astro:41` is the only caller of the loader in `src/`, and
`SYNTHETIC_FIXTURE_DIR` appears nowhere in `src/` outside `loader.ts` itself.

Judgement on evidence sufficiency: the writer's hand-derived artifact was NOT sufficient, and they were
right to disclose it. It is now moot — the full path is proven above from the Go pipeline through to the
bytes a reader would receive.

**CRITICAL-16 — CLOSED, including the spec amendment, which this verifier adjudicated as rigorously as
the code.**

*The amendment.* `publishing-export`'s requirement was renamed to "An ingestion cycle exports what it
learned, and only what it learned" and its two scenarios became three
(`specs/publishing-export/spec.md:160-210`).

Was narrowing the right call? Yes. The original clause was not merely inconvenient, it was internally
contradictory with two clauses of its own capability ("A series whose latest run failed MUST still appear,
carrying its last valid data" and "the series carries the state that drives PRD 6.1.3's validation
banner"), and it made `indicator-page`'s "The three page states of PRD 6.1.3" unsatisfiable, because a new
artifact is a reader's only route to that banner. Where two requirements in one change are mutually
unsatisfiable, one must yield; the amendment picks the one whose INTENT survives intact. The protective
intent — a suspect datum must never be published — is not weakened at all, because it is guaranteed
structurally rather than by the export gate: `applyGate` (`postgres/gate.go:123-128`) returns before the
write loop on a `GateBlock`, so no observation exists for the export to read back.

Is the amended requirement still falsifiable? Yes, and this is the test the amendment had to pass. It does
not collapse into "always export". The third scenario preserves the surviving half of the original
guarantee verbatim in force — a cycle that learned nothing MUST NOT export and MUST NOT dispatch — so
"export more" is bounded above as well as below. The second scenario adds four independently checkable
post-conditions (suspect value absent, previous value latest, page state names the failure and the date,
current site served until the rebuild lands).

Is the amendment honestly disclosed? Yes. `spec.md:170-183` quotes the superseded text verbatim, states
what it contradicted, states the reasoning, and points at `design.md:106`'s D-2 resolution. Nothing was
silently rewritten.

*The code.* The gate is now `if (published || failedValidation) && outDir != ""`
(`ingest_cmd.go:503`), with `failedValidation` set from `result.Outcome == validation.GateBlock`
(`:433`) and `published` from `len(result.Published) > 0` (`:436`), both read BEFORE the error is handled.

It distinguishes the outcome kinds rather than collapsing them, and the discriminator is safe by
construction: `GateOutcome` is a `string` type whose constants are `"publish"` and `"block"`
(`validation/gate.go:22-28`), so the ZERO value is `""` — a fetch failure returning the zero `Result`
therefore cannot be mistaken for a block. Had `GateBlock` been the zero value, every fetch failure would
have exported. That is the sharpest way this remediation could have gone wrong and it did not.

*The mutation check.* The writer claimed one. This verifier ran it in both directions rather than
believing it:

```text
mutant A (over-widen):  if (published || failedValidation || failed > 0) && outDir != ""
  --- FAIL a_fetch_failure_with_no_payload_exports_nothing
        "expected no rebuild dispatch for a run that fetched nothing"
        "expected no artifact written for a run that fetched nothing, got err=<nil>"
  --- FAIL a_series_with_no_resolvable_source_ref_exports_nothing   (same two assertions)

mutant B (narrow back to the pre-remediation gate):  if published && outDir != ""
  --- FAIL a_validation-failed_run_exports_the_failure_state_without_the_suspect_datum
        "expected a validation failure to STILL export and dispatch, so the 6.1.3 banner
         reaches a reader; got stdout=...outcome=block published=0 run_id=2"

file restored and byte-compared against the pre-mutation copy: identical; full Go suite re-run green
```

Both directions are caught. The new test is not vacuous in either direction.

*The joining test.* `app/cmd/concontexto/ingest_export_gate_test.go` — one migrated Postgres container,
three subtests, real `httptest` sources, real `IngestSeries`, real `publishing.Publish`, and every
assertion read off DISK (`readExportedSeriesDoc`) rather than from `Publish`'s return value, on the stated
grounds that bytes on disk are what the Astro build consumes. The validation-failure subtest runs two
cycles with distinct explicit `now` values, so `lastCorrectUpdate` is asserted as run 1's date
(`2026-07-28`) and not run 2's or today's, and it checks `manifest.GeneratedAt` to prove the artifact is
NEW rather than run 1's leftover. This is the seam-joining test WARNING-17 said was missing.

One boundary this test does not cross, stated plainly: it stops at the artifact bytes and does not drive
them on through `astro build` to a rendered banner. The web half of that path is covered separately
(`pageState.test.ts`, the container tests) against the same Zod schema, and the full Go-to-HTML chain is
`ingest-export-build.yml` plus `ci.yml`'s `container-smoke-test`. The artifact is the declared contract
boundary, so this verifier accepts the split rather than calling it a gap.

**WARNING-17 — CLOSED, at two distances from the defect.** `Deps.SeriesValidationOutcome` is required:
`Export` (`publishing/export.go:219-221`) returns an error naming the port before any read and any write.
`TestExport_AnUnboundValidationOutcomePortIsRefusedRatherThanReportedAsFresh`
(`publishing/page_state_test.go:255-278`) sets the port to nil, asserts the error names it, and asserts
`os.ReadDir(outDir)` is EMPTY — so the refusal is proven to happen before bytes land, not merely to
happen. `TestBuildExportDeps_BindsEveryPortIncludingTheValidationOutcome`
(`export_cmd_test.go:73-92`) asserts all six `Deps` fields are bound at the one production composition
point, with no database and no Docker. The `Deps` struct has exactly six function fields and all six are
named in that table — verified field by field, not taken from the writer's count.

**WARNING-18 — DOWNGRADED to SUGGESTION-25. The scan is genuinely widened; the residual the writer
disclosed is real, is workbench-only, and never ships.**

The scan (`test/pages/indicator-page.container.test.ts:645-718`) is now `globSync("**/*.{astro,svelte}")`
over `src/`, with a self-check (`:676-680`) that discovery found at least 15 files and includes
`pages/index.astro` and `templates/IndicatorPage.astro` — so a `cwd` typo cannot make every `it.each`
vacuously pass. `index.astro`'s inlined string, which WARNING-18 named, is resolved through `es.ts`.

The writer's disclosed remaining gap is confirmed and is the ONLY one. This verifier ran an independent
stricter scan over all 15 discovered files that STRIPS `{...}` expressions instead of being defeated by
them, then looked for Spanish text in the remaining bare nodes. Exactly one hit:

```text
src/workbench/WorkbenchShowcase.astro:35
  <h2 class="mb-4 text-heading-md font-semibold">Tema: {themeLabel}</h2>
```

Every other hit in the unfiltered run was a code fragment (`{ jsonHref && (`) or a component name
(`IndicatorCard ()`), not Spanish prose. So all fifteen shipping-relevant files are genuinely clean, and
they are clean on the merits rather than because a weak regex could not see them.

Why this is a SUGGESTION and not a WARNING: `WorkbenchShowcase.astro` is the component workbench, gated
behind `WORKBENCH=1`, and it is absent from the production image `dist/` this verifier extracted (only
`index.html` and the six `indicador/{slug}/index.html`). `indicator-page`'s requirement governs
reader-facing copy; a developer surface that never ships is not reader-facing, so the requirement is
COMPLIANT. The regex's structural blind spot on mixed text-plus-expression nodes is a real
future-regression risk on files that DO ship, and is carried forward as SUGGESTION-25 with a reproducible
stricter scan attached.

---

### Issues Found

#### A. NEW CRITICAL finding (blocks archive)

**CRITICAL-23 — The change does not exist in the repository. Every green result in this report, and in
every writer's report, was measured on an uncommitted working tree that no clone can reproduce.**

Measured, not inferred:

```text
git ls-files web | wc -l                                                    5
  web/.gitignore  web/astro.config.mjs  web/package-lock.json
  web/package.json  web/src/pages/index.astro

files on disk under web/ (excl. node_modules, dist, .astro, test-results)  116

git check-ignore -v web/src/lib/export/loader.ts web/tsconfig.json \
    web/playwright.config.ts web/test/fixtures/export/manifest.json
  -> no output: NONE of them is ignored. They were simply never added.

git status --porcelain --untracked-files=all | grep -c '^??'               191
git status --porcelain | grep -c '^ M'                                      46
git log --oneline -1                    009168c docs: ADR-5 and ADR-6, and the Fase 0 SDD record
```

Untracked by area: `web/test` 49, `web/src` 47, `app/internal` 44, `openspec/changes` 18, `web/tests` 10,
`app/cmd` 7, `app/migrations` 6, `docs/adr` 2, plus `web/tsconfig.json`, `web/vitest.config.ts`,
`web/playwright.config.ts`, `web/svelte.config.js`, `web/scripts`, `.dockerignore`,
`scripts/assert-corrupt-artifact-fails-build.sh` and `.github/workflows/ingest-export-build.yml`.

`web/.gitignore` excludes only `node_modules/`, `dist/`, `.astro/`, `test-results/`,
`playwright-report/`, `blob-report/`, `playwright/.cache/`, `data-derived/` and `.build-guard-out-*/`.
The source tree is not excluded by anything.

**What a checkout can actually do — measured, not reasoned about.** This verifier materialised the
tracked-files-only state (`git ls-files -z | tar --null -T - -cf - | tar -xf -C <tmp>`), which is exactly
what a `git commit -a` of the current tree would produce, ran `npm ci` (exit 0), and then ran the steps
the modified-and-tracked `ci.yml` declares:

```text
npm run check     exit 1   [astro] Unable to load your Astro config
                           Failed to load url ./src/lib/indicator/redirects.ts ... Does the file exist?
npm run build     exit 1   same failure — astro.config.mjs (TRACKED, modified) imports an UNTRACKED file
npm test          exit 1   No test files found, exiting with code 1
npm run test:e2e  exit 1   Error: No tests found
go build ./...    exit 1   app/cmd/concontexto/ingest_cmd.go:35: no required module provides package
                             .../app/internal/adapters/github
                           app/cmd/concontexto/ingest_cmd.go:42: no required module provides package
                             .../app/internal/publishing
go vet ./...      exit 1   same
```

The tracked-files-only state does not compile — neither the Go module nor the Astro site. Not "the web
tests are missing": nothing builds at all, because tracked-and-modified files import entire untracked
packages.

Three consequences, in order of severity:

1. **No CI run has ever exercised this change.** At `HEAD` the web job is `npm ci` + `astro build` over a
   hello-world page and there is no `astro check`, no Vitest and no Playwright step (verified with
   `git show HEAD:.github/workflows/ci.yml`). The new workflows and the code they test have never
   coexisted on any commit. Every green in this report is a local measurement, including my own.
2. **The natural commit is a broken commit.** `git commit -a` lands the new `ci.yml`, the new
   `Dockerfile` and the new `astro.config.mjs` while leaving `.dockerignore`,
   `ingest-export-build.yml`, `app/internal/publishing/`, `web/src/lib/`, `web/test/` and the four web
   config files behind. That is the failure above, on `main`.
3. **Archiving would record delivered work the repository does not contain.** `apply-progress.md:2353`
   notes the untracked state in passing ("every path under it remains untracked from session start") and
   its own "process finding" section (`:3268`) — which is about stale records — does not treat it as a
   defect. It is one.

**This is a repository-state problem, not a code defect, and its remedy is a commit, which is the product
owner's action, not a writer's.** No amount of `sdd-apply` work will close it, and sending it back to
`sdd-apply` would be the wrong route. It is nevertheless CRITICAL and it blocks archive: the claim a
verify report exists to support — that the specified implementation exists and passes — is not true of
this repository, only of one working tree on one machine.

#### B. NEW WARNING finding

**WARNING-24 — The deployed stack never runs the pipeline, so every indicator page's CSV and JSON action
link 404s in production. This is the pipeline-to-publish boundary again, one layer below the one that was
just fixed.**

The shipped image serves `/web/dist`, and this verifier extracted it from the real image built in
CRITICAL-15: it contains `index.html`, `_astro/` and the six `indicador/{slug}/index.html`, and **no
`data-derived/` directory**. Meanwhile every indicator page links to it:

```text
grep -o 'href="/data-derived[^"]*"' dist/indicador/tasa-de-paro-epa/index.html
  href="/data-derived/csv/tasa-de-paro-epa.csv"
  href="/data-derived/series/tasa-de-paro-epa.json"
```

Those files are written at RUNTIME by the Go binary into `STATIC_ROOT/data-derived`
(`export_cmd.go:103`, `STATIC_ROOT=/web/dist`, volume `public_html:/web/dist`). Nothing in the deployed
stack ever invokes it:

- `docker-compose.yml`'s `app` service has **no `command:` override**, so it runs the Dockerfile's
  `CMD ["serve"]`. `serve` never ingests and never exports.
- There is no second service, no sidecar, no cron and no systemd timer. The compose file has exactly two
  services, `app` and `postgres`.
- `grep -rn "schedule|ingest" docs/deploy.md` returns one line, about the local end-to-end export. `docs/`
  never mentions the `schedule` subcommand at all, so an operator following the documentation never
  starts the pipeline.

So on a deployed stack the six pages render, and every "Exportar CSV" and JSON action on them 404s
permanently. That is CRITICAL-1's reader-visible symptom returning for a different reason: CRITICAL-1 was
a path-shape disagreement and is genuinely fixed (the container test resolves the rendered href against
the exporter's real layout), but nothing asserts the target exists in a deployed container.

Why WARNING and not CRITICAL. Two reasons, both checked. First, it is pre-existing rather than introduced
here: `git show HEAD:docker-compose.yml` has no `command:` for `app` either, so the gap arrived with Fase
0's scheduler. Second, no delta-spec requirement in this change governs deployment wiring —
`publishing-export`'s `/data-derived` requirement is about the export writing the CSV from the same
in-memory artifact, which is implemented and tested, and `pipeline-operations` specifies the dispatch and
the latency budget, not the process supervisor. So no scenario is UNTESTED and the decision gate does not
make it CRITICAL. It is recorded at WARNING because this change is the one that shipped six pages linking
at those URLs, and because it sits on precisely the boundary the last two passes found defects on.

#### C. SUGGESTIONS

**SUGGESTION-19** (carried, still open) — `web/src/workbench/fixtures.ts:88,94` still use
`/data-derived/{slug}.csv` without the `csv/` segment, while the shipped pages and `csv.go:44` use
`/data-derived/csv/{slug}.csv`. Workbench-only, never reader-facing. Align them so the two do not teach a
future reader different layouts.

**SUGGESTION-20** (carried, still open) — `indicator-page`'s validation-failure scenario is still silent
on the no-prior-success sub-case and its dateless banner wording. The implementation's refusal to invent a
date is correct under P4 and is tested; the spec does not record it, so the next reader cannot tell a
decision from a drift. `grep` for that case in the spec returns nothing.

**SUGGESTION-21** (carried, still open) — custom-range e2e coverage still lives entirely on `/workbench`.
No e2e exercises the picker on one of the six real `/indicador/{slug}` routes, which is where a
composition difference between the harness and the real page would surface.

**SUGGESTION-22** (carried, partially discharged) — `.github/workflows/ingest-export-build.yml` and
`ci.yml`'s rebuilt `container-smoke-test` have still never run on a real runner, and per CRITICAL-23 the
first of the two is not even tracked. This verifier has now reproduced the container half locally end to
end (real Go export -> `docker build` -> extracted `dist`), which is materially stronger than pass 2's
script-only reproduction, but it is still not an observed CI run.

**SUGGESTION-25** (new, replacing WARNING-18) — the no-inlined-copy scan's regex `>([^<>{}]{4,})<`
structurally cannot see a text node that shares its node with an expression (the `Fuente: {src}` shape),
so it would not catch that regression in a file that ships. All 15 files are clean today, proven by an
independent stricter scan; the guard is what is weak, not the code. A scan that strips `{...}` before
matching (14 lines of Python, reproduced in this pass) closes it, and would flag the one residual at
`src/workbench/WorkbenchShowcase.astro:35` (`Tema: {themeLabel}`) — which should either be routed through
`es.ts` or explicitly excluded as a developer surface, so the exclusion is a decision rather than a blind
spot.

**SUGGESTION-26** (new) — test output is left inside the source tree at
`app/cmd/concontexto/web/dist/data-derived/{manifest.json, series/test-sched-series.json,
csv/test-sched-series.csv}`. It is untracked and not ignored, so the blanket `git add -A` that CRITICAL-23
invites would commit generated test garbage into `app/cmd/`. Either gitignore it or have the test write to
`t.TempDir()`.

#### D. Prior findings, re-adjudicated

| # | Prior finding | Status this pass | Evidence |
|---|---|---|---|
| CRITICAL-1 | Every "Exportar CSV" link 404s | **CLOSED** (path shape). Deployment presence is a new, different defect -> WARNING-24 | `IndicatorPage.astro` emits `/data-derived/csv/${doc.slug}.csv`; `csv.go:44` writes `outDir/csv/{slug}.csv`; container test resolves the rendered href against the real layout with `existsSync` |
| CRITICAL-2 | The blocking budget gate cannot fail | **CLOSED** (pass 2, re-confirmed: unchanged, suites green) | `assertMeasuringProductionBuild` / `assertRealPageLoad` / `assertProductionBuild` each unit-tested with throwing and non-throwing cases |
| CRITICAL-3 | 37 inlined Spanish strings across 9 components | **CLOSED** | Independently re-verified this pass with an expression-stripping scan over all 15 `src/**/*.{astro,svelte}` files: zero bare Spanish outside the workbench |
| CRITICAL-4 | Export artifact carries no page-state | **CLOSED**, and delivery to a reader is now closed too | Pass 2 traced the artifact and rendering layers; this pass adds the joining test and the mutation check (CRITICAL-16) |
| CRITICAL-15 | Production container publishes the synthesised fixture as real INE statistics | **CLOSED** | Loader refuses; `Dockerfile` pre-check fires (reproduced, exit 1); `.dockerignore` removes the fixture from the context (reproduced, exit 1); real Go-pipeline artifact -> `docker build` exit 0 -> extracted `dist` carries INE 9.87 and zero occurrences of the fabricated 2002-Q1; `deploy.yml` gates build/push/redeploy on `vars.EXPORT_URL` |
| CRITICAL-16 | Failed-ingestion publish path untested and self-contradictory | **CLOSED** | Spec narrowed with the superseded text preserved verbatim and the surviving no-export guarantee kept as its own scenario; gate keyed on `GateBlock` whose type's zero value is `""`; three-subtest joining test against real Postgres reading bytes off disk; bidirectional mutation check run by this verifier |
| WARNING-5 | `personalizado` not built | **CLOSED** (pass 2) | Unchanged this pass |
| WARNING-6 | Acceptance evidence rests on a 3-point fixture | **CLOSED** (pass 2), and the CRITICAL-15 it created is now closed too | — |
| WARNING-7 | `ChartIsland.svelte` a11y warning every build | **CLOSED** (pass 2) | No such warning in this pass's 438-test run |
| WARNING-8 | CI budget gate measured the WORKBENCH build | **CLOSED** (pass 2) | Unchanged; ordering plus `assertProductionBuild` |
| WARNING-9 | Slice 1 has no TDD Cycle Evidence table | **CLOSED** (pass 2) | Present and labelled RECONSTRUCTED |
| WARNING-10 | design.md Open Question #1 stale | **CLOSED** (pass 2) | — |
| WARNING-11 | Two disclosures point at nonexistent design.md entries | **CLOSED** (pass 2) | — |
| WARNING-17 | Port binding unasserted, port optional | **CLOSED** | `Export` refuses before any byte lands (asserted, incl. empty `outDir`); all six `Deps` ports asserted bound at `buildExportDeps` |
| WARNING-18 | Copy scan hand-maintained; `index.astro` inlined string | **CLOSED as specified**, residual downgraded to SUGGESTION-25 | Scan is glob-derived with a 15-file floor; `index.astro` externalised; independent stricter scan finds one hit, in the never-shipped workbench |
| SUGGESTION-12 | Budget logic hand-duplicated into the CI script | **CLOSED** (pass 2) | — |
| SUGGESTION-13 | `methodology-heading-pib-cvi` on the pib page | **CLOSED** (pass 2) | — |
| SUGGESTION-14 | No TypeScript type-check gate | **CLOSED** (pass 2) | `npm run check` 0 errors this pass |
| SUGGESTION-19 | Workbench CSV href layout | **OPEN** | See above |
| SUGGESTION-20 | Spec silent on the dateless banner | **OPEN** | See above |
| SUGGESTION-21 | No custom-range e2e on a real route | **OPEN** | See above |
| SUGGESTION-22 | Workflows never observed on a runner | **OPEN**, partially discharged | See above |

All 18 prior findings are closed except four suggestions. Both pass-2 blockers are closed. One new
blocker was opened in their place, and it is not a code defect.

---

### Spec Compliance Matrix

**74/74 requirements complete. 152/152 scenarios compliant** — in the WORKING TREE. Not one of them is
compliant in the repository, for the reason CRITICAL-23 gives: the code is untracked and the
tracked-files-only state does not compile. This report gives both numbers rather than one, because
collapsing them would hide the blocker behind a clean matrix.

No non-compliant or partial rows remain. The single UNTESTED row pass 2 recorded — `publishing-export`'s
failed-run scenario — is resolved: that scenario was superseded, and both scenarios that replaced it are
covered by `TestRunIngest_TheExportGateReflectsEachOutcomeKind`, mutation-verified above. The third
scenario of that requirement, "Ingestion success chains to a rebuild", is covered by the same test's first
cycle plus `publishing/trigger_test.go:53-57` for the two recorded instants.

Neither CRITICAL-23 nor WARNING-24 is mapped to a requirement, and that is deliberate rather than
convenient. No delta-spec requirement in this change governs which commits exist or which process
supervises the container. Forcing a mapping would be dishonest. CRITICAL-23 blocks archive because
archiving asserts delivery, and there is nothing delivered to a repository.

### Correctness (Static Evidence)

| Requirement area | Status | Notes |
|---|---|---|
| Six frozen routes, pib/pib-cvi alias | Implemented | Verified in the real image `dist`: six `indicador/{slug}/index.html` |
| Zero DB / zero computation at request time | Implemented | Import guard is its own CI step; `[slug].astro:41` is the only artifact read in `src/` |
| Two-state freshness semaphore | Implemented | No build-staleness element in artifact or markup |
| Breaks always visible, never dismissible | Implemented | Unchanged from pass 2 |
| Provisional/definitive per point | Implemented | Real image carries INE's own `T3_TipoDato` (`status:"D"` on 2026-Q2) |
| Three annotation groups, two off by default | Implemented | — |
| Methodology traceability | Implemented | All six pages |
| No-JS baseline | Implemented | 64/64 Playwright incl. the `javaScriptEnabled: false` context |
| Artifact schema/validation on write and read | Implemented | Plus the required-port refusal proven to precede any write |
| Segmented cadence, Rule 2 interior audit | Implemented | — |
| Page state from the artifact, and delivered to a reader | Implemented | The open question from pass 2 is now closed by the amended gate + joining test |
| Build refuses to guess its artifact source | Implemented | Three container builds run by this verifier: 1 pass, 2 correct refusals |
| Artifact reaches a deployed reader as CSV/JSON | **Not implemented** | WARNING-24 — no process in the deployed stack ever writes `STATIC_ROOT/data-derived` |

### Coherence (Design)

| Decision | Followed? | Notes |
|---|---|---|
| D-1 artifact is `/data-derived/` with per-run `vintages` provenance | Yes | |
| D-1 "EXPORT_URL live / EXPORT_DIR fixture" switch | Yes, and now with no third silent branch | The synthetic fixture is a named, ranked-last opt-in |
| D-2 two-state semaphore + ops-only staleness alert | Yes | |
| D-2 "Publish once after a cycle in which at least one series published" | **Superseded, in the open** | Widened to "recorded a terminal run outcome the artifact must reflect"; `design.md:106` carries the resolution and the spec quotes the superseded clause verbatim |
| D-3 status P/D/W + nullable verbatim `source_status` | Yes | |
| D-4 per-segment cadence tag on the quarterly grid | Yes | |
| D-5 one shared geometry module for static + island | Yes | Markup still duplicated across the two renderers; parity test remains unwritten, self-disclosed |
| D-6 dual explicit token sets, five stock scales zeroed | Yes | |
| Open Questions kept current | Yes | |
| CRITICAL-15's resolution recorded in design.md | No | It is documented thoroughly in `Dockerfile`, `.dockerignore`, `loader.ts`, `env.example` and `docs/deploy.md`, which is where an operator looks; noted for completeness, not raised as a finding |

### TDD Compliance

| Check | Result | Details |
|-------|--------|---------|
| TDD Evidence reported | Pass | Tables across all slices; reconstructed ones labelled as such |
| All tasks have tests | Pass | 191/191 complete; every RED task names a test file |
| RED confirmed (tests exist) | Pass | Spot-verified `ingest_export_gate_test.go`, `unconfigured-build-fails.test.ts`, `page_state_test.go`, `export_cmd_test.go`, `loader.test.ts`, `indicator-page.container.test.ts` |
| GREEN confirmed (tests pass) | Pass | 23 Go packages, 438 vitest / 33 files, 64 Playwright — all re-executed here |
| Triangulation adequate | Pass | The two new guards each carry matched positive and negative cases: the unconfigured build is paired with an opted-in build that must SUCCEED and emit pages; the export gate carries one exporting kind and two non-exporting kinds |
| Mutation resistance of the new guards | Pass | Bidirectional mutation of the export gate caught, by this verifier, not by report |
| Safety Net for modified files | Pass | Whole suites re-run and green here after my own mutation was reverted |
| End-to-end joining test for the cross-boundary state | **Pass** (was Fail) | `ingest_export_gate_test.go` joins gate, page-state read and artifact writer through real Postgres, reading bytes off disk |
| Runtime evidence exists in the repository | **Fail** | CRITICAL-23: the tests are untracked; no commit contains them |

**TDD Compliance**: 8/9 checks passed.

### Test Layer Distribution

| Layer | Tests | Files | Tools |
|-------|-------|-------|-------|
| Unit + integration (Go, incl. testcontainers) | 23 packages green | ~130 | `go test`, testcontainers-go |
| Unit + container (web) | 438 | 33 | Vitest, `experimental_AstroContainer`, `svelte/server` |
| E2E / browser | 64 | 7 | Playwright, `@axe-core/playwright`, Lighthouse |
| Container / image | 3 builds run by this verifier | — | `docker build`, `docker cp` |

### Changed File Coverage

Skipped — no coverage tool is configured in this repository. Not a failure.

### Assertion Quality

No tautologies in `web/test` or `web/tests`. The two guards added this round are both written so that a
green means something: `unconfigured-build-fails.test.ts` pairs the must-fail case with a must-succeed
case so the failure is pinned to the guard rather than to an unrelated breakage, and it spawns the REAL
`npm run build` rather than mocking Astro, with an in-file note explaining that a mock would have been
written against the same wrong assumption the code held. `ingest_export_gate_test.go`'s no-ref subtest
asserts the stderr text `no active source_ref` to pin the PATH taken, not merely the exit code — without
which it would still pass if the series had quietly reached the network.

One residual: the copy scan's regex blind spot (SUGGESTION-25). It passes today because the components are
clean, which this verifier confirmed independently rather than trusting the scan.

**Assertion quality**: 0 CRITICAL, 0 WARNING, 1 SUGGESTION.

### Quality Metrics

**Linter / formatter**: `gofmt -l .` clean, `go vet ./...` clean.
**Type checker**: `astro check` 0 errors, 0 warnings, 2 hints.

---

### Verdict

**FAIL**

**1 CRITICAL, 1 WARNING, 6 SUGGESTION. 74/74 requirements and 152/152 scenarios compliant in the working
tree.**

The engineering work handed to this pass is good, and the two blockers from pass 2 are genuinely closed —
not narrated as closed.

CRITICAL-15's closure is the stronger of the two. Three independent mechanisms now stand where a silent
fallback used to be: the loader refuses to guess, the Dockerfile pre-check refuses before the build runs,
and `.dockerignore` removes the synthetic fixture from the build context so the code cannot reach it even
if it regresses. This verifier exercised all three, and then closed the evidence gap the writer honestly
disclosed: a real `publishing.Export` run over the recorded INE payloads through a real Postgres
transaction, staged into the build context, built into an image, and the image's own `/web/dist` extracted
and read. It carries INE's real 9.87 for 2026-Q2 and zero occurrences of the fabricated `2002-Q1` value
pass 2 read out of the old build. The writer's hand-derived artifact was not sufficient proof; the
question is now moot because the real proof exists.

CRITICAL-16 was closed by amending the spec rather than the code, which is the harder thing to accept and
the right call here. The superseded clause was not merely awkward — it contradicted two clauses of its own
capability and made another capability's requirement unsatisfiable. The amendment keeps the protective
intent intact (a suspect datum is excluded structurally, by the publish gate writing nothing, not by
withholding the export), preserves the surviving half of the original guarantee as its own falsifiable
scenario, and quotes the text it replaced verbatim. The code keys on `GateBlock`, whose type's zero value
is the empty string, so a fetch failure cannot be mistaken for a rejection. This verifier mutated the gate
in both directions and both mutants died. That is the first time in three passes that a new guard on this
boundary has been shown to be able to fail.

What blocks archive is not a code defect. **Nothing of this change is in the repository.** Five files are
tracked under `web/` against 116 on disk; 191 files are untracked and 46 modified; the last commit is Fase
0's. And the state is not merely incomplete, it is inconsistent: `astro.config.mjs` is tracked and
modified and imports an untracked file, and `ingest_cmd.go` is tracked and modified and imports two
entirely untracked Go packages. This verifier materialised the tracked-files-only tree and measured it —
`go build ./...` fails, `astro check` fails, `astro build` fails, Vitest finds no tests, Playwright finds
no tests. So the most natural commit of this work, `git commit -a`, would put a red `main` in place of a
green one, and no CI run has ever exercised any part of this change on any commit. The remedy is a commit
of the full tree, which is the product owner's action and not something `sdd-apply` can or should do.

WARNING-24 is the pipeline-to-publish boundary once more, one layer below the one just fixed. The image
serves six pages that link to `/data-derived/csv/{slug}.csv` and `/data-derived/series/{slug}.json`; the
image contains no `data-derived/`; those files are written at runtime by a subcommand that nothing in
`docker-compose.yml`, in any workflow or in any document ever starts, because the `app` service takes the
Dockerfile's `CMD ["serve"]` unchanged. It is pre-existing from Fase 0 and no delta-spec requirement
governs deployment wiring, so it does not block archive — but it is the third consecutive pass to find a
defect on that same seam, and it is the reason a deployed site would answer 404 to every data-download
action this change built.

**Not ready to archive.** Exactly one thing blocks it: commit the change. Recommended sequence — commit
the full working tree (all 191 untracked files and 46 modifications, after dealing with SUGGESTION-26's
stray test output so generated bytes do not ride along), let CI run for the first time, and confirm the
suites green on a real runner (which also discharges SUGGESTION-22). Then archive. WARNING-24 and the six
suggestions are legitimate follow-ups; WARNING-24 in particular should be scheduled deliberately rather
than folded into an archive, because a deployed stack that never runs the pipeline is a product-level
question, not a cleanup.

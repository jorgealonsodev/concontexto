```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:9d5e71f36dd9fc9eddee070eb84b884cb11370b1ec9327fcb4d47ba6abfe5e8e
verdict: fail
blockers: 1
critical_findings: 1
requirements: 76/77
scenarios: 167/168
test_command: go test -race -count=1 ./... && npm --prefix web run check && npm --prefix web test && npx --prefix web playwright test --config web/playwright.config.ts
test_exit_code: 0
test_output_hash: sha256:ee2af67f6f920c4fa4791b504a63bee0cc5261c92adc0920a534a89a643acd94
build_command: EXPORT_DIR=data-derived npm --prefix web run build
build_exit_code: 0
build_output_hash: sha256:bd0f5fe9d5959b5b94fbeda348ac3dff430360b6dbd084405fe6f3b88c48bf25
```

# Verification Report — phase-1-indicator-page

**Verifier**: `sdd-verify` (pass 7)
**Date**: 2026-08-04
**Change**: `phase-1-indicator-page`
**Mode**: hybrid (OpenSpec file + Engram `sdd/phase-1-indicator-page/verify-report`), Strict TDD active
**Artifacts read**: proposal, design, 12 capability delta specs, tasks, apply-progress, pass-6 verify-report
**Verified against**: the REPOSITORY (`feat/phase-1-indicator-page` @ `5af95c5`, working tree clean before
and after every probe), the RUNNER, and the RUNNING STACK (`concontexto-app-1`, read-only). Every number
below was re-measured here. Nothing was taken from a session summary or a commit body.

---

## Verdict

**FAIL — one blocker, and it is not code.**

The code is in the best state this change has been in. Pass 6's blocker is genuinely closed: the
acknowledgement is signed by a named human, `ocupados-epa` is in the artifact, and a production build from
the real artifact exits 0 with all seven pages. Every declared command exits 0. The transferred-bytes gate
passes against the real artifact — I ran it, and the worst page is 76.6 KB against a 300 KB budget. All
four areas flagged for hard scrutiny survived adversarial probing, including the pipeline/publish seam,
which is the first pass in seven where that seam has not yielded a defect.

What blocks archive is the record. **17 commits, 112 files and 14,631 added lines have landed since
`tasks.md` and `apply-progress.md` were last updated**, and under Strict TDD that means no TDD Cycle
Evidence exists for the majority of this change's current implementation — including the homepage, the
footer, the whole es-ES formatting layer, the government filter, the event rails, the annotation labels,
and a policy-measures registry that was added and removed without leaving a single task row explaining
either direction. Pass 5 raised this as WARNING-40 at 2 commits and 638 lines. It is now twenty times
larger. Archiving now would freeze a record that describes roughly two fifths of the change it claims to
describe.

The remedy is the same shape as pass 6's: a record update, not a commit to `app/` or `web/`.

---

## Counts

| Dimension | Compliant | Total |
|---|---|---|
| **Requirements** | **76** | **77** |
| **Scenarios** | **167** | **168** |
| Tasks checked | 235 | 235 (0 unchecked) |

Authoritative totals parsed from `openspec/changes/phase-1-indicator-page/specs/*/spec.md` at `5af95c5`:
data-model-vintages 4/8, data-validation 2/16, design-system 9/14, editorial-config 3/9,
indicator-page 15/34, pipeline-operations 4/10, platform-runtime 1/2, publishing-export 12/21,
series-transformations 8/14, source-ingestion-eurostat 3/7, **source-ingestion-ine 6/17** (was 5/13),
web-accessibility-gates 10/16. Pass 6's 76/164 is superseded by the spec's own growth (`026c7fa` adds one
requirement and four scenarios to `source-ingestion-ine`), not by a recount disagreement. `publishing-export`
also grew by ten lines of prose in this window, but they add no requirement and no scenario: they record the
accepted consequence pass 6 filed as WARNING-44.

**Non-compliant requirement (1)**:

| Capability / Requirement | Why |
|---|---|
| `pipeline-operations` / "An ingestion not followed by a rebuild alerts operators" | 3 of 4 scenarios pass. "A failed rebuild raises an alert **immediately**" is substituted by budget-delayed detection: `alerting.go:231` alerts on a failed *dispatch*, and nothing anywhere observes the rebuild's *outcome* — no `workflow_run` receiver, no conclusion poll. WARNING-41, carried unchanged from pass 4. |

**Non-compliant scenario (1)**: `pipeline-operations` / "A failed rebuild raises an alert immediately" — ❌ UNTESTED.

**Newly compliant this pass**: `indicator-page` / "Six indicator routes with frozen slugs" and its scenario
"All six routes exist in the build". Measured, not assumed — see §A.

---

## A. Execution evidence — every number re-measured here

Working tree clean at `5af95c5` before and after every mutation probe (`git status --porcelain` → 0 lines,
verified after each). Go and Playwright ran sequentially, never concurrently.

| Command | Exit | Result |
|---|---|---|
| `go build ./...` | 0 | clean |
| `go vet ./...` | 0 | clean |
| `gofmt -l app/` | 0 | no output |
| `go test -race -count=1 ./...` | **0** | 22 packages ok + 2 with no test files, zero race reports |
| `validate-config` | 0 | `validate-config: ok` |
| `npm run check` (astro check) | 0 | 131 files, 0 errors, 0 warnings |
| `npm test` (Vitest) | 0 | **799 passed / 799**, 52 files |
| `EXPORT_DIR=data-derived npm run build` | **0** | **7 pages**: six `/indicador/{slug}/` + `/` |
| `npx playwright test` | 0 | **191 passed / 191** |
| `npm run budget:lighthouse` (against the **real-artifact production build**) | **0** | all six pages PASS |

**The transferred-bytes gate, run here against the real artifact rather than the synthetic fixture.** CI
runs this gate on a `BUILD_WITH_SYNTHETIC_FIXTURE=1` build and says so in its own comments. I built `dist/`
from `web/data-derived` (the artifact the pipeline actually wrote, `generated_at`
2026-08-04T18:42:18Z) and ran the same gate against that:

```
PASS /indicador/tasa-de-paro-epa: 57.4 KB transferred (excluding the typeface), budget 300 KB
PASS /indicador/ocupados-epa: 59.4 KB
PASS /indicador/ipc-general: 76.6 KB
PASS /indicador/ipc-subyacente: 76.6 KB
PASS /indicador/pib: 61.8 KB
PASS /indicador/poblacion-residente: 60.3 KB
```

This matters because `6556f0e` made the page render **two** SVG geometries server-side (wide and narrow).
That doubles the largest single payload on the page, and the budget is a blocking spec requirement. It
holds with a 3.9x margin on the worst page.

**The artifact is real and complete.** `web/data-derived/manifest.json` declares ten series with sha256
digests; `ocupados-epa.json` is present with 98 points. Every one of the six frozen route slugs resolves,
including the `pib` → `pib-cvi` alias (`dist/indicador/pib/index.html` links
`/data-derived/csv/pib-cvi.csv`).

**Editorial reconciliation reaches the artifact.** Every series document now carries `events: 10`, and
`tasa-de-paro-epa`, `ocupados-epa`, `ipc-general` and `ipc-subyacente` each carry `breaks: 1`. Before
`a7c3739` these arrays were empty in the deployed stack. This is the fix landing, measured in the bytes.

**Rendered output agrees with the artifact arithmetic.** `ocupados-epa` last point is `2026-Q2 = 22779`,
unit `miles de personas`. The page renders `22.779` (es-ES grouping), period `T2 2026`, YoY `+2,3%`
(computed 2.29% from 22268.7) and intra-annual `+2,2%` (computed 2.18% from 22293). No un-grouped `22779`
leaks into the reader-facing markup. The thousand-fold error is closed.

**Machine surfaces are byte-unchanged by the Spanish period work.** `data-derived/csv/ocupados-epa.csv`
ends `2026-Q2,22779,D,Definitivo,1`; the JSON carries `['2025-Q4','2026-Q1','2026-Q2']`. Only the reader
surface says `T2 2026`.

---

## B. The four areas flagged for hard scrutiny

### B.1 The add-then-remove of the policy-measures layer — CLEAN

`610290a` added 2,559 lines across Go, config, migrations, Zod, chart and tests. `5af95c5` removed it.
Five files were genuinely deleted, and nothing survives them:

```
D  app/internal/adapters/config/measures_test.go
D  config/medidas.yaml
D  web/src/lib/chart/measureMarks.ts
D  web/test/chart/measureMarks.test.ts
D  web/tests/e2e/workbench/chart-policy-measures.spec.ts
```

A tree-wide search for `measure|medida` in Go, TS, Astro, YAML, SQL and JSON returns **zero** references to
the removed concept — every hit is an unrelated English "measured". `web/src/` carries no
`policyMeasure`, `measureMark`, `selectPolicyMeasures` or `medidas` identifier. `astro check` is clean over
131 files, so no orphaned import or type survives. The single `methodology-measures` test id on the live
pages is `MethodologySheetFields.astro:70` — the methodology sheet's *medidas de la serie* field, which
predates the registry and is unrelated.

**The in-range filtering fix survived intact.** This was the specific risk: `578bb86` corrected a real
defect (`governments` and `exogenous` chips listing entries outside the visible window) one commit before
the revert. Diffing `578bb86..HEAD` for `web/src/lib/chart/annotationWindow.ts` after stripping comment
lines yields **zero functional changes** — the revert edited only the module's prose to stop naming the
removed third selector. Both consumers still read the one rule:
`IndicatorChart.astro:212` and `ChartIsland.svelte:747` both call `annotationsInWindow`.

**What the revert also carried, and it is worth naming**: `5af95c5` is not a pure revert. It additionally
adds `web/src/lib/chart/tableSummary.ts`, `web/test/chart/tableSummary.test.ts`,
`web/test/design-system/data-table-disclosure-parity.test.ts`,
`web/tests/e2e/indicator/indicator-data-table.spec.ts` and
`app/internal/adapters/config/event_scope_test.go` — i.e. the whole `<details>` disclosure feature and the
event-scope retention tests are bundled into a commit whose subject line says "remove". That is a review
hazard, not a defect: everything it adds is tested and green. Recorded as SUGGESTION-51.

### B.2 The `<details>` reading of the no-JS requirement — SOUND, and better defended than the requirement asks

Two requirements govern this, and both use the same phrase:

- `indicator-page` / "The page works with JavaScript disabled": *"...MUST all be present and legible"*;
  its scenario's THEN reads *"...are all present"* and *"no empty placeholder or loading state is shown"*.
- `web-accessibility-gates` / "The no-JavaScript baseline is a blocking gate": same wording, same scenario
  shape.

The apply phase's reading — that the scenario asserts presence — is correct as far as it goes, but it is
not what makes this compliant. What makes it compliant is that `<details>` is a **browser primitive**, so
"legible" costs a reader with no script exactly one activation and zero bytes, and that is asserted in a
context where no script runs at all:

`tests/e2e/indicator/indicator-pages-no-js.spec.ts` runs under `test.use({ javaScriptEnabled: false })`
and, per slug, asserts the table is in the document **with rows** (`toHaveCount(1)` plus
`tbody tr > 0` — strictly stronger than the bare `toBeVisible()` it replaced, which could not distinguish a
rendered table from a rendered empty one), that the summary is visible, and then in a **separate test**
that the disclosure opens by pointer *and* by Enter, and closes again on the second Enter. Nothing but the
browser's own behaviour can make those pass.

**`aria-describedby` resolves while closed — asserted, per slug, in both states.**
`tests/e2e/indicator/indicator-data-table.spec.ts` reads `svg[aria-describedby]`, splits the id list, and
asserts every id resolves via `document.getElementById` **while the disclosure is closed**, then asserts
the table's own id is among them, then re-checks after opening. The accname carve-out the code comment
cites (a hidden node directly referenced by `aria-describedby` is still included) is real, and the test does
not rely on it being real — it checks the reference is not dangling, which is the failure mode a disclosure
implemented by *removing* the table would have introduced.

**Two renderers, one shape.** The table exists twice — `AccessibleDataTable.astro:91` (static/no-JS) and
`ChartIsland.svelte:1332` (hydrated) — with identical `<details data-testid="accessible-data-table-details">`
/ `<summary class="min-h-11 ...">` markup. This is exactly the "two renderers of the same table" class this
session has been bitten by, and it is now guarded by a dedicated
`web/test/design-system/data-table-disclosure-parity.test.ts`.

**Adjudication: COMPLIANT.** I record no finding. The judgement call was made, argued in the source, and
then tested from the side that could disprove it.

### B.3 Migration 0007's columns are not dead — the claim HOLDS, verified by mutation

The claim to check was that `scope_kind` is `NOT NULL` with every row `'global'`, that `ListActiveEvents`
reads it on every export, and whether a column no configuration can populate is honest schema.

Confirmed, and the last clause of the claim is the one that is wrong in the *pessimistic* direction:
configuration **can** populate it.

- `0007_event_scope.up.sql:52` — `scope_kind text NOT NULL DEFAULT 'global'`, `scope_ref text NOT NULL
  DEFAULT ''`, `source_url text`, plus a partial index.
- `events_read.go:62-70` — `ListActiveEvents` executes
  `WHERE retired_at IS NULL AND (scope_kind='global' OR (scope_kind='series' AND scope_ref=$1) OR ...)`
  on **every** series on **every** export. The parameter it used to accept and not read is now load-bearing.
- `EventConfig.Scope` (`types.go:129`) is a real YAML field; the loader normalises an omitted scope to
  `global`; `Validate` calls `validateEventScope` for every event (`validate.go:54`).
- `config/eventos.yaml` and `config/gobiernos.yaml` declare no scope today, so every shipped row is
  `'global'` — the honest value for a change of government and a worldwide shock.

I mutated `config/eventos.yaml` four ways and ran `validate-config` on each:

| Mutation | Result |
|---|---|
| `scope: { kind: series, ref: no-such-series }` | **exit 1** — `scope.ref "no-such-series" does not resolve to any configured series` |
| `scope: { kind: galaxia, ref: x }` | **exit 1** — `scope.kind "galaxia" is not one of global, series, dataset, source` |
| `scope: { kind: series }` (no ref) | **exit 1** — `scope.kind "series" requires a scope.ref naming what it applies to` |
| `scope: { kind: series, ref: ocupados-epa }` (valid) | **exit 0** — accepted |

Persistence and read-back are covered by `TestReconcileEvents_PersistsScopeAndSourceURL`,
`TestListActiveEvents_ResolvesScopeForTheSeriesAsked` and
`TestListActiveEvents_UnknownSeriesStillResolvesGlobalEntries`.

**Verdict: keeping 0007 after the revert was correct.** These are not dead columns — they are a live,
validated, exercised read path that today carries one value because one value is the truth. Reverting them
would have restored the documented defect `ListActiveEvents` used to admit in its own doc comment.
One residual, recorded as SUGGESTION-52: `source_url` on events is wired end-to-end
(YAML → reconcile → column → `EventRef.SourceURL` → Zod `EventRefSchema`) and unit-tested, but no shipped
config entry populates it, so the key never appears in a production artifact.

### B.4 The pipeline/publish seam, seventh look — HOLDS

Six passes found a defect here every time. This one did not, and I looked at it three ways.

**The reconcile wiring, mutation-tested twice.** `a7c3739` wired `ingest --reconcile` into the deployed
stack as a one-shot service the `app` service waits on. I broke it two ways:

| Mutation to `docker-compose.yml` | Result |
|---|---|
| Delete `reconcile: condition: service_completed_successfully` from `app.depends_on` | **RED** — `TestDockerComposeReconcile_RunsAfterMigrationsAndGatesTheApp` fails naming the exact condition and why it matters |
| Change `command: ["ingest","--reconcile"]` to `command: ["ingest"]` | **RED** — `TestDockerComposeReconcile_TheCommittedCommandProjectsEditorialRows` fails with the binary's own usage on stderr |

The second is the strong one: that test **executes the committed command against a real database** and
asserts editorial rows land. It cannot pass by reading YAML that happens to look right.

**The footer's two runtime links.** `/transparencia/raw-files.sha256` and `/data-derived/**` are written by
the Go binary into mounted volumes, not by `astro build`, so `dist/` never contains them and the e2e suite
cannot assert they return 200. `tests/e2e/footer/site-footer.spec.ts` says so in its own header rather than
pretending otherwise, and the href is pinned **against the writer's Go source**:
`test/pages/site-footer.test.ts:196` regex-reads `publicHashPath = filepath.Join(...)` out of
`app/cmd/concontexto/ingest_cmd.go` and asserts it contains `"transparencia"` and `"raw-files.sha256"`,
with an explicit guard that fails if the regex stops matching ("this guard has gone blind"). That is the
CRITICAL-1 pattern applied prospectively. On the running stack both paths return 200.

**Two derivations of the same route list — collapsed to one.** `src/pages/index.astro` and
`src/pages/indicador/[slug].astro` both resolve through `resolveIndicatorRouteSlugs` (via
`lib/indicator/homeListing.ts` for the homepage). The built `dist/index.html` links exactly the six frozen
slugs. There is no second list to drift.

**Residual, unchanged and disclosed by CI itself**: the axe/no-JS/keyboard/44px gates run against a
`BUILD_WITH_SYNTHETIC_FIXTURE=1` build. `ci.yml:126-136` states this and states which workflow verifies a
build against a real artifact. I closed the **budget** half of that gap myself this pass (§A) by running
the gate against the real-artifact build; the accessibility half remains fixture-only. SUGGESTION-53.

---

## C. Strict TDD

### TDD Compliance
| Check | Result | Details |
|-------|--------|---------|
| TDD Evidence reported | ⚠️ | Table present in `apply-progress.md`, but it stops at Slice 19. Nothing after `823311e` has a row. |
| All tasks have tests | ✅ | 235/235 recorded tasks map to test files that exist |
| RED confirmed (tests exist) | ✅ | Every test file named by the recorded slices exists at `5af95c5` |
| GREEN confirmed (tests pass) | ✅ | Full suite exit 0; 517 Go test funcs, 799 Vitest, 191 Playwright |
| Triangulation adequate | ✅ | e.g. `TestDecodeSeries_NullValorFailsClosedUnderEveryTipoDatoToken` runs 4 sub-cases |
| Safety Net for modified files | ✅ | For recorded slices |
| **Evidence covers the current tree** | ❌ | **17 commits / 14,631 added lines have no TDD row at all** — CRITICAL-46 |

**TDD Compliance**: 6/7 checks passed.

### Test Layer Distribution
| Layer | Tests | Files | Tools |
|-------|-------|-------|-------|
| Unit / integration (Go) | 517 test funcs | 137 | `go test -race` |
| Unit / container (Vitest) | 799 | 52 | vitest 4 |
| E2E (Playwright) | 191 | 11 | @playwright/test + @axe-core/playwright |

26 of those test files were added in the 17 unrecorded commits.

### Assertion Quality
Scanned every test file in `web/test`, `web/tests` and `app/`.

- Tautologies (`expect(true).toBe(true)`, `assert.True(t, true)`): **zero**.
- Assertions with no production-code call: **zero found**.
- Smoke-test-only (`render` + `toBeInTheDocument`): **zero** — the pattern is not used in this codebase.
- Mock-heavy files (`vi.mock` > 2x `expect`): **zero** — `vi.mock` is not used at all.
- Type-only assertions used alone: none; every `toBeDefined()` occurrence sits beside a value assertion.
- Ghost loops: four candidates. Three (`favicon.test.ts:99`, `svg.test.ts:105/110`) iterate collections a
  sibling assertion in the same file proves non-empty. The fourth (`svg.test.ts:431`) is examined by
  mutation in SUGGESTION-49 and is safe in practice.

**Assertion quality**: 0 CRITICAL, 0 WARNING, 1 SUGGESTION.

### Quality Metrics
**Linter / type checker**: `astro check` — 0 errors over 131 files. `go vet` — clean. `gofmt -l app/` — no output.
**Coverage**: no coverage tool is configured in either stack. Coverage analysis skipped — not a failure.

---

## D. The signature, attacked

The whole pass-6 blocker turned on this record, so I did not read it — I broke it.

`config/reconocimientos.yaml` ships one record, `ocupados-epa-2020-q2-covid`, signed
`acknowledged_by: "jorgealonsodev"`, `acknowledged_on: 2026-08-04`, pinning INE's published 18607.2 with a
`source_url` to the INE press release.

| Mutation | `validate-config` |
|---|---|
| `acknowledged_by: "Claude (agente), bajo autoridad delegada — no es una firma"` (the exact original fabrication) | **exit 1** — *"is not a signature. Name the human who reviewed the datum, or declare `signature_status: \"unsigned\"`..."* |
| `acknowledged_by: "TODO"` | **exit 1**, same rule |
| `acknowledged_by: "Jorge Alonso"` (an ordinary human name) | **exit 0** — accepted, so the guard is not simply rejecting everything |

And the inverted test is real. Replacing the signature with a properly-declared
`signature_status: "unsigned"` + `drafted_by` + `todo` leaves `validate-config: ok` (an unsigned record is a
legal declared state — it just resolves nothing) while
`TestRealAcknowledgementRegistry_ShipsExactlyOneRecordSignedByARealHuman` **fails with five distinct
assertions**, including *"acknowledged_by is empty: a record with no signer carries no authority and must
never ship signed"*. CI goes red on the un-signing, which is precisely the guard that was missing when the
fabrication happened.

**CRITICAL-37 is CLOSED, both halves.**

---

## E. Prior findings, re-adjudicated

| ID | Summary | Status this pass | Evidence |
|---|---|---|---|
| CRITICAL-1 | Every "Exportar CSV" link 404s | **CLOSED** | Unchanged; `dist/indicador/pib/index.html` links `/data-derived/csv/pib-cvi.csv`, which exists |
| CRITICAL-2 | The blocking budget gate cannot fail | **CLOSED** | Re-verified: I ran the gate against a real-artifact build and it reported six real measurements |
| CRITICAL-3 | 37 inlined Spanish strings | **CLOSED** | Unchanged |
| CRITICAL-4 | Export artifact carries no page-state | **CLOSED** | Unchanged; `pageState` present on all ten series docs |
| CRITICAL-15 | Container publishes synthesised fixture as INE statistics | **CLOSED** | Unchanged |
| CRITICAL-16 | Failed-ingestion publish path untested | **CLOSED** | Unchanged |
| CRITICAL-23 | The change does not exist in the repository | **CLOSED** | Clean tree at `5af95c5` |
| CRITICAL-27 | Build silently drops a frozen slug | **CLOSED** | Unchanged |
| CRITICAL-28 | Publish loop open at four links | **CLOSED** | Unchanged |
| **CRITICAL-37** | Change cannot produce a deployable site | **CLOSED, both halves** | §D and §A: signed record, `ocupados-epa` in the artifact, build exit 0 with 7 pages |
| WARNING-29 | Run log claims a dispatch that did not happen | **CLOSED** | Unchanged |
| WARNING-31 | `SeverityBlockRequiresSignoff` names a mechanism that does not exist | **CLOSED** | Unchanged |
| WARNING-5,6,7,8,9,10,11,17,18,24 | (pass 2/3) | **CLOSED** | Unchanged |
| **WARNING-30** | INE nil-value crash class disclosed only in a commit message | **CLOSED** | `026c7fa` turns it into a spec requirement with 4 scenarios and 4 named passing tests; `envelope.go:87-146` carries the reasoning and the refusal |
| **WARNING-38** | One acknowledgement resolves more than one finding | **OPEN** | Unchanged; not reachable in the shipped config |
| **WARNING-39** | The registry's only anti-forgery control is a review gate with no second reviewer | **OPEN** | `.github/CODEOWNERS:17` still reads `/config/** @jorgealonsodev @TODO-second-config-reviewer`. The mechanism exists; the second pair of eyes does not. Materially relevant now that a human signature is the thing being protected |
| **WARNING-40** | `apply-progress.md` records none of the recent commits | **ESCALATED → CRITICAL-46** | Grown from 2 commits / 638 lines to **17 commits / 112 files / 14,631 added lines** |
| **WARNING-41** | "A failed rebuild raises an alert **immediately**" is substituted | **OPEN** | Re-verified: `alerting.go:231` alerts on failed *dispatch* only; no `workflow_run` receiver or conclusion poll exists anywhere |
| **WARNING-44** | A frozen permalink's two download links 404 on the running stack | **CLOSED** | Both `/data-derived/csv/ocupados-epa.csv` and `.../series/ocupados-epa.json` return 200. The condition was always transient-until-republish, and the series republished. The accepted consequence is now written into `publishing-export/spec.md:146-155` rather than left in a code comment |
| **WARNING-45** | The prune's ordering has no test | **OPEN** | No test in `app/internal/publishing/*_test.go` asserts the prune runs after the manifest rename |
| SUGGESTION-19 | Workbench CSV href layout | **OPEN** | Unchanged |
| SUGGESTION-20 | Spec silent on the dateless validation banner | **OPEN** | Unchanged |
| SUGGESTION-21 | No custom-range e2e on a real route | **CLOSED** | `tests/e2e/indicator/indicator-annotation-range.spec.ts` now exercises range selection on real indicator routes |
| SUGGESTION-22 | Workflows never observed on a runner | **OPEN, re-opened** | Green at `5310586`; the 17 commits since have not been observed on a runner by this pass |
| SUGGESTION-25 | Copy-scan regex blind spot | **OPEN** | Unchanged |
| SUGGESTION-26 | Test output inside the source tree | **CLOSED** | Unchanged |
| SUGGESTION-32 | `befa81f` commit body overstates | **OPEN** | Unchanged |
| SUGGESTION-33 | Container bring-up has no automated coverage | **OPEN** | Unchanged |
| SUGGESTION-34 | Record drift in `openspec/config.yaml` | **OPEN** | `openspec/config.yaml:15/92/94` still declares `go test ./...` while CI and this verification run `-race -count=1` |
| SUGGESTION-35 | Four-eyes documented, unenforced | **ESCALATED** (pass 5) → tracked as WARNING-39 |
| SUGGESTION-42 | Neighbouring-period residual narrower than disclosed | **OPEN** | Unchanged |
| SUGGESTION-43 | `smoke-test.sh` usage omits `EXPORT_DIR`/`EXPORT_URL` | **OPEN** | Unchanged |

---

## F. New findings, pass 7

### CRITICAL-46 — The SDD record is 17 commits and 14,631 lines behind the code

`tasks.md` and `apply-progress.md` were last written by `823311e`, which recorded Slices 18-19. Every commit
after it is unrecorded:

```
git log --oneline 823311e..HEAD | wc -l   →  17
git diff --shortstat 823311e..HEAD        →  112 files changed, 14631 insertions(+), 425 deletions(-)
git log -1 --oneline -- openspec/changes/phase-1-indicator-page/tasks.md          →  823311e
git log -1 --oneline -- openspec/changes/phase-1-indicator-page/apply-progress.md →  823311e
```

`tasks.md` reads 235/235 and the last section header is `## Slice 19`. The 235 tasks are genuinely all
checked and their tests genuinely all pass — that part of the record is sound. What is absent is any record
of the work that followed it:

- the homepage and back-link (`ac69a29`), the site footer (`9af3f86`), the favicon (`6556f0e`)
- the whole es-ES number layer and the unit-agreement guard (`52b7abe`)
- Spanish period labels with machine-surface parity (`0c40097`)
- editorial reconciliation wired into the deployed stack (`a7c3739`) — a production defect fix
- the government range filter (`ff2ea4f`), change markers (`154824f`), event-period rails (`d5cfbed`),
  on-drawing labels (`e1db0ea`), in-window annotation filtering (`578bb86`)
- the signature and its inverted guard (`4e11378`) — **the fix for pass 6's own blocker**
- a policy-measures registry added (`610290a`) and removed (`5af95c5`), with migration `0007` retained on
  an argument that appears nowhere in the change record

26 new test files landed in that window with no TDD Cycle Evidence row. Under Strict TDD that is the
primary artifact this phase is required to validate, and for the majority of the current implementation it
does not exist.

Two further consequences worth stating rather than leaving to be discovered:

1. **The review workload guard is unmeasurable.** 14,631 authored added lines against a 400-line default
   PR budget is 36x, delivered as one unsliced block with no deliverable work units recorded.
2. **The 0007-retention decision has no durable home.** The reasoning for keeping the migration after
   reverting the feature that motivated it currently lives in a commit body and a SQL comment. It is
   correct (§B.3) — and it is exactly the class of decision that decays when it is not in the record.

**This blocks archive.** Archiving freezes the change record; freezing this one would file a description of
roughly two fifths of the change. The remedy is a record update, not a code change.

### WARNING-47 — Two reader-facing surfaces ship with no spec requirement

The homepage (`/`) and the site footer are implemented, tested (`test/pages/home.container.test.ts`,
`test/pages/site-footer.test.ts`, `tests/e2e/home/home.spec.ts`, `tests/e2e/footer/site-footer.spec.ts`)
and shipping. No requirement in any of the twelve delta specs, and none in the ten baseline capabilities
under `openspec/specs/`, governs either. A grep for `homepage|página de inicio|site footer|pie de página`
across `openspec/` matches only `design.md`, `apply-progress.md` and the verify reports — never a spec.

Consequences: neither surface appears in the compliance matrix, so their behaviour is asserted against
tests alone with no requirement to adjudicate against; and `ingest-export-build.yml`'s real-artifact
assertion loop iterates the six indicator slugs only, so `/index.html` is never checked to exist in a
real-artifact build (it does — I verified — but nothing enforces it).

The footer is the more consequential of the two: it deliberately asserts the **absence** of a blanket data
licence. That is an editorial claim about redistribution rights, made on every page, with no requirement
stating it and no scenario pinning it.

### SUGGESTION-49 — One ghost loop without its own non-empty guard

`web/test/chart/svg.test.ts:431` asserts halo attributes inside
`for (const [, label] of svg.matchAll(/<text class="chart-annotation-label[^>]*>/g))` with no assertion that
the collection is non-empty. I mutation-tested both directions:

- Changing `paint-order="stroke"` → `"normal"`: the halo test goes **RED**. Non-vacuous for its subject.
- Renaming the class so the collection empties: **7 tests in the same file fail**, so the vacuous state
  cannot pass unnoticed at suite level.

Safe in practice; one `expect(labels.length).toBeGreaterThan(0)` would make it safe by construction.

### SUGGESTION-51 — A commit named "revert" adds a feature

`5af95c5` removes the measures layer *and* introduces the `<details>` disclosure, `tableSummary.ts`, a
parity test, a new e2e spec and the event-scope retention tests. Everything it adds is tested and green,
but a reviewer reading the subject line will not expect a new reader-facing interaction inside it.

### SUGGESTION-52 — `event.source_url` is wired end-to-end and populated by nothing

The column, the reconcile write, `EventRef.SourceURL` and the Zod `EventRefSchema` optional all exist and
are unit-tested (`TestReconcileEvents_PersistsScopeAndSourceURL`). No entry in `config/eventos.yaml` or
`config/gobiernos.yaml` sets `source_url`, so the key is `omitempty`-absent from every shipped artifact and
the JSON leg of the path has no production exercise.

### SUGGESTION-53 — The accessibility gates still measure the fixture, not the artifact

`ci.yml`'s Playwright job builds with `BUILD_WITH_SYNTHETIC_FIXTURE=1`; axe, keyboard traversal, 44px and
the no-JS context therefore never run against the artifact the pipeline writes. CI discloses this in its
own comments and names `ingest-export-build.yml` as the workflow that verifies a build against a real
artifact — but that workflow asserts only route existence and one rendered value, not accessibility. The
budget half of this gap is closed for this pass by §A; the accessibility half is not, and it is closable
in the same workflow that already produces a real artifact.

---

## G. Spec compliance — what moved

| Requirement | Scenario | Test | Result |
|---|---|---|---|
| `indicator-page` / Six indicator routes with frozen slugs | All six routes exist in the build | `EXPORT_DIR=data-derived npm run build` → exit 0, 7 pages; `scripts/assert-blocked-series-fails-build.sh` control arm | ✅ COMPLIANT *(was ❌)* |
| `source-ingestion-ine` / A null Valor has no decided meaning | A null value with a definitive token fails closed | `TestDecodeSeries_NullValorFailsClosedRatherThanEmittingANilValuedObservation` | ✅ COMPLIANT *(new)* |
| " | The refusal does not depend on the accompanying status token | `TestDecodeSeries_NullValorFailsClosedUnderEveryTipoDatoToken` (4 sub-cases) | ✅ COMPLIANT *(new)* |
| " | A published zero is a value, not a missing one | `TestDecodeSeries_ZeroIsAPublishedValueNotAMissingOne` | ✅ COMPLIANT *(new)* |
| " | Every decoded observation carries a value | `TestDecodeSeries_EveryDecodedObservationCarriesANonNilValue` | ✅ COMPLIANT *(new)* |
| `series-transformations` / Applicability matrix | Controls match the matrix exactly | Rendered `dist/`: `perCapita` on `ocupados-epa` and `pib` only; `yoy`+`qoq` on all six; no real/median anywhere | ✅ COMPLIANT *(re-verified)* |
| `indicator-page` / The page works with JavaScript disabled | The no-JS baseline is complete | `indicator-pages-no-js.spec.ts` (6 slugs) + `indicator-data-table.spec.ts` disclosure and `aria-describedby` tests | ✅ COMPLIANT *(re-adjudicated, §B.2)* |
| `indicator-page` / Under 300 KB transferred | The default-range page stays within budget | `npm run budget:lighthouse` against the real-artifact build: worst page 76.6 KB | ✅ COMPLIANT *(re-measured)* |
| `pipeline-operations` / An ingestion not followed by a rebuild alerts operators | A failed rebuild raises an alert immediately | (none found) | ❌ UNTESTED |

**Compliance summary**: 167/168 scenarios compliant.

---

## H. Verdict

**FAIL.**

One blocker: **CRITICAL-46** — the SDD record omits 17 commits and 14,631 added lines, so no Strict-TDD
evidence exists for most of the change's current implementation, and archive would freeze a record that
does not describe the change.

Everything else is in good order. The pass-6 blocker is closed and closed properly. Every declared command
exits 0. All four hard-scrutiny areas held under adversarial probing — the measures revert is complete and
took nothing it should not have, the `<details>` reading is sound and better tested than the requirement
demanded, migration 0007's columns are demonstrably live and populatable, and the pipeline/publish seam
survived a seventh look for the first time.

**Not ready to archive.** What blocks it: bring `tasks.md` and `apply-progress.md` up to `5af95c5` with
slice sections and TDD Cycle Evidence rows for the 17 unrecorded commits, and record the reasoning for
retaining migration 0007 after the revert. That, plus a spec decision on WARNING-47's two unspecced
surfaces, is the whole distance to archive.

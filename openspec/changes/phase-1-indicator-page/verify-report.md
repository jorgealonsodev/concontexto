```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:a8578310d10612241a7ae7283853768c3bc61abc78bcb0b047c67b9433f7b41a
verdict: fail
blockers: 1
critical_findings: 1
requirements: 74/76
scenarios: 162/164
test_command: cd app && go test -race -count=1 ./... && cd .. && npm --prefix web run check && npm --prefix web test && npm --prefix web run test:e2e
test_exit_code: 0
test_output_hash: sha256:bdf3a679ad6efaa67fd84eb32676f811928ca069b81dee7bbbff4968764cfda6
build_command: EXPORT_DIR=/tmp/user/1000/claude-1000/-mnt-480GB-Proyectos-Personales-20260728-concontexto-20260728-concontexto-git-concontexto/b5d27279-aa14-400b-8a04-6da53081997f/scratchpad/blocked npm --prefix web run build
build_exit_code: 1
build_output_hash: sha256:ea2809b9968cd2c111a34b8fb699e5958b3e042a09d7ed03d9a0467bde952999
```

# Verification Report — phase-1-indicator-page

**Verifier**: `sdd-verify` (pass 6)
**Date**: 2026-07-30
**Change**: `phase-1-indicator-page`
**Mode**: hybrid (OpenSpec file + Engram `sdd/phase-1-indicator-page/verify-report`), Strict TDD active
**Artifacts read**: proposal, design, 12 capability delta specs, tasks, apply-progress, pass-5 verify-report
**Verified against**: the REPOSITORY (`feat/phase-1-indicator-page` @ `5310586`, working tree clean),
the RUNNER (PR #1, four jobs green at `head_sha 5310586`) and the RUNNING STACK (`concontexto-app-1`,
read-only). Every number below was re-measured here. Nothing was taken from a session summary.

---

## Verdict

**FAIL — one blocker, and it is still not code.**

`CRITICAL-37`'s **structural half is genuinely closed**, and closed better than it was asked to be. CI can
now go red for a real threshold blocking a real series: I proved it by mutation, four ways, not by reading
the assertions. The prune (`5310586`) is a correct fix to a real, live, previously-unrequirement-covered
data-integrity defect, its scope is tight, and its three new scenarios all have tests that fail when the
code is broken.

What blocks archive is unchanged and is the same single fact: `ocupados-epa` is blocked by
`rule3-plausibility`, the record that would resolve it is correctly unsigned, and a production build
therefore **exits 1 and emits nothing**. I ran that build at `5310586`. The change's central MUST — six
indicator routes — cannot be satisfied by any artifact this pipeline can currently produce.

Three new WARNINGs come out of attacking the prune, and one of them is observable on the running stack
right now.

---

## Counts

| Dimension | Compliant | Total |
|---|---|---|
| **Requirements** | **74** | **76** |
| **Scenarios** | **162** | **164** |
| Tasks checked | 219 | 219 (0 unchecked) |

Authoritative totals parsed from `openspec/changes/phase-1-indicator-page/specs/*/spec.md` at `5310586`:
data-model-vintages 4/8, data-validation 2/16, design-system 9/14, editorial-config 3/9,
indicator-page 15/34, pipeline-operations 4/10, platform-runtime 1/2, **publishing-export 12/21** (was
11/18; `5310586` adds one requirement and three scenarios), series-transformations 8/14,
source-ingestion-eurostat 3/7, source-ingestion-ine 5/13, web-accessibility-gates 10/16. Pass 5's totals
(75/161) are superseded by the spec's own growth, not by a recount disagreement.

**Non-compliant requirements (2)** — the same two as pass 5:

| Capability / Requirement | Why |
|---|---|
| `indicator-page` / "Six indicator routes with frozen slugs" | The build produces **zero** of six from the only artifact production can supply. CRITICAL-37. |
| `pipeline-operations` / "An ingestion not followed by a rebuild alerts operators" | 3 of 4 scenarios pass; "A failed rebuild raises an alert **immediately**" is substituted by budget-delayed detection. WARNING-41. |

**Non-compliant scenarios (2)**: `indicator-page` / "All six routes exist in the build";
`pipeline-operations` / "A failed rebuild raises an alert immediately".

The new `publishing-export` requirement is **3/3 scenarios compliant** — see §D.

---

## A. Execution evidence — every number re-measured here

Working tree clean at `5310586` (`git status --porcelain` → 0 lines). Go and Playwright ran sequentially,
never concurrently.

| Command | Exit | Result |
|---|---|---|
| `go build ./...` | 0 | clean |
| `go vet ./...` | 0 | clean |
| `gofmt -l app/` | 0 | no output |
| `go test -race -count=1 ./...` | **0** | 22 packages ok + 2 with no test files, zero race reports |
| `npm run check` | 0 | 0 errors, 0 warnings, 2 hints |
| `npm test` (Vitest) | 0 | **448 passed / 448**, 35 files |
| `npm run test:e2e` (Playwright) | 0 | **64 passed / 64** |
| `scripts/assert-blocked-series-fails-build.sh <blocked> <honest>` | **0** | blocked build exit 1 naming `ocupados-epa`; honest build exit 0, all six routes |
| `npm run build` from the **real blocked artifact** | **1** | **no `indicador/` directory emitted** |

**Runner.** PR #1 OPEN and MERGEABLE at `headRefOid 5310586`. All four jobs pass at that sha: `Go test
suite` (1m25s), `Frontend build and tests` (3m22s), `Container smoke test` (1m51s), `Ingest fixtures ->
export -> astro build` (1m21s). Read from `gh pr checks`, not from the workflow files.

**Full CI chain reproduced locally.** I ran both e2e exports into two directories and then the matched-pair
script, exactly as `ingest-export-build.yml` does. Output verbatim:

```
=== the blocked artifact, before any build runs
exactly one frozen slug is absent (ocupados-epa); the other five are present.
=== the blocked artifact MUST fail the build
PASS: astro build exited 1, on the frozen-route guard, naming ocupados-epa.
=== the honest artifact MUST still build (the control)
PASS: the same build, same command, exits 0 on the honest artifact and emits all six routes.
```

---

## B. CRITICAL-37's structural half — closed, and proven by mutation

The question asked was: *can CI now go red when a real threshold blocks a real series, and is the new
assertion itself able to fail?* Both halves are **yes**, and I did not take the commit message's word for
it. Four mutations, each applied to the real tree, each reverted:

| Mutation | Result |
|---|---|
| `ineIngestConfig` reverted to `config.ValidationConfig{}` (the exact pre-`27cc0c9` state) | **RED** — `TestEndToEndBlockedSeriesIsAbsentFromTheExportedArtifact` fails with `outcome=publish findings=[]`; `TestIneIngestConfig_CarriesEveryShippedThresholdForTheSixFrozenSlugs` fails for **all six** slugs |
| `max_delta_abs: 1000 → 2000` in the shipped `config/series/ocupados-epa.yaml` | **RED** — three tests: the blocked e2e arm plus `TestIngestSeries_WithoutAnAcknowledgementTheCovidQuarterStillBlocks` and `TestIngestSeries_AnUnsignedDraftLeavesTheCovidQuarterBlocked` |
| The honest artifact fed to the script as the "blocked" one | **RED** — rejected at the pre-build inspection, before any build runs |
| (control) unmodified tree | **GREEN** for the right reason — script exit 0, both arms as designed |

The second row is the one that matters most, and it is the finding *behind* the finding. Before `27cc0c9`,
`acknowledgement_e2e_test.go` restated `max_delta_abs` as a Go literal, so raising it in the YAML left the
test whose entire subject is that YAML green. That is now closed at the source: `ocupadosCovidCase()` no
longer carries thresholds at all, and `ineIngestConfig` reads them through `shippedConfig` →
`configdata.FS` → `fs.Sub` → `config.Load` — the same three calls `validate-config` makes.

`TestIneIngestConfig_CarriesEveryShippedThresholdForTheSixFrozenSlugs` additionally checks its own fixture
before comparing (`declares no plausibility bound at all; this test would pass vacuously for it`), which is
the right defence against the exact failure mode this change has been bitten by five times.

**One residual, disclosed by the author and confirmed here.** The *successful* arm still runs the recorded
three-period fixtures, which cannot breach any shipped threshold (largest ratio 486.0 against 1000). So CI
proves the mechanism in both directions; it does not and cannot prove that today's production artifact
builds. The commit body's "goes red the day one bites there" is true only for a *lowered* threshold, not
for new data — the fixtures are frozen files. That is a fair narrowing, not a false claim.

**CRITICAL-37 structural half: CLOSED.** Substantive half: **OPEN** — §H.

---

## C. The prune (`5310586`), attacked

### C.1 What holds

- **Scope cannot escape.** `pruneUnpublishedFiles` reads only `filepath.Join(outDir, "series")` and
  `.../"csv"`, iterates `os.ReadDir` base names (no traversal is expressible), skips every non-regular
  entry (so a symlink named `x.json` is not followed, let alone removed), and requires the exact
  extension. Production `outDir` is `staticAssetRoot()/data-derived`; `retentionHistoryDir()` is
  `appDataRoot()/data-derived-history`, a different tree; `/web/build-manifest.json` is outside `dist/`
  by design. Astro's own output (`_astro/`, `indicador/`, `index.html`) contains no `series/` or `csv/`
  directory, so even a misconfigured `outDir` pointing at `/web/dist` would be inert.
- **`.tmp-*` really is unreachable.** `writeFileAtomic` creates `os.CreateTemp(dir, ".tmp-*")` →
  `.tmp-<digits>`, which cannot end in `.json` or `.csv`. Mutation confirms the extension filter is
  load-bearing: removing it turns `TestExport_LeavesEveryFileItDoesNotOwnUntouched` **RED** on
  `series/README.md`.
- **The zero-series guard is real.** Removing it turns
  `TestExport_RefusesToPruneWhenTheExportDeclaresNoSeries` **RED** (`before: 4 files / after: []`).
  Removing the prune call altogether turns two tests **RED**. The tests can fail.
- **Reporting is not decorative.** `PruneOutcome.Skipped` exists because an empty `Removed` cannot
  discriminate "nothing stale" from "the guard refused" — the same reasoning
  `PublishResult.DispatchSkipped` already uses, and correct.
- **The requirement was genuinely missing.** I checked: `/data-derived is generated from the same artifact`
  governs how each written file is derived and says nothing about files the export stops writing. The new
  requirement's own parenthetical states this accurately.

### C.2 WARNING-44 — the ordering rationale does not survive the build boundary, and the resulting 404 is live right now

`export.go:226-247` rejects prune-first because it "degrades to a live 404" on a declared file, and claims
prune-last's window is "bounded to milliseconds and unreachable through any manifest-driven path". Both
claims are true **within one `Export` call** and false **across the build boundary**.

Measured on the running stack at `5310586`:

```
/indicador/ocupados-epa/                    200   (26,448 bytes, renders a chart and a data table)
  href="/data-derived/csv/ocupados-epa.csv"       -> 404
  href="/data-derived/series/ocupados-epa.json"   -> 404
/data-derived/manifest.json                 200   (9 series, 9 digests, no ocupados-epa)
```

The page is served from a build made when the series still published; the two files it links to were
removed by the prune. And the window is not milliseconds: the frozen-route guard **guarantees** the site
cannot be rebuilt while a frozen series is blocked, so this state persists for exactly as long as the block
does — which today means until a human signs `config/reconocimientos.yaml`.

The built page **is** a manifest-driven path, frozen at build time: `IndicatorPage.astro:199-200` emits
those two hrefs unconditionally from `doc.slug`. The prune therefore reintroduces CRITICAL-1's exact
symptom ("Every 'Exportar CSV' link 404s") for a frozen permalink — and does so through the reasoning that
was written specifically to avoid a live 404.

**The trade may still be right.** Under principle P4, a 404 is honest and stale bytes presented as current
are not; the frozen-route guard's own error message says shipping a resolvable subset "would put a 404 on a
URL this project promised to keep". So I am not calling the choice wrong. I am recording that the reasoning
that chose it never considered this case, asserts the opposite property, and is now contradicted by the
deployed stack. Not a spec violation: no requirement covers the CSV/JSON download hrefs.

### C.3 WARNING-45 — the ordering is untested, and the stated reason for that gap is a category error

The author disclosed that "the prune's placement after the manifest is not covered by a test (forcing an
unlink failure needs a read-only directory, which also blocks the writes that must succeed first)".

The first clause is true. The second does not justify it: a read-only directory is what an
**unlink-failure** test needs, which is a different claim from the **ordering**. I proved the ordering is
cheaply testable:

- **Mutation**: move `pruneUnpublishedFiles` to before the series/CSV writes.
  `go test ./internal/publishing/...` → **`ok`**. The single most-reasoned decision in this commit is
  entirely unguarded.
- **Counter-proof**: a 25-line test that replaces `manifest.json` with a *directory* (so the manifest
  rename fails while every series/CSV write succeeds) then asserts the dropped series' file still exists.
  **PASS at HEAD** ("the stale file survives a failed manifest write"), **FAIL under the mutation**
  ("PRUNE RAN BEFORE THE MANIFEST"). No read-only directory, no blocked writes.

The disclosed unlink-failure gap and the partially-failed-prune gap (`Removed` is lost on error) I judge
**adequately disclosed**. The ordering gap I do not: the disclosure names an obstacle that does not apply
to it.

### C.4 WARNING-46 — the two outer defences cited for leaving PARTIAL unguarded do not cover PARTIAL

The decision to guard only the zero-doc case is defensible on its own terms: no ratio floor is principled,
and any floor eventually errs in the direction that loses data. I could not construct a *realistic*
partial-loss path — the read side is sound. `ListPublishedObservations`' inner join on the nullable
`ingestion_run.raw_file_hash` looked like one, but `runlifecycle.go` sets that column at INSERT and never
backfills, and nothing anywhere deletes `raw_file` or `ingestion_run` rows. A series with published history
keeps it through a block. The realistic ways a series leaves `ListPublishedSeries` (a retired mapping, a
retired series) are exactly the *legitimate retirement* the author names.

What does not hold is the **justification**, and both halves of it are cited in code comments and in the
commit message as load-bearing:

1. *"the ingest gate ('a cycle that learned nothing MUST NOT export'), which is what stops an empty or
   degraded read from reaching an export at all"* — the gate is
   `if (published || failedValidation) && outDir != ""` (`ingest_cmd.go:487`), evaluated over the **whole
   batch**. One series learning anything arms the export for all ten. The gate is a fact about what the
   *cycle* learned; the prune's input is an independent read of the *database* inside `Export`. It is
   structurally incapable of seeing a degraded read. And the standalone `concontexto export` command
   bypasses it entirely — `runExport` calls `publishing.Export` with no cycle gate at all, which is
   precisely the invocation an operator reaches for against a half-restored database.
2. *"artifact retention (retention.go), which keeps the last N artifacts for rollback"* — `Publish` calls
   `Export` (which now prunes) and only then `ArchiveArtifact` (`trigger.go:128-135`). **The first bad
   export's own snapshot already lacks the removed files.** With `DefaultRetainedArtifacts = 5` and
   `defaultScheduleInterval = 24h`, the recovery window is the five preceding snapshots and is fully
   evicted after five further exports. The commit message names this ordering as a *benefit* ("every
   retained rollback snapshot had been carrying the stale files forward... that stops now") without
   noticing that it is the same change that shortens the defence it cites.

Neither observation changes the decision. Both change what the record claims about why it is safe.

---

## D. The new requirement — spec compliance

**`publishing-export` / "The published directory contains exactly what the manifest declares"**

| Scenario | Test | Result |
|---|---|---|
| A series that stops being published leaves nothing behind | `publishing/export_prune_test.go > TestExport_ASeriesThatDropsOutOfALaterExportLeavesNothingBehind` + `cmd/.../TestRunExport_NamesEveryFileItRemovedFromTheServedDirectory` | ✅ COMPLIANT (mutation-RED when the prune is removed) |
| Removal never reaches a file the export does not own | `publishing/export_prune_test.go > TestExport_LeavesEveryFileItDoesNotOwnUntouched` | ✅ COMPLIANT (mutation-RED when the extension filter is removed) |
| An export that declares no series removes nothing | `TestExport_RefusesToPruneWhenTheExportDeclaresNoSeries` + `cmd/.../TestRunExport_ReportsThatTheZeroSeriesGuardRefusedToPrune` | ✅ COMPLIANT (mutation-RED when the guard is removed) |

The first scenario's "the directory holds exactly the files the new manifest declares, and no others" is
asserted as one set comparison against the manifest rather than as two hand-listed filenames, so a future
third projection cannot satisfy it while going stale. That is the right shape.

---

## E. Prior findings, re-adjudicated

| ID | Summary | Status this pass | Evidence |
|---|---|---|---|
| CRITICAL-1 | Every "Exportar CSV" link 404s | **CLOSED** | Unchanged; the *symptom* recurs narrowly as WARNING-44 |
| CRITICAL-2 | The blocking budget gate cannot fail | **CLOSED** | Unchanged |
| CRITICAL-3 | 37 inlined Spanish strings | **CLOSED** | Unchanged |
| CRITICAL-4 | Export artifact carries no page-state | **CLOSED** | Unchanged |
| CRITICAL-15 | Container publishes synthesised fixture as INE statistics | **CLOSED** | Re-verified this pass: `.dockerignore` removes the fixture from the build context and `BUILD_WITH_SYNTHETIC_FIXTURE` is deliberately not plumbed as a build arg. The running stack's pages were built via the sanctioned `EXPORT_DIR=data-derived` path from the real recorded-fixture e2e export (`ingestionRunId 2`), not from the synthetic fixture |
| CRITICAL-16 | Failed-ingestion publish path untested | **CLOSED** | Unchanged |
| CRITICAL-23 | The change does not exist in the repository | **CLOSED** | Clean tree at `5310586`, PR #1 open, four jobs green at that sha |
| CRITICAL-27 | Build silently drops a frozen slug | **CLOSED** | Re-verified: the real blocked artifact makes `astro build` exit 1 naming `ocupados-epa`, with a matched control at exit 0 |
| CRITICAL-28 | Publish loop open at four links | **CLOSED**, all four | Unchanged |
| **CRITICAL-37** | Change cannot produce a deployable site; no CI signal can see it | **structural half CLOSED, substantive half OPEN** | §B and §H |
| WARNING-29 | Run log claims a dispatch that did not happen | **CLOSED** | Unchanged |
| WARNING-31 | `SeverityBlockRequiresSignoff` names a mechanism that does not exist | **CLOSED** | Unchanged |
| WARNING-5,6,7,8,9,10,11,17,18,24 | (pass 2/3) | **CLOSED** | Unchanged |
| **WARNING-30** | INE nil-value crash class disclosed only in a commit message | **OPEN, narrowed** | `envelope.go:38/87` still carries no comment and no test. It is now recorded in `apply-progress.md:3598`, so it is no longer *only* in commit `befa81f` — a durability improvement, not a closure |
| **WARNING-38** | One acknowledgement resolves more than one finding | **OPEN** | Unchanged; not reachable in the shipped config |
| **WARNING-39** | The registry's only anti-forgery control is a review gate that does not exist | **OPEN, materially re-scoped** | Pass 5's "`404 Branch not protected`" is **stale**. `main` is now protected: 4 required checks, `required_approving_review_count: 1`, `require_code_owner_reviews: true`, `dismiss_stale_reviews: true`, `required_conversation_resolution: true`. But `enforce_admins: false`; the repository has exactly **one** collaborator (`jorgealonsodev`); and `.github/CODEOWNERS` names `/config/** @jorgealonsodev @TODO-second-config-reviewer` — a placeholder GitHub cannot resolve. The mechanism now exists; the second pair of eyes does not |
| **WARNING-40** | `apply-progress.md` records none of the recent commits | **PARTLY CLOSED, REOPENED** | `447d9d2` closed it for `4b20ca7`/`bdb6cc8`/`1f856e2` (6/17/11 references) and took tasks 191→219. It reopened one commit later: `27cc0c9` and `5310586` have **zero** references, no slice section, no task rows, and no TDD Cycle Evidence row — while `5310586` adds a delta-spec requirement and 638 lines under active Strict TDD. Separately, `apply-progress.md:3567` states that `grep -rn assert-blocked-series-fails-build .github/` "returns **nothing**"; that is false at HEAD *and was already false in its own parent commit* (`27cc0c9` added the invocation at `ingest-export-build.yml:262`). The paragraph is explicitly timestamped and self-labelled "not upgraded", so it is a disclosed snapshot rather than a fabrication — but archive would freeze a record whose own stated grep contradicts the tree |
| **WARNING-41** | "A failed rebuild raises an alert **immediately**" is substituted | **OPEN** | Unchanged |
| SUGGESTION-19 | Workbench CSV href layout | **OPEN** | Unchanged |
| SUGGESTION-20 | Spec silent on the dateless validation banner | **OPEN** | Unchanged |
| SUGGESTION-21 | No custom-range e2e on a real route | **OPEN** | Unchanged |
| SUGGESTION-22 | Workflows never observed on a runner | **CLOSED** | Four jobs green at `5310586` |
| SUGGESTION-25 | Copy-scan regex blind spot | **OPEN** | Unchanged |
| SUGGESTION-26 | Test output inside the source tree | **CLOSED** | Unchanged |
| SUGGESTION-32 | `befa81f` commit body overstates | **OPEN** | Unchanged |
| SUGGESTION-33 | Container bring-up has no automated coverage | **OPEN** | Unchanged |
| SUGGESTION-34 | Record drift | **OPEN** | `openspec/config.yaml:15/92/94` still declares `go test ./...` while CI and this verification run `-race -count=1` |
| SUGGESTION-35 | Four-eyes documented, unenforced | **ESCALATED** (pass 5) → now tracked as WARNING-39 |
| SUGGESTION-42 | Neighbouring-period residual narrower than disclosed | **OPEN** | Unchanged |
| SUGGESTION-43 | `smoke-test.sh` usage omits `EXPORT_DIR`/`EXPORT_URL` | **OPEN** | Re-verified: the usage block still reads `./scripts/smoke-test.sh`, line 151 is a bare `docker compose build app`, and `ci.yml:204` supplies `EXPORT_DIR: data-derived` that the script does not |

---

## F. New findings, pass 6

**CRITICAL** — none new. `CRITICAL-37`'s substantive half carries forward (§H).

**WARNING-44** — The prune's ordering rationale does not survive the build boundary; a frozen permalink's
two download links are 404 on the running stack right now, and the state is persistent rather than
transient because the frozen-route guard prevents the rebuild that would clear it. §C.2.

**WARNING-45** — The prune's ordering — the one decision the commit spends twenty lines justifying — has no
test. Moving it before the writes leaves the whole `publishing` package green. The disclosed obstacle (a
read-only directory) applies to the unlink-failure path, not to the ordering, which I proved testable in
25 lines. §C.3.

**WARNING-46** — Both outer defences cited to justify leaving the PARTIAL case unguarded fail to cover it:
the ingest gate is batch-scoped and structurally blind to a degraded read (and `concontexto export` skips
it entirely), and retention's snapshot is taken *after* the prune, bounding recovery to five preceding
snapshots at a 24-hour cadence. §C.4.

**SUGGESTION-48** — Concurrent exports have no lock, and the doc comment reasons about concurrency for
`.tmp-*` only. `concontexto export` run by an operator while `serve`'s in-process scheduler cycles could
prune a slug the other export had just written, if the two reads saw different database states. Narrow —
both reads normally produce the same set — but the asymmetry in the reasoning is worth closing.

**SUGGESTION-49** — The in-cycle publish path's prune report (`ingest_cmd.go:506-508`) has no test; only
`runExport`'s is covered. `pruneOutcomeMessage` is shared, so the rendering is proven and the wiring is not.

**SUGGESTION-50** — `web/src/pages/index.astro`'s header comment claims "Real indicator pages replace this
one starting slice 9". Slice 9 built `/indicador/[slug]` and never touched `/`, so the comment promises a
replacement that did not happen, and its "until then" framing has outlived its own condition (the page is
still the `home.spec.ts` axe/e2e target and is not temporary). **This is an obsolete disclosure, not an
unmet requirement**: I grepped all twelve delta specs for `portada|home page|homepage|landing|página de
inicio|índice de indicadores` and found **zero** hits — no spec defines a homepage, so there is nothing to
be non-compliant with, and inventing one would be manufacturing a finding. Worth recording alongside it:
`/` carries no link to any of the six frozen permalinks, and search/navigation is explicitly out of scope
(`proposal.md:72`, milestones 1.3–1.7), so the six pages are currently reachable only by direct URL. Also
not a spec violation.

---

## G. Strict TDD

### TDD Compliance

| Check | Result | Details |
|---|---|---|
| TDD Evidence reported | ⚠️ | Tables exist for slices 1–17 (19 of them). **Absent for `27cc0c9` and `5310586`** (WARNING-40 reopened) |
| All tasks have tests | ✅ | 219/219 checked, 0 unchecked |
| RED confirmed (test files exist) | ✅ | `export_prune_test.go` (263 lines), `e2e_blocked_export_test.go` (213 lines), new cases in `export_cmd_test.go` and `ingest_test.go` — all present |
| GREEN confirmed (tests pass) | ✅ | Every one re-executed here; all pass |
| RED confirmed **causally** | ✅ | Seven mutations applied to the real tree this pass; six went RED exactly where claimed, one (M5, ordering) did not — WARNING-45 |
| Triangulation adequate | ✅ | The prune alone carries drop-out, foreign-file, zero-series, and two command-layer reporting cases |
| Safety net for modified files | ✅ | Full suite re-run and green |

**TDD compliance: 6/7.**

### Test layer distribution (work added since pass 5)

| Layer | Tests | Files | Tools |
|---|---|---|---|
| Unit (real filesystem, fake ports) | 3 | 1 (`publishing/export_prune_test.go`) | Go testing |
| Command-layer (real `runExport`, in-memory deps) | 2 | 1 (`cmd/.../export_cmd_test.go`) | Go testing |
| Integration (real Postgres, real ingest, real export) | 1 | 1 (`ingestion/e2e_blocked_export_test.go`) | testcontainers tx harness |
| Config-contract (real embedded `/config` tree) | 1 | 1 (`ingestion/ingest_test.go`) | Go testing + `embed.FS` |
| Build-level (two real `npm run build` subprocesses, matched pair) | 1 | 1 (`scripts/assert-blocked-series-fails-build.sh`) | bash + Astro |
| **Total (new)** | **8** | **5** | |

### Assertion quality

Scanned all new and modified test files for tautologies, orphan empty checks, type-only assertions, ghost
loops, smoke-only tests and mock-heavy ratios. **Zero hits.** Three observations, all favourable:

- `TestExport_ASeriesThatDropsOutOfALaterExportLeavesNothingBehind` asserts the invariant as a set
  comparison against the manifest, not as two hand-listed names — it cannot be satisfied by a future third
  projection going stale.
- `TestIneIngestConfig_CarriesEveryShippedThresholdForTheSixFrozenSlugs` checks its own fixture for
  vacuity before comparing against it.
- `TestEndToEndBlockedSeriesIsAbsentFromTheExportedArtifact` asserts the rule name **and** the period, and
  separately asserts the other five slugs are present — so a block for any other reason, or an empty
  export, fails it.

**Assertion quality: ✅ All assertions verify real behaviour.**

### Quality metrics

**Linter/vet**: ✅ `go vet` clean, `gofmt -l` empty.
**Type checker**: ✅ `astro check` — 0 errors, 0 warnings, 2 hints across 97 files.
**Coverage**: ➖ no coverage threshold configured for this change; skipped, not a failure.

---

## H. Archivability — the plain answer

**No. Not archivable. One thing blocks it, and it is a human signature.** The pass-5 reading is
**CONFIRMED**, not refuted.

At `5310586` the pipeline can produce five of six frozen series, and the build — correctly — refuses to
ship five, so it produces none. I ran it: exit 1, no `indicador/` directory. `openspec archive` would move
into `openspec/specs/` a requirement asserting a MUST the system demonstrably cannot satisfy.

What has changed since pass 5 is that this is no longer *invisible*. `27cc0c9` made CI able to go red for
it, and I proved that by mutation rather than by reading the assertions. The recommendation pass 5 made
alongside the signature has therefore landed and been verified.

**What must happen — exactly one of these. Neither is code.**

1. **A named human reviews and signs `config/reconocimientos.yaml`.** Open
   `https://www.ine.es/daco/daco42/daco4211/epa0220.pdf`, confirm 2020-Q2 = 18607.2 and that the fall is
   COVID-19 rather than a methodology change or a parser fault, then replace `signature_status` /
   `drafted_by` / `todo` with `acknowledged_by` (full name) and `acknowledged_on`. After that:
   `ocupados-epa` publishes as `succeeded-with-acknowledgement`, the artifact carries six series, the build
   emits six pages, CRITICAL-37 closes with no Go or TypeScript change — **and WARNING-44's live 404 clears
   with it**, because the next export rewrites both files and the rebuild can finally succeed.
2. **Or the same human rejects it**, deletes the entry whole, and chooses a different remedy for
   `ocupados-epa` in its own SDD cycle.

**Do not**: raise `max_delta_abs`, add a break to `config/rupturas.yaml`, weaken
`resolveIndicatorRouteSlugs`, or let an agent sign the record. All four are now caught by tests; the last
was attempted once already and caught by a human.

**Recommended before archive, not blocking the signature**: WARNING-40 (record `27cc0c9` and `5310586` in
`tasks.md`/`apply-progress.md` with TDD evidence rows, and correct the `assert-blocked-series-fails-build`
grep claim at `apply-progress.md:3567`, since archive freezes both) and WARNING-45 (the ordering test is
25 lines and the mutation proving it necessary is already written above). WARNING-44 and WARNING-46 are
disclosure and reasoning corrections; WARNING-38, WARNING-39 and WARNING-41 are follow-ups.

---

## Verdict, restated

**FAIL. Do not archive.**

`CRITICAL-37`'s structural half is closed and was closed well — the guard now reads its thresholds from the
YAML it is about, in two places, and four mutations prove it can go red. The prune is a real fix to a real,
live, previously-unspecified integrity defect, correctly scoped, correctly reported, and backed by tests
that fail when the code is broken.

Three things are worth carrying forward. The prune's ordering argument is right inside one `Export` call
and silent about the build boundary, where it produces a persistent live 404 I measured on the running
stack. That ordering is the one thing in the commit with no test, and the reason given for not testing it
does not apply to it. And the two outer defences invoked to leave the partial case unguarded — the ingest
gate and retention — do not cover that case, one because it is batch-scoped and bypassed by
`concontexto export`, the other because this very change moved the snapshot to after the deletion.

The single blocker is unchanged and is not a defect: **a named human must review and sign, or reject and
delete, `config/reconocimientos.yaml`'s one draft.** Until then, six frozen permalinks cannot be built and
the change cannot honestly be archived as delivered.

**1 CRITICAL, 8 WARNING (5 carried open, 3 new), 12 SUGGESTION (9 carried open, 3 new).**
Counting basis: open findings only; closed findings are listed in §E with their evidence.

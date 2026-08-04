```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:e68e4d8b7ac7a9186adf8a68fd4d4f98c37459b66bce37919a7666e9ff4caf1d
verdict: pass
blockers: 0
critical_findings: 0
requirements: 79/79
scenarios: 176/176
test_command: go test -race -count=1 ./... && npm --prefix web run check && npm --prefix web test && npx --prefix web playwright test --config web/playwright.config.ts
test_exit_code: 0
test_output_hash: sha256:c00ddbf0df23a2c0f0054faaee2fdb443bfad855bfc045fa849a24fb3e9385b2
build_command: EXPORT_DIR=data-derived npm --prefix web run build
build_exit_code: 0
build_output_hash: sha256:cff334a383b9775d07eef185b117ba208c8e535f48c76dd6db3b7991506aef9d
```

# Verification Report — phase-1-indicator-page

**Verifier**: `sdd-verify` (pass 10)
**Date**: 2026-08-05
**Change**: `phase-1-indicator-page`
**Mode**: hybrid (OpenSpec file + Engram `sdd/phase-1-indicator-page/verify-report`), Strict TDD active
**Artifacts read**: proposal, design, 13 capability delta specs, tasks, apply-progress, pass-9 verify-report
**Verified against**: the REPOSITORY (`feat/phase-1-indicator-page` @ `f973390`, working tree clean before
and after every probe, including after four source mutations), the RUNNER, and a FRESH SITE BUILD at HEAD.
Every number below was re-measured here. Nothing was taken from a session summary, a commit body, or the
record under review.

---

## Verdict

**PASS WITH WARNINGS. Zero CRITICAL findings. Zero blockers. Requirements 79/79, scenarios 176/176.**

The last open scenario is closed, and it is closed by a **correction, not an accommodation**. That was the
central question of this pass and it is adjudicated below at length, on four independent grounds, each
measured rather than read.

This change is **ready to archive**, carrying three WARNINGs and three SUGGESTIONs, none of which is a
product defect and none of which blocks.

---

## A. Measurement correction inherited by this pass

Every suite figure in passes 1–9 came from `cd app && go test ./...`, which reports **24** packages.

`go.mod` is at the **repository root**; `app/` is a subdirectory. `cd app && go test ./...` therefore
cannot reach the root package at all — the package whose `config_embed_test.go` is the one test that reads
the `config/` tree, and whose `licensedata_test.go` guards the data licence. This is a structural
limitation of the command, not a flake.

Re-measured from the root: **25 packages, 23 with tests, 2 with none, exit 0** under `-race -count=1`.
`github.com/jorgealonsodev/concontexto` (the root package) is present in that list. Every figure in this
report is the root measurement. The correction is adopted permanently.

---

## B. The amendment — the central question, adjudicated

`f973390` replaced `pipeline-operations` / "A failed rebuild raises an alert **immediately**" with
"A failed rebuild alerts once the publish-latency budget elapses", and preserved the original in a
`(Previously: …)` block.

**A spec amended to match the implementation is how specs get hollowed out.** The question is not whether
the new scenario passes — a scenario written from the code always passes — but whether the amendment
removes a false claim or a real guarantee. **It removes a false claim.** Four grounds, each measured here.

### B.1 The parent requirement's normative sentence was never touched — and it was already budget-bounded

`git diff f973390^ f973390` on the spec shows exactly one hunk: the scenario. The requirement it belongs to
is unchanged, and it reads:

> When a successful ingestion is not followed by a completed rebuild and deploy **inside the publish-latency
> budget**, the system MUST raise an operational alert through the existing alerting sink…

The budget bound is in the requirement itself, and always was. The deleted scenario's word "immediately"
therefore **contradicted its own parent requirement**. The amendment does not bend a requirement to fit the
code; it removes a scenario that was inconsistent with the requirement above it. This is the decisive
ground, and it is the one that distinguishes a correction from an accommodation.

### B.2 The detection is real — the chain verified link by link, not accepted

The writer's chain was re-derived from the repository rather than read from the record:

| Link | Claim | Verified at |
|---|---|---|
| 1 | `RebuildLatencyBreached` compares the live artifact's `generated_at` against the artifact the DEPLOYED pages were built from | `app/internal/scheduler/watchdog.go:98-110` — read in full; the two instants move independently |
| 2 | `deploy.yml` gates on `workflow_run.conclusion == 'success'` | `.github/workflows/deploy.yml:39`. Qualified below |
| 3 | `/web/build-manifest.json` has exactly one writer, which fails the build rather than shipping an unobservable image | `Dockerfile:186` (`fs.writeFileSync`) and `Dockerfile:252` (`COPY`). `grep -rn build-manifest.json` across `*.go *.ts *.js *.yml Dockerfile` returns no other writer. The Node stamp calls `process.exit(1)` at `Dockerfile:192` when the manifest carries no `generated_at` |
| 4 | Therefore divergence necessarily outlives the budget | Follows from 1–3 |

**Link 2 is narrower than the record states, and the gap is covered by something the record does not
claim.** `deploy.yml:39` reads `github.event_name != 'workflow_run' || …conclusion == 'success'`, so the
conclusion gate governs only the `workflow_run` entry point. The path a *data-driven* rebuild actually
takes is `repository_dispatch` → `rebuild.yml` → `workflow_call` → `deploy.yml`, which is **not** gated on
that conclusion. Checked directly: `rebuild.yml:131-135` gates the reused `deploy.yml` on
`needs.verify-origin.outputs.proceed == 'true'`, and `verify-origin` refuses on an unnamed artifact
(`:60-63`), an unreachable origin (`:95-98`), and an origin older than the dispatch (`:114-117`).

The conclusion is unchanged and in fact over-determined: in **every** failure mode of the rebuild —
verify-origin refusing, the image build failing, the gates unconfigured, or the Portainer webhook failing —
no new image is pushed and no redeploy occurs, so `/web/build-manifest.json` cannot advance. It changes by
exactly one mechanism, and that mechanism did not run.

### B.3 The three claimed directions genuinely discriminate — mutation-checked

The record maps each of the amended scenario's three clauses to a test by file and line. Each mapping was
tested by mutating the production decision and re-running:

| Mutation | Expected to kill | Measured |
|---|---|---|
| **MA** — `RebuildLatencyBreached` drops its `elapsed < budget` guard | the "silent inside the budget" direction | `TestRebuildLatencyBreached` **FAILS** |
| **MB** — it drops the `!deployedGeneratedAt.Before(artifactGeneratedAt)` guard | the "silent once the pages carry this artifact" direction | `TestRebuildLatencyBreached` **and** `TestPublishLatencyWatchdog_SilentWhenTheDeployedPagesCarryTheCurrentArtifact` **FAIL** |
| **MC** — it never reports a breach | the "fires after the budget" direction | `TestRebuildLatencyBreached` **and** `TestPublishLatencyWatchdog_AlertsWhenTheDeployedPagesNeverCaughtUp` **FAIL** |

All three killed. Tree verified clean after each.

**The record's mapping is honest about its own weakest link.** MA killed only the *unit* test, not a wiring
test — the wiring suite has no inside-the-budget case. The record maps that direction to
`watchdog_test.go:106-113` and not to a wiring test, which is exactly right. A record inflating its coverage
would have claimed the wiring test there.

### B.4 What was dropped — and whether a guarantee went with it

| Deleted clause | Status |
|---|---|
| "WHEN the failure **is observed**" / "naming the ingestion run and **the build failure**" | **Dropped, and unreachable — verified.** `grep -rn workflow_run .github/` returns **five hits, all inside `deploy.yml`**, all its own trigger and the comments on it. No `workflow_run` receiver exists anywhere, and none feeds the Go `alerting.Sink`. Nothing in this system observes a dispatched run's conclusion |
| "AND the currently deployed site continues to be served unchanged" | **Not lost — relocated.** It is now the amended scenario's GIVEN ("the pages still being served are the ones built from an artifact older than the one this publish cycle produced"), which is the condition the alert fires on. The reader-facing half remains under this requirement's own "The condition never reaches a reader" scenario, unamended |

**One thing an accommodation would have done and this amendment did not.** Two further refusals of this
watchdog are implemented and tested — it declines to fire when `APP_REBUILD_DISPATCH` is off, and when the
deployed artifact is *unknown* rather than stale (`schedule_rebuild_watchdog_test.go:130-173`). Folding
them into the scenario would have inflated apparent coverage at zero cost. They were deliberately left out.
That is the behaviour of a writer correcting one bound, not of one maximising a green count.

### B.5 The one thing the amendment did move toward the code — recorded, not waved past

The old scenario said "naming the ingestion run"; the new one says "naming **the source** and the elapsed
time". The implementation passes `sourceID` and `series=""`
(`schedule.go:479`). So the amendment did align that clause with what the code supplies.

This does **not** hollow the spec, because the demand survives twice elsewhere, unamended:
`pipeline-operations` / "A stalled rebuild raises an alert" still requires "the series, the ingestion run
and the elapsed time", and the MODIFIED "Operational alerts" scenario still requires "the source, the
series and the elapsed time". The guarantee is still in the delta; the amended scenario simply no longer
restates it.

The underlying gap — one shared `manifest.json` covers every series of a source, so no series-level
identity exists at that layer — is disclosed with reasoning in `apply-progress.md:1017-1023` at slice time,
extends `Alert.Series`'s pre-existing "empty for a source-level alert" convention, and was adjudicated
across passes 4–9. **Not reopened.** It is recorded as SUGGESTION-62 because the delta is now internally
inconsistent about it, not because the behaviour changed.

### B.6 Verdict on the amendment

**Correction, not accommodation.** It removes a scenario that contradicted its own requirement, that named
an event no part of this system can observe, and that had been untested since pass 4. The detection it
describes is real, is bounded by a budget this same delta already declared, and is falsifiable in three
directions that three mutations killed. `pipeline-operations` / "An ingestion not followed by a rebuild
alerts operators" moves to **4 of 4 scenarios**, taking the change to 79/79 and 176/176.

---

## C. The seam, a tenth time

Seven of nine prior passes found a defect on the pipeline/publish boundary. All six links were exercised
this pass; the first-hand probe was the one link no prior pass had checked — whether the detection is
actually **live** in the shipped configuration.

| Link | State at HEAD |
|---|---|
| ingestion → export | `publishing.Publish`; covered |
| export → dispatch | `trigger.go` + `adapters/github`; `KindDispatchFailed` on a failed POST |
| dispatch → receiver | `rebuild.yml` exists and listens for `repository_dispatch: [rebuild]` |
| rebuild → deploy | `workflow_call`, gated on `verify-origin` |
| deploy → build stamp | one writer, fails the build rather than shipping an unobservable image |
| stamp → alert | `RebuildLatencyBreached`; three mutations killed |

**The new probe.** `rebuildDispatchExpected()` (`rebuild_dispatch.go:78-87`) returns **false** when
`APP_REBUILD_DISPATCH` is unset, and the deploy watchdog is skipped entirely when it does
(`schedule.go:464-466`). A watchdog that is inert in the shipped default would make this whole amendment
decorative. It is not: `docker-compose.yml:227` sets `APP_REBUILD_DISPATCH: ${APP_REBUILD_DISPATCH:-required}`,
so the deployed stack defaults to **required** and the comparison is live. The Go-level `false` is the
bare-binary default; `docker-compose.override.yml:61` sets `"off"` deliberately for a dev stack, which is
the case `TestPublishLatencyWatchdog_SilentWhenRebuildDispatchIsDeliberatelyOff` pins. `env.example:162-190`
documents all three variables and states what silence means in each mode. `logDeployWatchdogState`
(`schedule.go:508-525`) records active/inactive at start-up, so "no alert has ever fired" is
distinguishable from "the check was never running".

**No defect found on the seam this pass.** First time in ten.

---

## D. Test & build evidence (all re-measured at `f973390`)

| Command | Result |
|---|---|
| `go test -race -count=1 ./...` **(repository root)** | **exit 0** — 25 packages, 23 with tests, 2 without |
| `go run ./app/cmd/concontexto validate-config` | **exit 0** — `validate-config: ok` |
| `npm --prefix web run check` | **exit 0** — 131 files, **0 errors**, 0 warnings, 2 hints |
| `npm --prefix web test` (Vitest) | **exit 0** — **799 passed / 799**, 52 files |
| `EXPORT_DIR=data-derived npm --prefix web run build` | **exit 0** — 7 pages |
| `npx --prefix web playwright test` | **exit 0** — **191 passed / 191**, 38.4s. **No flake this run** (`chart-island.spec.ts` all green; the known flake did not reproduce — the Go suite was not running concurrently) |

Full declared chain run as one command: **exit 0**, output hash in the envelope.

### Completeness

| Metric | Value |
|--------|-------|
| Tasks total | 447 |
| Tasks complete | **447** |
| Tasks incomplete | **0** |

`tasks.md` 419 → 447 at `f973390`; `grep -c '^\s*- \[ \]'` returns **0**.

### Spec compliance

| Capability | Requirements | Scenarios |
|---|---|---|
| data-model-vintages | 4 | 8 |
| data-validation | 2 | 16 |
| design-system | 9 | 14 |
| editorial-config | 3 | 9 |
| indicator-page | 16 | 39 |
| pipeline-operations | 4 | **10 — all compliant for the first time** |
| platform-runtime | 1 | 2 |
| publishing-export | 12 | 21 |
| series-transformations | 8 | 14 |
| source-attribution-licensing | 1 | 3 |
| source-ingestion-eurostat | 3 | 7 |
| source-ingestion-ine | 6 | 17 |
| web-accessibility-gates | 10 | 16 |
| **Total** | **79** | **176** |

**Compliance: 79/79 requirements, 176/176 scenarios.**

**Scope of this pass, stated honestly.** `f973390` changed no product code — only `openspec/` and two
comment blocks in `config/`. This pass re-verified at full depth the one scenario that changed (§B, four
grounds, three mutations) and re-adjudicated every open finding against HEAD (§E). The remaining 175
scenarios carry forward pass 9's adjudication on unchanged code, with the complete suite re-run green at
HEAD from the corrected root command as the standing evidence.

---

## E. Findings ledger — every finding, re-adjudicated at `f973390`

### CRITICAL

**None.** CRITICAL-1 … CRITICAL-54 all remain **CLOSED**; none reopened at HEAD.

### WARNING

| # | Finding | Status at `f973390` | Evidence |
|---|---|---|---|
| **WARNING-41** | "A failed rebuild raises an alert immediately" substituted, not implemented — open since pass 4 | **CLOSED** | §B. Closed by amending the bound, adjudicated a correction on four grounds |
| **WARNING-55** | Six of seventeen slices carry no red-state evidence, permanently | **OPEN, unchanged** | Historical and unrecoverable. Adjudicated non-blocking at passes 8 and 9; that adjudication stands |
| **WARNING-57** | Editorial authoring docs still forbid the shape the fix blessed | **CLOSED** | Verified in the `f973390` diff: `config/rupturas.yaml:11-27` now states *"`date` PUEDE acompañar a `date_status: unconfirmed` … Lo que protege la serie es el estado declarado, no que el campo esté vacío"*, and `config/README.md:24-32` now states the same in English. Both were the documents an editor actually reads |
| **WARNING-58** | The new test's doc comment overstates what it discriminates | **OPEN, correctly — not a blocker** | Re-measured. Reproduced M4 myself: dropping `\|\| date == nil` from `isDatePending` (`reconcile.go:167-168`) leaves the **entire Go suite green** across all 25 packages. Adjudication below |
| **WARNING-59** | Record one commit behind; one `design.md` open item false at HEAD | **CLOSED** | `apply-progress.md` now carries slices 38 and 39 (+304 lines); `tasks.md` 419 → 447; `design.md:1572-1615` supersedes the slice-32 item with a four-row claim-vs-HEAD table rather than rewriting it; `design.md:705-715` corrects the parenthetical while correctly leaving the item open. All four sub-items discharged |
| **WARNING-61** | **NEW** — `design.md`'s slice-17 open item is falsified by the very commit under review | **OPEN** | §E.2 |

#### E.1 WARNING-58 — why OPEN is the correct classification, not a blocker

`reconcile_test.go:535` still reads *"were the guard to drop **either half** of its condition, the two would
disagree here."* Measured: only the status half is discriminated.

**It is not a product defect, and the reason is enforced elsewhere rather than asserted.** The nil half is a
dereference guard for a state `validate-config` already rejects — `validate.go:401-406` (*"break %q: date is
required unless date_status is \"unconfirmed\""*) and `validate.go:610-615` (the same for events). A
confirmed entry with no date cannot pass validation, so no configuration reaching `isDatePending` can
exercise that half. `isDatePending`'s own comment states this correctly two files away.

So the guarantee holds; what is wrong is one sentence in a test comment claiming a discriminating power the
test does not have. **WARNING, non-blocking.** The accurate sentence is "were the guard to drop the status
half". Recorded a second time because it is the same species this change has chased for nine passes — a
comment asserting a property nothing enforces — appearing this time in the very test written to prevent it.

#### E.2 WARNING-61 — NEW: the record went stale a sixth time, on the item this commit closed

`design.md:1510-1523` still carries, **unticked**:

> `- [ ]` **New (slice 17) — "a failed rebuild raises an alert immediately" is substituted, not implemented
> (verify-report pass-5 WARNING-41).** … **Open in the honest sense**: either the spec's timing clause is
> narrowed to what budget-delayed detection actually provides, or a rebuild-failure callback is built.
> **Neither has been done**, and the scenario is one of the two non-compliant ones in pass 5's 159/161.

The spec's timing clause **was** narrowed — by `f973390`, the same commit that wrote every other correction
in this batch. `git diff f973390^ f973390 -- design.md | grep -c "slice 17"` returns **0**: the commit
superseded the slice-32 item and corrected the slice-705 parenthetical, and left this one untouched.

The item names the resolution it received and then denies having received it. This is the sixth time the
record has gone stale, and the first time it has gone stale on the exact item the commit closed.

**WARNING, not a blocker.** No product impact; the resolution is documented at length in
`apply-progress.md`'s slice 39 and in the spec's own `(Previously: …)` block, so an archive reader would
find it. But archive freezes `design.md`, and freezing a `- [ ]` that says "Neither has been done" when it
was done in the same commit set puts a known-false open item into the permanent record. **A one-line
correction — tick it and append a supersession note in the style this file already uses at `:1572` — is
worth doing before archive.** It does not block.

### SUGGESTION

| # | Finding | Status at `f973390` | Evidence |
|---|---|---|---|
| **SUGGESTION-56** | `homeListing.ts`'s delegation to `resolveIndicatorRouteSlugs` is untested | **OPEN, correctly** | Re-checked. `web/test/indicator/homeListing.test.ts` exists and drives `homeIndicatorListing`, but `grep -n resolveIndicatorRouteSlugs` in it returns nothing — the "no second derivation exists" clause is still unpinned |
| **SUGGESTION-60** | The events half of the un-retire clause is untested, and this fix made it load-bearing | **OPEN, correctly — not a blocker** | Carried from pass 9 (M6). `editorial.go:397`'s `\|\| current.RetiredAt != nil` is the only path by which the three pending events can reach a reader, and `eventDigest` excludes `date_status`, so confirming an entry leaves `config_digest` byte-identical. The clause is **present and correct**; only its test is missing. A coverage gap on the same seam that produced CRITICAL-54 — worth closing, blocks nothing |
| **SUGGESTION-62** | **NEW** — the delta is now internally inconsistent about what the publish-latency alert names | **OPEN** | §B.5. After `f973390`, one scenario of this requirement says the alert names "the source and the elapsed time" (matching the implementation) while its unamended sibling still says "the series, the ingestion run and the elapsed time" and the MODIFIED "Operational alerts" scenario says "the source, the series and the elapsed time". The implementation supplies source and elapsed only — a disclosed, reasoned, nine-pass-adjudicated reading (`apply-progress.md:1017-1023`). Recorded so a later reader does not conclude the amendment was selective; it was not, it simply did not sweep the siblings |

---

## F. Strict TDD sections

### TDD Compliance

| Check | Result | Details |
|-------|--------|---------|
| TDD Evidence reported | ✅ | Tables now cover slices 1–39. `f973390` added slices 38 and 39, closing pass 9's WARNING-59 gap |
| All tasks have tests | ✅ | 447/447 tasks complete; suite green at HEAD |
| RED confirmed (tests exist) | ✅ | Every test file referenced by the slice-39 mapping exists and was read |
| GREEN confirmed (tests pass) | ✅ | Full declared chain exit 0 |
| Triangulation adequate | ✅ | The amended scenario's three clauses map to three distinct directions, each mutation-killed |
| Evidence covers the current tree | ✅ | `f973390` is recorded as slice 39. First pass since 6 with no record-coverage gap |
| RED evidence quality | ⚠️ | 6/19 historical slices carry none — WARNING-55, permanent |

### Test Layer Distribution

| Layer | Tests | Files | Tools |
|-------|-------|-------|-------|
| Unit + integration (Go) | 25 packages, 23 with tests | — | `go test -race` |
| Unit + component (web) | **799** | 52 | Vitest 4.1.10 |
| E2E / a11y | **191** | — | Playwright + axe |

### Assertion Quality

Audited the tests load-bearing for this pass — `schedule_rebuild_watchdog_test.go` (5 tests) and
`watchdog_test.go`'s two table tests. No tautologies, no ghost loops, no smoke-only tests, no
assertion without a production call. The three "must stay silent" tests assert an empty alert
collection, which is an empty-collection assertion — but each has a **non-empty companion with the same
fixture** (`AlertsWhenTheDeployedPagesNeverCaughtUp`, `AStaleExportStillAlertsExactlyOnce`), which is the
condition that makes it legitimate. `RebuildLatencyBreached`'s table asserts both `breached` and the exact
`elapsed` in minutes, so it has variance in expected values rather than a single repeated shape.

**Assertion quality: ✅ All audited assertions verify real behaviour.** One doc-comment overclaim recorded
as WARNING-58 — the assertion is sound; the comment about it is not.

### Quality Metrics

**Linter / type checker**: ✅ `npm --prefix web run check` — 131 files, 0 errors, 0 warnings, 2 hints.
**Go vet**: ✅ implicit in `go test`, exit 0.

---

## G. What blocks archive

**Nothing.**

Zero CRITICAL findings, zero blockers, 447/447 tasks, 79/79 requirements, 176/176 scenarios, every declared
command exit 0, and the last open scenario closed by a correction that survives four independent tests of
whether it is an accommodation.

## H. What does not block archive, and should travel with it

- **WARNING-55** — a permanent, honestly declared Strict-TDD evidence shortfall in six historical slices.
  Unrecoverable; adjudicated non-blocking at passes 8, 9 and 10.
- **WARNING-58** — one sentence in a test doc comment claims a discriminating power the test lacks. The
  guarantee itself is enforced by `validate-config`. A one-line edit in `app/`.
- **WARNING-61** — `design.md:1510` will be frozen carrying a claim its own commit falsified. A one-line
  tick plus a supersession note. **The single item most worth doing before running archive**, because
  archive is what makes it permanent.
- **SUGGESTION-56, SUGGESTION-60, SUGGESTION-62** — two coverage recommendations and one spec-internal
  consistency note. SUGGESTION-60 remains the most valuable of the three: it is a correct guard with no test
  on the seam that produced CRITICAL-54.

All three WARNINGs and two of the three SUGGESTIONs need edits in `app/`, `config/` or `openspec/design.md`
— territory the record writer's remit excluded, which is why they are carried rather than closed.

---

## Verdict

**PASS WITH WARNINGS** — the change is complete, the suite is green from the corrected root command, the
last open scenario is closed by a verified correction rather than an accommodation, and the six remaining
items are recorded open items with no product impact.

**Ready to archive.**

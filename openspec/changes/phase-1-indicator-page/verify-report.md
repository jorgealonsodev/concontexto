```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:58d2aaaa97ce50b699612717d9652aa0ddac9f03b5e268e219c98ea23bb51bd2
verdict: fail
blockers: 1
critical_findings: 1
requirements: 73/75
scenarios: 159/161
test_command: cd app && go test -race -count=1 ./... && cd .. && npm --prefix web run check && npm --prefix web test && npm --prefix web run test:e2e
test_exit_code: 0
test_output_hash: sha256:7b593c36530e24249864abf383e7ea6f098551f009e44d42b343a8adf92a0a3a
build_command: EXPORT_DIR=/tmp/user/1000/claude-1000/-mnt-480GB-Proyectos-Personales-20260728-concontexto-20260728-concontexto-git-concontexto/b5d27279-aa14-400b-8a04-6da53081997f/scratchpad/prodshape npm --prefix web run build -- --outDir .verify-prod
build_exit_code: 1
build_output_hash: sha256:e29a8bb153ec2005ab1b99afbf8f6557dd8293d5cedb305ec371f360a054cd5a
```

# Verification Report — phase-1-indicator-page

**Verifier**: `sdd-verify` (pass 5)
**Date**: 2026-07-30
**Change**: `phase-1-indicator-page`
**Mode**: hybrid (OpenSpec file + Engram `sdd/phase-1-indicator-page/verify-report`), Strict TDD active
**Artifacts read**: proposal, design, 12 capability delta specs, tasks, apply-progress, pass-4 verify-report
**Verified against**: the REPOSITORY (`feat/phase-1-indicator-page` @ `1f856e2`, working tree clean) and the
RUNNER (GitHub Actions runs 30566474755 / 30566474760, both at `head_sha 1f856e2`), never against a session
summary. Every claim handed to this verifier was re-executed or re-derived from source.

---

## Verdict

**FAIL — one blocker, and it is not code.**

Every engineering defect this change has produced in five passes is now closed. `CRITICAL-27` and
`CRITICAL-28` are both closed, and closed properly: the route guard fails a real build with a paired
control, and all four links of the publish loop have a receiver, a composition, a deploy-completed instant
and an alert. The acknowledgement registry that `bdb6cc8` introduced survived a deliberate attempt to break
it, in both of the layers it claims.

What blocks archive is the consequence. `ocupados-epa` is blocked by `rule3-plausibility`, the record that
would resolve it is correctly unsigned and correctly inert, and the new route guard therefore makes a
production build **exit 1 and emit nothing**. I executed that build. The change's own central MUST — six
indicator routes — cannot be satisfied by any artifact this pipeline can currently produce, and no CI job
can see it, because every CI build path is fed an artifact in which the failure cannot occur.

The remedy is a human signature, not a commit. That is stated precisely in §H.

---

## Counts

| Dimension | Compliant | Total |
|---|---|---|
| **Requirements** | **73** | **75** |
| **Scenarios** | **159** | **161** |
| Tasks checked | 191 | 191 (0 unchecked) |

Authoritative totals parsed from `openspec/changes/phase-1-indicator-page/specs/*/spec.md` at `1f856e2`:
data-model-vintages 4/8, **data-validation 2/16** (was 1/7; `bdb6cc8` adds one requirement and nine
scenarios), design-system 9/14, editorial-config 3/9, indicator-page 15/34, pipeline-operations 4/10,
platform-runtime 1/2, publishing-export 11/18, series-transformations 8/14, source-ingestion-eurostat 3/7,
source-ingestion-ine 5/13, web-accessibility-gates 10/16. Pass 4's totals (74/152) are superseded by the
spec's own growth, not by a recount disagreement.

**Non-compliant requirements (2)**

| Capability / Requirement | Why |
|---|---|
| `indicator-page` / "Six indicator routes with frozen slugs" | The build produces **zero** of the six from the only artifact production can supply. CRITICAL-37. |
| `pipeline-operations` / "An ingestion not followed by a rebuild alerts operators" | 3 of 4 scenarios pass; "A failed rebuild raises an alert **immediately**" is substituted by budget-delayed detection. WARNING-41. |

**Non-compliant scenarios (2)**: `indicator-page` / "All six routes exist in the build";
`pipeline-operations` / "A failed rebuild raises an alert immediately".

Pass 4's four non-compliant requirements resolve as: three `pipeline-operations` requirements **CLOSED**,
one `indicator-page` requirement still open for a different reason than pass 4 recorded, and one new
requirement added and satisfied.

---

## A. Execution evidence — every number re-measured here

Working tree clean at `1f856e2` (`git status --porcelain` → 0 lines). Go and Playwright run sequentially,
never concurrently.

| Command | Exit | Result |
|---|---|---|
| `go build ./...` | 0 | clean |
| `go vet ./...` | 0 | clean |
| `gofmt -l app/` | 0 | no output |
| `go test -race -count=1 ./...` | **0** | 22 packages ok + 2 with no test files, zero race reports |
| `go run ./cmd/concontexto validate-config` | 0 | `validate-config: ok` |
| `npm run check` | 0 | 97 files, **0 errors, 0 warnings**, 2 hints |
| `npm test` (Vitest) | 0 | **448 passed / 448**, 35 files |
| `npm run test:e2e` (Playwright) | 0 | **64 passed / 64** |
| `npm run build` from a **production-shaped** artifact | **1** | **no `indicador/` directory emitted** — §C |

Every figure reported to this verifier reproduced exactly. Nothing was taken on trust.

**Runner.** PR #1 is OPEN and MERGEABLE. All four jobs pass at `head_sha 1f856e2`: `Go test suite` (1m25s),
`Frontend build and tests` (3m0s), `Container smoke test` (1m21s), `Ingest fixtures -> export -> astro
build` (40s). Confirmed by `gh api` against the run objects, not by reading the workflow files.

**Citation.** `https://www.ine.es/daco/daco42/daco4211/epa0220.pdf` → HTTP **200**, **1,233,410 bytes**;
`pdftotext` finds "1.074.000" and "COVID-19" across 8 lines. Independently reproduced.

---

## B. `bdb6cc8` — the acknowledgement registry, attacked as a security mechanism

It was asked to be attacked. It was. It holds, with two residuals recorded below as WARNINGs.

### B.1 Can the scope be widened? No — every attempt rejected at the config gate

Eight mutations applied to the real `config/reconocimientos.yaml`, each run through the real
`validate-config` against the real embedded config tree, each restored afterwards:

| Mutation | Result |
|---|---|
| `period: "*"` | rejected — "not a single period on series ... `Q` frequency grid ... never a whole year, a range or a wildcard" |
| `period: "2020"` | rejected — same check; a whole year is not a quarterly label |
| `rule: "*"` | rejected — "not acknowledgeable (only [rule3-plausibility rule4-revision] are)" |
| `rule: rule2-continuity` | rejected — same, with the continuity remedy named |
| `series: "*"` | rejected — "does not resolve to any configured series" |
| `signature_status: signed` | rejected — only `""` and `"unsigned"` exist |
| `signature_status: UNSIGNED` | rejected — fails **closed**, not silently through |
| `value:` removed | rejected — "value is required — it pins the exact number the reviewer confirmed" |

The claim that the schema has no syntax for widening holds. `isPeriodOnFrequencyGrid` is a per-frequency
regex over the exact `indicators.Period.String()` shapes, deliberately re-implemented rather than reusing
`parseCadenceOrdinal` — and that reasoning is correct: the shared helper folds annual into a
quarterly-shaped label and would have rejected `2020` for `pib-eurostat`.

The honest residual: N entries, one per period, each with its own pinned value and signature, would
neutralise rule 3 across a series' history. That is not a wildcard — it is an exhaustive enumeration of
individually-signed, individually-pinned values — but it is expressible. Its only control is review; see
WARNING-39.

### B.2 Is the rule allowlist sound, and can it drift? Sound, and it cannot

`acknowledgeableRules = {rule3-plausibility, rule4-revision}`. The stated reason is the right one: these are
the only two findings where the machine genuinely cannot separate a legitimate cause from a broken one.
`rule1-schema` / `source-decode` / `source-status` describe a broken parser; `rule5-metadata` describes our
own incomplete config; `rule6-nonempty` has no datum to review; `rule2-continuity` already has a
period-scoped editorial remedy and the delta spec says in terms that the remedy is to correct the config.
Acknowledging any of them would be a false statement, not a lenient one.

Anti-drift is real, not asserted: `TestAcknowledgeableRules_AreExactlyRule3AndRule4` runs the **real** rules,
collects the rule names they actually emit and asserts set equality. A rename or a new judgement-call rule
fails a test rather than silently producing an unmatchable acknowledgement. **The reasoning holds.**

### B.3 Is the unsigned record inert in both layers? Verified independently, with a control

I did not rely on the shipped tests. I loaded the **real embedded** `config/reconocimientos.yaml` through the
real `config.Load`, and drove both layers directly:

```
shipped record: id="ocupados-epa-2020-q2-covid" series="ocupados-epa" period="2020-Q2"
                rule="rule3-plausibility" signature_status="unsigned" acknowledged_by="" acknowledged_on=<nil>
LAYER 1 (reconcile predicate `SignatureStatus == "unsigned" || AcknowledgedBy == ""`)  -> not projected
LAYER 2 (pure gate, fed the record as if it HAD reached the DB) -> outcome=block overridden=0
CONTROL (same record, By="Ada Lovelace")                        -> outcome=publish-overridden overridden=1
```

The control is what makes the assertion mean something: the harness can distinguish the two states, so
"blocked" is a measurement rather than an inert setup. **Both layers are genuinely independent**, and
`Acknowledgement.signed()`'s defence-in-depth argument (the database can be hand-edited; a future caller can
build these values elsewhere) is correct rather than ceremonial.

The shipped tests corroborate it at runtime against a real Postgres:
`TestIngestSeries_AnUnsignedDraftLeavesTheCovidQuarterBlocked`,
`TestReconcileEditorialConfig_RefusesAnUnsignedRecordEvenWhenOtherwiseProjectable`,
`TestReconcileEditorialConfig_ReportsAnUnsignedDraftAsPendingRatherThanDroppingIt` — all pass.

### B.4 Does `validate-config` reject placeholder signatures? Yes — and only those

`isPlaceholderSignature` trims, lowercases, rejects anything under 2 characters and 18 vacant tokens.
Verified: `signature_status: signed` and every mixture of the two states is rejected with a message naming
the contradicting field. This satisfies the delta spec's requirement verbatim — "a signature that is present
but empty or **placeholder-shaped**".

It does not, and cannot, reject a fabricated plausible name. I demonstrated this: replacing the three draft
lines with `acknowledged_by: "Jorge Alonso"` and a date yields `validate-config: ok`, exit 0. The code's own
comment says this is undecidable and names four-eyes review as the control. That is honest — and it is
WARNING-39, because that control does not exist on this repository.

### B.5 Staleness — is pinning the value enough?

The three candidates (expiry date, message hash, observed value) are weighed correctly, and the observed
value is the right choice for the stated reasons. Exact float equality is safe here and the argument for it
is exact: both sides are the same deterministic decimal-literal parse, and any difference *is* the revision.
A stale pin does not merely fail to apply — it raises its own blocking `acknowledgement-stale` finding naming
the record and both values. Verified at runtime (`TestIngestSeries_AStaleAcknowledgementBlocksAgainAndSaysWhy`).

**The disclosed neighbouring-period residual is real but narrower than disclosed** — see SUGGESTION-42. I
judge the disclosure **adequate**: it errs toward overstating the risk, which is the correct direction.

### B.6 Where it over-reaches

One record resolves **more than one finding**, demonstrated at runtime — WARNING-38.

---

## C. NEW CRITICAL finding

### CRITICAL-37 — The change cannot produce a deployable site, and no green signal in this repository can see it

Three facts, each measured:

**1. `ocupados-epa` is blocked.** Proven at runtime by
`TestIngestSeries_WithoutAnAcknowledgementTheCovidQuarterStillBlocks`, which runs the real `IngestSeries`
against real INE data (2019-Q4 / 2020-Q1 / 2020-Q2) with the real thresholds from
`config/series/ocupados-epa.yaml` (`max_delta_abs: 1000`) through a real Postgres transaction. Outcome
`block`, zero observations written. No break covers 2020-Q2 in `config/rupturas.yaml`. The acknowledgement
that would resolve it is unsigned and inert (§B.3).

**2. A blocked series is absent from the artifact, and the build now refuses.** `export.go` skips a series
with zero observations, so the slug is in neither `series/` nor `manifest.series`. I built the real site
against exactly that shape:

```
$ EXPORT_DIR=<fixture with ocupados-epa removed> npm --prefix web run build
...
  - "ocupados-epa" has no series in the export artifact.
    THIS IS A PIPELINE PROBLEM, not a problem in the web tree ...
exit 1
indicador/ directory emitted: NO
```

This is the correct behaviour and it is CRITICAL-27's remedy working exactly as designed. It is also the
reason the requirement is unmet: the build produces zero of six, not six of six.

**3. Nothing in CI can go red for this.** Every build path in the repository is fed an artifact in which the
failure cannot occur:

| CI path | Artifact it builds from | Contains all six? |
|---|---|---|
| `ci` / Frontend build and tests | `BUILD_WITH_SYNTHETIC_FIXTURE=1` → `web/test/fixtures/export` | Yes, always |
| `ci` / Container smoke test | `web/data-derived`, exported by the same Go e2e test | Yes, always |
| `ingest-export-build` | `TestEndToEndIngestExportBuild` → real ingest → real export | Yes, always |
| Playwright / Lighthouse gates | the committed fixture | Yes, always |

The third row is the one that looks like it should catch this, and it is the one worth naming precisely.
It *does* run a real `IngestSeries` for all six frozen slugs through a real Postgres — but
`ineIngestConfig` (`app/internal/ingestion/ingest_test.go:121`) passes `Validation:
config.ValidationConfig{}`. **No thresholds at all.** Rule 3 has no `MaxDeltaAbs`, so it cannot fire, over
fixtures that are three periods long and do not contain 2020-Q2. The one job that exercises the real
Go→Astro hand-off deliberately disables the guard that blocks the series in production.

So this is the fifth consecutive appearance of the same shape — **a check placed where the failure cannot
occur** — after CRITICAL-2 (`details?.items ?? []`), CRITICAL-15 (silent fixture fallback), CRITICAL-27's
`dist/` loop, and now the entire CI corpus. The difference is that this instance is not a bug in a gate: the
gate is correct. The gap is that no CI path is ever handed production's own artifact shape.

**Spec violated** — `indicator-page`, "Six indicator routes with frozen slugs": *"The build MUST produce
`/indicador/{slug}` for exactly these six slugs"*. Scenario "All six routes exist in the build" is compliant
against a complete artifact and non-compliant against the only artifact production can supply.

**Remedy**: see §H. It is a signature, not a commit.

---

## D. NEW WARNING findings

### WARNING-38 — One acknowledgement resolves more than one finding

`Acknowledgement.covers` matches on `(series, period, rule)`. `Rule3Plausibility` can emit **two
semantically distinct findings at one period** under that one rule name: a min/max range breach and a
period-over-period delta breach. I demonstrated it:

```
rule3 emitted 2 findings at 2026-Q1:
  "value 30500 is above the configured maximum 30000"
  "period-over-period change of 8500 exceeds the configured threshold 1000 with no recorded break"
one acknowledgement (2026-Q1, rule3-plausibility, value 30500, signed)
  -> outcome=publish-overridden  overridden=2  unresolved=0
```

The spec's prose is finding-singular throughout — "a **specific** blocking validation finding", "the finding
it names", "An acknowledgement resolving no finding". The scenario "An acknowledgement never widens beyond
the finding it names" asserts only that *no configuration* expresses it, which is true. The runtime
nevertheless covers two findings from one signature: a human vouching for a delta silently also vouches for
a range breach they may never have looked at.

**Mitigated, not closed.** The pinned value constrains both findings to the same number the human reviewed,
so a parser bug producing a different value fails closed. Not reachable in the shipped config (18607.2 is
well inside `[0, 30000]`). The narrow fix is to give rule 3's two emission sites distinct rule names, or to
key the scope on the finding kind.

### WARNING-39 — The registry's only anti-forgery control is a review gate that does not exist

The design's own claim is "authority is the human signature". The enforcement is: reject empty,
whitespace, sub-2-character and 18 known vacant tokens. Everything else is accepted. Demonstrated: a
record signed `acknowledged_by: "Jorge Alonso"` with a date passes `validate-config: ok`.

The code says so explicitly and names four-eyes review as the compensating control. That control is **not
enforced**: `gh api repos/:owner/:repo/branches/main/protection` still returns `404 Branch not protected` at
`1f856e2`. `.github/CODEOWNERS` and `.github/BRANCH_PROTECTION.md` are documentation.

This is spec-compliant — the delta spec asks only for placeholder rejection — and it is not a new gap
(SUGGESTION-35 carried it forward as documented-but-unenforced). It changes severity because it is now
**load-bearing**: a mechanism that overrides a validation gate has been added, and the sole thing standing
between it and a fabricated approval is a review gate that is off. The first version of `bdb6cc8` shipped
exactly that fabrication (`acknowledged_by: "Jorge Alonso"` for a review nobody performed) and was caught by
a human reading the diff, not by any check in this repository. Nothing added since would catch it either.

### WARNING-40 — `apply-progress.md` records none of the three commits, and Strict TDD is active

`grep` over `openspec/changes/phase-1-indicator-page/apply-progress.md` finds no `4b20ca7`, no `bdb6cc8`, no
`1f856e2`, no "CRITICAL-27", no "CRITICAL-28", no "acknowledg" in any casing. The record ends with a lesson
about a documentation pass drifting from an implementation pass — and has drifted again, past an entire new
capability (`validation/acknowledgement.go`, `config/acknowledgement.go`, `postgres/acknowledgement.go`,
migration `0006`, `config/reconocimientos.yaml`, a new delta-spec requirement) and both blocker
remediations.

Strict TDD verification is supposed to validate the TDD Cycle Evidence table against reality. There is no
row for any of the 2,328 lines of new test code. I validated the new tests directly instead (§F), and they
are good — but archiving now freezes a narrative that omits the three most consequential commits in the
change.

### WARNING-41 — "A failed rebuild raises an alert **immediately**" is substituted, not implemented

`pipeline-operations` scenario: *"GIVEN a rebuild dispatched by a successful ingestion that fails, WHEN the
failure is observed, THEN an alert is raised naming the ingestion run and the build failure."*

Nothing observes a failed CI rebuild. `alerting.DispatchFailed` fires when the **POST** fails, which is a
different event. A failed `rebuild.yml` run produces no callback; it is caught only by
`RebuildLatencyBreached` once the 30-minute budget elapses. That is a good substitution and it closes
CRITICAL-28's link 3 — but "immediately" is not what happens, and the substitution is disclosed only in a
comment inside `.github/workflows/rebuild.yml` ("the running container's own deploy-completed watchdog keeps
alerting until it does"), not in the spec, the design or `apply-progress.md`.

### WARNING-30 — `ine/envelope.go` nil-value crash class, still disclosed only in a commit message

Re-checked at `1f856e2`. `envelope.go:38/87` unchanged; `grep` finds no code comment, no test, no openspec
entry. The reasoning for why the INE fix must differ from the Eurostat one still exists only in the body of
commit `befa81f`. **OPEN, unchanged.**

---

## E. Prior findings, re-adjudicated

| ID | Summary | Status this pass | Evidence |
|---|---|---|---|
| CRITICAL-1 | Every "Exportar CSV" link 404s | **CLOSED** | Unchanged |
| CRITICAL-2 | The blocking budget gate cannot fail | **CLOSED** | Unchanged; the *shape* recurs as CRITICAL-37 |
| CRITICAL-3 | 37 inlined Spanish strings | **CLOSED** | Unchanged |
| CRITICAL-4 | Export artifact carries no page-state | **CLOSED** | Unchanged; correctly extended for the new `succeeded-with-acknowledgement` outcome |
| CRITICAL-15 | Container publishes synthesised fixture as INE statistics | **CLOSED** | Unchanged; the Dockerfile guard still refuses a source-less build |
| CRITICAL-16 | Failed-ingestion publish path untested | **CLOSED** | Unchanged |
| CRITICAL-23 | The change does not exist in the repository | **CLOSED** | Clean tree at `1f856e2`, PR #1 open, four jobs green at that sha |
| **CRITICAL-27** | Build silently drops a frozen slug | **CLOSED** | `routes.ts` iterates `FROZEN_INDICATOR_SLUGS` and throws; `missing-slug-fails-build.test.ts` drives a real `npm run build` against a three-part removal with a **paired control build** (case 2, exit 0, all six pages), so case 1 cannot be green for an unrelated reason. Verified: both cases pass; a real build takes ~1.5s, so the timings are genuine. The rejected fourth page state is reasoned correctly — the spec separately forbids hiding the chart. |
| **CRITICAL-28** | Publish loop open at four links | **CLOSED**, all four | L1: `.github/workflows/rebuild.yml` subscribes `repository_dispatch: [rebuild]` and calls `deploy.yml`, which declares `workflow_call: {}` — the seam checks out. L2: `APP_REBUILD_DISPATCH` with three states; required-but-unconfigured returns a *failing* dispatcher, taking the existing alert branch, never `nil`; compose defaults `app` to `required` and forwards `GITHUB_DISPATCH_*`. L3/L4: the Dockerfile stamps `/web/build-manifest.json` outside `dist/` (correct — the volume shadows `dist/data-derived`), and `RebuildLatencyBreached` compares it against the live artifact. Real dispatch round trip remains unprovable here; disclosed. |
| **WARNING-29** | Run log claims a dispatch that did not happen | **CLOSED** | `publishOutcomeMessage` branches on all three states; `DispatchSkipped` added to `PublishResult` precisely because `DispatchedAt == nil` could not discriminate. |
| **WARNING-31** | `SeverityBlockRequiresSignoff` names a mechanism that does not exist | **CLOSED** | The registry is severity-agnostic; `TestGateWithAcknowledgements_ResolvesBlockRequiresSignoffAlike` passes. Pass 4 called this a WARNING; that was the right call then and the resolving half now exists. |
| WARNING-5,6,7,8,9,10,11,17,18,24 | (pass 2/3) | **CLOSED** | Unchanged |
| **WARNING-30** | INE nil-value disclosure durability | **OPEN** | Re-verified above |
| SUGGESTION-19 | Workbench CSV href layout | **OPEN** | Unchanged |
| SUGGESTION-20 | Spec silent on the dateless validation banner | **OPEN** | Unchanged |
| SUGGESTION-21 | No custom-range e2e on a real route | **OPEN** | Unchanged |
| SUGGESTION-22 | Workflows never observed on a runner | **CLOSED** | Four jobs green at `1f856e2` |
| SUGGESTION-25 | Copy-scan regex blind spot | **OPEN** | Unchanged |
| SUGGESTION-26 | Test output inside the source tree | **CLOSED** | Unchanged |
| SUGGESTION-32 | `befa81f` commit body overstates | **OPEN** | Unchanged |
| SUGGESTION-33 | Container bring-up has no automated coverage | **OPEN** | Unchanged |
| SUGGESTION-34 | Record drift | **PARTLY OPEN** | Engram/`tasks.md` count drift persists (138 vs 191); `openspec/config.yaml:92` still declares `go test ./...` while CI runs `-race -count=1`. Escalated in substance by WARNING-40. |
| SUGGESTION-35 | Four-eyes documented, unenforced | **ESCALATED** | Now WARNING-39: still 404, and now load-bearing |

---

## F. Strict TDD

### TDD Compliance

| Check | Result | Details |
|---|---|---|
| TDD Evidence reported | ⚠️ | Table exists for the original 191 tasks; **absent for all three new commits** (WARNING-40) |
| All tasks have tests | ✅ | 191/191 checked, 0 unchecked |
| RED confirmed (test files exist) | ✅ | 8 new test files (2,328 lines) + 5 modified, all present |
| GREEN confirmed (tests pass) | ✅ | Every one re-executed here; all pass |
| Triangulation adequate | ✅ | The registry alone carries 14 gate cases, 11 config cases, 3 Postgres cases and 7 end-to-end cases |
| Safety net for modified files | ✅ | Pre-existing suites re-run and green |

**TDD compliance: 5/6.**

### Test layer distribution (new work only)

| Layer | Tests | Files | Tools |
|---|---|---|---|
| Unit (pure) | ~48 | 4 (`validation/acknowledgement_test.go`, `config/acknowledgement_validate_test.go`, `scheduler/watchdog_test.go`, `web/test/indicator/routes.test.ts`) | Go testing, Vitest |
| Integration (real Postgres / real pipeline / real process) | ~22 | 4 (`ingestion/acknowledgement_e2e_test.go`, `postgres/acknowledgement_test.go`, `cmd/.../rebuild_dispatch_test.go`, `cmd/.../schedule_rebuild_watchdog_test.go`) | testcontainers-style tx harness |
| Build-level (real `npm run build` subprocess) | 2 | 1 (`web/test/export/missing-slug-fails-build.test.ts`) | Vitest + `child_process` |
| **Total (new)** | **~72** | **9** | |

The build-level layer is the one that matters: it is the layer pass 4 said was missing, and it is now present
with a control.

### Assertion quality

Scanned all 8 new and 5 modified test files for tautologies, orphan empty checks, type-only assertions,
ghost loops, smoke-only tests and mock-heavy ratios. **Zero hits.** Two observations, both benign:

- `routes.test.ts:87` uses a bare `.toThrow()`, immediately followed by a case that asserts the message
  content — acceptable pairing.
- `missing-slug-fails-build.test.ts` case 1 asserts non-zero exit **and** four distinct message properties
  **and** the absence of the output directory, then case 2 pins the failure to the removed slug. This is the
  opposite of a trivial assertion.

**Assertion quality: ✅ All assertions verify real behaviour.**

### Quality metrics

**Linter/vet**: ✅ `go vet` clean, `gofmt -l` empty.
**Type checker**: ✅ `astro check` — 0 errors, 0 warnings, 2 hints across 97 files.
**Coverage**: ➖ no coverage threshold configured for this change; skipped, not a failure.

---

## G. Correctness and coherence

| Claim | Status | Evidence |
|---|---|---|
| Go suite is race-free | **Pass** | `-race -count=1` clean locally and on the runner |
| `validate-config` accepts the shipped tree | **Pass** | exit 0 |
| An unsigned acknowledgement moves nothing | **Pass** | Two layers, verified independently with a control (§B.3) |
| An acknowledgement cannot be widened in configuration | **Pass** | 8 adversarial mutations, all rejected (§B.1) |
| A stale acknowledgement fails closed and says why | **Pass** | `acknowledgement-stale` blocks and names both values |
| An acknowledged publish is distinguishable | **Pass** | `succeeded-with-acknowledgement` outcome + `acknowledgements` log attr + no `failed_rules` |
| One acknowledgement resolves exactly one finding | **Fail** | WARNING-38 — demonstrated resolving two |
| A fabricated plausible signature is rejected | **Fail** | WARNING-39 — `validate-config: ok` |
| The dispatch reaches a receiver | **Pass** | `rebuild.yml` ← `repository_dispatch`; `deploy.yml` declares `workflow_call` |
| An undispatched rebuild alerts operators | **Pass** | failing dispatcher → existing alert branch |
| A deploy-completed instant exists | **Pass** | `/web/build-manifest.json`, outside `dist/`, changed only by a deploy |
| A failed rebuild alerts *immediately* | **Fail** | WARNING-41 — budget-delayed |
| All six frozen routes exist in a production build | **Fail** | CRITICAL-37 — the build emits none |
| New data reaches a reader's page | **Pass, unprovable end to end** | The loop is closed in code; no VPS exists to run it. Disclosed. |

### Coherence (design)

| Design decision | Honoured | Note |
|---|---|---|
| D-1 the artifact IS `/data-derived/` | Yes | Unchanged |
| D-2 publish-after-cycle wiring | **Yes** | The receiving job planned at `design.md:611` now exists; pass 4's "partially" is closed |
| D-3 unknown tokens fail closed | Yes | Extended consistently to the unsigned-record case |
| D-4 periodicity over the whole payload | Yes | Unchanged |
| D-5/D-6 web layering | Yes | `routes.ts` correctly placed in `src/lib/` so the rule is directly assertable |
| New capability not in `design.md` | **No** | The acknowledgement registry (a new table, a new config file, a new migration) has no design entry — part of WARNING-40 |

---

## H. Archivability — the plain answer

**No. Not archivable. One thing blocks it, and it is a human signature.**

**The argument for archiving anyway**, stated fairly: every defect is closed. The unsigned draft is not an
omission — it is the mechanism working. A pipeline that says "a human decision is pending" instead of
manufacturing the approval is behaving exactly as designed, and refusing to archive because the design
refused to lie creates pressure toward the fabrication that was already caught once in this very commit.
Archiving records work; it does not deploy it.

**Why that argument loses.** Archive freezes the claim that the change delivered what it specified. The
change specified six indicator pages at six permanently frozen permalinks. At `1f856e2` the pipeline can
produce five, and the build — correctly — refuses to ship five, so it produces none. `openspec archive`
would move a spec into `openspec/specs/` asserting a MUST that the system demonstrably cannot satisfy, while
every CI signal reads green because no CI path is ever handed production's artifact shape. That is a worse
falsehood than the one the acknowledgement mechanism refuses to tell, and it is the same class of falsehood.

**What must happen — exactly one of these. None of them is code.**

1. **A named human reviews and signs `config/reconocimientos.yaml`.** Open
   `https://www.ine.es/daco/daco42/daco4211/epa0220.pdf` (verified reachable, 1,233,410 bytes, both strings
   present), confirm 2020-Q2 = 18607.2 and that the fall is COVID-19 rather than a methodology change or a
   parser fault, then replace `signature_status` / `drafted_by` / `todo` with `acknowledged_by` (full name)
   and `acknowledged_on`. The record's own `todo` field already spells out all three steps and the two
   edits to `note_md`. After that: `ocupados-epa` publishes as `succeeded-with-acknowledgement`, the artifact
   carries six series, the build emits six pages, and CRITICAL-37 closes with no commit to any Go or
   TypeScript file.
2. **Or the same human rejects it**, deletes the entry whole (the `todo` says so: "un registro rechazado no
   se deja a medias"), and chooses a different remedy for `ocupados-epa` — which then needs its own SDD
   cycle, because neither raising the threshold nor recording a false methodological break is acceptable and
   both were correctly ruled out.

**Do not**: raise `max_delta_abs`, add a break to `config/rupturas.yaml`, weaken `resolveIndicatorRouteSlugs`,
or let an agent sign the record. Each of those was considered and correctly rejected in this change's own
reasoning, and the last one was already attempted and caught.

**Recommended alongside, but not blocking the signature**: WARNING-40 (bring `apply-progress.md` and
`design.md` up to `1f856e2` before archiving, since archive freezes them) and CRITICAL-37's structural half
— one CI path that builds from an artifact produced by an ingestion running the **real** `config/series/*.yaml`
thresholds, so that "the production build works" stops being an unmeasured claim. WARNING-38 and WARNING-39
are follow-ups, not archive blockers.

---

## Verdict, restated

**FAIL. Do not archive.**

`CRITICAL-27` and `CRITICAL-28` are closed and were closed well. The acknowledgement registry is the best
piece of work in this change: it is scoped so narrowly that eight deliberate widening attempts all failed at
the schema, its allowlist is pinned by a test that runs the real rules, its staleness guard is the right one
for the right reasons, and it refused to fake the signature it was waiting for — twice, in two independent
layers, which I verified myself with a control rather than taking the tests' word for it.

The single blocker is the consequence of that honesty, and the remedy is the act the mechanism is waiting
for: **a named human must review and sign, or reject and delete, `config/reconocimientos.yaml`'s one draft.**
Until then, six frozen permalinks cannot be built, and the change cannot honestly be archived as delivered.

1 CRITICAL, 5 WARNING (2 carried), 12 SUGGESTION (10 carried, 2 new).

**New SUGGESTIONS**

- **SUGGESTION-42** — The neighbouring-period residual disclosed in `acknowledgement.go` is narrower than
  stated. For a delta-based finding at period P, a revision of P-1 changes the magnitude while the pin at P
  holds — but revising P-1 at any acknowledged period more than `max_backward_periods` (4) behind the latest
  raises its own `rule4-revision` finding at P-1, which the P-scoped acknowledgement cannot resolve, so the
  run blocks anyway. The residual bites only inside the revision window. Worth recording so a future reader
  does not over-fear a case rule 4 mostly covers.
- **SUGGESTION-43** — `scripts/smoke-test.sh`'s "Usage: ./scripts/smoke-test.sh" header does not mention
  that step 2 (`docker compose build app`) needs `EXPORT_URL` or `EXPORT_DIR`. `ci.yml` stages
  `web/data-derived` and sets `EXPORT_DIR: data-derived`; the script does not, so running it as documented
  fails on the Dockerfile's CRITICAL-15 guard. The guard is right and the message is good; the script's own
  usage line is stale. This is the seam between `52a2151`'s hermetic-smoke-test work and the artifact-source
  guard.

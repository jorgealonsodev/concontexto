```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:7b2811b578bb9f1e76164acdb07ab725611a472a3200c8a7e37579c2eafb01c0
verdict: fail
blockers: 1
critical_findings: 1
requirements: 58/65
scenarios: 105/114
test_command: go test -count=1 ./...
test_exit_code: 0
test_output_hash: sha256:7b2811b578bb9f1e76164acdb07ab725611a472a3200c8a7e37579c2eafb01c0
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report — pass 3 (post-batch-3/batch-4 re-verification)

**Change**: phase-0-data-foundations
**Version**: N/A (greenfield; `openspec/specs/` empty, all deltas ADDED-only)
**Mode**: Strict TDD
**Scope**: PRD Fase 0, milestones 0.1-0.7. Fase 1-4 surfaces are correctly absent and are not reported as missing.
**Supersedes**: pass 2 (FAIL, 2 CRITICAL). Pass 1 and pass 2 findings and their resolution are preserved in the cross-pass ledger below and in the retained pass-2 appendix.

### Recommendation: DO NOT ARCHIVE — one blocker, and it is the last structural one

This is a different verdict from pass 2 in kind, not just in count.

**Both pass-2 CRITICALs are genuinely closed, and I proved each by mutation.**
C6's 24-hour rule now survives a restart, and C7's `serve` wiring now fails a
test when deleted. Every batch-3 and batch-4 fix I probed is falsifiable. The
FAILING scenario is gone: **0 FAILING, down from 1**. Compliance rose 103/114 to
105/114.

**One CRITICAL remains, and it is new: C8.** The composition root
(`startScheduler`) has **0.0 % statement coverage** and never executes in any
test. Three separate seams inside it and inside `cmdIngest` can each be broken
with the *entire* suite still green — and one of those three mutations
**silently restores the exact C6 defect that made pass 2 fail**.

That is why this blocks. Not because the shipped code is wrong — I read it and
it is correct — but because the remediation for the finding that failed the last
pass is itself invisible to the test suite. Signing that off would repeat the
precise error passes 1 and 2 caught: apply-progress claiming a wiring is proven
when nothing would notice its removal.

**This remediation terminates.** `dispatch` is 100 % covered and `main` is the
only thing above it, so `startScheduler` and `cmdIngest` *are* the outermost
layer. There is no further ring for the defect to escape to. One bounded work
unit closes C8 for good.

### Headline movement since pass 2

| Pass-2 finding | Claimed by batch 3/4 | My verdict this pass |
|---|---|---|
| C6 24h rule defeated in production | fixed, mutation-proven | **CLOSED** — behaviour correct, mutation M3 falsifiable |
| C7 `serve` wiring unpinned | fixed via `schedulerStarter` seam | **CLOSED as stated** — mutation M1 fails on deletion |
| W11 non-atomic hash-listing write | fixed | **CLOSED** — mutation M5 falsifiable |
| W12 `MaxResponseBytes` never read | fixed (all 3 adapters) | **CLOSED** — verified wired for ine/eurostat/xlsx |
| W7 dead spy assertion | deleted | **CLOSED** — assertion gone, only the explanatory comment remains |
| W15 subtest miscount | fixed | **CLOSED** — gate reports 8, docs now say 8 |
| INE response ceiling (batch 4) | added + non-retryable | **CLOSED** — mutation M6 fails 2 tests |
| — | — | **C8 NEW** — composition root at 0 % coverage |

### Build & Tests Execution — verbatim

All commands run from the **repository root** (ADR-5).

**`go test -count=1 ./...`** — exit 0

```text
ok  	github.com/jorgealonsodev/concontexto	0.005s
ok  	github.com/jorgealonsodev/concontexto/app/cmd/concontexto	18.218s
ok  	github.com/jorgealonsodev/concontexto/app/internal/adapters/config	0.015s
ok  	github.com/jorgealonsodev/concontexto/app/internal/adapters/eurostat	0.008s
ok  	github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore	0.064s
ok  	github.com/jorgealonsodev/concontexto/app/internal/adapters/ine	0.010s
ok  	github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres	3.817s
ok  	github.com/jorgealonsodev/concontexto/app/internal/adapters/xlsx	0.224s
ok  	github.com/jorgealonsodev/concontexto/app/internal/guard	0.019s
ok  	github.com/jorgealonsodev/concontexto/app/internal/healthcheck	0.007s
ok  	github.com/jorgealonsodev/concontexto/app/internal/httpserver	0.048s
ok  	github.com/jorgealonsodev/concontexto/app/internal/indicators	0.002s
ok  	github.com/jorgealonsodev/concontexto/app/internal/ingestion	3.162s
ok  	github.com/jorgealonsodev/concontexto/app/internal/ingestion/alerting	0.004s
ok  	github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness	0.004s
ok  	github.com/jorgealonsodev/concontexto/app/internal/ingestion/pipelinelog	0.004s
ok  	github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr	0.003s
ok  	github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation	0.056s
ok  	github.com/jorgealonsodev/concontexto/app/internal/migrate	0.003s
ok  	github.com/jorgealonsodev/concontexto/app/internal/probe	0.005s
ok  	github.com/jorgealonsodev/concontexto/app/internal/scheduler	0.004s
?   	github.com/jorgealonsodev/concontexto/app/migrations	[no test files]
```

**`go test -short -count=1 ./...`** — exit 0. Same 21 packages `ok`;
`app/cmd/concontexto` drops 18.218s -> 0.317s, `adapters/postgres` 3.817s ->
0.062s, `ingestion` 3.162s -> 0.058s. Testcontainers work is genuinely
`-short`-guarded (ADR-3).

**`go vet ./...`** — exit 0, no output.

**`gofmt -l .`** — exit 0, no output.

**`./scripts/check-env-example.sh`** — exit 0

```text
check-env-example: OK — 6 variable(s) documented: APP_DATA_ROOT APP_SCHEDULE_INTERVAL DATABASE_URL PORT POSTGRES_ADDR STATIC_ROOT
```

**`go run ./app/cmd/concontexto validate-config`** — exit 0

```text
validate-config: ok
```

**`go test ./app/cmd/concontexto/ -run TestFase0ClosureGate -v`** — exit 0,
**8 subtests, all PASS**

```text
--- PASS: TestFase0ClosureGate (0.00s)
    --- PASS: TestFase0ClosureGate/0.1_repository_ci_environments (0.00s)
    --- PASS: TestFase0ClosureGate/0.1_deploy_step_gated_on_secrets_and_honestly_disclosed (0.00s)
    --- PASS: TestFase0ClosureGate/0.2_ine_six_series_full_history_loaded_and_validated (0.00s)
    --- PASS: TestFase0ClosureGate/0.3_eurostat_three_harmonized_datasets_loaded (0.00s)
    --- PASS: TestFase0ClosureGate/0.4_revision_creates_new_version_without_overwriting (0.00s)
    --- PASS: TestFase0ClosureGate/0.5_break_event_registry_populated (0.00s)
    --- PASS: TestFase0ClosureGate/0.6_xlsx_real_ingestion_and_malformed_suite (0.00s)
    --- PASS: TestFase0ClosureGate/0.7_licence_resolution_per_source_attribution_closed (0.00s)
```

**`./scripts/smoke-test.sh`** — exit 0, all 15 steps passed (Docker available)

```text
==> 10. docker exec healthcheck (shallow) inside the app container
  healthcheck (shallow) exited 0 OK
==> 11. docker exec healthcheck --deep while postgres is up
  healthcheck --deep exited 0 while postgres is up OK
==> 12. Stopping postgres: --deep must fail, shallow must still pass
  healthcheck --deep exited non-zero while postgres is down OK
  healthcheck (shallow) still exited 0 while postgres is down OK (DB outage must not kill a static site)
==> 14. Waiting for the app image's own exec-form HEALTHCHECK to report healthy
  app is healthy (waited 0s)
==> 15. pg_dump -> drop -> pg_restore round trip (scripts/backup/pg_dump.sh)
  pg_dump -> drop -> pg_restore round trip OK
SMOKE TEST: ALL CHECKS PASSED
```

**`go test -tags live ./app/internal/probe/... -v`** — not run. No outbound
network in this sandbox. Unchanged, disclosed, accepted.

**Coverage** (`app/cmd/concontexto`, measured this pass): total 62.7 % of
statements. Per-function figures for the composition layer are the evidence for
C8 and appear in that finding.

### Per-CRITICAL closure verdict — mutation evidence

Every mutation below was applied to the working tree, compiled, executed, then
reverted and byte-verified against a pre-mutation SHA-256 manifest.

| # | Target | Mutation | Result | Verdict |
|---|---|---|---|---|
| M1 | **C7** `cmdServe` wiring | replaced `schedulerStarter(ctx, stderr)` with `_ = schedulerStarter` | **FAIL** — `serve_test.go:85: cmdServe did not start the scheduler within the deadline` | **C7 call site now pinned** |
| M2 | **C6** production seed argument | `startScheduler` passes `nil` instead of `seedLastSuccess` to `runScheduler` | **ENTIRE SUITE GREEN**, exit 0 | **C8 — silently restores C6** |
| M3 | **C6** seeding logic | removed the `if !seeded[id] { … seedLastSuccess(ctx, id) }` block from `runScheduler` | **FAIL x2** — `expected seedLastSuccess to be called exactly once for the cold-start source, got 0` | **C6 logic pinned** |
| M4 | **C7** seam binding | `var schedulerStarter = startScheduler` -> a no-op closure | **ENTIRE SUITE GREEN**, exit 0 | **C8 — seam swallows its own defect** |
| M5 | **W11** atomic publish | `writeFileAtomically` -> `os.WriteFile` | **FAIL** — `the pre-opened reader observed the SECOND publish's content … exactly the W11 defect` | W11 pinned |
| M6 | **batch 4** INE ceiling | `readWithCeiling` -> `io.ReadAll` | **FAIL x2** — `expected a response-too-large error`; `it succeeded, meaning the hardcoded default was used instead` | batch 4 pinned |
| M7 | **C3** public listing path | `cmdIngest` sets `publicHashPath := ""` | **ENTIRE SUITE GREEN**, exit 0 | **C8 — third instance** |

**C6 — CLOSED.** `runScheduler` now consults `seedLastSuccess` at most once per
source per invocation (`schedule.go:154-159`), and `startScheduler` binds it to
`postgres.LastSuccessfulDownloadAttempt` (`schedule.go:233-240`), which reads
`download_attempt` and therefore survives restarts. Two tests distinguish
"persisted success is recent -> no incident" from "persisted success is 25 h old
-> exactly one `KindSourceDown` incident", and both assert `seedCalls == 1` so
neither can pass with the seeding removed — M3 proves that. The false
"down for over 24h0m0s" alert on a freshly started process is gone.

**C7 — CLOSED as stated.** M1 fails the moment the wiring line is removed. The
`schedulerStarter` package-level seam plus `TestCmdServe_StartsTheScheduler`
does what batch 3 claimed, and the claim was not overstated this time.

**C1, C2, C3, C5 — remain closed** (mutation-verified in pass 2; C3's inner call
re-confirmed unchanged, though see C8/M7 for its outer path).

### C8 — the seventh, eighth and ninth instances of the pattern

The brief asked me to re-run the unwired-function heuristic **and its inverse**
— a function that *is* called but whose production call path passes different
arguments than its tests do. The inverse is where everything was hiding.

**Objective evidence — `go tool cover -func` on `app/cmd/concontexto`:**

```text
schedule.go:183:  startScheduler       0.0%
schedule.go:71:   scheduleInterval     0.0%
ingest_cmd.go:68: appDataRoot          0.0%
schedule.go:140:  runScheduler       100.0%
ingest_cmd.go:81: cmdIngest           68.6%
dispatch.go:22:   dispatch           100.0%
```

`startScheduler` is **never executed by any test**. Its only appearances in
`_test.go` files are two prose comments. Everything it composes — the pgxpool,
the embedded config load, `appDataRoot()`/`staticAssetRoot()` resolution, the
per-source `Runner` map, `newOp`, and the `seedLastSuccess` closure that is the
entire C6 fix — is unverified.

Three mutations confirm the consequence:

1. **M2 — the C6 regression is invisible.** Change one argument from
   `seedLastSuccess` to `nil` and production reverts to exactly the pass-2
   defect: `freshness.Resolve(nil, …)` returns `StateFailed`, the first failed
   cycle after any restart fires a false `source-down`. **Suite green.** This is
   the same shape as the original C6 (test injects non-nil, production passes
   nil), one layer further out.
2. **M4 — the C7 seam swallows its own defect.** Rebinding
   `var schedulerStarter` to a no-op leaves the suite green, because the only
   test that touches the seam *replaces* it. The test proves `cmdServe` calls
   whatever the variable holds, not that the variable holds the real scheduler.
3. **M7 — C3's outer path is unpinned.** `cmdIngest` resolves `publicHashPath`
   at `ingest_cmd.go:130`; setting it to `""` disables public hash-listing
   publication entirely (PRD §14.2's traceability obligation), because
   `ingest.go:192` gates publication on both paths being non-empty.
   **Suite green.**

`scheduleInterval` at 0 % is a smaller sibling: `APP_SCHEDULE_INTERVAL` is a
documented, `check-env-example`-enforced variable whose parsing and fallback are
never exercised.

**Severity — CRITICAL, and why.** The shipped code is correct: I read
`startScheduler` line by line, and the containerised smoke run executes its
synchronous body (compose sets `DATABASE_URL`; the Dockerfile `CMD` is `serve`),
so a crash there would have failed smoke steps 8/9/14. This is not a live
behaviour defect. It is CRITICAL because **the fix for the finding that failed
the previous pass has no regression signal**, in a change with a documented
seven-instance history of exactly this failure mode, two of those instances
introduced by fixes for the others. "Correct today, undetectable tomorrow" is not
a defensible terminal state for C6 specifically.

**Why it terminates.** `dispatch` is 100 % covered and `main` is the only caller
above it. `startScheduler` and `cmdIngest` are the composition root — there is no
outer ring left. Closing C8 is one bounded work unit: execute `startScheduler`
with an injected pool/config (or extract its composition into a testable
function), assert it passes a non-nil seed, assert the default `schedulerStarter`
binding, and pin `cmdIngest`'s path resolution.

### The unwired-function heuristic, re-run whole-codebase

Scanned all 199 functions declared in non-test `.go` files, counting
non-comment references in production source. **21 have zero production
references** (down from pass 2's 30 candidates).

| Function(s) | Assessment |
|---|---|
| `seedLastSuccess`, `schedulerStarter`, `writeFileAtomically`, `readWithCeiling` (all 3 adapters), `resolveBreaks`, `ReconcileDimensions`, `PublishRawFileHashListing`, `startScheduler`, `alerting.DefaultSink`, `pipelinelog`, `LastSuccessfulDownloadAttempt` | **all genuinely wired.** Batch 3/4 closed W12: `WithMaxResponseBytes` is now threaded from `src.API.MaxResponseBytes` for **ine, eurostat and xlsx** (`ingest_cmd.go:164/173/182`) and no longer appears in the unwired set. |
| `probe.Targets`, `eurostat.FetchProbe` | wired via `.github/workflows/probe.yml` (`go test -tags live`), not the Go call graph. Legitimate; never executed (accepted). |
| `postgres.SeriesFreshness` -> `SourceFreshness` | **still zero production callers.** Batch 3 wired the lower-level `LastSuccessfulDownloadAttempt` instead — the better primitive, since `scheduler.Runner` owns the `freshness.Resolve` decision. These two are the Fase 1 read-side (amber banner) surface. Correctly absent; now dead rather than missing. |
| `ReconcileBreaks`, `ReconcileEvents` | superseded by the single-transaction `ReconcileEditorial`. **Dead code** (`ReconcileEvents` is new to this list). |
| `(*ine.Client).FetchSeries` | not part of the `indicators.SourceClient` port. Dead. |
| `eurostat.WithBackoff`, `WithMaxAttempts`, `WithSleep`; `ApplyGateWithWriter`, `SetDefaultSink`, `migrate.ExecutionCount` | legitimate test seams, documented as such. |
| `guard.scanConfigForRetiredIdentifiers` | a repository guard that runs *as* a test by design. Correct. |
| `ResolveAttribution`, `ResolveProvenance`, `VintageAsOf`, `RollbackRun`, `filestore.Read`, `ListEvents`, `ListMappings`, `ListSeriesBreaks` | read-side / operator surfaces whose consumers are Fase 1. Out of scope, correctly absent. |

**Inverse heuristic** (production passes different arguments than tests): three
positives, all in the composition root — M2, M4, M7 above. I cleared the other
high-risk candidates: `scheduler.NewRunner` genuinely sets
`AlertSink: alerting.DefaultSink()` (which is `LogSink{}`, a real destination),
and the `nil` at `ingest.go:208` is the decode-failure branch where nil
candidates is correct — the real publish path at `ingest.go:254` passes them.

### Spec Compliance Matrix — per-capability verdict

Requirement and scenario counts recomputed this pass directly from the ten
`specs/{capability}/spec.md` mirrors (`### Requirement` / `#### Scenario`
headings): **65 requirements, 114 scenarios**, matching the validator input.

| # | Capability | Req | Scen | COMPLIANT | PARTIAL | UNTESTED | FAILING | Verdict |
|---|---|---|---|---|---|---|---|---|
| 1 | platform-runtime | 10 | 14 | 9 | 2 | 3 | 0 | PASS WITH WARNINGS — deploy scenarios unexecutable |
| 2 | data-model-vintages | 7 | 11 | 11 | 0 | 0 | 0 | PASS |
| 3 | data-validation | 8 | 15 | 14 | 1 | 0 | 0 | PASS WITH WARNINGS |
| 4 | editorial-config | 9 | 15 | 15 | 0 | 0 | 0 | PASS |
| 5 | pipeline-operations | 4 | 8 | 6 | 0 | 2 | 0 | PASS WITH WARNINGS — 24h rule now holds |
| 6 | raw-file-archive | 4 | 9 | 9 | 0 | 0 | 0 | PASS |
| 7 | source-attribution-licensing | 4 | 6 | 6 | 0 | 0 | 0 | PASS |
| 8 | source-ingestion-eurostat | 7 | 10 | 10 | 0 | 0 | 0 | PASS |
| 9 | source-ingestion-ine | 6 | 10 | 9 | 1 | 0 | 0 | PASS WITH WARNINGS |
| 10 | source-ingestion-xlsx | 6 | 16 | 16 | 0 | 0 | 0 | PASS |
| | **Total** | **65** | **114** | **105** | **4** | **5** | **0** | |

**Scenario trajectory**: 98/114 (pass 1) -> 103/114 (pass 2) -> **105/114**.
PARTIAL 11 -> 5 -> **4**. UNTESTED 5 -> 5 -> **5**. FAILING 0 -> 1 -> **0**.

**Requirements: 58/65.** Derived by mapping every scenario to its `### Requirement`
heading and marking a requirement compliant only when all its scenarios are.
Note this corrects pass 2's stated `55/65`, which appears to have undercounted;
the seven non-compliant requirements are platform-runtime R5/R7/R8/R9,
data-validation "Rule 4 — revision consistency", source-ingestion-ine "Six
canonical series with pinned identifiers", and pipeline-operations "Synthetic
daily probe against every endpoint".

### Re-assessment of pass 2's 5 PARTIAL scenarios

| Capability | Scenario | Pass 3 status |
|---|---|---|
| platform-runtime | Serving pages issues no queries | **now COMPLIANT** — W7 closed. The permanently-dead `spy.n.Load() != 0` assertion is deleted; only the explanatory comment remains, and the property is carried by the mutation-verified import guard (pass-2 M6). Deleting the assertion was the correct remedy, and it was applied. |
| platform-runtime | Boot does not migrate | still PARTIAL — `migrate.ExecutionCount()` proxy counter, no DB, no schema version. Unchanged. |
| platform-runtime | Postgres unreachable from outside the stack | still PARTIAL — smoke step 5 asserts no host port binding but never attempts a connection from outside. Smoke suite re-run green this pass. |
| data-validation | A deep revision blocks publication | still PARTIAL — W4 deferred; the block half works and is tested, the sign-off release half does not exist. |
| source-ingestion-ine | All six series load full history and validate | still PARTIAL — trimmed fixtures are spec-mandated, so the tension is spec-internal; honest test naming (W5) does not make the scenario compliant. |

### Re-assessment of the 5 UNTESTED scenarios

All five are unchanged and all five are on the known-and-accepted list.

| Capability | Scenario | Pass 3 status |
|---|---|---|
| platform-runtime | Pushed commit deploys hello-world | still UNTESTED — `deploy.yml` exists and is honestly gated on `PORTAINER_WEBHOOK_URL`; the closure gate asserts the file and its gate but deliberately cannot assert a deploy happened. Milestone-0.1 criterion genuinely unmet, and the repository says so. |
| platform-runtime | Failing tests block deploy | still UNTESTED — `workflow_run` + `conclusion == 'success'` is structurally correct; no test exercises it. |
| platform-runtime | Single approval is rejected on a config change | still UNTESTED — `.github/CODEOWNERS` line 17 carries `@TODO-second-config-reviewer`; zero commits, no remote. Unenforceable until a second maintainer exists (PRD §18). |
| pipeline-operations | The probe detects a broken identifier | still UNTESTED — `live` suite never executed, no outbound network. |
| pipeline-operations | The probe asserts shape, not values | still UNTESTED — same. |

### Deferred items — do the reasons still hold?

| Item | Stated reason | Pass 3 judgement |
|---|---|---|
| **W4** rule 4 sign-off unimplemented | "design-level work" | **Still sound.** The safety-critical half blocks and is tested; only the release half is missing, so a flagged run fails closed, never open. Operability gap, not an integrity risk. |
| **W10** partial publish on mid-publish DB failure | "cross-cutting transaction-boundary change with real risk" | **Still sound, defect confirmed unchanged.** `applyGate` (`gate.go:130-140`) loops `writer.WriteRevision` per candidate; a failure at candidate *k* leaves 1…*k*-1 durable **and** returns before `recordRunOutcome`, so the run outcome is never recorded either. Consequence remains slightly elevated now that the scheduler runs unattended. |
| **W7** | superseded | **Now CLOSED, not deferred** — the dead assertion was deleted, which is what pass 2 asked for. |
| **S1** substring denylist for the non-dismissible guard | — | Sound, unchanged. |
| **S2** theatrical archive-before-parsing test | — | Sound, unchanged; real ordering is proven elsewhere. |
| **S3** `.codegraph/` untracked and not git-ignored | — | Sound, **confirmed still true**: `git status` shows `?? .codegraph/` and `.gitignore` contains no `codegraph` entry. |
| **S4** ADR-0006 belongs to its own change | — | Sound, unchanged. |
| **S5** PR 3 needs its 3a/3b/3c split | — | Sound, unchanged. |
| **S6** assert `cmdServe` starts a scheduler | pass 2 suggestion | **Acted on** (that is C7's fix), but only partially — see C8/M4. |

### Non-negotiable business rules — structural enforcement audit

| Rule | Enforced? | Evidence |
|---|---|---|
| Zero DB queries / zero computation at request time (§14.2) | **Yes, structurally** | `NewServer(root fs.FS)` takes no port; import guard mutation-verified in pass 2 (M6). Scheduler is composed **beside** the handler in `cmdServe`, never reachable from it. |
| No external source call at page-request time (§9.2) | **Yes** | Guard covers `adapters/{ine,eurostat,xlsx}`. |
| Validation failure never publishes | Yes | `applyGate` returns before the write loop on `GateBlock`. |
| Raw downloads archived with SHA-256 **before** parsing (§9.1) | Yes | `ArchiveRawFile` (`ingest.go:169`) precedes `client.Decode` (`ingest.go:203`). |
| Hash listing published and served atomically (§14.2) | **Yes — improved** | W11 closed: `writeFileAtomically` (temp + `os.Rename`); mutation M5 falsifiable. Caveat: the *path* reaching it is unpinned (C8/M7). |
| One current row per `(series_id, period)`, DB-enforced | Yes | Partial unique index; mutation-verified in pass 1 against real PostgreSQL 17. |
| Series breaks non-dismissible (P4) | Yes | Reflection guard; mutation-verified in pass 1. |
| Editorial YAML authoritative; DB a projection | **Yes — exception removed** | W12 closed: `max_response_bytes` is now read from config for all three adapters, so editing the YAML has effect. |
| `ReconcileEditorialConfig` never projects a nil date | **Confirmed, still holds** | `reconcile.go` routes nil `Date`/`DateStart` into `Pending*IDs`; `series_break.date` and `event.date_start` are `date NOT NULL` in the DDL as a second line of defence. |
| 24 h amber escalation per source | **Yes — restored** | C6 closed; freshness seeded from `download_attempt`, survives restarts. Mutation M3 falsifiable. Regression signal missing (C8/M2). |

### Known-and-accepted state — honesty check

| Disclosed item | Represented honestly? |
|---|---|
| 7 of 21 break/event entries `date_status: unconfirmed` | **Accurate, independently recounted.** A naive grep returns 9; two of those are prose (`config/README.md:24`, a `#` comment at `rupturas.yaml:12`). The real YAML entries are 7: `gobiernos.yaml:28`, `eventos.yaml:56/71`, `rupturas.yaml:39/56/104/250`. |
| 4 entries `ref_status: pending` | Accurate — exactly 4; `validate-config` enforces the non-pending case (pass-2 M7). |
| `ReconcileEditorialConfig` never projects a nil date | Accurate; re-verified in code and DDL. |
| `@TODO-second-config-reviewer`; four-eyes unenforceable (§18) | Accurate, unchanged (`CODEOWNERS:17`). |
| Milestone 0.1 deploy criterion unmet and the repo says so | Accurate — closure gate subtest and task 1.17 both state it. |
| INE `TipoDato` not surfaced; blocks Fase 1, not Fase 0 | Accurate. |
| `live` probe suite never run | Accurate; `probe.yml` exists and would run it given network. |

### Regression check on the batch-3 / batch-4 fixes

Two of this change's CRITICALs were introduced by fixes, so I assumed a third was
possible and looked for it specifically.

- **No functional regression found.** Full suite green (`-count=1`, uncached),
  `-short` green, `go vet` clean, `gofmt` clean, `validate-config` ok, closure
  gate 8/8, containerised smoke 15/15.
- **Batch 3's C6 fix does not alter existing behaviour**: `seedLastSuccess` is
  consulted at most once per source per `runScheduler` call, and a `nil` seed
  preserves the prior semantics exactly, so every pre-existing scheduler test
  keeps its meaning.
- **Batch 4's non-retryable `errors.As` early-return** is correctly scoped —
  mutation M6 shows the ceiling error fails once rather than retrying five times,
  and `TestFetchRaw` transport-error cases still retry.
- **The one thing batch 3 introduced that I do flag** is not a behaviour
  regression but a *verification* regression: the `schedulerStarter` seam (M4)
  can be rebound to a no-op with the suite green, so the seam introduced to catch
  C7 does not catch its own subversion.

### TDD Compliance

| Check | Result | Details |
|-------|--------|---------|
| TDD Evidence reported | Yes | Cycle tables present for all four batches |
| All tasks have tests | **Qualified** | 144/144 marked; `startScheduler`, `scheduleInterval`, `appDataRoot` at 0 % (C8) |
| RED confirmed | Yes | Every named test file exists; batch 4's RED states are recorded and plausible |
| GREEN confirmed | Yes | 21/21 packages `ok` on `-count=1` |
| Triangulation adequate | Yes | Cold-start recent vs stale is a genuine discriminating pair; the `seedCalls == 1` assertion prevents the "incident for the wrong reason" false pass |
| Safety net for modified files | Yes | Each batch records prior suite state |
| RED-first discipline | Qualified | Disclosed deviations, each compensated by mutation; I independently re-verified seven mutations this pass |

**TDD Compliance**: 5/7 clean, 2 qualified.

### Assertion Quality

| File | Line | Assertion | Issue | Severity |
|------|------|-----------|-------|----------|
| `app/cmd/concontexto/serve_test.go` | 66-100 | `TestCmdServe_StartsTheScheduler` | Proves `cmdServe` calls the seam, not that the seam holds `startScheduler` (M4) | WARNING (C8) |
| `app/cmd/concontexto/serve_test.go` | 48 | `migrate.ExecutionCount() != baseline` | Proxy counter; no DB, no schema version | WARNING |
| `app/internal/httpserver/static_test.go` | 81-88 | blocking `DialContext` on the **client** transport | Constrains the test client, not the server | WARNING |
| `app/internal/adapters/postgres/rawfile_test.go` | 108-118 | in-test `parseCrashed` panic | Demonstrates the test's ordering, not production's | SUGGESTION |

The pass-2 dead-spy assertion is **gone**. No tautologies, ghost loops, or orphan
empty-collection assertions found. `schedule_freshness_test.go`'s
`seedCalls == 1` checks are the strongest new assertions in the change: they are
explicitly designed to fail on the "right answer, wrong reason" path.

**Assertion quality**: 0 CRITICAL, 3 WARNING, 1 SUGGESTION.

### Issues Found

**CRITICAL**

- **C8 — The composition root is at 0 % coverage, and the C6 fix has no
  regression signal.** `startScheduler` (`schedule.go:183`) is executed by no
  test. Three mutations each leave the *entire* suite green: passing `nil`
  instead of `seedLastSuccess` (M2, which silently restores the pass-2 C6
  defect), rebinding `schedulerStarter` to a no-op (M4, which defeats the seam
  built to catch C7), and setting `cmdIngest`'s `publicHashPath` to `""` (M7,
  which disables the PRD §14.2 public hash listing). `scheduleInterval` and
  `appDataRoot` are likewise at 0 %. The shipped code is correct — verified by
  source inspection and by a containerised smoke run that executes
  `startScheduler`'s synchronous body — so this is a verification defect, not a
  live behaviour defect. It is CRITICAL because the remediation for the finding
  that failed the previous pass is itself undetectable if removed, in a change
  with seven prior instances of this exact failure mode. It is bounded and
  terminating: `dispatch` is 100 % covered and `main` is the only layer above,
  so there is no further ring to escape to.

**WARNING**

- **W16 (new) — `postgres.SeriesFreshness` / `SourceFreshness` are now dead
  rather than missing.** Batch 3 correctly wired the lower-level
  `LastSuccessfulDownloadAttempt` instead. These two remain tested against real
  PostgreSQL with zero production callers; their consumer (the Fase 1 amber
  banner) is out of scope. Retain-and-document or remove.
- **W17 (new) — `postgres.ReconcileEvents` joins `ReconcileBreaks` as dead
  code**, both superseded by the single-transaction `ReconcileEditorial`.
- **W13 — stale comment resolved in part.** `eurostat/client.go`'s reference to
  the deleted `stubs.go` was corrected by batch 4. Re-verify no sibling comment
  still claims the wiring "is still pending".
- **W14 — `deploy.yml`'s gate message** still annotates that an unconfigured run
  "builds and pushes the image" when build/push is itself gated off. Honest
  intent, wrong text. Carried forward.
- **W4, W10** — carried forward from pass 1; deferral reasons re-judged sound
  above.
- **Dead code**: `(*ine.Client).FetchSeries` (not part of the
  `indicators.SourceClient` port). Carried forward.

**SUGGESTION**

- S1-S5 carried forward unchanged (substring denylist; theatrical
  archive-before-parsing test; `.codegraph/` untracked and not git-ignored —
  re-confirmed; ADR-0006 belongs to its own change; PR 3 needs its 3a/3b/3c
  split).
- **S7 (new)** — give `scheduleInterval` a test. `APP_SCHEDULE_INTERVAL` is a
  documented, `check-env-example`-enforced variable whose parse-and-fallback path
  is never exercised.

### Cross-pass finding ledger

| ID | Pass raised | Finding | Resolution | State |
|---|---|---|---|---|
| C1 | 1 | Resolved breaks never reach rule 3 | batch 1 wired `Breaks` | CLOSED (pass-2 M2) |
| C2 | 1 | `ingest --series/--source` unwired; `ReconcileDimensions` prerequisite gap | batch 1 | CLOSED (pass-2 M4/M5) |
| C3 | 1 | Public hash listing never published from the pipeline | batch 1 | CLOSED (pass-2 M3); outer path unpinned -> C8/M7 |
| C4 | 1 | Scheduler never runs in `serve` | batch 2 | Superseded by C6 + C7 |
| C5 | 1 | Deploy step absent | batch 1 | CLOSED as far as environment allows |
| C6 | 2 | 24h rule defeated; `lastSuccess` seeded from an empty map | batch 3 `seedLastSuccess` + `LastSuccessfulDownloadAttempt` | **CLOSED** (pass-3 M3) |
| C7 | 2 | `serve` wiring unpinned; `startScheduler` untested | batch 3 `schedulerStarter` seam | **CLOSED as stated** (pass-3 M1); callee still 0 % -> C8 |
| C8 | **3** | Composition root at 0 %; C6/C7/C3 wiring regressions invisible | — | **OPEN — blocker** |
| W1,W2,W3,W5,W6,W8,W9 | 1 | various | batches 1-2 | CLOSED (pass 2) |
| W7 | 1 | Dead spy assertion retained | batch 3 deleted it | **CLOSED** |
| W11 | 2 | Non-atomic hash-listing write | batch 3 `writeFileAtomically` | **CLOSED** (pass-3 M5) |
| W12 | 2 | `max_response_bytes` parsed and ignored | batch 3 (eurostat/xlsx) + batch 4 (ine) | **CLOSED** (pass-3 M6) |
| W15 | 2 | Subtest miscount in apply-progress | batch 3 | CLOSED — gate reports 8, docs say 8 |
| W13, W14 | 2 | Stale/inaccurate doc text | partly batch 4 | Carried forward |
| W4, W10 | 1 | Sign-off path; partial publish | deferred | Deferral sound |
| W16, W17 | **3** | Dead freshness / reconcile surfaces | — | New, non-blocking |
| S1-S5 | 1 | various | deferred | Sound |
| S6 | 2 | Assert `cmdServe` starts a scheduler | batch 3, partially | Superseded by C8 |
| S7 | **3** | `scheduleInterval` untested | — | New, non-blocking |

### Working-tree integrity

Seven mutations were applied and reverted this pass. Before any mutation I took a
SHA-256 manifest of all six target files; after the final revert **all six are
byte-identical** to that manifest. Repository-wide scan: **no zero-byte `.go`
files, no `*.bak` or leftover probe files** (`app/internal/adapters/ine/probe_test.go`
is a legitimate repository file, not an artefact of this session). `gofmt -l .`
clean, `go build ./...` exit 0, full `go test -count=1 ./...` exit 0 after
restoration.

**Disk**: checked before and after every heavy step per the environment warning.
Stable throughout at **57 G used / 30 G free (66 %)**, including across the full
Docker compose smoke run. Nothing was pruned this pass.

No commit and no push were made; the repository still has **zero commits**.

### Verdict

**FAIL — DO NOT ARCHIVE.** One blocker: **C8**.

Reasoning, in order of weight:

1. **The C6 remediation has no regression signal.** One argument change (M2)
   silently restores the defect that failed pass 2. A fix that cannot be
   observed to break has not been durably closed — and accepting it would repeat
   the exact reviewing error that let C4 through as "proven end-to-end".
2. **The seam built to close C7 does not protect itself** (M4), and C3's outer
   path is unpinned too (M7). Three instances, one pass, all in the same
   0 %-coverage layer — this is the change's dominant, repeatedly fix-introduced
   failure mode, still unmitigated at its root.
3. **Everything else is genuinely done, and I want that stated plainly.** Both
   pass-2 CRITICALs are closed and mutation-proven. The FAILING scenario is gone.
   W7, W11, W12, W15 and batch 4's INE ceiling are all closed and falsifiable.
   Compliance is 105/114 with 0 FAILING; the remaining 4 PARTIAL and 5 UNTESTED
   are all pre-disclosed, all with sound reasons, and none of them is a defect
   this team can close in this environment.

**What blocks archive is one bounded work unit, and it is the last one.** Execute
`startScheduler` in a test with an injected pool and config; assert it passes a
non-nil seed to `runScheduler`; assert the default `schedulerStarter` binding;
pin `cmdIngest`'s hash-listing path resolution. Optionally add S7. There is no
outer layer left for the pattern to hide in, so this converges — the next pass
should be the last.

If the maintainer judges C8 acceptable as a documented follow-up rather than a
blocker, the spec case for archive is otherwise complete: 0 FAILING scenarios,
every remaining gap disclosed and sound. That is a legitimate call to make with
this evidence in hand — but it is not the one this gate makes on its own.

---

## Appendix — pass 2 report, retained verbatim


## Verification Report — pass 2 (post-remediation re-verification)

**Change**: phase-0-data-foundations
**Version**: N/A (greenfield; `openspec/specs/` empty, all deltas ADDED-only)
**Mode**: Strict TDD
**Scope**: PRD Fase 0, milestones 0.1–0.7. Fase 1–4 surfaces are correctly absent and are not reported as missing.
**Supersedes**: pass 1 (verdict FAIL, 5 CRITICAL / 10 WARNING / 5 SUGGESTION). Pass 1's findings and their resolution are preserved below rather than discarded.

### Recommendation: DO NOT ARCHIVE

Four of the five pass-1 CRITICALs are genuinely closed and I proved each one
falsifiable by mutation. **C4 is not closed.** Its wiring line is untested — I
deleted it and the entire suite stayed green — and, more seriously, the way the
scheduler was wired **defeats the 24-hour escalation rule it exists to
implement**. I proved that at runtime: a single failed cycle at process start
emits an incident reading *"source ine has been down for over 24h0m0s"* on a
process that started milliseconds earlier.

That is a false alert in the operational-alerting path, in the one capability
(`pipeline-operations`) whose whole purpose is telling a human the truth about
source health. It is a behaviour regression introduced by the remediation, not a
pre-existing gap, and it is the reason this pass returns FAIL.

### Headline changes since pass 1

| Pass-1 finding | Claimed | Verified verdict |
|---|---|---|
| C1 resolved breaks -> rule 3 | closed | CLOSED — mutation-verified |
| C2 `ingest --series/--source` | closed | CLOSED — mutation-verified at both layers |
| C3 hash listing from pipeline | closed | CLOSED — mutation-verified, 2 tests fail |
| C4 scheduler wired into `serve` | closed | **NOT CLOSED** — see C6/C7 |
| C5 deploy step | closed | CLOSED to the extent the environment allows |
| W1 import guard covers §9.2 | closed | CLOSED — mutation-verified |
| W2 `scope.ref` resolution | closed | CLOSED — mutation-verified |
| W3 ECOICOP scoping | closed | CLOSED — spec text genuinely amended |
| W5, W6, W8, W9 | closed | CLOSED — each re-checked |

### Completeness

| Metric | Value |
|--------|-------|
| Tasks total | 144 |
| Tasks complete (`[x]`) | 144 |
| Tasks incomplete (`[ ]`) | 0 |
| Tasks whose named deliverable is absent | 0 (pass 1's 1.17 deploy step now delivered) |
| Tasks whose deliverable is present but unpinned by any test | **1** (C4's `serve` wiring) |

### Build & Tests Execution

All commands run from the repository root (ADR-1/ADR-5).

**`go test ./...`** — exit 0

```text
ok  	github.com/jorgealonsodev/concontexto	0.006s
ok  	github.com/jorgealonsodev/concontexto/app/cmd/concontexto	15.411s
ok  	github.com/jorgealonsodev/concontexto/app/internal/adapters/config	0.021s
ok  	github.com/jorgealonsodev/concontexto/app/internal/adapters/eurostat	0.009s
ok  	github.com/jorgealonsodev/concontexto/app/internal/adapters/filestore	0.074s
ok  	github.com/jorgealonsodev/concontexto/app/internal/adapters/ine	0.018s
ok  	github.com/jorgealonsodev/concontexto/app/internal/adapters/postgres	4.225s
ok  	github.com/jorgealonsodev/concontexto/app/internal/adapters/xlsx	0.295s
ok  	github.com/jorgealonsodev/concontexto/app/internal/guard	0.017s
ok  	github.com/jorgealonsodev/concontexto/app/internal/healthcheck	0.014s
ok  	github.com/jorgealonsodev/concontexto/app/internal/httpserver	0.207s
ok  	github.com/jorgealonsodev/concontexto/app/internal/indicators	0.013s
ok  	github.com/jorgealonsodev/concontexto/app/internal/ingestion	3.542s
ok  	github.com/jorgealonsodev/concontexto/app/internal/ingestion/alerting	0.004s
ok  	github.com/jorgealonsodev/concontexto/app/internal/ingestion/freshness	0.003s
ok  	github.com/jorgealonsodev/concontexto/app/internal/ingestion/pipelinelog	0.008s
ok  	github.com/jorgealonsodev/concontexto/app/internal/ingestion/sourceerr	0.002s
ok  	github.com/jorgealonsodev/concontexto/app/internal/ingestion/validation	0.138s
ok  	github.com/jorgealonsodev/concontexto/app/internal/migrate	0.005s
ok  	github.com/jorgealonsodev/concontexto/app/internal/probe	0.005s
ok  	github.com/jorgealonsodev/concontexto/app/internal/scheduler	0.007s
?   	github.com/jorgealonsodev/concontexto/app/migrations	[no test files]
```

**`go test -short ./...`** — exit 0. Same 21 packages `ok`; `adapters/postgres`
drops to 0.082s and `ingestion` to 0.066s, confirming testcontainers work is
genuinely `-short`-guarded (ADR-3).

**`go vet ./...`** — exit 0, no output.

**`gofmt -l .`** — exit 0, no output.

**`./scripts/check-env-example.sh`** — exit 0

```text
check-env-example: OK — 6 variable(s) documented: APP_DATA_ROOT APP_SCHEDULE_INTERVAL DATABASE_URL PORT POSTGRES_ADDR STATIC_ROOT
```

(4 -> 6 variables: `APP_DATA_ROOT` and `APP_SCHEDULE_INTERVAL` added by the batches.)

**`go run ./app/cmd/concontexto validate-config`** — exit 0

```text
validate-config: ok
```

**`go test ./app/cmd/concontexto/ -run TestFase0ClosureGate -v`** — exit 0,
**8 subtests** PASS (0.1 repository/CI, 0.1 deploy step, 0.2, 0.3, 0.4, 0.5,
0.6, 0.7). Note: `apply-progress.md` states "PASS all 9 subtests"; there are 8.

**`./scripts/smoke-test.sh`** — exit 0, all 15 steps passed

```text
==> 5. Asserting no published ports on either service
  app: NetworkSettings.Ports = {"8080/tcp":null} (no host binding) OK
  postgres: NetworkSettings.Ports = {"5432/tcp":null} (no host binding) OK
==> 6. Asserting hard memory limits
  app: 268435456 bytes (256 MiB) OK
  postgres: 536870912 bytes (512 MiB) OK
==> 8. GET /healthz over the compose network expects HTTP 200 and body 'ok'
  /healthz -> 200 'ok' OK
==> 9. GET / expects HTTP 200 and the Astro hello-world HTML
  / -> 200, contains 'ConContexto' OK
==> 12. Stopping postgres: --deep must fail, shallow must still pass
  healthcheck --deep exited non-zero while postgres is down OK
  healthcheck (shallow) still exited 0 while postgres is down OK
==> 15. pg_dump -> drop -> pg_restore round trip (scripts/backup/pg_dump.sh)
  pg_dump -> drop -> pg_restore round trip OK
SMOKE TEST: ALL CHECKS PASSED
```

**`go test -tags live ./app/internal/probe/... -v`** — not run. No outbound
network in this sandbox. Unchanged, accepted, still disclosed.

**Coverage**: not measured; no threshold configured for this change.

### Mutation spot-checks performed this pass

Every mutation was reverted and re-confirmed green. The working tree is
byte-identical to its pre-verification state (see "Working-tree integrity").

| # | Target | Mutation applied | Result | Verdict |
|---|---|---|---|---|
| M1 | **C4** `serve` wiring | deleted `startScheduler(ctx, stderr)` from `cmdServe` | **ENTIRE SUITE STAYS GREEN** (exit 0, 21/21 `ok`) | **wiring unpinned** |
| M2 | **C1** rule-3 breaks | `Breaks: breaks` -> `Breaks: nil` | **FAIL** — `expected the break recorded in postgres at M05 to exempt the jump and publish, got outcome=block` | falsifiable |
| M3 | **C3** hash listing | removed `PublishRawFileHashListing` call from `IngestSeries` | **FAIL x2** — `TestIngestSeries_PublishesTheRawFileHashListingWhenPathsAreConfigured` and `TestRunIngest_SeriesFlagIngestsOneSeriesEndToEnd` | falsifiable |
| M4 | **C2a** flag parsing | removed `--series=`/`--source=` parsing from `cmdIngest` | **FAIL** — `expected stderr to mention DATABASE_URL, got "ingest: usage: ..."` | falsifiable |
| M5 | **C2b** dimensions | removed `postgres.ReconcileDimensions` call from `runIngest` | **FAIL x4** — all at the real FK level: `violates foreign key constraint "raw_file_source_id_fkey"` | falsifiable |
| M6 | **W1** import guard | added `_ ".../adapters/ine"` to `httpserver/static.go` | **FAIL** — `golden rule violated: httpserver transitively imports ".../adapters/ine"` | falsifiable |
| M7 | **W2** scope refs | removed `ref_status: pending` from the `aeat` break | **exit 1** — `scope.ref "aeat" does not resolve to any configured source` | falsifiable |

**M1 is the finding of this pass.** Deleting the single line that makes C4 real
leaves 21/21 packages `ok`. All five C4 tests call `runScheduler` or
`scheduleSourceOp` **directly**; none goes through `cmdServe`, and
`startScheduler` has zero test references. A wiring fix whose test passes with
the wiring removed proves nothing — exactly the standard the brief set.

### Runtime evidence for C6 (the 24-hour rule)

Written as a temporary probe, executed, then deleted (no repository file
retained):

```text
=== RUN   TestVerifyProbe_FirstFailureAfterRestartRaisesIncidentImmediately
    alerts raised after ONE failed cycle at process start: 1
      kind=source-down source=ine message="source ine has been down for over 24h0m0s"
    EVIDENCE: a source-down incident fired after a SINGLE failure at t=0,
    not after the 24h freshness Window (got 1 alert(s))
--- FAIL: TestVerifyProbe_FirstFailureAfterRestartRaisesIncidentImmediately (0.30s)
```

Mechanism, all three steps in shipped source:

1. `schedule.go:119` — `lastSuccess := make(map[string]*time.Time, len(runners))`
   starts **empty** on every process start.
2. `schedule.go:131` — `runner.Run(ctx, lastSuccess[id], now, ...)` passes `nil`
   for any source not yet successful *in this process*.
3. `freshness.go:56` — `Resolve(nil, asOf)` returns `StateFailed` ->
   `RaisesIncident()` is true -> `alerting.SourceDown` fires.

The unit test `TestRun_TwoFailuresThenSuccessRetriesWithBackoffAndRaisesNoIncident`
passes only because it injects a **non-nil** `lastSuccess`; the production path
that seeds that value from an empty map is never exercised.

The DB-backed functions that would fix this — `postgres.SourceFreshness` and
`postgres.SeriesFreshness`, which read the real last successful
`download_attempt` and survive restarts — have **zero production callers**.

### The unwired-function heuristic, re-run

Scanned every function defined in non-test Go source, counted references in
production code excluding its own definition. 30 candidates had zero
`name(`-style production calls; after also counting bare references (dispatch
tables, method values, interface satisfaction), the genuinely unreferenced set
resolves as follows.

| Function | Assessment |
|---|---|
| `ReconcileDimensions`, `PublishRawFileHashListing`, `resolveBreaks`, `IngestSeries`, `ReconcileEditorialConfig`, `ResolveActiveBreaksForSeries`, `Rule1..Rule6`, `cmdIngest/cmdMigrate/cmdValidateConfig` | all now have real production call sites |
| `startScheduler` | one production call site (`serve.go:74`), **zero test coverage** — see C7 |
| `FetchProbe`, `Targets` | wired via `.github/workflows/probe.yml` (`go test -tags live`), not the Go call graph. Legitimate; never executed (accepted). |
| `alerting.DefaultSink` | used in `ingest.go:295`, `alerting.go:138`, `scheduler.go:92` |
| `pipelinelog` | used in `ingest.go` (`FailedRules`, `Attrs`, `Verdicts`) |
| `SourceFreshness`, `SeriesFreshness` | **zero production callers** — the persistent amber clock is never consulted (root cause of C6) |
| `MaxResponseBytes` (config field) | **sixth instance of the pattern** — see W12 |
| `ExecutionCount`, `SetDefaultSink`, `ApplyGateWithWriter` | legitimate test seams |
| `ReconcileBreaks` | superseded by `ReconcileEditorial` (single-transaction version); now dead code |
| `FetchSeries` (`*ine.Client`) | not part of the `indicators.SourceClient` port; dead |
| `ResolveAttribution`, `ResolveProvenance`, `VintageAsOf`, `RollbackRun`, `filestore.Read`, `ListEvents`, `ListMappings`, `ListSeriesBreaks` | read-side / operator surfaces whose consumers are Fase 1. Out of scope, correctly absent. |

### Spec Compliance Matrix — per-capability verdict

| # | Capability | Req | Scen | COMPLIANT | PARTIAL | UNTESTED | FAILING | Verdict |
|---|---|---|---|---|---|---|---|---|
| 1 | platform-runtime | 10 | 14 | 8 | 3 | 3 | 0 | FAIL — deploy scenarios unexecutable |
| 2 | data-model-vintages | 7 | 11 | 11 | 0 | 0 | 0 | PASS |
| 3 | data-validation | 8 | 15 | 14 | 1 | 0 | 0 | PASS WITH WARNINGS |
| 4 | editorial-config | 9 | 15 | 15 | 0 | 0 | 0 | PASS |
| 5 | pipeline-operations | 4 | 8 | 5 | 0 | 2 | **1** | **FAIL** — 24h rule contradicted |
| 6 | raw-file-archive | 4 | 9 | 9 | 0 | 0 | 0 | PASS |
| 7 | source-attribution-licensing | 4 | 6 | 6 | 0 | 0 | 0 | PASS |
| 8 | source-ingestion-eurostat | 7 | 10 | 10 | 0 | 0 | 0 | PASS |
| 9 | source-ingestion-ine | 6 | 10 | 9 | 1 | 0 | 0 | PASS WITH WARNINGS |
| 10 | source-ingestion-xlsx | 6 | 16 | 16 | 0 | 0 | 0 | PASS |
| | **Total** | **65** | **114** | **103** | **5** | **5** | **1** | **FAIL** |

**Compliance summary**: 103/114 COMPLIANT (was 98), 5 PARTIAL (was 11),
5 UNTESTED (unchanged), 1 **FAILING** (new).

### Re-assessment of pass 1's 11 PARTIAL scenarios

| Capability | Scenario | Pass 2 status |
|---|---|---|
| platform-runtime | Request path performs no outbound source call | **now COMPLIANT** — W1 extended the guard to `adapters/{ine,eurostat,xlsx}`; mutation M6 proves it falsifiable |
| data-validation | A large jump at a recorded break passes | **now COMPLIANT** — C1 wired; mutation M2 |
| editorial-config | The ECOICOP v2 break is present and scoped | **now COMPLIANT** — the **spec text was genuinely amended** ("scoped to every CPI-derived series **that is genuinely affected**… not unconditionally"; scenario retitled "…**may resolve for zero series today**"). Implementation and spec now agree. |
| raw-file-archive | The listing matches the archive | **now COMPLIANT** — C3 wired; mutation M3 |
| pipeline-operations | A transient failure is retried | **now COMPLIANT** — `runScheduler` retries at `Attempt.NextAttemptAt`, proven by `TestRunScheduler_FailedSourceIsRetriedAtItsRunnerBackoffNotTheFullInterval` |
| pipeline-operations | Twenty-four hours of failure raises an incident | **DOWNGRADED to FAILING** — production raises it after the *first* failure. See C6. |
| platform-runtime | Serving pages issues no queries | still PARTIAL — W7 deferred; dead spy retained |
| platform-runtime | Boot does not migrate | still PARTIAL — proxy counter, unchanged |
| platform-runtime | Postgres unreachable from outside the stack | still PARTIAL — smoke step 5 asserts no port binding, never attempts a connection. Smoke suite re-run green this session. |
| data-validation | A deep revision blocks publication | still PARTIAL — W4 deferred; no sign-off release path |
| source-ingestion-ine | All six series load full history | still PARTIAL — test honestly renamed (W5), but renaming does not make the scenario compliant; trimmed fixtures are spec-mandated, so the tension is spec-internal |

### Re-assessment of pass 1's 5 UNTESTED scenarios

| Capability | Scenario | Pass 2 status |
|---|---|---|
| platform-runtime | Pushed commit deploys hello-world | still UNTESTED. `deploy.yml` now exists and is honestly gated, and the closure gate asserts the file plus its `PORTAINER_WEBHOOK_URL` gate — but it deliberately does **not** assert a deploy happened, and cannot. Milestone-0.1 criterion genuinely unmet (accepted). |
| platform-runtime | Failing tests block deploy | still UNTESTED. `deploy.yml` is structurally correct (`workflow_run` + `conclusion == 'success'`), but no test exercises it. |
| platform-runtime | Single approval is rejected on a config change | still UNTESTED — `@TODO-second-config-reviewer`, zero commits, no remote. Accepted. |
| pipeline-operations | The probe detects a broken identifier | still UNTESTED — `live` suite never executed. Accepted. |
| pipeline-operations | The probe asserts shape, not values | still UNTESTED — same. Accepted. |

### Deferred items — is the stated reason sound?

| Item | Stated reason | My judgement |
|---|---|---|
| **W4** rule 4 sign-off has no implementation | "design-level work" | **Sound.** The safety-critical half (block) works and is tested. Only the *release* half is missing, so a flagged run fails closed, never open. Operability gap, not an integrity risk. Stays WARNING. |
| **W7** spy assertion unfalsifiable by construction | "unfalsifiable BY CONSTRUCTION; real proof is the mutation-verified import guard" | **Claim true, remedy wrong. Not a CRITICAL in disguise.** I tested it rather than accepting it: `NewServer(root fs.FS)` genuinely has no parameter able to accept a counter, so the claim is literally correct; and I re-proved the import guard falsifiable (M6), so the *property* is genuinely enforced. But the right response to a permanently-dead assertion is to **delete it**, not keep it. `spy.n.Load() != 0` can never fire and misleads a future reader into thinking the scenario is covered. Stays WARNING. |
| **W10** partial publish on mid-publish DB failure | "cross-cutting transaction-boundary change with real risk" | **Sound as a deferral**, and I confirmed the defect is unchanged (`applyGate` loops `writer.WriteRevision` per candidate; a failure at candidate *k* leaves 1…*k*-1 durable **and** never reaches `recordRunOutcome`, so the run outcome is never recorded either). Consequence is now slightly higher because C4 runs the pipeline unattended. Stays WARNING. |
| **S1–S5** | various | All sound; unchanged. |

### Issues Found

**CRITICAL**

- **C6 — The scheduler's 24-hour escalation rule is defeated in production.**
  `runScheduler` seeds `lastSuccess` from an empty in-memory map, and
  `freshness.Resolve(nil, …)` returns `StateFailed`, so the **first** failed
  cycle after any process start raises a `source-down` incident whose message
  claims the source "has been down for over 24h0m0s". Runtime-proven above.
  This contradicts spec `pipeline-operations`'s "Twenty-four hours of failure
  raises an incident" and the Eurostat scenario in which scheduled maintenance
  "is retryable, not an incident, until the 24 h threshold". Because `serve`
  restarts on every deploy, the amber clock also resets on every deploy. The
  DB-backed `postgres.SourceFreshness` / `SeriesFreshness`, which exist
  precisely to supply a restart-surviving last-success timestamp, have zero
  production callers. **This is a behaviour regression introduced by the C4
  remediation, not a pre-existing gap.**

- **C7 — C4's `serve` wiring is unpinned and `startScheduler` is untested.**
  Mutation M1: deleting `startScheduler(ctx, stderr)` from `cmdServe` leaves the
  full suite green. `startScheduler` — which builds the pgxpool, loads the
  embedded config, constructs the per-source `Runner` map, resolves
  `APP_DATA_ROOT`/`STATIC_ROOT` paths and launches the ticker goroutine — has no
  test of any kind. The apply-progress claim that C4 is "proven by a real-Postgres
  end-to-end test" is **overstated**: that test proves `runScheduler` +
  `scheduleSourceOp`, both called directly, and would pass unchanged if `serve`
  never started a scheduler again. Given this change's history of five
  "built and tested, never wired" defects, an unguarded wiring line is a live
  regression risk.

**WARNING**

- **W11 (new) — The in-process scheduler writes non-atomically into the live
  static root.** `PublishRawFileHashListing` uses `os.Create` (truncate) then
  `os.WriteFile` on `staticAssetRoot()/transparencia/raw-files.sha256`, and
  `httpserver` serves that exact tree via `http.FileServer(http.FS(root))`.
  Before C4 these ran in separate processes; they now share one. A request
  landing mid-write can be served a **truncated hash listing** — the one
  artifact whose entire purpose (PRD §14.2) is to let a reader verify integrity.
  One-line fix: write to a temp file and `os.Rename` (atomic on the same
  filesystem). Note this does **not** breach the golden rule: it is still a plain
  file read, no DB query, no computation, no outbound call.

- **W12 (new) — `max_response_bytes` is configured but never read: the sixth
  instance of the pattern.** `config.APIConfig.MaxResponseBytes` is parsed
  (`types.go:139`) and set in all three of
  `config/sources/{ine,eurostat,seg-social}.yaml` to `8388608`, but no production
  code passes it to `WithMaxResponseBytes`; `buildSourceClient` constructs every
  client with no options. Behaviour is correct **only by coincidence** — the
  hardcoded `defaultMaxResponseBytes` happens to equal the configured value.
  Editing the YAML has zero effect, which contradicts the change's own
  "editorial YAML is authoritative" principle. Classified WARNING rather than
  CRITICAL because the effective ceiling is correct today.

- **W13 (new) — Stale comment pointing at a deleted file.**
  `eurostat/client.go:63–67` says production wiring "arrives with the ingest
  subcommand's real implementation (still a stub, `app/cmd/concontexto/stubs.go`)".
  That file no longer exists and the ingest subcommand is now real — but the
  wiring still did not arrive (W12). The comment now misleads twice over.

- **W14 (new) — `deploy.yml`'s gate message is inaccurate.** It annotates
  "this run **builds and pushes the image** but performs no redeploy", yet the
  build-and-push step is itself gated on `configured == 'true'`, so an
  unconfigured run does nothing at all. Honest intent, wrong text.

- **W15 (new, minor) — `apply-progress.md` states `TestFase0ClosureGate` passes
  "all 9 subtests"; there are 8.**

- **W4, W7, W10** — carried forward from pass 1, deferral reasons judged sound
  (see table above).

- **Dead code surfaced by the heuristic**: `postgres.ReconcileBreaks`
  (superseded by the single-transaction `ReconcileEditorial`) and
  `(*ine.Client).FetchSeries` (not part of the `indicators.SourceClient` port).
  Neither is reachable; both should be removed or documented as retained.

**SUGGESTION**

- S1–S5 carried forward unchanged from pass 1 (non-dismissible guard uses a
  substring denylist; the theatrical archive-before-parsing test; `.codegraph/`
  untracked and not git-ignored; ADR-0006 belongs to its own change; PR 3
  still needs its 3a/3b/3c split).
- S6 (new) — consider asserting `cmdServe` starts a scheduler (inject the
  starter, or assert the goroutine's observable effect) so C7 cannot recur.

### Non-negotiable business rules — structural enforcement audit

| Rule | Enforced? | Evidence |
|---|---|---|
| Zero DB queries / zero computation at request time (§14.2) | **Yes, structurally** | `go list -deps ./app/internal/httpserver` returns **no** internal package other than itself. `NewServer(root fs.FS)` takes no port. Guard mutation-verified (M6). Verified explicitly for the post-C4 process: the scheduler is composed **beside** the handler in `cmdServe`, never reachable from it. |
| No external source call at page-request time (§9.2) | **Now guarded** (was "true but unguarded") | W1 added `adapters/{ine,eurostat,xlsx}` to the forbidden-prefix list; M6 proves it fires. |
| Validation failure never publishes | Yes | `applyGate` returns before the write loop on `GateBlock`. |
| Raw downloads archived with SHA-256 **before** parsing (§9.1) | Yes | `ArchiveRawFile` (ingest.go:169) precedes `client.Decode` (ingest.go:203) in source order. |
| One current row per `(series_id, period)`, DB-enforced | Yes | Partial unique index; mutation-verified in pass 1 against real PostgreSQL 17. |
| Series breaks non-dismissible (P4) | Yes | Reflection guard; mutation-verified in pass 1. |
| Editorial YAML authoritative; DB a projection | **Mostly** | Reconcile is transactional/idempotent/soft-retire. **Exception**: `max_response_bytes` is authoritative in YAML but ignored by code (W12). |
| `ReconcileEditorialConfig` never projects a nil date | **Confirmed, still holds** | `reconcile.go` skips nil `Date`/`DateStart` into `Pending*IDs`; `series_break.date` and `event.date_start` are `date NOT NULL` in the DDL as a second line of defence. Closure gate reports 14 confirmed / 7 unconfirmed, matching the disclosed state. |
| 24 h amber escalation per source | **NO** | See C6. |

### Known-and-accepted state — honesty check

| Disclosed item | Represented honestly? |
|---|---|
| 7 of 21 entries `date_status: unconfirmed`; reconcile never projects a nil date | Accurate; guarantee re-verified in code and DDL |
| 4 entries `ref_status: pending` | Accurate; and `validate-config` now genuinely enforces the non-pending case (M7) |
| `@TODO-second-config-reviewer`; four-eyes unenforceable (§18) | Accurate, unchanged |
| Milestone 0.1 deploy criterion unmet and now says so | Accurate — closure gate `t.Log` and task 1.17 both state it |
| INE `TipoDato` not surfaced; all observations definitive | Accurate; blocks Fase 1, not Fase 0 |
| `live` probe suite never run | Accurate; `probe.yml` exists and would run it given network |

### TDD Compliance

| Check | Result | Details |
|-------|--------|---------|
| TDD Evidence reported | Yes | Cycle tables present for every work unit incl. both remediation batches |
| All tasks have tests | Qualified | 144/144 marked; `startScheduler` has none (C7) |
| RED confirmed (test files exist) | Yes | Every named test file exists |
| GREEN confirmed (tests pass) | Yes | 21/21 packages `ok` on `-count=1` |
| Triangulation adequate | Yes | Four fake-op scheduler cases + one real-Postgres cycle is good triangulation *of `runScheduler`* |
| Safety net for modified files | Yes | Both batches record prior suite state |
| RED-first discipline | Qualified | Disclosed deviations, each compensated by mutation; I independently re-verified seven mutations this pass |

**TDD Compliance**: 5/7 clean, 2 qualified.

### Assertion Quality

| File | Line | Assertion | Issue | Severity |
|------|------|-----------|-------|----------|
| `app/internal/httpserver/static_test.go` | 64 | `if got := spy.n.Load(); got != 0` | Dead assertion — the spy is never passed to production code and cannot be (W7) | WARNING |
| `app/internal/httpserver/static_test.go` | 81–88 | blocking `DialContext` on the **client** transport | Constrains the test client, not the server | WARNING |
| `app/cmd/concontexto/serve_test.go` | 44 | `migrate.ExecutionCount() != baseline` | Proxy counter; no DB, no schema version | WARNING |
| `app/internal/adapters/postgres/rawfile_test.go` | 108–118 | in-test `parseCrashed` panic | Demonstrates the test's ordering, not production's; real ordering proven elsewhere | SUGGESTION |

**Assertion quality**: 0 CRITICAL, 3 WARNING, 1 SUGGESTION. No tautologies,
ghost loops, or orphan empty-collection assertions across the test suite. The
new `schedule_test.go` assertions are genuine (distinct expected call counts,
interval vs backoff distinguished by a deliberately different duration).

### Quality Metrics

**Linter (`go vet`)**: No errors.
**Formatter (`gofmt -l .`)**: No unformatted files.
**Type checker (`go build ./...`)**: No errors.
**Coverage tool**: Not configured for this change.

### Working-tree integrity

Seven mutations were applied and reverted this session. During mutation M6 the
**host root filesystem reached 100 % (0 bytes free)**, which truncated
`app/internal/httpserver/static.go` to 0 bytes mid-write and captured an empty
backup. This was detected immediately (its SHA-256 was `e3b0c442…`, the digest
of the empty string), the file was restored from its verbatim content, and the
restoration was confirmed by `gofmt -l` (clean), `go build ./...` (exit 0) and a
full `go test -count=1 ./...` (exit 0, 21/21 `ok`). A repository-wide scan
confirms `static.go` was the only file affected and that **no zero-byte `.go`
files and no leftover `*.bak` or probe files remain**.

Environment note for the orchestrator: the disk pressure was pre-existing
(~34 GB of Docker build cache). Reclaimable build cache was pruned to restore
tooling; no images, containers, volumes, or user data were touched. Disk after:
56 G used / 31 G free.

No commit and no push were made; the repository still has zero commits.

### Verdict

**FAIL** — **DO NOT ARCHIVE.**

Reasoning, in order of weight:

1. **One scenario is now FAILING, not merely untested.** `pipeline-operations`'s
   24-hour escalation is contradicted by shipped behaviour, proven at runtime.
   A monitoring system that cries wolf on every restart is worse than one that
   is absent, because it trains its operator to ignore it.
2. **C4 is not closed.** Its wiring survives deletion with a green suite, and
   `startScheduler` is entirely untested — the same defect class this change has
   already produced five times.
3. Everything else genuinely improved: four CRITICALs closed and each proved
   falsifiable, seven of ten WARNINGs closed, and compliance rose from 98/114 to
   103/114 with the three deferred WARNINGs carrying sound reasons.

Smallest-blast-radius remediation order: **C6** (seed `lastSuccess` from
`postgres.SourceFreshness` at startup, or treat "no in-process success yet" as
distinct from "never succeeded"), then **C7** (make the `serve` wiring
assertable), then **W11** (atomic rename), then **W12** (read
`MaxResponseBytes` from config). W4, W7 and W10 remain reasonable deferrals and
need not block archive once C6 and C7 are closed.

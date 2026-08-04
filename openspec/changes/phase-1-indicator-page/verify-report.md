```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:26e39e4e22dabf5280cfba04bcfbc8d1f58800b6fabf368779fce257fd0a46e8
verdict: fail
blockers: 0
critical_findings: 0
requirements: 78/79
scenarios: 175/176
test_command: go test -race -count=1 ./... && npm --prefix web run check && npm --prefix web test && npx --prefix web playwright test --config web/playwright.config.ts
test_exit_code: 0
test_output_hash: sha256:5f13c2c5122da0104fa461f95c698fad6ad997d75b3a5872d4462955591af400
build_command: EXPORT_DIR=data-derived npm --prefix web run build
build_exit_code: 0
build_output_hash: sha256:f8605122d845816f1947fca196bb798e5aee65132bdf92b9891a676bfb4eb87f
```

# Verification Report — phase-1-indicator-page

**Verifier**: `sdd-verify` (pass 9)
**Date**: 2026-08-05
**Change**: `phase-1-indicator-page`
**Mode**: hybrid (OpenSpec file + Engram `sdd/phase-1-indicator-page/verify-report`), Strict TDD active
**Artifacts read**: proposal, design, 13 capability delta specs, tasks, apply-progress, pass-8 verify-report
**Verified against**: the REPOSITORY (`feat/phase-1-indicator-page` @ `9483d23`, working tree clean before
and after every probe, including after five source mutations), the RUNNER, a FRESH EXPORT taken from the
live database at HEAD, a FRESH SITE BUILD from that export, and the running stack. Every number below was
re-measured here. Nothing was taken from a session summary, a commit body, or the record under review.

---

## Verdict

**FAIL on evidence completeness. Zero CRITICAL findings. Zero blockers. No defect in the delivered
product.**

CRITICAL-54 is closed, and closed as a class rather than as an instance. `ngeu-primer-desembolso` is held
back by the declared status alone; the same predicate governs both registries; the operator backlog now
reports seven where the configuration declares seven; and the entry appears in zero of the ten freshly
exported series documents and zero of the seven freshly built pages.

The seam that produced a defect in seven of the previous eight passes was probed again, this time in both
directions — the projection path and the resurrection path. The projection path is correct and now
mutation-proof. The resurrection path is correct in the product and half-covered in the suite; that is
recorded as SUGGESTION-60, not as a defect.

**What holds the envelope at `fail` is not code.** One spec scenario —
`pipeline-operations` / "A failed rebuild raises an alert **immediately**" — has had no covering test since
pass 4. Scenario coverage is therefore 175/176 and requirement coverage 78/79, and the verification contract
treats incomplete coverage as not-archive-ready regardless of blocker count. Through passes 4–8 this sat
inside the counts alongside a real blocker and was never the last item; with CRITICAL-54 closed, it is.
This is a re-statement of an already-recorded gap, not a new finding.

Three new WARNINGs and one new SUGGESTION are recorded. None is a defect in the delivered product: two are
inaccurate prose that survived a commit whose stated purpose was to eliminate inaccurate prose about this
exact invariant, one is the SDD record falling one commit behind again, and one is a coverage gap on a
correct code path.

---

## Counts

| Dimension | Compliant | Total |
|---|---|---|
| **Requirements** | **78** | **79** |
| **Scenarios** | **175** | **176** |
| Tasks checked | 419 | 419 (0 unchecked) |

Authoritative totals re-parsed from `openspec/changes/phase-1-indicator-page/specs/*/spec.md` at `9483d23`:
data-model-vintages 4/8, data-validation 2/16, design-system 9/14, editorial-config 3/9,
indicator-page 16/39, pipeline-operations 4/10, platform-runtime 1/2, publishing-export 12/21,
series-transformations 8/14, source-attribution-licensing 1/3, source-ingestion-eurostat 3/7,
source-ingestion-ine 6/17, web-accessibility-gates 10/16. Totals: **79 requirements, 176 scenarios** —
unchanged from pass 8, as expected: `9483d23` touches no spec file.

**Non-compliant requirement (1)**:

| Capability / Requirement | Why |
|---|---|
| `pipeline-operations` / "An ingestion not followed by a rebuild alerts operators" | 3 of 4 scenarios pass. The requirement's own normative sentence — alert when a successful ingestion is not followed by a completed rebuild inside the budget — **is** implemented and tested. The sub-scenario "A failed rebuild raises an alert **immediately**" is not: nothing observes the dispatched run's conclusion. WARNING-41, open since pass 4, re-verified here |

**Non-compliant scenario (1)**: `pipeline-operations` / "A failed rebuild raises an alert immediately" — ❌ UNTESTED.

**Moved this pass**: `editorial-config` / "Unconfirmed editorial dates are operator-visible, never
reader-visible" ❌ → ✅, with both of its failing scenarios ❌ → ✅ (§B).

---

## A. Execution evidence — every number re-measured here

Working tree clean at `9483d23` before and after every probe (`git status --porcelain` → 0 lines), verified
again after each of the six source mutations in §C. Go and Playwright ran sequentially inside one `&&`
chain, never concurrently.

| Command | Exit | Result |
|---|---|---|
| `go build ./...` | 0 | clean |
| `go vet ./...` | 0 | clean |
| `gofmt -l .` | 0 | no output |
| `go test -race -count=1 ./...` | **0** | **23 packages ok** + 2 with no test files, zero race reports |
| `validate-config` | 0 | `validate-config: ok` |
| `npm run check` (astro check) | 0 | 131 files, 0 errors, 0 warnings, 2 hints |
| `npm test` (Vitest) | 0 | **799 passed / 799**, 52 files |
| `npx playwright test` | 0 | **191 passed / 191**, no flake observed (`chart-island.spec.ts` clean) |
| `EXPORT_DIR=data-derived npm run build` | **0** | **7 pages**: six `/indicador/{slug}/` + `/` |
| `ingest --reconcile` (live DB, binary built at HEAD) | 0 | see §B.1 |
| `export` (live DB, binary built at HEAD) | 0 | 10 series, `schema_version=1` |

Package count is 23, not pass 8's 22 — a counting difference in the earlier report, not a change: the full
`go test` output lists 23 `ok` lines and 2 `[no test files]` lines at both revisions.

Vitest and Playwright totals are byte-identical to passes 7 and 8. Expected and itself evidence: `9483d23`
adds no web test and touches no web source.

---

## B. CRITICAL-54, closed — four independent proofs

The governing requirement, `specs/editorial-config/spec.md:70-74`, has three clauses: not projected into the
database, not reaching any rendered page, and count-plus-identifiers reported to operators. Each was checked
separately, against the product.

### B.1 Clause 3 — the operator backlog, first-hand

A binary built from HEAD, run against the live database:

```
ingest --reconcile: breaks inserted=0 updated=0 retired=0; events inserted=0 updated=0 retired=0; ...
ingest --reconcile: breaks pending=4 (unconfirmed date), not projected: epa-cnae2025-doble-codificacion,
                    cn-revision-base-sept-2025, sec-cambios-deuda-deficit, ss-cnae2025-afiliacion
ingest --reconcile: events pending=3 (unconfirmed date), not projected: ngeu-primer-desembolso,
                    reforma-laboral-2021, gobierno-suarez-1976
```

Four plus three is seven, `ngeu-primer-desembolso` is named, and the run is idempotent.

### B.2 The arithmetic — all three surfaces agree, measured on the product

| Surface | Value | Measured by |
|---|---|---|
| The YAML itself | 4 breaks + 3 events = **7** | `grep -n date_status config/` → `rupturas.yaml` 39/56/104/250, `eventos.yaml` 56/71, `gobiernos.yaml` 28 |
| Shipped closure gate | **7** | `fase0_closure_test.go:234` → `registry has 14 confirmed-date and 7 unconfirmed-date entries` |
| Reconciliation output | **7** | `TestReconcileEditorialConfig_ShippedConfigPendingListsAreExactlyItsUnconfirmedEntries` → `shipped configuration: 4 pending breaks, 3 pending events, 7 total`; and §B.1's live run |
| Spec scenario | **7** | `specs/editorial-config/spec.md` — "GIVEN seven editorial entries with unconfirmed dates … records the count seven" |

The commit's claim is upheld on the product, not on the record: seven was always right, the closure gate and
the spec both counted what the YAML declares, and the product's six counted what the guard inferred.

### B.3 Clauses 1 and 2 — the database, the artifact, the pages

- **Database.** On a fresh database the entry is never inserted (`TestReconcileEditorialConfig_
  ShippedConfigPendingListsAreExactlyItsUnconfirmedEntries` asserts `series_break` and `event` hold no row
  for any pending id). On the live database — where the entry had been projected before the fix — the row is
  **soft-retired**, `retired_at = 2026-08-04 19:59:26+00`, and `ListActiveEvents` filters `retired_at IS
  NULL`. The only other `SELECT … FROM event` in the codebase (`editorial.go:422`, `allEvents`) is
  reconcile-internal; there is no unfiltered read on any publish path.
- **Artifact.** `web/data-derived` was deleted and regenerated from the live database with a binary built at
  HEAD. `grep -ro ngeu web/data-derived/` → **0 hits** across all ten series documents. The `events` array of
  `ipc-general.json` is now the nine confirmed entries.
- **Pages.** `web/dist` was deleted and rebuilt from that export. `grep -roi 'ngeu' web/dist/` → **0 hits**;
  a case-insensitive search for `ngeu|next generation|Recuperación y Resiliencia` across the whole build
  returns nothing. Seven pages built. Cross-checked against the running stack at `127.0.0.1:8080`: zero hits
  on `/indicador/ipc-general/`, `/indicador/tasa-de-paro-epa/`, `/indicador/pib/`, and the entry is absent
  from the artifact it serves at `/data-derived/series/ipc-general.json`.

All three clauses satisfied. Both previously failing scenarios are **✅ COMPLIANT**, each with a covering
test that passed at runtime in this pass's run.

### B.4 The fix is a class fix, not an instance fix

`reconcile.go:74` and `:87` both call `isDatePending(status, date)`; there is no second guard. `DateStatus`
is read in exactly three non-test places — the two call sites and `validate.go`'s two switch statements,
which now switch on the named `config.DateStatusUnconfirmed` constant rather than a bare literal.

The predicate's second half is a dereference guard, not a rule, and the code says so. That is a fair
description: `validateBreak`/`validateEvent` reject a confirmed entry with no date, so the branch is
unreachable through `validate-config`.

---

## C. Mutation testing — the fix, and the claims made about it

Six mutations. The tree was restored with `git checkout` after each and verified clean.

| # | Mutation | Result |
|---|---|---|
| M1 | `isDatePending` → `return date == nil` (the pre-fix guard, exactly) | **Both new tests FAIL.** This is the RED the commit body does not transcribe, produced here |
| M2 | Half-applied: breaks fixed, events left on `e.DateStart == nil` | **Both new tests FAIL** |
| M3 | Half-applied: events fixed, breaks left on `b.Date == nil` | `…AProvisionalDateOnAnUnconfirmedEntryIsStillHeldBack` **FAILS**; the shipped-config test passes |
| M4 | `isDatePending` → drop `|| date == nil` | **The entire Go suite stays green** — see WARNING-58 |
| M5 | Drop `|| current.RetiredAt != nil` from **both** un-retire clauses | `TestReconcileBreaks_FullReconcileIsTransactionalIdempotentAndSoftRetires` FAILS |
| M6 | Drop it from the **events** clause only (`editorial.go:397`) | **The entire Go suite stays green** — see SUGGESTION-60 |

**A half-applied fix does fail**, as claimed — M2 and M3 both kill a test. M3 shows which one carries the
guarantee for the breaks half: the class test, not the shipped-config test, because all four unconfirmed
breaks in the shipped YAML happen to omit their dates. `BreakConfig` did carry the same latent defect and is
fixed; the regression test that proves it asserts a break and an event in one call, exactly as claimed.

### The real-config test's independence — the claim verified, and upheld

`reconcile_test.go:553-561` derives its expectation from `DateStatus` alone:

```go
for _, b := range cfg.Breaks { if b.DateStatus == "unconfirmed" { wantBreaks = append(wantBreaks, b.ID) } }
for _, e := range cfg.Events { if e.DateStatus == "unconfirmed" { wantEvents = append(wantEvents, e.ID) } }
```

Not the `unconfirmed OR nil` disjunction the implementation applies. It uses the **string literal**, not
`config.DateStatusUnconfirmed`, so it is independent of the constant as well as of the predicate: a typo
introduced into the constant would flip the implementation and not the expectation, and M1 confirms the test
dies when the implementation and the declaration disagree. Count and identifiers are asserted separately, as
claimed, and the identifiers are compared as sets with an explicit justification for order-insensitivity.
**The writer's claim is accurate: this test states the rule, it does not restate the code.**

One qualification, recorded as WARNING-58: it does not discriminate the predicate's *second* half.

---

## D. New findings, pass 9

### WARNING-57 — The editorial authoring documentation still forbids the shape the fix deliberately blessed

`9483d23` corrected three source comments and the two type doc comments that claimed an unconfirmed entry
"MUST omit" its date. Two documents state the same retired rule and were not corrected — and they are the
two an editor actually reads before writing an entry:

- `config/rupturas.yaml:10-12`: *"una entrada cuya fecha efectiva NO está confirmada … **omite `date`** y
  declara `date_status: unconfirmed` + `todo`"*.
- `config/README.md:24-26`: *"carries `date_status: unconfirmed` + `todo` **instead of a guessed date**"* —
  and this one covers `eventos.yaml`, where the shipped configuration does the opposite.

Both contradict `types.go:63-68`, written by this same commit: *"A PROVISIONAL date alongside DateStatus
below is allowed, and is usually the better entry."* The behaviour is correct and no reader is affected;
what is wrong is that the guidance tells the next editor to write the weaker entry. The commit's own
argument — that the guess plus the todo carries more information than an empty field — is the reason this
matters rather than a nitpick.

`config/README.md:54` and `config/reconocimientos.yaml:19` reference the same discipline by analogy and
remain accurate.

### WARNING-58 — The new test's doc comment overstates what the test discriminates

`reconcile_test.go:530-535` says the expectation being derived from `DateStatus` alone *"keeps this from
degenerating into a restatement of the implementation: were the guard to drop **either half** of its
condition, the two would disagree here."*

Measured (M4): dropping `|| date == nil` leaves that test green, the sibling class test green, and **the
entire Go suite green**. Only the `DateStatus` half is discriminated. The nil half cannot be discriminated
by any test running the shipped configuration, because `validate-config` makes the confirmed-with-no-date
shape unreachable — which `isDatePending`'s own comment states correctly two files away.

Not a defect: the branch is deliberately inert defence and is documented as such. It is recorded because it
is the same species this change has been chasing for nine passes — a comment asserting a property nothing
enforces — this time in the very test written to prevent that species. The accurate sentence is "were the
guard to drop the status half".

### WARNING-59 — The record fell one commit behind again, and one open item is now false at HEAD

`9483d23` updated `verify-report.md` (sweeping in pass 8's report) and no other SDD artifact.

- `apply-progress.md` ends at **slice 36 (`5af95c5`)**. Neither the record pass (`374eb80`) nor this
  remediation is recorded as a slice, and there is no TDD Cycle Evidence row for it. The RED it lacks is
  producible — I produced it as M1 — but it is not in the record.
- `tasks.md` is 419/419 with no task rows for the remediation.
- `design.md:1564-1598` still carries the defect as an **unchecked open item** whose body states, in the
  present tense, that *"`app/internal/ingestion/reconcile.go:86` — the projection guard is `if e.DateStart
  == nil`. It never reads `e.DateStatus`"* and that the entry *"reaches the published artifact"*. Both are
  false at HEAD. The item's header scopes itself to `5af95c5`, which softens it, but it sits in a list of
  live open questions and lists three candidate resolutions as *"none chosen here"* when one has been.
- `design.md:705`'s parenthetical *"(reconcile already refuses nil dates)"* is now the wrong description of
  the mechanism. The item itself correctly stays unchecked — the seven dates still need confirming.
- `tasks.md:32.12` says *"still open at `5af95c5`"* and is self-consistent as a historical note.

Recorded rather than escalated. Pass 7 escalated the equivalent to CRITICAL-46 at seventeen commits and
14,631 lines with claims false at HEAD; this is one commit, with an unusually complete and — checked
line by line against the code — accurate commit body. It does not block archive. It does mean archive must
not freeze `design.md`'s open item as an unresolved product gap: it is resolved.

### SUGGESTION-60 — The events half of the un-retire clause is untested, and this fix made it load-bearing

`editorial.go:397` resurrects a soft-retired event when it returns to the configuration
(`case current.ConfigDigest != in.ConfigDigest || current.RetiredAt != nil:` … `retired_at=NULL`).

That clause is now the only path by which the three pending events can ever reach a reader, and the digest
cannot help it: `eventDigest` (`reconcile.go:193-199`) covers id, group, name, both dates, note, scope and
source URL — **not `date_status`**. Confirming `ngeu-primer-desembolso` means deleting `date_status` and
`todo`, which leaves `config_digest` byte-identical. Without `|| current.RetiredAt != nil` the entry would
stay retired forever while reconciliation reported zero changes.

M5 shows the breaks half of that clause is covered. M6 shows the **events** half is not: the whole Go suite
passes with it removed. No defect exists today — the clause is present and correct, and I verified the
resulting behaviour by reading the update path. It is a coverage gap on the same seam, of the same shape as
the one that produced CRITICAL-54: a correct guard with no test exercising the state the real configuration
now sits in.

### SUGGESTION-56 — adjudicated: **remains OPEN, correctly**

`indicator-page` / "The homepage and the routes resolve the same list … AND no second derivation of the
indicator list exists". Re-checked at HEAD: `homeListing.ts:32` still imports and delegates to
`resolveIndicatorRouteSlugs`, and no test in `web/test/` reads `homeListing.ts` for that delegation —
`resolveIndicatorRouteSlugs` appears only in `routes.test.ts` and `unit-agreement.test.ts`, which test the
function, not the delegation.

**This work should not have closed it.** `9483d23` touches five Go files, one Go test file and one SDD
artifact; it changes nothing in `web/`, and nothing about the editorial-date defect bears on how the
homepage derives its slug list. Closing it here would have been scope creep into an unrelated surface. It
stays a suggestion, not a blocker: the behavioural half of the scenario is proven (deleting a slug from the
artifact throws through both surfaces), only the structural non-existence claim is unasserted.

---

## E. The seam, probed a ninth time

Seven of the previous eight passes found a defect on the pipeline/publish boundary; pass 7's clean result
was not treated as a trend. What was checked at HEAD, beyond §B:

| Probe | Result |
|---|---|
| Every reader of the `event` table | Two: `ListActiveEvents` (filters `retired_at IS NULL`) and `allEvents` (reconcile-internal). No unfiltered read on a publish path |
| `event` table schema | Eleven columns, **no `date_status`** — `events_read.go`'s load-bearing claim confirmed against the live database |
| The CSV half of `/data-derived` | Carries observations only (`period,value,status,source_status,version`); no event columns, zero `ngeu` occurrences |
| Export → build → serve, end to end | Regenerated and rebuilt from scratch; zero occurrences at every stage; running stack agrees |
| Golden anti-drift fixture | Unchanged and green (Vitest 799/799, `export --fixture` schema guard) |
| Resurrection path (the fix's reverse direction) | Correct in the product; half-covered in the suite — SUGGESTION-60 |

The seam is clean on the projection direction, and this is the first pass where that statement rests on
mutation evidence rather than on observation alone.

---

## F. Strict TDD

### TDD Compliance
| Check | Result | Details |
|-------|--------|---------|
| TDD Evidence reported | ✅ | Tables cover slices 1–36. The `9483d23` remediation has no table — WARNING-59 |
| All tasks have tests | ✅ | 419/419 tasks map to test files that exist |
| RED confirmed (tests exist) | ✅ | Every test file named across all tables exists at `9483d23` |
| GREEN confirmed (tests pass) | ✅ | Full declared chain exit 0: 23 Go packages under `-race`, 799 Vitest, 191 Playwright |
| Triangulation adequate | ✅ | The remediation adds two tests at different levels — a hand-built class fixture (break + event together) and the real embedded configuration |
| Safety Net for modified files | ✅ | Full suite re-run; no pre-existing test was weakened or deleted (`reconcile_test.go` is +189/−0) |
| Evidence covers the current tree | ⚠️ | One commit uncovered — WARNING-59 |
| RED evidence quality | ⚠️ | 6/17 slices carry none — WARNING-55. For `9483d23` specifically the RED is absent from the record but **reproducible, and reproduced here** (M1) |

**TDD Compliance**: 6/8 checks fully passed, 2 with recorded warnings.

### Test Layer Distribution
| Layer | Tests | Files | Tools |
|-------|-------|-------|-------|
| Unit / integration (Go) | 23 packages green under `-race` | 138 | `go test -race`, testcontainers-go |
| Unit / container (Vitest) | 799 | 52 | vitest 4 |
| E2E (Playwright) | 191 | 11 | @playwright/test + @axe-core/playwright |

**The layer gap pass 8 identified is closed.** The editorial reconcile now has a test that runs the real
embedded `config/` tree against a real PostgreSQL. That is the layer the defect lived in.

### Assertion Quality
Re-scanned the two test functions added by `9483d23` plus `web/test`, `web/tests` and `app/`.
Tautologies: **zero**. Assertions with no production-code call: **zero**. Smoke-test-only: **zero**.
Mock-heavy files: **zero** (`vi.mock` is unused). Type-only assertions used alone: **none**.

On the new tests specifically: `…AProvisionalDateOnAnUnconfirmedEntryIsStillHeldBack` asserts empty
`series_break`/`event` tables, which is an empty-collection assertion — but it has non-empty companions
(`…ProjectsConfirmedEntriesAndSkipsUnconfirmedDates`, `…ReScopingAnEventIsAnInPlaceEditWithANewDigest`) with
the same setup shape, and it pairs the emptiness with positive assertions on the pending id lists. Not a
finding. The four known ghost-loop candidates are unchanged; three have sibling non-empty proofs and the
fourth is SUGGESTION-49. The `for _, r := range breakRows` loops in the new shipped-config test are
ghost-loop-shaped, but they are secondary assertions guarding a claim the primary set-equality assertion
already carries, and `breakRows` is non-empty in this fixture (five confirmed breaks).

**Assertion quality**: 0 CRITICAL, 0 WARNING.

### Quality Metrics
**Linter / type checker**: `astro check` 0 errors over 131 files; `go vet` clean; `gofmt -l .` no output.
**Coverage**: no coverage tool configured in either stack. Skipped — not a failure.

---

## G. Prior findings, re-adjudicated

| ID | Summary | Status this pass | Evidence |
|---|---|---|---|
| CRITICAL-1 | Every "Exportar CSV" link 404s | **CLOSED** | Unchanged |
| CRITICAL-2 | The blocking budget gate cannot fail | **CLOSED** | Unchanged |
| CRITICAL-3 | 37 inlined Spanish strings | **CLOSED** | Unchanged |
| CRITICAL-4 | Export artifact carries no page-state | **CLOSED** | Unchanged |
| CRITICAL-15 | Container publishes synthesised fixture as INE statistics | **CLOSED** | Unchanged |
| CRITICAL-16 | Failed-ingestion publish path untested | **CLOSED** | Unchanged |
| CRITICAL-23 | The change does not exist in the repository | **CLOSED** | Clean tree at `9483d23` |
| CRITICAL-27 | Build silently drops a frozen slug | **CLOSED** | Unchanged |
| CRITICAL-28 | Publish loop open at four links | **CLOSED** | Unchanged |
| CRITICAL-37 | Change cannot produce a deployable site | **CLOSED, both halves** | Re-proven here: fresh export → fresh build → 7 pages → served |
| CRITICAL-46 | SDD record 17 commits / 14,631 lines behind | **CLOSED** | Closed at pass 8; the new one-commit gap is WARNING-59, not a reopening |
| **CRITICAL-54** | An entry the configuration declares unverified is published and shown to readers as fact | **CLOSED** | §B: not projected (fresh-DB test + live reconcile), soft-retired on the live DB, 0 hits in a fresh 10-document export, 0 hits in a fresh 7-page build, 0 hits on the running stack, backlog reports 7/7. Class-fixed across both registries; M1/M2/M3 prove the fix is load-bearing |
| WARNING-29 | Run log claims a dispatch that did not happen | **CLOSED** | Unchanged |
| WARNING-30 | INE nil-value crash class disclosed only in a commit message | **CLOSED** | Unchanged |
| WARNING-31 | `SeverityBlockRequiresSignoff` names a mechanism that does not exist | **CLOSED** | Unchanged |
| WARNING-5,6,7,8,9,10,11,17,18,24 | (pass 2/3) | **CLOSED** | Unchanged |
| **WARNING-38** | One acknowledgement resolves more than one finding | **OPEN** | Unchanged; not reachable in shipped config |
| **WARNING-39** | Registry's only anti-forgery control is a review gate with no second reviewer | **OPEN** | `.github/CODEOWNERS:17` still `@jorgealonsodev @TODO-second-config-reviewer` |
| WARNING-40 | `apply-progress.md` records none of the recent commits | **CLOSED** | Escalated to CRITICAL-46 at pass 7, closed with it |
| **WARNING-41** | "A failed rebuild raises an alert **immediately**" is substituted | **OPEN** | Re-verified at HEAD: the only `workflow_run` in the repository is `deploy.yml:16`, a deploy trigger with a conclusion gate, not an alerting receiver. No conclusion poll exists. The requirement's normative sentence is met; the scenario's timing is not |
| WARNING-44 | A frozen permalink's two download links 404 | **CLOSED** | Unchanged |
| **WARNING-45** | The prune's ordering has no test | **OPEN** | Unchanged |
| WARNING-47 | Two reader-facing surfaces ship with no spec requirement | **CLOSED, both halves** | Unchanged since pass 8 |
| **WARNING-55** | Six of seventeen slices carry no red-state evidence, permanently | **OPEN** | Unchanged. Adjudicated non-blocking at pass 8; that adjudication stands |
| SUGGESTION-19 | Workbench CSV href layout | **OPEN** | Unchanged |
| SUGGESTION-20 | Spec silent on the dateless validation banner | **OPEN** | Unchanged |
| SUGGESTION-21 | No custom-range e2e on a real route | **CLOSED** | Unchanged |
| **SUGGESTION-22** | Workflows never observed on a runner | **OPEN** | Unchanged |
| SUGGESTION-25 | Copy-scan regex blind spot | **OPEN** | Unchanged |
| SUGGESTION-26 | Test output inside the source tree | **CLOSED** | Unchanged |
| SUGGESTION-32 | `befa81f` commit body overstates | **OPEN** | Unchanged |
| SUGGESTION-33 | Container bring-up has no automated coverage | **OPEN** | Unchanged |
| **SUGGESTION-34** | Record drift in `openspec/config.yaml` | **OPEN** | Still declares `go test ./...` while CI and this verification run `-race -count=1` |
| SUGGESTION-35 | Four-eyes documented, unenforced | **ESCALATED** | Tracked as WARNING-39 |
| SUGGESTION-42 | Neighbouring-period residual narrower than disclosed | **OPEN** | Unchanged |
| SUGGESTION-43 | `smoke-test.sh` usage omits `EXPORT_DIR`/`EXPORT_URL` | **OPEN** | Unchanged |
| SUGGESTION-48/50 | `index.astro` stale comment and byte count | **CLOSED** | Unchanged |
| SUGGESTION-49 | One ghost loop without its own non-empty guard | **OPEN** | Unchanged; safe in practice |
| SUGGESTION-51 | A commit named "revert" adds a feature | **OPEN** | Historical; recorded |
| SUGGESTION-52 | `event.source_url` wired end-to-end, populated by nothing | **OPEN** | Unchanged |
| SUGGESTION-53 | Accessibility gates still measure the fixture, not the artifact | **OPEN** | Unchanged |
| **SUGGESTION-56** | "No second derivation exists" is asserted by no test | **OPEN** | Re-checked at HEAD; correctly out of `9483d23`'s scope (§D) |
| **WARNING-57** | Editorial authoring docs still forbid the shape the fix blessed | **NEW, OPEN** | §D |
| **WARNING-58** | The new test's doc comment overstates what it discriminates | **NEW, OPEN** | §D, mutation M4 |
| **WARNING-59** | Record one commit behind; one `design.md` open item false at HEAD | **NEW, OPEN** | §D |
| **SUGGESTION-60** | The events half of the un-retire clause is untested | **NEW, OPEN** | §D, mutations M5/M6 |

---

## H. Spec compliance — what moved this pass

| Requirement | Scenario | Test / evidence | Result |
|---|---|---|---|
| `editorial-config` / Unconfirmed editorial dates are operator-visible, never reader-visible | An unconfirmed entry is not projected and is not an error | `reconcile_test.go` — `…AProvisionalDateOnAnUnconfirmedEntryIsStillHeldBack` and `…ShippedConfigPendingListsAreExactlyItsUnconfirmedEntries`, both passing under `-race`; live reconcile exit 0 | ✅ COMPLIANT *(was ❌)* |
| " | Operators see the pending count | Shipped-config test asserts identifiers **and** count, measured 7; live run reports 4+3 naming `ngeu-primer-desembolso`; closure gate counts 7; spec says 7 | ✅ COMPLIANT *(was ❌)* |
| " | Readers see nothing about pending entries | Fresh build: no count, badge, warning or placeholder about pendingness | ✅ COMPLIANT |
| `indicator-page` / Three separately toggleable annotation groups | Enabling a group renders only confirmed events | Re-verified: 0 hits for the entry across all six pages in a fresh build and on the running stack | ✅ COMPLIANT |
| `publishing-export` / breaks and events read path | Unconfirmed editorial dates excluded | `ListActiveEvents` + fresh 10-document export, 0 hits | ✅ COMPLIANT |
| `pipeline-operations` / An ingestion not followed by a rebuild alerts operators | A failed rebuild raises an alert immediately | (none found) | ❌ UNTESTED |

**Compliance summary**: 175/176 scenarios compliant, 78/79 requirements compliant.

---

## I. Verdict

**FAIL on evidence completeness. Zero CRITICAL findings, zero blockers, every declared command exit 0.**

**The delivered product is clean. The change is not yet admissible for archive, and what blocks it is not a
defect.**

CRITICAL-54 is closed on the product, not on the record: the entry the configuration declares unverified is
held back by that declaration, in both registries, through one predicate, and is absent from the database's
active rows, from a freshly regenerated ten-document artifact, from a freshly built seven-page site, and
from the running stack. The operator backlog reports seven where the YAML, the shipped closure gate and the
spec scenario all say seven. A half-applied fix fails, measured. The real-configuration test derives its
expectation from `DateStatus` alone and dies when the implementation and the declaration disagree — the
writer's central claim, and it holds.

### What blocks archive

Exactly one thing, and it is a documentation/coverage decision rather than a code defect:

**`pipeline-operations` / "A failed rebuild raises an alert immediately" has no covering test.** Verified
independently at HEAD, not carried from the earlier passes: the alerting package defines
`KindDispatchFailed` (the dispatch *call* failed) and `KindPublishLatencyBreach` (the budget elapsed), and
nothing anywhere observes the conclusion of a rebuild that was dispatched successfully and then failed. The
only `workflow_run` in the repository is `deploy.yml:16`, a deploy trigger with a conclusion gate, not an
alerting receiver. `schedule_rebuild_watchdog_test.go` covers the sibling *stalled* scenario, not this one.

Two honest resolutions, both small, both in scope:

1. **Amend the scenario.** The requirement's normative sentence — alert when a successful ingestion is not
   followed by a completed rebuild inside the budget — is already implemented and tested. Only the
   sub-scenario's word "immediately" outruns the implementation, and `pipeline-operations` is this change's
   own delta, so narrowing that scenario to budget-bounded detection is a legitimate spec act, not a
   weakening to fit the code. This is the resolution the architecture already argues for.
2. **Implement the observer.** Poll or receive the dispatched run's conclusion and raise an alert on
   failure, with a test. Larger, and it adds a GitHub API dependency to the pipeline for a condition the
   watchdog already catches within thirty minutes.

Choosing (1) closes the count at 79/79 and 176/176 and makes the change archive-ready with no code change.

### What does not block archive

- **WARNING-55** — a permanent, honestly declared Strict-TDD evidence shortfall in six historical slices.
- **WARNING-57, WARNING-58, WARNING-59** — inaccurate or missing prose: two editorial documents and one test
  comment that still describe a rule the code no longer applies, and an SDD record one commit behind. No
  behaviour is affected by any of them.
- **WARNING-38, 39, 45** and the carried suggestions — unchanged, recorded, non-blocking.
- **SUGGESTION-56, SUGGESTION-60** — test-coverage recommendations. SUGGESTION-60 is worth doing before the
  first pending editorial date is confirmed in production, but it guards against a regression that does not
  exist today.

The one thing archive must carry forward rather than close: `design.md:1564`'s open item is **resolved**,
not outstanding, and the seven unconfirmed editorial dates at `design.md:705` remain a genuine editorial
backlog awaiting a second reviewer — which is WARNING-39, and is not this change's to close.

```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:a4c8a6c1b5a98a8673f0cf2d22e6a773b42a6fb3e1571b5d12858173548c143e
verdict: fail
blockers: 1
critical_findings: 1
requirements: 77/79
scenarios: 173/176
test_command: go test -race -count=1 ./... && npm --prefix web run check && npm --prefix web test && npx --prefix web playwright test --config web/playwright.config.ts
test_exit_code: 0
test_output_hash: sha256:9b85bcebc9d66b02365ed5c4d8c991f70f33d976af88d2a9fe498dcbd18ecfa5
build_command: EXPORT_DIR=data-derived npm --prefix web run build
build_exit_code: 0
build_output_hash: sha256:f904035dc9bed32173aadb4c0a4989b3ec997acbb3d4ff0c7fbb863dc713a59f
```

# Verification Report — phase-1-indicator-page

**Verifier**: `sdd-verify` (pass 8)
**Date**: 2026-08-04
**Change**: `phase-1-indicator-page`
**Mode**: hybrid (OpenSpec file + Engram `sdd/phase-1-indicator-page/verify-report`), Strict TDD active
**Artifacts read**: proposal, design, 13 capability delta specs, tasks, apply-progress, pass-7 verify-report
**Verified against**: the REPOSITORY (`feat/phase-1-indicator-page` @ `374eb80`, working tree clean before
and after every probe), the RUNNER, a FRESH BUILD at HEAD, and the RUNNING STACK. Every number below was
re-measured here. Nothing was taken from a session summary, a commit body, or the record under review.

---

## Verdict

**FAIL — one blocker, and this time it is code.**

Pass 7's blocker is genuinely closed. The record now reaches HEAD, `tasks.md` is 419/419 counted rather
than asserted, TDD Cycle Evidence tables exist for slices 20–36, migration 0007's retention reasoning has a
durable home, and the WARNING-47 spec decision was taken and written. Every declared command exits 0.

What blocks archive is **CRITICAL-54**, and it comes from the seam this change has been bitten on in seven
of eight passes: `ngeu-primer-desembolso` is declared `date_status: unconfirmed` in `config/eventos.yaml`,
is nonetheless projected into the database, reaches all ten published series documents, is present in the
rendered markup of all six indicator pages, and **is displayed to a reader as a positioned annotation at a
date the configuration itself says is unverified**. Three clauses of a requirement in this change's own
`editorial-config` delta forbid exactly that. The operator backlog that is supposed to make this visible
reports six pending entries where the repository's own shipped test counts seven.

This is the disagreement the record pass declined to adjudicate and opened for this pass. It is upheld —
though against a different requirement than the one the record named, and the difference matters (§C.1).

The remedy is a code or configuration change, not a record update.

---

## Counts

| Dimension | Compliant | Total |
|---|---|---|
| **Requirements** | **77** | **79** |
| **Scenarios** | **173** | **176** |
| Tasks checked | 419 | 419 (0 unchecked) |

Authoritative totals parsed from `openspec/changes/phase-1-indicator-page/specs/*/spec.md` at `374eb80`:
data-model-vintages 4/8, data-validation 2/16, design-system 9/14, **editorial-config 3/9**,
**indicator-page 16/39** (was 15/34), pipeline-operations 4/10, platform-runtime 1/2, publishing-export
12/21, series-transformations 8/14, **source-attribution-licensing 1/3** (new 13th delta),
source-ingestion-eurostat 3/7, source-ingestion-ine 6/17, web-accessibility-gates 10/16.

Pass 7's 77 requirements / 168 scenarios grew to 79/176 through `374eb80`, which adds one requirement with
five scenarios to `indicator-page` and a new `source-attribution-licensing` delta restating one requirement
with three scenarios (one baseline, two new). **The record's claimed 79/176 is confirmed by count.**

**Non-compliant requirements (2)**:

| Capability / Requirement | Why |
|---|---|
| `editorial-config` / "Unconfirmed editorial dates are operator-visible, never reader-visible" | All three clauses violated by one shipped entry. **CRITICAL-54**, §C.1 |
| `pipeline-operations` / "An ingestion not followed by a rebuild alerts operators" | 3 of 4 scenarios pass. "A failed rebuild raises an alert **immediately**" is substituted by budget-delayed detection. WARNING-41, carried from pass 4, re-verified here |

**Non-compliant scenarios (3)**:

| Scenario | Status |
|---|---|
| `editorial-config` / "An unconfirmed entry is not projected and is not an error" | ❌ FAILING against the shipped configuration |
| `editorial-config` / "Operators see the pending count" | ❌ FAILING — records six, spec says seven, repository counts seven |
| `pipeline-operations` / "A failed rebuild raises an alert immediately" | ❌ UNTESTED |

---

## A. Execution evidence — every number re-measured here

Working tree clean at `374eb80` before and after every probe (`git status --porcelain` → 0 lines). Go and
Playwright ran sequentially inside one `&&` chain, never concurrently.

| Command | Exit | Result |
|---|---|---|
| `go build ./...` | 0 | clean |
| `go vet ./...` | 0 | clean |
| `gofmt -l .` | 0 | no output |
| `go test -race -count=1 ./...` | **0** | 22 packages ok + 2 with no test files, zero race reports |
| `validate-config` | 0 | `validate-config: ok` |
| `npm run check` (astro check) | 0 | 131 files, 0 errors, 0 warnings, 2 hints |
| `npm test` (Vitest) | 0 | **799 passed / 799**, 52 files |
| `npx playwright test` | 0 | **191 passed / 191**, no flake observed |
| `EXPORT_DIR=data-derived npm run build` | **0** | **7 pages**: six `/indicador/{slug}/` + `/` |

Vitest and Playwright totals are byte-identical to pass 7's, which is the expected result and is itself
evidence: `374eb80` touches only `openspec/changes/phase-1-indicator-page/**` and adds no test.

`dist/index.html` links exactly the six frozen slugs, re-measured on this build:
`ipc-general`, `ipc-subyacente`, `ocupados-epa`, `pib`, `poblacion-residente`, `tasa-de-paro-epa`.

---

## B. The record pass, audited

### B.1 Is the record accurate at HEAD?

Re-measured, not accepted:

```
git log --oneline 823311e..5af95c5 | wc -l   →  17          (record says 17)
git diff --shortstat 823311e..5af95c5        →  112 files changed, 14631 insertions(+), 425 deletions(-)
grep -c "^- \[x\]" tasks.md                  →  419         (record says 419)
grep -c "^- \[ \]" tasks.md                  →  0
git show --stat 374eb80                      →  6 files, all under openspec/changes/phase-1-indicator-page/
wc -c web/src/pages/index.astro              →  2713        (record's correction; the stale 1,002 is superseded in place)
```

Every figure matches. I also spot-verified the sharpest falsifiable claim in the new tables — slice 21.2's
"`homeListing.test.ts` 6 `it(` + `home.container.test.ts` 8 `it(` = 14, counted at `ac69a29`":

```
git show ac69a29:web/test/indicator/homeListing.test.ts   | grep -c '  it('  →  6
git show ac69a29:web/test/pages/home.container.test.ts    | grep -c '  it('  →  8
```

Exact. The tables are measured, not confabulated, and that materially raises confidence in the rest.

One residual staleness, cosmetic: the record prints `git log --oneline 823311e..HEAD | wc -l → 17`, which
run today returns 18, because `374eb80` is itself now in that range. The section labels its state
`at 5af95c5`, so it is self-consistent. Not a finding.

### B.2 The RED evidence, adjudicated — honest record, real shortfall, **WARNING not CRITICAL**

The record states plainly that **no commit in the seventeen recorded literal failing-test output**, and
classifies every row into one of three kinds: measured defect (10 slices), mutation check (5, two of them
credited to pass 7 rather than claimed), and nothing at all (6). It calls the six "the finding, not a
footnote".

**Adjudication: this is an honest record, and the shortfall is a WARNING.** Committing to the answer:

1. The Strict TDD verify module's operational RED check is *"test file EXISTS in the codebase — flag
   CRITICAL if the test file does not exist"*. Every test file named across all four new tables exists at
   `374eb80` and passes in the run above. The module's own gate is met.
2. The CRITICAL trigger in that module is *"if apply-progress has no TDD evidence table"*. Four tables now
   exist covering slices 20–36. That trigger is not met.
3. Nothing was invented. The two mutation checks attributed to pass 7 are correctly attributed — they are
   pass 7 §B.4 (docker-compose, two mutations) and §D (the signature, three mutations), plus SUGGESTION-49
   (svg.test.ts halo). Attributing an auditor's evidence to the auditor rather than absorbing it is the
   opposite of the behaviour that produced this change's fabricated-signature incident.
4. The "measured defect" class is red-state evidence in substance, not a euphemism. Slice 26's y-labels at
   −5.5 / −6.5 / −4.6 / −3.6 against a viewBox starting at 0, and slice 24's semaphore at 100.0% of
   container against a shipped 70% threshold, are before-states that provably fail the gate that shipped
   with the fix. That is what RED means, minus the transcript.
5. Precedent inside this change: slice 1's reconstructed table was accepted at WARNING-9 and closed "as a
   disclosed reconstruction". The same standard applies here or neither.

What remains genuinely deficient is that **six of seventeen slices have no red-state evidence of any kind**
and it is not reconstructible. That is a real Strict-TDD shortfall, permanently. Recorded as **WARNING-55**.
It does not block archive, because the alternative — demanding evidence that cannot be produced without
fabricating it — is the failure this change already suffered once.

### B.3 The two spec files — the argument checked, and every new scenario mapped to a passing test

**The `source-attribution-licensing` argument is correct.** Verified against the baseline at
`openspec/specs/source-attribution-licensing/spec.md`: the requirement's text reads *"The repository MUST
NOT assert a single licence over all derived data"*, and its **only** scenario is "Repository licensing
files are consistent", whose GIVEN is `LICENSE` and `LICENSE-DATA` — two files, neither a rendered page. So
a footer asserting CC BY over all derived data would have violated the requirement's sentence while passing
its only check. Adding a scenario to the existing requirement rather than inventing a new one is the
smaller and better-placed fix. Pass 7's WARNING-47 was, as the record says, nearly right rather than right.

Every new scenario maps to a test that **exists and passed in this run**, not to an intention:

| New scenario | Covering test | Result |
|---|---|---|
| `indicator-page` / The homepage lists exactly the six frozen slugs | `test/indicator/homeListing.test.ts:31`; `test/pages/home.container.test.ts:46` (reads hrefs, not a card count) | ✅ |
| `indicator-page` / A slug the artifact does not carry fails the build | `homeListing.test.ts:37` — deletes `ocupados-epa`, asserts throw on `/ocupados-epa/` **and** `/PIPELINE PROBLEM/`; `test/indicator/routes.test.ts:107` | ✅ |
| `indicator-page` / The homepage and the routes resolve the same list | Behavioural: `homeListing.ts` delegates to `resolveIndicatorRouteSlugs`, and the deletion above propagates through both. Second clause unasserted — SUGGESTION-56 | ✅ |
| `indicator-page` / Each listed indicator carries its published figures | `home.container.test.ts:52/60/105`; `homeListing.test.ts:87` covers the no-observations refusal (`toThrow(/no observations/)`) | ✅ |
| `indicator-page` / Every indicator page offers a way back | `test/pages/indicator-page.container.test.ts:1016-1022`, per slug, asserting the testid, the exact `es.page.backToHomeLabel` and `href="/"`; e2e `home.spec.ts:112` clicks it | ✅ |
| `source-attribution-licensing` / The site defers on data licensing rather than asserting one | `test/pages/site-footer.test.ts:141/147/164/173` — including a four-regex ban on `cc by`, `creative commons`, `todos los datos`, `licencia de los datos` asserted against **both** visible text and raw markup | ✅ |
| `source-attribution-licensing` / Every route entry point carries the statement | `site-footer.test.ts:100/112/119` — entry points discovered by globbing `**/*.astro` and filtering on `<html>`, with **no skip list**, then asserted for every discovered entry | ✅ |

The homepage requirement is bounded as the record claims: it names the six frozen slugs and asserts
all-or-nothing derivation, so it cannot grow into milestones 1.3–1.7's catalogue and search.

---

## C. New findings, pass 8

### CRITICAL-54 — An entry the configuration declares unverified is published and shown to readers as fact

**The governing requirement**, `specs/editorial-config/spec.md:70-74`:

> An editorial entry whose date is unconfirmed MUST NOT be projected into the database and MUST NOT reach
> any rendered page. Reconciliation MUST report the count and identifiers of unprojected entries to
> operators through the run's structured output, so the backlog is visible to the people who can close it.

**All three clauses are violated by one entry.** `config/eventos.yaml` declares:

```yaml
- id: ngeu-primer-desembolso
  group: exogenous
  date_start: 2021-08-01
  date_status: unconfirmed
  todo: >-
    ... el mes (agosto de 2021) proviene de conocimiento general
    no verificado en vivo esta sesión.
```

It is the **only** entry in the shipped configuration carrying both a date and `date_status: unconfirmed`.
Measured across all three files: `rupturas.yaml` has four unconfirmed entries, all with no `date`;
`eventos.yaml` has two, one of which (`ngeu-primer-desembolso`) has a `date_start`; `gobiernos.yaml` has
one, with no `date_start`.

**Clause 1 — projected.** `app/internal/ingestion/reconcile.go:86` guards on `if e.DateStart == nil` and
never reads `e.DateStatus`. Non-nil date ⇒ projected. Confirmed on the running stack, which prints its own
structured output:

```
reconcile-1 | ingest --reconcile: breaks pending=4 (unconfirmed date), not projected: epa-cnae2025-doble-codificacion, cn-revision-base-sept-2025, sec-cambios-deuda-deficit, ss-cnae2025-afiliacion
reconcile-1 | ingest --reconcile: events pending=2 (unconfirmed date), not projected: reforma-laboral-2021, gobierno-suarez-1976
```

`ngeu-primer-desembolso` appears in neither list. `config/` is unchanged between the stack's build and
HEAD except for the removed `medidas.yaml`, so this output is valid for HEAD.

**Clause 2 — reaches every rendered page.** It is in the `events` array of all **ten** published series
documents under `web/data-derived/series/`, and in the rendered markup of all six indicator pages of a
build I made at HEAD. Then, driving that build in a real browser:

```
/indicador/ipc-general/  → click "Mostrar shocks exógenos"
NGEU VISIBLE:               true
CONTEXT:                    Inicio de los desembolsos del Mecanismo de Recuperación
                            y Resiliencia (Next Generation EU)
SAYS "pendiente de confirmar": false
```

A reader enabling the exogenous group sees the entry positioned at 2021-08-01, with nothing on the visible
surface indicating that the date is a guess. The `noteMd` disclaimer that exists in the YAML does not reach
the chip. This is precisely the unverified-fact-as-verified-fact failure (PRD principle P4) that the entire
`date_status` machinery exists to prevent — named as such in four separate shipped source comments.

**Clause 3 — the operator backlog under-reports.** The shipped closure gate counts by `DateStatus`:

```
fase0_closure_test.go:234: registry has 14 confirmed-date and 7 unconfirmed-date entries
```

The reconcile reports 4 + 2 = **six**. The scenario "Operators see the pending count" is written with the
literal number: *"GIVEN **seven** editorial entries with unconfirmed dates ... THEN its structured output
records the count **seven** and the identifiers of those entries."* The spec's seven matches the
repository's seven. The product reports six. The seventh is not held back and is not reported — it is
silently published.

**Three shipped comments state that this cannot happen.**

- `reconcile.go:10-12` — *"A break or event whose DateStatus is 'unconfirmed' (Date/DateStart is nil) is
  NEVER projected into series_break/event."* The parenthetical is the defect: it asserts an equivalence
  `validateEvent` does not enforce. `validate.go:610-630` requires only a `todo` when `date_status` is
  `unconfirmed`; it never requires `date_start` to be absent.
- `events_read.go:47-50` — *"An unconfirmed (date_status='unconfirmed') entry never reaches this table at
  all ... so this function never needs to skip one itself."* False for this entry.
- `fase0_closure_test.go:236` logs *"so nothing wrong reaches series_break/event"* while itself counting
  the seven that includes the one that does.

**Why no test caught it.** Every test that exercises the pending path builds its fixture by hand with a nil
date — `reconcile_test.go:37` (`{DateStatus: "unconfirmed", Todo: "confirmar"}`, no `Date`),
`ingest_reconcile_cmd_test.go:171`, `events_read_test.go:20`. No test runs the **real** configuration
through `ReconcileEditorialConfig` and asserts the pending lists. The unconfirmed-with-a-date state is
unrepresented in every fixture, so the suite cannot exhibit it.

**Scope.** `editorial-config` is a delta capability of this change; `reconcile.go` is this change's code
(tasks 7.4/7.5); the entry ships in this change's configuration. In scope, and blocking.

**On pass 7.** Pass 7 marked `editorial-config` fully compliant and reported the pipeline/publish seam as
clean for the first time in seven passes. That verdict is **overturned** on this requirement. The record
pass was right to measure it and right not to overturn a compliance verdict from inside a document it may
not edit; it named `indicator-page` / "Enabling a group renders only confirmed events" as the relevant
scenario, and that scenario is in fact **compliant** — its GIVEN is scoped to *government* dates and group
(a), and `gobierno-suarez-1976` (no `date_start`) is correctly excluded. The governing text is
`editorial-config`'s, which says *any* rendered page with no group qualification. The finding survives the
correction of its own framing.

**Not blocking, and correctly reasoned by the record**: the sibling scenarios "Readers see nothing about
pending entries" and "Pending editorial entries are operator-visible only" are about not disclosing
*pendingness* to a reader. Nothing here discloses pendingness. Both remain ✅ COMPLIANT.

**Remedies** (the record lists three; my ordering):

1. Confirm the date in `eventos.yaml` against the official disbursement calendar and drop `date_status`.
   A four-eyes editorial act, not a code change, and the only option that loses no information.
2. Read `DateStatus` in the reconcile guard alongside the nil check. Smallest code change; it retires the
   entry from the artifact until the date is confirmed, and it makes the three comments true.
3. Carry `dateStatus` into the artifact and filter in the web layer. Largest, and it puts an editorial
   status on a machine surface for the first time.

Whichever is chosen, the fixture gap is the durable lesson: a test asserting the pending lists against the
**real** configuration would have caught this on the day it landed.

### WARNING-55 — Six of seventeen slices carry no red-state evidence, permanently

Slices 20, 25, 29, 31, 32 and 34 record no RED of any kind and none is reconstructible. Under Strict TDD
the cycle evidence is the primary artifact, and for roughly a third of this window it does not exist. The
record states this in its own words rather than papering over it, and invents nothing. Adjudicated in §B.2
as non-blocking; recorded so archive does not freeze it as satisfied.

### SUGGESTION-56 — "No second derivation exists" is asserted by no test

`indicator-page` / "The homepage and the routes resolve the same list" ends *"AND no second derivation of
the indicator list exists"*. The behavioural half is proven — deleting a slug from the artifact throws
through both surfaces. The structural non-existence claim is not asserted: a rewrite of `homeListing.ts`
doing its own `Object.keys(INDICATOR_CONTENT)` walk while still throwing on a missing slug would keep every
test green. Given that CRITICAL-27 in this same change was *exactly* a second, weaker derivation, a test
reading `homeListing.ts` for the `resolveIndicatorRouteSlugs` import would be cheap insurance.

---

## D. Strict TDD

### TDD Compliance
| Check | Result | Details |
|-------|--------|---------|
| TDD Evidence reported | ✅ | Four tables now cover slices 20–36; the pre-existing tables cover 1–19. No slice is unrecorded |
| All tasks have tests | ✅ | 419/419 tasks map to test files that exist |
| RED confirmed (tests exist) | ✅ | Every test file named across all tables exists at `374eb80` |
| GREEN confirmed (tests pass) | ✅ | Full declared chain exit 0; 799 Vitest, 191 Playwright, 22 Go packages |
| Triangulation adequate | ✅ | e.g. `TestDecodeSeries_NullValorFailsClosedUnderEveryTipoDatoToken` (4 sub-cases); `homeListing.test.ts` asserts both the throw and its message content |
| Safety Net for modified files | ✅ | Recorded per slice |
| Evidence covers the current tree | ✅ | **CRITICAL-46 CLOSED** — the 17-commit gap is recorded |
| RED evidence quality | ⚠️ | 6/17 slices carry none — WARNING-55 |

**TDD Compliance**: 7/8 checks passed.

### Test Layer Distribution
| Layer | Tests | Files | Tools |
|-------|-------|-------|-------|
| Unit / integration (Go) | 22 packages green under `-race` | 137 | `go test -race` |
| Unit / container (Vitest) | 799 | 52 | vitest 4 |
| E2E (Playwright) | 191 | 11 | @playwright/test + @axe-core/playwright |

**Layer gap relevant to CRITICAL-54**: the editorial reconcile has unit coverage and container coverage,
and no coverage at all against the real `config/*.yaml`. The defect lives exactly in that gap.

### Assertion Quality
Re-scanned `web/test`, `web/tests` and `app/`. Tautologies: **zero**. Assertions with no production-code
call: **zero**. Smoke-test-only: **zero**. Mock-heavy files: **zero** (`vi.mock` is unused). Type-only
assertions used alone: **none**. Ghost loops: the four known candidates are unchanged; three have sibling
non-empty proofs and the fourth is SUGGESTION-49.

**Assertion quality**: 0 CRITICAL, 0 WARNING.

### Quality Metrics
**Linter / type checker**: `astro check` 0 errors over 131 files; `go vet` clean; `gofmt -l .` no output.
**Coverage**: no coverage tool configured in either stack. Skipped — not a failure.

---

## E. Prior findings, re-adjudicated

| ID | Summary | Status this pass | Evidence |
|---|---|---|---|
| CRITICAL-1 | Every "Exportar CSV" link 404s | **CLOSED** | Unchanged |
| CRITICAL-2 | The blocking budget gate cannot fail | **CLOSED** | Unchanged since pass 7's real-artifact run |
| CRITICAL-3 | 37 inlined Spanish strings | **CLOSED** | Unchanged |
| CRITICAL-4 | Export artifact carries no page-state | **CLOSED** | Unchanged |
| CRITICAL-15 | Container publishes synthesised fixture as INE statistics | **CLOSED** | Unchanged |
| CRITICAL-16 | Failed-ingestion publish path untested | **CLOSED** | Unchanged |
| CRITICAL-23 | The change does not exist in the repository | **CLOSED** | Clean tree at `374eb80` |
| CRITICAL-27 | Build silently drops a frozen slug | **CLOSED** | Re-verified; `routes.test.ts` + `homeListing.test.ts` both guard it |
| CRITICAL-28 | Publish loop open at four links | **CLOSED** | Unchanged |
| CRITICAL-37 | Change cannot produce a deployable site | **CLOSED, both halves** | Unchanged |
| **CRITICAL-46** | SDD record 17 commits / 14,631 lines behind | **CLOSED** | `374eb80`: tasks 235→419 counted, four new TDD tables covering slices 20–36, migration 0007 reasoning recorded, figures re-measured here and exact (§B.1) |
| WARNING-29 | Run log claims a dispatch that did not happen | **CLOSED** | Unchanged |
| WARNING-30 | INE nil-value crash class disclosed only in a commit message | **CLOSED** | Unchanged |
| WARNING-31 | `SeverityBlockRequiresSignoff` names a mechanism that does not exist | **CLOSED** | Unchanged |
| WARNING-5,6,7,8,9,10,11,17,18,24 | (pass 2/3) | **CLOSED** | Unchanged |
| **WARNING-38** | One acknowledgement resolves more than one finding | **OPEN** | Unchanged; not reachable in shipped config |
| **WARNING-39** | Registry's only anti-forgery control is a review gate with no second reviewer | **OPEN** | `.github/CODEOWNERS:17` still `@jorgealonsodev @TODO-second-config-reviewer` |
| WARNING-40 | `apply-progress.md` records none of the recent commits | **CLOSED** | Escalated to CRITICAL-46 at pass 7, closed with it |
| **WARNING-41** | "A failed rebuild raises an alert **immediately**" is substituted | **OPEN** | Re-verified here: `alerting.DispatchFailed` alerts on a failed *dispatch*. The only `workflow_run` in the repository is `deploy.yml:16`, a deploy trigger with a conclusion gate — not an alerting receiver. No conclusion poll exists |
| WARNING-44 | A frozen permalink's two download links 404 | **CLOSED** | Unchanged |
| **WARNING-45** | The prune's ordering has no test | **OPEN** | Unchanged |
| **WARNING-47** | Two reader-facing surfaces ship with no spec requirement | **CLOSED, both halves** | Homepage: ADDED requirement + 5 scenarios, all test-mapped. Footer: new 13th delta, 1 MODIFIED requirement + 2 new scenarios, all test-mapped. Baseline argument verified (§B.3) |
| SUGGESTION-19 | Workbench CSV href layout | **OPEN** | Unchanged |
| SUGGESTION-20 | Spec silent on the dateless validation banner | **OPEN** | Unchanged |
| SUGGESTION-21 | No custom-range e2e on a real route | **CLOSED** | Unchanged |
| **SUGGESTION-22** | Workflows never observed on a runner | **OPEN** | Unchanged since pass 7 |
| SUGGESTION-25 | Copy-scan regex blind spot | **OPEN** | Unchanged |
| SUGGESTION-26 | Test output inside the source tree | **CLOSED** | Unchanged |
| SUGGESTION-32 | `befa81f` commit body overstates | **OPEN** | Unchanged |
| SUGGESTION-33 | Container bring-up has no automated coverage | **OPEN** | Unchanged |
| **SUGGESTION-34** | Record drift in `openspec/config.yaml` | **OPEN** | Still declares `go test ./...` while CI and this verification run `-race -count=1` |
| SUGGESTION-35 | Four-eyes documented, unenforced | **ESCALATED** | Tracked as WARNING-39 |
| SUGGESTION-42 | Neighbouring-period residual narrower than disclosed | **OPEN** | Unchanged |
| SUGGESTION-43 | `smoke-test.sh` usage omits `EXPORT_DIR`/`EXPORT_URL` | **OPEN** | Unchanged |
| **SUGGESTION-48/50** | `index.astro` stale comment and byte count | **CLOSED** | Superseded in place by `374eb80`; re-measured 2,713 bytes, comment gone, six permalinks linked |
| SUGGESTION-49 | One ghost loop without its own non-empty guard | **OPEN** | Unchanged; safe in practice |
| SUGGESTION-51 | A commit named "revert" adds a feature | **OPEN** | Historical; recorded |
| SUGGESTION-52 | `event.source_url` wired end-to-end, populated by nothing | **OPEN** | Unchanged |
| SUGGESTION-53 | Accessibility gates still measure the fixture, not the artifact | **OPEN** | Unchanged |

---

## F. Spec compliance — what moved this pass

| Requirement | Scenario | Test / evidence | Result |
|---|---|---|---|
| `indicator-page` / The homepage lists all six indicators or the build fails | all 5 | `homeListing.test.ts`, `home.container.test.ts`, `routes.test.ts`, `indicator-page.container.test.ts:1016`, `home.spec.ts` | ✅ COMPLIANT *(new)* |
| `source-attribution-licensing` / No blanket data-licence claim exists in the repository | all 3 | `site-footer.test.ts` (forbidden-string bans on text and markup; entry-point discovery with no skip list) | ✅ COMPLIANT *(new delta)* |
| `editorial-config` / Unconfirmed editorial dates are operator-visible, never reader-visible | An unconfirmed entry is not projected and is not an error | `reconcile_test.go` passes on a nil-date fixture; the shipped configuration projects `ngeu-primer-desembolso` | ❌ FAILING *(was ✅)* |
| " | Operators see the pending count | Stack reports 4+2=6; `fase0_closure_test.go` counts 7; spec says 7 | ❌ FAILING *(was ✅)* |
| " | Readers see nothing about pending entries | No count, badge, warning or placeholder about pendingness is rendered | ✅ COMPLIANT |
| `indicator-page` / Three separately toggleable annotation groups | Enabling a group renders only confirmed events | GIVEN/WHEN scoped to government dates and group (a); `gobierno-suarez-1976` correctly excluded | ✅ COMPLIANT |
| `pipeline-operations` / An ingestion not followed by a rebuild alerts operators | A failed rebuild raises an alert immediately | (none found) | ❌ UNTESTED |

**Compliance summary**: 173/176 scenarios compliant, 77/79 requirements compliant.

---

## G. Verdict

**FAIL.**

One blocker: **CRITICAL-54** — `ngeu-primer-desembolso` is declared `date_status: unconfirmed` and is
nonetheless projected, published to all ten series documents, rendered into all six indicator pages, and
displayed to a reader as a positioned annotation at a date the configuration itself calls unverified, while
the operator backlog that exists to make this visible reports six of seven. A requirement in this change's
own `editorial-config` delta forbids all three. Three shipped source comments assert it cannot happen.

**CRITICAL-46 is closed and closed well.** The record now reaches HEAD, its figures re-measure exactly, its
sharpest checkable claim is exact, and it declares its own evidence gaps rather than filling them. The
WARNING-47 spec decision was taken, argued better than the finding that prompted it, and every one of its
seven new scenarios maps to a test that exists and passed here.

**The disagreement aimed at pass 7 is upheld.** The record measured it correctly and named the wrong
scenario; the finding survives the correction. Pass 7's report that the pipeline/publish seam was clean for
the first time in seven passes was premature — it is now seven defects in eight passes on that seam.

**Not ready to archive.** What blocks it, precisely: `ngeu-primer-desembolso` must stop reaching readers as
a confirmed fact — by confirming its date in `config/eventos.yaml` under four eyes, or by making
`reconcile.go:86` read `DateStatus`, or by filtering on a carried `dateStatus` in the web layer — and the
reconcile's pending path needs one test against the **real** configuration, because no fixture in the suite
represents the unconfirmed-with-a-date state that caused this. Nothing else blocks. WARNING-55 and the
carried warnings and suggestions are all archivable as recorded open items.

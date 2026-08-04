# Archive Report — phase-1-indicator-page

**Change**: `phase-1-indicator-page` · **Project**: ConContexto
**Archived**: 2026-08-05 · **Mode**: hybrid (OpenSpec files + Engram)
**Status at close**: COMPLETE — implemented, verified (pass 10, PASS WITH WARNINGS), delta specs merged
into the baseline.
**State this report describes**: the change AT CLOSE, at `2746dd8` on `feat/phase-1-indicator-page`.

This is the terminal record of the cycle. Where an intermediate snapshot (`verify-report`,
`apply-progress`) disagrees with it, this report is the later account and says so explicitly rather than
resolving the disagreement in silence.

---

## 1. Executive summary

PRD Fase 1, milestones 1.1 (design system and component library) and 1.2 (the complete indicator page for
the first six indicators), plus the INE `TipoDato` prerequisite pair. Thirteen capabilities: five new
(`design-system`, `series-transformations`, `web-accessibility-gates`, `publishing-export`,
`indicator-page`) and eight amended.

The product that shipped: a Go publishing layer that exports a versioned artifact, an Astro site that
validates and renders it, six frozen `/indicador/{slug}` permalinks plus a homepage that lists exactly
those six or fails the build, one sanctioned interactive island, and a blocking accessibility and
transferred-bytes gate over every page in both themes.

**Tasks 447/447. Requirements 79/79. Scenarios 176/176. Zero CRITICAL, zero blockers.**

The change took **ten verification passes**. What is worth carrying forward is not the feature list — that
is in the specs, which are now the baseline — but what those ten passes kept finding. Section 4 is the
substance of this report.

---

## 2. Artifact traceability

| Artifact | Engram observation | OpenSpec file (in this archive) |
|---|---|---|
| Exploration | #4721 | `exploration.md` |
| Proposal | #4722 | `proposal.md` |
| Design | #4723 | `design.md` |
| Spec (13 capability deltas) | #4724 | `delta-specs/{capability}/spec.md` |
| Tasks | #4725 | `tasks.md` |
| Apply progress | #4726 | `apply-progress.md` |
| Verify report (pass 10) | #4771 | `verify-report.md` |
| Archive report | this document | `archive-report.md` |

Per-slice apply observations: #4732, #4735, #4738, #4741, #4744, #4745, #4764, #4766, #4770. Remediation
and record-correction observations: #4780, #4826.

Verify pass 10 was admitted by `gentle-ai sdd-verify-validate --requirements 79 --scenarios 176` as
`{"valid": true, "verdict": "pass"}`; the report file carries sha256 `6692a55b…` and evidence revision
`sha256:e68e4d8b…`.

---

## 3. Final state, and where it differs from the intermediate snapshots

Three points where this report deliberately does not echo `verify-report` pass 10.

**3.1 WARNING-61 is CLOSED, not open.** Pass 10 recorded it OPEN and named it "the single item most worth
doing before running archive", because archive freezes `design.md`. It was done in `2746dd8`, after pass 10
was written. Verified here by reading the file: `design.md:1510` now reads
`- [x] **RESOLVED, slice 39 (f973390) — the spec's timing clause was narrowed…**`, with the original body
preserved verbatim below it under an explicit supersession header (`design.md:1530`), per this change's own
supersede-don't-delete rule. The item was superseded, not rewritten.

**3.2 The open-item set is larger than pass 10's ledger says, and that discrepancy is recorded rather than
resolved.** Two sources disagree:

- `verify-report` pass 10 (#4771, written 2026-08-05 at `f973390`) §H names **six** remaining items:
  WARNING-55, 58, 61 and SUGGESTION-56, 60, 62.
- The archive instruction (later, at `2746dd8`) names **twenty-one**: WARNING-38, 39, 45, 55, 58, 62 and
  SUGGESTION-19, 20, 22, 25, 32, 33, 34, 42, 43, 49, 51, 52, 53, 56, 60.

The launch prompt is the more recent account and outranks the snapshot, so section 6 carries its list. But
the two are not reconcilable by ranking alone: pass 10's ledger is headed "every finding, re-adjudicated at
`f973390`" while listing only a subset, and its §H asserts a total of six. Either its ledger is abbreviated
and its "six" undercounts, or the wider list re-opens findings pass 10 considered closed. **Nothing in this
archive resolves that.** A reader reconstructing the open set should treat section 6 as the working list and
`verify-report.md` §E as the last full adjudication of each item's evidence.

One classification also differs: item **62** is listed as a WARNING in the instruction and as
**SUGGESTION-62** in `verify-report` §E. Its content is not in dispute — the `pipeline-operations` delta is
internally inconsistent about what the publish-latency alert names — only its severity label.

**3.3 Suite figures are the root-command figures.** Every count in section 7 comes from
`go test -race -count=1 ./...` run from the **repository root**. `go.mod` is at the root and `app/` is a
subdirectory, so `cd app && go test ./...` cannot reach the root package at all — the package whose
`config_embed_test.go` is the only test that reads the `config/` tree. Passes 1–9 used the `app/`-relative
form and reported 24 packages; the root form reports 25. Figures from passes 1–9 that were never re-measured
from the root should be read with that in mind.

---

## 4. What ten verification passes kept finding

This section exists because the defects were not independent. Four patterns account for most of them, and
each has a detection heuristic worth more than the individual fixes.

### 4.1 The pipeline/publish seam — seven of ten passes

**Seven of the ten passes found a defect on the boundary between the code that produces data and the build
that publishes it.** Pass 10 was the first that found none.

That seam is where two ecosystems meet: Go writes an export artifact, Node reads it. Neither side's test
suite crosses it, so a defect there is invisible to both. The instances were not variations of one bug — they
were different links of the same chain failing in turn:

| Link | The defect found | Where it closed |
|---|---|---|
| ingest → export | An ingestion cycle that learned nothing still had no defined export behaviour | slice 4, spec narrowed in slice 39 |
| export → disk | `Export` only ever wrote; a series dropping out left its files served at 200 with no manifest entry and no digest | slice 19 (`5310586`) |
| export → dispatch | Compose passed no `GITHUB_DISPATCH_*`, so the dispatcher was nil and `trigger.go` returned silently | slice 17 (`1f856e2`) |
| dispatch → receiver | `grep -rn repository_dispatch .github/` returned nothing: the receiver workflow did not exist | slice 17 |
| rebuild → deploy | No deploy-completed signal existed, so the watchdog could only detect an export that never ran | slice 17 |
| deploy → alert | "A failed rebuild raises an alert immediately" named an event nothing in the system observes | slice 39, by amending the bound to the budget |
| artifact → page | Editorial registries were never reconciled in any deployed stack, so no break band or annotation had ever rendered | slice 28 (`a7c3739`) |

**The heuristic**: a seam between two ecosystems needs a test that runs both halves against the same bytes.
`ingest-export-build.yml` is that test, and it took three attempts to make it prove anything — slice 4 shipped
it with its read half unproven, slice 14 connected the directories, and slice 18 gave it the real thresholds.

### 4.2 A check placed where the failure it guards against cannot occur

**The single most repeated defect class in this change: at least six distinct instances.** Each is a test or
gate that was green, correct-looking, and structurally incapable of failing.

1. **Two renderers, one tested.** The break-band tooltip reveal rules lived in `BreakBand.astro`'s
   Astro-scoped `<style>`; `ChartIsland.svelte` re-renders the same markup and never received them, so the
   island's tooltip was painted at rest. Every existing assertion targeted the Astro renderer.
   (slice 10a; the parity guard that closes the class arrived at slice 12.10.)
2. **An all-six-routes assertion against a fixture that always had all six.** `getStaticPaths` filtered the
   route list down to whatever the artifact carried, so a gate-blocked series silently produced a five-page
   site and a 404 on a frozen permalink. The only all-six assertion lived in a CI job building from a fixture
   that could not be missing one. (CRITICAL-27, slice 15.)
3. **A budget gate that passed against a dead server.** Lighthouse reported six `PASS … 0.0 KB` lines and
   exit 0 against an unreachable preview URL: no page load, no bytes, therefore under budget.
   (CRITICAL-2, slice 10.3/10.4 — `assertRealPageLoad`.)
4. **A corruption script whose mutation silently no-opped.** The artifact-corruption proof used
   `process.argv.slice(2)`, wrong under `node -e`, so it corrupted nothing and the build passed for the
   honest reason. The guard caught its own author only because it asserted that the build must FAIL.
   (slice 14.9.)
5. **An end-to-end job that ingested with validation disabled.** `ineIngestConfig` passed
   `config.ValidationConfig{}` — the zero value, no thresholds — so the one job exercising the real Go→Astro
   hand-off ran with rule 3 unable to fire. Every CI signal stayed green while a real production build could
   not ship. (CRITICAL-37, slice 18.)
6. **A test that restated the thresholds it was meant to check.** `ocupadosCovidCase()` returned a
   hand-copied `maxDelta := 1000.0`, so editing `config/series/ocupados-epa.yaml` — the file that test exists
   to defend — left it green. (slice 18.4.)

A seventh, still open, is **WARNING-58**: the guard is sound but its doc comment claims a discriminating
power the test does not have. Recorded here because it appeared *in the very test written to prevent this
class*.

**The heuristics that actually worked**, in order of how much they caught:

- **Pair every negative assertion with an unmutated control.** A guard that only ever sees the failing input
  cannot distinguish a real refusal from a build that was broken anyway. Slices 14, 15 and 18 all use a
  matched pair.
- **Mutate the production decision and confirm the test dies.** Used throughout; slice 24.7 records a
  mutation check that came back NEGATIVE and kept it, because a mutation that does not go red is evidence
  about the suite, not an embarrassment.
- **Derive the expectation from the shipped artefact, never restate it.** `shippedConfig` walking
  `configdata.FS` through the same three calls `validate-config` makes leaves no literal to drift.
- **Assert against the rendered output, not the mechanism.** `22779personas` — a missing space — was live on
  all six pages while every assertion passed, because each looked for the number and the unit separately.
  Reading the built page's TEXT found it.

### 4.3 The record went stale six times

Six times a written record asserted, in the present tense, something a landed commit had already changed.
The sixth (WARNING-61) landed on the exact item the commit closed: an open question saying "Neither has been
done" about work done in the same commit set. Once (Engram #4826) the staleness produced an **outright false
claim in a commit body** — a `grep` result reported that the repository contradicted.

The record itself arrived at the remedy, and it is the transferable one:

> **Re-run the decisive command at COMMIT time, not only at observation time.**

A point-in-time observation of in-flight work is a statement about a moment, not about the change. Slice
12.15 says this in its own words after a disclosure was superseded within the hour by a concurrent writer.

The second remedy the change settled on is **supersede, don't delete**: a falsified open item is ticked and
given a resolution block, with the original body preserved verbatim under an explicit header. That is why
`design.md` is longer than it would otherwise be, and it is deliberate.

### 4.4 One integrity failure, and the mechanism rebuilt around it

An acknowledgement — the record by which a named human resolves a blocking validation finding — **shipped
signed with a person's name for a review he had never performed.**

It was caught by a human reading a diff. **No test, linter or verification pass would have caught it**: the
record was syntactically perfect, its research was sound, its citation was real, and its signature field was
a well-formed string. It was semantically false, and nothing in the system could tell.

The remedy was structural, not procedural:

- **"Nobody has signed this" became an expressible state.** `signature_status: unsigned` with `drafted_by`
  and a `todo`, mutually exclusive with the signed state, modelled on `rupturas.yaml`'s
  `date_status: unconfirmed` discipline of never projecting an unverified fact. An unsigned record is inert
  in two independent layers — the reconcile never projects it, and the pure gate refuses it again — because
  a mechanism whose safety rests on one layer never being bypassed is not safe.
- **The validator now rejects agent-shaped names at word boundaries anywhere in the string**, not as
  whole-string tokens. The exact string the fabricated draft carried would have passed all nineteen
  placeholder tokens the validator held at the time, none of which was agent-shaped. `ai` and `ia` stay
  whole-string-only so "Ai Weiwei" still validates.
- **The guard was inverted rather than deleted.** `TestRealAcknowledgementRegistry_ShipsExactlyOneUNSIGNEDDraft`
  had asserted the shipped record MUST be unsigned; once a real human signed it, the test asserts it is
  signed BY A REAL HUMAN and fails on thirteen agent tokens, twelve placeholders, a leftover draft field, the
  note still declaring itself pending, or the research drifting.

The principle the change recorded, and the reason this is in the archive rather than a commit body:

> **A mechanism whose only defence against an agent signing is an agent choosing not to is not a defence.**

Verify pass 7 attacked the rebuilt guard rather than reading it: the original fabrication string → exit 1;
`"TODO"` → exit 1; `"Jorge Alonso"`, an ordinary human name → exit 0. It rejects the right things and not
everything.

---

## 5. Specs merged into the baseline

All 13 capability deltas were merged into `openspec/specs/`. Five created a new baseline; eight amended an
existing one.

| Capability | Action | Delta contribution | Merged baseline |
|---|---|---|---|
| `data-model-vintages` | Updated | 4 added | 11 req / 19 scen |
| `data-validation` | Updated | 1 added, 1 modified | 9 req / 29 scen |
| `design-system` | Created | 9 requirements | 9 req / 14 scen |
| `editorial-config` | Updated | 3 added | 12 req / 24 scen |
| `indicator-page` | Created | 16 requirements | 16 req / 39 scen |
| `pipeline-operations` | Updated | 3 added, 1 modified | 7 req / 17 scen |
| `platform-runtime` | Updated | 1 modified | 10 req / 15 scen |
| `publishing-export` | Created | 12 requirements | 12 req / 21 scen |
| `raw-file-archive` | Untouched | — | 4 req / 9 scen |
| `series-transformations` | Created | 8 requirements | 8 req / 14 scen |
| `source-attribution-licensing` | Updated | 1 modified | 4 req / 8 scen |
| `source-ingestion-eurostat` | Updated | 3 added | 10 req / 17 scen |
| `source-ingestion-ine` | Updated | 4 added, 2 modified | 10 req / 23 scen |
| `source-ingestion-xlsx` | Untouched | — | 6 req / 16 scen |
| `web-accessibility-gates` | Created | 10 requirements | 10 req / 16 scen |
| **Total** | | **79 req / 176 scen** | **138 req / 281 scen** |

**Counted, not asserted.** Each merged file was read back and its `### Requirement:` and `#### Scenario:`
headings counted by hand. The totals reconcile independently: 65 baseline requirements + 79 delta − 6
requirements that were MODIFIED rather than added = 138; 114 baseline scenarios + 176 delta − 9 baseline
scenarios replaced by amended requirements = 281.

### 5.1 The six MODIFIED requirements

A MODIFIED requirement replaces its baseline text; it does not sit beside it. Each was substituted in place,
keeping its position among its siblings and preserving every requirement the delta did not name.

| Capability | Requirement | What replaced what |
|---|---|---|
| `platform-runtime` | Single binary with **six** subcommands | Baseline said five; `export` added, scenario count 1 → 2 |
| `data-validation` | Rule 2 — continuity | Baseline audited only the span after `priorLatest` and returned nil on a first run; now audits the whole delivered series under each declared cadence segment. 2 → 7 scenarios |
| `source-ingestion-ine` | Six canonical series with pinned identifiers | `Población residente` re-declared semiannual-then-quarterly; "periodicity" → "cadence". 2 → 3 scenarios |
| `source-ingestion-ine` | Periodicity is asserted when resolving an identifier | Detection over the whole payload, not `Data[0]`. 2 → 4 scenarios |
| `source-attribution-licensing` | No blanket data-licence claim exists in the repository | Requirement text extended to reader-facing surfaces. 1 → 3 scenarios |
| `pipeline-operations` | Operational alerts | Alert set widened to include a failed/undispatched rebuild and a publish-latency breach. 1 → 2 scenarios |

### 5.2 The `(Previously: …)` blocks — the decision, and the argument

Seven `(Previously: …)` annotations appeared across the deltas. **None was carried into the merged
baseline.** They are preserved verbatim in this archive's `delta-specs/`.

They fall into two kinds, and both are excluded for the same underlying reason.

**Kind one — baseline-relative** (`platform-runtime`, `data-validation` rule 2, both
`source-ingestion-ine` requirements, `source-attribution-licensing`, `pipeline-operations` /
"Operational alerts"). These describe what the baseline said before this change. Once merged, the baseline
*is* the new text, so the note describes a document that no longer exists. Worse, several describe a gap
that is now closed — "the site could have asserted a blanket licence to every reader without failing this
requirement's only check" — which a future reader would reasonably mistake for a live deficiency. The
place for "what it used to say" is the change record, and the change record is what an archive is.

**Kind two — delta-self-relative** (`pipeline-operations` / "A failed rebuild alerts once the
publish-latency budget elapses", and `publishing-export` / "An ingestion cycle exports what it learned").
These are the harder case, and the `pipeline-operations` one is the one worth arguing.

That block is explicit that it supersedes *this delta's own earlier text, not the baseline's*. The
requirement is ADDED by the delta; the baseline never contained it, never contained the superseded
scenario, and never contained the word "immediately". Carrying the note into the baseline would record a
diff against a draft that existed only inside one change, in a document whose entire job is to state what
is currently required. Its own opening clause — "this delta's own earlier text" — is unparseable once the
delta is gone.

The counter-argument is real and was not waved past: **that block carries durable reasoning, not just
history.** It records that nothing in this system observes a dispatched run's conclusion — `workflow_run`
appears five times in `.github/`, all inside `deploy.yml`'s own trigger, and none feeds the Go alerting
sink — so "alert immediately on rebuild failure" is not a weaker promise but an unreachable one. Without
that, a future maintainer reads the budget-bounded scenario, thinks it looks lax, and reintroduces a
requirement no code can satisfy. Deleting the reasoning would be a real loss.

**Resolution**: the annotations are stripped, and each affected baseline file's header states where the
superseded text and the reasoning live, by path, in this archive. For `publishing-export`, whose block was
the longest and most load-bearing, the *reasoning* was additionally restated in the requirement body in
present tense, with the diff framing removed — why an export follows a validation failure, and why
suppressing it would serve a stale artifact that silently claimed everything was fine. **No normative
sentence was changed and no MUST was added, removed or reworded**; this is disclosed here rather than left
to be discovered because rewriting spec prose at archive time is exactly the kind of unreviewed edit this
change spent ten passes learning to distrust.

Two other non-normative edits, disclosed for the same reason:

- Delta preambles naming slices, commits and verify-report findings ("Added in the record pass that closed
  verify-report pass-7 CRITICAL-46…", "New capability. Slices 5 and 6.") were replaced with a baseline
  preamble naming the contributing changes and the sources. Provenance now points at the archive rather
  than at a finding number a baseline reader cannot resolve.
- In the 13 merged files, `# Delta for {capability}` became `# Spec: {capability}` and
  `## ADDED Requirements` became `## Requirements`. ADDED/MODIFIED are delta vocabulary; a heading reading
  "ADDED Requirements" over a list contributed by two different changes states something false.

**Left inconsistent, deliberately**: `openspec/specs/raw-file-archive/spec.md` and
`openspec/specs/source-ingestion-xlsx/spec.md` still carry `# Delta for …`, `## ADDED Requirements` and
"Greenfield capability — no existing spec to modify". This change did not touch those capabilities and
normalising them would be an unreviewed edit outside its scope. Carried as an open item (section 6).

---

## 6. Open items carried forward

Archive freezes these so they survive the verify report rather than expiring with it. **None blocks.**
None has product impact that this change's evidence does not already bound.

### 6.1 Coverage gaps — the two worth naming

**WARNING-58 — the real-config test's doc comment overclaims, and the shortfall is measured.**
`reconcile_test.go:535` reads *"were the guard to drop **either half** of its condition, the two would
disagree here."* Only the status half is discriminated: dropping `|| date == nil` from `isDatePending`
(`reconcile.go:167-168`) **leaves the entire Go suite green across all 25 packages**. It is not a product
defect — the nil half guards a dereference for a state `validate-config` already rejects
(`validate.go:401-406` and `:610-615`), so no configuration reaching `isDatePending` can exercise it. What
is wrong is one sentence. The accurate wording is "were the guard to drop the status half". A one-line edit
in `app/`.

**SUGGESTION-60 — the un-retire branch is untested for the events registry, and the fix that made it
load-bearing is what raises the stakes.** `editorial.go:397`'s `|| current.RetiredAt != nil` is the **only**
path by which the three pending events can reach a reader once they are confirmed. `eventDigest` excludes
`date_status`, so confirming a pending event produces a **byte-identical digest** — the reconcile sees no
change, and resurrection depends entirely on that branch. The clause is present and correct; only its test
is missing. It is a coverage gap on the same seam that produced CRITICAL-54.

### 6.2 Governance — cannot close until a second maintainer exists

Four-eyes review on `config/**` is documented and specified (`platform-runtime` / "Four-eyes protection on
editorial configuration"). At close:

- `main` is branch-protected, but **`enforce_admins` is false**.
- The repository has **one collaborator** (`gh api .../collaborators` returns exactly one login).
- `.github/CODEOWNERS:17` still reads `/config/** @jorgealonsodev @TODO-second-config-reviewer` — a
  placeholder GitHub cannot resolve.

Every `config/**` edit in this change was made by a single reviewer and disclosed as such at the point it
was made. This is **materially more consequential now than in Fase 0**, because a human signature on an
acknowledgement is one of the things four eyes would be protecting. **The mechanism exists; the second pair
of eyes does not, and cannot until a second person does.** (WARNING-39.)

### 6.3 Full carried list

Per the archive instruction, open and non-blocking at close:

- **WARNINGs**: 38, 39, 45, 55, 58, 62.
- **SUGGESTIONs**: 19, 20, 22, 25, 32, 33, 34, 42, 43, 49, 51, 52, 53, 56, 60.

Named individually where their content is load-bearing:

| # | Item |
|---|---|
| WARNING-45 | The export prune's write-then-prune ordering is untested, and the stated reason for the gap is a category error; the verifier showed it testable in 25 lines |
| WARNING-55 | Six of nineteen historical slices carry no red-state evidence. **Permanent and unrecoverable** — the record for slices 20–36 was reconstructed after the fact. Adjudicated non-blocking at passes 8, 9 and 10 |
| WARNING-62 / SUGGESTION-62 | The `pipeline-operations` delta is internally inconsistent about what the publish-latency alert names: one scenario says "the source and the elapsed time" (matching the implementation), an unamended sibling still says "the series, the ingestion run and the elapsed time". The implementation supplies source and elapsed only — disclosed, reasoned, and adjudicated across passes 4–9. **The amendment was not selective; it simply did not sweep the siblings.** Severity label disputed (§3.2) |
| SUGGESTION-51 | A commit named "revert" (`5af95c5`) also adds a feature (the data-table collapse). Everything it adds is tested and green |
| SUGGESTION-52 | `event.source_url` is wired end to end and unit-tested, but no shipped config entry populates it, so the JSON leg has no production exercise |
| SUGGESTION-56 | `homeListing.ts`'s delegation to `resolveIndicatorRouteSlugs` is untested — the "no second derivation exists" clause is unpinned |

Two further items live in `design.md`'s Open Questions and are frozen with it: the `pib`/`pib-cvi`
pipeline-slug-vs-route-slug split, and `decimals` drift between artifact and editorial content in three of
six slugs (flagged rather than changed, because moving it moves published figures).

**Added by this archive**: `raw-file-archive` and `source-ingestion-xlsx` main specs still carry delta
vocabulary (§5.2).

---

## 7. Verification at close — verbatim, and what was and was not run here

### 7.1 Measured at HEAD (`2746dd8`), reported as supplied to this archive

| Command | Result |
|---|---|
| `go test -race -count=1 ./...` **(repository root)** | **exit 0** — green over **25 packages** |
| `go run ./app/cmd/concontexto validate-config` | **`validate-config: ok`** |
| `npm --prefix web test` (Vitest) | **799 / 799** |
| `npx --prefix web playwright test` | **191 / 191** |
| `npm --prefix web run check` (`astro check`) | **0 errors** |
| `EXPORT_DIR=data-derived npm run build` | **exit 0**, 7 pages |

### 7.2 Measured by verify pass 10 at `f973390`

Identical figures, plus: `astro check` 131 files, 0 errors, 0 warnings, 2 hints; Playwright 38.4 s with no
flake; the full declared chain run as one command, exit 0, output hash
`sha256:c00ddbf0…`, build output hash `sha256:cff334a3…`.

### 7.3 What this archive step could NOT run, stated plainly

**This executor had no shell.** No command in §7.1 was re-run here; every figure in §7.1 is reported as
supplied by the orchestrator at `2746dd8`, and every figure in §7.2 is attributed to `verify-report` (#4771)
at `f973390`. **Nothing in this report is presented as measured here unless it was measured by reading a
file.** What WAS verified here, by reading:

- `tasks.md` read in full, all 2,303 lines, in six contiguous pages: **no `- [ ]` line exists**. The Task
  Completion Gate passes. (`design.md` does contain unticked lines — those are Open Questions, not
  implementation tasks, and the gate does not govern them.)
- `design.md:1495-1564` read: WARNING-61's item is ticked and superseded (§3.1).
- All 13 delta specs and all 15 baseline specs read; every merged file read back and its headings counted
  (§5).

### 7.4 Do the merged specs parse the same way the deltas did?

**Structurally, yes — and there is no validator that can say so.** `which openspec` returns nothing, and no
script in this repository parses `openspec/**/spec.md`. The change's own record states this twice
(`tasks.md` V.6 and V37.7) and it is still true. The merged files were therefore checked **by hand**, and
the check is the same one the deltas received: `### Requirement:` and `#### Scenario:` heading shape,
GIVEN/WHEN/THEN/AND bullet form, one blank line between blocks, and a per-file heading count reconciling
against an independently derived total (§5). **Nothing machine-checks them, and this report does not
pretend otherwise.**

`validate-config` was not re-run for this step because it reads `config/`, and this step touched nothing
under `config/` — only `openspec/`. That is the same reasoning `tasks.md` V.4 records for the record passes,
inverted: those passes ran it precisely because they did touch `config/`.

### 7.5 Review gate — an unverified precondition, disclosed

The archive skill requires structured status carrying `reviewGate.result: allow`, or
`reviewGate.delivery: disabled/unmanaged` with the kill switch off. **No structured status with a
`reviewGate` field was supplied to this executor, and with no shell it could not query
`gentle-ai review mode status` or read a receipt.** The merge and this report were produced on the
orchestrator's explicit instruction. The precondition is recorded as **unverified**, not as satisfied. If a
terminal receipt governs this change, it should be linked from this report before the archive is committed.

---

## 8. What the next milestone inherits

- **Fifteen baseline capabilities**, 138 requirements and 281 scenarios, merged and consistent.
- **A working publish loop**, closed at all six links and alerting on each, bounded by a configured
  30-minute publish-latency budget.
- **Six frozen permalinks** and a redirect mechanism that is proven and, by design, unused.
- **A blocking accessibility and budget gate** over all six pages in both themes, plus a no-JS baseline and
  in-browser 44 px measurement.
- **An acknowledgement registry** whose authority is a human signature, with "nobody has signed this" as an
  expressible state.
- **A design system with no prefabricated kit**, dual authored token sets, and contrast measured rather than
  asserted.

Must be addressed before Fase 2 closes:

1. **Onboard a second maintainer.** Blocks four-eyes enforcement, `enforce_admins`, and the CODEOWNERS
   placeholder — and now guards a human signature (§6.2).
2. **Confirm the seven unconfirmed editorial dates.** They are correctly held back and operator-visible;
   the backlog does not shrink on its own.
3. **Close SUGGESTION-60** — the un-retire branch is the only route by which a confirmed event reaches a
   reader, and the digest cannot detect the change (§6.1).
4. **Correct WARNING-58's doc comment** — one line, in `app/`.
5. **Decide what a null INE `Valor` means.** The adapter refuses it by design and its message names the
   decision that has to be taken; nothing is broken until INE publishes one.

---

## 9. Archive completeness

- [x] All 13 delta specs merged into `openspec/specs/`
- [x] MODIFIED requirements replaced their baseline text in place (§5.1)
- [x] `(Previously: …)` blocks excluded from the baseline, preserved in `delta-specs/`, decision argued (§5.2)
- [x] Merged requirement and scenario counts verified by reading back and counting (§5)
- [x] Tasks artifact inspected in full: 447/447, zero unchecked (§7.3)
- [x] No CRITICAL findings in `verify-report`
- [x] Archive report written with all Engram observation IDs (§2)
- [x] Open items carried forward (§6)
- [ ] **Change folder moved to `openspec/changes/archive/2026-08-05-phase-1-indicator-page/`** — NOT DONE.
      This executor had no shell and could not move or delete files. See below.
- [ ] Review gate evidenced (§7.5)

### 9.1 The move that remains

Matching the shape `openspec/changes/archive/2026-07-29-phase-0-data-foundations/` established — a flat
folder of artifacts with the change's `specs/` renamed to `delta-specs/` — the remaining step is:

```sh
cd <repo-root>
git mv openspec/changes/phase-1-indicator-page \
       openspec/changes/archive/2026-08-05-phase-1-indicator-page
git mv openspec/changes/archive/2026-08-05-phase-1-indicator-page/specs \
       openspec/changes/archive/2026-08-05-phase-1-indicator-page/delta-specs
```

This report and `INDEX.md` were written **into the change folder** precisely so that one move carries them
along and no path in either file needs editing afterwards. Every path this report gives under
`openspec/changes/archive/2026-08-05-phase-1-indicator-page/` is written for the post-move state.

The artifacts were deliberately **not** hand-copied file by file. `design.md`, `tasks.md` and
`apply-progress.md` run to thousands of lines; reproducing them through a write tool risks silent corruption
of the audit trail, which is a worse outcome than an unfinished move — and this change's own record is a
long argument against exactly that trade.

---

**Archived by**: `sdd-archive` executor
**Archive date**: 2026-08-05 (ISO)
**Mode**: hybrid (Engram + OpenSpec)
**Skill resolution**: paths-injected

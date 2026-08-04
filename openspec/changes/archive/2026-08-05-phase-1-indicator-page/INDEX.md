# Archive Index — phase-1-indicator-page

**Archived**: 2026-08-05
**Status**: COMPLETE — PRD Fase 1 milestones 1.1 and 1.2 implemented, verified (pass 10), specs merged.
**Verification**: PASS WITH WARNINGS — 447/447 tasks, 79/79 requirements, 176/176 scenarios, 0 CRITICAL,
0 blockers.

## Change Artifacts

### Primary deliverable

**archive-report.md** — the terminal record of the change at close. Read this first. It documents:

- Final state at `2746dd8`, and the three points where it deliberately differs from `verify-report`
- **What ten verification passes kept finding** — the pipeline/publish seam (seven of ten passes), the
  check-where-the-failure-cannot-occur class (six instances), six record-staleness incidents, and one
  integrity failure no automated check could have caught
- How the 13 delta specs were merged, how the six MODIFIED requirements were handled, and the argued
  decision on the `(Previously: …)` blocks
- Twenty-one carried-forward open items, with WARNING-58, SUGGESTION-60 and the four-eyes governance gap
  named individually
- Verification at close, stating plainly what was and was not re-run
- The one archive step that remains (§9.1)

### delta-specs/ — thirteen capability deltas as written

Five new capabilities:

- `design-system/spec.md`
- `series-transformations/spec.md`
- `web-accessibility-gates/spec.md`
- `publishing-export/spec.md`
- `indicator-page/spec.md`

Eight amendments to existing capabilities:

- `platform-runtime/spec.md` (1 MODIFIED)
- `data-model-vintages/spec.md` (4 ADDED)
- `editorial-config/spec.md` (3 ADDED)
- `data-validation/spec.md` (1 ADDED, 1 MODIFIED)
- `source-ingestion-ine/spec.md` (4 ADDED, 2 MODIFIED)
- `source-ingestion-eurostat/spec.md` (3 ADDED)
- `source-attribution-licensing/spec.md` (1 MODIFIED)
- `pipeline-operations/spec.md` (3 ADDED, 1 MODIFIED)

**These files hold the seven `(Previously: …)` blocks, which were deliberately not carried into the merged
baseline.** They are the only place the superseded requirement text and the reasoning behind each amendment
survive. `archive-report.md` §5.2 argues that decision.

### Supporting artifacts

- `proposal.md` — intent, scope, out-of-scope, settled decisions, risks
- `design.md` — technical approach, D-1…D-9, and the Open Questions frozen at archive (some still open by
  design; each falsified item is ticked and superseded rather than rewritten)
- `tasks.md` — 447 tasks across 39 slices, 447 complete, zero unchecked
- `apply-progress.md` — per-slice implementation record with TDD Cycle Evidence tables
- `verify-report.md` — verification pass 10, admitted by `gentle-ai sdd-verify-validate`, sha256 `6692a55b…`
- `exploration.md` — external API reality check and blocking product decisions

## Main specs (merged)

All delta specs are merged into the baseline. Fifteen capabilities, **138 requirements, 281 scenarios**
(counted, not asserted — `archive-report.md` §5):

- `openspec/specs/data-model-vintages/spec.md` — 11 / 19
- `openspec/specs/data-validation/spec.md` — 9 / 29
- `openspec/specs/design-system/spec.md` — 9 / 14
- `openspec/specs/editorial-config/spec.md` — 12 / 24
- `openspec/specs/indicator-page/spec.md` — 16 / 39
- `openspec/specs/pipeline-operations/spec.md` — 7 / 17
- `openspec/specs/platform-runtime/spec.md` — 10 / 15
- `openspec/specs/publishing-export/spec.md` — 12 / 21
- `openspec/specs/raw-file-archive/spec.md` — 4 / 9 (untouched by this change)
- `openspec/specs/series-transformations/spec.md` — 8 / 14
- `openspec/specs/source-attribution-licensing/spec.md` — 4 / 8
- `openspec/specs/source-ingestion-eurostat/spec.md` — 10 / 17
- `openspec/specs/source-ingestion-ine/spec.md` — 10 / 23
- `openspec/specs/source-ingestion-xlsx/spec.md` — 6 / 16 (untouched by this change)
- `openspec/specs/web-accessibility-gates/spec.md` — 10 / 16

## Engram observation IDs

| Artifact | ID |
|---|---|
| Exploration | #4721 |
| Proposal | #4722 |
| Design | #4723 |
| Spec | #4724 |
| Tasks | #4725 |
| Apply progress | #4726 |
| Verify report (pass 10) | #4771 |
| Archive report | `sdd/phase-1-indicator-page/archive-report` |

Per-slice apply observations: #4732, #4735, #4738, #4741, #4744, #4745, #4764, #4766, #4770.
Remediation and record-correction observations: #4780, #4826.

## Archive completeness

- [x] All delta specs merged into main specs
- [x] MODIFIED requirements replaced baseline text in place
- [x] Merged counts verified by reading back and counting
- [x] Tasks artifact inspected in full — zero unchecked
- [x] Archive report written with all observation IDs
- [ ] Change folder moved to `openspec/changes/archive/2026-08-05-phase-1-indicator-page/` — see
      `archive-report.md` §9.1 for the exact two `git mv` commands
- [ ] Review gate evidenced — see `archive-report.md` §7.5

## For the next milestone

Inherited: fifteen baseline capabilities, a closed publish loop with alerting at every link, six frozen
permalinks, blocking accessibility and budget gates, an acknowledgement registry whose authority is a human
signature, and a design system with measured contrast in both themes.

Must be addressed:

1. Onboard a second maintainer — blocks four-eyes enforcement, `enforce_admins` and the CODEOWNERS
   placeholder, and now guards a human signature
2. Confirm the seven unconfirmed editorial dates
3. Close SUGGESTION-60 — the un-retire branch for events is untested and `eventDigest` cannot detect the
   change that makes it fire
4. Correct WARNING-58's doc comment
5. Decide what a null INE `Valor` means

---

**Archived by**: `sdd-archive` executor
**Archive date**: 2026-08-05 (ISO)
**Mode**: hybrid (Engram + OpenSpec)

# Archive Index — phase-0-data-foundations

**Archived**: 2026-07-29  
**Status**: COMPLETE — Fase 0 fully implemented, verified, and archived

## Change Artifacts

This archive contains the complete record of the `phase-0-data-foundations` SDD change:

### Primary Deliverables

1. **archive-report.md** — Terminal record of the change at close. Documents:
   - Executive summary and final state
   - Verification arc (three passes)
   - C8 closure and orchestrator verification
   - Eight known-open items with disclosure
   - The eight-instance pattern and detection heuristics
   - Remediation batches and closure timeline
   - Coverage and quality metrics
   - What Fase 1 inherits

2. **specs/** subdirectory — Ten capability specifications (65 requirements, 114 scenarios)
   - platform-runtime/spec.md
   - data-model-vintages/spec.md
   - editorial-config/spec.md
   - data-validation/spec.md
   - raw-file-archive/spec.md
   - source-ingestion-ine/spec.md
   - source-ingestion-eurostat/spec.md
   - source-ingestion-xlsx/spec.md
   - source-attribution-licensing/spec.md
   - pipeline-operations/spec.md

### Supporting Artifacts (Source of Truth)

For the complete audit trail, refer to the original location:
- **openspec/changes/phase-0-data-foundations/proposal.md** — Intent, scope, settled decisions, approach, risks
- **openspec/changes/phase-0-data-foundations/design.md** — Technical approach, ADRs, schema, Go layout, validation rules, deployment
- **openspec/changes/phase-0-data-foundations/tasks.md** — 144 tasks across 10 slices, forecast accuracy, measured actuals
- **openspec/changes/phase-0-data-foundations/verify-report.md** — Full verification report (pass 3), all findings, test commands, remediation batches
- **openspec/changes/phase-0-data-foundations/apply-progress.md** — Implementation progress snapshot (per-PR)
- **openspec/changes/phase-0-data-foundations/exploration.md** — External API reality check, blocking product decisions

### Main Specs (Merged)

All delta specs have been merged into the main specification baseline:
- openspec/specs/platform-runtime/spec.md
- openspec/specs/data-model-vintages/spec.md
- openspec/specs/editorial-config/spec.md
- openspec/specs/data-validation/spec.md
- openspec/specs/raw-file-archive/spec.md
- openspec/specs/source-ingestion-ine/spec.md
- openspec/specs/source-ingestion-eurostat/spec.md
- openspec/specs/source-ingestion-xlsx/spec.md
- openspec/specs/source-attribution-licensing/spec.md
- openspec/specs/pipeline-operations/spec.md

## Engram Observation IDs (Traceability)

All artifacts persist to Engram with observation IDs for traceability and historical reference:

| Artifact | Observation ID | Created | Revisions |
|---|---|---|---|
| Proposal | #4691 | 2026-07-28 17:04:59 | 1 |
| Spec | #4694 | 2026-07-28 17:15:01 | 1 |
| Design | #4695 | 2026-07-28 17:15:33 | 1 |
| Tasks | #4698 | 2026-07-28 17:23:16 | 20 |
| Verify Report | #4712 | 2026-07-28 23:53:58 | 3 |
| Archive Report | (generated at save) | 2026-07-29 | — |

## Final State Summary

- **Deliverables**: ~18,000 lines of Go (tests and production code combined)
- **Tasks**: 144/144 COMPLETE across 10 slices (PR 1–9, with internal splits)
- **Specifications**: 65 requirements, 114 scenarios, all baseline specs merged
- **Test Coverage**: 77.0% (package total); composition root extracted and mutation-tested
- **Verification**: Pass 3 confirmed, C8 closed, orchestrator green (21/21 packages, 0 failures)

## Known-Open Items (Disclosed)

Eight items are documented in the archive report. All have:
1. Exact location (file/line or config entry)
2. Why they're open (deferred, infrastructure missing, design choice, etc.)
3. Impact on Fase 0 (none — all safe)
4. Impact on Fase 1 (must fix before that phase launches)
5. Status (acceptable, known blocker, etc.)

See archive-report.md § "Known-Open Items and Disclosure" for full details.

## Archive Completeness

- [x] All delta specs merged to main specs
- [x] Archive report written with all observation IDs
- [x] Change folder moved to archive with date prefix
- [x] All artifacts documented and indexed
- [x] Engram observations linked for traceability

## For Fase 1

Fase 1 starts with:
- A complete, tested, traceable data pipeline
- 10 capabilities with full specifications
- Versioned observation model with immutability guarantees
- Editorial config (breaks, events, governments) reconciled and enforced
- Validation (six rules, publish gate) with structured logging and alerts
- All source adapters (INE, Eurostat, Social Security) with failure-mode coverage

Fase 1 MUST address:
1. Onboard second maintainer (config governance blocker)
2. Resolve 7 unconfirmed editorial dates (events UI blocker)
3. Surface INE P/D distinction (Verificado module requirement)
4. Implement indicator-page capability (public launch requirement)
5. Migrate `/data-derived` CSV from deferred to shipped (P5 commitment once Fase 1 launches)

---

**Archived by**: sdd-archive executor  
**Archive Date**: 2026-07-29 (ISO format)  
**Skill Resolution**: paths-injected  
**Mode**: hybrid (Engram + openspec)

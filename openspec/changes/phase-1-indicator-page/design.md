# Design — phase-1-indicator-page

Change: `phase-1-indicator-page` · Project: **ConContexto** · Store: hybrid.
Inputs: proposal.md, exploration.md (§1–§6), ADR-6/ADR-7/ADR-8 (all accepted — this document designs
*inside* them), Fase 0 design (`openspec/changes/archive/2026-07-29-phase-0-data-foundations/design.md`),
the live code under `app/internal/` and `web/`. Settled decisions D1–D6 are not re-opened.

## Technical Approach

Two ecosystems meet through one versioned artifact. The Go pipeline gains its reserved `publishing`
layer: after a successful ingest it reads current vintages, breaks, events and freshness through the
existing postgres read functions, writes a validated artifact into the statically served
`/data-derived/`, and dispatches a CI rebuild. Astro fetches that same public artifact, re-validates it,
and builds six `/indicador/{slug}` pages: static components plus one Svelte chart island, both drawing
geometry from a single shared TypeScript module so the build-time SVG and the client-side redraw can
never diverge. The golden rule stays structural: `httpserver` is untouched and the import-graph guard
keeps passing — publishing writes files at ingest time; the request path serves bytes.

Two prerequisite defect fixes land first, because everything downstream (per-capita, the P/D disclosure)
depends on them: periodicity judged over the whole payload with mixed-cadence config support, and
`Rule2Continuity` auditing the historical interior. Both may turn currently-green ingests red; that is
the designed outcome, not a regression.

## Architecture Decisions

### D-1 — Export artifact: public, per-series files, manifest with digests, validated on both sides

**Choice**: the artifact IS `/data-derived/` (P5 discharged by construction, per ADR-7):

```
public/data-derived/
  manifest.json          # schema_version (int), generated_at, exporter build info,
                         # series index, sha256 per file, latest ingestion summary
  series/{slug}.json     # one per published series (see schema sketch below)
  events.json            # confirmed-date events only, grouped by event_group
  sources.json           # per-source freshness, licence/attribution text
  csv/{slug}.csv         # same data, CSV projection (one writer, two formats)
```

Validation is symmetric and mandatory:
- **Write side (Go)**: `publishing.ValidateArtifact` runs over the in-memory model before a byte is
  written — statuses ∈ {P, D}, periods canonical and gap-audited against the series cadence, every
  break/event reference resolvable, digests computed last. A validation failure aborts the export and
  alerts; the previous artifact stays in place (atomic directory swap: write to `data-derived.tmp`,
  rename).
- **Read side (Astro)**: a content loader (`web/src/lib/export/loader.ts`) fetches `EXPORT_URL` (CI:
  the live site) or reads `EXPORT_DIR` (dev/tests: a Go-generated golden fixture), verifies every
  sha256 against the manifest, parses with Zod schemas, and requires `schema_version` to equal the one
  version the loader supports — **exact integer equality, not ≥**. Any failure exits the build non-zero;
  the previously deployed site remains live.
- **Anti-drift device**: Go exports a golden artifact fixture (`web/test/fixtures/export/`, regenerated
  by `go run ./app/cmd/concontexto export --fixture` via the repo's golden-update path) and the web unit
  suite validates it with the same Zod schemas the real build uses. CI additionally runs one end-to-end
  job: ingest fixtures → export → Astro build. This is the test that kills the "built, tested, never
  connected" shape.
- **Version-bump protocol**: a schema bump changes Go writer and Zod loader in the same PR; `export`
  re-runs on pipeline boot when the on-disk manifest's version differs from the binary's own, so the
  artifact self-heals to the deployed binary without request-time work.

**Alternatives considered**: (a) one monolithic JSON — rejected: 26 series in 1.3 make diffs and
partial reads worse, and per-series files map 1:1 onto Astro's content collections; (b) a shared JSON
Schema file consumed by both ecosystems — rejected: cross-field invariants (cadence audit, break
resolution) exceed JSON Schema, and the golden-fixture handshake proves compatibility more directly
than a schema file either side could silently mis-read; (c) private artifact transport (CI secret
download, repo commit) — rejected: publishing it *is* P5's obligation, and "the site is reproducible
from a public artifact" is ADR-7's stated promise.

**Vintages**: the artifact carries the current vintage only (vintage UI is Fase 2+). Each series doc
records `vintage: {ingestion_run_id, extracted_at}`; each observation carries its `version` int, so a
revision is visible without shipping history. Current `W` tombstones are excluded from `points` and
listed under `withdrawn` — the chart shows the gap honestly.

### D-2 — `publishing` package: composed at the command layer, never inside `IngestSeries`

**Choice**: `app/internal/publishing/` exposes `Export(ctx, deps, asOf)` (pure read → model →
validate → write) and `Publish(ctx, deps, asOf)` (`Export` + dispatch + latency stamp). The **command
layer** (`app/cmd/concontexto/ingest_cmd.go`) calls `Publish` once after an ingest cycle in which at
least one series published (`GateApplyResult.Published` non-empty); `IngestSeries` itself is not
modified for this — it already returns everything the caller needs. A standalone `concontexto export`
subcommand runs the same path for recovery, boot-time self-heal, and fixture generation.

Reads go through existing functions — `postgres.ListCurrentObservations`, `SeriesFreshness` (its first
real consumer; archive-report W16 stands corrected), `ResolveActiveBreaksForSeries` — plus two new read
functions this change adds: `postgres.ListActiveEvents` and `postgres.ListPublishedSeries` (series
metadata joined with latest succeeded run). The breaks/events *write* path (reconcile) is untouched.

**Rebuild trigger**: `publishing.Dispatcher` port; adapter `adapters/github/` POSTs a
`repository_dispatch` (`event_type: rebuild`, payload: `generated_at`, manifest digest) with a
fine-grained token from env. Dispatch failure is an alert on the Fase 0 alerting sink, never a retry
loop and never an ingest failure — data is safe; only publication latency suffers.

**Latency budget**: **30 minutes** from ingest success to deployed page (well inside §19.3's 24 h;
covers CI queue + build + Portainer pull). Instrumented without touching the request path: the deployed
site serves its own `manifest.json`, so a scheduler watchdog compares the live manifest's
`generated_at` against the newest local export and alerts on budget breach. Per the orchestrator
decision, **this state is ops-only**: the reader-facing amber semaphore means exactly "the source has
not published the expected period yet" (from `sources.json` freshness at build time). A statically
built page cannot honestly report a fact postdating its own generation. *This narrows the proposal's
"three-state semaphore" exit criterion to two reader-visible states — the spec must adopt this.*

**Alternatives considered**: calling `Publish` inside `IngestSeries` — rejected: one export per series
fetch instead of one per cycle, and it would drag a GitHub adapter into the application layer's
blast radius. Golden rule impact: none — `httpserver` gains no imports; `publishing` may import
`adapters/postgres` (it is a build/ingest-time layer, exactly like `ingestion`).

#### D-2 resolution (verify-report CRITICAL-16, 2026-07-30) — the export gate is widened, and `publishing-export`'s "no new artifact" clause is narrowed

**The decision.** The export gate is no longer "at least one series published". It is "this cycle recorded
a terminal run outcome the artifact must now reflect", which is true when a run published an observation
**or** when a run was blocked by the publish gate. `publishing-export`'s failed-run clause ("THEN no new
artifact is exported / AND no rebuild is dispatched") is **narrowed** accordingly — see that spec's
amended "A failed ingestion exports the failure state, never the suspect datum" scenario, which preserves
the original wording as history.

**Why that clause is the half that yields.** Two requirements in this same change were, read literally,
mutually unsatisfiable. `indicator-page`'s "The three page states of PRD §6.1.3" requires that a series
whose latest run failed validation render a banner naming the date of the last correct update; a reader's
only route to that banner is a newly exported artifact and a rebuild; `publishing-export` forbade exactly
that on the only event that raises the banner. The shipped code implemented the `publishing-export` side,
so the banner could only ever appear as a side effect of some *unrelated* series succeeding in a later
cycle — nowhere designed, nowhere tested, and with latency bounded by an unrelated schedule.

The requirement's *intent* is that **a suspect datum must never be published**, not that the pipeline must
go silent. Three facts settle which reading survives:

1. `publishing-export`'s own other clauses already assume the export runs: "A series whose latest run
   failed MUST still appear, carrying its last valid data", and "AND the series carries the state that
   drives PRD §6.1.3's validation banner". Neither is reachable under the broad reading. The spec was
   internally inconsistent, not merely under-specified.
2. The suppression was **redundant** as a safety measure. `postgres.ApplyGate` writes zero observations on
   a `GateBlock` verdict, and the export reads only currently published observations, so a suspect value
   is structurally absent from any artifact **whether or not the export runs**. Withholding the export
   protected nothing.
3. The suppression was **harmful** as a disclosure measure. An export after a validation failure publishes
   exactly two things: the last valid data, and the honest fact that the latest run failed. Suppressing it
   serves a stale artifact that silently claims everything is fine — the opposite of what PRD §6.1.3
   exists for, and a breach of P7 (errors are documented, never erased) rather than a defence of P4.

In one line: **the suspect datum must not be published; the failure state must be.**

**The rebuild dispatch is narrowed with the export, deliberately.** `publishing.Publish` is
`Export` + dispatch as one unit, and an artifact that is never built is invisible to a reader — which is
the entire defect CRITICAL-16 names. Withholding only the dispatch would have re-created the bug one
layer down. The original clause's third promise still holds unchanged: the currently published site
continues to be served, and it never serves the suspect datum.

**What the gate does per outcome kind** (`runIngest`, `app/cmd/concontexto/ingest_cmd.go`; the three
failing kinds are deliberately not collapsed into one another):

| Cycle outcome | Exports + dispatches | Why |
|---|---|---|
| At least one series published | Yes | The artifact must carry the new datum. Unchanged behaviour. |
| A run was blocked by the publish gate (validation failure, decode failure, unclassified source status — all three converge on one `ApplyGate` Block and one `outcome='validation-failed'` row) | **Yes (new)** | `postgres.SeriesValidationOutcome` now resolves the series to `PageStateValidationFailure`; the artifact must carry the failure, with the last valid datum still standing as the latest value. |
| Fetch failure with no payload | No | `IngestSeries` records a `download_attempt` with no resulting hash, never creates an `ingestion_run` row, and returns the **zero** `Result`. Nothing new is knowable; re-exporting would rewrite `generated_at` and dispatch a rebuild that changes nothing a reader can see. |
| Config resolution failure (unknown source, no active `source_ref`, unbuildable client, bad cadence segments) | No | Same reason, one layer earlier: the series never reached its source. |
| `nothing-new` | No | A documented value of `ingestion_run.outcome` that no Go code writes yet (`postgres.RunOutcome`). When it lands it produces neither a published observation nor a Block verdict, so it falls in this branch by the same rule rather than needing a new one. |

**Implementation note worth flagging**, because it is the subtle part: `runIngest` now reads
`result.Outcome` **before** handling `IngestSeries`'s error. A returned error does not mean nothing was
recorded — the decode-failure and unclassified-status paths return a real Block result *and* an error,
because both already went through the publish gate. A fetch failure returns the zero `Result`. Reading
`result.Outcome` is therefore the exact discriminator between "a verdict was recorded" and "nothing
happened", with no error inspection at all.

**Deliberately out of scope.** A re-fetch whose payload is byte-identical still counts as published today
(`ApplyGate` records every candidate it hands the writer, whether or not `WriteRevision` changed a row),
so it still exports. Narrowing that would change what a *successful* cycle does and needs its own decision
about what `generated_at` means; it is untouched here and disclosed rather than silently folded in.

**Proof.** `TestRunIngest_TheExportGateReflectsEachOutcomeKind`
(`app/cmd/concontexto/ingest_export_gate_test.go`) drives a real validation failure through a real
Postgres, a real `IngestSeries` and the real production export trigger, then reads the resulting
`pageState` **off disk** — bytes on disk being what the Astro build consumes and therefore the only
artifact a reader can be shown. Its three subtests cover the validation failure, a fetch failure and an
unresolvable source ref. This is the joining test WARNING-17 observed was missing: the publish gate, the
page-state read and the artifact writer were each already covered, and the defect lived entirely in the
seam.

**Related closure (verify-report WARNING-17).** `publishing.Deps.SeriesValidationOutcome` is now
**required**: `Export` refuses an unbound port instead of treating nil as "no failure known". It shipped
optional, so the single line binding it in `buildExportDeps` was silently load-bearing — deleting it would
have reverted CRITICAL-4 in full (every page reporting "fresh") with the whole Go suite green. "I cannot
know whether this series' latest run failed" is not the same claim as "it did not fail", and only the
second is what a `fresh` page state tells a reader; refusing to export makes the regression *impossible*
rather than merely detectable. `TestBuildExportDeps_BindsEveryPortIncludingTheValidationOutcome`
(`app/cmd/concontexto/export_cmd_test.go`) additionally names the defect at its source with no database,
so the red is immediate rather than three layers away.

### D-3 — `observation.status` stays P/D/W; nullable `source_status` preserves the verbatim token

**Choice**: additive migration `0003_observation_source_status`:

```sql
ALTER TABLE observation ADD COLUMN source_status text;  -- verbatim source token, NULL = none recorded
-- down: ALTER TABLE observation DROP COLUMN source_status;
```

`indicators.Observation` gains `Status ObservationStatus` and `SourceStatus string` (rules ignore both;
the purity guard is unaffected). `postgres.ObservationInput/Observation` and `WriteRevision`'s
"value **or status** differs" comparison extend to `source_status`. `ingest.go` maps from the decoded
observation instead of hardcoding `StatusDefinitive`.

Mapping, fail-closed:

| Source | Token | `status` | `source_status` |
|---|---|---|---|
| INE `T3_TipoDato` | `"Definitivo"` | D | `Definitivo` |
| INE | `"Provisional"` | P | `Provisional` |
| INE | anything else | — | **fails `sourceerr.SchemaDrift`** (INE publishes no enum; mirror `detectPeriodicity`) |
| Eurostat flag | *absent* | D | NULL (absence means definitive — there is no "definitive" flag) |
| Eurostat | `p` | P | `p` |
| Eurostat | `b`, `d` | D (not a status) | `b`/`d`, **and** routed as a break signal (below) |
| Eurostat | `e f u c n :` or unknown | — | **fails `SchemaDrift`** until the spec allowlists a token with a decided projection |

`b`/`d` routing: the decoder emits `SourceResult.BreakSignals []{Period, Flag}`; ingest logs them and
alerts when no active `series_break` covers that period. It never writes `series_break` —
`rupturas.yaml` remains the only writer (editorial follow-up, not automation). The phantom
`wireObservation.Secreto` field is deleted.

**Implementation clarification (`sdd-apply` slice 2a, INE half only — Eurostat's row of the mapping
table above remains a slice-2b commitment, not yet built).** This section's prose reads "`ingest.go`
maps from the decoded observation"; the actual fail-closed classification for INE lives one layer
lower, in `ine.Client.Decode` (`classifyTipoDato`, `envelope.go`), NOT literally inside
`ingestion/ingest.go`. Two reasons, both load-bearing: (1) `envelope.go`'s `decodeAndNormalize` /
`DecodeSeries` — the function this package's own periodicity/cadence unit tests call directly
(`cadence_test.go`, `periodicity_test.go`, none of which supply a status token) — carries
`T3_TipoDato` through **verbatim and unvalidated** as `Observation.SourceStatus`; classification only
happens one call later, in the `indicators.SourceClient`-satisfying `Client.Decode` wrapper, so those
existing unit tests keep exercising cadence/periodicity in isolation without also needing a status
token; (2) `IngestSeries` stays genuinely source-agnostic: by the time an INE observation reaches
`ingest.go`'s candidate-building loop, `Client.Decode` has ALREADY classified it (unrecognised/missing
token already aborted the whole decode as `sourceerr.SchemaDrift`, reusing the exact same
decode-failure path a periodicity mismatch already takes — no new code in `ingest.go`'s error handling).
`ingest.go` itself only does the trivial remaining step: convert the already-classified
`indicators.ObservationStatus` into `postgres.ObservationStatus`
(`mapObservationStatus`/`sourceStatusPtr`), with one disclosed fallback — a zero-value (never
classified) `Status` defaults to `StatusDefinitive`, which can only ever be reached by
`adapters/eurostat` today (its own status decoding is slice 2b, untouched this pass), preserving Fase
0's pre-existing, unchanged behaviour for that source rather than silently rejecting every Eurostat
observation this slice never touched. Every spec scenario (definite/provisional mapping, unrecognised
token, missing token, "No phantom field survives") is still satisfied end-to-end — this note documents
WHERE, not a behavioural deviation.

**Implementation clarification (`sdd-apply` slice 2b, Eurostat half — closes the sentence above).**
Eurostat's classification lives directly inside `eurostat.Decode` (`envelope.go`'s package-level
function, also `Client.Decode`'s entire body — this adapter never split a separate
`decodeAndNormalize`/`Client.Decode` layering the way INE's slice 2a did), not in a separate wrapper.
This is NOT a structural inconsistency with INE's split: INE's own two-layer shape exists solely
because `ine/envelope.go`'s `decodeAndNormalize` is called directly by that package's own
periodicity/cadence unit tests, none of which supply a status token — classifying inside that function
would have broken them. Eurostat's package-level `Decode` carries no equivalent test-isolation
constraint (an absent `status` map is itself a legitimate, spec-required case — "Absence of a flag
means definitive" — not a rejection a test would need to route around), so classifying directly inside
it is the same functional point INE's `Client.Decode` occupies, reached through one function instead
of two. Both adapters answer "is this status token known?" the same fail-closed way for `p`/`b`/`d` vs.
anything else (`classifyEurostatFlag`, mirroring `classifyTipoDato`'s unrecognised-token handling) —
the CONTRACT (design D-3's mapping table) is identical; only the adapter-internal call depth differs,
for a disclosed, adapter-specific reason. `SourceResult.BreakSignals` (new field, `indicators/ports.go`)
carries `b`/`d` flags to `IngestSeries`, which logs and alerts (never blocks, never writes
`series_break`) when no already-active break covers the flagged period.

**`ingest.go`'s disclosed compatibility shim (slice 2a) is now closed (slice 2b).** Both governed
adapters (INE, Eurostat) classify every observation's status before `IngestSeries` ever sees it, so
`mapObservationStatus`'s `"" → StatusDefinitive` default branch is provably unreachable for either —
it is REMOVED, not merely guarded, because a new earlier check (`firstUnclassifiedStatus`) now fails
the whole run closed (the same synthetic-Block-finding path a decode failure already takes) if any
`SourceClient` ever hands `IngestSeries` an unclassified observation. Discovered, disclosed side
effect: `adapters/xlsx` does not classify `Status` at all today (no row in D-3's mapping table covers
it), so a real, successfully-decoded XLSX ingest cycle (e.g. `afiliacion-ss`) would now BLOCK on this
guard rather than silently publishing Definitive as it does today — no currently-checked-in XLSX test
exercises a successful `IngestSeries` decode end-to-end (the one existing XLSX/`IngestSeries` test,
`TestIngestSeries_XLSXPartialSuccessWithinOneWorkbookWritesNothingAndPreservesThePublishedDatum`, fails
at decode, before this guard is ever reached), so this is NOT caught by `go test ./...`, but it is a
real production-facing change in scope beyond "Eurostat status and break-flag decoding" and is
recorded here as an open item, not silently absorbed. Giving XLSX its own explicit status
classification (trivially, always Definitive — administrative register data) is the correct fix and is
deliberately NOT made in this slice (out of file scope); it is required before `afiliacion-ss`'s next
successful ingest cycle in production.

**Implementation clarification (`sdd-apply` slice 2c, corrective — closes the Open Question slice 2b
raised).** `adapters/xlsx` gets its own, third row in D-3's mapping table: the Social Security
"Afiliación media mensual por regímenes (Total Sistema)" workbook publishes no provisional/definitive
marker of any kind — verified against the real, sha256-matched fixture (task 8.1, re-confirmed this
slice): a single flat sheet (`Hoja1`), zero formula cells, columns B–M as régimen components, N as the
system total, O as a memo item — no status column, no status concept anywhere in the file.
`xlsx.Decode` (`app/internal/adapters/xlsx/decode.go`) therefore sets every parsed observation's
`Status` to `indicators.ObservationStatusDefinitive` directly at construction, leaving `SourceStatus`
as the domain's empty-string "none recorded" convention (`ingest.go`'s `sourceStatusPtr` already maps
that to a NULL `source_status` column — migration 0003's own comment: NULL means "none recorded", never
"unknown/invalid", and that is exactly true here since there is no token to record). This is documented
in `decode.go` as a positive statement about the source, not a silently-reinstated default — the
distinction D-3's own governing principle turns on ("coercing unknown tokens to D is exactly the silent
lie this change exists to remove" describes a *source that has a status concept we failed to read*;
XLSX has no status concept to read at all, which is a different, honestly-stated fact). The three
governed sources now contrast explicitly:

| Source | Status concept | Unrecognised/missing token |
|---|---|---|
| INE `T3_TipoDato` | Present, must be recognised | Fails the whole decode closed (`sourceerr.SchemaDrift`) |
| Eurostat flag | Defined vocabulary; absence means definitive | Absence is valid (D, null `source_status`); anything outside `p`/`b`/`d` fails closed |
| XLSX / Social Security | None — the source publishes no status information | N/A — always Definitive, `source_status` always NULL |

`firstUnclassifiedStatus` (`ingest.go`, slice 2b) is unchanged by this slice — its job (fail the whole
run closed if any `SourceClient` ever hands `IngestSeries` a still-unclassified observation) stays
exactly as slice 2b built it; the fix belongs in the adapter that owns the knowledge of its own source,
not in the shared candidate-building path. Proof:
`TestIngestSeries_XLSXSuccessfulDecodePublishesDefinitiveWithNullSourceStatus`
(`app/internal/ingestion/xlsx_status_test.go`) — a real Postgres, the real writer, a workbook with zero
malformed rows, asserting the run publishes (not blocks) and every published row carries `status='D'`,
`source_status IS NULL`.

**Backfill**: none bespoke. The next scheduled ingest re-decodes and `WriteRevision` appends version-2
rows where status now differs; the run log records the cause. The bug-caused revisions stay publicly
visible in the vintage history — P7: disclose, never erase.

**Alternatives considered**: widening the status enum with source tokens — rejected: §6.1.1's UI
contract is binary, Eurostat's flags are not a P/D vocabulary, and every consumer would need to learn
each source's dialect. Coercing unknown tokens to D — rejected: that is exactly the silent lie this
change exists to remove.

### D-4 — Periodicity over the full payload; cadence segments in config; Rule 2 audits the interior

**D3 resolution (orchestrator adjudication, settled during `sdd-apply` slice 1 — supersedes this
section's original "Alternatives considered" rejection of a semiannual cadence value below).** Spec and
this design disagreed on how a cadence segment records its cadence: the spec calls the historical
population span "semiannual" and requires each segment to carry its own cadence; this design's original
draft kept every segment at `frequency: Q` with a `present: [Q1, Q3]` filter and explicitly rejected a
distinct semiannual value. **Decision: the spec wins on naming, the design wins on mechanism.** A cadence
segment DECLARES its own cadence via a `Cadence` field (`indicators.CadenceSegment.Cadence`,
`config.CadenceSegmentConfig.Cadence` — a plain, source-descriptive string: `"semiannual"` for the
historical population span, never an `indicators.Frequency`), while storage and period arithmetic stay
on the series' base quarterly grid, filtered by `Present`. **No new `indicators.Frequency` variant is
added** — no semiannual arithmetic reaches `Period` (`stepsPerYear`, `Next`, `Previous`, the label
parser). The segment's `Cadence` is a source-descriptive field, not a domain frequency; naming it
honestly (rather than only via an implicit `present: [Q1, Q3]` filter a later reader must reverse-
engineer) is what "each segment MUST be audited under its own cadence" (spec data-validation) requires,
and Rule 2's interior audit reads that declared cadence directly (`indicators.CadenceExpects`) instead of
reverse-engineering it.

**Choice** (three coordinated fixes):

1. **Adapter** (`ine/envelope.go`, same principle in `eurostat`): classify *every* observation's
   period, then audit the whole set of periods against the config's declared cadence via the shared
   `indicators.AssertCadence` — reused verbatim by `ine/envelope.go`, `eurostat/envelope.go` and
   `Rule2Continuity`, so the three call sites can never silently diverge. Mismatch → `SchemaDrift`, and
   the diagnostic **prints the observed segmentation** (a run-length encoding of consecutive periods
   sharing the same step size), so the editor can correct config from the failure message. AssertCadence's
   structural check is a **majority-vote over the step size** between consecutive observed periods within
   a segment's own span (not a raw density ratio): a segment genuinely declared dense should show a
   step-1 majority regardless of how many individually-tolerated isolated gaps it also has — an isolated
   gap is Rule 2's job (with its documented-gap allowlist), not this coarser structural check's.
2. **Config**: `frequency` remains the current cadence (unchanged for five of six series). A series MAY
   add `cadence_segments`, validated by `validate-config` (ordered, non-overlapping, no gap, last segment
   open-ended):

   ```yaml
   # config/series/poblacion-residente.yaml (implemented — boundary disclosed as provisional:
   # 2023-Q3 is the earliest LIVE-VERIFIED continuous-quarterly point available offline; a live
   # full-history fetch may narrow it further. Four-eyes on /config is documented but unenforced
   # — see Migration / Rollout.)
   frequency: Q
   cadence_segments:
     - { from: "1977-Q1", to: "2023-Q2", cadence: semiannual, present: [1, 3] }   # Q1/Q3 on the quarterly grid
     - { from: "2023-Q3", cadence: quarterly }
   ```

   The semiannual era is expressed as *quarterly labels with only Q1/Q3 present* (`present: [1, 3]`) plus
   its own declared `cadence: semiannual` label — no new domain `Frequency`, no new period grammar, no
   migration of decades of stored rows, because INE's own labels ("1 de enero de"/"1 de julio de") already
   normalize to Q1/Q3.
3. **`Rule2Continuity`**: audits the union of prior + incoming over the **whole span** — earliest to
   latest — against the expected period sequence derived from the cadence segments
   (`indicators.CadenceExpects`), with the documented-gap allowlist unchanged. The `!hadPrior → nil` early
   return is removed: a first run with an interior gap now fails. (`!hasIncoming` still returns nil; rule
   6 owns emptiness.) Before the per-period walk, `Rule2Continuity` also runs `indicators.AssertCadence`
   over the whole prior+incoming history, so a declared cadence that contradicts the observed history
   fails closed as one consolidated finding, independently of the adapter's own same-run check.

**Alternatives considered**: (a) a real `FrequencySemiannual` domain type — rejected: Fase 0 already
documented that as a cross-cutting domain change (`stepsPerYear`, `Next`, `Prev`, labels), and it would
force re-labelling stored history for zero reader benefit; the D3 resolution's `Cadence` string field
gets the same self-documenting benefit without this cost; (b) allowlisting ~100 missing Q2/Q4 periods
as documented gaps — rejected: hundreds of config lines asserting nothing, and the guard would still
judge cadence from `Data[0]`; (c) leaving Rule 2 forward-only — rejected: it is one of the two holes
that admitted the defect.

**Consequence, accepted**: the corrected guards may reject other live series. Per proposal Q5, only the
affected series blocks; the rejection is disclosed (P7). Rollback is always config correction, never
guard reversion. Confirmed via the `sdd-apply` slice-1 triage: only `poblacion-residente` needed
correction; the other five pinned series' checked-in fixtures (trimmed, 3 periods each) show no
regression under the corrected guards — a live full-history run against all six remains the outstanding
verification this offline session could not perform.

### D-5 — Chart: one shared geometry module feeds both the build-time SVG and the island

**Choice** (implements settled D6): all scales, path/band/tick geometry and transformations are pure
TypeScript in `web/src/lib/chart/` and `web/src/lib/transform/` (`yoy`, `perCapita`, `sliceRange` —
data in, geometry/points out, no DOM). The static Astro component `IndicatorChart.astro` calls the
module at build time and emits the default-view SVG plus break bands, data table, and generated textual
description — the complete no-JS experience. The Svelte island `ChartIsland.svelte` (`client:idle`)
progressively enhances that same markup: tooltips, keyboard navigation of points, range presets and
transformation toggles, redrawing client-side by calling the *same* module. A Vitest golden test
asserts the island's initial render equals the build-time SVG — the intra-frontend version of ADR-7's
option-C rejection (never two renderers that must agree).

**Island payload** (inline `<script type="application/json">`, ~2–4 KB gzipped for the longest
294-point series):

```ts
type SeriesPayload = {
  slug: string; unit: string; decimals: number; frequency: "M" | "Q" | "A";
  points: [period: string, value: number | null, status: "P" | "D", version: number][];
  sourceStatus: Record<string, string>;          // period → verbatim token, sparse
  breaks: { key: string; date: string; kind: string; noteMd: string }[];   // never dismissible
  annotations: { group: "governments" | "exogenous" | "milestones";        // (a),(b) OFF by default
                 id: string; name: string; dateStart: string; dateEnd: string | null }[];
  transforms: { yoy: boolean;
                perCapita: { coverage: { from: string; to: string } } | null };
  denominator?: [period: string, value: number][];  // only when perCapita !== null
};
```

**Per-capita coverage rule**: computed at export — the maximal contiguous span in which *every*
indicator period has a denominator observation at the identical period (no interpolation; §6.1.2). The
toggle renders **iff** `perCapita !== null` and applies only inside `coverage` (absent, not disabled —
"a toggle that does not apply does not exist"). Attribution rule stated in the methodology sheet,
flagged for editorial sign-off. YoY is a pure transform over the raw points; which pages carry it, and
each page's default view, are presentation config in `web/src/content/indicators/{slug}.ts` beside the
externalised Spanish strings (`web/src/i18n/es.ts`) — pipeline data and editorial presentation stay
separate (P1).

**Alternatives considered**: rendering the initial SVG with a separate Astro-only renderer — rejected
(divergence risk stated above); `client:load` hydration — rejected: `client:idle` keeps main-thread
work off first paint with no functional loss, protecting the 300 KB/§12.3 budget's spirit.

### D-6 — Tailwind theme: one `@theme` block, defaults zeroed, dark as a second explicit token set

**D5 resolution (`sdd-apply` slice 5, tasks.md reconciliation table).** This section's original code
sketch zeroed five properties (`--color-*`, `--font-*`, `--text-*`, `--radius-*`, `--shadow-*`) under a
comment reading "the four forbidden stock scales" — the miscount D5 flagged. **Decision: keep zeroing all
five, fix the comment.** ADR-6/the `design-system` spec name four scales as forbidden (colours, type
scale, border radii, shadows); `--font-*` is zeroed too because a stock Tailwind font-family utility
(`font-serif`, `font-mono`) is the same category of unauthored default identity the other four exist to
prevent, even though the RFC-2119 requirement text enumerates only four — zeroing a fifth scale beyond
that floor is permitted, not a contract violation (as the reconciliation table itself noted). As-built in
`web/src/styles/theme.css`; covered by `test/design-system/token-guard.test.ts`'s `stockFontClasses` case
(`font-serif`/`font-mono` emit no rule; `font-sans`/`font-numeric`, the re-declared project tokens under
the same namespace, still work).

**Choice**: `web/src/styles/theme.css` is the design-token file (ADR-6):

```css
@import "tailwindcss";
@custom-variant dark (&:where([data-theme="dark"], [data-theme="dark"] *));

@theme {
  --color-*: initial; --font-*: initial; --text-*: initial;
  --radius-*: initial; --shadow-*: initial;      /* the four ADR-6-forbidden scales, plus --font-* (D5) */
  /* project tokens — semantic names, light values */
  --color-bg: …; --color-surface: …; --color-ink: …; --color-ink-muted: …;
  --color-accent: …;                                     /* free choice — ADR-8 */
  --color-pending: …;      /* amber — reserved semantic: pending data */
  --color-provisional: …;  /* grey, paired with dotted pattern — reserved semantic */
  --font-sans: …; --font-numeric: …;                     /* tabular figures for data */
  --text-…: …; --radius-…: …; --shadow-…: …;
}
[data-theme="dark"] {  /* second EXPLICIT token set — never derived (§12.5, measured AA) */
  --color-bg: …; --color-surface: …; --color-ink: …; /* every token re-stated */
}
```

Zeroing `--color-*`/`--font-*`/`--text-*`/`--radius-*`/`--shadow-*` means `bg-blue-500` or `text-sm`
generates **no CSS rule at all** — the mistake is loud, asserted by a Vitest token guard that compiles
a probe file and asserts empty output for stock classes. Spacing and breakpoints keep Tailwind's
numeric scale (ADR-6 forbids palette, type scale, radii, shadows — spacing is not identity). Dark mode
is attribute-driven (`data-theme`), giving the no-JS path a server-set default; both token sets carry
measured AA contrast asserted by a contrast harness test over every declared text/background/tooltip
pairing. Chart SVG consumes tokens via `var(--color-…)` on SVG attributes (the one sanctioned `var()`
surface — presentational attributes, not utility classes).

**Alternatives considered**: `--*: initial` (zero everything) — rejected: destroys spacing/breakpoints
and buys nothing ADR-6 asks for; media-query-only dark mode — rejected: no explicit user toggle and
harder to test both themes deterministically.

### Supporting decisions

| Decision | Choice | Rejected | Why |
|---|---|---|---|
| Web scaffold | `@astrojs/svelte` + Tailwind Vite plugin; content layer = the export loader | separate data-fetch per page | one validated load, pages consume typed collections |
| Workbench | routes injected only when `WORKBENCH=1` (CI test/preview builds) | shipping unlinked prod routes | keeps prod surface = product pages; budget gates measure real pages only |
| Component-kit guard | Vitest test asserting `web/package.json` deps against a denylist (flowbite, shadcn, bootstrap, daisyui, @mui, @chakra-ui, …) | review-time vigilance | ADR-6 constraint becomes mechanical |
| Freshness in artifact | per-source `sources.json` (state + last success + attribution) | per-series rows | Fase 0 rule: a series is exactly as fresh as its source |
| CSV shape | one file per series: `period,value,status,source_status,version` + header comment with licence/attribution | one giant CSV | matches per-series JSON grain; attribution travels with the file (D2 licensing) |
| `openspec/config.yaml` | update web test commands (lines 18/92/95: `npm --prefix web test`, `test:e2e`, build) | — | ADR-8 stale-guideline concern in the proposal is **already resolved** in the current file; only test/build commands remain TBD |
| Six slugs | frozen in this change (Anexo E.1: permalinks never break) | freezing later in 1.3 | deliberate, per proposal Q3 |

## Export artifact schema (as-built, `schema_version: 1` — corrected `sdd-apply` slice 3, resolves D1)

**D1's resolution (task 3.1, tasks.md reconciliation table).** The reconciliation table found this
section's original illustrative sketch carried `vintage.ingestionRunId` ONCE per series document and NO
raw-file hash at all — insufficient for the spec's "Provenance survives the export" scenario, which
requires source, origin identifier, extraction timestamp, ingestion run AND raw-file SHA-256 to be
reachable **per observation**, not once per series (a revision means different observations of the same
series can legitimately come from different runs). The as-built schema below fixes this with a per-run
`vintages` lookup keyed by `ingestion_run_id` (an "equivalent" to a literal per-version lookup, as task
3.1 explicitly permits: many observations sharing one run's extraction instant and raw file need not
repeat both on every point) plus an `ingestionRunId` on every point, resolving any point's full
provenance chain with zero further database queries. `points` also moved from a `columns`-plus-tuple
shape to explicit named objects — clearer to validate and no less machine-readable — and gained a
`schema_version` field per document (not only the manifest) so a partially-regenerated artifact directory
can never mix documents from two incompatible schema generations undetected.

```jsonc
// series/tasa-de-paro-epa.json
{
  "schema_version": 1,
  "slug": "tasa-de-paro-epa",
  "name": "Tasa de paro (EPA)",                    // Spanish — data fixed by PRD
  "unit": "% población activa", "frequency": "Q", "decimals": 2, "geo": "ES",
  "operation": "ine-epa",                           // see "Disclosed gaps" below — stands in for a
                                                     // descriptive statistical-operation name
  "base": null,                                     // index base period (e.g. "2021=100"); no configured
                                                     // series declares one yet — see "Disclosed gaps"
  "source": { "id": "ine", "name": "INE", "attribution": "Fuente: INE …",
              "licenceName": "…", "licenceUrl": "https://ine.es/…/licencia" },  // slice 4: source.licence_url
                                                                                 // (migration 0004), not source.url
  "origin": { "kind": "ine-series-cod", "ref": "EPA453100", "requestUrl": "https://servicios.ine.es/…" },
  "vintage": { "ingestionRunId": 412, "extractedAt": "2026-07-29T06:10:00Z" },  // MAX run id among the
                                                                                 // series' current rows
  "points": [
    { "period": "2002-Q1", "value": 11.47, "status": "D", "version": 3, "ingestionRunId": 200 },
    { "period": "2026-Q2", "value": 10.29, "status": "P", "version": 1, "ingestionRunId": 412 }
  ],
  "sourceStatus": { "2026-Q2": "Provisional" },
  "withdrawn": [],
  "vintages": {
    "200": { "extractedAt": "2019-04-01T06:00:00Z", "rawFileSha256": "…", "requestUrl": "https://…" },
    "412": { "extractedAt": "2026-07-29T06:10:00Z", "rawFileSha256": "…", "requestUrl": "https://…" }
  },
  "breaks": [],                                     // slice 4: ListActiveEvents/ResolveActiveBreaksForSeries
  "events": [],                                     // wiring; structurally present, always empty this slice
  "freshness": "fresh"                               // "fresh" | "source-pending" — see D-2's own section
}
```

**Disclosed gaps (found while implementing slice 3, not part of D1, recorded here per this project's
established transparency convention rather than fabricated):**
- **`operation` (statistical operation).** Neither `config.SeriesConfig` nor any dataset-level config
  carries a descriptive statistical-operation name; `postgres.reconcileDataset` sets `dataset.name`
  literally equal to the dataset id (e.g. `"ine-epa"`). `operation` is populated from that existing,
  honest fact — never a fabricated Spanish description — until a future slice adds a real field.
- **`base` (index base period).** No configured series declares one anywhere in
  `config/series/*.yaml` today (e.g. IPC's "base 2021=100" lives only in a YAML comment, not a
  structured field). Always `null` until a future slice adds one.
- **`source.licenceUrl`** — **resolved, slice 4** (migration `0004_source_licence_url`): `source` now
  carries its own `licence_url` column, `reconcileSource` persists `config.LicenceConfig.URL` into it on
  every ingest cycle, and `ListPublishedSeries`/`publishing.SourceRef.LicenceURL` read it back directly.
  `export.go`'s `sourceLicenceURL` keeps `source.url` as a transitional fallback only for a row not yet
  re-reconciled since the migration shipped (no bespoke backfill — the next ingest cycle self-heals it,
  the same convention D-3's backfill note established).
- **`freshness`.** Reuses the EXISTING `postgres.SeriesFreshness`/`freshness.State` (fresh/failed,
  computed only from the source's last successful `download_attempt` against the fixed 24h window) —
  relabelled, not recomputed, onto this artifact's two reader-facing states (`publishing.ArtifactFreshness`).
  This is `SeriesFreshness`'s first real reader-facing consumer (archive-report W16 stands corrected).

## Data Flow

```
scheduler ─▶ IngestSeries×N ─▶ Gate ─▶ observations (unchanged Fase 0 path)
                 │ ≥1 published?
                 ▼
cmd layer ─▶ publishing.Publish
                 │ read: ListPublishedSeries · ListCurrentObservations
                 │       ResolveActiveBreaksForSeries · ListActiveEvents · SeriesFreshness
                 ▼
        ValidateArtifact ─fail─▶ alert; previous artifact untouched
                 │ ok
                 ▼
        write /public/data-derived (tmp + atomic rename; JSON + CSV)
                 ▼
        adapters/github repository_dispatch ─▶ GitHub Actions rebuild
                                                  │ fetch EXPORT_URL → verify sha256 → Zod
                                                  │ fail ⇒ exit ≠ 0, previous deploy stays
                                                  ▼
                                             astro build ─▶ Playwright/axe · Lighthouse (blocking)
                                                  ▼
                                             Portainer webhook deploy
scheduler watchdog: live manifest.generated_at vs newest export ─▶ ops alert on 30-min breach
      (request path: http.FileServer only — import guard unchanged)
```

## File Changes

| Path | Action | Description |
|---|---|---|
| `app/internal/publishing/{export,artifact,validate,csv,trigger}.go` | Create | D-1/D-2: model, validation, writers, dispatcher port |
| `app/internal/adapters/github/dispatch.go` | Create | `repository_dispatch` adapter |
| `app/internal/adapters/postgres/{events_read,published_series}.go` | Create | `ListActiveEvents`, `ListPublishedSeries` |
| `app/internal/adapters/postgres/observation.go` | Modify | `source_status` in types, columns, `WriteRevision` diff |
| `app/migrations/0003_observation_source_status.{up,down}.sql` | Create | D-3, additive + reversible |
| `app/internal/adapters/ine/{envelope,periodicity}.go` | Modify | TipoDato wired, `Secreto` dropped, full-payload segmentation |
| `app/internal/adapters/eurostat/envelope.go` | Modify | `status` read at `pos`; absence = D; `b`/`d` → BreakSignals |
| `app/internal/indicators/{observation,ports}.go` | Modify | Status/SourceStatus/BreakSignals on domain types |
| `app/internal/ingestion/ingest.go` | Modify | status mapping replaces hardcoded `StatusDefinitive`; break-signal alert |
| `app/internal/ingestion/validation/rule2_continuity.go` | Modify | D-4 interior audit |
| `app/internal/adapters/config/` | Modify | `cadence_segments` parse + validate |
| `config/series/poblacion-residente.yaml` | Modify | cadence correction — four-eyes documented but **unenforced** (disclosed governance gap, see Migration / Rollout) |
| `app/cmd/concontexto/{ingest_cmd,export_cmd}.go` | Modify/Create | publish-after-cycle wiring; `export` subcommand |
| `app/internal/scheduler/` | Modify | publish-latency watchdog |
| `web/` (theme, `src/lib/{export,chart,transform}`, 8 components, island, `src/content`, `src/i18n/es.ts`, 6 pages, tests) | Create | D-5/D-6; Vitest + Playwright + Lighthouse config |
| `.github/workflows/` | Modify | `repository_dispatch` rebuild job, web tests, blocking Lighthouse, e2e ingest→build job |
| `openspec/config.yaml` | Modify | web test/build commands (supporting-decisions table) |

## Testing Strategy (Strict TDD, applied honestly)

| Layer | What | Approach |
|---|---|---|
| Go publishing | model building, `ValidateArtifact` accept/reject, CSV projection, atomic swap, dispatcher failure → alert | **red-first**, table-driven; fakes for ports; `t.TempDir()` for writes |
| Go adapters | TipoDato mapping incl. unknown-token SchemaDrift; Eurostat flag matrix incl. absence=D and `b`/`d` signals; **ECP320 mixed-cadence fixture** passing segmented config and failing `frequency: Q` | **red-first**, trimmed real fixtures with `source.txt` provenance |
| Go validation | Rule 2 interior audit: first-run interior gap fails; allowlisted gap passes; segment-aware expectation | **red-first**, pure, table-driven |
| Go repository | `source_status` round-trip; status-only transition appends a version | testcontainers-go, `testing.Short()` guard (Fase 0 ADR-3 pattern) |
| Web unit (Vitest) | geometry + transforms (**red-first** — pure functions); golden fixture validates against Zod; token guard (stock class ⇒ zero CSS); contrast harness (AA both themes); island-initial-render == build SVG golden; dependency denylist | `npm --prefix web test`; `experimental_AstroContainer` for component output |
| E2E (Playwright) | axe on all six pages both themes; keyboard point navigation; 44 px targets; `javaScriptEnabled: false` context asserting SVG + table + description + methodology + break bands present; annotation groups (a)/(b) off by default; no affordance hides a break | **acceptance gates written alongside, not red-first** — forcing red-first here is theatre; page objects under `web/tests/e2e/` |
| Budget | < 300 KB transferred per page excl. typeface | blocking Lighthouse CI from the first web slice |
| Integration | ingest (fixtures) → export → validate → astro build succeeds; corrupted artifact fails the build | CI job — the two ecosystems provably meet |

## Threat Matrix

| Boundary | Applicability |
|---|---|
| Documentation-like paths | N/A — no executable-file classification; artifact is data JSON/CSV served statically |
| Git repository selection / commit / push state | N/A — no git automation; the pipeline never touches a repository |
| PR commands | N/A — no PR automation; `repository_dispatch` opens no PR and pushes no ref |

The one process-integration boundary — the Go binary's outbound `repository_dispatch` HTTPS call — is
designed above (D-2): fine-grained token from env, typed failure → alert, no retry loop, no shell, no
subprocess. Its failure behavior gets red-first unit tests as ordinary publishing-layer work.

## Migration / Rollout

`0003` is additive and reversible; a `down` loses annotation, never an observation. Rollout order is
the proposal's slice sequence: guards first (1), status second (2), publishing (3–4), web (5–9) —
slices 5–8 are additive and unreferenced until slice 9 wires routes, so each is independently
revertible. The cadence-fix rollback rule stands: correct config, never revert the guard. Artifact
rollback = redeploy previous pipeline binary; boot-time export self-heal regenerates the matching
artifact; site rollback = CI re-run of last green commit.

**Governance disclosure — four-eyes on `/config/**` is documented but unenforced.** Verified 2026-07-29:
`gh api repos/jorgealonsodev/concontexto/branches/main/protection` returns HTTP 404 ("Branch not
protected"). The control exists in `.github/CODEOWNERS` and `.github/BRANCH_PROTECTION.md`, but the
second owner is still the `@TODO-second-config-reviewer` placeholder from Fase 0, so nothing on GitHub
blocks an edit to `config/series/poblacion-residente.yaml`. This design deliberately adds **no
substitute control and no workaround**: the gap is a governance item already disclosed in the Fase 0
archive report and stays open until a second maintainer exists. Recorded here so no later reader
assumes the control is live.

## Open Questions

- [ ] **The `poblacion-residente` cadence-segment boundary — the boundary half is RESOLVED; the four-eyes
      half of this entry is still OPEN.** This entry originally recorded the boundary as `2023-Q3` (the
      earliest LIVE-VERIFIED continuous-quarterly point, exploration.md §4), because slice 1's offline
      `sdd-apply` session had no live network access to fetch `DATOS_SERIE/ECP320`'s true full history. That
      value has since been superseded and this entry was stale until now; corrected here rather than left
      to mislead a later reader (verify-report WARNING-10).

      **Boundary — resolved.** `config/series/poblacion-residente.yaml`'s own header records the live
      verification (2026-07-29, `DATOS_SERIE/ECP320?nult=9999&tip=A`): 122 observations spanning 1971-Q1
      through 2026-Q2, with interval steps of 100 × 2 quarters followed by 21 × 1 quarter. The arithmetic
      closes exactly — 100 semiannual intervals × 2 = 200 quarters = 50 years, and 1971 + 50 = 2021 — so the
      series is uniformly semiannual through 2020-Q3 and uniformly quarterly from 2021-Q1, with no mixed
      stretch between them. Read off disk 2026-07-30, `cadence_segments` now declares exactly:
      `{ from: "1971-Q1", to: "2020-Q4", cadence: semiannual, present: [1, 3] }` /
      `{ from: "2021-Q1", cadence: quarterly }`. Per apply-progress.md's "Orchestrator correction applied
      after slice 1 closed" section, the correction moved BOTH ends — the segment start from `1977-Q1` to
      `1971-Q1` as well as the boundary from `2023-Q3` to `2021-Q1`; only the resulting on-disk values were
      independently re-read here, not the intermediate `1977-Q1` state, which no longer exists to check.
      The guards were not touched, exactly as this entry required ("correct `from`/`to` in the YAML, never
      the guards").

      **Why the first segment ends at `2020-Q4` and not at `2020-Q3`, the last semiannual observation.**
      Not a discrepancy: `validateCadenceSegments` (`app/internal/adapters/config/validate.go`) requires
      each segment's successor to start at exactly `prev.to.next()` — a later start is reported as a gap, an
      earlier one as an overlap. `2020-Q4`'s next ordinal is `2021-Q1`, so `to: "2020-Q4"` is the only value
      that makes the two segments contiguous. Extending the boundary over Q4 adds no expectation, because
      `present: [1, 3]` means the semiannual segment never expects a Q4 observation in the first place.
      Confirmed live 2026-07-30: `go run ./app/cmd/concontexto validate-config` → `validate-config: ok`, and
      the slice-1 focused command `go test ./app/internal/adapters/ine/... ./app/internal/ingestion/validation/...
      ./app/internal/adapters/config/...` → all three packages `ok`.

      **What the correction avoided**, recorded because it is the concrete cost of the stale value: left at
      `2023-Q3`, the five genuinely quarterly observations between 2021-Q2 and 2023-Q2 would have been
      audited against the semiannual segment, which expects Q1/Q3 only, and rejected as invalid — real INE
      data discarded by a boundary set nine quarters late.

      **Four-eyes on `/config/**` — still open, re-verified 2026-07-30 and unchanged.** The edit that
      corrected this boundary was itself unguarded, and any future one still is.
      `gh api repos/jorgealonsodev/concontexto/branches/main/protection` returns HTTP 404
      ("Branch not protected"), and `.github/CODEOWNERS:17` still reads
      `/config/** @jorgealonsodev @TODO-second-config-reviewer`, with its own `TODO(maintainer)` note at
      lines 14–16 stating that a single-owner entry cannot satisfy a two-approval requirement. No substitute
      control and no workaround were added (see the governance disclosure under Migration / Rollout). This
      half of the entry is why the checkbox stays unticked; it closes only when a second maintainer exists.
- [ ] The spec must narrow the proposal's "three-state semaphore" exit criterion to two reader-visible
      states + ops alert (orchestrator decision recorded in D-2).
- [ ] Seven unconfirmed editorial dates: annotation layers render only confirmed events (reconcile
      already refuses nil dates); confirm or exclude before slice 7 or ship the layer disclosed-incomplete.
- [x] **`adapters/xlsx` does not classify `Status` at all** (discovered during slice 2b, D-3's
      "`ingest.go`'s disclosed compatibility shim ... is now closed" paragraph). **Resolved, slice 2c**
      (corrective slice, `sdd-apply`): `xlsx.Decode` (`app/internal/adapters/xlsx/decode.go`) now sets
      `Status: indicators.ObservationStatusDefinitive` on every parsed observation, with `SourceStatus`
      left as the empty string (`ingest.go`'s `sourceStatusPtr` maps that to a NULL `source_status`
      column). This is a positive, documented fact about the source — the Social Security "Afiliación
      media mensual" workbook (task 8.1: single flat sheet, zero formula cells, verified live) publishes
      no provisional/definitive marker at all, not a silently-reinstated default. See D-3's new
      "Implementation clarification (slice 2c" paragraph below for the full three-source contrast and
      `TestIngestSeries_XLSXSuccessfulDecodePublishesDefinitiveWithNullSourceStatus`
      (`app/internal/ingestion/xlsx_status_test.go`) for the end-to-end proof that a real, successfully-
      decoded XLSX ingest now publishes instead of blocking on `firstUnclassifiedStatus`.
- [ ] `PORTAINER_WEBHOOK_URL` / VPS provisioning blocks end-to-end latency verification (not the build).
- [x] **Artifact retention (tasks 4.9/4.10) and the publish-latency watchdog (tasks 4.11/4.12), deferred
      out of slice 4, closed in the follow-up batch.** `publishing.ArchiveArtifact` (`retention.go`)
      archives each successful `Publish` into a timestamped snapshot under a configured `historyDir`
      (`APP_PUBLISH_RETAIN_ARTIFACTS`, floored at 5, never honoured lower) and prunes down to the newest
      N; the live, currently-served `outDir` is never itself a pruning candidate, by construction (the two
      directory trees are always distinct — `retentionHistoryDir()` in `ingest_cmd.go` resolves history
      under `APP_DATA_ROOT`, deliberately never under `STATIC_ROOT`, so a retained — possibly
      since-corrected — snapshot is never itself publicly served the way the live artifact is). The
      publish-latency watchdog (`app/internal/scheduler/watchdog.go`'s `PublishLatencyBreached`, a pure
      decision taking `now`/`lastIngestionSuccess`/`manifestGeneratedAt`/`budget` as explicit parameters,
      never `time.Now()`) is wired into `startSchedulerLoop`'s existing 15-minute tick loop
      (`app/cmd/concontexto/schedule.go`'s `publishLatencyWatchdog`), evaluated once per tick for every
      source with a known last successful ingestion, decoupled from that source's own next-due gating —
      a stalled rebuild has to be caught on a regular cadence, not only when the 24h re-ingest interval
      happens to come back around. `alerting.PublishLatencyBreach` now has a real production call site.
      **One deliberate, disclosed reading**: this watchdog compares `lastIngestionSuccess` (per SOURCE,
      the same value `runScheduler`'s own `lastSuccess` map already tracks — `postgres.
      LastSuccessfulDownloadAttempt`) against the LOCAL export's own `manifest.json.generated_at`
      (`publishing.ReadManifest`), not a live deployed URL — the original design language ("the deployed
      site serves its own manifest.json") would need an HTTP fetch adapter against
      `PORTAINER_WEBHOOK_URL`/a provisioned VPS, which remains unprovisioned (a separate, already-disclosed
      dependency below) and would only prove END-TO-END deploy completion, not whether the rebuild
      followed AT ALL. The local-manifest comparison is a strictly WEAKER but still genuinely useful
      signal: since `Publish` always dispatches synchronously within the same call as a successful
      `Export`, a local manifest still older than a known ingestion success means the export (and
      therefore the rebuild trigger) never ran for that success at all — the case most worth alerting on
      regardless of whether the eventual CI build/deploy itself later succeeds. Also evaluated at SOURCE
      level (one shared manifest.json covers every series), matching `Alert.Series`'s own documented
      "empty for a source-level alert" convention already established for `KindSourceDown` — the existing
      `TestPublishLatencyBreach_NamesSourceSeriesAndElapsed` unit test (alerting package) still proves the
      function itself accepts a per-series call; this caller simply does not supply one. True end-to-end
      "the deployed page actually rebuilt" verification stays tied to the still-unprovisioned VPS/
      `PORTAINER_WEBHOOK_URL` dependency below.
- [x] **New (slice 4), RESOLVED slice 14 (2026-07-30): the ingest→export→build CI job does not yet prove
      "corrupted artifact fails the build" on the Astro side.** `.github/workflows/ingest-export-build.yml` runs a real
      `TestEndToEndIngestExportBuild` (`app/internal/ingestion/e2e_export_test.go`: real `IngestSeries`
      against a served fixture, real Postgres, real `publishing.Export`) followed by a real `astro build`
      in the same job — the structural chain the task asks for. What it does NOT yet prove: that a
      *corrupted* artifact fails that `astro build` step.

      **The stated blocker has EXPIRED; the scenario is now provable but still unproven, for a different
      and narrower reason.** Re-verified 2026-07-30. This entry originally blamed the missing Astro-side
      content loader. That loader has existed since slice 9a: `web/src/lib/export/loader.ts` reads
      `manifest.json` + `series/{slug}.json`, verifies every file against the manifest's declared sha256
      BEFORE parsing, parses with the slice-5 Zod schemas, requires exact `schema_version` equality, and
      throws on any failure with no catch-and-continue path anywhere in the module. It is genuinely wired
      into the build: `web/src/pages/indicador/[slug].astro:41` calls
      `loadExportArtifact(resolveLoadOptionsFromEnv())` inside `getStaticPaths`, and an uncaught error
      there is a fatal, non-zero-exit `astro build` failure that emits no page output. So a corrupted
      artifact DOES fail the build today — that half of the scenario is real, and
      `web/test/export/loader.test.ts` covers it (11 tests, re-run 2026-07-30, all passing).

      **What is still not proven is the CHAIN, in this job.** `.github/workflows/ingest-export-build.yml`'s
      `astro build` step sets neither `EXPORT_URL` nor `EXPORT_DIR`, so the loader falls back to
      `DEFAULT_FIXTURE_DIR` (`test/fixtures/export`) — the checked-in golden fixture. Meanwhile the Go step
      above it (`TestEndToEndIngestExportBuild`, `app/internal/ingestion/e2e_export_test.go:78`) exports
      into a `t.TempDir()` that is discarded when the test ends. The two halves of the job therefore never
      touch the same bytes: the build validates a fixture, not the artifact the ingest just produced.

      **What would close it**, recorded concretely so this entry stops being vague: have the Go step write
      its export to a path that outlives the test (a `TESTEXPORT_DIR`-style env var, or a small
      `go run ./app/cmd/concontexto export` invocation as its own step), pass that path to the build as
      `EXPORT_DIR`, and add a second build step that corrupts one exported byte and asserts a non-zero
      exit. None of that needs new production code — only workflow wiring — which is why this stays an open
      question rather than a design gap. The corresponding write-side protection
      (`publishing.ValidateArtifact` never writes an invalid artifact in the first place) has existed since
      slice 3 and is exercised by the Go test half of this job. The workflow file's own comment on the
      `astro build` step still repeats the expired "loader … not yet built" reasoning and should be
      corrected by whichever slice closes this; that file is outside the documentation pass that rewrote
      this entry.

      **CLOSED, slice 14.** The diagnosis above was confirmed in full by the writer who fixed it, element by
      element, and the workflow's own header now records the superseded rationale rather than deleting it.
      What closed it:

      **The chain is now real, declared once.** `.github/workflows/ingest-export-build.yml` declares one
      job-level `EXPORT_ARTIFACT_DIR: ${{ github.workspace }}/.e2e-export-artifact`. The Go step passes it as
      `E2E_EXPORT_DIR`; the build step passes the same variable as `EXPORT_DIR`. One definition read by both
      halves, not two literals that could drift apart. `app/internal/ingestion/e2e_export_test.go`'s
      `exportOutputDir` reads `E2E_EXPORT_DIR`, resolves it to an absolute path, fatals if it cannot, and
      falls back to `t.TempDir()` when unset — so an ordinary `go test ./...` still leaves nothing behind.
      `.gitignore` gained `/.e2e-export-artifact/`. A hand-off guard fails the job if `manifest.json` is
      absent at that path, so a renamed variable or a skipped test can no longer degrade into a silent
      fixture fallback — which is precisely the failure mode this entry described.

      **Connecting the directories was necessary but NOT sufficient, and this is the non-obvious part.**
      `getStaticPaths` filters `INDICATOR_CONTENT` down to the slugs the artifact actually carries
      (`Object.keys(INDICATOR_CONTENT).filter((slug) => seriesBySlug.has(artifactSlugFor(slug)) && …)`).
      So merely pointing `EXPORT_DIR` at the old test's output would have built a site with **zero indicator
      pages and still exited 0**. A green build proves nothing on its own. That is why the job also asserts
      consumption: all six frozen routes must exist in `dist/`, and the latest value the ARTIFACT carries
      must appear in the rendered HTML — read out of the artifact at run time, never hard-coded, so the
      assertion cannot drift from the data it checks. The original wording of this entry, "the rendered page
      shows the value that was ingested", is now mechanically true against `9.87`, the real INE 2026-Q2
      figure.

      **Why an env-driven output directory rather than invoking `concontexto export` as a separate step.**
      The end-to-end test's rows live in a transaction that is rolled back on exit, so no separate process
      could ever see them. The export must happen in-process, which is what `E2E_EXPORT_DIR` enables.

      **Three corruption scenarios, one per validation layer, each required to fail for the RIGHT reason.**
      `scripts/assert-corrupt-artifact-fails-build.sh` runs `astro build` three more times, each against its
      own throwaway copy: (1) sha256 mismatch — bytes edited, manifest digest left stale, caught before
      parsing; (2) `schema_version: 2` with a **reissued** digest, so the digest check passes and the exact
      version check is demonstrably what rejects it; (3) required `pageState` removed with a **reissued**
      digest, so Zod is demonstrably what rejects it. Each scenario must produce both the expected message
      AND attribution to the loader (`export loader:|parseSeriesDoc|parseManifest`), so a build that dies in
      Vite, on a missing dependency, or in an unrelated page cannot be mistaken for proof that artifact
      validation worked.

      **The inverted-assertion risk was PROVEN absent, not asserted — twice, independently.** This is the
      strongest part of the work and is recorded as such. (a) Accidentally: the first version used
      `process.argv.slice(2)`, wrong for `node -e`, so the corruption silently no-op'd; the script reported
      "3 of 3 corruption scenarios did NOT fail the build as required" and exited 1. The guard caught its own
      author's bug. The script now passes arguments through the environment and explains why in a comment,
      and additionally `diff`s the corrupted copy against the honest one before the build is allowed to mean
      anything. (b) Deliberately, as a negative control: `delete doc.pageState` was swapped for ADDING an
      unknown field with a reissued digest. Zod strips unknown keys, so the build legitimately succeeds — and
      the script correctly reported FAIL and exited 1. A guard that cannot be shown to fail is not a guard.

      **Two gaps this deliberately does NOT close, kept as disclosures.** (i) `manifest.json` is itself not
      digest-verified, and cannot be: it CARRIES the digests. It is Zod-validated, which is what catches a
      corrupted manifest. The schema-version scenario deliberately targets a SERIES document so the two gaps
      are not conflated. (ii) The corruption steps leave `web/dist` holding a failed build's output —
      harmless, since nothing deploys from this job, and documented in the workflow comment as the reason the
      consumption assertions run first.

      **What is proven versus what is only reasoned about — recorded precisely, because the distinction is
      the whole point of this entry's history.** PROVEN locally: the entire step sequence with real Docker,
      real Postgres, real ingest, real export, real build, and all three corruption failures. Verified
      independently for this record: `shellcheck` clean on the script, `bash -n` clean,
      `scripts/check-env-example.sh` OK, `go build ./...` and `go vet ./...` clean, the `ingestion` package
      compiles, `npm --prefix web test` 426/426, `astro check` 0 errors / 0 warnings / 2 hints. Reported by
      the writer and NOT re-verified here: `actionlint` 1.7.12 clean on all four workflows, sanity-checked
      against a deliberately broken copy that flagged both injected faults (`actionlint` is not installed on
      this machine). ONLY REASONED ABOUT, never observed — **no CI run has happened and no claim is made
      about one**: GitHub-hosted runners having Docker preinstalled (unchanged from the previous version of
      this job, which already relied on it), job-level `env` expansion and `github.workspace` resolving at
      runtime (actionlint validates context availability, not runtime expansion), and
      `setup-go`/`setup-node`/`npm ci` behaviour on a clean runner.
- [x] **D1 — export artifact shape lost per-observation provenance** (tasks.md reconciliation table).
      **Resolved, slice 3** (`sdd-apply`): the artifact model gained a per-run `vintages` lookup
      (`ingestion_run_id -> {extractedAt, rawFileSha256, requestUrl}`) plus `ingestionRunId` on every
      point; `publishing.ListPublishedObservations`/`postgres.ListPublishedObservations` joins
      `observation -> ingestion_run -> raw_file` in one query per series, so every published figure's
      provenance is reachable with zero further database queries. See "Export artifact schema (as-built
      ...)" above for the corrected schema and `TestExport_ProvenanceResolvesPerObservationWithoutAFurtherQuery`
      (`app/internal/publishing/export_test.go`) for the proof.
- [ ] **New (slice 3): the export artifact's `operation` and `base` fields have no backing config field.**
      `operation` (spec publishing-export's "statistical operation") is populated from `dataset.id`
      (an honest existing DB fact, not a fabricated description); `base` (index base period, e.g.
      "2021=100" for IPC) is always `null` — no `config/series/*.yaml` field carries it today. Neither
      is fabricated; both need a real config field in a future slice. See "Export artifact schema
      (as-built ...)" above, "Disclosed gaps".
- [x] **New (slice 3): `source.licenceUrl` has no distinct database column.** **Resolved, slice 4**:
      migration `0004_source_licence_url` adds `source.licence_url`; `reconcileSource` persists
      `config.LicenceConfig.URL` into it; `ListPublishedSeries`/`publishing.SourceRef.LicenceURL` read it
      back directly, falling back to `source.url` only for a row not yet re-reconciled since the
      migration shipped. This closes PRD §9.3 rule 5 / P2's "attribution is traceable" promise: the
      artifact's `licenceUrl` is no longer silently substituted by a source's general homepage URL.
- [x] **New (slice 3): `export` is a sixth subcommand the merged `platform-runtime` spec does not yet
      document.** `openspec/specs/platform-runtime/spec.md` (Fase 0, no delta spec in this change) still
      reads "Single binary with five subcommands". This change's own design D-2 explicitly commits to a
      standalone `export` subcommand (task 3.11), so it was added and
      `TestRealCommands_ExposesExactlySixRequiredSubcommands` (`app/cmd/concontexto/main_test.go`) was
      updated to match. **Resolved, slice 4**: `openspec/changes/phase-1-indicator-page/specs/platform-runtime/spec.md`
      now carries the owed delta (MODIFIED "Single binary with six subcommands", naming `export`).
- [x] **D4 — chart island/static split, resolved (slice 6, `sdd-apply`).** `BreakBand.astro`,
      `AnnotationChip.astro` and `AccessibleDataTable.astro` are three independent top-level components
      (tasks.md's reconciliation table, and see the updated row there), each with its own props interface,
      workbench section and `experimental_AstroContainer` test — none is fused into a chart-only partial.
      The generated textual description remains a slice-7 chart-internal generator with no standalone
      workbench entry (unchanged from the original resolution), not miscounted as a ninth catalog
      component.
- [x] **New (slice 6): a `--color-fresh` (green) token was added to `theme.css`, beyond the tokens D-6
      originally sketched.** PRD §6.1.1 names the freshness semaphore's normal state "verde" (green)
      literally; no existing token covered it (`--color-pending`/`--color-provisional` are RESERVED for
      pending/provisional data only — reusing either for "fresh" would violate "Reserved semantics are
      exclusive"). `--color-fresh` is a free ADR-8 colour choice, NOT a reserved semantic — it carries no
      meaning beyond FreshnessSemaphore's own "fresh" state, always paired with a distinct check-mark icon
      and the visible label "Al día" (this session's own proposed copy; the spec fixes only the amber
      label's exact wording, "Pendiente de actualización por la fuente" — "Al día" needs the same
      editorial sign-off as every other new reader-facing string this slice introduces). Contrast verified
      in both themes, both as body text on page background/surface and as a non-text graphic stroke;
      `DECLARED_PAIRINGS` in `src/lib/design-system/contrast.ts` extended accordingly.
      **Real finding during this slice**: an initial draft rendered the fresh/pending labels on a
      `/10`-opacity tinted pill background (`bg-fresh/10`) — the pure hex-pair contrast check
      (`contrast.test.ts`, computed against the opaque token alone) could not see this, but a genuine
      browser measurement (`tests/e2e/workbench/workbench.spec.ts`'s new AA contrast test, and
      independently axe-core's own scan) caught the translucent blend dropping real contrast to 4.29:1 and
      4.18:1 — below the required 4.5:1 — in the light theme. Fixed by dropping the translucent fill
      entirely (border-only, full-opacity text on the page's real opaque background). Concrete
      confirmation of this slice's own instruction to wire the workbench for real rendered-component
      measurement rather than token pairs alone: it found a defect the token-only harness structurally
      could not.
- [ ] **New (slice 6): `ActionBar`'s scope is narrower than PRD §6.1.1's full list** ("copiar permalink,
      exportar PNG / SVG / CSV / JSON, embeber (fase 3), citar"). Ships only what a zero-runtime-JS static
      component can do honestly: a permalink shown as selectable text ("Enlace permanente"), and CSV/JSON
      export links to the already-published `/data-derived` artifacts. One-click clipboard copy and native
      Web Share both need `navigator.*` JS; a formatted citation needs a per-request "access date" this
      static architecture cannot compute at build time; PNG/SVG export needs a rendered chart (slice 7/8);
      "embeber" is PRD's own "fase 3". Still open: where the JS-dependent actions eventually live (the
      chart island's own bundle, or a dedicated future micro-island) is a decision for the slice that
      actually needs them, not decided here.
- [x] **New (slice 6), resolved by measurement (slice 12): `MethodologySheet`'s responsive expand/collapse
      renders its full field set TWICE**
      (once inside a native `<details>` for mobile, once in an always-visible `hidden md:block` div for
      desktop), toggled by a CSS breakpoint rather than JavaScript, so exactly one copy is ever in the
      accessibility tree per viewport. This roughly doubles that component's own markup bytes on the page
      (methodology text only — a few KB, not images or data) — acceptable for this slice's static-component
      scope; worth revisiting against the 300 KB/page budget (§12.3) once slice 9 assembles a full page
      with everything else that budget has to share room with.
      **Revisited exactly as this entry asked, slice 12 (2026-07-30), and closed on the evidence.** The
      condition it set — a full page carrying everything the budget must share room with — is now met at
      REALISTIC data scale, not against the 3-point stub slice 9 measured: with the full-history fixture
      (98/98/294/294/125/122 observations, real breaks and events), a real production `astro build` +
      `astro preview` + `npm run budget:lighthouse` measures the six pages at **34.2, 45.0, 47.4, 47.7,
      49.0 and 60.0 KB** transferred excluding the typeface, against a 300 KB budget. The double render
      costs a few KB inside a page that uses under a fifth of its allowance at the widest. No change made;
      the disclosure closes because the measurement it was waiting for now exists, not because the
      duplication went away.
- [x] **New (slice 8), RESOLVED slice 13 (2026-07-30): "personalizado" (a free-form custom date-range
      picker) is NOT built.**
      `series-transformations`'s "Range presets" requirement names five options — 5 años, 10 años, desde
      2008, desde 2018, personalizado — and `ChartIsland.svelte` ships only the first four as real
      controls. Slice 7's `sliceCustomRange` pure primitive (`web/src/lib/transform/sliceRange.ts`)
      already exists and is directly reusable; what is missing is genuine UI (two date inputs, validation
      against the series' own span, permalink encoding for an arbitrary `from`/`to` pair rather than a
      named preset token) — materially larger scope than the four other presets, and outside this
      session's `RangePreset` type (`"full" | "5y" | "10y" | "since-2008" | "since-2018"`, no
      `"custom"` member). Disclosed rather than silently claimed satisfied; a follow-up slice needs to
      extend `RangePreset`, `permalink.ts`'s encode/decode and `ChartIsland.svelte`'s controls together.

      **RESOLVED, slice 13.** The three things this entry named as needing to move together did move
      together. The type question was answered differently from the way this entry framed it, and the
      difference is the interesting part: `RangePreset` was NOT extended with a `"custom"` member. A custom
      range carries its own `from`/`to` pair, so it cannot be a bare member of an enum whose every member is
      fully determined by the series' own span; it is modelled as `CUSTOM_RANGE`, a separate selection
      alongside the preset enum, unified by `type RangeSelection = RangePreset | typeof CUSTOM_RANGE`.
      `RANGE_PRESETS` is therefore correctly unchanged, and the test whose title the verify report flagged
      now says exactly that.

      **The design decision worth recording is what happens at the edges**, because the spec's own rule for
      the fixed presets ("a preset whose start precedes the series' first observation MUST be absent, not
      disabled") cannot be applied literally to a range that has no start until the reader types one.
      `resolveCustomRange` is that rule's equivalent, applied at commit time instead of render time, in
      three branches. A range PARTLY outside the span is clamped to the span AND the narrowing is disclosed
      in a live-region status line — a silent clamp would be the free-form equivalent of the disabled
      preset the spec forbids, a control that appears to honour the request while quietly showing something
      else. A range containing not one observation is REFUSED, the chart left untouched and the reason
      shown — the direct analogue of "absent, not disabled", since an empty chart is a view of nothing. An
      inverted, blank or unparseable range is refused. The coverage test runs against the series' real
      observation list rather than against `[first, last]`, which also catches a range landing inside a
      cadence gap and is what makes "never render an empty chart" a guarantee.

      **No-JS: absent, not present-but-dead.** The picker renders only after `onMount`. A statically built
      page cannot make a free-form range work without JavaScript, so shipping it unconditionally would put
      two date inputs and a commit button in front of a no-JS reader that look exactly like every working
      control on the page and do nothing. Gated at SSR level and proven under a real
      `javaScriptEnabled: false` context. **Still open and disclosed, not swept in**: the five FIXED preset
      buttons ARE server-rendered and are equally inert without JavaScript. That predates slice 13 and is
      outside WARNING-5's scope; it is recorded in `chart-no-js.spec.ts` and remains a real item for a
      future slice.

      Two further disclosures from that slice. `web/src/lib/chart/permalink.ts` was just outside the
      writer's given file territory and was extended anyway, justified on the grounds that a custom range
      must encode into the same query string the presets already own and splitting that across two
      independent owners would be worse — recorded as a territory deviation rather than left implicit. And
      the 44 px touch-target sweep was **blind to `<input>`** until this change added the product's first
      real form control; `interactiveControls()` now matches `input:not(.sr-only)`, the carve-out being
      `IndicatorChart.astro`'s visually-hidden annotation checkboxes whose entire touch target is the
      `<label>` the sweep already measures.

      Measured after it landed, not estimated: `astro check` 0 errors / 0 warnings / 2 hints; 426/426 unit
      tests across 32 files; 64 Playwright tests; the six pages at 34.2-62.1 KB against the 300 KB budget.
      `es.chart.customRange`'s 10 new Spanish strings need editorial sign-off, on the same footing as every
      other new reader-facing string in this change.
- [ ] **New (slice 8): `ChartIsland.svelte` renders its own complete chart experience** (SVG, break bands,
      annotation groups, accessible data table, textual description) rather than hydrating INTO
      `IndicatorChart.astro`'s existing DOM. This is a deliberate, disclosed architectural choice — the
      Astro/Svelte component boundary means a Svelte island cannot dynamically re-render an already-
      compiled `.astro` component's markup, so making the data table/description/breaks genuinely update
      live with the active toggle required a self-contained Svelte template that calls the SAME shared
      pure functions (`renderChartSVG`, `describeSeries`, `computeYoY`/`computeIntraPeriodRate`/
      `computePerCapita`) rather than a second, independently-authored renderer. `IndicatorChart.astro`
      remains the workbench's/no-JS-gate's own static-only catalog entry and is unaffected. Slice 9a (real
      page composition) must decide explicitly whether a production indicator page renders `ChartIsland`
      alone (with `IndicatorChart.astro` reserved for the workbench/no-JS proof only) or some other
      composition — not decided here, and NOT the same question as D-5's "one shared geometry module",
      which this slice satisfies exactly (there is still only one function producing the SVG string).
- [x] **New (slice 9b), resolved same slice — the export artifact's `pib` data is slugged `pib-cvi`, not
      `pib`.** Real, previously-undisclosed gap found while building this slice's third new page:
      `config/series/pib-cvi.yaml` is the Go pipeline's own series slug (and therefore the export
      artifact's `series/pib-cvi.json`, `csv/pib-cvi.csv`), while `indicator-page` spec's frozen route
      table names the page `pib` (Anexo E.1's permanent-permalink commitment). Every other series'
      pipeline slug and route slug coincide; only PIB diverges. **Resolved entirely at the web layer**,
      without touching the Go pipeline (out of this slice's file scope, and renaming a live production
      series slug is a separate, weightier decision than this slice's own remit): `IndicatorContentConfig`
      gained an optional `artifactSlug` field (`web/src/content/indicators/types.ts`), set only on
      `pib.ts` (`artifactSlug: "pib-cvi"`); `[slug].astro`'s `getStaticPaths` resolves every `seriesBySlug`
      lookup through `artifactSlugFor(routeSlug)` instead of the route slug directly, while `doc.slug`
      itself is left exactly as the artifact declares it (`pib-cvi`) because `ActionBar`'s CSV/JSON hrefs
      must resolve to the REAL published filenames; `IndicatorPage.astro`'s `data-slug` attribute and
      `ChartIsland`'s `slug` prop use `content.slug` (the route slug, `pib`) instead, so every
      reader-facing/test-facing identity is `pib` while every asset URL correctly stays `pib-cvi`. Verified
      by a real `astro build`: `/indicador/pib/index.html` renders with `data-slug="pib"` and
      `href="/data-derived/pib-cvi.csv"` side by side. **Open follow-up, not decided here**: whether
      `config/series/pib-cvi.yaml`'s own slug should eventually be renamed to `pib` in the Go pipeline
      (a schema/data-migration decision, since `series.id` is a stored primary key other tables reference)
      is out of scope for this change and left for a future slice/change to weigh.
- [x] **New (slice 9b), resolved same slice — the redirect mechanism (task 9b.2).** `indicator-page`
      spec's "a slug change ships a permanent redirect" scenario has no live use in THIS change (all six
      slugs are frozen here, none renamed) — `web/src/lib/indicator/redirects.ts`'s `SLUG_REDIRECTS` map
      is therefore empty in production, by design. The mechanism itself is Astro's own native `redirects`
      config option (wired in `astro.config.mjs`, fed via `buildAstroRedirects()`): with no SSR adapter
      configured (this project's exact static-output deployment shape), Astro emits a static HTML page
      carrying a meta-refresh + `rel="canonical"` for every configured entry, so a renamed slug's old
      permalink never 404s even without a server-level HTTP 301 rewrite. Proven via a test fixture
      (`redirects.test.ts`, task 9b.2's own instruction), not a live production redirect.
- [x] **New (slice 9b), resolved same slice — the Lighthouse transferred-bytes budget gate was NOT
      actually wired from the first web slice, contradicting `web-accessibility-gates` spec's own explicit
      "wired from the first web slice, not added at the end" requirement.** Checked directly against
      `.github/workflows/ci.yml`'s history across slices 5-8: no Lighthouse job, script or config existed
      anywhere in this repository before this slice. This is a genuine, disclosed spec-compliance gap, not
      a claimed-but-unverified pass — earlier apply-progress batches never asserted the gate was live, but
      neither did they flag that it was missing, which this entry now corrects. **Backfilled this slice**:
      `web/src/lib/budget/transferredBytes.ts` (pure, unit-tested — `computeTransferredBytesExcludingFonts`,
      `evaluatePageBudget`) plus `web/scripts/check-lighthouse-budget.mjs` (a real Lighthouse run via the
      `lighthouse` npm package, reusing Playwright's own installed Chromium via its remote-debugging port —
      no second headless-Chrome install), wired as a new blocking CI step over all six real indicator
      pages. Measured locally this slice (real numbers, not estimated): all six pages transferred
      **~35.5-35.8 KB** excluding the typeface — comfortably under the 300 KB budget. **Disclosed,
      unverified detail**: the CI wiring's cross-step background-process pattern (`nohup ... & disown` to
      keep the preview server alive past its own step boundary) follows a standard, widely-used GitHub
      Actions pattern but was not verified against a real GitHub Actions runner in this offline `sdd-apply`
      session — only against a local `npm run preview` + the budget script directly, both of which passed
      for real.

      **Superseded on three points by slice 12 (2026-07-30); the timeline gap above is unchanged and
      permanent.** (a) `web/scripts/check-lighthouse-budget.mjs` was DELETED and replaced by
      `web/scripts/check-lighthouse-budget.ts`, which imports the unit-tested budget module instead of
      hand-duplicating it (verify-report SUGGESTION-12); every `.mjs` mention in this change's records is
      historical from that date on. (b) The `nohup … & disown` cross-step pattern is gone: server and gate
      now share ONE CI step with a `trap`, so nothing depends on a background process surviving a step
      boundary. (c) The measured numbers above were taken against the 3-point fixture that shipped at the
      time; against the full-history fixture slice 12 installed, the six pages measure 34.2-60.0 KB, still
      far under the 300 KB budget.
- [ ] **New (slice 9a, written up here 2026-07-30): the methodology sheet's "Próxima publicación" and
      revision-history fields are rendered from fallback copy, not from data.** `tasks.md` 9a.9 promised
      "a new design.md Open Question" for exactly this and never wrote one (verify-report WARNING-11); this
      entry is that owed disclosure. `indicator-page`'s "The methodology sheet carries full traceability"
      requirement names both fields, and both render on every indicator page — the values are composed in
      `IndicatorPage.astro`, which every slug goes through — but neither is backed by pipeline data the way
      the sheet's source, operation, origin, periodicity, extraction, unit and vintage fields are. Every
      fact below was read off the working tree on 2026-07-30. The rendered result was additionally
      spot-checked against `web/dist/indicador/tasa-de-paro-epa/index.html` as it stood on disk (an existing
      build, not one produced for this check — it carries a `/workbench` page, so it was built with
      `WORKBENCH=1`), which matched the source on both fields.

      **Field 1 — next-publication calendar. No data source exists, anywhere.** Nothing in `config/`, in
      `app/internal/adapters/config/types.go`, or in `app/internal/publishing/artifact.go` carries a
      next-publication date or a per-series release calendar; the export artifact has no such field, so the
      web layer has nothing to read. What the page renders instead is a hand-maintained editorial link:
      `MethodologyContent.nextPublicationHref` (`web/src/content/indicators/methodology.ts`) is the SAME
      constant for all six series — `INE_CALENDAR_HREF`, a single INE calendar URL — so the value is not
      even per-series, let alone per-release. `IndicatorPage.astro` composes it as
      `` `${es.page.nextPublicationFallback} (${methodology.nextPublicationHref})` `` and
      `MethodologySheetFields.astro` prints that under the `es.methodologySheet.nextPublicationLabel`
      ("Próxima publicación") term. `es.page.nextPublicationFallback` is "Consultar el calendario de
      publicaciones de la fuente". So the reader sees a generic instruction plus a bare URL, rendered as
      plain text inside the `<dd>` — not an anchor, and not a date. Nothing false is asserted, which is what
      P4 requires, but it is fallback copy occupying a field the spec expects to carry a fact.

      **Field 2 — revision-history access. Partially backed; the missing half is the surface, not the
      data.** `tasks.md` 9a.9's own wording ("no backing data source anywhere in this project") is precise
      for field 1 and overstated for this one, so the accurate position is recorded here. Real data DOES
      exist and IS rendered: every artifact point carries `version` and `ingestionRunId`, the document
      carries the per-run `vintages` provenance lookup (design D-1), and the sheet's "Vintage mostrado"
      value is `` `Versión ${maxVersion}` `` computed from `doc.points` in `IndicatorPage.astro` — a real
      number off real data. Two things are genuinely absent. (a) The SUPERSEDED versions are not exported:
      `postgres.ListPublishedObservations` selects `WHERE o.series_id = $1 AND o.is_current`, so the
      artifact carries exactly one row per period and a reader cannot see what a figure used to be, only
      which version number the current one is. (b) There is no revision-history surface to link to.
      `IndicatorPage.astro` passes a `revisionHistoryHref` prop built as canonical-path + `#vintage`, which
      `MethodologySheetFields.astro` renders as an anchor labelled "historial de revisiones" — but
      `grep -rn 'id="vintage"'` over `web/src` and `web/dist` returns nothing, so the fragment has no target
      and the link resolves to the top of the page the reader is already on. Nor is there a dedicated route:
      `web/src/pages/indicador/` contains only `[slug].astro`. (`web/src/workbench/fixtures.ts` illustrates
      the component with `nextPublicationLabel: "2026-10-29"` and
      `revisionHistoryHref: "/indicador/tasa-de-paro-epa/revisiones"` — both workbench-only prop values with
      no production counterpart; neither the date nor that route exists.)

      **What would have to exist to close this.** For the calendar: a real per-series field carrying a
      published next-release date — either a `config/series/*.yaml` field with editorial sign-off, or an
      adapter that reads the source's own calendar — plus a decided rule for what the sheet renders when the
      source has announced nothing, since "no date yet" is a permanent, legitimate state and must not become
      a blank. For the revision history: an export path that carries superseded versions (relaxing the
      `is_current` filter, or a separate per-series revisions document with its own schema-version contract),
      and then either a real `id="vintage"` section on the indicator page or a `/indicador/{slug}/revisiones`
      route — at which point the existing `revisionHistoryHref` finally points at something. Until both
      exist, the fallback copy stays, and it stays disclosed rather than counted as satisfied.
- [x] **New (slice 12, resolved same slice) — the acceptance evidence no longer rests on a 3-point fixture,
      and the fixture now says plainly which of its numbers are real.** verify-report WARNING-6 established
      that every page-level test, every e2e traversal and every budget measurement ran against 3
      observations per series, while the real series are 98/98/294/294/125/122 points long — so the shipped
      evidence was unrepresentative, not wrong. `web/test/fixtures/export/` was regenerated through the real
      `ingest --reconcile` → `ingest --source=ine` → `export --fixture` chain against a testcontainers
      Postgres, and now carries exactly those lengths, spanning 1971-Q1 to 2026-Q2, with the real configured
      breaks (one each on the three EPA/IPC series) and ten reconciled events per series.
      **The honesty half matters as much as the length half**, and is recorded in the fixture's own
      `source.txt`: the structural and metadata fields, the breaks and events, and the newest THREE
      observations of every series are REAL (the tail rows are copied verbatim from the live-verified INE
      captures under `app/internal/ingestion/testdata/datos_serie/`, so the "latest value" each page renders
      is a genuine published statistic). **Every other observation is synthesised** — a straight line plus a
      fixed seasonal offset, chosen deliberately over a realistic-looking curve so that anyone glancing at
      the chart can see the history is not real, with a published recipe, no randomness, and
      byte-reproducible output. The file leads with an explicit "MUST NOT be quoted, charted, exported or
      cited" warning. Recorded here because a longer fixture that LOOKED real would have been a worse
      outcome than the 3-point stub it replaced.
- [x] **New (slice 12, resolved same slice) — the web layer had no static type verification at all, and the
      budget gate ran a copy of the logic the tests exercised.** verify-report SUGGESTION-14 and
      SUGGESTION-12, closed together because both were symptoms of `web/` having no TypeScript toolchain.
      `web/tsconfig.json` now exists (extends `astro/tsconfigs/strict`; `include` covers `src/`, `test/`,
      `tests/` and `scripts/`, not just `src/` — the suites are the files most likely to drift from the
      modules they exercise), `@astrojs/check` and `typescript` are devDependencies, and `astro check` is a
      BLOCKING step in `ci.yml`'s `web` job placed before the test steps. Result on 2026-07-30: **0 errors,
      0 warnings, 2 hints across 93 files**. Slice 11 had recorded this as blocked by the ADR-6 dependency
      guard; that concern was checked and did not apply — the denylist names component kits, not tooling —
      and the guard was left unmodified. With a type-checked `scripts/` directory, the stated reason for
      SUGGESTION-12's hand-duplication ("plain `node` cannot import a TS module") no longer held either:
      `check-lighthouse-budget.mjs` was deleted and `check-lighthouse-budget.ts` imports the unit-tested
      module directly, run under Node's own type stripping with no build step and no new dependency. Four
      guards in `check-lighthouse-budget-gate.test.ts` fail if the logic, the budget constant, or the orphan
      `.mjs` ever comes back. `strictest` was deliberately NOT adopted: `noUncheckedIndexedAccess` and
      `exactOptionalPropertyTypes` are real decisions about how this codebase models optionality and belong
      in their own change.
- [x] **New (slice 12, resolved same slice) — the budget gate was measuring the WORKBENCH build.**
      verify-report WARNING-8. `playwright.config.ts` sets `reuseExistingServer: false` under `CI` and its
      `webServer.command` is `WORKBENCH=1 npm run build && npm run preview`, so the e2e step always
      overwrote `dist/`; the production `astro build` ran BEFORE it, and the preview server and Lighthouse
      gate that followed measured the workbench artifact rather than the one that ships. Two changes, and
      the second is the one that lasts. (1) The production build now runs AFTER the e2e step. (2) The gate
      no longer depends on that ordering being right: `assertProductionBuild` probes `/workbench` — a route
      `astro.config.mjs` injects only under `WORKBENCH=1` — before Chromium is even launched, and refuses to
      measure a build that answers 2xx there. The ordering is the fix; the assertion is what stops a future
      edit from silently undoing it, and it is covered end to end against a real HTTP server. The same
      change collapsed the two-step `nohup … & disown` pattern (verify-report scrutinised claim 6) into one
      step with a `trap`, removing the cross-step process-survival assumption entirely.
- [x] **New (slice 12, resolved same slice) — the two break-band renderers can no longer drift apart, and
      the trigger is a real button.** Two findings that shared one root cause.
      verify-report WARNING-7: `ChartIsland.svelte`'s break-band trigger was a `<span tabindex="0">` with no
      role and no handler, emitting `a11y_no_noninteractive_tabindex` on every build — no demonstrated WCAG
      failure, but a warning left neither fixed nor suppressed will mask the next real one. Both renderers
      now emit `<button type="button">`; the production build log carries zero a11y warnings, and the only
      remaining `tabindex` in the island is the legitimate roving-tabindex on chart points.
      The carried-forward disclosure (raised by the post-slice break-band tooltip defect, repeated by slice
      11) is also closed: `web/test/design-system/break-band-parity.test.ts` asserts that both renderers
      emit the same classes and `data-testid`s, that both use the same element and it is a `<button>`, and
      that `src/styles/components.css` actually carries rules for those exact names with neither component
      having taken them back into a scoped `<style>`. The third assertion is not redundant — a rename
      applied consistently to both components but not to the stylesheet keeps them in perfect parity while
      reproducing the original painted-at-rest defect exactly. **The underlying architecture is unchanged
      and still disclosed**: two renderers still hand-write the same markup (see the slice-8 entry above);
      the guard makes that survivable, it does not make the markup single-sourced.
- [x] **New (slice 12, resolved same slice) — a real heading-order defect the 3-point fixture had been
      hiding.** Not a verify-report finding; found only because task 12.6's full-history fixture brought
      real breaks onto the pages for the first time. With `breaks: []` on every series, `ChartIsland`'s
      break list never rendered, so nothing exercised its heading. With breaks present it rendered under an
      `<h4>` directly after the page `<h1>` — a two-level skip that axe's `heading-order` rule fails. Fixed
      with a `breaksHeadingLevel?: 2 | 3 | 4` prop defaulting to 4 (correct in the workbench, where the
      island sits under a section heading) with `IndicatorPage.astro` passing 2. Recorded as its own entry
      because it is the concrete evidence for WARNING-6's actual argument: the thin fixture was not merely
      unrepresentative, it was structurally concealing defects. Also closed alongside: SUGGESTION-13 —
      `MethodologySheet` now receives `slug={content.slug}` (the route slug), so the pib page no longer
      carries the DOM id `methodology-heading-pib-cvi`; `ActionBar`'s two hrefs still use `doc.slug`
      deliberately, because they must name real published files.
- [x] **New (slice 16), resolved same slice — `SeverityBlockRequiresSignoff` had no resolving half, so it
      was a comment rather than a control.** The severity has existed since PR 4b and `rule4_revision.go`
      argues its purpose at length: a deep revision is either a legitimate methodology revision or a parser
      silently rewriting history, the machine cannot tell, so the decision goes to a human. Nothing ever
      resolved it — `validation/gate.go:97`'s `blocks()` returned true for `SeverityBlock` and
      `SeverityBlockRequiresSignoff` alike, so a finding that named a human decision was permanently
      terminal. This design document never recorded the gap, and the capability that closes it had no design
      entry at all until this one (verify-report pass-5 WARNING-40 names that omission).

      **What closed it (commit `bdb6cc8`).** An editorial acknowledgement registry,
      `config/reconocimientos.yaml`, reconciled to `validation_acknowledgement` (migration
      `0006_validation_acknowledgement`) the same way `rupturas.yaml` and `eventos.yaml` are: the file is the
      source of truth, the table is a projection. The design decisions worth carrying forward, each of which
      constrains a future change:

      - **Scope is one series, one period, one rule, compared by exact equality.** There is no schema syntax
        for "all periods", "the whole series" or "all rules", and `adapters/config/acknowledgement.go`
        rejects any attempt to widen. A mechanism that can blanket-disable a guard is worse than the gap it
        fills.
      - **Only `rule3-plausibility` and `rule4-revision` are acknowledgeable**, and
        `TestAcknowledgeableRules_AreExactlyRule3AndRule4` runs the real rules so the allowlist cannot drift
        from what they emit. Those two are the ones whose finding genuinely means "this number is surprising
        and I cannot tell legitimate from broken". Rule 2 is excluded because its own requirement says the
        remedy is to correct the series configuration, never to relax the rule.
      - **Staleness is a pinned value, not an expiry date.** The calendar is unrelated to whether the datum
        changed; an expiry would re-block correct data while still covering revised data — wrong in both
        directions. A revised value raises its own blocking `acknowledgement-stale` finding naming both
        values.
      - **An override is never mistakable for a pass**: outcome `publish-overridden`, a WARN-level log
        carrying an `acknowledgements` attribute naming the record and the signer, and
        `ingestion_run.outcome = succeeded-with-acknowledgement`.
      - **The authority is the signature, and the unsigned state is modelled rather than absent.**
        `signature_status: unsigned` + `drafted_by` + `todo` is `rupturas.yaml`'s `date_status: unconfirmed`
        discipline applied a second time — never project an unverified fact. An unsigned record is inert in
        two independent layers (`reconcile.go:116` never projects it; `validation/acknowledgement.go:168`
        refuses it again), because a mechanism whose safety rests on one layer never being bypassed is not
        safe. This is not a hypothetical: the first version of `bdb6cc8` shipped a forged signature, and the
        two-state schema is the fix for the schema hole that produced it. See "A second process finding" in
        `apply-progress.md`.

      **The registry is designed, shipped, tested and currently resolving nothing**, because its one record
      is an unsigned draft. That is the intended behaviour, and it is also this change's only archive
      blocker — recorded as its own open question below rather than folded in here.
- [ ] **New (slice 15/16/17) — "a check placed where the failure it guards cannot occur" has now appeared
      FIVE times in this change, and deserves a design entry rather than five separate findings.** The
      instances, in order: CRITICAL-2 (`details?.items ?? []` made the blocking budget gate unable to fail);
      CRITICAL-15 (the silent fixture fallback); CRITICAL-27's `dist/` loop (the only all-six route
      assertion lived in a job that always builds from a six-series fixture); slice 14's finding that
      `ingest-export-build.yml`'s two halves never touched the same bytes; and now CRITICAL-37, where the
      **entire CI corpus** builds from artifacts in which the production failure cannot occur.

      Verify-report pass 5 tabulates the last one precisely: `ci` / Frontend uses
      `BUILD_WITH_SYNTHETIC_FIXTURE=1`; `ci` / Container smoke uses `web/data-derived` from the Go e2e test;
      `ingest-export-build` uses `TestEndToEndIngestExportBuild`; the Playwright and Lighthouse gates use the
      committed fixture. All four always contain all six series. The third looks like it should catch this
      and is the one worth naming: it *does* run a real `IngestSeries` for all six frozen slugs through a
      real Postgres, but `ineIngestConfig` (`app/internal/ingestion/ingest_test.go:121`) passes
      `Validation: config.ValidationConfig{}` — no thresholds at all — over three-period fixtures that do not
      contain 2020-Q2. The one job exercising the real Go→Astro hand-off disables the guard that blocks the
      series in production.

      **What distinguishes this instance from the previous four**: it is not a bug in a gate. Each gate is
      correct. The gap is that **no CI path is ever handed production's own artifact shape**. That makes it a
      design question about the test corpus rather than a defect in any one check, which is why it is
      recorded here.

      **The generalisable rule**, stated so a future slice can apply it without rediscovering it: a check is
      only evidence if its input can carry the failure. When adding a guard, name the artifact that would
      trip it and confirm some job actually supplies that artifact — otherwise the guard proves the fixture,
      not the system. Slice 14's paired negative control and slice 15's paired control build are the two
      places in this change where that was done properly; both were done by deliberately constructing the
      failing input, not by trusting the existing corpus.

      **Open.** The concrete remedy is one CI path that builds from an artifact produced by an ingestion
      running the **real** `config/series/*.yaml` thresholds, so that "the production build works" stops
      being an unmeasured claim. Verify-report pass 5 calls this CRITICAL-37's structural half and states it
      is **recommended alongside, not blocking**, the signature. **Checked 2026-07-30 18:03 UTC and NOT yet
      landed**: `git status --short` reported only `verify-report.md` modified, HEAD `1f856e2`, and
      `ingest_test.go:121` still read `Validation: config.ValidationConfig{}`. Another writer was working on
      it concurrently; per `apply-progress.md`'s own staleness rule, that is a statement about a moment, and
      the decisive check is the content of `ineIngestConfig`.

      **Superseded at 2026-07-30 18:09 UTC, six minutes later, and left visible rather than rewritten.** The
      decisive check flipped: `Validation: config.ValidationConfig{}` no longer appears in
      `app/internal/ingestion/ingest_test.go`, which now carries `shippedConfig` and `realValidationConfig`
      (the latter's comment cites CRITICAL-37 by name), alongside two new files —
      `app/internal/ingestion/e2e_blocked_export_test.go` and
      `scripts/assert-blocked-series-fails-build.sh`. **Not upgraded beyond what was observed**: all of it
      was uncommitted working-tree state (`git log --oneline -1` still `1f856e2`), and
      `grep -rn assert-blocked-series-fails-build .github/` returned nothing, so no workflow yet invoked the
      new script. The Go half exists; the CI wiring that makes a green signal able to go red for CRITICAL-37
      was not yet observable. **This entry stays open** until a CI path demonstrably builds from an artifact
      produced under the real thresholds — the generalisable rule above is the reason: the guard is only
      evidence once some job supplies the failing input.
- [ ] **New (slice 15/16) — the change cannot currently produce a deployable site, and the remedy is a human
      signature rather than a commit (verify-report pass-5 CRITICAL-37, the one archive blocker).** The
      chain, each link measured by the pass-5 verifier: `ocupados-epa` is blocked by `rule3-plausibility`;
      the acknowledgement that would resolve it is unsigned and therefore inert in both layers; `export.go`
      skips a series with zero observations, so the slug is in neither `series/` nor `manifest.series`; and
      slice 15's frozen-route guard therefore refuses the build, emitting zero pages rather than five. Every
      link is behaving as designed, which is precisely why no code change is the right response.

      **The remedy is exactly one of two acts, both human**: a named person reviews and signs
      `config/reconocimientos.yaml`'s one draft, following the three review steps and three edits its own
      `todo` field already spells out; or the same person rejects it and deletes the entry whole ("un
      registro rechazado no se deja a medias"), after which `ocupados-epa` needs a different remedy and its
      own SDD cycle. **Do not** raise `max_delta_abs`, add a break to `config/rupturas.yaml`, weaken
      `resolveIndicatorRouteSlugs`, or let an agent sign the record — the first two were considered and
      correctly rejected in slice 16's reasoning, the third would reintroduce CRITICAL-27, and the fourth was
      already attempted and caught.

      This entry stays open until the signature or the deletion exists on disk. It is the only thing standing
      between this change and archive.
- [ ] **New (slice 16) — one acknowledgement can resolve more than one finding (verify-report pass-5
      WARNING-38).** `Acknowledgement.covers` matches on `(series, period, rule)`, and `Rule3Plausibility`
      emits two semantically distinct findings under that one rule name — a min/max range breach and a
      period-over-period delta breach — which can both occur at one period. The verifier demonstrated it at
      runtime: one signed acknowledgement, `overridden=2`, `unresolved=0`. So a human vouching for a delta
      silently also vouches for a range breach they may never have looked at.

      **Mitigated, not closed.** The pinned value constrains both findings to the same number the human
      reviewed, so a parser bug producing a different value fails closed, and the case is not reachable in
      the shipped config (18607.2 is well inside `[0, 30000]`). The spec's prose is finding-singular
      throughout, and the scenario "An acknowledgement never widens beyond the finding it names" asserts only
      that no *configuration* expresses it — which remains true. The narrow fix is to give rule 3's two
      emission sites distinct rule names, or to key the acknowledgement scope on the finding kind as well as
      the rule. Follow-up, not an archive blocker.
- [ ] **New (slice 16) — the registry's only anti-forgery control is a review gate that is not enforced
      (verify-report pass-5 WARNING-39).** The design's own claim is that authority is the human signature.
      The enforcement is a placeholder filter: empty, whitespace, under two characters, or one of a table of
      vacant tokens (19 of them — both the commit body and the verify-report say 18; counted on disk for
      `apply-progress.md`). Everything else is accepted, and a record signed with a plausible full name and a
      date passes `validate-config: ok`.

      The code names four-eyes review on `/config/**` as the compensating control. That control is **not
      enforced**: `gh api repos/:owner/:repo/branches/main/protection` returns `404 Branch not protected` at
      `1f856e2`, and `.github/CODEOWNERS` / `.github/BRANCH_PROTECTION.md` are documentation. This is not a
      new gap — SUGGESTION-35 carried it forward as documented-but-unenforced — but adding a mechanism that
      **overrides a validation gate** changed its severity without anyone re-adjudicating it. It is also
      empirically load-bearing: the fabricated signature in the first version of `bdb6cc8` was caught by a
      human reading the diff, and nothing in this repository would have caught it. Carries WARNING-10's
      four-eyes half, which has been open since remediation B for the same reason: it needs a second
      maintainer, not code.
- [ ] **New (slice 17) — "a failed rebuild raises an alert immediately" is substituted, not implemented
      (verify-report pass-5 WARNING-41).** The `pipeline-operations` scenario reads: *"GIVEN a rebuild
      dispatched by a successful ingestion that fails, WHEN the failure is observed, THEN an alert is raised
      naming the ingestion run and the build failure."* Nothing observes a failed CI rebuild.
      `alerting.DispatchFailed` fires when the **POST** fails, which is a different event; a failed
      `rebuild.yml` run produces no callback and is caught only by `RebuildLatencyBreached` once the
      30-minute budget elapses.

      The substitution is a good one — a receiver-side callback would need a second inbound path into the
      deployed stack, and the build-manifest comparison covers the failure with no call to GitHub, Portainer
      or the public site — but "immediately" is not what happens, and until this entry the substitution was
      disclosed only in a comment inside `.github/workflows/rebuild.yml`. **Open in the honest sense**:
      either the spec's timing clause is narrowed to what budget-delayed detection actually provides, or a
      rebuild-failure callback is built. Neither has been done, and the scenario is one of the two
      non-compliant ones in pass 5's 159/161.

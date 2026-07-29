# Tasks: phase-0-data-foundations

Note on scope: this change covers PRD Fase 0 in full (65 requirements / 114 scenarios across
10 capabilities). The generic 530-word tasks-artifact budget does not fit that scope without
producing an unusable checklist; per orchestrator instruction, completeness and per-requirement
TDD pairing take precedence here. Tasks are grouped by slice (dependency order, not PRD order),
hierarchically numbered, 1–2 lines each.

## Review Workload Forecast

| Slice | Milestone | Est. changed lines | Budget risk | Split needed |
|---|---|---|---|---|
| 1 | 0.1 | ~750–850 | High | Yes → PR 1a (binary/httpserver/healthcheck), PR 1b (container/CI/deploy) |
| 2 | 0.4 | ~700–800 | High | Yes → PR 2a (schema+testcontainers), PR 2b (writer/repository methods) |
| 3 | 0.7 structure | ~450–550 | Medium-High | No (single PR, tight) |
| 4 | §9.3 | ~500–600 | Medium-High | No (single PR; rules are short, tests are wide) |
| 5a | 0.2 (client) | ~700–800 | High | Yes → PR 5a-i (filestore/freshness), PR 5a-ii (INE client/fixtures) |
| 5b | 0.2 (load) | ~300–400 | Low-Medium | No |
| 6 | 0.3 | ~600–700 | High | Yes → PR 6a (adapter/datasets), PR 6b (guards: pinning/ceiling/retry/probe) |
| 7 | 0.5 | ~500–600 | Medium-High | No (single PR; reconcile is one cohesive unit) |
| 8 | 0.6 | evidence-blocked | Unknown until 8.1 | Plan for split → PR 8a (structure/happy path), PR 8b (malformed suite) |
| 9 | 0.7 closure | ~500–600 | Medium-High | Yes → PR 9a (scheduler/probe), PR 9b (logging/alerts/attribution closure) |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Forecast accuracy — measured after slices 1–3

The estimates above were consistently LOW. Measured against the delivered code:

| Slice | Estimated | Actual | Ratio |
|---|---|---|---|
| 1 | ~750–850 | ~1,071 Go (PR 1a) + container/CI/deploy config (PR 1b) | ~1.4× on the Go half alone |
| 2 | ~700–800 | ~865 (PR 2a) + PR 2b; `adapters/postgres` totals 1,723 | ~2×+ |
| 3 | ~450–550 | ~1,563 | ~3× |

Repository total after slice 3: **4,255 lines of Go**, against a whole-Fase-0 estimate of ~5,300.
Three of ten slices are done and the estimate is nearly exhausted.

Cause: the estimates counted implementation and under-counted tests. Under Strict TDD the tests
are the larger half, and the specification's failure-mode coverage (six validation rules, the
INE refusal envelope, Eurostat's silent-empty and 157 MB oversized-success modes, eight malformed
XLSX cases) multiplies test volume rather than production code.

**Revised projection for Fase 0: roughly 12,000–15,000 lines, not ~5,300.** Slices 4–9 should be
read as ~2–3× their table estimate above.

**Confirmed again on slice 7 (PR 7a, tasks 7.1-7.9):** authored ~1,980 lines against the table's
~500-600 estimate for the WHOLE slice — ~3.5× on 9 of 15 tasks alone. Per orchestrator instruction,
PR 7a stopped at the 7.9/7.10 boundary (the natural seam between "author the YAML + build/prove the
reconcile engine" and "domain-type guarantee + CLI wiring + branch-protection verification") rather
than accumulate further. PR 7b covers tasks 7.10-7.15.

**Consequence for delivery.** Slice 3 shipped as one unit at ~4× the 400-line budget and MUST be
split before review. Natural seams, in dependency order:

- PR 3a — `app/internal/adapters/config/{types,loader}.go`, `config_embed_test.go` (loader + the
  compile-time pinning proof)
- PR 3b — `app/internal/adapters/config/validate.go`, `app/internal/guard/`,
  `app/cmd/concontexto/validate_config_cmd.go` (validation + the origin-identifier static scan)
- PR 3c — `config/sources/*.yaml`, `LICENSE-DATA`, `licensedata_test.go`,
  `app/internal/adapters/postgres/attribution*.go` (licensing + attribution)

Apply the same discipline to slices 4–9: assume the table's estimate is the floor, not the ceiling,
and pre-plan a split rather than discovering the overrun after the code exists.

**Confirmed again on PR 9a (tasks 9.1-9.8):** authored ~1,560 lines (1,483 across 13 new files +
~76 net lines added to `app/internal/adapters/ine/client.go` and `app/internal/ingestion/ingest.go`)
against the table's own ~500-600 estimate for the WHOLE slice 9 — roughly 2.6-3× on 8 of 11 tasks
alone, continuing the exact pattern this section already documents. Per the batch's explicit scope
boundary, PR 9a stopped cleanly at the 9.8/9.9 boundary (scheduler + probe + logging + alerting
fully done and tested) without starting 9.9-9.11 (attribution closure + the Fase-0 closure gate),
which is PR 9b, the final batch of the whole change.

Rationale for **stacked-to-main**: every slice has its own testable exit criterion (proposal.md),
the schema is append-only/greenfield, and the proposal's own rollback plan is "`git revert` the
slice PR" — i.e. slices are designed to merge to `main` sequentially, not accumulate on a tracker
branch. `auto-chain` delivery strategy means the orchestrator proceeds directly with this strategy;
no user prompt is required before `sdd-apply` begins slice 1.

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Go binary skeleton: dispatch, httpserver golden-rule guard, healthcheck, cache headers, migrate stub | PR 1a | `go test ./app/cmd/... ./app/internal/httpserver/...` | `go run ./app/cmd/concontexto serve` + `curl /healthz` | revert `go.mod`, `config_embed.go`, `app/cmd`, `app/internal/httpserver` |
| 2 | Container/CI/deploy: Dockerfile, compose, CI workflow, CODEOWNERS, hello-world web, backup script, LICENSE | PR 1b | N/A — no Go unit surface | container smoke test: `docker compose up` + `curl /healthz` + `docker exec ... healthcheck` | revert Dockerfile/compose/workflows/CODEOWNERS/web/scripts/LICENSE |
| 3 | Ten-table schema, `is_current` invariant, reversibility, testcontainers harness | PR 2a | `go test ./app/internal/adapters/postgres/... -run Migration` (testcontainers, Docker required) | `migrate up` against local `postgres:17-alpine` | revert `app/migrations/*` + `TestMain` harness |
| 4 | Observation writer, provenance, source-mapping, tombstone, rollback | PR 2b | `go test ./app/internal/adapters/postgres/...` | simulated-revision script against local Postgres | revert writer/repository files added on 2a |
| 5 | Config schema, `go:embed`, `validate-config`, attribution structure | PR 3 | `go test ./... -run Config` | `go run ./app/cmd/concontexto validate-config` | revert `config_embed.go`, `app/internal/adapters/config`, `config/sources/*`, `LICENSE-DATA` |
| 6 | Six pure validation rules + publish gate | PR 4 | `go test ./app/internal/ingestion/validation/...` | N/A — pure functions, no I/O; focused tests are complete proof | revert `app/internal/ingestion/validation` |
| 7 | Raw file store, download-attempt log, freshness | PR 5a-i | `go test ./app/internal/adapters/filestore/... ./app/internal/adapters/postgres/... -run DownloadAttempt` | ingest a fixture twice; inspect `app_data/` + `raw_files.sha256` | revert filestore + `download_attempt` files |
| 8 | INE `DATOS_SERIE` client, envelope guard, fixtures | PR 5a-ii | `go test ./app/internal/adapters/ine/...` | `ingest --series tasa-de-paro-epa` against `httptest` fixture (offline) | revert `app/internal/adapters/ine` + `testdata/ine` |
| 9 | Six-series full historical load, e2e wiring | PR 5b | `go test ./app/internal/ingestion/... -run INESixSeries` | `ingest --source ine` end-to-end against fixtures | revert six `config/series/*.yaml` + e2e wiring test |
| 10 | Eurostat adapter, three datasets | PR 6a | `go test ./app/internal/adapters/eurostat/...` | `ingest --source eurostat` against fixtures | revert `app/internal/adapters/eurostat` + eurostat `config/series/*.yaml` |
| 11 | Dimension-pin validation, size ceiling, maintenance retry, probe param | PR 6b | `go test ./... -run EurostatGuard` | N/A — guard logic has no separate runtime surface | revert guard functions added on 6a |
| 12 | Editorial YAML, transactional reconcile, four-eyes wiring | PR 7 → split PR 7a (tasks 7.1-7.9, DONE) + PR 7b (tasks 7.10-7.15, DONE) | `go test ./app/internal/ingestion/... -run Reconcile` | `ingest --reconcile` run twice (idempotence, real Postgres) — operational manual single-approval-PR-is-blocked verification against `/config/**` remains BLOCKED (placeholder second reviewer, zero-commit repo, see 7.14) | revert `config/{rupturas,eventos,gobiernos}.yaml` + `app/internal/adapters/postgres/editorial*.go` + `app/internal/ingestion/reconcile*.go` + `app/cmd/concontexto/ingest_cmd.go` (restoring `stubs.go`) + `app/internal/guard/codeowners_test.go` |
| 13 | XLSX structure + happy path (8.1 resolved, DONE — tasks 8.1-8.9) | PR 8a | `go test ./app/internal/adapters/xlsx/...` (all pass, includes `TestDecode_HappyPath`) | ingest the real fixture workbook via `xlsx.Client.FetchRaw`+`Decode` (proven in `TestClient_FetchRawThenDecode`) | revert `app/internal/adapters/xlsx` + `config/series/afiliacion-ss.yaml` + `config/sources/seg-social.yaml` + the additive `ObservedSchema` field/wiring in `indicators/ports.go` + `ingestion/ingest.go` + `config/types.go`/`validate.go`'s XLSX schema fields |
| 14 | Malformed-file suite (8.1/8a resolved, DONE — tasks 8.10-8.11). **This closes Milestone 0.6.** | PR 8b | `go test ./app/internal/adapters/xlsx/... -run Malformed` (offline, 9 cases) + `go test ./app/internal/adapters/postgres/... -run TestApplyGate_BlockNeverCallsTheObservationWriter` (spy, real Postgres) | `go test ./app/internal/ingestion/... -run TestIngestSeries_XLSXPartialSuccess` (real Postgres, real HTTP fixture, the sharpest case end-to-end) | revert `app/internal/adapters/xlsx/malformed_test.go`, `app/internal/adapters/xlsx/testdata/README.md`, `app/internal/adapters/postgres/gate_spy_test.go`, `app/internal/ingestion/malformed_xlsx_test.go`, and the additive `ObservationWriterPort`/`ApplyGateWithWriter` seam in `app/internal/adapters/postgres/gate.go` (`ApplyGate`'s own exported signature is unchanged) |
| 15 | Scheduler + synthetic daily probe + structured logging + alerting (9.1-9.8, DONE) | PR 9a | `go test ./app/internal/scheduler/... ./app/internal/probe/... ./app/internal/adapters/ine/... ./app/internal/ingestion/... ./app/internal/ingestion/pipelinelog/... ./app/internal/ingestion/alerting/...` (all offline, all pass) + `go test -tags live ./app/internal/probe/...` (scheduled only, NOT executed in this sandbox — no outbound network access, disclosed) | `scheduler_integration_test.go` runs one real scheduled cycle against a real `ine.Client` + the checked-in volume-restriction fixture (exactly 1 HTTP request, classification respected); `logging_alerting_test.go` runs `IngestSeries` end-to-end against real Postgres, capturing both the structured log line and the raised alert | revert `app/internal/scheduler`, `app/internal/probe`, `app/internal/ingestion/pipelinelog`, `app/internal/ingestion/alerting`, `.github/workflows/probe.yml`, the additive `ProbeURL`/`FetchProbe` on `ine.Client`, and `ingest.go`'s `logAndAlertRun` call sites (its own exported `IngestSeries` signature is unchanged) |
| 16 | Structured logging, alerts, attribution table closure | PR 9b | `go test ./... -run Attribution` + `go test ./app/internal/ingestion/... -run Logging` | full `validate-config` run over the closed attribution table | revert logging/alerting code + final `config/sources/*.yaml` edits |

---

## Phase 1: Platform Runtime (Milestone 0.1)

- [x] 1.1 Create `go.mod` at repository root (`github.com/jorgealonsodev/concontexto`, ADR-5); stub root `config_embed.go`.
- [x] 1.2 RED: `app/cmd/concontexto/main_test.go` — each of `serve/ingest/migrate/validate-config/healthcheck` dispatches; unknown subcommand exits non-zero with usage.
- [x] 1.3 GREEN: implement `app/cmd/concontexto/main.go` dispatcher satisfying 1.2.
- [x] 1.4 RED: `app/internal/httpserver/static_test.go` — constructor takes no repository port; 100 served requests record zero DB queries; blocking outbound source HTTP still serves `/healthz` and pages.
- [x] 1.5 GREEN: implement `app/internal/httpserver` static handler + `/healthz` satisfying 1.4 (golden rule + no-external-call requirements).
- [x] 1.6 RED: import-graph guard test (`go list -deps`) — `app/internal/httpserver` does not transitively import `adapters/postgres` or `github.com/jackc/pgx`.
- [x] 1.7 GREEN: enforce package boundaries so 1.6 passes; add the guard as a CI step. (Go-side boundary already satisfied by 1.5's implementation; wiring the guard into `ci.yml` itself is task 1.17, PR 1b.)
- [x] 1.8 RED: `healthcheck_test.go` — shallow healthcheck exits 0; `--deep` exits 1 when Postgres unreachable, 0 when reachable; plain healthcheck unaffected by DB state.
- [x] 1.9 GREEN: implement `healthcheck` subcommand satisfying 1.8.
- [x] 1.10 RED: `cache_test.go` — hashed asset gets long `max-age`+`immutable`; non-hashed HTML does not.
- [x] 1.11 GREEN: implement cache-header logic in the Go handler satisfying 1.10.
- [x] 1.12 RED: `migrate` boot-safety test — `serve` start against a schema one migration behind applies zero migrations.
- [x] 1.13 GREEN: implement `migrate up|down|status` subcommand skeleton; confirm `serve` never invokes it, satisfying 1.12.
- [x] 1.14 Write `Dockerfile` — distroless multi-stage, `CGO_ENABLED=0 -trimpath`, repo-root build context, exec-form `HEALTHCHECK`. No unit-testable surface; proven by 1.21's smoke test.
- [x] 1.15 Write `docker-compose.yml` — 256 MB/512 MB limits, external `proxy` + internal networks, no published ports, `json-file` logging. No unit-testable surface; proven by 1.21.
- [x] 1.16 Write `CODEOWNERS` + branch-protection config requiring two approvals on `/config/**`. Verified operationally (7.14), not by `go test`.
- [x] 1.17 Write `.github/workflows/ci.yml` (`go test ./...`, `validate-config` stub, frontend build) + deploy step. The CI job itself is the test.
  **CORRECTION (remediation 2026-07-29, sdd-verify CRITICAL C5):** the original "+ deploy step" claim was false — `ci.yml` shipped exactly three jobs (`go`, `web`, `container-smoke-test`) and no deploy automation of any kind existed anywhere in the repository (the same "marked `[x]`, deliverable absent" failure mode already disclosed once for task 1.21). This is now split honestly rather than silently left wrong: `.github/workflows/deploy.yml` is a real, correctly structured deploy workflow (build+push to `ghcr.io`, trigger a Portainer stack redeploy webhook), explicitly gated on `secrets.PORTAINER_WEBHOOK_URL` — it builds and pushes the image either way but performs zero redeploy action and emits a visible `::warning::` when that secret is absent, instead of reporting a fake green deploy. **PRD §17's milestone-0.1 exit criterion ("automated deploy of a hello world") remains genuinely UNMET**: this environment has no VPS/Portainer target and no secret configured to reach one. See `docs/deploy.md` for exactly which secret and infrastructure are required to close it. Kept `[x]` because the workflow itself is now correctly built, reviewable and honestly self-disclosing — not because the exit criterion is met.
- [x] 1.18 Add minimal `web/` hello-world Astro artifact (no components, no CSS framework) for the 0.1 deploy scenario.
- [x] 1.19 Write `scripts/backup/pg_dump.sh` (docker-exec dump) + document the hot-copy-is-unsafe requirement. Proven by a dump+restore smoke test, not a Go unit test.
- [x] 1.20 Add `LICENSE` (MIT). Doc-only.
- [x] 1.21 CI smoke test: build image, run compose, `curl /healthz` expects 200, exercise `healthcheck` inside the container — primary verification for 1.14–1.19. **Corrective batch (post-9.11):** originally marked `[x]` after a human ran this sequence by hand ONCE in a PR review session and reported the real output, but it was never automated — `.github/workflows/ci.yml` had no docker/compose/curl step, a gap `TestFase0ClosureGate`'s 0.1 subtest found and logged (9.11) without failing the build. Now delivered for real: `scripts/smoke-test.sh` (runnable locally, `set -euo pipefail`, distinct `COMPOSE_PROJECT_NAME`, tears down everything including the `proxy` network) covers every item in this task plus the memory-limit/no-published-port/log-rotation/pg_dump-round-trip guarantees PRD §14.3 makes; wired into `.github/workflows/ci.yml` as its own `container-smoke-test` job; `TestFase0ClosureGate`'s 0.1 subtest now asserts (not logs) that the script exists, is executable, and that `ci.yml` references it — a regression guard against this exact defect recurring silently. Run for real against Docker 29.6.2 / Compose v5.3.1 during this batch: all 15 steps passed.

## Phase 2: Data Model & Vintages (Milestone 0.4)

- [x] 2.1 Set up testcontainers-go harness: `TestMain` starts `postgres:17-alpine` per package, `testing.Short()` guard, tx-rollback isolation helper.
- [x] 2.2 RED: migration test — `migrate up` on an empty DB creates exactly the ten Fase-0 tables, none of the four excluded ones, and `event_group` (not `group`). Also includes the `is_current` partial-unique-index database-rejection assertion (schema-only half of 2.8/2.9; writer-dependent half stays PR 2b).
- [x] 2.3 GREEN: write `app/migrations/0001_fase0_schema.{up,down}.sql` (ten-table DDL) satisfying 2.2.
- [x] 2.4 RED: reversibility test — rolling back every migration to zero, one step at a time, leaves an empty schema.
- [x] 2.5 GREEN: verify/complete `down` migrations satisfying 2.4.
- [x] 2.6 RED: revision test — re-ingesting a changed value inserts `version = prior+1`, prior row intact, current resolves to new; identical resubmission creates no row.
- [x] 2.7 GREEN: implement `ObservationWriter` version-insert + `is_current` flip in one transaction, satisfying 2.6.
- [x] 2.8 RED: invariant test — a second `is_current=true` row for the same `(series,period)` is rejected by Postgres with a unique violation; a one-transaction promotion leaves exactly one current row.
- [x] 2.9 GREEN: confirm `one_current_row` partial unique index + writer transaction satisfy 2.8 (extends 2.3/2.7).
- [x] 2.10 RED: provenance test — `observation → ingestion_run.raw_file_hash → raw_file.hash` resolves with all fields non-null; vintage-as-of-D resolves to the max version among runs at/before D.
- [x] 2.11 GREEN: implement `ingestion_run` writer + provenance query satisfying 2.10.
- [x] 2.12 RED: mapping test — closing a `series_source_mapping` (`valid_to`) and inserting a replacement keeps both rows queryable.
- [x] 2.13 GREEN: implement `series_source_mapping` repository methods satisfying 2.12.
- [x] 2.14 RED: withdrawal test — ingesting a withdrawal creates a new `status='W'` current version; prior version retained unchanged.
- [x] 2.15 GREEN: implement the tombstone-write path satisfying 2.14.
- [x] 2.16 RED: rollback test — rolling back a bad run re-points `is_current` to the prior version, records a reason, never deletes the raw file or hash.
- [x] 2.17 GREEN: implement the rollback operation satisfying 2.16.
- [x] 2.18 Wire `migrate up|down|status` (1.13) to the postgres adapter's embedded migration files.

## Phase 3: Config Structure, validate-config, Attribution Structure (0.7 structure)

- [x] 3.1 RED: config-loader test — a well-formed `series/{slug}.yaml` resolves slug, ≥1 validity-ranged source ref, unit, frequency, decimals.
- [x] 3.2 GREEN: implement `app/internal/adapters/config` typed loader satisfying 3.1.
- [x] 3.3 RED: static-scan test — no INE/Eurostat/XLSX origin identifier literal exists in Go source outside config-loading code and `testdata/`.
- [x] 3.4 GREEN: eliminate any hard-coded identifiers satisfying 3.3.
- [x] 3.5 RED: `config_embed_test.go` — the embedded `fs.FS` serves `/config`; on-disk mutation after build does not change what the running process reads.
- [x] 3.6 GREEN: implement root `config_embed.go` (`//go:embed config`) satisfying 3.5; wire the config adapter to consume it.
- [x] 3.7 RED: `validate-config` test — missing `unit` exits non-zero naming file+field; complete tree exits zero; unknown source reference exits non-zero naming it.
- [x] 3.8 GREEN: implement `validate-config` satisfying 3.7; wire into `ci.yml` (replaces 1.17's stub).
- [x] 3.9 RED: `sources/{source}.yaml` schema test — licence/attribution/access-type/redistribution all required; Eurostat source records Commission Decision 2011/833/EU acknowledgement-only scope + commercial restriction.
- [x] 3.10 GREEN: implement the source schema + licensing checks satisfying 3.9; author `config/sources/ine.yaml`, `config/sources/eurostat.yaml`.
- [x] 3.11 Write `LICENSE-DATA` (defers to per-source terms); assert `LICENSE`=MIT and no blanket-licence statement exists.
- [x] 3.12 RED: attribution test — resolving a published observation's attribution yields the source's configured text + origin series id + extraction timestamp.
- [x] 3.13 GREEN: implement attribution resolution satisfying 3.12.

## Phase 4: Validation Engine (§9.3)

- [x] 4.1 RED: purity test — identical inputs across two rule evaluations return identical verdicts; no I/O or clock access.
- [x] 4.2 GREEN: define `Rule`/`Finding`/`SeriesContext` in `app/internal/ingestion/validation` satisfying 4.1.
- [x] 4.3 RED: rule 1 (schema) — renamed value column fails naming the field; XLSX sheet/header/anchor checks included.
- [x] 4.4 GREEN: implement rule 1 satisfying 4.3.
- [x] 4.5 RED: rule 2 (continuity) — skipped period fails naming it; allowlisted gap passes; INE/Eurostat labels normalise.
- [x] 4.6 GREEN: implement rule 2 satisfying 4.5.
- [x] 4.7 RED: rule 3 (plausibility) — out-of-range fails; large jump at a recorded break passes; same jump without a break fails.
- [x] 4.8 GREEN: implement rule 3 (consumes resolved `Breaks`) satisfying 4.7.
- [x] 4.9 RED: rule 4 (revision) — within default N=4 passes; 9-back fails + human-review block; per-series N=12 override passes the 9-back case.
- [x] 4.10 GREEN: implement rule 4 satisfying 4.9.
- [x] 4.11 RED: rule 5 (metadata completeness) — missing licence fails; nothing publishes.
- [x] 4.12 GREEN: implement rule 5 satisfying 4.11.
- [x] 4.13 RED: rule 6 (non-empty) — structurally valid empty payload fails, distinct from transport error; nothing written/published.
- [x] 4.14 GREEN: implement rule 6 satisfying 4.13.
- [x] 4.15 RED: gate test — failed run leaves current observation untouched, zero writes, recorded failed outcome+hash; all-pass publishes; rules-2-and-3 failure reports both.
- [x] 4.16 GREEN: implement `Gate(findings) → Publish|Block` wired to the slice-2 writer, satisfying 4.15.

## Phase 5a: Raw File Store + INE Client + Fixtures (0.2, part 1)

- [x] 5a.1 RED: filestore test — payload archived under SHA-256 with source/URL/timestamp before parsing; identical redownload does not duplicate; rollback leaves the file/hash unchanged.
- [x] 5a.2 GREEN: implement `app/internal/adapters/filestore` satisfying 5a.1.
- [x] 5a.3 RED: `download_attempt` test — unchanged redownload records success+existing hash; transport error records failure+null hash.
- [x] 5a.4 GREEN: implement `download_attempt` writer satisfying 5a.3.
- [x] 5a.5 RED: hash-listing test — `raw_files.sha256`/`/public/transparencia/raw-files.sha256` lists every archived file; recomputed hash matches.
- [x] 5a.6 GREEN: implement the raw-hash listing writer satisfying 5a.5; wire the copy-to-`/public` step.
- [x] 5a.7 RED: freshness test — 24h without success fails the source and propagates amber to its series; within-window stays fresh; one failing source does not affect another.
- [x] 5a.8 GREEN: implement per-source freshness resolution satisfying 5a.7.
- [x] 5a.9 RED: INE client test — ingesting `EPA453100` issues exactly one `DATOS_SERIE/EPA453100` request, zero `DATOS_TABLA` requests, against an `httptest.Server` fixture.
- [x] 5a.10 GREEN: implement `app/internal/adapters/ine` `DATOS_SERIE` client (ADR-2/D5) satisfying 5a.9.
- [x] 5a.11 RED: periodicity test — quarterly-configured series receiving annual periods fails naming expected/actual and writes nothing; matching periodicity proceeds.
- [x] 5a.12 GREEN: implement the periodicity assertion satisfying 5a.11.
- [x] 5a.13 RED: volume-restriction test — the envelope decodes to a named non-retryable error (not an opaque type error); a 5-attempt policy issues exactly one request; a 503-then-success sequence retries and succeeds.
- [x] 5a.14 GREEN: implement the discriminating decode + `sourceerr.FailureClass` taxonomy satisfying 5a.13; check in the envelope fixture with `source.txt`.
- [x] 5a.15 RED: period-normalisation test — `T1 2026` and `M06 2026` normalise to canonical quarterly/monthly `Period`.
- [x] 5a.16 GREEN: implement INE period normalisation satisfying 5a.15.
- [x] 5a.17 Confirm `go test ./...` runs the full INE suite offline against `testdata/` with no network access.

## Phase 5b: Six-Series Full Historical Load (0.2, part 2)

- [x] 5b.1 Author `config/series/{slug}.yaml` for the six pinned series (`tasa-de-paro-epa`, `ocupados-epa`, `ipc-general`, `ipc-subyacente`, `pib-cvi`, `poblacion-residente`) with periodicity + thresholds; run `validate-config`.
- [x] 5b.2 RED: config-scan test — `/config` references none of table `4247`, table `50902`, operation `72`, code `CP`.
- [x] 5b.3 GREEN: confirm 5b.1's configs satisfy 5b.2 (no code change expected).
- [x] 5b.4 Check in trimmed real `testdata/` fixtures (≈3 periods, `source.txt`) for all six series.
- [x] 5b.5 RED: e2e test — each of the six series loads full history, passes every applicable rule, and every observation carries source/COD/timestamp/hash.
- [x] 5b.6 GREEN: wire `IngestSeries` end-to-end for the six series satisfying 5b.5.
- [x] 5b.7 Investigate whether live EPA tables already reflect CNAE 2025 double-coding; if present, note for slice 7's `rupturas.yaml`.

## Phase 6: Eurostat Adapter (0.3)

- [x] 6.1 RED: JSON-stat test — a recorded fixture decodes into canonical observations with normalised periods, through the same writer/validation harness as INE.
- [x] 6.2 GREEN: implement `app/internal/adapters/eurostat` decoder + Normalize satisfying 6.1.
- [x] 6.3 Author `config/series/*.yaml` for `prc_hicp_minr` (geo=ES/unit=RCH_A/coicop18=TOTAL), `une_rt_q`, `nama_10_gdp` with full dimension pinning.
- [x] 6.4 RED: e2e test — the three datasets load/validate; config-scan finds no `prc_hicp_manr`/`prc_hicp_midx`/dimension `coicop` for `prc_hicp_minr`.
- [x] 6.5 GREEN: wire the three datasets through `IngestSeries` satisfying 6.4.
- [x] 6.6 RED: pinning test — `prc_hicp_minr` config missing `coicop18` fails naming it; fully pinned `une_rt_q` passes.
- [x] 6.7 GREEN: extend `validate-config` (slice 3) with the every-dimension-except-time check satisfying 6.6.
- [x] 6.8 RED: ceiling test — a body larger than the configured ceiling aborts before full buffering, fails with a named error, memory stays within ceiling+buffer.
- [x] 6.9 GREEN: implement the `io.LimitReader`-based ceiling satisfying 6.8.
- [x] 6.10 RED: zero-observation test — HTTP 200 valid JSON-stat with `"value": {}` fails as the distinct zero-observation class; nothing written/published.
- [x] 6.11 GREEN: confirm rule 6 (slice 4) + adapter classification satisfy 6.10; add the dead-code (`coicop18=CP00`) fixture.
- [x] 6.12 RED: maintenance test — 2h-old-success unavailability retries with no incident; >24h escalates to failure+incident.
- [x] 6.13 GREEN: extend the 5a.7 freshness/backoff handling for Eurostat maintenance satisfying 6.12.
- [x] 6.14 RED: probe-param test — a probe request carries `lastTimePeriod=1` plus every pinned filter.
- [x] 6.15 GREEN: implement `lastTimePeriod=N` support satisfying 6.14.

## Phase 7: Editorial YAML + Reconcile + Four-Eyes (0.5)

- [x] 7.1 Author `config/rupturas.yaml`, `config/eventos.yaml`, `config/gobiernos.yaml` — PRD §9.6 minimum list + ECOICOP v2/January 2026 break (scoped to every CPI-derived series), each entry with a stable `id`. Three entries (EPA CNAE2025, ECOICOP v2 ×2, CN base sept-2025, SEC changes, SS CNAE2025 — 6 of 9 break entries, plus 1 of 5 event entries) ship with `date_status: unconfirmed` + `todo` instead of a guessed date; see PR 7a's Deviations.
- [x] 7.2 RED: duplicate-id test — two entries sharing an `id` fail `validate-config` naming the duplicate.
- [x] 7.3 GREEN: implement duplicate-id validation satisfying 7.2 (`app/internal/adapters/config/validate.go`, checked across eventos.yaml+gobiernos.yaml together since `event.id` is one shared PK).
- [x] 7.4 RED: in-place-update test — editing only a break's description updates the same row, changes `config_digest`, records no retirement.
- [x] 7.5 GREEN: implement `config_digest` (SHA-256 of source YAML) + update-in-place path satisfying 7.4 (`app/internal/ingestion/reconcile.go` digest computation, `app/internal/adapters/postgres/editorial.go` in-place update).
- [x] 7.6 RED: scope test — a CPI-family-scoped break resolves for every member series, stored once.
- [x] 7.7 GREEN: implement `series_break` scope resolution satisfying 7.6 (`postgres.ResolveActiveBreaksForSeries`).
- [x] 7.8 RED: full-reconcile test — transactional + idempotent: removal soft-retires; identical re-run changes zero rows; a mid-transaction failure leaves the DB byte-identical to pre-reconcile; reverting YAML restores the entry with retirement history intact.
- [x] 7.9 GREEN: implement `ReconcileEditorialConfig` (transactional, soft-retire, no hard delete) satisfying 7.8 (`app/internal/ingestion/reconcile.go` + `postgres.ReconcileBreaks`/`ReconcileEvents`).
- [x] 7.10 RED: non-dismissible test — the `series_break` schema/domain type exposes no dismissible/optional/default-hidden attribute. **PR 7b.** Mutation-tested: `Dismissed bool` temporarily added to `SeriesBreakInput`, confirmed FAIL, reverted.
- [x] 7.11 GREEN: confirm/adjust the domain type satisfying 7.10. **PR 7b.** No production change needed — `SeriesBreakInput`/`SeriesBreak`/`indicators.Break` already carry no such field; the test is a permanent regression guard.
- [x] 7.12 RED: ECOICOP test — after reconcile, the v2/January-2026 break is present for any CPI-derived series with a link to the methodological note. **PR 7b.** Dates for `ecoicop-v2-2026-ine-ipc`/`ecoicop-v2-2026-eurostat-hicp` confirmed 2026-01 by the orchestrator (Commission Delegated Regulation (EU) 2024/3159); scope re-narrowed from "every CPI-derived series" to COICOP-subclass-level disaggregations only, EXCLUDING the headline aggregates (ipc-general, ipc-subyacente, ipc-armonizado-eurostat) that INE states are linked with unmodified variation rates. Test written and confirmed genuinely failing against the pre-edit rupturas.yaml before the YAML was updated.
- [x] 7.13 GREEN: confirm 7.1's entry satisfies 7.12 end-to-end. **PR 7b.** `config/rupturas.yaml` updated (dates, narrowed scope, source citations); `TestReconcileEditorialConfig_ECOICOPv2BreakIsConfirmedAndScopedToDisaggregationsNotHeadlineAggregates` passes against the real embedded config.
- [x] 7.14 Wire branch protection (1.16) against real `/config/**` paths; verify operationally that a single-approval PR touching `rupturas.yaml` is blocked. **PR 7b.** Added structural tests (`app/internal/guard/codeowners_test.go`) proving the CODEOWNERS `/config/**` rule requires ≥2 distinct owners and BRANCH_PROTECTION.md documents the required settings (mutation-tested: single-owner CODEOWNERS confirmed FAIL, reverted). Operational GitHub verification (blocking a real single-approval PR) remains BLOCKED: `@TODO-second-config-reviewer` is still a placeholder (needs a real second maintainer per PRD §18) and the repository has zero commits/no remote to open a PR against. Not fabricated — disclosed plainly.
- [x] 7.15 Wire `ReconcileEditorialConfig` into the `ingest` subcommand as its own step. **PR 7b.** `ingest --reconcile` implemented in `app/cmd/concontexto/ingest_cmd.go` (replaces the `stubs.go` placeholder for that one flag); proven against a real Postgres container, run twice for idempotence.

## Phase 8: XLSX Parser — Social Security Affiliation (0.6)

- [x] 8.1 **INVESTIGATION (blocking — do first, no code)**: download and inspect a real "Afiliación media mensual" workbook. RESOLVED 2026-07-28 (Engram #4699): the series file `19_Serie afiliación media por regímenes (Total Sistema).xlsx` (54 KB, 1 sheet `Hoja1`, 0 formula cells) is the ingestion source; the 25-sheet `Afiliación 2026_CNAE25.xlsx` (INDICE selector, 12,826 formulas) is NOT used. **Correction made during PR 8a implementation**: direct `excelize` verification (`GetCellValue` on explicit cell refs, cross-checked against `GetRows`) found the real total column is **N**, not K as Engram #4699 recorded, and the arithmetic invariant only holds across the full pre-/post-2012 régimen history when `component_columns` spans B-M (not merely B-I). Recorded correctly in `config/series/afiliacion-ss.yaml`. **Correction batch (Work Unit 13c) found and refuted a THIRD claim**: Engram #4709 asserted the total column itself moves across five overlapping eras (N/M/L/J/K), sourced from the same sha256-matched fixture. Exhaustive re-verification of all 306 rows (not a sample) refutes this: column N is the max of B..N — and therefore the total — in literally every row, matching the workbook's own row-2 header label "TOTAL SISTEMA" for N. The genuine defect the sweep found was a tolerance value (Julio 2013 rounds to a 3.03 discrepancy; `Tolerance=1.0` rejected it), fixed by widening to `5.0`. See `app/internal/adapters/xlsx/decode.go`'s package doc comment for the full evidence trail and `decode_test.go`'s `TestDecode_FullHistoryEveryRowSatisfiesArithmeticInvariant`, which now checks every row against the full, untrimmed real fixture.
- [x] 8.2 RED: `validate-config` test — XLSX config with no sheet name or header fingerprint fails naming the missing fields. `app/internal/adapters/config/validate_test.go` (`TestValidate_XLSXSeriesWithNoSheetNameOrHeaderFingerprintFailsNamingTheMissingFields`); confirmed failing before 8.3.
- [x] 8.3 GREEN: implement XLSX `schema:` validation (extends slice 3) satisfying 8.2. `app/internal/adapters/config/validate.go` (`validateXLSXSchema`); also validates `total_column`/`component_columns` (the arithmetic-invariant guard).
- [x] 8.4 RED: config-only-structure test — a column-anchor change reflected only in config ingests successfully with no Go source change. `app/internal/adapters/xlsx/decode_test.go` (`TestDecode_ConfigOnlyColumnAnchorChange`).
- [x] 8.5 GREEN: implement `app/internal/adapters/xlsx` structure-from-config reader (excelize) satisfying 8.4. `app/internal/adapters/xlsx/decode.go`, `period.go`, `fingerprint.go`.
- [x] 8.6 RED: selector-independence test — a selector-dependent visible cell for a non-requested month is not read as truth; unresolvable month fails the run and writes nothing. Implemented as a genuine property of the CHOSEN source (0 formula cells), per orchestrator instruction, not a defence against a hazard the ingested file does not have: `TestDecode_SelectorIndependenceRealFixtureHasNoFormulaCells` + `TestDecode_UnresolvablePeriodFailsAndWritesNothing`.
- [x] 8.7 GREEN: implement month-targeting logic bypassing cached selector-dependent formulas satisfying 8.6. Satisfied by `decode.go` reading `RawCellValue` (never a formula-cached display value) plus the terminator/period-parse-failure path returning zero observations.
- [x] 8.8 RED: happy-path test — a real monthly fixture parses into canonical observations with full provenance and passes every applicable rule. `app/internal/adapters/xlsx/decode_test.go` (`TestDecode_HappyPath`) against the checked-in trimmed real fixture.
- [x] 8.9 GREEN: wire the XLSX adapter through `IngestSeries` satisfying 8.8; check in the trimmed real fixture + `testdata/README.md`. `app/internal/adapters/xlsx/client.go` (`Client` satisfies `indicators.SourceClient`); `app/internal/ingestion/ingest.go` now threads `decoded.ObservedSchema` into `validation.SeriesContext` so rule 1 can compare it; `indicators.SourceResult` gained an additive `ObservedSchema` field. Fixture: `app/internal/adapters/xlsx/testdata/afiliacion-ss/afiliacion-ss.xlsx` + `source.txt` (no separate `testdata/README.md` this batch — matches the established ine/eurostat per-fixture `source.txt` convention; `testdata/README.md` is explicitly 8.10/8.11's own deliverable for the malformed suite). `config/sources/seg-social.yaml` + `config/series/afiliacion-ss.yaml` added; `go run ./app/cmd/concontexto validate-config` passes.
- [x] 8.10 RED: malformed-file suite (spy repository) — header renamed, header moved, sheet missing, text in value cell, `#REF!`, formula with no cached value, truncated file, invalid zip, partial-success-within-one-workbook: each asserts zero writes + unchanged published datum; unreadable cases additionally assert no panic. **PR 8b.** `app/internal/adapters/xlsx/malformed_test.go` (`TestDecode_MalformedFileSuite` table + `TestDecode_PartialSuccessWithinOneWorkbookWritesNothing`, 9 cases total, offline/no Docker) proves each fixture makes `Decode` fail for the named reason (confirmed non-vacuous: each case's exact error message was inspected during RED to confirm it fails via the right branch). The "zero write calls" interaction property is proven ONCE, generically — every case funnels through the identical `ApplyGate` Block branch — via a genuine RED→GREEN cycle (`ApplyGateWithWriter`/`ObservationWriterPort` temporarily removed, confirmed compile-failure, restored) in `app/internal/adapters/postgres/gate_spy_test.go` (`TestApplyGate_BlockNeverCallsTheObservationWriter`, a spy substituted via the new writer-injection seam). The "published datum unchanged, at its original version" state property is proven end-to-end against a real Postgres for the sharpest case (partial success) in `app/internal/ingestion/malformed_xlsx_test.go`, per the orchestrator's explicit design decision ("use both, because the spec asks for two different things").
- [x] 8.11 GREEN: implement malformed-input handling (validate-before-write, all-or-nothing transaction) satisfying 8.10; craft and document each fixture in `testdata/README.md`. **PR 8b.** No new production parsing logic was needed: `decode.go`'s existing row loop (built in PR 8a for the arithmetic-invariant/selector-independence requirements) already discards all accumulated observations and returns on the first row error — genuinely all-or-nothing already, confirmed by RED-first fixture construction rather than assumed. The one real production change this batch makes is `app/internal/adapters/postgres/gate.go`'s `ObservationWriterPort` extraction (`ApplyGate` now delegates to an unexported `applyGate` taking an injectable writer; `ApplyGate`'s own exported signature is unchanged, so no existing caller is affected) — the seam the spy test needed, per the orchestrator's design decision ("if a writer is a concrete struct, extract a minimal port"). Fixtures are built as Go source (`excelize`, matching decode_test.go's pre-existing `buildWorkbook` convention) rather than checked-in binary files — disclosed and justified in `testdata/README.md`, which documents every case in a table (what's broken, how it's built, why `Decode` rejects it).

## Phase 9: Synthetic Probe, Scheduler, Observability, Attribution Closure (0.7 closure)

- [x] 9.1 RED: scheduler test — 2-failures-then-success retries with backoff, no incident; >24h failure raises an incident + failed freshness (5a.7); a named non-retryable error issues exactly one request. **PR 9a.** `app/internal/scheduler/scheduler_test.go` (3 pure scenarios) + `scheduler_integration_test.go` (real `ine.Client` against the checked-in volume-restriction fixture, runtime harness).
- [x] 9.2 GREEN: implement `app/internal/scheduler` per-source runner satisfying 9.1. **PR 9a.** `Runner.Run` calls its `op` callback exactly once per scheduled tick (retryability is never re-decided — every adapter's own `fetchWithRetry` already owns that), resolves the 24h incident boundary by calling `freshness.Resolve`/`State.RaisesIncident()` (5a.7/6b) rather than forking it, and reports `NextAttemptAt` via an injectable, increasing `Backoff` for a driving loop (not built this batch) to consult between ticks.
- [x] 9.3 RED: probe test — the `live`-tagged suite issues INE `nult=1`/Eurostat `lastTimePeriod=1` against every configured endpoint, asserts shape only; default `go test ./...` runs zero probe tests and makes no network call. **PR 9a.** `app/internal/adapters/ine/probe_test.go` (genuine RED→GREEN: confirmed compile failure before `ProbeURL`/`FetchProbe` existed) + `app/internal/probe/targets_test.go` (genuine RED→GREEN: confirmed "no non-test Go files" build failure before `Targets` existed) + `app/internal/probe/live_test.go` (`//go:build live`, compiled via `go vet -tags live` but NOT executed against real network — no outbound network access in this sandbox, disclosed rather than fabricated).
- [x] 9.4 GREEN: implement the shared `live`-tagged probe suite satisfying 9.3, reusing tier-2 clients from 5a/6; wire the scheduled `probe.yml` (1.17). **PR 9a.** Added `ine.Client.ProbeURL`/`FetchProbe` (mirrors `eurostat.Client`'s own 6.14/6.15 pair exactly); `probe.Targets` resolves every active `ine-series-cod`/`eurostat-dataset` source_ref from an already-loaded `*config.Config` (never a Go literal — xlsx-url is out of scope, no "last period only" query parameter exists for it); `live_test.go` decodes each probe response through the SAME adapter `Decode`/`DecodeSeries` production ingestion uses, asserting shape/periodicity, never values. `.github/workflows/probe.yml` added as its own scheduled (+ `workflow_dispatch`) workflow, separate from `ci.yml`.
- [x] 9.5 RED: logging test — completed-run logs carry run id/source/dataset/series/outcome/verdicts/hash/duration; failed runs also record which rules failed. **PR 9a.** `app/internal/ingestion/pipelinelog/pipelinelog_test.go` (pure builder, authored alongside its implementation — disclosed, not test-first) + `app/internal/ingestion/logging_alerting_test.go`'s completed/failed-run cases (genuine RED→GREEN: confirmed `0` captured log records against real Postgres before `IngestSeries` was wired to `slog`).
- [x] 9.6 GREEN: implement structured logging across the pipeline satisfying 9.5. **PR 9a.** `app/internal/ingestion/pipelinelog` (pure `Entry`/`Attrs`/`Verdicts`/`FailedRules` builder) wired into `IngestSeries` via `slog.Default()` (a process-wide default, not a new exported parameter — every existing caller of this already-widely-used function keeps compiling unchanged); a real wall-clock `startedAt` measures only the log `Duration` metric, never a decision (documented distinction from the "explicit time parameter" rule, which applies to freshness/scheduler DECISIONS).
- [x] 9.7 RED: alert test — a validation-failed run alerts naming source/series/failing rules while the previously published datum is still served. **PR 9a.** `app/internal/ingestion/alerting/alerting_test.go` (pure, authored alongside its implementation — disclosed) + `logging_alerting_test.go`'s failed-run case (genuine RED→GREEN, same cycle as 9.5's wiring test) + `scheduler_test.go`'s incident scenario (spy `Sink` asserts a `KindSourceDown` alert naming the source).
- [x] 9.8 GREEN: implement alert emission (wired to the slice-4 gate + slice-5a freshness) satisfying 9.7. **PR 9a.** `app/internal/ingestion/alerting`: `Sink` interface + `NoopSink` + `LogSink` (the genuine default — an ERROR-level structured log line, not a discarded no-op, needing no new env var) + process-wide `DefaultSink`/`SetDefaultSink`. `IngestSeries` calls `alerting.ValidationFailed` on every `GateBlock` outcome (slice-4 gate); `scheduler.Runner.Run` calls `alerting.SourceDown` the moment `freshness.State.RaisesIncident()` becomes true (slice-5a/6b freshness) — both wired at the exact point each fact is first known, never re-derived. A third named trigger ("failed ingestion" short of the 24h incident window) is deliberately NOT paged separately this batch — already independently auditable via `download_attempt` (spec raw-file-archive); disclosed, not silently dropped.
- [x] 9.9 RED: closure test — every source referenced by any configured series (INE/Eurostat/XLSX) has complete licensing; `validate-config` fails naming any gap; a series with no resolvable licence fails rule 5 at the gate. **PR 9b.** `app/internal/adapters/config/licensing_test.go`: `TestClosure_IneRecordsSpanishPublicSectorReuseFramework` (genuine RED: INE's `conditions_md` did not cite Ley 37/2007) + `TestClosure_SegSocialRecordsSpanishPublicSectorReuseFrameworkWithAttributionRequired` (genuine RED: `redistribution.acknowledgement_required` defaulted to `false`, contradicting the source's own conditions_md) + `TestClosure_EveryConfiguredSeriesResolvesACompleteLicenceAndPassesRule5` (walks every real `config/series/*.yaml` entry, resolves its source's licence, and runs it through the real `validation.Rule5MetadataCompleteness` — the closure proof that "a series with no resolvable licence fails rule 5" holds against the ACTUAL config, not only a synthetic fixture). `validate-config` failing on a gap and rule 5 blocking on a missing licence were already covered generically by Phase 3 (`TestValidate_SourceMissingLicenceFieldFails`) and Phase 4 (`TestRule5MetadataCompleteness_MissingLicenceFailsAndNothingPublishes`) — not duplicated here.
- [x] 9.10 GREEN: close the attribution table — finalize `config/sources/{ine,eurostat,seg-social}.yaml` satisfying 9.9. **PR 9b.** `ine.yaml`: `conditions_md` now cites Ley 37/2007 alongside INE's own aviso legal. `seg-social.yaml`: added `redistribution.acknowledgement_required: true` + `third_party_excluded: false`; corrected a stale comment claiming the §9.4 probe (PR 9a) is the URL-rotation early warning — `probe.Targets` deliberately excludes every `xlsx-url` ref (no "last period only" query parameter exists to probe with), so the `source_refs` validity range is the only early-warning mechanism today, checked administratively, not live. `eurostat.yaml` needed no change (already complete since Phase 3).
- [x] 9.11 Verify all seven milestone exit criteria pass as automated tests (except 0.1's deploy check, which is CI + container smoke); run full `go test ./...` + `validate-config` as the Fase-0 closure gate. **PR 9b.** `app/cmd/concontexto/fase0_closure_test.go`'s `TestFase0ClosureGate` — one subtest per PRD §17 criterion (0.1-0.7): asserts directly where cheap/offline (series/source counts, registry population, attribution completeness), names the exact existing test as proof where re-asserting would duplicate a Postgres/fixture cost already paid elsewhere in `go test ./...` (0.2, 0.3, 0.4, 0.6). 0.5's subtest logs the real count: 7 of 21 break/event entries are `date_status: unconfirmed`. **Corrective batch (post-1.21, verify-report WARNING W8):** this entry originally said the 0.1 subtest "logs — without failing the build — that `ci.yml` contains no docker build/compose/curl step"; that was true when 9.11 was first written but is now STALE — task 1.21's corrective batch changed the 0.1 subtest to ASSERT (not log) that `scripts/smoke-test.sh` exists, is executable and is wired into `ci.yml`, matching 1.21's own corrected text. Left inconsistent by the earlier batch; corrected here.

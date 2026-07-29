# Design — phase-0-data-foundations

Change: `phase-0-data-foundations` · Project: **ConContexto** · Store: hybrid.
Inputs: proposal.md, exploration.md (§2–§5), PRD §9/§10/§13/§14, Engram #4690 (INE verified), #4692 (Eurostat verified).
The stack is decided (PRD §14, "Estado: decidida") and is not re-evaluated here. This document designs *inside* it.

## Technical Approach

One Go binary, five subcommands, hexagonal layout. The database is an append-only projection: observations are immutable per version, provenance flows through `ingestion_run` to `raw_file`, and editorial YAML is reconciled — never hand-edited — into `series_break` and `event`. Validation is five pure functions plus a gate; a blocked run records its audit trail but writes zero observations, so the previously published datum keeps being served by construction. The golden rule (zero DB queries at request time) is enforced in the import graph, not in prose.

## Architecture Decisions (ADRs)

### ADR-1 — Configuration compiled in with `go:embed` (settled D4; consequences designed here)

**Choice**: all `/config` YAML is embedded into the binary. Consequence: `go.mod` lives at the **repository root** (module `github.com/jorgealonsodev/concontexto`), because `//go:embed` patterns cannot escape the embedding package's directory — a module rooted at `/app` could never embed `/config`. A single root-level file `configdata/embed.go` (directory `configdata/` at root re-exporting `//go:embed` of `config/...` via a root shim `config_embed.go`) exposes `embed.FS`; all other Go code stays under `/app/...`.
**Alternatives considered**: (a) mounted volume — rejected: config version drifts from binary version, breaking P2 traceability and §9.4's "change by reviewed PR"; (b) module in `/app` plus a `go:generate` copy of `/config` into the module with a CI drift check — rejected: duplicated config in the repo and a new failure mode (stale copy) for zero benefit.
**Rationale**: embed pins config version to binary version atomically — a `git revert` of a bad editorial edit reverts data and code together. The rebuild-per-edit cost is real but already paid: Portainer "Pull and redeploy" is the deploy path and `/config/**` already requires a four-eyes PR. Tradeoff accepted: an editorial typo fix requires a full image rebuild (~minutes), which is acceptable at a few-edits-per-month cadence and unacceptable to trade for provenance.
**Note**: `openspec/config.yaml`'s test command becomes `go test ./...` from the repo root, not `/app`.

### ADR-2 — INE ingestion via `DATOS_SERIE/{COD}` (settled D5; guard designed here)

**Choice**: one HTTP call per canonical series against `DATOS_SERIE/{COD}`. `DATOS_TABLA`/`SERIES_TABLA` are discovery tools only, never in the ingest path.
**Alternatives considered**: `DATOS_TABLA/{tableId}` per table — rejected: verified live (Engram #4690) to return HTTP 200 with body `{"status" : "No puede mostrarse por restricciones de volumen"}` — a JSON object where success is a JSON array — on wide tables; table Ids are containers, not identifiers (`ECP320` is identical in tables 56934 and 59238); fixtures would blow the 400-line review budget.
**Rationale**: the series COD is the stable identifier; blast radius per failure is one series; fixtures stay reviewable. The refusal envelope remains a first-class fixture because discovery tooling still touches `DATOS_TABLA`.

### ADR-3 — testcontainers-go over CI service containers

**Choice**: repository/SQL tests use testcontainers-go with `postgres:17-alpine` (the production image), guarded by `testing.Short()`, one container per package via `TestMain`, per-test isolation by transaction rollback. Domain and validation use in-memory fakes.
**Alternatives considered**: (a) GitHub Actions `services: postgres` — rejected: unusable on the dev laptop, so the milestone-0.4 exit criterion could only be proven in CI; version drift from prod image. (b) Fakes only — rejected: the `is_current` partial unique index and `MAX(version)` semantics only exist in real PostgreSQL; a fake proves nothing about milestone 0.4.
**Rationale**: 0.4's exit criterion is literally "the database itself rejects a second current row". Only a real PostgreSQL 17 can assert that. Cost (~2–5 s container start, Docker required) is confined to `-short`-skippable packages.

### ADR-4 — `is_current` with a partial unique index as a database-enforced invariant

**Choice**: `observation.is_current boolean` plus `CREATE UNIQUE INDEX ... ON observation(series_id, period) WHERE is_current`. Writer flips old current → false and inserts the new version in one transaction.
**Alternatives considered**: (a) compute current as `MAX(version)` in views only — rejected as the *sole* mechanism: nothing then prevents two rows claiming currency after a buggy writer, and 0.4 needs a constraint to assert against; (b) a separate `current_observation` table — rejected: duplicates data and reintroduces overwrite semantics that P7 forbids.
**Rationale**: the point is a database-enforced invariant ("exactly one current row per (series, period)"), not read speed — `MAX(version)` performance is a non-issue because it only runs at pre-render (golden rule). The index is the testable artifact of milestone 0.4.

### Supporting decisions

| Decision | Choice | Rejected | Why |
|---|---|---|---|
| Provenance grain | `observation.ingestion_run_id → ingestion_run.raw_file_hash → raw_file(hash)` | `raw_file_hash` per observation row | one raw file yields many observations; per-row hash duplicates without an FK and conflates grain (exploration §2) |
| Version semantics | `version` = per-`(series_id, period)` monotonic int; vintage = via run timestamps | one column for both | frozen-view-at-date-D = max version among runs with `started_at <= D`; revision counter satisfies §9.5 verbatim |
| Withdrawn periods | tombstone: new version with `status='W'`, `value NULL` | deleting rows | P7 — nothing is erased; pre-render treats a current `W` row as "period no longer published" |
| `series_break` scope | `scope_kind` (`series`\|`dataset`\|`source`) + `scope_ref` columns; stable `break_key` from YAML `id` | bare `series_id` FK | "ECOICOP v2 / Jan 2026" applies to a family; one `dataset`-scoped row replaces dozens; keeps D3's ten-table set |
| Reserved word | `event.event_group` | `event.group` | `group` is a SQL reserved word |
| Reconcile | transactional full reconcile, soft-retire (`retired_at`) + `config_digest` (SHA-256 of source YAML) per row | upsert-only; hard delete | removed entries must retire, not linger (idempotence) and not vanish (P7 audit trail) |
| Response ceiling | `io.LimitReader` per source (`max_response_bytes`, default 8 MiB) | trust the source | Eurostat served 157 MB for an unfiltered query (Engram #4692) against a 256 MB container |
| Error taxonomy | typed `FailureClass`: `RetryableTransport` / `SourceRefusal` / `SilentEmpty` / `SchemaDrift` / `ResponseTooLarge` | error strings | §9.2 retry/backoff must never loop on the non-retryable classes; `download_attempt.outcome` records the class |

## Schema (illustrative DDL, ten tables — settled D3)

```sql
CREATE TABLE source (
  id text PRIMARY KEY, name text NOT NULL, url text NOT NULL,
  license text NOT NULL, attribution_text text NOT NULL, access_type text NOT NULL,
  config_digest text NOT NULL, retired_at timestamptz);

CREATE TABLE dataset (
  id text PRIMARY KEY, source_id text NOT NULL REFERENCES source,
  name text NOT NULL, refresh_calendar text,
  config_digest text NOT NULL, retired_at timestamptz);

CREATE TABLE series (
  id text PRIMARY KEY,                      -- = slug
  dataset_id text NOT NULL REFERENCES dataset,
  name text NOT NULL, unit text NOT NULL, frequency text NOT NULL,  -- 'M'|'Q'|'A'
  geo text NOT NULL, decimals int NOT NULL, is_harmonized bool NOT NULL,
  config_digest text NOT NULL, retired_at timestamptz);

CREATE TABLE series_source_mapping (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  series_id text NOT NULL REFERENCES series,
  ref_kind text NOT NULL,                   -- 'ine-series-cod'|'eurostat-dataset'|'xlsx-url'
  ref text NOT NULL, ref_params jsonb,      -- e.g. pinned Eurostat filters
  valid_from date NOT NULL, valid_to date,  -- NULL = active
  config_digest text NOT NULL, retired_at timestamptz);
CREATE UNIQUE INDEX one_active_mapping ON series_source_mapping(series_id)
  WHERE valid_to IS NULL AND retired_at IS NULL;

CREATE TABLE raw_file (
  hash text PRIMARY KEY,                    -- sha256 hex
  source_id text NOT NULL REFERENCES source,
  url text NOT NULL, downloaded_at timestamptz NOT NULL,
  storage_path text NOT NULL, size_bytes bigint NOT NULL);

CREATE TABLE download_attempt (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  source_id text NOT NULL REFERENCES source,
  url text NOT NULL, attempted_at timestamptz NOT NULL,
  resulting_hash text REFERENCES raw_file,  -- NULL on failure
  outcome text NOT NULL);                   -- 'new-file'|'unchanged'|'retryable-transport'|
                                            -- 'source-refusal'|'silent-empty'|'schema-drift'|'response-too-large'

CREATE TABLE ingestion_run (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  dataset_id text NOT NULL REFERENCES dataset,
  series_id text REFERENCES series,         -- set for per-series fetches (INE)
  started_at timestamptz NOT NULL, finished_at timestamptz,
  raw_file_hash text REFERENCES raw_file,   -- NULL if fetch failed
  outcome text NOT NULL);                   -- 'succeeded'|'validation-failed'|'fetch-failed'|'nothing-new'

CREATE TABLE observation (
  series_id text NOT NULL REFERENCES series,
  period text NOT NULL,                     -- canonical: '2026-Q2'|'2026-06'|'2026'
  version int NOT NULL,                     -- per-(series,period) monotonic
  value numeric,                            -- NULL only for tombstones
  status text NOT NULL,                     -- 'P' provisional |'D' definitive |'W' withdrawn
  extracted_at timestamptz NOT NULL,
  ingestion_run_id bigint NOT NULL REFERENCES ingestion_run,
  is_current bool NOT NULL DEFAULT true,
  superseded_at timestamptz,
  rollback_reason text,                     -- see note below; migration 0002
  PRIMARY KEY (series_id, period, version),
  CHECK (value IS NOT NULL OR status = 'W'));
CREATE UNIQUE INDEX one_current_row ON observation(series_id, period) WHERE is_current;

**Amendment during implementation (slice 2b).** `observation.rollback_reason`
was not in this design's original DDL. The gap surfaced while implementing the
spec scenario "Rollback restores the prior current version", which requires a
recorded rollback reason to be queryable against the demoted row: `superseded_at`
records *when* a version stopped being current, but nothing recorded *why* when
the demotion was an operator rolling back a bad run rather than ordinary
supersession by a newer revision. Without it, a rollback and a normal revision
are indistinguishable after the fact — unacceptable under principle P7, which
requires errors to be documented rather than erased.

Applied as the additive migration `0002_observation_rollback_reason`, leaving
the settled ten-table set (decision D3) and the `0001` migration untouched.

CREATE TABLE series_break (
  break_key text NOT NULL,                  -- stable id from rupturas.yaml
  scope_kind text NOT NULL, scope_ref text NOT NULL,  -- 'series'|'dataset'|'source' + id
  date date NOT NULL, kind text NOT NULL, note_md text NOT NULL, source_url text,
  config_digest text NOT NULL, retired_at timestamptz,
  PRIMARY KEY (break_key, scope_kind, scope_ref));

CREATE TABLE event (
  id text PRIMARY KEY,                      -- stable id from YAML
  event_group text NOT NULL,                -- 'exogenous'|'governments'|'milestones'
  name text NOT NULL, date_start date NOT NULL, date_end date, note_md text,
  config_digest text NOT NULL, retired_at timestamptz);
```

Frozen view at date D: for each `(series_id, period)`, the max `version` among runs with `started_at <= D`. Never executed at request time — pre-render only.

## Go package layout

```
go.mod                          # module github.com/jorgealonsodev/concontexto (root — ADR-1)
config_embed.go                 # package configdata: //go:embed config/**/*.yaml → embed.FS
config/                         # sources/, series/, rupturas.yaml, eventos.yaml, gobiernos.yaml
app/
  cmd/concontexto/main.go       # dispatch: serve|ingest|migrate|validate-config|healthcheck
  internal/
    indicators/                 # DOMAIN — stdlib only: Series, Observation, Period, Vintage, Break, Event
      ports.go                  #   driven ports: SeriesRepo, ObservationWriter, RawFileStore, SourceClient
    ingestion/                  # APPLICATION: IngestSeries, ReconcileEditorialConfig, RunSyntheticProbe
      validation/               #   five PURE rules + Gate (no I/O, no ports)
      sourceerr/                #   FailureClass taxonomy shared by all source adapters
    publishing/                 # Fase 1 pre-render; Fase 0 stub + raw-hash listing writer
    adapters/
      ine/ eurostat/ xlsx/      # SourceClient implementations + Normalize → domain
      postgres/                 # pgx repos; owns migrate SQL via embed
      filestore/                # app_data raw store, sha256 naming, raw_files.sha256 index
      config/                   # parses configdata.FS → typed config + digests
    httpserver/                 # DRIVING: static /public + /healthz + cache headers — NO repo ports
    scheduler/                  # DRIVING: per-source cron per §9.2, backoff, 24 h amber escalation
  migrations/                   # NNNN_name.{up,down}.sql — applied ONLY by `migrate`
```

**Golden rule as a compile-time-visible invariant**: `httpserver` imports only stdlib and (at most) domain value types — no repository port, no pgx, no adapters. CI enforces it with a `go list -deps`-based test that fails if `app/internal/httpserver` transitively imports `app/internal/adapters/postgres` or `github.com/jackc/pgx`. "Zero DB queries at request time" is therefore falsifiable by the import graph, not a convention.

**Subcommands**: `serve` (static server + `/healthz` + in-process scheduler); `ingest [--source|--series]` (one-shot pipeline run); `migrate up|down|status` (the only path that touches schema — never on boot: auto-migrate races replicas and fires during rollbacks); `validate-config` (schema-validates embedded config; CI gate); `healthcheck [--deep]` (GET `127.0.0.1:8080/healthz`, exit 0/1; `--deep` also pings PostgreSQL).

## Validation: five pure rules + gate

```go
type Finding struct{ Rule string; Severity Severity; Period, Message string } // Info|Block|BlockRequiresSignoff
type Rule func(ctx SeriesContext, incoming []indicators.Observation) []Finding

type SeriesContext struct {
  Series indicators.Series          // unit, frequency, decimals, metadata incl. licence
  Config SeriesValidationConfig     // thresholds from series/{slug}.yaml
  Prior  []indicators.Observation   // current vintage before this run
  Breaks []indicators.Break         // pre-resolved by scope expansion (series ⊂ dataset ⊂ source)
}
```

| # | Rule | Per-series config | Blocks when |
|---|---|---|---|
| 1 | Schema & non-emptiness | required fields; XLSX sheet/header/anchors live in config, not code | missing/ill-typed fields, or **zero observations** (the silent-empty backstop) |
| 2 | Continuity | frequency, documented-gap allowlist | new undocumented gap; period not the expected successor |
| 3 | Plausibility | `min`, `max`, `max_delta_abs` | out of range or delta breach — **unless** period falls on a resolved break date |
| 4 | Revision consistency | `max_backward_periods` (default **4**, per-series override) | source revised > N prior periods → `BlockRequiresSignoff` (human review; may be legitimate CN revision) |
| 5 | Metadata completeness | — (reads series + source config) | series lacks source, unit, frequency or licence |

Callers pre-resolve `SeriesContext`; rules do no I/O, making them the natural home of the strict-TDD red/green loop. Period canonicalization (INE `T1 2026`/`M06` vs Eurostat `2026-Q1`/`2026-06`) happens in adapters into the domain `Period` type; rules never see source formats.

**Publish gate**: `Gate(findings) → Publish | Block`. On `Block`: the run is recorded with `outcome='validation-failed'`, the raw file and `download_attempt` persist (audit, P5), and **no observation row is written** — the transaction that appends versions and flips `is_current` never opens. The last valid datum keeps being served because serving reads pre-rendered output of untouched current rows. Alert emitted per §13 observability.

## Ingest data flow (sequence)

```
scheduler ──▶ ingestion.IngestSeries(slug)
   │  resolve active series_source_mapping from embedded config
   ▼
adapters/{ine|eurostat|xlsx}.Fetch          ── io.LimitReader(max_response_bytes)
   │  classify → RetryableTransport? backoff per §9.2; after 24 h → incident + AMBER (source-scoped)
   │            SourceRefusal | ResponseTooLarge | SchemaDrift → incident, NO retry
   ▼
filestore.Put(sha256) ─┬▶ [Tx1] download_attempt + raw_file + ingestion_run(pending)
   │                   └▶ append app_data/raw_files.sha256; copy to /public (§14.2 hash listing)
   ▼
adapter.Normalize → []Observation           ── "value": {} → SilentEmpty (never publishes quietly)
   ▼
validation.Run(ctx) → Gate
   │  Block ⇒ [Tx1'] outcome='validation-failed' — zero observation writes
   ▼  Publish
postgres.ObservationWriter [Tx2, atomic]:
   flip prior is_current=false + superseded_at; INSERT version = prior+1; is_current=true
   ▼
outcome='succeeded' ──▶ publishing stub (Fase 1: pre-render + cache invalidation)
```

Amber semaphore: per **source** — the newest `download_attempt` with a success outcome (`new-file`/`unchanged`) older than the source's expected cadence + 24 h turns the source amber, propagating to all its series. `unchanged` (same hash re-downloaded, or HTTP 304) is exactly why `download_attempt` exists apart from `raw_file`.

## Two opposite adapter guards + shared third

| Failure | Verified behaviour (2026-07-28) | Guard |
|---|---|---|
| INE **refuses** volume | `DATOS_TABLA` → HTTP 200, JSON **object** `{"status" : "No puede mostrarse por restricciones de volumen"}`; success is a JSON **array** | decode `json.RawMessage`; first non-space byte `{` → decode status envelope → `SourceRefusal` (named, non-retryable, first-class fixture) |
| Eurostat **serves** anything | unfiltered `prc_hicp_minr` → HTTP 200, **157 MB** vs 256 MB container | (1) `validate-config` REJECTS any Eurostat series whose `filters` do not pin **every** declared dimension except `time`; (2) client body ceiling via `io.LimitReader` → `ResponseTooLarge` |
| Silent empty success | dead code `coicop18=CP00` (ECOICOP v2 all-items is `TOTAL`) → HTTP 200, valid JSON-stat, `"value": {}` | adapter classifies `SilentEmpty` for `download_attempt`; rule 1 backstops: zero observations never publish |

Retry policy: backoff only on `RetryableTransport` (5xx, timeouts, Eurostat scheduled maintenance). All other classes fail fast to incident + amber. `SchemaDrift` covers renamed dimensions (`coicop`→`coicop18`), missing fields, and **periodicity mismatch**: the adapter compares source-reported frequency against the config assertion (tables 65962/72982 share 65109's name but are annual).

## `/config` file schemas (English keys; Spanish data preserved per PRD)

```yaml
# config/sources/ine.yaml
id: ine
name: "INE — Instituto Nacional de Estadística"
url: https://www.ine.es
access_type: api-json
api: { base_url: "https://servicios.ine.es/wstempus/js/ES", max_response_bytes: 8388608 }
licence:
  name: "Reutilización con atribución (condiciones INE)"
  url: "https://www.ine.es/aviso_legal"
  attribution_text: "Fuente: Instituto Nacional de Estadística (INE)"
  redistribution: { allowed: true, conditions_md: "...", commercial_restrictions_md: "..." }
```

```yaml
# config/series/tasa-de-paro-epa.yaml
slug: tasa-de-paro-epa
name: "Tasa de paro (EPA)"
source: ine
dataset: ine-epa
unit: "% población activa"
frequency: Q            # ASSERTED — adapter fails SchemaDrift on mismatch
decimals: 2
geo: ES
harmonized: false
source_refs:
  - { kind: ine-series-cod, ref: EPA453100, table_hint: 65349,
      valid_from: 2026-07-28, valid_to: null }     # churn → close valid_to, add new ref + rupturas entry
validation:
  plausibility: { min: 0, max: 40, max_delta_abs: 5 }
  continuity:   { documented_gaps: [] }
  revision:     { max_backward_periods: 4 }        # rule-4 N; default 4 when omitted
```

Eurostat refs add the pinning `validate-config` enforces:

```yaml
  - kind: eurostat-dataset
    ref: prc_hicp_minr
    dimensions: [freq, unit, coicop18, geo, time]            # full declared list
    filters: { freq: M, unit: RCH_A, coicop18: TOTAL, geo: ES }  # every dimension except time — REJECTED otherwise
    valid_from: 2026-07-28
    valid_to: null
```

XLSX series carry `schema:` (sheet name, header row, column anchors, header fingerprint) beside `source_refs` — expectations in config, never in Go code (§9.4).

Editorial files (names fixed by PRD): `config/rupturas.yaml` entries `{id, date, kind, scope: {kind, ref}, note_md, source_url}` — initial content includes the PRD §9.6 minimum list **plus the ECOICOP v2 / January 2026 break** (PRD correction); `config/eventos.yaml` and `config/gobiernos.yaml` map to `event` with `event_group` `exogenous|milestones` and `governments` respectively. `validate-config` checks: YAML schema, unique stable ids, every `scope.ref` resolves, every series has source/unit/frequency/licence (rule 5 at CI time), Eurostat full pinning, periodicity present.

## Testing strategy (Strict TDD active)

| Layer | What | Approach |
|---|---|---|
| Domain + validation | Period math, version semantics, the five rules, Gate | in-memory fakes, table-driven — **the red/green loop lives here** |
| Adapters (tier 1) | decode, classify, normalize | `testdata/` trimmed real fixtures (~3 periods) + `httptest.Server`; every fixture has `source.txt` (URL + fetch date); the INE refusal envelope, Eurostat `"value": {}` and an oversized-body case are first-class fixtures |
| Adapters (tier 2) | live contract, shape only, never values | build tag `live`; INE `nult=1`, Eurostat `lastTimePeriod=1`; scheduled CI job — **this test IS the §9.4 synthetic daily probe** (built once, used twice) |
| Repository/SQL | milestone 0.4 exit: revision creates version N+1, prior row intact, second current row **rejected by the database**; reconcile idempotence + soft-retire | testcontainers-go `postgres:17-alpine` (ADR-3), `testing.Short()` guard, `TestMain` container per package, tx-rollback isolation |
| XLSX | happy path + malformed suite (renamed header, missing sheet, `#REF!`, no cached formula value, truncated zip) | table-driven over checked-in `testdata/*.xlsx` documented in `testdata/README.md`; assertion is "**wrote nothing AND previously published datum unchanged**" via spy repository |
| Import-graph guard | golden rule | `go list -deps` test on `httpserver` |
| Milestone 0.1 | deploy + serve | honest: almost no unit-testable surface — the test is the CI job + a container smoke test (`healthcheck` + fetch hello-world). Do not manufacture ceremony tests for YAML. |

## Deployment design

- **Dockerfile**: multi-stage — `golang:1.x` builder (`CGO_ENABLED=0`, `-trimpath`, repo-root build context so embed sees `/config`) → `gcr.io/distroless/static-debian12:nonroot`. `HEALTHCHECK CMD ["/concontexto","healthcheck"]` — **exec form is mandatory: distroless has no shell**.
- **Compose** (Portainer Repository method): services `app` (256 MB limit) and `postgres` `postgres:17-alpine` (512 MB limit, `pg_isready` healthcheck, tuned per §14.2). Networks: external `proxy` (NPM reaches `app:8080` by name) and stack-internal `internal` (PostgreSQL only, invisible to NPM and host). **No service publishes ports.** Volumes: `public_html`, `app_data`, `pg_data`. Logging per service: `json-file`, `max-size: 10m`, `max-file: 3`, `compress: true`.
- **Backups**: ship `scripts/backup/pg_dump.sh` executed via `docker exec` on the postgres container (the app image has no pg_dump); documents §14.3's requirement that hot file-level copies of `pg_data` are unsafe and must be replaced/complemented by `pg_dump`. Wiring into the VPS backup system is a user-side action (out of scope, stated).
- **Raw-hash listing (§14.2 Fase 0 obligation)**: `filestore` maintains append-only `app_data/raw_files.sha256`; each ingest copies it into `/public/transparencia/raw-files.sha256`, served statically — published at ingest time, zero request-time work. `/data-derived` CSV is deferred to Fase 1 (settled).

## File Changes (all new — greenfield)

| Path | Description |
|---|---|
| `go.mod`, `config_embed.go` | module root + embed shim (ADR-1) |
| `app/cmd/concontexto/main.go` | subcommand dispatch |
| `app/internal/{indicators,ingestion,publishing,adapters,httpserver,scheduler}/...` | layout above |
| `app/migrations/*.{up,down}.sql` | ten-table schema, reversible |
| `config/{sources,series}/*.yaml`, `config/{rupturas,eventos,gobiernos}.yaml` | schemas above |
| `web/` | hello-world Astro artifact only (0.1) |
| `Dockerfile`, `docker-compose.yml` | deployment design above |
| `.github/workflows/ci.yml`, `.github/workflows/probe.yml`, `CODEOWNERS` | CI, scheduled tier-2 probe, four-eyes on `/config/**` |
| `scripts/backup/pg_dump.sh`, `LICENSE`, `LICENSE-DATA` | ops + D2 licensing |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary: a single static binary with in-process subcommands, no shell execution, and standard CI only.

## Migration / Rollout

Greenfield: no data migration. Schema applied only via `migrate up` per slice; every migration ships a reversible `down`. Rollback per proposal: revert PR → Portainer redeploy (embed makes config+code revert atomic); bad ingest → re-point `is_current` to the prior version, never delete.

## Open Questions

- [ ] Social Security XLSX file shape (single annual workbook with INDEX-sheet month selector?) — **blocks slice 8 spec**; confirm the real file before milestone 0.6.
- [ ] Do live EPA tables already reflect CNAE 2025 double-coding? Confirm during slice 5b; add to `rupturas.yaml` if present.
- [ ] `openspec/config.yaml` test command should be updated to repo-root `go test ./...` (consequence of ADR-1) — orchestrator to confirm.

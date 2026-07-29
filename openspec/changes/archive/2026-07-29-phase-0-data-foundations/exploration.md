# Exploration — phase-0-data-foundations

Change: `phase-0-data-foundations`
Scope: PRD Fase 0 "Fundaciones" (§17), milestones 0.1–0.7 only.
Status: complete (blocking product decisions listed in §8).
Artifact store: hybrid (Engram topic `sdd/phase-0-data-foundations/explore` + this file).

## 0. Current state

Repository confirmed greenfield: branch `main` with zero commits, no `go.mod`, no
`package.json`, no CI. Contents are the PRD, the `openspec/` scaffold,
`.atl/skill-registry.md` and `.gitignore`.

Architecture is DECIDED in PRD §14 (v2.2, "Estado: decidida") and was not
re-evaluated: Go single binary + PostgreSQL 17 + Astro/Svelte at build time only
+ hand-written CSS with design tokens. Utility CSS frameworks are forbidden.

**Scope boundary.** IN: milestones 0.1–0.7. OUT: all of Fase 1–4 — indicator
pages, search, international comparator, the Verificado module, glossary,
embeds, public API, design system, chart component and OG images.

## 1. External API reality check

Verified live against the real endpoints. No identifier was invented.

### 1.1 INE Tempus3

Endpoint shape confirmed exactly as PRD §8.1 documents:
`https://servicios.ine.es/wstempus/js/{idioma}/{función}/{input}[?parámetros]`.
No key, no registration.

Operations confirmed: EPA = Id 293 / `EPA`; IPC = Id 25 / `IPC`;
CNTR = Id 237 / `CNTR2010`; Estadística Continua de Población = Id 450 / `ECP`.

**PRD §7.3 row 17 is outdated.** "Cifras de Población" (Id 72 / `CP`) returns an
empty table list. The live quarterly population source is the Estadística
Continua de Población (`ECP`, Id 450).

All six milestone-0.2 series are now pinned:

| Series | Table Id | Series COD | Latest verified value |
|---|---|---|---|
| Tasa de paro (EPA) | 65349 | `EPA453100` | 2026-Q2 = 9.87 |
| Ocupados (EPA) | 65109 | `EPA387796` | 2026-Q2 = 22,779.0 (thousands) |
| IPC general (index) | 76125 | `IPC290751` | 2026-M06 = 103.598 |
| IPC subyacente | 76130 | `IPC292511` | 2026-M06 |
| PIB (chained volume index) | 67822 | `CNTR6721` | 2026-Q1 |
| Población residente | 59238 | `ECP320` | 2026-04-01 = 49,687,120 |

Cross-check: PRD §11.3's sample API response shows `2026-Q2 = 9.87` and
`2026-Q1 = 10.83` for `tasa-de-paro-epa`. The live series `EPA453100` returns
exactly those two values, which independently confirms the identifier.

Disambiguation resolved during exploration: tables 65962 and 72982 carry the
same `Nombre` as 65109 but hold **annual averages** (latest 2025-A), while 65109
is the **quarterly** series (latest 2026-Q2). Name alone cannot distinguish
them; periodicity must be asserted when pinning any identifier.

### 1.2 Two findings that change the ingestion design

**(a) `DATOS_TABLA` has an undocumented volume-restriction failure mode.**
`DATOS_TABLA/69792?nult=1&tip=A` returns HTTP 200 with the body:

```json
{"status" : "No puede mostrarse por restricciones de volumen"}
```

This is a JSON **object**, whereas the success case is a JSON **array**. A Go
client decoding straight into `[]Series` fails with an opaque type error instead
of a meaningful diagnosis. The INE adapter must decode into a discriminating
type and surface this envelope as a named error. It is not mentioned in PRD §8.1
and is not an HTTP-level error, so retry/backoff would loop forever on it.

**(b) The series COD is the stable identifier; the table Id is a container.**
`ECP320` resolves to the identical national-total population series in both
table 56934 (annual) and table 59238 (quarterly). `DATOS_SERIE/{COD}` was
verified live for `EPA453100`, `EPA387796`, `ECP320` and `IPC290751`: it returns
a single series with its observations, and **it is not subject to the volume
restriction** that blocks `DATOS_TABLA` on wide tables.

Recommendation: ingest via `DATOS_SERIE/{COD}`, one call per canonical series,
and use `DATOS_TABLA`/`SERIES_TABLA` only for discovery. This gives a smaller
blast radius per failure, avoids the volume restriction entirely, keeps fixtures
small enough to review, and matches the portal's one-canonical-series model.

### 1.3 Verified-stale identifiers

Every one of these is live in the PRD and dead in reality:

- INE table `4247` (EPA unemployment): last datum 2023-Q4, frozen.
- INE table `50902` (IPC ECOICOP v1): last datum 2025-12 on the old base
  (119.942 vs the new-base 103.598).
- Eurostat `prc_hicp_manr`: **discontinued**, replaced by `prc_hicp_minr`.
  `prc_hicp_midx` likewise.

Source identifiers churn at both levels — table Ids change *and* series CODs
change (`IPC251852` → `IPC290751`). PRD §10's single `dataset(origin_id)` column
cannot express this. Risk R1 is confirmed live, not theoretical.

### 1.4 Eurostat

Endpoint shape confirmed as PRD §8.2 documents, JSON-stat 2.0. Recommended
milestone-0.3 datasets: `une_rt_q` (live to 2026-Q1), `nama_10_gdp` (updated
2026-07-27), `prc_hicp_minr` (updated 2026-07-17, latest 2026-06).

`prc_hicp_minr` dimensions are `freq, unit, coicop18, geo, time` — the
dimension is **`coicop18`, not `coicop`**. Unit codes: `I25, I15, RCH_M, RCH_A,
RCH_MV12MAVR`.

**ECOICOP ver.2 / January 2026 transition.** ECOICOP 1 is archived and frozen;
from January 2026 data exists only under ECOICOP 2, and Eurostat warns of
potential January 2026 breaks. This break is **absent from PRD §9.6's minimum
break list** and must be added — it affects every CPI-derived page.

### 1.5 Not confirmed

- OECD SDMX: a constructed dataflow key returned 404, validating PRD §8.3's own
  warning that keys must come from the Data Explorer "Developer API" button.
- ECB Data Portal: HTTP 503 at check time.

Neither is needed for Fase 0.

## 2. Data model tension (PRD §10)

**`version` semantics are conflated.** Two incompatible needs share one column:
a per-`(series, period)` revision counter (satisfies §9.5's "this datum was 0.6%
then 0.7%") versus a run-level vintage (needed by §10's frozen-view permalink).
Recommend an explicit `ingestion_run(id, dataset_id, started_at, raw_file_hash,
outcome)`; keep `observation.version` as a per-`(series, period)` monotonic
integer and add `ingestion_run_id`. Current = `MAX(version)`; frozen-at-date D =
max version among runs at or before D. Also missing: no `superseded_at`, and no
way to represent a source *withdrawing* a period (needs a tombstone status).

**`MAX(version)` performance is a non-issue by design.** Per the golden rule
(§14.2) it never runs at request time — only at pre-render, a few times a day,
over a few thousand rows per series. Do not optimise it. But *do* add
`is_current boolean` with a partial unique index `(series_id, period) WHERE
is_current` — not for speed, but because it makes "exactly one current row per
period" a database-enforced invariant, which is exactly what milestone 0.4's
exit criterion needs to assert.

**`raw_file_hash` is at the wrong grain.** No FK is declared, and one raw file
yields many observations across many series, so the hash duplicates per row.
Route it through the run instead: `observation.ingestion_run_id →
ingestion_run.raw_file_hash → raw_file(hash)`.

Two further gaps: `raw_file` keyed by hash cannot record "we checked and nothing
changed", which the §6.1.1 amber freshness semaphore needs — split out
`download_attempt(source_id, url, attempted_at, resulting_hash, outcome)`. And
derived series (indicator 16, PIB per cápita) have no single raw file; the model
has no derived-provenance concept. Anticipate it, do not build it in Fase 0.

**Arrays should be join tables.** PostgreSQL cannot FK an array element, so a
typo'd slug in `indicator_page.series_ids[]` is undetectable until render —
unacceptable against Anexo E.1's "permalinks will never break". Arrays also
cannot carry a per-series role, which the PRD needs: indicator 25 shows mean
*and* median with distinct roles, indicator 16 needs a numerator and a
denominator. Use `indicator_page_series(page_slug, series_id, role, position)`
and `indicator_page_related(...)`. Join cost is irrelevant since nothing is
queried at request time.

Recommend **excluding `indicator_page`, `verification`, `glossary` and
`correction` from the Fase 0 migration set entirely** — they have no Fase 0
consumer.

**YAML → `series_break` / `event` sync.** YAML is authoritative and the database
is a projection, so the sync must be a transactional, idempotent **full
reconcile**, not upsert-only, or removed entries linger forever. But hard
deletes destroy the audit trail (P7), so use soft-retire plus a `config_digest`
(SHA-256 of the source YAML) per row.

`series_break` has no primary key in §10, and without a stable per-entry `id` in
the YAML an edited description is indistinguishable from delete+insert. A break
like "ECOICOP v2, January 2026" applies to a whole *family* of series, so
`series_break.series_id` would force dozens of duplicate rows — add a scope
column or a join table. In `event(id, group, …)`, `group` is a SQL reserved
word: rename to `event_group`.

Four-eyes review (§9.6, §15.2) is a **process** control, not a schema one:
CODEOWNERS plus branch protection requiring two approvals on `/config/**`,
wired in milestone 0.1 and consumed by 0.5.

## 3. Validation configuration surface (PRD §9.3)

| Rule | Configuration needed | Unit | Hard dependency |
|---|---|---|---|
| 1 Schema | expected fields and types; for XLSX: sheet name, header row index, column anchors, header fingerprint | per adapter/dataset | — |
| 2 Continuity | frequency, period-format convention (INE `T1 2026` / `M06` vs Eurostat `2026-Q1` / `2026-06`), next-period rule, documented-gap allowlist | per series | — |
| 3 Plausibility | absolute min/max, max period-over-period delta, exemptions keyed to break dates | per series | reads `rupturas.yaml` → **0.5 is a dependency of this rule** |
| 4 Revision consistency | how many periods may be revised backwards without sign-off; per-series revision profile | per series | needs a prior vintage → **depends on 0.4** |
| 5 Metadata completeness | required set: source, unit, frequency, licence | per series | **0.7 is a precondition for publishing anything** |

Proposed `/config/` layout:

- `sources/{source}.yaml` — identity, licence, attribution text, access type.
- `series/{slug}.yaml` — canonical slug, source references **with validity
  ranges**, unit, frequency, decimals, thresholds for rules 2–4.
- The three editorial files: `rupturas.yaml`, `eventos.yaml`, `gobiernos.yaml`.

XLSX schema expectations live beside the series config, never in Go code
(PRD §9.4). Config is schema-validated at start-up via a `validate-config`
subcommand that CI also runs.

**Open fork — embed vs mount.** `go:embed` pins config version to binary
version, which is what §9.4 and principle P2 traceability actually want; the
cost is a rebuild per editorial edit. Since Portainer "Pull and redeploy" is
already the deploy path and four-eyes is already a PR gate, the recommendation
is **embed**, recorded as an ADR.

The §9.4 synthetic daily probe belongs in Fase 0 — it is the early warning for
exactly the identifier churn that §1.3 proves is already happening.

## 4. Go project structure

```
/app
  cmd/concontexto/main.go     # serve | ingest | migrate | validate-config | healthcheck
  internal/
    indicators/               # DOMAIN: Series, Observation, Period, Vintage, Break, Event + driven ports
    ingestion/                # APPLICATION: IngestSeries, ReconcileEditorialConfig, RunSyntheticProbe
      validation/             # the five rules as PURE functions over domain types
    publishing/               # APPLICATION: pre-render (Fase 1; stub in Fase 0)
    adapters/{ine,eurostat,xlsx,postgres,filestore,config}
    http/                     # DRIVING: static server + internal API + /healthz
    scheduler/                # DRIVING: job runner
```

The three responsibilities are three **driving adapters over one domain**;
`serve` and `ingest` are two entry points into the same core, so splitting into
one process or two later is a compose change, not a refactor.

`healthcheck` (§14.3): the binary re-invoked as `healthcheck` performs an HTTP
GET to `127.0.0.1:$PORT/healthz` and exits 0 or 1. **Distroless has no shell, so
the Docker `HEALTHCHECK` must use exec form** —
`HEALTHCHECK CMD ["/concontexto","healthcheck"]`. Shallow by default; `--deep`
also pings PostgreSQL.

The golden rule becomes structural: the static-file handler depends on **no
repository port at all**, which makes "zero DB queries at request time" a
compile-time-visible, testable invariant rather than a convention.

Cache headers live in the Go handler, per §14.3's single-source-of-truth
requirement. Migrations run only via the explicit `migrate` subcommand, never on
boot — auto-migrate races across replicas and fires during rollbacks.

## 5. Testing strategy under Strict TDD

Strict TDD mode is ACTIVE for this project.

### 5.1 External APIs

| Approach | Pros | Cons | Effort |
|---|---|---|---|
| `testdata/` fixtures + `httptest.Server` | deterministic, offline, fast, small | drifts silently from reality | Low |
| go-vcr cassettes | records real traffic, deterministic replay | extra dependency, cassette hygiene, large files | Medium |
| Live calls in tests | catches drift instantly | flaky, rate-limited, breaks the offline TDD loop | Low but wrong |

**Recommended: two tiers.** Tier 1 (default `go test ./...`) uses trimmed real
responses under `testdata/` served by `httptest.Server`, table-driven — this is
what the TDD red/green loop runs against. Tier 2 is a build-tagged contract test
hitting the live endpoint with `nult=1`, asserting **shape only, never values**,
scheduled in CI rather than per-PR. That tier-2 test *is* the §9.4 synthetic
probe: build it once, use it twice.

Each fixture carries a `source.txt` with its URL and fetch date. Keep fixtures
to roughly three periods; per §1.2(b), `DATOS_SERIE` fixtures are naturally
small, whereas a full `DATOS_TABLA` EPA response would consume the entire
400-line review budget by itself.

Fixture set must include the volume-restriction envelope from §1.2(a) as a
first-class error case.

### 5.2 XLSX parser (milestone 0.6)

Table-driven over `testdata/*.xlsx`: happy path; header renamed or moved; sheet
missing; text or `#REF!` in a value cell; formula cell with no cached value;
truncated / not-a-zip file.

The decisive assertion is not "returns an error" but **"wrote nothing, and the
previously published datum is unchanged"** (§6.1.3, §9.3) — so the parser test
needs a spy repository. That phrasing is milestone 0.6's exit criterion made
testable. Check in the crafted malformed files and document each one in
`testdata/README.md`, since binary fixtures are opaque to reviewers.

### 5.3 PostgreSQL

| Approach | Pros | Cons | Effort |
|---|---|---|---|
| testcontainers-go | identical to the production image, hermetic, works locally and in CI | needs Docker, ~2–5 s start, heavy deps | Medium |
| GitHub Actions `services: postgres` | no test-code dependencies, fast | unusable on a dev laptop, version drift, shared state | Low |
| In-memory fake | instant, ideal inner loop | proves nothing about SQL or `MAX(version)` — exactly what 0.4 must prove | Low |

**Recommended: all three, layered.** Fakes for the domain and validation rules,
where the Strict TDD loop lives. **testcontainers-go with `postgres:17-alpine`**
for repository and SQL behaviour, because milestone 0.4's exit criterion is a
database-constraint assertion that a fake cannot prove, and the partial unique
index only exists in real PostgreSQL. Guard with `testing.Short()`, one
container per package via `TestMain`, per-test isolation by transaction
rollback.

Honest friction: milestone 0.1 has almost no unit-testable surface. There, "the
test" is the CI job plus a smoke test against the running container. Stated
explicitly so `sdd-apply` does not manufacture ceremony tests for YAML.

## 6. Sequencing (auto-chain, 400-line review budget)

Dependencies differ from the PRD's own numbering. **0.4 must precede 0.2, 0.3
and 0.6** — nothing can be written before the schema exists, so implementing in
PRD order would build 0.2 against a throwaway schema. 0.5 precedes the
plausibility rule. 0.7 precedes the metadata rule, which precedes publishing
anything. 0.2 precedes 0.3, because Eurostat reuses the same domain, writer and
validation harness and only the adapter differs — which is why 0.3 is cheap.

| # | Slice | Milestone | Budget risk |
|---|---|---|---|
| 1 | Go module, `cmd` serve/healthcheck, distroless multi-stage Dockerfile, compose, GitHub Actions, hello-world static serve, CODEOWNERS + branch protection | 0.1 | Medium |
| 2 | Fase 0 core migrations incl. `is_current` partial unique index, `migrate` subcommand, testcontainers harness, simulated-revision test | 0.4 | Medium-High |
| 3 | `/config` schema, source + series config with validity-ranged refs, `validate-config`, licence/attribution fields | 0.7 structure + §9.4 | Low-Medium |
| 4 | Validation engine: five rules + publish gate ("never publish bad data; keep serving the last valid datum") | §9.3 | Medium |
| 5a | Raw file store (SHA-256, immutable, download-attempt log) + Tempus3 `DATOS_SERIE` client + fixtures incl. the volume-restriction envelope | 0.2 | High — split required |
| 5b | Full historical load and validation of the six INE series | 0.2 | High — split required |
| 6 | Eurostat JSON-stat adapter + three datasets | 0.3 | Low-Medium |
| 7 | Editorial YAML + reconcile-to-DB + four-eyes wiring | 0.5 | Medium |
| 8 | XLSX parser for Social Security affiliation + malformed-file suite | 0.6 | Medium |
| 9 | Synthetic daily probe + scheduler + structured logs/alerts + attribution table closed | 0.7 closure | Low-Medium |

Approximately **9 slices, realistically 10–11** allowing for the slice-5 split
and possibly slice 1. Milestone 0.7 splits in two: structure in slice 3, content
closure in slice 9 once open question A4 is answered.

## 7. Risks

1. **PRD §7/§8 identifiers are verifiably stale** (§1.3). Correct them as part
   of 0.2/0.3 rather than discovering it as a surprise.
2. **ECOICOP v2 / January 2026 break missing from §9.6's minimum list** — likely
   the most consequential break for every CPI page. Also confirm whether live
   EPA tables already reflect CNAE 2025 double-coding.
3. **`DATOS_TABLA` volume restriction** (§1.2a) returns HTTP 200 with a non-array
   body; naive retry/backoff loops forever on it.
4. **Social Security XLSX design risk (0.6)**: the seg-social.es "Afiliación
   media mensual" resource appears to be a single annual workbook whose visible
   tables are driven by an INDEX-sheet month selector. If so, excelize will read
   cached values for whichever month the publisher last selected, and the parser
   must target the underlying data sheets or a per-month artifact. Needs manual
   confirmation of the real file before 0.6 is specced.
5. **Backup consistency (§14.3)** depends on the VPS's existing backup system,
   outside this repository. Fase 0 should ship the `pg_dump` script and document
   the requirement; wiring it is a user-side action.
6. **Could not confirm**: ECB Data Portal (HTTP 503) and the exact Social
   Security XLSX download URL (datos.gob.es catalogue page 404). Neither blocks
   slices 1–4.

## 8. Blocking decisions (need a product answer before `sdd-spec`)

- **A1 — project name and domain.** PRD Anexo A says it blocks Fase 1, but in
  practice it blocks slice 1: the Go module path, image name, compose stack name
  and repository name all bake it in.
- **A4 — final data and code licence.** Blocks 0.7 closure, the
  metadata-completeness validation rule, and the LICENSE file in slice 1. A
  complication the PRD does not note: Eurostat's policy (Commission Decision
  2011/833/EU) authorises reuse with acknowledgement but explicitly does **not**
  extend to third-party material and restricts some commercial redissemination,
  so a blanket "all derived data is CC BY 4.0" may overclaim for
  Eurostat-derived series.
- **Fase 0 migration scope** — exclude `indicator_page`, `verification`,
  `glossary` and `correction` from the Fase 0 schema? Recommended: yes. Needed
  before `sdd-spec` so the spec does not over-scope.
- **Config embed vs mounted volume** — ADR-level, affects the editorial
  workflow. Recommended: embed.

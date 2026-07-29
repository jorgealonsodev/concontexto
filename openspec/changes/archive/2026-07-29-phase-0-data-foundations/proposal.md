# Proposal — phase-0-data-foundations

Change: `phase-0-data-foundations` · Project: **ConContexto** · PRD Fase 0 (§17), milestones 0.1–0.7 only.
Input: `openspec/changes/phase-0-data-foundations/exploration.md` (complete). Artifact store: hybrid.

## Intent

Spanish economic debate systematically misuses real figures (PRD §2.1: temporal cherry-picking, ignored
series breaks, nominal-for-real, selective vintages). The product answer is a traceable chart per
indicator. That answer is only credible if the data underneath it is provably trustworthy — so Fase 0
builds the pipeline, not the pixels.

Fase 0 exists to make four non-negotiable product principles **structurally true before any public page
exists**, rather than promised in copy:

| Principle | What Fase 0 must make structurally true |
|---|---|
| P2 — total traceability | Every published value carries source, origin series identifier, extraction timestamp, raw-file hash and vintage. |
| P4 — context by default | Series breaks and exogenous events live in versioned editorial config and reach the database; breaks are never user-dismissible. |
| P5 — open pipeline | Ingestion code, config and derived data are public; raw downloads are retained with a published hash. |
| P7 — errors are documented, not erased | Revisions append a new version; nothing is overwritten; retirements are soft, not deleted. |

Why now: exploration verified that PRD identifiers are **already stale in production** (§1.3). Risk R1
(source-identifier churn) is live, not theoretical. Every week without a validated, versioned pipeline
increases the volume of data that will have to be re-ingested later.

## Scope

### In scope — milestones 0.1–0.7

- **0.1** Go module, monorepo layout, distroless multi-stage image, docker-compose, GitHub Actions, CODEOWNERS + four-eyes branch protection on `/config/**`, hello-world static deploy.
- **0.2** INE Tempus3 ingestion — six canonical series, full history, validated.
- **0.3** Eurostat JSON-stat ingestion — three harmonised datasets.
- **0.4** Data model + vintages, `migrate` subcommand, testcontainers harness.
- **0.5** Editorial YAML (`rupturas.yaml`, `eventos.yaml`, `gobiernos.yaml`) populated and reconciled to the database under four-eyes review.
- **0.6** XLSX parser for Social Security affiliation, including a malformed-file suite.
- **0.7** Per-source licence and attribution table closed; `validate-config`; synthetic daily probe.

### Out of scope

- All of **Fase 1–4**: indicator pages, search, international comparator, the *Verificado* module, glossary, embeds, public API, design system, chart component, OG images.
- **Schema tables with no Fase 0 consumer**: `indicator_page`, `verification`, `glossary`, `correction` are excluded from the Fase 0 migration set (settled decision).
- **`/web`** appears only as the hello-world deploy artifact of milestone 0.1. No design system, no components, no styling work. Utility/component CSS frameworks remain forbidden (PRD §12.1, §14.2).
- Derived-series provenance (indicator 16, PIB per cápita): anticipated in the model, not built.
- Wiring the VPS backup system (user-side action; Fase 0 ships the `pg_dump` script and the requirement).

## Capabilities

### New capabilities

| Capability | Covers | Slice |
|---|---|---|
| `platform-runtime` | Go binary subcommands (`serve`/`ingest`/`migrate`/`validate-config`/`healthcheck`), static file serving, `/healthz`, cache headers, container/CI/deploy. | 1 |
| `data-model-vintages` | Fase 0 schema, immutable observations, per-`(series, period)` versioning, `ingestion_run`, `series_source_mapping`, current-row invariant. | 2 |
| `editorial-config` | `/config` schema (`sources/`, `series/`, three editorial YAML files), `validate-config`, transactional idempotent reconcile to `series_break`/`event`, four-eyes gate. | 3, 7 |
| `data-validation` | The five PRD §9.3 rules as pure functions plus the publish gate. | 4 |
| `raw-file-archive` | Immutable raw storage keyed by SHA-256 with timestamp, plus `download_attempt` log for the amber freshness semaphore. | 5a |
| `source-ingestion-ine` | Tempus3 `DATOS_SERIE/{COD}` client, volume-restriction error handling, six canonical series. | 5a, 5b |
| `source-ingestion-eurostat` | JSON-stat 2.0 adapter, three datasets. | 6 |
| `source-ingestion-xlsx` | Social Security affiliation workbook parser and malformed-file handling. | 8 |
| `source-attribution-licensing` | Per-source licence, attribution formula and redistribution restrictions as validated config. | 3, 9 |
| `pipeline-operations` | Scheduler, synthetic daily probe, structured logs and alerts. | 9 |

### Modified capabilities

None — `openspec/specs/` is empty (greenfield).

## Settled decisions (do not re-open)

| # | Decision | Rationale |
|---|---|---|
| D1 | Project name **ConContexto**; Go module `github.com/jorgealonsodev/concontexto`; image and Portainer stack `concontexto`. | PRD Anexo A/A1 resolved. §12.1 requires a shared chart to be recognisable "de ConContexto"; the remote already exists. |
| D2 | Code **MIT**. Derived data and editorial text **CC BY 4.0 with chained attribution**, but **never asserted as one blanket licence**. | Eurostat policy (Commission Decision 2011/833/EU) permits reuse with acknowledgement, does not extend to third-party material, and restricts some commercial redissemination. Per-source terms are therefore recorded in `sources/{source}.yaml` and are authoritative over any site-wide statement. |
| D3 | Fase 0 migration set: `source`, `dataset`, `series`, `series_source_mapping`, `observation`, `ingestion_run`, `raw_file`, `download_attempt`, `series_break`, `event`. | The four excluded tables have no Fase 0 consumer; including them would over-scope the spec. |
| D4 | **ADR — configuration is compiled in with `go:embed`**, not mounted. | Pins config version to binary version, which is what §9.4 and P2 traceability require. Rebuild cost is already paid: Portainer "Pull and redeploy" is the deploy path and `/config/**` already passes a four-eyes PR gate. |
| D5 | **ADR — INE ingestion uses `DATOS_SERIE/{COD}`**, one call per canonical series. `DATOS_TABLA` is discovery-only. | The series COD is the stable identifier; the table Id is only a container (`ECP320` resolves identically in tables 56934 and 59238). `DATOS_SERIE` is not subject to the volume restriction that makes `DATOS_TABLA` return HTTP 200 with `{"status":"No puede mostrarse por restricciones de volumen"}` — a JSON object where an array is expected. Smaller blast radius, reviewable fixtures. |

## Non-negotiable business rules (must survive into the spec)

| Rule | Source |
|---|---|
| **Zero database queries and zero computation at page-request time.** Everything is pre-rendered when the pipeline ingests. | §14.2 golden rule |
| **No external source is ever called at page-request time. Ever.** | §9.2 |
| **Validation failure never publishes.** The last valid datum keeps being served, with a banner; the chart is never hidden and the suspect datum is never published. | §6.1.3, §9.3 |
| **Raw downloads are stored immutably** with SHA-256 and timestamp, hash published. | §9.1, P5 |
| **Observations are immutable per version.** A revision creates a new version; current = `MAX(version)`. Nothing is overwritten. | §9.5, §10 |
| **Series breaks are never user-dismissible.** | P4, §6.1.1 |
| **Source identifiers live in versioned configuration, never in code**, and change by reviewed pull request. | §9.4 |
| **Editorial YAML changes require four-eyes review.** | §9.6, §15.2 |

## Approach

### Hexagonal layout (exploration §4)

```
/app
  cmd/concontexto/main.go   # serve | ingest | migrate | validate-config | healthcheck
  internal/
    indicators/             # DOMAIN: Series, Observation, Period, Vintage, Break, Event + driven ports
    ingestion/              # APPLICATION: IngestSeries, ReconcileEditorialConfig, RunSyntheticProbe
      validation/           # the five rules as PURE functions over domain types
    publishing/             # APPLICATION: pre-render (Fase 1; stub in Fase 0)
    adapters/{ine,eurostat,xlsx,postgres,filestore,config}
    http/                   # DRIVING: static server + internal API + /healthz
    scheduler/              # DRIVING: job runner
```

`serve` and `ingest` are two driving adapters over one domain, so splitting into one process or two later
is a compose change, not a refactor. The golden rule becomes structural: the static-file handler depends
on **no repository port at all**, making "zero DB queries at request time" a compile-time-visible,
testable invariant rather than a convention. Migrations run only via the explicit `migrate` subcommand,
never on boot. Distroless has no shell, so `HEALTHCHECK` must use exec form.

### Two-tier testing (exploration §5; Strict TDD is active)

- **Tier 1 — default `go test ./...`**: trimmed real responses under `testdata/` served by `httptest.Server`, table-driven. This is what the red/green loop runs against. Each fixture carries a `source.txt` with URL and fetch date. The volume-restriction envelope is a first-class fixture.
- **Tier 2 — build-tagged contract test** against live endpoints with `nult=1`, asserting **shape only, never values**, scheduled in CI rather than per-PR. This test *is* the §9.4 synthetic probe: built once, used twice.
- **PostgreSQL, layered**: fakes for domain and validation rules; **testcontainers-go with `postgres:17-alpine`** for repository and SQL behaviour, because milestone 0.4's exit criterion is a database-constraint assertion a fake cannot prove. Guard with `testing.Short()`, one container per package via `TestMain`, isolation by transaction rollback.
- Milestone 0.1 has almost no unit-testable surface. There the test is the CI job plus a smoke test against the running container. Stated explicitly so `sdd-apply` does not manufacture ceremony tests for YAML.

### Sequencing — reordered from PRD numbering (exploration §6)

Dependency order, not PRD order: **0.4 precedes 0.2, 0.3 and 0.6** (nothing can be written before the
schema exists); **0.5 precedes the plausibility rule** (it reads `rupturas.yaml`); **0.7 precedes the
metadata-completeness rule**, which precedes publishing anything; **0.2 precedes 0.3** because Eurostat
reuses the same domain, writer and validation harness and only the adapter differs.

## Outcomes and exit criteria per milestone

| Milestone | Testable exit criterion |
|---|---|
| 0.1 | A pushed commit produces an automatic deploy of a hello-world static page; CI is green; `/healthz` returns 200; branch protection rejects a single-approval change to `/config/**`. |
| 0.2 | All six canonical series load full history and pass validation; the pinned identifiers below resolve live; a `DATOS_TABLA` volume-restriction body surfaces as a **named error**, not an opaque decode failure or an infinite retry. |
| 0.3 | Three harmonised Eurostat datasets load and pass validation using the same domain, writer and validation harness as 0.2. |
| 0.4 | **A simulated revision creates a new version without overwriting**: after re-ingesting a changed value for an existing `(series, period)`, the prior row still exists with its original value, the new row has `version = prior + 1`, and the database itself rejects a second current row for the same `(series, period)`. |
| 0.5 | YAML populated and reconciled; removing an entry retires it rather than leaving it live or hard-deleting it; re-running the reconcile changes nothing (idempotent); four-eyes review is enforced by the repository, not by convention. |
| 0.6 | A real monthly affiliation ingest passes validation. For each malformed file, the assertion is **not** "returns an error" but **"wrote nothing AND the previously published datum is unchanged"**. |
| 0.7 | Per-source attribution table closed and schema-validated; a series missing source, unit, frequency or licence cannot be published. |

## PRD corrections this change must carry

| PRD location | Correction |
|---|---|
| §7/§8 — INE table `4247` | Frozen at 2023-Q4. Replaced: table 65349, series `EPA453100`. |
| §7/§8 — INE table `50902` | Frozen at 2025-12 on the old base (119.942 vs new-base 103.598). Replaced: table 76125, series `IPC290751`. |
| §8.2 — Eurostat `prc_hicp_manr`, `prc_hicp_midx` | Discontinued. Use `prc_hicp_minr`, whose dimension is **`coicop18`, not `coicop`**. |
| §7.3 row 17 — population | Operation 72 / `CP` returns an empty table list. Live source is operation 450 / `ECP` (Estadística Continua de Población), table 59238, series `ECP320`. |
| §9.6 — minimum break list | **Add the ECOICOP v2 / January 2026 break.** ECOICOP 1 is archived and frozen; from January 2026 data exists only under ECOICOP 2 and Eurostat warns of potential January 2026 breaks. It affects every CPI-derived page and is currently absent. |

Pinned milestone-0.2 identifiers (verified live, cross-checked against PRD §11.3's own sample response):
`EPA453100` (65349), `EPA387796` (65109), `IPC290751` (76125), `IPC292511` (76130), `CNTR6721` (67822),
`ECP320` (59238). Periodicity must be asserted when pinning any identifier — tables 65962 and 72982 carry
the same `Nombre` as 65109 but hold annual averages.

## Affected areas

| Area | Impact | Description |
|---|---|---|
| `/app` | New | Go module, hexagonal layout, all pipeline code. |
| `/config` | New | `sources/`, `series/`, `rupturas.yaml`, `eventos.yaml`, `gobiernos.yaml`. |
| `/web` | New | Hello-world Astro artifact only (milestone 0.1). |
| `/data-derived` | New | Placeholder; populated from Fase 1. |
| `docker-compose.yml`, `Dockerfile` | New | Portainer Repository-method stack, distroless multi-stage. |
| `.github/workflows/`, `CODEOWNERS` | New | CI, four-eyes gate on `/config/**`. |
| `LICENSE`, `LICENSE-DATA` | New | MIT for code; per-source data terms referenced, not blanket-asserted. |

## Delivery forecast

Roughly **9 slices, realistically 10–11** under a 400-line review budget with `auto-chain`.

| # | Slice | Milestone | Budget risk | Depends on |
|---|---|---|---|---|
| 1 | Module, `cmd`, Dockerfile, compose, CI, hello-world, CODEOWNERS | 0.1 | Medium | — |
| 2 | Fase 0 migrations + current-row invariant, `migrate`, testcontainers harness, simulated-revision test | 0.4 | Medium-High | 1 |
| 3 | `/config` schema, source + series config with validity-ranged refs, `validate-config` | 0.7 structure, §9.4 | Low-Medium | 1 |
| 4 | Validation engine: five rules + publish gate | §9.3 | Medium | 2, 3 |
| 5a | Raw file store + `DATOS_SERIE` client + fixtures incl. volume-restriction envelope | 0.2 | High — split required | 2, 3, 4 |
| 5b | Full historical load and validation of the six INE series | 0.2 | High — split required | 5a |
| 6 | Eurostat JSON-stat adapter + three datasets | 0.3 | Low-Medium | 5a |
| 7 | Editorial YAML + reconcile-to-DB + four-eyes wiring | 0.5 | Medium | 2, 3 |
| 8 | XLSX parser + malformed-file suite | 0.6 | Medium | 2, 3, 4 |
| 9 | Synthetic probe + scheduler + structured logs/alerts + attribution table closed | 0.7 closure | Low-Medium | 5a, 6, 8 |

Milestone 0.7 deliberately splits: structure in slice 3, content closure in slice 9. Slice 1 may also
need a split if CI and container work exceed the budget together.

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Source identifiers churn again mid-implementation (R1, proven live). | High | Identifiers in versioned config only; `series_source_mapping` with validity ranges; synthetic daily probe as early warning. |
| Social Security workbook is a single annual file driven by an INDEX-sheet month selector, so excelize reads whichever month the publisher last selected. | Medium | **Blocking for slice 8**: manually confirm the real file before milestone 0.6 is specced. Parser must target underlying data sheets or a per-month artifact. |
| `DATOS_TABLA` volume restriction loops naive retry/backoff forever (HTTP 200, non-array body). | Confirmed | Discriminating decode type, named error, no retry on that envelope, first-class fixture. |
| Social Security download URL unverified (datos.gob.es catalogue page 404); ECB Data Portal returned HTTP 503. | Medium | Neither blocks slices 1–7. Confirm the SS URL before slice 8; ECB is not needed in Fase 0. |
| Blanket CC BY 4.0 over derived data overclaims for Eurostat-derived series. | Medium | D2: per-source terms in `sources/{source}.yaml` are authoritative; no site-wide licence assertion over source-derived values. |
| CNAE 2025 double-coding may already be reflected in live EPA tables. | Medium | Confirm during slice 5b; add to `rupturas.yaml` if present. |
| Backup consistency depends on the VPS's existing backup system, outside this repository. | Medium | Ship the `pg_dump` script and document the requirement; wiring is a user-side action. |

## Rollback plan

Rollback is per slice, and cheap because the repository is greenfield and the data model is append-only.

1. **Code / config**: `git revert` the slice PR. Because configuration is embedded (D4), reverting the binary reverts the config atomically — there is no drifting mounted volume to reconcile.
2. **Deploy**: Portainer redeploy of the previous image. No published ports change; the `proxy` network attachment is unchanged.
3. **Schema**: every migration ships a reversible `down`. Migrations run only via the explicit `migrate` subcommand, so a rollback never races a container restart.
4. **Bad ingest**: never a delete. Observations are immutable per version, so a bad run is rolled back by re-pointing the current row to the prior version and recording the reason — which preserves P7 and leaves the raw file and its hash intact for audit.
5. **Editorial reconcile**: entries are soft-retired with a `config_digest`, so reverting the YAML and re-running the idempotent reconcile restores the prior state without losing the audit trail.
6. **Point of no return**: none in Fase 0. Nothing is public yet, so no permalink commitment (Anexo E.1) is at risk.

## Success criteria

- [ ] All seven milestone exit criteria above pass as automated tests, except 0.1's deploy check, which is a CI job plus a container smoke test.
- [ ] Six INE series and three Eurostat datasets carry full history, validated, with vintage and raw-file hash traceable end to end.
- [ ] A simulated revision demonstrably creates a new version; the database rejects a second current row for the same `(series, period)`.
- [ ] Every malformed-XLSX case proves "wrote nothing and the previously published datum is unchanged".
- [ ] `validate-config` runs in CI and fails on a series missing source, unit, frequency or licence.
- [ ] The per-source attribution table is closed and schema-validated; no blanket data-licence claim exists in the repository.
- [ ] The static-file handler has no dependency on any repository port (structural proof of the golden rule).
- [ ] The five PRD corrections above are reflected in `/config`, not only in prose.

# Deploy (Portainer, VPS)

Remediation batch (sdd-verify CRITICAL C5, 2026-07-29): task 1.17
originally claimed a "deploy step" that did not exist anywhere in the
repository — the same "marked `[x]`, deliverable absent" failure mode
task 1.21 already disclosed once (see `openspec/changes/phase-0-data-foundations/apply-progress.md`).

`.github/workflows/deploy.yml` is a real, correctly wired deploy workflow.
It is honest about its own status: **PRD §17's milestone-0.1 exit
criterion ("automated deploy of a hello world") is genuinely UNMET as of
this batch**, because this environment has no VPS running Portainer and
no secrets configured to reach one. The workflow gates on that fact
explicitly — when the one required secret below is absent, its gate step
prints a `::warning::` and every deploy step is skipped, rather than
reporting a fake green deploy.

## What a clean bring-up does

`docker compose up` on a machine with no volumes and no database reaches a
working site with real data and **no manual step**. The sequence Compose
enforces:

| # | Service | What runs | Gate on the next step |
|---|---|---|---|
| 1 | `postgres` | Postgres starts | `condition: service_healthy` |
| 2 | `migrate` | `concontexto migrate up`, then exits | `condition: service_completed_successfully` |
| 3 | `reconcile` | `concontexto ingest --reconcile`, then exits | `condition: service_completed_successfully` |
| 4 | `app` | `concontexto serve` — HTTP server **plus** the in-process scheduler, whose first cycle runs immediately | — |

Measured on a clean slate (`docker compose down -v`, then up): `up` returned
in **7 seconds**, and all three sources (INE, Seguridad Social, Eurostat —
10 series) had ingested, validated, published and exported within **17
seconds** of `up`. `/healthz`, `/indicador/tasa-de-paro-epa/` and
`/data-derived/csv/tasa-de-paro-epa.csv` all returned 200 with real data;
the artifact's `manifest.json` carried a `generated_at` 10 seconds after
`up`, which is what distinguishes a runtime export from the copy the image
seeds the volume with.

### Why `migrate` is a service and not an operator instruction

`app` used to start against whatever schema happened to exist, which on a
clean machine was none. A fresh deploy therefore needed someone to run
`docker compose exec app /concontexto migrate up` before the pipeline could
write anything — and nothing in the repository said so, so the failure mode
was a stack that came up healthy, served pages, and 404'd every
`/data-derived/` download forever.

This does **not** weaken spec platform-runtime's "migrations run only on
explicit command". It *is* the explicit command: the same `migrate up`
subcommand, with the same `migrate.ExecutionCount()` accounting. What
changed is who issues it — Compose, as an ordered deploy step, rather than
a human who has to know to. `serve` still never migrates on boot.
`migrate up` is idempotent (applied versions are recorded in
`migration_state.schema_migrations`), so re-running it on every `up` and
every Portainer redeploy is a no-op once the schema is current.

### Why reconciliation is a one-shot service

`config/rupturas.yaml`, `config/eventos.yaml`, `config/gobiernos.yaml` and
`config/reconocimientos.yaml` are the authoritative editorial registries;
the `series_break`, `event` and `validation_acknowledgement` tables are a
projection of them (`ingestion.ReconcileEditorialConfig`). Until this
service existed, **nothing in a deployed stack ever produced that
projection.** The reconcile had exactly one call site — the
`ingest --reconcile` flag path — and the stack ran `migrate up` and then
`serve`, whose in-process scheduler only ever runs per-source *ingestion*.
Measured on the live stack:

```
SELECT count(*) FROM event;        -> 0
SELECT count(*) FROM series_break; -> 0
```

Three separately-built, separately-tested features were dead as a
consequence, none of them visibly: the break band (`BreakBand.astro` and
the chart's shaded geometry render from the artifact's `breaks` array,
which `publishing.Export` reads back out of `series_break`), event
annotations, and validation **rule 3's break exemption** — `breakAt` always
received an empty slice, so a jump at a genuinely recorded methodological
break blocked exactly as if no break had ever been recorded.

Three shapes were available. The one-shot service was chosen:

- **A one-shot Compose service (chosen).** The editorial YAML is *embedded
  in the binary* (ADR-1, `//go:embed`), so it cannot change without a new
  image, and a new image cannot arrive without a container recreation.
  "Reconcile when the image changes" and "run a one-shot service on `up`"
  are therefore the same instant. It also mirrors `migrate`, which the
  stack already uses for exactly this "run once, to completion, before the
  app starts" shape, and it turns a failed reconcile into a non-zero exit
  that *gates the app* rather than a log line nobody reads.
- **The scheduler's cycle (rejected).** Idempotent, but it re-does
  byte-identical work every fifteen minutes forever, and it couples
  editorial reconciliation to per-source ingest scheduling — a source in
  backoff would delay the registries reaching the database, for no reason
  related to the registries.
- **`serve` reconciling at boot (rejected).** Fewest moving parts, but it
  puts a database write in the boot path of a process whose entire job is
  to serve static files. The reasoning that keeps `migrate` out of `serve`
  transfers only partly — a reconcile is not a schema migration, and spec
  platform-runtime's "migrations run only on explicit command" does not
  cover it — but the operational half transfers exactly: `runServe` is
  deliberately resilient to every missing prerequisite (no `STATIC_ROOT`,
  no `DATABASE_URL`), so a failed reconcile there would have to be
  swallowed to preserve that resilience, and a silently swallowed reconcile
  is the defect this section exists to describe.

**Ordering is declared, never timed.** `reconcile` writes to tables the
migrations create, so it gates on `migrate` having *exited 0*. `app` gates
on `reconcile` having exited 0 in turn, because the scheduler's first ingest
cycle runs *at boot* and that cycle both validates against `series_break`
(rule 3) and exports the breaks and events into the artifact the pages
render. A reconcile landing after that cycle would publish an artifact with
empty `breaks`/`events` arrays and leave it that way until the next cycle.

`ingest --reconcile` is idempotent — rows are matched on their
`config_digest` and updated in place, never deleted and re-inserted
(`app/internal/adapters/postgres/editorial.go`) — so re-running it on every
`up` and every Portainer redeploy changes zero rows once the database
already matches the embedded YAML.

It contacts **no third-party API**: the registries are embedded, not
fetched. That is why it still runs, and must still run, in a stack brought
up with `APP_SCHEDULE_DISABLED=true`.

#### What the reconcile container logs

`docker compose logs reconcile` is the whole operator-facing record of what
the editorial YAML did to the database:

```
ingest --reconcile: breaks inserted=5 updated=0 retired=0; events inserted=10 updated=0 retired=0; acknowledgements inserted=0 updated=0 retired=0
ingest --reconcile: breaks pending=4 (unconfirmed date), not projected: epa-cnae2025-doble-codificacion, cn-revision-base-sept-2025, sec-cambios-deuda-deficit, ss-cnae2025-afiliacion
ingest --reconcile: events pending=2 (unconfirmed date), not projected: reforma-laboral-2021, gobierno-suarez-1976
ingest --reconcile: acknowledgements pending=1 (no human signature), not projected: ocupados-epa-2020-q2-covid
```

Both the **count and the identifiers** are printed, which is what spec
editorial-config asks for verbatim: the count is not derivable at a glance
from a list, and the identifiers are the only part an operator can act on.
An entry appears here instead of in the database because its effective date
is not yet confirmed against the source's methodological note — a guessed
break date silently corrupts every comparison that crosses it (PRD
principle P4).

The **acknowledgements** line is the newest of the three. A series blocked
by a validation rule has two very different explanations — "the resolving
record is waiting on a human signature" and "there is no record at all" —
with opposite next actions, and until this line existed the log could not
tell them apart. `ocupados-epa-2020-q2-covid` is currently in the first
state: `config/reconocimientos.yaml` carries the record, drafted and cited,
with `signature_status: unsigned`.

### Why the scheduler runs inside `serve` rather than in its own container

`serve` has always launched the scheduler in its own goroutine beside the
HTTP server (`app/cmd/concontexto/serve.go` → `startScheduler`; design.md's
"serve = static server + /healthz + in-process scheduler"). There is no
`schedule` subcommand and there deliberately is not one. Weighed against a
second container running the same image with a different `command`:

- **Memory.** PRD §14.3 budgets app 256 MB and postgres 512 MB. A separate
  scheduler container would need a limit of its own, and the ingest half is
  the memory-hungry half, so the stack would grow from 768 MB to roughly
  1 GB on a VPS the PRD explicitly sizes for low memory. Splitting the
  existing 256 MB instead is worse than sharing it: an idle static file
  server has a tiny resident set, so one shared cgroup lets an ingest peak
  into nearly the whole budget and hand it straight back.
- **Crash isolation.** This is the real cost of the choice, and it is
  accepted rather than dismissed: an ingest that panics or is OOM-killed
  takes the HTTP server with it. What bounds the damage is that the site is
  100% pre-rendered static files shipped in the image — there is no cache to
  warm and no query to replay, so `restart: unless-stopped` restores service
  in a process start. The golden rule (PRD §14.2, zero DB queries at request
  time) means served content never depended on the scheduler succeeding.
  The blast radius is seconds of 502 from Nginx Proxy Manager, not degraded
  or stale data.
- **Restart semantics.** A restart is already a designed-for case, not an
  edge case: `startSchedulerLoop` seeds each source's last success from
  `download_attempt` in Postgres rather than from process memory, precisely
  so a restart does not raise a false "down for over 24h" incident.
- **Shared state.** One process means one pgxpool, one embedded config, and
  exactly one writer for `/web/dist/data-derived`. Two containers mounting
  the same `export_artifact` volume could both export into it.

### The first cycle is immediate, not 15 minutes away

The scheduler wakes every `scheduleCheckInterval` (15 minutes) and runs any
source that is due; on the first pass every source is due. Production used
to hand that loop a bare `time.NewTicker(15m).C`, whose *first* send lands
one full interval after start — so a fresh deployment served pages backed by
an empty database, with every download link 404ing, for 15 minutes.
`schedulerTicks` (`app/cmd/concontexto/schedule.go`) prepends one tick at
start-up and then relays the real ticker. Nothing about the loop's own
due-gating changed.

A consequence worth stating plainly: **the container now contacts the live
INE, Eurostat and Seguridad Social endpoints as soon as it starts**, once
per source, on every start. That is the app's purpose in production. For a
stack that must come up without touching a third-party API — a hermetic test
stack above all — set `APP_SCHEDULE_DISABLED=true` (see `env.example`),
which leaves `serve` a pure static file server while `migrate` and
`healthcheck --deep` keep working.

## Reaching the app from the host

`docker-compose.yml` publishes **no** ports. That is the production
topology it documents, and `scripts/smoke-test.sh` step 5 asserts it on
every CI run: a published port binds the container to the host's
interfaces, where it is reachable without passing through Nginx Proxy
Manager, so TLS becomes optional in practice.

For local development, copy the committed template once:

```bash
cp docker-compose.override.yml.example docker-compose.override.yml
docker compose up -d
curl http://localhost:8080/healthz
```

Compose merges `docker-compose.override.yml` automatically, with no extra
`-f` flags. The copy is git-ignored (`.gitignore`), so the committed
topology stays the production one and the opt-in cannot be committed by
accident. The template binds `127.0.0.1:8080` rather than `0.0.0.0:8080`,
so the app is not published to whatever network the workstation is on.

Without the override — which is how CI does it — the container is reachable
over the `proxy` network without publishing anything:

```bash
docker run --rm --network proxy curlimages/curl:8.11.1 \
  -fsS http://app:8080/healthz
```

**One interaction to know about:** `scripts/smoke-test.sh` invokes plain
`docker compose`, so an override left in place is merged into the smoke
test too, and its step 5 ("no published ports") then fails. Either remove
`docker-compose.override.yml` before running the smoke test, or run it with
`COMPOSE_FILE=docker-compose.yml ./scripts/smoke-test.sh`.

## Closing the publish loop

The site is fully pre-rendered, so **new data does not reach a reader until
the site is rebuilt.** The loop that makes that happen has four links, and
before this section existed it was open at every one of them (verify-report
CRITICAL-28). What that produced was measured on the running stack: a
publishing cycle exported into the container's `export_artifact` volume
every 15 minutes, nothing rebuilt the pages, and the downloadable artifact
under `/data-derived/` drifted ahead of the numbers printed on the pages —
silently, with no alert and no way to notice from the outside.

| # | Link | What carries it |
|---|---|---|
| 1 | A publish cycle asks for a rebuild | `publishing.Publish` → `adapters/github` POSTs `repository_dispatch` (`event_type: rebuild`) |
| 2 | The deployment is actually wired to ask | `APP_REBUILD_DISPATCH` + `GITHUB_DISPATCH_REPO`/`GITHUB_DISPATCH_TOKEN`, passed to `app` by `docker-compose.yml` |
| 3 | GitHub receives it and rebuilds | `.github/workflows/rebuild.yml` → `deploy.yml` |
| 4 | The running stack can tell whether the rebuild landed | `/web/build-manifest.json` + the scheduler's deploy-completed watchdog |

### Link 2: telling "deliberately off" from "broken"

`buildDispatcher` used to return `nil` whenever either `GITHUB_DISPATCH_*`
variable was empty, and `publishing.Publish` treats `nil` as "skip, say
nothing". That is the right behaviour on a laptop and in `go test` — nobody
asked for a dispatch. It is the wrong behaviour in a deployment, where
nobody asked *because the compose file forgot to*. The two states were
indistinguishable, and the deployed one was the silent one.

`APP_REBUILD_DISPATCH` separates them, and it records **whether the operator
ever asked**:

| Value | Meaning |
|---|---|
| `off`, or unset | Deliberately off. No dispatch, one INFO-level structured record, no alert ever. The default for a bare binary. |
| `required` | A rebuild is expected. Configured means dispatch; **unconfigured means broken** — every publish cycle raises `alerting.DispatchFailed` naming the unset variable. |

`docker-compose.yml` defaults the `app` service to `required`. That is the
whole point: the difference between "a deployment" and "a laptop" *is* the
compose file, so the default lives there and no operator has to remember
anything. `docker-compose.override.yml.example` sets `off` for local
development, so a developer who copies it is not paged for a rebuild they
never wanted. An unrecognised value is treated as `required` and reported —
a typo must never silently switch the publish loop off.

Every state reaches the structured log under `component=rebuild-dispatch`
with `state=disabled` / `enabled` / `unconfigured`, so the distinction is
greppable rather than being two readings of the same silence.

### Link 4: the deploy-completed instant

This is the link with no obvious signal, and it is worth being precise about
what was wrong before. The scheduler's publish-latency watchdog compared a
source's last ingestion success against the **local** `manifest.json` — the
one `publishing.Publish` had written seconds earlier in the same call. So
the only failure it could ever catch was an export that did not run. A
dispatch never sent, a CI rebuild that failed and a Portainer redeploy that
never happened were all invisible to it.

The image now records the artifact its pages were **built** from, at
`/web/build-manifest.json` (the Dockerfile's web-builder stage writes it;
`APP_BUILD_MANIFEST` overrides the path). That file changes by exactly one
mechanism: **a new image being deployed** — which cannot happen unless the
dispatch, the CI rebuild and the redeploy all succeeded. So one comparison
inside the running container covers every remaining link at once, with no
call to GitHub, to Portainer or to the public site:

> the artifact this container is publishing has been live for longer than
> `APP_PUBLISH_LATENCY_BUDGET` and the deployed pages were still built from
> an older one ⇒ the rebuild did not land ⇒ alert.

Divergence itself is normal — every publish cycle creates it, and a rebuild
is supposed to close it. Only divergence that outlives the budget is a
breach. Not under `dist/`, deliberately: the `export_artifact` volume is
mounted over `/web/dist/data-derived`, so a stamp inside `dist/` would either
be shadowed at runtime or served to readers as though it were part of the
published artifact.

The check declines to fire in two cases, and says which at start-up
(`component=deploy-watchdog`, `state=active|inactive`, with a reason):

- **Rebuild dispatch is off.** Divergence is then the operator's stated
  intent, not a fault.
- **The deployed artifact is unknown** — no build manifest, i.e. a bare
  binary or an image built before this existed. *Unknown is not stale.* A
  watchdog reporting a deploy failure it cannot observe would be exactly the
  fabricated verdict this repository refuses elsewhere.

### What remains unprovable here, stated plainly

This environment has no VPS, no Portainer stack, no `PORTAINER_WEBHOOK_URL`
and no `EXPORT_URL`, so the following are **built and unit-tested but have
never been executed end to end**, and nothing in this repository should be
read as claiming otherwise:

1. **A real `repository_dispatch` round trip.** No dispatch has ever been
   accepted by GitHub from this repository — the only live attempt returned
   `401` from a deliberately invalid token, which proves the request is
   well-formed and reaches `api.github.com`, and proves nothing about the
   receiving workflow. `rebuild.yml`'s own steps were exercised as shell
   scripts against a local HTTP origin (payload present/absent, origin
   matching/newer/older/unreachable, `EXPORT_URL` unset), not as a GitHub
   run.
2. **`rebuild.yml` → `deploy.yml` as a reusable-workflow call.** Validated
   by `actionlint`; never dispatched.
3. **A redeploy actually replacing the running container**, and therefore
   the build manifest actually advancing. The comparison that detects a
   stalled rebuild is unit-tested in both directions and was exercised
   against real manifests on disk; the event it is watching for has never
   occurred here because nothing has ever deployed this stack.

What *is* proven locally: the stamp is written by a real `docker build` in
both the `EXPORT_DIR` and `EXPORT_URL` forms and is byte-identical to the
artifact the build read; a missing or unreachable artifact fails that build
rather than shipping an unobservable image; and all three dispatch states
were driven through a real `concontexto ingest` against a real Postgres and
the live INE endpoint.

## What must exist before this criterion can close

1. A VPS (or any host) running Portainer, reachable from the internet
   (or from GitHub Actions' runner IP range) over HTTPS.
2. A Portainer **stack** on that host, defined to pull
   `ghcr.io/jorgealonsodev/concontexto:latest` (the image `deploy.yml`
   builds and pushes to GitHub Container Registry — no extra registry
   secret is needed for the push half; it uses the workflow's own
   built-in `GITHUB_TOKEN`).
3. A **webhook** enabled on that stack (Portainer → stack → Webhooks),
   which redeploys the stack (re-pulling the `:latest` tag) whenever its
   URL receives an HTTP POST.

## Required GitHub Actions secret

| Secret | What it is | Where it comes from |
|---|---|---|
| `PORTAINER_WEBHOOK_URL` | The stack webhook URL from step 3 above | Portainer's own UI, once the stack exists |

No other secret is required: the image push to `ghcr.io` authenticates
with the workflow's automatically provided `GITHUB_TOKEN`, which already
has `packages: write` scope for a workflow running in this repository.

## Required container environment (the stack, not Actions)

These are read by the running binary, not by a workflow, so they belong in
the Portainer stack's environment (or a `.env` beside `docker-compose.yml`)
and never in the repository. See `env.example` for the full text.

| Variable | What it is |
|---|---|
| `APP_REBUILD_DISPATCH` | `required` (the compose default) or `off`. Whether this deployment expects a site rebuild after each publish cycle. |
| `GITHUB_DISPATCH_REPO` | `owner/repo` the `repository_dispatch` is POSTed to. |
| `GITHUB_DISPATCH_TOKEN` | A **fine-grained** personal access token scoped to `Contents: read` and `Actions: write` on that one repository. Never a classic, account-wide token. |

With `APP_REBUILD_DISPATCH=required` and the other two unset, the stack
does not fail: it publishes normally and raises a `dispatch-failed` alert
on every cycle naming the missing variable. That is the intended noise —
it is what a deployment that cannot reach its readers should sound like.

## Required GitHub Actions variable

| Variable | What it is | Where it comes from |
|---|---|---|
| `EXPORT_URL` | The site's own public `/data-derived/` origin, e.g. `https://concontexto.example/data-derived/` | The hostname configured in Nginx Proxy Manager, once the stack serves traffic |

A **variable**, not a secret, deliberately: it is a public URL that the
running container already serves to anyone, and a variable is visible in
the run log — which is what you want for the one input that decides which
numbers ship.

`deploy.yml` gates on it exactly the way it gates on
`PORTAINER_WEBHOOK_URL`: unconfigured means a visible `::warning::` and no
image built, no image pushed, no redeploy triggered. See the next section
for why it cannot simply be defaulted.

## Where the published numbers come from

The site is fully pre-rendered: the Go binary writes one JSON artifact and
the Astro build reads it (`web/src/lib/export/loader.ts`; design.md D-1).
So the image build must be told which artifact to render, and the build
**fails** if it is not told. There is no default and no fallback.

That refusal is the fix for verify-report **CRITICAL-15**, the most severe
finding this project has recorded. The loader used to fall back to
`web/test/fixtures/export` when neither variable was set, and the
Dockerfile's web-builder stage was exactly such a build: `COPY web/ .`
brought the fixture into the image build, there was no `.dockerignore`,
and the fixture's own `source.txt` opens with

> READ THIS FIRST: MOST OF THE NUMBERS IN THIS DIRECTORY ARE SYNTHESISED.
> They were never published by INE.

Only the newest three observations per series are real; the 95 to 291
before them are a straight line produced by a formula. Every image ever
built from this repository therefore published invented numbers under real
INE attribution, with a real origin identifier and a real extraction
timestamp, and disclosed nothing on the page. Read out of the real built
HTML: `2002-Q1: 11.85 % población activa, Definitivo`.

Two supported sources, both explicit:

| Source | Build arg | When |
|---|---|---|
| Live origin | `--build-arg EXPORT_URL=https://<host>/data-derived/` | Normal operation. A publishing cycle writes the artifact into the container's `export_artifact` volume and dispatches a rebuild, which fetches it back from the site it is rebuilding. |
| Staged directory | `--build-arg EXPORT_DIR=data-derived` | The first deploy, before any live origin exists. Stage a real artifact under `web/` in the build context — the path is relative to the builder's `/web` working directory. |

### The bootstrap, stated plainly

**An image build cannot obtain a real artifact by itself.** The exporter
reads PostgreSQL (`app/cmd/concontexto/export_cmd.go`:
`concontexto export` → `STATIC_ROOT/data-derived`) and runs inside the
started container; an image build has neither the database nor the volume.
Before the first publication there is also no live `/data-derived/` origin
to fetch. So the first image must be built the staged way:

```bash
# 1. Bring up the stack once with a previously-built image, or run the
#    pipeline against a database reachable from your workstation, and let
#    it write the artifact:
DATABASE_URL=... STATIC_ROOT=/some/root go run ./app/cmd/concontexto export

# 2. Stage those bytes into the build context, under web/:
mkdir -p web/data-derived && cp -r /some/root/data-derived/. web/data-derived/

# 3. Build against them:
docker build -f Dockerfile --build-arg EXPORT_DIR=data-derived -t concontexto .
```

Until then `docker build` exits non-zero at the web build with
instructions. That is the intended behaviour, not a gap to work around: an
image that builds by inventing data is worse than an image that does not
build.

`docker compose build app` forwards `EXPORT_URL` / `EXPORT_DIR` from the
environment (`docker-compose.yml`), so the same two forms work there:

```bash
EXPORT_DIR=data-derived docker compose build app
```

CI's own container smoke test (`ci.yml`) takes the staged route: it runs
the real end-to-end ingest-and-export first, into `web/data-derived`, and
builds the image against that. The image it smoke-tests therefore serves
real data.

## Volumes: what persists, and why the pages must not

A named Docker volume is seeded from the image **only when it is first
created**, and then persists untouched across image rebuilds and container
recreation. That is the intended behaviour for data; it is fatal for
pre-rendered pages.

`docker-compose.yml` used to mount a single `public_html` volume at
`/web/dist`. The consequence was measured on a live stack and it disables
the entire deploy pipeline: `deploy.yml` builds a new image, pushes it and
POSTs the Portainer webhook; Portainer recreates the container; the volume
survives; **the reader keeps seeing the first deploy's pages forever.** The
freshly built image carried five indicator pages rendered from a
98-to-306-observation artifact, while the served volume still held six —
including an `ocupados-epa` the current artifact no longer supports, and a
`tasa-de-paro-epa` page with four table rows and no range controls.

`/web/dist` mixes two opposite lifecycles:

| Path | Written by | Lifecycle |
|---|---|---|
| `/web/dist/**` (pages, `_astro/`) | the image build | **replaced on every deploy** |
| `/web/dist/data-derived/` | `concontexto export` at runtime | **must survive recreation** |
| `/web/dist/transparencia/` | every ingest, at runtime | **must survive recreation** |
| `/app_data/` | `filestore` + retention, at runtime | **must survive recreation** |

So the volumes are narrowed to the runtime-written subtrees only:

```yaml
volumes:
  - export_artifact:/web/dist/data-derived
  - raw_hashes:/web/dist/transparencia
  - app_data:/app_data
```

Pages now come from the image on every deploy, and the artifact behind
their download links still persists. `app_data` was checked rather than
assumed: the image contains no `/app_data` content, so that mount shadows
nothing — it was already correct.

### First boot

The image ships `/web/dist/data-derived` and `/web/dist/transparencia`, so
both volumes seed from it. In an `EXPORT_DIR` build the Dockerfile copies
the staged artifact into `dist/data-derived`, which means the CSV/JSON
download links resolve from the very first boot instead of 404ing until the
first publish cycle. In an `EXPORT_URL` build that directory ships empty,
and nothing is lost: `EXPORT_URL` names a live, already-published origin,
so the deployment being rebuilt already has a populated `export_artifact`
volume, and an existing volume is never re-seeded.

### Volume ownership

The container runs as distroless `nonroot` (uid 65532) and has no shell, so
it cannot repair permissions at start-up. Docker stamps a newly created
volume with the ownership and mode of the image directory it seeds from, so
the three mount points are `COPY --chown=65532:65532`'d in the Dockerfile's
final stage. Without that, a clean `docker compose up` failed the first
ingest with `mkdir /app_data/raw: permission denied` and needed a manual
`chown`. No operator step is required any more.

## Upgrading a stack deployed before the volume was narrowed

A stack deployed before this change has a `public_html` volume holding a
whole `dist/` — pages, `_astro/`, plus the live `data-derived/` and
`transparencia/` subtrees. Nothing mounts `public_html` any more, so **the
upgrade is correct with no manual step**: the volume is simply left
orphaned, and the new `export_artifact` and `raw_hashes` volumes are created
and seeded from the new image.

The one thing to understand before cleaning up: `export_artifact` starts
either from the image's staged artifact (`EXPORT_DIR` build) or empty
(`EXPORT_URL` build). Both are regenerable — the next scheduled ingest, or
one explicit `concontexto export`, rewrites the artifact from PostgreSQL,
which is the actual system of record and lives in `pg_data`. If you want the
published artifact restored immediately rather than at the next cycle:

```bash
# Optional: republish now instead of waiting for the scheduler.
docker compose exec app /concontexto export
```

Then remove the orphan once you are satisfied the site serves correctly:

```bash
docker volume ls                          # expect concontexto_public_html, unused
docker volume rm concontexto_public_html  # holds only regenerable content
```

Do **not** rename the new volumes back to `public_html`. Reusing the name
would mount the old whole-`dist/` content at `/web/dist/data-derived`,
publishing the previous deploy's pages under `/data-derived/` URLs beside
the real artifact.

If you would rather carry the existing artifact across explicitly instead of
re-exporting, copy it before removing the old volume:

```bash
docker run --rm \
  -v concontexto_public_html:/old \
  -v concontexto_export_artifact:/new \
  alpine:3 sh -c 'cp -a /old/data-derived/. /new/ && chown -R 65532:65532 /new'
```

## What the workflow does once the secret is configured

1. Triggers after `ci.yml` completes successfully on `main` (or on
   manual `workflow_dispatch`).
2. Builds the repository's `Dockerfile` image — passing
   `--build-arg EXPORT_URL=${{ vars.EXPORT_URL }}`, so the pages are
   rendered from the live artifact and never from the synthetic test
   fixture — and pushes it to `ghcr.io/jorgealonsodev/concontexto` tagged
   both `:latest` and with the triggering commit SHA (so a specific
   deployed build is always traceable).
3. POSTs to `PORTAINER_WEBHOOK_URL`, which tells the already-configured
   stack to redeploy — Portainer, not this workflow, decides how the
   running container is replaced.

`rebuild.yml` is the same three steps reached from the other direction: it
receives the `repository_dispatch` a publish cycle sends, verifies that the
origin at `EXPORT_URL` really is serving the artifact the dispatch names,
and then **calls `deploy.yml` as a reusable workflow** rather than restating
its build/push/redeploy steps. A rebuild triggered by data and a deploy
triggered by code must produce the same image from the same inputs, and two
copies of those steps is exactly how they would stop doing so.

If the origin is serving something **older** than the dispatched artifact,
`rebuild.yml` stops with an error instead of building: publishing pages that
do not carry the dispatched data while reporting a successful rebuild is the
one outcome worse than not rebuilding. It self-corrects — the next publish
cycle dispatches again, and the deploy-completed watchdog keeps alerting
until one lands.

This workflow never provisions the VPS or the Portainer stack itself:
that is a one-time manual infrastructure step (1 and 2 above), not
something a CI job should be trusted to do unattended against a
production host.

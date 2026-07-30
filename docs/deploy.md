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
| Live origin | `--build-arg EXPORT_URL=https://<host>/data-derived/` | Normal operation. A publishing cycle writes the artifact into the container's `public_html` volume and dispatches a rebuild, which fetches it back from the site it is rebuilding. |
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

This workflow never provisions the VPS or the Portainer stack itself:
that is a one-time manual infrastructure step (1 and 2 above), not
something a CI job should be trusted to do unattended against a
production host.

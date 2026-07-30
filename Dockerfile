# syntax=docker/dockerfile:1.7

# ConContexto — single Go binary + build-time-only Astro frontend.
# Build context MUST be the repository root (ADR-5): the Go module and the
# `//go:embed config` shim both live at the root, and `//go:embed` cannot
# traverse into a parent directory, so the module cannot be relocated to
# make the context smaller.
#
# The web build also REQUIRES an export-artifact source, passed as a build arg
# — see the web-builder stage below for why there is no default and no fallback:
#
#   docker build -f Dockerfile \
#     --build-arg EXPORT_URL=https://<host>/data-derived/ -t concontexto .

# ---- Go builder ------------------------------------------------------
FROM golang:1.26-alpine AS go-builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY app/ app/
COPY config/ config/
COPY config_embed.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
    -o /out/concontexto ./app/cmd/concontexto

# An empty /app_data skeleton, so the final image OWNS that path and the
# `app_data` named volume can be seeded from it. A named volume whose
# mount point does not exist in the image is created ROOT-owned, and this
# container runs as `nonroot` (uid 65532) on a distroless base with no
# shell — so the first ingest died on `mkdir /app_data/raw: permission
# denied` and no `RUN chown` was available to repair it. Docker copies the
# image directory's ownership and mode onto a freshly created volume, so
# shipping this directory `--chown`ed below is what makes a clean
# `docker compose up` work with no manual operator step.
RUN mkdir -p /out/app_data

# ---- Web builder (Node, build-time only — PRD §14.2: no Node in prod) -
#
# THIS STAGE REQUIRES AN EXPLICIT ARTIFACT SOURCE (verify-report CRITICAL-15).
# It used to be a bare `RUN npm run build`, and the loader
# (web/src/lib/export/loader.ts) used to fall back to `web/test/fixtures/export`
# when no source was configured. With `COPY web/ .` bringing that fixture into
# the image and no `.dockerignore` to stop it, every image built from this file
# published the fixture's SYNTHESISED history — invented numbers carrying real
# INE attribution, a real origin identifier and a real extraction timestamp —
# with no disclosure on the page. `2002-Q1: 11.85 % población activa,
# Definitivo` was read out of the real built HTML.
#
# Two independent changes close that, and both are load-bearing:
#   - the loader now REFUSES to guess (no default; an unconfigured build exits
#     non-zero, asserted by web/test/export/unconfigured-build-fails.test.ts);
#   - `.dockerignore` keeps `web/test/` out of the build context entirely, so
#     this stage cannot reach the fixture even if the code regresses.
#
# CAN THIS STAGE OBTAIN A REAL ARTIFACT ON ITS OWN? No — and that is a property
# of the architecture, not an oversight. The artifact is written by the Go
# binary from PostgreSQL at RUNTIME (`concontexto export` ->
# STATIC_ROOT/data-derived; app/cmd/concontexto/export_cmd.go), and an image
# build has neither the database nor the volume. So the artifact must be handed
# IN, one of exactly two ways (design.md D-1's "EXPORT_URL live / EXPORT_DIR
# fixture" switch):
#
#   1. From the live site, which is what the rebuild-on-publish path uses
#      (design D-2: a publishing cycle POSTs a repository_dispatch that
#      rebuilds the site against the origin the running container already
#      serves):
#
#        docker build -f Dockerfile \
#          --build-arg EXPORT_URL=https://<host>/data-derived/ -t concontexto .
#
#   2. From a real exported artifact staged into the build context. The path is
#      relative to /web because that is this stage's WORKDIR, and only `web/`
#      is copied in, so the artifact must be placed under `web/`:
#
#        mkdir -p web/data-derived   # then `concontexto export`, or the Go
#                                   # end-to-end export test, writes here
#        docker build -f Dockerfile \
#          --build-arg EXPORT_DIR=data-derived -t concontexto .
#
# THE BOOTSTRAP CASE IS REAL AND IS NOT PAPERED OVER: before the first
# publication there is no live /data-derived/ origin to point EXPORT_URL at, so
# the very first image must be built option 2's way, from an artifact a real
# ingestion produced. Until then `docker build` FAILS at the `npm run build`
# line below, loudly and with instructions. That is the intended behaviour. An
# image that builds by inventing data is worse than an image that does not
# build.
FROM node:22-alpine AS web-builder
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ .

# Declared AFTER `npm ci` on purpose: an ARG invalidates every layer below it,
# so declaring these earlier would re-run `npm ci` on every change of artifact
# source. Both default to empty, which is the "not configured" case — the
# loader treats an empty string exactly as it treats an unset variable.
ARG EXPORT_URL=""
ARG EXPORT_DIR=""

# The pre-check exists only to say the one thing the loader cannot know: that
# in a container build these values arrive as `--build-arg`. The loader remains
# the authority on what counts as a usable source, and it runs on every build
# whether or not this check passes — the two refusals are independent, and
# removing either still leaves a build that cannot silently ship fabrications.
#
# BUILD_WITH_SYNTHETIC_FIXTURE (the loader's opt-in for the synthetic fixture)
# is deliberately NOT plumbed through as a build arg. There is nothing for it to
# unlock here: `.dockerignore` has already removed the fixture from the context.
RUN if [ -z "$EXPORT_URL" ] && [ -z "$EXPORT_DIR" ]; then \
      echo "Dockerfile: the web build has no export artifact source." >&2; \
      echo "  Pass --build-arg EXPORT_URL=https://<host>/data-derived/  (a live, already-published origin)" >&2; \
      echo "  or   --build-arg EXPORT_DIR=<path under web/ in the build context>  (an artifact 'concontexto export' wrote)." >&2; \
      echo "  This image will not be built from the synthetic test fixture: see the loader's own refusal message." >&2; \
      exit 1; \
    fi; \
    EXPORT_URL="$EXPORT_URL" EXPORT_DIR="$EXPORT_DIR" npm run build

# THE DEPLOY-COMPLETED STAMP (verify-report CRITICAL-28, link 3).
#
# Nothing in the running stack could observe whether a rebuild ever landed.
# The scheduler's publish-latency watchdog compared a source's last ingestion
# success against the LOCAL manifest that `publishing.Publish` had written
# seconds earlier in the same call, so the only failure it could ever catch
# was an export that did not run. A dispatch never sent, a CI rebuild that
# failed and a Portainer redeploy that never happened were all invisible —
# and that is precisely the state the live stack was in, with
# /data-derived advancing every cycle while the pre-rendered pages stayed
# frozen at whatever artifact this stage had rendered them from.
#
# This file records that artifact. `/web/build-manifest.json` is the manifest
# of the export the pages above were BUILT from, and it can change by exactly
# one mechanism: a new image being deployed. So a running container can
# compare it against the artifact it is currently publishing and know whether
# the rebuild it triggered ever arrived, without reaching GitHub, Portainer or
# the public site. app/cmd/concontexto/schedule.go reads it
# (APP_BUILD_MANIFEST) and app/internal/scheduler.RebuildLatencyBreached is
# the decision.
#
# NOT under dist/: docker-compose.yml mounts the export_artifact volume over
# /web/dist/data-derived, so a stamp written there would be shadowed by the
# volume at runtime, and one written elsewhere in dist/ would be served to
# readers as though it were part of the published artifact.
#
# ONE HONEST LIMITATION, in the EXPORT_URL form only. The artifact is fetched
# here a second time, moments after the loader fetched it for the build, so if
# a publish cycle lands between the two fetches this stamp names the artifact
# fetched here rather than the one rendered. The window is the few seconds
# between two adjacent HTTP requests against a source that changes at most
# every 15 minutes, and the consequence of losing that race is bounded and
# self-correcting: the watchdog compares generated_at instants, so a stamp one
# cycle newer suppresses one alert until the next cycle rewrites the artifact.
# It is disclosed rather than engineered away because removing it means
# threading the loader's own fetched bytes out of the Astro build, which is a
# change to web/ this remediation does not own.
#
# In the EXPORT_DIR form there is no race at all: the manifest copied is the
# exact file the build read out of the context.
#
# A failure here FAILS THE BUILD rather than shipping an image whose deploy
# cannot be observed, which is the same choice the artifact-source pre-check
# above already makes.
RUN EXPORT_URL="$EXPORT_URL" EXPORT_DIR="$EXPORT_DIR" node <<'NODE'
const fs = require("node:fs");

const dir = process.env.EXPORT_DIR;
const url = process.env.EXPORT_URL;

async function readManifest() {
  if (dir) {
    return fs.readFileSync(`${dir}/manifest.json`, "utf-8");
  }
  const manifestURL = `${url.replace(/\/+$/, "")}/manifest.json`;
  const response = await fetch(manifestURL);
  if (!response.ok) {
    throw new Error(`fetching ${manifestURL}: HTTP ${response.status}`);
  }
  return await response.text();
}

readManifest()
  .then((text) => {
    const manifest = JSON.parse(text);
    if (!manifest.generated_at) {
      throw new Error("the artifact manifest carries no generated_at, so the deployed artifact could not be identified");
    }
    fs.writeFileSync("/web/build-manifest.json", text);
    console.log(`build-manifest stamp: these pages were rendered from the artifact generated at ${manifest.generated_at}`);
  })
  .catch((error) => {
    console.error(`build-manifest stamp: ${error.message}`);
    console.error("This image would be unable to report whether a site rebuild ever landed, so the build stops here.");
    process.exit(1);
  });
NODE

# FIRST-BOOT SEEDING OF THE RUNTIME-WRITTEN SUBTREES.
#
# `docker-compose.yml` no longer mounts a volume over the whole of
# /web/dist — that mount is what made every deploy a no-op, because a named
# volume is seeded from the image only when it is FIRST created and then
# survives every rebuild, so the pages served were forever the first
# deploy's. The volumes are now narrowed to the two subtrees the RUNNING
# container writes, and both must therefore exist in the image:
#
#   dist/data-derived  — `concontexto export` writes the published artifact
#                        here (STATIC_ROOT/data-derived, export_cmd.go).
#                        Every page's CSV/JSON download link resolves into
#                        it.
#   dist/transparencia — each ingest copies app_data/raw_files.sha256 to
#                        STATIC_ROOT/transparencia/raw-files.sha256
#                        (ingest_cmd.go: resolveIngestPaths).
#
# `astro build` produces neither, so they are created here. Without them
# the mount points would be absent from the image and Docker would create
# both volumes root-owned — the same permission failure /app_data hit.
#
# The artifact copy: in the EXPORT_DIR form the build context HOLDS the
# exact artifact these pages were rendered from, so it is shipped into
# dist/data-derived and the download links work from the very first boot
# instead of 404ing until the first publish cycle completes. This cannot
# leak the synthetic fixture: `.dockerignore` removes `web/test/` from the
# build context entirely, so no EXPORT_DIR can name it.
#
# The EXPORT_URL form ships an empty dist/data-derived, and that is
# correct rather than a gap: EXPORT_URL points at a live, already-published
# origin, which by definition means the deployment it rebuilds ALREADY has
# a populated `export_artifact` volume. A volume is seeded only at
# creation, so nothing this branch could copy would ever be read.
RUN mkdir -p dist/data-derived dist/transparencia; \
    if [ -n "$EXPORT_DIR" ]; then cp -R "$EXPORT_DIR"/. dist/data-derived/; fi

# ---- Final image -------------------------------------------------------
# distroless: no shell, no package manager, no curl — this is the entire
# reason the `healthcheck` subcommand exists (exec-form HEALTHCHECK below
# re-invokes the binary instead of shelling out).
FROM gcr.io/distroless/static-debian12:nonroot AS final

COPY --from=go-builder /out/concontexto /concontexto

# `--chown=65532:65532` is the distroless `nonroot` uid:gid, spelled
# numerically because this stage has no shell and no name resolution during
# COPY. It matters for exactly one reason: /web/dist/data-derived,
# /web/dist/transparencia and /app_data are named-volume mount points, and
# Docker stamps a newly created volume with the ownership and mode of the
# image directory it seeds from. Root-owned mount points here mean a
# root-owned volume, which a `nonroot` process cannot write — the failure
# that had to be repaired by hand with `docker run ... chown` on the first
# deployment of this stack.
COPY --from=web-builder --chown=65532:65532 /web/dist /web/dist
# The deploy-completed stamp (see the web-builder stage). Outside /web/dist
# on purpose: no volume shadows it and no HTTP route serves it.
COPY --from=web-builder --chown=65532:65532 /web/build-manifest.json /web/build-manifest.json
COPY --from=go-builder --chown=65532:65532 /out/app_data /app_data

USER nonroot:nonroot
ENV PORT=8080
ENV STATIC_ROOT=/web/dist
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/concontexto", "healthcheck"]

ENTRYPOINT ["/concontexto"]
CMD ["serve"]

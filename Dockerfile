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

# ---- Final image -------------------------------------------------------
# distroless: no shell, no package manager, no curl — this is the entire
# reason the `healthcheck` subcommand exists (exec-form HEALTHCHECK below
# re-invokes the binary instead of shelling out).
FROM gcr.io/distroless/static-debian12:nonroot AS final

COPY --from=go-builder /out/concontexto /concontexto
COPY --from=web-builder /web/dist /web/dist

USER nonroot:nonroot
ENV PORT=8080
ENV STATIC_ROOT=/web/dist
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/concontexto", "healthcheck"]

ENTRYPOINT ["/concontexto"]
CMD ["serve"]

# syntax=docker/dockerfile:1.7

# ConContexto — single Go binary + build-time-only Astro frontend.
# Build context MUST be the repository root (ADR-5): the Go module and the
# `//go:embed config` shim both live at the root, and `//go:embed` cannot
# traverse into a parent directory, so the module cannot be relocated to
# make the context smaller.
#
#   docker build -f Dockerfile -t concontexto .

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
FROM node:22-alpine AS web-builder
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ .
RUN npm run build

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

# Delta for platform-runtime

Slice 1 · Milestone 0.1. Greenfield capability — no existing spec to modify.

## ADDED Requirements

### Requirement: Single binary with five subcommands

The system MUST ship one Go binary exposing exactly the subcommands `serve`, `ingest`, `migrate`, `validate-config` and `healthcheck`. `serve` and `ingest` MUST be driving adapters over the same domain core.

#### Scenario: Every subcommand is dispatchable

- GIVEN the built binary
- WHEN it is invoked with each of `serve`, `ingest`, `migrate`, `validate-config`, `healthcheck`
- THEN each subcommand is recognised and executes its own entry point
- AND an unknown subcommand exits non-zero with a usage message

### Requirement: Zero database access at page-request time

The static-file handler MUST NOT depend on any repository port. No database query and no computation SHALL occur while serving a page request (PRD §14.2 golden rule).

#### Scenario: Static handler has no repository dependency

- GIVEN the HTTP driving adapter
- WHEN the static-file handler is constructed
- THEN its constructor accepts no repository port of any kind
- AND removing the database from the process still serves every static asset

#### Scenario: Serving pages issues no queries

- GIVEN a `serve` process with an instrumented database connection
- WHEN 100 static page requests are served
- THEN the recorded query count is zero

### Requirement: No external source call at page-request time

The `serve` path MUST NOT call any external data source. Ever (PRD §9.2).

#### Scenario: Request path performs no outbound source call

- GIVEN a `serve` process with outbound HTTP to source hosts blocked
- WHEN any page or `/healthz` request is served
- THEN the response succeeds and no outbound source request is attempted

### Requirement: Health endpoint and shell-free healthcheck

The system MUST expose `GET /healthz` returning HTTP 200 when the process is serving. The `healthcheck` subcommand MUST perform an HTTP GET against `127.0.0.1:$PORT/healthz` and exit 0 on success or 1 on failure. It MUST support a `--deep` flag that additionally verifies PostgreSQL reachability. The container `HEALTHCHECK` MUST use exec form because the distroless image has no shell.

#### Scenario: Shallow healthcheck succeeds

- GIVEN a running `serve` process
- WHEN the binary is re-invoked as `healthcheck`
- THEN it exits 0

#### Scenario: Deep healthcheck fails when the database is unreachable

- GIVEN a running `serve` process and an unreachable PostgreSQL
- WHEN the binary is invoked as `healthcheck --deep`
- THEN it exits 1
- AND a plain `healthcheck` without `--deep` still exits 0

### Requirement: Migrations run only on explicit command

Schema migrations MUST run only via the `migrate` subcommand and MUST NOT run on process boot.

#### Scenario: Boot does not migrate

- GIVEN a database whose schema is one migration behind
- WHEN `serve` starts
- THEN no migration is applied
- AND the schema version is unchanged

### Requirement: Cache headers owned by the Go handler

Cache-control headers for static assets MUST be set by the Go handler that serves them, as the single source of truth (PRD §14.3). Hashed assets MUST receive long-lived `immutable` caching.

#### Scenario: Hashed asset receives immutable caching

- GIVEN a content-hashed asset path
- WHEN it is requested
- THEN the response carries a long `max-age` and `immutable`
- AND a non-hashed HTML document does not

### Requirement: Container and deployment shape

The image MUST be a distroless multi-stage build. The compose stack MUST apply hard limits of 256 MB RAM for `app` and 512 MB for `postgres`, attach `app` to the external `proxy` network and `postgres` only to the stack-internal network, and publish NO ports to the host. Log rotation MUST use `json-file` with `max-size: 10m`, `max-file: 3`, `compress: true`.

#### Scenario: Postgres is unreachable from outside the stack

- GIVEN the deployed stack
- WHEN a connection to PostgreSQL is attempted from the host or from the `proxy` network
- THEN the connection fails
- AND `app` still connects over the internal network

#### Scenario: No service publishes a host port

- GIVEN `docker-compose.yml`
- WHEN service definitions are inspected
- THEN no service declares a host port mapping

### Requirement: Four-eyes protection on editorial configuration

The repository MUST enforce, through CODEOWNERS plus branch protection, that a change touching `/config/**` cannot merge with a single approval (PRD §9.6, §15.2).

#### Scenario: Single approval is rejected on a config change

- GIVEN a pull request modifying a file under `/config/`
- WHEN it has exactly one approval
- THEN the repository blocks the merge
- AND merging becomes possible only after a second approval from a code owner

### Requirement: Continuous integration and automatic deploy

A push to `main` MUST run the CI pipeline and, when green, produce an automatically deployed hello-world static page. CI MUST run `go test ./...`, `validate-config`, and the frontend build.

#### Scenario: Pushed commit deploys hello-world

- GIVEN a green CI run on `main`
- WHEN the deploy step completes
- THEN the public URL serves the hello-world static page
- AND `/healthz` returns HTTP 200

#### Scenario: Failing tests block deploy

- GIVEN a commit whose `go test ./...` fails
- WHEN CI runs
- THEN the pipeline fails and no deploy occurs

### Requirement: Backup script shipped

The repository MUST ship a `pg_dump`-based backup script and document the consistency requirement that a hot file-level copy of `pg_data` is insufficient (PRD §14.3). Wiring it into the VPS backup system is explicitly out of scope.

#### Scenario: Backup script produces a restorable dump

- GIVEN a running PostgreSQL container with seeded data
- WHEN the shipped backup script runs
- THEN it produces a dump file
- AND restoring it into an empty database reproduces the seeded rows

# Delta for platform-runtime

Slice **3–4**. Delta against `openspec/specs/platform-runtime/spec.md`. Sources: ADR-7, design D-2.

Slice 3 added a standalone `export` subcommand (task 3.11) as this change's own design D-2 explicitly
directs, and updated `main_test.go`'s guard test (`TestRealCommands_ExposesExactlySixRequiredSubcommands`)
to match — but disclosed, at the time, that the merged `platform-runtime` spec still documented "five
subcommands" and that this delta was still owed before the change archives. This file closes that gap.

## MODIFIED Requirements

### Requirement: Single binary with six subcommands

The system MUST ship one Go binary exposing exactly the subcommands `serve`, `ingest`, `migrate`,
`validate-config`, `healthcheck` and `export`. `serve` and `ingest` MUST be driving adapters over the same
domain core. `export` MUST run the same read → validate → write path `ingest` triggers automatically after
a successful cycle (design D-2), on demand — for recovery, boot-time self-heal, and fixture generation.
(Previously: five subcommands, `export` not yet named.)

#### Scenario: Every subcommand is dispatchable

- GIVEN the built binary
- WHEN it is invoked with each of `serve`, `ingest`, `migrate`, `validate-config`, `healthcheck`, `export`
- THEN each subcommand is recognised and executes its own entry point
- AND an unknown subcommand exits non-zero with a usage message

#### Scenario: `export` runs independently of `ingest`

- GIVEN a database already holding published observations
- WHEN `export` is invoked with no preceding `ingest` in the same process
- THEN it produces a valid export artifact from the currently published data
- AND it requires no source-adapter or scheduler dependency

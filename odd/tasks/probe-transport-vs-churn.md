# probe-transport-vs-churn

## Objective
Stop the daily `probe` workflow from emailing a failure when the only thing
wrong is that INE refuses connections from GitHub-hosted runners, while
keeping a real hard failure when an identifier or response shape actually
breaks.

## Problem (verified 2026-09-17)
1. `servicios.ine.es` answers HTTP 200 in 0.32s from a normal host
   (`IPC290751` -> `Fecha: 2026-08-01`, `Valor: 104.638`) but gives
   `connect: connection refused` / header timeouts to the runner from the
   SAME IP `195.254.149.35`. Eurostat passes on the runner in 0.2-1.7s, so
   runner egress is healthy. No identifier churned.
2. The probe cannot tell "unreachable" from "churned": both print the same
   `possible identifier churn` message from `live_test.go`.
3. `defaultMaxAttempts = 5` x 30s timeout + 5s backoff = ~155s per dead INE
   target; 6 INE targets ~= 15.5 min, so Go's DEFAULT 10m `-timeout` fires
   first and the binary PANICS, killing the run before the last target
   reports. The `timeout-minutes: 15` job ceiling never engages.

## Why
An alarm that fires daily for a non-problem gets muted, and then the real
identifier churn it exists to catch goes unnoticed.

## Key insight
`ingestion/sourceerr` ALREADY encodes the needed distinction. No new
taxonomy: `RetryableTransport` means the source never answered (network
error, timeout, 5xx) = not churn. Every other class means the source DID
answer and the answer was wrong = real signal.

## Scope (authorized)
- `app/internal/probe/outcome.go` (new, package `probe`, NO build tag)
- `app/internal/probe/outcome_test.go` (new, offline)
- `app/internal/probe/live_test.go` (modify)
- `.github/workflows/probe.yml` (modify)
Out of scope: adapter defaults, ci.yml, config, any self-hosted runner.

## TDD
Enabled (strict, global config). Runner: `go test -race -count=1 ./...`.
RED -> GREEN -> REFACTOR on `outcome_test.go`.

## Tasks
- [x] T1 RED: `outcome_test.go` covering nil/Retryable/wrapped/4 non-retryable
      classes/unclassified-fails-closed. Observe it fail to compile/pass.
- [x] T2 GREEN: `outcome.go` with `Outcome` + `Classify(err) Outcome`.
- [x] T3 Rewire `live_test.go`: per-target deadline, bounded attempts, short
      probe HTTP timeout, `t.Skipf` on Unreachable, `t.Fatalf` on Broken.
- [x] T4 `probe.yml`: explicit `-timeout` below the job ceiling; correct the
      stale WARNING-W6 comment that assumed the job ceiling was binding.
- [x] T5 Verify: `go test -race -count=1 ./app/internal/probe/...` and
      `go vet ./...`; confirm `-tags live` still COMPILES.

## Acceptance
- Unreachable INE => job GREEN, `--- SKIP` lines naming the target, no email.
- Broken shape/identifier => job RED (still alerts).
- Unclassified error => treated as Broken (fail closed).
- Probe run completes well inside its `-timeout`; no panic.

## Known limitation to report
While INE blocks GitHub egress, INE churn detection from CI is effectively
ZERO. This change removes false alarms; it does not restore INE coverage.

## Progress
All tasks (T1-T5) complete.

- T1 RED observed: `go test -race -count=1 ./app/internal/probe/...`
  failed with `probe [build failed]` -- `undefined: probe.Classify`,
  `undefined: probe.OutcomeHealthy`, `undefined: probe.OutcomeUnreachable`
  (each referenced 2-3x across the 5 test functions), confirming
  `outcome_test.go` did not compile before `outcome.go` existed.
- T2 GREEN: `app/internal/probe/outcome.go` added (`Outcome` type,
  `OutcomeHealthy`/`OutcomeUnreachable`/`OutcomeBroken`, `Classify(err)`
  via `errors.As` + `sourceerr.FailureClass.Retryable()`, fails closed to
  Broken for any unrecognised error). `go test -race -count=1 -v
  ./app/internal/probe/...` -> `11 passed in 1 packages`.
- T3: `live_test.go` rewired -- per-target `context.WithTimeout`
  (`perTargetTimeout` = 25s), both clients built with a bounded
  `&http.Client{Timeout: probeHTTPTimeout}` (10s) and
  `WithMaxAttempts(probeMaxAttempts)` (2), fetch and decode errors both
  routed through one new `reportOutcome` helper (`probe.Classify` ->
  `t.Skipf` on Unreachable / `t.Fatalf` on Broken / pass-through on
  Healthy), header comment rewritten to explain the transport-vs-shape
  split. `go vet -tags live ./app/internal/probe/...` clean.
- T4: `.github/workflows/probe.yml` -- added explicit `-timeout 8m` to
  the `go test -tags live` step; rewrote the WARNING-W6 comment above
  `timeout-minutes: 15`, which had falsely claimed the job ceiling was an
  independent second bound -- `go test`'s own DEFAULT `-timeout` is 10m
  (below 15m and never overridden), which is what actually fired and
  panicked the binary per the verified problem. Comment now states the
  real ordering: explicit 8m inner bound, 15m outer job ceiling as
  intended last-resort backstop.
- T5 verification, all commands run, actual output:
  - `go test -race -count=1 -v ./app/internal/probe/...` ->
    `Go test: 11 passed in 1 packages`
  - `go vet ./...` -> no output (clean)
  - `go build -tags live ./...` -> no output (clean)
  - `go vet -tags live ./app/internal/probe/...` -> no output (clean)
  - `go test -count=1 ./...` -> `Go test: 685 passed in 25 packages`
  - `go test -count=1 -run TestLiveProbe -v ./app/internal/probe/...`
    (no `-tags live`) -> `Go test: No tests found`, confirming the live
    suite still does not compile/run in the default suite.
- `git status --porcelain` after all edits: only
  `.github/workflows/probe.yml` (modified), `app/internal/probe/live_test.go`
  (modified), `app/internal/probe/outcome.go` (new),
  `app/internal/probe/outcome_test.go` (new) touched -- no file outside
  scope. No commit made (not authorized).
- Known limitation stands as recorded above: while INE blocks
  GitHub-hosted runner egress, this change removes the false alarm but
  does not restore actual INE churn detection from CI.

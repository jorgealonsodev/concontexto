# Embedded configuration

This directory is compiled into the binary by the root-level
`config_embed.go` (`//go:embed config`, ADR-1 / ADR-5). Config version is
pinned to binary version atomically: mutating a file here after the
binary is built does not change what the running process reads until a
rebuild + redeploy (see `config_embed_test.go` at the repository root).

## Layout

- `sources/{source}.yaml` — source identity, licence, attribution,
  access type and redistribution terms (spec source-attribution-licensing).
  `ine.yaml` and `eurostat.yaml` are authored here (PR 3 of
  `phase-0-data-foundations`).
- `series/{slug}.yaml` — canonical slug, validity-ranged source
  references, unit, frequency, decimals, per-series validation thresholds
  (spec editorial-config). Arrives in Phase 5b (INE) and Phase 6
  (Eurostat) of `phase-0-data-foundations`.
- `rupturas.yaml`, `eventos.yaml`, `gobiernos.yaml` — editorial break/event
  entries (PRD §9.6 filenames, kept in Spanish). Authored in Phase 7 (PR 7)
  of `phase-0-data-foundations` and reconciled by
  `ingestion.ReconcileEditorialConfig` into `series_break`/`event`. An
  entry whose effective date is not yet confirmed against its source's own
  methodological note carries `date_status: unconfirmed` + `todo` instead
  of a guessed date; ReconcileEditorialConfig never projects such an entry
  into the database (see `app/internal/ingestion/reconcile.go`).
- `embed-marker.txt` — a fixed-content fixture with no data meaning of
  its own; it exists only so `config_embed_test.go` can prove the
  embedded bytes are unaffected by an on-disk mutation after compile.
  Do not treat it as configuration.

Every file here is schema-validated by the `validate-config` subcommand,
which CI runs as a blocking gate (spec editorial-config,
"validate-config subcommand").

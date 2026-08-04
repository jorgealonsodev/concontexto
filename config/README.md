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
  An event entry may also declare an optional `scope`
  (`global` | `series` | `dataset` | `source`) and an optional `source_url`.
  Every entry authored today is `global` — a change of government and a
  worldwide shock are facts about the calendar and apply wherever the
  calendar does — and omitting `scope` means exactly that; the loader
  normalises it to `global` so no reader downstream has to decide what an
  empty value means. A narrower scope widens `series` ⊂ `dataset` ⊂ `source`
  exactly as a break's does (`postgres.ListActiveEvents`), so an entry is
  stored once rather than once per series.
- `reconocimientos.yaml` — the editorial acknowledgement registry (spec
  data-validation, "Acknowledged findings"), reconciled by
  `ingestion.ReconcileEditorialConfig` into `validation_acknowledgement`.
  One entry records that a **named human** reviewed **one specific**
  blocking validation finding and confirmed the underlying datum; the
  publish gate then treats that finding as informational and records the
  run as `succeeded-with-acknowledgement`, never as a plain success.
  An entry's scope is exactly one series, one period and one rule, matched
  by exact equality — the schema has no syntax for "every period" or
  "every rule", and only `rule3-plausibility` and `rule4-revision` admit
  one at all. The required `value` field pins the number the reviewer
  looked at, so a later revision of that datum invalidates the
  acknowledgement (raising a blocking `acknowledgement-stale` finding)
  rather than silently inheriting the approval.
  **A record's authority is the human signature.** `signature_status:
  unsigned` marks a draft — research checked in for review, carrying
  `drafted_by` and `todo` and *no* signature. A draft is never projected
  into the database and resolves nothing, the same discipline
  `date_status: unconfirmed` applies in `rupturas.yaml`: never project an
  unverified fact. `validate-config` rejects a placeholder signature and
  rejects any record that is both a draft and signed. The single entry
  shipped today is **unsigned**, so `ocupados-epa` remains blocked until a
  person reviews and signs it. See
  `app/internal/adapters/config/acknowledgement.go` and
  `app/internal/ingestion/validation/acknowledgement.go`.
- `embed-marker.txt` — a fixed-content fixture with no data meaning of
  its own; it exists only so `config_embed_test.go` can prove the
  embedded bytes are unaffected by an on-disk mutation after compile.
  Do not treat it as configuration.

Every file here is schema-validated by the `validate-config` subcommand,
which CI runs as a blocking gate (spec editorial-config,
"validate-config subcommand").

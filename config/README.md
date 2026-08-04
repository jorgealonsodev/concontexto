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
  entries (PRD §9.6 filenames, kept in Spanish; `medidas.yaml` below is a
  fourth file in the same family and under the same review discipline).
  Authored in Phase 7 (PR 7)
  of `phase-0-data-foundations` and reconciled by
  `ingestion.ReconcileEditorialConfig` into `series_break`/`event`. An
  entry whose effective date is not yet confirmed against its source's own
  methodological note carries `date_status: unconfirmed` + `todo` instead
  of a guessed date; ReconcileEditorialConfig never projects such an entry
  into the database (see `app/internal/ingestion/reconcile.go`).
- `medidas.yaml` — the policy-measures registry, reconciled by
  `ingestion.ReconcileEditorialConfig` into `event` with
  `event_group: measures` (assigned by the loader, like `gobiernos.yaml`'s
  own group). One entry is one legal instrument: a stable id, the norm's
  official name, its date of **entry into force**, the `source_url` the date
  was verified against, and a `scope`.
  **The scope is required and may not be `global`.** A measure is addressed
  at a specific market — a labour-market reform belongs on the EPA charts and
  is noise on an IPC chart — so `validate-config` rejects an unscoped one.
  The scope widens `series` ⊂ `dataset` ⊂ `source` exactly as a break's does
  (`postgres.ListActiveEvents`), so one `dataset: ine-epa` entry reaches every
  EPA series without being written twice.
  **`source_url` is required too.** The whole content of the annotation is a
  date, and a date nobody can check against a primary source is an editorial
  assertion rather than a fact.
  **This registry says nothing about whether a measure worked, and cannot.**
  There is no field for an effect, an outcome, a direction or an evaluation —
  here, in `config.EventConfig`, in the `event` table, in the export artifact
  or in the chart. The chart marks each date in the axis margin, below the
  tick labels and outside the plot area, so the mark never crosses the data;
  the sentence beside the chart states outright that the drawing represents no
  relation between those measures and the series. Same structural refusal
  `gobiernos.yaml` already applies to party colour (PRD §12.1).
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

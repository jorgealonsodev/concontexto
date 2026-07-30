-- 0006_validation_acknowledgement.up.sql
--
-- Additive: one new table, no change to any existing one.
--
-- Closes the gap verify-report WARNING-31 named: validation/types.go has
-- declared SeverityBlockRequiresSignoff since PR 4b and
-- rule4_revision.go documents its purpose at length -- "a deep revision
-- is either legitimate (a national-accounts methodology revision) or a
-- parser bug silently rewriting history ... SeverityBlockRequiresSignoff
-- hands the decision to a human rather than guessing either way" --
-- while gate.go treated it identically to SeverityBlock and nothing
-- anywhere resolved it. The comment described handing a decision to a
-- human and gave the human no way to hand it back, so every blocked
-- series stayed blocked forever, absent from the export artifact, its
-- page returning 404.
--
-- Editorial, not observable: this table is a projection of
-- config/reconocimientos.yaml through
-- ingestion.ReconcileEditorialConfig, exactly like series_break and
-- event (design.md "editorial YAML is reconciled -- never hand-edited").
-- The YAML is the source of truth; a row here is never authored by hand.
--
-- SCOPE COLUMNS. series_id, period and rule are all NOT NULL and are
-- matched by exact equality at run time. There is deliberately no
-- wildcard column, no period range and no "applies to every rule" form:
-- an acknowledgement is scoped exactly as narrowly as the finding it
-- resolves. A registry able to express "ignore rule 3 for this series"
-- would be worse than the gap it fills, so the schema simply has no way
-- to say it.
--
-- acknowledged_value IS THE ANTI-STALENESS GUARD, and is the reason this
-- table is not just an allowlist. It pins the exact number the reviewing
-- human looked at. validation.GateWithAcknowledgements applies the
-- acknowledgement only when the incoming observation at `period` still
-- equals this value; any difference means the datum was revised after it
-- was approved, the approval is no longer about the number in front of
-- the gate, and the run blocks again with an explicit
-- acknowledgement-stale finding rather than silently inheriting a stale
-- human decision. numeric, not double precision, for the same reason
-- observation.value is numeric: the pinned figure is a published decimal
-- and must round-trip as one.
--
-- NO FOREIGN KEY on series_id, deliberately, and for the same reason
-- migration 0005's discontinued_successor_slug carries none. `ingest
-- --reconcile` is its own invocation and runs BEFORE `ingest --series`
-- has called ReconcileDimensions, so on a fresh database the series rows
-- need not exist yet; an FK would make the reconcile order load-bearing
-- for a reference already gated upstream. validate-config rejects an
-- acknowledgement naming no configured series, with a message that names
-- the file and the field -- which is where a referential guarantee
-- belongs, rather than as a constraint violation surfacing mid-ingest.
--
-- retired_at, not DELETE: an approval is audit history. Removing an entry
-- from the YAML soft-retires the row (principle P7, errors and decisions
-- are documented, never erased), and restoring the YAML un-retires that
-- same row rather than inserting a fresh one.
CREATE TABLE validation_acknowledgement (
  ack_key text PRIMARY KEY,                  -- stable id from reconocimientos.yaml
  series_id text NOT NULL,                   -- exactly one series (no FK: see above)
  period text NOT NULL,                      -- exactly one canonical period: '2020-Q2'|'2026-06'|'2026'
  rule text NOT NULL,                        -- exactly one rule: 'rule3-plausibility'|'rule4-revision'
  acknowledged_value numeric NOT NULL,       -- the pinned value; see above
  acknowledged_by text NOT NULL,             -- the named human, never a role or a team
  acknowledged_on date NOT NULL,
  note_md text NOT NULL,                     -- why, in prose
  source_url text,                           -- the source document, where one exists
  config_digest text NOT NULL,
  retired_at timestamptz
);

-- Database-enforced invariant, the same discipline as one_current_row
-- (ADR-4): PostgreSQL itself, not application code, refuses two LIVE
-- acknowledgements covering one finding. Which of two conflicting human
-- approvals applies is not a question this pipeline should ever have to
-- answer at run time. Partial on retired_at IS NULL so a retired record
-- and its live replacement can coexist as history.
CREATE UNIQUE INDEX one_live_acknowledgement_per_finding
  ON validation_acknowledgement (series_id, period, rule) WHERE retired_at IS NULL;

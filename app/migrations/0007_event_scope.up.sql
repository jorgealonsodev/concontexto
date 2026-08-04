-- 0007_event_scope.up.sql
--
-- Additive: three new columns on `event`, no change to any other table
-- and no change to any existing row's meaning.
--
-- WHY. Until now `event` carried no scope columns at all, and
-- postgres.ListActiveEvents said so in its own doc comment: "every
-- currently active event is, by the schema this change inherited,
-- global", with seriesID accepted only "to leave room for a real
-- per-series scope in a future slice". That was honest for the three
-- groups the registry held: a change of government and a worldwide shock
-- are facts about the calendar and apply wherever the calendar does.
--
-- It stops being honest the moment the registry holds a POLICY MEASURE.
-- A labour-market reform is addressed at a specific market: it belongs on
-- the EPA charts and is noise on an IPC chart. Without a scope the only
-- available behaviours were to publish every measure on every chart or to
-- publish none, and both are worse than the gap.
--
-- SHAPE BORROWED WHOLESALE from series_break (migration 0001), which
-- solved this exact problem for ruptures: scope_kind + scope_ref, widened
-- at read time by series ⊂ dataset ⊂ source, so one dataset-scoped row
-- resolves for every series under it and is stored ONCE rather than once
-- per member (spec editorial-config, "it is stored once, not once per
-- series"). One difference, and it is the reason these are separate
-- columns rather than a reuse of the break's own: 'global' is a fourth
-- kind that a break cannot have and an event must, because it is what
-- every event written before this migration means.
--
-- DEFAULT 'global' IS LOAD-BEARING, not convenience. Every row already in
-- this table -- eight governments, four exogenous shocks and milestones --
-- was authored under a schema in which "applies everywhere" was the only
-- thing an entry could mean. Backfilling them as global states exactly
-- that prior meaning rather than inventing a narrower one, and it is why
-- this migration needs no data step and changes no reader-facing page.
--
-- NO CHECK CONSTRAINT on scope_kind, deliberately, and for the same
-- reason series_break carries none on its own: the enumeration lives in
-- config/validate.go, which rejects an unrecognised value at the gate with
-- a message naming the file and the field. A database constraint would
-- surface the same mistake as an opaque failure mid-reconcile, after the
-- point at which anyone is reading.
--
-- NO FOREIGN KEY on scope_ref, for the reason migrations 0005 and 0006
-- already record: `ingest --reconcile` runs before ReconcileDimensions
-- has created the series/dataset/source rows, so on a fresh database the
-- referent need not exist yet. validate-config is where a dangling ref is
-- caught.
ALTER TABLE event
  ADD COLUMN scope_kind text NOT NULL DEFAULT 'global',   -- 'global'|'series'|'dataset'|'source'
  ADD COLUMN scope_ref  text NOT NULL DEFAULT '',         -- id within scope_kind; '' when global
  ADD COLUMN source_url text;                             -- the document the entry was verified against

-- The read path filters on (scope_kind, scope_ref) for every published
-- series on every export. Small table, so this is about keeping the plan
-- honest as the registry grows rather than about today's row count.
CREATE INDEX event_scope ON event (scope_kind, scope_ref) WHERE retired_at IS NULL;

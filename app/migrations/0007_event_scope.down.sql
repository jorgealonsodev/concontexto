-- 0007_event_scope.down.sql
--
-- Reverses 0007_event_scope.up.sql exactly. The three columns are a
-- projection of config/{eventos,medidas,gobiernos}.yaml through
-- ingestion.ReconcileEditorialConfig, so dropping them loses no authored
-- fact: the registry itself, with its full provenance and its citations,
-- lives in the repository and is re-projected on the next reconcile.
--
-- What DOES change while the columns are absent is behaviour, and it is
-- worth naming: every event becomes global again, so a dataset-scoped
-- policy measure would be published on every chart in the portal rather
-- than on the ones it is addressed at. That is the direction that shows a
-- reader more than the registry claims, so a rollback of this migration
-- must be paired with a rollback of the code that reads the columns --
-- which refuses to compile against a schema without them, by construction.
DROP INDEX event_scope;

ALTER TABLE event
  DROP COLUMN scope_kind,
  DROP COLUMN scope_ref,
  DROP COLUMN source_url;

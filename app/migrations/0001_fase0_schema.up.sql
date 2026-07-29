-- 0001_fase0_schema.up.sql
--
-- Fase 0 schema (design.md "Schema", settled D3): exactly ten tables.
-- Table order follows foreign-key dependency order so this file can be
-- applied top to bottom with no forward references.
--
-- Explicitly EXCLUDED by decision D3 (no Fase 0 consumer): indicator_page,
-- verification, glossary, correction. Do not add them here.

CREATE TABLE source (
  id text PRIMARY KEY,
  name text NOT NULL,
  url text NOT NULL,
  license text NOT NULL,
  attribution_text text NOT NULL,
  access_type text NOT NULL,
  config_digest text NOT NULL,
  retired_at timestamptz
);

CREATE TABLE dataset (
  id text PRIMARY KEY,
  source_id text NOT NULL REFERENCES source,
  name text NOT NULL,
  refresh_calendar text,
  config_digest text NOT NULL,
  retired_at timestamptz
);

CREATE TABLE series (
  id text PRIMARY KEY,                       -- = slug
  dataset_id text NOT NULL REFERENCES dataset,
  name text NOT NULL,
  unit text NOT NULL,
  frequency text NOT NULL,                   -- 'M'|'Q'|'A'
  geo text NOT NULL,
  decimals int NOT NULL,
  is_harmonized bool NOT NULL,
  config_digest text NOT NULL,
  retired_at timestamptz
);

CREATE TABLE series_source_mapping (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  series_id text NOT NULL REFERENCES series,
  ref_kind text NOT NULL,                    -- 'ine-series-cod'|'eurostat-dataset'|'xlsx-url'
  ref text NOT NULL,
  ref_params jsonb,                          -- e.g. pinned Eurostat filters
  valid_from date NOT NULL,
  valid_to date,                             -- NULL = active
  config_digest text NOT NULL,
  retired_at timestamptz
);

-- Exactly one active (open-ended, non-retired) mapping per series at a
-- time; source identifier churn is recorded as a new row, never an
-- overwrite (spec data-model-vintages, "Source identifiers are
-- validity-ranged mappings").
CREATE UNIQUE INDEX one_active_mapping ON series_source_mapping(series_id)
  WHERE valid_to IS NULL AND retired_at IS NULL;

CREATE TABLE raw_file (
  hash text PRIMARY KEY,                     -- sha256 hex
  source_id text NOT NULL REFERENCES source,
  url text NOT NULL,
  downloaded_at timestamptz NOT NULL,
  storage_path text NOT NULL,
  size_bytes bigint NOT NULL
);

CREATE TABLE download_attempt (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  source_id text NOT NULL REFERENCES source,
  url text NOT NULL,
  attempted_at timestamptz NOT NULL,
  resulting_hash text REFERENCES raw_file,   -- NULL on failure
  outcome text NOT NULL                      -- 'new-file'|'unchanged'|'retryable-transport'|
                                              -- 'source-refusal'|'silent-empty'|'schema-drift'|'response-too-large'
);

CREATE TABLE ingestion_run (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  dataset_id text NOT NULL REFERENCES dataset,
  series_id text REFERENCES series,          -- set for per-series fetches (INE)
  started_at timestamptz NOT NULL,
  finished_at timestamptz,
  raw_file_hash text REFERENCES raw_file,    -- NULL if fetch failed
  outcome text NOT NULL                      -- 'pending'|'succeeded'|'validation-failed'|'fetch-failed'|'nothing-new'
);

CREATE TABLE observation (
  series_id text NOT NULL REFERENCES series,
  period text NOT NULL,                      -- canonical: '2026-Q2'|'2026-06'|'2026'
  version int NOT NULL,                      -- per-(series,period) monotonic
  value numeric,                             -- NULL only for tombstones
  status text NOT NULL,                      -- 'P' provisional |'D' definitive |'W' withdrawn
  extracted_at timestamptz NOT NULL,
  ingestion_run_id bigint NOT NULL REFERENCES ingestion_run,
  is_current bool NOT NULL DEFAULT true,
  superseded_at timestamptz,
  PRIMARY KEY (series_id, period, version),
  CHECK (value IS NOT NULL OR status = 'W')
);

-- Database-enforced invariant (ADR-4, spec data-model-vintages "The
-- database enforces exactly one current row"): PostgreSQL itself, not
-- application code, rejects a second current row for the same
-- (series_id, period).
CREATE UNIQUE INDEX one_current_row ON observation(series_id, period) WHERE is_current;

CREATE TABLE series_break (
  break_key text NOT NULL,                   -- stable id from rupturas.yaml
  scope_kind text NOT NULL,                  -- 'series'|'dataset'|'source'
  scope_ref text NOT NULL,                   -- id within scope_kind
  date date NOT NULL,
  kind text NOT NULL,
  note_md text NOT NULL,
  source_url text,
  config_digest text NOT NULL,
  retired_at timestamptz,
  PRIMARY KEY (break_key, scope_kind, scope_ref)
);

CREATE TABLE event (
  id text PRIMARY KEY,                       -- stable id from YAML
  event_group text NOT NULL,                 -- 'exogenous'|'governments'|'milestones'
                                              -- (named event_group, not "group": SQL reserved word)
  name text NOT NULL,
  date_start date NOT NULL,
  date_end date,
  note_md text,
  config_digest text NOT NULL,
  retired_at timestamptz
);

-- 0001_fase0_schema.down.sql
--
-- Reverses 0001_fase0_schema.up.sql exactly: drops the ten Fase 0 tables
-- in reverse dependency order so no FK ever blocks a DROP. Indexes
-- (including the partial unique indexes) are dropped automatically with
-- their owning table.

DROP TABLE event;
DROP TABLE series_break;
DROP TABLE observation;
DROP TABLE ingestion_run;
DROP TABLE download_attempt;
DROP TABLE raw_file;
DROP TABLE series_source_mapping;
DROP TABLE series;
DROP TABLE dataset;
DROP TABLE source;

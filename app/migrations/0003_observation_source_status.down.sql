-- 0003_observation_source_status.down.sql
--
-- Reverses 0003_observation_source_status.up.sql exactly. Dropping the
-- column loses only the verbatim-token annotation, never an observation
-- row, its value, its domain status, or its version (spec
-- data-model-vintages, "The migration reverses without data loss").
ALTER TABLE observation DROP COLUMN source_status;

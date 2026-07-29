-- 0002_observation_rollback_reason.down.sql
--
-- Reverses 0002_observation_rollback_reason.up.sql exactly.

ALTER TABLE observation DROP COLUMN rollback_reason;

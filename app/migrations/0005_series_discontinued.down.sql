-- 0005_series_discontinued.down.sql
--
-- Reverses 0005_series_discontinued.up.sql exactly. Dropping the two
-- columns loses only the editorial discontinuation annotation, which is
-- reconciled from config/series/{slug}.yaml on every ingest cycle and is
-- therefore fully reconstructible; every series identity field, its
-- observations and its source mappings are untouched.
ALTER TABLE series DROP COLUMN discontinued_successor_slug;
ALTER TABLE series DROP COLUMN discontinued_since;

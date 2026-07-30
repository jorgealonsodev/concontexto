-- 0004_source_licence_url.down.sql
--
-- Reverses 0004_source_licence_url.up.sql exactly. Dropping the column
-- loses only the distinct licence-URL annotation -- source.url (the
-- source's general website) and every other source field are untouched.
ALTER TABLE source DROP COLUMN licence_url;

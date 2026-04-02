-- 001_initial_schema.down.sql
-- Reverse the initial schema

BEGIN;

DROP TABLE IF EXISTS positions;
DROP TABLE IF EXISTS fills;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS instruments;

COMMIT;

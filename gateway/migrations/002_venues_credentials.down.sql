-- 002_venues_credentials.down.sql
-- Reverse venues and credentials tables

BEGIN;

DROP TABLE IF EXISTS venue_credentials;
DROP TABLE IF EXISTS venues;

COMMIT;

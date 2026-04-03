-- 002_venues_credentials.up.sql
-- Venues and encrypted credentials tables

BEGIN;

CREATE TABLE IF NOT EXISTS venues (
    id              TEXT PRIMARY KEY,
    type            TEXT NOT NULL,
    name            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'disconnected',
    config_json     JSONB,
    last_heartbeat  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS venue_credentials (
    venue_id              TEXT PRIMARY KEY REFERENCES venues(id) ON DELETE CASCADE,
    encrypted_api_key     BYTEA NOT NULL,
    encrypted_api_secret  BYTEA NOT NULL,
    encrypted_passphrase  BYTEA,
    salt                  BYTEA NOT NULL,
    nonce                 BYTEA NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_venue_credentials_venue_id ON venue_credentials(venue_id);

COMMIT;

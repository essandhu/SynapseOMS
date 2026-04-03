-- 001_initial_schema.up.sql
-- Initial schema for SynapseOMS Gateway

BEGIN;

CREATE TABLE IF NOT EXISTS instruments (
    id              TEXT PRIMARY KEY,
    symbol          TEXT NOT NULL,
    name            TEXT NOT NULL,
    asset_class     TEXT NOT NULL,
    quote_currency  TEXT NOT NULL DEFAULT 'USD',
    base_currency   TEXT,
    tick_size       NUMERIC NOT NULL,
    lot_size        NUMERIC NOT NULL,
    settlement_cycle TEXT NOT NULL,
    trading_hours   JSONB,
    venues          TEXT[],
    margin_required NUMERIC NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
    id              TEXT PRIMARY KEY,
    client_order_id TEXT NOT NULL UNIQUE,
    instrument_id   TEXT NOT NULL REFERENCES instruments(id),
    side            TEXT NOT NULL,
    type            TEXT NOT NULL,
    quantity        NUMERIC NOT NULL,
    price           NUMERIC NOT NULL DEFAULT 0,
    filled_quantity NUMERIC NOT NULL DEFAULT 0,
    average_price   NUMERIC NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'new',
    venue_id        TEXT,
    asset_class     TEXT NOT NULL,
    settlement_cycle TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_instrument_id ON orders(instrument_id);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at DESC);

CREATE TABLE IF NOT EXISTS fills (
    id              TEXT PRIMARY KEY,
    order_id        TEXT NOT NULL REFERENCES orders(id),
    venue_id        TEXT,
    quantity        NUMERIC NOT NULL,
    price           NUMERIC NOT NULL,
    fee             NUMERIC NOT NULL DEFAULT 0,
    fee_asset       TEXT NOT NULL DEFAULT 'USD',
    liquidity       TEXT,
    venue_exec_id   TEXT,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fills_order_id ON fills(order_id);

CREATE TABLE IF NOT EXISTS positions (
    instrument_id       TEXT NOT NULL REFERENCES instruments(id),
    venue_id            TEXT NOT NULL,
    quantity            NUMERIC NOT NULL DEFAULT 0,
    average_cost        NUMERIC,
    market_price        NUMERIC,
    unrealized_pnl      NUMERIC,
    realized_pnl        NUMERIC,
    unsettled_quantity   NUMERIC,
    settled_quantity     NUMERIC,
    asset_class         TEXT,
    quote_currency      TEXT NOT NULL DEFAULT 'USD',
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (instrument_id, venue_id)
);

CREATE TABLE IF NOT EXISTS app_settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMIT;

#!/usr/bin/env bash
# seed-instruments.sh — Manual fallback to seed the 6 default instruments.
#
# The gateway auto-seeds instruments on startup (see cmd/gateway/main.go).
# This script is a manual alternative that inserts directly via psql.
#
# Usage:
#   ./scripts/seed-instruments.sh
#
# Environment variables (with defaults):
#   POSTGRES_URL  — full connection string (default: postgres://synapse:synapse@localhost:5432/synapse)

set -euo pipefail

POSTGRES_URL="${POSTGRES_URL:-postgres://synapse:synapse@localhost:5432/synapse}"

echo "==> Seeding instruments into ${POSTGRES_URL%%@*}@***"

psql "${POSTGRES_URL}" <<'SQL'
BEGIN;

INSERT INTO instruments (id, symbol, name, asset_class, quote_currency, tick_size, lot_size, settlement_cycle, trading_hours, venues, margin_required)
VALUES
  ('AAPL', 'AAPL', 'Apple Inc.', 'equity', 'USD', 0.01, 1, 'T+2',
   '{"MarketOpen":"09:30","MarketClose":"16:00","PreMarket":"04:00","AfterHours":"20:00","Timezone":"America/New_York","Is24x7":false}',
   '{simulated}', 0),
  ('MSFT', 'MSFT', 'Microsoft Corp.', 'equity', 'USD', 0.01, 1, 'T+2',
   '{"MarketOpen":"09:30","MarketClose":"16:00","PreMarket":"04:00","AfterHours":"20:00","Timezone":"America/New_York","Is24x7":false}',
   '{simulated}', 0),
  ('GOOG', 'GOOG', 'Alphabet Inc.', 'equity', 'USD', 0.01, 1, 'T+2',
   '{"MarketOpen":"09:30","MarketClose":"16:00","PreMarket":"04:00","AfterHours":"20:00","Timezone":"America/New_York","Is24x7":false}',
   '{simulated}', 0),
  ('BTC-USD', 'BTC-USD', 'Bitcoin', 'crypto', 'USD', 0.01, 0.00001, 'T+0',
   '{"Is24x7":true,"MarketOpen":"","MarketClose":"","PreMarket":"","AfterHours":"","Timezone":""}',
   '{simulated}', 0),
  ('ETH-USD', 'ETH-USD', 'Ethereum', 'crypto', 'USD', 0.01, 0.0001, 'T+0',
   '{"Is24x7":true,"MarketOpen":"","MarketClose":"","PreMarket":"","AfterHours":"","Timezone":""}',
   '{simulated}', 0),
  ('SOL-USD', 'SOL-USD', 'Solana', 'crypto', 'USD', 0.01, 0.01, 'T+0',
   '{"Is24x7":true,"MarketOpen":"","MarketClose":"","PreMarket":"","AfterHours":"","Timezone":""}',
   '{simulated}', 0)
ON CONFLICT (id) DO UPDATE SET
  symbol           = EXCLUDED.symbol,
  name             = EXCLUDED.name,
  asset_class      = EXCLUDED.asset_class,
  quote_currency   = EXCLUDED.quote_currency,
  tick_size        = EXCLUDED.tick_size,
  lot_size         = EXCLUDED.lot_size,
  settlement_cycle = EXCLUDED.settlement_cycle,
  trading_hours    = EXCLUDED.trading_hours,
  venues           = EXCLUDED.venues,
  margin_required  = EXCLUDED.margin_required;

COMMIT;
SQL

echo "==> Verifying seeded instruments:"
psql "${POSTGRES_URL}" -c "SELECT id, symbol, asset_class, settlement_cycle FROM instruments ORDER BY symbol;"
echo "==> Done. 6 instruments seeded."

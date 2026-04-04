# Quickstart

Get SynapseOMS running on your machine in about 3 minutes.

## Prerequisites

- **Docker** and **Docker Compose** (v2.20+)
- ~4 GB RAM available for Docker
- Ports 3000, 8080, 8081 available

## Steps

### 1. Clone the repository

```bash
git clone https://github.com/essandhu/SynapseOMS.git
cd SynapseOMS
```

### 2. Configure environment

```bash
cp deploy/.env.example deploy/.env
```

Edit `deploy/.env` and set your master passphrase:

```
SYNAPSE_MASTER_PASSPHRASE=your-strong-passphrase-here
```

This passphrase encrypts all stored venue credentials. Choose something strong — you'll need it if you restart the system.

### 3. Start the system

```bash
docker compose -f deploy/docker-compose.yml up
```

First run pulls images and builds containers (~2-3 minutes). Subsequent starts take seconds.

Wait until you see health check logs from all services:

```
gateway-1      | {"level":"info","msg":"server_started","port":8080}
risk-engine-1  | {"level":"info","msg":"risk_engine_started"}
dashboard-1    | ready in 500ms
```

### 4. Open the dashboard

Go to [http://localhost:3000](http://localhost:3000) in your browser.

### 5. Complete onboarding

The onboarding wizard walks you through:

1. **Welcome** — overview of what SynapseOMS does
2. **Passphrase** — create your master passphrase (same one from step 2)
3. **Venue choice** — select **Simulated Exchange** (no external credentials needed)
4. **Credentials** — automatic for the simulated exchange
5. **Ready** — you're connected

### 6. Submit your first order

1. Open the **Blotter** view
2. Click the order ticket panel
3. Select an instrument (e.g., AAPL or BTC-USD)
4. Choose Buy/Sell, Market order, quantity
5. Submit — you'll see the order fill instantly on the simulated exchange
6. Check the **Portfolio** view to see your position
7. Check the **Risk Dashboard** for VaR and exposure analytics

## What's running?

| Service | Port | Purpose |
|---------|------|---------|
| Dashboard | 3000 | Trading terminal UI |
| Gateway | 8080 | REST API + WebSocket |
| Risk Engine | 8081 | Risk analytics + AI |
| PostgreSQL | 5432 | Order/position storage |
| Redis | 6379 | Cache |
| Kafka | 9092 | Event streaming |
| ML Scorer | 8090 | Venue scoring model |

## Optional: Enable monitoring

```bash
docker compose -f deploy/docker-compose.yml --profile monitoring up
```

This adds Prometheus (port 9090) and Grafana (port 3001, login: admin/synapse) with pre-built dashboards.

## Next steps

- [Connect a real exchange](connect-venue.md) — Alpaca paper trading or Binance testnet
- [Write a venue adapter](write-adapter.md) — add support for your favorite exchange

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Port already in use | Stop the conflicting service or change ports in `docker-compose.yml` |
| Docker out of memory | Increase Docker memory limit to at least 4 GB (Docker Desktop → Settings → Resources) |
| Slow first build | Normal — images need to download and build. Subsequent starts are fast. |
| Gateway health check failing | Wait 15-30 seconds for PostgreSQL and Kafka to become ready |
| "passphrase must not be empty" | Ensure `SYNAPSE_MASTER_PASSPHRASE` is set in `deploy/.env` |

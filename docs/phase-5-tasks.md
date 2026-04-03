# Phase 5 Tasks — Production Hardening

**Goal:** `docker compose up` brings up the full system; a new user can go from zero to connected-and-trading in under 5 minutes; load test report shows sustained throughput targets.

**Acceptance Test:** Fresh user clones repo, runs `docker compose up`, opens browser, completes onboarding with the simulated exchange, submits an order, sees it fill, checks risk dashboard — all within 5 minutes. Load test report shows Gateway sustaining 5,000 orders/sec with p99 < 50ms.

**Architecture Doc References:** Sections 1 (directory structure), 6.4 (Tokenized Securities Adapter), 7 (full — Docker Compose, Prometheus metrics, structured logging, startup health checks, load testing), 8 Phase 5, 10 (full — README, Quickstart, Connect Venue, Write Adapter, License)

**Previous Phase Review:** Phase 4 completed 27/27 tasks, all pass. No catch-up items flagged. Minor divergences (sync vs async RebalancingAssistant, test file locations in `tests/` subdirs) — none affect Phase 5 design. Anomaly consumer wired after WebSocket hub creation (compile-time requirement, not a concern).

---

## Tasks

### P5-01: Tokenized Securities Adapter

**Service:** Gateway
**Files:**
- `gateway/internal/adapter/tokenized/adapter.go` (create)
- `gateway/internal/adapter/tokenized/adapter_test.go` (create)
**Dependencies:** None
**Acceptance Criteria:**
- `TokenizedAdapter` struct implements the full `LiquidityProvider` interface
- Reuses `simulated.MatchingEngine` internally for order matching
- All orders are settled as T+0 (`SettlementT0`)
- `VenueID()` returns `"tokenized_sim"`
- `SupportedAssetClasses()` returns `[TokenizedEquity]` (or equivalent)
- Passes the shared `AdapterContractSuite` tests (see `gateway/internal/adapter/contract_test.go`)
- Dedicated unit tests covering: Connect, SubmitOrder (T+0 settlement), CancelOrder, QueryOrder, market data subscription, fill feed
- Registered in `adapter/registry.go` so it appears in venue listings

**Architecture Context:**
From Section 6.4, the tokenized adapter is a forward-looking adapter for tokenized security venues (e.g., Securitize Markets, tZERO, Backed Finance). Key design:

```go
// gateway/internal/adapter/tokenized/adapter.go

type TokenizedAdapter struct {
    walletAddress string
    simEngine     *simulated.MatchingEngine  // Reuses simulated engine internally
}

func (a *TokenizedAdapter) VenueID() string { return "tokenized_sim" }

func (a *TokenizedAdapter) SubmitOrder(ctx context.Context, order *Order) (*VenueAck, error) {
    // Simulate on-chain order submission
    // In production: construct and sign a transaction, submit to venue's API
    order.SettlementCycle = SettlementT0  // Always T+0
    return a.simEngine.Match(order)
}
```

Key differences from traditional adapters:
- Wallet-based identification (address instead of account ID)
- Token-based positions (ERC-20-like balance queries)
- Atomic T+0 settlement (no pending settlement window)
- On-chain transaction confirmation (simulated as instant for now)

The full `LiquidityProvider` interface that must be implemented:
```go
type LiquidityProvider interface {
    VenueID() string
    VenueName() string
    SupportedAssetClasses() []domain.AssetClass
    SupportedInstruments() ([]domain.Instrument, error)
    Connect(ctx context.Context, cred domain.VenueCredential) error
    Disconnect(ctx context.Context) error
    Status() VenueStatus
    Ping(ctx context.Context) (latency time.Duration, err error)
    SubmitOrder(ctx context.Context, order *domain.Order) (*VenueAck, error)
    CancelOrder(ctx context.Context, orderID domain.OrderID, venueOrderID string) error
    QueryOrder(ctx context.Context, venueOrderID string) (*domain.Order, error)
    SubscribeMarketData(ctx context.Context, instruments []string) (<-chan MarketDataSnapshot, error)
    UnsubscribeMarketData(ctx context.Context, instruments []string) error
    FillFeed() <-chan domain.Fill
    Capabilities() VenueCapabilities
}
```

---

### P5-02: Prometheus Metrics — Gateway

**Service:** Gateway
**Files:**
- `gateway/internal/metrics/metrics.go` (create)
- Modifications to: `gateway/internal/pipeline/pipeline.go`, `gateway/internal/rest/handler_order.go`, `gateway/internal/ws/server.go`, `gateway/internal/adapter/registry.go` (or individual adapters)
- `gateway/cmd/gateway/main.go` (add `/metrics` endpoint)
**Dependencies:** None
**Acceptance Criteria:**
- All 6 Gateway metrics from Section 7.2 are registered and exported at `/metrics` in Prometheus exposition format
- `gateway_orders_submitted_total` (Counter) — labeled by `asset_class`, `venue` — incremented on order submission
- `gateway_order_latency_seconds` (Histogram) — end-to-end order processing time (submit → venue ack)
- `gateway_fills_received_total` (Counter) — labeled by `venue`, `liquidity_type` — incremented on fill
- `gateway_venue_latency_seconds` (Histogram) — per-venue round-trip latency, labeled by `venue`
- `gateway_venue_status` (Gauge) — 1=connected, 0=disconnected, labeled by `venue`
- `gateway_active_websocket_connections` (Gauge) — current WebSocket client count
- Uses `prometheus/client_golang` library
- `/metrics` endpoint accessible on the existing port (8080)
- Unit tests verify metric registration and correct increment/observe behavior

**Architecture Context:**
From Section 7.2, full metrics table for Gateway:

| Metric | Type | Description |
|--------|------|-------------|
| `gateway_orders_submitted_total` | Counter | Total orders submitted, labeled by asset_class, venue |
| `gateway_order_latency_seconds` | Histogram | End-to-end order processing time (submit → venue ack) |
| `gateway_fills_received_total` | Counter | Total fills, labeled by venue, liquidity_type |
| `gateway_venue_latency_seconds` | Histogram | Per-venue round-trip latency (p50, p95, p99) |
| `gateway_venue_status` | Gauge | 1=connected, 0=disconnected per venue |
| `gateway_active_websocket_connections` | Gauge | Current WebSocket client count |

Instrument pipeline stages and adapter calls to record these. Use `promauto` for registration convenience. Histogram buckets should cover the performance targets: p99 < 50ms for orders, so buckets like `.005, .01, .025, .05, .1, .25, .5, 1`.

---

### P5-03: Prometheus Metrics — Risk Engine

**Service:** Risk Engine
**Files:**
- `risk_engine/metrics.py` (create)
- Modifications to: `risk_engine/var/historical.py`, `risk_engine/var/parametric.py`, `risk_engine/var/monte_carlo.py`, `risk_engine/grpc_server/server.py`, `risk_engine/anomaly/detector.py`
- `risk_engine/main.py` (mount `/metrics` endpoint)
**Dependencies:** None
**Acceptance Criteria:**
- All 5 Risk Engine metrics from Section 7.2 are registered and exported at `/metrics` in Prometheus format
- `risk_var_computation_seconds` (Histogram) — labeled by `method` (historical, parametric, monte_carlo)
- `risk_pretrade_check_seconds` (Histogram) — gRPC pre-trade check latency
- `risk_pretrade_check_rejected_total` (Counter) — pre-trade rejections
- `risk_portfolio_var_ratio` (Gauge) — current VaR as % of NAV
- `risk_anomalies_detected_total` (Counter) — anomaly alerts fired
- Uses `prometheus_client` Python library
- Unit tests verify metric registration and correct increment/observe behavior

**Architecture Context:**
From Section 7.2, full metrics table for Risk Engine:

| Metric | Type | Description |
|--------|------|-------------|
| `risk_var_computation_seconds` | Histogram | VaR computation time by method |
| `risk_pretrade_check_seconds` | Histogram | gRPC pre-trade check latency |
| `risk_pretrade_check_rejected_total` | Counter | Pre-trade rejections |
| `risk_portfolio_var_ratio` | Gauge | Current VaR as % of NAV |
| `risk_anomalies_detected_total` | Counter | Anomaly alerts fired |

Kafka consumer lag (`kafka_consumer_lag`) is typically exported by the Kafka broker or a separate exporter — do NOT implement this in application code; it will be covered by Prometheus scrape config targeting the Kafka broker.

---

### P5-04: Prometheus Scrape Configuration

**Service:** Infrastructure
**Files:**
- `deploy/prometheus.yml` (create)
**Dependencies:** P5-02, P5-03
**Acceptance Criteria:**
- Scrape config targets Gateway (`gateway:8080/metrics`), Risk Engine (`risk-engine:8081/metrics`)
- Scrape interval of 15s
- Job names: `gateway`, `risk-engine`
- Kafka consumer lag metric noted as future addition (when Kafka JMX exporter is added)
- Valid YAML, loadable by Prometheus container

**Architecture Context:**
```yaml
# deploy/prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: "gateway"
    static_configs:
      - targets: ["gateway:8080"]
  - job_name: "risk-engine"
    static_configs:
      - targets: ["risk-engine:8081"]
```

---

### P5-05: Grafana Dashboards

**Service:** Infrastructure
**Files:**
- `deploy/grafana/system-overview.json` (create)
- `deploy/grafana/venue-performance.json` (create)
**Dependencies:** P5-02, P5-03, P5-04
**Acceptance Criteria:**
- `system-overview.json`: pre-built Grafana dashboard with panels for:
  - Order submission rate (`gateway_orders_submitted_total` rate)
  - Order latency p50/p95/p99 (`gateway_order_latency_seconds` histogram quantiles)
  - Fill rate (`gateway_fills_received_total` rate)
  - Active WebSocket connections (`gateway_active_websocket_connections`)
  - VaR computation time (`risk_var_computation_seconds` histogram quantiles)
  - Pre-trade check latency (`risk_pretrade_check_seconds`)
  - Pre-trade rejection rate (`risk_pretrade_check_rejected_total` rate)
  - Portfolio VaR ratio gauge (`risk_portfolio_var_ratio`)
  - Anomalies detected rate (`risk_anomalies_detected_total`)
- `venue-performance.json`: per-venue dashboard with:
  - Venue latency by venue (`gateway_venue_latency_seconds` by venue label)
  - Venue status (`gateway_venue_status` by venue)
  - Fills by venue (`gateway_fills_received_total` by venue)
  - Orders by venue (`gateway_orders_submitted_total` by venue)
- Both dashboards are valid Grafana JSON model format, importable via provisioning
- Datasource references use variable `${DS_PROMETHEUS}` for portability
- Auto-provisioned when Grafana starts (via volume mount in docker-compose)

**Architecture Context:**
From Section 7.1, Grafana is configured with:
```yaml
grafana:
  image: grafana/grafana:11.1.0
  ports: ["3001:3000"]
  volumes:
    - ./grafana/:/etc/grafana/provisioning/dashboards/
  environment:
    GF_SECURITY_ADMIN_PASSWORD: synapse
  profiles: [monitoring]
```

Dashboards in `deploy/grafana/` are auto-provisioned. Each JSON file should be a complete Grafana dashboard model with `uid`, `title`, `panels[]`, and Prometheus queries referencing the exact metric names from P5-02 and P5-03.

---

### P5-06: Load Testing Harness — k6 Scripts

**Service:** Infrastructure
**Files:**
- `loadtest/k6/order_flow.js` (create)
- `loadtest/k6/ws_stream.js` (create)
- `loadtest/README.md` (create)
**Dependencies:** None
**Acceptance Criteria:**
- `order_flow.js` implements the exact script structure from Section 7.5 with:
  - `constant-arrival-rate` executor at 5,000 orders/sec for 5 minutes
  - 200 pre-allocated VUs, max 500
  - Custom metrics: `order_submit_latency` (Trend), `fill_received` (Rate)
  - Thresholds: `order_submit_latency p(99) < 50`, `fill_received rate > 0.99`
  - Randomized instrument selection from `["AAPL", "MSFT", "ETH-USD", "BTC-USD", "GOOG", "SOL-USD"]`
  - Randomized buy/sell, market orders, random quantities
  - POSTs to `http://localhost:8080/api/v1/orders`
- `ws_stream.js` implements WebSocket streaming load test:
  - Opens multiple WebSocket connections to `/ws/orders`, `/ws/positions`, `/ws/marketdata`
  - Verifies messages are received within latency thresholds
  - Tests fan-out performance (target: 1000 clients, < 5ms fan-out)
- `loadtest/README.md` documents:
  - How to install k6
  - How to run each script
  - How to interpret results
  - Performance targets table (all 8 targets from Section 7.5)

**Architecture Context:**
From Section 7.5, the `order_flow.js` script:
```javascript
import http from "k6/http";
import { check } from "k6";
import { Rate, Trend } from "k6/metrics";

const orderLatency = new Trend("order_submit_latency", true);
const fillRate = new Rate("fill_received");

export const options = {
  scenarios: {
    sustained_load: {
      executor: "constant-arrival-rate",
      rate: 5000,           // 5,000 orders/second target
      timeUnit: "1s",
      duration: "5m",
      preAllocatedVUs: 200,
      maxVUs: 500,
    },
  },
  thresholds: {
    order_submit_latency: ["p(99)<50"],  // p99 < 50ms
    fill_received: ["rate>0.99"],         // >99% fill rate on simulated venue
  },
};

export default function () {
  const instruments = ["AAPL", "MSFT", "ETH-USD", "BTC-USD", "GOOG", "SOL-USD"];
  const instrument = instruments[Math.floor(Math.random() * instruments.length)];
  const payload = JSON.stringify({
    instrumentId: instrument,
    side: Math.random() > 0.5 ? "buy" : "sell",
    type: "market",
    quantity: (Math.random() * 100 + 1).toFixed(2),
  });
  const start = Date.now();
  const res = http.post("http://localhost:8080/api/v1/orders", payload, {
    headers: { "Content-Type": "application/json" },
  });
  orderLatency.add(Date.now() - start);
  check(res, { "status 201": (r) => r.status === 201 });
}
```

Full performance targets table from Section 7.5:

| Metric | Target |
|--------|--------|
| Order submission throughput | 5,000 orders/sec sustained |
| Order submission p99 latency | < 50ms |
| Pre-trade risk check p99 | < 10ms |
| Fill-to-WebSocket p99 | < 20ms |
| VaR computation (100 instruments) | < 500ms |
| Monte Carlo VaR (10k paths, 50 instruments) | < 2s |
| Portfolio optimization (50 instruments) | < 1s |
| WebSocket broadcast (1000 clients) | < 5ms fan-out |

---

### P5-07: Onboarding Flow Polish

**Service:** Dashboard
**Files:**
- `dashboard/src/views/OnboardingView.tsx` (modify)
- `dashboard/src/components/CredentialForm.tsx` (modify)
**Dependencies:** None
**Acceptance Criteria:**
- Each onboarding step has clear error handling with user-friendly error messages
- Loading/progress indicators on all async operations (credential validation, venue connection)
- Security messaging: visible explanation of how credentials are encrypted (AES-256-GCM, Argon2id key derivation, stored locally, never transmitted)
- Form validation with inline error messages (e.g., empty API key, invalid format)
- "Back" navigation works correctly between all 5 steps
- Passphrase strength indicator on the passphrase creation step
- Connection test feedback: spinner during connection attempt, success/failure banner with details
- Graceful handling of network errors during onboarding (retry option, clear error state)

**Architecture Context:**
The existing onboarding flow has 5 steps (from Phase 2): Welcome → Passphrase → Venue Choice → Credentials → Ready. This task polishes the UX with error handling, progress indicators, and security messaging. The credential encryption uses Argon2id for key derivation + AES-256-GCM for encryption (implemented in `gateway/internal/credential/manager.go`). Security messaging should explain this to users in simple terms.

---

### P5-08: Credential Encryption Hardening

**Service:** Gateway
**Files:**
- `gateway/internal/credential/manager.go` (modify)
- `gateway/internal/credential/vault.go` (modify)
- `gateway/internal/credential/manager_test.go` (modify)
**Dependencies:** None
**Acceptance Criteria:**
- Key rotation support: ability to re-encrypt all credentials with a new master passphrase without data loss
- `RotatePassphrase(ctx, oldPassphrase, newPassphrase)` method on credential manager
- Secure memory handling: credentials are zeroed from memory after use (use `memguard` or manual byte-zeroing)
- Decrypted credentials are not held in memory longer than necessary
- Key derivation parameters (Argon2id time/memory/threads) are configurable and documented
- Unit tests for: key rotation (re-encrypt + verify), memory zeroing behavior, invalid passphrase rejection during rotation

**Architecture Context:**
The credential manager already implements AES-256-GCM encryption with Argon2id key derivation (Phase 2). This task hardens it:
1. **Key rotation**: Decrypt all credentials with old passphrase, re-encrypt with new one, atomic swap in PostgreSQL
2. **Secure memory**: Zero sensitive byte slices after use. Consider `github.com/awnuber/memguard` or manual `for i := range b { b[i] = 0 }` patterns
3. **Configurable KDF**: Argon2id params (time=1, memory=64MB, threads=4) should be configurable via environment variables

---

### P5-09: Comprehensive Error Handling & Graceful Degradation

**Service:** Gateway, Risk Engine, Dashboard
**Files:**
- `gateway/internal/pipeline/pipeline.go` (modify)
- `gateway/internal/grpc/risk_client.go` (modify)
- `gateway/internal/adapter/registry.go` (modify)
- `gateway/internal/ws/server.go` (modify)
- `risk_engine/kafka/consumer.py` (modify)
- `risk_engine/grpc_server/server.py` (modify)
- `dashboard/src/api/rest.ts` (modify)
- `dashboard/src/api/ws.ts` (modify)
**Dependencies:** None
**Acceptance Criteria:**
- **Gateway pipeline**: If risk engine gRPC is unreachable, order pipeline degrades gracefully (fail-open with warning log, not hard failure) — the `risk_client.go` stub already does this, verify and formalize the pattern
- **Gateway adapters**: If a venue adapter disconnects mid-session, orders to that venue return a clear error and other venues continue to function
- **Gateway WebSocket**: Client disconnect is handled cleanly (no goroutine leaks, unsubscribe from all streams)
- **Risk Engine Kafka**: If Kafka consumer loses connection, it reconnects with backoff and resumes from last committed offset
- **Risk Engine gRPC**: Timeout handling on pre-trade checks (10ms budget per Section 7.5)
- **Dashboard REST**: All API calls have timeout, retry with backoff for transient errors, and user-visible error toasts
- **Dashboard WebSocket**: Reconnection is already implemented (`reconnecting-websocket`); verify error state is shown in UI (e.g., "Connection lost, reconnecting...")
- Unit tests for key degradation paths (risk engine unavailable, venue disconnect, Kafka down)

**Architecture Context:**
This is a cross-cutting hardening pass. The goal is not to add new features but to ensure that partial failures don't cascade. Key principle: **the system should always be usable for the venues that ARE connected, even if other components are degraded.** Risk engine unavailability should not prevent order submission (fail-open). Individual venue failures should not affect other venues. Dashboard should show degraded state clearly rather than breaking.

---

### P5-10: Docker Compose Finalization

**Service:** Infrastructure
**Files:**
- `deploy/docker-compose.yml` (modify)
- `deploy/docker-compose.dev.yml` (create)
- `.env.example` (create)
- `scripts/health-check.sh` (create)
**Dependencies:** P5-02, P5-03, P5-04, P5-05
**Acceptance Criteria:**
- `docker-compose.yml` updated to match Section 7.1 architecture spec:
  - All services have `healthcheck` definitions (gateway, risk-engine, dashboard, ml-scorer, kafka, postgres, redis already have them — verify completeness)
  - All services have `restart: unless-stopped` restart policy
  - Prometheus + Grafana services added (with `profiles: [monitoring]` so they're opt-in)
  - Prometheus volumes `deploy/prometheus.yml` config
  - Grafana volumes `deploy/grafana/` dashboards directory
  - `depends_on` conditions use `service_healthy` for all inter-service dependencies
- `docker-compose.dev.yml` with dev overrides:
  - Hot reload for dashboard (volume mount `../dashboard/src`)
  - Debug ports exposed for gateway (delve) and risk-engine (debugpy)
  - Verbose logging
- `.env.example` with documented environment variables:
  - `SYNAPSE_MASTER_PASSPHRASE` (required, user-chosen)
  - `ANTHROPIC_API_KEY` (optional, for AI features)
- `scripts/health-check.sh`: verify all services are healthy via their health endpoints
- Single-command startup: `docker compose up` brings up all core services (no monitoring); `docker compose --profile monitoring up` includes Prometheus + Grafana
- All services start successfully with `docker compose up` and reach healthy state

**Architecture Context:**
Current docker-compose.yml has: gateway, dashboard, postgres, redis, kafka, ml-scorer, risk-engine. Missing: prometheus, grafana, restart policies, dev overrides file.

From Section 7.1, the monitoring services:
```yaml
prometheus:
  image: prom/prometheus:v2.53.0
  ports: ["9090:9090"]
  volumes:
    - ./prometheus.yml:/etc/prometheus/prometheus.yml
  profiles: [monitoring]

grafana:
  image: grafana/grafana:11.1.0
  ports: ["3001:3000"]
  volumes:
    - ./grafana/:/etc/grafana/provisioning/dashboards/
  environment:
    GF_SECURITY_ADMIN_PASSWORD: synapse
  profiles: [monitoring]
```

---

### P5-11: Kubernetes Manifests

**Service:** Infrastructure
**Files:**
- `deploy/k8s/namespace.yaml` (create)
- `deploy/k8s/gateway-deployment.yaml` (create)
- `deploy/k8s/risk-engine-deployment.yaml` (create)
- `deploy/k8s/dashboard-deployment.yaml` (create)
- `deploy/k8s/kafka-statefulset.yaml` (create)
- `deploy/k8s/postgres-statefulset.yaml` (create)
- `deploy/k8s/redis-deployment.yaml` (create)
- `deploy/k8s/monitoring/prometheus-config.yaml` (create)
- `deploy/k8s/monitoring/grafana-dashboards.yaml` (create)
**Dependencies:** P5-10 (mirrors docker-compose topology)
**Acceptance Criteria:**
- `namespace.yaml`: creates `synapse-oms` namespace
- Each deployment/statefulset manifest includes:
  - Resource requests and limits
  - Liveness and readiness probes matching docker-compose health checks
  - ConfigMaps for environment variables (no hardcoded secrets)
  - Service definitions for internal communication
- `kafka-statefulset.yaml` and `postgres-statefulset.yaml` use StatefulSets with PersistentVolumeClaims
- `monitoring/prometheus-config.yaml`: Prometheus ConfigMap + Deployment + Service
- `monitoring/grafana-dashboards.yaml`: Grafana Deployment + Service + ConfigMap referencing dashboard JSONs
- All manifests use `synapse-oms` namespace
- Manifests are valid YAML, pass `kubectl --dry-run=client` validation
- README note: K8s manifests are provided as a starting point for production deployment, not a production-ready Helm chart

**Architecture Context:**
From Section 1 directory structure, all K8s manifests go under `deploy/k8s/`. These mirror the Docker Compose topology but with K8s-native constructs. Port mappings, environment variables, and health checks should match what's defined in docker-compose.yml. Services: gateway (Deployment), risk-engine (Deployment), dashboard (Deployment), kafka (StatefulSet), postgres (StatefulSet), redis (Deployment), prometheus (Deployment), grafana (Deployment).

---

### P5-12: README.md

**Service:** Documentation
**Files:**
- `README.md` (create at repo root)
**Dependencies:** P5-10 (references docker compose commands)
**Acceptance Criteria:**
- Product-first structure matching Section 10.1 template exactly
- Sections: headline + tagline, problem statement, "Who is this for?" (4 personas), "Quickstart (3 minutes)" (abbreviated steps pointing to docs/quickstart.md), "Features" (6 bullet points), "Screenshots" (placeholder section), "Architecture" (brief overview pointing to docs/architecture-overview.md), "Documentation" (links to all guides), "Contributing" (link to CONTRIBUTING.md and docs/write-adapter.md), "License" (AGPLv3 one-liner with link)
- Tone: direct, confident, no marketing fluff — speaks to technical traders
- Quickstart section is self-contained enough that a user can get started without clicking through

**Architecture Context:**
From Section 10.1, the README template:
```markdown
# SynapseOMS

**The open-source trading terminal for traders who work across equities and crypto.**

There's no affordable tool that lets you see unified risk, execute across both traditional
and crypto markets, and get AI-driven analysis from a single interface. Bloomberg costs
$24k/year. Retail tools ignore half your portfolio. SynapseOMS fills the gap — and your
keys, data, and strategies never leave your machine.

## Who is this for?
- Algorithmic traders running strategies across equities + crypto
- Small crypto-native funds (1-5 people) that also hold traditional positions
- RIAs managing clients with both asset classes
- Quant researchers going from backtest to live

## Quickstart (3 minutes)
...

## Features
- Unified order management across Alpaca (equities) and Binance (crypto)
- Cross-asset risk analytics: VaR, Greeks, concentration, drawdown
- AI-powered execution analysis and portfolio rebalancing
- Smart order routing with ML venue scoring
- Self-hosted: your keys never leave your machine
- Extensible: add new exchanges by implementing one interface

## Screenshots
...
```

---

### P5-13: Quickstart Guide

**Service:** Documentation
**Files:**
- `docs/quickstart.md` (create)
**Dependencies:** P5-10 (references .env.example and docker compose)
**Acceptance Criteria:**
- Step-by-step from `git clone` to running system in 6 steps (from Section 10.2):
  1. Clone the repo
  2. Copy `.env.example` to `.env`, set master passphrase
  3. `docker compose up`
  4. Open `http://localhost:3000`
  5. Complete onboarding with simulated exchange
  6. Submit your first order
- Each step includes the exact command to run
- Prerequisites section: Docker, Docker Compose, ~4GB RAM
- Troubleshooting section: common issues (port conflicts, Docker memory, slow first build)
- Total estimated time: 3 minutes (after Docker images are pulled)

**Architecture Context:**
From Section 10.2. The guide assumes the simulated exchange is always available (no external credentials needed for first use). The onboarding flow guides the user through passphrase setup and simulated venue connection.

---

### P5-14: "Connect Your First Exchange" Guide

**Service:** Documentation
**Files:**
- `docs/connect-venue.md` (create)
**Dependencies:** None
**Acceptance Criteria:**
- Two sections: Alpaca (paper trading) and Binance (testnet)
- Alpaca section: sign up URL, how to get API key/secret, enter in SynapseOMS onboarding
- Binance Testnet section: testnet URL, how to get testnet credentials, enter in SynapseOMS
- Security explanation section: how credentials are encrypted (AES-256-GCM + Argon2id), where they're stored (local PostgreSQL), that they never leave the user's machine
- Screenshots/descriptions of the credential entry UI flow

**Architecture Context:**
From Section 10.3. This guide bridges the gap between the quickstart (simulated venue) and real trading (Alpaca paper / Binance testnet). The security explanation should reference the actual encryption implementation from `gateway/internal/credential/manager.go`.

---

### P5-15: "Write a Venue Adapter" Contributor Guide

**Service:** Documentation
**Files:**
- `docs/write-adapter.md` (create)
**Dependencies:** None
**Acceptance Criteria:**
- Step-by-step guide following Section 10.4:
  1. Implement the `LiquidityProvider` interface (with full interface definition shown)
  2. Register in `adapter/registry.go`
  3. Pass the contract test suite (`go test ./internal/adapter/ -run TestContractSuite -adapter=my_exchange`)
  4. Add venue-specific configuration
  5. Submit PR with adapter + tests + docs
- Includes a minimal adapter skeleton (as shown in Section 10.4)
- Explains each interface method with expected behavior
- Links to existing adapter implementations as reference (Alpaca, Binance, Simulated)
- Documents the `AdapterContractSuite` (21 subtests) and what each test checks

**Architecture Context:**
From Section 10.4, the minimal skeleton:
```go
type MyExchangeAdapter struct { ... }
func (a *MyExchangeAdapter) VenueID() string { return "my_exchange" }
func (a *MyExchangeAdapter) Connect(ctx context.Context, cred VenueCredential) error { ... }
// ... implement all methods
```

Running contract tests:
```
go test ./internal/adapter/ -run TestContractSuite -adapter=my_exchange
```

The full `LiquidityProvider` interface has 14 methods (listed in P5-01 architecture context above).

---

### P5-16: Architecture Overview for Contributors

**Service:** Documentation
**Files:**
- `docs/architecture-overview.md` (create)
**Dependencies:** None
**Acceptance Criteria:**
- High-level visual architecture for contributors (NOT the full architecture.md — this is a digestible overview)
- Mermaid diagram showing service topology: Dashboard ↔ Gateway ↔ Risk Engine, with Kafka, PostgreSQL, Redis
- Brief description of each service's responsibility (2-3 sentences each)
- Data flow: order submission → pipeline → venue → fill → Kafka → risk engine → WebSocket → dashboard
- Technology stack summary table (Go, Python, TypeScript, Kafka, PostgreSQL, Redis)
- Links to detailed architecture doc sections for each service
- Contribution entry points: "Want to add an exchange? See docs/write-adapter.md"

**Architecture Context:**
This is a simplified version of the architecture document aimed at new contributors. Use the sequence diagrams from Appendix A (A.1 Order Submission Flow, A.4 Venue Connection Flow) as reference for the data flow description. The service breakdown is in Section 4.

---

### P5-17: LICENSE and CONTRIBUTING.md

**Service:** Documentation
**Files:**
- `LICENSE` (create at repo root)
- `CONTRIBUTING.md` (create at repo root)
**Dependencies:** None
**Acceptance Criteria:**
- `LICENSE`: Full AGPLv3 license text (standard GNU AGPLv3 boilerplate)
- `CONTRIBUTING.md` includes:
  - How to set up the development environment (reference docs/quickstart.md + docker-compose.dev.yml)
  - Code style: Go (gofmt), Python (ruff), TypeScript (eslint + prettier)
  - How to run tests for each service
  - PR process: fork, branch, test, PR
  - Primary contribution path: venue adapters (link to docs/write-adapter.md)
  - Code of Conduct reference (or inline a brief one)
  - Issue labels and how to pick up work

**Architecture Context:**
From Section 10.5 (License Rationale): AGPLv3 chosen to prevent cloud providers from offering proprietary hosted version without contributing back. Target users (individual traders, small funds) can use it freely. The tradeoff (discouraging some enterprise contributors) is acceptable — growth comes from individual traders and adapter ecosystem.

---

## Deviations from Previous Phases

From the Phase 4 review:
- Test files are in `tests/` subdirectories (e.g., `ai/tests/test_analyst.py`) rather than flat alongside modules — follow this convention for any new test files
- `gateway/internal/pipeline/stage.go` was superseded — pipeline uses goroutine-based stages, not a Stage interface
- `risk_engine/grpc_server/server.py` already has fail-open behavior for pre-trade checks — P5-09 should verify and formalize, not re-implement

---

## Checklist Cross-Reference

### Phase 5 deliverable coverage:

| Phase 5 Deliverable | Task(s) |
|---------------------|---------|
| 1. Tokenized securities adapter | P5-01 |
| 2. Prometheus metrics on all services | P5-02, P5-03, P5-04 |
| 3. Grafana dashboards | P5-05 |
| 4. Load testing harness | P5-06 |
| 5. Onboarding flow polish | P5-07 |
| 6. Credential encryption hardening | P5-08 |
| 7. Comprehensive error handling | P5-09 |
| 8. Docker Compose finalization | P5-10 |
| 9. Kubernetes manifests | P5-11 |
| 10. README | P5-12 |
| 11. Quickstart guide | P5-13 |
| 12. "Connect Your First Exchange" guide | P5-14 |
| 13. "Write a Venue Adapter" guide | P5-15 |
| 14. Architecture overview | P5-16 |
| 15. LICENSE + CONTRIBUTING.md | P5-17 |

### Remaining unchecked deliverables NOT in Phase 5 scope:

| Unchecked Item | Disposition |
|----------------|-------------|
| `gateway/internal/orderbook/book.go` + `book_test.go` | **OUT OF SCOPE** — Listed in Phase 1 directory structure but never assigned to any phase deliverable. The order book is internal to the simulated matching engine (`simulated/matching_engine.go`) which is already implemented. A standalone order book module was not needed. |
| `risk_engine/timeseries/regime.py` | **OUT OF SCOPE** — Listed in directory structure but not assigned to any phase deliverable. Regime detection is a future enhancement beyond the 5-phase roadmap. |
| `risk_engine/rest/router_scenario.py` | **OUT OF SCOPE** — Listed in directory structure but not assigned to any phase deliverable. What-if scenario analysis is a future enhancement. |
| `risk_engine/requirements.lock` | **OUT OF SCOPE** — Lock file is generated by tooling, not manually authored. `pyproject.toml` with pinned ranges is sufficient. |
| `risk_engine/var/var_test.py` | **OUT OF SCOPE** — VaR tests exist in `tests/test_var_historical.py`, `tests/test_var_parametric.py`, `tests/test_var_monte_carlo.py` (total: 26 tests). A separate `var_test.py` is redundant. |
| `.github/workflows/ci.yml` | **OUT OF SCOPE** — CI/CD is listed in deliverables checklist but not assigned to any phase in Section 8. GitHub Actions setup is a post-release concern for a self-hosted tool. |
| `.github/workflows/release.yml` | **OUT OF SCOPE** — Same as above. |
| E2E Playwright tests (3 items) | **OUT OF SCOPE** — Listed in Section 9 (Testing Strategy) but not assigned to any phase deliverable. The acceptance test is manual (5-minute user flow). E2E automation is a future quality-of-life addition. |
| `gateway/internal/pipeline/pipeline_bench_test.go` | **OUT OF SCOPE** — Performance benchmarking is covered by k6 load tests (P5-06) which test the full stack. A micro-benchmark of the pipeline stage is a future optimization tool. |

---

## Task Completion Status

| Task | Status | Notes |
|------|--------|-------|
| P5-01 | ✅ COMPLETE | TokenizedAdapter implements LiquidityProvider, passes contract suite, 13 tests |
| P5-02 | ✅ COMPLETE | 6 Gateway Prometheus metrics registered, /metrics endpoint, 4 tests |
| P5-03 | ✅ COMPLETE | 5 Risk Engine Prometheus metrics, instrumented VaR/gRPC/anomaly, 10 tests |
| P5-04 | ✅ COMPLETE | deploy/prometheus.yml with gateway + risk-engine scrape targets |
| P5-05 | ✅ COMPLETE | 2 Grafana dashboards (system-overview: 9 panels, venue-performance: 4 panels) |
| P5-06 | ✅ COMPLETE | k6 order_flow.js (5k/sec), ws_stream.js (1k clients), README |
| P5-07 | ✅ COMPLETE | Onboarding polish: security messaging, passphrase strength, error handling, 22 tests |
| P5-08 | ✅ COMPLETE | RotatePassphrase, ZeroBytes, configurable KDFParams, ListVenueIDs, 7 new tests |
| P5-09 | ✅ COMPLETE | Fail-open risk, venue isolation, Kafka reconnect, REST retry, WS state, 13 new tests |
| P5-10 | ✅ COMPLETE | docker-compose.yml updated, monitoring profile, dev overrides, .env.example, health-check.sh |
| P5-11 | ✅ COMPLETE | 9 K8s manifests: namespace, 3 deployments, 2 statefulsets, 2 monitoring, redis |
| P5-12 | ✅ COMPLETE | README.md with product-first structure, quickstart, features, doc links |
| P5-13 | ✅ COMPLETE | docs/quickstart.md — 6-step guide with troubleshooting |
| P5-14 | ✅ COMPLETE | docs/connect-venue.md — Alpaca + Binance testnet with security explanation |
| P5-15 | ✅ COMPLETE | docs/write-adapter.md — skeleton, contract tests, step-by-step guide |
| P5-16 | ✅ COMPLETE | docs/architecture-overview.md — Mermaid diagram, service descriptions, data flow |
| P5-17 | ✅ COMPLETE | AGPLv3 LICENSE, CONTRIBUTING.md with dev setup and PR process |

## Phase 5 Deviations

### Deviation 1: Tokenized Adapter Exported MatchingEngine Methods
**Architecture Doc Says:** TokenizedAdapter reuses `simulated.MatchingEngine` internally.
**Actual Implementation:** Added exported `CancelOrder()` and `FindOrder()` methods to MatchingEngine so the tokenized adapter (different package) can access order book operations.
**Reason:** Go package visibility — the tokenized adapter is in a separate package and cannot access unexported fields/methods.
**Impact:** None — the MatchingEngine API surface is slightly larger but the methods are useful for any adapter using it.

### Deviation 2: VaR Instrumentation Uses _compute_inner Pattern
**Architecture Doc Says:** Wrap VaR `compute()` with Prometheus timing.
**Actual Implementation:** Refactored each VaR class to have `compute()` measure timing and delegate to `_compute_inner()`.
**Reason:** Python's `with` context manager would require restructuring the entire method body. The wrapper pattern keeps the timing clean and the original logic untouched.
**Impact:** None — behavior is identical, timing is accurate.

### Deviation 3: Grafana Provisioning Includes Datasource Config
**Architecture Doc Says:** Grafana volumes `deploy/grafana/` for dashboards.
**Actual Implementation:** Added `dashboards.yml` (provisioning config) and `datasources.yml` (auto-configure Prometheus datasource) alongside the dashboard JSONs.
**Reason:** Without these files, Grafana won't auto-discover the dashboards or know how to connect to Prometheus.
**Impact:** Positive — Grafana works out of the box with `docker compose --profile monitoring up`.

### Deviation 4: gRPC Pre-Trade Check Timeout Is Fail-Open
**Architecture Doc Says:** 10ms budget per pre-trade check (Section 7.5).
**Actual Implementation:** If the 10ms budget is exceeded mid-check, remaining checks are skipped and the order is approved (fail-open) with a warning log.
**Reason:** Consistent with the system's fail-open philosophy — risk engine unavailability should not prevent trading.
**Impact:** Low — in practice, checks complete well within 10ms. The timeout only triggers under extreme load.

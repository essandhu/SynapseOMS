# SynapseOMS Architecture Document

> **Version:** 1.0.0 | **Date:** 2026-04-01 | **Status:** Pre-Implementation

## Executive Summary

SynapseOMS is a self-hosted, open-source personal trading terminal that unifies execution and risk management across traditional equities and digital assets, with AI-powered portfolio intelligence. It fills the gap between $24k/yr institutional terminals and fragmented retail tools by giving individual traders, small funds, and RIAs a single interface for cross-asset order management, unified risk analytics, and AI-driven portfolio insights — all running on their own machine so keys, data, and strategies never leave their control.

---

## 1. Project Directory Structure

```
synapse-oms/
├── proto/                              # Shared Protobuf definitions (source of truth)
│   ├── order/
│   │   └── order.proto                 # Order, Fill, OrderRequest, ExecutionReport
│   ├── risk/
│   │   └── risk.proto                  # RiskCheckRequest/Response, VaR, Greeks
│   ├── instrument/
│   │   └── instrument.proto            # Instrument, InstrumentType, SettlementCycle
│   ├── portfolio/
│   │   └── portfolio.proto             # Position, Portfolio, Exposure
│   ├── marketdata/
│   │   └── marketdata.proto            # MarketDataSnapshot, OHLCV, OrderBookLevel
│   ├── venue/
│   │   └── venue.proto                 # VenueStatus, VenueCapabilities
│   └── buf.yaml                        # Buf build configuration
│
├── gateway/                            # Order Gateway & Execution Engine (Go)
│   ├── cmd/
│   │   └── gateway/
│   │       └── main.go                 # Entry point
│   ├── internal/
│   │   ├── domain/                     # Pure domain types, no external deps
│   │   │   ├── order.go                # Order aggregate, state machine
│   │   │   ├── fill.go                 # Fill / ExecutionReport
│   │   │   ├── instrument.go           # Instrument value object
│   │   │   ├── position.go             # Position aggregate
│   │   │   └── venue_credential.go     # VenueCredential (encrypted)
│   │   ├── orderbook/                  # In-memory order management
│   │   │   ├── book.go                 # Order book per instrument
│   │   │   └── book_test.go
│   │   ├── router/                     # Smart Order Router
│   │   │   ├── router.go              # Routing engine
│   │   │   ├── strategy.go            # Routing strategies (best-price, TWAP, ML-scored)
│   │   │   ├── ml_scorer.go           # ML model inference for venue scoring
│   │   │   └── router_test.go
│   │   ├── crossing/                   # Dark pool / internal crossing engine
│   │   │   ├── engine.go
│   │   │   └── engine_test.go
│   │   ├── adapter/                    # Venue adapter framework
│   │   │   ├── provider.go            # LiquidityProvider interface definition
│   │   │   ├── registry.go            # Adapter registration and discovery
│   │   │   ├── alpaca/                # Alpaca adapter (real equities)
│   │   │   │   ├── adapter.go
│   │   │   │   ├── ws_feed.go
│   │   │   │   └── adapter_test.go
│   │   │   ├── binance/               # Binance testnet adapter (real crypto)
│   │   │   │   ├── adapter.go
│   │   │   │   ├── ws_feed.go
│   │   │   │   └── adapter_test.go
│   │   │   ├── simulated/             # Simulated multi-asset exchange
│   │   │   │   ├── matching_engine.go
│   │   │   │   ├── price_walk.go
│   │   │   │   └── matching_engine_test.go
│   │   │   └── tokenized/             # Tokenized securities adapter (forward-looking)
│   │   │       ├── adapter.go
│   │   │       └── adapter_test.go
│   │   ├── credential/                 # Venue credential manager
│   │   │   ├── manager.go             # Encrypt/decrypt with master passphrase
│   │   │   ├── vault.go               # On-disk encrypted storage
│   │   │   └── manager_test.go
│   │   ├── pipeline/                   # Order processing pipeline
│   │   │   ├── pipeline.go            # Intake → risk check → route → report
│   │   │   ├── stage.go               # Pipeline stage interface
│   │   │   └── pipeline_test.go
│   │   ├── kafka/                      # Kafka producer
│   │   │   └── producer.go
│   │   ├── grpc/                       # gRPC client (risk pre-checks)
│   │   │   └── risk_client.go
│   │   ├── ws/                         # WebSocket server
│   │   │   └── server.go
│   │   └── rest/                       # REST API handlers
│   │       ├── handler_order.go
│   │       ├── handler_position.go
│   │       ├── handler_venue.go
│   │       └── handler_credential.go
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
│
├── risk-engine/                        # Risk & Analytics Engine (Python)
│   ├── risk_engine/
│   │   ├── __init__.py
│   │   ├── main.py                     # Entry point (FastAPI + Kafka consumer startup)
│   │   ├── domain/                     # Pure domain objects
│   │   │   ├── position.py             # Position aggregate (from events)
│   │   │   ├── portfolio.py            # Portfolio with cross-asset positions
│   │   │   ├── instrument.py
│   │   │   └── risk_result.py          # VaR, Greeks, concentration metrics
│   │   ├── var/                        # Value-at-Risk computations
│   │   │   ├── historical.py           # Historical simulation VaR
│   │   │   ├── parametric.py           # Variance-covariance VaR
│   │   │   ├── monte_carlo.py          # Monte Carlo VaR (correlated paths)
│   │   │   └── var_test.py
│   │   ├── greeks/                     # Portfolio Greeks
│   │   │   ├── calculator.py
│   │   │   └── calculator_test.py
│   │   ├── concentration/              # Concentration risk
│   │   │   ├── analyzer.py
│   │   │   └── analyzer_test.py
│   │   ├── settlement/                 # Settlement-aware cash-at-risk
│   │   │   ├── tracker.py
│   │   │   └── tracker_test.py
│   │   ├── optimizer/                  # Portfolio construction optimizer
│   │   │   ├── mean_variance.py        # cvxpy-based optimization
│   │   │   ├── constraints.py          # Constraint definitions
│   │   │   └── optimizer_test.py
│   │   ├── anomaly/                    # Market data anomaly detection
│   │   │   ├── detector.py             # Isolation forest streaming detector
│   │   │   └── detector_test.py
│   │   ├── timeseries/                 # Rolling statistics, covariance, regime
│   │   │   ├── statistics.py
│   │   │   ├── covariance.py
│   │   │   └── regime.py
│   │   ├── kafka/                      # Kafka consumer
│   │   │   └── consumer.py
│   │   ├── grpc_server/                # gRPC server (pre-trade risk checks)
│   │   │   └── server.py
│   │   └── rest/                       # REST API (FastAPI)
│   │       ├── router_risk.py
│   │       ├── router_optimizer.py
│   │       └── router_scenario.py
│   ├── tests/
│   │   ├── conftest.py
│   │   ├── test_var_historical.py
│   │   ├── test_var_parametric.py
│   │   ├── test_var_monte_carlo.py
│   │   ├── test_optimizer.py
│   │   ├── test_settlement.py
│   │   └── test_anomaly.py
│   ├── pyproject.toml                  # uv / pip project config
│   ├── Dockerfile
│   └── requirements.lock
│
├── dashboard/                          # Frontend Dashboard (TypeScript + React)
│   ├── src/
│   │   ├── main.tsx                    # App entry
│   │   ├── App.tsx                     # Root layout + routing
│   │   ├── api/                        # API client layer
│   │   │   ├── rest.ts                 # REST client (axios/fetch wrapper)
│   │   │   ├── ws.ts                   # WebSocket client with reconnect
│   │   │   └── types.ts               # Generated from proto → TS types
│   │   ├── stores/                     # Zustand state stores
│   │   │   ├── orderStore.ts
│   │   │   ├── positionStore.ts
│   │   │   ├── riskStore.ts
│   │   │   ├── venueStore.ts
│   │   │   └── insightStore.ts
│   │   ├── views/                      # Top-level page views
│   │   │   ├── BlotterView.tsx         # Unified order blotter
│   │   │   ├── PortfolioView.tsx       # Positions + P&L
│   │   │   ├── RiskDashboard.tsx       # VaR, Greeks, drawdown
│   │   │   ├── LiquidityNetwork.tsx    # Venue connections + health
│   │   │   ├── InsightsPanel.tsx       # AI analysis + alerts
│   │   │   └── OnboardingView.tsx      # First-run experience
│   │   ├── components/                 # Reusable UI components
│   │   │   ├── OrderTicket.tsx         # Order entry form
│   │   │   ├── OrderTable.tsx          # AG Grid blotter
│   │   │   ├── PositionTable.tsx       # Position grid
│   │   │   ├── VaRGauge.tsx            # VaR visualization gauge
│   │   │   ├── ExposureTreemap.tsx     # D3 treemap
│   │   │   ├── DrawdownChart.tsx       # Recharts drawdown
│   │   │   ├── MonteCarloPlot.tsx      # MC simulation distribution
│   │   │   ├── CandlestickChart.tsx    # Lightweight Charts wrapper
│   │   │   ├── VenueCard.tsx           # Venue status card
│   │   │   ├── CredentialForm.tsx      # API key input (secure)
│   │   │   └── TerminalLayout.tsx      # Dark terminal shell + panels
│   │   └── theme/
│   │       └── terminal.ts             # Dark theme tokens
│   ├── public/
│   ├── index.html
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── package.json
│   └── Dockerfile
│
├── ai/                                 # AI feature modules
│   ├── execution_analyst/              # Post-trade TCA via Anthropic API
│   │   ├── analyst.py
│   │   ├── prompt_templates.py
│   │   └── analyst_test.py
│   ├── rebalancing_assistant/          # NL → optimizer constraints
│   │   ├── assistant.py
│   │   ├── prompt_templates.py
│   │   └── assistant_test.py
│   ├── smart_router_ml/                # ML model for venue scoring
│   │   ├── features.py                 # Feature engineering pipeline
│   │   ├── train.py                    # Training script (XGBoost)
│   │   ├── model.py                    # Inference wrapper
│   │   └── model_test.py
│   └── requirements.txt
│
├── deploy/                             # Deployment configurations
│   ├── docker-compose.yml              # Primary: single `docker compose up`
│   ├── docker-compose.dev.yml          # Dev overrides (hot reload, debug ports)
│   ├── k8s/                            # Kubernetes manifests
│   │   ├── namespace.yaml
│   │   ├── gateway-deployment.yaml
│   │   ├── risk-engine-deployment.yaml
│   │   ├── dashboard-deployment.yaml
│   │   ├── kafka-statefulset.yaml
│   │   ├── postgres-statefulset.yaml
│   │   ├── redis-deployment.yaml
│   │   └── monitoring/
│   │       ├── prometheus-config.yaml
│   │       └── grafana-dashboards.yaml
│   └── grafana/
│       ├── system-overview.json        # Pre-built Grafana dashboard
│       └── venue-performance.json
│
├── loadtest/                           # Load testing harness
│   ├── k6/
│   │   ├── order_flow.js              # Realistic order submission load test
│   │   └── ws_stream.js               # WebSocket streaming load test
│   └── README.md
│
├── scripts/                            # Developer scripts
│   ├── proto-gen.sh                    # Generate Go + Python + TS from protos
│   ├── seed-instruments.sh             # Seed instrument reference data
│   └── health-check.sh                # Verify all services healthy
│
├── docs/
│   ├── planning-prompt.md
│   ├── architecture.md                 # This document
│   ├── quickstart.md                   # git clone → running in 3 min
│   ├── connect-venue.md                # First exchange connection guide
│   ├── write-adapter.md                # Contributor guide: new venue adapter
│   └── architecture-overview.md        # Visual architecture for contributors
│
├── LICENSE                             # AGPLv3 (see ADR-009)
├── README.md
├── Makefile                            # Top-level build targets
└── .github/
    └── workflows/
        ├── ci.yml                      # Build + test all services
        └── release.yml                 # Docker image builds
```

---

## 2. Architecture Decision Records

### ADR-001: Go for Order Gateway & Execution Engine

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Context** | The Order Gateway processes order submissions, manages lifecycle state machines, communicates with multiple venue adapters concurrently, and streams updates to clients in real time. It requires low-latency message processing, high concurrency, strong typing for financial message semantics, and a small deployment footprint (self-hosted). Candidates: Go, Java, C++, Rust. |
| **Decision** | Go 1.22+ |
| **Rationale** | Go's goroutine/channel model maps directly to per-order and per-venue concurrent processing without thread-pool complexity. Single static binary simplifies Docker images and self-hosted deployment. Compilation speed keeps the inner dev loop fast. Go's standard library handles HTTP, WebSocket, and gRPC natively. Compared to Java: no JVM overhead for a self-hosted desktop app. Compared to C++/Rust: dramatically faster development velocity for a project that needs to ship a usable product, not squeeze nanoseconds. The latency floor we target (sub-millisecond internal processing, p99 < 50ms end-to-end) is well within Go's range. |
| **Consequences** | Lack of generics for some collection patterns (mitigated in Go 1.22+). No inheritance for adapter implementations — use interface composition instead. Community Go finance libraries are thinner than Java's, so some FIX semantics will be hand-implemented. |
| **Product Justification** | Self-hosted users need a service that starts in <2 seconds, uses minimal memory, and is a single binary to deploy. Go gives us that while still handling the concurrency model a multi-venue gateway requires. Fast compilation means community contributors face low friction. |

### ADR-002: Python for Risk & Analytics Engine

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Context** | The Risk Engine computes VaR (historical, parametric, Monte Carlo), portfolio Greeks, covariance matrices, optimization, and anomaly detection. These are numerically intensive computations with well-established scientific Python libraries. |
| **Decision** | Python 3.12+ with NumPy, SciPy, pandas, cvxpy, scikit-learn, and FastAPI |
| **Rationale** | The entire quantitative finance ecosystem is Python-native: cvxpy for convex optimization, NumPy/SciPy for linear algebra and statistics, scikit-learn for anomaly detection (isolation forest), pandas for time-series manipulation. Writing this in Go would mean reimplementing or wrapping every one of these libraries. FastAPI provides async REST with auto-generated OpenAPI docs. grpcio provides the synchronous risk-check endpoint. |
| **Consequences** | Python is slower than Go for raw computation — mitigated by NumPy's C-backed vectorized operations and by keeping the risk engine as an independent service that doesn't sit in the order hot path (gRPC risk check is the single sync call, designed for <10ms). Must manage Python environment carefully (use uv or pip-compile for reproducible deps). |
| **Product Justification** | Cross-asset risk modeling is the core differentiator. Python lets us deliver institutional-grade analytics (Monte Carlo VaR, mean-variance optimization) with battle-tested numerical libraries rather than reimplementing statistics from scratch. A quant contributor can extend the risk models in a language they already know. |

### ADR-003: TypeScript + React for Frontend Dashboard

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Context** | The dashboard is a professional trading terminal rendering real-time streaming data (order blotter, positions, risk metrics) with complex visualizations (treemaps, candlestick charts, MC distribution plots). |
| **Decision** | TypeScript 5.x, React 19, Vite 6, Zustand for state, AG Grid Community for data grids, Lightweight Charts for candlestick/OHLC, Recharts for risk charts, D3 for custom visualizations |
| **Rationale** | React has the largest ecosystem of performance-oriented data grid libraries. AG Grid Community Edition handles 10k+ streaming row updates. TypeScript provides compile-time safety for complex financial data types (preventing currency mismatch, wrong instrument type in a position). Zustand is minimal and avoids Redux boilerplate while providing the selector-based reactivity needed for high-frequency updates (only re-render the row that changed). Vite gives sub-second HMR for development. |
| **Consequences** | AG Grid Community lacks some enterprise features (server-side row model, integrated charts). If needed later, TanStack Table is a fallback. React 19's concurrent features help with streaming data but require careful use of transitions. |
| **Product Justification** | This IS the product surface. Users sit in front of this 8+ hours a day. AG Grid handles the streaming blotter performance. Zustand avoids unnecessary re-renders so the UI stays responsive during high-volume trading. Dark terminal aesthetic signals "this is a professional tool" to the target user. |

### ADR-004: Apache Kafka as Event Backbone

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Context** | Order lifecycle events must flow from the Gateway to the Risk Engine (for portfolio state), AI modules (for analysis), and potentially future consumers (audit, compliance, backtesting replay). Need durable, ordered, replayable event stream. |
| **Decision** | Apache Kafka 3.7+ (KRaft mode, no Zookeeper) |
| **Rationale** | KRaft eliminates Zookeeper dependency, reducing operational complexity for self-hosted users (one fewer service). Kafka provides partition-based ordering by instrument symbol (all events for AAPL go to the same partition, preserving order lifecycle sequence). Log-based durability means a restarted Risk Engine can replay events to rebuild state. Consumer groups allow independent consumption by risk engine, AI modules, and future services. |
| **Consequences** | Kafka is heavyweight for a self-hosted deployment — mitigated by using a single-broker setup in Docker Compose (sufficient for personal/small-fund use). KRaft mode is production-ready as of Kafka 3.5. Memory footprint ~512MB–1GB for the broker. For users who find even this too heavy, a future ADR could evaluate NATS JetStream as a lighter alternative. |
| **Product Justification** | Event replay is critical: if the risk engine restarts, it rebuilds its portfolio state from the event log rather than requiring a complex state-sync protocol. Partition-by-symbol ordering guarantees correct order lifecycle sequencing. Durable event log also enables future features: trade journal, backtest replay, audit trail. |

### ADR-005: Communication Protocols — WebSocket + REST + gRPC

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Context** | Three distinct communication patterns exist: (1) real-time streaming to the browser, (2) human-initiated commands, (3) low-latency internal service calls. |
| **Decision** | WebSocket for client streaming (order updates, market data, risk metrics). REST (JSON over HTTP) for client commands (submit order, cancel, query). gRPC for internal synchronous calls (risk pre-trade checks). |
| **Rationale** | WebSocket: browsers natively support it, and the streaming use case (blotter updates, position P&L ticks) maps perfectly. REST: human-speed operations (submit order, connect venue) don't need gRPC's complexity on the client side. gRPC: the pre-trade risk check is the only synchronous internal call and needs <10ms latency; gRPC with Protobuf is optimal here. Using gRPC for browser communication would require grpc-web proxy, adding deployment complexity for self-hosted users. |
| **Consequences** | Three protocol implementations to maintain. WebSocket reconnection logic needed in the frontend (handled by standard reconnecting-websocket libraries). REST API needs OpenAPI documentation (auto-generated by Go's swag or equivalent). |
| **Product Justification** | Users see smooth, real-time streaming updates in the blotter and risk dashboard (WebSocket). They submit orders via simple REST calls from the UI. The risk check happens invisibly in <10ms (gRPC). Each protocol is chosen for its specific interaction pattern, not for architectural purity. |

### ADR-006: Protocol Buffers for Internal Schemas

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Context** | Three services (Go, Python, TypeScript) need shared message definitions for domain events, gRPC service contracts, and Kafka message payloads. |
| **Decision** | Protocol Buffers v3 with `buf` for linting, breaking change detection, and code generation targeting Go, Python, and TypeScript. |
| **Rationale** | Single source of truth for all cross-service types. Buf provides schema evolution safety (detect breaking changes in CI). Code generation eliminates hand-maintained type duplication. Proto3's compact binary encoding reduces Kafka message sizes. TypeScript types generated from proto ensure frontend type safety matches backend contracts. |
| **Consequences** | Proto generation step in CI/CD pipeline. Developers must run `scripts/proto-gen.sh` after proto changes. Buf adds a dev dependency. JSON encoding available for debugging (Proto3 has canonical JSON mapping). |
| **Product Justification** | When a community contributor adds a new venue adapter, the Protobuf definitions tell them exactly what messages to produce and consume. Schema-first development prevents integration bugs between Go, Python, and TypeScript codebases. |

### ADR-007: PostgreSQL + Redis for Data Storage

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Context** | Need persistent storage for orders, fills, positions, instruments, venue credentials, and historical market data. Need hot caches for order book state, session data, and frequently-accessed risk snapshots. |
| **Decision** | PostgreSQL 16 for persistent state. Redis 7 for hot caches, order book state, and pub/sub for WebSocket fanout. |
| **Rationale** | PostgreSQL: ACID transactions for order state transitions (prevent double-fills), JSONB columns for venue-specific metadata, partitioned tables for historical fill data, strong ecosystem. Redis: sub-millisecond reads for order book state, Sorted Sets for price-level order books, pub/sub for fan-out to multiple WebSocket connections, TTL-based cache expiry for risk snapshots. |
| **Consequences** | Two data stores to operate — mitigated by both being well-supported in Docker Compose. PostgreSQL connection pooling (pgxpool in Go, asyncpg in Python) needed for concurrent access. Redis persistence (RDB snapshots) for crash recovery of hot state. |
| **Product Justification** | Self-hosted users need reliable data storage that survives restarts. PostgreSQL gives them a mature, well-understood database they can back up with standard tools. Redis keeps the trading interface responsive with sub-millisecond cache reads for position lookups and risk snapshots. |

### ADR-008: Docker Compose Primary, Kubernetes Secondary

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Context** | Target users are self-hosting on personal machines, NUCs, or small cloud VMs. The primary deployment story must be `docker compose up`. Some users (small funds, RIAs) may want production-grade deployment on Kubernetes. |
| **Decision** | Docker Compose as the primary and default deployment method. Kubernetes manifests provided for production-like deployment. |
| **Rationale** | `docker compose up` is the lowest-friction deployment for the target user. One file defines the full topology. No Helm, no kubectl, no cluster setup. For users who want more (health checks, rolling updates, resource limits), Kubernetes manifests are provided but are not the happy path. |
| **Consequences** | Must ensure Docker Compose topology is well-tested and documented. Startup ordering matters (Kafka must be healthy before Gateway starts). Health checks and depends_on with conditions address this. Kubernetes manifests may drift from Docker Compose — CI should validate both. |
| **Product Justification** | The promise is "from git clone to running system in 3 minutes." Docker Compose is the only deployment tool that delivers this. A user shouldn't need to understand Kubernetes to trade across multiple venues with unified risk. |

### ADR-009: Self-Hosted Architecture & License

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Context** | API keys, trading strategies, position data, and execution history are extremely sensitive. A SaaS model would require users to trust a third party with their exchange credentials and trading activity. The target user actively avoids this. |
| **Decision** | Self-hosted-only architecture. No cloud dependency. All data stored locally. License: AGPLv3. |
| **Rationale** | Self-hosted: exchange API keys are stored encrypted on the user's machine, never transmitted. Trading strategies (order patterns, timing) are private. Position data reveals portfolio composition. None of this should leave the user's control. AGPLv3: ensures modifications to the system that are deployed as a service must be open-sourced (prevents cloud providers from offering a proprietary hosted SynapseOMS). Allows commercial use by the target users (small funds, RIAs) while protecting the open-source project from capture. |
| **Consequences** | No managed service revenue model. Documentation must clearly explain self-hosting requirements. AGPLv3 may deter some corporate contributors — but the target users (individual traders, small funds) are not constrained by AGPLv3. If corporate adoption becomes a goal, dual-licensing or BSL can be evaluated later. |
| **Product Justification** | "Your keys never leave your machine" is not a limitation — it's the headline feature. Users who trade across Binance, Alpaca, and potentially Interactive Brokers are entrusting the system with API keys that can move real money. Self-hosted architecture makes this trust model viable. |

---

## 3. Service Contracts & Domain Model

### 3.1 Core Domain Entities

#### Order

```
Owner: Order Gateway (gateway)
Storage: PostgreSQL (orders table)
Communication: Kafka topic `order-lifecycle`, gRPC risk check, WebSocket to dashboard
```

```go
// gateway/internal/domain/order.go

type OrderID string
type OrderSide int
const (
    SideBuy OrderSide = iota
    SideSell
)

type OrderType int
const (
    OrderTypeMarket OrderType = iota
    OrderTypeLimit
    OrderTypeStopLimit
)

type OrderStatus int
const (
    OrderStatusNew OrderStatus = iota
    OrderStatusAcknowledged     // Venue accepted
    OrderStatusPartiallyFilled  // Some fills received
    OrderStatusFilled           // Fully filled
    OrderStatusCanceled         // User-initiated cancel confirmed
    OrderStatusRejected         // Venue or risk check rejected
)

type Order struct {
    ID              OrderID
    ClientOrderID   string          // User-supplied idempotency key
    InstrumentID    string          // e.g., "AAPL", "ETH-USD"
    Side            OrderSide
    Type            OrderType
    Quantity        decimal.Decimal // Total order quantity
    Price           decimal.Decimal // Limit price (zero for market)
    FilledQuantity  decimal.Decimal
    AveragePrice    decimal.Decimal // Volume-weighted fill price
    Status          OrderStatus
    VenueID         string          // Target or routed venue
    AssetClass      AssetClass      // Equity, Crypto, TokenizedSecurity
    SettlementCycle SettlementCycle // T0, T1, T2
    CreatedAt       time.Time
    UpdatedAt       time.Time
    Fills           []Fill
}

// State machine transitions — returns error on invalid transition
func (o *Order) ApplyTransition(newStatus OrderStatus) error
```

**State Machine:**
```
New → Acknowledged → PartiallyFilled → Filled
New → Rejected
Acknowledged → Canceled
Acknowledged → Rejected
PartiallyFilled → Filled
PartiallyFilled → Canceled (partial cancel)
```

#### Fill (Execution Report)

```
Owner: Order Gateway
Storage: PostgreSQL (fills table, partitioned by date)
Communication: Kafka topic `order-lifecycle` (FillReceived event)
```

```go
type Fill struct {
    ID          string
    OrderID     OrderID
    VenueID     string
    Quantity    decimal.Decimal
    Price       decimal.Decimal
    Fee         decimal.Decimal
    FeeAsset    string          // USD, BTC, etc.
    FeeModel    FeeModel        // MakerTaker, Commission, Flat
    Liquidity   LiquidityType   // Maker, Taker, Internal (crossing)
    Timestamp   time.Time
    VenueExecID string          // Venue's execution ID
}
```

#### Instrument

```
Owner: Order Gateway (reference data)
Storage: PostgreSQL (instruments table), Redis (hot cache)
Communication: Loaded at startup, refreshed periodically, available via REST
```

```go
type AssetClass int
const (
    AssetClassEquity AssetClass = iota
    AssetClassCrypto
    AssetClassTokenizedSecurity
    AssetClassFuture
    AssetClassOption
)

type SettlementCycle int
const (
    SettlementT0 SettlementCycle = iota // Crypto, tokenized
    SettlementT1                        // Some markets
    SettlementT2                        // US equities
)

type Instrument struct {
    ID              string          // "AAPL", "ETH-USD", "TSLA-2026-06-19-C-200"
    Symbol          string          // Display symbol
    Name            string
    AssetClass      AssetClass
    QuoteCurrency   string          // USD, USDT, BTC
    BaseCurrency    string          // For crypto pairs
    TickSize        decimal.Decimal // Minimum price increment
    LotSize         decimal.Decimal // Minimum quantity increment
    SettlementCycle SettlementCycle
    TradingHours    TradingSchedule // 24/7 for crypto, market hours for equity
    Venues          []string        // Venue IDs where this instrument trades
    MarginRequired  decimal.Decimal // Initial margin requirement
}

type TradingSchedule struct {
    Is24x7      bool
    MarketOpen  string // "09:30" ET
    MarketClose string // "16:00" ET
    PreMarket   string // "04:00" ET
    AfterHours  string // "20:00" ET
    Timezone    string // "America/New_York"
}
```

#### Position

```
Owner: Order Gateway (authoritative), Risk Engine (derived from events)
Storage: PostgreSQL (positions table), Redis (hot cache)
Communication: Kafka topic `order-lifecycle`, REST query
```

```go
type Position struct {
    InstrumentID     string
    VenueID          string
    Quantity         decimal.Decimal // Signed: positive = long, negative = short
    AverageCost      decimal.Decimal
    MarketPrice      decimal.Decimal // Latest market price
    UnrealizedPnL    decimal.Decimal
    RealizedPnL      decimal.Decimal
    UnsettledQuantity decimal.Decimal // For T+2 equities
    SettledQuantity  decimal.Decimal
    AssetClass       AssetClass
    QuoteCurrency    string
    UpdatedAt        time.Time
}
```

#### Portfolio

```
Owner: Risk Engine
Storage: In-memory (rebuilt from Kafka events on startup), snapshots to PostgreSQL
Communication: REST (query), gRPC (risk checks reference portfolio state)
```

```python
# risk_engine/domain/portfolio.py
from dataclasses import dataclass, field
from decimal import Decimal

@dataclass
class Portfolio:
    positions: dict[str, Position]  # instrument_id → Position
    total_nav: Decimal              # Net Asset Value in base currency (USD)
    cash_balance: Decimal
    unsettled_cash: Decimal         # Cash tied up in T+2 settlements
    available_cash: Decimal         # cash_balance - unsettled_cash
    last_updated: datetime

    def exposure_by_asset_class(self) -> dict[AssetClass, Decimal]: ...
    def exposure_by_venue(self) -> dict[str, Decimal]: ...
    def exposure_by_sector(self) -> dict[str, Decimal]: ...
    def concentration_single_name(self) -> dict[str, float]: ...  # instrument → % of NAV
```

#### LiquidityVenue

```
Owner: Order Gateway
Storage: PostgreSQL (venues table), Redis (health state)
Communication: REST (venue management), WebSocket (health status streaming)
```

```go
type VenueStatus int
const (
    VenueStatusConnected VenueStatus = iota
    VenueStatusDisconnected
    VenueStatusDegraded
    VenueStatusAuthentication // Credentials invalid
)

type LiquidityVenue struct {
    ID                string
    Name              string
    Type              VenueType       // Exchange, DarkPool, Simulated, Tokenized
    Status            VenueStatus
    SupportedAssets   []AssetClass
    LatencyP50        time.Duration
    LatencyP99        time.Duration
    FillRate          float64         // Percentage of orders filled
    LastHeartbeat     time.Time
    Capabilities      VenueCapabilities
}

type VenueCapabilities struct {
    SupportsMarketOrders bool
    SupportsLimitOrders  bool
    SupportsStreaming    bool
    MaxOrdersPerSecond   int
}
```

#### VenueCredential

```
Owner: Order Gateway (credential manager)
Storage: PostgreSQL (encrypted at rest)
Communication: Never transmitted externally. REST for CRUD (write-only for secret fields)
```

```go
type VenueCredential struct {
    VenueID         string
    Label           string          // User-friendly name ("My Alpaca Paper")
    APIKey          string          // Encrypted with master passphrase
    APISecret       string          // Encrypted with master passphrase
    AdditionalData  map[string]string // Encrypted, venue-specific (passphrase, subaccount, etc.)
    CreatedAt       time.Time
    LastValidated   time.Time
    IsValid         bool
}
// Secret fields are NEVER returned in REST responses.
// Encryption: AES-256-GCM with key derived from master passphrase via Argon2id.
```

#### RiskCheckResult

```
Owner: Risk Engine
Communication: gRPC response to gateway pre-trade check
```

```python
@dataclass
class RiskCheckResult:
    approved: bool
    order_id: str
    checks: list[RiskCheck]
    portfolio_var_before: Decimal
    portfolio_var_after: Decimal    # Projected VaR if order fills
    concentration_warning: str | None
    max_loss_estimate: Decimal
    computed_at: datetime

@dataclass
class RiskCheck:
    name: str            # "position_limit", "var_limit", "concentration", "drawdown"
    passed: bool
    message: str
    threshold: Decimal
    actual: Decimal
```

#### MarketDataSnapshot

```
Owner: Order Gateway (via venue adapters)
Storage: Redis (latest), PostgreSQL (historical OHLCV)
Communication: Kafka topic `market-data`, WebSocket to dashboard
```

```go
type MarketDataSnapshot struct {
    InstrumentID string
    VenueID      string
    BidPrice     decimal.Decimal
    AskPrice     decimal.Decimal
    BidSize      decimal.Decimal
    AskSize      decimal.Decimal
    LastPrice    decimal.Decimal
    Volume24h    decimal.Decimal
    VWAP         decimal.Decimal
    Timestamp    time.Time
}
```

### 3.2 Entity Ownership & Communication Matrix

| Entity | Owner Service | PostgreSQL | Redis | Kafka Topic | gRPC | REST | WebSocket |
|--------|--------------|------------|-------|-------------|------|------|-----------|
| Order | Gateway | Write/Read | Status cache | `order-lifecycle` (publish) | — | CRUD | Status stream |
| Fill | Gateway | Write/Read | — | `order-lifecycle` (publish) | — | Read | Fill stream |
| Position | Gateway | Write/Read | Hot cache | `order-lifecycle` (derived) | — | Read | P&L stream |
| Portfolio | Risk Engine | Snapshots | — | `order-lifecycle` (consume) | Risk check context | Read | Risk stream |
| Instrument | Gateway | Write/Read | Hot cache | — | — | Read | — |
| LiquidityVenue | Gateway | Write/Read | Health state | — | — | CRUD | Health stream |
| VenueCredential | Gateway | Encrypted Write/Read | — | — | — | Write-only | — |
| RiskCheckResult | Risk Engine | — | — | — | Response | Read | — |
| MarketDataSnapshot | Gateway | Historical | Latest tick | `market-data` (publish) | — | Read | Tick stream |

---

## 4. Service Breakdown

### 4A. Order Gateway & Execution Engine (Go)

#### Technology Stack

| Component | Library | Version |
|-----------|---------|---------|
| HTTP/REST | `net/http` + `chi` router | chi v5 |
| WebSocket | `gorilla/websocket` | v1.5 |
| gRPC client | `google.golang.org/grpc` | v1.63 |
| Kafka producer | `github.com/confluentinc/confluent-kafka-go/v2` | v2.4 |
| PostgreSQL | `github.com/jackc/pgx/v5` | v5.6 |
| Redis | `github.com/redis/go-redis/v9` | v9.5 |
| Decimal | `github.com/shopspring/decimal` | v1.4 |
| Config | `github.com/spf13/viper` | v1.19 |
| Logging | `log/slog` (stdlib structured logging) | Go 1.22 |
| Metrics | `github.com/prometheus/client_golang` | v1.19 |
| Protobuf | `google.golang.org/protobuf` | v1.34 |
| Encryption | `golang.org/x/crypto` (Argon2id, AES-256-GCM) | latest |

#### LiquidityProvider Interface

```go
// gateway/internal/adapter/provider.go

// LiquidityProvider is the interface every venue adapter implements.
// Community contributors add new venues by implementing this interface.
type LiquidityProvider interface {
    // Identity
    VenueID() string
    VenueName() string
    SupportedAssetClasses() []AssetClass
    SupportedInstruments() ([]Instrument, error)

    // Connection lifecycle
    Connect(ctx context.Context, cred VenueCredential) error
    Disconnect(ctx context.Context) error
    Status() VenueStatus
    Ping(ctx context.Context) (latency time.Duration, err error)

    // Order management
    SubmitOrder(ctx context.Context, order *Order) (*VenueAck, error)
    CancelOrder(ctx context.Context, orderID OrderID, venueOrderID string) error
    QueryOrder(ctx context.Context, venueOrderID string) (*Order, error)

    // Market data
    SubscribeMarketData(ctx context.Context, instruments []string) (<-chan MarketDataSnapshot, error)
    UnsubscribeMarketData(ctx context.Context, instruments []string) error

    // Fills — venue pushes fills through this channel
    FillFeed() <-chan Fill

    // Capabilities
    Capabilities() VenueCapabilities
}
```

#### Adapter Registration

```go
// gateway/internal/adapter/registry.go

type AdapterFactory func(config map[string]string) LiquidityProvider

var registry = map[string]AdapterFactory{}

func Register(venueType string, factory AdapterFactory) {
    registry[venueType] = factory
}

func init() {
    // Built-in adapters register themselves
    Register("alpaca", alpaca.NewAdapter)
    Register("binance_testnet", binance.NewAdapter)
    Register("simulated", simulated.NewAdapter)
    Register("tokenized", tokenized.NewAdapter)
}
```

#### Concurrency Model

The gateway processes orders through a pipeline of goroutines connected by channels:

```
                    ┌──────────────┐
   REST/WS Input →  │  Intake Chan  │  (buffered, 10,000 capacity)
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │  Risk Check   │  N goroutines (pool of 32)
                    │  (gRPC call)  │  Synchronous per order, concurrent across orders
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │  Router       │  Single goroutine per instrument (fan-out by symbol)
                    │  (venue pick) │  Uses ML scorer, best-price, or rule-based strategy
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │  Venue Dispatch│  One goroutine per venue adapter
                    │  (submit)     │  Each adapter manages its own connection
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │  Fill Collector│  Merges fills from all venue FillFeed() channels
                    │               │  Updates order state, publishes to Kafka
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │  Notifier     │  Fans out to WebSocket clients + Kafka
                    └──────────────┘
```

Key concurrency patterns:
- **Per-instrument routing goroutine:** Orders for the same instrument are serialized through a dedicated goroutine (selected by hash of instrument ID → channel). This prevents race conditions in order book state while allowing full parallelism across instruments.
- **Venue adapter goroutines:** Each adapter runs its own event loop goroutine. Connection management, heartbeats, and reconnection are isolated per venue.
- **Context-based cancellation:** All goroutines accept `context.Context` for graceful shutdown. Main process cancels root context on SIGTERM, all goroutines drain within 5s.

```go
// Simplified pipeline initialization
func NewPipeline(riskClient RiskClient, router *Router, venues AdapterRegistry) *Pipeline {
    intake := make(chan *Order, 10000)
    riskChecked := make(chan *Order, 5000)
    routed := make(chan *RoutedOrder, 5000)
    fills := make(chan *Fill, 10000)

    return &Pipeline{
        intake:      intake,
        riskChecked: riskChecked,
        routed:      routed,
        fills:       fills,
    }
}

func (p *Pipeline) Start(ctx context.Context) {
    // Risk check pool — 32 concurrent gRPC calls
    for i := 0; i < 32; i++ {
        go p.riskCheckWorker(ctx, p.intake, p.riskChecked)
    }
    // Router — single goroutine per instrument (lazy init)
    go p.routerDispatcher(ctx, p.riskChecked, p.routed)
    // Venue dispatch — one per adapter
    for _, venue := range p.venues.All() {
        go p.venueWorker(ctx, venue, p.routed, p.fills)
    }
    // Fill collector
    go p.fillCollector(ctx, p.fills)
}
```

#### Kafka Topics Published

| Topic | Partition Key | Message Type | Schema |
|-------|--------------|--------------|--------|
| `order-lifecycle` | `instrument_id` | `OrderCreated`, `OrderAcknowledged`, `OrderRouted`, `FillReceived`, `OrderCompleted`, `OrderCanceled`, `OrderRejected` | `proto/order/order.proto` |
| `market-data` | `instrument_id` | `MarketDataUpdate` | `proto/marketdata/marketdata.proto` |
| `venue-status` | `venue_id` | `VenueConnected`, `VenueDisconnected`, `VenueDegraded` | `proto/venue/venue.proto` |

#### REST API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/orders` | Submit new order |
| `DELETE` | `/api/v1/orders/{id}` | Cancel order |
| `GET` | `/api/v1/orders` | List orders (filter by status, instrument, venue) |
| `GET` | `/api/v1/orders/{id}` | Get order detail with fills |
| `GET` | `/api/v1/positions` | List all positions |
| `GET` | `/api/v1/positions/{instrument_id}` | Get position for instrument |
| `GET` | `/api/v1/venues` | List connected venues with status |
| `POST` | `/api/v1/venues/{id}/connect` | Connect to a venue |
| `POST` | `/api/v1/venues/{id}/disconnect` | Disconnect from a venue |
| `POST` | `/api/v1/credentials` | Store venue credentials (encrypted) |
| `DELETE` | `/api/v1/credentials/{venue_id}` | Remove venue credentials |
| `GET` | `/api/v1/instruments` | List available instruments |
| `GET` | `/api/v1/health` | System health check |

#### WebSocket Streams

| Endpoint | Payload | Rate |
|----------|---------|------|
| `/ws/orders` | Order status changes, new fills | Per event |
| `/ws/positions` | Position updates, P&L ticks | 100ms throttled |
| `/ws/marketdata/{instrument_id}` | BBO updates | Per venue tick |
| `/ws/venues` | Venue health status | Per state change |

#### Credential Manager Design

```go
// gateway/internal/credential/manager.go

// Encryption flow:
// 1. User provides master passphrase on first run (or via env var SYNAPSE_MASTER_PASSPHRASE)
// 2. Argon2id derives a 256-bit key from the passphrase (salt stored alongside)
// 3. Each credential field encrypted with AES-256-GCM using the derived key
// 4. Encrypted blobs stored in PostgreSQL
// 5. On startup, master passphrase unlocks all credentials in memory
// 6. Credentials never logged, never returned via REST API

type CredentialManager struct {
    derivedKey []byte          // In-memory only, derived from master passphrase
    db         *pgxpool.Pool
}

func (m *CredentialManager) Store(ctx context.Context, cred VenueCredential) error
func (m *CredentialManager) Retrieve(ctx context.Context, venueID string) (*VenueCredential, error)
func (m *CredentialManager) Delete(ctx context.Context, venueID string) error
func (m *CredentialManager) ValidateAll(ctx context.Context) map[string]error // Startup health check
```

#### Simulated Exchange Design

The simulated exchange provides a fully functional matching engine for development and demo:

```go
// gateway/internal/adapter/simulated/matching_engine.go

type MatchingEngine struct {
    books      map[string]*OrderBook  // instrument → order book
    priceWalks map[string]*PriceWalk  // instrument → synthetic price generator
}

// PriceWalk generates synthetic prices using geometric Brownian motion
type PriceWalk struct {
    currentPrice decimal.Decimal
    volatility   float64         // Annualized vol (0.30 for equity-like, 0.80 for crypto-like)
    drift        float64
    ticker       *time.Ticker    // Tick interval (100ms default)
}
```

The simulated exchange:
- Pre-loads a universe of instruments (AAPL, MSFT, GOOG, BTC-USD, ETH-USD, SOL-USD)
- Generates synthetic prices via geometric Brownian motion with configurable volatility
- Fills market orders immediately at synthetic price with random slippage (0-5bps)
- Fills limit orders when synthetic price crosses the limit
- Simulates partial fills for large orders (>10% of synthetic volume)
- Applies realistic fee models (0.1% maker/taker for crypto-like, $0.005/share for equity-like)

### 4B. Risk & Analytics Engine (Python)

#### Technology Stack

| Component | Library | Version |
|-----------|---------|---------|
| REST API | FastAPI | 0.111 |
| ASGI Server | uvicorn | 0.30 |
| gRPC server | grpcio + grpcio-tools | 1.64 |
| Kafka consumer | confluent-kafka | 2.4 |
| Numerical | numpy | 1.26 |
| Statistics | scipy | 1.13 |
| DataFrames | pandas | 2.2 |
| Optimization | cvxpy | 1.5 |
| ML (anomaly) | scikit-learn | 1.5 |
| PostgreSQL | asyncpg | 0.29 |
| Redis | redis-py | 5.0 |
| Protobuf | protobuf | 5.27 |
| Metrics | prometheus-client | 0.20 |
| Logging | structlog | 24.2 |

#### gRPC Service Definition

```protobuf
// proto/risk/risk.proto

syntax = "proto3";
package synapse.risk;

service RiskGate {
    // Synchronous pre-trade risk check — called by Order Gateway before routing
    rpc CheckPreTradeRisk(PreTradeRiskRequest) returns (PreTradeRiskResponse);
}

message PreTradeRiskRequest {
    string order_id = 1;
    string instrument_id = 2;
    string side = 3;              // BUY, SELL
    string quantity = 4;          // decimal string
    string price = 5;             // decimal string (0 for market)
    string asset_class = 6;
    string venue_id = 7;
}

message PreTradeRiskResponse {
    bool approved = 1;
    string reject_reason = 2;     // Empty if approved
    repeated RiskCheck checks = 3;
    string portfolio_var_before = 4;  // decimal string
    string portfolio_var_after = 5;   // decimal string
    int64 computed_at_ms = 6;
}

message RiskCheck {
    string name = 1;
    bool passed = 2;
    string message = 3;
    string threshold = 4;
    string actual = 5;
}
```

#### VaR Computation Design

```python
# risk_engine/var/historical.py

class HistoricalVaR:
    """
    Historical simulation VaR.
    Uses a rolling window of actual returns to estimate portfolio loss distribution.
    Handles mixed trading calendars: crypto returns are daily (24/7), equity returns
    skip weekends/holidays.
    """
    def __init__(self, window_days: int = 252, confidence: float = 0.99):
        self.window_days = window_days
        self.confidence = confidence

    def compute(
        self,
        positions: dict[str, Position],
        returns_matrix: pd.DataFrame,  # columns = instrument_id, index = date
        base_currency: str = "USD",
    ) -> VaRResult:
        """
        1. Align returns to common dates (forward-fill crypto returns on equity holidays)
        2. Compute portfolio returns: sum(position_weight_i * return_i) for each date
        3. Sort portfolio returns
        4. VaR = -percentile(portfolio_returns, 1 - confidence)
        5. CVaR = -mean(portfolio_returns below VaR threshold)
        """
        ...
```

```python
# risk_engine/var/monte_carlo.py

class MonteCarloVaR:
    """
    Monte Carlo VaR with correlated return paths.
    Respects fat-tailed distributions for crypto (Student-t) vs
    near-normal for equities.
    """
    def __init__(
        self,
        num_simulations: int = 10_000,
        horizon_days: int = 1,
        confidence: float = 0.99,
    ):
        self.num_simulations = num_simulations
        self.horizon_days = horizon_days
        self.confidence = confidence

    def compute(
        self,
        positions: dict[str, Position],
        covariance_matrix: np.ndarray,
        expected_returns: np.ndarray,
        distribution_params: dict[str, DistributionParams],  # per-instrument
    ) -> VaRResult:
        """
        1. Cholesky decomposition of covariance matrix
        2. For each simulation:
           a. Generate correlated random returns (Student-t for crypto, normal for equity)
           b. Apply Cholesky factor to induce correlation
           c. Compute portfolio value change
        3. VaR = -percentile(simulated_pnl, 1 - confidence)
        4. Return full distribution for frontend visualization
        """
        ...
```

#### Settlement-Aware Risk

```python
# risk_engine/settlement/tracker.py

class SettlementTracker:
    """
    Tracks unsettled trades and their impact on available cash/buying power.

    Crypto (T+0): settled immediately — full buying power available
    Equities (T+2): cash from sell not available for 2 business days
                     bought shares are in portfolio but cash is committed

    This matters for risk: a user who sold $50k of AAPL and bought $50k of ETH
    has $50k of unsettled equity cash for 2 business days. If ETH drops 20%
    during that window, they can't sell the unsettled equity cash to cover.
    """
    def compute_settlement_risk(
        self,
        pending_settlements: list[PendingSettlement],
        current_positions: dict[str, Position],
    ) -> SettlementRiskResult:
        ...

@dataclass
class PendingSettlement:
    trade_date: date
    settlement_date: date
    instrument_id: str
    asset_class: AssetClass
    amount: Decimal                 # Positive = cash incoming, negative = cash outgoing
    status: SettlementStatus        # Pending, Settled, Failed
```

#### Portfolio Construction Optimizer

```python
# risk_engine/optimizer/mean_variance.py

class PortfolioOptimizer:
    """
    Mean-variance optimization using cvxpy.
    Accepts user constraints and produces a target allocation + trade list.
    """
    def optimize(
        self,
        current_positions: dict[str, Position],
        expected_returns: np.ndarray,
        covariance_matrix: np.ndarray,
        constraints: OptimizationConstraints,
    ) -> OptimizationResult:
        n = len(current_positions)
        w = cp.Variable(n)

        # Objective: maximize Sharpe (or minimize variance for given return target)
        portfolio_return = expected_returns @ w
        portfolio_risk = cp.quad_form(w, covariance_matrix)

        objective = cp.Maximize(portfolio_return - constraints.risk_aversion * portfolio_risk)

        constraint_list = [cp.sum(w) == 1]  # Fully invested

        if constraints.long_only:
            constraint_list.append(w >= 0)
        if constraints.max_single_weight:
            constraint_list.append(w <= constraints.max_single_weight)
        if constraints.target_volatility:
            constraint_list.append(cp.sqrt(portfolio_risk) <= constraints.target_volatility)
        if constraints.max_turnover:
            current_weights = self._current_weights(current_positions)
            constraint_list.append(cp.norm(w - current_weights, 1) <= constraints.max_turnover)
        if constraints.asset_class_bounds:
            for ac, (lo, hi) in constraints.asset_class_bounds.items():
                mask = self._asset_class_mask(ac)
                constraint_list.append(w @ mask >= lo)
                constraint_list.append(w @ mask <= hi)

        problem = cp.Problem(objective, constraint_list)
        problem.solve(solver=cp.ECOS)

        target_weights = w.value
        trades = self._compute_trade_list(current_positions, target_weights)
        return OptimizationResult(target_weights=target_weights, trades=trades, ...)

@dataclass
class OptimizationConstraints:
    risk_aversion: float = 1.0
    long_only: bool = True
    max_single_weight: float | None = None       # e.g., 0.20 = 20% max
    sector_limits: dict[str, float] | None = None
    target_volatility: float | None = None
    max_turnover: float | None = None             # e.g., 0.30 = 30% turnover limit
    asset_class_bounds: dict[str, tuple[float, float]] | None = None  # e.g., {"crypto": (0.1, 0.3)}
```

#### REST API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/risk/var` | Current VaR (historical, parametric, Monte Carlo) |
| `GET` | `/api/v1/risk/greeks` | Portfolio Greeks |
| `GET` | `/api/v1/risk/concentration` | Concentration risk breakdown |
| `GET` | `/api/v1/risk/drawdown` | Drawdown history and current |
| `GET` | `/api/v1/risk/settlement` | Settlement risk timeline |
| `POST` | `/api/v1/risk/scenario` | What-if scenario (add hypothetical position) |
| `GET` | `/api/v1/portfolio` | Current portfolio state |
| `GET` | `/api/v1/portfolio/exposure` | Exposure breakdown |
| `POST` | `/api/v1/optimizer/optimize` | Run portfolio optimization |
| `GET` | `/api/v1/anomalies` | Recent anomaly alerts |
| `GET` | `/api/v1/health` | Service health |

#### Anomaly Detection

```python
# risk_engine/anomaly/detector.py

class StreamingAnomalyDetector:
    """
    Isolation Forest on streaming market data features.
    Maintains a sliding window of feature vectors and retrains periodically.

    Features per instrument per window:
    - Volume z-score (vs 30-day rolling mean)
    - Price return z-score
    - Bid-ask spread z-score
    - Volume/price correlation shift
    - Cross-venue price divergence (if multi-venue)
    """
    def __init__(self, retrain_interval_minutes: int = 60, contamination: float = 0.01):
        self.model = IsolationForest(contamination=contamination, random_state=42)
        self.feature_window: deque[np.ndarray] = deque(maxlen=10_000)
        self.retrain_interval = retrain_interval_minutes

    def ingest(self, snapshot: MarketDataSnapshot) -> AnomalyAlert | None:
        features = self._extract_features(snapshot)
        self.feature_window.append(features)

        if self._should_retrain():
            self.model.fit(np.array(self.feature_window))

        score = self.model.decision_function(features.reshape(1, -1))[0]
        if score < self.anomaly_threshold:
            return AnomalyAlert(
                instrument_id=snapshot.instrument_id,
                venue_id=snapshot.venue_id,
                anomaly_score=score,
                features=self._describe_features(features),
                timestamp=snapshot.timestamp,
            )
        return None
```

### 4C. Frontend Dashboard (TypeScript + React)

#### Technology Stack

| Component | Library | Version |
|-----------|---------|---------|
| Framework | React | 19.x |
| Build | Vite | 6.x |
| Language | TypeScript | 5.5+ |
| State | Zustand | 5.x |
| Data Grid | AG Grid Community | 32.x |
| Charts (financial) | Lightweight Charts | 4.x |
| Charts (analytics) | Recharts | 2.x |
| Charts (custom) | D3 | 7.x |
| HTTP client | ky | 1.x |
| Router | React Router | 7.x |
| CSS | Tailwind CSS | 4.x |
| UI primitives | Radix UI | latest |
| Icons | Lucide React | latest |
| WebSocket | reconnecting-websocket | 1.x |

#### Core TypeScript Types

```typescript
// dashboard/src/api/types.ts — Generated from proto, extended for UI

type AssetClass = "equity" | "crypto" | "tokenized_security" | "future" | "option";
type SettlementCycle = "T0" | "T1" | "T2";
type OrderStatus = "new" | "acknowledged" | "partially_filled" | "filled" | "canceled" | "rejected";
type OrderSide = "buy" | "sell";
type OrderType = "market" | "limit" | "stop_limit";

interface Order {
  id: string;
  clientOrderId: string;
  instrumentId: string;
  side: OrderSide;
  type: OrderType;
  quantity: string;         // decimal as string for precision
  price: string;
  filledQuantity: string;
  averagePrice: string;
  status: OrderStatus;
  venueId: string;
  assetClass: AssetClass;
  createdAt: string;        // ISO 8601
  updatedAt: string;
  fills: Fill[];
}

interface Fill {
  id: string;
  orderId: string;
  venueId: string;
  quantity: string;
  price: string;
  fee: string;
  feeAsset: string;
  liquidity: "maker" | "taker" | "internal";
  timestamp: string;
}

interface Position {
  instrumentId: string;
  venueId: string;
  quantity: string;
  averageCost: string;
  marketPrice: string;
  unrealizedPnl: string;
  realizedPnl: string;
  unsettledQuantity: string;
  assetClass: AssetClass;
  quoteCurrency: string;
}

interface VaRMetrics {
  historicalVaR: string;
  parametricVaR: string;
  monteCarloVaR: string;
  cvar: string;
  confidence: number;
  horizon: string;
  computedAt: string;
  monteCarloDistribution: number[];  // For histogram rendering
}

interface Venue {
  id: string;
  name: string;
  type: "exchange" | "dark_pool" | "simulated" | "tokenized";
  status: "connected" | "disconnected" | "degraded" | "authentication";
  supportedAssets: AssetClass[];
  latencyP50Ms: number;
  latencyP99Ms: number;
  fillRate: number;
  lastHeartbeat: string;
  hasCredentials: boolean;
}
```

#### Zustand Store Design

```typescript
// dashboard/src/stores/orderStore.ts

interface OrderStore {
  orders: Map<string, Order>;
  activeOrders: () => Order[];       // Derived: status not terminal
  ordersByInstrument: (id: string) => Order[];

  // Actions
  submitOrder: (req: OrderRequest) => Promise<Order>;
  cancelOrder: (orderId: string) => Promise<void>;
  applyUpdate: (update: OrderUpdate) => void;  // Called by WebSocket handler
}

export const useOrderStore = create<OrderStore>((set, get) => ({
  orders: new Map(),

  activeOrders: () => {
    const terminal = new Set(["filled", "canceled", "rejected"]);
    return [...get().orders.values()].filter(o => !terminal.has(o.status));
  },

  ordersByInstrument: (id: string) =>
    [...get().orders.values()].filter(o => o.instrumentId === id),

  submitOrder: async (req) => {
    const order = await api.post<Order>("/api/v1/orders", req);
    set(state => {
      const next = new Map(state.orders);
      next.set(order.id, order);
      return { orders: next };
    });
    return order;
  },

  cancelOrder: async (orderId) => {
    await api.delete(`/api/v1/orders/${orderId}`);
  },

  applyUpdate: (update) => {
    set(state => {
      const next = new Map(state.orders);
      const existing = next.get(update.orderId);
      if (existing) {
        next.set(update.orderId, { ...existing, ...update });
      }
      return { orders: next };
    });
  },
}));
```

#### WebSocket Client

```typescript
// dashboard/src/api/ws.ts

import ReconnectingWebSocket from "reconnecting-websocket";

const BASE_WS = import.meta.env.VITE_WS_URL || "ws://localhost:8080";

export function createOrderStream(onUpdate: (update: OrderUpdate) => void): ReconnectingWebSocket {
  const ws = new ReconnectingWebSocket(`${BASE_WS}/ws/orders`);
  ws.onmessage = (event) => {
    const update: OrderUpdate = JSON.parse(event.data);
    onUpdate(update);
  };
  return ws;
}

// Initialize all streams in App.tsx
export function initializeStreams() {
  const orderStore = useOrderStore.getState();
  const positionStore = usePositionStore.getState();
  const riskStore = useRiskStore.getState();
  const venueStore = useVenueStore.getState();

  createOrderStream(orderStore.applyUpdate);
  createPositionStream(positionStore.applyUpdate);
  createRiskStream(riskStore.applyUpdate);
  createVenueStream(venueStore.applyUpdate);
}
```

#### View Descriptions

**Unified Blotter (BlotterView.tsx):**
- AG Grid with columns: Time, Instrument, Side (color-coded green/red), Type, Qty, Price, Filled, Avg Price, Status (color-coded badge), Venue (icon + label), Actions (cancel button for active orders)
- Streaming row updates via `applyTransaction()` API (AG Grid's efficient delta update)
- Filters: status (active/all/filled/canceled), asset class, venue, instrument search
- Click row to expand fill detail sub-grid
- Order ticket panel (slide-out): instrument picker, side toggle, order type, quantity, price, venue selector (or "Smart Route")

**Portfolio View (PortfolioView.tsx):**
- Position table: Instrument, Venue, Qty, Avg Cost, Market Price, Unrealized P&L (color), Realized P&L, % of NAV, Asset Class
- Summary cards at top: Total NAV, Day P&L, Unsettled Cash, Available Cash
- Exposure breakdown charts: by asset class (pie), by venue (bar), by sector (treemap)
- FX conversion: all P&L converted to user's base currency (configurable, default USD)

**Risk Dashboard (RiskDashboard.tsx):**
- VaR gauges: three cards showing Historical, Parametric, Monte Carlo VaR with color coding (green < 2% NAV, yellow 2-5%, red > 5%)
- Monte Carlo distribution histogram (Recharts) showing the full simulated P&L distribution with VaR line marked
- Greeks heatmap: instruments on Y-axis, Greek measures on X-axis, color intensity for magnitude
- Drawdown chart: time series of portfolio drawdown from peak, current drawdown highlighted
- Concentration risk: instrument treemap (D3) sized by exposure, colored by asset class
- Settlement risk timeline: horizontal bar chart showing unsettled amounts by settlement date

**Liquidity Network Panel (LiquidityNetwork.tsx):**
- Card grid for each venue: status indicator (green dot/red dot), name, type badge, latency metrics, fill rate, last heartbeat
- "Connect New Venue" card with plus icon → opens credential form modal
- Per-venue drill-down: order count, fill stats, historical latency chart
- "Test Connection" button per venue

**AI Insights Panel (InsightsPanel.tsx):**
- Tabbed view: Execution Analysis | Rebalancing | Anomaly Alerts
- Execution Analysis tab: rendered markdown reports from the AI execution analyst, sorted by recency
- Rebalancing tab: natural language input box → loading state → proposed trade list table with "Execute All" button
- Anomaly Alerts tab: timeline of anomaly detections with instrument, venue, score, and feature breakdown

**Onboarding View (OnboardingView.tsx):**
- Step 1: Welcome screen explaining self-hosted security model ("Your keys never leave this machine")
- Step 2: Set master passphrase (for credential encryption)
- Step 3: "Choose your path" — Connect Alpaca (equities), Connect Binance Testnet (crypto), or Start with Simulator
- Step 4: Enter API credentials → validate → show first market data flowing
- Step 5: Land on the blotter with a simulated or real order ready to submit

#### Theme

```typescript
// dashboard/src/theme/terminal.ts
export const terminalTheme = {
  colors: {
    bg: {
      primary: "#0a0e17",       // Near-black blue
      secondary: "#111827",     // Card backgrounds
      tertiary: "#1f2937",      // Input fields, hover states
    },
    text: {
      primary: "#e5e7eb",       // High contrast
      secondary: "#9ca3af",     // Labels, secondary info
      muted: "#6b7280",
    },
    accent: {
      blue: "#3b82f6",          // Primary actions
      green: "#22c55e",         // Buy, profit, connected
      red: "#ef4444",           // Sell, loss, disconnected
      yellow: "#eab308",        // Warning, pending
      purple: "#a855f7",        // AI features
    },
    border: "#374151",
  },
  fonts: {
    mono: "'JetBrains Mono', 'Fira Code', monospace",
    sans: "'Inter', system-ui, sans-serif",
  },
};
```

---

## 5. AI-Integrated Features

### 5.1 Smart Order Router ML Model

**User value statement:** "The system routed my ETH buy across Binance and a secondary venue, saving me 12bps vs filling on a single exchange."

**Architecture:**

```
Order arrives → Feature extraction → ML scorer → Venue ranking → Split decision → Route
```

**Feature engineering (per candidate venue, per order):**

| Feature | Source | Description |
|---------|--------|-------------|
| `order_size_pct_adv` | Order + market data | Order size as % of venue's average daily volume |
| `spread_bps` | Market data | Current bid-ask spread in basis points |
| `book_depth_at_price` | Market data | Available quantity within 5bps of midpoint |
| `venue_fill_rate_30d` | Historical fills | % of orders on this venue that were fully filled |
| `venue_latency_p50` | Venue metrics | Current P50 round-trip latency |
| `cross_venue_price_diff` | Market data | Price difference vs best alternative venue (bps) |
| `hour_of_day` | Clock | 0-23, cyclical encoding (sin/cos) |
| `instrument_volatility` | Risk engine | 30-day realized volatility |
| `maker_taker_fee` | Venue config | Fee rate for this order type |
| `time_since_last_fill` | Historical fills | Seconds since last fill on this venue for this instrument |

**Model:** XGBoost gradient-boosted tree (`xgboost==2.0`)
- Target: execution quality score = -1 * (slippage_bps + fee_bps + latency_penalty)
- Training data: historical fills from all connected venues (accumulates over time)
- Retraining: daily batch retrain if >100 new fills, model hot-swapped
- Cold start: rule-based strategy (best-price) until sufficient training data accumulated (~500 fills)

**Integration point:**

```go
// gateway/internal/router/ml_scorer.go

type MLScorer struct {
    modelPath string
    // XGBoost model loaded via cgo or called via local HTTP to Python sidecar
    // For simplicity: Python sidecar exposes POST /score endpoint
    client    *http.Client
    endpoint  string  // "http://localhost:8090/score"
}

func (s *MLScorer) ScoreVenues(order *Order, candidates []VenueCandidate) ([]VenueScore, error) {
    features := extractFeatures(order, candidates)
    resp, err := s.client.Post(s.endpoint, "application/json", features)
    // Returns scored venue list, router uses top venue(s)
    ...
}
```

**Order splitting logic:** If the order is large relative to best venue's depth (>50% of displayed quantity at price), the router splits across top-N scored venues proportionally to their available depth.

### 5.2 AI Execution Analyst

**User value statement:** "After trading, I get a plain-language report telling me whether my fills were good, which venue performed best, and what to adjust next time — something only institutional desks with TCA teams get today."

**Architecture:**

```
Trade completes → Collect fill data + market context → Build structured prompt → Anthropic API → Parse response → Store + display
```

**Prompt template:**

```python
# ai/execution_analyst/prompt_templates.py

EXECUTION_ANALYSIS_PROMPT = """You are an institutional-grade trade execution analyst.
Analyze the following completed trade and provide a concise execution quality report.

## Trade Summary
- Order: {side} {quantity} {instrument_id} ({asset_class})
- Order type: {order_type}, limit price: {limit_price}
- Submitted: {submitted_at}, completed: {completed_at}
- Venue(s) used: {venues}
- Total fills: {fill_count}

## Fill Details
{fill_table}

## Market Context at Submission
- Arrival price (mid at submission): {arrival_price}
- Spread at submission: {spread_bps} bps
- 5-minute VWAP around execution: {vwap_5min}
- 30-day average daily volume: {adv_30d}
- Order size as % of ADV: {size_pct_adv}%

## Venue Comparison
{venue_comparison_table}

## Instructions
Provide your analysis in the following JSON structure:
{{
  "overall_grade": "A/B/C/D/F",
  "implementation_shortfall_bps": <number>,
  "summary": "<2-3 sentence executive summary>",
  "venue_analysis": [
    {{"venue": "<name>", "grade": "<A-F>", "comment": "<1 sentence>"}}
  ],
  "recommendations": ["<actionable suggestion 1>", "..."],
  "market_impact_estimate_bps": <number>
}}
"""
```

**API call:**

```python
# ai/execution_analyst/analyst.py

import anthropic

class ExecutionAnalyst:
    def __init__(self):
        self.client = anthropic.Anthropic()  # Uses ANTHROPIC_API_KEY env var
        self.model = "claude-sonnet-4-6"     # Fast + capable for structured analysis

    async def analyze_execution(self, trade_context: TradeContext) -> ExecutionReport:
        prompt = EXECUTION_ANALYSIS_PROMPT.format(**trade_context.to_prompt_vars())

        response = self.client.messages.create(
            model=self.model,
            max_tokens=1024,
            messages=[{"role": "user", "content": prompt}],
        )

        report_json = json.loads(response.content[0].text)
        return ExecutionReport(**report_json)
```

**Trigger:** Automatically runs when an order reaches terminal state (Filled, Canceled with partial fills). Rate-limited to avoid excessive API calls (max 10 analyses per hour).

### 5.3 Portfolio Rebalancing Assistant

**User value statement:** "I describe what I want in English and get an executable trade list that accounts for positions across all my exchanges."

**Architecture:**

```
User NL input → Anthropic API (constraint extraction) → Validate constraints →
  → Portfolio Optimizer (cvxpy) → Trade list → User approval → Execute across venues
```

**NL → Constraints prompt:**

```python
# ai/rebalancing_assistant/prompt_templates.py

CONSTRAINT_EXTRACTION_PROMPT = """You are a portfolio construction assistant.
The user will describe a rebalancing goal in natural language. Extract structured
optimization constraints from their description.

Current portfolio:
{portfolio_summary}

Available instruments:
{available_instruments}

User request: "{user_input}"

Extract constraints as JSON:
{{
  "objective": "maximize_sharpe" | "minimize_variance" | "target_return",
  "target_return": <float or null>,
  "risk_aversion": <float, default 1.0>,
  "long_only": <bool>,
  "max_single_weight": <float or null>,
  "asset_class_bounds": {{"equity": [<min>, <max>], "crypto": [<min>, <max>]}} or null,
  "sector_limits": {{}} or null,
  "target_volatility": <float or null>,
  "max_turnover_usd": <float or null>,
  "instruments_to_include": [<list>] or null,
  "instruments_to_exclude": [<list>] or null,
  "reasoning": "<1-2 sentences explaining interpretation>"
}}

If the user's request is ambiguous, set conservative defaults and explain in reasoning.
"""
```

**Flow:**

1. User types: "Reduce crypto to 30% of portfolio, maximize Sharpe, keep turnover under $5K"
2. Frontend sends to Risk Engine REST: `POST /api/v1/ai/rebalance` with `{ "prompt": "..." }`
3. Risk Engine calls Anthropic API with current portfolio context + user prompt
4. LLM returns structured constraints JSON
5. Constraints validated (bounds feasible, instruments exist)
6. Portfolio optimizer runs with extracted constraints
7. Optimizer returns target weights + trade list
8. Response to frontend: proposed trades with before/after comparison
9. User reviews → clicks "Execute All" → orders submitted via Gateway REST API

### 5.4 Market Data Anomaly Detection

**User value statement:** "I got an alert that ETH volume on Binance spiked 4x normal while my equity positions were asleep — the system watches everything 24/7 even when I don't."

**Architecture:**

The `StreamingAnomalyDetector` (described in 4B) runs as a background task in the Risk Engine, consuming from the `market-data` Kafka topic. Alerts are:
1. Published to Kafka topic `anomaly-alerts`
2. Stored in PostgreSQL (anomaly_alerts table)
3. Streamed to frontend via WebSocket (`/ws/anomalies`)
4. Displayed in the AI Insights Panel anomaly tab

**Alert structure:**

```python
@dataclass
class AnomalyAlert:
    id: str
    instrument_id: str
    venue_id: str
    anomaly_score: float          # Isolation forest decision function score
    severity: str                 # "info", "warning", "critical"
    features: dict[str, float]    # Which features triggered (e.g., volume_zscore: 4.2)
    description: str              # Human-readable: "ETH-USD volume on Binance 4.2x above 30-day mean"
    timestamp: datetime
    acknowledged: bool = False
```

**Severity thresholds:**
- `info`: anomaly score < -0.3 (unusual but not alarming)
- `warning`: anomaly score < -0.5 (significant deviation)
- `critical`: anomaly score < -0.7 AND user has an open position in the affected instrument

---

## 6. Digital Asset & Tokenization Layer

### 6.1 Settlement Semantics

| Asset Class | Settlement | Impact |
|-------------|-----------|--------|
| Crypto | T+0 (instant) | Cash and position immediately available |
| US Equities | T+2 (2 business days) | Cash from sells unavailable for 2 days; bought shares in portfolio but cash committed |
| Tokenized Securities | T+0 (on-chain) | Same as crypto — atomic settlement |

**Implementation:** The `SettlementTracker` in the Risk Engine maintains a table of `PendingSettlement` records. On each fill event:
1. Create a `PendingSettlement` with `settlement_date = trade_date + settlement_cycle`
2. Reduce `available_cash` by committed amount
3. Background task marks settlements as complete when `settlement_date` arrives
4. Portfolio's `available_cash` vs `total_cash` distinction flows to the UI

### 6.2 Trading Hours Handling

```python
# risk_engine/domain/instrument.py

class TradingCalendar:
    """
    Determines if a market is currently open and when the next session starts/ends.
    Used by risk engine for:
    - Intraday vs overnight VaR calculations
    - Alert suppression (don't alert on equity price staleness at midnight)
    - Return computation (skip weekends for equities, continuous for crypto)
    """
    def is_market_open(self, instrument: Instrument, now: datetime) -> bool:
        if instrument.trading_hours.is_24x7:
            return True
        # Check against market hours, pre-market, after-hours, holidays
        ...

    def align_returns(
        self,
        equity_returns: pd.Series,
        crypto_returns: pd.Series,
    ) -> tuple[pd.Series, pd.Series]:
        """
        Align equity and crypto return series for covariance computation.
        - Crypto: daily returns exist for every calendar day
        - Equities: returns only on trading days
        Strategy: forward-fill equity returns on weekends/holidays
        (assumes no price change when market closed — standard practice)
        """
        ...
```

### 6.3 Fee Models

```python
# risk_engine/domain/instrument.py

@dataclass
class FeeSchedule:
    model: str  # "maker_taker", "commission", "flat", "tiered"
    # Maker/taker (crypto)
    maker_fee_bps: Decimal | None = None   # e.g., 10 = 0.10%
    taker_fee_bps: Decimal | None = None   # e.g., 10 = 0.10%
    # Commission (equities)
    per_share_commission: Decimal | None = None  # e.g., 0.005
    min_commission: Decimal | None = None
    # Regulatory fees (equities)
    sec_fee_per_dollar: Decimal | None = None
    finra_taf_per_share: Decimal | None = None

    def compute_fee(self, quantity: Decimal, price: Decimal, side: str, liquidity: str) -> Decimal:
        """Returns total fee for a fill."""
        ...
```

### 6.4 Tokenized Securities Adapter

Forward-looking adapter for when tokenized asset venues become available:

```go
// gateway/internal/adapter/tokenized/adapter.go

// TokenizedAdapter implements LiquidityProvider for tokenized security venues.
// Currently simulated — designed so the interface is correct for real tokenized
// asset venues (e.g., Securitize Markets, tZERO, Backed Finance).
//
// Key differences from traditional adapters:
// - Wallet-based identification (address instead of account ID)
// - Token-based positions (ERC-20-like balance queries)
// - Atomic T+0 settlement (no pending settlement window)
// - On-chain transaction confirmation (simulated as instant for now)

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

---

## 7. Infrastructure & Observability

### 7.1 Docker Compose Topology

```yaml
# deploy/docker-compose.yml

services:
  gateway:
    build: ../gateway
    ports:
      - "8080:8080"     # REST + WebSocket
    environment:
      - KAFKA_BROKERS=kafka:9092
      - POSTGRES_URL=postgres://synapse:synapse@postgres:5432/synapse
      - REDIS_URL=redis://redis:6379
      - RISK_ENGINE_GRPC=risk-engine:50051
      - SYNAPSE_MASTER_PASSPHRASE=${SYNAPSE_MASTER_PASSPHRASE}
    depends_on:
      kafka:
        condition: service_healthy
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v1/health"]
      interval: 10s
      timeout: 5s
      retries: 3

  risk-engine:
    build: ../risk-engine
    ports:
      - "8081:8081"     # REST
      - "50051:50051"   # gRPC
    environment:
      - KAFKA_BROKERS=kafka:9092
      - POSTGRES_URL=postgresql://synapse:synapse@postgres:5432/synapse
      - REDIS_URL=redis://redis:6379
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    depends_on:
      kafka:
        condition: service_healthy
      postgres:
        condition: service_healthy

  dashboard:
    build: ../dashboard
    ports:
      - "3000:3000"
    environment:
      - VITE_API_URL=http://localhost:8080
      - VITE_RISK_API_URL=http://localhost:8081
      - VITE_WS_URL=ws://localhost:8080

  ml-scorer:
    build: ../ai
    ports:
      - "8090:8090"     # ML scoring sidecar
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}

  kafka:
    image: apache/kafka:3.7.0
    ports:
      - "9092:9092"
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_LISTENERS: PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka:9093
      KAFKA_LOG_DIRS: /tmp/kraft-combined-logs
      CLUSTER_ID: synapse-oms-local-001
    healthcheck:
      test: ["CMD", "kafka-topics.sh", "--bootstrap-server", "localhost:9092", "--list"]
      interval: 10s
      timeout: 10s
      retries: 5

  postgres:
    image: postgres:16-alpine
    ports:
      - "5432:5432"
    environment:
      POSTGRES_USER: synapse
      POSTGRES_PASSWORD: synapse
      POSTGRES_DB: synapse
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U synapse"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5

  prometheus:
    image: prom/prometheus:v2.53.0
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    profiles:
      - monitoring

  grafana:
    image: grafana/grafana:11.1.0
    ports:
      - "3001:3000"
    volumes:
      - ./grafana/:/etc/grafana/provisioning/dashboards/
    environment:
      GF_SECURITY_ADMIN_PASSWORD: synapse
    profiles:
      - monitoring

volumes:
  pgdata:
```

### 7.2 Prometheus Metrics

| Service | Metric | Type | Description |
|---------|--------|------|-------------|
| Gateway | `gateway_orders_submitted_total` | Counter | Total orders submitted, labeled by asset_class, venue |
| Gateway | `gateway_order_latency_seconds` | Histogram | End-to-end order processing time (submit → venue ack) |
| Gateway | `gateway_fills_received_total` | Counter | Total fills, labeled by venue, liquidity_type |
| Gateway | `gateway_venue_latency_seconds` | Histogram | Per-venue round-trip latency (p50, p95, p99) |
| Gateway | `gateway_venue_status` | Gauge | 1=connected, 0=disconnected per venue |
| Gateway | `gateway_active_websocket_connections` | Gauge | Current WebSocket client count |
| Risk Engine | `risk_var_computation_seconds` | Histogram | VaR computation time by method |
| Risk Engine | `risk_pretrade_check_seconds` | Histogram | gRPC pre-trade check latency |
| Risk Engine | `risk_pretrade_check_rejected_total` | Counter | Pre-trade rejections |
| Risk Engine | `risk_portfolio_var_ratio` | Gauge | Current VaR as % of NAV |
| Risk Engine | `risk_anomalies_detected_total` | Counter | Anomaly alerts fired |
| Kafka | `kafka_consumer_lag` | Gauge | Consumer lag per topic partition |

### 7.3 Structured Logging

All services emit JSON-structured logs with correlation IDs:

```json
{
  "timestamp": "2026-04-01T14:30:22.451Z",
  "level": "info",
  "service": "gateway",
  "component": "order_pipeline",
  "correlation_id": "ord-a1b2c3d4",
  "order_id": "ord-a1b2c3d4",
  "instrument_id": "ETH-USD",
  "venue_id": "binance_testnet",
  "message": "Order routed to venue",
  "latency_ms": 2.3
}
```

**Correlation ID propagation:**
- Generated at order submission (REST handler)
- Passed through pipeline stages via context
- Included in Kafka message headers
- Risk Engine extracts from gRPC metadata
- Frontend receives in WebSocket messages

### 7.4 Startup Health Checks

```go
// gateway/cmd/gateway/main.go — startup sequence

func startupHealthChecks(ctx context.Context) error {
    checks := []struct {
        name string
        fn   func(context.Context) error
    }{
        {"PostgreSQL", checkPostgres},
        {"Redis", checkRedis},
        {"Kafka", checkKafka},
        {"Risk Engine gRPC", checkRiskEngine},
    }

    for _, c := range checks {
        if err := c.fn(ctx); err != nil {
            slog.Error("startup check failed", "check", c.name, "error", err)
            return fmt.Errorf("startup check %s failed: %w", c.name, err)
        }
        slog.Info("startup check passed", "check", c.name)
    }

    // Validate stored venue credentials (without logging secrets)
    credResults := credentialManager.ValidateAll(ctx)
    for venueID, err := range credResults {
        if err != nil {
            slog.Warn("venue credential validation failed",
                "venue_id", venueID,
                "error", err.Error(),
                // NEVER log the credential itself
            )
        }
    }

    return nil
}
```

### 7.5 Load Testing

```javascript
// loadtest/k6/order_flow.js

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

  check(res, {
    "status 201": (r) => r.status === 201,
  });
}
```

**Performance targets:**

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

## 8. Development Roadmap

### Phase 1 — Single-Venue Trading Loop (Weeks 1-3)

**Goal:** A user can submit an order via the UI, see it route to the simulated venue, receive a fill, and see their position update.

**Deliverables:**
1. Proto definitions for Order, Fill, Instrument, Position
2. Gateway: domain model (Order state machine, Fill, Position, Instrument)
3. Gateway: Simulated exchange adapter with matching engine + price walk
4. Gateway: Order processing pipeline (intake → route → fill) — no risk check yet
5. Gateway: REST API (submit order, cancel, list orders, list positions)
6. Gateway: WebSocket server (order updates, position updates)
7. Gateway: PostgreSQL schema + migrations (orders, fills, positions, instruments)
8. Dashboard: Project scaffolding (Vite + React + TypeScript + Zustand + Tailwind)
9. Dashboard: Terminal layout shell with dark theme
10. Dashboard: Order ticket component (instrument picker, side, type, qty, price)
11. Dashboard: Blotter view with AG Grid (streaming order updates via WebSocket)
12. Dashboard: Basic position table
13. Docker Compose with Gateway + Dashboard + PostgreSQL + Redis
14. Seed script: load simulated instruments (AAPL, MSFT, GOOG, BTC-USD, ETH-USD, SOL-USD)

**Acceptance test:** User opens dashboard at localhost:3000, submits a market buy for 10 shares of AAPL on the simulated exchange, sees the order appear in the blotter as "New" → "Acknowledged" → "Filled", and sees a position of 10 AAPL appear in the position table.

### Phase 2 — Real Venue Connectivity + Risk (Weeks 4-7)

**Goal:** A user connects Alpaca and Binance paper accounts, sees unified positions and a combined VaR number across both asset classes.

**Deliverables:**
1. Gateway: LiquidityProvider interface + adapter registry
2. Gateway: Alpaca adapter (REST + WebSocket market data, paper trading)
3. Gateway: Binance testnet adapter (REST + WebSocket, testnet execution)
4. Gateway: Venue credential manager (AES-256-GCM encryption, Argon2id key derivation)
5. Gateway: Kafka producer (order-lifecycle events)
6. Risk Engine: Project scaffolding (FastAPI + grpcio + confluent-kafka)
7. Risk Engine: Kafka consumer → portfolio state builder (from events)
8. Risk Engine: Historical VaR (cross-asset, mixed calendar)
9. Risk Engine: Parametric VaR (cross-asset covariance)
10. Risk Engine: gRPC server for pre-trade risk checks
11. Risk Engine: REST API for risk metrics
12. Gateway: gRPC risk check integration in pipeline (before routing)
13. Dashboard: Onboarding flow (set passphrase, connect venue, enter credentials)
14. Dashboard: Portfolio view with real-time P&L, exposure breakdown charts
15. Dashboard: Risk dashboard with VaR gauges and drawdown chart
16. Dashboard: Venue connection panel (status cards, connect/disconnect)
17. Docker Compose: add Kafka (KRaft), Risk Engine
18. Settlement tracker: T+0 vs T+2 handling in positions and available cash

**Acceptance test:** User runs `docker compose up`, completes onboarding, connects Alpaca (paper) and Binance (testnet), sees positions from both in a unified portfolio view, and sees a combined VaR metric that accounts for cross-asset correlation.

### Phase 3 — Smart Routing + Portfolio Optimization (Weeks 8-11)

**Goal:** An order is routed to the best-priced venue automatically; the optimizer produces a rebalancing trade list respecting user constraints.

**Deliverables:**
1. Gateway: Smart Order Router with rule-based strategies (best-price, venue preference)
2. Gateway: Cross-venue price comparison logic
3. Gateway: Dark pool / internal crossing engine
4. Gateway: Order splitting across venues
5. AI: ML scorer sidecar (XGBoost model serving via FastAPI)
6. AI: Feature engineering pipeline for venue scoring
7. AI: Training script using historical fill data
8. Risk Engine: Monte Carlo VaR (correlated paths, fat-tailed distributions)
9. Risk Engine: Portfolio construction optimizer (cvxpy, constraint support)
10. Risk Engine: Concentration risk analyzer
11. Risk Engine: Portfolio Greeks calculator
12. Dashboard: Liquidity Network panel (venue cards with latency, fill rate)
13. Dashboard: Order ticket enhancement: "Smart Route" option
14. Dashboard: Optimizer UI (constraint inputs → trade list → execute)
15. Dashboard: Greeks heatmap, concentration treemap, MC distribution plot

**Acceptance test:** User submits a large ETH-USD order → router splits it across Binance testnet and simulated venue based on price/depth → fills aggregate correctly. User runs optimizer with "max 30% crypto, minimize variance" → gets a trade list → executes with one click.

### Phase 4 — AI Features (Weeks 12-15)

**Goal:** User gets post-trade analysis in plain English; user describes a rebalancing goal in natural language and gets an executable trade list.

**Deliverables:**
1. AI: Execution analyst (Anthropic API integration, structured prompt, response parsing)
2. AI: Rebalancing assistant (NL → optimizer constraints via Anthropic API)
3. AI: Anomaly detection (Isolation Forest on streaming market data features)
4. Risk Engine: Anomaly detection integration (Kafka consumer, alert pipeline)
5. Dashboard: AI Insights panel (execution reports, rebalancing chat, anomaly timeline)
6. Dashboard: NL rebalancing input → loading → trade list → execute flow
7. Dashboard: Anomaly alert badges and notification system
8. Gateway: Anomaly alerts via WebSocket
9. Kafka topic: `anomaly-alerts`

**Acceptance test:** After a trade, user sees an AI-generated execution quality report with a letter grade, implementation shortfall, and recommendations. User types "reduce crypto to 30%, maximize Sharpe, keep turnover under $5K" → gets a proposed trade list → executes it. User sees an anomaly alert when the system detects unusual volume on a monitored instrument.

### Phase 5 — Production Hardening (Weeks 16-20)

**Goal:** `docker compose up` brings up the full system; a new user can go from zero to connected-and-trading in under 5 minutes; load test report shows sustained throughput targets.

**Deliverables:**
1. Tokenized securities adapter (simulated, correct interfaces)
2. Prometheus metrics on all services
3. Grafana dashboards (system overview, per-venue performance, risk engine health)
4. Load testing harness (k6 scripts, published benchmark reports)
5. Onboarding flow polish (error handling, progress indicators, security messaging)
6. Credential encryption hardening (key rotation, secure memory handling)
7. Comprehensive error handling and graceful degradation across all services
8. Docker Compose finalization (single-command startup, health checks, restart policies)
9. Kubernetes manifests (deployments, services, configmaps, persistent volumes)
10. README (product-first, problem statement, quickstart)
11. Quickstart guide (git clone → running in 3 minutes)
12. "Connect Your First Exchange" guide
13. "Write a Venue Adapter" contributor guide
14. Architecture overview for contributors
15. LICENSE file (AGPLv3) and CONTRIBUTING.md

**Acceptance test:** Fresh user clones repo, runs `docker compose up`, opens browser, completes onboarding with the simulated exchange, submits an order, sees it fill, checks risk dashboard — all within 5 minutes. Load test report shows Gateway sustaining 5,000 orders/sec with p99 < 50ms.

---

## 9. Testing Strategy

### 9.1 Unit Tests

**Go (Gateway):** Table-driven tests using `testing` stdlib. Each domain function and pipeline stage tested in isolation.

```go
// gateway/internal/domain/order_test.go
func TestOrderStateTransitions(t *testing.T) {
    tests := []struct {
        name      string
        from      OrderStatus
        to        OrderStatus
        wantError bool
    }{
        {"new to acknowledged", OrderStatusNew, OrderStatusAcknowledged, false},
        {"new to filled", OrderStatusNew, OrderStatusFilled, true},  // Invalid
        {"partially filled to filled", OrderStatusPartiallyFilled, OrderStatusFilled, false},
        {"filled to canceled", OrderStatusFilled, OrderStatusCanceled, true},  // Terminal
        // ...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            order := &Order{Status: tt.from}
            err := order.ApplyTransition(tt.to)
            if (err != nil) != tt.wantError {
                t.Errorf("ApplyTransition(%v → %v) error = %v, wantError %v",
                    tt.from, tt.to, err, tt.wantError)
            }
        })
    }
}
```

**Python (Risk Engine):** pytest with fixtures for portfolio state, return matrices, covariance matrices.

```python
# risk_engine/tests/test_var_historical.py
import pytest
import numpy as np
from risk_engine.var.historical import HistoricalVaR

@pytest.fixture
def mixed_portfolio():
    """Portfolio with equity and crypto positions for cross-asset VaR testing."""
    ...

def test_historical_var_basic(mixed_portfolio):
    var = HistoricalVaR(window_days=252, confidence=0.99)
    result = var.compute(mixed_portfolio.positions, mixed_portfolio.returns)
    assert result.var_amount > 0
    assert result.confidence == 0.99
    assert result.cvar >= result.var_amount  # CVaR >= VaR by definition

def test_historical_var_crypto_only_higher_than_equity_only():
    """Crypto-only portfolio should have higher VaR than equity-only at same notional."""
    ...
```

**TypeScript (Dashboard):** React Testing Library + Vitest for component tests.

```typescript
// dashboard/src/components/OrderTicket.test.tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { OrderTicket } from "./OrderTicket";

test("submits market order with correct parameters", async () => {
  const onSubmit = vi.fn();
  render(<OrderTicket onSubmit={onSubmit} instruments={mockInstruments} />);

  fireEvent.click(screen.getByText("Buy"));
  fireEvent.change(screen.getByLabelText("Quantity"), { target: { value: "10" } });
  fireEvent.click(screen.getByText("Submit Order"));

  expect(onSubmit).toHaveBeenCalledWith(
    expect.objectContaining({ side: "buy", quantity: "10", type: "market" })
  );
});
```

### 9.2 Integration Tests

Cross-service integration tests using Docker Compose test topology with the simulated exchange:

**Critical flow: Order → Risk Check → Route → Fill → Position Update**

```go
// gateway/integration_test.go (build tag: //go:build integration)

func TestOrderToPositionFlow(t *testing.T) {
    // 1. Submit order via REST
    resp := httpPost("/api/v1/orders", OrderRequest{
        InstrumentID: "AAPL",
        Side: "buy",
        Type: "market",
        Quantity: "100",
    })
    assert.Equal(t, 201, resp.StatusCode)
    var order Order
    json.Decode(resp.Body, &order)

    // 2. Wait for fill via WebSocket (max 5s)
    fill := waitForWS[Fill]("/ws/orders", order.ID, 5*time.Second)
    assert.Equal(t, "filled", fill.Status)

    // 3. Verify position updated
    posResp := httpGet("/api/v1/positions/AAPL")
    var pos Position
    json.Decode(posResp.Body, &pos)
    assert.Equal(t, "100", pos.Quantity)

    // 4. Verify risk engine received the event (query VaR)
    riskResp := httpGet("http://risk-engine:8081/api/v1/risk/var")
    var risk VaRMetrics
    json.Decode(riskResp.Body, &risk)
    assert.NotZero(t, risk.HistoricalVaR)
}
```

### 9.3 End-to-End Tests

Using Playwright for browser-driven E2E tests of critical user journeys:

1. **Connect venue → Submit order → See fill → Check risk:** Browser opens dashboard, completes onboarding with simulated venue, submits a buy order, waits for fill in blotter, navigates to risk dashboard, verifies VaR is displayed.

2. **Multi-venue portfolio:** Connect simulated equities + simulated crypto, submit orders in both, verify unified portfolio view shows both positions with correct asset class labels.

3. **Order cancellation:** Submit a limit order far from market, verify it appears as "Acknowledged", click cancel, verify it moves to "Canceled".

### 9.4 Venue Adapter Contract Tests

A shared test suite that any `LiquidityProvider` implementation must pass:

```go
// gateway/internal/adapter/contract_test.go

// AdapterContractSuite runs the standard contract tests against any LiquidityProvider.
// Community contributors must pass this suite for their adapter to be accepted.
func AdapterContractSuite(t *testing.T, factory func() LiquidityProvider) {
    t.Run("Connect and Disconnect", func(t *testing.T) {
        adapter := factory()
        err := adapter.Connect(ctx, testCredential)
        assert.NoError(t, err)
        assert.Equal(t, VenueStatusConnected, adapter.Status())
        err = adapter.Disconnect(ctx)
        assert.NoError(t, err)
    })

    t.Run("Submit and receive fill for market order", func(t *testing.T) {
        adapter := factory()
        adapter.Connect(ctx, testCredential)
        defer adapter.Disconnect(ctx)

        instruments, _ := adapter.SupportedInstruments()
        assert.NotEmpty(t, instruments)

        order := &Order{
            InstrumentID: instruments[0].ID,
            Side: SideBuy,
            Type: OrderTypeMarket,
            Quantity: decimal.NewFromFloat(1.0),
        }
        ack, err := adapter.SubmitOrder(ctx, order)
        assert.NoError(t, err)
        assert.NotEmpty(t, ack.VenueOrderID)

        // Must receive a fill within 30 seconds
        select {
        case fill := <-adapter.FillFeed():
            assert.Equal(t, order.ID, fill.OrderID)
            assert.True(t, fill.Quantity.GreaterThan(decimal.Zero))
        case <-time.After(30 * time.Second):
            t.Fatal("timeout waiting for fill")
        }
    })

    t.Run("Cancel order", func(t *testing.T) { ... })
    t.Run("Market data subscription", func(t *testing.T) { ... })
    t.Run("Ping returns latency", func(t *testing.T) { ... })
    t.Run("Handles invalid credentials gracefully", func(t *testing.T) { ... })
}
```

### 9.5 Performance Benchmarks

Run as part of CI (gated, not blocking) and published as release artifacts:

```go
// gateway/internal/pipeline/pipeline_bench_test.go

func BenchmarkOrderPipeline(b *testing.B) {
    pipeline := setupBenchPipeline() // Simulated venue, no risk check
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            pipeline.Submit(ctx, randomOrder())
        }
    })
    // Report: ns/op, orders/sec
}
```

---

## 10. Documentation & Open-Source Strategy

### 10.1 README Structure

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

### 10.2 Quickstart Guide (docs/quickstart.md)

Step-by-step from `git clone` to a running system with the simulated venue:
1. Clone the repo
2. Copy `.env.example` to `.env`, set master passphrase
3. `docker compose up`
4. Open `http://localhost:3000`
5. Complete onboarding with simulated exchange
6. Submit your first order

### 10.3 "Connect Your First Exchange" Guide (docs/connect-venue.md)

- Alpaca: sign up for paper trading, get API key/secret, enter in SynapseOMS
- Binance Testnet: get testnet credentials, enter in SynapseOMS
- Security explanation: how credentials are encrypted, where they're stored

### 10.4 "Write a Venue Adapter" Contributor Guide (docs/write-adapter.md)

The primary open-source contribution path:
1. Implement the `LiquidityProvider` interface (Go)
2. Register in `adapter/registry.go`
3. Pass the contract test suite
4. Add venue-specific configuration
5. Submit PR with adapter + tests + docs

```markdown
## Implementing LiquidityProvider

Your adapter must implement all methods of the `LiquidityProvider` interface.
Here's a minimal skeleton:

    type MyExchangeAdapter struct { ... }
    func (a *MyExchangeAdapter) VenueID() string { return "my_exchange" }
    func (a *MyExchangeAdapter) Connect(ctx context.Context, cred VenueCredential) error { ... }
    // ... implement all methods

## Running Contract Tests

    go test ./internal/adapter/ -run TestContractSuite -adapter=my_exchange

All tests must pass before your PR will be reviewed.
```

### 10.5 License Rationale

**Choice: AGPLv3**

- **Why AGPLv3 over MIT/Apache:** Prevents cloud providers from offering a proprietary hosted version of SynapseOMS without contributing back. The target users (individual traders, small funds) can use it freely. Companies that want to modify and deploy it as a service must open-source their changes.
- **Why AGPLv3 over BSL:** BSL (Business Source License) restricts commercial use for a time period, which would limit small fund usage — directly counter to the product thesis. AGPLv3 allows all use, just requires open-sourcing modifications to deployed services.
- **Tradeoff:** Some enterprise contributors may be discouraged by AGPLv3. Acceptable — the project's growth comes from individual traders and the adapter contribution ecosystem, not enterprise adoption.

---

## Appendix A: Sequence Diagrams

### A.1 Order Submission Flow

```mermaid
sequenceDiagram
    participant U as User (Dashboard)
    participant GW as Gateway REST
    participant PL as Pipeline
    participant RE as Risk Engine (gRPC)
    participant RT as Router
    participant VA as Venue Adapter
    participant K as Kafka
    participant WS as WebSocket

    U->>GW: POST /api/v1/orders
    GW->>PL: Submit to intake channel
    GW-->>U: 201 Created (order with status=New)

    PL->>RE: CheckPreTradeRisk (gRPC)
    RE-->>PL: Approved (VaR impact: +0.3%)

    PL->>RT: Route order
    RT->>RT: Score venues (ML or rule-based)
    RT->>VA: SubmitOrder to best venue

    VA-->>PL: VenueAck (venue order ID)
    PL->>K: Publish OrderAcknowledged
    PL->>WS: Broadcast status=Acknowledged

    VA-->>PL: Fill received
    PL->>PL: Update order state
    PL->>K: Publish FillReceived
    PL->>WS: Broadcast fill + status=Filled

    Note over K,RE: Risk Engine consumes from Kafka,<br/>updates portfolio state
```

### A.2 Pre-Trade Risk Check

```mermaid
sequenceDiagram
    participant PL as Pipeline
    participant RE as Risk Engine
    participant PS as Portfolio State

    PL->>RE: CheckPreTradeRisk(order)
    RE->>PS: Get current portfolio
    RE->>RE: Compute current VaR
    RE->>RE: Project VaR with hypothetical order
    RE->>RE: Check concentration limits
    RE->>RE: Check drawdown circuit breaker
    RE-->>PL: RiskCheckResult(approved=true, checks=[...])
```

### A.3 AI Rebalancing Flow

```mermaid
sequenceDiagram
    participant U as User
    participant DB as Dashboard
    participant RE as Risk Engine REST
    participant AI as Anthropic API
    participant OPT as Optimizer

    U->>DB: "Reduce crypto to 30%, max Sharpe, turnover < $5K"
    DB->>RE: POST /api/v1/ai/rebalance
    RE->>AI: Constraint extraction prompt + portfolio context
    AI-->>RE: Structured constraints JSON
    RE->>RE: Validate constraints (feasibility)
    RE->>OPT: Run optimization with constraints
    OPT-->>RE: Target weights + trade list
    RE-->>DB: Proposed trades + before/after comparison
    DB-->>U: Trade list table with "Execute All"
    U->>DB: Click "Execute All"
    DB->>DB: Submit each trade via Gateway REST
```

### A.4 Venue Connection Flow

```mermaid
sequenceDiagram
    participant U as User
    participant DB as Dashboard
    participant GW as Gateway REST
    participant CM as Credential Manager
    participant VA as Venue Adapter

    U->>DB: Enter API key + secret for Alpaca
    DB->>GW: POST /api/v1/credentials {venueId, apiKey, apiSecret}
    GW->>CM: Encrypt with master passphrase
    CM->>CM: AES-256-GCM encrypt, store in PostgreSQL
    GW-->>DB: 201 Created

    DB->>GW: POST /api/v1/venues/alpaca/connect
    GW->>CM: Retrieve + decrypt credentials
    GW->>VA: Connect(credential)
    VA->>VA: Authenticate with Alpaca API
    VA-->>GW: Connected
    GW->>GW: Start market data subscription
    GW-->>DB: {status: "connected"}
    GW->>WS: Broadcast venue status change
```

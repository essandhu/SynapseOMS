# SynapseOMS — Project Planning Prompt for Claude Code

## Your Role

You are a senior systems architect planning **SynapseOMS** — a self-hosted, open-source personal trading terminal that unifies execution and risk management across traditional and digital asset markets, with AI-powered portfolio intelligence that makes institutional-grade analysis accessible to individual traders, small funds, and RIAs.

## The Problem This Solves

There is a genuine, unserved gap in the market. Traders who operate across both traditional equities and crypto/digital assets have no affordable, unified tool:

- **Traditional brokers** (Interactive Brokers, Schwab, Fidelity) support equities/options/futures but either ignore crypto or treat it as an afterthought. Risk tools are basic and uncustomizable.
- **Crypto-native platforms** (Binance, Coinbase, Kraken) know nothing about the user's equity portfolio. No unified risk view, no cross-asset correlation analysis, no single blotter.
- **Portfolio trackers** (CoinGecko, Delta, Kubera) are passive read-only dashboards. No execution, no risk computation, no decision support.
- **Institutional solutions** (Bloomberg Terminal, Talos, FlexTrade) solve all of this and cost $24,000–$100,000+/year, designed for 50-person desks.

**SynapseOMS fills this gap.** The target users are algorithmic traders running personal strategies across both asset classes, small crypto-native funds (1–5 people) that also hold traditional positions, RIAs whose clients hold both equities and digital assets, and quantitative researchers who want to go from backtest to live execution in one system.

The "why does this exist" answer is one sentence: *There is no affordable tool that lets a trader see unified risk, execute across both traditional and crypto markets, and get AI-driven portfolio analysis from a single interface — SynapseOMS is that tool, and it's open-source and self-hosted so your keys, data, and strategies never leave your machine.*

**Every architectural and feature decision in this document must trace back to this product thesis.** If a component doesn't serve a real user need, it doesn't belong.

---

## What I Need From You

Produce a comprehensive **project plan and architecture document** structured as follows. Think step by step through each section. Every decision must be grounded in the product thesis above AND the domain requirements below.

### Deliverables

1. **Project Directory Structure** — Full monorepo layout showing all services, shared libraries, infrastructure configs, and documentation locations. Use clear bounded-context naming (no `utils/`, `helpers/`, `common/` dumping grounds).

2. **Architecture Decision Records (ADRs)** — For each of the following key decisions, provide a short ADR (context, decision, consequences). Each ADR must include a "Product Justification" field explaining why this choice serves the target user:
   - Go for the Order Gateway & Execution Engine (why Go over Java/C++ for this use case)
   - Python for the Risk & Analytics Engine
   - TypeScript + React for the frontend dashboard
   - Apache Kafka as the event backbone between services
   - WebSocket for real-time client streaming, REST for command operations, gRPC for internal synchronous service calls (risk pre-checks)
   - Protocol Buffers for internal message schemas
   - PostgreSQL for persistent state, Redis for hot caches and order book state
   - Docker Compose for local dev, Kubernetes manifests for production topology
   - Self-hosted architecture (why not SaaS — security of API keys, trading strategies, and position data)

3. **Service Contracts & Domain Model** — Define the core domain entities and their relationships:
   - Order (with full lifecycle states: New → Acknowledged → PartiallyFilled → Filled → Canceled → Rejected)
   - Execution / Fill
   - Position
   - Portfolio
   - Instrument (supporting both traditional equities/futures AND crypto/digital assets, with different properties: settlement cycles, trading hours, tick sizes, fee models)
   - LiquidityVenue (the abstraction each adapter implements)
   - VenueCredential (encrypted API key/secret storage for real exchange connections)
   - RiskCheckResult
   - MarketDataSnapshot

   For each entity, specify which service owns it and how it's communicated across service boundaries (Kafka events, gRPC calls, REST endpoints).

4. **Service Breakdown** — Detailed design for each of the three major subsystems:

   ### A. Order Gateway & Execution Engine (Go)
   
   **Product justification:** Users are connected to multiple fragmented venues simultaneously. This service unifies order lifecycle management so the user submits orders from one interface regardless of destination venue or asset class.
   
   - High-throughput, low-latency order processing pipeline
   - FIX-protocol-inspired internal message semantics (NewOrderSingle, ExecutionReport, OrderCancelRequest, OrderCancelReject)
   - Smart Order Router that selects venues based on configurable rules and an ML model. **Product value:** For crypto, real price differences across exchanges (often 10–50+ bps for larger orders) mean intelligent routing saves the user actual money. For equities, the value is execution quality analysis and venue selection.
   - **Liquidity Provider Adapter Framework**: define the `LiquidityProvider` interface with methods: `SubmitOrder`, `CancelOrder`, `SubscribeMarketData`, `GetVenueStatus`, `GetSupportedInstruments`
   - **Venue adapters — prioritize real connections alongside simulated ones:**
     - (1) **Alpaca adapter** (real) — connects to Alpaca's paper trading API for US equities. Provides real market data and simulated fills against live market conditions. This is the primary traditional-market adapter for demo and real use.
     - (2) **Binance testnet adapter** (real) — connects to Binance's testnet for crypto. Provides realistic crypto order book data and execution. Primary crypto adapter.
     - (3) **Simulated multi-asset exchange** — internal matching engine with synthetic price walks for development, testing, and demo when users haven't connected real accounts yet.
     - (4) **Dark pool / internal crossing engine** — matches orders internally across connected venues before routing residual externally. Demonstrates cross-venue arbitrage logic.
     - The adapter framework must be designed so that **community contributors can add new venues** (Coinbase, Kraken, Interactive Brokers, etc.) by implementing a single interface. This is the extensibility story and the open-source growth path.
   - Concurrency model: explain the goroutine architecture, channel-based communication patterns, and how to achieve safe concurrent processing of thousands of orders/second
   - Kafka producer: publish order lifecycle events (OrderCreated, OrderRouted, FillReceived, OrderCompleted) to topic partitioned by instrument symbol
   - Synchronous gRPC call to Risk Engine for pre-trade risk checks before routing any order
   - WebSocket server for streaming order status and fill updates to the frontend
   - REST API for human-initiated operations (submit order, cancel order, query order status, query positions)
   - **Venue Credential Manager**: securely store and retrieve user-provided API keys/secrets for each connected venue. Credentials encrypted at rest using a user-provided master passphrase. Never transmitted externally — this is a core self-hosted security promise.

   ### B. Risk & Analytics Engine (Python)
   
   **Product justification:** No existing retail or prosumer tool computes unified risk across traditional and crypto positions. A user's NVDA equity position and ETH crypto position have real correlation dynamics, different volatility regimes, different trading hours (24/7 vs market hours), and different settlement cycles (T+0 vs T+2). This engine handles all of that and gives users institutional-grade risk metrics they literally cannot get elsewhere without a Bloomberg Terminal.
   
   - Kafka consumer that maintains real-time portfolio state from the order/fill event stream
   - **Cross-asset risk modeling** — this is the core differentiator:
     - Historical VaR (rolling window of returns, configurable confidence level and horizon) accounting for different trading calendars across asset classes
     - Parametric VaR (variance-covariance method) with a cross-asset covariance matrix (equities + crypto)
     - Monte Carlo VaR (correlated return path simulation) respecting the different return distributions of crypto vs equities (fat tails, regime switching)
     - Portfolio Greeks (delta, gamma, vega for options-like instruments; beta for equities)
     - Concentration risk metrics (single-name, sector, venue, asset-class exposure)
     - Drawdown tracking and circuit breaker logic
     - **Settlement-aware risk**: unsettled equity trades (T+2) vs instantly settled crypto create different cash-at-risk profiles. The engine must model this.
   - **Portfolio Construction Optimizer**: mean-variance optimization using `cvxpy` or `scipy.optimize`, accepting constraints (max single-name weight, sector limits, target volatility, turnover limits, long-only or long-short, asset-class allocation bands). **Product value:** Users currently rebalance across two asset classes on different platforms using spreadsheets. This replaces that.
   - gRPC server exposing `CheckPreTradeRisk` endpoint (called synchronously by Order Gateway)
   - REST API for the frontend to query current risk metrics, run what-if scenarios, and trigger optimization
   - Time-series analysis: maintain a rolling window of market data, compute realized volatility, correlation matrices, and regime indicators
   - Anomaly detection on streaming market data (isolation forest or autoencoder) to flag unusual price/volume activity across all connected venues

   ### C. Frontend Dashboard (TypeScript + React)
   
   **Product justification:** This is the product surface. Everything architectural converges here into something a user sits in front of daily. It must feel like a Bloomberg Terminal for one person, not a consumer fintech app.
   
   - Professional dark-themed trading terminal aesthetic
   - **First-run onboarding experience:**
     - "Connect a Venue" flow where the user selects a venue type (Alpaca, Binance, etc.), provides API credentials, and sees data start flowing immediately
     - Credentials stored locally with encryption — the UI should clearly communicate "your keys never leave this machine"
     - Option to start with the simulated exchange if the user wants to explore before connecting real accounts
   - Core views:
     - **Unified Blotter**: live order/execution table streaming via WebSocket, showing orders across ALL connected venues in one view, with status color-coding, venue tags, click-to-cancel, filtering/sorting on high-volume data
     - **Portfolio View**: positions across all asset classes and venues, real-time P&L (handling different quote currencies and FX conversion), exposure breakdowns by asset class / sector / venue
     - **Risk Dashboard**: unified cross-asset VaR gauges, Greeks heatmaps, exposure treemaps, drawdown charts, Monte Carlo distribution plots, settlement risk timeline
     - **Liquidity Network Panel**: all connected venues with health status, connection state, latency metrics, fill rate analytics, and a "Connect New Venue" action
     - **AI Insights Panel**: execution analysis reports, portfolio rebalancing suggestions, anomaly alerts
   - Use AG Grid or TanStack Table for high-performance data grids handling thousands of streaming row updates
   - Charting via Recharts, Lightweight Charts (for candlestick/OHLC), or D3 for custom risk visualizations
   - WebSocket client for real-time streaming; REST client for commands and queries
   - State management via Zustand or Redux Toolkit

5. **AI-Integrated Features** — Design specs for each. Every AI feature must have a clear "what can the user do now that they couldn't before" statement:

   - **Smart Order Routing ML Model**: feature engineering (order size, instrument liquidity, venue historical fill rates, time of day, recent spread, cross-exchange price differential), model choice (gradient-boosted tree via XGBoost or LightGBM), training pipeline using historical fill data from connected venues, integration point in the Order Gateway routing decision. **User value:** "The system routed my ETH buy across Binance and the secondary venue, saving me 12bps vs filling on a single exchange."
   
   - **AI Execution Analyst**: post-trade analysis via Anthropic API — define the structured prompt template, what data gets sent (fills, arrival prices, venue performance, market conditions), expected output format, and how it renders in the frontend. **User value:** "After trading, I get a plain-language report telling me whether my fills were good, which venue performed best, and what to adjust next time — something only institutional desks with TCA teams get today."
   
   - **Portfolio Rebalancing Assistant**: natural-language-to-optimization-parameters pipeline via Anthropic API — user describes a target strategy in plain English ("reduce crypto to 30% of portfolio, maximize Sharpe, keep turnover under $5K"), LLM translates to structured optimizer constraints, optimizer runs, proposed trade list presented for one-click execution across the relevant venues. **User value:** "I describe what I want in English and get an executable trade list that accounts for positions across all my exchanges."
   
   - **Market Data Anomaly Detection**: streaming anomaly detection model, alert pipeline, dashboard integration. **User value:** "I got an alert that ETH volume on Binance spiked 4x normal while my equity positions were asleep — the system watches everything 24/7 even when I don't."

6. **Digital Asset & Tokenization Layer** — Design for handling the real differences between traditional and digital asset infrastructure:
   - Different settlement semantics: T+0 for crypto, T+2 for equities — the system must track unsettled positions and available-to-trade balances separately
   - Different trading hours: crypto is 24/7, equities are market-hours with pre/post-market sessions. Risk calculations and alerts must account for this.
   - Different fee models: maker/taker fees on crypto, commission + SEC/FINRA fees on equities. The P&L engine must compute net P&L accurately per asset class.
   - Wallet-based vs account-based position tracking
   - Design a tokenized securities venue adapter as a forward-looking extension: wallet-based identification, token-based positions, T+0 settlement. This can be simulated (not real blockchain) but interfaces must be correctly modeled for when real tokenized asset venues emerge.

7. **Infrastructure & Observability**
   - Docker Compose topology for local development and self-hosted deployment (all services, Kafka, Zookeeper, PostgreSQL, Redis, Prometheus, Grafana). **This is the primary deployment method for target users** — a single `docker compose up` should bring up the entire system.
   - Kubernetes manifests or Helm charts for users who want production-like deployment or run on a home server / small cloud instance
   - Prometheus metrics: order throughput, fill latency percentiles (p50/p95/p99), risk computation time, venue adapter health, Kafka consumer lag
   - Grafana dashboards: system overview, per-venue performance, risk engine health
   - Structured logging (JSON) with correlation IDs across services
   - Load testing harness (k6 or Locust): generate realistic order flow at configurable rates, produce throughput/latency reports
   - **Startup health checks**: on boot, verify all venue connections, validate stored credentials (without exposing them in logs), report system readiness

8. **Development Roadmap** — Break the project into 5 phases with clear milestones. Each phase should produce a usable increment:

   - **Phase 1 — Single-venue trading loop**: Core domain model, Order Gateway with the simulated exchange adapter, basic REST API, minimal frontend with blotter view. **Acceptance: a user can submit an order via the UI, see it route to the simulated venue, receive a fill, and see their position update.**
   
   - **Phase 2 — Real venue connectivity + risk**: Alpaca adapter (equities) and Binance testnet adapter (crypto), Kafka event backbone, Risk Engine with basic cross-asset VaR, frontend portfolio and risk views. **Acceptance: a user connects Alpaca and Binance paper accounts, sees unified positions and a combined VaR number across both asset classes.**
   
   - **Phase 3 — Smart routing + portfolio optimization**: Smart Order Router with cross-venue price comparison, dark pool/crossing logic, portfolio construction optimizer with constraint support, frontend liquidity network panel. **Acceptance: an order is routed to the best-priced venue automatically; the optimizer produces a rebalancing trade list respecting user constraints.**
   
   - **Phase 4 — AI features**: Execution analyst (Anthropic API), rebalancing assistant (natural language → optimizer), anomaly detection, AI Insights panel in frontend. **Acceptance: user gets a post-trade analysis report in plain English; user describes a rebalancing goal in natural language and gets an executable trade list.**
   
   - **Phase 5 — Production hardening**: Tokenized asset layer, observability stack (Prometheus/Grafana), load testing with published benchmarks, credential encryption, onboarding flow polish, comprehensive documentation and README. **Acceptance: `docker compose up` brings up the full system; a new user can go from zero to connected-and-trading in under 5 minutes; load test report shows sustained throughput targets.**

9. **Testing Strategy**
   - Unit tests per service (Go table-driven tests, Python pytest, React Testing Library)
   - Integration tests for cross-service flows (order submission → risk check → routing → fill → position update) using the simulated venue for deterministic behavior
   - End-to-end tests for critical user journeys (connect venue → submit order → see fill → check risk dashboard)
   - Performance benchmarks with specific targets (e.g., "Order Gateway sustains 5,000 orders/sec with p99 < 50ms")
   - **Venue adapter contract tests**: a shared test suite that any adapter implementation must pass, ensuring new community-contributed adapters meet the interface contract

10. **Documentation & Open-Source Strategy**
    - README must lead with the problem and product, not the architecture. First paragraph answers "why does this exist" for a user, not an engineer.
    - Quickstart guide: from `git clone` to a running system with the simulated venue in under 3 minutes
    - "Connect Your First Exchange" guide with screenshots
    - "Write a Venue Adapter" contributor guide — this is the primary open-source contribution path
    - Architecture overview for contributors (this is where the technical depth lives)
    - LICENSE selection rationale (recommend AGPLv3 or BSL for a project like this — discuss tradeoffs)

---

## Constraints & Principles

- **Product-first architecture.** Every component must trace to a user need. The adapter framework exists because venues are fragmented. The risk engine exists because no retail tool does cross-asset risk. The AI features exist because institutional analysis is gatekept by cost.
- **Self-hosted is a feature, not a limitation.** API keys, position data, and trading strategies never leave the user's machine. This is a core value proposition and must be reflected in credential management, deployment architecture, and documentation messaging.
- **Domain-driven naming everywhere.** No `utils`, `helpers`, `common`. Use names a trader or portfolio manager would recognize: `OrderRouter`, `RiskGate`, `VenueAdapter`, `PositionKeeper`, `ExecutionAnalyzer`.
- **Clean Architecture boundaries.** Domain logic must be independent of frameworks. Infrastructure concerns (Kafka, gRPC, HTTP) live at the edges.
- **Library-first approach.** Use established libraries (gorilla/websocket, confluent-kafka-go, gRPC-go, cvxpy, AG Grid, Zustand) instead of writing custom infrastructure code. Custom code is reserved for domain-specific business logic.
- **Every interface should be designed for extensibility and community contribution.** The `LiquidityProvider` adapter pattern is the flagship example — adding a new venue is implementing one interface, not modifying core routing logic. This is the open-source growth path.
- **Early return pattern in all code.** Max 3 levels of nesting. Functions under 50 lines. Files under 200 lines where possible.
- **Protobuf for all inter-service message schemas.** Define `.proto` files in a shared `proto/` directory at the repo root. Generate Go and Python bindings.

---

## Output Format

Structure your response as a single, comprehensive architecture document in Markdown. Use headers for each section. Include:
- Directory tree (using code blocks)
- Interface definitions (in Go for adapters, Python for risk engine APIs, TypeScript for frontend types)
- Kafka topic schemas
- ADRs in a consistent format (with the Product Justification field)
- Sequence diagrams described in text (or Mermaid syntax if supported)
- Concrete technology versions and library choices

Do NOT produce vague hand-wavy descriptions. Every component should have enough specificity that a developer could begin implementation from this document. And every component should clearly serve the product thesis: unified cross-asset trading and risk for the underserved middle market between retail and institutional.

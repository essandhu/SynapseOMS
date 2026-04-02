# SynapseOMS Deliverables Checklist

## Proto Schemas

- [x] proto/order/order.proto (Order, Fill, OrderRequest, ExecutionReport, order-lifecycle events)
- [x] proto/risk/risk.proto (RiskCheckRequest/Response, PreTradeRiskRequest/Response, RiskCheck, RiskGate service)
- [x] proto/instrument/instrument.proto (Instrument, InstrumentType, SettlementCycle)
- [x] proto/portfolio/portfolio.proto (Position, Portfolio, Exposure)
- [x] proto/marketdata/marketdata.proto (MarketDataSnapshot, OHLCV, OrderBookLevel)
- [x] proto/venue/venue.proto (VenueStatus, VenueCapabilities, VenueConnected/Disconnected/Degraded events)
- [x] proto/buf.yaml (Buf build configuration, linting, breaking change detection)

## Gateway Service

- [x] gateway/cmd/gateway/main.go (entry point, startup health checks, graceful shutdown)
- [x] gateway/internal/domain/order.go (Order aggregate, state machine with transitions)
- [x] gateway/internal/domain/fill.go (Fill / ExecutionReport value object)
- [x] gateway/internal/domain/instrument.go (Instrument value object, AssetClass, SettlementCycle, TradingSchedule)
- [x] gateway/internal/domain/position.go (Position aggregate with P&L, settled/unsettled quantities)
- [x] gateway/internal/domain/venue_credential.go (VenueCredential with encrypted fields)
- [ ] gateway/internal/orderbook/book.go (in-memory order book per instrument)
- [x] gateway/internal/router/router.go (routing engine)
- [x] gateway/internal/router/strategy.go (routing strategies: best-price, venue-preference, ML-scored)
- [x] gateway/internal/router/ml_scorer.go (ML model inference for venue scoring via Python sidecar)
- [x] gateway/internal/crossing/engine.go (dark pool / internal crossing engine)
- [x] gateway/internal/adapter/provider.go (LiquidityProvider interface definition)
- [x] gateway/internal/adapter/registry.go (adapter registration and discovery)
- [x] gateway/internal/adapter/alpaca/adapter.go (Alpaca adapter, REST + paper trading)
- [x] gateway/internal/adapter/alpaca/ws_feed.go (Alpaca WebSocket market data feed)
- [x] gateway/internal/adapter/binance/adapter.go (Binance testnet adapter, REST execution)
- [x] gateway/internal/adapter/binance/ws_feed.go (Binance WebSocket market data feed)
- [x] gateway/internal/adapter/simulated/matching_engine.go (simulated multi-asset matching engine)
- [x] gateway/internal/adapter/simulated/price_walk.go (geometric Brownian motion price generator)
- [ ] gateway/internal/adapter/tokenized/adapter.go (tokenized securities adapter, T+0 settlement)
- [x] gateway/internal/credential/manager.go (encrypt/decrypt with Argon2id + AES-256-GCM)
- [x] gateway/internal/credential/vault.go (on-disk encrypted storage in PostgreSQL)
- [x] gateway/internal/pipeline/pipeline.go (intake -> risk check -> route -> fill -> notify)
- [~] gateway/internal/pipeline/stage.go — SUPERSEDED: pipeline uses goroutine-based stages, not a Stage interface
- [x] gateway/internal/kafka/producer.go (Kafka producer for order-lifecycle, market-data, venue-status topics)
- [x] gateway/internal/grpc/risk_client.go (gRPC client for pre-trade risk checks) — NOTE: fail-open stub until proto stubs generated
- [x] gateway/internal/ws/server.go (WebSocket server: /ws/orders, /ws/positions, /ws/marketdata, /ws/venues)
- [x] gateway/internal/rest/handler_order.go (REST: submit, cancel, list, get orders)
- [x] gateway/internal/rest/handler_position.go (REST: list positions, get position by instrument)
- [x] gateway/internal/rest/handler_venue.go (REST: list venues, connect, disconnect)
- [x] gateway/internal/rest/handler_credential.go (REST: store/delete venue credentials)
- [x] gateway/go.mod
- [x] gateway/go.sum
- [x] gateway/Dockerfile
- [x] PostgreSQL schema + migrations (orders, fills, positions, instruments, venues, credentials tables)

## Risk Engine

- [x] risk_engine/__init__.py
- [x] risk_engine/main.py (FastAPI + Kafka consumer + gRPC server co-start)
- [x] risk_engine/domain/position.py (Position aggregate from events)
- [x] risk_engine/domain/portfolio.py (Portfolio with cross-asset positions, NAV, exposure breakdowns)
- [x] risk_engine/domain/instrument.py (Instrument, TradingCalendar, FeeSchedule)
- [x] risk_engine/domain/risk_result.py (VaR, RiskCheck, RiskCheckResult)
- [x] risk_engine/var/historical.py (Historical simulation VaR, mixed calendar handling)
- [x] risk_engine/var/parametric.py (Variance-covariance VaR)
- [x] risk_engine/var/monte_carlo.py (Monte Carlo VaR with correlated paths, fat-tailed distributions)
- [x] risk_engine/greeks/calculator.py (Portfolio Greeks calculator)
- [x] risk_engine/concentration/analyzer.py (Concentration risk analyzer)
- [x] risk_engine/settlement/tracker.py (Settlement-aware cash-at-risk, T+0 vs T+2)
- [x] risk_engine/optimizer/mean_variance.py (cvxpy-based mean-variance optimization)
- [x] risk_engine/optimizer/constraints.py (OptimizationConstraints definitions)
- [ ] risk_engine/anomaly/detector.py (Isolation Forest streaming anomaly detector)
- [x] risk_engine/timeseries/statistics.py (Rolling statistics)
- [x] risk_engine/timeseries/covariance.py (Covariance matrix computation, Ledoit-Wolf shrinkage)
- [ ] risk_engine/timeseries/regime.py (Regime detection)
- [x] risk_engine/kafka/consumer.py (Kafka consumer for order-lifecycle topic, portfolio state builder)
- [x] risk_engine/grpc_server/server.py (gRPC server: CheckPreTradeRisk with 4 risk checks)
- [x] risk_engine/rest/router_risk.py (REST: VaR, drawdown, settlement, portfolio, exposure)
- [x] risk_engine/rest/router_optimizer.py (REST: portfolio optimization)
- [ ] risk_engine/rest/router_scenario.py (REST: what-if scenario analysis)
- [x] risk_engine/pyproject.toml (pip project config with all dependencies)
- [ ] risk_engine/requirements.lock
- [x] risk-engine/Dockerfile

## Dashboard

- [x] dashboard/src/main.tsx (app entry point)
- [x] dashboard/src/App.tsx (root layout + routing + stream initialization)
- [x] dashboard/src/api/rest.ts (REST client wrapper)
- [x] dashboard/src/api/ws.ts (WebSocket client with reconnect for all streams)
- [x] dashboard/src/api/types.ts (TypeScript types generated from proto)
- [x] dashboard/src/stores/orderStore.ts (Zustand order state, submit, cancel, applyUpdate)
- [x] dashboard/src/stores/positionStore.ts (Zustand position state)
- [x] dashboard/src/stores/riskStore.ts (Zustand risk metrics state)
- [x] dashboard/src/stores/venueStore.ts (Zustand venue status state)
- [ ] dashboard/src/stores/insightStore.ts (Zustand AI insights state)
- [x] dashboard/src/views/BlotterView.tsx (AG Grid order blotter with streaming updates, filters, order ticket panel)
- [x] dashboard/src/views/PortfolioView.tsx (position table, NAV summary cards, exposure breakdown charts)
- [x] dashboard/src/views/RiskDashboard.tsx (VaR gauges, MC histogram, Greeks heatmap, concentration treemap, drawdown chart, settlement timeline)
- [x] dashboard/src/views/LiquidityNetwork.tsx (venue status cards, connect new venue)
- [ ] dashboard/src/views/InsightsPanel.tsx (execution analysis tab, rebalancing tab with NL input, anomaly alerts tab)
- [x] dashboard/src/views/OnboardingView.tsx (5-step onboarding: welcome, passphrase, venue choice, credentials, ready)
- [x] dashboard/src/components/OrderTicket.tsx (order entry form: instrument picker, side, type, qty, price, venue/"Smart Route")
- [x] dashboard/src/components/OrderTable.tsx (AG Grid blotter table)
- [x] dashboard/src/components/PositionTable.tsx (position data grid)
- [x] dashboard/src/components/VaRGauge.tsx (VaR visualization gauge with color coding)
- [x] dashboard/src/components/ExposureTreemap.tsx (Recharts donut chart for exposure) — NOTE: D3 treemap version is Phase 3
- [x] dashboard/src/components/DrawdownChart.tsx (Recharts drawdown time series)
- [x] dashboard/src/components/MonteCarloPlot.tsx (MC simulation distribution histogram)
- [ ] dashboard/src/components/CandlestickChart.tsx (Lightweight Charts wrapper for OHLC)
- [x] dashboard/src/components/VenueCard.tsx (venue status card with latency, fill rate, heartbeat)
- [x] dashboard/src/components/CredentialForm.tsx (secure API key input for onboarding)
- [x] dashboard/src/components/TerminalLayout.tsx (dark terminal shell + panel layout)
- [x] dashboard/src/theme/terminal.ts (dark theme tokens: colors, fonts)
- [x] dashboard/index.html
- [x] dashboard/vite.config.ts
- [x] dashboard/tsconfig.json
- [x] dashboard/package.json
- [x] dashboard/Dockerfile

## AI Modules

- [ ] ai/execution_analyst/analyst.py (Anthropic API integration, ExecutionAnalyst class)
- [ ] ai/execution_analyst/prompt_templates.py (EXECUTION_ANALYSIS_PROMPT template)
- [ ] ai/rebalancing_assistant/assistant.py (NL -> optimizer constraints via Anthropic API)
- [ ] ai/rebalancing_assistant/prompt_templates.py (CONSTRAINT_EXTRACTION_PROMPT template)
- [x] ai/smart_router_ml/features.py (feature engineering pipeline for venue scoring)
- [x] ai/smart_router_ml/train.py (XGBoost training script)
- [x] ai/smart_router_ml/model.py (inference wrapper / FastAPI scoring sidecar)
- [x] ai/requirements.txt

## Infrastructure

- [x] deploy/docker-compose.yml (gateway, risk-engine, dashboard, ml-scorer, kafka, postgres, redis) — Phase 3: added ml-scorer sidecar
- [ ] deploy/docker-compose.dev.yml (dev overrides: hot reload, debug ports)
- [ ] deploy/k8s/namespace.yaml
- [ ] deploy/k8s/gateway-deployment.yaml
- [ ] deploy/k8s/risk-engine-deployment.yaml
- [ ] deploy/k8s/dashboard-deployment.yaml
- [ ] deploy/k8s/kafka-statefulset.yaml
- [ ] deploy/k8s/postgres-statefulset.yaml
- [ ] deploy/k8s/redis-deployment.yaml
- [ ] deploy/k8s/monitoring/prometheus-config.yaml
- [ ] deploy/k8s/monitoring/grafana-dashboards.yaml
- [ ] deploy/grafana/system-overview.json (pre-built Grafana dashboard)
- [ ] deploy/grafana/venue-performance.json (per-venue performance dashboard)
- [ ] deploy/prometheus.yml (Prometheus scrape config for gateway + risk-engine metrics)
- [ ] loadtest/k6/order_flow.js (realistic order submission load test, 5k orders/sec target)
- [ ] loadtest/k6/ws_stream.js (WebSocket streaming load test)
- [ ] loadtest/README.md
- [x] Makefile (top-level build targets)

## Scripts

- [x] scripts/proto-gen.sh (generate Go + Python + TS from protos using buf)
- [x] scripts/seed-instruments.sh (seed instrument reference data: AAPL, MSFT, GOOG, BTC-USD, ETH-USD, SOL-USD)
- [ ] scripts/health-check.sh (verify all services healthy)

## Documentation

- [ ] README.md (product-first: problem statement, who is this for, features, quickstart, screenshots)
- [ ] docs/quickstart.md (git clone -> running in 3 minutes with simulated venue)
- [ ] docs/connect-venue.md (Alpaca paper trading + Binance testnet setup, security explanation)
- [ ] docs/write-adapter.md (contributor guide: implement LiquidityProvider, register, pass contract tests)
- [ ] docs/architecture-overview.md (visual architecture for contributors)
- [ ] LICENSE (AGPLv3)
- [ ] CONTRIBUTING.md

## CI/CD

- [ ] .github/workflows/ci.yml (build + test all services: Go, Python, TypeScript)
- [ ] .github/workflows/release.yml (Docker image builds)

## Tests

- [ ] gateway/internal/orderbook/book_test.go (order book unit tests)
- [x] gateway/internal/router/router_test.go (router unit tests — 49 tests)
- [x] gateway/internal/crossing/engine_test.go (crossing engine unit tests — 12 tests)
- [x] gateway/internal/adapter/alpaca/adapter_test.go (Alpaca adapter unit tests — 21 tests)
- [x] gateway/internal/adapter/binance/adapter_test.go (Binance adapter unit tests — 21 tests)
- [x] gateway/internal/adapter/simulated/matching_engine_test.go (simulated exchange unit tests)
- [ ] gateway/internal/adapter/tokenized/adapter_test.go (tokenized adapter unit tests)
- [x] gateway/internal/credential/manager_test.go (credential manager unit tests — 6 tests)
- [x] gateway/internal/pipeline/pipeline_test.go (pipeline unit tests)
- [x] gateway/internal/domain/order_test.go (order state machine transition tests)
- [ ] risk_engine/var/var_test.py (VaR module-level tests)
- [x] risk_engine/greeks/calculator_test.py (Greeks calculator tests — 14 tests)
- [x] risk_engine/concentration/analyzer_test.py (concentration analyzer tests — 17 tests)
- [x] risk_engine/settlement/tracker_test.py (settlement tracker tests — 15 tests)
- [x] risk_engine/optimizer/optimizer_test.py (optimizer tests — 12 tests)
- [ ] risk_engine/anomaly/detector_test.py (anomaly detector tests)
- [x] risk_engine/tests/conftest.py (pytest fixtures: portfolio state, return matrices, covariance matrices)
- [x] risk_engine/tests/test_var_historical.py (historical VaR tests including cross-asset — 6 tests)
- [x] risk_engine/tests/test_var_parametric.py (parametric VaR tests — 7 tests)
- [x] risk_engine/tests/test_var_monte_carlo.py (Monte Carlo VaR tests — 13 tests)
- [x] risk_engine/tests/test_optimizer.py (optimizer integration tests — 12 tests)
- [x] risk_engine/tests/test_settlement.py (settlement risk tests — 15 tests)
- [ ] risk_engine/tests/test_anomaly.py (anomaly detection tests)
- [ ] ai/execution_analyst/analyst_test.py (execution analyst tests)
- [ ] ai/rebalancing_assistant/assistant_test.py (rebalancing assistant tests)
- [x] ai/smart_router_ml/model_test.py (ML scorer tests — 14 tests)
- [x] dashboard/src/components/OrderTicket.test.tsx (order ticket component tests)
- [x] gateway/integration_test.go (order -> risk check -> route -> fill -> position flow)
- [x] gateway/internal/adapter/contract_test.go (AdapterContractSuite shared contract tests — 21 subtests)
- [ ] E2E: connect venue -> submit order -> see fill -> check risk (Playwright)
- [ ] E2E: multi-venue portfolio unified view (Playwright)
- [ ] E2E: order cancellation flow (Playwright)
- [ ] gateway/internal/pipeline/pipeline_bench_test.go (order pipeline performance benchmark)

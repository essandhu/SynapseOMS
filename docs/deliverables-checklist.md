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

- [ ] gateway/cmd/gateway/main.go (entry point, startup health checks, graceful shutdown)
- [ ] gateway/internal/domain/order.go (Order aggregate, state machine with transitions)
- [ ] gateway/internal/domain/fill.go (Fill / ExecutionReport value object)
- [ ] gateway/internal/domain/instrument.go (Instrument value object, AssetClass, SettlementCycle, TradingSchedule)
- [ ] gateway/internal/domain/position.go (Position aggregate with P&L, settled/unsettled quantities)
- [ ] gateway/internal/domain/venue_credential.go (VenueCredential with encrypted fields)
- [ ] gateway/internal/orderbook/book.go (in-memory order book per instrument)
- [ ] gateway/internal/router/router.go (routing engine)
- [ ] gateway/internal/router/strategy.go (routing strategies: best-price, TWAP, ML-scored)
- [ ] gateway/internal/router/ml_scorer.go (ML model inference for venue scoring via Python sidecar)
- [ ] gateway/internal/crossing/engine.go (dark pool / internal crossing engine)
- [ ] gateway/internal/adapter/provider.go (LiquidityProvider interface definition)
- [ ] gateway/internal/adapter/registry.go (adapter registration and discovery)
- [ ] gateway/internal/adapter/alpaca/adapter.go (Alpaca adapter, REST + paper trading)
- [ ] gateway/internal/adapter/alpaca/ws_feed.go (Alpaca WebSocket market data feed)
- [ ] gateway/internal/adapter/binance/adapter.go (Binance testnet adapter, REST execution)
- [ ] gateway/internal/adapter/binance/ws_feed.go (Binance WebSocket market data feed)
- [ ] gateway/internal/adapter/simulated/matching_engine.go (simulated multi-asset matching engine)
- [ ] gateway/internal/adapter/simulated/price_walk.go (geometric Brownian motion price generator)
- [ ] gateway/internal/adapter/tokenized/adapter.go (tokenized securities adapter, T+0 settlement)
- [ ] gateway/internal/credential/manager.go (encrypt/decrypt with Argon2id + AES-256-GCM)
- [ ] gateway/internal/credential/vault.go (on-disk encrypted storage in PostgreSQL)
- [ ] gateway/internal/pipeline/pipeline.go (intake -> risk check -> route -> fill -> notify)
- [ ] gateway/internal/pipeline/stage.go (pipeline stage interface)
- [ ] gateway/internal/kafka/producer.go (Kafka producer for order-lifecycle, market-data, venue-status topics)
- [ ] gateway/internal/grpc/risk_client.go (gRPC client for pre-trade risk checks)
- [ ] gateway/internal/ws/server.go (WebSocket server: /ws/orders, /ws/positions, /ws/marketdata, /ws/venues)
- [ ] gateway/internal/rest/handler_order.go (REST: submit, cancel, list, get orders)
- [ ] gateway/internal/rest/handler_position.go (REST: list positions, get position by instrument)
- [ ] gateway/internal/rest/handler_venue.go (REST: list venues, connect, disconnect)
- [ ] gateway/internal/rest/handler_credential.go (REST: store/delete venue credentials)
- [ ] gateway/go.mod
- [ ] gateway/go.sum
- [ ] gateway/Dockerfile
- [ ] PostgreSQL schema + migrations (orders, fills, positions, instruments, venues, credentials tables)

## Risk Engine

- [ ] risk_engine/__init__.py
- [ ] risk_engine/main.py (FastAPI + Kafka consumer startup)
- [ ] risk_engine/domain/position.py (Position aggregate from events)
- [ ] risk_engine/domain/portfolio.py (Portfolio with cross-asset positions, NAV, exposure breakdowns)
- [ ] risk_engine/domain/instrument.py (Instrument, TradingCalendar, FeeSchedule)
- [ ] risk_engine/domain/risk_result.py (VaR, Greeks, concentration metrics)
- [ ] risk_engine/var/historical.py (Historical simulation VaR, mixed calendar handling)
- [ ] risk_engine/var/parametric.py (Variance-covariance VaR)
- [ ] risk_engine/var/monte_carlo.py (Monte Carlo VaR with correlated paths, fat-tailed distributions)
- [ ] risk_engine/greeks/calculator.py (Portfolio Greeks calculator)
- [ ] risk_engine/concentration/analyzer.py (Concentration risk analyzer)
- [ ] risk_engine/settlement/tracker.py (Settlement-aware cash-at-risk, T+0 vs T+2)
- [ ] risk_engine/optimizer/mean_variance.py (cvxpy-based mean-variance optimization)
- [ ] risk_engine/optimizer/constraints.py (OptimizationConstraints definitions)
- [ ] risk_engine/anomaly/detector.py (Isolation Forest streaming anomaly detector)
- [ ] risk_engine/timeseries/statistics.py (Rolling statistics)
- [ ] risk_engine/timeseries/covariance.py (Covariance matrix computation)
- [ ] risk_engine/timeseries/regime.py (Regime detection)
- [ ] risk_engine/kafka/consumer.py (Kafka consumer for order-lifecycle and market-data topics)
- [ ] risk_engine/grpc_server/server.py (gRPC server: CheckPreTradeRisk)
- [ ] risk_engine/rest/router_risk.py (REST: VaR, Greeks, concentration, drawdown, settlement, scenario)
- [ ] risk_engine/rest/router_optimizer.py (REST: portfolio optimization)
- [ ] risk_engine/rest/router_scenario.py (REST: what-if scenario analysis)
- [ ] risk_engine/pyproject.toml (uv / pip project config)
- [ ] risk_engine/requirements.lock
- [ ] risk-engine/Dockerfile

## Dashboard

- [ ] dashboard/src/main.tsx (app entry point)
- [ ] dashboard/src/App.tsx (root layout + routing + stream initialization)
- [ ] dashboard/src/api/rest.ts (REST client wrapper)
- [ ] dashboard/src/api/ws.ts (WebSocket client with reconnect for all streams)
- [ ] dashboard/src/api/types.ts (TypeScript types generated from proto)
- [ ] dashboard/src/stores/orderStore.ts (Zustand order state, submit, cancel, applyUpdate)
- [ ] dashboard/src/stores/positionStore.ts (Zustand position state)
- [ ] dashboard/src/stores/riskStore.ts (Zustand risk metrics state)
- [ ] dashboard/src/stores/venueStore.ts (Zustand venue status state)
- [ ] dashboard/src/stores/insightStore.ts (Zustand AI insights state)
- [ ] dashboard/src/views/BlotterView.tsx (AG Grid order blotter with streaming updates, filters, order ticket panel)
- [ ] dashboard/src/views/PortfolioView.tsx (position table, NAV summary cards, exposure breakdown charts)
- [ ] dashboard/src/views/RiskDashboard.tsx (VaR gauges, MC distribution histogram, Greeks heatmap, drawdown chart, concentration treemap, settlement timeline)
- [ ] dashboard/src/views/LiquidityNetwork.tsx (venue status cards, connect new venue, per-venue drill-down)
- [ ] dashboard/src/views/InsightsPanel.tsx (execution analysis tab, rebalancing tab with NL input, anomaly alerts tab)
- [ ] dashboard/src/views/OnboardingView.tsx (5-step onboarding: welcome, passphrase, venue choice, credentials, first order)
- [ ] dashboard/src/components/OrderTicket.tsx (order entry form: instrument picker, side, type, qty, price, venue/"Smart Route")
- [ ] dashboard/src/components/OrderTable.tsx (AG Grid blotter table)
- [ ] dashboard/src/components/PositionTable.tsx (position data grid)
- [ ] dashboard/src/components/VaRGauge.tsx (VaR visualization gauge with color coding)
- [ ] dashboard/src/components/ExposureTreemap.tsx (D3 treemap for exposure/concentration)
- [ ] dashboard/src/components/DrawdownChart.tsx (Recharts drawdown time series)
- [ ] dashboard/src/components/MonteCarloPlot.tsx (MC simulation distribution histogram)
- [ ] dashboard/src/components/CandlestickChart.tsx (Lightweight Charts wrapper for OHLC)
- [ ] dashboard/src/components/VenueCard.tsx (venue status card with latency, fill rate, heartbeat)
- [ ] dashboard/src/components/CredentialForm.tsx (secure API key input modal)
- [ ] dashboard/src/components/TerminalLayout.tsx (dark terminal shell + panel layout)
- [ ] dashboard/src/theme/terminal.ts (dark theme tokens: colors, fonts)
- [ ] dashboard/index.html
- [ ] dashboard/vite.config.ts
- [ ] dashboard/tsconfig.json
- [ ] dashboard/package.json
- [ ] dashboard/Dockerfile

## AI Modules

- [ ] ai/execution_analyst/analyst.py (Anthropic API integration, ExecutionAnalyst class)
- [ ] ai/execution_analyst/prompt_templates.py (EXECUTION_ANALYSIS_PROMPT template)
- [ ] ai/rebalancing_assistant/assistant.py (NL -> optimizer constraints via Anthropic API)
- [ ] ai/rebalancing_assistant/prompt_templates.py (CONSTRAINT_EXTRACTION_PROMPT template)
- [ ] ai/smart_router_ml/features.py (feature engineering pipeline for venue scoring)
- [ ] ai/smart_router_ml/train.py (XGBoost training script)
- [ ] ai/smart_router_ml/model.py (inference wrapper / FastAPI scoring sidecar)
- [ ] ai/requirements.txt

## Infrastructure

- [ ] deploy/docker-compose.yml (gateway, risk-engine, dashboard, ml-scorer, kafka, postgres, redis, prometheus, grafana)
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
- [ ] Makefile (top-level build targets)

## Scripts

- [x] scripts/proto-gen.sh (generate Go + Python + TS from protos using buf)
- [ ] scripts/seed-instruments.sh (seed instrument reference data: AAPL, MSFT, GOOG, BTC-USD, ETH-USD, SOL-USD)
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
- [ ] gateway/internal/router/router_test.go (router unit tests)
- [ ] gateway/internal/crossing/engine_test.go (crossing engine unit tests)
- [ ] gateway/internal/adapter/alpaca/adapter_test.go (Alpaca adapter unit tests)
- [ ] gateway/internal/adapter/binance/adapter_test.go (Binance adapter unit tests)
- [ ] gateway/internal/adapter/simulated/matching_engine_test.go (simulated exchange unit tests)
- [ ] gateway/internal/adapter/tokenized/adapter_test.go (tokenized adapter unit tests)
- [ ] gateway/internal/credential/manager_test.go (credential manager unit tests)
- [ ] gateway/internal/pipeline/pipeline_test.go (pipeline unit tests)
- [ ] gateway/internal/domain/order_test.go (order state machine transition tests)
- [ ] risk_engine/var/var_test.py (VaR module-level tests)
- [ ] risk_engine/greeks/calculator_test.py (Greeks calculator tests)
- [ ] risk_engine/concentration/analyzer_test.py (concentration analyzer tests)
- [ ] risk_engine/settlement/tracker_test.py (settlement tracker tests)
- [ ] risk_engine/optimizer/optimizer_test.py (optimizer tests)
- [ ] risk_engine/anomaly/detector_test.py (anomaly detector tests)
- [ ] risk_engine/tests/conftest.py (pytest fixtures: portfolio state, return matrices, covariance matrices)
- [ ] risk_engine/tests/test_var_historical.py (historical VaR tests including cross-asset)
- [ ] risk_engine/tests/test_var_parametric.py (parametric VaR tests)
- [ ] risk_engine/tests/test_var_monte_carlo.py (Monte Carlo VaR tests)
- [ ] risk_engine/tests/test_optimizer.py (optimizer integration tests)
- [ ] risk_engine/tests/test_settlement.py (settlement risk tests)
- [ ] risk_engine/tests/test_anomaly.py (anomaly detection tests)
- [ ] ai/execution_analyst/analyst_test.py (execution analyst tests)
- [ ] ai/rebalancing_assistant/assistant_test.py (rebalancing assistant tests)
- [ ] ai/smart_router_ml/model_test.py (ML scorer tests)
- [ ] dashboard/src/components/OrderTicket.test.tsx (order ticket component tests)
- [ ] gateway/integration_test.go (order -> risk check -> route -> fill -> position flow)
- [ ] gateway/internal/adapter/contract_test.go (AdapterContractSuite shared contract tests)
- [ ] E2E: connect venue -> submit order -> see fill -> check risk (Playwright)
- [ ] E2E: multi-venue portfolio unified view (Playwright)
- [ ] E2E: order cancellation flow (Playwright)
- [ ] gateway/internal/pipeline/pipeline_bench_test.go (order pipeline performance benchmark)

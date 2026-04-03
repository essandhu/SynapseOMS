# Phase 7 Tasks — Catch-Up & Deferred Items

**Goal:** Address all catch-up items from the Phase 6 review and all deferred deliverables from the checklist — completing the 5m interval UI, dynamic market data subscription, volume aggregation, regime detection, what-if scenario analysis, Docker release automation, and Playwright E2E tests.

**Acceptance Test:** User opens BlotterView, switches the candlestick chart between 1m and 5m intervals via a UI selector. User connects a new venue at runtime and sees its market data appear in the chart without restarting the gateway. Risk dashboard exposes a what-if scenario endpoint that returns projected VaR for a hypothetical position. Three Playwright E2E tests pass: order flow, multi-venue portfolio, and order cancellation.

**Architecture Doc References:** Sections 5.4 (Risk REST endpoints — `/api/v1/risk/scenario`), 9.3 (E2E tests — Playwright), 1 (directory structure — `release.yml`), Appendix B (deferred items: `regime.py`, `router_scenario.py`, `release.yml`)

**Previous Phase Review:** Phase 6 passed all 6 tasks. Three catch-up items flagged: (1) 5m interval UI selector missing, (2) dynamic market data subscription not wired on venue connect, (3) volume aggregation always zero. Two deviations relevant to this phase: market data subscribes at startup only (Deviation 2), and lightweight-charts v5 API uses `addSeries()` pattern (Deviation 1).

---

## Tasks

### P7-01: ✅ COMPLETE — [CATCH-UP] 5m Interval UI Selector for CandlestickChart

**Service:** Dashboard
**Files:**
- `dashboard/src/views/BlotterView.tsx` (modify — add interval toggle)
- `dashboard/src/components/CandlestickChart.tsx` (modify — accept and use interval prop)
- `dashboard/src/stores/marketDataStore.ts` (modify — support interval in subscribe/getBars)
- `dashboard/src/views/BlotterView.test.tsx` (modify — test interval toggle)
**Dependencies:** None
**Acceptance Criteria:**
- Chart panel header in BlotterView shows an interval toggle with "1m" and "5m" options
- Selecting an interval updates the `CandlestickChart` component's `interval` prop
- `marketDataStore` filters bars by interval (bars arrive tagged with interval from the gateway aggregator)
- Default remains 1m
- Existing BlotterView and CandlestickChart tests continue to pass
- New test verifies interval toggle switches between 1m and 5m

**Architecture Context:**
The `CandlestickChart` component already accepts an `interval` prop typed as `"1m" | "5m"` (default `"1m"`). The gateway `Aggregator` (`gateway/internal/marketdata/aggregator.go`) already supports configurable intervals. The WebSocket message includes an `"interval"` field. The missing piece is purely a UI control: a toggle/dropdown in `BlotterView.tsx`'s chart panel header that sets the interval state and passes it to `CandlestickChart`. The `marketDataStore` should key its bar arrays by `{instrumentId}:{interval}` to keep 1m and 5m bars separate.

**Note:** The gateway currently only runs one aggregator at 1m interval. To fully support 5m from the gateway side, either a second aggregator instance at 5m is needed in `main.go`, or the single aggregator should be replaced with a map of interval→aggregator. For this task, focus on the dashboard UI and store changes. If the gateway only emits 1m bars, the 5m option will show no data until P7-02a is implemented (see below).

---

### P7-01a: ✅ COMPLETE — Gateway Multi-Interval Aggregator Support

**Service:** Gateway
**Files:**
- `gateway/cmd/gateway/main.go` (modify — create aggregators for both 1m and 5m)
- `gateway/internal/marketdata/aggregator.go` (modify if needed — ensure interval is included in emitted bars)
- `gateway/internal/marketdata/aggregator_test.go` (modify — test multi-interval)
**Dependencies:** None
**Acceptance Criteria:**
- Gateway runs two aggregator instances (1m and 5m) sharing the same tick feed
- Both emit bars to the same output channel (or merged channels) with the interval encoded in the bar
- WebSocket broadcast includes the interval field so clients can filter
- Existing aggregator tests pass; new test verifies 5m aggregation

**Architecture Context:**
Currently `main.go` creates one aggregator: `mdAgg := marketdata.NewAggregator(time.Minute, mdOut)`. Add a second: `mdAgg5m := marketdata.NewAggregator(5*time.Minute, mdOut)`. Both read from the same tick goroutines via a fan-out (each tick is sent to both aggregators). The `OHLCBar` struct already has `PeriodStart`/`PeriodEnd` which implicitly encode interval, but add an explicit `Interval string` field (e.g., `"1m"`, `"5m"`) for clarity in the WebSocket message. The relay goroutine in main.go should include `"interval"` in the JSON it broadcasts.

---

### P7-02: ✅ COMPLETE — [CATCH-UP] Dynamic Market Data Subscription on Venue Connect

**Service:** Gateway
**Files:**
- `gateway/internal/rest/handler_venue.go` (modify — subscribe to market data after successful connect)
- `gateway/cmd/gateway/main.go` (modify — expose aggregator reference for handler to use)
**Dependencies:** P7-01a
**Acceptance Criteria:**
- When a venue is connected via `POST /api/v1/venues/{id}/connect`, the handler subscribes to that venue's market data and feeds ticks to the aggregator(s)
- No gateway restart required for new venues to appear in chart data
- Disconnecting a venue cancels its market data subscription
- Existing venue handler tests pass

**Architecture Context:**
The `connectVenue` handler in `handler_venue.go` (line 106) currently calls `p.Connect(ctx, cred)` and returns. After a successful connect, it should also call `p.SubscribeMarketData(ctx, nil)` and start a goroutine to feed the returned channel into the aggregator(s) — mirroring the pattern in `main.go` (lines 411-429). The `VenueHandler` struct needs access to the aggregator(s). Options: (1) add aggregator references to `VenueHandler`, or (2) use a callback/hook pattern. Option 1 is simpler. Store the goroutine's cancel context so `disconnectVenue` can stop it.

---

### P7-03: ✅ COMPLETE — [CATCH-UP] Volume Aggregation in OHLC Bars

**Service:** Gateway
**Files:**
- `gateway/internal/adapter/provider.go` (modify — add `TickVolume` field to `MarketDataSnapshot` if not present)
- `gateway/internal/adapter/simulated/adapter.go` (modify — populate tick volume from price walk)
- `gateway/internal/marketdata/aggregator.go` (modify — accumulate volume from ticks)
- `gateway/internal/marketdata/aggregator_test.go` (modify — test volume accumulation)
**Dependencies:** None
**Acceptance Criteria:**
- `MarketDataSnapshot` includes a `TickVolume` field (per-tick traded volume, distinct from `Volume24h`)
- Simulated adapter's PriceWalk generates a random tick volume per step (e.g., uniform 1-1000 shares)
- `Aggregator.Ingest` accumulates `TickVolume` into the bar's `Volume` field
- OHLCBar `Volume` is no longer always zero
- Unit test verifies volume accumulation across multiple ticks within one bar

**Architecture Context:**
The `MarketDataSnapshot` struct (in `gateway/internal/adapter/provider.go`) has `Volume24h decimal.Decimal` but no per-tick volume. Add `TickVolume decimal.Decimal` — this represents the volume transacted in the single tick/trade. The `Aggregator.Ingest` method (aggregator.go:50) updates high/low/close but sets `Volume: decimal.Zero` in `newBar`. Change it to add `snap.TickVolume` to `bar.Volume` on each tick. The simulated adapter's price walk goroutine should generate `TickVolume` as a random value (e.g., `decimal.NewFromFloat(rand.Float64() * 1000 + 1)`).

---

### P7-04: ✅ COMPLETE — Regime Detection Module

**Service:** Risk Engine
**Files:**
- `risk-engine/risk_engine/timeseries/regime.py` (create)
- `risk-engine/tests/test_regime.py` (create)
**Dependencies:** None
**Acceptance Criteria:**
- `RegimeDetector` class identifies market regimes (e.g., bull, bear, crisis) from return series
- Uses a Hidden Markov Model (HMM) with 3 states or a simpler threshold-based regime classifier
- Exposes `detect_regime(returns: pd.Series) -> RegimeState` returning the current regime
- Exposes `regime_history(returns: pd.Series) -> pd.Series` returning regime labels for each observation
- `RegimeState` enum: `BULL`, `BEAR`, `CRISIS`
- Can be used by VaR modules to condition risk estimates on current regime
- At least 8 unit tests covering: regime detection on trending-up data, trending-down data, high-volatility data, transition detection, edge cases (empty/short series)

**Architecture Context:**
This module lives in `risk-engine/risk_engine/timeseries/regime.py` alongside `statistics.py` and `covariance.py`. Use `hmmlearn.GaussianHMM` (3-state) if available, with a fallback to a threshold-based classifier using rolling volatility. The three VaR implementations (`historical.py`, `parametric.py`, `monte_carlo.py`) each have a `compute()` method taking `positions` and `returns_matrix`. Regime integration is a future step — for now, `RegimeDetector` is standalone and testable. States map to risk multipliers: BULL=1.0, BEAR=1.3, CRISIS=1.8 (configurable).

---

### P7-05: ✅ COMPLETE — What-If Scenario Analysis REST Router

**Service:** Risk Engine
**Files:**
- `risk-engine/risk_engine/rest/router_scenario.py` (create)
- `risk-engine/risk_engine/main.py` (modify — register scenario router)
- `risk-engine/tests/test_rest_scenario.py` (create)
**Dependencies:** None
**Acceptance Criteria:**
- `POST /api/v1/risk/scenario` accepts a JSON body with hypothetical positions (instrument, side, quantity, price)
- Computes projected VaR (historical + parametric + Monte Carlo) for the portfolio with the hypothetical positions added
- Returns: current VaR, projected VaR, delta, and per-check risk results
- Validates input (reject empty scenarios, unknown instruments gracefully)
- At least 6 tests: valid scenario, empty scenario rejection, single hypothetical position, multiple hypothetical positions, scenario with existing portfolio positions

**Architecture Context:**
Follow the pattern of existing routers (`router_risk.py`, `router_optimizer.py`). Create a `ScenarioDependencies` class similar to `RiskDependencies` — it needs access to the shared `Portfolio`, `HistoricalVaR`, `ParametricVaR`, `MonteCarloVaR`, and `returns_matrix`. The scenario endpoint clones the current portfolio, adds the hypothetical positions, recomputes VaR, and returns the comparison. In `main.py`, register with `app.include_router(scenario_router)` and configure dependencies in the lifespan handler — follow the pattern at lines 38-63.

Request body schema:
```json
{
  "hypothetical_positions": [
    {"instrument_id": "AAPL", "side": "buy", "quantity": "100", "price": "150.00"}
  ]
}
```

Response schema:
```json
{
  "current_var": {"historical": "...", "parametric": "...", "monte_carlo": "..."},
  "projected_var": {"historical": "...", "parametric": "...", "monte_carlo": "..."},
  "var_delta": {"historical": "...", "parametric": "...", "monte_carlo": "..."},
  "hypothetical_positions_added": 1
}
```

---

### P7-06: ✅ COMPLETE — Docker Image Release Workflow

**Service:** Infrastructure / CI
**Files:**
- `.github/workflows/release.yml` (create)
**Dependencies:** None
**Acceptance Criteria:**
- Triggers on GitHub release creation (tag push matching `v*`)
- Builds Docker images for gateway, risk-engine, and dashboard
- Tags images with the release version and `latest`
- Pushes to GitHub Container Registry (ghcr.io)
- Uses multi-stage builds already defined in each service's Dockerfile
- Build matrix: gateway (Go), risk-engine (Python), dashboard (Node)

**Architecture Context:**
The project has three Dockerfiles: `gateway/Dockerfile`, `risk-engine/Dockerfile`, `dashboard/Dockerfile`. The existing `ci.yml` runs on push/PR to main with `actions/checkout@v4`, `actions/setup-go@v5`, etc. The release workflow should use `docker/build-push-action` with GHCR. Follow standard GitHub Actions patterns. Image names: `ghcr.io/${{ github.repository }}/gateway`, `ghcr.io/${{ github.repository }}/risk-engine`, `ghcr.io/${{ github.repository }}/dashboard`.

---

### P7-07: ✅ COMPLETE — E2E Playwright Test — Order Flow

**Service:** Dashboard (E2E)
**Files:**
- `e2e/order-flow.spec.ts` (create)
- `e2e/playwright.config.ts` (create — if not exists)
- `e2e/package.json` (create — if not exists)
**Dependencies:** None (runs against existing services)
**Acceptance Criteria:**
- Test starts with dashboard loaded against a running gateway with simulated exchange
- Completes onboarding: enters passphrase, selects simulated venue, submits credentials
- Submits a market buy order for an instrument (e.g., AAPL)
- Waits for fill to appear in the blotter (status changes to "Filled")
- Navigates to risk dashboard and verifies VaR gauge is displayed with a non-zero value
- Test uses Playwright's auto-waiting and assertion retry for WebSocket-driven updates

**Architecture Context:**
Per architecture doc Section 9.3 test #1: "Browser opens dashboard, completes onboarding with simulated venue, submits a buy order, waits for fill in blotter, navigates to risk dashboard, verifies VaR is displayed." The dashboard runs on port 5173 (Vite dev) or 3000 (Docker). The gateway runs on port 8080. The onboarding flow is in `OnboardingView.tsx` (5 steps: welcome, passphrase, venue choice, credentials, ready). After onboarding, the user lands on `BlotterView.tsx` where `OrderTicket.tsx` allows order submission.

---

### P7-08: ✅ COMPLETE — E2E Playwright Test — Multi-Venue Portfolio

**Service:** Dashboard (E2E)
**Files:**
- `e2e/multi-venue-portfolio.spec.ts` (create)
**Dependencies:** P7-07 (shares Playwright config)
**Acceptance Criteria:**
- Connects simulated equities exchange and simulated crypto exchange
- Submits orders in both venues (e.g., buy AAPL on equities, buy BTC-USD on crypto)
- Navigates to Portfolio view
- Verifies unified position table shows both positions with correct asset class labels
- Verifies exposure breakdown includes both asset classes

**Architecture Context:**
Per architecture doc Section 9.3 test #2: "Connect simulated equities + simulated crypto, submit orders in both, verify unified portfolio view shows both positions with correct asset class labels." The simulated adapter supports multiple instrument types via the seed data. `PortfolioView.tsx` renders `PositionTable.tsx` with instrument details. The venue connect flow is in `LiquidityNetwork.tsx` for connecting additional venues post-onboarding.

---

### P7-09: ✅ COMPLETE — E2E Playwright Test — Order Cancellation

**Service:** Dashboard (E2E)
**Files:**
- `e2e/order-cancellation.spec.ts` (create)
**Dependencies:** P7-07 (shares Playwright config)
**Acceptance Criteria:**
- Submits a limit order far from market price (e.g., buy AAPL at $1.00)
- Verifies order appears in blotter with "Acknowledged" status
- Clicks cancel on the order
- Verifies order status changes to "Canceled"
- Order remains visible in blotter with final status

**Architecture Context:**
Per architecture doc Section 9.3 test #3: "Submit a limit order far from market, verify it appears as 'Acknowledged', click cancel, verify it moves to 'Canceled'." The `OrderTicket.tsx` component supports limit orders (type selector). The `OrderTable.tsx` blotter shows order status from the `orderStore`. Cancel is triggered via the REST API `DELETE /api/v1/orders/{id}` which the order store's `cancelOrder` action calls.

---

## Checklist Cross-Reference

All deferred/catch-up items accounted for:

| Item | Source | Task |
|------|--------|------|
| 5m interval UI selector | Phase 6 review catch-up #1 | P7-01, P7-01a |
| Dynamic market data subscription | Phase 6 review catch-up #2 | P7-02 |
| Volume aggregation | Phase 6 review catch-up #3 | P7-03 |
| `risk_engine/timeseries/regime.py` | Deliverables checklist (deferred) | P7-04 |
| `risk_engine/rest/router_scenario.py` | Deliverables checklist (deferred) | P7-05 |
| `.github/workflows/release.yml` | Deliverables checklist (deferred) | P7-06 |
| E2E: order flow (Playwright) | Deliverables checklist (deferred) | P7-07 |
| E2E: multi-venue portfolio (Playwright) | Deliverables checklist (deferred) | P7-08 |
| E2E: order cancellation (Playwright) | Deliverables checklist (deferred) | P7-09 |

**No remaining unchecked items in the deliverables checklist after this phase.**

---

## Phase 7 Deviations

### Deviation 1: Regime Detection uses threshold-based classifier only (no HMM)
**Architecture Doc Says:** "Use `hmmlearn.GaussianHMM` (3-state) if available, with a fallback to a threshold-based classifier"
**Actual Implementation:** Uses threshold-based classifier only (rolling volatility + rolling returns). No `hmmlearn` dependency added.
**Reason:** The threshold approach is simpler, has no external C dependency (hmmlearn requires a C compiler), and produces deterministic results suitable for unit testing. The HMM approach can be added as an enhancement.
**Impact:** None — the `RegimeDetector` API is identical regardless of internal algorithm. Adding HMM later is backward-compatible.

### Deviation 2: VenueHandler uses variadic aggregator parameters instead of struct injection
**Architecture Doc Says:** "add aggregator references to `VenueHandler`" (option 1)
**Actual Implementation:** Used a `MarketDataIngester` interface with variadic constructor parameters (`NewVenueHandler(credMgr, logger, aggregators...)`) rather than adding bare struct fields.
**Reason:** Using an interface decouples the REST handler from the concrete `marketdata.Aggregator` type, improving testability. Variadic params keep the existing test code working (tests pass zero aggregators).
**Impact:** None — functionally equivalent, existing tests unaffected.

### Deviation 3: Aggregator creation moved to section 13a (before REST router)
**Architecture Doc Says:** Aggregator created in section 14 of main.go.
**Actual Implementation:** Aggregator creation moved to section 13a, before section 13 (REST router), so VenueHandler can reference aggregators at construction time.
**Reason:** VenueHandler needs aggregator references when created. Dependency ordering requires aggregators to exist first.
**Impact:** None — functionally equivalent, startup order is the same.

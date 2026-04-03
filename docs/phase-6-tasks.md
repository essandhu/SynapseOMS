# Phase 6 Tasks — OHLC Candlestick Charting

**Goal:** Add real-time OHLC candlestick charting to the dashboard, wiring the existing market data infrastructure through to a `lightweight-charts` visualization. A user can view live candlestick charts for any instrument with data from connected venues.

**Acceptance Test:** User opens the Blotter or a dedicated Market Data view, selects an instrument (e.g., AAPL on simulated exchange), and sees a live-updating candlestick chart with OHLC bars aggregated from the venue's market data feed. Chart supports 1m and 5m intervals, pans/zooms, and updates in real-time via WebSocket.

**Architecture Doc References:** Section 1 (dashboard/src/components/CandlestickChart.tsx), proto/marketdata/marketdata.proto (OHLCV message), gateway/internal/adapter/provider.go (SubscribeMarketData interface), gateway/internal/ws/ (WebSocket hub)

**Previous Phase Review:** Phase 5 completed all tasks. Final audit identified CandlestickChart.tsx as deferred. Backend infrastructure exists: MarketDataSnapshot type in adapter interface, SubscribeMarketData on all adapters, PriceWalk GBM generator in simulated adapter, Kafka market-data topic, OHLCV proto message. Missing: gateway market data WebSocket stream, OHLC bar aggregation, frontend component.

---

## Tasks

### P6-01: OHLC Bar Aggregator in Gateway ✅ COMPLETE

**Service:** Gateway
**Files:**
- `gateway/internal/marketdata/aggregator.go` (create)
- `gateway/internal/marketdata/aggregator_test.go` (create)
**Dependencies:** None
**Acceptance Criteria:**
- `Aggregator` struct accepts `adapter.MarketDataSnapshot` ticks and emits completed OHLC bars
- Supports configurable intervals (1m, 5m)
- Each bar contains: instrument ID, venue ID, open, high, low, close, volume, period start/end
- Emits partial (in-progress) bar updates at a configurable flush interval (e.g., every 5 seconds) for real-time chart updates
- Correctly handles: first tick opens a new bar, subsequent ticks update high/low/close/volume, period boundary closes bar and opens next
- Thread-safe for concurrent tick ingestion from multiple adapters
- Unit tests cover: single tick, multiple ticks within one bar, bar rollover at period boundary, partial bar flush, concurrent access

**Architecture Context:**
The `adapter.MarketDataSnapshot` struct (in `gateway/internal/adapter/provider.go`) provides: InstrumentID, VenueID, BidPrice, AskPrice, LastPrice, Volume24h, Timestamp. The aggregator should use `LastPrice` as the trade price for OHLC computation. The `OHLCV` proto message (in `proto/marketdata/marketdata.proto`) defines the output schema: instrument_id, open, high, low, close, volume, period_start, period_end.

---

### P6-02: Market Data WebSocket Stream on Gateway ✅ COMPLETE

**Service:** Gateway
**Files:**
- `gateway/internal/ws/server.go` (modify — add HandleMarketData handler)
- `gateway/internal/ws/hub.go` (modify — add StreamMarketData type, BroadcastMarketData method)
- `gateway/cmd/gateway/main.go` (modify — wire market data stream, start aggregator)
**Dependencies:** P6-01
**Acceptance Criteria:**
- New WebSocket endpoint at `/ws/marketdata` streams OHLC bar updates to connected clients
- Hub supports `StreamMarketData` broadcast
- Gateway main.go subscribes to market data from all connected adapters, feeds ticks to the aggregator, and broadcasts completed/partial bars via WebSocket
- Message format: `{"type": "ohlc_update", "data": {"instrumentId": "...", "interval": "1m", "open": "...", "high": "...", "low": "...", "close": "...", "volume": "...", "periodStart": "...", "periodEnd": "...", "complete": true/false}}`
- Existing WebSocket tests continue to pass

**Architecture Context:**
The WebSocket hub (`gateway/internal/ws/hub.go`) uses `StreamType` constants and a `broadcast(stream, data)` method. Add `StreamMarketData StreamType = "marketdata"`. The server (`gateway/internal/ws/server.go`) has `HandleOrders`, `HandlePositions`, etc. — follow the same pattern for `HandleMarketData`. In `main.go`, the adapter market data channels are available after venue connection; wire them to the aggregator after the hub is created.

---

### P6-03: Dashboard Market Data WebSocket Client ✅ COMPLETE

**Service:** Dashboard
**Files:**
- `dashboard/src/api/ws.ts` (modify — add createMarketDataStream)
- `dashboard/src/api/types.ts` (modify — add OHLCUpdate type)
- `dashboard/src/stores/marketDataStore.ts` (create)
- `dashboard/src/stores/marketDataStore.test.ts` (create)
**Dependencies:** P6-02
**Acceptance Criteria:**
- `OHLCUpdate` type matches the gateway WebSocket message format
- `createMarketDataStream()` connects to `/ws/marketdata` with automatic reconnection
- `marketDataStore` (Zustand) maintains a map of instrument → bar array, capped at 500 bars per instrument
- Store exposes `subscribe(instrument, interval)` and `getBars(instrument, interval)` actions
- Partial bar updates replace the last bar in the array; complete bars append
- Unit tests cover: initial state, applying a complete bar, applying a partial bar update, bar cap enforcement

**Architecture Context:**
Follow the patterns in `dashboard/src/api/ws.ts` (ReconnectingWebSocket, message parsing) and `dashboard/src/stores/riskStore.ts` (Zustand store with subscribe/unsubscribe lifecycle). The `OHLCUpdate` type should live in `dashboard/src/api/types.ts` alongside other shared types.

---

### P6-04: CandlestickChart Component ✅ COMPLETE

**Service:** Dashboard
**Files:**
- `dashboard/src/components/CandlestickChart.tsx` (create)
- `dashboard/src/components/CandlestickChart.test.tsx` (create)
**Dependencies:** P6-03
**Acceptance Criteria:**
- Uses `lightweight-charts` library for OHLC rendering
- Props: `instrumentId: string`, `interval: "1m" | "5m"` (default "1m")
- Subscribes to marketDataStore on mount, cleans up on unmount
- Renders candlestick series with the terminal dark theme (bg: #0a0e14, up candle: #00e5a0, down candle: #ff3b5c)
- Handles empty state gracefully (shows "Waiting for market data..." message)
- Auto-scrolls to latest bar, supports pan/zoom
- Updates in real-time as new bars arrive from the store
- Component test verifies: renders without crash, displays empty state message when no data

**Architecture Context:**
The terminal theme tokens are in `dashboard/src/theme/terminal.ts`. Follow existing component patterns (e.g., `MonteCarloPlot.tsx` for chart lifecycle). `lightweight-charts` is TradingView's open-source charting library — use `createChart()` and `addCandlestickSeries()` APIs.

---

### P6-05: Integrate CandlestickChart into BlotterView ✅ COMPLETE

**Service:** Dashboard
**Files:**
- `dashboard/src/views/BlotterView.tsx` (modify — add chart panel)
- `dashboard/src/views/BlotterView.test.tsx` (modify — verify chart renders)
**Dependencies:** P6-04
**Acceptance Criteria:**
- BlotterView shows a collapsible chart panel above or beside the order table
- Chart panel displays a CandlestickChart for the currently selected instrument (from the order ticket or first instrument in the blotter)
- Instrument selection in the order ticket updates the chart
- Chart panel can be toggled open/closed
- Existing BlotterView tests continue to pass
- New test verifies chart panel renders when toggled open

**Architecture Context:**
`BlotterView.tsx` currently renders an order table (AG Grid) and an OrderTicket panel. The chart panel should integrate cleanly — either as a resizable top panel or a tabbed side panel. Use the terminal theme for styling.

---

### P6-06: Wire Simulated Adapter Market Data to Aggregator ✅ COMPLETE

**Service:** Gateway
**Files:**
- `gateway/internal/adapter/simulated/adapter.go` (modify — push snapshots from price walk to mdCh)
**Dependencies:** P6-01, P6-02
**Acceptance Criteria:**
- Simulated adapter's PriceWalk ticks are forwarded as MarketDataSnapshot events to the mdCh channel
- Each price walk step produces a snapshot with LastPrice, BidPrice (last - spread/2), AskPrice (last + spread/2)
- Market data flows through aggregator → WebSocket → dashboard without requiring real venue credentials
- Integration test or manual verification: start with simulated exchange, observe OHLC bars updating in real-time

**Architecture Context:**
The simulated adapter already has `PriceWalk` ticking at configurable intervals and an `mdCh` channel created in `SubscribeMarketData()`. The missing piece is a goroutine that reads price walk ticks and pushes `MarketDataSnapshot` structs to `mdCh`. This should start when `SubscribeMarketData` is called (or when the adapter connects).

---

## Phase 6 Deviations

### Deviation 1: lightweight-charts v5 API
**Architecture Doc Says:** Use `addCandlestickSeries()` method on chart instance.
**Actual Implementation:** Uses `chart.addSeries(CandlestickSeries, options)` — the v5 API replaced method-per-series-type with a generic `addSeries(definition, options)` pattern.
**Reason:** `lightweight-charts` v5.1.0 was installed (latest), which uses the new API.
**Impact:** None — functionally identical. The test mock was updated accordingly.

### Deviation 2: Market data subscription happens at startup, not on-demand
**Architecture Doc Says:** "wire them to the aggregator after the hub is created" (implying dynamic wiring per adapter connection).
**Actual Implementation:** Market data is subscribed once at gateway startup for all currently-connected adapters. New adapter connections after startup won't auto-subscribe.
**Reason:** Simpler initial implementation. Dynamic subscription on adapter connect/disconnect would require refactoring the venue connection REST handler.
**Impact:** Low — in typical use, the simulated adapter connects at startup. For real venues connected later via the UI, a gateway restart would be needed to see their chart data. Can be enhanced in a future phase.

### Deviation 3: /ws/risk removal included in worktree
**Architecture Doc Says:** Dashboard uses REST polling for risk data (fixed in final audit).
**Actual Implementation:** The worktree branch includes the `/ws/risk` removal changes (riskStore.ts, ws.ts, App.tsx) since the worktree was based on the pre-fix commit.
**Reason:** These changes were already made in the main working directory but hadn't been committed. The worktree needed the same fixes to pass TypeScript checks.
**Impact:** None — these changes match what was already done in the main workspace.

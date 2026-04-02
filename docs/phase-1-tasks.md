# Phase 1 Tasks — Single-Venue Trading Loop

**Goal:** A user can submit an order via the UI, see it route to the simulated venue, receive a fill, and see their position update.

**Acceptance Test:** User opens dashboard at `localhost:3000`, submits a market buy for 10 shares of AAPL on the simulated exchange, sees the order appear in the blotter as "New" → "Acknowledged" → "Filled", and sees a position of 10 AAPL appear in the position table.

**Architecture Doc References:** Sections 1 (directory structure), 3.1 (domain entities), 4A (Order Gateway), 4C (Frontend Dashboard), 7.1 (Docker Compose), 7.3 (Structured Logging), 8 Phase 1 (roadmap), 9 (Testing Strategy)

**Previous Phase Review:** N/A (first phase)

---

## Tasks

### P1-01: Gateway Go Module + Project Scaffolding

**Service:** Gateway
**Files:**
- `gateway/go.mod`
- `gateway/cmd/gateway/main.go` (minimal stub — full wiring in P1-16)
- `gateway/Dockerfile`
- Directory skeleton: `gateway/internal/{domain,adapter,pipeline,rest,ws,store,logging,apperror}/`
**Dependencies:** None
**Acceptance Criteria:**
- `go build ./cmd/gateway` succeeds
- `go vet ./...` passes
- Binary starts, listens on configured port, and shuts down cleanly on SIGTERM
- Dockerfile builds a working image

**Architecture Context:**
Technology stack (Section 4A):

| Component | Library | Version |
|-----------|---------|---------|
| HTTP/REST | `net/http` + `chi` router | chi v5 |
| WebSocket | `gorilla/websocket` | v1.5 |
| PostgreSQL | `github.com/jackc/pgx/v5` | v5.6 |
| Redis | `github.com/redis/go-redis/v9` | v9.5 |
| Decimal | `github.com/shopspring/decimal` | v1.4 |
| Config | `github.com/spf13/viper` | v1.19 |
| Logging | `log/slog` (stdlib structured logging) | Go 1.22 |
| Metrics | `github.com/prometheus/client_golang` | v1.19 |
| Protobuf | `google.golang.org/protobuf` | v1.34 |

Module path: `github.com/synapse-oms/gateway`. Multi-stage Dockerfile: Go 1.22 builder → `gcr.io/distroless/static-debian12`. Config via viper from env vars: `PORT`, `POSTGRES_URL`, `REDIS_URL`. Graceful shutdown: listen SIGTERM/SIGINT, cancel root context, drain within 5s.

---

### P1-02: Gateway Domain Model (Order State Machine, Fill, Position, Instrument)

**Service:** Gateway
**Files:**
- `gateway/internal/domain/order.go`
- `gateway/internal/domain/fill.go`
- `gateway/internal/domain/instrument.go`
- `gateway/internal/domain/position.go`
- `gateway/internal/domain/order_test.go`
- `gateway/internal/domain/position_test.go`
**Dependencies:** P1-01
**Acceptance Criteria:**
- All domain types compile and match architecture Section 3.1 field-for-field
- `Order.ApplyTransition` enforces the exact state machine
- `Order.ApplyFill` correctly computes VWAP average price
- `Position.ApplyFill` correctly adjusts quantity and P&L
- `go test ./internal/domain/...` passes with minimum 10 test cases

**Architecture Context:**

**Order** (Section 3.1):
```go
type OrderID string
type OrderSide int   // SideBuy, SideSell
type OrderType int   // OrderTypeMarket, OrderTypeLimit, OrderTypeStopLimit
type OrderStatus int // OrderStatusNew, OrderStatusAcknowledged, OrderStatusPartiallyFilled,
                     // OrderStatusFilled, OrderStatusCanceled, OrderStatusRejected

type Order struct {
    ID              OrderID
    ClientOrderID   string
    InstrumentID    string
    Side            OrderSide
    Type            OrderType
    Quantity        decimal.Decimal
    Price           decimal.Decimal
    FilledQuantity  decimal.Decimal
    AveragePrice    decimal.Decimal
    Status          OrderStatus
    VenueID         string
    AssetClass      AssetClass
    SettlementCycle SettlementCycle
    CreatedAt       time.Time
    UpdatedAt       time.Time
    Fills           []Fill
}

func (o *Order) ApplyTransition(newStatus OrderStatus) error // returns error on invalid
func (o *Order) ApplyFill(fill Fill) error                   // updates qty, VWAP, transitions
```

**State machine transitions:**
```
New → Acknowledged → PartiallyFilled → Filled
New → Rejected
Acknowledged → Canceled
Acknowledged → Rejected
PartiallyFilled → Filled
PartiallyFilled → Canceled (partial cancel)
```

**Fill** (Section 3.1):
```go
type Fill struct {
    ID          string
    OrderID     OrderID
    VenueID     string
    Quantity    decimal.Decimal
    Price       decimal.Decimal
    Fee         decimal.Decimal
    FeeAsset    string
    FeeModel    FeeModel
    Liquidity   LiquidityType  // Maker, Taker, Internal
    Timestamp   time.Time
    VenueExecID string
}
```

**Instrument** (Section 3.1):
```go
type AssetClass int      // Equity, Crypto, TokenizedSecurity, Future, Option
type SettlementCycle int // T0, T1, T2

type Instrument struct {
    ID              string
    Symbol          string
    Name            string
    AssetClass      AssetClass
    QuoteCurrency   string
    BaseCurrency    string
    TickSize        decimal.Decimal
    LotSize         decimal.Decimal
    SettlementCycle SettlementCycle
    TradingHours    TradingSchedule
    Venues          []string
    MarginRequired  decimal.Decimal
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

**Position** (Section 3.1):
```go
type Position struct {
    InstrumentID      string
    VenueID           string
    Quantity          decimal.Decimal // Signed: positive = long, negative = short
    AverageCost       decimal.Decimal
    MarketPrice       decimal.Decimal
    UnrealizedPnL     decimal.Decimal
    RealizedPnL       decimal.Decimal
    UnsettledQuantity decimal.Decimal
    SettledQuantity   decimal.Decimal
    AssetClass        AssetClass
    QuoteCurrency     string
    UpdatedAt         time.Time
}

func (p *Position) ApplyFill(fill Fill, side OrderSide) error
```

**Tests** (Section 9.1): Table-driven tests for `ApplyTransition` covering all valid and invalid transitions. Tests for `ApplyFill` covering partial fills, full fills, overfill rejection. Position tests for opening buy, adding to position, closing (sell fills update realized P&L).

---

### P1-03: Cross-Cutting — Structured JSON Logging with Correlation IDs

**Service:** Gateway
**Files:**
- `gateway/internal/logging/logger.go`
- `gateway/internal/logging/logger_test.go`
**Dependencies:** P1-01
**Acceptance Criteria:**
- All log output is valid JSON with fields: `timestamp` (RFC 3339), `level`, `service`, `component`, `correlation_id`
- Correlation ID propagates from HTTP middleware through context to all downstream log calls
- Missing correlation ID auto-generates a UUID
- Tests pass

**Architecture Context:**

Section 7.3 — Structured Logging. All services emit JSON-structured logs:
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

Use Go stdlib `log/slog` with JSON handler. Implement:
- `WithCorrelationID(ctx, id) context.Context`
- `FromContext(ctx) *slog.Logger` — returns logger with `correlation_id` and `service=gateway` pre-populated
- `CorrelationIDMiddleware(next http.Handler) http.Handler` — extracts `X-Correlation-ID` header or generates UUID, injects into context

Correlation ID propagation: generated at order submission (REST handler), passed through pipeline stages via context, included in WebSocket messages to frontend.

---

### P1-04: Cross-Cutting — Error Handling Patterns

**Service:** Gateway
**Files:**
- `gateway/internal/apperror/errors.go`
**Dependencies:** P1-01
**Acceptance Criteria:**
- All error types implement `error` interface
- `errors.Is` / `errors.As` work correctly with `AppError`
- `WriteError` produces consistent JSON error responses with correct HTTP status codes
- Domain functions use early-return pattern (no nested if/else)

**Architecture Context:**

Typed sentinel errors:
- `ErrInvalidTransition` — invalid order state transition
- `ErrOrderNotFound`, `ErrInstrumentNotFound`, `ErrPositionNotFound`
- `ErrInvalidQuantity` — zero or negative quantity
- `ErrInvalidPrice` — negative price, or zero for limit orders
- `ErrDuplicateClientOrderID` — idempotency violation

`AppError` struct: Code (string, e.g. `"INVALID_TRANSITION"`), Message (human-readable), HTTPStatus (int), supports `Unwrap`.

REST error response helper: `WriteError(w, err)` writes `{"error": {"code": "...", "message": "..."}}` with correct HTTP status.

---

### P1-05: Cross-Cutting — Makefile

**Service:** Root (cross-service)
**Files:**
- `Makefile`
**Dependencies:** None
**Acceptance Criteria:**
- `make help` prints all targets with descriptions
- `make build` builds gateway binary and dashboard
- `make test` runs both gateway and dashboard test suites
- `make lint` runs `go vet` and `tsc --noEmit`
- `make proto`, `make docker`, `make up`, `make down`, `make seed`, `make clean` all function correctly

**Architecture Context:**

Top-level Makefile targets:
- `make build` — `cd gateway && go build ./cmd/gateway` + `cd dashboard && npm run build`
- `make test` — `cd gateway && go test ./...` + `cd dashboard && npm test`
- `make lint` — `go vet ./...` + `tsc --noEmit`
- `make proto` — `scripts/proto-gen.sh`
- `make docker` — `docker compose -f deploy/docker-compose.yml build`
- `make up` / `make down` — compose up/down
- `make seed` — seed script
- `make clean` — remove build artifacts
- `make help` — list targets (parsed from `## description` comments)

All compose commands use `-f deploy/docker-compose.yml`. `.PHONY` on all targets.

---

### P1-06: Dashboard Project Scaffolding

**Service:** Dashboard
**Files:**
- `dashboard/package.json`
- `dashboard/tsconfig.json`
- `dashboard/vite.config.ts`
- `dashboard/index.html`
- `dashboard/src/main.tsx`
- `dashboard/src/App.tsx`
- `dashboard/src/api/types.ts`
- `dashboard/src/api/rest.ts`
- `dashboard/src/api/ws.ts`
- `dashboard/src/stores/orderStore.ts` (interface + stubs)
- `dashboard/src/stores/positionStore.ts` (interface + stubs)
- `dashboard/src/theme/terminal.ts`
- `dashboard/Dockerfile`
**Dependencies:** None
**Acceptance Criteria:**
- `npm install && npm run build` succeeds with zero errors
- `npm run dev` starts dev server on port 3000
- TypeScript strict mode enabled, `tsc --noEmit` passes
- Tailwind config includes all terminal theme colors
- `types.ts` interfaces match architecture Section 4C exactly
- Dockerfile builds and serves on port 3000

**Architecture Context:**

Technology stack (Section 4C):

| Component | Library | Version |
|-----------|---------|---------|
| Framework | React | 19.x |
| Build | Vite | 6.x |
| Language | TypeScript | 5.5+ |
| State | Zustand | 5.x |
| Data Grid | AG Grid Community | 32.x |
| HTTP client | ky | 1.x |
| Router | React Router | 7.x |
| CSS | Tailwind CSS | 4.x |
| UI primitives | Radix UI | latest |
| Icons | Lucide React | latest |
| WebSocket | reconnecting-websocket | 1.x |

**Core TypeScript types** (Section 4C — `dashboard/src/api/types.ts`):
```typescript
type AssetClass = "equity" | "crypto" | "tokenized_security" | "future" | "option";
type SettlementCycle = "T0" | "T1" | "T2";
type OrderStatus = "new" | "acknowledged" | "partially_filled" | "filled" | "canceled" | "rejected";
type OrderSide = "buy" | "sell";
type OrderType = "market" | "limit" | "stop_limit";

interface Order {
  id: string; clientOrderId: string; instrumentId: string; side: OrderSide;
  type: OrderType; quantity: string; price: string; filledQuantity: string;
  averagePrice: string; status: OrderStatus; venueId: string;
  assetClass: AssetClass; createdAt: string; updatedAt: string; fills: Fill[];
}

interface Fill {
  id: string; orderId: string; venueId: string; quantity: string;
  price: string; fee: string; feeAsset: string;
  liquidity: "maker" | "taker" | "internal"; timestamp: string;
}

interface Position {
  instrumentId: string; venueId: string; quantity: string;
  averageCost: string; marketPrice: string; unrealizedPnl: string;
  realizedPnl: string; unsettledQuantity: string; assetClass: AssetClass;
  quoteCurrency: string;
}

interface Venue {
  id: string; name: string; type: "exchange" | "dark_pool" | "simulated" | "tokenized";
  status: "connected" | "disconnected" | "degraded" | "authentication";
  supportedAssets: AssetClass[]; latencyP50Ms: number; latencyP99Ms: number;
  fillRate: number; lastHeartbeat: string; hasCredentials: boolean;
}
```

**Theme** (Section 4C — `dashboard/src/theme/terminal.ts`):
```typescript
export const terminalTheme = {
  colors: {
    bg: { primary: "#0a0e17", secondary: "#111827", tertiary: "#1f2937" },
    text: { primary: "#e5e7eb", secondary: "#9ca3af", muted: "#6b7280" },
    accent: { blue: "#3b82f6", green: "#22c55e", red: "#ef4444", yellow: "#eab308", purple: "#a855f7" },
    border: "#374151",
  },
  fonts: { mono: "'JetBrains Mono', 'Fira Code', monospace", sans: "'Inter', system-ui, sans-serif" },
};
```

**WebSocket client** (Section 4C — `dashboard/src/api/ws.ts`):
```typescript
import ReconnectingWebSocket from "reconnecting-websocket";
const BASE_WS = import.meta.env.VITE_WS_URL || "ws://localhost:8080";
export function createOrderStream(onUpdate: (update: OrderUpdate) => void): ReconnectingWebSocket { ... }
export function createPositionStream(onUpdate: (update: PositionUpdate) => void): ReconnectingWebSocket { ... }
```

**REST client** — thin wrapper around `ky` with `VITE_API_URL` base.

**Zustand stores** — `orderStore` interface per Section 4C: `orders: Map<string, Order>`, `activeOrders()`, `submitOrder()`, `cancelOrder()`, `applyUpdate()`. `positionStore`: `positions: Map<string, Position>`, `applyUpdate()`.

Vite dev server proxies `/api` and `/ws` to `localhost:8080`. Env vars: `VITE_API_URL`, `VITE_WS_URL`.

Dockerfile: Node 22 build stage → nginx static serve on port 3000.

---

### P1-07: Simulated Exchange Adapter (Matching Engine + Price Walk)

**Service:** Gateway
**Files:**
- `gateway/internal/adapter/provider.go`
- `gateway/internal/adapter/registry.go`
- `gateway/internal/adapter/simulated/adapter.go`
- `gateway/internal/adapter/simulated/matching_engine.go`
- `gateway/internal/adapter/simulated/price_walk.go`
- `gateway/internal/adapter/simulated/matching_engine_test.go`
**Dependencies:** P1-01, P1-02, P1-03, P1-04
**Acceptance Criteria:**
- Price walk produces GBM prices (variance scales with time, no mean reversion)
- Market orders fill immediately within 5bps of synthetic price
- Limit orders queue and fill when price crosses
- Fee model: $0.005/share equity, 0.1% maker/taker crypto
- All six instruments pre-loaded with correct asset classes
- `FillFeed()` channel receives fills asynchronously
- Tests pass

**Architecture Context:**

**LiquidityProvider interface** (Section 4A — Phase 1 subset, no credentials):
```go
type LiquidityProvider interface {
    VenueID() string
    VenueName() string
    SupportedInstruments() ([]Instrument, error)
    Connect(ctx context.Context) error
    Disconnect(ctx context.Context) error
    Status() VenueStatus
    SubmitOrder(ctx context.Context, order *Order) (*VenueAck, error)
    CancelOrder(ctx context.Context, orderID OrderID, venueOrderID string) error
    FillFeed() <-chan Fill
}
```

`VenueAck` struct: VenueOrderID, ReceivedAt. `VenueStatus` enum: Connected, Disconnected.

**Adapter registration** (Section 4A):
```go
type AdapterFactory func(config map[string]string) LiquidityProvider
var registry = map[string]AdapterFactory{}
func Register(venueType string, factory AdapterFactory)
func Get(venueType string) (AdapterFactory, bool)
// init() registers "simulated" adapter
```

**Simulated exchange** (Section 4A):
```go
type MatchingEngine struct {
    books      map[string]*OrderBook
    priceWalks map[string]*PriceWalk
}

type PriceWalk struct {
    currentPrice decimal.Decimal
    volatility   float64       // 0.30 equity, 0.80 crypto
    drift        float64
    ticker       *time.Ticker  // 100ms default
}
```

Behavior:
- Pre-loads: AAPL (~$185), MSFT (~$420), GOOG (~$175), BTC-USD (~$65000), ETH-USD (~$3500), SOL-USD (~$140)
- GBM price generation: `dS = S * (mu*dt + sigma*sqrt(dt)*Z)` where Z ~ N(0,1)
- Market orders: fill immediately at synthetic price + random slippage (0–5bps)
- Limit orders: fill when synthetic price crosses limit
- Partial fills: orders >10% synthetic volume get partial fills
- Fees: $0.005/share equity-like, 0.1% maker/taker crypto-like
- Fills pushed to `FillFeed()` channel

---

### P1-08: PostgreSQL Schema + Migrations + Repository Layer

**Service:** Gateway
**Files:**
- `gateway/migrations/001_initial_schema.up.sql`
- `gateway/migrations/001_initial_schema.down.sql`
- `gateway/internal/store/postgres.go`
- `gateway/internal/store/order_repo.go`
- `gateway/internal/store/fill_repo.go`
- `gateway/internal/store/position_repo.go`
- `gateway/internal/store/instrument_repo.go`
**Dependencies:** P1-01, P1-02
**Acceptance Criteria:**
- Migration applies cleanly to fresh PostgreSQL 16
- Down migration reverses cleanly
- All domain types round-trip through DB without precision loss (NUMERIC for decimals)
- `client_order_id` uniqueness enforced at DB level
- Indexes on `orders(status)`, `orders(instrument_id)`, `orders(created_at)`, `fills(order_id)`
- All repository methods use parameterized queries (no SQL injection)

**Architecture Context:**

Schema derived from Section 3.1 entities and Section 3.2 ownership matrix:

**instruments** table: `id TEXT PK`, `symbol TEXT`, `name TEXT`, `asset_class TEXT`, `quote_currency TEXT DEFAULT 'USD'`, `base_currency TEXT`, `tick_size NUMERIC`, `lot_size NUMERIC`, `settlement_cycle TEXT`, `trading_hours JSONB`, `venues TEXT[]`, `margin_required NUMERIC DEFAULT 0`, `created_at TIMESTAMPTZ DEFAULT NOW()`

**orders** table: `id TEXT PK`, `client_order_id TEXT UNIQUE`, `instrument_id TEXT FK→instruments`, `side TEXT`, `type TEXT`, `quantity NUMERIC`, `price NUMERIC DEFAULT 0`, `filled_quantity NUMERIC DEFAULT 0`, `average_price NUMERIC DEFAULT 0`, `status TEXT DEFAULT 'new'`, `venue_id TEXT`, `asset_class TEXT`, `settlement_cycle TEXT`, `created_at TIMESTAMPTZ DEFAULT NOW()`, `updated_at TIMESTAMPTZ DEFAULT NOW()`. Indexes: `status`, `instrument_id`, `created_at DESC`.

**fills** table: `id TEXT PK`, `order_id TEXT FK→orders`, `venue_id TEXT`, `quantity NUMERIC`, `price NUMERIC`, `fee NUMERIC DEFAULT 0`, `fee_asset TEXT DEFAULT 'USD'`, `liquidity TEXT`, `venue_exec_id TEXT`, `timestamp TIMESTAMPTZ DEFAULT NOW()`. Index: `order_id`.

**positions** table: `instrument_id TEXT FK→instruments`, `venue_id TEXT`, composite PK `(instrument_id, venue_id)`, `quantity NUMERIC DEFAULT 0`, `average_cost NUMERIC`, `market_price NUMERIC`, `unrealized_pnl NUMERIC`, `realized_pnl NUMERIC`, `unsettled_quantity NUMERIC`, `settled_quantity NUMERIC`, `asset_class TEXT`, `quote_currency TEXT DEFAULT 'USD'`, `updated_at TIMESTAMPTZ DEFAULT NOW()`.

Repository layer uses `pgx/v5/pgxpool`. `NewPostgresStore(pool) *PostgresStore`. `RunMigrations(ctx) error` applies SQL files in order. Repos: `CreateOrder`, `GetOrder`, `ListOrders` (filter by status/instrument), `UpdateOrder`, `CreateFill`, `ListFillsByOrder`, `UpsertPosition` (INSERT ON CONFLICT), `GetPosition`, `ListPositions`, `UpsertInstrument`, `GetInstrument`, `ListInstruments`.

---

### P1-09: Order Processing Pipeline

**Service:** Gateway
**Files:**
- `gateway/internal/pipeline/pipeline.go`
- `gateway/internal/pipeline/notifier.go`
- `gateway/internal/pipeline/pipeline_test.go`
**Dependencies:** P1-02, P1-03, P1-04, P1-07, P1-08
**Acceptance Criteria:**
- Market order flows through entire pipeline: submit → route → fill → position update
- Order state transitions follow the state machine from P1-02
- Position correctly accumulates across multiple fills
- Pipeline shuts down cleanly within 5 seconds when context is canceled
- No goroutine leaks
- All operations persisted to PostgreSQL via store layer
- Tests pass

**Architecture Context:**

Section 4A concurrency model — Phase 1 simplified pipeline (no risk check, no Kafka, single venue):

```
REST Input → Intake Chan (buffered 10,000) → Router (direct to simulated) → Venue Dispatch → Fill Collector → Notifier
```

```go
func NewPipeline(store *PostgresStore, venue LiquidityProvider, notifier Notifier) *Pipeline

func (p *Pipeline) Start(ctx context.Context)  // launches goroutines
func (p *Pipeline) Submit(ctx context.Context, order *Order) error  // validates, generates UUID, pushes to intake
```

Goroutines:
1. **Router** — reads from intake, sets VenueID="simulated", persists order as New, forwards to venue dispatch
2. **Venue dispatch** — calls `venue.SubmitOrder()`, updates order to Acknowledged, persists
3. **Fill collector** — reads from `venue.FillFeed()`, calls `order.ApplyFill()`, updates position via store, transitions order status, persists, sends to notifier

**Notifier** interface:
```go
type Notifier interface {
    NotifyOrderUpdate(order *Order)
    NotifyPositionUpdate(position *Position)
}
```

Context-based cancellation: all goroutines respect `ctx.Done()`. Main process cancels root context on SIGTERM, goroutines drain within 5s.

---

### P1-10: REST API

**Service:** Gateway
**Files:**
- `gateway/internal/rest/router.go`
- `gateway/internal/rest/handler_order.go`
- `gateway/internal/rest/handler_position.go`
- `gateway/internal/rest/handler_instrument.go`
- `gateway/internal/rest/handler_order_test.go`
**Dependencies:** P1-01, P1-03, P1-04, P1-08, P1-09
**Acceptance Criteria:**
- All Phase 1 REST endpoints respond with correct status codes and JSON
- Request validation returns 400 with structured error JSON
- CORS headers allow `localhost:3000` origin
- Correlation ID middleware on every request
- Decimal values serialized as strings in JSON (not floats)
- Tests pass

**Architecture Context:**

Section 4A REST API endpoints (Phase 1 subset):

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/orders` | Submit new order |
| `DELETE` | `/api/v1/orders/{id}` | Cancel order |
| `GET` | `/api/v1/orders` | List orders (filter: status, instrument_id, limit, offset) |
| `GET` | `/api/v1/orders/{id}` | Get order detail with fills |
| `GET` | `/api/v1/positions` | List all positions |
| `GET` | `/api/v1/positions/{instrument_id}` | Get position for instrument |
| `GET` | `/api/v1/instruments` | List available instruments |
| `GET` | `/api/v1/health` | Health check → `{"status": "ok"}` |

Chi router with middleware: correlation ID (P1-03), request logging, panic recovery, CORS (allow `localhost:3000`).

**POST `/api/v1/orders`** request body:
```json
{
  "instrument_id": "AAPL",
  "side": "buy",
  "type": "market",
  "quantity": "10",
  "price": "0",
  "client_order_id": "optional-idempotency-key"
}
```

Validation: quantity must be positive, side must be buy/sell, type must be market/limit, instrument must exist, limit price required for limit orders. Returns 201 with created order. Errors: 400 with `{"error": {"code": "...", "message": "..."}}`.

---

### P1-11: WebSocket Server

**Service:** Gateway
**Files:**
- `gateway/internal/ws/hub.go`
- `gateway/internal/ws/server.go`
- `gateway/internal/ws/server_test.go`
**Dependencies:** P1-01, P1-03, P1-09
**Acceptance Criteria:**
- WebSocket upgrade works at `/ws/orders` and `/ws/positions`
- Order status changes broadcast to all connected `/ws/orders` clients
- Position updates broadcast to all connected `/ws/positions` clients
- JSON message format matches dashboard expectations (Section 4C)
- Stale connections cleaned up via ping/pong (30s interval)
- No goroutine leaks when clients disconnect
- Tests pass

**Architecture Context:**

Section 4A WebSocket streams:

| Endpoint | Payload | Rate |
|----------|---------|------|
| `/ws/orders` | Order status changes, new fills | Per event |
| `/ws/positions` | Position updates, P&L ticks | 100ms throttled |

**Hub** manages connected clients per stream type. Implements `Notifier` interface from P1-09 — receives `NotifyOrderUpdate` and `NotifyPositionUpdate` calls from the pipeline fill collector. Fan-out broadcasts each update to all subscribed clients.

Message format:
```json
{
  "type": "order_update",
  "data": {
    "id": "ord-xxx",
    "status": "filled",
    "filledQuantity": "10",
    "averagePrice": "185.23"
  }
}
```

Uses `gorilla/websocket`. Ping/pong at 30s interval to detect stale connections. Client cleanup on disconnect.

---

### P1-12: Dashboard Terminal Layout Shell

**Service:** Dashboard
**Files:**
- `dashboard/src/components/TerminalLayout.tsx`
- Updated `dashboard/src/App.tsx` (wrap routes in layout)
**Dependencies:** P1-06
**Acceptance Criteria:**
- Dark theme applied globally — no white flashes
- Navigation tabs switch between Blotter / Portfolio views
- Theme colors match architecture Section 4C exactly
- JetBrains Mono font loaded and applied to data elements
- Layout stable at 1280px, 1440px, 1920px widths
- Bottom status bar with placeholder connection indicator

**Architecture Context:**

Section 4C View Descriptions + Theme:

Full-viewport layout with dark background (`#0a0e17`). Top bar: "SynapseOMS" branding (mono font), navigation tabs (Phase 1: Blotter, Portfolio — others added later). Active tab: accent blue (`#3b82f6`) underline. Main content area renders active route's view. Bottom status bar: connection indicator (green dot = WS connected, red = disconnected).

Fonts: JetBrains Mono for data/numbers, Inter for labels/UI text. Responsive at 1280px+ (trading terminal, not mobile-first).

React Router navigation: `/` → Blotter, `/portfolio` → Portfolio.

---

### P1-13: Dashboard Order Ticket Component

**Service:** Dashboard
**Files:**
- `dashboard/src/components/OrderTicket.tsx`
- `dashboard/src/components/OrderTicket.test.tsx`
**Dependencies:** P1-06, P1-12
**Acceptance Criteria:**
- All form fields present: instrument picker, side toggle, type selector, quantity, price (conditional)
- Buy (green `#22c55e`) / Sell (red `#ef4444`) visually distinct
- Price field hidden for market orders, shown for limit
- Submit calls `orderStore.submitOrder` with correct shape
- Form clears after successful submission
- Validation prevents empty quantity or missing instrument
- Component tests pass

**Architecture Context:**

Section 4C — Order ticket panel (slide-out from blotter):
- Instrument picker: dropdown/combobox with search (loads from `GET /api/v1/instruments`)
- Side toggle: Buy (green) / Sell (red) — prominent buttons
- Order type: Market / Limit (segmented control)
- Quantity input: numeric, lot-size aware
- Price input: shown only for Limit, hidden for Market
- Submit → calls `orderStore.submitOrder()` which POSTs to `/api/v1/orders`
- Loading state during submission, disabled while in-flight
- Error display inline, form resets on success

Section 9.1 test example:
```typescript
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

---

### P1-14: Dashboard Blotter View with AG Grid

**Service:** Dashboard
**Files:**
- `dashboard/src/views/BlotterView.tsx`
- `dashboard/src/components/OrderTable.tsx`
- `dashboard/src/stores/orderStore.ts` (complete implementation)
**Dependencies:** P1-06, P1-12, P1-13
**Acceptance Criteria:**
- AG Grid renders with all specified columns
- AG Grid dark theme matches terminal theme
- Order submission from OrderTicket appears in grid as "New"
- WebSocket updates transition order status in-place (New → Acknowledged → Filled visible)
- Side column color-coded green/red
- Status column uses colored badges
- Cancel button on active orders, hidden on terminal
- Status filter works (Active/All/Filled/Canceled)
- Decimal values display correctly

**Architecture Context:**

Section 4C — Unified Blotter (BlotterView.tsx):

AG Grid columns: Time, Instrument, Side (green/red), Type, Qty, Price (or "MKT"), Filled, Avg Price, Status (badge: green=Filled, yellow=Acknowledged/PartiallyFilled, red=Rejected/Canceled, blue=New), Venue, Actions (cancel for active).

Streaming: use AG Grid `applyTransaction()` for efficient delta updates when `orderStore.applyUpdate` fires. Filters: status (Active/All/Filled/Canceled).

**orderStore** complete implementation (Section 4C):
```typescript
export const useOrderStore = create<OrderStore>((set, get) => ({
  orders: new Map(),
  activeOrders: () => {
    const terminal = new Set(["filled", "canceled", "rejected"]);
    return [...get().orders.values()].filter(o => !terminal.has(o.status));
  },
  submitOrder: async (req) => {
    const order = await api.post<Order>("/api/v1/orders", req);
    set(state => { const next = new Map(state.orders); next.set(order.id, order); return { orders: next }; });
    return order;
  },
  cancelOrder: async (orderId) => { await api.delete(`/api/v1/orders/${orderId}`); },
  applyUpdate: (update) => {
    set(state => { const next = new Map(state.orders); const existing = next.get(update.orderId);
      if (existing) { next.set(update.orderId, { ...existing, ...update }); }
      return { orders: next }; });
  },
}));
```

Wire WebSocket: connect `createOrderStream` → pipe to `orderStore.applyUpdate`. Include OrderTicket above/beside the grid.

---

### P1-15: Dashboard Position Table

**Service:** Dashboard
**Files:**
- `dashboard/src/components/PositionTable.tsx`
- `dashboard/src/views/PortfolioView.tsx`
- `dashboard/src/stores/positionStore.ts` (complete implementation)
**Dependencies:** P1-06, P1-12
**Acceptance Criteria:**
- Position table renders columns: Instrument, Venue, Qty, Avg Cost, Market Price, Unrealized P&L, Realized P&L, Asset Class
- Positions update in real-time via WebSocket
- P&L color-coded (green positive, red negative)
- Quantity displays sign
- Dark theme applied
- After filling an order in blotter, position appears here
- Decimal values display correctly

**Architecture Context:**

Section 4C — Portfolio View (Phase 1 subset: position table only, no summary cards or charts):

Table columns: Instrument, Venue, Qty (signed, green positive, red negative), Avg Cost, Market Price, Unrealized P&L (color-coded), Realized P&L, Asset Class.

**positionStore** implementation:
- `positions: Map<string, Position>` keyed by `${instrumentId}-${venueId}`
- `applyUpdate(update)` — merge from WebSocket
- Initial load: `GET /api/v1/positions` on mount

Wire `createPositionStream` → `positionStore.applyUpdate`.

---

### P1-16: Gateway Startup Wiring

**Service:** Gateway
**Files:**
- `gateway/cmd/gateway/main.go` (complete implementation)
**Dependencies:** P1-07, P1-08, P1-09, P1-10, P1-11
**Acceptance Criteria:**
- `go run ./cmd/gateway` starts successfully with PostgreSQL and Redis available
- All components initialized in correct dependency order
- `GET /api/v1/health` returns 200
- SIGTERM triggers graceful shutdown — HTTP stops, pipeline drains, connections close
- Startup logs structured JSON showing each initialization step
- Instruments auto-seeded on first startup (empty instruments table)

**Architecture Context:**

Section 4A + 7.4 — Startup sequence:
1. Load config via viper (env vars: PORT, POSTGRES_URL, REDIS_URL)
2. Initialize slog JSON handler (P1-03)
3. Connect to PostgreSQL (pgxpool), run migrations (P1-08)
4. Connect to Redis
5. Seed instruments if table empty (P1-17)
6. Initialize simulated adapter (P1-07), call Connect()
7. Initialize WebSocket hub (P1-11)
8. Initialize pipeline with store + adapter + hub as notifier (P1-09)
9. Start pipeline goroutines
10. Initialize REST router (P1-10), mount WS upgrade endpoints
11. Start HTTP server on :8080
12. Log "Gateway ready"

Graceful shutdown: SIGTERM/SIGINT → cancel root context → HTTP server shutdown (10s) → pipeline drain (5s) → close DB pool → close Redis.

Health check (Phase 1 subset): PostgreSQL `SELECT 1` + Redis `PING`. No Kafka or Risk Engine checks yet.

---

### P1-17: Seed Script — Simulated Instruments

**Service:** Gateway + Scripts
**Files:**
- `scripts/seed-instruments.sh`
- Seed logic in `gateway/cmd/gateway/main.go` (auto-seed on startup)
**Dependencies:** P1-08, P1-10
**Acceptance Criteria:**
- All six instruments available via `GET /api/v1/instruments` after startup
- Correct asset classes: AAPL/MSFT/GOOG = equity, BTC-USD/ETH-USD/SOL-USD = crypto
- Correct settlement cycles: equity = T2, crypto = T0
- Correct tick/lot sizes
- `make seed` works as manual alternative

**Architecture Context:**

Section 4A + Section 8 Phase 1 deliverable #14. Instruments:

| ID | Symbol | Name | Asset Class | Quote | Tick Size | Lot Size | Settlement | Venues |
|----|--------|------|-------------|-------|-----------|----------|------------|--------|
| AAPL | AAPL | Apple Inc. | equity | USD | 0.01 | 1 | T2 | simulated |
| MSFT | MSFT | Microsoft Corp. | equity | USD | 0.01 | 1 | T2 | simulated |
| GOOG | GOOG | Alphabet Inc. | equity | USD | 0.01 | 1 | T2 | simulated |
| BTC-USD | BTC-USD | Bitcoin | crypto | USD | 0.01 | 0.00001 | T0 | simulated |
| ETH-USD | ETH-USD | Ethereum | crypto | USD | 0.01 | 0.0001 | T0 | simulated |
| SOL-USD | SOL-USD | Solana | crypto | USD | 0.01 | 0.01 | T0 | simulated |

Preferred: gateway auto-seeds on startup if instruments table is empty. `scripts/seed-instruments.sh` as manual fallback using `curl POST /api/v1/instruments` or direct `psql INSERT`.

---

### P1-18: Docker Compose (Phase 1 Topology)

**Service:** Infrastructure
**Files:**
- `deploy/docker-compose.yml`
**Dependencies:** P1-01, P1-06, P1-08
**Acceptance Criteria:**
- `docker compose up` brings up gateway, dashboard, postgres, redis
- Health checks pass for all services within 60 seconds
- Dashboard reachable at `localhost:3000`
- Gateway reachable at `localhost:8080`
- Gateway auto-runs migrations on startup
- `docker compose down -v` cleanly tears down

**Architecture Context:**

Section 7.1 — Phase 1 subset (no Kafka, no Risk Engine, no ML Scorer):

```yaml
services:
  gateway:
    build: ../gateway
    ports: ["8080:8080"]
    environment:
      - POSTGRES_URL=postgres://synapse:synapse@postgres:5432/synapse
      - REDIS_URL=redis://redis:6379
    depends_on:
      postgres: { condition: service_healthy }
      redis: { condition: service_healthy }
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v1/health"]
      interval: 10s
      timeout: 5s
      retries: 3

  dashboard:
    build: ../dashboard
    ports: ["3000:3000"]
    environment:
      - VITE_API_URL=http://localhost:8080
      - VITE_WS_URL=ws://localhost:8080

  postgres:
    image: postgres:16-alpine
    ports: ["5432:5432"]
    environment: { POSTGRES_USER: synapse, POSTGRES_PASSWORD: synapse, POSTGRES_DB: synapse }
    volumes: [pgdata:/var/lib/postgresql/data]
    healthcheck: { test: ["CMD-SHELL", "pg_isready -U synapse"], interval: 5s, timeout: 5s, retries: 5 }

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
    healthcheck: { test: ["CMD", "redis-cli", "ping"], interval: 5s, timeout: 5s, retries: 5 }

volumes:
  pgdata:
```

Phase 1 scoping: no Kafka (in-process channels), no Risk Engine (pipeline skips risk stage), no ML Scorer.

---

### P1-19: End-to-End Acceptance Test

**Service:** Gateway + Integration
**Files:**
- `gateway/integration_test.go` (build tag `//go:build integration`)
- `scripts/acceptance-test.sh`
**Dependencies:** All prior tasks (P1-01 through P1-18)
**Acceptance Criteria:**
- Integration test passes against running Docker Compose stack
- Full flow verified: submit order → fill → position update, all via API
- WebSocket delivers real-time status transitions (New → Acknowledged → Filled)
- Matches exact acceptance scenario: "market buy 10 AAPL → blotter shows New → Acknowledged → Filled → position of 10 AAPL"

**Architecture Context:**

Section 8 acceptance test + Section 9.2/9.3:

```go
// gateway/integration_test.go (//go:build integration)
func TestPhase1AcceptanceFlow(t *testing.T) {
    // 1. GET /api/v1/instruments — verify 6 instruments
    // 2. Connect WebSocket to /ws/orders
    // 3. POST /api/v1/orders — market buy 10 AAPL
    // 4. Verify 201, order status "new"
    // 5. Wait for WS: New → Acknowledged → Filled (within 5s)
    // 6. GET /api/v1/orders/{id} — status "filled", filledQuantity "10", fills non-empty
    // 7. GET /api/v1/positions/AAPL — quantity "10"
    // 8. Connect /ws/positions — verify position update received
}
```

`scripts/acceptance-test.sh`: curl + wscat manual version, prints pass/fail per step.

---

## Checklist Cross-Reference

### Architecture Deliverables → Tasks

| # | Deliverable | Task |
|---|-------------|------|
| 1 | Proto definitions for Order, Fill, Instrument, Position | **Already complete** (checked in deliverables-checklist.md) |
| 2 | Gateway: domain model | P1-02 |
| 3 | Gateway: Simulated exchange adapter | P1-07 |
| 4 | Gateway: Order processing pipeline | P1-09 |
| 5 | Gateway: REST API | P1-10 |
| 6 | Gateway: WebSocket server | P1-11 |
| 7 | Gateway: PostgreSQL schema + migrations | P1-08 |
| 8 | Dashboard: Project scaffolding | P1-06 |
| 9 | Dashboard: Terminal layout shell | P1-12 |
| 10 | Dashboard: Order ticket component | P1-13 |
| 11 | Dashboard: Blotter view with AG Grid | P1-14 |
| 12 | Dashboard: Basic position table | P1-15 |
| 13 | Docker Compose | P1-18 |
| 14 | Seed script | P1-17 |

### Cross-Cutting → Tasks

| Item | Task |
|------|------|
| Structured JSON logging with correlation IDs | P1-03 |
| Error handling patterns | P1-04 |
| Makefile with top-level build targets | P1-05 |

### Deliverables Checklist Items Mapped to Phase 1

All unchecked items from `docs/deliverables-checklist.md` that map to Phase 1 deliverables have corresponding tasks:

- `gateway/cmd/gateway/main.go` → P1-01 (stub), P1-16 (wired)
- `gateway/internal/domain/*.go` → P1-02
- `gateway/internal/adapter/provider.go`, `registry.go`, `simulated/*` → P1-07
- `gateway/internal/pipeline/*` → P1-09
- `gateway/internal/rest/handler_order.go`, `handler_position.go` → P1-10
- `gateway/internal/ws/server.go` → P1-11
- `gateway/go.mod`, `Dockerfile` → P1-01
- PostgreSQL schema + migrations → P1-08
- `dashboard/src/main.tsx`, `App.tsx` → P1-06
- `dashboard/src/api/*` → P1-06
- `dashboard/src/stores/orderStore.ts` → P1-06 (stubs), P1-14 (complete)
- `dashboard/src/stores/positionStore.ts` → P1-06 (stubs), P1-15 (complete)
- `dashboard/src/views/BlotterView.tsx` → P1-14
- `dashboard/src/views/PortfolioView.tsx` → P1-15 (basic)
- `dashboard/src/components/OrderTicket.tsx` → P1-13
- `dashboard/src/components/OrderTable.tsx` → P1-14
- `dashboard/src/components/PositionTable.tsx` → P1-15
- `dashboard/src/components/TerminalLayout.tsx` → P1-12
- `dashboard/src/theme/terminal.ts` → P1-06
- `dashboard/index.html`, `vite.config.ts`, `tsconfig.json`, `package.json`, `Dockerfile` → P1-06
- `deploy/docker-compose.yml` → P1-18
- `scripts/seed-instruments.sh` → P1-17
- `Makefile` → P1-05
- `gateway/internal/adapter/simulated/matching_engine_test.go` → P1-07
- `gateway/internal/pipeline/pipeline_test.go` → P1-09
- `gateway/internal/domain/order_test.go` → P1-02
- `dashboard/src/components/OrderTicket.test.tsx` → P1-13
- `gateway/integration_test.go` → P1-19

### Explicitly Deferred to Phase 2+

These unchecked checklist items are NOT in Phase 1 scope:

- `gateway/internal/adapter/alpaca/*`, `binance/*`, `tokenized/*` → Phase 2
- `gateway/internal/credential/*` → Phase 2 (no credentials needed for simulated)
- `gateway/internal/kafka/*` → Phase 2 (in-process channels for Phase 1)
- `gateway/internal/grpc/*` → Phase 2 (no risk check)
- `gateway/internal/router/*` (smart routing) → Phase 3
- `gateway/internal/crossing/*` → Phase 3
- `gateway/internal/orderbook/*` → Phase 2+ (simulated adapter has its own)
- `gateway/internal/rest/handler_venue.go`, `handler_credential.go` → Phase 2
- All `risk_engine/*` → Phase 2
- All `ai/*` → Phase 4
- `dashboard/src/stores/riskStore.ts`, `venueStore.ts`, `insightStore.ts` → Phase 2+
- `dashboard/src/views/RiskDashboard.tsx`, `LiquidityNetwork.tsx`, `InsightsPanel.tsx`, `OnboardingView.tsx` → Phase 2+
- `dashboard/src/components/VaRGauge.tsx`, `ExposureTreemap.tsx`, `DrawdownChart.tsx`, `MonteCarloPlot.tsx`, `CandlestickChart.tsx`, `VenueCard.tsx`, `CredentialForm.tsx` → Phase 2+
- `deploy/docker-compose.dev.yml`, `deploy/k8s/*`, `deploy/grafana/*`, `deploy/prometheus.yml` → Phase 2/5
- `loadtest/*` → Phase 5
- `.github/workflows/*` → Phase 5
- `scripts/health-check.sh` → Phase 2
- All documentation (`README.md`, `docs/quickstart.md`, etc.) → Phase 5
- `LICENSE`, `CONTRIBUTING.md` → Phase 5
- Playwright E2E tests → Phase 2+ (P1-19 covers programmatic acceptance)
- `gateway/internal/adapter/contract_test.go` → Phase 2 (need multiple adapters)
- `gateway/internal/pipeline/pipeline_bench_test.go` → Phase 5

---

## Out of Scope for Phase 1

- **Kafka** — replaced by in-process Go channels
- **gRPC risk check** — pipeline skips risk stage
- **Real venue adapters** (Alpaca, Binance) — only simulated
- **Credential encryption** (AES-256-GCM, Argon2id) — not needed for simulated
- **Smart order routing / ML scoring** — hardcoded to simulated venue
- **Dark pool / crossing engine**
- **Risk dashboard, VaR, portfolio charts**
- **AI features**
- **Kubernetes, Prometheus, Grafana**
- **Load testing, CI/CD pipelines**

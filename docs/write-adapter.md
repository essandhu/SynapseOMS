# Write a Venue Adapter

SynapseOMS is designed to be extended with new exchange connections. Each venue adapter implements a single Go interface. This guide walks you through creating one from scratch.

## The LiquidityProvider Interface

Every adapter must implement the `LiquidityProvider` interface defined in `gateway/internal/adapter/provider.go`:

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

## Step 1: Create the Adapter Package

Create a new directory under `gateway/internal/adapter/`:

```
gateway/internal/adapter/myexchange/
    adapter.go
    adapter_test.go
```

## Step 2: Implement the Interface

Start with this skeleton:

```go
package myexchange

import (
    "context"
    "fmt"
    "log/slog"
    "time"

    "github.com/synapse-oms/gateway/internal/adapter"
    "github.com/synapse-oms/gateway/internal/domain"
    "github.com/synapse-oms/gateway/internal/logging"
)

const (
    venueID   = "my_exchange"
    venueName = "My Exchange"
)

type Adapter struct {
    status adapter.VenueStatus
    fillCh chan domain.Fill
    logger *slog.Logger
    // Add exchange-specific fields: API client, WebSocket conn, etc.
}

func NewAdapter(config map[string]string) adapter.LiquidityProvider {
    return &Adapter{
        status: adapter.Disconnected,
        fillCh: make(chan domain.Fill, 1000),
        logger: logging.NewDefault("gateway", "myexchange-adapter"),
    }
}

func (a *Adapter) VenueID() string  { return venueID }
func (a *Adapter) VenueName() string { return venueName }

func (a *Adapter) SupportedAssetClasses() []domain.AssetClass {
    return []domain.AssetClass{domain.AssetClassCrypto}
}

func (a *Adapter) SupportedInstruments() ([]domain.Instrument, error) {
    // Return instruments this venue supports.
    // Can be hardcoded or fetched from the exchange API.
    return []domain.Instrument{
        {ID: "BTC-USD", Symbol: "BTC-USD", Name: "Bitcoin",
         AssetClass: domain.AssetClassCrypto, QuoteCurrency: "USD"},
    }, nil
}

func (a *Adapter) Connect(ctx context.Context, cred domain.VenueCredential) error {
    // 1. Initialize API client with cred.APIKey, cred.APISecret
    // 2. Verify credentials with a test API call
    // 3. Start WebSocket connections for market data / fills
    a.status = adapter.Connected
    return nil
}

func (a *Adapter) Disconnect(ctx context.Context) error {
    // Close all connections, stop goroutines
    a.status = adapter.Disconnected
    return nil
}

func (a *Adapter) Status() adapter.VenueStatus { return a.status }

func (a *Adapter) Ping(ctx context.Context) (time.Duration, error) {
    // Make a lightweight API call to measure latency
    start := time.Now()
    // ... ping exchange ...
    return time.Since(start), nil
}

func (a *Adapter) SubmitOrder(ctx context.Context, order *domain.Order) (*adapter.VenueAck, error) {
    if a.status != adapter.Connected {
        return nil, fmt.Errorf("%s not connected", venueName)
    }
    // 1. Translate domain.Order to exchange-specific format
    // 2. Submit via REST/WebSocket
    // 3. Return venue's order ID
    return &adapter.VenueAck{
        VenueOrderID: "EXCHANGE-123",
        ReceivedAt:   time.Now(),
    }, nil
}

func (a *Adapter) CancelOrder(ctx context.Context, orderID domain.OrderID, venueOrderID string) error {
    // Cancel the order on the exchange
    return nil
}

func (a *Adapter) QueryOrder(ctx context.Context, venueOrderID string) (*domain.Order, error) {
    // Fetch order status from the exchange
    return nil, fmt.Errorf("order %s not found", venueOrderID)
}

func (a *Adapter) SubscribeMarketData(ctx context.Context, instruments []string) (<-chan adapter.MarketDataSnapshot, error) {
    ch := make(chan adapter.MarketDataSnapshot, 100)
    // Start WebSocket subscription, push snapshots to ch
    return ch, nil
}

func (a *Adapter) UnsubscribeMarketData(ctx context.Context, instruments []string) error {
    return nil
}

func (a *Adapter) FillFeed() <-chan domain.Fill { return a.fillCh }

func (a *Adapter) Capabilities() adapter.VenueCapabilities {
    return adapter.VenueCapabilities{
        SupportedOrderTypes:   []domain.OrderType{domain.OrderTypeMarket, domain.OrderTypeLimit},
        SupportedAssetClasses: a.SupportedAssetClasses(),
        SupportsStreaming:     true,
        MaxOrdersPerSecond:    100,
    }
}

// Register the adapter factory so it appears in venue listings.
func init() {
    adapter.Register("my_exchange", NewAdapter)
}
```

## Step 3: Register the Adapter

The `init()` function at the bottom of your adapter file registers it with the adapter registry. To ensure it runs at startup, add a blank import in `gateway/cmd/gateway/main.go`:

```go
import (
    _ "github.com/synapse-oms/gateway/internal/adapter/myexchange"
)
```

## Step 4: Pass the Contract Test Suite

SynapseOMS includes a shared contract test suite (`gateway/internal/adapter/contract_test.go`) that validates every adapter meets baseline requirements.

Add your adapter to the contract tests:

```go
// In gateway/internal/adapter/contract_test.go

import "github.com/synapse-oms/gateway/internal/adapter/myexchange"

func TestMyExchangeAdapterContract(t *testing.T) {
    a := myexchange.NewAdapter(nil)
    contractSuite(t, a)
}
```

Run the contract tests:

```bash
cd gateway
go test ./internal/adapter/ -run TestMyExchangeAdapterContract -v
# Or run all adapter tests:
go test ./internal/adapter/... -v
```

### What the Contract Suite Tests

The contract suite validates 7 baseline behaviors:

| Test | What It Checks |
|------|----------------|
| VenueID returns non-empty string | Adapter has a venue identifier |
| VenueName returns non-empty string | Adapter has a display name |
| Status returns Disconnected before Connect | Correct initial state |
| SupportedInstruments returns at least one | Adapter declares tradeable instruments |
| SupportedAssetClasses returns at least one | Adapter declares asset classes |
| FillFeed returns non-nil channel | Fill channel is initialized |
| Capabilities returns valid capabilities | Order types and asset classes declared |

## Step 5: Write Adapter-Specific Tests

Beyond the contract suite, write tests for your adapter's specific behavior:

```go
// gateway/internal/adapter/myexchange/adapter_test.go
package myexchange

import (
    "context"
    "testing"
    "github.com/synapse-oms/gateway/internal/adapter"
    "github.com/synapse-oms/gateway/internal/domain"
)

func TestSubmitOrderReturnsVenueAck(t *testing.T) {
    a := NewAdapter(nil).(*Adapter)
    a.status = adapter.Connected
    // ... test order submission
}

func TestSubmitOrderFailsWhenDisconnected(t *testing.T) {
    a := NewAdapter(nil).(*Adapter)
    _, err := a.SubmitOrder(context.Background(), &domain.Order{})
    if err == nil {
        t.Error("expected error when disconnected")
    }
}
```

## Step 6: Submit a PR

1. Fork the repository
2. Create a branch: `git checkout -b adapter/my-exchange`
3. Implement and test your adapter
4. Ensure all tests pass: `cd gateway && go test ./...`
5. Submit a PR with:
   - The adapter implementation
   - Unit tests
   - Contract test entry
   - A brief description of the exchange and any setup instructions

## Reference Implementations

Study these existing adapters for patterns:

| Adapter | Path | Notes |
|---------|------|-------|
| Simulated | `adapter/simulated/` | Simplest — in-memory matching engine, no external API |
| Alpaca | `adapter/alpaca/` | REST + WebSocket, paper trading, US equities |
| Binance | `adapter/binance/` | REST execution, WebSocket data, crypto |
| Tokenized | `adapter/tokenized/` | T+0 settlement, tokenized securities |

## Tips

- **Start with the simulated adapter** as your template — it's the simplest
- **Use structured logging** (`slog`) for all adapter operations
- **Handle reconnection** — exchange WebSocket connections drop; reconnect with backoff
- **Respect rate limits** — set `MaxOrdersPerSecond` in Capabilities appropriately
- **Map instrument IDs** — your exchange's symbol format may differ from SynapseOMS's internal format

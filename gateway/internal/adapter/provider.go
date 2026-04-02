// Package adapter defines the LiquidityProvider interface and adapter registry.
package adapter

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
)

// VenueStatus represents the connection status of a venue.
type VenueStatus int

const (
	// Connected indicates the venue adapter is connected and operational.
	Connected VenueStatus = iota
	// Disconnected indicates the venue adapter is not connected.
	Disconnected
	// Degraded indicates the venue adapter is connected but experiencing issues.
	Degraded
)

// String returns a human-readable representation of a VenueStatus.
func (s VenueStatus) String() string {
	switch s {
	case Connected:
		return "Connected"
	case Disconnected:
		return "Disconnected"
	case Degraded:
		return "Degraded"
	default:
		return "Unknown"
	}
}

// VenueAck represents an acknowledgement from a venue for an order submission.
type VenueAck struct {
	VenueOrderID string
	ReceivedAt   time.Time
}

// VenueCapabilities describes the capabilities of a venue adapter.
type VenueCapabilities struct {
	SupportedOrderTypes   []domain.OrderType
	SupportedAssetClasses []domain.AssetClass
	SupportsStreaming      bool
	MaxOrdersPerSecond    int
}

// MarketDataSnapshot represents a point-in-time market data update for an instrument.
type MarketDataSnapshot struct {
	InstrumentID string
	VenueID      string
	BidPrice     decimal.Decimal
	AskPrice     decimal.Decimal
	LastPrice    decimal.Decimal
	Volume24h    decimal.Decimal
	Timestamp    time.Time
}

// LiquidityProvider is the interface all venue adapters must implement.
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

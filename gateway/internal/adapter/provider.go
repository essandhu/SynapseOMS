// Package adapter defines the LiquidityProvider interface and adapter registry.
package adapter

import (
	"context"
	"time"

	"github.com/synapse-oms/gateway/internal/domain"
)

// VenueStatus represents the connection status of a venue.
type VenueStatus int

const (
	// Connected indicates the venue adapter is connected and operational.
	Connected VenueStatus = iota
	// Disconnected indicates the venue adapter is not connected.
	Disconnected
)

// VenueAck represents an acknowledgement from a venue for an order submission.
type VenueAck struct {
	VenueOrderID string
	ReceivedAt   time.Time
}

// LiquidityProvider is the interface all venue adapters must implement.
type LiquidityProvider interface {
	VenueID() string
	VenueName() string
	SupportedInstruments() ([]domain.Instrument, error)
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Status() VenueStatus
	SubmitOrder(ctx context.Context, order *domain.Order) (*VenueAck, error)
	CancelOrder(ctx context.Context, orderID domain.OrderID, venueOrderID string) error
	FillFeed() <-chan domain.Fill
}

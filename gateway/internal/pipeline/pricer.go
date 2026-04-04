package pipeline

import (
	"context"
	"log/slog"

	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/logging"
)

// PositionPricer listens to market data snapshots and updates position
// market prices and unrealized P&L in real time.
type PositionPricer struct {
	store    Store
	notifier Notifier
	logger   *slog.Logger
}

// NewPositionPricer creates a new pricer that updates positions from market data.
func NewPositionPricer(store Store, notifier Notifier) *PositionPricer {
	return &PositionPricer{
		store:    store,
		notifier: notifier,
		logger:   logging.NewDefault("gateway", "position-pricer"),
	}
}

// OnMarketData processes a market data snapshot, updating the corresponding
// position's market price and unrealized P&L if a position exists for the
// given instrument and venue.
func (pp *PositionPricer) OnMarketData(ctx context.Context, snap adapter.MarketDataSnapshot) {
	if snap.LastPrice.IsZero() {
		return
	}

	pos, err := pp.store.GetPosition(ctx, snap.InstrumentID, snap.VenueID)
	if err != nil {
		// No position for this instrument/venue — nothing to update.
		return
	}

	pos.UpdateMarketPrice(snap.LastPrice)

	if err := pp.store.UpsertPosition(ctx, pos); err != nil {
		pp.logger.Error("failed to persist position after market price update",
			slog.String("instrument", snap.InstrumentID),
			slog.String("venue", snap.VenueID),
			slog.String("error", err.Error()),
		)
		return
	}

	pp.notifier.NotifyPositionUpdate(pos)
}

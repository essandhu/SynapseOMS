package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
)

// UpsertPosition inserts or updates a position using INSERT ON CONFLICT.
func (s *PostgresStore) UpsertPosition(ctx context.Context, p *domain.Position) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO positions (
			instrument_id, venue_id, quantity, average_cost, market_price,
			unrealized_pnl, realized_pnl, unsettled_quantity, settled_quantity,
			asset_class, quote_currency, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12
		)
		ON CONFLICT (instrument_id, venue_id) DO UPDATE SET
			quantity = EXCLUDED.quantity,
			average_cost = EXCLUDED.average_cost,
			market_price = EXCLUDED.market_price,
			unrealized_pnl = EXCLUDED.unrealized_pnl,
			realized_pnl = EXCLUDED.realized_pnl,
			unsettled_quantity = EXCLUDED.unsettled_quantity,
			settled_quantity = EXCLUDED.settled_quantity,
			asset_class = EXCLUDED.asset_class,
			quote_currency = EXCLUDED.quote_currency,
			updated_at = EXCLUDED.updated_at`,
		p.InstrumentID,
		p.VenueID,
		p.Quantity.String(),
		p.AverageCost.String(),
		p.MarketPrice.String(),
		p.UnrealizedPnL.String(),
		p.RealizedPnL.String(),
		p.UnsettledQuantity.String(),
		p.SettledQuantity.String(),
		assetClassToString(p.AssetClass),
		p.QuoteCurrency,
		p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upserting position: %w", err)
	}
	return nil
}

// GetPosition retrieves a position by its composite key (instrument_id, venue_id).
func (s *PostgresStore) GetPosition(ctx context.Context, instrumentID, venueID string) (*domain.Position, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT instrument_id, venue_id, quantity, average_cost, market_price,
			unrealized_pnl, realized_pnl, unsettled_quantity, settled_quantity,
			asset_class, quote_currency, updated_at
		FROM positions
		WHERE instrument_id = $1 AND venue_id = $2`, instrumentID, venueID)

	return scanPosition(row)
}

// ListPositions retrieves all positions.
func (s *PostgresStore) ListPositions(ctx context.Context) ([]domain.Position, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT instrument_id, venue_id, quantity, average_cost, market_price,
			unrealized_pnl, realized_pnl, unsettled_quantity, settled_quantity,
			asset_class, quote_currency, updated_at
		FROM positions
		ORDER BY instrument_id, venue_id`)
	if err != nil {
		return nil, fmt.Errorf("listing positions: %w", err)
	}
	defer rows.Close()

	var positions []domain.Position
	for rows.Next() {
		var (
			p          domain.Position
			assetClass string
			quantity, averageCost, marketPrice                   decimal.Decimal
			unrealizedPnL, realizedPnL                           decimal.Decimal
			unsettledQuantity, settledQuantity                    decimal.Decimal
		)

		err := rows.Scan(
			&p.InstrumentID, &p.VenueID, &quantity, &averageCost, &marketPrice,
			&unrealizedPnL, &realizedPnL, &unsettledQuantity, &settledQuantity,
			&assetClass, &p.QuoteCurrency, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning position row: %w", err)
		}

		p.Quantity = quantity
		p.AverageCost = averageCost
		p.MarketPrice = marketPrice
		p.UnrealizedPnL = unrealizedPnL
		p.RealizedPnL = realizedPnL
		p.UnsettledQuantity = unsettledQuantity
		p.SettledQuantity = settledQuantity
		p.AssetClass = stringToAssetClass(assetClass)

		positions = append(positions, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating positions: %w", err)
	}

	return positions, nil
}

// scanPosition scans a single position from a pgx.Row.
func scanPosition(row pgx.Row) (*domain.Position, error) {
	var (
		p          domain.Position
		assetClass string
		quantity, averageCost, marketPrice                   decimal.Decimal
		unrealizedPnL, realizedPnL                           decimal.Decimal
		unsettledQuantity, settledQuantity                    decimal.Decimal
	)

	err := row.Scan(
		&p.InstrumentID, &p.VenueID, &quantity, &averageCost, &marketPrice,
		&unrealizedPnL, &realizedPnL, &unsettledQuantity, &settledQuantity,
		&assetClass, &p.QuoteCurrency, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning position: %w", err)
	}

	p.Quantity = quantity
	p.AverageCost = averageCost
	p.MarketPrice = marketPrice
	p.UnrealizedPnL = unrealizedPnL
	p.RealizedPnL = realizedPnL
	p.UnsettledQuantity = unsettledQuantity
	p.SettledQuantity = settledQuantity
	p.AssetClass = stringToAssetClass(assetClass)

	return &p, nil
}

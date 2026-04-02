package store

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
)

// CreateFill inserts a new fill into the database.
func (s *PostgresStore) CreateFill(ctx context.Context, f *domain.Fill) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO fills (
			id, order_id, venue_id, quantity, price,
			fee, fee_asset, liquidity, venue_exec_id, timestamp
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10
		)`,
		f.ID,
		string(f.OrderID),
		f.VenueID,
		f.Quantity.String(),
		f.Price.String(),
		f.Fee.String(),
		f.FeeAsset,
		liquidityToString(f.Liquidity),
		f.VenueExecID,
		f.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("creating fill: %w", err)
	}
	return nil
}

// ListFillsByOrder retrieves all fills for a given order, ordered by timestamp.
func (s *PostgresStore) ListFillsByOrder(ctx context.Context, orderID domain.OrderID) ([]domain.Fill, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, order_id, venue_id, quantity, price,
			fee, fee_asset, liquidity, venue_exec_id, timestamp
		FROM fills
		WHERE order_id = $1
		ORDER BY timestamp ASC`, string(orderID))
	if err != nil {
		return nil, fmt.Errorf("listing fills: %w", err)
	}
	defer rows.Close()

	var fills []domain.Fill
	for rows.Next() {
		var (
			f                        domain.Fill
			orderIDStr, liquidity    string
			quantity, price, fee     decimal.Decimal
		)

		err := rows.Scan(
			&f.ID, &orderIDStr, &f.VenueID, &quantity, &price,
			&fee, &f.FeeAsset, &liquidity, &f.VenueExecID, &f.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning fill row: %w", err)
		}

		f.OrderID = domain.OrderID(orderIDStr)
		f.Quantity = quantity
		f.Price = price
		f.Fee = fee
		f.Liquidity = stringToLiquidity(liquidity)

		fills = append(fills, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating fills: %w", err)
	}

	return fills, nil
}

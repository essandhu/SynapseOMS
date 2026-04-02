package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
)

// OrderFilter specifies optional filters for listing orders.
type OrderFilter struct {
	Status       *domain.OrderStatus
	InstrumentID *string
}

// CreateOrder inserts a new order into the database.
func (s *PostgresStore) CreateOrder(ctx context.Context, o *domain.Order) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO orders (
			id, client_order_id, instrument_id, side, type,
			quantity, price, filled_quantity, average_price, status,
			venue_id, asset_class, settlement_cycle, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15
		)`,
		string(o.ID),
		o.ClientOrderID,
		o.InstrumentID,
		orderSideToString(o.Side),
		orderTypeToString(o.Type),
		o.Quantity.String(),
		o.Price.String(),
		o.FilledQuantity.String(),
		o.AveragePrice.String(),
		orderStatusToString(o.Status),
		o.VenueID,
		assetClassToString(o.AssetClass),
		settlementCycleToString(o.SettlementCycle),
		o.CreatedAt,
		o.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("creating order: %w", err)
	}
	return nil
}

// GetOrder retrieves a single order by its ID.
func (s *PostgresStore) GetOrder(ctx context.Context, id domain.OrderID) (*domain.Order, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, client_order_id, instrument_id, side, type,
			quantity, price, filled_quantity, average_price, status,
			venue_id, asset_class, settlement_cycle, created_at, updated_at
		FROM orders
		WHERE id = $1`, string(id))

	return scanOrder(row)
}

// ListOrders retrieves orders matching the given filter.
func (s *PostgresStore) ListOrders(ctx context.Context, filter OrderFilter) ([]domain.Order, error) {
	query := `
		SELECT id, client_order_id, instrument_id, side, type,
			quantity, price, filled_quantity, average_price, status,
			venue_id, asset_class, settlement_cycle, created_at, updated_at
		FROM orders WHERE 1=1`

	args := []any{}
	argIdx := 1

	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, orderStatusToString(*filter.Status))
		argIdx++
	}
	if filter.InstrumentID != nil {
		query += fmt.Sprintf(" AND instrument_id = $%d", argIdx)
		args = append(args, *filter.InstrumentID)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing orders: %w", err)
	}
	defer rows.Close()

	var orders []domain.Order
	for rows.Next() {
		o, err := scanOrderFromRows(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, *o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating orders: %w", err)
	}

	return orders, nil
}

// UpdateOrder updates a mutable subset of order fields.
func (s *PostgresStore) UpdateOrder(ctx context.Context, o *domain.Order) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE orders SET
			filled_quantity = $1,
			average_price = $2,
			status = $3,
			venue_id = $4,
			updated_at = $5
		WHERE id = $6`,
		o.FilledQuantity.String(),
		o.AveragePrice.String(),
		orderStatusToString(o.Status),
		o.VenueID,
		o.UpdatedAt,
		string(o.ID),
	)
	if err != nil {
		return fmt.Errorf("updating order: %w", err)
	}
	return nil
}

// scanOrder scans a single order row from pgx.Row.
func scanOrder(row pgx.Row) (*domain.Order, error) {
	var (
		o                                                      domain.Order
		id, side, typ, status, assetClass, settlementCycle     string
		quantity, price, filledQuantity, averagePrice           decimal.Decimal
	)

	err := row.Scan(
		&id, &o.ClientOrderID, &o.InstrumentID, &side, &typ,
		&quantity, &price, &filledQuantity, &averagePrice, &status,
		&o.VenueID, &assetClass, &settlementCycle, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning order: %w", err)
	}

	o.ID = domain.OrderID(id)
	o.Side = stringToOrderSide(side)
	o.Type = stringToOrderType(typ)
	o.Status = stringToOrderStatus(status)
	o.AssetClass = stringToAssetClass(assetClass)
	o.SettlementCycle = stringToSettlementCycle(settlementCycle)
	o.Quantity = quantity
	o.Price = price
	o.FilledQuantity = filledQuantity
	o.AveragePrice = averagePrice

	return &o, nil
}

// scanOrderFromRows scans an order from pgx.Rows (same columns as scanOrder).
func scanOrderFromRows(rows pgx.Rows) (*domain.Order, error) {
	var (
		o                                                      domain.Order
		id, side, typ, status, assetClass, settlementCycle     string
		quantity, price, filledQuantity, averagePrice           decimal.Decimal
	)

	err := rows.Scan(
		&id, &o.ClientOrderID, &o.InstrumentID, &side, &typ,
		&quantity, &price, &filledQuantity, &averagePrice, &status,
		&o.VenueID, &assetClass, &settlementCycle, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning order row: %w", err)
	}

	o.ID = domain.OrderID(id)
	o.Side = stringToOrderSide(side)
	o.Type = stringToOrderType(typ)
	o.Status = stringToOrderStatus(status)
	o.AssetClass = stringToAssetClass(assetClass)
	o.SettlementCycle = stringToSettlementCycle(settlementCycle)
	o.Quantity = quantity
	o.Price = price
	o.FilledQuantity = filledQuantity
	o.AveragePrice = averagePrice

	return &o, nil
}

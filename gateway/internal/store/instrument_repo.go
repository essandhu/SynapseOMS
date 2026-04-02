package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
)

// UpsertInstrument inserts or updates an instrument using INSERT ON CONFLICT.
func (s *PostgresStore) UpsertInstrument(ctx context.Context, inst *domain.Instrument) error {
	tradingHoursJSON, err := json.Marshal(inst.TradingHours)
	if err != nil {
		return fmt.Errorf("marshaling trading hours: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO instruments (
			id, symbol, name, asset_class, quote_currency, base_currency,
			tick_size, lot_size, settlement_cycle, trading_hours,
			venues, margin_required
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12
		)
		ON CONFLICT (id) DO UPDATE SET
			symbol = EXCLUDED.symbol,
			name = EXCLUDED.name,
			asset_class = EXCLUDED.asset_class,
			quote_currency = EXCLUDED.quote_currency,
			base_currency = EXCLUDED.base_currency,
			tick_size = EXCLUDED.tick_size,
			lot_size = EXCLUDED.lot_size,
			settlement_cycle = EXCLUDED.settlement_cycle,
			trading_hours = EXCLUDED.trading_hours,
			venues = EXCLUDED.venues,
			margin_required = EXCLUDED.margin_required`,
		inst.ID,
		inst.Symbol,
		inst.Name,
		assetClassToString(inst.AssetClass),
		inst.QuoteCurrency,
		inst.BaseCurrency,
		inst.TickSize.String(),
		inst.LotSize.String(),
		settlementCycleToString(inst.SettlementCycle),
		tradingHoursJSON,
		inst.Venues,
		inst.MarginRequired.String(),
	)
	if err != nil {
		return fmt.Errorf("upserting instrument: %w", err)
	}
	return nil
}

// GetInstrument retrieves a single instrument by ID.
func (s *PostgresStore) GetInstrument(ctx context.Context, id string) (*domain.Instrument, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, symbol, name, asset_class, quote_currency, base_currency,
			tick_size, lot_size, settlement_cycle, trading_hours,
			venues, margin_required, created_at
		FROM instruments
		WHERE id = $1`, id)

	return scanInstrument(row)
}

// ListInstruments retrieves all instruments.
func (s *PostgresStore) ListInstruments(ctx context.Context) ([]domain.Instrument, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, symbol, name, asset_class, quote_currency, base_currency,
			tick_size, lot_size, settlement_cycle, trading_hours,
			venues, margin_required, created_at
		FROM instruments
		ORDER BY symbol`)
	if err != nil {
		return nil, fmt.Errorf("listing instruments: %w", err)
	}
	defer rows.Close()

	var instruments []domain.Instrument
	for rows.Next() {
		var (
			inst                                          domain.Instrument
			assetClass, settlementCycle                   string
			baseCurrency                                  *string
			tickSize, lotSize, marginRequired              decimal.Decimal
			tradingHoursJSON                               []byte
			createdAt                                     interface{}
		)

		err := rows.Scan(
			&inst.ID, &inst.Symbol, &inst.Name, &assetClass, &inst.QuoteCurrency,
			&baseCurrency, &tickSize, &lotSize, &settlementCycle, &tradingHoursJSON,
			&inst.Venues, &marginRequired, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning instrument row: %w", err)
		}

		inst.AssetClass = stringToAssetClass(assetClass)
		inst.SettlementCycle = stringToSettlementCycle(settlementCycle)
		inst.TickSize = tickSize
		inst.LotSize = lotSize
		inst.MarginRequired = marginRequired
		if baseCurrency != nil {
			inst.BaseCurrency = *baseCurrency
		}
		if tradingHoursJSON != nil {
			if err := json.Unmarshal(tradingHoursJSON, &inst.TradingHours); err != nil {
				return nil, fmt.Errorf("unmarshaling trading hours: %w", err)
			}
		}

		instruments = append(instruments, inst)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating instruments: %w", err)
	}

	return instruments, nil
}

// scanInstrument scans a single instrument from a pgx.Row.
func scanInstrument(row pgx.Row) (*domain.Instrument, error) {
	var (
		inst                                          domain.Instrument
		assetClass, settlementCycle                   string
		baseCurrency                                  *string
		tickSize, lotSize, marginRequired              decimal.Decimal
		tradingHoursJSON                               []byte
		createdAt                                     interface{}
	)

	err := row.Scan(
		&inst.ID, &inst.Symbol, &inst.Name, &assetClass, &inst.QuoteCurrency,
		&baseCurrency, &tickSize, &lotSize, &settlementCycle, &tradingHoursJSON,
		&inst.Venues, &marginRequired, &createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning instrument: %w", err)
	}

	inst.AssetClass = stringToAssetClass(assetClass)
	inst.SettlementCycle = stringToSettlementCycle(settlementCycle)
	inst.TickSize = tickSize
	inst.LotSize = lotSize
	inst.MarginRequired = marginRequired
	if baseCurrency != nil {
		inst.BaseCurrency = *baseCurrency
	}
	if tradingHoursJSON != nil {
		if err := json.Unmarshal(tradingHoursJSON, &inst.TradingHours); err != nil {
			return nil, fmt.Errorf("unmarshaling trading hours: %w", err)
		}
	}

	return &inst, nil
}

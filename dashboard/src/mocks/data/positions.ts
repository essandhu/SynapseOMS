import type { Position } from "../../api/types";

/**
 * Converts a camelCase Position to the snake_case JSON the Go backend actually returns.
 * Used by MSW handlers to realistically simulate backend responses.
 */
export function toRawPosition(pos: Position): Record<string, unknown> {
  return {
    instrument_id: pos.instrumentId,
    venue_id: pos.venueId,
    quantity: pos.quantity,
    average_cost: pos.averageCost,
    market_price: pos.marketPrice,
    unrealized_pnl: pos.unrealizedPnl,
    realized_pnl: pos.realizedPnl,
    unsettled_quantity: pos.unsettledQuantity,
    asset_class: pos.assetClass,
    quote_currency: pos.quoteCurrency,
  };
}

export const mockPositions: Position[] = [
  {
    instrumentId: "AAPL",
    venueId: "alpaca",
    quantity: "100",
    averageCost: "150.00",
    marketPrice: "155.00",
    unrealizedPnl: "500.00",
    realizedPnl: "0.00",
    unsettledQuantity: "0",
    assetClass: "equity",
    quoteCurrency: "USD",
  },
  {
    instrumentId: "BTC-USD",
    venueId: "binance",
    quantity: "0.5",
    averageCost: "40000.00",
    marketPrice: "42000.00",
    unrealizedPnl: "1000.00",
    realizedPnl: "200.00",
    unsettledQuantity: "0",
    assetClass: "crypto",
    quoteCurrency: "USD",
  },
];

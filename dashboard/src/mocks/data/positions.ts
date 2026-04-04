import type { Position } from "../../api/types";

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

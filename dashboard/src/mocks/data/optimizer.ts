import type { OptimizationResult } from "../../api/types";

export const mockOptimizationResult: OptimizationResult = {
  targetWeights: { AAPL: 0.3, GOOG: 0.4, MSFT: 0.3 },
  trades: [
    {
      instrumentId: "AAPL",
      side: "buy",
      quantity: "100",
      estimatedCost: "15000.00",
    },
    {
      instrumentId: "GOOG",
      side: "sell",
      quantity: "50",
      estimatedCost: "7500.00",
    },
  ],
  expectedReturn: 0.12,
  expectedVolatility: 0.18,
  sharpeRatio: 0.67,
};

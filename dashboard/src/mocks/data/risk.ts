import type {
  VaRMetrics,
  DrawdownData,
  SettlementTimeline,
  PortfolioGreeks,
  ConcentrationResult,
} from "../../api/types";

/** Generate a deterministic Monte Carlo P&L distribution for mock data */
function generateMockDistribution(
  count: number,
  mean: number,
  stddev: number,
  seed: number,
): number[] {
  const result: number[] = [];
  for (let i = 0; i < count; i++) {
    // Deterministic pseudo-normal via simple linear congruential approach
    const t = (i + seed) / count;
    const z = mean + stddev * Math.tan(Math.PI * (t - 0.5)) * 0.15;
    result.push(Math.round(z * 100) / 100);
  }
  return result;
}

export const mockVaR: VaRMetrics = {
  historicalVaR: "12500.00",
  parametricVaR: "11200.00",
  monteCarloVaR: "13000.00",
  cvar: "15800.00",
  confidence: 95,
  horizon: "1d",
  computedAt: "2026-04-01T10:00:00Z",
  monteCarloDistribution: generateMockDistribution(1000, 500, 8000, 42),
};

export const mockDrawdown: DrawdownData = {
  current: -3.2,
  peak: "105000.00",
  trough: "101640.00",
  history: [
    { date: "2026-03-28", drawdown: -1.0 },
    { date: "2026-03-29", drawdown: -2.5 },
    { date: "2026-03-30", drawdown: -3.2 },
  ],
};

export const mockSettlement: SettlementTimeline = {
  totalUnsettled: "25000.00",
  entries: [
    {
      date: "2026-04-02",
      amount: "15000.00",
      instrumentId: "AAPL",
      assetClass: "equity",
    },
    {
      date: "2026-04-03",
      amount: "10000.00",
      instrumentId: "BTC/USD",
      assetClass: "crypto",
    },
  ],
};

export const mockGreeks: PortfolioGreeks = {
  total: { delta: 0.85, gamma: 0.02, vega: 15.3, theta: -2.1, rho: 0.5 },
  byInstrument: {
    AAPL: { delta: 0.45, gamma: 0.01, vega: 8.0, theta: -1.0, rho: 0.3 },
    "BTC/USD": {
      delta: 0.4,
      gamma: 0.01,
      vega: 7.3,
      theta: -1.1,
      rho: 0.2,
    },
  },
  computedAt: "2026-04-01T10:00:00Z",
};

export const mockConcentration: ConcentrationResult = {
  singleName: { AAPL: 35, "BTC/USD": 25, ETH: 20, GOOG: 20 },
  byAssetClass: { equity: 55, crypto: 45 },
  byVenue: { "venue-1": 60, "venue-2": 40 },
  warnings: ["AAPL exceeds 30% single-name threshold"],
  hhi: 2450,
};

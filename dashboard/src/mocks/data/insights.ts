import type {
  ExecutionReport,
  AnomalyAlert,
  RebalanceResult,
} from "../../api/types";

export const mockExecutionReports: ExecutionReport[] = [
  {
    overallGrade: "A",
    implementationShortfallBps: 2.5,
    summary: "Good execution",
    venueAnalysis: [
      { venue: "binance", grade: "A", comment: "Fast fills" },
    ],
    recommendations: ["Consider limit orders"],
    marketImpactEstimateBps: 1.2,
    orderId: "order-1",
    analyzedAt: "2026-04-15T14:30:00Z",
  },
];

export const mockAnomalyAlerts: AnomalyAlert[] = [
  {
    id: "alert-1",
    instrumentId: "ETH-USD",
    venueId: "binance",
    anomalyScore: -0.65,
    severity: "warning",
    features: { volume_zscore: 4.2 },
    description: "Volume spike",
    timestamp: "2026-04-15T14:30:00Z",
    acknowledged: false,
  },
  {
    id: "alert-2",
    instrumentId: "BTC-USD",
    venueId: "coinbase",
    anomalyScore: -0.85,
    severity: "critical",
    features: { spread_zscore: 6.1 },
    description: "Spread anomaly",
    timestamp: "2026-04-15T14:35:00Z",
    acknowledged: false,
  },
];

export const mockRebalanceResult: RebalanceResult = {
  constraints: {
    objective: "minimize_risk",
    targetReturn: null,
    riskAversion: 2.0,
    longOnly: true,
    maxSingleWeight: 0.3,
    assetClassBounds: null,
    sectorLimits: null,
    targetVolatility: null,
    maxTurnoverUsd: null,
    instrumentsToInclude: null,
    instrumentsToExclude: null,
    reasoning: "User wants to reduce risk exposure",
  },
  optimization: {
    targetWeights: { AAPL: 0.25, GOOG: 0.35, MSFT: 0.4 },
    trades: [
      {
        instrumentId: "AAPL",
        side: "sell",
        quantity: "20",
        estimatedCost: "3000.00",
      },
    ],
    expectedReturn: 0.09,
    expectedVolatility: 0.12,
    sharpeRatio: 0.75,
  },
  reasoning: "Reduced concentration in high-volatility positions",
};

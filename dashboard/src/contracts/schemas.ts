/**
 * Zod schemas for API response validation.
 *
 * These schemas are derived from the Go gateway structs and Python risk engine
 * Pydantic models. They serve as the contract between frontend and backend.
 *
 * Key serialization rules from both backends:
 * - Decimal values are JSON strings (e.g., "150.00"), never numbers
 * - Timestamps are ISO8601 strings
 * - Enums are lowercase strings
 * - Arrays are always [], never null
 */
import { z } from "zod";

// ── Shared enums ─────────────────────────────────────────────────────

export const AssetClassSchema = z.enum([
  "equity",
  "crypto",
  "tokenized_security",
  "future",
  "option",
]);

export const OrderStatusSchema = z.enum([
  "new",
  "acknowledged",
  "partially_filled",
  "filled",
  "canceled",
  "rejected",
]);

export const OrderSideSchema = z.enum(["buy", "sell"]);

export const OrderTypeSchema = z.enum(["market", "limit", "stop_limit"]);

export const VenueTypeSchema = z.enum([
  "exchange",
  "dark_pool",
  "simulated",
  "tokenized",
]);

export const VenueStatusSchema = z.enum([
  "connected",
  "disconnected",
  "degraded",
  "authentication",
]);

export const SeveritySchema = z.enum(["info", "warning", "critical"]);

// ── Gateway schemas ──────────────────────────────────────────────────

export const FillSchema = z.object({
  id: z.string(),
  orderId: z.string(),
  venueId: z.string(),
  quantity: z.string(),
  price: z.string(),
  fee: z.string(),
  feeAsset: z.string(),
  liquidity: z.enum(["maker", "taker", "internal"]),
  timestamp: z.string(),
});

export const OrderSchema = z.object({
  id: z.string(),
  clientOrderId: z.string(),
  instrumentId: z.string(),
  side: OrderSideSchema,
  type: OrderTypeSchema,
  quantity: z.string(),
  price: z.string(),
  filledQuantity: z.string(),
  averagePrice: z.string(),
  status: OrderStatusSchema,
  venueId: z.string(),
  assetClass: AssetClassSchema,
  createdAt: z.string(),
  updatedAt: z.string(),
  fills: z.array(FillSchema),
});

export const PositionSchema = z.object({
  instrumentId: z.string(),
  venueId: z.string(),
  quantity: z.string(),
  averageCost: z.string(),
  marketPrice: z.string(),
  unrealizedPnl: z.string(),
  realizedPnl: z.string(),
  unsettledQuantity: z.string(),
  assetClass: AssetClassSchema,
  quoteCurrency: z.string(),
});

export const InstrumentSchema = z.object({
  id: z.string(),
  symbol: z.string(),
  name: z.string(),
  assetClass: AssetClassSchema,
  baseCurrency: z.string(),
  quoteCurrency: z.string(),
  venueId: z.string(),
});

export const VenueSchema = z.object({
  id: z.string(),
  name: z.string(),
  type: VenueTypeSchema,
  status: VenueStatusSchema,
  supportedAssets: z.array(AssetClassSchema),
  latencyP50Ms: z.number(),
  latencyP99Ms: z.number(),
  fillRate: z.number(),
  lastHeartbeat: z.string(),
  hasCredentials: z.boolean(),
});

// ── Risk engine schemas ──────────────────────────────────────────────

export const VaRMetricsSchema = z.object({
  historicalVaR: z.string(),
  parametricVaR: z.string(),
  monteCarloVaR: z.string().nullable(),
  cvar: z.string(),
  confidence: z.number(),
  horizon: z.string(),
  computedAt: z.string(),
  monteCarloDistribution: z.array(z.number()).nullable(),
});

export const DrawdownDataSchema = z.object({
  current: z.number(),
  peak: z.string(),
  trough: z.string(),
  history: z.array(
    z.object({
      date: z.string(),
      drawdown: z.number(),
    }),
  ),
});

export const SettlementTimelineSchema = z.object({
  totalUnsettled: z.string(),
  entries: z.array(
    z.object({
      date: z.string(),
      amount: z.string(),
      instrumentId: z.string(),
      assetClass: AssetClassSchema,
    }),
  ),
});

export const GreeksSchema = z.object({
  delta: z.number(),
  gamma: z.number(),
  vega: z.number(),
  theta: z.number(),
  rho: z.number(),
});

export const PortfolioGreeksSchema = z.object({
  total: GreeksSchema,
  byInstrument: z.record(z.string(), GreeksSchema),
  computedAt: z.string(),
});

export const ConcentrationResultSchema = z.object({
  singleName: z.record(z.string(), z.number()),
  byAssetClass: z.record(z.string(), z.number()),
  byVenue: z.record(z.string(), z.number()),
  warnings: z.array(z.string()),
  hhi: z.number(),
});

export const PortfolioSummarySchema = z.object({
  totalNav: z.string(),
  totalPnl: z.string(),
  dailyPnl: z.string(),
  positionCount: z.number().int(),
});

export const ExposureDataSchema = z.object({
  byAssetClass: z.array(
    z.object({
      assetClass: AssetClassSchema,
      notional: z.string(),
      percentage: z.number(),
    }),
  ),
  byVenue: z.array(
    z.object({
      venueId: z.string(),
      notional: z.string(),
      percentage: z.number(),
    }),
  ),
});

// ── Optimizer schemas ────────────────────────────────────────────────

export const TradeActionSchema = z.object({
  instrumentId: z.string(),
  side: OrderSideSchema,
  quantity: z.string(),
  estimatedCost: z.string(),
});

export const OptimizationResultSchema = z.object({
  targetWeights: z.record(z.string(), z.number()),
  trades: z.array(TradeActionSchema),
  expectedReturn: z.number(),
  expectedVolatility: z.number(),
  sharpeRatio: z.number(),
});

// ── AI / Insight schemas ─────────────────────────────────────────────

export const ExecutionReportSchema = z.object({
  overallGrade: z.string(),
  implementationShortfallBps: z.number(),
  summary: z.string(),
  venueAnalysis: z.array(
    z.object({
      venue: z.string(),
      grade: z.string(),
      comment: z.string(),
    }),
  ),
  recommendations: z.array(z.string()),
  marketImpactEstimateBps: z.number(),
  orderId: z.string(),
  analyzedAt: z.string(),
});

export const AnomalyAlertSchema = z.object({
  id: z.string(),
  instrumentId: z.string(),
  venueId: z.string(),
  anomalyScore: z.number(),
  severity: SeveritySchema,
  features: z.record(z.string(), z.number()),
  description: z.string(),
  timestamp: z.string(),
  acknowledged: z.boolean(),
});

export const ExtractedConstraintsSchema = z.object({
  objective: z.string(),
  targetReturn: z.number().nullable(),
  riskAversion: z.number(),
  longOnly: z.boolean(),
  maxSingleWeight: z.number().nullable(),
  assetClassBounds: z.record(z.string(), z.tuple([z.number(), z.number()])).nullable(),
  sectorLimits: z.record(z.string(), z.number()).nullable(),
  targetVolatility: z.number().nullable(),
  maxTurnoverUsd: z.number().nullable(),
  instrumentsToInclude: z.array(z.string()).nullable(),
  instrumentsToExclude: z.array(z.string()).nullable(),
  reasoning: z.string(),
});

export const RebalanceResultSchema = z.object({
  constraints: ExtractedConstraintsSchema,
  optimization: OptimizationResultSchema,
  reasoning: z.string(),
});

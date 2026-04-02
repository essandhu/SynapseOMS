export type AssetClass =
  | "equity"
  | "crypto"
  | "tokenized_security"
  | "future"
  | "option";

export type SettlementCycle = "T0" | "T1" | "T2";

export type OrderStatus =
  | "new"
  | "acknowledged"
  | "partially_filled"
  | "filled"
  | "canceled"
  | "rejected";

export type OrderSide = "buy" | "sell";

export type OrderType = "market" | "limit" | "stop_limit";

export interface Fill {
  id: string;
  orderId: string;
  venueId: string;
  quantity: string;
  price: string;
  fee: string;
  feeAsset: string;
  liquidity: "maker" | "taker" | "internal";
  timestamp: string;
}

export interface Order {
  id: string;
  clientOrderId: string;
  instrumentId: string;
  side: OrderSide;
  type: OrderType;
  quantity: string;
  price: string;
  filledQuantity: string;
  averagePrice: string;
  status: OrderStatus;
  venueId: string;
  assetClass: AssetClass;
  createdAt: string;
  updatedAt: string;
  fills: Fill[];
}

export interface Position {
  instrumentId: string;
  venueId: string;
  quantity: string;
  averageCost: string;
  marketPrice: string;
  unrealizedPnl: string;
  realizedPnl: string;
  unsettledQuantity: string;
  assetClass: AssetClass;
  quoteCurrency: string;
}

export interface Venue {
  id: string;
  name: string;
  type: "exchange" | "dark_pool" | "simulated" | "tokenized";
  status: "connected" | "disconnected" | "degraded" | "authentication";
  supportedAssets: AssetClass[];
  latencyP50Ms: number;
  latencyP99Ms: number;
  fillRate: number;
  lastHeartbeat: string;
  hasCredentials: boolean;
}

/** WebSocket update envelope for order changes */
export interface OrderUpdate {
  type: "order_update";
  order: Order;
}

/** WebSocket update envelope for position changes */
export interface PositionUpdate {
  type: "position_update";
  position: Position;
}

/** Tradeable instrument */
export interface Instrument {
  id: string;
  symbol: string;
  name: string;
  assetClass: AssetClass;
  baseCurrency: string;
  quoteCurrency: string;
  venueId: string;
}

/** Request payload for submitting a new order */
export interface SubmitOrderRequest {
  instrumentId: string;
  side: OrderSide;
  type: OrderType;
  quantity: string;
  price?: string;
  venueId: string;
}

/** Value-at-Risk metrics from risk engine */
export interface VaRMetrics {
  historicalVaR: string;
  parametricVaR: string;
  monteCarloVaR: string | null;
  cvar: string;
  confidence: number;
  horizon: string;
  computedAt: string;
  monteCarloDistribution: number[] | null;
}

/** Drawdown tracking data */
export interface DrawdownData {
  current: number;
  peak: string;
  trough: string;
  history: { date: string; drawdown: number }[];
}

/** Settlement timeline data */
export interface SettlementTimeline {
  totalUnsettled: string;
  entries: {
    date: string;
    amount: string;
    instrumentId: string;
    assetClass: AssetClass;
  }[];
}

/** Portfolio summary from risk engine */
export interface PortfolioSummary {
  totalNav: string;
  totalPnl: string;
  dailyPnl: string;
  positionCount: number;
}

/** Exposure data from risk engine */
export interface ExposureData {
  byAssetClass: { assetClass: AssetClass; notional: string; percentage: number }[];
  byVenue: { venueId: string; notional: string; percentage: number }[];
}

/** WebSocket update envelope for risk changes */
export type RiskUpdate =
  | { type: "var_update"; payload: VaRMetrics }
  | { type: "drawdown_update"; payload: DrawdownData }
  | { type: "settlement_update"; payload: SettlementTimeline };

/** Constraints for portfolio optimization */
export interface OptimizationConstraints {
  riskAversion: number;
  longOnly: boolean;
  maxSingleWeight: number | null;
  targetVolatility: number | null;
  maxTurnover: number | null;
  assetClassBounds: Record<string, [number, number]> | null;
}

/** Result from the portfolio optimizer */
export interface OptimizationResult {
  targetWeights: Record<string, number>;
  trades: TradeAction[];
  expectedReturn: number;
  expectedVolatility: number;
  sharpeRatio: number;
}

/** A single trade action recommended by the optimizer */
export interface TradeAction {
  instrumentId: string;
  side: "buy" | "sell";
  quantity: string;
  estimatedCost: string;
}

/** Individual Greeks for an instrument or portfolio total */
export interface Greeks {
  delta: number;
  gamma: number;
  vega: number;
  theta: number;
  rho: number;
}

/** Portfolio-level Greeks with per-instrument breakdown */
export interface PortfolioGreeks {
  total: Greeks;
  byInstrument: Record<string, Greeks>;
  computedAt: string;
}

/** WebSocket update envelope for venue status changes */
export interface VenueStatusUpdate {
  type: "venue_connected" | "venue_disconnected" | "venue_degraded";
  venueId: string;
  status: string;
  latencyMs?: number;
}

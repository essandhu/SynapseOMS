import ky from "ky";
import type {
  DrawdownData,
  ExposureData,
  Instrument,
  OptimizationConstraints,
  OptimizationResult,
  Order,
  PortfolioSummary,
  Position,
  SettlementTimeline,
  VaRMetrics,
  Venue,
  SubmitOrderRequest,
} from "./types";

const api = ky.create({
  prefixUrl: import.meta.env.VITE_API_URL || "/api",
  timeout: 10_000,
  headers: {
    "Content-Type": "application/json",
  },
});

const riskApi = ky.create({
  prefixUrl: import.meta.env.VITE_RISK_API_URL || "http://localhost:8081",
  timeout: 10_000,
  headers: {
    "Content-Type": "application/json",
  },
});

/** Fetch all orders */
export async function fetchOrders(): Promise<Order[]> {
  return api.get("orders").json<Order[]>();
}

/** Fetch a single order by ID */
export async function fetchOrder(id: string): Promise<Order> {
  return api.get(`orders/${id}`).json<Order>();
}

/** Submit a new order */
export async function submitOrder(request: SubmitOrderRequest): Promise<Order> {
  return api.post("orders", { json: request }).json<Order>();
}

/** Cancel an order by ID */
export async function cancelOrder(id: string): Promise<void> {
  await api.delete(`orders/${id}`);
}

/** Fetch all positions */
export async function fetchPositions(): Promise<Position[]> {
  return api.get("positions").json<Position[]>();
}

/** Fetch all instruments */
export async function fetchInstruments(): Promise<Instrument[]> {
  return api.get("instruments").json<Instrument[]>();
}

/** Fetch all venues */
export async function fetchVenues(): Promise<Venue[]> {
  return api.get("venues").json<Venue[]>();
}

/** Connect a venue */
export async function connectVenue(venueId: string): Promise<void> {
  await api.post(`venues/${venueId}/connect`);
}

/** Disconnect a venue */
export async function disconnectVenue(venueId: string): Promise<void> {
  await api.post(`venues/${venueId}/disconnect`);
}

/** Store venue credentials */
export async function storeCredentials(
  venueId: string,
  apiKey: string,
  apiSecret: string,
  passphrase?: string,
): Promise<void> {
  await api.post("credentials", {
    json: { venueId, apiKey, apiSecret, passphrase },
  });
}

/** Delete venue credentials */
export async function deleteCredentials(venueId: string): Promise<void> {
  await api.delete(`credentials/${venueId}`);
}

// ── Risk Engine API ──────────────────────────────────────────────────

/** Fetch VaR metrics */
export async function fetchVaR(): Promise<VaRMetrics> {
  return riskApi.get("api/v1/risk/var").json<VaRMetrics>();
}

/** Fetch drawdown data */
export async function fetchDrawdown(): Promise<DrawdownData> {
  return riskApi.get("api/v1/risk/drawdown").json<DrawdownData>();
}

/** Fetch settlement timeline */
export async function fetchSettlement(): Promise<SettlementTimeline> {
  return riskApi.get("api/v1/risk/settlement").json<SettlementTimeline>();
}

/** Fetch portfolio summary */
export async function fetchPortfolioSummary(): Promise<PortfolioSummary> {
  return riskApi.get("api/v1/portfolio").json<PortfolioSummary>();
}

/** Fetch exposure data */
export async function fetchExposure(): Promise<ExposureData> {
  return riskApi.get("api/v1/portfolio/exposure").json<ExposureData>();
}

/** Run portfolio optimization with given constraints */
export async function optimizePortfolio(
  constraints: OptimizationConstraints,
): Promise<OptimizationResult> {
  return riskApi
    .post("api/v1/optimizer/optimize", { json: constraints })
    .json<OptimizationResult>();
}

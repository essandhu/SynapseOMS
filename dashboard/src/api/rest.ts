import ky from "ky";
import type {
  ConcentrationResult,
  DrawdownData,
  ExposureData,
  Instrument,
  OptimizationConstraints,
  OptimizationResult,
  Order,
  PortfolioGreeks,
  PortfolioSummary,
  Position,
  SettlementTimeline,
  VaRMetrics,
  Venue,
  SubmitOrderRequest,
  ExecutionReport,
  AnomalyAlert,
  RebalanceResult,
} from "./types";
import { mapOrder, mapPosition } from "./mappers";

/**
 * Error handler hook that fires a CustomEvent for UI toast notifications.
 * Components can listen for "api-error" events to display user-friendly messages.
 */
const afterResponseErrorHook = (
  _request: Request,
  _options: unknown,
  response: Response,
): void => {
  if (typeof window !== "undefined" && response.status >= 400) {
    window.dispatchEvent(
      new CustomEvent("api-error", {
        detail: {
          status: response.status,
          url: response.url,
          message: `Request failed: ${response.status} ${response.statusText}`,
        },
      }),
    );
  }
};

const api = ky.create({
  prefixUrl: import.meta.env.VITE_API_URL || "/api/v1",
  timeout: 10_000,
  retry: {
    limit: 3,
    methods: ["get", "head", "options"],
    statusCodes: [408, 500, 502, 503, 504],
    backoffLimit: 3000,
  },
  headers: {
    "Content-Type": "application/json",
  },
  hooks: {
    afterResponse: [afterResponseErrorHook as never],
  },
});

const riskApi = ky.create({
  prefixUrl: import.meta.env.VITE_RISK_API_URL || "http://localhost:8081",
  timeout: 10_000,
  retry: {
    limit: 3,
    methods: ["get", "head", "options"],
    statusCodes: [408, 500, 502, 503, 504],
    backoffLimit: 3000,
  },
  headers: {
    "Content-Type": "application/json",
  },
  hooks: {
    afterResponse: [afterResponseErrorHook as never],
  },
});

/** Fetch all orders */
export async function fetchOrders(): Promise<Order[]> {
  const raw = await api.get("orders").json<unknown[]>();
  return raw.map(mapOrder);
}

/** Fetch a single order by ID */
export async function fetchOrder(id: string): Promise<Order> {
  const raw = await api.get(`orders/${id}`).json<unknown>();
  return mapOrder(raw);
}

/** Submit a new order */
export async function submitOrder(request: SubmitOrderRequest): Promise<Order> {
  const raw = await api
    .post("orders", {
      json: {
        instrument_id: request.instrumentId,
        side: request.side,
        type: request.type,
        quantity: request.quantity,
        price: request.price,
        venue_id: request.venueId,
      },
    })
    .json<unknown>();
  return mapOrder(raw);
}

/** Cancel an order by ID */
export async function cancelOrder(id: string): Promise<void> {
  await api.delete(`orders/${id}`);
}

/** Fetch all positions */
export async function fetchPositions(): Promise<Position[]> {
  const raw = await api.get("positions").json<unknown[]>();
  return raw.map(mapPosition);
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

/** Check if onboarding has been completed */
export async function fetchOnboardingStatus(): Promise<boolean> {
  const res = await api.get("settings/onboarding_completed").json<{ completed: boolean }>();
  return res.completed;
}

/** Mark onboarding as completed */
export async function completeOnboarding(): Promise<void> {
  await api.post("settings/onboarding_completed");
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

/** Fetch portfolio Greeks */
export async function fetchGreeks(): Promise<PortfolioGreeks> {
  return riskApi.get("api/v1/risk/greeks").json<PortfolioGreeks>();
}

/** Fetch concentration risk analysis */
export async function fetchConcentration(): Promise<ConcentrationResult> {
  return riskApi.get("api/v1/risk/concentration").json<ConcentrationResult>();
}

/** Run portfolio optimization with given constraints */
export async function optimizePortfolio(
  constraints: OptimizationConstraints,
): Promise<OptimizationResult> {
  return riskApi
    .post("api/v1/optimizer/optimize", { json: constraints })
    .json<OptimizationResult>();
}

// ── AI & Anomaly Endpoints ──────────────────────────────────────────

/** Fetch AI execution reports */
export async function fetchExecutionReports(
  limit: number = 20,
): Promise<ExecutionReport[]> {
  return riskApi
    .get("api/v1/ai/execution-reports", { searchParams: { limit } })
    .json<ExecutionReport[]>();
}

/** Submit a natural language rebalancing prompt */
export async function submitRebalancePrompt(
  prompt: string,
): Promise<RebalanceResult> {
  return riskApi
    .post("api/v1/ai/rebalance", { json: { prompt } })
    .json<RebalanceResult>();
}

/** Fetch anomaly alerts */
export async function fetchAnomalyAlerts(params?: {
  limit?: number;
  severity?: string;
  instrumentId?: string;
}): Promise<{ alerts: AnomalyAlert[]; total: number }> {
  const searchParams: Record<string, string | number> = {};
  if (params?.limit) searchParams.limit = params.limit;
  if (params?.severity) searchParams.severity = params.severity;
  if (params?.instrumentId) searchParams.instrument_id = params.instrumentId;
  return riskApi
    .get("api/v1/anomalies", { searchParams })
    .json<{ alerts: AnomalyAlert[]; total: number }>();
}

/** Acknowledge an anomaly alert */
export async function acknowledgeAnomalyAlert(
  alertId: string,
): Promise<AnomalyAlert> {
  return riskApi
    .post(`api/v1/anomalies/${alertId}/acknowledge`)
    .json<AnomalyAlert>();
}

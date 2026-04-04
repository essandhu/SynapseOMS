import { http, HttpResponse } from "msw";
import {
  mockOrders,
  makeOrder,
  toRawOrder,
  mockPositions,
  toRawPosition,
  mockVenues,
  mockInstruments,
  mockVaR,
  mockDrawdown,
  mockSettlement,
  mockGreeks,
  mockConcentration,
  mockPortfolioSummary,
  mockExposure,
  mockOptimizationResult,
  mockExecutionReports,
  mockAnomalyAlerts,
  mockRebalanceResult,
} from "./data";

// ---------------------------------------------------------------------------
// Gateway handlers (ky prefixUrl: "/api/v1")
// In jsdom, requests go to http://localhost/api/v1/...
// ---------------------------------------------------------------------------

const gatewayHandlers = [
  http.get("*/api/v1/orders", () =>
    HttpResponse.json(mockOrders.map(toRawOrder)),
  ),

  http.get("*/api/v1/orders/:id", ({ params }) => {
    const order = mockOrders.find((o) => o.id === params.id);
    if (!order)
      return HttpResponse.json(
        { error: { code: "NOT_FOUND", message: "Order not found" } },
        { status: 404 },
      );
    return HttpResponse.json(toRawOrder(order));
  }),

  http.post("*/api/v1/orders", async ({ request }) => {
    const body = (await request.json()) as Record<string, unknown>;
    const order = makeOrder({
      id: `order-${Date.now()}`,
      instrumentId: (body.instrument_id ?? body.instrumentId) as string,
      side: body.side as "buy" | "sell",
      type: body.type as "market" | "limit" | "stop_limit",
      quantity: body.quantity as string,
      price: (body.price as string) || "0",
      venueId: ((body.venue_id ?? body.venueId) as string) || "smart",
      status: "new",
    });
    return HttpResponse.json(toRawOrder(order), { status: 201 });
  }),

  http.delete("*/api/v1/orders/:id", () => new HttpResponse(null, { status: 204 })),

  http.get("*/api/v1/positions", () => HttpResponse.json(mockPositions.map(toRawPosition))),

  http.get("*/api/v1/instruments", () => HttpResponse.json(mockInstruments)),

  http.get("*/api/v1/venues", () => HttpResponse.json(mockVenues)),

  http.post("*/api/v1/venues/:venueId/connect", () =>
    HttpResponse.json({ status: "connected" }),
  ),

  http.post("*/api/v1/venues/:venueId/disconnect", () =>
    HttpResponse.json({ status: "disconnected" }),
  ),

  http.post("*/api/v1/credentials", () => new HttpResponse(null, { status: 200 })),

  http.delete("*/api/v1/credentials/:venueId", () =>
    new HttpResponse(null, { status: 204 }),
  ),

  http.get("*/api/v1/settings/onboarding_completed", () =>
    HttpResponse.json({ completed: true }),
  ),

  http.post("*/api/v1/settings/onboarding_completed", () =>
    new HttpResponse(null, { status: 200 }),
  ),
];

// ---------------------------------------------------------------------------
// Risk Engine handlers (ky prefixUrl: "http://localhost:8081")
// ---------------------------------------------------------------------------

const riskEngineHandlers = [
  http.get("*/api/v1/risk/var", () => HttpResponse.json(mockVaR)),

  http.get("*/api/v1/risk/drawdown", () => HttpResponse.json(mockDrawdown)),

  http.get("*/api/v1/risk/settlement", () => HttpResponse.json(mockSettlement)),

  http.get("*/api/v1/risk/greeks", () => HttpResponse.json(mockGreeks)),

  http.get("*/api/v1/risk/concentration", () =>
    HttpResponse.json(mockConcentration),
  ),

  http.get("*/api/v1/portfolio", () => HttpResponse.json(mockPortfolioSummary)),

  http.get("*/api/v1/portfolio/exposure", () =>
    HttpResponse.json(mockExposure),
  ),

  http.post("*/api/v1/optimizer/optimize", () =>
    HttpResponse.json(mockOptimizationResult),
  ),

  http.get("*/api/v1/ai/execution-reports", () =>
    HttpResponse.json(mockExecutionReports),
  ),

  http.post("*/api/v1/ai/rebalance", () =>
    HttpResponse.json(mockRebalanceResult),
  ),

  http.get("*/api/v1/anomalies", () =>
    HttpResponse.json({ alerts: mockAnomalyAlerts, total: mockAnomalyAlerts.length }),
  ),

  http.post("*/api/v1/anomalies/:alertId/acknowledge", ({ params }) => {
    const alert = mockAnomalyAlerts.find((a) => a.id === params.alertId);
    if (!alert)
      return HttpResponse.json(
        { error: "Alert not found" },
        { status: 404 },
      );
    return HttpResponse.json({ ...alert, acknowledged: true });
  }),
];

export const handlers = [...gatewayHandlers, ...riskEngineHandlers];

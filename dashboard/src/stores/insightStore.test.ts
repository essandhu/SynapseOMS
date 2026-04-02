import { describe, it, expect, vi, beforeEach } from "vitest";
import { useInsightStore } from "./insightStore";
import type { AnomalyAlert, ExecutionReport } from "../api/types";

// Mock the rest module
vi.mock("../api/rest", () => ({
  fetchExecutionReports: vi.fn(),
  submitRebalancePrompt: vi.fn(),
  fetchAnomalyAlerts: vi.fn(),
  acknowledgeAnomalyAlert: vi.fn(),
}));

const mockAlert: AnomalyAlert = {
  id: "alert-1",
  instrumentId: "ETH-USD",
  venueId: "binance",
  anomalyScore: -0.65,
  severity: "warning",
  features: { volume_zscore: 4.2 },
  description: "Volume spike",
  timestamp: "2026-04-15T14:30:00Z",
  acknowledged: false,
};

const mockAlert2: AnomalyAlert = {
  id: "alert-2",
  instrumentId: "BTC-USD",
  venueId: "coinbase",
  anomalyScore: -0.85,
  severity: "critical",
  features: { spread_zscore: 6.1 },
  description: "Spread anomaly",
  timestamp: "2026-04-15T14:35:00Z",
  acknowledged: false,
};

describe("insightStore", () => {
  beforeEach(() => {
    // Reset store state between tests
    useInsightStore.setState({
      executionReports: [],
      anomalyAlerts: [],
      rebalanceState: { loading: false, result: null, error: null },
    });
    vi.clearAllMocks();
  });

  it("applyAnomalyAlert prepends to list", () => {
    useInsightStore.getState().applyAnomalyAlert(mockAlert);
    useInsightStore.getState().applyAnomalyAlert(mockAlert2);

    const alerts = useInsightStore.getState().anomalyAlerts;
    expect(alerts).toHaveLength(2);
    expect(alerts[0].id).toBe("alert-2");
    expect(alerts[1].id).toBe("alert-1");
  });

  it("acknowledgeAlert updates flag", async () => {
    const { acknowledgeAnomalyAlert } = await import("../api/rest");
    (acknowledgeAnomalyAlert as ReturnType<typeof vi.fn>).mockResolvedValue(
      undefined,
    );

    useInsightStore.getState().applyAnomalyAlert(mockAlert);
    await useInsightStore.getState().acknowledgeAlert("alert-1");

    const alerts = useInsightStore.getState().anomalyAlerts;
    expect(alerts[0].acknowledged).toBe(true);
  });

  it("unacknowledgedCount computes correctly", async () => {
    const { acknowledgeAnomalyAlert } = await import("../api/rest");
    (acknowledgeAnomalyAlert as ReturnType<typeof vi.fn>).mockResolvedValue(
      undefined,
    );

    useInsightStore.getState().applyAnomalyAlert(mockAlert);
    useInsightStore.getState().applyAnomalyAlert(mockAlert2);

    expect(useInsightStore.getState().unacknowledgedCount()).toBe(2);

    await useInsightStore.getState().acknowledgeAlert("alert-1");

    expect(useInsightStore.getState().unacknowledgedCount()).toBe(1);
  });

  it("clearRebalanceResult resets state", () => {
    useInsightStore.setState({
      rebalanceState: {
        loading: true,
        result: null,
        error: "something went wrong",
      },
    });

    useInsightStore.getState().clearRebalanceResult();

    const { rebalanceState } = useInsightStore.getState();
    expect(rebalanceState.loading).toBe(false);
    expect(rebalanceState.result).toBeNull();
    expect(rebalanceState.error).toBeNull();
  });

  it("fetchExecutionReports populates state", async () => {
    const mockReports: ExecutionReport[] = [
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

    const { fetchExecutionReports } = await import("../api/rest");
    (fetchExecutionReports as ReturnType<typeof vi.fn>).mockResolvedValue(
      mockReports,
    );

    await useInsightStore.getState().fetchExecutionReports();

    const reports = useInsightStore.getState().executionReports;
    expect(reports).toHaveLength(1);
    expect(reports[0].overallGrade).toBe("A");
    expect(reports[0].orderId).toBe("order-1");
  });
});

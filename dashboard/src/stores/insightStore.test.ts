import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../mocks/server";
import { useInsightStore } from "./insightStore";
import { mockAnomalyAlerts, mockExecutionReports } from "../mocks/data";
import type { AnomalyAlert } from "../api/types";

const mockAlert: AnomalyAlert = mockAnomalyAlerts[0];
const mockAlert2: AnomalyAlert = mockAnomalyAlerts[1];

describe("insightStore", () => {
  beforeEach(() => {
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
    useInsightStore.getState().applyAnomalyAlert(mockAlert);
    await useInsightStore.getState().acknowledgeAlert("alert-1");

    const alerts = useInsightStore.getState().anomalyAlerts;
    expect(alerts[0].acknowledged).toBe(true);
  });

  it("unacknowledgedCount computes correctly", async () => {
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
    await useInsightStore.getState().fetchExecutionReports();

    const reports = useInsightStore.getState().executionReports;
    expect(reports).toHaveLength(1);
    expect(reports[0].overallGrade).toBe("A");
    expect(reports[0].orderId).toBe("order-1");
  });
});

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { InsightsPanel } from "./InsightsPanel";

vi.mock("../stores/insightStore", () => ({
  useInsightStore: vi.fn(() => ({
    fetchExecutionReports: vi.fn(),
    fetchAnomalyAlerts: vi.fn(),
    unacknowledgedCount: vi.fn(() => 0),
    executionReports: [],
    anomalyAlerts: [],
    rebalanceState: { loading: false, result: null, error: null },
    submitRebalancePrompt: vi.fn(),
    clearRebalanceResult: vi.fn(),
    acknowledgeAlert: vi.fn(),
  })),
}));

vi.mock("../stores/orderStore", () => ({
  useOrderStore: vi.fn(() => ({
    submitOrder: vi.fn(),
  })),
}));

import { useInsightStore } from "../stores/insightStore";

describe("InsightsPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders three tabs", () => {
    render(<InsightsPanel />);
    expect(screen.getByText("Execution Analysis")).toBeDefined();
    expect(screen.getByText("Rebalancing")).toBeDefined();
    expect(screen.getByText("Anomaly Alerts")).toBeDefined();
  });

  it("switches content when clicking tabs", () => {
    render(<InsightsPanel />);
    // Default tab is execution - shows empty state
    expect(screen.getByText(/no execution reports/i)).toBeDefined();

    // Click rebalancing tab
    fireEvent.click(screen.getByText("Rebalancing"));
    expect(screen.getByTestId("rebalance-input")).toBeDefined();

    // Click anomaly tab
    fireEvent.click(screen.getByText("Anomaly Alerts"));
    expect(screen.getByText(/no anomalies detected/i)).toBeDefined();
  });

  it("shows alert count badge when unacknowledged alerts exist", () => {
    vi.mocked(useInsightStore).mockReturnValue({
      fetchExecutionReports: vi.fn(),
      fetchAnomalyAlerts: vi.fn(),
      unacknowledgedCount: vi.fn(() => 3),
      executionReports: [],
      anomalyAlerts: [],
      rebalanceState: { loading: false, result: null, error: null },
      submitRebalancePrompt: vi.fn(),
      clearRebalanceResult: vi.fn(),
      acknowledgeAlert: vi.fn(),
    } as any);
    render(<InsightsPanel />);
    expect(screen.getByTestId("alert-count-badge").textContent).toBe("3");
  });
});

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { AnomalyAlertsTab } from "./AnomalyAlertsTab";
import type { AnomalyAlert } from "../api/types";

const mockAcknowledgeAlert = vi.fn();

vi.mock("../stores/insightStore", () => ({
  useInsightStore: vi.fn(),
}));

import { useInsightStore } from "../stores/insightStore";

const mockAlert: AnomalyAlert = {
  id: "alert-1",
  instrumentId: "ETH-USD",
  venueId: "binance",
  anomalyScore: -0.65,
  severity: "warning",
  features: { volume_zscore: 4.2, spread_zscore: 1.1 },
  description: "ETH-USD volume on Binance 4.2x above 30-day mean",
  timestamp: new Date().toISOString(),
  acknowledged: false,
};

describe("AnomalyAlertsTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useInsightStore).mockReturnValue({
      anomalyAlerts: [],
      acknowledgeAlert: mockAcknowledgeAlert,
    } as any);
  });

  it("shows empty state when no alerts", () => {
    render(<AnomalyAlertsTab />);
    expect(screen.getByText(/no anomalies detected/i)).toBeDefined();
  });

  it("renders alert list", () => {
    vi.mocked(useInsightStore).mockReturnValue({
      anomalyAlerts: [mockAlert],
      acknowledgeAlert: mockAcknowledgeAlert,
    } as any);
    render(<AnomalyAlertsTab />);
    expect(screen.getByText("ETH-USD")).toBeDefined();
    expect(screen.getByText(/4.2x above/)).toBeDefined();
  });

  it("severity badge has correct color for warning", () => {
    vi.mocked(useInsightStore).mockReturnValue({
      anomalyAlerts: [mockAlert],
      acknowledgeAlert: mockAcknowledgeAlert,
    } as any);
    render(<AnomalyAlertsTab />);
    const badge = screen.getByTestId("severity-badge");
    expect(badge.style.backgroundColor).toBe("rgb(234, 179, 8)"); // yellow
  });

  it("acknowledge button calls store", () => {
    vi.mocked(useInsightStore).mockReturnValue({
      anomalyAlerts: [mockAlert],
      acknowledgeAlert: mockAcknowledgeAlert,
    } as any);
    render(<AnomalyAlertsTab />);
    fireEvent.click(screen.getByTestId("acknowledge-btn"));
    expect(mockAcknowledgeAlert).toHaveBeenCalledWith("alert-1");
  });

  it("acknowledged alert is muted", () => {
    vi.mocked(useInsightStore).mockReturnValue({
      anomalyAlerts: [{ ...mockAlert, acknowledged: true }],
      acknowledgeAlert: mockAcknowledgeAlert,
    } as any);
    render(<AnomalyAlertsTab />);
    const card = screen.getByTestId("alert-card");
    expect(card.className).toContain("opacity-50");
  });
});

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { AlertBadge } from "./AlertBadge";
import type { AnomalyAlert } from "../api/types";

vi.mock("../stores/insightStore", () => ({
  useInsightStore: vi.fn(),
}));

import { useInsightStore } from "../stores/insightStore";

const makeAlert = (
  overrides: Partial<AnomalyAlert> = {},
): AnomalyAlert => ({
  id: "a1",
  instrumentId: "ETH-USD",
  venueId: "binance",
  anomalyScore: -0.65,
  severity: "warning",
  features: {},
  description: "test",
  timestamp: new Date().toISOString(),
  acknowledged: false,
  ...overrides,
});

describe("AlertBadge", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("hidden when count is 0", () => {
    vi.mocked(useInsightStore).mockImplementation(((selector: any) =>
      selector({
        anomalyAlerts: [],
      })) as any);
    const { container } = render(<AlertBadge />);
    expect(container.innerHTML).toBe("");
  });

  it("shows correct count", () => {
    vi.mocked(useInsightStore).mockImplementation(((selector: any) =>
      selector({
        anomalyAlerts: [makeAlert(), makeAlert({ id: "a2" })],
      })) as any);
    render(<AlertBadge />);
    expect(screen.getByTestId("alert-badge").textContent).toBe("2");
  });

  it("color matches highest severity (critical > warning)", () => {
    vi.mocked(useInsightStore).mockImplementation(((selector: any) =>
      selector({
        anomalyAlerts: [
          makeAlert({ severity: "warning" }),
          makeAlert({ id: "a2", severity: "critical" }),
        ],
      })) as any);
    render(<AlertBadge />);
    const badge = screen.getByTestId("alert-badge");
    expect(badge.style.backgroundColor).toBe("rgb(239, 68, 68)"); // red
  });
});

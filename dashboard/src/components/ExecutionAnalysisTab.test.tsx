import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ExecutionAnalysisTab } from "./ExecutionAnalysisTab";
import type { ExecutionReport } from "../api/types";

const mockReport: ExecutionReport = {
  overallGrade: "B",
  implementationShortfallBps: 3.2,
  summary: "Solid execution with minor slippage",
  venueAnalysis: [
    { venue: "binance", grade: "A", comment: "Best fills" },
  ],
  recommendations: ["Consider increasing Binance allocation"],
  marketImpactEstimateBps: 1.5,
  orderId: "order-1",
  analyzedAt: "2026-04-15T14:30:00Z",
};

vi.mock("../stores/insightStore", () => ({
  useInsightStore: vi.fn(),
}));

import { useInsightStore } from "../stores/insightStore";

describe("ExecutionAnalysisTab", () => {
  beforeEach(() => {
    vi.mocked(useInsightStore).mockReturnValue({
      executionReports: [],
    } as any);
  });

  it("shows empty state when no reports", () => {
    render(<ExecutionAnalysisTab />);
    expect(screen.getByText(/no execution reports/i)).toBeDefined();
  });

  it("renders report list with grade badge", () => {
    vi.mocked(useInsightStore).mockReturnValue({
      executionReports: [mockReport],
    } as any);
    render(<ExecutionAnalysisTab />);
    expect(screen.getByText("B")).toBeDefined();
    expect(screen.getByText(/solid execution/i)).toBeDefined();
  });

  it("grade badge has correct color for B grade", () => {
    vi.mocked(useInsightStore).mockReturnValue({
      executionReports: [mockReport],
    } as any);
    render(<ExecutionAnalysisTab />);
    const badge = screen.getByTestId("grade-badge");
    expect(badge.style.backgroundColor).toBe("rgb(59, 130, 246)"); // blue #3b82f6
  });

  it("expands to show venue analysis", () => {
    vi.mocked(useInsightStore).mockReturnValue({
      executionReports: [mockReport],
    } as any);
    render(<ExecutionAnalysisTab />);
    fireEvent.click(screen.getByText(/show venue analysis/i));
    expect(screen.getByText("binance:")).toBeDefined();
    expect(screen.getByText("Best fills")).toBeDefined();
  });
});

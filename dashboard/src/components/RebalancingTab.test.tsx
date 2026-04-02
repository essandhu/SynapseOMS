import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { RebalancingTab } from "./RebalancingTab";

const mockSubmitRebalancePrompt = vi.fn();
const mockClearRebalanceResult = vi.fn();
const mockSubmitOrder = vi.fn();

vi.mock("../stores/insightStore", () => ({
  useInsightStore: vi.fn(),
}));

vi.mock("../stores/orderStore", () => ({
  useOrderStore: vi.fn(() => ({
    submitOrder: mockSubmitOrder,
  })),
}));

import { useInsightStore } from "../stores/insightStore";

describe("RebalancingTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useInsightStore).mockReturnValue({
      rebalanceState: { loading: false, result: null, error: null },
      submitRebalancePrompt: mockSubmitRebalancePrompt,
      clearRebalanceResult: mockClearRebalanceResult,
    } as any);
  });

  it("renders input and submit button", () => {
    render(<RebalancingTab />);
    expect(screen.getByTestId("rebalance-input")).toBeDefined();
    expect(screen.getByTestId("rebalance-submit")).toBeDefined();
  });

  it("submits prompt on button click", () => {
    render(<RebalancingTab />);
    const input = screen.getByTestId("rebalance-input") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "reduce crypto to 30%" } });
    fireEvent.click(screen.getByTestId("rebalance-submit"));
    expect(mockSubmitRebalancePrompt).toHaveBeenCalledWith("reduce crypto to 30%");
  });

  it("shows loading state", () => {
    vi.mocked(useInsightStore).mockReturnValue({
      rebalanceState: { loading: true, result: null, error: null },
      submitRebalancePrompt: mockSubmitRebalancePrompt,
      clearRebalanceResult: mockClearRebalanceResult,
    } as any);
    render(<RebalancingTab />);
    expect(screen.getByTestId("rebalance-loading")).toBeDefined();
  });

  it("shows error state", () => {
    vi.mocked(useInsightStore).mockReturnValue({
      rebalanceState: { loading: false, result: null, error: "API error" },
      submitRebalancePrompt: mockSubmitRebalancePrompt,
      clearRebalanceResult: mockClearRebalanceResult,
    } as any);
    render(<RebalancingTab />);
    expect(screen.getByTestId("rebalance-error")).toBeDefined();
    expect(screen.getByText("API error")).toBeDefined();
  });

  it("renders trade list in result", () => {
    vi.mocked(useInsightStore).mockReturnValue({
      rebalanceState: {
        loading: false,
        result: {
          constraints: { objective: "maximize_sharpe", riskAversion: 1.0 },
          optimization: {
            targetWeights: {},
            trades: [
              { instrumentId: "ETH-USD", side: "sell", quantity: "5", estimatedCost: "1500" },
            ],
            expectedReturn: 0.08,
            expectedVolatility: 0.15,
            sharpeRatio: 0.53,
          },
          reasoning: "Reducing crypto exposure to 30%",
        },
        error: null,
      },
      submitRebalancePrompt: mockSubmitRebalancePrompt,
      clearRebalanceResult: mockClearRebalanceResult,
    } as any);
    render(<RebalancingTab />);
    expect(screen.getByTestId("rebalance-result")).toBeDefined();
    expect(screen.getByText("ETH-USD")).toBeDefined();
    expect(screen.getByText("SELL")).toBeDefined();
    expect(screen.getByTestId("execute-all")).toBeDefined();
  });
});

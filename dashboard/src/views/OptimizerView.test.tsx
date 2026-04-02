import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { OptimizerView } from "./OptimizerView";
import type { OptimizationConstraints, OptimizationResult } from "../api/types";

// Mock store state
const mockSetConstraint = vi.fn();
const mockRunOptimize = vi.fn().mockResolvedValue(undefined);
const mockExecuteTradeList = vi.fn().mockResolvedValue(undefined);
const mockReset = vi.fn();

let mockStoreState: Record<string, unknown> = {};

vi.mock("../stores/optimizerStore", () => ({
  useOptimizerStore: (selector: (s: Record<string, unknown>) => unknown) => {
    return selector(mockStoreState);
  },
}));

const DEFAULT_CONSTRAINTS: OptimizationConstraints = {
  riskAversion: 1,
  longOnly: true,
  maxSingleWeight: null,
  targetVolatility: null,
  maxTurnover: null,
  assetClassBounds: null,
};

const MOCK_RESULT: OptimizationResult = {
  targetWeights: {
    "BTC/USD": 0.3,
    "ETH/USD": 0.2,
    "SPY": 0.5,
  },
  trades: [
    {
      instrumentId: "BTC/USD",
      side: "buy",
      quantity: "0.5",
      estimatedCost: "15000.00",
    },
    {
      instrumentId: "SPY",
      side: "sell",
      quantity: "10",
      estimatedCost: "4500.00",
    },
  ],
  expectedReturn: 0.12,
  expectedVolatility: 0.18,
  sharpeRatio: 0.667,
};

function resetMockState(overrides: Partial<Record<string, unknown>> = {}) {
  mockStoreState = {
    constraints: { ...DEFAULT_CONSTRAINTS },
    result: null,
    isOptimizing: false,
    error: null,
    setConstraint: mockSetConstraint,
    runOptimize: mockRunOptimize,
    executeTradeList: mockExecuteTradeList,
    reset: mockReset,
    ...overrides,
  };
}

describe("OptimizerView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetMockState();
  });

  it("renders constraint form with default values", () => {
    render(<OptimizerView />);

    // Title
    expect(screen.getByText("Portfolio Optimizer")).toBeInTheDocument();

    // Risk aversion slider
    expect(screen.getByLabelText(/risk aversion/i)).toBeInTheDocument();

    // Long-only toggle
    expect(screen.getByLabelText(/long.only/i)).toBeInTheDocument();

    // Optimize button
    expect(
      screen.getByRole("button", { name: /optimize/i }),
    ).toBeInTheDocument();
  });

  it("renders risk aversion slider with default value", () => {
    render(<OptimizerView />);

    const slider = screen.getByLabelText(/risk aversion/i);
    expect(slider).toHaveValue("1");
  });

  it("calls setConstraint when risk aversion changes", () => {
    render(<OptimizerView />);

    const slider = screen.getByLabelText(/risk aversion/i);
    // Use native event for range inputs (userEvent doesn't support range well)
    Object.getOwnPropertyDescriptor(
      HTMLInputElement.prototype,
      "value",
    )!.set!.call(slider, "5");
    slider.dispatchEvent(new Event("input", { bubbles: true }));

    expect(mockSetConstraint).toHaveBeenCalled();
  });

  it("triggers runOptimize when Optimize button is clicked", async () => {
    const user = userEvent.setup();
    render(<OptimizerView />);

    const optimizeBtn = screen.getByRole("button", { name: /optimize/i });
    await user.click(optimizeBtn);

    expect(mockRunOptimize).toHaveBeenCalledOnce();
  });

  it("shows loading state while optimizing", () => {
    resetMockState({ isOptimizing: true });
    render(<OptimizerView />);

    expect(screen.getByText(/optimizing/i)).toBeInTheDocument();
  });

  it("renders results panel after optimization", () => {
    resetMockState({ result: MOCK_RESULT });
    render(<OptimizerView />);

    // Metrics summary
    expect(screen.getByText(/expected return/i)).toBeInTheDocument();
    expect(screen.getByText("12.00%")).toBeInTheDocument();
    expect(screen.getByText(/expected volatility/i)).toBeInTheDocument();
    expect(screen.getByText("18.00%")).toBeInTheDocument();
    expect(screen.getByText(/sharpe ratio/i)).toBeInTheDocument();
    expect(screen.getByText("0.667")).toBeInTheDocument();
  });

  it("renders trade list from mock result", () => {
    resetMockState({ result: MOCK_RESULT });
    render(<OptimizerView />);

    // Trade list headers
    expect(screen.getByText("Trade List")).toBeInTheDocument();

    // Trade rows (may appear in both allocation and trade tables)
    expect(screen.getAllByText("BTC/USD").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("SPY").length).toBeGreaterThanOrEqual(1);

    // Side indicators
    const buyBadges = screen.getAllByText(/buy/i);
    expect(buyBadges.length).toBeGreaterThanOrEqual(1);
    const sellBadges = screen.getAllByText(/sell/i);
    expect(sellBadges.length).toBeGreaterThanOrEqual(1);
  });

  it("renders target allocation table from result", () => {
    resetMockState({ result: MOCK_RESULT });
    render(<OptimizerView />);

    expect(screen.getByText("Target Allocation")).toBeInTheDocument();
    // Target weights shown as percentages
    expect(screen.getByText("30.00%")).toBeInTheDocument(); // BTC 0.3
    expect(screen.getByText("20.00%")).toBeInTheDocument(); // ETH 0.2
    expect(screen.getByText("50.00%")).toBeInTheDocument(); // SPY 0.5
  });

  it("triggers executeTradeList when Execute All is clicked", async () => {
    const user = userEvent.setup();
    resetMockState({ result: MOCK_RESULT });
    render(<OptimizerView />);

    const executeBtn = screen.getByRole("button", { name: /execute all/i });
    await user.click(executeBtn);

    expect(mockExecuteTradeList).toHaveBeenCalledWith(MOCK_RESULT.trades);
  });

  it("displays error banner when error exists", () => {
    resetMockState({
      error:
        "Optimization infeasible: constraints on max single weight and asset class bounds are contradictory",
    });
    render(<OptimizerView />);

    expect(
      screen.getByText(/optimization infeasible/i),
    ).toBeInTheDocument();
  });

  it("shows max single weight input when enabled", async () => {
    const user = userEvent.setup();
    render(<OptimizerView />);

    // There should be a checkbox/toggle to enable max single weight
    const toggle = screen.getByLabelText(/max single weight/i);
    expect(toggle).toBeInTheDocument();
  });

  it("shows max turnover input", () => {
    render(<OptimizerView />);

    expect(screen.getByLabelText(/max turnover/i)).toBeInTheDocument();
  });

  it("renders asset class bounds section with add button", () => {
    render(<OptimizerView />);

    expect(screen.getByText(/asset class bounds/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /add/i }),
    ).toBeInTheDocument();
  });

  it("does not render results panel when no result", () => {
    render(<OptimizerView />);

    expect(screen.queryByText("Trade List")).not.toBeInTheDocument();
    expect(screen.queryByText("Target Allocation")).not.toBeInTheDocument();
  });
});

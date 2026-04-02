import { describe, it, expect, vi, beforeEach } from "vitest";
import { useOptimizerStore } from "./optimizerStore";
import type { OptimizationResult, TradeAction } from "../api/types";

// Mock the REST API module
vi.mock("../api/rest", () => ({
  optimizePortfolio: vi.fn(),
}));

// Mock the order store
vi.mock("./orderStore", () => ({
  useOrderStore: {
    getState: vi.fn(() => ({
      submitOrder: vi.fn(),
    })),
  },
}));

import { optimizePortfolio } from "../api/rest";
import { useOrderStore } from "./orderStore";

const mockResult: OptimizationResult = {
  targetWeights: { AAPL: 0.3, GOOG: 0.4, MSFT: 0.3 },
  trades: [
    { instrumentId: "AAPL", side: "buy", quantity: "100", estimatedCost: "15000.00" },
    { instrumentId: "GOOG", side: "sell", quantity: "50", estimatedCost: "7500.00" },
  ],
  expectedReturn: 0.12,
  expectedVolatility: 0.18,
  sharpeRatio: 0.67,
};

describe("optimizerStore", () => {
  beforeEach(() => {
    useOptimizerStore.setState({
      constraints: {
        riskAversion: 1,
        longOnly: true,
        maxSingleWeight: null,
        targetVolatility: null,
        maxTurnover: null,
        assetClassBounds: null,
      },
      result: null,
      isOptimizing: false,
      error: null,
    });
    vi.clearAllMocks();
  });

  it("has correct initial state", () => {
    const state = useOptimizerStore.getState();
    expect(state.constraints.riskAversion).toBe(1);
    expect(state.constraints.longOnly).toBe(true);
    expect(state.constraints.maxSingleWeight).toBeNull();
    expect(state.constraints.targetVolatility).toBeNull();
    expect(state.constraints.maxTurnover).toBeNull();
    expect(state.constraints.assetClassBounds).toBeNull();
    expect(state.result).toBeNull();
    expect(state.isOptimizing).toBe(false);
    expect(state.error).toBeNull();
  });

  it("setConstraint updates a single constraint", () => {
    useOptimizerStore.getState().setConstraint("riskAversion", 2.5);
    expect(useOptimizerStore.getState().constraints.riskAversion).toBe(2.5);

    useOptimizerStore.getState().setConstraint("longOnly", false);
    expect(useOptimizerStore.getState().constraints.longOnly).toBe(false);

    useOptimizerStore.getState().setConstraint("maxSingleWeight", 0.25);
    expect(useOptimizerStore.getState().constraints.maxSingleWeight).toBe(0.25);
  });

  it("setConstraint preserves other constraints", () => {
    useOptimizerStore.getState().setConstraint("riskAversion", 3);
    const state = useOptimizerStore.getState();
    expect(state.constraints.riskAversion).toBe(3);
    expect(state.constraints.longOnly).toBe(true);
    expect(state.constraints.maxSingleWeight).toBeNull();
  });

  it("runOptimize calls API and stores result", async () => {
    vi.mocked(optimizePortfolio).mockResolvedValue(mockResult);

    await useOptimizerStore.getState().runOptimize();

    expect(optimizePortfolio).toHaveBeenCalledOnce();
    expect(optimizePortfolio).toHaveBeenCalledWith(
      useOptimizerStore.getState().constraints,
    );
    const state = useOptimizerStore.getState();
    expect(state.result).toEqual(mockResult);
    expect(state.isOptimizing).toBe(false);
    expect(state.error).toBeNull();
  });

  it("runOptimize sets isOptimizing during request", async () => {
    let resolvePromise: (value: OptimizationResult) => void;
    vi.mocked(optimizePortfolio).mockImplementation(
      () => new Promise((resolve) => { resolvePromise = resolve; }),
    );

    const promise = useOptimizerStore.getState().runOptimize();
    expect(useOptimizerStore.getState().isOptimizing).toBe(true);

    resolvePromise!(mockResult);
    await promise;

    expect(useOptimizerStore.getState().isOptimizing).toBe(false);
  });

  it("runOptimize sets error on API failure", async () => {
    vi.mocked(optimizePortfolio).mockRejectedValue(new Error("Optimization failed"));

    await useOptimizerStore.getState().runOptimize();

    const state = useOptimizerStore.getState();
    expect(state.result).toBeNull();
    expect(state.isOptimizing).toBe(false);
    expect(state.error).toBe("Optimization failed");
  });

  it("runOptimize sends current constraints to API", async () => {
    vi.mocked(optimizePortfolio).mockResolvedValue(mockResult);

    useOptimizerStore.getState().setConstraint("riskAversion", 5);
    useOptimizerStore.getState().setConstraint("longOnly", false);
    useOptimizerStore.getState().setConstraint("maxSingleWeight", 0.1);

    await useOptimizerStore.getState().runOptimize();

    expect(optimizePortfolio).toHaveBeenCalledWith({
      riskAversion: 5,
      longOnly: false,
      maxSingleWeight: 0.1,
      targetVolatility: null,
      maxTurnover: null,
      assetClassBounds: null,
    });
  });

  it("executeTradeList submits N orders via orderStore", async () => {
    const mockSubmitOrder = vi.fn().mockResolvedValue(undefined);
    vi.mocked(useOrderStore.getState).mockReturnValue({
      submitOrder: mockSubmitOrder,
    } as any);

    const trades: TradeAction[] = [
      { instrumentId: "AAPL", side: "buy", quantity: "100", estimatedCost: "15000.00" },
      { instrumentId: "GOOG", side: "sell", quantity: "50", estimatedCost: "7500.00" },
      { instrumentId: "MSFT", side: "buy", quantity: "75", estimatedCost: "11000.00" },
    ];

    await useOptimizerStore.getState().executeTradeList(trades);

    expect(mockSubmitOrder).toHaveBeenCalledTimes(3);
    expect(mockSubmitOrder).toHaveBeenCalledWith({
      instrumentId: "AAPL",
      side: "buy",
      type: "market",
      quantity: "100",
      venueId: "default",
    });
    expect(mockSubmitOrder).toHaveBeenCalledWith({
      instrumentId: "GOOG",
      side: "sell",
      type: "market",
      quantity: "50",
      venueId: "default",
    });
    expect(mockSubmitOrder).toHaveBeenCalledWith({
      instrumentId: "MSFT",
      side: "buy",
      type: "market",
      quantity: "75",
      venueId: "default",
    });
  });

  it("executeTradeList sets error if a submission fails", async () => {
    const mockSubmitOrder = vi.fn()
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error("Order rejected"));
    vi.mocked(useOrderStore.getState).mockReturnValue({
      submitOrder: mockSubmitOrder,
    } as any);

    const trades: TradeAction[] = [
      { instrumentId: "AAPL", side: "buy", quantity: "100", estimatedCost: "15000.00" },
      { instrumentId: "GOOG", side: "sell", quantity: "50", estimatedCost: "7500.00" },
    ];

    await useOptimizerStore.getState().executeTradeList(trades);

    const state = useOptimizerStore.getState();
    expect(state.error).toBe("Order rejected");
  });

  it("reset restores initial state", () => {
    // Set some non-default state
    useOptimizerStore.setState({
      constraints: {
        riskAversion: 5,
        longOnly: false,
        maxSingleWeight: 0.2,
        targetVolatility: 0.15,
        maxTurnover: 0.3,
        assetClassBounds: { equity: [0.1, 0.5] },
      },
      result: mockResult,
      isOptimizing: false,
      error: "some error",
    });

    useOptimizerStore.getState().reset();

    const state = useOptimizerStore.getState();
    expect(state.constraints.riskAversion).toBe(1);
    expect(state.constraints.longOnly).toBe(true);
    expect(state.constraints.maxSingleWeight).toBeNull();
    expect(state.result).toBeNull();
    expect(state.isOptimizing).toBe(false);
    expect(state.error).toBeNull();
  });
});

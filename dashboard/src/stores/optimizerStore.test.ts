import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../mocks/server";
import { useOptimizerStore } from "./optimizerStore";
import { mockOptimizationResult } from "../mocks/data";
import type { OptimizationResult, TradeAction } from "../api/types";

// Mock the order store (cross-store dependency, not an API call)
vi.mock("./orderStore", () => ({
  useOrderStore: {
    getState: vi.fn(() => ({
      submitOrder: vi.fn(),
    })),
  },
}));

import { useOrderStore } from "./orderStore";

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
    await useOptimizerStore.getState().runOptimize();

    const state = useOptimizerStore.getState();
    expect(state.result).toEqual(mockOptimizationResult);
    expect(state.isOptimizing).toBe(false);
    expect(state.error).toBeNull();
  });

  it("runOptimize sets error on API failure", async () => {
    server.use(
      http.post("*/api/v1/optimizer/optimize", () =>
        HttpResponse.json(
          { message: "Optimization failed" },
          { status: 422 },
        ),
      ),
    );

    await useOptimizerStore.getState().runOptimize();

    const state = useOptimizerStore.getState();
    expect(state.result).toBeNull();
    expect(state.isOptimizing).toBe(false);
    expect(state.error).toBeTruthy();
  });

  it("executeTradeList submits N orders via orderStore", async () => {
    const mockSubmitOrder = vi.fn().mockResolvedValue(undefined);
    vi.mocked(useOrderStore.getState).mockReturnValue({
      submitOrder: mockSubmitOrder,
    } as any);

    const trades: TradeAction[] = [
      {
        instrumentId: "AAPL",
        side: "buy",
        quantity: "100",
        estimatedCost: "15000.00",
      },
      {
        instrumentId: "GOOG",
        side: "sell",
        quantity: "50",
        estimatedCost: "7500.00",
      },
      {
        instrumentId: "MSFT",
        side: "buy",
        quantity: "75",
        estimatedCost: "11000.00",
      },
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
  });

  it("executeTradeList sets error if a submission fails", async () => {
    const mockSubmitOrder = vi
      .fn()
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error("Order rejected"));
    vi.mocked(useOrderStore.getState).mockReturnValue({
      submitOrder: mockSubmitOrder,
    } as any);

    const trades: TradeAction[] = [
      {
        instrumentId: "AAPL",
        side: "buy",
        quantity: "100",
        estimatedCost: "15000.00",
      },
      {
        instrumentId: "GOOG",
        side: "sell",
        quantity: "50",
        estimatedCost: "7500.00",
      },
    ];

    await useOptimizerStore.getState().executeTradeList(trades);

    const state = useOptimizerStore.getState();
    expect(state.error).toBe("Order rejected");
  });

  it("reset restores initial state", () => {
    useOptimizerStore.setState({
      constraints: {
        riskAversion: 5,
        longOnly: false,
        maxSingleWeight: 0.2,
        targetVolatility: 0.15,
        maxTurnover: 0.3,
        assetClassBounds: { equity: [0.1, 0.5] },
      },
      result: mockOptimizationResult,
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

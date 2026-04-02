import { describe, it, expect, vi, beforeEach } from "vitest";
import { useRiskStore } from "./riskStore";
import type { VaRMetrics, DrawdownData, SettlementTimeline } from "../api/types";

// Mock the REST API module
vi.mock("../api/rest", () => ({
  fetchVaR: vi.fn(),
  fetchDrawdown: vi.fn(),
  fetchSettlement: vi.fn(),
}));

// Mock the WebSocket module
vi.mock("../api/ws", () => ({
  createRiskStream: vi.fn(() => ({ close: vi.fn() })),
}));

// Import mocked modules so we can control their return values
import { fetchVaR, fetchDrawdown, fetchSettlement } from "../api/rest";

const mockVaR: VaRMetrics = {
  historicalVaR: "12500.00",
  parametricVaR: "11200.00",
  monteCarloVaR: null,
  cvar: "15800.00",
  confidence: 95,
  horizon: "1d",
  computedAt: "2026-04-01T10:00:00Z",
  monteCarloDistribution: null,
};

const mockDrawdown: DrawdownData = {
  current: 0.032,
  peak: "105000.00",
  trough: "101640.00",
  history: [
    { date: "2026-03-28", drawdown: 0.01 },
    { date: "2026-03-29", drawdown: 0.025 },
    { date: "2026-03-30", drawdown: 0.032 },
  ],
};

const mockSettlement: SettlementTimeline = {
  totalUnsettled: "25000.00",
  entries: [
    {
      date: "2026-04-02",
      amount: "15000.00",
      instrumentId: "AAPL",
      assetClass: "equity",
    },
    {
      date: "2026-04-03",
      amount: "10000.00",
      instrumentId: "BTC/USD",
      assetClass: "crypto",
    },
  ],
};

describe("riskStore", () => {
  beforeEach(() => {
    // Reset the store to initial state between tests
    useRiskStore.setState({
      var: null,
      drawdown: null,
      settlement: null,
      loading: false,
      error: null,
    });
    vi.clearAllMocks();
  });

  it("has null/empty initial state", () => {
    const state = useRiskStore.getState();
    expect(state.var).toBeNull();
    expect(state.drawdown).toBeNull();
    expect(state.settlement).toBeNull();
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
  });

  it("applyUpdate with var_update sets VaR metrics", () => {
    useRiskStore.getState().applyUpdate({
      type: "var_update",
      payload: mockVaR,
    });

    const state = useRiskStore.getState();
    expect(state.var).toEqual(mockVaR);
    expect(state.var?.historicalVaR).toBe("12500.00");
    expect(state.var?.confidence).toBe(95);
  });

  it("applyUpdate with drawdown_update sets drawdown data", () => {
    useRiskStore.getState().applyUpdate({
      type: "drawdown_update",
      payload: mockDrawdown,
    });

    const state = useRiskStore.getState();
    expect(state.drawdown).toEqual(mockDrawdown);
    expect(state.drawdown?.current).toBe(0.032);
    expect(state.drawdown?.history).toHaveLength(3);
  });

  it("applyUpdate with settlement_update sets settlement timeline", () => {
    useRiskStore.getState().applyUpdate({
      type: "settlement_update",
      payload: mockSettlement,
    });

    const state = useRiskStore.getState();
    expect(state.settlement).toEqual(mockSettlement);
    expect(state.settlement?.totalUnsettled).toBe("25000.00");
    expect(state.settlement?.entries).toHaveLength(2);
  });

  it("fetchVaR calls the correct API endpoint and sets state", async () => {
    vi.mocked(fetchVaR).mockResolvedValue(mockVaR);

    await useRiskStore.getState().fetchVaR();

    expect(fetchVaR).toHaveBeenCalledOnce();
    const state = useRiskStore.getState();
    expect(state.var).toEqual(mockVaR);
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
  });

  it("fetchVaR sets error on failure", async () => {
    vi.mocked(fetchVaR).mockRejectedValue(new Error("Network error"));

    await useRiskStore.getState().fetchVaR();

    const state = useRiskStore.getState();
    expect(state.var).toBeNull();
    expect(state.loading).toBe(false);
    expect(state.error).toBe("Network error");
  });

  it("fetchDrawdown calls the correct API endpoint and sets state", async () => {
    vi.mocked(fetchDrawdown).mockResolvedValue(mockDrawdown);

    await useRiskStore.getState().fetchDrawdown();

    expect(fetchDrawdown).toHaveBeenCalledOnce();
    const state = useRiskStore.getState();
    expect(state.drawdown).toEqual(mockDrawdown);
    expect(state.loading).toBe(false);
  });

  it("fetchDrawdown sets error on failure", async () => {
    vi.mocked(fetchDrawdown).mockRejectedValue(new Error("Server unavailable"));

    await useRiskStore.getState().fetchDrawdown();

    const state = useRiskStore.getState();
    expect(state.drawdown).toBeNull();
    expect(state.error).toBe("Server unavailable");
  });

  it("fetchSettlement calls the correct API endpoint and sets state", async () => {
    vi.mocked(fetchSettlement).mockResolvedValue(mockSettlement);

    await useRiskStore.getState().fetchSettlement();

    expect(fetchSettlement).toHaveBeenCalledOnce();
    const state = useRiskStore.getState();
    expect(state.settlement).toEqual(mockSettlement);
    expect(state.loading).toBe(false);
  });

  it("sets loading to true during fetch", async () => {
    let resolvePromise: (value: VaRMetrics) => void;
    vi.mocked(fetchVaR).mockImplementation(
      () => new Promise((resolve) => { resolvePromise = resolve; }),
    );

    const fetchPromise = useRiskStore.getState().fetchVaR();
    expect(useRiskStore.getState().loading).toBe(true);

    resolvePromise!(mockVaR);
    await fetchPromise;

    expect(useRiskStore.getState().loading).toBe(false);
  });
});

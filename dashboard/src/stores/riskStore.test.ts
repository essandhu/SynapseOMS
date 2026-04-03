import { describe, it, expect, vi, beforeEach } from "vitest";
import { useRiskStore } from "./riskStore";
import type {
  VaRMetrics,
  DrawdownData,
  SettlementTimeline,
  PortfolioGreeks,
  ConcentrationResult,
} from "../api/types";

// Mock the REST API module
vi.mock("../api/rest", () => ({
  fetchVaR: vi.fn(),
  fetchDrawdown: vi.fn(),
  fetchSettlement: vi.fn(),
  fetchGreeks: vi.fn(),
  fetchConcentration: vi.fn(),
}));

// Import mocked modules so we can control their return values
import {
  fetchVaR,
  fetchDrawdown,
  fetchSettlement,
  fetchGreeks,
  fetchConcentration,
} from "../api/rest";

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

const mockGreeks: PortfolioGreeks = {
  total: { delta: 0.85, gamma: 0.02, vega: 15.3, theta: -2.1, rho: 0.5 },
  byInstrument: {
    AAPL: { delta: 0.45, gamma: 0.01, vega: 8.0, theta: -1.0, rho: 0.3 },
    "BTC/USD": { delta: 0.4, gamma: 0.01, vega: 7.3, theta: -1.1, rho: 0.2 },
  },
  computedAt: "2026-04-01T10:00:00Z",
};

const mockConcentration: ConcentrationResult = {
  singleName: { AAPL: 35, "BTC/USD": 25, ETH: 20, GOOG: 20 },
  byAssetClass: { equity: 55, crypto: 45 },
  byVenue: { "venue-1": 60, "venue-2": 40 },
  warnings: ["AAPL exceeds 30% single-name threshold"],
  hhi: 2450,
};

describe("riskStore", () => {
  beforeEach(() => {
    // Reset the store to initial state between tests
    useRiskStore.setState({
      var: null,
      drawdown: null,
      settlement: null,
      greeks: null,
      concentration: null,
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
    expect(state.greeks).toBeNull();
    expect(state.concentration).toBeNull();
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

  it("fetchGreeks calls API and sets greeks state", async () => {
    vi.mocked(fetchGreeks).mockResolvedValue(mockGreeks);

    await useRiskStore.getState().fetchGreeks();

    expect(fetchGreeks).toHaveBeenCalledOnce();
    const state = useRiskStore.getState();
    expect(state.greeks).toEqual(mockGreeks);
    expect(state.greeks?.total.delta).toBe(0.85);
  });

  it("fetchGreeks silently handles errors without setting error state", async () => {
    vi.mocked(fetchGreeks).mockRejectedValue(new Error("Greeks unavailable"));

    await useRiskStore.getState().fetchGreeks();

    const state = useRiskStore.getState();
    expect(state.greeks).toBeNull();
    expect(state.error).toBeNull(); // Should NOT set error
  });

  it("fetchConcentration calls API and sets concentration state", async () => {
    vi.mocked(fetchConcentration).mockResolvedValue(mockConcentration);

    await useRiskStore.getState().fetchConcentration();

    expect(fetchConcentration).toHaveBeenCalledOnce();
    const state = useRiskStore.getState();
    expect(state.concentration).toEqual(mockConcentration);
    expect(state.concentration?.hhi).toBe(2450);
    expect(state.concentration?.warnings).toHaveLength(1);
  });

  it("fetchConcentration silently handles errors without setting error state", async () => {
    vi.mocked(fetchConcentration).mockRejectedValue(
      new Error("Concentration unavailable"),
    );

    await useRiskStore.getState().fetchConcentration();

    const state = useRiskStore.getState();
    expect(state.concentration).toBeNull();
    expect(state.error).toBeNull(); // Should NOT set error
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

import { create } from "zustand";
import type {
  ConcentrationResult,
  PortfolioGreeks,
  VaRMetrics,
  DrawdownData,
  SettlementTimeline,
  RiskUpdate,
} from "../api/types";
import {
  fetchVaR as apiFetchVaR,
  fetchDrawdown as apiFetchDrawdown,
  fetchSettlement as apiFetchSettlement,
  fetchGreeks as apiFetchGreeks,
  fetchConcentration as apiFetchConcentration,
} from "../api/rest";

export interface RiskStoreState {
  /** Current VaR metrics */
  var: VaRMetrics | null;

  /** Current drawdown data */
  drawdown: DrawdownData | null;

  /** Current settlement timeline */
  settlement: SettlementTimeline | null;

  /** Current portfolio Greeks */
  greeks: PortfolioGreeks | null;

  /** Current concentration risk analysis */
  concentration: ConcentrationResult | null;

  /** Whether the store is currently loading */
  loading: boolean;

  /** Last error message */
  error: string | null;

  /** Fetch VaR metrics from the risk engine */
  fetchVaR: () => Promise<void>;

  /** Fetch drawdown data from the risk engine */
  fetchDrawdown: () => Promise<void>;

  /** Fetch settlement timeline from the risk engine */
  fetchSettlement: () => Promise<void>;

  /** Fetch portfolio Greeks from the risk engine */
  fetchGreeks: () => Promise<void>;

  /** Fetch concentration risk from the risk engine */
  fetchConcentration: () => Promise<void>;

  /** Apply a real-time risk update from WebSocket */
  applyUpdate: (update: RiskUpdate) => void;

  /** Subscribe to real-time risk updates via WebSocket and load initial data */
  subscribe: () => () => void;
}

export const useRiskStore = create<RiskStoreState>()((set, get) => ({
  var: null,
  drawdown: null,
  settlement: null,
  greeks: null,
  concentration: null,
  loading: false,
  error: null,

  fetchVaR: async (): Promise<void> => {
    set({ loading: true, error: null });
    try {
      const data = await apiFetchVaR();
      set({ var: data, loading: false });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to fetch VaR metrics";
      set({ loading: false, error: message });
    }
  },

  fetchDrawdown: async (): Promise<void> => {
    set({ loading: true, error: null });
    try {
      const data = await apiFetchDrawdown();
      set({ drawdown: data, loading: false });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to fetch drawdown data";
      set({ loading: false, error: message });
    }
  },

  fetchSettlement: async (): Promise<void> => {
    set({ loading: true, error: null });
    try {
      const data = await apiFetchSettlement();
      set({ settlement: data, loading: false });
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Failed to fetch settlement timeline";
      set({ loading: false, error: message });
    }
  },

  fetchGreeks: async (): Promise<void> => {
    try {
      const data = await apiFetchGreeks();
      set({ greeks: data });
    } catch {
      // Greeks are supplementary — don't overwrite primary error state
    }
  },

  fetchConcentration: async (): Promise<void> => {
    try {
      const data = await apiFetchConcentration();
      set({ concentration: data });
    } catch {
      // Concentration is supplementary — don't overwrite primary error state
    }
  },

  applyUpdate: (update: RiskUpdate): void => {
    switch (update.type) {
      case "var_update":
        set({ var: update.payload });
        break;
      case "drawdown_update":
        set({ drawdown: update.payload });
        break;
      case "settlement_update":
        set({ settlement: update.payload });
        break;
    }
  },

  subscribe: (): (() => void) => {
    // Load initial risk data
    const fetchAll = () => {
      get().fetchVaR();
      get().fetchDrawdown();
      get().fetchSettlement();
      get().fetchGreeks();
      get().fetchConcentration();
    };

    fetchAll();

    // Poll risk data every 30 seconds (risk engine is REST-only, no WebSocket)
    const interval = setInterval(fetchAll, 30_000);

    return () => {
      clearInterval(interval);
    };
  },
}));

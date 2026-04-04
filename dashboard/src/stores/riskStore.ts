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

/** Tracks in-flight fetches so loading goes false only when all complete. */
export let pendingCount = 0;

/** Reset pending counter (for tests only). */
export function resetPendingCount(): void {
  pendingCount = 0;
}

function startFetch(set: (partial: Partial<RiskStoreState>) => void): void {
  if (pendingCount === 0) set({ loading: true, error: null });
  pendingCount++;
}

function endFetch(set: (partial: Partial<RiskStoreState>) => void): void {
  pendingCount = Math.max(0, pendingCount - 1);
  if (pendingCount === 0) set({ loading: false });
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
    startFetch(set);
    try {
      const data = await apiFetchVaR();
      set({ var: data });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to fetch VaR metrics";
      set((s) => ({ error: s.error ?? message }));
    } finally {
      endFetch(set);
    }
  },

  fetchDrawdown: async (): Promise<void> => {
    startFetch(set);
    try {
      const data = await apiFetchDrawdown();
      set({ drawdown: data });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to fetch drawdown data";
      set((s) => ({ error: s.error ?? message }));
    } finally {
      endFetch(set);
    }
  },

  fetchSettlement: async (): Promise<void> => {
    startFetch(set);
    try {
      const data = await apiFetchSettlement();
      set({ settlement: data });
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Failed to fetch settlement timeline";
      set((s) => ({ error: s.error ?? message }));
    } finally {
      endFetch(set);
    }
  },

  fetchGreeks: async (): Promise<void> => {
    startFetch(set);
    try {
      const data = await apiFetchGreeks();
      set({ greeks: data });
    } catch {
      // Greeks are supplementary — don't overwrite primary error state
    } finally {
      endFetch(set);
    }
  },

  fetchConcentration: async (): Promise<void> => {
    startFetch(set);
    try {
      const data = await apiFetchConcentration();
      set({ concentration: data });
    } catch {
      // Concentration is supplementary — don't overwrite primary error state
    } finally {
      endFetch(set);
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

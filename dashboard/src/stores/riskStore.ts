import { create } from "zustand";
import type {
  VaRMetrics,
  DrawdownData,
  SettlementTimeline,
  RiskUpdate,
} from "../api/types";
import {
  fetchVaR as apiFetchVaR,
  fetchDrawdown as apiFetchDrawdown,
  fetchSettlement as apiFetchSettlement,
} from "../api/rest";
import { createRiskStream } from "../api/ws";
import type ReconnectingWebSocket from "reconnecting-websocket";

export interface RiskStoreState {
  /** Current VaR metrics */
  var: VaRMetrics | null;

  /** Current drawdown data */
  drawdown: DrawdownData | null;

  /** Current settlement timeline */
  settlement: SettlementTimeline | null;

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

  /** Apply a real-time risk update from WebSocket */
  applyUpdate: (update: RiskUpdate) => void;

  /** Subscribe to real-time risk updates via WebSocket and load initial data */
  subscribe: () => () => void;
}

export const useRiskStore = create<RiskStoreState>()((set, get) => ({
  var: null,
  drawdown: null,
  settlement: null,
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
    get().fetchVaR();
    get().fetchDrawdown();
    get().fetchSettlement();

    // Connect WebSocket for real-time updates
    let ws: ReconnectingWebSocket | null = createRiskStream((update) => {
      get().applyUpdate(update);
    });

    // Return unsubscribe function
    return () => {
      if (ws) {
        ws.close();
        ws = null;
      }
    };
  },
}));

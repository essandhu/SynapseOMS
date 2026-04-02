import { create } from "zustand";
import type {
  ExecutionReport,
  AnomalyAlert,
  RebalanceResult,
} from "../api/types";
import {
  fetchExecutionReports as apiFetchReports,
  submitRebalancePrompt as apiRebalance,
  fetchAnomalyAlerts as apiFetchAlerts,
  acknowledgeAnomalyAlert as apiAcknowledge,
} from "../api/rest";

export interface InsightStoreState {
  executionReports: ExecutionReport[];
  anomalyAlerts: AnomalyAlert[];
  rebalanceState: {
    loading: boolean;
    result: RebalanceResult | null;
    error: string | null;
  };

  fetchExecutionReports: () => Promise<void>;
  submitRebalancePrompt: (prompt: string) => Promise<void>;
  clearRebalanceResult: () => void;
  applyAnomalyAlert: (alert: AnomalyAlert) => void;
  acknowledgeAlert: (alertId: string) => Promise<void>;
  fetchAnomalyAlerts: () => Promise<void>;
  unacknowledgedCount: () => number;
}

export const useInsightStore = create<InsightStoreState>()((set, get) => ({
  executionReports: [],
  anomalyAlerts: [],
  rebalanceState: { loading: false, result: null, error: null },

  fetchExecutionReports: async () => {
    try {
      const reports = await apiFetchReports();
      set({ executionReports: reports });
    } catch (err) {
      console.error("Failed to fetch execution reports", err);
    }
  },

  submitRebalancePrompt: async (prompt: string) => {
    set({ rebalanceState: { loading: true, result: null, error: null } });
    try {
      const result = await apiRebalance(prompt);
      set({ rebalanceState: { loading: false, result, error: null } });
    } catch (err) {
      const message = err instanceof Error ? err.message : "Rebalance failed";
      set({ rebalanceState: { loading: false, result: null, error: message } });
    }
  },

  clearRebalanceResult: () => {
    set({ rebalanceState: { loading: false, result: null, error: null } });
  },

  applyAnomalyAlert: (alert: AnomalyAlert) => {
    set((state) => ({
      anomalyAlerts: [alert, ...state.anomalyAlerts],
    }));
  },

  acknowledgeAlert: async (alertId: string) => {
    try {
      await apiAcknowledge(alertId);
      set((state) => ({
        anomalyAlerts: state.anomalyAlerts.map((a) =>
          a.id === alertId ? { ...a, acknowledged: true } : a,
        ),
      }));
    } catch (err) {
      console.error("Failed to acknowledge alert", err);
    }
  },

  fetchAnomalyAlerts: async () => {
    try {
      const { alerts } = await apiFetchAlerts();
      set({ anomalyAlerts: alerts });
    } catch (err) {
      console.error("Failed to fetch anomaly alerts", err);
    }
  },

  unacknowledgedCount: () => {
    return get().anomalyAlerts.filter((a) => !a.acknowledged).length;
  },
}));

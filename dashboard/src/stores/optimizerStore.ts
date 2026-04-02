import { create } from "zustand";
import type {
  OptimizationConstraints,
  OptimizationResult,
  TradeAction,
} from "../api/types";
import { optimizePortfolio } from "../api/rest";
import { useOrderStore } from "./orderStore";

const DEFAULT_CONSTRAINTS: OptimizationConstraints = {
  riskAversion: 1,
  longOnly: true,
  maxSingleWeight: null,
  targetVolatility: null,
  maxTurnover: null,
  assetClassBounds: null,
};

export interface OptimizerStoreState {
  /** Current optimization constraints (form state) */
  constraints: OptimizationConstraints;

  /** Last optimization result */
  result: OptimizationResult | null;

  /** Whether an optimization is currently running */
  isOptimizing: boolean;

  /** Last error message */
  error: string | null;

  /** Update a single constraint field */
  setConstraint: <K extends keyof OptimizationConstraints>(
    key: K,
    value: OptimizationConstraints[K],
  ) => void;

  /** Run portfolio optimization with current constraints */
  runOptimize: () => Promise<void>;

  /** Execute a list of trade actions via the order store */
  executeTradeList: (trades: TradeAction[]) => Promise<void>;

  /** Reset store to initial state */
  reset: () => void;
}

export const useOptimizerStore = create<OptimizerStoreState>()((set, get) => ({
  constraints: { ...DEFAULT_CONSTRAINTS },
  result: null,
  isOptimizing: false,
  error: null,

  setConstraint: <K extends keyof OptimizationConstraints>(
    key: K,
    value: OptimizationConstraints[K],
  ): void => {
    set((state) => ({
      constraints: { ...state.constraints, [key]: value },
    }));
  },

  runOptimize: async (): Promise<void> => {
    set({ isOptimizing: true, error: null });
    try {
      const constraints = get().constraints;
      const result = await optimizePortfolio(constraints);
      set({ result, isOptimizing: false });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Optimization failed";
      set({ isOptimizing: false, error: message });
    }
  },

  executeTradeList: async (trades: TradeAction[]): Promise<void> => {
    set({ error: null });
    const { submitOrder } = useOrderStore.getState();
    for (const trade of trades) {
      try {
        await submitOrder({
          instrumentId: trade.instrumentId,
          side: trade.side,
          type: "market",
          quantity: trade.quantity,
          venueId: "default",
        });
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Failed to execute trade";
        set({ error: message });
        return;
      }
    }
  },

  reset: (): void => {
    set({
      constraints: { ...DEFAULT_CONSTRAINTS },
      result: null,
      isOptimizing: false,
      error: null,
    });
  },
}));

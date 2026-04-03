import { create } from "zustand";
import type { OHLCUpdate } from "../api/types";
import { createMarketDataStream } from "../api/ws";
import type ReconnectingWebSocket from "reconnecting-websocket";

const MAX_BARS_PER_INSTRUMENT = 500;

export interface MarketDataState {
  /** Map of "instrumentId:interval" -> bar array */
  bars: Record<string, OHLCUpdate[]>;

  /** Apply a real-time OHLC bar update from the WebSocket */
  applyUpdate: (update: OHLCUpdate) => void;

  /** Get bars for an instrument and interval */
  getBars: (instrumentId: string, interval: string) => OHLCUpdate[];

  /** Subscribe to the market data WebSocket stream */
  subscribe: () => () => void;
}

export const useMarketDataStore = create<MarketDataState>()((set, get) => ({
  bars: {},

  applyUpdate: (update: OHLCUpdate): void => {
    set((state) => {
      const key = `${update.instrumentId}:${update.interval}`;
      const existing = state.bars[key] ?? [];
      let updated: OHLCUpdate[];

      const last = existing[existing.length - 1];
      if (last && last.periodStart === update.periodStart) {
        // Same period — replace the last bar (partial update or completion)
        updated = [...existing.slice(0, -1), update];
      } else {
        // New period — append
        updated = [...existing, update];
      }

      // Cap at MAX_BARS_PER_INSTRUMENT
      if (updated.length > MAX_BARS_PER_INSTRUMENT) {
        updated = updated.slice(updated.length - MAX_BARS_PER_INSTRUMENT);
      }

      return { bars: { ...state.bars, [key]: updated } };
    });
  },

  getBars: (instrumentId: string, interval: string): OHLCUpdate[] => {
    const key = `${instrumentId}:${interval}`;
    return get().bars[key] ?? [];
  },

  subscribe: (): (() => void) => {
    let ws: ReconnectingWebSocket | null = createMarketDataStream((update) => {
      get().applyUpdate(update);
    });

    return () => {
      if (ws) {
        ws.close();
        ws = null;
      }
    };
  },
}));

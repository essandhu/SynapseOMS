import { create } from "zustand";
import type { Position, PositionUpdate } from "../api/types";
import { fetchPositions } from "../api/rest";
import { createPositionStream } from "../api/ws";
import type ReconnectingWebSocket from "reconnecting-websocket";

/** Unique key for a position (instrument + venue) */
function positionKey(p: Pick<Position, "instrumentId" | "venueId">): string {
  return `${p.instrumentId}:${p.venueId}`;
}

export interface PositionStoreState {
  /** All positions indexed by instrumentId:venueId */
  positions: Map<string, Position>;

  /** Whether the store is currently loading */
  loading: boolean;

  /** Last error message */
  error: string | null;

  /** Apply a real-time position update from WebSocket */
  applyUpdate: (update: PositionUpdate) => void;

  /** Load initial positions from REST API */
  loadPositions: () => Promise<void>;

  /** Subscribe to real-time position updates via WebSocket */
  subscribe: () => () => void;
}

export const usePositionStore = create<PositionStoreState>()((set, get) => ({
  positions: new Map<string, Position>(),
  loading: false,
  error: null,

  applyUpdate: (update: PositionUpdate): void => {
    set((state) => {
      const next = new Map(state.positions);
      const key = positionKey(update.position);
      next.set(key, update.position);
      return { positions: next };
    });
  },

  loadPositions: async (): Promise<void> => {
    set({ loading: true, error: null });
    try {
      const positions = await fetchPositions();
      const map = new Map<string, Position>();
      for (const pos of positions) {
        map.set(positionKey(pos), pos);
      }
      set({ positions: map, loading: false });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load positions";
      set({ loading: false, error: message });
    }
  },

  subscribe: (): (() => void) => {
    // Load initial positions
    get().loadPositions();

    // Connect WebSocket for real-time updates
    let ws: ReconnectingWebSocket | null = createPositionStream((update) => {
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

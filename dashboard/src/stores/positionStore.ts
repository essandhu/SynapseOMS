import { create } from "zustand";
import type { Position, PositionUpdate } from "../api/types";

/** Unique key for a position (instrument + venue) */
function positionKey(p: Pick<Position, "instrumentId" | "venueId">): string {
  return `${p.instrumentId}:${p.venueId}`;
}

export interface PositionStoreState {
  /** All positions indexed by instrumentId:venueId */
  positions: Map<string, Position>;

  /** Apply a real-time position update from WebSocket */
  applyUpdate: (update: PositionUpdate) => void;
}

export const usePositionStore = create<PositionStoreState>()((set) => ({
  positions: new Map<string, Position>(),

  applyUpdate: (update: PositionUpdate): void => {
    set((state) => {
      const next = new Map(state.positions);
      const key = positionKey(update.position);
      next.set(key, update.position);
      return { positions: next };
    });
  },
}));

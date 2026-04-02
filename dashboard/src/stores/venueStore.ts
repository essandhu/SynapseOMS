import { create } from "zustand";
import type { Venue, VenueStatusUpdate } from "../api/types";
import {
  fetchVenues,
  connectVenue as apiConnectVenue,
  disconnectVenue as apiDisconnectVenue,
  storeCredentials as apiStoreCredentials,
} from "../api/rest";
import { createVenueStream } from "../api/ws";
import type ReconnectingWebSocket from "reconnecting-websocket";

export interface VenueStoreState {
  /** All venues indexed by venue ID */
  venues: Map<string, Venue>;

  /** Whether the store is currently loading */
  loading: boolean;

  /** Last error message */
  error: string | null;

  /** Returns only venues with connected status */
  connectedVenues: () => Venue[];

  /** Fetch all venues from the API */
  loadVenues: () => Promise<void>;

  /** Connect a venue by ID */
  connectVenue: (venueId: string) => Promise<void>;

  /** Disconnect a venue by ID */
  disconnectVenue: (venueId: string) => Promise<void>;

  /** Store credentials for a venue */
  storeCredentials: (
    venueId: string,
    apiKey: string,
    apiSecret: string,
    passphrase?: string,
  ) => Promise<void>;

  /** Apply a real-time venue status update from WebSocket */
  applyUpdate: (update: VenueStatusUpdate) => void;

  /** Subscribe to real-time venue updates via WebSocket and load initial data */
  subscribe: () => () => void;
}

export const useVenueStore = create<VenueStoreState>()((set, get) => ({
  venues: new Map<string, Venue>(),
  loading: false,
  error: null,

  connectedVenues: () => {
    return Array.from(get().venues.values()).filter(
      (v) => v.status === "connected",
    );
  },

  loadVenues: async (): Promise<void> => {
    set({ loading: true, error: null });
    try {
      const venues = await fetchVenues();
      const map = new Map<string, Venue>();
      for (const venue of venues) {
        map.set(venue.id, venue);
      }
      set({ venues: map, loading: false });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load venues";
      set({ loading: false, error: message });
    }
  },

  connectVenue: async (venueId: string): Promise<void> => {
    set({ loading: true, error: null });
    try {
      await apiConnectVenue(venueId);
      set((state) => {
        const next = new Map(state.venues);
        const existing = next.get(venueId);
        if (existing) {
          next.set(venueId, { ...existing, status: "connected" });
        }
        return { venues: next, loading: false };
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to connect venue";
      set({ loading: false, error: message });
      throw err;
    }
  },

  disconnectVenue: async (venueId: string): Promise<void> => {
    set({ loading: true, error: null });
    try {
      await apiDisconnectVenue(venueId);
      set((state) => {
        const next = new Map(state.venues);
        const existing = next.get(venueId);
        if (existing) {
          next.set(venueId, { ...existing, status: "disconnected" });
        }
        return { venues: next, loading: false };
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to disconnect venue";
      set({ loading: false, error: message });
      throw err;
    }
  },

  storeCredentials: async (
    venueId: string,
    apiKey: string,
    apiSecret: string,
    passphrase?: string,
  ): Promise<void> => {
    set({ loading: true, error: null });
    try {
      await apiStoreCredentials(venueId, apiKey, apiSecret, passphrase);
      set((state) => {
        const next = new Map(state.venues);
        const existing = next.get(venueId);
        if (existing) {
          next.set(venueId, { ...existing, hasCredentials: true });
        }
        return { venues: next, loading: false };
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to store credentials";
      set({ loading: false, error: message });
      throw err;
    }
  },

  applyUpdate: (update: VenueStatusUpdate): void => {
    set((state) => {
      const next = new Map(state.venues);
      const existing = next.get(update.venueId);
      if (existing) {
        const statusMap: Record<string, Venue["status"]> = {
          venue_connected: "connected",
          venue_disconnected: "disconnected",
          venue_degraded: "degraded",
        };
        next.set(update.venueId, {
          ...existing,
          status: statusMap[update.type] ?? existing.status,
          ...(update.latencyMs !== undefined && {
            latencyP50Ms: update.latencyMs,
          }),
        });
      }
      return { venues: next };
    });
  },

  subscribe: (): (() => void) => {
    // Load initial venues
    get().loadVenues();

    // Connect WebSocket for real-time updates
    let ws: ReconnectingWebSocket | null = createVenueStream((update) => {
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

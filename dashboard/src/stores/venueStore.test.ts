import { describe, it, expect, vi, beforeEach } from "vitest";
import { useVenueStore } from "./venueStore";
import type { Venue } from "../api/types";

// Mock the REST API module
vi.mock("../api/rest", () => ({
  fetchVenues: vi.fn(),
  connectVenue: vi.fn(),
  disconnectVenue: vi.fn(),
  storeCredentials: vi.fn(),
}));

// Mock the WebSocket module
vi.mock("../api/ws", () => ({
  createVenueStream: vi.fn(() => ({ close: vi.fn() })),
}));

import {
  fetchVenues,
  connectVenue as apiConnectVenue,
  disconnectVenue as apiDisconnectVenue,
} from "../api/rest";

const mockVenues: Venue[] = [
  {
    id: "alpaca",
    name: "Alpaca",
    type: "exchange",
    status: "connected",
    supportedAssets: ["equity"],
    latencyP50Ms: 45,
    latencyP99Ms: 120,
    fillRate: 0.98,
    lastHeartbeat: "2026-04-01T10:00:00Z",
    hasCredentials: true,
  },
  {
    id: "binance",
    name: "Binance Testnet",
    type: "exchange",
    status: "disconnected",
    supportedAssets: ["crypto"],
    latencyP50Ms: 30,
    latencyP99Ms: 85,
    fillRate: 0.99,
    lastHeartbeat: "2026-04-01T09:55:00Z",
    hasCredentials: true,
  },
  {
    id: "simulator",
    name: "Simulator",
    type: "simulated",
    status: "connected",
    supportedAssets: ["equity", "crypto"],
    latencyP50Ms: 1,
    latencyP99Ms: 5,
    fillRate: 1.0,
    lastHeartbeat: "2026-04-01T10:00:00Z",
    hasCredentials: false,
  },
];

describe("venueStore", () => {
  beforeEach(() => {
    useVenueStore.setState({
      venues: new Map(),
      loading: false,
      error: null,
    });
    vi.clearAllMocks();
  });

  it("has empty initial state", () => {
    const state = useVenueStore.getState();
    expect(state.venues.size).toBe(0);
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
  });

  it("loadVenues populates the venue map", async () => {
    vi.mocked(fetchVenues).mockResolvedValue(mockVenues);

    await useVenueStore.getState().loadVenues();

    const state = useVenueStore.getState();
    expect(state.venues.size).toBe(3);
    expect(state.venues.get("alpaca")?.name).toBe("Alpaca");
    expect(state.venues.get("binance")?.status).toBe("disconnected");
    expect(state.venues.get("simulator")?.type).toBe("simulated");
    expect(state.loading).toBe(false);
  });

  it("loadVenues sets error on failure", async () => {
    vi.mocked(fetchVenues).mockRejectedValue(new Error("Failed to fetch"));

    await useVenueStore.getState().loadVenues();

    const state = useVenueStore.getState();
    expect(state.venues.size).toBe(0);
    expect(state.error).toBe("Failed to fetch");
  });

  it("applyUpdate with venue_connected updates venue status", () => {
    // Pre-populate venues
    const map = new Map<string, Venue>();
    for (const v of mockVenues) map.set(v.id, v);
    useVenueStore.setState({ venues: map });

    useVenueStore.getState().applyUpdate({
      type: "venue_connected",
      venueId: "binance",
      status: "connected",
    });

    const updated = useVenueStore.getState().venues.get("binance");
    expect(updated?.status).toBe("connected");
  });

  it("applyUpdate with venue_disconnected updates venue status", () => {
    const map = new Map<string, Venue>();
    for (const v of mockVenues) map.set(v.id, v);
    useVenueStore.setState({ venues: map });

    useVenueStore.getState().applyUpdate({
      type: "venue_disconnected",
      venueId: "alpaca",
      status: "disconnected",
    });

    const updated = useVenueStore.getState().venues.get("alpaca");
    expect(updated?.status).toBe("disconnected");
  });

  it("applyUpdate with latencyMs updates venue latency", () => {
    const map = new Map<string, Venue>();
    for (const v of mockVenues) map.set(v.id, v);
    useVenueStore.setState({ venues: map });

    useVenueStore.getState().applyUpdate({
      type: "venue_connected",
      venueId: "alpaca",
      status: "connected",
      latencyMs: 22,
    });

    const updated = useVenueStore.getState().venues.get("alpaca");
    expect(updated?.latencyP50Ms).toBe(22);
  });

  it("connectedVenues returns only connected venues", () => {
    const map = new Map<string, Venue>();
    for (const v of mockVenues) map.set(v.id, v);
    useVenueStore.setState({ venues: map });

    const connected = useVenueStore.getState().connectedVenues();
    expect(connected).toHaveLength(2);
    expect(connected.map((v) => v.id).sort()).toEqual(["alpaca", "simulator"]);
  });

  it("connectVenue calls correct API endpoint and updates status", async () => {
    vi.mocked(apiConnectVenue).mockResolvedValue(undefined);

    const map = new Map<string, Venue>();
    for (const v of mockVenues) map.set(v.id, v);
    useVenueStore.setState({ venues: map });

    await useVenueStore.getState().connectVenue("binance");

    expect(apiConnectVenue).toHaveBeenCalledWith("binance");
    const updated = useVenueStore.getState().venues.get("binance");
    expect(updated?.status).toBe("connected");
  });

  it("disconnectVenue calls correct API endpoint and updates status", async () => {
    vi.mocked(apiDisconnectVenue).mockResolvedValue(undefined);

    const map = new Map<string, Venue>();
    for (const v of mockVenues) map.set(v.id, v);
    useVenueStore.setState({ venues: map });

    await useVenueStore.getState().disconnectVenue("alpaca");

    expect(apiDisconnectVenue).toHaveBeenCalledWith("alpaca");
    const updated = useVenueStore.getState().venues.get("alpaca");
    expect(updated?.status).toBe("disconnected");
  });

  it("connectVenue sets error and rethrows on failure", async () => {
    vi.mocked(apiConnectVenue).mockRejectedValue(new Error("Connection refused"));

    const map = new Map<string, Venue>();
    for (const v of mockVenues) map.set(v.id, v);
    useVenueStore.setState({ venues: map });

    await expect(useVenueStore.getState().connectVenue("binance")).rejects.toThrow(
      "Connection refused",
    );

    expect(useVenueStore.getState().error).toBe("Connection refused");
  });
});

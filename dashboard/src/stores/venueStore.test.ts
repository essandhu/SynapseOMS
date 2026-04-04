import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../mocks/server";
import { useVenueStore } from "./venueStore";
import { mockVenues } from "../mocks/data";
import type { Venue } from "../api/types";

// Mock the WebSocket module (MSW cannot intercept WebSockets)
vi.mock("../api/ws", () => ({
  createVenueStream: vi.fn(() => ({ close: vi.fn() })),
}));

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
    await useVenueStore.getState().loadVenues();

    const state = useVenueStore.getState();
    expect(state.venues.size).toBe(3);
    expect(state.venues.get("alpaca")?.name).toBe("Alpaca");
    expect(state.venues.get("binance_testnet")?.status).toBe("disconnected");
    expect(state.venues.get("sim-exchange")?.type).toBe("simulated");
    expect(state.loading).toBe(false);
  });

  it("loadVenues sets error on failure", async () => {
    server.use(
      http.get("*/api/v1/venues", () =>
        HttpResponse.json({ message: "Failed to fetch" }, { status: 422 }),
      ),
    );

    await useVenueStore.getState().loadVenues();

    const state = useVenueStore.getState();
    expect(state.venues.size).toBe(0);
    expect(state.error).toBeTruthy();
  });

  it("applyUpdate with venue_connected updates venue status", () => {
    const map = new Map<string, Venue>();
    for (const v of mockVenues) map.set(v.id, v);
    useVenueStore.setState({ venues: map });

    useVenueStore.getState().applyUpdate({
      type: "venue_connected",
      venueId: "binance_testnet",
      status: "connected",
    });

    const updated = useVenueStore.getState().venues.get("binance_testnet");
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
    expect(connected.map((v) => v.id).sort()).toEqual(["alpaca", "sim-exchange"]);
  });

  it("connectVenue calls correct API endpoint and updates status", async () => {
    const map = new Map<string, Venue>();
    for (const v of mockVenues) map.set(v.id, v);
    useVenueStore.setState({ venues: map });

    await useVenueStore.getState().connectVenue("binance_testnet");

    const updated = useVenueStore.getState().venues.get("binance_testnet");
    expect(updated?.status).toBe("connected");
  });

  it("disconnectVenue calls correct API endpoint and updates status", async () => {
    const map = new Map<string, Venue>();
    for (const v of mockVenues) map.set(v.id, v);
    useVenueStore.setState({ venues: map });

    await useVenueStore.getState().disconnectVenue("alpaca");

    const updated = useVenueStore.getState().venues.get("alpaca");
    expect(updated?.status).toBe("disconnected");
  });

  it("connectVenue sets error and rethrows on failure", async () => {
    server.use(
      http.post("*/api/v1/venues/:venueId/connect", () =>
        HttpResponse.json(
          { error: { message: "Connection refused" } },
          { status: 422 },
        ),
      ),
    );

    const map = new Map<string, Venue>();
    for (const v of mockVenues) map.set(v.id, v);
    useVenueStore.setState({ venues: map });

    await expect(
      useVenueStore.getState().connectVenue("binance_testnet"),
    ).rejects.toThrow();

    expect(useVenueStore.getState().error).toBeTruthy();
  });
});

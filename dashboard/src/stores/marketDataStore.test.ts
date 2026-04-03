import { describe, it, expect, vi, beforeEach } from "vitest";
import { useMarketDataStore } from "./marketDataStore";
import type { OHLCUpdate } from "../api/types";

// Mock the WebSocket module
vi.mock("../api/ws", () => ({
  createMarketDataStream: vi.fn(() => ({ close: vi.fn() })),
}));

const makeBar = (
  instrumentId: string,
  close: string,
  complete: boolean,
  periodStart = "2026-04-02T10:00:00Z",
): OHLCUpdate => ({
  instrumentId,
  venueId: "test-venue",
  interval: "1m",
  open: "150.00",
  high: "152.00",
  low: "148.00",
  close,
  volume: "1000",
  periodStart,
  periodEnd: "2026-04-02T10:01:00Z",
  complete,
});

describe("marketDataStore", () => {
  beforeEach(() => {
    useMarketDataStore.setState({ bars: {} });
    vi.clearAllMocks();
  });

  it("has empty initial state", () => {
    const state = useMarketDataStore.getState();
    expect(state.bars).toEqual({});
  });

  it("applyUpdate adds a complete bar to the array", () => {
    const bar = makeBar("AAPL", "151.00", true);
    useMarketDataStore.getState().applyUpdate(bar);

    const bars = useMarketDataStore.getState().getBars("AAPL", "1m");
    expect(bars).toHaveLength(1);
    expect(bars[0].close).toBe("151.00");
    expect(bars[0].complete).toBe(true);
  });

  it("applyUpdate replaces last bar when partial update arrives", () => {
    // First: a partial bar
    useMarketDataStore.getState().applyUpdate(makeBar("AAPL", "150.50", false));
    expect(useMarketDataStore.getState().getBars("AAPL", "1m")).toHaveLength(1);

    // Second: another partial bar (same period) should replace, not append
    useMarketDataStore.getState().applyUpdate(makeBar("AAPL", "151.00", false));
    const bars = useMarketDataStore.getState().getBars("AAPL", "1m");
    expect(bars).toHaveLength(1);
    expect(bars[0].close).toBe("151.00");
  });

  it("applyUpdate appends when complete bar arrives after partial", () => {
    useMarketDataStore.getState().applyUpdate(makeBar("AAPL", "150.50", false));
    useMarketDataStore.getState().applyUpdate(makeBar("AAPL", "151.00", true));

    // The completed bar replaces the partial
    const bars = useMarketDataStore.getState().getBars("AAPL", "1m");
    expect(bars).toHaveLength(1);
    expect(bars[0].complete).toBe(true);
  });

  it("applyUpdate starts new bar after completed bar", () => {
    useMarketDataStore
      .getState()
      .applyUpdate(makeBar("AAPL", "151.00", true, "2026-04-02T10:00:00Z"));
    useMarketDataStore
      .getState()
      .applyUpdate(makeBar("AAPL", "152.00", false, "2026-04-02T10:01:00Z"));

    const bars = useMarketDataStore.getState().getBars("AAPL", "1m");
    expect(bars).toHaveLength(2);
    expect(bars[0].close).toBe("151.00");
    expect(bars[1].close).toBe("152.00");
  });

  it("caps bars at 500 per instrument", () => {
    const store = useMarketDataStore.getState();
    for (let i = 0; i < 510; i++) {
      const ts = new Date(2026, 3, 2, 10, i, 0).toISOString();
      store.applyUpdate(makeBar("AAPL", `${150 + i}`, true, ts));
    }

    const bars = useMarketDataStore.getState().getBars("AAPL", "1m");
    expect(bars.length).toBeLessThanOrEqual(500);
    // Should keep the newest bars (last 500)
    expect(bars[bars.length - 1].close).toBe("659");
  });

  it("getBars returns empty array for unknown instrument", () => {
    const bars = useMarketDataStore.getState().getBars("UNKNOWN", "1m");
    expect(bars).toEqual([]);
  });

  it("keeps instruments independent", () => {
    useMarketDataStore.getState().applyUpdate(makeBar("AAPL", "150.00", true));
    useMarketDataStore.getState().applyUpdate(makeBar("BTC-USD", "60000.00", true));

    expect(useMarketDataStore.getState().getBars("AAPL", "1m")).toHaveLength(1);
    expect(useMarketDataStore.getState().getBars("BTC-USD", "1m")).toHaveLength(1);
  });
});

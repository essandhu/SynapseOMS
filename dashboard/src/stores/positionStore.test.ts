import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../mocks/server";
import { usePositionStore } from "./positionStore";
import { mockPositions } from "../mocks/data";
import type { Position, PositionUpdate } from "../api/types";

// Mock the WebSocket module (MSW cannot intercept WebSockets)
vi.mock("../api/ws", () => ({
  createPositionStream: vi.fn(() => ({ close: vi.fn() })),
}));

import { createPositionStream } from "../api/ws";

describe("positionStore", () => {
  beforeEach(() => {
    usePositionStore.setState({
      positions: new Map(),
      loading: false,
      error: null,
    });
    vi.clearAllMocks();
  });

  it("has empty initial state", () => {
    const state = usePositionStore.getState();
    expect(state.positions.size).toBe(0);
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
  });

  it("loadPositions populates the position map", async () => {
    await usePositionStore.getState().loadPositions();

    const state = usePositionStore.getState();
    expect(state.positions.size).toBe(2);
    expect(state.positions.get("AAPL:alpaca")?.quantity).toBe("100");
    expect(state.positions.get("BTC-USD:binance")?.marketPrice).toBe(
      "42000.00",
    );
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
  });

  it("loadPositions sets error on failure", async () => {
    server.use(
      http.get("*/api/v1/positions", () =>
        HttpResponse.json({ message: "Network error" }, { status: 422 }),
      ),
    );

    await usePositionStore.getState().loadPositions();

    const state = usePositionStore.getState();
    expect(state.positions.size).toBe(0);
    expect(state.loading).toBe(false);
    expect(state.error).toBeTruthy();
  });

  it("loadPositions sets default error message for non-Error throws", async () => {
    server.use(
      http.get("*/api/v1/positions", () =>
        HttpResponse.json({ message: "bad" }, { status: 422 }),
      ),
    );

    await usePositionStore.getState().loadPositions();

    const state = usePositionStore.getState();
    expect(state.error).toBeTruthy();
  });

  it("applyUpdate adds a new position with correct key", () => {
    const update: PositionUpdate = {
      type: "position_update",
      position: {
        instrumentId: "ETH-USD",
        venueId: "binance",
        quantity: "10",
        averageCost: "3000.00",
        marketPrice: "3100.00",
        unrealizedPnl: "1000.00",
        realizedPnl: "0.00",
        unsettledQuantity: "0",
        assetClass: "crypto",
        quoteCurrency: "USD",
      },
    };

    usePositionStore.getState().applyUpdate(update);

    const state = usePositionStore.getState();
    expect(state.positions.size).toBe(1);
    expect(state.positions.get("ETH-USD:binance")?.quantity).toBe("10");
  });

  it("applyUpdate overwrites an existing position", () => {
    const map = new Map<string, Position>();
    for (const p of mockPositions)
      map.set(`${p.instrumentId}:${p.venueId}`, p);
    usePositionStore.setState({ positions: map });

    const update: PositionUpdate = {
      type: "position_update",
      position: {
        ...mockPositions[0],
        quantity: "200",
        marketPrice: "160.00",
      },
    };

    usePositionStore.getState().applyUpdate(update);

    const state = usePositionStore.getState();
    expect(state.positions.size).toBe(2);
    expect(state.positions.get("AAPL:alpaca")?.quantity).toBe("200");
    expect(state.positions.get("AAPL:alpaca")?.marketPrice).toBe("160.00");
  });

  it("subscribe calls loadPositions and creates WebSocket stream", () => {
    const cleanup = usePositionStore.getState().subscribe();

    expect(createPositionStream).toHaveBeenCalledWith(expect.any(Function));
    expect(typeof cleanup).toBe("function");
  });

  it("subscribe cleanup closes the WebSocket", () => {
    const mockClose = vi.fn();
    vi.mocked(createPositionStream).mockReturnValue({
      close: mockClose,
    } as any);

    const cleanup = usePositionStore.getState().subscribe();
    cleanup();

    expect(mockClose).toHaveBeenCalled();
  });

  it("subscribe WebSocket callback applies position updates", () => {
    let wsCallback: ((update: PositionUpdate) => void) | undefined;
    vi.mocked(createPositionStream).mockImplementation((cb: any) => {
      wsCallback = cb;
      return { close: vi.fn() } as any;
    });

    usePositionStore.getState().subscribe();

    const update: PositionUpdate = {
      type: "position_update",
      position: {
        instrumentId: "AAPL",
        venueId: "alpaca",
        quantity: "50",
        averageCost: "150.00",
        marketPrice: "158.00",
        unrealizedPnl: "400.00",
        realizedPnl: "0.00",
        unsettledQuantity: "0",
        assetClass: "equity",
        quoteCurrency: "USD",
      },
    };

    wsCallback!(update);

    const state = usePositionStore.getState();
    expect(state.positions.get("AAPL:alpaca")?.quantity).toBe("50");
  });
});

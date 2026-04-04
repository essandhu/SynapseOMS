import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../mocks/server";
import { useOrderStore } from "./orderStore";
import { makeOrder, mockOrders } from "../mocks/data";
import type { Order, OrderUpdate, SubmitOrderRequest } from "../api/types";

// Mock the WebSocket module (MSW cannot intercept WebSockets)
vi.mock("../api/ws", () => ({
  createOrderStream: vi.fn(() => ({ close: vi.fn() })),
}));

import { createOrderStream } from "../api/ws";

describe("orderStore", () => {
  beforeEach(() => {
    useOrderStore.setState({
      orders: new Map(),
      loading: false,
      error: null,
    });
    vi.clearAllMocks();
  });

  it("has empty initial state", () => {
    const state = useOrderStore.getState();
    expect(state.orders.size).toBe(0);
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
  });

  // --- loadOrders ---

  it("loadOrders populates the order map", async () => {
    await useOrderStore.getState().loadOrders();

    const state = useOrderStore.getState();
    expect(state.orders.size).toBe(5);
    expect(state.orders.get("order-1")?.status).toBe("new");
    expect(state.orders.get("order-2")?.instrumentId).toBe("TSLA");
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
  });

  it("loadOrders sets error on failure", async () => {
    server.use(
      http.get("*/api/v1/orders", () =>
        HttpResponse.json({ message: "Network error" }, { status: 422 }),
      ),
    );

    await useOrderStore.getState().loadOrders();

    const state = useOrderStore.getState();
    expect(state.orders.size).toBe(0);
    expect(state.error).toBeTruthy();
    expect(state.loading).toBe(false);
  });

  // --- submitOrder ---

  it("submitOrder adds the returned order to state", async () => {
    const request: SubmitOrderRequest = {
      instrumentId: "AAPL",
      side: "buy",
      type: "limit",
      quantity: "100",
      price: "150.00",
      venueId: "alpaca",
    };

    await useOrderStore.getState().submitOrder(request);

    const state = useOrderStore.getState();
    expect(state.orders.size).toBe(1);
    const order = Array.from(state.orders.values())[0];
    expect(order.status).toBe("new");
    expect(order.instrumentId).toBe("AAPL");
    expect(order.side).toBe("buy");
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
  });

  it("submitOrder sets error and rethrows on failure", async () => {
    server.use(
      http.post("*/api/v1/orders", () =>
        HttpResponse.json(
          { error: { message: "Insufficient funds" } },
          { status: 422 },
        ),
      ),
    );

    const request: SubmitOrderRequest = {
      instrumentId: "AAPL",
      side: "buy",
      type: "limit",
      quantity: "100",
      price: "150.00",
      venueId: "alpaca",
    };

    await expect(
      useOrderStore.getState().submitOrder(request),
    ).rejects.toThrow();

    const state = useOrderStore.getState();
    expect(state.error).toBeTruthy();
    expect(state.loading).toBe(false);
  });

  // --- cancelOrder ---

  it("cancelOrder updates order status to canceled", async () => {
    // Pre-populate an order
    const map = new Map<string, Order>();
    map.set("order-1", makeOrder({ id: "order-1", status: "new" }));
    useOrderStore.setState({ orders: map });

    await useOrderStore.getState().cancelOrder("order-1");

    const state = useOrderStore.getState();
    expect(state.orders.get("order-1")?.status).toBe("canceled");
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
  });

  it("cancelOrder sets error and rethrows on failure", async () => {
    server.use(
      http.delete("*/api/v1/orders/:id", () =>
        HttpResponse.json(
          { error: { message: "Order not found" } },
          { status: 422 },
        ),
      ),
    );

    await expect(
      useOrderStore.getState().cancelOrder("order-1"),
    ).rejects.toThrow();

    const state = useOrderStore.getState();
    expect(state.error).toBeTruthy();
    expect(state.loading).toBe(false);
  });

  // --- applyUpdate ---

  it("applyUpdate applies a WebSocket order update", () => {
    const map = new Map<string, Order>();
    map.set("order-1", makeOrder({ id: "order-1", status: "new" }));
    useOrderStore.setState({ orders: map });

    const updatedOrder = makeOrder({
      id: "order-1",
      status: "partially_filled",
      filledQuantity: "50",
    });
    const update: OrderUpdate = { type: "order_update", order: updatedOrder };

    useOrderStore.getState().applyUpdate(update);

    const state = useOrderStore.getState();
    expect(state.orders.get("order-1")?.status).toBe("partially_filled");
    expect(state.orders.get("order-1")?.filledQuantity).toBe("50");
  });

  it("applyUpdate adds a new order if not already present", () => {
    const newOrder = makeOrder({ id: "order-new", status: "acknowledged" });
    const update: OrderUpdate = { type: "order_update", order: newOrder };

    useOrderStore.getState().applyUpdate(update);

    const state = useOrderStore.getState();
    expect(state.orders.size).toBe(1);
    expect(state.orders.get("order-new")?.status).toBe("acknowledged");
  });

  // --- activeOrders ---

  it("activeOrders returns only new, acknowledged, and partially_filled orders", () => {
    const map = new Map<string, Order>();
    for (const o of mockOrders) map.set(o.id, o);
    useOrderStore.setState({ orders: map });

    const active = useOrderStore.getState().activeOrders();
    expect(active).toHaveLength(3);

    const ids = active.map((o) => o.id).sort();
    expect(ids).toEqual(["order-1", "order-3", "order-5"]);
  });

  it("activeOrders returns empty array when no active orders exist", () => {
    const map = new Map<string, Order>();
    map.set("filled", makeOrder({ id: "filled", status: "filled" }));
    map.set("canceled", makeOrder({ id: "canceled", status: "canceled" }));
    map.set("rejected", makeOrder({ id: "rejected", status: "rejected" }));
    useOrderStore.setState({ orders: map });

    const active = useOrderStore.getState().activeOrders();
    expect(active).toHaveLength(0);
  });

  // --- subscribe ---

  it("subscribe creates WebSocket stream", () => {
    const unsubscribe = useOrderStore.getState().subscribe();

    expect(createOrderStream).toHaveBeenCalledWith(expect.any(Function));

    unsubscribe();
  });

  it("subscribe cleanup closes the WebSocket", () => {
    const mockClose = vi.fn();
    vi.mocked(createOrderStream).mockReturnValue({
      close: mockClose,
    } as any);

    const unsubscribe = useOrderStore.getState().subscribe();
    unsubscribe();

    expect(mockClose).toHaveBeenCalled();
  });

  it("subscribe WebSocket callback invokes applyUpdate", () => {
    let capturedCallback: ((update: OrderUpdate) => void) | undefined;
    vi.mocked(createOrderStream).mockImplementation((cb: any) => {
      capturedCallback = cb;
      return { close: vi.fn() } as any;
    });

    const unsubscribe = useOrderStore.getState().subscribe();

    const updatedOrder = makeOrder({ id: "ws-order", status: "filled" });
    capturedCallback!({ type: "order_update", order: updatedOrder });

    const state = useOrderStore.getState();
    expect(state.orders.get("ws-order")?.status).toBe("filled");

    unsubscribe();
  });
});

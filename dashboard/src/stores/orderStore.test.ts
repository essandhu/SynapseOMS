import { describe, it, expect, vi, beforeEach } from "vitest";
import { useOrderStore } from "./orderStore";
import type { Order, OrderUpdate, SubmitOrderRequest } from "../api/types";

// Mock the REST API module
vi.mock("../api/rest", () => ({
  submitOrder: vi.fn(),
  cancelOrder: vi.fn(),
  fetchOrders: vi.fn(),
}));

// Mock the WebSocket module
vi.mock("../api/ws", () => ({
  createOrderStream: vi.fn(() => ({ close: vi.fn() })),
}));

import {
  submitOrder as apiSubmitOrder,
  cancelOrder as apiCancelOrder,
  fetchOrders,
} from "../api/rest";
import { createOrderStream } from "../api/ws";

const makeOrder = (overrides: Partial<Order> = {}): Order => ({
  id: "order-1",
  clientOrderId: "client-1",
  instrumentId: "AAPL",
  side: "buy",
  type: "limit",
  quantity: "100",
  price: "150.00",
  filledQuantity: "0",
  averagePrice: "0",
  status: "new",
  venueId: "alpaca",
  assetClass: "equity",
  createdAt: "2026-04-01T10:00:00Z",
  updatedAt: "2026-04-01T10:00:00Z",
  fills: [],
  ...overrides,
});

const mockOrders: Order[] = [
  makeOrder({ id: "order-1", status: "new" }),
  makeOrder({ id: "order-2", status: "filled", instrumentId: "TSLA" }),
  makeOrder({ id: "order-3", status: "partially_filled", instrumentId: "GOOG" }),
  makeOrder({ id: "order-4", status: "canceled", instrumentId: "MSFT" }),
  makeOrder({ id: "order-5", status: "acknowledged", instrumentId: "AMZN" }),
];

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
    vi.mocked(fetchOrders).mockResolvedValue(mockOrders);

    await useOrderStore.getState().loadOrders();

    const state = useOrderStore.getState();
    expect(state.orders.size).toBe(5);
    expect(state.orders.get("order-1")?.status).toBe("new");
    expect(state.orders.get("order-2")?.instrumentId).toBe("TSLA");
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
  });

  it("loadOrders sets error on failure", async () => {
    vi.mocked(fetchOrders).mockRejectedValue(new Error("Network error"));

    await useOrderStore.getState().loadOrders();

    const state = useOrderStore.getState();
    expect(state.orders.size).toBe(0);
    expect(state.error).toBe("Network error");
    expect(state.loading).toBe(false);
  });

  // --- submitOrder ---

  it("submitOrder adds the returned order to state", async () => {
    const returned = makeOrder({ id: "new-order", status: "new" });
    vi.mocked(apiSubmitOrder).mockResolvedValue(returned);

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
    expect(state.orders.get("new-order")?.status).toBe("new");
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
    expect(apiSubmitOrder).toHaveBeenCalledWith(request);
  });

  it("submitOrder sets error and rethrows on failure", async () => {
    vi.mocked(apiSubmitOrder).mockRejectedValue(new Error("Insufficient funds"));

    const request: SubmitOrderRequest = {
      instrumentId: "AAPL",
      side: "buy",
      type: "limit",
      quantity: "100",
      price: "150.00",
      venueId: "alpaca",
    };

    await expect(useOrderStore.getState().submitOrder(request)).rejects.toThrow(
      "Insufficient funds",
    );

    const state = useOrderStore.getState();
    expect(state.error).toBe("Insufficient funds");
    expect(state.loading).toBe(false);
  });

  // --- cancelOrder ---

  it("cancelOrder updates order status to canceled", async () => {
    vi.mocked(apiCancelOrder).mockResolvedValue(undefined);

    // Pre-populate an order
    const map = new Map<string, Order>();
    map.set("order-1", makeOrder({ id: "order-1", status: "new" }));
    useOrderStore.setState({ orders: map });

    await useOrderStore.getState().cancelOrder("order-1");

    const state = useOrderStore.getState();
    expect(state.orders.get("order-1")?.status).toBe("canceled");
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
    expect(apiCancelOrder).toHaveBeenCalledWith("order-1");
  });

  it("cancelOrder sets error and rethrows on failure", async () => {
    vi.mocked(apiCancelOrder).mockRejectedValue(new Error("Order not found"));

    await expect(useOrderStore.getState().cancelOrder("order-1")).rejects.toThrow(
      "Order not found",
    );

    const state = useOrderStore.getState();
    expect(state.error).toBe("Order not found");
    expect(state.loading).toBe(false);
  });

  // --- applyUpdate ---

  it("applyUpdate applies a WebSocket order update", () => {
    // Pre-populate an order
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

  it("subscribe calls loadOrders and creates WebSocket stream", () => {
    vi.mocked(fetchOrders).mockResolvedValue([]);

    const unsubscribe = useOrderStore.getState().subscribe();

    expect(fetchOrders).toHaveBeenCalled();
    expect(createOrderStream).toHaveBeenCalledWith(expect.any(Function));

    unsubscribe();
  });

  it("subscribe cleanup closes the WebSocket", () => {
    vi.mocked(fetchOrders).mockResolvedValue([]);
    const mockClose = vi.fn();
    vi.mocked(createOrderStream).mockReturnValue({ close: mockClose } as any);

    const unsubscribe = useOrderStore.getState().subscribe();
    unsubscribe();

    expect(mockClose).toHaveBeenCalled();
  });

  it("subscribe WebSocket callback invokes applyUpdate", () => {
    vi.mocked(fetchOrders).mockResolvedValue([]);
    let capturedCallback: ((update: OrderUpdate) => void) | undefined;
    vi.mocked(createOrderStream).mockImplementation((cb: any) => {
      capturedCallback = cb;
      return { close: vi.fn() } as any;
    });

    const unsubscribe = useOrderStore.getState().subscribe();

    // Simulate a WebSocket message
    const updatedOrder = makeOrder({ id: "ws-order", status: "filled" });
    capturedCallback!({ type: "order_update", order: updatedOrder });

    const state = useOrderStore.getState();
    expect(state.orders.get("ws-order")?.status).toBe("filled");

    unsubscribe();
  });
});

import { create } from "zustand";
import type { Order, OrderUpdate, SubmitOrderRequest } from "../api/types";
import { submitOrder as apiSubmitOrder, cancelOrder as apiCancelOrder, fetchOrders } from "../api/rest";

export interface OrderStoreState {
  /** All orders indexed by order ID */
  orders: Map<string, Order>;

  /** Whether the store is currently loading */
  loading: boolean;

  /** Last error message */
  error: string | null;

  /** Returns only orders with active statuses (new, acknowledged, partially_filled) */
  activeOrders: () => Order[];

  /** Submit a new order to the API */
  submitOrder: (request: SubmitOrderRequest) => Promise<void>;

  /** Cancel an existing order by ID */
  cancelOrder: (id: string) => Promise<void>;

  /** Apply a real-time order update from WebSocket */
  applyUpdate: (update: OrderUpdate) => void;

  /** Load initial orders from REST API */
  loadOrders: () => Promise<void>;
}

export const useOrderStore = create<OrderStoreState>()((set, get) => ({
  orders: new Map<string, Order>(),
  loading: false,
  error: null,

  activeOrders: () => {
    const activeStatuses = new Set(["new", "acknowledged", "partially_filled"]);
    return Array.from(get().orders.values()).filter((o) =>
      activeStatuses.has(o.status),
    );
  },

  submitOrder: async (request: SubmitOrderRequest): Promise<void> => {
    set({ loading: true, error: null });
    try {
      const order = await apiSubmitOrder(request);
      set((state) => {
        const next = new Map(state.orders);
        next.set(order.id, order);
        return { orders: next, loading: false };
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to submit order";
      set({ loading: false, error: message });
      throw err;
    }
  },

  cancelOrder: async (id: string): Promise<void> => {
    set({ loading: true, error: null });
    try {
      await apiCancelOrder(id);
      set((state) => {
        const next = new Map(state.orders);
        const existing = next.get(id);
        if (existing) {
          next.set(id, { ...existing, status: "canceled" });
        }
        return { orders: next, loading: false };
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to cancel order";
      set({ loading: false, error: message });
      throw err;
    }
  },

  applyUpdate: (update: OrderUpdate): void => {
    set((state) => {
      const next = new Map(state.orders);
      next.set(update.order.id, update.order);
      return { orders: next };
    });
  },

  loadOrders: async (): Promise<void> => {
    set({ loading: true, error: null });
    try {
      const orders = await fetchOrders();
      const map = new Map<string, Order>();
      for (const order of orders) {
        map.set(order.id, order);
      }
      set({ orders: map, loading: false });
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to load orders";
      set({ loading: false, error: message });
    }
  },
}));

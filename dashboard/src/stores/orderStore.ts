import { create } from "zustand";
import type { Order, OrderUpdate, SubmitOrderRequest } from "../api/types";

export interface OrderStoreState {
  /** All orders indexed by order ID */
  orders: Map<string, Order>;

  /** Returns only orders with active statuses (new, acknowledged, partially_filled) */
  activeOrders: () => Order[];

  /** Submit a new order to the API */
  submitOrder: (request: SubmitOrderRequest) => Promise<void>;

  /** Cancel an existing order by ID */
  cancelOrder: (id: string) => Promise<void>;

  /** Apply a real-time order update from WebSocket */
  applyUpdate: (update: OrderUpdate) => void;
}

export const useOrderStore = create<OrderStoreState>()((set, get) => ({
  orders: new Map<string, Order>(),

  activeOrders: () => {
    const activeStatuses = new Set(["new", "acknowledged", "partially_filled"]);
    return Array.from(get().orders.values()).filter((o) =>
      activeStatuses.has(o.status),
    );
  },

  submitOrder: async (_request: SubmitOrderRequest): Promise<void> => {
    // TODO: P1-14 — call REST API and update store
    console.warn("[orderStore] submitOrder is a stub");
  },

  cancelOrder: async (_id: string): Promise<void> => {
    // TODO: P1-14 — call REST API and update store
    console.warn("[orderStore] cancelOrder is a stub");
  },

  applyUpdate: (update: OrderUpdate): void => {
    set((state) => {
      const next = new Map(state.orders);
      next.set(update.order.id, update.order);
      return { orders: next };
    });
  },
}));

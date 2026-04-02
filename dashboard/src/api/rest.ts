import ky from "ky";
import type { Instrument, Order, Position, Venue, SubmitOrderRequest } from "./types";

const api = ky.create({
  prefixUrl: import.meta.env.VITE_API_URL || "/api",
  timeout: 10_000,
  headers: {
    "Content-Type": "application/json",
  },
});

/** Fetch all orders */
export async function fetchOrders(): Promise<Order[]> {
  return api.get("orders").json<Order[]>();
}

/** Fetch a single order by ID */
export async function fetchOrder(id: string): Promise<Order> {
  return api.get(`orders/${id}`).json<Order>();
}

/** Submit a new order */
export async function submitOrder(request: SubmitOrderRequest): Promise<Order> {
  return api.post("orders", { json: request }).json<Order>();
}

/** Cancel an order by ID */
export async function cancelOrder(id: string): Promise<void> {
  await api.delete(`orders/${id}`);
}

/** Fetch all positions */
export async function fetchPositions(): Promise<Position[]> {
  return api.get("positions").json<Position[]>();
}

/** Fetch all instruments */
export async function fetchInstruments(): Promise<Instrument[]> {
  return api.get("instruments").json<Instrument[]>();
}

/** Fetch all venues */
export async function fetchVenues(): Promise<Venue[]> {
  return api.get("venues").json<Venue[]>();
}

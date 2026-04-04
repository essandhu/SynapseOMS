import type { Order } from "../../api/types";

export const makeOrder = (overrides: Partial<Order> = {}): Order => ({
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

export const mockOrders: Order[] = [
  makeOrder({ id: "order-1", status: "new" }),
  makeOrder({ id: "order-2", status: "filled", instrumentId: "TSLA" }),
  makeOrder({
    id: "order-3",
    status: "partially_filled",
    instrumentId: "GOOG",
  }),
  makeOrder({ id: "order-4", status: "canceled", instrumentId: "MSFT" }),
  makeOrder({ id: "order-5", status: "acknowledged", instrumentId: "AMZN" }),
];

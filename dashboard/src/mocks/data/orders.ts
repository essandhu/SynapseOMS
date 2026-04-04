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

/**
 * Converts a camelCase Order to the snake_case JSON the Go backend actually returns.
 * Used by MSW handlers to realistically simulate backend responses.
 */
export function toRawOrder(order: Order): Record<string, unknown> {
  return {
    id: order.id,
    client_order_id: order.clientOrderId,
    instrument_id: order.instrumentId,
    side: order.side,
    type: order.type,
    quantity: order.quantity,
    price: order.price,
    filled_quantity: order.filledQuantity,
    average_price: order.averagePrice,
    status: order.status,
    asset_class: order.assetClass,
    venue_id: order.venueId,
    created_at: order.createdAt,
    updated_at: order.updatedAt,
    fills: order.fills.map((f) => ({
      id: f.id,
      order_id: f.orderId,
      venue_id: f.venueId,
      quantity: f.quantity,
      price: f.price,
      fee: f.fee,
      fee_asset: f.feeAsset,
      liquidity: f.liquidity,
      timestamp: f.timestamp,
    })),
  };
}

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

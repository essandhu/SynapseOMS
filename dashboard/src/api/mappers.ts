import type { Order, OrderUpdate, Fill, Position, PositionUpdate } from "./types";

/**
 * Maps a raw API/WebSocket order object (snake_case JSON from Go backend)
 * to the frontend Order type (camelCase).
 *
 * Handles both snake_case and camelCase input gracefully so that mock
 * data and real backend responses both work.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function mapOrder(raw: any): Order {
  return {
    id: raw.id ?? "",
    clientOrderId: raw.clientOrderId ?? raw.client_order_id ?? "",
    instrumentId: raw.instrumentId ?? raw.instrument_id ?? "",
    side: raw.side ?? "buy",
    type: raw.type ?? "market",
    quantity: raw.quantity ?? "0",
    price: raw.price ?? "0",
    filledQuantity: raw.filledQuantity ?? raw.filled_quantity ?? "0",
    averagePrice: raw.averagePrice ?? raw.average_price ?? "0",
    status: raw.status ?? "new",
    venueId: raw.venueId ?? raw.venue_id ?? "",
    assetClass: raw.assetClass ?? raw.asset_class ?? "",
    createdAt: raw.createdAt ?? raw.created_at ?? "",
    updatedAt: raw.updatedAt ?? raw.updated_at ?? "",
    fills: Array.isArray(raw.fills) ? raw.fills.map(mapFill) : [],
  };
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function mapFill(raw: any): Fill {
  return {
    id: raw.id ?? "",
    orderId: raw.orderId ?? raw.order_id ?? "",
    venueId: raw.venueId ?? raw.venue_id ?? "",
    quantity: raw.quantity ?? "0",
    price: raw.price ?? "0",
    fee: raw.fee ?? "0",
    feeAsset: raw.feeAsset ?? raw.fee_asset ?? "",
    liquidity: raw.liquidity ?? "taker",
    timestamp: raw.timestamp ?? "",
  };
}

/**
 * Maps a raw WebSocket message to an OrderUpdate.
 *
 * The Go backend sends: { "type": "order_update", "data": {...} }
 * The frontend OrderUpdate expects: { type: "order_update", order: Order }
 *
 * This function bridges the mismatch, accepting either "data" or "order" key.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function mapRawOrderUpdate(raw: any): OrderUpdate {
  const orderData = raw.data ?? raw.order;
  return {
    type: "order_update",
    order: mapOrder(orderData),
  };
}

/**
 * Maps a raw API/WebSocket position object (snake_case JSON from Go backend)
 * to the frontend Position type (camelCase).
 *
 * Handles both snake_case and camelCase input gracefully so that mock
 * data and real backend responses both work.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function mapPosition(raw: any): Position {
  return {
    instrumentId: raw.instrumentId ?? raw.instrument_id ?? "",
    venueId: raw.venueId ?? raw.venue_id ?? "",
    quantity: raw.quantity ?? "0",
    averageCost: raw.averageCost ?? raw.average_cost ?? "0",
    marketPrice: raw.marketPrice ?? raw.market_price ?? "0",
    unrealizedPnl: raw.unrealizedPnl ?? raw.unrealized_pnl ?? "0",
    realizedPnl: raw.realizedPnl ?? raw.realized_pnl ?? "0",
    unsettledQuantity: raw.unsettledQuantity ?? raw.unsettled_quantity ?? "0",
    assetClass: raw.assetClass ?? raw.asset_class ?? "",
    quoteCurrency: raw.quoteCurrency ?? raw.quote_currency ?? "",
  };
}

/**
 * Maps a raw WebSocket message to a PositionUpdate.
 *
 * The Go backend sends: { "type": "position_update", "data": {...} }
 * The frontend PositionUpdate expects: { type: "position_update", position: Position }
 *
 * This function bridges the mismatch, accepting either "data" or "position" key.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function mapRawPositionUpdate(raw: any): PositionUpdate {
  const positionData = raw.data ?? raw.position;
  return {
    type: "position_update",
    position: mapPosition(positionData),
  };
}

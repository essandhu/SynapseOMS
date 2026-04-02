export type AssetClass =
  | "equity"
  | "crypto"
  | "tokenized_security"
  | "future"
  | "option";

export type SettlementCycle = "T0" | "T1" | "T2";

export type OrderStatus =
  | "new"
  | "acknowledged"
  | "partially_filled"
  | "filled"
  | "canceled"
  | "rejected";

export type OrderSide = "buy" | "sell";

export type OrderType = "market" | "limit" | "stop_limit";

export interface Fill {
  id: string;
  orderId: string;
  venueId: string;
  quantity: string;
  price: string;
  fee: string;
  feeAsset: string;
  liquidity: "maker" | "taker" | "internal";
  timestamp: string;
}

export interface Order {
  id: string;
  clientOrderId: string;
  instrumentId: string;
  side: OrderSide;
  type: OrderType;
  quantity: string;
  price: string;
  filledQuantity: string;
  averagePrice: string;
  status: OrderStatus;
  venueId: string;
  assetClass: AssetClass;
  createdAt: string;
  updatedAt: string;
  fills: Fill[];
}

export interface Position {
  instrumentId: string;
  venueId: string;
  quantity: string;
  averageCost: string;
  marketPrice: string;
  unrealizedPnl: string;
  realizedPnl: string;
  unsettledQuantity: string;
  assetClass: AssetClass;
  quoteCurrency: string;
}

export interface Venue {
  id: string;
  name: string;
  type: "exchange" | "dark_pool" | "simulated" | "tokenized";
  status: "connected" | "disconnected" | "degraded" | "authentication";
  supportedAssets: AssetClass[];
  latencyP50Ms: number;
  latencyP99Ms: number;
  fillRate: number;
  lastHeartbeat: string;
  hasCredentials: boolean;
}

/** WebSocket update envelope for order changes */
export interface OrderUpdate {
  type: "order_update";
  order: Order;
}

/** WebSocket update envelope for position changes */
export interface PositionUpdate {
  type: "position_update";
  position: Position;
}

/** Request payload for submitting a new order */
export interface SubmitOrderRequest {
  instrumentId: string;
  side: OrderSide;
  type: OrderType;
  quantity: string;
  price?: string;
  venueId: string;
}

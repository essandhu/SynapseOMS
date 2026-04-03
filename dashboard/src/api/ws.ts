import ReconnectingWebSocket from "reconnecting-websocket";
import type {
  OrderUpdate,
  PositionUpdate,
  VenueStatusUpdate,
  AnomalyAlert,
  OHLCUpdate,
} from "./types";

const BASE_WS = import.meta.env.VITE_WS_URL || "ws://localhost:8080";

/** Connection state for UI indicators. */
export type ConnectionState = "connected" | "disconnected" | "reconnecting";

/** Callback invoked when any WebSocket stream changes connection state. */
export type ConnectionStateCallback = (
  stream: string,
  state: ConnectionState,
) => void;

/**
 * Attaches open/close/error listeners to a ReconnectingWebSocket that
 * invoke the provided callback with the current connection state.
 * This enables the UI to display "Connection lost, reconnecting..." banners.
 */
function attachConnectionStateListeners(
  ws: ReconnectingWebSocket,
  streamName: string,
  onStateChange?: ConnectionStateCallback,
): void {
  if (!onStateChange) return;

  ws.addEventListener("open", () => {
    onStateChange(streamName, "connected");
  });
  ws.addEventListener("close", () => {
    onStateChange(streamName, "reconnecting");
  });
  ws.addEventListener("error", () => {
    onStateChange(streamName, "disconnected");
  });
}

/**
 * Creates a WebSocket connection for real-time order updates.
 * Automatically reconnects on disconnect.
 */
export function createOrderStream(
  onUpdate: (update: OrderUpdate) => void,
): ReconnectingWebSocket {
  const ws = new ReconnectingWebSocket(`${BASE_WS}/ws/orders`);

  ws.addEventListener("message", (event: MessageEvent) => {
    try {
      const data = JSON.parse(event.data as string) as OrderUpdate;
      onUpdate(data);
    } catch {
      console.error("[ws:orders] Failed to parse message");
    }
  });

  return ws;
}

/**
 * Creates a WebSocket connection for real-time position updates.
 * Automatically reconnects on disconnect.
 */
export function createPositionStream(
  onUpdate: (update: PositionUpdate) => void,
): ReconnectingWebSocket {
  const ws = new ReconnectingWebSocket(`${BASE_WS}/ws/positions`);

  ws.addEventListener("message", (event: MessageEvent) => {
    try {
      const data = JSON.parse(event.data as string) as PositionUpdate;
      onUpdate(data);
    } catch {
      console.error("[ws:positions] Failed to parse message");
    }
  });

  return ws;
}

/**
 * Creates a WebSocket connection for real-time venue status updates.
 * Automatically reconnects on disconnect.
 */
export function createVenueStream(
  onUpdate: (update: VenueStatusUpdate) => void,
): ReconnectingWebSocket {
  const ws = new ReconnectingWebSocket(`${BASE_WS}/ws/venues`);

  ws.addEventListener("message", (event: MessageEvent) => {
    try {
      const data = JSON.parse(event.data as string) as VenueStatusUpdate;
      onUpdate(data);
    } catch {
      console.error("[ws:venues] Failed to parse message");
    }
  });

  return ws;
}

/**
 * Creates a WebSocket connection for real-time anomaly alerts.
 * Automatically reconnects on disconnect.
 */
export function createAnomalyStream(
  onAlert: (alert: AnomalyAlert) => void,
): ReconnectingWebSocket {
  const ws = new ReconnectingWebSocket(`${BASE_WS}/ws/anomalies`);

  ws.addEventListener("message", (event: MessageEvent) => {
    try {
      const data = JSON.parse(event.data as string);
      if (data.type === "anomaly_alert" && data.data) {
        onAlert(data.data as AnomalyAlert);
      }
    } catch {
      console.error("[ws:anomalies] Failed to parse message");
    }
  });

  return ws;
}

/**
 * Creates a WebSocket connection for real-time OHLC market data updates.
 * Automatically reconnects on disconnect.
 */
export function createMarketDataStream(
  onUpdate: (update: OHLCUpdate) => void,
): ReconnectingWebSocket {
  const ws = new ReconnectingWebSocket(`${BASE_WS}/ws/marketdata`);

  ws.addEventListener("message", (event: MessageEvent) => {
    try {
      const msg = JSON.parse(event.data as string);
      if (msg.type === "ohlc_update" && msg.data) {
        onUpdate(msg.data as OHLCUpdate);
      }
    } catch {
      console.error("[ws:marketdata] Failed to parse message");
    }
  });

  return ws;
}

/**
 * Initialize all WebSocket streams and return a cleanup function.
 * Connects order, position, and venue streams simultaneously.
 *
 * When `onConnectionStateChange` is provided, it is called whenever any
 * stream connects, disconnects, or begins reconnecting. The UI can use
 * this to display a "Connection lost, reconnecting..." banner.
 */
export function initializeStreams(handlers: {
  onOrderUpdate: (update: OrderUpdate) => void;
  onPositionUpdate: (update: PositionUpdate) => void;
  onVenueUpdate: (update: VenueStatusUpdate) => void;
  onAnomalyAlert?: (alert: AnomalyAlert) => void;
  onConnectionStateChange?: ConnectionStateCallback;
}): () => void {
  const orderWs = createOrderStream(handlers.onOrderUpdate);
  const positionWs = createPositionStream(handlers.onPositionUpdate);
  const venueWs = createVenueStream(handlers.onVenueUpdate);

  attachConnectionStateListeners(orderWs, "orders", handlers.onConnectionStateChange);
  attachConnectionStateListeners(positionWs, "positions", handlers.onConnectionStateChange);
  attachConnectionStateListeners(venueWs, "venues", handlers.onConnectionStateChange);

  const streams: ReconnectingWebSocket[] = [orderWs, positionWs, venueWs];

  if (handlers.onAnomalyAlert) {
    const anomalyWs = createAnomalyStream(handlers.onAnomalyAlert);
    attachConnectionStateListeners(anomalyWs, "anomalies", handlers.onConnectionStateChange);
    streams.push(anomalyWs);
  }

  return () => {
    for (const ws of streams) {
      ws.close();
    }
  };
}

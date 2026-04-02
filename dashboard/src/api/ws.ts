import ReconnectingWebSocket from "reconnecting-websocket";
import type {
  OrderUpdate,
  PositionUpdate,
  RiskUpdate,
  VenueStatusUpdate,
  AnomalyAlert,
} from "./types";

const BASE_WS = import.meta.env.VITE_WS_URL || "ws://localhost:8080";
const RISK_WS = import.meta.env.VITE_RISK_WS_URL || "ws://localhost:8081";

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
 * Creates a WebSocket connection for real-time risk updates.
 * Automatically reconnects on disconnect.
 */
export function createRiskStream(
  onUpdate: (update: RiskUpdate) => void,
): ReconnectingWebSocket {
  const ws = new ReconnectingWebSocket(`${RISK_WS}/ws/risk`);

  ws.addEventListener("message", (event: MessageEvent) => {
    try {
      const data = JSON.parse(event.data as string) as RiskUpdate;
      onUpdate(data);
    } catch {
      console.error("[ws:risk] Failed to parse message");
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
 * Initialize all WebSocket streams and return a cleanup function.
 * Connects order, position, risk, and venue streams simultaneously.
 */
export function initializeStreams(handlers: {
  onOrderUpdate: (update: OrderUpdate) => void;
  onPositionUpdate: (update: PositionUpdate) => void;
  onRiskUpdate: (update: RiskUpdate) => void;
  onVenueUpdate: (update: VenueStatusUpdate) => void;
  onAnomalyAlert?: (alert: AnomalyAlert) => void;
}): () => void {
  const streams: ReconnectingWebSocket[] = [
    createOrderStream(handlers.onOrderUpdate),
    createPositionStream(handlers.onPositionUpdate),
    createRiskStream(handlers.onRiskUpdate),
    createVenueStream(handlers.onVenueUpdate),
  ];

  if (handlers.onAnomalyAlert) {
    streams.push(createAnomalyStream(handlers.onAnomalyAlert));
  }

  return () => {
    for (const ws of streams) {
      ws.close();
    }
  };
}

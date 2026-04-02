import ReconnectingWebSocket from "reconnecting-websocket";
import type { OrderUpdate, PositionUpdate } from "./types";

const BASE_WS = import.meta.env.VITE_WS_URL || "ws://localhost:8080";

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

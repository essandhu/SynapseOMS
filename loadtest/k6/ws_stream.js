import ws from "k6/ws";
import { check, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

const messageLatency = new Trend("ws_message_latency", true);
const messageReceived = new Rate("ws_message_received");

export const options = {
  scenarios: {
    orders_stream: {
      executor: "constant-vus",
      vus: 334,
      duration: "2m",
      exec: "ordersStream",
    },
    positions_stream: {
      executor: "constant-vus",
      vus: 333,
      duration: "2m",
      exec: "positionsStream",
    },
    marketdata_stream: {
      executor: "constant-vus",
      vus: 333,
      duration: "2m",
      exec: "marketdataStream",
    },
  },
  thresholds: {
    ws_message_latency: ["p(99)<5"],
    ws_message_received: ["rate>0.95"],
  },
};

function connectAndListen(url, streamName) {
  const res = ws.connect(url, {}, function (socket) {
    let messagesReceived = 0;

    socket.on("open", function () {
      // Connection established
    });

    socket.on("message", function (data) {
      const now = Date.now();
      messagesReceived++;
      messageReceived.add(true);

      try {
        const msg = JSON.parse(data);
        if (msg.timestamp) {
          const serverTime = new Date(msg.timestamp).getTime();
          messageLatency.add(now - serverTime);
        }
      } catch (_) {
        // Non-JSON message, still counts as received
      }
    });

    socket.on("error", function (e) {
      messageReceived.add(false);
    });

    // Keep connection open for the scenario duration
    socket.setTimeout(function () {
      socket.close();
    }, 115000); // Close slightly before 2m duration
  });

  check(res, {
    [`${streamName} connected`]: (r) => r && r.status === 101,
  });
}

export function ordersStream() {
  connectAndListen("ws://localhost:8080/ws/orders", "orders");
}

export function positionsStream() {
  connectAndListen("ws://localhost:8080/ws/positions", "positions");
}

export function marketdataStream() {
  connectAndListen("ws://localhost:8080/ws/marketdata", "marketdata");
}

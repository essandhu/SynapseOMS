import http from "k6/http";
import { check } from "k6";
import { Rate, Trend } from "k6/metrics";

const orderLatency = new Trend("order_submit_latency", true);
const fillRate = new Rate("fill_received");

export const options = {
  scenarios: {
    sustained_load: {
      executor: "constant-arrival-rate",
      rate: 5000,
      timeUnit: "1s",
      duration: "5m",
      preAllocatedVUs: 200,
      maxVUs: 500,
    },
  },
  thresholds: {
    order_submit_latency: ["p(99)<50"],
    fill_received: ["rate>0.99"],
  },
};

const instruments = ["AAPL", "MSFT", "ETH-USD", "BTC-USD", "GOOG", "SOL-USD"];

export default function () {
  const instrument =
    instruments[Math.floor(Math.random() * instruments.length)];
  const payload = JSON.stringify({
    instrumentId: instrument,
    side: Math.random() > 0.5 ? "buy" : "sell",
    type: "market",
    quantity: (Math.random() * 100 + 1).toFixed(2),
  });

  const start = Date.now();
  const res = http.post("http://localhost:8080/api/v1/orders", payload, {
    headers: { "Content-Type": "application/json" },
  });
  const elapsed = Date.now() - start;

  orderLatency.add(elapsed);

  const accepted = check(res, {
    "status 201": (r) => r.status === 201,
  });

  fillRate.add(accepted);
}

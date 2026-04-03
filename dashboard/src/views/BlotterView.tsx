import { useEffect, useState, useMemo, useCallback } from "react";
import { OrderTable } from "../components/OrderTable";
import { OrderTicket } from "../components/OrderTicket";
import { CandlestickChart } from "../components/CandlestickChart";
import { useOrderStore } from "../stores/orderStore";
import { useVenueStore } from "../stores/venueStore";
import { useMarketDataStore } from "../stores/marketDataStore";
import { fetchInstruments } from "../api/rest";
import type { Instrument, OrderStatus } from "../api/types";

type StatusFilter = "active" | "all" | "filled" | "canceled";

const TERMINAL_STATUSES = new Set<OrderStatus>(["filled", "canceled", "rejected"]);

const FILTER_TABS: { key: StatusFilter; label: string }[] = [
  { key: "active", label: "Active" },
  { key: "all", label: "All" },
  { key: "filled", label: "Filled" },
  { key: "canceled", label: "Canceled" },
];

export function BlotterView() {
  const orders = useOrderStore((s) => s.orders);
  const loading = useOrderStore((s) => s.loading);
  const error = useOrderStore((s) => s.error);
  const subscribe = useOrderStore((s) => s.subscribe);
  const submitOrder = useOrderStore((s) => s.submitOrder);
  const cancelOrder = useOrderStore((s) => s.cancelOrder);

  const venueMap = useVenueStore((s) => s.venues);
  const loadVenues = useVenueStore((s) => s.loadVenues);
  const venues = useMemo(() => Array.from(venueMap.values()), [venueMap]);

  const [instruments, setInstruments] = useState<Instrument[]>([]);
  const [filter, setFilter] = useState<StatusFilter>("active");
  const [ticketOpen, setTicketOpen] = useState(true);
  const [chartOpen, setChartOpen] = useState(false);
  const [chartInstrument, setChartInstrument] = useState("AAPL");
  const subscribeMarketData = useMarketDataStore((s) => s.subscribe);

  // Subscribe to WebSocket and load initial orders
  useEffect(() => {
    const unsubscribe = subscribe();
    return unsubscribe;
  }, [subscribe]);

  // Subscribe to market data WebSocket when chart is open
  useEffect(() => {
    if (!chartOpen) return;
    const unsub = subscribeMarketData();
    return unsub;
  }, [chartOpen, subscribeMarketData]);

  // Load instruments and venues for the order ticket
  useEffect(() => {
    fetchInstruments()
      .then(setInstruments)
      .catch(() => {
        /* instruments will be empty */
      });
    loadVenues().catch(() => {
      /* venues will be empty */
    });
  }, [loadVenues]);

  // Filter orders based on current tab
  const filteredOrders = useMemo(() => {
    const all = Array.from(orders.values());
    switch (filter) {
      case "active":
        return all.filter((o) => !TERMINAL_STATUSES.has(o.status));
      case "filled":
        return all.filter((o) => o.status === "filled");
      case "canceled":
        return all.filter((o) => o.status === "canceled" || o.status === "rejected");
      case "all":
      default:
        return all;
    }
  }, [orders, filter]);

  const handleCancel = useCallback(
    (orderId: string) => {
      cancelOrder(orderId).catch(() => {
        /* error is set in store */
      });
    },
    [cancelOrder],
  );

  return (
    <div className="flex h-full gap-3">
      {/* Main content: filter tabs + grid */}
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        {/* Header row */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-1">
            {FILTER_TABS.map((tab) => (
              <button
                key={tab.key}
                onClick={() => setFilter(tab.key)}
                className="rounded px-3 py-1 font-mono text-xs font-medium transition-colors"
                style={{
                  backgroundColor:
                    filter === tab.key ? "rgba(59,130,246,0.2)" : "transparent",
                  color: filter === tab.key ? "#3b82f6" : "#6b7280",
                  border: `1px solid ${filter === tab.key ? "rgba(59,130,246,0.3)" : "transparent"}`,
                }}
              >
                {tab.label}
              </button>
            ))}
          </div>

          <div className="flex items-center gap-3">
            {loading && (
              <span className="font-mono text-xs text-text-muted">Loading...</span>
            )}
            <button
              onClick={() => setChartOpen((v) => !v)}
              className="rounded border border-border px-2 py-1 font-mono text-xs text-text-muted transition-colors hover:border-accent-blue hover:text-accent-blue"
              data-testid="chart-toggle"
            >
              {chartOpen ? "Hide Chart" : "Chart"}
            </button>
            <button
              onClick={() => setTicketOpen((v) => !v)}
              className="rounded border border-border px-2 py-1 font-mono text-xs text-text-muted transition-colors hover:border-accent-blue hover:text-accent-blue"
            >
              {ticketOpen ? "Hide Ticket" : "New Order"}
            </button>
          </div>
        </div>

        {/* Error banner */}
        {error && (
          <div className="rounded border border-accent-red/30 bg-accent-red/10 px-3 py-2 font-mono text-xs text-accent-red">
            {error}
          </div>
        )}

        {/* Candlestick chart panel */}
        {chartOpen && (
          <div
            className="shrink-0 rounded border border-border"
            style={{ height: 280 }}
            data-testid="chart-panel"
          >
            <div className="flex items-center gap-2 border-b border-border px-3 py-1">
              <span className="font-mono text-xs text-text-muted">Chart:</span>
              <select
                value={chartInstrument}
                onChange={(e) => setChartInstrument(e.target.value)}
                className="rounded border border-border bg-bg-secondary px-2 py-0.5 font-mono text-xs text-text-primary"
              >
                {instruments.length > 0 ? (
                  instruments.map((inst) => (
                    <option key={inst.id} value={inst.id}>
                      {inst.symbol}
                    </option>
                  ))
                ) : (
                  <option value="AAPL">AAPL</option>
                )}
              </select>
            </div>
            <div style={{ height: 248 }}>
              <CandlestickChart instrumentId={chartInstrument} />
            </div>
          </div>
        )}

        {/* AG Grid */}
        <div className="flex-1">
          <OrderTable orders={filteredOrders} onCancel={handleCancel} />
        </div>
      </div>

      {/* Order ticket sidebar */}
      {ticketOpen && (
        <div className="w-[360px] shrink-0">
          <OrderTicket instruments={instruments} venues={venues} onSubmit={submitOrder} />
        </div>
      )}
    </div>
  );
}

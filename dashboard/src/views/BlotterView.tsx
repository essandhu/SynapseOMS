import { useEffect, useState, useMemo, useCallback } from "react";
import { OrderTable } from "../components/OrderTable";
import { OrderTicket } from "../components/OrderTicket";
import { CandlestickChart } from "../components/CandlestickChart";
import { useOrderStore } from "../stores/orderStore";
import { useVenueStore } from "../stores/venueStore";
import { usePositionStore } from "../stores/positionStore";
import { useMarketDataStore } from "../stores/marketDataStore";
import { fetchInstruments } from "../api/rest";
import type { Instrument, OrderStatus } from "../api/types";
import { useThemeColors } from "../theme/terminal";

type StatusFilter = "active" | "all" | "filled" | "canceled";

const TERMINAL_STATUSES = new Set<OrderStatus>(["filled", "canceled", "rejected"]);

const FILTER_TABS: { key: StatusFilter; label: string }[] = [
  { key: "active", label: "Active" },
  { key: "all", label: "All" },
  { key: "filled", label: "Filled" },
  { key: "canceled", label: "Canceled" },
];

export function BlotterView() {
  const theme = useThemeColors();
  const orders = useOrderStore((s) => s.orders);
  const loading = useOrderStore((s) => s.loading);
  const error = useOrderStore((s) => s.error);
  const submitOrder = useOrderStore((s) => s.submitOrder);
  const cancelOrder = useOrderStore((s) => s.cancelOrder);

  const venueMap = useVenueStore((s) => s.venues);
  const loadVenues = useVenueStore((s) => s.loadVenues);
  const venues = useMemo(() => Array.from(venueMap.values()), [venueMap]);

  const positionMap = usePositionStore((s) => s.positions);
  const loadPositions = usePositionStore((s) => s.loadPositions);
  const positions = useMemo(() => Array.from(positionMap.values()), [positionMap]);

  const [instruments, setInstruments] = useState<Instrument[]>([]);
  const [filter, setFilter] = useState<StatusFilter>("active");
  const [ticketOpen, setTicketOpen] = useState(true);
  const [chartOpen, setChartOpen] = useState(false);
  const [chartInstrument, setChartInstrument] = useState("AAPL");
  const [chartInterval, setChartInterval] = useState<"1m" | "5m">("1m");
  const subscribeMarketData = useMarketDataStore((s) => s.subscribe);

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
    loadPositions().catch(() => {
      /* positions will be empty */
    });
  }, [loadVenues, loadPositions]);

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
                className="rounded-xl px-3 py-1 text-xs font-medium transition-colors"
                style={{
                  backgroundColor:
                    filter === tab.key ? theme.colors.accent.blue + "29" : "transparent",
                  color: filter === tab.key ? theme.colors.accent.blue : theme.colors.text.muted,
                  border: `1px solid ${filter === tab.key ? theme.colors.accent.blue + "4D" : "transparent"}`,
                }}
              >
                {tab.label}
              </button>
            ))}
          </div>

          <div className="flex items-center gap-3">
            {loading && (
              <span className="text-xs text-text-muted">Loading...</span>
            )}
            <button
              onClick={() => setChartOpen((v) => !v)}
              className="rounded-xl border border-border px-2 py-1 text-xs text-text-muted transition-colors hover:border-accent-blue hover:text-accent-blue"
              data-testid="chart-toggle"
            >
              {chartOpen ? "Hide Chart" : "Chart"}
            </button>
            <button
              onClick={() => setTicketOpen((v) => !v)}
              className="rounded-xl border border-border px-2 py-1 text-xs text-text-muted transition-colors hover:border-accent-blue hover:text-accent-blue"
            >
              {ticketOpen ? "Hide Ticket" : "New Order"}
            </button>
          </div>
        </div>

        {/* Error banner */}
        {error && (
          <div className="rounded border border-accent-red/30 bg-accent-red/10 px-3 py-2 text-xs text-accent-red">
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
              <span className="text-xs text-text-muted">Chart:</span>
              <select
                value={chartInstrument}
                onChange={(e) => setChartInstrument(e.target.value)}
                className="rounded border border-border bg-bg-secondary px-2 py-0.5 text-xs text-text-primary"
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
              {(["1m", "5m"] as const).map((iv) => (
                <button
                  key={iv}
                  onClick={() => setChartInterval(iv)}
                  data-testid={`interval-${iv}`}
                  className="rounded-xl px-2 py-0.5 text-xs font-medium transition-colors"
                  style={{
                    backgroundColor: chartInterval === iv ? theme.colors.accent.blue + "29" : "transparent",
                    color: chartInterval === iv ? theme.colors.accent.blue : theme.colors.text.muted,
                    border: `1px solid ${chartInterval === iv ? theme.colors.accent.blue + "4D" : "transparent"}`,
                  }}
                >
                  {iv}
                </button>
              ))}
            </div>
            <div style={{ height: 248 }}>
              <CandlestickChart instrumentId={chartInstrument} interval={chartInterval} />
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
          <OrderTicket instruments={instruments} venues={venues} positions={positions} onSubmit={submitOrder} />
        </div>
      )}
    </div>
  );
}

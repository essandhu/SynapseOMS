import { useEffect, useState, useMemo, useRef } from "react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from "recharts";
import { PositionTable } from "../components/PositionTable";
import { ExposureTreemap } from "../components/ExposureTreemap";
import { usePositionStore } from "../stores/positionStore";
import { useRiskStore } from "../stores/riskStore";
import { fetchPortfolioSummary } from "../api/rest";
import type { PortfolioSummary } from "../api/types";

// ── Helpers ────────────────────────────────────────────────────────────

function formatCurrency(value: string | number): string {
  const num = typeof value === "string" ? Number(value) : value;
  if (isNaN(num)) return "$--";
  return num.toLocaleString("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
}

const ASSET_CLASS_COLORS: Record<string, string> = {
  equity: "#7132f5",
  crypto: "#f97316",
  tokenized_security: "#a855f7",
  future: "#14b8a6",
  option: "#eab308",
};

const VENUE_COLORS = [
  "#7132f5",
  "#149e61",
  "#f97316",
  "#a855f7",
  "#14b8a6",
  "#ef4444",
  "#eab308",
];

// ── Summary Card ───────────────────────────────────────────────────────

interface SummaryCardProps {
  label: string;
  value: string;
  valueClass?: string;
  icon?: React.ReactNode;
  loading?: boolean;
}

function SummaryCard({
  label,
  value,
  valueClass = "text-text-primary",
  icon,
  loading,
}: SummaryCardProps) {
  return (
    <div className="flex flex-col gap-1 rounded-lg border border-border bg-bg-secondary px-4 py-3" style={{ boxShadow: "rgba(0,0,0,0.03) 0px 4px 24px" }}>
      <span className="text-[10px] uppercase tracking-wider text-text-muted">
        {label}
      </span>
      {loading ? (
        <span className="text-lg text-text-muted animate-pulse">
          --
        </span>
      ) : (
        <div className="flex items-center gap-1.5">
          {icon}
          <span className={`text-lg font-semibold ${valueClass}`}>
            {value}
          </span>
        </div>
      )}
    </div>
  );
}

// ── Arrow icons ────────────────────────────────────────────────────────

function ArrowUp() {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="text-accent-green"
    >
      <path d="M12 19V5M5 12l7-7 7 7" />
    </svg>
  );
}

function ArrowDown() {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="text-accent-red"
    >
      <path d="M12 5v14M19 12l-7 7-7-7" />
    </svg>
  );
}

// ── Bar chart tooltip ──────────────────────────────────────────────────

function VenueTooltip({
  active,
  payload,
}: {
  active?: boolean;
  payload?: { payload: { venueId: string; percentage: number } }[];
}) {
  if (!active || !payload?.length) return null;
  const d = payload[0].payload;
  return (
    <div className="rounded border border-border bg-bg-secondary px-3 py-2 text-xs" style={{ boxShadow: "rgba(0,0,0,0.03) 0px 4px 24px" }}>
      <span className="text-text-primary">{d.venueId}</span>
      <span className="ml-2 text-text-muted">{d.percentage.toFixed(1)}%</span>
    </div>
  );
}

// ── Main View ──────────────────────────────────────────────────────────

export function PortfolioView() {
  const positions = usePositionStore((s) => s.positions);
  const posLoading = usePositionStore((s) => s.loading);
  const posError = usePositionStore((s) => s.error);
  const subscribe = usePositionStore((s) => s.subscribe);

  const settlement = useRiskStore((s) => s.settlement);
  const riskSubscribe = useRiskStore((s) => s.subscribe);

  const [summary, setSummary] = useState<PortfolioSummary | null>(null);
  const [summaryLoading, setSummaryLoading] = useState(true);

  // Subscribe to positions & risk on mount
  useEffect(() => {
    const unsubPos = subscribe();
    const unsubRisk = riskSubscribe();
    return () => {
      unsubPos();
      unsubRisk();
    };
  }, [subscribe, riskSubscribe]);

  // Fetch portfolio summary (cash values)
  useEffect(() => {
    let cancelled = false;

    async function load() {
      setSummaryLoading(true);
      try {
        const s = await fetchPortfolioSummary();
        if (!cancelled) {
          setSummary(s);
        }
      } catch {
        // API unavailable — leave as null, show placeholder
      } finally {
        if (!cancelled) setSummaryLoading(false);
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, []);

  const positionList = useMemo(() => Array.from(positions.values()), [positions]);
  const dailyPnl = summary ? Number(summary.dailyPnl) : 0;
  const pnlPositive = dailyPnl >= 0;

  // Compute unsettled cash from settlement store
  const unsettledCash = settlement ? Number(settlement.totalUnsettled) : 0;

  // Cash values from risk engine (only change on fills, not market moves)
  const cash = summary ? Number(summary.cash) : undefined;
  const availableCash = summary ? Number(summary.availableCash) : undefined;

  // Compute NAV client-side from live positions + availableCash so it
  // updates in real-time as market prices change via WebSocket.
  const totalNav = useMemo(() => {
    if (availableCash === undefined) return undefined;
    const posMarketValue = positionList.reduce((acc, p) => {
      return acc + Number(p.quantity) * Number(p.marketPrice);
    }, 0);
    return availableCash + posMarketValue;
  }, [availableCash, positionList]);

  // Re-fetch portfolio summary periodically so cash values stay current
  // after fills are processed by the risk engine via Kafka.
  useEffect(() => {
    const interval = setInterval(async () => {
      try {
        const s = await fetchPortfolioSummary();
        setSummary(s);
      } catch {
        // Risk engine may be unavailable — keep previous values
      }
    }, 10_000);
    return () => clearInterval(interval);
  }, []);

  // Compute exposure from live position data
  const assetClassRaw = useMemo(() => {
    if (positionList.length === 0) return [];
    const groups: Record<string, number> = {};
    let total = 0;
    for (const p of positionList) {
      const mv = Math.abs(Number(p.quantity) * Number(p.marketPrice));
      groups[p.assetClass] = (groups[p.assetClass] ?? 0) + mv;
      total += mv;
    }
    if (total === 0) return [];
    return Object.entries(groups)
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([name, value]) => ({
        name,
        value: Math.round((value / total) * 1000) / 10,
        color: ASSET_CLASS_COLORS[name] ?? "#9497a9",
      }));
  }, [positionList]);

  const assetClassKey = assetClassRaw
    .map((d) => `${d.name}:${d.value}`)
    .join(",");
  const prevAssetClassRef = useRef({ key: "", data: assetClassRaw });
  const assetClassData =
    assetClassKey === prevAssetClassRef.current.key
      ? prevAssetClassRef.current.data
      : (prevAssetClassRef.current = { key: assetClassKey, data: assetClassRaw }).data;

  const venueData = useMemo(() => {
    if (positionList.length === 0) return [];
    const groups: Record<string, number> = {};
    let total = 0;
    for (const p of positionList) {
      const mv = Math.abs(Number(p.quantity) * Number(p.marketPrice));
      groups[p.venueId] = (groups[p.venueId] ?? 0) + mv;
      total += mv;
    }
    if (total === 0) return [];
    return Object.entries(groups).map(([venueId, value]) => ({
      venueId,
      percentage: (value / total) * 100,
      notional: value,
    }));
  }, [positionList]);

  return (
    <div className="flex flex-col gap-4">
      {/* ── Header ─────────────────────────────────────────── */}
      <div className="flex items-center justify-between">
        <h2 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
          Portfolio
        </h2>
        {posLoading && (
          <span className="text-xs text-text-muted">Loading...</span>
        )}
      </div>

      {posError && (
        <div className="rounded border border-accent-red/30 bg-accent-red/10 px-3 py-2 text-xs text-accent-red">
          {posError}
        </div>
      )}

      {/* ── Summary Cards ──────────────────────────────────── */}
      <div className="grid grid-cols-4 gap-3">
        <SummaryCard
          label="Total NAV"
          value={totalNav !== undefined ? formatCurrency(totalNav) : "$--"}
          loading={summaryLoading}
        />
        <SummaryCard
          label="Day P&L"
          value={summary ? formatCurrency(summary.dailyPnl) : "$--"}
          valueClass={pnlPositive ? "text-accent-green" : "text-accent-red"}
          icon={
            summary ? (
              pnlPositive ? (
                <ArrowUp />
              ) : (
                <ArrowDown />
              )
            ) : undefined
          }
          loading={summaryLoading}
        />
        <SummaryCard
          label="Unsettled Cash"
          value={formatCurrency(unsettledCash)}
          valueClass="text-text-secondary"
          loading={summaryLoading}
        />
        <SummaryCard
          label="Available Cash"
          value={
            availableCash !== undefined
              ? formatCurrency(availableCash)
              : "$--"
          }
          valueClass="text-accent-blue"
          loading={summaryLoading}
        />
      </div>

      {/* ── Exposure Charts ────────────────────────────────── */}
      <div className="grid grid-cols-2 gap-3">
        {/* Asset Class Donut */}
        <div className="rounded-lg border border-border bg-bg-secondary px-4 py-3">
          <h3 className="mb-2 text-[10px] uppercase tracking-wider text-text-muted">
            Exposure by Asset Class
          </h3>
          <div className="relative h-48">
            {posLoading ? (
              <div className="flex h-full items-center justify-center text-xs text-text-muted animate-pulse">
                Loading...
              </div>
            ) : (
              <>
                <ExposureTreemap data={assetClassData} />
                {/* Legend */}
                <div className="absolute bottom-0 left-0 flex flex-wrap gap-x-3 gap-y-1 px-1">
                  {assetClassData.map((d) => (
                    <div
                      key={d.name}
                      className="flex items-center gap-1 text-[10px] text-text-muted"
                    >
                      <span
                        className="inline-block h-2 w-2 rounded-full"
                        style={{ backgroundColor: d.color }}
                      />
                      {d.name}
                    </div>
                  ))}
                </div>
              </>
            )}
          </div>
        </div>

        {/* Venue Bar Chart */}
        <div className="rounded-lg border border-border bg-bg-secondary px-4 py-3">
          <h3 className="mb-2 text-[10px] uppercase tracking-wider text-text-muted">
            Exposure by Venue
          </h3>
          <div className="h-48">
            {posLoading ? (
              <div className="flex h-full items-center justify-center text-xs text-text-muted animate-pulse">
                Loading...
              </div>
            ) : venueData.length === 0 ? (
              <div className="flex h-full items-center justify-center text-xs text-text-muted">
                No venue data
              </div>
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <BarChart
                  data={venueData}
                  layout="vertical"
                  margin={{ top: 4, right: 12, bottom: 4, left: 4 }}
                >
                  <XAxis
                    type="number"
                    domain={[0, 100]}
                    tick={{ fontSize: 10, fill: "#9497a9", fontFamily: "'IBM Plex Sans', sans-serif" }}
                    tickFormatter={(v: number) => `${v}%`}
                    axisLine={false}
                    tickLine={false}
                  />
                  <YAxis
                    type="category"
                    dataKey="venueId"
                    width={80}
                    tick={{ fontSize: 10, fill: "#686b82", fontFamily: "'IBM Plex Sans', sans-serif" }}
                    axisLine={false}
                    tickLine={false}
                  />
                  <Tooltip content={<VenueTooltip />} cursor={{ fill: "rgba(0,0,0,0.04)" }} />
                  <Bar dataKey="percentage" radius={[0, 4, 4, 0]} barSize={18}>
                    {venueData.map((_, i) => (
                      <Cell
                        key={`bar-${i}`}
                        fill={VENUE_COLORS[i % VENUE_COLORS.length]}
                      />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            )}
          </div>
        </div>
      </div>

      {/* ── Position Table ─────────────────────────────────── */}
      <div className="flex flex-col gap-2">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
          Positions
        </h3>
        <PositionTable positions={positionList} totalNav={totalNav} />
      </div>
    </div>
  );
}

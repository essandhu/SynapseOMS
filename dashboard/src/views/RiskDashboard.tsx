import { useEffect } from "react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import { useRiskStore } from "../stores/riskStore";
import { VaRGauge } from "../components/VaRGauge";
import { DrawdownChart } from "../components/DrawdownChart";
import { MonteCarloPlot } from "../components/MonteCarloPlot";
import { GreeksHeatmap } from "../components/GreeksHeatmap";
import { ConcentrationTreemap } from "../components/ConcentrationTreemap";
import type { SettlementTimeline } from "../api/types";

function formatCurrency(amount: string): string {
  const num = parseFloat(amount);
  if (Number.isNaN(num)) return amount;
  return num.toLocaleString("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  });
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

/** Derive NAV percentage from VaR amount (approximation using total unsettled as proxy) */
function varNavPct(varAmount: string | null): number | null {
  if (!varAmount) return null;
  const num = parseFloat(varAmount);
  if (Number.isNaN(num)) return null;
  // VaR amount is already a percentage-like value in the store;
  // we display as-is and let the backend compute the actual NAV%
  return Math.abs(num);
}

function SettlementBarTooltip({
  active,
  payload,
  label,
}: {
  active?: boolean;
  payload?: { value: number }[];
  label?: string;
}) {
  if (!active || !payload?.length || !label) return null;
  return (
    <div className="rounded border border-border bg-bg-secondary px-3 py-2 font-mono text-xs shadow-lg">
      <p className="text-text-muted">{formatDate(label)}</p>
      <p className="text-accent-blue font-medium">
        ${payload[0].value.toLocaleString()}
      </p>
    </div>
  );
}

function SettlementSection({
  settlement,
}: {
  settlement: SettlementTimeline | null;
}) {
  if (!settlement) {
    return (
      <div className="rounded border border-border bg-bg-secondary p-4">
        <h3 className="mb-2 font-mono text-xs font-semibold uppercase tracking-wider text-text-muted">
          Settlement Risk
        </h3>
        <div className="flex h-32 items-center justify-center">
          <span className="animate-pulse font-mono text-xs text-text-muted">
            Loading...
          </span>
        </div>
      </div>
    );
  }

  const barData = settlement.entries.map((e) => ({
    date: e.date,
    amount: parseFloat(e.amount),
    instrumentId: e.instrumentId,
    assetClass: e.assetClass,
  }));

  return (
    <div className="rounded border border-border bg-bg-secondary p-4">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="font-mono text-xs font-semibold uppercase tracking-wider text-text-muted">
          Settlement Risk
        </h3>
        <span className="font-mono text-sm font-bold text-accent-yellow">
          {formatCurrency(settlement.totalUnsettled)} unsettled
        </span>
      </div>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        {/* Horizontal bar chart */}
        <div>
          <ResponsiveContainer width="100%" height={Math.max(barData.length * 36, 120)}>
            <BarChart
              data={barData}
              layout="vertical"
              margin={{ top: 0, right: 4, left: 0, bottom: 0 }}
            >
              <CartesianGrid
                strokeDasharray="3 3"
                stroke="#1f2937"
                horizontal={false}
              />
              <XAxis
                type="number"
                tick={{
                  fill: "#6b7280",
                  fontSize: 10,
                  fontFamily: "monospace",
                }}
                axisLine={{ stroke: "#374151" }}
                tickLine={false}
                tickFormatter={(v: number) =>
                  `$${(v / 1000).toFixed(0)}k`
                }
              />
              <YAxis
                type="category"
                dataKey="date"
                tick={{
                  fill: "#6b7280",
                  fontSize: 10,
                  fontFamily: "monospace",
                }}
                axisLine={{ stroke: "#374151" }}
                tickLine={false}
                tickFormatter={formatDate}
                width={64}
              />
              <Tooltip content={<SettlementBarTooltip />} />
              <Bar
                dataKey="amount"
                fill="#3b82f6"
                radius={[0, 4, 4, 0]}
                isAnimationActive={false}
              />
            </BarChart>
          </ResponsiveContainer>
        </div>

        {/* Table of pending settlements */}
        <div className="overflow-auto">
          <table className="w-full font-mono text-xs">
            <thead>
              <tr className="border-b border-border text-left text-text-muted">
                <th className="pb-2 pr-4 font-medium">Date</th>
                <th className="pb-2 pr-4 font-medium">Instrument</th>
                <th className="pb-2 pr-4 font-medium">Class</th>
                <th className="pb-2 text-right font-medium">Amount</th>
              </tr>
            </thead>
            <tbody>
              {settlement.entries.map((entry, i) => (
                <tr
                  key={`${entry.date}-${entry.instrumentId}-${i}`}
                  className="border-b border-border/50"
                >
                  <td className="py-1.5 pr-4 text-text-secondary">
                    {formatDate(entry.date)}
                  </td>
                  <td className="py-1.5 pr-4 text-text-primary">
                    {entry.instrumentId}
                  </td>
                  <td className="py-1.5 pr-4">
                    <span className="rounded bg-bg-tertiary px-1.5 py-0.5 text-[10px] uppercase text-text-muted">
                      {entry.assetClass}
                    </span>
                  </td>
                  <td className="py-1.5 text-right text-accent-yellow">
                    {formatCurrency(entry.amount)}
                  </td>
                </tr>
              ))}
              {settlement.entries.length === 0 && (
                <tr>
                  <td
                    colSpan={4}
                    className="py-4 text-center text-text-muted"
                  >
                    No pending settlements
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

export function RiskDashboard() {
  const varMetrics = useRiskStore((s) => s.var);
  const drawdown = useRiskStore((s) => s.drawdown);
  const settlement = useRiskStore((s) => s.settlement);
  const greeks = useRiskStore((s) => s.greeks);
  const concentration = useRiskStore((s) => s.concentration);
  const loading = useRiskStore((s) => s.loading);
  const error = useRiskStore((s) => s.error);
  const subscribe = useRiskStore((s) => s.subscribe);

  // Subscribe on mount — handles initial fetch + 30s polling
  useEffect(() => {
    const unsubscribe = subscribe();
    return unsubscribe;
  }, [subscribe]);

  // Parse Monte Carlo values from VaR response
  const mcVarAmount =
    varMetrics?.monteCarloVaR != null
      ? parseFloat(varMetrics.monteCarloVaR)
      : null;
  const mcCvarAmount =
    varMetrics?.cvar != null ? parseFloat(varMetrics.cvar) : null;

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h2 className="font-mono text-xs font-semibold uppercase tracking-wider text-text-muted">
          Risk Dashboard
        </h2>
        {loading && (
          <span className="animate-pulse font-mono text-xs text-text-muted">
            Refreshing...
          </span>
        )}
      </div>

      {error && (
        <div className="rounded border border-accent-red/30 bg-accent-red/10 px-3 py-2 font-mono text-xs text-accent-red">
          {error}
        </div>
      )}

      {/* VaR Gauges — top row */}
      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        <VaRGauge
          title="Historical VaR"
          amount={varMetrics?.historicalVaR ?? null}
          navPercentage={varNavPct(varMetrics?.historicalVaR ?? null)}
          confidence={varMetrics?.confidence ?? 95}
          lastComputed={varMetrics?.computedAt ?? null}
          method="Historical"
          loading={loading}
        />
        <VaRGauge
          title="Parametric VaR"
          amount={varMetrics?.parametricVaR ?? null}
          navPercentage={varNavPct(varMetrics?.parametricVaR ?? null)}
          confidence={varMetrics?.confidence ?? 95}
          lastComputed={varMetrics?.computedAt ?? null}
          method="Parametric"
          loading={loading}
        />
        <VaRGauge
          title="Monte Carlo VaR"
          amount={varMetrics?.monteCarloVaR ?? null}
          navPercentage={varNavPct(varMetrics?.monteCarloVaR ?? null)}
          confidence={varMetrics?.confidence ?? 95}
          lastComputed={varMetrics?.computedAt ?? null}
          method="Monte Carlo"
          loading={loading}
        />
      </div>

      {/* Monte Carlo Distribution Histogram */}
      <MonteCarloPlot
        distribution={varMetrics?.monteCarloDistribution ?? null}
        varAmount={mcVarAmount}
        cvarAmount={mcCvarAmount}
      />

      {/* Greeks Heatmap */}
      <GreeksHeatmap greeks={greeks} />

      {/* Drawdown + Concentration — side by side */}
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
        <DrawdownChart
          data={drawdown?.history ?? []}
          currentDrawdown={drawdown?.current ?? 0}
        />
        <ConcentrationTreemap concentration={concentration} />
      </div>

      {/* Settlement Risk — bottom */}
      <SettlementSection settlement={settlement} />
    </div>
  );
}

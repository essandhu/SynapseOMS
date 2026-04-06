import { useMemo } from "react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Cell,
  ReferenceLine,
  ResponsiveContainer,
} from "recharts";
import { useThemeColors } from "../theme/terminal";

const NUM_BINS = 50;
const COLOR_NORMAL = "#7132f5";
const COLOR_CVAR = "#991b1b";
const COLOR_VAR_LINE = "#ef4444";
const COLOR_CVAR_LINE = "#f97316";

export interface MonteCarloPlotProps {
  distribution: number[] | null;
  varAmount: number | null; // VaR threshold (positive number = loss)
  cvarAmount: number | null; // CVaR threshold (positive number = loss)
}

interface Bin {
  rangeStart: number;
  rangeEnd: number;
  midpoint: number;
  count: number;
}

function buildHistogram(values: number[], numBins: number): Bin[] {
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min;

  if (range === 0) {
    return [{ rangeStart: min, rangeEnd: max, midpoint: min, count: values.length }];
  }

  const binWidth = range / numBins;
  const bins: Bin[] = [];

  for (let i = 0; i < numBins; i++) {
    const rangeStart = min + i * binWidth;
    const rangeEnd = rangeStart + binWidth;
    bins.push({
      rangeStart,
      rangeEnd,
      midpoint: rangeStart + binWidth / 2,
      count: 0,
    });
  }

  for (const v of values) {
    let idx = Math.floor((v - min) / binWidth);
    if (idx >= numBins) idx = numBins - 1;
    bins[idx].count++;
  }

  return bins;
}

function formatPnL(value: number): string {
  const abs = Math.abs(value);
  if (abs >= 1_000_000) return `$${(value / 1_000_000).toFixed(1)}M`;
  if (abs >= 1_000) return `$${(value / 1_000).toFixed(0)}k`;
  return `$${value.toFixed(0)}`;
}

function HistogramTooltip({
  active,
  payload,
}: {
  active?: boolean;
  payload?: { payload: Bin; value: number }[];
}) {
  if (!active || !payload?.length) return null;
  const bin = payload[0].payload;
  return (
    <div className="rounded border border-border bg-bg-secondary px-3 py-2 text-xs" style={{ boxShadow: "rgba(0,0,0,0.03) 0px 4px 24px" }}>
      <p className="text-text-muted">
        {formatPnL(bin.rangeStart)} to {formatPnL(bin.rangeEnd)}
      </p>
      <p className="text-accent-blue font-medium">
        {bin.count.toLocaleString()} simulations
      </p>
    </div>
  );
}

export function MonteCarloPlot({
  distribution,
  varAmount,
  cvarAmount,
}: MonteCarloPlotProps) {
  const theme = useThemeColors();
  const bins = useMemo(() => {
    if (!distribution || distribution.length === 0) return null;
    return buildHistogram(distribution, NUM_BINS);
  }, [distribution]);

  // VaR and CVaR are provided as positive loss numbers; on the P&L axis they are negative
  const varX = varAmount != null ? -varAmount : null;
  const cvarX = cvarAmount != null ? -cvarAmount : null;

  if (!bins) {
    return (
      <div className="rounded border border-border border-dashed bg-bg-secondary p-4">
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-text-muted">
          Monte Carlo Distribution
        </h3>
        <div className="flex h-40 items-center justify-center">
          <span className="text-xs text-text-muted">
            No Monte Carlo data available
          </span>
        </div>
      </div>
    );
  }

  return (
    <div className="rounded border border-border bg-bg-secondary p-4">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
          Monte Carlo Distribution
        </h3>
        <span className="text-[10px] text-text-muted">
          {distribution!.length.toLocaleString()} simulations &middot; {NUM_BINS} bins
        </span>
      </div>

      <ResponsiveContainer width="100%" height={240}>
        <BarChart
          data={bins}
          margin={{ top: 8, right: 8, left: 0, bottom: 0 }}
        >
          <CartesianGrid strokeDasharray="3 3" stroke={theme.colors.bg.tertiary} vertical={false} />
          <XAxis
            dataKey="midpoint"
            type="number"
            domain={["dataMin", "dataMax"]}
            tick={{ fill: theme.colors.text.muted, fontSize: 10, fontFamily: theme.fonts.sans }}
            axisLine={{ stroke: theme.colors.border }}
            tickLine={false}
            tickFormatter={formatPnL}
            label={{
              value: "P&L ($)",
              position: "insideBottomRight",
              offset: -4,
              style: { fill: theme.colors.text.muted, fontSize: 10, fontFamily: theme.fonts.sans },
            }}
          />
          <YAxis
            tick={{ fill: theme.colors.text.muted, fontSize: 10, fontFamily: theme.fonts.sans }}
            axisLine={{ stroke: theme.colors.border }}
            tickLine={false}
            label={{
              value: "Frequency",
              angle: -90,
              position: "insideLeft",
              offset: 12,
              style: { fill: theme.colors.text.muted, fontSize: 10, fontFamily: theme.fonts.sans },
            }}
          />
          <Tooltip content={<HistogramTooltip />} />

          {varX != null && (
            <ReferenceLine
              x={varX}
              stroke={COLOR_VAR_LINE}
              strokeWidth={2}
              strokeDasharray="4 2"
              label={{
                value: `VaR ${formatPnL(varX)}`,
                position: "top",
                fill: COLOR_VAR_LINE,
                fontSize: 10,
                fontFamily: theme.fonts.sans,
              }}
            />
          )}

          {cvarX != null && (
            <ReferenceLine
              x={cvarX}
              stroke={COLOR_CVAR_LINE}
              strokeWidth={2}
              strokeDasharray="4 2"
              label={{
                value: `CVaR ${formatPnL(cvarX)}`,
                position: "top",
                fill: COLOR_CVAR_LINE,
                fontSize: 10,
                fontFamily: theme.fonts.sans,
              }}
            />
          )}

          <Bar dataKey="count" isAnimationActive={false} radius={[1, 1, 0, 0]}>
            {bins.map((bin, index) => {
              const isCvarRegion = varX != null && bin.midpoint < varX;
              return (
                <Cell
                  key={`cell-${index}`}
                  fill={isCvarRegion ? COLOR_CVAR : COLOR_NORMAL}
                />
              );
            })}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}

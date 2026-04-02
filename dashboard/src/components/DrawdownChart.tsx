import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  ReferenceLine,
} from "recharts";

interface DrawdownChartProps {
  data: { date: string; drawdown: number }[];
  currentDrawdown: number;
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function CustomTooltip({
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
      <p className="text-accent-red font-medium">
        {payload[0].value.toFixed(2)}%
      </p>
    </div>
  );
}

export function DrawdownChart({ data, currentDrawdown }: DrawdownChartProps) {
  if (!data.length) {
    return (
      <div className="rounded border border-border bg-bg-secondary p-4">
        <h3 className="mb-2 font-mono text-xs font-semibold uppercase tracking-wider text-text-muted">
          Drawdown from Peak
        </h3>
        <div className="flex h-48 items-center justify-center">
          <span className="font-mono text-xs text-text-muted">
            No drawdown data available
          </span>
        </div>
      </div>
    );
  }

  return (
    <div className="rounded border border-border bg-bg-secondary p-4">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="font-mono text-xs font-semibold uppercase tracking-wider text-text-muted">
          Drawdown from Peak
        </h3>
        <span
          className={`font-mono text-sm font-bold ${
            currentDrawdown < -5
              ? "text-accent-red"
              : currentDrawdown < -2
                ? "text-accent-yellow"
                : "text-accent-green"
          }`}
        >
          {currentDrawdown.toFixed(2)}%
        </span>
      </div>

      <ResponsiveContainer width="100%" height={220}>
        <AreaChart
          data={data}
          margin={{ top: 4, right: 4, left: 0, bottom: 0 }}
        >
          <defs>
            <linearGradient id="drawdownFill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#3b82f6" stopOpacity={0.1} />
              <stop offset="100%" stopColor="#ef4444" stopOpacity={0.4} />
            </linearGradient>
          </defs>
          <CartesianGrid
            strokeDasharray="3 3"
            stroke="#1f2937"
            vertical={false}
          />
          <XAxis
            dataKey="date"
            tickFormatter={formatDate}
            tick={{ fill: "#6b7280", fontSize: 10, fontFamily: "monospace" }}
            axisLine={{ stroke: "#374151" }}
            tickLine={false}
          />
          <YAxis
            tick={{ fill: "#6b7280", fontSize: 10, fontFamily: "monospace" }}
            axisLine={{ stroke: "#374151" }}
            tickLine={false}
            tickFormatter={(v: number) => `${v}%`}
            domain={["dataMin - 1", 0]}
          />
          <Tooltip content={<CustomTooltip />} />
          <ReferenceLine y={0} stroke="#374151" strokeWidth={1} />
          <Area
            type="monotone"
            dataKey="drawdown"
            stroke="#3b82f6"
            strokeWidth={2}
            fill="url(#drawdownFill)"
            isAnimationActive={false}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}

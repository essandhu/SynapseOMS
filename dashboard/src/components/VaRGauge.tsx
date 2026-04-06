import { useThemeColors } from "../theme/terminal";

interface VaRGaugeProps {
  title: string;
  amount: string | null;
  navPercentage: number | null;
  confidence: number;
  lastComputed: string | null;
  method: string;
  loading?: boolean;
}

function colorForPercentage(pct: number | null): string {
  if (pct === null) return "#9497a9"; // muted
  if (pct < 2) return "#149e61"; // green
  if (pct <= 5) return "#eab308"; // yellow
  return "#ef4444"; // red
}

function twColorForPercentage(pct: number | null): string {
  if (pct === null) return "text-text-muted";
  if (pct < 2) return "text-accent-green";
  if (pct <= 5) return "text-accent-yellow";
  return "text-accent-red";
}

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

function formatTime(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleTimeString("en-US", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

export function VaRGauge({
  title,
  amount,
  navPercentage,
  confidence,
  lastComputed,
  method,
  loading = false,
}: VaRGaugeProps) {
  const theme = useThemeColors();
  const isLoading = loading && amount === null;
  const color = colorForPercentage(navPercentage);
  const twColor = twColorForPercentage(navPercentage);

  // Gauge arc parameters
  const radius = 54;
  const strokeWidth = 8;
  const circumference = Math.PI * radius; // half circle
  const pct = navPercentage !== null ? Math.min(navPercentage / 10, 1) : 0;
  const dashOffset = circumference * (1 - pct);

  return (
    <div className="rounded border border-border bg-bg-secondary p-4">
      <div className="mb-1 flex items-center justify-between">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
          {title}
        </h3>
        <span className="text-[10px] text-text-muted">{method}</span>
      </div>

      {isLoading ? (
        <div className="flex h-32 items-center justify-center">
          <span className="text-xs text-text-muted animate-pulse">
            Loading...
          </span>
        </div>
      ) : (
        <div className="flex flex-col items-center">
          {/* SVG gauge arc */}
          <svg viewBox="0 0 120 70" className="mb-1 w-full max-w-[180px]">
            {/* Background arc */}
            <path
              d="M 6 64 A 54 54 0 0 1 114 64"
              fill="none"
              stroke={theme.colors.bg.tertiary}
              strokeWidth={strokeWidth}
              strokeLinecap="round"
            />
            {/* Value arc */}
            <path
              d="M 6 64 A 54 54 0 0 1 114 64"
              fill="none"
              stroke={color}
              strokeWidth={strokeWidth}
              strokeLinecap="round"
              strokeDasharray={circumference}
              strokeDashoffset={dashOffset}
              className="transition-all duration-700 ease-out"
            />
          </svg>

          {/* Large value */}
          <span className={`-mt-2 text-2xl font-bold ${twColor}`}>
            {amount !== null ? formatCurrency(amount) : "—"}
          </span>

          {/* NAV percentage */}
          <span className={`mt-1 text-sm font-medium ${twColor}`}>
            {navPercentage !== null ? `${navPercentage.toFixed(2)}% NAV` : "—"}
          </span>

          {/* Footer info */}
          <div className="mt-3 flex w-full items-center justify-between text-text-muted">
            <span className="text-[10px]">
              {confidence}% confidence
            </span>
            <span className="text-[10px]">
              {lastComputed ? formatTime(lastComputed) : "—"}
            </span>
          </div>
        </div>
      )}
    </div>
  );
}

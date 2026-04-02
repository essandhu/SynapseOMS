import { useState, useEffect, useRef, useCallback } from "react";
import type { Venue } from "../api/types";

export interface VenueCardProps {
  venue: Venue;
  onConnect: () => void;
  onDisconnect: () => void;
  onTestConnection: () => void;
}

const STATUS_CONFIG: Record<
  Venue["status"],
  { color: string; label: string; dotClass: string }
> = {
  connected: {
    color: "text-accent-green",
    label: "Connected",
    dotClass: "bg-accent-green animate-status-pulse",
  },
  disconnected: {
    color: "text-accent-red",
    label: "Disconnected",
    dotClass: "bg-accent-red",
  },
  degraded: {
    color: "text-accent-yellow",
    label: "Degraded",
    dotClass: "bg-accent-yellow animate-status-pulse",
  },
  authentication: {
    color: "text-accent-yellow",
    label: "Authenticating",
    dotClass: "bg-accent-yellow animate-status-pulse",
  },
};

const VENUE_TYPE_COLORS: Record<Venue["type"], string> = {
  exchange: "border-accent-blue/40 text-accent-blue bg-accent-blue/10",
  dark_pool: "border-accent-purple/40 text-accent-purple bg-accent-purple/10",
  simulated: "border-text-muted/40 text-text-muted bg-text-muted/10",
  tokenized:
    "border-accent-yellow/40 text-accent-yellow bg-accent-yellow/10",
};

/** SVG icon paths per venue type */
const VENUE_TYPE_ICONS: Record<Venue["type"], string> = {
  exchange:
    "M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z",
  dark_pool:
    "M3.98 8.223A10.477 10.477 0 001.934 12c1.292 4.338 5.31 7.5 10.066 7.5.993 0 1.953-.138 2.863-.395M6.228 6.228A10.45 10.45 0 0112 4.5c4.756 0 8.773 3.162 10.065 7.498a10.523 10.523 0 01-4.293 5.774M6.228 6.228L3 3m3.228 3.228l3.65 3.65m7.894 7.894L21 21m-3.228-3.228l-3.65-3.65m0 0a3 3 0 10-4.243-4.243m4.242 4.242L9.88 9.88",
  simulated:
    "M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.104c.251.023.501.05.75.082M19.8 15.3l-1.57.393A9.065 9.065 0 0112 15a9.065 9.065 0 00-6.23.693L5 14.5m14.8.8l1.402 1.402c1.232 1.232.65 3.318-1.067 3.611A48.309 48.309 0 0112 21c-2.773 0-5.491-.235-8.135-.687-1.718-.293-2.3-2.379-1.067-3.61L5 14.5",
  tokenized:
    "M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 0v3.75m-16.5-3.75v3.75m16.5 0v3.75C20.25 16.153 16.556 18 12 18s-8.25-1.847-8.25-4.125v-3.75m16.5 0c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125",
};

const ASSET_CLASS_LABELS: Record<string, string> = {
  equity: "Equity",
  crypto: "Crypto",
  tokenized_security: "Tokenized",
  future: "Futures",
  option: "Options",
};

function formatRelativeTime(iso: string): string {
  const now = Date.now();
  const then = new Date(iso).getTime();
  const diffMs = now - then;

  if (Number.isNaN(diffMs) || diffMs < 0) return "just now";
  if (diffMs < 1000) return "just now";
  if (diffMs < 60_000) {
    const sec = Math.floor(diffMs / 1000);
    return `${sec} second${sec !== 1 ? "s" : ""} ago`;
  }
  if (diffMs < 3_600_000) {
    const min = Math.floor(diffMs / 60_000);
    return `${min} minute${min !== 1 ? "s" : ""} ago`;
  }
  const hrs = Math.floor(diffMs / 3_600_000);
  return `${hrs} hour${hrs !== 1 ? "s" : ""} ago`;
}

/** Returns a color class based on fill rate percentage */
function fillRateColor(rate: number): string {
  if (rate >= 0.8) return "text-accent-green";
  if (rate >= 0.5) return "text-accent-yellow";
  return "text-accent-red";
}

/** Inline SVG sparkline for latency history */
function LatencySparkline({
  points,
  width = 120,
  height = 28,
}: {
  points: number[];
  width?: number;
  height?: number;
}) {
  if (points.length < 2) return null;

  const max = Math.max(...points);
  const min = Math.min(...points);
  const range = max - min || 1;
  const padding = 2;
  const usableH = height - padding * 2;
  const usableW = width - padding * 2;
  const step = usableW / (points.length - 1);

  const pathData = points
    .map((v, i) => {
      const x = padding + i * step;
      const y = padding + usableH - ((v - min) / range) * usableH;
      return `${i === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");

  // Determine color based on last point relative to trend
  const last = points[points.length - 1];
  const strokeColor =
    last > max * 0.8
      ? "stroke-accent-red"
      : last > max * 0.5
        ? "stroke-accent-yellow"
        : "stroke-accent-green";

  return (
    <svg
      width={width}
      height={height}
      className="inline-block"
      data-testid="latency-sparkline"
      aria-label={`Latency sparkline: ${points.length} data points`}
    >
      <path
        d={pathData}
        fill="none"
        className={strokeColor}
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      {/* Dot at latest point */}
      <circle
        cx={padding + (points.length - 1) * step}
        cy={
          padding +
          usableH -
          ((last - min) / range) * usableH
        }
        r="2"
        className={`${strokeColor.replace("stroke-", "fill-")}`}
      />
    </svg>
  );
}

/** Mock drill-down stats for a venue (placeholder until backend endpoint exists) */
function useMockDrillDownStats(venue: Venue) {
  // Derive mock stats from the venue's existing metrics
  return {
    orderCount: Math.floor(venue.fillRate * 1000 + venue.latencyP50Ms * 2),
    fillCount: Math.floor(
      venue.fillRate * (venue.fillRate * 1000 + venue.latencyP50Ms * 2),
    ),
    rejectCount: Math.floor(
      (1 - venue.fillRate) *
        (venue.fillRate * 1000 + venue.latencyP50Ms * 2) *
        0.3,
    ),
    avgFillTimeMs: Math.round(venue.latencyP50Ms * 1.2),
    partialFillRate: Math.max(0, venue.fillRate - 0.05),
  };
}

export function VenueCard({
  venue,
  onConnect,
  onDisconnect,
  onTestConnection,
}: VenueCardProps) {
  const [testing, setTesting] = useState(false);
  const [testLatency, setTestLatency] = useState<number | null>(null);
  const [expanded, setExpanded] = useState(false);
  const [latencyHistory, setLatencyHistory] = useState<number[]>([]);
  const drillDownRef = useRef<HTMLDivElement>(null);

  const status = STATUS_CONFIG[venue.status] ?? STATUS_CONFIG.disconnected;
  const isConnected = venue.status === "connected";
  const typeColor = VENUE_TYPE_COLORS[venue.type] ?? VENUE_TYPE_COLORS.exchange;
  const typeIcon = VENUE_TYPE_ICONS[venue.type] ?? VENUE_TYPE_ICONS.exchange;
  const drillDown = useMockDrillDownStats(venue);

  // Track latency history (last 20 data points) from venue P50 updates
  useEffect(() => {
    if (isConnected && venue.latencyP50Ms > 0) {
      setLatencyHistory((prev) => {
        const next = [...prev, venue.latencyP50Ms];
        return next.length > 20 ? next.slice(-20) : next;
      });
    }
  }, [venue.latencyP50Ms, isConnected]);

  // Seed latency history with simulated data points when connected
  useEffect(() => {
    if (isConnected && latencyHistory.length === 0) {
      const base = venue.latencyP50Ms || 50;
      const seed = Array.from({ length: 15 }, () =>
        Math.round(base + (Math.random() - 0.5) * base * 0.4),
      );
      setLatencyHistory(seed);
    }
  }, [isConnected]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleTest = useCallback(async () => {
    setTesting(true);
    setTestLatency(null);
    const start = performance.now();
    try {
      onTestConnection();
    } finally {
      // Simulate measuring round-trip time
      const elapsed = Math.round(performance.now() - start);
      // Use a synthetic latency based on venue P50 for demo purposes
      const syntheticLatency =
        venue.latencyP50Ms > 0
          ? venue.latencyP50Ms + Math.round(Math.random() * 20 - 10)
          : elapsed;
      setTimeout(() => {
        setTestLatency(syntheticLatency);
        setTesting(false);
      }, 800);
    }
  }, [onTestConnection, venue.latencyP50Ms]);

  const toggleExpand = useCallback(() => {
    setExpanded((prev) => !prev);
  }, []);

  return (
    <div
      className={`flex flex-col gap-3 rounded-lg border border-border bg-bg-secondary p-4 transition-all hover:border-gray-600 ${expanded ? "ring-1 ring-accent-blue/20" : ""}`}
    >
      {/* Header: name + status (clickable for expand) */}
      <div
        className="flex cursor-pointer items-start justify-between"
        onClick={toggleExpand}
        role="button"
        tabIndex={0}
        aria-expanded={expanded}
        aria-label={`${venue.name} details`}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            toggleExpand();
          }
        }}
      >
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            {/* Venue type icon */}
            <svg
              className={`h-4 w-4 flex-shrink-0 ${typeColor.split(" ").find((c) => c.startsWith("text-")) ?? "text-text-muted"}`}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={1.5}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d={typeIcon}
              />
            </svg>
            <h3 className="truncate font-mono text-base font-semibold text-text-primary">
              {venue.name}
            </h3>
          </div>
          <span
            className={`mt-1 inline-block rounded-full border px-2 py-0.5 font-mono text-[10px] font-medium uppercase tracking-wider ${typeColor}`}
          >
            {venue.type.replace("_", " ")}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1.5">
            <span
              className={`inline-block h-2 w-2 rounded-full ${status.dotClass}`}
            />
            <span className={`font-mono text-xs ${status.color}`}>
              {status.label}
            </span>
          </div>
          {/* Expand/collapse chevron */}
          <svg
            className={`h-4 w-4 text-text-muted transition-transform duration-200 ${expanded ? "rotate-180" : ""}`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M19.5 8.25l-7.5 7.5-7.5-7.5"
            />
          </svg>
        </div>
      </div>

      {/* Asset classes */}
      <div className="flex flex-wrap gap-1">
        {venue.supportedAssets.map((ac) => (
          <span
            key={ac}
            className="rounded border border-border bg-bg-tertiary px-1.5 py-0.5 font-mono text-[10px] text-text-secondary"
          >
            {ASSET_CLASS_LABELS[ac] ?? ac}
          </span>
        ))}
      </div>

      {/* Fill rate */}
      <div className="font-mono text-xs text-text-muted">
        <span className="text-text-secondary">Fill Rate:</span>{" "}
        <span className={fillRateColor(venue.fillRate)} data-testid="fill-rate">
          {(venue.fillRate * 100).toFixed(1)}%
        </span>
      </div>

      {/* Latency + sparkline (when connected) */}
      {isConnected && (
        <div className="flex items-center justify-between gap-2">
          <div className="font-mono text-xs text-text-muted">
            <span className="text-text-secondary">P50:</span>{" "}
            <span className="text-accent-green">{venue.latencyP50Ms}ms</span>
            <span className="mx-1.5 text-border">|</span>
            <span className="text-text-secondary">P99:</span>{" "}
            <span className="text-accent-yellow">{venue.latencyP99Ms}ms</span>
          </div>
          {latencyHistory.length >= 2 && (
            <LatencySparkline points={latencyHistory} />
          )}
        </div>
      )}

      {/* Last heartbeat */}
      {venue.lastHeartbeat && (
        <div className="font-mono text-[11px] text-text-muted">
          Heartbeat: {formatRelativeTime(venue.lastHeartbeat)}
        </div>
      )}

      {/* Credentials indicator */}
      <div className="flex items-center gap-1.5 font-mono text-xs">
        {venue.hasCredentials ? (
          <>
            <svg
              className="h-3.5 w-3.5 text-accent-green"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z"
              />
            </svg>
            <span className="text-text-secondary">Credentials stored</span>
          </>
        ) : (
          <>
            <svg
              className="h-3.5 w-3.5 text-accent-yellow"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z"
              />
            </svg>
            <span className="text-accent-yellow">No credentials</span>
          </>
        )}
      </div>

      {/* Test connection result */}
      {testLatency !== null && (
        <div className="rounded border border-accent-blue/20 bg-accent-blue/5 px-2 py-1 font-mono text-xs text-accent-blue" data-testid="test-latency-result">
          Round-trip: {testLatency}ms
        </div>
      )}

      {/* Expandable drill-down section */}
      <div
        ref={drillDownRef}
        className={`overflow-hidden transition-all duration-300 ease-in-out ${
          expanded ? "max-h-96 opacity-100" : "max-h-0 opacity-0"
        }`}
        data-testid="drill-down-section"
      >
        <div className="border-t border-border pt-3">
          <h4 className="mb-2 font-mono text-xs font-semibold text-text-secondary">
            Venue Metrics
          </h4>
          <div className="grid grid-cols-2 gap-2">
            <div className="rounded border border-border bg-bg-tertiary px-2 py-1.5">
              <div className="font-mono text-[10px] text-text-muted">
                Orders
              </div>
              <div className="font-mono text-sm font-medium text-text-primary" data-testid="order-count">
                {drillDown.orderCount.toLocaleString()}
              </div>
            </div>
            <div className="rounded border border-border bg-bg-tertiary px-2 py-1.5">
              <div className="font-mono text-[10px] text-text-muted">
                Fills
              </div>
              <div className="font-mono text-sm font-medium text-accent-green">
                {drillDown.fillCount.toLocaleString()}
              </div>
            </div>
            <div className="rounded border border-border bg-bg-tertiary px-2 py-1.5">
              <div className="font-mono text-[10px] text-text-muted">
                Rejects
              </div>
              <div className="font-mono text-sm font-medium text-accent-red">
                {drillDown.rejectCount.toLocaleString()}
              </div>
            </div>
            <div className="rounded border border-border bg-bg-tertiary px-2 py-1.5">
              <div className="font-mono text-[10px] text-text-muted">
                Avg Fill Time
              </div>
              <div className="font-mono text-sm font-medium text-text-primary">
                {drillDown.avgFillTimeMs}ms
              </div>
            </div>
          </div>

          {/* Partial fill rate */}
          <div className="mt-2 font-mono text-xs text-text-muted">
            <span className="text-text-secondary">Partial Fill Rate:</span>{" "}
            <span className={fillRateColor(drillDown.partialFillRate)}>
              {(drillDown.partialFillRate * 100).toFixed(1)}%
            </span>
          </div>

          {/* Historical latency mini-chart (wider in drill-down) */}
          {latencyHistory.length >= 2 && (
            <div className="mt-3">
              <div className="mb-1 font-mono text-[10px] text-text-muted">
                Latency History (last {latencyHistory.length} samples)
              </div>
              <div className="rounded border border-border bg-bg-tertiary p-2">
                <LatencySparkline
                  points={latencyHistory}
                  width={260}
                  height={40}
                />
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Action buttons */}
      <div className="mt-auto flex gap-2 border-t border-border pt-3">
        {isConnected ? (
          <button
            onClick={onDisconnect}
            className="flex-1 rounded border border-accent-red/30 bg-accent-red/10 px-3 py-1.5 font-mono text-xs font-medium text-accent-red transition-colors hover:bg-accent-red/20"
          >
            Disconnect
          </button>
        ) : (
          <button
            onClick={onConnect}
            disabled={!venue.hasCredentials}
            className="flex-1 rounded border border-accent-green/30 bg-accent-green/10 px-3 py-1.5 font-mono text-xs font-medium text-accent-green transition-colors hover:bg-accent-green/20 disabled:cursor-not-allowed disabled:opacity-40"
          >
            Connect
          </button>
        )}
        <button
          onClick={handleTest}
          disabled={testing || !venue.hasCredentials}
          className="flex-1 rounded border border-accent-blue/30 bg-accent-blue/10 px-3 py-1.5 font-mono text-xs font-medium text-accent-blue transition-colors hover:bg-accent-blue/20 disabled:cursor-not-allowed disabled:opacity-40"
        >
          {testing ? "Testing..." : "Test Connection"}
        </button>
      </div>
    </div>
  );
}

// Export for testing
export { LatencySparkline, fillRateColor, STATUS_CONFIG, VENUE_TYPE_COLORS, VENUE_TYPE_ICONS, ASSET_CLASS_LABELS };

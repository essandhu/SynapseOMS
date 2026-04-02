import { useState } from "react";
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

export function VenueCard({
  venue,
  onConnect,
  onDisconnect,
  onTestConnection,
}: VenueCardProps) {
  const [testing, setTesting] = useState(false);

  const status = STATUS_CONFIG[venue.status] ?? STATUS_CONFIG.disconnected;
  const isConnected = venue.status === "connected";
  const typeColor = VENUE_TYPE_COLORS[venue.type] ?? VENUE_TYPE_COLORS.exchange;

  const handleTest = async () => {
    setTesting(true);
    try {
      onTestConnection();
    } finally {
      // Allow the parent to resolve; reset after short delay for UX
      setTimeout(() => setTesting(false), 1500);
    }
  };

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-border bg-bg-secondary p-4 transition-colors hover:border-gray-600">
      {/* Header: name + status */}
      <div className="flex items-start justify-between">
        <div className="min-w-0 flex-1">
          <h3 className="truncate font-mono text-base font-semibold text-text-primary">
            {venue.name}
          </h3>
          <span
            className={`mt-1 inline-block rounded-full border px-2 py-0.5 font-mono text-[10px] font-medium uppercase tracking-wider ${typeColor}`}
          >
            {venue.type.replace("_", " ")}
          </span>
        </div>
        <div className="flex items-center gap-1.5">
          <span className={`inline-block h-2 w-2 rounded-full ${status.dotClass}`} />
          <span className={`font-mono text-xs ${status.color}`}>{status.label}</span>
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

      {/* Latency (when connected) */}
      {isConnected && (
        <div className="font-mono text-xs text-text-muted">
          <span className="text-text-secondary">P50:</span>{" "}
          <span className="text-accent-green">{venue.latencyP50Ms}ms</span>
          <span className="mx-1.5 text-border">|</span>
          <span className="text-text-secondary">P99:</span>{" "}
          <span className="text-accent-yellow">{venue.latencyP99Ms}ms</span>
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

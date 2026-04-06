import { useInsightStore } from "../stores/insightStore";
import type { AnomalyAlert } from "../api/types";

const severityColors: Record<string, string> = {
  critical: "#ef4444",
  warning: "#eab308",
  info: "#7132f5",
};

function relativeTime(timestamp: string): string {
  const diff = Date.now() - new Date(timestamp).getTime();
  const mins = Math.floor(diff / 60_000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

export function AnomalyAlertsTab() {
  const { anomalyAlerts, acknowledgeAlert } = useInsightStore();

  if (anomalyAlerts.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-text-muted">
        <p className="text-lg">No anomalies detected.</p>
        <p className="text-sm mt-2">
          The system monitors market data 24/7.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {anomalyAlerts.map((alert) => (
        <AlertCard
          key={alert.id}
          alert={alert}
          onAcknowledge={() => acknowledgeAlert(alert.id)}
        />
      ))}
    </div>
  );
}

function AlertCard({
  alert,
  onAcknowledge,
}: {
  alert: AnomalyAlert;
  onAcknowledge: () => void;
}) {
  const color = severityColors[alert.severity] ?? severityColors.info;
  const isAcknowledged = alert.acknowledged;

  return (
    <div
      className={`bg-bg-secondary rounded-lg p-4 transition-opacity ${
        isAcknowledged ? "opacity-50" : ""
      }`}
      style={{
        borderLeft: isAcknowledged ? "3px solid var(--border-color)" : `3px solid ${color}`,
      }}
      data-testid="alert-card"
    >
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          {/* Severity badge */}
          <span
            className="inline-block px-2 py-0.5 rounded text-xs font-bold uppercase"
            style={{ backgroundColor: color, color: "#fff" }}
            data-testid="severity-badge"
          >
            {alert.severity}
          </span>
          <span className="text-text-primary text-sm font-medium">
            {alert.instrumentId}
          </span>
          <span className="text-text-muted text-sm">{alert.venueId}</span>
        </div>
        <span className="text-text-muted text-xs">
          {relativeTime(alert.timestamp)}
        </span>
      </div>

      <p className="text-text-secondary text-sm mt-2">{alert.description}</p>

      {/* Feature breakdown */}
      <div className="flex flex-wrap gap-3 mt-2">
        {Object.entries(alert.features).map(([key, value]) => (
          <span key={key} className="text-xs text-text-muted">
            {key}: <span className="text-text-secondary">{value.toFixed(2)}</span>
          </span>
        ))}
      </div>

      <div className="flex items-center justify-between mt-3">
        <span className="text-xs text-text-muted">
          Score: {alert.anomalyScore.toFixed(3)}
        </span>
        {!isAcknowledged && (
          <button
            onClick={onAcknowledge}
            className="text-xs text-text-secondary hover:text-text-primary border border-border rounded px-2 py-1"
            data-testid="acknowledge-btn"
          >
            Acknowledge
          </button>
        )}
      </div>
    </div>
  );
}

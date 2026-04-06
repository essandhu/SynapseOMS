import { useInsightStore } from "../stores/insightStore";

const severityColors: Record<string, string> = {
  critical: "#ef4444",
  warning: "#eab308",
  info: "#7132f5",
};

export function AlertBadge() {
  const alerts = useInsightStore((s) => s.anomalyAlerts);
  const unacknowledged = alerts.filter((a) => !a.acknowledged);
  const count = unacknowledged.length;

  if (count === 0) return null;

  const highestSeverity = unacknowledged.reduce((worst, alert) => {
    const order = { critical: 3, warning: 2, info: 1 };
    const alertPriority = order[alert.severity as keyof typeof order] ?? 0;
    const worstPriority = order[worst as keyof typeof order] ?? 0;
    return alertPriority > worstPriority ? alert.severity : worst;
  }, "info");

  const bgColor = severityColors[highestSeverity] ?? severityColors.info;

  return (
    <span
      className="inline-flex h-5 min-w-[20px] items-center justify-center rounded-full px-1.5 text-xs font-bold text-white"
      style={{ backgroundColor: bgColor }}
      data-testid="alert-badge"
    >
      {count}
    </span>
  );
}

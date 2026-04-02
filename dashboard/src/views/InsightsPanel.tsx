import { useEffect, useState } from "react";
import { useInsightStore } from "../stores/insightStore";
import { ExecutionAnalysisTab } from "../components/ExecutionAnalysisTab";
import { RebalancingTab } from "../components/RebalancingTab";
import { AnomalyAlertsTab } from "../components/AnomalyAlertsTab";

type TabId = "execution" | "rebalancing" | "anomalies";

const TABS: { id: TabId; label: string }[] = [
  { id: "execution", label: "Execution Analysis" },
  { id: "rebalancing", label: "Rebalancing" },
  { id: "anomalies", label: "Anomaly Alerts" },
];

export function InsightsPanel() {
  const [activeTab, setActiveTab] = useState<TabId>("execution");
  const { fetchExecutionReports, fetchAnomalyAlerts, unacknowledgedCount } =
    useInsightStore();

  useEffect(() => {
    fetchExecutionReports();
    fetchAnomalyAlerts();
  }, [fetchExecutionReports, fetchAnomalyAlerts]);

  const alertCount = unacknowledgedCount();

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold text-[#a855f7]">AI Insights</h2>

      {/* Tab bar */}
      <div className="flex border-b border-[#2a2a3e]" role="tablist">
        {TABS.map((tab) => (
          <button
            key={tab.id}
            role="tab"
            aria-selected={activeTab === tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={[
              "relative px-4 py-2 text-sm font-medium transition-colors",
              activeTab === tab.id
                ? "text-[#a855f7] border-b-2 border-[#a855f7]"
                : "text-gray-500 hover:text-gray-400",
            ].join(" ")}
          >
            {tab.label}
            {tab.id === "anomalies" && alertCount > 0 && (
              <span
                className="ml-2 inline-flex items-center justify-center h-5 min-w-[20px] rounded-full bg-red-500 px-1.5 text-xs font-bold text-white"
                data-testid="alert-count-badge"
              >
                {alertCount}
              </span>
            )}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div role="tabpanel">
        {activeTab === "execution" && <ExecutionAnalysisTab />}
        {activeTab === "rebalancing" && <RebalancingTab />}
        {activeTab === "anomalies" && <AnomalyAlertsTab />}
      </div>
    </div>
  );
}

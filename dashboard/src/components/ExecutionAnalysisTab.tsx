import { useState } from "react";
import { useInsightStore } from "../stores/insightStore";
import type { ExecutionReport } from "../api/types";

const gradeColors: Record<string, string> = {
  A: "#22c55e", // green
  B: "#3b82f6", // blue
  C: "#eab308", // yellow
  D: "#f97316", // orange
  F: "#ef4444", // red
  "N/A": "#6b7280", // gray
};

export function ExecutionAnalysisTab() {
  const { executionReports } = useInsightStore();

  if (executionReports.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-gray-500">
        <p className="text-lg">No execution reports yet.</p>
        <p className="text-sm mt-2">
          Reports are generated automatically after trades complete.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {executionReports.map((report, idx) => (
        <ReportCard key={report.orderId ?? idx} report={report} />
      ))}
    </div>
  );
}

function ReportCard({ report }: { report: ExecutionReport }) {
  const [expanded, setExpanded] = useState(false);
  const gradeColor = gradeColors[report.overallGrade] ?? gradeColors["N/A"];

  return (
    <div className="bg-[#1e1e2e] border border-[#2a2a3e] rounded-lg p-4">
      <div className="flex items-start gap-4">
        {/* Grade badge */}
        <div
          className="flex-shrink-0 w-14 h-14 rounded-lg flex items-center justify-center text-2xl font-bold text-black"
          style={{ backgroundColor: gradeColor }}
          data-testid="grade-badge"
        >
          {report.overallGrade}
        </div>

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-3 mb-1">
            <span className="text-sm text-gray-400">
              Shortfall: {report.implementationShortfallBps.toFixed(1)} bps
            </span>
            <span className="text-sm text-gray-400">
              Impact: {report.marketImpactEstimateBps.toFixed(1)} bps
            </span>
          </div>
          <p className="text-gray-200 text-sm">{report.summary}</p>
        </div>
      </div>

      {/* Expandable detail */}
      <button
        onClick={() => setExpanded(!expanded)}
        className="mt-3 text-sm text-[#a855f7] hover:text-[#c084fc] cursor-pointer"
      >
        {expanded ? "Hide details" : "Show venue analysis & recommendations"}
      </button>

      {expanded && (
        <div className="mt-3 space-y-3">
          {report.venueAnalysis.length > 0 && (
            <div>
              <h4 className="text-sm font-semibold text-gray-300 mb-2">
                Venue Analysis
              </h4>
              {report.venueAnalysis.map((va) => (
                <div
                  key={va.venue}
                  className="flex items-center gap-2 text-sm text-gray-400 mb-1"
                >
                  <span
                    className="inline-block w-6 h-6 rounded text-center text-xs font-bold leading-6 text-black"
                    style={{
                      backgroundColor:
                        gradeColors[va.grade] ?? gradeColors["N/A"],
                    }}
                  >
                    {va.grade}
                  </span>
                  <span className="text-gray-300">{va.venue}:</span>
                  <span>{va.comment}</span>
                </div>
              ))}
            </div>
          )}

          {report.recommendations.length > 0 && (
            <div>
              <h4 className="text-sm font-semibold text-gray-300 mb-2">
                Recommendations
              </h4>
              <ul className="list-disc list-inside text-sm text-gray-400 space-y-1">
                {report.recommendations.map((rec, i) => (
                  <li key={i}>{rec}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

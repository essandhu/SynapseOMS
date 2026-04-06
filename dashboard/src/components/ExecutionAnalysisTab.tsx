import { useState } from "react";
import { useInsightStore } from "../stores/insightStore";
import type { ExecutionReport } from "../api/types";

const gradeColors: Record<string, string> = {
  A: "#149e61",
  B: "#7132f5",
  C: "#eab308",
  D: "#f97316",
  F: "#ef4444",
  "N/A": "#9497a9",
};

export function ExecutionAnalysisTab() {
  const { executionReports } = useInsightStore();

  if (executionReports.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-text-muted">
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
    <div className="bg-bg-secondary border border-border rounded-lg p-4">
      <div className="flex items-start gap-4">
        {/* Grade badge */}
        <div
          className="flex-shrink-0 w-14 h-14 rounded-lg flex items-center justify-center text-2xl font-bold text-white"
          style={{ backgroundColor: gradeColor }}
          data-testid="grade-badge"
        >
          {report.overallGrade}
        </div>

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-3 mb-1">
            <span className="text-sm text-text-secondary">
              Shortfall: {report.implementationShortfallBps.toFixed(1)} bps
            </span>
            <span className="text-sm text-text-secondary">
              Impact: {report.marketImpactEstimateBps.toFixed(1)} bps
            </span>
          </div>
          <p className="text-text-primary text-sm">{report.summary}</p>
        </div>
      </div>

      {/* Expandable detail */}
      <button
        onClick={() => setExpanded(!expanded)}
        className="mt-3 text-sm text-accent-blue hover:text-accent-blue/80 cursor-pointer"
      >
        {expanded ? "Hide details" : "Show venue analysis & recommendations"}
      </button>

      {expanded && (
        <div className="mt-3 space-y-3">
          {report.venueAnalysis.length > 0 && (
            <div>
              <h4 className="text-sm font-semibold text-text-primary mb-2">
                Venue Analysis
              </h4>
              {report.venueAnalysis.map((va) => (
                <div
                  key={va.venue}
                  className="flex items-center gap-2 text-sm text-text-secondary mb-1"
                >
                  <span
                    className="inline-block w-6 h-6 rounded text-center text-xs font-bold leading-6 text-white"
                    style={{
                      backgroundColor:
                        gradeColors[va.grade] ?? gradeColors["N/A"],
                    }}
                  >
                    {va.grade}
                  </span>
                  <span className="text-text-primary">{va.venue}:</span>
                  <span>{va.comment}</span>
                </div>
              ))}
            </div>
          )}

          {report.recommendations.length > 0 && (
            <div>
              <h4 className="text-sm font-semibold text-text-primary mb-2">
                Recommendations
              </h4>
              <ul className="list-disc list-inside text-sm text-text-secondary space-y-1">
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

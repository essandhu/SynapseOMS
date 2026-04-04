import { describe, it, expect } from "vitest";
import {
  VaRMetricsSchema,
  DrawdownDataSchema,
  SettlementTimelineSchema,
  PortfolioGreeksSchema,
  ConcentrationResultSchema,
  PortfolioSummarySchema,
  ExposureDataSchema,
  OptimizationResultSchema,
  ExecutionReportSchema,
  AnomalyAlertSchema,
  RebalanceResultSchema,
} from "./schemas";
import {
  mockVaR,
  mockDrawdown,
  mockSettlement,
  mockGreeks,
  mockConcentration,
  mockPortfolioSummary,
  mockExposure,
  mockOptimizationResult,
  mockExecutionReports,
  mockAnomalyAlerts,
  mockRebalanceResult,
} from "../mocks/data";

function expectValid(schema: { safeParse: (d: unknown) => { success: boolean; error?: { issues: { path: PropertyKey[]; message: string }[] } } }, data: unknown, label: string) {
  const result = schema.safeParse(data);
  if (!result.success) {
    expect.fail(
      `${label} schema validation failed:\n${result.error!.issues.map((i) => `  ${i.path.join(".")}: ${i.message}`).join("\n")}`,
    );
  }
}

describe("Risk Engine API contracts", () => {
  it("VaR metrics match schema", () => {
    expectValid(VaRMetricsSchema, mockVaR, "VaRMetrics");
  });

  it("Drawdown data matches schema", () => {
    expectValid(DrawdownDataSchema, mockDrawdown, "DrawdownData");
  });

  it("Settlement timeline matches schema", () => {
    expectValid(SettlementTimelineSchema, mockSettlement, "SettlementTimeline");
  });

  it("Portfolio Greeks match schema", () => {
    expectValid(PortfolioGreeksSchema, mockGreeks, "PortfolioGreeks");
  });

  it("Concentration result matches schema", () => {
    expectValid(
      ConcentrationResultSchema,
      mockConcentration,
      "ConcentrationResult",
    );
  });

  it("Portfolio summary matches schema", () => {
    expectValid(
      PortfolioSummarySchema,
      mockPortfolioSummary,
      "PortfolioSummary",
    );
  });

  it("Exposure data matches schema", () => {
    expectValid(ExposureDataSchema, mockExposure, "ExposureData");
  });

  it("Optimization result matches schema", () => {
    expectValid(
      OptimizationResultSchema,
      mockOptimizationResult,
      "OptimizationResult",
    );
  });

  describe("Execution reports", () => {
    it.each(mockExecutionReports.map((r, i) => [i, r]))(
      "report %i matches schema",
      (_i, report) => {
        expectValid(ExecutionReportSchema, report, "ExecutionReport");
      },
    );
  });

  describe("Anomaly alerts", () => {
    it.each(mockAnomalyAlerts.map((a) => [a.id, a]))(
      "alert %s matches schema",
      (_id, alert) => {
        expectValid(AnomalyAlertSchema, alert, "AnomalyAlert");
      },
    );
  });

  it("Rebalance result matches schema", () => {
    expectValid(
      RebalanceResultSchema,
      mockRebalanceResult,
      "RebalanceResult",
    );
  });
});

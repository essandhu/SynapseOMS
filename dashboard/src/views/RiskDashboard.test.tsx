import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { RiskDashboard } from "./RiskDashboard";

// Mock the risk store
const mockSubscribe = vi.fn(() => vi.fn());
const mockFetchVaR = vi.fn();
const mockFetchDrawdown = vi.fn();
const mockFetchSettlement = vi.fn();
const mockFetchGreeks = vi.fn();
const mockFetchConcentration = vi.fn();

let mockStoreState: Record<string, unknown> = {};

vi.mock("../stores/riskStore", () => ({
  useRiskStore: (selector: (s: Record<string, unknown>) => unknown) => {
    return selector(mockStoreState);
  },
}));

// Mock recharts
vi.mock("recharts", () => ({
  BarChart: ({ children }: { children: React.ReactNode }) => <div data-testid="bar-chart">{children}</div>,
  Bar: () => <div />,
  XAxis: () => <div />,
  YAxis: () => <div />,
  CartesianGrid: () => <div />,
  Tooltip: () => <div />,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

// Mock child components
vi.mock("../components/VaRGauge", () => ({
  VaRGauge: ({ title }: { title: string }) => (
    <div data-testid="var-gauge">{title}</div>
  ),
}));

vi.mock("../components/DrawdownChart", () => ({
  DrawdownChart: () => <div data-testid="drawdown-chart" />,
}));

vi.mock("../components/MonteCarloPlot", () => ({
  MonteCarloPlot: () => <div data-testid="monte-carlo-plot" />,
}));

vi.mock("../components/GreeksHeatmap", () => ({
  GreeksHeatmap: () => <div data-testid="greeks-heatmap" />,
}));

vi.mock("../components/ConcentrationTreemap", () => ({
  ConcentrationTreemap: () => <div data-testid="concentration-treemap" />,
}));

describe("RiskDashboard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockStoreState = {
      var: null,
      drawdown: null,
      settlement: null,
      greeks: null,
      concentration: null,
      loading: false,
      error: null,
      subscribe: mockSubscribe,
      fetchVaR: mockFetchVaR,
      fetchDrawdown: mockFetchDrawdown,
      fetchSettlement: mockFetchSettlement,
      fetchGreeks: mockFetchGreeks,
      fetchConcentration: mockFetchConcentration,
    };
  });

  it("renders VaR gauge components including Monte Carlo", () => {
    mockStoreState.var = {
      historicalVaR: "12500.00",
      parametricVaR: "11200.00",
      monteCarloVaR: "13000.00",
      cvar: "15800.00",
      confidence: 95,
      horizon: "1d",
      computedAt: "2026-04-01T10:00:00Z",
      monteCarloDistribution: [100, 200, 300],
    };

    render(<RiskDashboard />);

    const gauges = screen.getAllByTestId("var-gauge");
    expect(gauges).toHaveLength(3);
    expect(screen.getByText("Historical VaR")).toBeInTheDocument();
    expect(screen.getByText("Parametric VaR")).toBeInTheDocument();
    expect(screen.getByText("Monte Carlo VaR")).toBeInTheDocument();
  });

  it("renders Monte Carlo plot component", () => {
    render(<RiskDashboard />);

    expect(screen.getByTestId("monte-carlo-plot")).toBeInTheDocument();
  });

  it("renders Greeks heatmap component", () => {
    render(<RiskDashboard />);

    expect(screen.getByTestId("greeks-heatmap")).toBeInTheDocument();
  });

  it("renders concentration treemap component", () => {
    render(<RiskDashboard />);

    expect(screen.getByTestId("concentration-treemap")).toBeInTheDocument();
  });

  it("renders drawdown chart section", () => {
    render(<RiskDashboard />);

    expect(screen.getByTestId("drawdown-chart")).toBeInTheDocument();
  });

  it("renders settlement risk section", () => {
    render(<RiskDashboard />);

    expect(screen.getByText("Settlement Risk")).toBeInTheDocument();
  });

  it("handles null/loading state gracefully", () => {
    mockStoreState.loading = true;
    mockStoreState.var = null;
    mockStoreState.drawdown = null;
    mockStoreState.settlement = null;

    render(<RiskDashboard />);

    // Should render without crashing
    expect(screen.getByText("Risk Dashboard")).toBeInTheDocument();
    expect(screen.getByText("Refreshing...")).toBeInTheDocument();
  });

  it("displays error message when error exists", () => {
    mockStoreState.error = "Failed to fetch risk data";

    render(<RiskDashboard />);

    expect(screen.getByText("Failed to fetch risk data")).toBeInTheDocument();
  });

  it("subscribes to store on mount", () => {
    render(<RiskDashboard />);

    expect(mockSubscribe).toHaveBeenCalledOnce();
  });
});

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { PortfolioView } from "./PortfolioView";

// Mock the stores
const mockPositionSubscribe = vi.fn(() => vi.fn());
const mockRiskSubscribe = vi.fn(() => vi.fn());

vi.mock("../stores/positionStore", () => ({
  usePositionStore: (selector: (s: Record<string, unknown>) => unknown) => {
    const state: Record<string, unknown> = {
      positions: new Map(),
      loading: false,
      error: null,
      subscribe: mockPositionSubscribe,
    };
    return selector(state);
  },
}));

vi.mock("../stores/riskStore", () => ({
  useRiskStore: (selector: (s: Record<string, unknown>) => unknown) => {
    const state: Record<string, unknown> = {
      settlement: null,
      subscribe: mockRiskSubscribe,
    };
    return selector(state);
  },
}));

// Mock the REST API
vi.mock("../api/rest", () => ({
  fetchPortfolioSummary: vi.fn().mockResolvedValue({
    totalNav: "100000.00",
    totalPnl: "5000.00",
    dailyPnl: "1200.00",
    positionCount: 3,
  }),
  fetchExposure: vi.fn().mockResolvedValue({
    byAssetClass: [],
    byVenue: [],
  }),
}));

// Mock recharts to avoid rendering issues in jsdom
vi.mock("recharts", () => ({
  BarChart: ({ children }: { children: React.ReactNode }) => <div data-testid="bar-chart">{children}</div>,
  Bar: () => <div />,
  XAxis: () => <div />,
  YAxis: () => <div />,
  Tooltip: () => <div />,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  Cell: () => <div />,
}));

// Mock child components
vi.mock("../components/PositionTable", () => ({
  PositionTable: ({ positions }: { positions: unknown[] }) => (
    <div data-testid="position-table">
      {positions.length === 0 ? "No positions" : `${positions.length} positions`}
    </div>
  ),
}));

vi.mock("../components/ExposureTreemap", () => ({
  ExposureTreemap: () => <div data-testid="exposure-treemap" />,
}));

describe("PortfolioView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders summary cards", async () => {
    render(<PortfolioView />);

    // Check that summary card labels are present
    expect(screen.getByText("Total NAV")).toBeInTheDocument();
    expect(screen.getByText("Day P&L")).toBeInTheDocument();
    expect(screen.getByText("Unsettled Cash")).toBeInTheDocument();
    expect(screen.getByText("Available Cash")).toBeInTheDocument();
  });

  it("renders position table section", () => {
    render(<PortfolioView />);

    expect(screen.getByText("Positions")).toBeInTheDocument();
    expect(screen.getByTestId("position-table")).toBeInTheDocument();
  });

  it("handles empty positions gracefully", () => {
    render(<PortfolioView />);

    expect(screen.getByTestId("position-table")).toHaveTextContent("No positions");
  });

  it("renders Portfolio header", () => {
    render(<PortfolioView />);

    expect(screen.getByText("Portfolio")).toBeInTheDocument();
  });

  it("renders exposure sections", () => {
    render(<PortfolioView />);

    expect(screen.getByText("Exposure by Asset Class")).toBeInTheDocument();
    expect(screen.getByText("Exposure by Venue")).toBeInTheDocument();
  });
});

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { PortfolioView } from "./PortfolioView";
import type { Position } from "../api/types";

// Mock the stores
const mockPositionSubscribe = vi.fn(() => vi.fn());
const mockRiskSubscribe = vi.fn(() => vi.fn());

const mockPosition: Position = {
  instrumentId: "AAPL",
  venueId: "alpaca",
  quantity: "100",
  averageCost: "150.00",
  marketPrice: "155.00",
  unrealizedPnl: "500.00",
  realizedPnl: "0.00",
  unsettledQuantity: "0",
  assetClass: "equity",
  quoteCurrency: "USD",
};

let mockPositions = new Map<string, Position>();

vi.mock("../stores/positionStore", () => ({
  usePositionStore: (selector: (s: Record<string, unknown>) => unknown) => {
    const state: Record<string, unknown> = {
      positions: mockPositions,
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
    totalPnl: "500.00",
    dailyPnl: "500.00",
    cash: "84500.00",
    availableCash: "84500.00",
    positionCount: 1,
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

// Mock child components — capture props for assertion
let capturedPositionTableProps: { positions: unknown[]; totalNav?: number } = { positions: [] };
vi.mock("../components/PositionTable", () => ({
  PositionTable: (props: { positions: unknown[]; totalNav?: number }) => {
    capturedPositionTableProps = props;
    return (
      <div data-testid="position-table">
        {props.positions.length === 0 ? "No positions" : `${props.positions.length} positions`}
      </div>
    );
  },
}));

let capturedTreemapData: { name: string; value: number }[] = [];
vi.mock("../components/ExposureTreemap", () => ({
  ExposureTreemap: ({ data }: { data: { name: string; value: number }[] }) => {
    capturedTreemapData = data;
    return <div data-testid="exposure-treemap">{data.length} entries</div>;
  },
}));

describe("PortfolioView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPositions = new Map();
    capturedPositionTableProps = { positions: [] };
    capturedTreemapData = [];
  });

  it("renders summary cards", async () => {
    render(<PortfolioView />);

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

  it("computes NAV from availableCash + position market values", async () => {
    // Position: 100 shares × $155 = $15,500 market value
    // availableCash from risk engine: $84,500
    // Expected NAV: $84,500 + $15,500 = $100,000
    mockPositions = new Map([["AAPL:alpaca", mockPosition]]);

    render(<PortfolioView />);

    // Wait for summary to load
    await vi.waitFor(() => {
      // NAV should be passed to PositionTable
      expect(capturedPositionTableProps.totalNav).toBeCloseTo(100000, 0);
    });
  });

  it("computes exposure from position data by asset class", async () => {
    const crypto: Position = {
      ...mockPosition,
      instrumentId: "BTC-USD",
      venueId: "binance",
      quantity: "0.5",
      marketPrice: "31000.00",
      assetClass: "crypto",
    };
    mockPositions = new Map([
      ["AAPL:alpaca", mockPosition],
      ["BTC-USD:binance", crypto],
    ]);

    render(<PortfolioView />);

    await vi.waitFor(() => {
      // AAPL: 100 * 155 = 15500, BTC: 0.5 * 31000 = 15500
      // Each should be 50%
      expect(capturedTreemapData).toHaveLength(2);
      const equityEntry = capturedTreemapData.find((d) => d.name === "equity");
      const cryptoEntry = capturedTreemapData.find((d) => d.name === "crypto");
      expect(equityEntry?.value).toBeCloseTo(50, 0);
      expect(cryptoEntry?.value).toBeCloseTo(50, 0);
    });
  });

  it("uses availableCash from risk engine directly", async () => {
    mockPositions = new Map([["AAPL:alpaca", mockPosition]]);

    render(<PortfolioView />);

    // Available Cash should show the value from the risk engine ($84,500),
    // NOT a derived value that fluctuates with market prices.
    await vi.waitFor(() => {
      expect(screen.getByText("$84,500.00")).toBeInTheDocument();
    });
  });
});

import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ConcentrationTreemap } from "./ConcentrationTreemap";
import type { ConcentrationResult, Position } from "../api/types";

const sampleConcentration: ConcentrationResult = {
  singleName: {
    "BTC-USD": 35,
    "ETH-USD": 25,
    "AAPL": 20,
    "SPY-FUT": 12,
    "BTC-CALL-50K": 8,
  },
  byAssetClass: {
    crypto: 60,
    equity: 20,
    future: 12,
    option: 8,
  },
  byVenue: {
    binance: 45,
    coinbase: 15,
    nyse: 20,
    cme: 12,
    deribit: 8,
  },
  warnings: ["BTC-USD exceeds single-name limit"],
  hhi: 2234,
};

const samplePositions: Position[] = [
  {
    instrumentId: "BTC-USD",
    venueId: "binance",
    quantity: "1.5",
    averageCost: "42000",
    marketPrice: "45000",
    unrealizedPnl: "4500",
    realizedPnl: "0",
    unsettledQuantity: "0",
    assetClass: "crypto",
    quoteCurrency: "USD",
  },
  {
    instrumentId: "ETH-USD",
    venueId: "coinbase",
    quantity: "10",
    averageCost: "2800",
    marketPrice: "3000",
    unrealizedPnl: "2000",
    realizedPnl: "0",
    unsettledQuantity: "0",
    assetClass: "crypto",
    quoteCurrency: "USD",
  },
  {
    instrumentId: "AAPL",
    venueId: "nyse",
    quantity: "100",
    averageCost: "175",
    marketPrice: "180",
    unrealizedPnl: "500",
    realizedPnl: "0",
    unsettledQuantity: "0",
    assetClass: "equity",
    quoteCurrency: "USD",
  },
  {
    instrumentId: "SPY-FUT",
    venueId: "cme",
    quantity: "2",
    averageCost: "4500",
    marketPrice: "4520",
    unrealizedPnl: "40",
    realizedPnl: "0",
    unsettledQuantity: "0",
    assetClass: "future",
    quoteCurrency: "USD",
  },
  {
    instrumentId: "BTC-CALL-50K",
    venueId: "deribit",
    quantity: "5",
    averageCost: "2000",
    marketPrice: "2500",
    unrealizedPnl: "2500",
    realizedPnl: "0",
    unsettledQuantity: "0",
    assetClass: "option",
    quoteCurrency: "USD",
  },
];

describe("ConcentrationTreemap", () => {
  it("renders with sample data and shows instrument names", () => {
    render(
      <ConcentrationTreemap
        concentration={sampleConcentration}
        positions={samplePositions}
      />,
    );

    expect(screen.getByText("BTC-USD")).toBeInTheDocument();
    expect(screen.getByText("ETH-USD")).toBeInTheDocument();
    expect(screen.getByText("AAPL")).toBeInTheDocument();
  });

  it("renders treemap rectangles for each instrument", () => {
    const { container } = render(
      <ConcentrationTreemap
        concentration={sampleConcentration}
        positions={samplePositions}
      />,
    );

    const rects = container.querySelectorAll("[data-testid='treemap-cell']");
    expect(rects.length).toBe(5);
  });

  it("shows warning badge on instruments exceeding threshold", () => {
    const { container } = render(
      <ConcentrationTreemap
        concentration={sampleConcentration}
        positions={samplePositions}
      />,
    );

    const warnings = container.querySelectorAll(
      "[data-testid='treemap-warning']",
    );
    // BTC-USD (35% > 25%) and ETH-USD (25% >= 25%) should have warnings
    // Also BTC-USD appears in warnings array
    expect(warnings.length).toBeGreaterThanOrEqual(1);
  });

  it("shows tooltip on cell hover", () => {
    const { container } = render(
      <ConcentrationTreemap
        concentration={sampleConcentration}
        positions={samplePositions}
      />,
    );

    const cells = container.querySelectorAll("[data-testid='treemap-cell']");
    fireEvent.mouseEnter(cells[0]);

    const tooltip = screen.getByTestId("treemap-tooltip");
    expect(tooltip).toBeInTheDocument();
  });

  it("hides tooltip on mouse leave", () => {
    const { container } = render(
      <ConcentrationTreemap
        concentration={sampleConcentration}
        positions={samplePositions}
      />,
    );

    const cells = container.querySelectorAll("[data-testid='treemap-cell']");
    fireEvent.mouseEnter(cells[0]);
    expect(screen.getByTestId("treemap-tooltip")).toBeInTheDocument();

    fireEvent.mouseLeave(cells[0]);
    expect(screen.queryByTestId("treemap-tooltip")).not.toBeInTheDocument();
  });

  it("shows placeholder when concentration is null", () => {
    render(<ConcentrationTreemap concentration={null} />);

    expect(screen.getByText(/no concentration data/i)).toBeInTheDocument();
  });

  it("shows placeholder when singleName is empty", () => {
    const empty: ConcentrationResult = {
      singleName: {},
      byAssetClass: {},
      byVenue: {},
      warnings: [],
      hhi: 0,
    };

    render(<ConcentrationTreemap concentration={empty} />);

    expect(screen.getByText(/no concentration data/i)).toBeInTheDocument();
  });

  it("handles single-position portfolio", () => {
    const single: ConcentrationResult = {
      singleName: { "BTC-USD": 100 },
      byAssetClass: { crypto: 100 },
      byVenue: { binance: 100 },
      warnings: ["BTC-USD exceeds single-name limit"],
      hhi: 10000,
    };

    const singlePos: Position[] = [
      {
        instrumentId: "BTC-USD",
        venueId: "binance",
        quantity: "1",
        averageCost: "45000",
        marketPrice: "45000",
        unrealizedPnl: "0",
        realizedPnl: "0",
        unsettledQuantity: "0",
        assetClass: "crypto",
        quoteCurrency: "USD",
      },
    ];

    const { container } = render(
      <ConcentrationTreemap concentration={single} positions={singlePos} />,
    );

    expect(screen.getByText("BTC-USD")).toBeInTheDocument();
    const rects = container.querySelectorAll("[data-testid='treemap-cell']");
    expect(rects.length).toBe(1);
  });

  it("renders the component title", () => {
    render(
      <ConcentrationTreemap
        concentration={sampleConcentration}
        positions={samplePositions}
      />,
    );

    expect(screen.getByText(/concentration risk/i)).toBeInTheDocument();
  });

  it("renders HHI indicator", () => {
    render(
      <ConcentrationTreemap
        concentration={sampleConcentration}
        positions={samplePositions}
      />,
    );

    const hhiEl = screen.getByText(/hhi/i);
    expect(hhiEl).toBeInTheDocument();
    expect(hhiEl.textContent).toMatch(/2,?234/);
  });
});

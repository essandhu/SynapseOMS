import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { PositionTable } from "./PositionTable";
import type { Position } from "../api/types";

const makePosition = (overrides: Partial<Position> = {}): Position => ({
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
  ...overrides,
});

describe("PositionTable", () => {
  it("renders empty state when no positions", () => {
    render(<PositionTable positions={[]} />);
    expect(screen.getByText("No positions")).toBeInTheDocument();
  });

  it("renders position data in all columns", () => {
    const pos = makePosition();
    render(<PositionTable positions={[pos]} totalNav={100000} />);

    expect(screen.getByText("AAPL")).toBeInTheDocument();
    expect(screen.getByText("alpaca")).toBeInTheDocument();
    // Quantity formatted with sign and 4 decimals
    expect(screen.getByText("+100.0000")).toBeInTheDocument();
    // Average cost and market price formatted to 2 decimals
    expect(screen.getByText("150.00")).toBeInTheDocument();
    expect(screen.getByText("155.00")).toBeInTheDocument();
    // P&L values
    expect(screen.getByText("500.00")).toBeInTheDocument();
    // Asset class
    expect(screen.getByText("equity")).toBeInTheDocument();
  });

  it("computes % of NAV correctly", () => {
    const pos = makePosition({ quantity: "100", marketPrice: "155.00" });
    // Market value = |100 * 155| = 15500, NAV = 100000, pct = 15.5%
    render(<PositionTable positions={[pos]} totalNav={100000} />);

    expect(screen.getByText("15.5%")).toBeInTheDocument();
  });

  it("hides % of NAV column when totalNav is not provided", () => {
    render(<PositionTable positions={[makePosition()]} />);

    expect(screen.queryByText("% of NAV")).not.toBeInTheDocument();
  });

  it("hides % of NAV column when totalNav is 0", () => {
    render(<PositionTable positions={[makePosition()]} totalNav={0} />);

    expect(screen.queryByText("% of NAV")).not.toBeInTheDocument();
  });

  it("does not produce NaN when marketPrice is '0'", () => {
    const pos = makePosition({ marketPrice: "0" });
    render(<PositionTable positions={[pos]} totalNav={100000} />);

    expect(screen.getByText("0.0%")).toBeInTheDocument();
    expect(screen.queryByText("NaN%")).not.toBeInTheDocument();
  });

  it("handles negative quantity (short position)", () => {
    const pos = makePosition({ quantity: "-50", unrealizedPnl: "-200.00" });
    render(<PositionTable positions={[pos]} totalNav={100000} />);

    expect(screen.getByText("-50.0000")).toBeInTheDocument();
  });
});

import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { GreeksHeatmap } from "./GreeksHeatmap";
import type { PortfolioGreeks } from "../api/types";

const sampleGreeks: PortfolioGreeks = {
  total: { delta: 0.85, gamma: 0.02, vega: 12.5, theta: -3.4, rho: 0.15 },
  byInstrument: {
    "BTC-PERP": { delta: 0.6, gamma: 0.01, vega: 8.0, theta: -2.0, rho: 0.1 },
    "ETH-PERP": { delta: 0.25, gamma: 0.01, vega: 4.5, theta: -1.4, rho: 0.05 },
  },
  computedAt: "2026-04-02T12:00:00Z",
};

describe("GreeksHeatmap", () => {
  it("renders with sample data and shows instrument names", () => {
    render(<GreeksHeatmap greeks={sampleGreeks} />);

    expect(screen.getByText("BTC-PERP")).toBeInTheDocument();
    expect(screen.getByText("ETH-PERP")).toBeInTheDocument();
  });

  it("renders Greek column headers", () => {
    render(<GreeksHeatmap greeks={sampleGreeks} />);

    expect(screen.getByText("Delta")).toBeInTheDocument();
    expect(screen.getByText("Gamma")).toBeInTheDocument();
    expect(screen.getByText("Vega")).toBeInTheDocument();
    expect(screen.getByText("Theta")).toBeInTheDocument();
    expect(screen.getByText("Rho")).toBeInTheDocument();
  });

  it("renders heatmap cells for each instrument-greek pair", () => {
    const { container } = render(<GreeksHeatmap greeks={sampleGreeks} />);

    // 2 instruments x 5 greeks = 10 cells
    const cells = container.querySelectorAll("[data-testid='heatmap-cell']");
    expect(cells.length).toBe(10);
  });

  it("shows tooltip on cell hover with exact value", () => {
    const { container } = render(<GreeksHeatmap greeks={sampleGreeks} />);

    const cells = container.querySelectorAll("[data-testid='heatmap-cell']");
    // Hover over the first cell (BTC-PERP / Delta = 0.6)
    fireEvent.mouseEnter(cells[0]);

    expect(screen.getByTestId("heatmap-tooltip")).toBeInTheDocument();
    expect(screen.getByTestId("heatmap-tooltip")).toHaveTextContent("0.6");
    expect(screen.getByTestId("heatmap-tooltip")).toHaveTextContent("BTC-PERP");
    expect(screen.getByTestId("heatmap-tooltip")).toHaveTextContent("Delta");
  });

  it("hides tooltip on mouse leave", () => {
    const { container } = render(<GreeksHeatmap greeks={sampleGreeks} />);

    const cells = container.querySelectorAll("[data-testid='heatmap-cell']");
    fireEvent.mouseEnter(cells[0]);
    expect(screen.getByTestId("heatmap-tooltip")).toBeInTheDocument();

    fireEvent.mouseLeave(cells[0]);
    expect(screen.queryByTestId("heatmap-tooltip")).not.toBeInTheDocument();
  });

  it("shows placeholder when greeks is null", () => {
    render(<GreeksHeatmap greeks={null} />);

    expect(screen.getByText(/no greeks data/i)).toBeInTheDocument();
  });

  it("shows placeholder when byInstrument is empty", () => {
    const emptyGreeks: PortfolioGreeks = {
      total: { delta: 0, gamma: 0, vega: 0, theta: 0, rho: 0 },
      byInstrument: {},
      computedAt: "2026-04-02T12:00:00Z",
    };

    render(<GreeksHeatmap greeks={emptyGreeks} />);

    expect(screen.getByText(/no greeks data/i)).toBeInTheDocument();
  });

  it("renders the component title", () => {
    render(<GreeksHeatmap greeks={sampleGreeks} />);

    expect(screen.getByText(/greeks heatmap/i)).toBeInTheDocument();
  });
});

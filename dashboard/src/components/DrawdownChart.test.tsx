import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { DrawdownChart } from "./DrawdownChart";

// Mock recharts
vi.mock("recharts", () => ({
  AreaChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="area-chart">{children}</div>
  ),
  Area: () => <div data-testid="area" />,
  XAxis: () => <div data-testid="x-axis" />,
  YAxis: () => <div data-testid="y-axis" />,
  CartesianGrid: () => <div />,
  Tooltip: () => <div />,
  ReferenceLine: () => <div data-testid="reference-line" />,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-container">{children}</div>
  ),
}));

describe("DrawdownChart", () => {
  it("shows placeholder when data is empty", () => {
    render(<DrawdownChart data={[]} currentDrawdown={0} />);

    expect(screen.getByText(/no drawdown data available/i)).toBeInTheDocument();
    expect(screen.queryByTestId("area-chart")).not.toBeInTheDocument();
  });

  it("renders the chart when data has entries", () => {
    const data = [
      { date: "2026-03-28", drawdown: -1.0 },
      { date: "2026-03-29", drawdown: -2.5 },
      { date: "2026-03-30", drawdown: -3.2 },
    ];

    render(<DrawdownChart data={data} currentDrawdown={-3.2} />);

    expect(screen.getByTestId("area-chart")).toBeInTheDocument();
    expect(screen.getByText("Drawdown from Peak")).toBeInTheDocument();
  });

  it("displays current drawdown badge with correct color for mild drawdown", () => {
    const data = [{ date: "2026-03-30", drawdown: -1.5 }];

    render(<DrawdownChart data={data} currentDrawdown={-1.5} />);

    // -1.5% is mild (not < -2) → green
    const badge = screen.getByText("-1.50%");
    expect(badge).toBeInTheDocument();
    expect(badge.className).toContain("text-accent-green");
  });

  it("displays yellow badge for moderate drawdown", () => {
    const data = [{ date: "2026-03-30", drawdown: -3.0 }];

    render(<DrawdownChart data={data} currentDrawdown={-3.0} />);

    // -3.0% is moderate (< -2 but not < -5) → yellow
    const badge = screen.getByText("-3.00%");
    expect(badge).toBeInTheDocument();
    expect(badge.className).toContain("text-accent-yellow");
  });

  it("displays red badge for severe drawdown", () => {
    const data = [{ date: "2026-03-30", drawdown: -7.0 }];

    render(<DrawdownChart data={data} currentDrawdown={-7.0} />);

    // -7.0% is severe (< -5) → red
    const badge = screen.getByText("-7.00%");
    expect(badge).toBeInTheDocument();
    expect(badge.className).toContain("text-accent-red");
  });
});

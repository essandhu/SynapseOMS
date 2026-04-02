import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MonteCarloPlot } from "./MonteCarloPlot";

// Mock recharts to avoid canvas/DOM measurement issues in tests
vi.mock("recharts", () => ({
  BarChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="bar-chart">{children}</div>
  ),
  Bar: ({ fill, dataKey }: { fill: string; dataKey: string }) => (
    <div data-testid="bar" data-fill={fill} data-datakey={dataKey} />
  ),
  XAxis: () => <div data-testid="x-axis" />,
  YAxis: () => <div data-testid="y-axis" />,
  CartesianGrid: () => <div />,
  Tooltip: () => <div />,
  Cell: ({ fill }: { fill: string }) => <div data-testid="cell" data-fill={fill} />,
  ReferenceLine: ({ x, stroke, label }: { x: number; stroke: string; label?: unknown }) => (
    <div
      data-testid="reference-line"
      data-x={x}
      data-stroke={stroke}
      data-label={typeof label === "object" && label !== null ? (label as { value: string }).value : ""}
    />
  ),
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-container">{children}</div>
  ),
}));

function generateDistribution(count: number, mean: number, stddev: number): number[] {
  // Simple deterministic pseudo-distribution for tests
  const result: number[] = [];
  for (let i = 0; i < count; i++) {
    result.push(mean + stddev * ((i / count) * 2 - 1));
  }
  return result;
}

describe("MonteCarloPlot", () => {
  it("renders without crash with valid data", () => {
    const dist = generateDistribution(1000, 0, 5000);

    render(
      <MonteCarloPlot distribution={dist} varAmount={3000} cvarAmount={4000} />,
    );

    expect(screen.getByTestId("responsive-container")).toBeInTheDocument();
    expect(screen.getByTestId("bar-chart")).toBeInTheDocument();
  });

  it("renders VaR reference line when varAmount is provided", () => {
    const dist = generateDistribution(1000, 0, 5000);

    render(
      <MonteCarloPlot distribution={dist} varAmount={3000} cvarAmount={4000} />,
    );

    const refLines = screen.getAllByTestId("reference-line");
    // Should have VaR line (and possibly CVaR line)
    const varLine = refLines.find((el) => el.getAttribute("data-stroke") === "#ef4444");
    expect(varLine).toBeTruthy();
    expect(varLine!.getAttribute("data-x")).toBe("-3000");
  });

  it("shows placeholder when distribution is null", () => {
    render(
      <MonteCarloPlot distribution={null} varAmount={null} cvarAmount={null} />,
    );

    expect(screen.getByText(/no monte carlo data/i)).toBeInTheDocument();
    expect(screen.queryByTestId("bar-chart")).not.toBeInTheDocument();
  });

  it("shows placeholder when distribution is empty", () => {
    render(
      <MonteCarloPlot distribution={[]} varAmount={null} cvarAmount={null} />,
    );

    expect(screen.getByText(/no monte carlo data/i)).toBeInTheDocument();
    expect(screen.queryByTestId("bar-chart")).not.toBeInTheDocument();
  });

  it("renders CVaR reference line when cvarAmount is provided", () => {
    const dist = generateDistribution(1000, 0, 5000);

    render(
      <MonteCarloPlot distribution={dist} varAmount={3000} cvarAmount={4000} />,
    );

    const refLines = screen.getAllByTestId("reference-line");
    const cvarLine = refLines.find((el) => el.getAttribute("data-x") === "-4000");
    expect(cvarLine).toBeTruthy();
  });

  it("renders without VaR/CVaR lines when amounts are null", () => {
    const dist = generateDistribution(1000, 0, 5000);

    render(
      <MonteCarloPlot distribution={dist} varAmount={null} cvarAmount={null} />,
    );

    expect(screen.getByTestId("bar-chart")).toBeInTheDocument();
    expect(screen.queryByTestId("reference-line")).not.toBeInTheDocument();
  });

  it("renders the header title", () => {
    const dist = generateDistribution(100, 0, 1000);

    render(
      <MonteCarloPlot distribution={dist} varAmount={null} cvarAmount={null} />,
    );

    expect(screen.getByText(/monte carlo distribution/i)).toBeInTheDocument();
  });
});

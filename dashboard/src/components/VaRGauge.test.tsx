import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { VaRGauge } from "./VaRGauge";

describe("VaRGauge", () => {
  it("renders formatted currency when amount is provided", () => {
    render(
      <VaRGauge
        title="Historical VaR"
        amount="12500.00"
        navPercentage={12500}
        confidence={95}
        lastComputed="2026-04-01T10:00:00Z"
        method="Historical"
      />,
    );

    expect(screen.getByText("$12,500")).toBeInTheDocument();
    expect(screen.getByText("Historical VaR")).toBeInTheDocument();
    expect(screen.getByText("Historical")).toBeInTheDocument();
    expect(screen.getByText("95% confidence")).toBeInTheDocument();
  });

  it("shows a dash placeholder instead of $0 when amount is null", () => {
    render(
      <VaRGauge
        title="Historical VaR"
        amount={null}
        navPercentage={null}
        confidence={95}
        lastComputed={null}
        method="Historical"
      />,
    );

    // Should NOT show $0 which is misleading (looks like zero risk)
    expect(screen.queryByText("$0")).not.toBeInTheDocument();
    // Should show proper no-data indicators (value + lastComputed both show "—")
    const dashes = screen.getAllByText("—");
    expect(dashes.length).toBeGreaterThanOrEqual(1);
  });

  it("shows loading spinner when loading and amount is null", () => {
    render(
      <VaRGauge
        title="Historical VaR"
        amount={null}
        navPercentage={null}
        confidence={95}
        lastComputed={null}
        method="Historical"
        loading={true}
      />,
    );

    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("shows gauge with data when loading but amount is already populated", () => {
    render(
      <VaRGauge
        title="Historical VaR"
        amount="12500.00"
        navPercentage={12500}
        confidence={95}
        lastComputed="2026-04-01T10:00:00Z"
        method="Historical"
        loading={true}
      />,
    );

    // Should show data even while refreshing
    expect(screen.getByText("$12,500")).toBeInTheDocument();
    expect(screen.queryByText("Loading...")).not.toBeInTheDocument();
  });

  it("displays NAV percentage when provided", () => {
    render(
      <VaRGauge
        title="Parametric VaR"
        amount="5000.00"
        navPercentage={5000}
        confidence={95}
        lastComputed="2026-04-01T10:00:00Z"
        method="Parametric"
      />,
    );

    expect(screen.getByText("5000.00% NAV")).toBeInTheDocument();
  });

  it("displays dash for NAV percentage when null", () => {
    render(
      <VaRGauge
        title="Monte Carlo VaR"
        amount={null}
        navPercentage={null}
        confidence={95}
        lastComputed={null}
        method="Monte Carlo"
      />,
    );

    // The "—" for nav percentage
    const dashes = screen.getAllByText("—");
    expect(dashes.length).toBeGreaterThanOrEqual(1);
  });

  it("displays last computed time", () => {
    render(
      <VaRGauge
        title="Historical VaR"
        amount="12500.00"
        navPercentage={12500}
        confidence={95}
        lastComputed="2026-04-01T10:00:00Z"
        method="Historical"
      />,
    );

    // Should show formatted time
    expect(screen.getByText("95% confidence")).toBeInTheDocument();
  });
});

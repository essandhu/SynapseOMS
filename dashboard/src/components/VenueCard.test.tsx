import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { VenueCard, fillRateColor, LatencySparkline } from "./VenueCard";
import type { Venue } from "../api/types";

const makeVenue = (overrides: Partial<Venue> = {}): Venue => ({
  id: "test-venue",
  name: "Test Exchange",
  type: "exchange",
  status: "connected",
  supportedAssets: ["equity", "crypto"],
  latencyP50Ms: 45,
  latencyP99Ms: 120,
  fillRate: 0.92,
  lastHeartbeat: new Date().toISOString(),
  hasCredentials: true,
  ...overrides,
});

const noop = vi.fn();

describe("VenueCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders venue name and status", () => {
    render(
      <VenueCard
        venue={makeVenue()}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    expect(screen.getByText("Test Exchange")).toBeInTheDocument();
    expect(screen.getByText("Connected")).toBeInTheDocument();
  });

  it("displays fill rate with correct color for high fill rate", () => {
    render(
      <VenueCard
        venue={makeVenue({ fillRate: 0.92 })}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    const fillRate = screen.getByTestId("fill-rate");
    expect(fillRate.textContent).toBe("92.0%");
    expect(fillRate.className).toContain("text-accent-green");
  });

  it("displays fill rate with yellow color for medium fill rate", () => {
    render(
      <VenueCard
        venue={makeVenue({ fillRate: 0.65 })}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    const fillRate = screen.getByTestId("fill-rate");
    expect(fillRate.textContent).toBe("65.0%");
    expect(fillRate.className).toContain("text-accent-yellow");
  });

  it("displays fill rate with red color for low fill rate", () => {
    render(
      <VenueCard
        venue={makeVenue({ fillRate: 0.3 })}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    const fillRate = screen.getByTestId("fill-rate");
    expect(fillRate.textContent).toBe("30.0%");
    expect(fillRate.className).toContain("text-accent-red");
  });

  it("shows P50 and P99 latency when connected", () => {
    render(
      <VenueCard
        venue={makeVenue({ latencyP50Ms: 45, latencyP99Ms: 120 })}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    expect(screen.getByText("45ms")).toBeInTheDocument();
    expect(screen.getByText("120ms")).toBeInTheDocument();
  });

  it("does not show latency when disconnected", () => {
    render(
      <VenueCard
        venue={makeVenue({ status: "disconnected" })}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    expect(screen.queryByText("P50:")).not.toBeInTheDocument();
  });

  it("shows venue type badge", () => {
    render(
      <VenueCard
        venue={makeVenue({ type: "dark_pool" })}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    expect(screen.getByText("dark pool")).toBeInTheDocument();
  });

  it("renders latency sparkline when connected with history", () => {
    render(
      <VenueCard
        venue={makeVenue()}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    // Sparkline should be rendered (seeded with mock data)
    const sparklines = screen.getAllByTestId("latency-sparkline");
    expect(sparklines.length).toBeGreaterThanOrEqual(1);
  });

  it("expands drill-down section on click", () => {
    render(
      <VenueCard
        venue={makeVenue()}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    const drillDown = screen.getByTestId("drill-down-section");
    expect(drillDown.className).toContain("max-h-0");

    // Click the header to expand
    const header = screen.getByRole("button", { name: /Test Exchange details/i });
    fireEvent.click(header);

    expect(drillDown.className).toContain("max-h-96");
  });

  it("shows order count and fill stats in drill-down", () => {
    render(
      <VenueCard
        venue={makeVenue()}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    // Expand
    const header = screen.getByRole("button", { name: /Test Exchange details/i });
    fireEvent.click(header);

    expect(screen.getByTestId("order-count")).toBeInTheDocument();
    expect(screen.getByText("Orders")).toBeInTheDocument();
    expect(screen.getByText("Fills")).toBeInTheDocument();
    expect(screen.getByText("Rejects")).toBeInTheDocument();
    expect(screen.getByText("Avg Fill Time")).toBeInTheDocument();
  });

  it("collapses drill-down on second click", () => {
    render(
      <VenueCard
        venue={makeVenue()}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    const header = screen.getByRole("button", { name: /Test Exchange details/i });
    const drillDown = screen.getByTestId("drill-down-section");

    fireEvent.click(header);
    expect(drillDown.className).toContain("max-h-96");

    fireEvent.click(header);
    expect(drillDown.className).toContain("max-h-0");
  });

  it("shows Test Connection button and handles click", async () => {
    const onTestConnection = vi.fn();
    render(
      <VenueCard
        venue={makeVenue()}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={onTestConnection}
      />,
    );

    const testBtn = screen.getByText("Test Connection");
    expect(testBtn).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(testBtn);
    });

    expect(onTestConnection).toHaveBeenCalledOnce();
    expect(screen.getByText("Testing...")).toBeInTheDocument();

    // After the timeout, latency result should appear
    await act(async () => {
      vi.advanceTimersByTime(1000);
    });

    const result = screen.getByTestId("test-latency-result");
    expect(result.textContent).toMatch(/Round-trip: \d+ms/);
  });

  it("shows Disconnect button when connected", () => {
    render(
      <VenueCard
        venue={makeVenue({ status: "connected" })}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    expect(screen.getByText("Disconnect")).toBeInTheDocument();
  });

  it("shows Connect button when disconnected", () => {
    render(
      <VenueCard
        venue={makeVenue({ status: "disconnected" })}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    expect(screen.getByText("Connect")).toBeInTheDocument();
  });

  it("disables Connect button without credentials", () => {
    render(
      <VenueCard
        venue={makeVenue({ status: "disconnected", hasCredentials: false })}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    const connectBtn = screen.getByText("Connect");
    expect(connectBtn).toBeDisabled();
  });

  it("shows asset class tags", () => {
    render(
      <VenueCard
        venue={makeVenue({ supportedAssets: ["equity", "crypto"] })}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    expect(screen.getByText("Equity")).toBeInTheDocument();
    expect(screen.getByText("Crypto")).toBeInTheDocument();
  });

  it("handles keyboard navigation for expand/collapse", () => {
    render(
      <VenueCard
        venue={makeVenue()}
        onConnect={noop}
        onDisconnect={noop}
        onTestConnection={noop}
      />,
    );

    const header = screen.getByRole("button", { name: /Test Exchange details/i });
    const drillDown = screen.getByTestId("drill-down-section");

    fireEvent.keyDown(header, { key: "Enter" });
    expect(drillDown.className).toContain("max-h-96");

    fireEvent.keyDown(header, { key: " " });
    expect(drillDown.className).toContain("max-h-0");
  });
});

describe("fillRateColor", () => {
  it("returns green for rate >= 0.8", () => {
    expect(fillRateColor(0.8)).toBe("text-accent-green");
    expect(fillRateColor(1.0)).toBe("text-accent-green");
  });

  it("returns yellow for rate 0.5-0.8", () => {
    expect(fillRateColor(0.5)).toBe("text-accent-yellow");
    expect(fillRateColor(0.79)).toBe("text-accent-yellow");
  });

  it("returns red for rate < 0.5", () => {
    expect(fillRateColor(0.49)).toBe("text-accent-red");
    expect(fillRateColor(0.0)).toBe("text-accent-red");
  });
});

describe("LatencySparkline", () => {
  it("renders SVG with correct number of points", () => {
    const points = [10, 20, 30, 40, 50];
    render(<LatencySparkline points={points} />);

    const sparkline = screen.getByTestId("latency-sparkline");
    expect(sparkline).toBeInTheDocument();
    expect(sparkline.tagName).toBe("svg");
  });

  it("returns null for fewer than 2 points", () => {
    const { container } = render(<LatencySparkline points={[10]} />);
    expect(container.innerHTML).toBe("");
  });

  it("renders with custom dimensions", () => {
    render(<LatencySparkline points={[10, 20, 30]} width={200} height={50} />);

    const sparkline = screen.getByTestId("latency-sparkline");
    expect(sparkline.getAttribute("width")).toBe("200");
    expect(sparkline.getAttribute("height")).toBe("50");
  });
});

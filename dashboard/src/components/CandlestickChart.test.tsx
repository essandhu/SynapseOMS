import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { CandlestickChart } from "./CandlestickChart";
import { useMarketDataStore } from "../stores/marketDataStore";
import type { OHLCUpdate } from "../api/types";

// Polyfill ResizeObserver for jsdom
globalThis.ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
} as unknown as typeof ResizeObserver;

// Mock lightweight-charts — it requires a real DOM canvas
vi.mock("lightweight-charts", () => ({
  createChart: vi.fn(() => ({
    addSeries: vi.fn(() => ({
      setData: vi.fn(),
      update: vi.fn(),
    })),
    applyOptions: vi.fn(),
    timeScale: vi.fn(() => ({
      fitContent: vi.fn(),
      scrollToRealTime: vi.fn(),
    })),
    remove: vi.fn(),
    resize: vi.fn(),
  })),
  CandlestickSeries: {},
  ColorType: { Solid: "Solid" },
}));

// Mock the store's subscribe to avoid WebSocket connections
vi.mock("../api/ws", () => ({
  createMarketDataStream: vi.fn(() => ({ close: vi.fn() })),
}));

const makeBar = (close: string, periodStart: string): OHLCUpdate => ({
  instrumentId: "AAPL",
  venueId: "test-venue",
  interval: "1m",
  open: "150.00",
  high: "152.00",
  low: "148.00",
  close,
  volume: "1000",
  periodStart,
  periodEnd: "2026-04-02T10:01:00Z",
  complete: true,
});

describe("CandlestickChart", () => {
  beforeEach(() => {
    useMarketDataStore.setState({ bars: {} });
  });

  it("renders without crash", () => {
    render(<CandlestickChart instrumentId="AAPL" />);
    // The chart container should exist
    expect(screen.getByTestId("candlestick-chart")).toBeInTheDocument();
  });

  it("shows empty state message when no data", () => {
    render(<CandlestickChart instrumentId="AAPL" />);
    expect(screen.getByText(/waiting for market data/i)).toBeInTheDocument();
  });

  it("hides empty state when bars exist", () => {
    useMarketDataStore.setState({
      bars: {
        "AAPL:1m": [makeBar("151.00", "2026-04-02T10:00:00Z")],
      },
    });

    render(<CandlestickChart instrumentId="AAPL" />);
    expect(screen.queryByText(/waiting for market data/i)).not.toBeInTheDocument();
  });
});

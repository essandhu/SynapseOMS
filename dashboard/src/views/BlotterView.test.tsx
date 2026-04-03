import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BlotterView } from "./BlotterView";
import type { Order, Instrument } from "../api/types";

// Mock the order store
const mockSubscribe = vi.fn(() => vi.fn());
const mockSubmitOrder = vi.fn();
const mockCancelOrder = vi.fn().mockResolvedValue(undefined);

let mockOrders = new Map<string, Order>();
let mockLoading = false;
let mockError: string | null = null;

vi.mock("../stores/orderStore", () => ({
  useOrderStore: (selector: (s: Record<string, unknown>) => unknown) => {
    const state: Record<string, unknown> = {
      orders: mockOrders,
      loading: mockLoading,
      error: mockError,
      subscribe: mockSubscribe,
      submitOrder: mockSubmitOrder,
      cancelOrder: mockCancelOrder,
    };
    return selector(state);
  },
}));

// Mock the REST API for instruments
vi.mock("../api/rest", () => ({
  fetchInstruments: vi.fn().mockResolvedValue([
    {
      id: "btc-usd",
      symbol: "BTC/USD",
      name: "Bitcoin",
      assetClass: "crypto",
      baseCurrency: "BTC",
      quoteCurrency: "USD",
      venueId: "sim-exchange",
    },
  ] as Instrument[]),
}));

// Mock child components
vi.mock("../components/OrderTable", () => ({
  OrderTable: ({ orders }: { orders: Order[] }) => (
    <div data-testid="order-table">
      {orders.length === 0 ? "No orders" : `${orders.length} orders`}
    </div>
  ),
}));

vi.mock("../components/OrderTicket", () => ({
  OrderTicket: () => <div data-testid="order-ticket">Order Ticket</div>,
}));

vi.mock("../components/CandlestickChart", () => ({
  CandlestickChart: ({ instrumentId, interval }: { instrumentId: string; interval?: string }) => (
    <div data-testid="candlestick-chart">Chart: {instrumentId} ({interval || "1m"})</div>
  ),
}));

vi.mock("../stores/marketDataStore", () => ({
  useMarketDataStore: (selector: (s: Record<string, unknown>) => unknown) => {
    const state: Record<string, unknown> = {
      subscribe: () => () => {},
    };
    return selector(state);
  },
}));

describe("BlotterView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockOrders = new Map();
    mockLoading = false;
    mockError = null;
  });

  it("renders order table", () => {
    render(<BlotterView />);

    expect(screen.getByTestId("order-table")).toBeInTheDocument();
  });

  it("renders order ticket sidebar", () => {
    render(<BlotterView />);

    expect(screen.getByTestId("order-ticket")).toBeInTheDocument();
  });

  it("renders without crashing with empty orders", () => {
    render(<BlotterView />);

    expect(screen.getByTestId("order-table")).toHaveTextContent("No orders");
  });

  it("renders filter tabs", () => {
    render(<BlotterView />);

    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.getByText("All")).toBeInTheDocument();
    expect(screen.getByText("Filled")).toBeInTheDocument();
    expect(screen.getByText("Canceled")).toBeInTheDocument();
  });

  it("subscribes to order store on mount", () => {
    render(<BlotterView />);

    expect(mockSubscribe).toHaveBeenCalledOnce();
  });

  it("displays error banner when error exists", () => {
    mockError = "Failed to submit order";

    render(<BlotterView />);

    expect(screen.getByText("Failed to submit order")).toBeInTheDocument();
  });

  it("shows chart panel when Chart button is clicked", async () => {
    const user = userEvent.setup();
    render(<BlotterView />);

    // Chart panel should not be visible initially
    expect(screen.queryByTestId("chart-panel")).not.toBeInTheDocument();

    // Click the Chart toggle button
    const toggle = screen.getByTestId("chart-toggle");
    await user.click(toggle);

    // Chart panel should now be visible
    expect(screen.getByTestId("chart-panel")).toBeInTheDocument();
    expect(screen.getByTestId("candlestick-chart")).toBeInTheDocument();
  });

  it("toggles chart interval between 1m and 5m", async () => {
    const user = userEvent.setup();
    render(<BlotterView />);

    // Open the chart panel
    await user.click(screen.getByTestId("chart-toggle"));

    // Default interval should be 1m
    expect(screen.getByTestId("candlestick-chart")).toHaveTextContent("(1m)");
    expect(screen.getByTestId("interval-1m")).toBeInTheDocument();
    expect(screen.getByTestId("interval-5m")).toBeInTheDocument();

    // Click 5m interval button
    await user.click(screen.getByTestId("interval-5m"));

    // Chart should now show 5m interval
    expect(screen.getByTestId("candlestick-chart")).toHaveTextContent("(5m)");

    // Click 1m to go back
    await user.click(screen.getByTestId("interval-1m"));
    expect(screen.getByTestId("candlestick-chart")).toHaveTextContent("(1m)");
  });
});

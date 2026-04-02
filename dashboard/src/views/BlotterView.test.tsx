import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
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
});

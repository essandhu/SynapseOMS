import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { LiquidityNetwork } from "./LiquidityNetwork";
import type { Venue } from "../api/types";

// Mock the venue store
const mockSubscribe = vi.fn(() => vi.fn());
const mockConnectVenue = vi.fn().mockResolvedValue(undefined);
const mockDisconnectVenue = vi.fn().mockResolvedValue(undefined);

let mockVenues = new Map<string, Venue>();
let mockLoading = false;
let mockError: string | null = null;

vi.mock("../stores/venueStore", () => ({
  useVenueStore: (selector: (s: Record<string, unknown>) => unknown) => {
    const state: Record<string, unknown> = {
      venues: mockVenues,
      loading: mockLoading,
      error: mockError,
      subscribe: mockSubscribe,
      connectVenue: mockConnectVenue,
      disconnectVenue: mockDisconnectVenue,
      storeCredentials: vi.fn(),
      loadVenues: vi.fn(),
    };
    return selector(state);
  },
}));

// Mock VenueCard component
vi.mock("../components/VenueCard", () => ({
  VenueCard: ({ venue }: { venue: Venue }) => (
    <div data-testid="venue-card">{venue.name}</div>
  ),
}));

const sampleVenues: Venue[] = [
  {
    id: "alpaca",
    name: "Alpaca",
    type: "exchange",
    status: "connected",
    supportedAssets: ["equity"],
    latencyP50Ms: 45,
    latencyP99Ms: 120,
    fillRate: 0.98,
    lastHeartbeat: "2026-04-01T10:00:00Z",
    hasCredentials: true,
  },
  {
    id: "binance",
    name: "Binance Testnet",
    type: "exchange",
    status: "disconnected",
    supportedAssets: ["crypto"],
    latencyP50Ms: 30,
    latencyP99Ms: 85,
    fillRate: 0.99,
    lastHeartbeat: "2026-04-01T09:55:00Z",
    hasCredentials: true,
  },
];

describe("LiquidityNetwork", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockVenues = new Map();
    mockLoading = false;
    mockError = null;
  });

  it("renders venue cards when venues exist", () => {
    mockVenues = new Map(sampleVenues.map((v) => [v.id, v]));

    render(<LiquidityNetwork />);

    const cards = screen.getAllByTestId("venue-card");
    expect(cards).toHaveLength(2);
    expect(screen.getByText("Alpaca")).toBeInTheDocument();
    expect(screen.getByText("Binance Testnet")).toBeInTheDocument();
  });

  it("shows Connect New Venue button", () => {
    render(<LiquidityNetwork />);

    expect(screen.getByText("Connect New Venue")).toBeInTheDocument();
  });

  it("handles empty venues list", () => {
    mockVenues = new Map();

    render(<LiquidityNetwork />);

    expect(screen.queryAllByTestId("venue-card")).toHaveLength(0);
    // The "Connect New Venue" button should still be visible
    expect(screen.getByText("Connect New Venue")).toBeInTheDocument();
  });

  it("renders header", () => {
    render(<LiquidityNetwork />);

    expect(screen.getByText("Liquidity Network")).toBeInTheDocument();
    expect(
      screen.getByText(/Manage venue connections/),
    ).toBeInTheDocument();
  });

  it("subscribes to venue store on mount", () => {
    render(<LiquidityNetwork />);

    expect(mockSubscribe).toHaveBeenCalledOnce();
  });

  it("displays error banner when error exists", () => {
    mockError = "Failed to load venues";

    render(<LiquidityNetwork />);

    expect(screen.getByText("Failed to load venues")).toBeInTheDocument();
  });
});

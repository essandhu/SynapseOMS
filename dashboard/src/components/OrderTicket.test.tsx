import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { OrderTicket } from "./OrderTicket";
import type { Instrument, Venue } from "../api/types";

const mockInstruments: Instrument[] = [
  {
    id: "btc-usd",
    symbol: "BTC/USD",
    name: "Bitcoin",
    assetClass: "crypto",
    baseCurrency: "BTC",
    quoteCurrency: "USD",
    venueId: "sim-exchange",
  },
  {
    id: "eth-usd",
    symbol: "ETH/USD",
    name: "Ethereum",
    assetClass: "crypto",
    baseCurrency: "ETH",
    quoteCurrency: "USD",
    venueId: "sim-exchange",
  },
];

const mockVenues: Venue[] = [
  {
    id: "sim-exchange",
    name: "Simulated Exchange",
    type: "simulated",
    status: "connected",
    supportedAssets: ["crypto"],
    latencyP50Ms: 5,
    latencyP99Ms: 20,
    fillRate: 0.98,
    lastHeartbeat: "2026-04-02T00:00:00Z",
    hasCredentials: true,
  },
  {
    id: "dark-pool-1",
    name: "Dark Pool Alpha",
    type: "dark_pool",
    status: "connected",
    supportedAssets: ["crypto", "equity"],
    latencyP50Ms: 10,
    latencyP99Ms: 50,
    fillRate: 0.85,
    lastHeartbeat: "2026-04-02T00:00:00Z",
    hasCredentials: true,
  },
];

describe("OrderTicket", () => {
  it("submits market order with correct parameters", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<OrderTicket onSubmit={onSubmit} instruments={mockInstruments} />);

    // Select instrument
    fireEvent.change(screen.getByLabelText("Instrument"), {
      target: { value: "btc-usd" },
    });

    // Buy is default, but click it explicitly
    fireEvent.click(screen.getByText("Buy"));

    // Set quantity
    fireEvent.change(screen.getByLabelText("Quantity"), {
      target: { value: "10" },
    });

    // Submit
    fireEvent.click(screen.getByText("Submit Order"));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          side: "buy",
          quantity: "10",
          type: "market",
          instrumentId: "btc-usd",
          venueId: "smart",
        }),
      );
    });
  });

  it("hides price field for market orders", () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<OrderTicket onSubmit={onSubmit} instruments={mockInstruments} />);

    // Market is default
    expect(screen.queryByLabelText("Price")).not.toBeInTheDocument();
  });

  it("shows price field for limit orders", () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<OrderTicket onSubmit={onSubmit} instruments={mockInstruments} />);

    fireEvent.click(screen.getByText("Limit"));

    expect(screen.getByLabelText("Price")).toBeInTheDocument();
  });

  it("validates quantity is required", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<OrderTicket onSubmit={onSubmit} instruments={mockInstruments} />);

    // Select instrument but leave quantity empty
    fireEvent.change(screen.getByLabelText("Instrument"), {
      target: { value: "btc-usd" },
    });

    fireEvent.click(screen.getByText("Submit Order"));

    await waitFor(() => {
      expect(screen.getByText("Quantity is required")).toBeInTheDocument();
    });

    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("validates instrument is required", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<OrderTicket onSubmit={onSubmit} instruments={mockInstruments} />);

    fireEvent.change(screen.getByLabelText("Quantity"), {
      target: { value: "10" },
    });

    fireEvent.click(screen.getByText("Submit Order"));

    await waitFor(() => {
      expect(screen.getByText("Instrument is required")).toBeInTheDocument();
    });

    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("Buy/Sell toggle changes side", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<OrderTicket onSubmit={onSubmit} instruments={mockInstruments} />);

    // Select instrument and quantity
    fireEvent.change(screen.getByLabelText("Instrument"), {
      target: { value: "btc-usd" },
    });
    fireEvent.change(screen.getByLabelText("Quantity"), {
      target: { value: "5" },
    });

    // Click sell
    fireEvent.click(screen.getByText("Sell"));
    fireEvent.click(screen.getByText("Submit Order"));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ side: "sell" }),
      );
    });
  });

  it("clears form after successful submission", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<OrderTicket onSubmit={onSubmit} instruments={mockInstruments} />);

    fireEvent.change(screen.getByLabelText("Instrument"), {
      target: { value: "btc-usd" },
    });
    fireEvent.change(screen.getByLabelText("Quantity"), {
      target: { value: "10" },
    });

    fireEvent.click(screen.getByText("Submit Order"));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalled();
    });

    // Quantity should be cleared
    expect(screen.getByLabelText("Quantity")).toHaveValue(null);
  });

  it("disables submit button while submitting", async () => {
    let resolveSubmit: () => void;
    const onSubmit = vi.fn(
      () => new Promise<void>((resolve) => { resolveSubmit = resolve; }),
    );
    render(<OrderTicket onSubmit={onSubmit} instruments={mockInstruments} />);

    fireEvent.change(screen.getByLabelText("Instrument"), {
      target: { value: "btc-usd" },
    });
    fireEvent.change(screen.getByLabelText("Quantity"), {
      target: { value: "10" },
    });

    fireEvent.click(screen.getByText("Submit Order"));

    await waitFor(() => {
      expect(screen.getByText("Submitting...")).toBeDisabled();
    });

    resolveSubmit!();

    await waitFor(() => {
      expect(screen.getByText("Submit Order")).not.toBeDisabled();
    });
  });

  it("submits limit order with price", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<OrderTicket onSubmit={onSubmit} instruments={mockInstruments} />);

    fireEvent.change(screen.getByLabelText("Instrument"), {
      target: { value: "eth-usd" },
    });
    fireEvent.click(screen.getByText("Limit"));
    fireEvent.change(screen.getByLabelText("Quantity"), {
      target: { value: "20" },
    });
    fireEvent.change(screen.getByLabelText("Price"), {
      target: { value: "3500" },
    });

    fireEvent.click(screen.getByText("Submit Order"));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          type: "limit",
          quantity: "20",
          price: "3500",
          instrumentId: "eth-usd",
        }),
      );
    });
  });

  // --- Smart Route tests ---

  it("shows Smart Route option in venue selector as default", () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <OrderTicket
        onSubmit={onSubmit}
        instruments={mockInstruments}
        venues={mockVenues}
      />,
    );

    const venueSelect = screen.getByLabelText("Venue") as HTMLSelectElement;
    expect(venueSelect.value).toBe("smart");

    // Smart Route should be the first option
    const options = venueSelect.querySelectorAll("option");
    expect(options[0].value).toBe("smart");
    expect(options[0].textContent).toContain("Smart Route");
  });

  it("submits with venueId 'smart' when Smart Route is selected", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <OrderTicket
        onSubmit={onSubmit}
        instruments={mockInstruments}
        venues={mockVenues}
      />,
    );

    fireEvent.change(screen.getByLabelText("Instrument"), {
      target: { value: "btc-usd" },
    });
    fireEvent.change(screen.getByLabelText("Quantity"), {
      target: { value: "5" },
    });

    // Smart Route is default — submit without changing venue
    fireEvent.click(screen.getByText("Submit Order"));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          venueId: "smart",
          instrumentId: "btc-usd",
          quantity: "5",
        }),
      );
    });
  });

  it("submits with specific venueId when a venue is selected", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <OrderTicket
        onSubmit={onSubmit}
        instruments={mockInstruments}
        venues={mockVenues}
      />,
    );

    fireEvent.change(screen.getByLabelText("Instrument"), {
      target: { value: "btc-usd" },
    });
    fireEvent.change(screen.getByLabelText("Quantity"), {
      target: { value: "5" },
    });

    // Select a specific venue
    fireEvent.change(screen.getByLabelText("Venue"), {
      target: { value: "dark-pool-1" },
    });

    fireEvent.click(screen.getByText("Submit Order"));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          venueId: "dark-pool-1",
        }),
      );
    });
  });

  it("shows Smart Route info tooltip on hover", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <OrderTicket
        onSubmit={onSubmit}
        instruments={mockInstruments}
        venues={mockVenues}
      />,
    );

    // Info button should be visible when Smart Route is selected (default)
    const infoButton = screen.getByLabelText("Smart Route info");
    expect(infoButton).toBeInTheDocument();

    // Tooltip should not be visible initially
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();

    // Hover to show tooltip
    fireEvent.mouseEnter(infoButton);
    expect(screen.getByRole("tooltip")).toHaveTextContent(
      "Order will be automatically routed to the best venue(s) based on price, depth, and execution quality",
    );

    // Mouse leave hides tooltip
    fireEvent.mouseLeave(infoButton);
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });

  it("hides Smart Route info when a specific venue is selected", () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <OrderTicket
        onSubmit={onSubmit}
        instruments={mockInstruments}
        venues={mockVenues}
      />,
    );

    // Info button visible with Smart Route
    expect(screen.getByLabelText("Smart Route info")).toBeInTheDocument();

    // Switch to specific venue
    fireEvent.change(screen.getByLabelText("Venue"), {
      target: { value: "sim-exchange" },
    });

    // Info button should be gone
    expect(screen.queryByLabelText("Smart Route info")).not.toBeInTheDocument();
  });

  it("lists all venues in the dropdown", () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <OrderTicket
        onSubmit={onSubmit}
        instruments={mockInstruments}
        venues={mockVenues}
      />,
    );

    const venueSelect = screen.getByLabelText("Venue") as HTMLSelectElement;
    const options = venueSelect.querySelectorAll("option");

    // Smart Route + 2 venues = 3 options
    expect(options).toHaveLength(3);
    expect(options[1].textContent).toBe("Simulated Exchange");
    expect(options[2].textContent).toBe("Dark Pool Alpha");
  });

  it("renders venue selector even without venues prop", () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<OrderTicket onSubmit={onSubmit} instruments={mockInstruments} />);

    const venueSelect = screen.getByLabelText("Venue") as HTMLSelectElement;
    expect(venueSelect.value).toBe("smart");

    // Only Smart Route option when no venues provided
    const options = venueSelect.querySelectorAll("option");
    expect(options).toHaveLength(1);
  });
});

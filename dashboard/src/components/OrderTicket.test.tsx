import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { OrderTicket } from "./OrderTicket";
import type { Instrument } from "../api/types";

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
});

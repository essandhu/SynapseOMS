import { useState, useCallback, useMemo } from "react";
import type { Instrument, OrderSide, OrderType, SubmitOrderRequest } from "../api/types";

export interface OrderTicketProps {
  instruments: Instrument[];
  onSubmit: (request: SubmitOrderRequest) => Promise<void>;
}

export function OrderTicket({ instruments, onSubmit }: OrderTicketProps) {
  const [instrumentId, setInstrumentId] = useState("");
  const [side, setSide] = useState<OrderSide>("buy");
  const [orderType, setOrderType] = useState<OrderType>("market");
  const [quantity, setQuantity] = useState("");
  const [price, setPrice] = useState("");
  const [search, setSearch] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const filteredInstruments = useMemo(() => {
    if (!search) return instruments;
    const term = search.toLowerCase();
    return instruments.filter(
      (i) =>
        i.symbol.toLowerCase().includes(term) ||
        i.name.toLowerCase().includes(term),
    );
  }, [instruments, search]);

  const selectedInstrument = useMemo(
    () => instruments.find((i) => i.id === instrumentId),
    [instruments, instrumentId],
  );

  const validate = useCallback((): string | null => {
    if (!instrumentId) return "Instrument is required";
    if (!quantity || Number(quantity) <= 0) return "Quantity is required";
    if (orderType === "limit" && (!price || Number(price) <= 0))
      return "Price is required for limit orders";
    return null;
  }, [instrumentId, quantity, price, orderType]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      const validationError = validate();
      if (validationError) {
        setError(validationError);
        return;
      }

      setSubmitting(true);
      setError(null);

      const request: SubmitOrderRequest = {
        instrumentId,
        side,
        type: orderType,
        quantity,
        venueId: selectedInstrument?.venueId ?? "",
        ...(orderType === "limit" ? { price } : {}),
      };

      try {
        await onSubmit(request);
        // Reset form on success
        setQuantity("");
        setPrice("");
        setError(null);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Failed to submit order",
        );
      } finally {
        setSubmitting(false);
      }
    },
    [instrumentId, side, orderType, quantity, price, selectedInstrument, onSubmit, validate],
  );

  return (
    <form
      onSubmit={handleSubmit}
      className="flex flex-col gap-3 rounded border border-border bg-bg-secondary p-4"
      style={{ maxWidth: 360 }}
    >
      <h2 className="font-mono text-xs font-semibold uppercase tracking-wider text-text-muted">
        Order Ticket
      </h2>

      {/* Instrument picker */}
      <div className="flex flex-col gap-1">
        <label
          htmlFor="instrument-select"
          className="font-mono text-xs text-text-muted"
        >
          Instrument
        </label>
        <input
          id="instrument-search"
          aria-label="Search instruments"
          type="text"
          placeholder="Search instruments..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="rounded border border-border bg-bg-primary px-2 py-1 font-mono text-xs text-text-primary placeholder:text-text-muted focus:border-accent-blue focus:outline-none"
        />
        <select
          id="instrument-select"
          value={instrumentId}
          onChange={(e) => setInstrumentId(e.target.value)}
          className="rounded border border-border bg-bg-primary px-2 py-1 font-mono text-xs text-text-primary focus:border-accent-blue focus:outline-none"
        >
          <option value="">Select instrument</option>
          {filteredInstruments.map((inst) => (
            <option key={inst.id} value={inst.id}>
              {inst.symbol} - {inst.name}
            </option>
          ))}
        </select>
      </div>

      {/* Side toggle */}
      <div className="flex flex-col gap-1">
        <span className="font-mono text-xs text-text-muted">Side</span>
        <div className="flex gap-1">
          <button
            type="button"
            onClick={() => setSide("buy")}
            className="flex-1 rounded px-3 py-1.5 font-mono text-xs font-bold transition-colors"
            style={{
              backgroundColor: side === "buy" ? "#22c55e" : undefined,
              color: side === "buy" ? "#0a0e17" : "#22c55e",
              border: `1px solid #22c55e`,
            }}
          >
            Buy
          </button>
          <button
            type="button"
            onClick={() => setSide("sell")}
            className="flex-1 rounded px-3 py-1.5 font-mono text-xs font-bold transition-colors"
            style={{
              backgroundColor: side === "sell" ? "#ef4444" : undefined,
              color: side === "sell" ? "#0a0e17" : "#ef4444",
              border: `1px solid #ef4444`,
            }}
          >
            Sell
          </button>
        </div>
      </div>

      {/* Order type selector */}
      <div className="flex flex-col gap-1">
        <span className="font-mono text-xs text-text-muted">Order Type</span>
        <div className="flex gap-1">
          {(["market", "limit"] as const).map((t) => (
            <button
              key={t}
              type="button"
              onClick={() => setOrderType(t)}
              className="flex-1 rounded px-3 py-1.5 font-mono text-xs font-medium transition-colors"
              style={{
                backgroundColor:
                  orderType === t ? "#3b82f6" : "transparent",
                color: orderType === t ? "#0a0e17" : "#9ca3af",
                border: `1px solid ${orderType === t ? "#3b82f6" : "#374151"}`,
              }}
            >
              {t.charAt(0).toUpperCase() + t.slice(1)}
            </button>
          ))}
        </div>
      </div>

      {/* Quantity */}
      <div className="flex flex-col gap-1">
        <label
          htmlFor="order-quantity"
          className="font-mono text-xs text-text-muted"
        >
          Quantity
        </label>
        <input
          id="order-quantity"
          aria-label="Quantity"
          type="number"
          min="0"
          step="any"
          placeholder="0"
          value={quantity}
          onChange={(e) => setQuantity(e.target.value)}
          className="rounded border border-border bg-bg-primary px-2 py-1 font-mono text-xs text-text-primary placeholder:text-text-muted focus:border-accent-blue focus:outline-none"
        />
      </div>

      {/* Price (limit only) */}
      {orderType === "limit" && (
        <div className="flex flex-col gap-1">
          <label
            htmlFor="order-price"
            className="font-mono text-xs text-text-muted"
          >
            Price
          </label>
          <input
            id="order-price"
            aria-label="Price"
            type="number"
            min="0"
            step="any"
            placeholder="0.00"
            value={price}
            onChange={(e) => setPrice(e.target.value)}
            className="rounded border border-border bg-bg-primary px-2 py-1 font-mono text-xs text-text-primary placeholder:text-text-muted focus:border-accent-blue focus:outline-none"
          />
        </div>
      )}

      {/* Error display */}
      {error && (
        <div className="rounded border border-accent-red/30 bg-accent-red/10 px-2 py-1 font-mono text-xs text-accent-red">
          {error}
        </div>
      )}

      {/* Submit */}
      <button
        type="submit"
        disabled={submitting}
        className="rounded px-3 py-2 font-mono text-xs font-bold uppercase tracking-wider transition-colors disabled:opacity-50"
        style={{
          backgroundColor: side === "buy" ? "#22c55e" : "#ef4444",
          color: "#0a0e17",
        }}
      >
        {submitting ? "Submitting..." : "Submit Order"}
      </button>
    </form>
  );
}

import { useState, useCallback, useMemo } from "react";
import type { AssetClass, Instrument, OrderSide, OrderType, Position, SubmitOrderRequest, Venue } from "../api/types";
import { useThemeColors } from "../theme/terminal";

const SMART_ROUTE_ID = "smart";

export interface OrderTicketProps {
  instruments: Instrument[];
  venues?: Venue[];
  positions?: Position[];
  onSubmit: (request: SubmitOrderRequest) => Promise<void>;
}

/** Check whether a venue supports the given asset class. */
function venueSupportsAsset(venue: Venue, assetClass: AssetClass): boolean {
  return venue.supportedAssets.includes(assetClass);
}

/** Sum net position quantity across all venues for a given instrument. */
function netPositionForInstrument(positions: Position[], instrumentId: string): number {
  return positions
    .filter((p) => p.instrumentId === instrumentId)
    .reduce((sum, p) => sum + Number(p.quantity), 0);
}

export function OrderTicket({ instruments, venues = [], positions = [], onSubmit }: OrderTicketProps) {
  const theme = useThemeColors();
  const [instrumentId, setInstrumentId] = useState("");
  const [side, setSide] = useState<OrderSide>("buy");
  const [orderType, setOrderType] = useState<OrderType>("market");
  const [quantity, setQuantity] = useState("");
  const [price, setPrice] = useState("");
  const [venueId, setVenueId] = useState(SMART_ROUTE_ID);
  const [search, setSearch] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showSmartRouteTooltip, setShowSmartRouteTooltip] = useState(false);

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

  // Only show connected venues; additionally filter by asset class when an instrument is selected.
  const availableVenues = useMemo(() => {
    return venues.filter((v) => {
      if (v.status !== "connected") return false;
      if (selectedInstrument && !venueSupportsAsset(v, selectedInstrument.assetClass)) return false;
      return true;
    });
  }, [venues, selectedInstrument]);

  // Reset venue to smart route when the available venue list changes and the
  // currently selected venue is no longer valid.
  const effectiveVenueId = useMemo(() => {
    if (venueId === SMART_ROUTE_ID) return SMART_ROUTE_ID;
    if (availableVenues.some((v) => v.id === venueId)) return venueId;
    return SMART_ROUTE_ID;
  }, [venueId, availableVenues]);

  const hasConnectedVenues = availableVenues.length > 0;

  const validate = useCallback((): string | null => {
    if (!instrumentId) return "Instrument is required";
    if (!quantity || Number(quantity) <= 0) return "Quantity is required";
    if (orderType === "limit" && (!price || Number(price) <= 0))
      return "Price is required for limit orders";

    // Sell-side validation: ensure the user holds enough shares
    if (side === "sell") {
      const netQty = netPositionForInstrument(positions, instrumentId);
      if (netQty <= 0) {
        return "No position held — cannot sell an instrument you do not own";
      }
      if (Number(quantity) > netQty) {
        return `Sell quantity exceeds held position (${netQty} available)`;
      }
    }

    // Venue connectivity check (for explicit venue selection)
    if (effectiveVenueId !== SMART_ROUTE_ID) {
      const venue = venues.find((v) => v.id === effectiveVenueId);
      if (venue && venue.status !== "connected") {
        return `Venue "${venue.name}" is not connected`;
      }
    }

    // At least one compatible venue must be connected for smart routing
    if (effectiveVenueId === SMART_ROUTE_ID && !hasConnectedVenues) {
      return "No connected venues available — connect a venue before trading";
    }

    return null;
  }, [instrumentId, quantity, price, orderType, side, positions, effectiveVenueId, venues, hasConnectedVenues]);

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
        venueId: effectiveVenueId === SMART_ROUTE_ID ? SMART_ROUTE_ID : effectiveVenueId,
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
    [instrumentId, side, orderType, quantity, price, effectiveVenueId, onSubmit, validate],
  );

  return (
    <form
      onSubmit={handleSubmit}
      className="flex flex-col gap-3 rounded border border-border bg-bg-secondary p-4"
      style={{ maxWidth: 360 }}
    >
      <h2 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
        Order Ticket
      </h2>

      {/* Instrument picker */}
      <div className="flex flex-col gap-1">
        <label
          htmlFor="instrument-select"
          className="text-xs text-text-muted"
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
          className="rounded border border-border bg-bg-primary px-2 py-1 text-xs text-text-primary placeholder:text-text-muted focus:border-accent-blue focus:outline-none"
        />
        <select
          id="instrument-select"
          value={instrumentId}
          onChange={(e) => setInstrumentId(e.target.value)}
          className="rounded border border-border bg-bg-primary px-2 py-1 text-xs text-text-primary focus:border-accent-blue focus:outline-none"
        >
          <option value="">Select instrument</option>
          {filteredInstruments.map((inst) => (
            <option key={inst.id} value={inst.id}>
              {inst.symbol} - {inst.name}
            </option>
          ))}
        </select>
      </div>

      {/* Venue selector — only connected, asset-compatible venues */}
      <div className="flex flex-col gap-1">
        <label
          htmlFor="venue-select"
          className="text-xs text-text-muted"
        >
          Venue
        </label>
        <div className="relative">
          <select
            id="venue-select"
            aria-label="Venue"
            value={effectiveVenueId}
            onChange={(e) => setVenueId(e.target.value)}
            className="w-full rounded border border-border bg-bg-primary px-2 py-1 text-xs text-text-primary focus:border-accent-blue focus:outline-none"
          >
            <option value={SMART_ROUTE_ID}>⚡ Smart Route</option>
            {availableVenues.map((v) => (
              <option key={v.id} value={v.id}>
                {v.name}
              </option>
            ))}
          </select>
          {!hasConnectedVenues && (
            <p className="mt-1 text-xs text-accent-yellow">
              No connected venues{selectedInstrument?.assetClass ? ` for ${selectedInstrument.assetClass}` : ""}
            </p>
          )}
          {effectiveVenueId === SMART_ROUTE_ID && hasConnectedVenues && (
            <div className="relative mt-1">
              <button
                type="button"
                aria-label="Smart Route info"
                className="text-xs text-accent-blue underline decoration-dotted"
                onMouseEnter={() => setShowSmartRouteTooltip(true)}
                onMouseLeave={() => setShowSmartRouteTooltip(false)}
                onFocus={() => setShowSmartRouteTooltip(true)}
                onBlur={() => setShowSmartRouteTooltip(false)}
              >
                ℹ What is Smart Route?
              </button>
              {showSmartRouteTooltip && (
                <div
                  role="tooltip"
                  className="absolute left-0 top-full z-10 mt-1 w-full rounded border border-border bg-bg-primary px-2 py-1.5 text-xs text-text-muted"
                  style={{ boxShadow: "rgba(0,0,0,0.03) 0px 4px 24px" }}
                >
                  Order will be automatically routed to the best venue(s) based
                  on price, depth, and execution quality
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Side toggle */}
      <div className="flex flex-col gap-1">
        <span className="text-xs text-text-muted">Side</span>
        <div className="flex gap-1">
          <button
            type="button"
            onClick={() => setSide("buy")}
            className="flex-1 rounded-xl px-3 py-1.5 text-xs font-bold transition-colors"
            style={{
              backgroundColor: side === "buy" ? theme.colors.accent.green : undefined,
              color: side === "buy" ? "#fff" : theme.colors.accent.green,
              border: `1px solid ${theme.colors.accent.green}`,
            }}
          >
            Buy
          </button>
          <button
            type="button"
            onClick={() => setSide("sell")}
            className="flex-1 rounded-xl px-3 py-1.5 text-xs font-bold transition-colors"
            style={{
              backgroundColor: side === "sell" ? theme.colors.accent.red : undefined,
              color: side === "sell" ? "#fff" : theme.colors.accent.red,
              border: `1px solid ${theme.colors.accent.red}`,
            }}
          >
            Sell
          </button>
        </div>
      </div>

      {/* Sell-side position info */}
      {side === "sell" && instrumentId && (
        <div className="rounded border border-border/50 bg-bg-primary/50 px-2 py-1 text-xs text-text-muted">
          Held: {netPositionForInstrument(positions, instrumentId)} shares
        </div>
      )}

      {/* Order type selector */}
      <div className="flex flex-col gap-1">
        <span className="text-xs text-text-muted">Order Type</span>
        <div className="flex gap-1">
          {(["market", "limit"] as const).map((t) => (
            <button
              key={t}
              type="button"
              onClick={() => setOrderType(t)}
              className="flex-1 rounded-xl px-3 py-1.5 text-xs font-medium transition-colors"
              style={{
                backgroundColor:
                  orderType === t ? theme.colors.accent.blue : "transparent",
                color: orderType === t ? "#fff" : theme.colors.text.muted,
                border: `1px solid ${orderType === t ? theme.colors.accent.blue : theme.colors.border}`,
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
          className="text-xs text-text-muted"
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
          className="rounded border border-border bg-bg-primary px-2 py-1 text-xs text-text-primary placeholder:text-text-muted focus:border-accent-blue focus:outline-none"
        />
      </div>

      {/* Price (limit only) */}
      {orderType === "limit" && (
        <div className="flex flex-col gap-1">
          <label
            htmlFor="order-price"
            className="text-xs text-text-muted"
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
            className="rounded border border-border bg-bg-primary px-2 py-1 text-xs text-text-primary placeholder:text-text-muted focus:border-accent-blue focus:outline-none"
          />
        </div>
      )}

      {/* Error display */}
      {error && (
        <div className="rounded border border-accent-red/30 bg-accent-red/10 px-2 py-1 text-xs text-accent-red">
          {error}
        </div>
      )}

      {/* Submit */}
      <button
        type="submit"
        disabled={submitting}
        className="rounded-xl px-3 py-2 text-xs font-bold uppercase tracking-wider transition-colors disabled:opacity-50"
        style={{
          backgroundColor: side === "buy" ? theme.colors.accent.green : theme.colors.accent.red,
          color: "#fff",
        }}
      >
        {submitting ? "Submitting..." : "Submit Order"}
      </button>
    </form>
  );
}

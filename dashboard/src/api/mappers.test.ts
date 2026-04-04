import { describe, it, expect } from "vitest";
import { mapOrder, mapRawOrderUpdate } from "./mappers";
import type { Order } from "./types";

describe("mapOrder", () => {
  it("maps snake_case API response to camelCase Order", () => {
    const raw = {
      id: "order-1",
      client_order_id: "client-1",
      instrument_id: "AAPL",
      side: "buy",
      type: "market",
      quantity: "100",
      price: "0",
      filled_quantity: "50",
      average_price: "149.25",
      status: "partially_filled",
      venue_id: "simulated",
      asset_class: "equity",
      created_at: "2026-04-01T10:00:00Z",
      updated_at: "2026-04-01T10:01:00Z",
      fills: [
        {
          id: "fill-1",
          order_id: "order-1",
          venue_id: "simulated",
          quantity: "50",
          price: "149.25",
          fee: "0.10",
          fee_asset: "USD",
          liquidity: "taker",
          timestamp: "2026-04-01T10:00:05Z",
        },
      ],
    };

    const order = mapOrder(raw);

    expect(order.id).toBe("order-1");
    expect(order.clientOrderId).toBe("client-1");
    expect(order.instrumentId).toBe("AAPL");
    expect(order.side).toBe("buy");
    expect(order.type).toBe("market");
    expect(order.quantity).toBe("100");
    expect(order.price).toBe("0");
    expect(order.filledQuantity).toBe("50");
    expect(order.averagePrice).toBe("149.25");
    expect(order.status).toBe("partially_filled");
    expect(order.venueId).toBe("simulated");
    expect(order.assetClass).toBe("equity");
    expect(order.createdAt).toBe("2026-04-01T10:00:00Z");
    expect(order.updatedAt).toBe("2026-04-01T10:01:00Z");
    expect(order.fills).toHaveLength(1);
    expect(order.fills[0].id).toBe("fill-1");
    expect(order.fills[0].orderId).toBe("order-1");
    expect(order.fills[0].venueId).toBe("simulated");
    expect(order.fills[0].quantity).toBe("50");
    expect(order.fills[0].price).toBe("149.25");
    expect(order.fills[0].fee).toBe("0.10");
    expect(order.fills[0].feeAsset).toBe("USD");
    expect(order.fills[0].liquidity).toBe("taker");
    expect(order.fills[0].timestamp).toBe("2026-04-01T10:00:05Z");
  });

  it("handles missing optional fields gracefully", () => {
    const raw = {
      id: "order-2",
      instrument_id: "BTC-USD",
      side: "sell",
      type: "limit",
      quantity: "1.5",
      price: "45000.00",
      filled_quantity: "0",
      average_price: "0",
      status: "new",
      created_at: "2026-04-01T10:00:00Z",
      updated_at: "2026-04-01T10:00:00Z",
      // no client_order_id, venue_id, asset_class, fills
    };

    const order = mapOrder(raw);

    expect(order.clientOrderId).toBe("");
    expect(order.venueId).toBe("");
    expect(order.assetClass).toBe("");
    expect(order.fills).toEqual([]);
  });

  it("handles null fills array", () => {
    const raw = {
      id: "order-3",
      instrument_id: "TSLA",
      side: "buy",
      type: "market",
      quantity: "10",
      price: "0",
      filled_quantity: "0",
      average_price: "0",
      status: "new",
      created_at: "2026-04-01T10:00:00Z",
      updated_at: "2026-04-01T10:00:00Z",
      fills: null,
    };

    const order = mapOrder(raw);
    expect(order.fills).toEqual([]);
  });

  it("passes through data that is already camelCase", () => {
    // If the response is already camelCase (e.g., from mock), it should still work
    const camelCase: Order = {
      id: "order-4",
      clientOrderId: "client-4",
      instrumentId: "GOOG",
      side: "buy",
      type: "limit",
      quantity: "5",
      price: "2800.00",
      filledQuantity: "0",
      averagePrice: "0",
      status: "new",
      venueId: "alpaca",
      assetClass: "equity",
      createdAt: "2026-04-01T10:00:00Z",
      updatedAt: "2026-04-01T10:00:00Z",
      fills: [],
    };

    const order = mapOrder(camelCase);

    expect(order.instrumentId).toBe("GOOG");
    expect(order.clientOrderId).toBe("client-4");
    expect(order.venueId).toBe("alpaca");
    expect(order.createdAt).toBe("2026-04-01T10:00:00Z");
  });
});

describe("mapRawOrderUpdate", () => {
  it("extracts order from 'data' envelope and maps to camelCase", () => {
    const rawMessage = {
      type: "order_update",
      data: {
        id: "order-1",
        client_order_id: "client-1",
        instrument_id: "AAPL",
        side: "buy",
        type: "market",
        quantity: "100",
        price: "0",
        filled_quantity: "100",
        average_price: "150.00",
        status: "filled",
        venue_id: "simulated",
        created_at: "2026-04-01T10:00:00Z",
        updated_at: "2026-04-01T10:00:05Z",
      },
    };

    const update = mapRawOrderUpdate(rawMessage);

    expect(update.type).toBe("order_update");
    expect(update.order.id).toBe("order-1");
    expect(update.order.instrumentId).toBe("AAPL");
    expect(update.order.filledQuantity).toBe("100");
    expect(update.order.averagePrice).toBe("150.00");
    expect(update.order.venueId).toBe("simulated");
    expect(update.order.createdAt).toBe("2026-04-01T10:00:00Z");
  });

  it("handles 'order' envelope key as well (forward compatibility)", () => {
    const rawMessage = {
      type: "order_update",
      order: {
        id: "order-2",
        instrument_id: "TSLA",
        side: "sell",
        type: "limit",
        quantity: "10",
        price: "200.00",
        filled_quantity: "0",
        average_price: "0",
        status: "new",
        venue_id: "alpaca",
        created_at: "2026-04-01T10:00:00Z",
        updated_at: "2026-04-01T10:00:00Z",
      },
    };

    const update = mapRawOrderUpdate(rawMessage);

    expect(update.type).toBe("order_update");
    expect(update.order.instrumentId).toBe("TSLA");
  });
});

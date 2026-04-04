import { describe, it, expect } from "vitest";
import { mapOrder, mapRawOrderUpdate, mapPosition, mapRawPositionUpdate } from "./mappers";
import type { Order, Position } from "./types";

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

describe("mapPosition", () => {
  it("maps snake_case API response to camelCase Position", () => {
    const raw = {
      instrument_id: "AAPL",
      venue_id: "alpaca",
      quantity: "100",
      average_cost: "150.00",
      market_price: "155.00",
      unrealized_pnl: "500.00",
      realized_pnl: "0.00",
      unsettled_quantity: "0",
      asset_class: "equity",
      quote_currency: "USD",
    };

    const pos = mapPosition(raw);

    expect(pos.instrumentId).toBe("AAPL");
    expect(pos.venueId).toBe("alpaca");
    expect(pos.quantity).toBe("100");
    expect(pos.averageCost).toBe("150.00");
    expect(pos.marketPrice).toBe("155.00");
    expect(pos.unrealizedPnl).toBe("500.00");
    expect(pos.realizedPnl).toBe("0.00");
    expect(pos.unsettledQuantity).toBe("0");
    expect(pos.assetClass).toBe("equity");
    expect(pos.quoteCurrency).toBe("USD");
  });

  it("handles missing optional fields with sensible defaults", () => {
    const raw = {
      instrument_id: "BTC-USD",
      venue_id: "binance",
      quantity: "1.5",
    };

    const pos = mapPosition(raw);

    expect(pos.instrumentId).toBe("BTC-USD");
    expect(pos.venueId).toBe("binance");
    expect(pos.quantity).toBe("1.5");
    expect(pos.averageCost).toBe("0");
    expect(pos.marketPrice).toBe("0");
    expect(pos.unrealizedPnl).toBe("0");
    expect(pos.realizedPnl).toBe("0");
    expect(pos.unsettledQuantity).toBe("0");
    expect(pos.assetClass).toBe("");
    expect(pos.quoteCurrency).toBe("");
  });

  it("passes through data that is already camelCase", () => {
    const camelCase: Position = {
      instrumentId: "GOOG",
      venueId: "alpaca",
      quantity: "50",
      averageCost: "2800.00",
      marketPrice: "2850.00",
      unrealizedPnl: "2500.00",
      realizedPnl: "100.00",
      unsettledQuantity: "10",
      assetClass: "equity",
      quoteCurrency: "USD",
    };

    const pos = mapPosition(camelCase);

    expect(pos.instrumentId).toBe("GOOG");
    expect(pos.venueId).toBe("alpaca");
    expect(pos.averageCost).toBe("2800.00");
    expect(pos.marketPrice).toBe("2850.00");
    expect(pos.unrealizedPnl).toBe("2500.00");
    expect(pos.quoteCurrency).toBe("USD");
  });
});

describe("mapRawPositionUpdate", () => {
  it("extracts position from 'data' envelope and maps to camelCase", () => {
    const rawMessage = {
      type: "position_update",
      data: {
        instrument_id: "AAPL",
        venue_id: "alpaca",
        quantity: "200",
        average_cost: "152.00",
        market_price: "160.00",
        unrealized_pnl: "1600.00",
        realized_pnl: "0.00",
        unsettled_quantity: "50",
        asset_class: "equity",
        quote_currency: "USD",
      },
    };

    const update = mapRawPositionUpdate(rawMessage);

    expect(update.type).toBe("position_update");
    expect(update.position.instrumentId).toBe("AAPL");
    expect(update.position.venueId).toBe("alpaca");
    expect(update.position.quantity).toBe("200");
    expect(update.position.averageCost).toBe("152.00");
    expect(update.position.marketPrice).toBe("160.00");
    expect(update.position.unrealizedPnl).toBe("1600.00");
    expect(update.position.unsettledQuantity).toBe("50");
  });

  it("handles 'position' envelope key as well (forward compatibility)", () => {
    const rawMessage = {
      type: "position_update",
      position: {
        instrument_id: "ETH-USD",
        venue_id: "binance",
        quantity: "10",
        average_cost: "3000.00",
        market_price: "3100.00",
        unrealized_pnl: "1000.00",
        realized_pnl: "0.00",
        unsettled_quantity: "0",
        asset_class: "crypto",
        quote_currency: "USD",
      },
    };

    const update = mapRawPositionUpdate(rawMessage);

    expect(update.type).toBe("position_update");
    expect(update.position.instrumentId).toBe("ETH-USD");
    expect(update.position.venueId).toBe("binance");
  });
});

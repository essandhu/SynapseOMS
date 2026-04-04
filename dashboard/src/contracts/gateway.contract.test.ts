import { describe, it, expect } from "vitest";
import {
  OrderSchema,
  PositionSchema,
  InstrumentSchema,
  VenueSchema,
} from "./schemas";
import {
  mockOrders,
  mockPositions,
  mockInstruments,
  mockVenues,
} from "../mocks/data";

describe("Gateway API contracts", () => {
  describe("Order", () => {
    it.each(mockOrders.map((o) => [o.id, o]))(
      "order %s matches schema",
      (_id, order) => {
        const result = OrderSchema.safeParse(order);
        if (!result.success) {
          expect.fail(
            `Order schema validation failed:\n${result.error.issues.map((i) => `  ${i.path.join(".")}: ${i.message}`).join("\n")}`,
          );
        }
      },
    );
  });

  describe("Position", () => {
    it.each(mockPositions.map((p) => [p.instrumentId, p]))(
      "position %s matches schema",
      (_id, position) => {
        const result = PositionSchema.safeParse(position);
        if (!result.success) {
          expect.fail(
            `Position schema validation failed:\n${result.error.issues.map((i) => `  ${i.path.join(".")}: ${i.message}`).join("\n")}`,
          );
        }
      },
    );
  });

  describe("Instrument", () => {
    it.each(mockInstruments.map((i) => [i.id, i]))(
      "instrument %s matches schema",
      (_id, instrument) => {
        const result = InstrumentSchema.safeParse(instrument);
        if (!result.success) {
          expect.fail(
            `Instrument schema validation failed:\n${result.error.issues.map((i) => `  ${i.path.join(".")}: ${i.message}`).join("\n")}`,
          );
        }
      },
    );
  });

  describe("Venue", () => {
    it.each(mockVenues.map((v) => [v.id, v]))(
      "venue %s matches schema",
      (_id, venue) => {
        const result = VenueSchema.safeParse(venue);
        if (!result.success) {
          expect.fail(
            `Venue schema validation failed:\n${result.error.issues.map((i) => `  ${i.path.join(".")}: ${i.message}`).join("\n")}`,
          );
        }
      },
    );
  });
});

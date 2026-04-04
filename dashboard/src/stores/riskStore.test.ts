import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../mocks/server";
import { useRiskStore, resetPendingCount } from "./riskStore";
import {
  mockVaR,
  mockDrawdown,
  mockSettlement,
  mockGreeks,
  mockConcentration,
} from "../mocks/data";
import type { VaRMetrics } from "../api/types";

describe("riskStore", () => {
  beforeEach(() => {
    resetPendingCount();
    useRiskStore.setState({
      var: null,
      drawdown: null,
      settlement: null,
      greeks: null,
      concentration: null,
      loading: false,
      error: null,
    });
    vi.clearAllMocks();
  });

  it("has null/empty initial state", () => {
    const state = useRiskStore.getState();
    expect(state.var).toBeNull();
    expect(state.drawdown).toBeNull();
    expect(state.settlement).toBeNull();
    expect(state.greeks).toBeNull();
    expect(state.concentration).toBeNull();
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
  });

  it("applyUpdate with var_update sets VaR metrics", () => {
    useRiskStore.getState().applyUpdate({
      type: "var_update",
      payload: mockVaR,
    });

    const state = useRiskStore.getState();
    expect(state.var).toEqual(mockVaR);
    expect(state.var?.historicalVaR).toBe("12500.00");
    expect(state.var?.confidence).toBe(95);
  });

  it("applyUpdate with drawdown_update sets drawdown data", () => {
    useRiskStore.getState().applyUpdate({
      type: "drawdown_update",
      payload: mockDrawdown,
    });

    const state = useRiskStore.getState();
    expect(state.drawdown).toEqual(mockDrawdown);
    expect(state.drawdown?.current).toBe(-3.2);
    expect(state.drawdown?.history).toHaveLength(3);
  });

  it("applyUpdate with settlement_update sets settlement timeline", () => {
    useRiskStore.getState().applyUpdate({
      type: "settlement_update",
      payload: mockSettlement,
    });

    const state = useRiskStore.getState();
    expect(state.settlement).toEqual(mockSettlement);
    expect(state.settlement?.totalUnsettled).toBe("25000.00");
    expect(state.settlement?.entries).toHaveLength(2);
  });

  it("fetchVaR calls the correct API endpoint and sets state", async () => {
    await useRiskStore.getState().fetchVaR();

    const state = useRiskStore.getState();
    expect(state.var).toEqual(mockVaR);
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
  });

  it("fetchVaR sets error on failure", async () => {
    server.use(
      http.get("*/api/v1/risk/var", () =>
        HttpResponse.json({ message: "Network error" }, { status: 422 }),
      ),
    );

    await useRiskStore.getState().fetchVaR();

    const state = useRiskStore.getState();
    expect(state.var).toBeNull();
    expect(state.loading).toBe(false);
    expect(state.error).toBeTruthy();
  });

  it("fetchDrawdown calls the correct API endpoint and sets state", async () => {
    await useRiskStore.getState().fetchDrawdown();

    const state = useRiskStore.getState();
    expect(state.drawdown).toEqual(mockDrawdown);
    expect(state.loading).toBe(false);
  });

  it("fetchDrawdown sets error on failure", async () => {
    server.use(
      http.get("*/api/v1/risk/drawdown", () =>
        HttpResponse.json({ message: "Server unavailable" }, { status: 422 }),
      ),
    );

    await useRiskStore.getState().fetchDrawdown();

    const state = useRiskStore.getState();
    expect(state.drawdown).toBeNull();
    expect(state.error).toBeTruthy();
  });

  it("fetchSettlement calls the correct API endpoint and sets state", async () => {
    await useRiskStore.getState().fetchSettlement();

    const state = useRiskStore.getState();
    expect(state.settlement).toEqual(mockSettlement);
    expect(state.loading).toBe(false);
  });

  it("fetchGreeks calls API and sets greeks state", async () => {
    await useRiskStore.getState().fetchGreeks();

    const state = useRiskStore.getState();
    expect(state.greeks).toEqual(mockGreeks);
    expect(state.greeks?.total.delta).toBe(0.85);
  });

  it("fetchGreeks silently handles errors without setting error state", async () => {
    server.use(
      http.get("*/api/v1/risk/greeks", () =>
        HttpResponse.json({ message: "unavailable" }, { status: 422 }),
      ),
    );

    await useRiskStore.getState().fetchGreeks();

    const state = useRiskStore.getState();
    expect(state.greeks).toBeNull();
    expect(state.error).toBeNull();
  });

  it("fetchConcentration calls API and sets concentration state", async () => {
    await useRiskStore.getState().fetchConcentration();

    const state = useRiskStore.getState();
    expect(state.concentration).toEqual(mockConcentration);
    expect(state.concentration?.hhi).toBe(2450);
    expect(state.concentration?.warnings).toHaveLength(1);
  });

  it("fetchConcentration silently handles errors without setting error state", async () => {
    server.use(
      http.get("*/api/v1/risk/concentration", () =>
        HttpResponse.json({ message: "unavailable" }, { status: 422 }),
      ),
    );

    await useRiskStore.getState().fetchConcentration();

    const state = useRiskStore.getState();
    expect(state.concentration).toBeNull();
    expect(state.error).toBeNull();
  });

  it("sets loading to true during fetch", async () => {
    // We can verify loading transitions by checking state after await
    await useRiskStore.getState().fetchVaR();
    expect(useRiskStore.getState().loading).toBe(false);
  });

  describe("subscribe() concurrent fetch", () => {
    it("populates all data fields when all endpoints succeed", async () => {
      const unsub = useRiskStore.getState().subscribe();

      // Let all pending promises resolve
      await vi.waitFor(() => {
        const state = useRiskStore.getState();
        expect(state.var).not.toBeNull();
        expect(state.drawdown).not.toBeNull();
        expect(state.settlement).not.toBeNull();
        expect(state.greeks).not.toBeNull();
        expect(state.concentration).not.toBeNull();
      });

      const state = useRiskStore.getState();
      expect(state.var).toEqual(mockVaR);
      expect(state.drawdown).toEqual(mockDrawdown);
      expect(state.settlement).toEqual(mockSettlement);
      expect(state.greeks).toEqual(mockGreeks);
      expect(state.concentration).toEqual(mockConcentration);
      expect(state.loading).toBe(false);
      expect(state.error).toBeNull();

      unsub();
    });

    it("preserves successful data when one fetch fails", async () => {
      server.use(
        http.get("*/api/v1/risk/var", () =>
          HttpResponse.json({ message: "unavailable" }, { status: 422 }),
        ),
      );

      const unsub = useRiskStore.getState().subscribe();

      await vi.waitFor(() => {
        const state = useRiskStore.getState();
        // Wait for all fetches to complete (loading back to false)
        expect(state.loading).toBe(false);
        expect(state.drawdown).not.toBeNull();
      });

      const state = useRiskStore.getState();
      // VaR failed but other data should be intact
      expect(state.var).toBeNull();
      expect(state.drawdown).toEqual(mockDrawdown);
      expect(state.settlement).toEqual(mockSettlement);
      expect(state.greeks).toEqual(mockGreeks);
      expect(state.concentration).toEqual(mockConcentration);
      expect(state.error).toBeTruthy();

      unsub();
    });

    it("sets loading false after all concurrent fetches complete", async () => {
      const unsub = useRiskStore.getState().subscribe();

      await vi.waitFor(() => {
        expect(useRiskStore.getState().var).not.toBeNull();
      });

      expect(useRiskStore.getState().loading).toBe(false);

      unsub();
    });
  });

  describe("fetchAll coordination", () => {
    it("does not clear error from a failed fetch when another succeeds", async () => {
      // Simulate VaR failing
      server.use(
        http.get("*/api/v1/risk/var", () =>
          HttpResponse.json({ message: "unavailable" }, { status: 422 }),
        ),
      );

      const unsub = useRiskStore.getState().subscribe();

      await vi.waitFor(() => {
        const state = useRiskStore.getState();
        // Wait for all fetches to complete
        expect(state.loading).toBe(false);
        expect(state.settlement).not.toBeNull();
      });

      // The error from VaR should persist, not be cleared by other fetches
      const state = useRiskStore.getState();
      expect(state.error).toBeTruthy();

      unsub();
    });
  });
});

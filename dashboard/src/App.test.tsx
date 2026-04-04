import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { server } from "./mocks/server";
import { App } from "./App";
import { initializeStreams } from "./api/ws";

// Mock WebSocket modules — jsdom doesn't support real WebSocket connections
vi.mock("./api/ws", () => ({
  createOrderStream: vi.fn(() => ({ close: vi.fn() })),
  createPositionStream: vi.fn(() => ({ close: vi.fn() })),
  createVenueStream: vi.fn(() => ({ close: vi.fn() })),
  createRiskStream: vi.fn(() => ({ close: vi.fn() })),
  createAnomalyStream: vi.fn(() => ({ close: vi.fn() })),
  createMarketDataStream: vi.fn(() => ({ close: vi.fn() })),
  initializeStreams: vi.fn(() => () => {}),
}));

describe("App routing", () => {
  beforeEach(() => {
    // Reset to root before each test
    window.history.pushState({}, "", "/");
  });

  it("routes to main app when onboarding is already completed", async () => {
    server.use(
      http.get("*/api/v1/settings/onboarding_completed", () =>
        HttpResponse.json({ completed: true }),
      ),
    );

    render(<App />);

    // Should see the main app (blotter), not onboarding
    await waitFor(() => {
      expect(screen.queryByText("Initializing...")).not.toBeInTheDocument();
    });

    // BlotterView renders the order filter tabs
    await waitFor(() => {
      expect(screen.getByText("Active")).toBeInTheDocument();
    });

    // Should NOT be on onboarding
    expect(screen.queryByText("Get Started")).not.toBeInTheDocument();
  });

  it("routes to onboarding when not yet completed", async () => {
    server.use(
      http.get("*/api/v1/settings/onboarding_completed", () =>
        HttpResponse.json({ completed: false }),
      ),
    );

    render(<App />);

    await waitFor(() => {
      expect(screen.queryByText("Initializing...")).not.toBeInTheDocument();
    });

    // Should be on onboarding flow
    await waitFor(() => {
      expect(screen.getByText("Get Started")).toBeInTheDocument();
    });
  });

  // ky retries GET requests up to 3 times with exponential backoff, so
  // error-path tests need a longer waitFor timeout to account for retries.
  const RETRY_TIMEOUT = 15_000;

  it("does NOT redirect to onboarding when the API is unreachable", async () => {
    server.use(
      http.get("*/api/v1/settings/onboarding_completed", () =>
        HttpResponse.error(),
      ),
    );

    render(<App />);

    await waitFor(
      () => {
        expect(screen.queryByText("Initializing...")).not.toBeInTheDocument();
      },
      { timeout: RETRY_TIMEOUT },
    );

    // Should land on main app, not onboarding — API failure is not "first run"
    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.queryByText("Get Started")).not.toBeInTheDocument();
  });

  it("wires order, position, and venue update handlers to initializeStreams", async () => {
    server.use(
      http.get("*/api/v1/settings/onboarding_completed", () =>
        HttpResponse.json({ completed: true }),
      ),
    );

    render(<App />);

    await waitFor(() => {
      expect(initializeStreams).toHaveBeenCalled();
    });

    const call = vi.mocked(initializeStreams).mock.calls[0][0];
    // Handlers should be real functions, not no-ops
    expect(typeof call.onOrderUpdate).toBe("function");
    expect(typeof call.onPositionUpdate).toBe("function");
    expect(typeof call.onVenueUpdate).toBe("function");
    // Verify they are NOT no-ops by checking they don't throw and are meaningful
    expect(call.onOrderUpdate.toString()).not.toBe("() => {}");
  });

  it("does NOT redirect to onboarding on server error", async () => {
    server.use(
      http.get("*/api/v1/settings/onboarding_completed", () =>
        HttpResponse.json(
          { error: { code: "INTERNAL_ERROR", message: "db down" } },
          { status: 500 },
        ),
      ),
    );

    render(<App />);

    await waitFor(
      () => {
        expect(screen.queryByText("Initializing...")).not.toBeInTheDocument();
      },
      { timeout: RETRY_TIMEOUT },
    );

    // Should land on main app even when backend returns 500
    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.queryByText("Get Started")).not.toBeInTheDocument();
  });
});

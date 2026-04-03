import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { OnboardingView } from "./OnboardingView";

// Mock react-router
const mockNavigate = vi.fn();
vi.mock("react-router", () => ({
  useNavigate: () => mockNavigate,
}));

// Mock venueStore for CredentialForm
const mockStoreCredentials = vi.fn();
const mockConnectVenue = vi.fn();
vi.mock("../stores/venueStore", () => ({
  useVenueStore: (selector: (s: unknown) => unknown) =>
    selector({
      storeCredentials: mockStoreCredentials,
      connectVenue: mockConnectVenue,
    }),
}));

// Mock completeOnboarding API call
const mockCompleteOnboarding = vi.fn();
vi.mock("../api/rest", () => ({
  completeOnboarding: (...args: unknown[]) => mockCompleteOnboarding(...args),
}));

describe("OnboardingView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockStoreCredentials.mockResolvedValue(undefined);
    mockConnectVenue.mockResolvedValue(undefined);
    mockCompleteOnboarding.mockResolvedValue(undefined);
  });

  // ── Step 1: Welcome ──────────────────────────────────────────────────

  it("renders welcome step initially", () => {
    render(<OnboardingView />);

    expect(screen.getByText("Welcome to SynapseOMS")).toBeInTheDocument();
    expect(screen.getByText("Get Started")).toBeInTheDocument();
    expect(
      screen.getByText(/professional-grade order management system/),
    ).toBeInTheDocument();
  });

  it("displays security messaging on welcome step", () => {
    render(<OnboardingView />);

    expect(screen.getByText("Self-Hosted Security Model")).toBeInTheDocument();
    expect(
      screen.getByText(/AES-256-GCM/),
    ).toBeInTheDocument();
  });

  it("clicking Get Started advances to passphrase step", async () => {
    render(<OnboardingView />);

    fireEvent.click(screen.getByText("Get Started"));

    expect(screen.getByText("Set Master Passphrase")).toBeInTheDocument();
    expect(screen.getByLabelText("Passphrase")).toBeInTheDocument();
    expect(screen.getByLabelText("Confirm Passphrase")).toBeInTheDocument();
  });

  // ── Step 2: Passphrase ───────────────────────────────────────────────

  it("passphrase step shows strength indicator for short password", async () => {
    const user = userEvent.setup();
    render(<OnboardingView />);

    fireEvent.click(screen.getByText("Get Started"));

    const passphraseInput = screen.getByLabelText("Passphrase");
    await user.type(passphraseInput, "abc");

    // Short password should show "Weak" strength
    expect(screen.getByText(/Weak/)).toBeInTheDocument();
  });

  it("passphrase strength shows Medium for moderate password", async () => {
    const user = userEvent.setup();
    render(<OnboardingView />);

    fireEvent.click(screen.getByText("Get Started"));

    const passphraseInput = screen.getByLabelText("Passphrase");
    await user.type(passphraseInput, "Abcdef12");

    expect(screen.getByText(/Medium/)).toBeInTheDocument();
  });

  it("passphrase strength shows Strong for strong password", async () => {
    const user = userEvent.setup();
    render(<OnboardingView />);

    fireEvent.click(screen.getByText("Get Started"));

    const passphraseInput = screen.getByLabelText("Passphrase");
    await user.type(passphraseInput, "MyStr0ng!Pass");

    expect(screen.getByText(/Strong/)).toBeInTheDocument();
  });

  it("passphrase Set Passphrase button is disabled when too short", async () => {
    const user = userEvent.setup();
    render(<OnboardingView />);

    fireEvent.click(screen.getByText("Get Started"));

    const passphraseInput = screen.getByLabelText("Passphrase");
    const confirmInput = screen.getByLabelText("Confirm Passphrase");

    await user.type(passphraseInput, "short");
    await user.type(confirmInput, "short");

    const setButton = screen.getByText("Set Passphrase");
    expect(setButton).toBeDisabled();
  });

  it("passphrase confirmation must match", async () => {
    const user = userEvent.setup();
    render(<OnboardingView />);

    fireEvent.click(screen.getByText("Get Started"));

    const passphraseInput = screen.getByLabelText("Passphrase");
    const confirmInput = screen.getByLabelText("Confirm Passphrase");

    await user.type(passphraseInput, "MyStr0ng!Pass");
    await user.type(confirmInput, "Different");

    expect(screen.getByText("Passphrases do not match")).toBeInTheDocument();
    expect(screen.getByText("Set Passphrase")).toBeDisabled();
  });

  it("passphrase step shows encryption details", async () => {
    render(<OnboardingView />);

    fireEvent.click(screen.getByText("Get Started"));

    expect(screen.getByText(/Argon2id/)).toBeInTheDocument();
    expect(screen.getByText(/AES-256-GCM/)).toBeInTheDocument();
  });

  // ── Step 3: Venue Choice ─────────────────────────────────────────────

  it("venue selection step shows 3 options", async () => {
    const user = userEvent.setup();
    render(<OnboardingView />);

    // Step 1 -> 2
    fireEvent.click(screen.getByText("Get Started"));

    // Fill passphrase correctly to enable next
    const passphraseInput = screen.getByLabelText("Passphrase");
    const confirmInput = screen.getByLabelText("Confirm Passphrase");
    await user.type(passphraseInput, "MyStr0ng!Pass");
    await user.type(confirmInput, "MyStr0ng!Pass");

    // Step 2 -> 3
    fireEvent.click(screen.getByText("Set Passphrase"));

    // Verify venue selection step
    expect(screen.getByText("Choose Your Venue")).toBeInTheDocument();
    expect(screen.getByText("Alpaca (Equities)")).toBeInTheDocument();
    expect(screen.getByText("Binance Testnet (Crypto)")).toBeInTheDocument();
    expect(screen.getByText("Start with Simulator")).toBeInTheDocument();
  });

  // ── Step 4: Credentials ──────────────────────────────────────────────

  it("credential step shows security messaging", async () => {
    const user = userEvent.setup();
    render(<OnboardingView />);

    // Navigate to credentials step
    fireEvent.click(screen.getByText("Get Started"));
    await user.type(screen.getByLabelText("Passphrase"), "MyStr0ng!Pass");
    await user.type(screen.getByLabelText("Confirm Passphrase"), "MyStr0ng!Pass");
    fireEvent.click(screen.getByText("Set Passphrase"));

    // Select Alpaca
    fireEvent.click(screen.getByText("Alpaca (Equities)"));
    fireEvent.click(screen.getByText("Continue"));

    // Security messaging should be visible (multiple elements may contain AES-256-GCM)
    const aesElements = screen.getAllByText(/AES-256-GCM/);
    expect(aesElements.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/never leave this machine/i)).toBeInTheDocument();
  });

  it("credential form shows inline validation for empty fields", async () => {
    const user = userEvent.setup();
    render(<OnboardingView />);

    // Navigate to credentials step
    fireEvent.click(screen.getByText("Get Started"));
    await user.type(screen.getByLabelText("Passphrase"), "MyStr0ng!Pass");
    await user.type(screen.getByLabelText("Confirm Passphrase"), "MyStr0ng!Pass");
    fireEvent.click(screen.getByText("Set Passphrase"));
    fireEvent.click(screen.getByText("Alpaca (Equities)"));
    fireEvent.click(screen.getByText("Continue"));

    // The Test Connection button should be disabled when fields empty
    expect(screen.getByText("Test Connection")).toBeDisabled();
  });

  it("credential form shows spinner during connection test", async () => {
    // Make storeCredentials hang to keep loading state
    mockStoreCredentials.mockImplementation(
      () => new Promise(() => {}), // never resolves
    );

    const user = userEvent.setup();
    render(<OnboardingView />);

    // Navigate to credentials step
    fireEvent.click(screen.getByText("Get Started"));
    await user.type(screen.getByLabelText("Passphrase"), "MyStr0ng!Pass");
    await user.type(screen.getByLabelText("Confirm Passphrase"), "MyStr0ng!Pass");
    fireEvent.click(screen.getByText("Set Passphrase"));
    fireEvent.click(screen.getByText("Alpaca (Equities)"));
    fireEvent.click(screen.getByText("Continue"));

    // Fill credential fields
    await user.type(screen.getByLabelText("API Key ID"), "test-key-123");
    await user.type(screen.getByLabelText("Secret Key"), "test-secret-456");

    // Click test connection
    fireEvent.click(screen.getByText("Test Connection"));

    // Should show loading state
    await waitFor(() => {
      expect(screen.getByText("Testing Connection...")).toBeInTheDocument();
    });
  });

  it("credential form shows error message on connection failure", async () => {
    mockStoreCredentials.mockRejectedValue(new Error("Invalid API key format"));

    const user = userEvent.setup();
    render(<OnboardingView />);

    // Navigate to credentials step
    fireEvent.click(screen.getByText("Get Started"));
    await user.type(screen.getByLabelText("Passphrase"), "MyStr0ng!Pass");
    await user.type(screen.getByLabelText("Confirm Passphrase"), "MyStr0ng!Pass");
    fireEvent.click(screen.getByText("Set Passphrase"));
    fireEvent.click(screen.getByText("Alpaca (Equities)"));
    fireEvent.click(screen.getByText("Continue"));

    // Fill credential fields
    await user.type(screen.getByLabelText("API Key ID"), "bad-key");
    await user.type(screen.getByLabelText("Secret Key"), "bad-secret");

    // Click test connection
    fireEvent.click(screen.getByText("Test Connection"));

    // Should show error message
    await waitFor(() => {
      expect(screen.getByText("Invalid API key format")).toBeInTheDocument();
    });

    // Should show retry button
    expect(screen.getByText("Retry Connection")).toBeInTheDocument();
  });

  it("credential form shows success banner after successful connection", async () => {
    const user = userEvent.setup();
    render(<OnboardingView />);

    // Navigate to credentials step
    fireEvent.click(screen.getByText("Get Started"));
    await user.type(screen.getByLabelText("Passphrase"), "MyStr0ng!Pass");
    await user.type(screen.getByLabelText("Confirm Passphrase"), "MyStr0ng!Pass");
    fireEvent.click(screen.getByText("Set Passphrase"));
    fireEvent.click(screen.getByText("Alpaca (Equities)"));
    fireEvent.click(screen.getByText("Continue"));

    // Fill credential fields
    await user.type(screen.getByLabelText("API Key ID"), "valid-key");
    await user.type(screen.getByLabelText("Secret Key"), "valid-secret");

    // Click test connection
    fireEvent.click(screen.getByText("Test Connection"));

    // Should show success message
    await waitFor(() => {
      expect(screen.getByText(/Connected!/)).toBeInTheDocument();
    });
  });

  // ── Back navigation ──────────────────────────────────────────────────

  it("back button on passphrase returns to welcome", async () => {
    render(<OnboardingView />);

    fireEvent.click(screen.getByText("Get Started"));
    expect(screen.getByText("Set Master Passphrase")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Back"));
    expect(screen.getByText("Welcome to SynapseOMS")).toBeInTheDocument();
  });

  it("back button on venue returns to passphrase", async () => {
    const user = userEvent.setup();
    render(<OnboardingView />);

    // Go to venue step
    fireEvent.click(screen.getByText("Get Started"));
    await user.type(screen.getByLabelText("Passphrase"), "MyStr0ng!Pass");
    await user.type(screen.getByLabelText("Confirm Passphrase"), "MyStr0ng!Pass");
    fireEvent.click(screen.getByText("Set Passphrase"));
    expect(screen.getByText("Choose Your Venue")).toBeInTheDocument();

    // Go back
    fireEvent.click(screen.getByText("Back"));
    expect(screen.getByText("Set Master Passphrase")).toBeInTheDocument();
  });

  it("back button on credentials returns to venue choice", async () => {
    const user = userEvent.setup();
    render(<OnboardingView />);

    // Navigate to credentials step
    fireEvent.click(screen.getByText("Get Started"));
    await user.type(screen.getByLabelText("Passphrase"), "MyStr0ng!Pass");
    await user.type(screen.getByLabelText("Confirm Passphrase"), "MyStr0ng!Pass");
    fireEvent.click(screen.getByText("Set Passphrase"));
    fireEvent.click(screen.getByText("Alpaca (Equities)"));
    fireEvent.click(screen.getByText("Continue"));

    // Go back
    fireEvent.click(screen.getByText("Back"));
    expect(screen.getByText("Choose Your Venue")).toBeInTheDocument();
  });

  it("back from ready with simulator goes to venue choice", async () => {
    const user = userEvent.setup();
    render(<OnboardingView />);

    // Navigate to ready via simulator
    fireEvent.click(screen.getByText("Get Started"));
    await user.type(screen.getByLabelText("Passphrase"), "MyStr0ng!Pass");
    await user.type(screen.getByLabelText("Confirm Passphrase"), "MyStr0ng!Pass");
    fireEvent.click(screen.getByText("Set Passphrase"));
    fireEvent.click(screen.getByText("Start with Simulator"));
    fireEvent.click(screen.getByText("Skip to Finish"));

    // Should be on ready step
    expect(screen.getByText(/You're all set!/)).toBeInTheDocument();

    // Go back should return to venue choice (skipping credentials)
    fireEvent.click(screen.getByText("Back"));
    expect(screen.getByText("Choose Your Venue")).toBeInTheDocument();
  });

  // ── Network error handling ───────────────────────────────────────────

  it("credential form shows retry option after network error", async () => {
    mockStoreCredentials.mockRejectedValue(new Error("Network error: connection refused"));

    const user = userEvent.setup();
    render(<OnboardingView />);

    // Navigate to credentials step
    fireEvent.click(screen.getByText("Get Started"));
    await user.type(screen.getByLabelText("Passphrase"), "MyStr0ng!Pass");
    await user.type(screen.getByLabelText("Confirm Passphrase"), "MyStr0ng!Pass");
    fireEvent.click(screen.getByText("Set Passphrase"));
    fireEvent.click(screen.getByText("Alpaca (Equities)"));
    fireEvent.click(screen.getByText("Continue"));

    await user.type(screen.getByLabelText("API Key ID"), "test-key");
    await user.type(screen.getByLabelText("Secret Key"), "test-secret");
    fireEvent.click(screen.getByText("Test Connection"));

    await waitFor(() => {
      expect(screen.getByText(/Network error/)).toBeInTheDocument();
    });

    // Retry button should be available
    expect(screen.getByText("Retry Connection")).toBeInTheDocument();
  });

  // ── Ready step ───────────────────────────────────────────────────────

  it("ready step has back button and finish button", async () => {
    const user = userEvent.setup();
    render(<OnboardingView />);

    // Navigate to ready via simulator
    fireEvent.click(screen.getByText("Get Started"));
    await user.type(screen.getByLabelText("Passphrase"), "MyStr0ng!Pass");
    await user.type(screen.getByLabelText("Confirm Passphrase"), "MyStr0ng!Pass");
    fireEvent.click(screen.getByText("Set Passphrase"));
    fireEvent.click(screen.getByText("Start with Simulator"));
    fireEvent.click(screen.getByText("Skip to Finish"));

    expect(screen.getByText("Open Trading Terminal")).toBeInTheDocument();
    expect(screen.getByText("Back")).toBeInTheDocument();
  });

  it("finish button navigates to home", async () => {
    const user = userEvent.setup();
    render(<OnboardingView />);

    // Navigate to ready via simulator
    fireEvent.click(screen.getByText("Get Started"));
    await user.type(screen.getByLabelText("Passphrase"), "MyStr0ng!Pass");
    await user.type(screen.getByLabelText("Confirm Passphrase"), "MyStr0ng!Pass");
    fireEvent.click(screen.getByText("Set Passphrase"));
    fireEvent.click(screen.getByText("Start with Simulator"));
    fireEvent.click(screen.getByText("Skip to Finish"));

    fireEvent.click(screen.getByText("Open Trading Terminal"));

    await waitFor(() => {
      expect(mockCompleteOnboarding).toHaveBeenCalled();
      expect(mockNavigate).toHaveBeenCalledWith("/", { replace: true });
    });
  });
});

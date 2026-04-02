import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { OnboardingView } from "./OnboardingView";

// Mock react-router
const mockNavigate = vi.fn();
vi.mock("react-router", () => ({
  useNavigate: () => mockNavigate,
}));

// Mock CredentialForm component used in step 4
vi.mock("../components/CredentialForm", () => ({
  CredentialForm: ({ onBack }: { onBack: () => void }) => (
    <div data-testid="credential-form">
      <button onClick={onBack}>Back</button>
    </div>
  ),
}));

describe("OnboardingView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders welcome step initially", () => {
    render(<OnboardingView />);

    expect(screen.getByText("Welcome to SynapseOMS")).toBeInTheDocument();
    expect(screen.getByText("Get Started")).toBeInTheDocument();
    expect(
      screen.getByText(/professional-grade order management system/),
    ).toBeInTheDocument();
  });

  it("clicking Get Started advances to passphrase step", async () => {
    render(<OnboardingView />);

    fireEvent.click(screen.getByText("Get Started"));

    expect(screen.getByText("Set Master Passphrase")).toBeInTheDocument();
    expect(screen.getByLabelText("Passphrase")).toBeInTheDocument();
    expect(screen.getByLabelText("Confirm Passphrase")).toBeInTheDocument();
  });

  it("passphrase step shows strength indicator for short password", async () => {
    const user = userEvent.setup();
    render(<OnboardingView />);

    fireEvent.click(screen.getByText("Get Started"));

    const passphraseInput = screen.getByLabelText("Passphrase");
    await user.type(passphraseInput, "abc");

    // Short password should show "Weak" strength
    expect(screen.getByText(/Weak/)).toBeInTheDocument();
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
});

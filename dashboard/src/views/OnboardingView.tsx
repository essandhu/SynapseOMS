import { useState, useCallback } from "react";
import { useNavigate } from "react-router";
import {
  Shield,
  Lock,
  TrendingUp,
  Bitcoin,
  Activity,
  Rocket,
  ChevronRight,
  ArrowLeft,
  Eye,
  EyeOff,
} from "lucide-react";
import { CredentialForm } from "../components/CredentialForm";

// ── Types ───────────────────────────────────────────────────────────────

type Step = 1 | 2 | 3 | 4 | 5;
type VenueChoice = "alpaca" | "binance_testnet" | "simulator" | null;
type PasswordStrength = "weak" | "medium" | "strong";

// ── Helpers ─────────────────────────────────────────────────────────────

function evaluateStrength(pw: string): PasswordStrength {
  let score = 0;
  if (pw.length >= 8) score++;
  if (pw.length >= 12) score++;
  if (/[A-Z]/.test(pw) && /[a-z]/.test(pw)) score++;
  if (/\d/.test(pw)) score++;
  if (/[^A-Za-z0-9]/.test(pw)) score++;
  if (score <= 2) return "weak";
  if (score <= 3) return "medium";
  return "strong";
}

const STRENGTH_CONFIG: Record<
  PasswordStrength,
  { label: string; color: string; barColor: string; width: string }
> = {
  weak: { label: "Weak", color: "text-accent-red", barColor: "bg-accent-red", width: "w-1/3" },
  medium: {
    label: "Medium",
    color: "text-accent-yellow",
    barColor: "bg-accent-yellow",
    width: "w-2/3",
  },
  strong: {
    label: "Strong",
    color: "text-accent-green",
    barColor: "bg-accent-green",
    width: "w-full",
  },
};

// ── Step indicator ──────────────────────────────────────────────────────

function StepIndicator({ current, total }: { current: Step; total: number }) {
  return (
    <div className="flex items-center justify-center gap-2">
      {Array.from({ length: total }, (_, i) => {
        const step = i + 1;
        const isCompleted = step < current;
        const isActive = step === current;

        return (
          <div key={step} className="flex items-center gap-2">
            <div
              className={[
                "flex h-8 w-8 items-center justify-center rounded-full font-mono text-xs font-bold transition-all duration-300",
                isActive
                  ? "bg-accent-blue text-white shadow-[0_0_12px_rgba(59,130,246,0.4)]"
                  : isCompleted
                    ? "bg-accent-green/20 text-accent-green"
                    : "bg-bg-tertiary text-text-muted",
              ].join(" ")}
            >
              {isCompleted ? "\u2713" : step}
            </div>
            {step < total && (
              <div
                className={[
                  "h-px w-8 transition-colors duration-300",
                  isCompleted ? "bg-accent-green/40" : "bg-border",
                ].join(" ")}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

// ── Venue cards config ──────────────────────────────────────────────────

const VENUE_OPTIONS: {
  id: VenueChoice;
  title: string;
  subtitle: string;
  details: string;
  icon: React.ReactNode;
}[] = [
  {
    id: "alpaca",
    title: "Alpaca (Equities)",
    subtitle: "Paper Trading",
    details: "US Stocks \u00B7 Commission-free \u00B7 REST + WebSocket",
    icon: <TrendingUp className="h-6 w-6" />,
  },
  {
    id: "binance_testnet",
    title: "Binance Testnet (Crypto)",
    subtitle: "Testnet Mode",
    details: "BTC \u00B7 ETH \u00B7 SOL \u00B7 Spot + Futures",
    icon: <Bitcoin className="h-6 w-6" />,
  },
  {
    id: "simulator",
    title: "Start with Simulator",
    subtitle: "No credentials needed",
    details: "Simulated fills \u00B7 Perfect for exploration",
    icon: <Activity className="h-6 w-6" />,
  },
];

// ── Main component ──────────────────────────────────────────────────────

export function OnboardingView() {
  const navigate = useNavigate();
  const [step, setStep] = useState<Step>(1);
  const [passphrase, setPassphrase] = useState("");
  const [confirmPassphrase, setConfirmPassphrase] = useState("");
  const [showPassphrase, setShowPassphrase] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [selectedVenue, setSelectedVenue] = useState<VenueChoice>(null);

  const strength = passphrase.length > 0 ? evaluateStrength(passphrase) : null;
  const passphraseValid = passphrase.length >= 8 && passphrase === confirmPassphrase;

  const goBack = useCallback(() => {
    setStep((s) => {
      // If we're on step 5 and venue was simulator, go back to step 3
      if (s === 5 && selectedVenue === "simulator") return 3 as Step;
      return Math.max(1, s - 1) as Step;
    });
  }, [selectedVenue]);

  const finishOnboarding = useCallback(() => {
    navigate("/", { replace: true });
  }, [navigate]);

  // ── Step renderers ──────────────────────────────────────────────────

  function renderWelcome() {
    return (
      <div className="flex flex-col items-center text-center">
        <div className="mb-6 flex h-16 w-16 items-center justify-center rounded-2xl bg-accent-blue/10">
          <Shield className="h-8 w-8 text-accent-blue" />
        </div>

        <h1 className="font-mono text-3xl font-bold tracking-tight text-text-primary">
          Welcome to SynapseOMS
        </h1>

        <p className="mx-auto mt-4 max-w-lg text-base leading-relaxed text-text-secondary">
          A professional-grade order management system running entirely on your
          machine. Let&apos;s get you set up in under two minutes.
        </p>

        <div className="mt-8 rounded-lg border border-accent-green/20 bg-accent-green/5 px-6 py-4">
          <div className="flex items-start gap-3">
            <Lock className="mt-0.5 h-5 w-5 shrink-0 text-accent-green" />
            <div className="text-left">
              <p className="text-sm font-medium text-accent-green">
                Self-Hosted Security Model
              </p>
              <p className="mt-1 text-sm leading-relaxed text-text-muted">
                Your keys never leave this machine. All credentials are encrypted
                with AES-256-GCM and stored locally.
              </p>
            </div>
          </div>
        </div>

        <button
          type="button"
          onClick={() => setStep(2)}
          className="mt-10 flex items-center gap-2 rounded-lg bg-accent-blue px-8 py-3 font-mono text-sm font-semibold text-white shadow-[0_0_20px_rgba(59,130,246,0.25)] transition-all hover:bg-accent-blue/90 hover:shadow-[0_0_24px_rgba(59,130,246,0.35)] focus:outline-none focus:ring-2 focus:ring-accent-blue/50"
        >
          Get Started
          <ChevronRight className="h-4 w-4" />
        </button>
      </div>
    );
  }

  function renderPassphrase() {
    const strengthInfo = strength ? STRENGTH_CONFIG[strength] : null;

    return (
      <div className="mx-auto w-full max-w-md">
        <div className="mb-6 flex h-12 w-12 items-center justify-center rounded-xl bg-accent-purple/10">
          <Lock className="h-6 w-6 text-accent-purple" />
        </div>

        <h2 className="font-mono text-xl font-semibold text-text-primary">
          Set Master Passphrase
        </h2>
        <p className="mt-2 text-sm leading-relaxed text-text-muted">
          This passphrase encrypts your venue API keys. Choose something strong
          &mdash; you&apos;ll need it if you ever re-import credentials.
        </p>

        <div className="mt-6 space-y-4">
          {/* Passphrase input */}
          <div>
            <label
              htmlFor="passphrase"
              className="mb-1.5 block font-mono text-xs font-medium text-text-secondary"
            >
              Passphrase
            </label>
            <div className="relative">
              <input
                id="passphrase"
                type={showPassphrase ? "text" : "password"}
                value={passphrase}
                onChange={(e) => setPassphrase(e.target.value)}
                placeholder="Enter a strong passphrase"
                className="w-full rounded-md border border-border bg-bg-primary px-3 py-2.5 pr-10 font-mono text-sm text-text-primary placeholder:text-text-muted/50 focus:border-accent-blue focus:outline-none focus:ring-1 focus:ring-accent-blue"
              />
              <button
                type="button"
                onClick={() => setShowPassphrase((v) => !v)}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-secondary"
                aria-label={showPassphrase ? "Hide passphrase" : "Show passphrase"}
              >
                {showPassphrase ? (
                  <EyeOff className="h-4 w-4" />
                ) : (
                  <Eye className="h-4 w-4" />
                )}
              </button>
            </div>
          </div>

          {/* Strength indicator */}
          {passphrase.length > 0 && strengthInfo && (
            <div className="space-y-1.5">
              <div className="h-1.5 w-full overflow-hidden rounded-full bg-bg-tertiary">
                <div
                  className={`h-full rounded-full transition-all duration-300 ${strengthInfo.barColor} ${strengthInfo.width}`}
                />
              </div>
              <p className={`font-mono text-xs ${strengthInfo.color}`}>
                {strengthInfo.label}
                {strength === "weak" && " \u2014 try adding numbers and symbols"}
              </p>
            </div>
          )}

          {/* Confirm input */}
          <div>
            <label
              htmlFor="confirm-passphrase"
              className="mb-1.5 block font-mono text-xs font-medium text-text-secondary"
            >
              Confirm Passphrase
            </label>
            <div className="relative">
              <input
                id="confirm-passphrase"
                type={showConfirm ? "text" : "password"}
                value={confirmPassphrase}
                onChange={(e) => setConfirmPassphrase(e.target.value)}
                placeholder="Re-enter passphrase"
                className={[
                  "w-full rounded-md border bg-bg-primary px-3 py-2.5 pr-10 font-mono text-sm text-text-primary placeholder:text-text-muted/50 focus:outline-none focus:ring-1",
                  confirmPassphrase.length > 0 && confirmPassphrase !== passphrase
                    ? "border-accent-red focus:border-accent-red focus:ring-accent-red"
                    : "border-border focus:border-accent-blue focus:ring-accent-blue",
                ].join(" ")}
              />
              <button
                type="button"
                onClick={() => setShowConfirm((v) => !v)}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-secondary"
                aria-label={showConfirm ? "Hide confirmation" : "Show confirmation"}
              >
                {showConfirm ? (
                  <EyeOff className="h-4 w-4" />
                ) : (
                  <Eye className="h-4 w-4" />
                )}
              </button>
            </div>
            {confirmPassphrase.length > 0 && confirmPassphrase !== passphrase && (
              <p className="mt-1 font-mono text-xs text-accent-red">
                Passphrases do not match
              </p>
            )}
          </div>
        </div>

        {/* Actions */}
        <div className="mt-8 flex items-center gap-3">
          <button
            type="button"
            onClick={goBack}
            className="flex items-center gap-1.5 rounded-md px-4 py-2.5 text-sm font-medium text-text-muted transition-colors hover:text-text-primary"
          >
            <ArrowLeft className="h-4 w-4" />
            Back
          </button>
          <button
            type="button"
            onClick={() => setStep(3)}
            disabled={!passphraseValid}
            className="flex flex-1 items-center justify-center gap-2 rounded-md bg-accent-blue px-6 py-2.5 font-mono text-sm font-semibold text-white transition-all hover:bg-accent-blue/90 focus:outline-none focus:ring-2 focus:ring-accent-blue/50 disabled:cursor-not-allowed disabled:opacity-50"
          >
            Set Passphrase
            <ChevronRight className="h-4 w-4" />
          </button>
        </div>

        <div className="mt-6 rounded-lg border border-accent-green/20 bg-accent-green/5 px-5 py-3">
          <div className="flex items-start gap-3">
            <Shield className="mt-0.5 h-4 w-4 shrink-0 text-accent-green" />
            <div className="text-left">
              <p className="text-xs font-medium text-accent-green">
                How your credentials are protected
              </p>
              <p className="mt-1 text-xs leading-relaxed text-text-muted">
                Key derivation via Argon2id (time=1, memory=64MB, parallelism=4).
                Encryption with AES-256-GCM. Stored locally, never transmitted.
              </p>
            </div>
          </div>
        </div>

        <p className="mt-4 text-center text-xs text-text-muted">
          Minimum 8 characters. Both fields must match.
        </p>
      </div>
    );
  }

  function renderChooseVenue() {
    return (
      <div className="mx-auto w-full max-w-lg">
        <h2 className="font-mono text-xl font-semibold text-text-primary">
          Choose Your Venue
        </h2>
        <p className="mt-2 text-sm text-text-muted">
          Select a trading venue to connect. You can add more venues later.
        </p>

        <div className="mt-6 space-y-3">
          {VENUE_OPTIONS.map((venue) => {
            const isSelected = selectedVenue === venue.id;
            return (
              <button
                key={venue.id}
                type="button"
                onClick={() => setSelectedVenue(venue.id)}
                className={[
                  "group flex w-full items-start gap-4 rounded-lg border p-4 text-left transition-all duration-200",
                  isSelected
                    ? "border-accent-blue bg-accent-blue/5 shadow-[0_0_12px_rgba(59,130,246,0.15)]"
                    : "border-border bg-bg-secondary hover:border-text-muted/30 hover:bg-bg-tertiary/50",
                ].join(" ")}
              >
                <div
                  className={[
                    "flex h-10 w-10 shrink-0 items-center justify-center rounded-lg transition-colors",
                    isSelected
                      ? "bg-accent-blue/20 text-accent-blue"
                      : "bg-bg-tertiary text-text-muted group-hover:text-text-secondary",
                  ].join(" ")}
                >
                  {venue.icon}
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-sm font-semibold text-text-primary">
                      {venue.title}
                    </span>
                    <span
                      className={[
                        "rounded-full px-2 py-0.5 font-mono text-[10px] font-medium",
                        venue.id === "simulator"
                          ? "bg-accent-green/10 text-accent-green"
                          : "bg-accent-yellow/10 text-accent-yellow",
                      ].join(" ")}
                    >
                      {venue.subtitle}
                    </span>
                  </div>
                  <p className="mt-1 text-xs text-text-muted">{venue.details}</p>
                </div>
                {/* Radio indicator */}
                <div
                  className={[
                    "mt-1 flex h-5 w-5 shrink-0 items-center justify-center rounded-full border-2 transition-all",
                    isSelected
                      ? "border-accent-blue"
                      : "border-text-muted/30",
                  ].join(" ")}
                >
                  {isSelected && (
                    <div className="h-2.5 w-2.5 rounded-full bg-accent-blue" />
                  )}
                </div>
              </button>
            );
          })}
        </div>

        {/* Actions */}
        <div className="mt-8 flex items-center gap-3">
          <button
            type="button"
            onClick={goBack}
            className="flex items-center gap-1.5 rounded-md px-4 py-2.5 text-sm font-medium text-text-muted transition-colors hover:text-text-primary"
          >
            <ArrowLeft className="h-4 w-4" />
            Back
          </button>
          <button
            type="button"
            onClick={() => {
              if (selectedVenue === "simulator") {
                setStep(5);
              } else if (selectedVenue) {
                setStep(4);
              }
            }}
            disabled={!selectedVenue}
            className="flex flex-1 items-center justify-center gap-2 rounded-md bg-accent-blue px-6 py-2.5 font-mono text-sm font-semibold text-white transition-all hover:bg-accent-blue/90 focus:outline-none focus:ring-2 focus:ring-accent-blue/50 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {selectedVenue === "simulator" ? "Skip to Finish" : "Continue"}
            <ChevronRight className="h-4 w-4" />
          </button>
        </div>
      </div>
    );
  }

  function renderCredentials() {
    return (
      <CredentialForm
        venueId={selectedVenue!}
        onSuccess={() => setStep(5)}
        onBack={goBack}
      />
    );
  }

  function renderReady() {
    return (
      <div className="flex flex-col items-center text-center">
        <div className="mb-6 flex h-16 w-16 items-center justify-center rounded-2xl bg-accent-green/10">
          <Rocket className="h-8 w-8 text-accent-green" />
        </div>

        <h1 className="font-mono text-3xl font-bold tracking-tight text-text-primary">
          You&apos;re all set!
        </h1>

        <p className="mx-auto mt-4 max-w-md text-base leading-relaxed text-text-secondary">
          {selectedVenue === "simulator"
            ? "The simulator is ready. Place orders, test strategies, and explore the platform with zero risk."
            : "Your venue is connected and market data is flowing. You're ready to trade."}
        </p>

        <div className="mt-8 grid grid-cols-3 gap-4 text-center">
          {[
            { label: "Encrypted Keys", icon: Lock },
            { label: "Local Storage", icon: Shield },
            { label: "Real-time Feed", icon: Activity },
          ].map(({ label, icon: Icon }) => (
            <div
              key={label}
              className="rounded-lg border border-border bg-bg-secondary px-4 py-3"
            >
              <Icon className="mx-auto mb-2 h-5 w-5 text-accent-green" />
              <span className="font-mono text-[11px] text-text-muted">{label}</span>
            </div>
          ))}
        </div>

        <div className="mt-10 flex items-center justify-center gap-4">
          <button
            type="button"
            onClick={goBack}
            className="flex items-center gap-1.5 rounded-md px-4 py-2.5 text-sm font-medium text-text-muted transition-colors hover:text-text-primary"
          >
            <ArrowLeft className="h-4 w-4" />
            Back
          </button>
          <button
            type="button"
            onClick={finishOnboarding}
            className="flex items-center gap-2 rounded-lg bg-accent-green px-8 py-3 font-mono text-sm font-semibold text-white shadow-[0_0_20px_rgba(34,197,94,0.25)] transition-all hover:bg-accent-green/90 hover:shadow-[0_0_24px_rgba(34,197,94,0.35)] focus:outline-none focus:ring-2 focus:ring-accent-green/50"
          >
            Open Trading Terminal
            <ChevronRight className="h-4 w-4" />
          </button>
        </div>
      </div>
    );
  }

  // ── Render ──────────────────────────────────────────────────────────

  const stepRenderers: Record<Step, () => React.ReactNode> = {
    1: renderWelcome,
    2: renderPassphrase,
    3: renderChooseVenue,
    4: renderCredentials,
    5: renderReady,
  };

  return (
    <div className="relative flex min-h-screen flex-col bg-bg-primary font-sans text-text-primary">
      {/* Scanline overlay — same as TerminalLayout */}
      <div
        className="pointer-events-none absolute inset-0 z-50"
        style={{
          background:
            "repeating-linear-gradient(0deg, transparent, transparent 2px, rgba(255,255,255,0.015) 2px, rgba(255,255,255,0.015) 4px)",
        }}
      />

      {/* Header */}
      <header className="z-10 flex items-center border-b border-border px-4 py-3">
        <h1 className="font-mono text-sm font-semibold tracking-wider text-accent-blue">
          SynapseOMS
        </h1>
        <span className="ml-3 rounded-full bg-accent-blue/10 px-2.5 py-0.5 font-mono text-[10px] font-medium text-accent-blue">
          Setup
        </span>
      </header>

      {/* Main content */}
      <main className="z-10 flex flex-1 flex-col items-center justify-center px-6 py-12">
        {/* Step indicator */}
        <div className="mb-12">
          <StepIndicator current={step} total={5} />
        </div>

        {/* Step content */}
        <div className="w-full max-w-2xl">{stepRenderers[step]()}</div>
      </main>

      {/* Footer */}
      <footer className="z-10 flex items-center justify-center border-t border-border px-4 py-2">
        <span className="font-mono text-xs text-text-muted">
          SynapseOMS v0.1.0
        </span>
      </footer>
    </div>
  );
}

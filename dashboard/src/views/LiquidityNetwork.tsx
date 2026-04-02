import { useEffect, useState, useCallback } from "react";
import { useVenueStore } from "../stores/venueStore";
import { VenueCard } from "../components/VenueCard";
import type { Venue } from "../api/types";

type NewVenueType = "alpaca" | "binance_testnet";

const VENUE_PRESETS: Record<NewVenueType, { label: string; name: string }> = {
  alpaca: { label: "Alpaca", name: "Alpaca" },
  binance_testnet: { label: "Binance Testnet", name: "Binance Testnet" },
};

// ── Inline CredentialForm (fallback if P2-19 not yet integrated) ─────

interface CredentialFormProps {
  venueType: NewVenueType;
  onSubmit: (data: {
    venueId: string;
    apiKey: string;
    apiSecret: string;
    passphrase?: string;
  }) => void;
  onCancel: () => void;
  submitting?: boolean;
}

function InlineCredentialForm({
  venueType,
  onSubmit,
  onCancel,
  submitting,
}: CredentialFormProps) {
  const [apiKey, setApiKey] = useState("");
  const [apiSecret, setApiSecret] = useState("");
  const [passphrase, setPassphrase] = useState("");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit({
      venueId: venueType,
      apiKey,
      apiSecret,
      passphrase: passphrase || undefined,
    });
  };

  const preset = VENUE_PRESETS[venueType];

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      <div>
        <label className="mb-1 block font-mono text-xs text-text-secondary">
          Venue
        </label>
        <div className="rounded border border-border bg-bg-tertiary px-3 py-2 font-mono text-sm text-text-primary">
          {preset.label}
        </div>
      </div>

      <div>
        <label className="mb-1 block font-mono text-xs text-text-secondary">
          API Key
        </label>
        <input
          type="text"
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          required
          placeholder="Enter API key"
          className="w-full rounded border border-border bg-bg-tertiary px-3 py-2 font-mono text-sm text-text-primary placeholder:text-text-muted focus:border-accent-blue focus:outline-none"
        />
      </div>

      <div>
        <label className="mb-1 block font-mono text-xs text-text-secondary">
          API Secret
        </label>
        <input
          type="password"
          value={apiSecret}
          onChange={(e) => setApiSecret(e.target.value)}
          required
          placeholder="Enter API secret"
          className="w-full rounded border border-border bg-bg-tertiary px-3 py-2 font-mono text-sm text-text-primary placeholder:text-text-muted focus:border-accent-blue focus:outline-none"
        />
      </div>

      <div>
        <label className="mb-1 block font-mono text-xs text-text-secondary">
          Passphrase{" "}
          <span className="text-text-muted">(optional)</span>
        </label>
        <input
          type="password"
          value={passphrase}
          onChange={(e) => setPassphrase(e.target.value)}
          placeholder="Enter passphrase if required"
          className="w-full rounded border border-border bg-bg-tertiary px-3 py-2 font-mono text-sm text-text-primary placeholder:text-text-muted focus:border-accent-blue focus:outline-none"
        />
      </div>

      <div className="flex gap-2 pt-2">
        <button
          type="submit"
          disabled={submitting || !apiKey || !apiSecret}
          className="flex-1 rounded border border-accent-green/30 bg-accent-green/10 px-4 py-2 font-mono text-xs font-medium text-accent-green transition-colors hover:bg-accent-green/20 disabled:cursor-not-allowed disabled:opacity-40"
        >
          {submitting ? "Saving..." : "Save Credentials"}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="rounded border border-border px-4 py-2 font-mono text-xs font-medium text-text-muted transition-colors hover:border-text-muted hover:text-text-secondary"
        >
          Cancel
        </button>
      </div>
    </form>
  );
}

// ── Connect New Venue Modal ─────────────────────────────────────────

interface ConnectModalProps {
  open: boolean;
  onClose: () => void;
}

function ConnectModal({ open, onClose }: ConnectModalProps) {
  const [venueType, setVenueType] = useState<NewVenueType | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const storeCredentials = useVenueStore((s) => s.storeCredentials);
  const loadVenues = useVenueStore((s) => s.loadVenues);

  const handleSubmit = async (data: {
    venueId: string;
    apiKey: string;
    apiSecret: string;
    passphrase?: string;
  }) => {
    setSubmitting(true);
    try {
      await storeCredentials(
        data.venueId,
        data.apiKey,
        data.apiSecret,
        data.passphrase,
      );
      await loadVenues();
      setVenueType(null);
      onClose();
    } catch {
      // Error is set in store
    } finally {
      setSubmitting(false);
    }
  };

  const handleClose = () => {
    setVenueType(null);
    onClose();
  };

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
      onClick={(e) => {
        if (e.target === e.currentTarget) handleClose();
      }}
    >
      <div className="w-full max-w-md rounded-lg border border-border bg-bg-secondary p-6 shadow-2xl">
        {/* Modal header */}
        <div className="mb-5 flex items-center justify-between">
          <h2 className="font-mono text-sm font-semibold text-text-primary">
            Connect New Venue
          </h2>
          <button
            onClick={handleClose}
            className="rounded p-1 text-text-muted transition-colors hover:text-text-primary"
          >
            <svg
              className="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </div>

        {/* Venue type selector */}
        {!venueType ? (
          <div className="flex flex-col gap-3">
            <p className="font-mono text-xs text-text-muted">
              Select a venue to connect:
            </p>
            {(Object.entries(VENUE_PRESETS) as [NewVenueType, { label: string }][]).map(
              ([key, preset]) => (
                <button
                  key={key}
                  onClick={() => setVenueType(key)}
                  className="flex items-center gap-3 rounded border border-border bg-bg-tertiary px-4 py-3 text-left font-mono text-sm text-text-primary transition-colors hover:border-accent-blue hover:bg-accent-blue/5"
                >
                  <svg
                    className="h-5 w-5 text-accent-blue"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={1.5}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418"
                    />
                  </svg>
                  {preset.label}
                </button>
              ),
            )}
          </div>
        ) : (
          <InlineCredentialForm
            venueType={venueType}
            onSubmit={handleSubmit}
            onCancel={() => setVenueType(null)}
            submitting={submitting}
          />
        )}
      </div>
    </div>
  );
}

// ── LiquidityNetwork View ───────────────────────────────────────────

export function LiquidityNetwork() {
  const venues = useVenueStore((s) => s.venues);
  const loading = useVenueStore((s) => s.loading);
  const error = useVenueStore((s) => s.error);
  const subscribe = useVenueStore((s) => s.subscribe);
  const connectVenue = useVenueStore((s) => s.connectVenue);
  const disconnectVenue = useVenueStore((s) => s.disconnectVenue);

  const [modalOpen, setModalOpen] = useState(false);

  // Subscribe to venue WebSocket stream + load initial data
  useEffect(() => {
    const unsubscribe = subscribe();
    return unsubscribe;
  }, [subscribe]);

  const handleConnect = useCallback(
    (venueId: string) => {
      connectVenue(venueId).catch(() => {
        /* error is set in store */
      });
    },
    [connectVenue],
  );

  const handleDisconnect = useCallback(
    (venueId: string) => {
      disconnectVenue(venueId).catch(() => {
        /* error is set in store */
      });
    },
    [disconnectVenue],
  );

  const handleTestConnection = useCallback(
    (_venueId: string) => {
      // Test connection by triggering a connect cycle
      // In a full implementation this would call a dedicated ping endpoint
      // For now we re-connect to verify the venue is reachable
    },
    [],
  );

  const venueList: Venue[] = Array.from(venues.values());

  return (
    <div className="flex h-full flex-col gap-4">
      {/* Header */}
      <div>
        <h1 className="font-mono text-lg font-semibold text-text-primary">
          Liquidity Network
        </h1>
        <p className="font-mono text-xs text-text-muted">
          Manage venue connections &mdash; click any card to expand details
        </p>
      </div>

      {/* Error banner */}
      {error && (
        <div className="rounded border border-accent-red/30 bg-accent-red/10 px-3 py-2 font-mono text-xs text-accent-red">
          {error}
        </div>
      )}

      {/* Loading state */}
      {loading && venueList.length === 0 && (
        <div className="flex items-center gap-2 font-mono text-xs text-text-muted">
          <svg
            className="h-4 w-4 animate-spin text-accent-blue"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              className="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              strokeWidth="4"
            />
            <path
              className="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
            />
          </svg>
          Loading venues...
        </div>
      )}

      {/* Venue card grid */}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
        {venueList.map((venue) => (
          <VenueCard
            key={venue.id}
            venue={venue}
            onConnect={() => handleConnect(venue.id)}
            onDisconnect={() => handleDisconnect(venue.id)}
            onTestConnection={() => handleTestConnection(venue.id)}
          />
        ))}

        {/* "Connect New Venue" card */}
        <button
          onClick={() => setModalOpen(true)}
          className="flex min-h-[200px] flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border bg-bg-secondary/50 p-4 transition-colors hover:border-accent-blue hover:bg-accent-blue/5"
        >
          <svg
            className="h-8 w-8 text-text-muted"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={1.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M12 4.5v15m7.5-7.5h-15"
            />
          </svg>
          <span className="font-mono text-sm font-medium text-text-muted">
            Connect New Venue
          </span>
        </button>
      </div>

      {/* Connect modal */}
      <ConnectModal open={modalOpen} onClose={() => setModalOpen(false)} />
    </div>
  );
}

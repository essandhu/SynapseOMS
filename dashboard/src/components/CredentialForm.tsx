import { useState } from "react";
import { Loader2, CheckCircle2, AlertCircle, ArrowLeft } from "lucide-react";
import { useVenueStore } from "../stores/venueStore";

interface CredentialFormProps {
  venueId: string;
  onSuccess: () => void;
  onBack: () => void;
}

const VENUE_FIELDS: Record<string, { keyLabel: string; secretLabel: string }> = {
  alpaca: { keyLabel: "API Key ID", secretLabel: "Secret Key" },
  binance_testnet: { keyLabel: "API Key", secretLabel: "API Secret" },
};

type FormState = "idle" | "testing" | "success" | "error";

export function CredentialForm({ venueId, onSuccess, onBack }: CredentialFormProps) {
  const [apiKey, setApiKey] = useState("");
  const [apiSecret, setApiSecret] = useState("");
  const [formState, setFormState] = useState<FormState>("idle");
  const [errorMessage, setErrorMessage] = useState("");

  const storeCredentials = useVenueStore((s) => s.storeCredentials);
  const connectVenue = useVenueStore((s) => s.connectVenue);

  const fields = VENUE_FIELDS[venueId] ?? { keyLabel: "API Key", secretLabel: "API Secret" };
  const canSubmit = apiKey.trim().length > 0 && apiSecret.trim().length > 0;

  async function handleTestConnection() {
    setFormState("testing");
    setErrorMessage("");

    try {
      await storeCredentials(venueId, apiKey.trim(), apiSecret.trim());
      await connectVenue(venueId);
      setFormState("success");
      // Brief delay so user sees the success state
      setTimeout(onSuccess, 1500);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Connection failed. Please check your credentials.";
      setErrorMessage(message);
      setFormState("error");
    }
  }

  return (
    <div className="mx-auto w-full max-w-md space-y-6">
      <div>
        <h2 className="font-mono text-xl font-semibold text-text-primary">
          Enter Credentials
        </h2>
        <p className="mt-2 text-sm text-text-muted">
          Your keys are encrypted with AES-256-GCM before storage. They never leave this machine.
        </p>
      </div>

      <div className="space-y-4">
        <div>
          <label
            htmlFor="api-key"
            className="mb-1.5 block font-mono text-xs font-medium text-text-secondary"
          >
            {fields.keyLabel}
          </label>
          <input
            id="api-key"
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            disabled={formState === "testing" || formState === "success"}
            placeholder="Paste your key here"
            className="w-full rounded-md border border-border bg-bg-primary px-3 py-2.5 font-mono text-sm text-text-primary placeholder:text-text-muted/50 focus:border-accent-blue focus:outline-none focus:ring-1 focus:ring-accent-blue disabled:opacity-50"
          />
        </div>

        <div>
          <label
            htmlFor="api-secret"
            className="mb-1.5 block font-mono text-xs font-medium text-text-secondary"
          >
            {fields.secretLabel}
          </label>
          <input
            id="api-secret"
            type="password"
            value={apiSecret}
            onChange={(e) => setApiSecret(e.target.value)}
            disabled={formState === "testing" || formState === "success"}
            placeholder="Paste your secret here"
            className="w-full rounded-md border border-border bg-bg-primary px-3 py-2.5 font-mono text-sm text-text-primary placeholder:text-text-muted/50 focus:border-accent-blue focus:outline-none focus:ring-1 focus:ring-accent-blue disabled:opacity-50"
          />
        </div>
      </div>

      {/* Status messages */}
      {formState === "success" && (
        <div className="flex items-center gap-2 rounded-md border border-accent-green/30 bg-accent-green/10 px-4 py-3">
          <CheckCircle2 className="h-5 w-5 shrink-0 text-accent-green" />
          <span className="text-sm text-accent-green">
            Connected! Market data flowing...
          </span>
        </div>
      )}

      {formState === "error" && (
        <div className="flex items-center gap-2 rounded-md border border-accent-red/30 bg-accent-red/10 px-4 py-3">
          <AlertCircle className="h-5 w-5 shrink-0 text-accent-red" />
          <span className="text-sm text-accent-red">
            {errorMessage}
          </span>
        </div>
      )}

      {/* Action buttons */}
      <div className="flex items-center gap-3 pt-2">
        <button
          type="button"
          onClick={onBack}
          disabled={formState === "testing"}
          className="flex items-center gap-1.5 rounded-md px-4 py-2.5 text-sm font-medium text-text-muted transition-colors hover:text-text-primary disabled:opacity-50"
        >
          <ArrowLeft className="h-4 w-4" />
          Back
        </button>

        <button
          type="button"
          onClick={handleTestConnection}
          disabled={!canSubmit || formState === "testing" || formState === "success"}
          className="flex flex-1 items-center justify-center gap-2 rounded-md bg-accent-blue px-6 py-2.5 font-mono text-sm font-semibold text-white transition-all hover:bg-accent-blue/90 focus:outline-none focus:ring-2 focus:ring-accent-blue/50 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {formState === "testing" && (
            <Loader2 className="h-4 w-4 animate-spin" />
          )}
          {formState === "testing"
            ? "Testing Connection..."
            : formState === "error"
              ? "Retry Connection"
              : "Test Connection"}
        </button>
      </div>
    </div>
  );
}

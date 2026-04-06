import { useEffect, useState } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router";
import { TerminalLayout } from "./components/TerminalLayout";
import { PortfolioView } from "./views/PortfolioView";
import { BlotterView } from "./views/BlotterView";
import { RiskDashboard } from "./views/RiskDashboard";
import { LiquidityNetwork } from "./views/LiquidityNetwork";
import { OptimizerView } from "./views/OptimizerView";
import { InsightsPanel } from "./views/InsightsPanel";
import { OnboardingView } from "./views/OnboardingView";
import { fetchOnboardingStatus } from "./api/rest";
import { initializeStreams } from "./api/ws";
import { useInsightStore } from "./stores/insightStore";
import { useOrderStore } from "./stores/orderStore";
import { usePositionStore } from "./stores/positionStore";
import { useVenueStore } from "./stores/venueStore";

function AppRoutes() {
  const [checking, setChecking] = useState(true);
  const [needsOnboarding, setNeedsOnboarding] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function checkFirstRun() {
      try {
        const completed = await fetchOnboardingStatus();
        if (!cancelled) {
          setNeedsOnboarding(!completed);
        }
      } catch {
        // API unreachable — skip onboarding rather than trapping the user
        // in the onboarding flow when the gateway is temporarily down.
        if (!cancelled) {
          setNeedsOnboarding(false);
        }
      } finally {
        if (!cancelled) {
          setChecking(false);
        }
      }
    }

    checkFirstRun();
    return () => {
      cancelled = true;
    };
  }, []);

  if (checking) {
    return (
      <div className="flex h-screen items-center justify-center bg-bg-primary">
        <div className="flex flex-col items-center gap-3">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-accent-blue border-t-transparent" />
          <span className="text-xs text-text-muted">Initializing...</span>
        </div>
      </div>
    );
  }

  if (needsOnboarding) {
    return (
      <Routes>
        <Route path="/onboarding" element={<OnboardingView onComplete={() => setNeedsOnboarding(false)} />} />
        <Route path="*" element={<Navigate to="/onboarding" replace />} />
      </Routes>
    );
  }

  return (
    <Routes>
      <Route element={<TerminalLayout />}>
        <Route index element={<BlotterView />} />
        <Route path="portfolio" element={<PortfolioView />} />
        <Route path="risk" element={<RiskDashboard />} />
        <Route path="venues" element={<LiquidityNetwork />} />
        <Route path="optimizer" element={<OptimizerView />} />
        <Route path="insights" element={<InsightsPanel />} />
      </Route>
    </Routes>
  );
}

export function App() {
  const { applyAnomalyAlert } = useInsightStore();
  const orderApplyUpdate = useOrderStore((s) => s.applyUpdate);
  const orderLoadOrders = useOrderStore((s) => s.loadOrders);
  const positionApplyUpdate = usePositionStore((s) => s.applyUpdate);
  const venueApplyUpdate = useVenueStore((s) => s.applyUpdate);

  useEffect(() => {
    // Load initial orders at app level so they persist across view transitions
    orderLoadOrders();

    const cleanup = initializeStreams({
      onOrderUpdate: (update) => orderApplyUpdate(update),
      onPositionUpdate: (update) => positionApplyUpdate(update),
      onVenueUpdate: (update) => venueApplyUpdate(update),
      onAnomalyAlert: applyAnomalyAlert,
    });
    return cleanup;
  }, [applyAnomalyAlert, orderApplyUpdate, orderLoadOrders, positionApplyUpdate, venueApplyUpdate]);

  return (
    <BrowserRouter>
      <AppRoutes />
    </BrowserRouter>
  );
}

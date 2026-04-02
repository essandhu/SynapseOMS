import { useEffect, useState } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router";
import { TerminalLayout } from "./components/TerminalLayout";
import { PortfolioView } from "./views/PortfolioView";
import { BlotterView } from "./views/BlotterView";
import { RiskDashboard } from "./views/RiskDashboard";
import { LiquidityNetwork } from "./views/LiquidityNetwork";
import { OptimizerView } from "./views/OptimizerView";
import { OnboardingView } from "./views/OnboardingView";
import { fetchVenues } from "./api/rest";

function AppRoutes() {
  const [checking, setChecking] = useState(true);
  const [needsOnboarding, setNeedsOnboarding] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function checkFirstRun() {
      try {
        const venues = await fetchVenues();
        const hasConnected = venues.some(
          (v) => v.status === "connected" || v.hasCredentials,
        );
        if (!cancelled) {
          setNeedsOnboarding(!hasConnected);
        }
      } catch {
        // If the API is unreachable, assume first run
        if (!cancelled) {
          setNeedsOnboarding(true);
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
          <span className="font-mono text-xs text-text-muted">Initializing...</span>
        </div>
      </div>
    );
  }

  return (
    <Routes>
      <Route path="/onboarding" element={<OnboardingView />} />
      <Route element={<TerminalLayout />}>
        <Route index element={<BlotterView />} />
        <Route path="portfolio" element={<PortfolioView />} />
        <Route path="risk" element={<RiskDashboard />} />
        <Route path="venues" element={<LiquidityNetwork />} />
        <Route path="optimizer" element={<OptimizerView />} />
      </Route>
      {needsOnboarding && (
        <Route path="*" element={<Navigate to="/onboarding" replace />} />
      )}
    </Routes>
  );
}

export function App() {
  return (
    <BrowserRouter>
      <AppRoutes />
    </BrowserRouter>
  );
}

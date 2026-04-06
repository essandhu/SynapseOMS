import { NavLink, Outlet } from "react-router";
import { useInsightStore } from "../stores/insightStore";
import { useThemeStore } from "../stores/themeStore";

const NAV_TABS = [
  { label: "Blotter", to: "/" },
  { label: "Portfolio", to: "/portfolio" },
  { label: "Risk", to: "/risk" },
  { label: "Venues", to: "/venues" },
  { label: "Optimizer", to: "/optimizer" },
  { label: "Insights", to: "/insights" },
] as const;

function ThemeToggle() {
  const { mode, toggle } = useThemeStore();
  const isDark = mode === "dark";

  return (
    <button
      onClick={toggle}
      className="rounded-lg p-1.5 text-text-muted transition-colors hover:bg-bg-tertiary hover:text-text-primary"
      aria-label={isDark ? "Switch to light mode" : "Switch to dark mode"}
      title={isDark ? "Switch to light mode" : "Switch to dark mode"}
    >
      {isDark ? (
        <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M12 3v2.25m6.364.386l-1.591 1.591M21 12h-2.25m-.386 6.364l-1.591-1.591M12 18.75V21m-4.773-4.227l-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0z" />
        </svg>
      ) : (
        <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M21.752 15.002A9.718 9.718 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z" />
        </svg>
      )}
    </button>
  );
}

export function TerminalLayout() {
  const alertCount = useInsightStore((s) => s.unacknowledgedCount());

  return (
    <div className="relative flex h-screen flex-col bg-bg-primary font-sans text-text-primary">
      {/* Top bar */}
      <header className="z-10 flex items-center border-b border-border px-3 py-2">
        <h1 className="mr-8 text-sm font-semibold tracking-wider text-accent-blue">
          SynapseOMS
        </h1>
        <nav className="flex gap-1">
          {NAV_TABS.map((tab) => (
            <NavLink
              key={tab.to}
              to={tab.to}
              end={tab.to === "/"}
              className={({ isActive }) =>
                [
                  "border-b-2 px-3 py-1.5 text-xs font-medium transition-colors",
                  isActive
                    ? "border-accent-blue text-text-primary"
                    : "border-transparent text-text-muted hover:text-text-secondary",
                ].join(" ")
              }
            >
              {tab.label}
              {tab.to === "/insights" && alertCount > 0 && (
                <span
                  className="ml-1.5 inline-flex h-4 min-w-[16px] items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-bold text-white"
                  data-testid="nav-alert-badge"
                >
                  {alertCount}
                </span>
              )}
            </NavLink>
          ))}
        </nav>
        <div className="ml-auto">
          <ThemeToggle />
        </div>
      </header>

      {/* Main content */}
      <main className="z-10 min-h-0 flex-1 overflow-auto p-3">
        <Outlet />
      </main>

      {/* Bottom status bar */}
      <footer className="z-10 flex items-center border-t border-border px-3 py-1">
        <span className="flex items-center gap-2 text-xs text-text-muted">
          <span className="inline-block h-2 w-2 animate-status-pulse rounded-full bg-accent-green" />
          Connected
        </span>
        <span className="ml-auto text-xs text-text-muted">
          SynapseOMS v0.1.0
        </span>
      </footer>
    </div>
  );
}

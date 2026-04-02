import { NavLink, Outlet } from "react-router";

const NAV_TABS = [
  { label: "Blotter", to: "/" },
  { label: "Portfolio", to: "/portfolio" },
  { label: "Risk", to: "/risk" },
  { label: "Venues", to: "/venues" },
  { label: "Optimizer", to: "/optimizer" },
] as const;

export function TerminalLayout() {
  return (
    <div className="relative flex h-screen flex-col bg-bg-primary font-sans text-text-primary">
      {/* Scanline overlay */}
      <div
        className="pointer-events-none absolute inset-0 z-50"
        style={{
          background:
            "repeating-linear-gradient(0deg, transparent, transparent 2px, rgba(255,255,255,0.015) 2px, rgba(255,255,255,0.015) 4px)",
        }}
      />

      {/* Top bar */}
      <header className="z-10 flex items-center border-b border-border px-3 py-2">
        <h1 className="mr-8 font-mono text-sm font-semibold tracking-wider text-accent-blue">
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
            </NavLink>
          ))}
        </nav>
      </header>

      {/* Main content */}
      <main className="z-10 min-h-0 flex-1 overflow-auto p-3">
        <Outlet />
      </main>

      {/* Bottom status bar */}
      <footer className="z-10 flex items-center border-t border-border px-3 py-1">
        <span className="flex items-center gap-2 font-mono text-xs text-text-muted">
          <span className="inline-block h-2 w-2 animate-status-pulse rounded-full bg-accent-green" />
          Connected
        </span>
        <span className="ml-auto font-mono text-xs text-text-muted">
          SynapseOMS v0.1.0
        </span>
      </footer>
    </div>
  );
}

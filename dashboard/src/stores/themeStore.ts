import { create } from "zustand";

export type ThemeMode = "dark" | "light";

interface ThemeState {
  mode: ThemeMode;
  toggle: () => void;
}

function applyMode(mode: ThemeMode) {
  if (typeof document === "undefined") return;
  const root = document.documentElement;
  if (mode === "dark") {
    root.classList.add("dark");
  } else {
    root.classList.remove("dark");
  }
  try {
    localStorage.setItem("theme", mode);
  } catch {
    // localStorage may be unavailable in some environments
  }
}

function getInitialMode(): ThemeMode {
  if (typeof window === "undefined") return "dark";
  try {
    const stored = localStorage.getItem("theme");
    if (stored === "light" || stored === "dark") return stored;
  } catch {
    // localStorage unavailable
  }
  return "dark";
}

export const useThemeStore = create<ThemeState>((set) => {
  const initial = getInitialMode();
  applyMode(initial);

  return {
    mode: initial,
    toggle: () =>
      set((state) => {
        const next: ThemeMode = state.mode === "dark" ? "light" : "dark";
        applyMode(next);
        return { mode: next };
      }),
  };
});

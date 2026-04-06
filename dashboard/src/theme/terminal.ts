import { useThemeStore } from "../stores/themeStore";

export const darkTheme = {
  colors: {
    bg: {
      primary: "#0a0e17",
      secondary: "#111827",
      tertiary: "#1f2937",
    },
    text: {
      primary: "#e5e7eb",
      secondary: "#9ca3af",
      muted: "#6b7280",
    },
    accent: {
      blue: "#7132f5",
      green: "#22c55e",
      red: "#ef4444",
      yellow: "#eab308",
      purple: "#7132f5",
    },
    border: "#374151",
  },
  fonts: {
    mono: "'IBM Plex Sans', system-ui, sans-serif",
    sans: "'IBM Plex Sans', system-ui, sans-serif",
  },
} as const;

export const lightTheme = {
  colors: {
    bg: {
      primary: "#ffffff",
      secondary: "#f8f9fa",
      tertiary: "#f0f1f3",
    },
    text: {
      primary: "#101114",
      secondary: "#686b82",
      muted: "#9497a9",
    },
    accent: {
      blue: "#7132f5",
      green: "#149e61",
      red: "#ef4444",
      yellow: "#eab308",
      purple: "#7132f5",
    },
    border: "#dedee5",
  },
  fonts: {
    mono: "'IBM Plex Sans', system-ui, sans-serif",
    sans: "'IBM Plex Sans', system-ui, sans-serif",
  },
} as const;

export interface TerminalTheme {
  readonly colors: {
    readonly bg: { readonly primary: string; readonly secondary: string; readonly tertiary: string };
    readonly text: { readonly primary: string; readonly secondary: string; readonly muted: string };
    readonly accent: { readonly blue: string; readonly green: string; readonly red: string; readonly yellow: string; readonly purple: string };
    readonly border: string;
  };
  readonly fonts: { readonly mono: string; readonly sans: string };
}

/** Returns the active theme colors based on current dark/light mode */
export function useThemeColors(): TerminalTheme {
  const mode = useThemeStore((s) => s.mode);
  return mode === "dark" ? darkTheme : lightTheme;
}

/** Default theme for use outside React (e.g. tests, static contexts) */
export const terminalTheme = darkTheme;

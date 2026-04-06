import { describe, it, expect, beforeEach } from "vitest";
import { useThemeStore } from "./themeStore";

describe("themeStore", () => {
  beforeEach(() => {
    // Reset to dark (default) before each test
    useThemeStore.setState({ mode: "dark" });
    document.documentElement.classList.add("dark");
  });

  it("defaults to dark mode", () => {
    expect(useThemeStore.getState().mode).toBe("dark");
  });

  it("toggle switches from dark to light", () => {
    useThemeStore.getState().toggle();
    expect(useThemeStore.getState().mode).toBe("light");
  });

  it("toggle switches from light back to dark", () => {
    useThemeStore.getState().toggle();
    useThemeStore.getState().toggle();
    expect(useThemeStore.getState().mode).toBe("dark");
  });

  it("adds 'dark' class to documentElement in dark mode", () => {
    useThemeStore.getState().toggle(); // → light
    expect(document.documentElement.classList.contains("dark")).toBe(false);

    useThemeStore.getState().toggle(); // → dark
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });

  it("removes 'dark' class from documentElement in light mode", () => {
    useThemeStore.getState().toggle(); // → light
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });
});

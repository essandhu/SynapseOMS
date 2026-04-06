import { describe, it, expect, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useThemeStore } from "../stores/themeStore";

// TerminalLayout renders the ThemeToggle internally but also requires
// react-router (Outlet, NavLink). Instead of importing the whole layout,
// we extract the toggle via its accessible aria-label which is stable.
// We render TerminalLayout inside a MemoryRouter so the router context exists.

import { TerminalLayout } from "./TerminalLayout";
import { MemoryRouter } from "react-router";

function renderToggle() {
  return render(
    <MemoryRouter>
      <TerminalLayout />
    </MemoryRouter>,
  );
}

describe("ThemeToggle", () => {
  beforeEach(() => {
    useThemeStore.setState({ mode: "dark" });
    document.documentElement.classList.add("dark");
  });

  it("renders with 'Switch to light mode' label in dark mode", () => {
    renderToggle();
    expect(
      screen.getByRole("button", { name: "Switch to light mode" }),
    ).toBeInTheDocument();
  });

  it("renders with 'Switch to dark mode' label in light mode", () => {
    useThemeStore.setState({ mode: "light" });
    renderToggle();
    expect(
      screen.getByRole("button", { name: "Switch to dark mode" }),
    ).toBeInTheDocument();
  });

  it("toggles from dark to light on click", async () => {
    const user = userEvent.setup();
    renderToggle();

    const btn = screen.getByRole("button", { name: "Switch to light mode" });
    await user.click(btn);

    expect(useThemeStore.getState().mode).toBe("light");
    expect(
      screen.getByRole("button", { name: "Switch to dark mode" }),
    ).toBeInTheDocument();
  });

  it("toggles from light back to dark on click", async () => {
    useThemeStore.setState({ mode: "light" });
    const user = userEvent.setup();
    renderToggle();

    const btn = screen.getByRole("button", { name: "Switch to dark mode" });
    await user.click(btn);

    expect(useThemeStore.getState().mode).toBe("dark");
    expect(
      screen.getByRole("button", { name: "Switch to light mode" }),
    ).toBeInTheDocument();
  });
});

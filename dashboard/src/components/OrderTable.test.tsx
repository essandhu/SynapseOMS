import { describe, it, expect, vi, beforeEach } from "vitest";
import { render } from "@testing-library/react";
import { OrderTable, buildColumnDefs } from "./OrderTable";
import { makeOrder } from "../mocks/data";
import { useThemeStore } from "../stores/themeStore";
import type { ColDef } from "ag-grid-community";
import type { Order } from "../api/types";

/** Helper: find the ag-Grid wrapper regardless of current theme */
function findGridWrapper(container: HTMLElement) {
  return container.querySelector(
    ".ag-theme-alpine-dark, .ag-theme-alpine:not(.ag-theme-alpine-dark)",
  );
}

describe("OrderTable", () => {
  const onCancel = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    // Reset to default dark mode
    useThemeStore.setState({ mode: "dark" });
  });

  // --- Rendering ---

  it("renders without crashing with empty orders", () => {
    const { container } = render(
      <OrderTable orders={[]} onCancel={onCancel} />,
    );
    expect(findGridWrapper(container)).toBeTruthy();
  });

  it("renders with orders", () => {
    const orders = [
      makeOrder({ id: "1", status: "new" }),
      makeOrder({ id: "2", status: "filled" }),
    ];
    const { container } = render(
      <OrderTable orders={orders} onCancel={onCancel} />,
    );
    expect(findGridWrapper(container)).toBeTruthy();
  });

  // --- Theme-aware grid class ---

  it("uses ag-theme-alpine-dark in dark mode", () => {
    useThemeStore.setState({ mode: "dark" });
    const { container } = render(
      <OrderTable orders={[]} onCancel={onCancel} />,
    );
    expect(container.querySelector(".ag-theme-alpine-dark")).toBeTruthy();
  });

  it("uses ag-theme-alpine in light mode", () => {
    useThemeStore.setState({ mode: "light" });
    const { container } = render(
      <OrderTable orders={[]} onCancel={onCancel} />,
    );
    const wrapper = findGridWrapper(container) as HTMLElement;
    expect(wrapper).toBeTruthy();
    expect(wrapper.classList.contains("ag-theme-alpine")).toBe(true);
    expect(wrapper.classList.contains("ag-theme-alpine-dark")).toBe(false);
  });

  // --- Layout: scrolling and sizing ---

  it("uses normal domLayout for scrollable container", () => {
    const { container } = render(
      <OrderTable orders={[]} onCancel={onCancel} />,
    );
    const gridWrapper = findGridWrapper(container) as HTMLElement;
    expect(gridWrapper.style.height).toBe("100%");
    expect(gridWrapper.style.width).toBe("100%");
  });

  it("does not use autoHeight domLayout", () => {
    const { container } = render(
      <OrderTable orders={[]} onCancel={onCancel} />,
    );
    const autoHeight = container.querySelector(".ag-layout-auto-height");
    expect(autoHeight).toBeNull();
  });

  it("does not pin any columns to the right", () => {
    const { container } = render(
      <OrderTable orders={[]} onCancel={onCancel} />,
    );
    const pinnedContainer = container.querySelector(
      ".ag-pinned-right-cols-container",
    );
    if (pinnedContainer) {
      const pinnedCols = pinnedContainer.querySelectorAll(".ag-cell");
      expect(pinnedCols).toHaveLength(0);
    }
  });
});

// --- Column definition tests (unit tests, no DOM needed) ---
// These test buildColumnDefs() directly to guard against regressions
// in column order, naming, alignment, minWidth, and sizing behavior.

describe("buildColumnDefs", () => {
  const onCancel = vi.fn();
  let cols: ColDef<Order>[];

  beforeEach(() => {
    cols = buildColumnDefs(onCancel);
  });

  it("returns exactly 11 columns", () => {
    expect(cols).toHaveLength(11);
  });

  it("has columns in correct order with correct headers", () => {
    const headers = cols.map((c) => c.headerName);
    expect(headers).toEqual([
      "Time",
      "Instrument",
      "Side",
      "Type",
      "Qty",
      "Price",
      "Filled",
      "Avg Price",
      "Status",
      "Venue",
      "Action",
    ]);
  });

  it("every column has a minWidth to prevent zero-width compression", () => {
    for (const col of cols) {
      expect(col.minWidth, `Column "${col.headerName}" missing minWidth`).toBeGreaterThan(0);
    }
  });

  it("numeric columns use rightAligned type", () => {
    const rightAligned = ["Qty", "Price", "Filled", "Avg Price"];
    for (const name of rightAligned) {
      const col = cols.find((c) => c.headerName === name);
      expect(col?.type, `Column "${name}" should be rightAligned`).toBe("rightAligned");
    }
  });

  it("text columns do not use rightAligned type", () => {
    const textCols = ["Time", "Instrument", "Side", "Type", "Status", "Venue", "Action"];
    for (const name of textCols) {
      const col = cols.find((c) => c.headerName === name);
      expect(col?.type, `Column "${name}" should not be rightAligned`).toBeUndefined();
    }
  });

  it("Venue column uses flex to absorb remaining space", () => {
    const venue = cols.find((c) => c.headerName === "Venue");
    expect(venue?.flex).toBe(1);
  });

  it("Action column is fixed-width and excluded from sizeColumnsToFit", () => {
    const action = cols.find((c) => c.headerName === "Action");
    expect(action?.width).toBe(80);
    expect(action?.minWidth).toBe(80);
    expect(action?.maxWidth).toBe(80);
    expect(action?.suppressSizeToFit).toBe(true);
    expect(action?.resizable).toBe(false);
    expect(action?.sortable).toBe(false);
  });

  it("Action column is not pinned", () => {
    const action = cols.find((c) => c.headerName === "Action");
    expect(action?.pinned).toBeUndefined();
  });

  it("no column uses display:flex in cellStyle", () => {
    for (const col of cols) {
      const style = col.cellStyle;
      if (style && typeof style === "object" && "display" in style) {
        expect(
          (style as Record<string, unknown>).display,
          `Column "${col.headerName}" should not use display:flex`,
        ).not.toBe("flex");
      }
    }
  });
});

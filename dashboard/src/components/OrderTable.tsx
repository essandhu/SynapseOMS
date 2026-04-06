import { useCallback, useMemo, useRef } from "react";
import { AgGridReact } from "ag-grid-react";
import { ModuleRegistry, ClientSideRowModelModule } from "ag-grid-community";
import type { ColDef, ICellRendererParams, GridReadyEvent, GridSizeChangedEvent, ColumnResizedEvent, GridApi, CellStyle } from "ag-grid-community";
import "ag-grid-community/styles/ag-grid.css";
import "ag-grid-community/styles/ag-theme-alpine.css";
import type { Order, OrderStatus } from "../api/types";
import { terminalTheme } from "../theme/terminal";

// Register required modules
ModuleRegistry.registerModules([ClientSideRowModelModule]);

export interface OrderTableProps {
  orders: Order[];
  onCancel: (orderId: string) => void;
}

const TERMINAL_STATUSES = new Set<OrderStatus>(["filled", "canceled", "rejected"]);

/** Status badge color mapping — 16% opacity backgrounds with darker text */
function statusBadge(status: OrderStatus): { bg: string; text: string; label: string } {
  switch (status) {
    case "new":
      return { bg: "rgba(113,50,245,0.16)", text: "#7132f5", label: "New" };
    case "acknowledged":
      return { bg: "rgba(234,179,8,0.16)", text: "#eab308", label: "Ack" };
    case "partially_filled":
      return { bg: "rgba(234,179,8,0.16)", text: "#eab308", label: "Partial" };
    case "filled":
      return { bg: "rgba(20,158,97,0.16)", text: "#149e61", label: "Filled" };
    case "canceled":
      return { bg: "rgba(239,68,68,0.16)", text: "#ef4444", label: "Canceled" };
    case "rejected":
      return { bg: "rgba(239,68,68,0.16)", text: "#ef4444", label: "Rejected" };
    default:
      return { bg: "rgba(148,151,169,0.16)", text: "#9497a9", label: status };
  }
}

/** Format decimal string for display */
function formatDecimal(value: string, decimals = 2): string {
  const num = Number(value);
  if (isNaN(num)) return value;
  return num.toLocaleString(undefined, {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
}

/** Format time from ISO string */
function formatTime(iso: string): string {
  try {
    const d = new Date(iso);
    return d.toLocaleTimeString(undefined, {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    });
  } catch {
    return iso;
  }
}

/** Build column definitions for the order blotter grid. Exported for testing. */
export function buildColumnDefs(
  onCancel: (orderId: string) => void,
): ColDef<Order>[] {
  return [
      {
        headerName: "Time",
        field: "createdAt",
        minWidth: 90,
        width: 100,
        valueFormatter: (p) => (p.value ? formatTime(p.value) : ""),
        sort: "desc" as const,
      },
      {
        headerName: "Instrument",
        field: "instrumentId",
        minWidth: 100,
        width: 140,
        cellStyle: { color: terminalTheme.colors.text.primary, fontWeight: "600" },
      },
      {
        headerName: "Side",
        field: "side",
        minWidth: 60,
        width: 70,
        cellRenderer: (params: ICellRendererParams<Order>) => {
          if (!params.value) return null;
          const isBuy = params.value === "buy";
          return (
            <span
              style={{
                color: isBuy
                  ? terminalTheme.colors.accent.green
                  : terminalTheme.colors.accent.red,
                fontWeight: 700,
                textTransform: "uppercase",
              }}
            >
              {params.value}
            </span>
          );
        },
      },
      {
        headerName: "Type",
        field: "type",
        minWidth: 75,
        width: 80,
        valueFormatter: (p) => {
          if (!p.value) return "";
          return p.value === "stop_limit"
            ? "Stop Limit"
            : p.value.charAt(0).toUpperCase() + p.value.slice(1);
        },
      },
      {
        headerName: "Qty",
        field: "quantity",
        minWidth: 80,
        width: 90,
        type: "rightAligned",
        valueFormatter: (p) => (p.value ? formatDecimal(p.value, 4) : ""),
      },
      {
        headerName: "Price",
        field: "price",
        minWidth: 80,
        width: 100,
        type: "rightAligned",
        cellRenderer: (params: ICellRendererParams<Order>) => {
          if (!params.data) return null;
          if (params.data.type === "market") {
            return (
              <span style={{ color: terminalTheme.colors.text.muted, fontStyle: "italic" }}>
                MKT
              </span>
            );
          }
          return <span>{formatDecimal(params.value ?? "0")}</span>;
        },
      },
      {
        headerName: "Filled",
        field: "filledQuantity",
        minWidth: 80,
        width: 90,
        type: "rightAligned",
        valueFormatter: (p) => (p.value ? formatDecimal(p.value, 4) : ""),
      },
      {
        headerName: "Avg Price",
        field: "averagePrice",
        minWidth: 80,
        width: 100,
        type: "rightAligned",
        valueFormatter: (p) => {
          if (!p.value || Number(p.value) === 0) return "-";
          return formatDecimal(p.value);
        },
      },
      {
        headerName: "Status",
        field: "status",
        minWidth: 85,
        width: 100,
        cellRenderer: (params: ICellRendererParams<Order>) => {
          if (!params.value) return null;
          const badge = statusBadge(params.value as OrderStatus);
          return (
            <span
              style={{
                backgroundColor: badge.bg,
                color: badge.text,
                padding: "2px 8px",
                borderRadius: "9999px",
                fontSize: "11px",
                fontWeight: 600,
                lineHeight: "1.5",
              }}
            >
              {badge.label}
            </span>
          );
        },
      },
      {
        headerName: "Venue",
        field: "venueId",
        minWidth: 80,
        flex: 1,
        cellStyle: { color: terminalTheme.colors.text.secondary },
      },
      {
        headerName: "Action",
        field: "id",
        width: 80,
        maxWidth: 80,
        minWidth: 80,
        sortable: false,
        resizable: false,
        filter: false,
        suppressSizeToFit: true,
        cellRenderer: (params: ICellRendererParams<Order>) => {
          if (!params.data || TERMINAL_STATUSES.has(params.data.status)) {
            return null;
          }
          return (
            <button
              onClick={() => onCancel(params.data!.id)}
              style={{
                background: "transparent",
                border: "none",
                color: terminalTheme.colors.text.muted,
                padding: "0",
                cursor: "pointer",
                fontSize: "11px",
                fontFamily: terminalTheme.fonts.sans,
                transition: "color 0.15s",
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.color = terminalTheme.colors.accent.red;
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.color = terminalTheme.colors.text.muted;
              }}
            >
              Cancel
            </button>
          );
        },
      },
  ];
}

export function OrderTable({ orders, onCancel }: OrderTableProps) {
  const gridRef = useRef<AgGridReact<Order>>(null);
  const gridApiRef = useRef<GridApi<Order> | null>(null);

  const onGridReady = useCallback((params: GridReadyEvent<Order>) => {
    gridApiRef.current = params.api;
    params.api.sizeColumnsToFit();
  }, []);

  const onGridSizeChanged = useCallback((params: GridSizeChangedEvent<Order>) => {
    params.api.sizeColumnsToFit();
  }, []);

  const onColumnResized = useCallback((params: ColumnResizedEvent<Order>) => {
    if (params.finished && params.source === "uiColumnResized") {
      params.api.sizeColumnsToFit();
    }
  }, []);

  const columnDefs = useMemo(
    () => buildColumnDefs(onCancel),
    [onCancel],
  );

  const defaultColDef = useMemo<ColDef>(
    () => ({
      sortable: true,
      resizable: true,
      suppressMovable: true,
      cellStyle: {
        fontFamily: terminalTheme.fonts.sans,
        fontSize: "12px",
        color: terminalTheme.colors.text.secondary,
      },
    }),
    [],
  );

  const getRowId = useCallback((params: { data: Order }) => params.data.id, []);

  return (
    <div
      className="ag-theme-alpine"
      style={{
        width: "100%",
        height: "100%",
        minHeight: 300,
        "--ag-background-color": terminalTheme.colors.bg.primary,
        "--ag-odd-row-background-color": terminalTheme.colors.bg.secondary,
        "--ag-header-background-color": terminalTheme.colors.bg.tertiary,
        "--ag-header-foreground-color": terminalTheme.colors.text.muted,
        "--ag-foreground-color": terminalTheme.colors.text.secondary,
        "--ag-border-color": terminalTheme.colors.border,
        "--ag-row-border-color": `${terminalTheme.colors.border}80`,
        "--ag-header-column-separator-color": terminalTheme.colors.border,
        "--ag-font-family": terminalTheme.fonts.sans,
        "--ag-font-size": "12px",
        "--ag-row-height": "36px",
        "--ag-header-height": "36px",
        "--ag-selected-row-background-color": `${terminalTheme.colors.accent.blue}15`,
        "--ag-row-hover-color": `${terminalTheme.colors.bg.tertiary}80`,
      } as React.CSSProperties}
    >
      <AgGridReact<Order>
        ref={gridRef}
        rowData={orders}
        columnDefs={columnDefs}
        defaultColDef={defaultColDef}
        getRowId={getRowId}
        onGridReady={onGridReady}
        onGridSizeChanged={onGridSizeChanged}
        onColumnResized={onColumnResized}
        animateRows={true}
        suppressCellFocus={true}
        noRowsOverlayComponent={() => (
          <div
            style={{
              fontFamily: terminalTheme.fonts.sans,
              fontSize: "12px",
              color: terminalTheme.colors.text.muted,
              padding: "40px 0",
            }}
          >
            No orders
          </div>
        )}
      />
    </div>
  );
}

import { useCallback, useMemo, useRef } from "react";
import { AgGridReact } from "ag-grid-react";
import { ModuleRegistry, ClientSideRowModelModule } from "ag-grid-community";
import type { ColDef, ICellRendererParams, GridReadyEvent, GridApi, CellStyle } from "ag-grid-community";
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

/** Status badge color mapping */
function statusBadge(status: OrderStatus): { bg: string; text: string; label: string } {
  switch (status) {
    case "new":
      return { bg: "rgba(59,130,246,0.2)", text: "#3b82f6", label: "New" };
    case "acknowledged":
      return { bg: "rgba(234,179,8,0.2)", text: "#eab308", label: "Ack" };
    case "partially_filled":
      return { bg: "rgba(234,179,8,0.2)", text: "#eab308", label: "Partial" };
    case "filled":
      return { bg: "rgba(34,197,94,0.2)", text: "#22c55e", label: "Filled" };
    case "canceled":
      return { bg: "rgba(239,68,68,0.2)", text: "#ef4444", label: "Canceled" };
    case "rejected":
      return { bg: "rgba(239,68,68,0.2)", text: "#ef4444", label: "Rejected" };
    default:
      return { bg: "rgba(107,114,128,0.2)", text: "#6b7280", label: status };
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

export function OrderTable({ orders, onCancel }: OrderTableProps) {
  const gridRef = useRef<AgGridReact<Order>>(null);
  const gridApiRef = useRef<GridApi<Order> | null>(null);

  const onGridReady = useCallback((params: GridReadyEvent<Order>) => {
    gridApiRef.current = params.api;
    params.api.sizeColumnsToFit();
  }, []);

  const columnDefs = useMemo(
    (): ColDef<Order>[] => [
      {
        headerName: "Time",
        field: "createdAt",
        width: 100,
        valueFormatter: (p) => (p.value ? formatTime(p.value) : ""),
        sort: "desc" as const,
      },
      {
        headerName: "Instrument",
        field: "instrumentId",
        width: 140,
        cellStyle: { color: terminalTheme.colors.text.primary, fontWeight: "600" },
      },
      {
        headerName: "Side",
        field: "side",
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
        width: 90,
        type: "rightAligned",
        valueFormatter: (p) => (p.value ? formatDecimal(p.value, 4) : ""),
      },
      {
        headerName: "Price",
        field: "price",
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
        width: 90,
        type: "rightAligned",
        valueFormatter: (p) => (p.value ? formatDecimal(p.value, 4) : ""),
      },
      {
        headerName: "Avg Price",
        field: "averagePrice",
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
        width: 110,
        cellStyle: { color: terminalTheme.colors.text.secondary },
      },
      {
        headerName: "",
        field: "id",
        width: 80,
        sortable: false,
        filter: false,
        cellRenderer: (params: ICellRendererParams<Order>) => {
          if (!params.data || TERMINAL_STATUSES.has(params.data.status)) {
            return null;
          }
          return (
            <button
              onClick={() => onCancel(params.data!.id)}
              style={{
                background: "transparent",
                border: `1px solid ${terminalTheme.colors.border}`,
                color: terminalTheme.colors.text.muted,
                padding: "2px 8px",
                borderRadius: "4px",
                cursor: "pointer",
                fontSize: "11px",
                fontFamily: terminalTheme.fonts.mono,
                transition: "all 0.15s",
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.borderColor = terminalTheme.colors.accent.red;
                e.currentTarget.style.color = terminalTheme.colors.accent.red;
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.borderColor = terminalTheme.colors.border;
                e.currentTarget.style.color = terminalTheme.colors.text.muted;
              }}
            >
              Cancel
            </button>
          );
        },
      },
    ],
    [onCancel],
  );

  const defaultColDef = useMemo<ColDef>(
    () => ({
      sortable: true,
      resizable: true,
      suppressMovable: true,
      cellStyle: {
        fontFamily: terminalTheme.fonts.mono,
        fontSize: "12px",
        color: terminalTheme.colors.text.secondary,
        display: "flex",
        alignItems: "center",
      },
    }),
    [],
  );

  const getRowId = useCallback((params: { data: Order }) => params.data.id, []);

  return (
    <div
      className="ag-theme-alpine-dark"
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
        "--ag-font-family": terminalTheme.fonts.mono,
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
        animateRows={true}
        suppressCellFocus={true}
        domLayout="autoHeight"
        noRowsOverlayComponent={() => (
          <div
            style={{
              fontFamily: terminalTheme.fonts.mono,
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

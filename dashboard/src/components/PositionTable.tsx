import type { Position } from "../api/types";

export interface PositionTableProps {
  positions: Position[];
  /** Total NAV for computing % of NAV column. If omitted the column is hidden. */
  totalNav?: number;
}

/** Format a decimal string to a fixed number of decimal places */
function formatDecimal(value: string, decimals = 2): string {
  const num = Number(value);
  if (isNaN(num)) return value;
  return num.toLocaleString(undefined, {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
}

/** Return color class based on value sign */
function pnlColor(value: string): string {
  const num = Number(value);
  if (num > 0) return "text-accent-green";
  if (num < 0) return "text-accent-red";
  return "text-text-muted";
}

/** Format quantity with sign */
function formatQuantity(value: string): string {
  const num = Number(value);
  if (isNaN(num)) return value;
  if (num > 0) return `+${formatDecimal(value, 4)}`;
  return formatDecimal(value, 4);
}

const BASE_COLUMNS = [
  "Instrument",
  "Venue",
  "Qty",
  "Avg Cost",
  "Market Price",
  "Unrealized P&L",
  "Realized P&L",
  "Asset Class",
] as const;

export function PositionTable({ positions, totalNav }: PositionTableProps) {
  const showNavWeight = totalNav !== undefined && totalNav > 0;
  const columns = showNavWeight
    ? [...BASE_COLUMNS, "% of NAV" as const]
    : BASE_COLUMNS;
  if (positions.length === 0) {
    return (
      <div className="flex items-center justify-center py-12 text-xs text-text-muted">
        No positions
      </div>
    );
  }

  return (
    <div className="overflow-auto">
      <table className="w-full border-collapse text-xs">
        <thead>
          <tr className="border-b border-border">
            {columns.map((col) => (
              <th
                key={col}
                className="px-3 py-2 text-left font-medium uppercase tracking-wider text-text-muted"
              >
                {col}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {positions.map((pos) => {
            const key = `${pos.instrumentId}-${pos.venueId}`;
            const qtyNum = Number(pos.quantity);
            return (
              <tr
                key={key}
                className="border-b border-border/50 transition-colors hover:bg-bg-tertiary/50"
              >
                <td className="px-3 py-2 font-semibold text-text-primary">
                  {pos.instrumentId}
                </td>
                <td className="px-3 py-2 text-text-secondary">{pos.venueId}</td>
                <td className={`px-3 py-2 ${qtyNum >= 0 ? "text-accent-green" : "text-accent-red"}`}>
                  {formatQuantity(pos.quantity)}
                </td>
                <td className="px-3 py-2 text-text-secondary">
                  {formatDecimal(pos.averageCost)}
                </td>
                <td className="px-3 py-2 text-text-primary">
                  {formatDecimal(pos.marketPrice)}
                </td>
                <td className={`px-3 py-2 ${pnlColor(pos.unrealizedPnl)}`}>
                  {formatDecimal(pos.unrealizedPnl)}
                </td>
                <td className={`px-3 py-2 ${pnlColor(pos.realizedPnl)}`}>
                  {formatDecimal(pos.realizedPnl)}
                </td>
                <td className="px-3 py-2 text-text-muted uppercase">
                  {pos.assetClass}
                </td>
                {showNavWeight && (
                  <td className="px-3 py-2 text-text-secondary">
                    {(() => {
                      const mktVal = Math.abs(Number(pos.quantity) * Number(pos.marketPrice));
                      const pct = (mktVal / totalNav) * 100;
                      return `${pct.toFixed(1)}%`;
                    })()}
                  </td>
                )}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

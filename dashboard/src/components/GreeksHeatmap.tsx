import { useState, useMemo } from "react";
import * as d3 from "d3";
import type { PortfolioGreeks } from "../api/types";

const GREEK_KEYS = ["delta", "gamma", "vega", "theta", "rho"] as const;
const GREEK_LABELS = ["Delta", "Gamma", "Vega", "Theta", "Rho"] as const;

type GreekKey = (typeof GREEK_KEYS)[number];

export interface GreeksHeatmapProps {
  greeks: PortfolioGreeks | null;
}

interface TooltipInfo {
  instrument: string;
  greek: string;
  value: number;
  x: number;
  y: number;
}

/** Diverging color scale: blue for negative, white at zero, red for positive */
function buildColorScale(maxAbs: number) {
  return d3
    .scaleDiverging<string>()
    .domain([-maxAbs, 0, maxAbs])
    .interpolator((t: number) => d3.interpolateRdBu(1 - t));
}

export function GreeksHeatmap({ greeks }: GreeksHeatmapProps) {
  const [tooltip, setTooltip] = useState<TooltipInfo | null>(null);

  const instruments = useMemo(() => {
    if (!greeks) return [];
    return Object.keys(greeks.byInstrument);
  }, [greeks]);

  const maxAbs = useMemo(() => {
    if (!greeks || instruments.length === 0) return 1;
    let max = 0;
    for (const inst of instruments) {
      const g = greeks.byInstrument[inst];
      for (const key of GREEK_KEYS) {
        max = Math.max(max, Math.abs(g[key]));
      }
    }
    return max || 1;
  }, [greeks, instruments]);

  const colorScale = useMemo(() => buildColorScale(maxAbs), [maxAbs]);

  if (!greeks || instruments.length === 0) {
    return (
      <div className="rounded border border-border border-dashed bg-bg-secondary p-4">
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-text-muted">
          Greeks Heatmap
        </h3>
        <div className="flex h-40 items-center justify-center">
          <span className="text-xs text-text-muted">
            No Greeks data available
          </span>
        </div>
      </div>
    );
  }

  // Layout constants
  const cellW = 72;
  const cellH = 32;
  const labelW = 100;
  const headerH = 28;
  const gap = 2;

  const svgWidth = labelW + GREEK_KEYS.length * (cellW + gap);
  const svgHeight = headerH + instruments.length * (cellH + gap);

  return (
    <div className="rounded border border-border bg-bg-secondary p-4 relative">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
          Greeks Heatmap
        </h3>
        <span className="text-[10px] text-text-muted">
          {new Date(greeks.computedAt).toLocaleTimeString("en-US", {
            hour: "2-digit",
            minute: "2-digit",
            second: "2-digit",
            hour12: false,
          })}
        </span>
      </div>

      <div className="overflow-x-auto">
        <svg
          width={svgWidth}
          height={svgHeight}
          className="block"
          style={{ minWidth: svgWidth }}
        >
          {/* Column headers (Greek names) */}
          {GREEK_LABELS.map((label, ci) => (
            <text
              key={label}
              x={labelW + ci * (cellW + gap) + cellW / 2}
              y={headerH - 8}
              textAnchor="middle"
              className="fill-current text-text-muted"
              style={{ fontSize: 11, fontFamily: "'IBM Plex Sans', sans-serif" }}
            >
              {label}
            </text>
          ))}

          {/* Rows */}
          {instruments.map((inst, ri) => {
            const g = greeks.byInstrument[inst];
            const rowY = headerH + ri * (cellH + gap);

            return (
              <g key={inst}>
                {/* Row label (instrument name) */}
                <text
                  x={labelW - 8}
                  y={rowY + cellH / 2 + 4}
                  textAnchor="end"
                  className="fill-current text-text-muted"
                  style={{ fontSize: 11, fontFamily: "'IBM Plex Sans', sans-serif" }}
                >
                  {inst}
                </text>

                {/* Cells */}
                {GREEK_KEYS.map((key, ci) => {
                  const value = g[key as GreekKey];
                  const cellX = labelW + ci * (cellW + gap);
                  const color = colorScale(value);

                  return (
                    <rect
                      key={key}
                      data-testid="heatmap-cell"
                      x={cellX}
                      y={rowY}
                      width={cellW}
                      height={cellH}
                      rx={3}
                      fill={color}
                      className="cursor-pointer transition-opacity hover:opacity-80"
                      onMouseEnter={(e) => {
                        const svg = e.currentTarget.closest("svg");
                        const rect = svg?.getBoundingClientRect();
                        setTooltip({
                          instrument: inst,
                          greek: GREEK_LABELS[ci],
                          value,
                          x: cellX + cellW / 2,
                          y: rowY,
                        });
                      }}
                      onMouseLeave={() => setTooltip(null)}
                    />
                  );
                })}
              </g>
            );
          })}
        </svg>
      </div>

      {/* Tooltip */}
      {tooltip && (
        <div
          data-testid="heatmap-tooltip"
          className="pointer-events-none absolute z-50 rounded border border-border bg-bg-secondary px-3 py-2 text-xs"
          style={{
            left: tooltip.x,
            top: tooltip.y - 8,
            transform: "translate(-50%, -100%)",
            boxShadow: "rgba(0,0,0,0.03) 0px 4px 24px",
          }}
        >
          <p className="text-text-muted">{tooltip.instrument}</p>
          <p>
            <span className="text-text-muted">{tooltip.greek}: </span>
            <span className="text-text-primary font-medium">
              {tooltip.value}
            </span>
          </p>
        </div>
      )}
    </div>
  );
}

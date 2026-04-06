import { useState, useMemo, useRef, useEffect } from "react";
import * as d3 from "d3";
import type { ConcentrationResult, Position, AssetClass } from "../api/types";

const ASSET_CLASS_COLORS: Record<AssetClass, string> = {
  equity: "#7132f5",
  crypto: "#f59e0b",
  tokenized_security: "#8b5cf6",
  future: "#10b981",
  option: "#ef4444",
};

const DEFAULT_COLOR = "#9497a9";
const CONCENTRATION_THRESHOLD = 25;
const DEFAULT_WIDTH = 500;
const DEFAULT_HEIGHT = 300;

export interface ConcentrationTreemapProps {
  concentration: ConcentrationResult | null;
  positions?: Position[];
  /** Override width (useful for testing; normally auto-detected via ResizeObserver) */
  width?: number;
  /** Override height (useful for testing; normally auto-detected via ResizeObserver) */
  height?: number;
}

interface TreemapNode {
  instrumentId: string;
  concentration: number;
  assetClass: AssetClass | null;
  hasWarning: boolean;
}

interface TooltipInfo {
  instrumentId: string;
  concentration: number;
  assetClass: AssetClass | null;
  x: number;
  y: number;
}

function lookupAssetClass(
  instrumentId: string,
  positions?: Position[],
): AssetClass | null {
  if (!positions) return null;
  const pos = positions.find((p) => p.instrumentId === instrumentId);
  return pos?.assetClass ?? null;
}

function hasWarning(
  instrumentId: string,
  concentration: number,
  warnings: string[],
): boolean {
  if (concentration > CONCENTRATION_THRESHOLD) return true;
  return warnings.some((w) => w.includes(instrumentId));
}

function formatAssetClass(ac: AssetClass | null): string {
  if (!ac) return "Unknown";
  const labels: Record<AssetClass, string> = {
    equity: "Equity",
    crypto: "Crypto",
    tokenized_security: "Tokenized",
    future: "Future",
    option: "Option",
  };
  return labels[ac];
}

export function ConcentrationTreemap({
  concentration,
  positions,
  width: propWidth,
  height: propHeight,
}: ConcentrationTreemapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [tooltip, setTooltip] = useState<TooltipInfo | null>(null);
  const [dimensions, setDimensions] = useState({
    width: propWidth ?? DEFAULT_WIDTH,
    height: propHeight ?? DEFAULT_HEIGHT,
  });

  // Observe container size for responsiveness (skip if explicit size provided)
  useEffect(() => {
    if (propWidth != null && propHeight != null) return;
    const el = containerRef.current;
    if (!el || typeof ResizeObserver === "undefined") return;

    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        if (width > 0 && height > 0) {
          setDimensions({
            width: propWidth ?? Math.floor(width),
            height: propHeight ?? Math.max(Math.floor(height), 200),
          });
        }
      }
    });

    observer.observe(el);
    return () => observer.disconnect();
  }, [propWidth, propHeight]);

  const nodes = useMemo<TreemapNode[]>(() => {
    if (!concentration) return [];
    return Object.entries(concentration.singleName).map(([id, pct]) => ({
      instrumentId: id,
      concentration: pct,
      assetClass: lookupAssetClass(id, positions),
      hasWarning: hasWarning(id, pct, concentration.warnings),
    }));
  }, [concentration, positions]);

  const treemapLayout = useMemo(() => {
    if (nodes.length === 0) return null;

    const root = d3
      .hierarchy({
        children: nodes.map((n) => ({
          ...n,
          value: n.concentration,
        })),
      })
      .sum((d: any) => d.value ?? 0);

    const layout = d3
      .treemap<any>()
      .tile(d3.treemapSquarify)
      .size([dimensions.width, dimensions.height])
      .padding(2)
      .round(true);

    layout(root);
    return root.leaves() as unknown as d3.HierarchyRectangularNode<TreemapNode & { value: number }>[];
  }, [nodes, dimensions]);

  if (
    !concentration ||
    Object.keys(concentration.singleName).length === 0
  ) {
    return (
      <div className="rounded border border-border border-dashed bg-bg-secondary p-4">
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-text-muted">
          Concentration Risk
        </h3>
        <div className="flex h-40 items-center justify-center">
          <span className="text-xs text-text-muted">
            No concentration data available
          </span>
        </div>
      </div>
    );
  }

  return (
    <div className="rounded border border-border bg-bg-secondary p-4 relative">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
          Concentration Risk
        </h3>
        <span className="text-[10px] text-text-muted">
          HHI: {concentration.hhi.toLocaleString()}
        </span>
      </div>

      <div ref={containerRef} className="relative" style={{ minHeight: 200 }}>
        <svg
          width={dimensions.width}
          height={dimensions.height}
          className="block"
        >
          {treemapLayout?.map((leaf) => {
            const d = leaf.data as TreemapNode & { value: number };
            const x = leaf.x0;
            const y = leaf.y0;
            const w = leaf.x1 - leaf.x0;
            const h = leaf.y1 - leaf.y0;
            const color = d.assetClass
              ? ASSET_CLASS_COLORS[d.assetClass]
              : DEFAULT_COLOR;

            const showLabel = w > 40 && h > 24;
            const showPct = w > 50 && h > 36;

            return (
              <g key={d.instrumentId}>
                <rect
                  data-testid="treemap-cell"
                  x={x}
                  y={y}
                  width={w}
                  height={h}
                  rx={3}
                  fill={color}
                  fillOpacity={0.85}
                  stroke={d.hasWarning ? "#fbbf24" : "transparent"}
                  strokeWidth={d.hasWarning ? 2 : 0}
                  className="cursor-pointer transition-opacity hover:opacity-75"
                  onMouseEnter={() =>
                    setTooltip({
                      instrumentId: d.instrumentId,
                      concentration: d.concentration,
                      assetClass: d.assetClass,
                      x: x + w / 2,
                      y: y,
                    })
                  }
                  onMouseLeave={() => setTooltip(null)}
                />
                {showLabel && (
                  <text
                    x={x + w / 2}
                    y={y + (showPct ? h / 2 - 4 : h / 2 + 4)}
                    textAnchor="middle"
                    fill="#ffffff"
                    style={{
                      fontSize: Math.min(11, w / 8),
                      fontFamily: "'IBM Plex Sans', sans-serif",
                      pointerEvents: "none",
                    }}
                  >
                    {d.instrumentId}
                  </text>
                )}
                {showPct && (
                  <text
                    x={x + w / 2}
                    y={y + h / 2 + 10}
                    textAnchor="middle"
                    fill="#e5e7eb"
                    style={{
                      fontSize: 10,
                      fontFamily: "'IBM Plex Sans', sans-serif",
                      pointerEvents: "none",
                    }}
                  >
                    {d.concentration.toFixed(1)}%
                  </text>
                )}
                {d.hasWarning && w > 18 && h > 18 && (
                  <g data-testid="treemap-warning">
                    <circle
                      cx={x + w - 10}
                      cy={y + 10}
                      r={7}
                      fill="#dc2626"
                      fillOpacity={0.9}
                    />
                    <text
                      x={x + w - 10}
                      y={y + 14}
                      textAnchor="middle"
                      fill="#fff"
                      style={{
                        fontSize: 10,
                        fontWeight: "bold",
                        fontFamily: "'IBM Plex Sans', sans-serif",
                        pointerEvents: "none",
                      }}
                    >
                      !
                    </text>
                  </g>
                )}
              </g>
            );
          })}
        </svg>

        {/* Tooltip */}
        {tooltip && (
          <div
            data-testid="treemap-tooltip"
            className="pointer-events-none absolute z-50 rounded border border-border bg-bg-secondary px-3 py-2 text-xs shadow-lg"
            style={{
              left: tooltip.x,
              top: tooltip.y - 8,
              transform: "translate(-50%, -100%)",
              boxShadow: "rgba(0,0,0,0.03) 0px 4px 24px",
            }}
          >
            <p className="text-text-primary font-medium">
              {tooltip.instrumentId}
            </p>
            <p className="text-text-muted">
              {tooltip.concentration.toFixed(1)}% of NAV
            </p>
            <p className="text-text-muted">
              {formatAssetClass(tooltip.assetClass)}
            </p>
          </div>
        )}
      </div>

      {/* Asset class legend */}
      <div className="mt-3 flex flex-wrap gap-3">
        {Object.entries(concentration.byAssetClass).map(([ac, pct]) => (
          <div key={ac} className="flex items-center gap-1.5">
            <span
              className="inline-block h-2.5 w-2.5 rounded-sm"
              style={{
                backgroundColor:
                  ASSET_CLASS_COLORS[ac as AssetClass] ?? DEFAULT_COLOR,
              }}
            />
            <span className="text-[10px] text-text-muted">
              {formatAssetClass(ac as AssetClass)} {pct.toFixed(0)}%
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

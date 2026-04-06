import { useState, useCallback } from "react";
import { useOptimizerStore } from "../stores/optimizerStore";
import type {
  AssetClass,
  OptimizationResult,
  TradeAction,
} from "../api/types";

const ASSET_CLASSES: AssetClass[] = [
  "equity",
  "crypto",
  "tokenized_security",
  "future",
  "option",
];

function formatPct(value: number): string {
  return (value * 100).toFixed(2) + "%";
}

function formatCurrency(value: string): string {
  const num = parseFloat(value);
  if (Number.isNaN(num)) return value;
  return "$" + num.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

// ---- Constraint Form ----

function ConstraintForm() {
  const constraints = useOptimizerStore((s) => s.constraints);
  const setConstraint = useOptimizerStore((s) => s.setConstraint);
  const runOptimize = useOptimizerStore((s) => s.runOptimize);
  const isOptimizing = useOptimizerStore((s) => s.isOptimizing);

  // Local state for asset class bound rows
  const [boundRows, setBoundRows] = useState<
    { assetClass: AssetClass; min: number; max: number }[]
  >(() => {
    if (!constraints.assetClassBounds) return [];
    return Object.entries(constraints.assetClassBounds).map(([ac, [min, max]]) => ({
      assetClass: ac as AssetClass,
      min,
      max,
    }));
  });

  // Local toggles for optional fields
  const [maxWeightEnabled, setMaxWeightEnabled] = useState(
    constraints.maxSingleWeight !== null,
  );
  const [maxTurnoverEnabled, setMaxTurnoverEnabled] = useState(
    constraints.maxTurnover !== null,
  );

  const syncBounds = useCallback(
    (rows: { assetClass: AssetClass; min: number; max: number }[]) => {
      if (rows.length === 0) {
        setConstraint("assetClassBounds", null);
      } else {
        const bounds: Record<string, [number, number]> = {};
        for (const row of rows) {
          bounds[row.assetClass] = [row.min / 100, row.max / 100];
        }
        setConstraint("assetClassBounds", bounds);
      }
    },
    [setConstraint],
  );

  const addBoundRow = () => {
    const used = new Set(boundRows.map((r) => r.assetClass));
    const available = ASSET_CLASSES.find((ac) => !used.has(ac));
    if (!available) return;
    const next = [...boundRows, { assetClass: available, min: 0, max: 100 }];
    setBoundRows(next);
    syncBounds(next);
  };

  const removeBoundRow = (index: number) => {
    const next = boundRows.filter((_, i) => i !== index);
    setBoundRows(next);
    syncBounds(next);
  };

  const updateBoundRow = (
    index: number,
    field: "assetClass" | "min" | "max",
    value: string,
  ) => {
    const next = [...boundRows];
    if (field === "assetClass") {
      next[index] = { ...next[index], assetClass: value as AssetClass };
    } else {
      next[index] = { ...next[index], [field]: parseFloat(value) || 0 };
    }
    setBoundRows(next);
    syncBounds(next);
  };

  return (
    <div className="flex flex-col gap-4 rounded border border-border bg-bg-secondary p-4">
      <h3 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
        Constraints
      </h3>

      {/* Risk Aversion */}
      <div className="flex flex-col gap-1">
        <label
          htmlFor="risk-aversion"
          className="text-xs text-text-muted"
        >
          Risk Aversion: {constraints.riskAversion}
        </label>
        <input
          id="risk-aversion"
          type="range"
          min="0.1"
          max="20"
          step="0.1"
          value={constraints.riskAversion}
          onInput={(e) =>
            setConstraint(
              "riskAversion",
              parseFloat((e.target as HTMLInputElement).value),
            )
          }
          className="w-full accent-accent-blue"
        />
        <div className="flex justify-between text-[10px] text-text-muted">
          <span>0.1 (growth)</span>
          <span>20 (min variance)</span>
        </div>
      </div>

      {/* Long Only Toggle */}
      <div className="flex items-center gap-2">
        <input
          id="long-only"
          type="checkbox"
          checked={constraints.longOnly}
          onChange={(e) => setConstraint("longOnly", e.target.checked)}
          className="h-4 w-4 rounded border-border accent-accent-blue"
        />
        <label htmlFor="long-only" className="text-xs text-text-muted">
          Long Only
        </label>
      </div>

      {/* Max Single Weight */}
      <div className="flex flex-col gap-1">
        <div className="flex items-center gap-2">
          <input
            id="max-single-weight"
            type="checkbox"
            checked={maxWeightEnabled}
            onChange={(e) => {
              setMaxWeightEnabled(e.target.checked);
              setConstraint(
                "maxSingleWeight",
                e.target.checked ? 30 : null,
              );
            }}
            className="h-4 w-4 rounded border-border accent-accent-blue"
          />
          <label
            htmlFor="max-single-weight"
            className="text-xs text-text-muted"
          >
            Max Single Weight
          </label>
        </div>
        {maxWeightEnabled && (
          <div className="flex items-center gap-1 pl-6">
            <input
              type="number"
              min="1"
              max="100"
              value={constraints.maxSingleWeight ?? 30}
              onChange={(e) =>
                setConstraint(
                  "maxSingleWeight",
                  parseFloat(e.target.value) || null,
                )
              }
              className="w-20 rounded border border-border bg-bg-primary px-2 py-1 text-xs text-text-primary"
            />
            <span className="text-xs text-text-muted">%</span>
          </div>
        )}
      </div>

      {/* Max Turnover */}
      <div className="flex flex-col gap-1">
        <div className="flex items-center gap-2">
          <input
            id="max-turnover"
            type="checkbox"
            checked={maxTurnoverEnabled}
            onChange={(e) => {
              setMaxTurnoverEnabled(e.target.checked);
              setConstraint("maxTurnover", e.target.checked ? 50 : null);
            }}
            className="h-4 w-4 rounded border-border accent-accent-blue"
          />
          <label
            htmlFor="max-turnover"
            className="text-xs text-text-muted"
          >
            Max Turnover
          </label>
        </div>
        {maxTurnoverEnabled && (
          <div className="flex items-center gap-1 pl-6">
            <input
              type="number"
              min="1"
              max="100"
              value={constraints.maxTurnover ?? 50}
              onChange={(e) =>
                setConstraint(
                  "maxTurnover",
                  parseFloat(e.target.value) || null,
                )
              }
              className="w-20 rounded border border-border bg-bg-primary px-2 py-1 text-xs text-text-primary"
            />
            <span className="text-xs text-text-muted">%</span>
          </div>
        )}
      </div>

      {/* Asset Class Bounds */}
      <div className="flex flex-col gap-2">
        <div className="flex items-center justify-between">
          <span className="text-xs font-semibold uppercase tracking-wider text-text-muted">
            Asset Class Bounds
          </span>
          <button
            type="button"
            onClick={addBoundRow}
            disabled={boundRows.length >= ASSET_CLASSES.length}
            className="rounded border border-border bg-bg-primary px-2 py-0.5 text-xs text-accent-blue hover:bg-bg-secondary disabled:opacity-40"
          >
            + Add
          </button>
        </div>
        {boundRows.map((row, i) => (
          <div key={i} className="flex items-center gap-2">
            <select
              value={row.assetClass}
              onChange={(e) => updateBoundRow(i, "assetClass", e.target.value)}
              className="rounded border border-border bg-bg-primary px-2 py-1 text-xs text-text-primary"
            >
              {ASSET_CLASSES.map((ac) => (
                <option key={ac} value={ac}>
                  {ac}
                </option>
              ))}
            </select>
            <input
              type="number"
              min="0"
              max="100"
              value={row.min}
              onChange={(e) => updateBoundRow(i, "min", e.target.value)}
              className="w-16 rounded border border-border bg-bg-primary px-2 py-1 text-xs text-text-primary"
              placeholder="Min %"
            />
            <span className="text-xs text-text-muted">-</span>
            <input
              type="number"
              min="0"
              max="100"
              value={row.max}
              onChange={(e) => updateBoundRow(i, "max", e.target.value)}
              className="w-16 rounded border border-border bg-bg-primary px-2 py-1 text-xs text-text-primary"
              placeholder="Max %"
            />
            <span className="text-xs text-text-muted">%</span>
            <button
              type="button"
              onClick={() => removeBoundRow(i)}
              className="text-xs text-accent-red hover:text-accent-red/80"
            >
              Remove
            </button>
          </div>
        ))}
      </div>

      {/* Optimize Button */}
      <button
        type="button"
        onClick={runOptimize}
        disabled={isOptimizing}
        className="mt-2 rounded-xl bg-accent-blue px-4 py-2 text-sm font-semibold text-white hover:bg-accent-blue/80 disabled:opacity-50"
      >
        {isOptimizing ? "Optimizing..." : "Optimize"}
      </button>
    </div>
  );
}

// ---- Results Panel ----

function MetricCard({
  label,
  value,
}: {
  label: string;
  value: string;
}) {
  return (
    <div className="flex flex-col rounded border border-border bg-bg-secondary p-3">
      <span className="text-[10px] uppercase tracking-wider text-text-muted">
        {label}
      </span>
      <span className="text-lg font-bold text-text-primary">
        {value}
      </span>
    </div>
  );
}

function AllocationTable({
  targetWeights,
}: {
  targetWeights: Record<string, number>;
}) {
  const entries = Object.entries(targetWeights).sort(
    ([, a], [, b]) => b - a,
  );

  return (
    <div className="rounded border border-border bg-bg-secondary p-4">
      <h3 className="mb-3 text-xs font-semibold uppercase tracking-wider text-text-muted">
        Target Allocation
      </h3>
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-border text-left text-text-muted">
            <th className="pb-2 pr-4 font-medium">Instrument</th>
            <th className="pb-2 text-right font-medium">Target Weight</th>
          </tr>
        </thead>
        <tbody>
          {entries.map(([instrument, weight]) => (
            <tr key={instrument} className="border-b border-border/50">
              <td className="py-1.5 pr-4 text-text-primary">{instrument}</td>
              <td className="py-1.5 text-right text-accent-blue">
                {formatPct(weight)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function TradeListTable({
  trades,
  onExecuteAll,
  isExecuting,
  executionProgress,
}: {
  trades: TradeAction[];
  onExecuteAll: () => void;
  isExecuting: boolean;
  executionProgress: { submitted: number; total: number } | null;
}) {
  return (
    <div className="rounded border border-border bg-bg-secondary p-4">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
          Trade List
        </h3>
        <button
          type="button"
          onClick={onExecuteAll}
          disabled={isExecuting || trades.length === 0}
          className="rounded-xl bg-accent-green px-3 py-1 text-xs font-semibold text-white hover:bg-accent-green/80 disabled:opacity-50"
        >
          {isExecuting
            ? `Submitting ${executionProgress?.submitted ?? 0}/${executionProgress?.total ?? trades.length}`
            : "Execute All"}
        </button>
      </div>

      {executionProgress &&
        executionProgress.submitted === executionProgress.total && (
          <div className="mb-3 rounded border border-accent-green/30 bg-accent-green/10 px-3 py-2 text-xs text-accent-green">
            All {executionProgress.total} orders submitted successfully.
          </div>
        )}

      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-border text-left text-text-muted">
            <th className="pb-2 pr-4 font-medium">Instrument</th>
            <th className="pb-2 pr-4 font-medium">Side</th>
            <th className="pb-2 pr-4 text-right font-medium">Quantity</th>
            <th className="pb-2 text-right font-medium">Est. Cost</th>
          </tr>
        </thead>
        <tbody>
          {trades.map((trade) => (
            <tr
              key={trade.instrumentId}
              className="border-b border-border/50"
            >
              <td className="py-1.5 pr-4 text-text-primary">
                {trade.instrumentId}
              </td>
              <td className="py-1.5 pr-4">
                <span
                  className={`rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase ${
                    trade.side === "buy"
                      ? "bg-accent-green/20 text-accent-green"
                      : "bg-accent-red/20 text-accent-red"
                  }`}
                >
                  {trade.side}
                </span>
              </td>
              <td className="py-1.5 pr-4 text-right text-text-primary">
                {trade.quantity}
              </td>
              <td className="py-1.5 text-right text-text-muted">
                {formatCurrency(trade.estimatedCost)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ResultsPanel({ result }: { result: OptimizationResult }) {
  const executeTradeList = useOptimizerStore((s) => s.executeTradeList);
  const [isExecuting, setIsExecuting] = useState(false);
  const [executionProgress, setExecutionProgress] = useState<{
    submitted: number;
    total: number;
  } | null>(null);

  const handleExecuteAll = useCallback(async () => {
    setIsExecuting(true);
    setExecutionProgress({ submitted: 0, total: result.trades.length });

    try {
      await executeTradeList(result.trades);
      setExecutionProgress({
        submitted: result.trades.length,
        total: result.trades.length,
      });
    } finally {
      setIsExecuting(false);
    }
  }, [executeTradeList, result.trades]);

  return (
    <div className="flex flex-col gap-3">
      {/* Metrics Summary */}
      <div className="grid grid-cols-3 gap-3">
        <MetricCard
          label="Expected Return"
          value={formatPct(result.expectedReturn)}
        />
        <MetricCard
          label="Expected Volatility"
          value={formatPct(result.expectedVolatility)}
        />
        <MetricCard
          label="Sharpe Ratio"
          value={result.sharpeRatio.toFixed(3)}
        />
      </div>

      {/* Allocation Table */}
      <AllocationTable targetWeights={result.targetWeights} />

      {/* Trade List */}
      <TradeListTable
        trades={result.trades}
        onExecuteAll={handleExecuteAll}
        isExecuting={isExecuting}
        executionProgress={executionProgress}
      />
    </div>
  );
}

// ---- Main View ----

export function OptimizerView() {
  const result = useOptimizerStore((s) => s.result);
  const error = useOptimizerStore((s) => s.error);

  return (
    <div className="flex flex-col gap-3">
      <h2 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
        Portfolio Optimizer
      </h2>

      {error && (
        <div className="rounded border border-accent-red/30 bg-accent-red/10 px-3 py-2 text-xs text-accent-red">
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 gap-3 lg:grid-cols-[380px_1fr]">
        {/* Left: Constraint Form */}
        <ConstraintForm />

        {/* Right: Results */}
        {result ? (
          <ResultsPanel result={result} />
        ) : (
          <div className="flex items-center justify-center rounded border border-border bg-bg-secondary p-8">
            <p className="text-xs text-text-muted">
              Configure constraints and click Optimize to see results.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}

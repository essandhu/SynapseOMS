import { useState } from "react";
import { useInsightStore } from "../stores/insightStore";
import { useOrderStore } from "../stores/orderStore";
import type { TradeAction } from "../api/types";

export function RebalancingTab() {
  const [prompt, setPrompt] = useState("");
  const { rebalanceState, submitRebalancePrompt, clearRebalanceResult } =
    useInsightStore();
  const { submitOrder } = useOrderStore();

  const handleSubmit = () => {
    if (!prompt.trim()) return;
    submitRebalancePrompt(prompt.trim());
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  const handleExecuteAll = async (trades: TradeAction[]) => {
    for (const trade of trades) {
      await submitOrder({
        instrumentId: trade.instrumentId,
        side: trade.side,
        type: "market",
        quantity: trade.quantity,
        venueId: "smart",
      });
    }
  };

  return (
    <div className="space-y-4">
      {/* NL Input */}
      <div className="flex gap-2">
        <input
          type="text"
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Describe your rebalancing goal..."
          className="flex-1 bg-bg-secondary border border-border rounded-lg px-4 py-3 text-text-primary placeholder-text-muted focus:border-accent-blue focus:outline-none"
          data-testid="rebalance-input"
        />
        <button
          onClick={handleSubmit}
          disabled={rebalanceState.loading || !prompt.trim()}
          className="px-6 py-3 bg-accent-blue hover:bg-accent-blue/80 disabled:bg-accent-blue/40 disabled:cursor-not-allowed rounded-xl text-white font-medium transition-colors"
          data-testid="rebalance-submit"
        >
          Analyze
        </button>
      </div>

      {/* Loading */}
      {rebalanceState.loading && (
        <div className="flex items-center justify-center py-12" data-testid="rebalance-loading">
          <div className="animate-spin w-6 h-6 border-2 border-accent-blue border-t-transparent rounded-full mr-3" />
          <span className="text-text-secondary">Analyzing your request...</span>
        </div>
      )}

      {/* Error */}
      {rebalanceState.error && (
        <div className="bg-accent-red/10 border border-accent-red/30 rounded-lg p-4" data-testid="rebalance-error">
          <p className="text-accent-red">{rebalanceState.error}</p>
          <button
            onClick={clearRebalanceResult}
            className="mt-2 text-sm text-accent-red/80 hover:text-accent-red"
          >
            Try Again
          </button>
        </div>
      )}

      {/* Result */}
      {rebalanceState.result && (
        <div className="space-y-4" data-testid="rebalance-result">
          {/* AI Interpretation */}
          <div className="bg-bg-secondary border border-border rounded-lg p-4">
            <h3 className="text-sm font-semibold text-accent-blue mb-2">
              AI Interpretation
            </h3>
            <p className="text-text-primary text-sm">
              {rebalanceState.result.reasoning}
            </p>
            <div className="mt-2 text-xs text-text-muted">
              Objective: {rebalanceState.result.constraints.objective} | Risk
              Aversion: {rebalanceState.result.constraints.riskAversion}
            </div>
          </div>

          {/* Trade List */}
          {rebalanceState.result.optimization && (
            <div className="bg-bg-secondary border border-border rounded-lg p-4">
              <h3 className="text-sm font-semibold text-text-primary mb-3">
                Proposed Trades
              </h3>
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-text-muted border-b border-border">
                    <th className="text-left py-2">Instrument</th>
                    <th className="text-left py-2">Side</th>
                    <th className="text-right py-2">Quantity</th>
                    <th className="text-right py-2">Est. Cost</th>
                  </tr>
                </thead>
                <tbody>
                  {rebalanceState.result.optimization.trades.map((trade, i) => (
                    <tr key={i} className="border-b border-border/50">
                      <td className="py-2 text-text-primary">
                        {trade.instrumentId}
                      </td>
                      <td className="py-2">
                        <span
                          className={
                            trade.side === "buy"
                              ? "text-accent-green"
                              : "text-accent-red"
                          }
                        >
                          {trade.side.toUpperCase()}
                        </span>
                      </td>
                      <td className="py-2 text-right text-text-primary">
                        {trade.quantity}
                      </td>
                      <td className="py-2 text-right text-text-secondary">
                        ${trade.estimatedCost}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>

              <div className="flex justify-between items-center mt-4">
                <button
                  onClick={clearRebalanceResult}
                  className="text-sm text-text-muted hover:text-text-secondary"
                >
                  Clear
                </button>
                <button
                  onClick={() =>
                    handleExecuteAll(
                      rebalanceState.result!.optimization.trades,
                    )
                  }
                  className="px-4 py-2 bg-accent-green hover:bg-accent-green/80 rounded-xl text-white text-sm font-medium"
                  data-testid="execute-all"
                >
                  Execute All
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

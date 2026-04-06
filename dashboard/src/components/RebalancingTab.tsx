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
          className="flex-1 bg-[#f8f9fa] border border-[#dedee5] rounded-lg px-4 py-3 text-[#101114] placeholder-[#9497a9] focus:border-[#7132f5] focus:outline-none"
          data-testid="rebalance-input"
        />
        <button
          onClick={handleSubmit}
          disabled={rebalanceState.loading || !prompt.trim()}
          className="px-6 py-3 bg-[#7132f5] hover:bg-[#5b1ecf] disabled:bg-[#c4b5fd] disabled:cursor-not-allowed rounded-xl text-white font-medium transition-colors"
          data-testid="rebalance-submit"
        >
          Analyze
        </button>
      </div>

      {/* Loading */}
      {rebalanceState.loading && (
        <div className="flex items-center justify-center py-12" data-testid="rebalance-loading">
          <div className="animate-spin w-6 h-6 border-2 border-[#7132f5] border-t-transparent rounded-full mr-3" />
          <span className="text-[#686b82]">Analyzing your request...</span>
        </div>
      )}

      {/* Error */}
      {rebalanceState.error && (
        <div className="bg-red-50 border border-red-200 rounded-lg p-4" data-testid="rebalance-error">
          <p className="text-red-600">{rebalanceState.error}</p>
          <button
            onClick={clearRebalanceResult}
            className="mt-2 text-sm text-red-500 hover:text-red-700"
          >
            Try Again
          </button>
        </div>
      )}

      {/* Result */}
      {rebalanceState.result && (
        <div className="space-y-4" data-testid="rebalance-result">
          {/* AI Interpretation */}
          <div className="bg-[#f8f9fa] border border-[#dedee5] rounded-lg p-4">
            <h3 className="text-sm font-semibold text-[#7132f5] mb-2">
              AI Interpretation
            </h3>
            <p className="text-[#101114] text-sm">
              {rebalanceState.result.reasoning}
            </p>
            <div className="mt-2 text-xs text-[#9497a9]">
              Objective: {rebalanceState.result.constraints.objective} | Risk
              Aversion: {rebalanceState.result.constraints.riskAversion}
            </div>
          </div>

          {/* Trade List */}
          {rebalanceState.result.optimization && (
            <div className="bg-[#f8f9fa] border border-[#dedee5] rounded-lg p-4">
              <h3 className="text-sm font-semibold text-[#101114] mb-3">
                Proposed Trades
              </h3>
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-[#9497a9] border-b border-[#dedee5]">
                    <th className="text-left py-2">Instrument</th>
                    <th className="text-left py-2">Side</th>
                    <th className="text-right py-2">Quantity</th>
                    <th className="text-right py-2">Est. Cost</th>
                  </tr>
                </thead>
                <tbody>
                  {rebalanceState.result.optimization.trades.map((trade, i) => (
                    <tr key={i} className="border-b border-[#f0f1f3]">
                      <td className="py-2 text-[#101114]">
                        {trade.instrumentId}
                      </td>
                      <td className="py-2">
                        <span
                          className={
                            trade.side === "buy"
                              ? "text-[#149e61]"
                              : "text-[#ef4444]"
                          }
                        >
                          {trade.side.toUpperCase()}
                        </span>
                      </td>
                      <td className="py-2 text-right text-[#101114]">
                        {trade.quantity}
                      </td>
                      <td className="py-2 text-right text-[#686b82]">
                        ${trade.estimatedCost}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>

              <div className="flex justify-between items-center mt-4">
                <button
                  onClick={clearRebalanceResult}
                  className="text-sm text-[#9497a9] hover:text-[#686b82]"
                >
                  Clear
                </button>
                <button
                  onClick={() =>
                    handleExecuteAll(
                      rebalanceState.result!.optimization.trades,
                    )
                  }
                  className="px-4 py-2 bg-[#149e61] hover:bg-[#0f7a4d] rounded-xl text-white text-sm font-medium"
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

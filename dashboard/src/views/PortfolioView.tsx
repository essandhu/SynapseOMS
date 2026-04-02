import { useEffect } from "react";
import { PositionTable } from "../components/PositionTable";
import { usePositionStore } from "../stores/positionStore";

export function PortfolioView() {
  const positions = usePositionStore((s) => s.positions);
  const loading = usePositionStore((s) => s.loading);
  const error = usePositionStore((s) => s.error);
  const subscribe = usePositionStore((s) => s.subscribe);

  useEffect(() => {
    const unsubscribe = subscribe();
    return unsubscribe;
  }, [subscribe]);

  const positionList = Array.from(positions.values());

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h2 className="font-mono text-xs font-semibold uppercase tracking-wider text-text-muted">
          Positions
        </h2>
        {loading && (
          <span className="font-mono text-xs text-text-muted">Loading...</span>
        )}
      </div>

      {error && (
        <div className="rounded border border-accent-red/30 bg-accent-red/10 px-3 py-2 font-mono text-xs text-accent-red">
          {error}
        </div>
      )}

      <PositionTable positions={positionList} />
    </div>
  );
}

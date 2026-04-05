import { PieChart, Pie, Cell, Tooltip } from "recharts";
import { useRef, useState, useEffect } from "react";

export interface ExposureTreemapDatum {
  name: string;
  value: number;
  color: string;
}

export interface ExposureTreemapProps {
  data: ExposureTreemapDatum[];
}

function CustomTooltip({
  active,
  payload,
}: {
  active?: boolean;
  payload?: { payload: ExposureTreemapDatum }[];
}) {
  if (!active || !payload?.length) return null;
  const d = payload[0].payload;
  return (
    <div className="rounded border border-border bg-bg-secondary px-3 py-2 font-mono text-xs shadow-lg">
      <span className="text-text-primary">{d.name}</span>
      <span className="ml-2 text-text-muted">{d.value.toFixed(1)}%</span>
    </div>
  );
}

export function ExposureTreemap({ data }: ExposureTreemapProps) {
  if (data.length === 0) {
    return (
      <div className="flex h-full items-center justify-center font-mono text-xs text-text-muted">
        No exposure data
      </div>
    );
  }

  const containerRef = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ width: 0, height: 0 });

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect;
      if (width > 0 && height > 0) {
        setSize({ width, height });
      }
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  return (
    <div ref={containerRef} style={{ width: "100%", height: "100%" }}>
      {size.width > 0 && size.height > 0 && (
        <PieChart width={size.width} height={size.height}>
          <Pie
            data={data}
            cx="50%"
            cy="50%"
            innerRadius="55%"
            outerRadius="85%"
            dataKey="value"
            nameKey="name"
            stroke="none"
            paddingAngle={2}
            isAnimationActive={false}
          >
            {data.map((entry) => (
              <Cell key={entry.name} fill={entry.color} />
            ))}
          </Pie>
          <Tooltip content={<CustomTooltip />} />
        </PieChart>
      )}
    </div>
  );
}

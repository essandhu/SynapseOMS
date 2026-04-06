import { useEffect, useRef, useMemo } from "react";
import { createChart, ColorType, CandlestickSeries } from "lightweight-charts";
import type { IChartApi, ISeriesApi, CandlestickData, Time } from "lightweight-charts";
import { useMarketDataStore } from "../stores/marketDataStore";
import { useThemeColors } from "../theme/terminal";

interface CandlestickChartProps {
  instrumentId: string;
  interval?: "1m" | "5m";
}

function toChartData(bar: {
  open: string;
  high: string;
  low: string;
  close: string;
  periodStart: string;
}): CandlestickData<Time> {
  return {
    time: (Math.floor(new Date(bar.periodStart).getTime() / 1000)) as unknown as Time,
    open: parseFloat(bar.open),
    high: parseFloat(bar.high),
    low: parseFloat(bar.low),
    close: parseFloat(bar.close),
  };
}

export function CandlestickChart({
  instrumentId,
  interval = "1m",
}: CandlestickChartProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const seriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
  const theme = useThemeColors();
  const key = `${instrumentId}:${interval}`;
  const barsFromStore = useMarketDataStore((s) => s.bars[key]);
  const bars = useMemo(() => barsFromStore ?? [], [barsFromStore]);

  // Create / recreate chart when theme changes
  useEffect(() => {
    if (!containerRef.current) return;

    const chart = createChart(containerRef.current, {
      layout: {
        background: { type: ColorType.Solid, color: theme.colors.bg.primary },
        textColor: theme.colors.text.muted,
        fontFamily: theme.fonts.sans,
        fontSize: 11,
      },
      grid: {
        vertLines: { color: theme.colors.bg.tertiary },
        horzLines: { color: theme.colors.bg.tertiary },
      },
      crosshair: {
        vertLine: { color: theme.colors.border, width: 1 },
        horzLine: { color: theme.colors.border, width: 1 },
      },
      timeScale: {
        borderColor: theme.colors.border,
        timeVisible: true,
        secondsVisible: false,
      },
      rightPriceScale: {
        borderColor: theme.colors.border,
      },
      width: containerRef.current.clientWidth,
      height: containerRef.current.clientHeight,
    });

    const series = chart.addSeries(CandlestickSeries, {
      upColor: theme.colors.accent.green,
      downColor: theme.colors.accent.red,
      borderUpColor: theme.colors.accent.green,
      borderDownColor: theme.colors.accent.red,
      wickUpColor: theme.colors.accent.green,
      wickDownColor: theme.colors.accent.red,
    });

    chartRef.current = chart;
    seriesRef.current = series;

    const handleResize = () => {
      if (containerRef.current) {
        chart.resize(containerRef.current.clientWidth, containerRef.current.clientHeight);
      }
    };
    const observer = new ResizeObserver(handleResize);
    observer.observe(containerRef.current);

    return () => {
      observer.disconnect();
      chart.remove();
      chartRef.current = null;
      seriesRef.current = null;
    };
  }, [theme]);

  // Update data when bars change
  useEffect(() => {
    if (!seriesRef.current || bars.length === 0) return;

    const chartData = bars.map(toChartData);
    seriesRef.current.setData(chartData);
    chartRef.current?.timeScale().scrollToRealTime();
  }, [bars]);

  const hasData = bars.length > 0;

  return (
    <div
      data-testid="candlestick-chart"
      className="relative w-full h-full min-h-[200px]"
    >
      <div ref={containerRef} className="w-full h-full" />
      {!hasData && (
        <div className="absolute inset-0 flex items-center justify-center">
          <span
            className="text-xs animate-pulse"
            style={{ color: theme.colors.text.muted }}
          >
            Waiting for market data...
          </span>
        </div>
      )}
    </div>
  );
}

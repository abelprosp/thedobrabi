"use client";

import { Card } from "@/components/ui";
import { cn } from "@/lib/cn";
import {
  chartChrome,
  chartPalette,
  formatAxisTick,
  formatNumber,
  hexToRgba,
  type LegendPosition,
} from "@/lib/widget-config";
import { chartLegend, chartTooltip } from "@/lib/chartjs";
import { ChartJsCanvas } from "@/components/chartjs-canvas";
import { useTheme } from "@/components/theme-provider";
import type { ChartType } from "chart.js";

type ChartProps = {
  type?: "line" | "bar" | "area" | "pie";
  title?: string;
  columns?: string[];
  rows?: Record<string, any>[];
  height?: number | string;
  config?: Record<string, any>;
  onClick?: (payload: { dimension: string; value: string }) => void;
};

const ISO_DATE = /^\d{4}-\d{2}-\d{2}(T\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:?\d{2})?)?$/;
const YEAR_MONTH = /^(\d{4})-(\d{2})$/;

export function formatCategory(v: unknown) {
  const s = String(v ?? "");
  const ym = YEAR_MONTH.exec(s);
  if (ym) {
    const d = new Date(Number(ym[1]), Number(ym[2]) - 1, 1);
    if (!Number.isNaN(d.getTime())) {
      return d.toLocaleDateString("pt-BR", { month: "short", year: "numeric" });
    }
  }
  if (!ISO_DATE.test(s)) return s;
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return s;
  const hasTime = s.includes("T") && !/T00:00(:00(\.0+)?)?(Z|[+-]00:?00)?$/.test(s);
  return d.toLocaleDateString("pt-BR", hasTime ? { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit" } : { day: "2-digit", month: "2-digit", year: "numeric" });
}

export function pivotSeries(rows: Record<string, any>[], catCol: string, seriesCol: string, valueCol: string) {
  const cats: string[] = [];
  const catSeen = new Set<string>();
  const series: string[] = [];
  const seriesSeen = new Set<string>();
  const map = new Map<string, number>();
  for (const r of rows) {
    const cat = String(r[catCol] ?? "");
    const ser = String(r[seriesCol] ?? "");
    if (!catSeen.has(cat)) {
      catSeen.add(cat);
      cats.push(cat);
    }
    if (!seriesSeen.has(ser)) {
      seriesSeen.add(ser);
      series.push(ser);
    }
    map.set(`${cat}\0${ser}`, Number(r[valueCol] ?? 0));
  }
  return {
    rawCats: cats,
    cats,
    series,
    values: series.map((s) => cats.map((c) => map.get(`${c}\0${s}`) ?? 0)),
  };
}

export function Chart({ type = "bar", columns = [], rows = [], height, onClick, config = {} }: ChartProps) {
  const { theme } = useTheme();
  const chrome = chartChrome(theme);
  const dim = columns.find((c) => typeof rows[0]?.[c] === "string") || columns[0];
  const rawCatsAll = rows.map((r) => String(r[dim] ?? ""));
  const numericCols = columns.filter((c) => c !== dim && typeof rows[0]?.[c] === "number");
  const measCols = numericCols.length ? numericCols : [columns.find((c) => c !== dim) || columns[1] || columns[0]].filter(Boolean);
  const seriesDim = type === "pie" ? undefined : columns.find((c) => c !== dim && !measCols.includes(c));
  const pivoted = seriesDim && measCols[0] ? pivotSeries(rows, dim, seriesDim, measCols[0]) : null;
  const rawCats = pivoted ? pivoted.rawCats : rawCatsAll;
  const cats = (pivoted ? pivoted.cats : rawCatsAll).map((s) => formatCategory(s));
  const seriesNames = pivoted ? pivoted.series : measCols;
  const seriesValues = pivoted ? pivoted.values : measCols.map((m) => rows.map((r) => Number(r[m] ?? 0)));
  const palette = chartPalette(config.color);
  const showLegend = type === "pie" ? config.showLegend !== false : !!config.showLegend || seriesNames.length > 1;
  const showTooltip = config.showTooltip !== false;
  const showGrid = config.showGrid !== false;
  const showX = config.showXAxis !== false;
  const showY = config.showYAxis !== false;
  const stacked = !!config.stacked;
  const horizontal = !!config.horizontal && type === "bar";
  const smooth = config.smooth !== false && (type === "line" || type === "area");
  const legendPos = (config.legendPosition || "top") as LegendPosition;
  const axisFmt = (v: string | number) => formatAxisTick(v, config);

  const chartType: ChartType = type === "pie" ? "doughnut" : type === "area" ? "line" : type;
  const data: any =
    type === "pie"
      ? {
          labels: cats,
          datasets: [
            {
              data: cats.map((_, i) => Number(rows[i]?.[measCols[0]] ?? 0)),
              backgroundColor: cats.map((_, i) => palette[i % palette.length]),
              borderColor: chrome.surface,
              borderWidth: 2,
              hoverOffset: 6,
              cutout: "62%",
            },
          ],
        }
      : {
          labels: cats,
          datasets: seriesNames.map((name, i) => {
            const c = palette[i % palette.length];
            return {
              label: String(name),
              data: seriesValues[i] || [],
              backgroundColor: type === "bar" ? c : hexToRgba(c, type === "area" ? 0.32 : 0.16),
              borderColor: c,
              borderWidth: type === "bar" ? 0 : 2.5,
              fill: type === "area" || type === "line",
              tension: smooth ? 0.35 : 0,
              pointRadius: type === "bar" ? 0 : 3,
              pointHoverRadius: 5,
              borderRadius: type === "bar" ? 6 : 0,
              maxBarThickness: 52,
            };
          }),
        };

  const options: any = {
    indexAxis: horizontal ? "y" : "x",
    interaction: { mode: type === "pie" ? "nearest" : "index", intersect: type === "pie" },
    plugins: {
      legend: chartLegend(showLegend, legendPos, theme),
      tooltip: showTooltip
        ? {
            ...chartTooltip(theme),
            callbacks: {
              label: (ctx: any) => {
                const parsed = ctx.parsed as number | { x?: number; y?: number };
                const n = typeof parsed === "number" ? parsed : Number((horizontal ? parsed.x : parsed.y) ?? 0);
                if (type === "pie") {
                  const total = (ctx.dataset.data as number[]).reduce((s, x) => s + Number(x || 0), 0);
                  const pct = total ? Math.round((n / total) * 100) : 0;
                  return `${formatNumber(n, config)} · ${pct}%`;
                }
                return `${ctx.dataset.label}: ${formatNumber(n, config)}`;
              },
            },
          }
        : { enabled: false },
    },
    scales:
      type === "pie"
        ? undefined
        : {
            x: {
              type: horizontal ? "linear" : "category",
              display: horizontal ? showY : showX,
              stacked,
              title: { display: !!config.xAxisLabel, text: config.xAxisLabel || "", color: chrome.mute, font: { size: 11 } },
              ticks: {
                color: chrome.mute,
                maxRotation: horizontal ? 0 : Number(config.xAxisRotate ?? 0),
                minRotation: horizontal ? 0 : Number(config.xAxisRotate ?? 0),
                ...(horizontal ? { callback: (v: any) => axisFmt(v as number) } : {}),
              },
              grid: { display: horizontal ? showGrid : false, color: chrome.line },
              border: { display: false },
            },
            y: {
              type: horizontal ? "category" : "linear",
              display: horizontal ? showX : showY,
              stacked,
              title: { display: !!config.yAxisLabel, text: config.yAxisLabel || "", color: chrome.mute, font: { size: 11 } },
              ticks: {
                color: chrome.mute,
                ...(horizontal ? {} : { callback: (v: any) => axisFmt(v as number) }),
              },
              grid: { display: horizontal ? false : showGrid, color: chrome.line },
              border: { display: false },
            },
          },
    onClick: onClick
      ? (_evt: any, elements: any) => {
          if (!elements.length) return;
          const idx = elements[0].index;
          if (type === "pie") onClick({ dimension: dim, value: rawCats[idx] ?? cats[idx] ?? "" });
          else onClick({ dimension: dim, value: rawCats[idx] ?? "" });
        }
      : undefined,
  } as any;

  return (
    <div className="h-full w-full min-h-0" style={height == null ? undefined : { height }}>
      <ChartJsCanvas type={chartType} data={data} options={options} />
    </div>
  );
}

const KPI_SIZE = { sm: "text-2xl", md: "text-[28px]", lg: "text-4xl" } as const;

export function Kpi({
  label,
  value,
  delta,
  comparisonLabel,
  align,
  fontSize,
  showTitle = true,
  color,
  goalLabel,
  progress,
}: {
  label: string;
  value: string;
  delta?: number;
  comparisonLabel?: string;
  align?: "left" | "center";
  fontSize?: "sm" | "md" | "lg";
  showTitle?: boolean;
  color?: string;
  goalLabel?: string;
  progress?: number;
}) {
  const pos = delta === undefined ? null : delta >= 0;
  return (
    <Card className={cn("flex h-full flex-col justify-between gap-1 p-4", align === "center" && "text-center")}>
      {showTitle !== false && <div className="text-[11px] font-medium uppercase tracking-wide text-mute">{label}</div>}
      <div className={cn("font-semibold tracking-tight text-ink", KPI_SIZE[fontSize || "md"])} style={color ? { color } : undefined}>
        {value}
      </div>
      {goalLabel && <div className="text-[11px] text-mute">{goalLabel}</div>}
      {progress != null && (
        <div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-surface-2">
          <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${Math.min(100, Math.max(0, progress))}%`, backgroundColor: color }} />
        </div>
      )}
      {delta !== undefined && (
        <div className={`text-[12px] font-medium ${pos ? "text-ok" : "text-danger"}`}>
          {pos ? "+" : ""}
          {delta.toFixed(1)}% {comparisonLabel || "vs. período anterior"}
        </div>
      )}
    </Card>
  );
}

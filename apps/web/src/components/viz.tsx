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
import { KpiIconBadge } from "@/components/kpi-icon";
import { useTheme } from "@/components/theme-provider";
import type { ChartType } from "chart.js";

type ChartProps = {
  type?: "line" | "bar" | "area" | "pie";
  title?: string;
  columns?: string[];
  rows?: Record<string, any>[];
  measures?: string[];
  height?: number | string;
  config?: Record<string, any>;
  onClick?: (payload: { dimension: string; value: string }) => void;
};

const ISO_DATE = /^\d{4}-\d{2}-\d{2}([T ]\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:?\d{2})?)?$/;
const YEAR_MONTH = /^(\d{4})[-/](\d{2})$/;
const MONTH_YEAR = /^(\d{2})[-/](\d{4})$/;
const YEAR_MONTH_COMPACT = /^(\d{4})(\d{2})$/;
const YEAR_ONLY = /^\d{4}$/;
const PT_MONTH: Record<string, number> = {
  janeiro: 1, jan: 1, fevereiro: 2, fev: 2, marco: 3, março: 3, mar: 3,
  abril: 4, abr: 4, maio: 5, mai: 5, junho: 6, jun: 6, julho: 7, jul: 7,
  agosto: 8, ago: 8, setembro: 9, set: 9, outubro: 10, out: 10, novembro: 11, nov: 11, dezembro: 12, dez: 12,
};

function utcMonth(year: number, month: number): number | null {
  if (month < 1 || month > 12 || year < 1900 || year > 2100) return null;
  return Date.UTC(year, month - 1, 1);
}

export function timeSortKey(v: unknown): number | null {
  if (v instanceof Date && !Number.isNaN(v.getTime())) return v.getTime();
  if (typeof v === "number" && Number.isFinite(v)) {
    if (v >= 1900 && v <= 2100) return Date.UTC(v, 0, 1);
    if (v >= 190001 && v <= 210012) {
      const compact = utcMonth(Math.floor(v / 100), v % 100);
      if (compact != null) return compact;
    }
    if (v > 1e11) return v;
  }
  const s = String(v ?? "").trim();
  if (!s) return null;
  const ym = YEAR_MONTH.exec(s);
  if (ym) return utcMonth(Number(ym[1]), Number(ym[2]));
  const myNum = MONTH_YEAR.exec(s);
  if (myNum) return utcMonth(Number(myNum[2]), Number(myNum[1]));
  const compact = YEAR_MONTH_COMPACT.exec(s);
  if (compact) return utcMonth(Number(compact[1]), Number(compact[2]));
  if (YEAR_ONLY.test(s)) return Date.UTC(Number(s), 0, 1);
  if (ISO_DATE.test(s)) {
    const t = Date.parse(s.includes("T") || s.includes("Z") || s.includes("+") ? s : s.replace(" ", "T"));
    return Number.isNaN(t) ? null : t;
  }
  const monthYear = s.toLowerCase().replace(/\./g, "").normalize("NFD").replace(/[\u0300-\u036f]/g, "");
  const my = monthYear.match(/^([a-z]+)[\s\-/]*(?:de\s*)?(\d{4})$/);
  if (my && PT_MONTH[my[1]]) return utcMonth(Number(my[2]), PT_MONTH[my[1]]);
  if (PT_MONTH[monthYear]) return PT_MONTH[monthYear];
  return null;
}

export function compareTimeCategory(a: string, b: string): number {
  const ka = timeSortKey(a);
  const kb = timeSortKey(b);
  if (ka != null && kb != null && ka !== kb) return ka - kb;
  if (ka != null && kb == null) return -1;
  if (ka == null && kb != null) return 1;
  return a.localeCompare(b, "pt", { numeric: true });
}

export function sortRowsByTimeCategory(rows: Record<string, any>[], dim?: string) {
  if (!dim || rows.length < 2) return rows;
  const sample = rows.slice(0, Math.min(12, rows.length));
  const hits = sample.filter((r) => timeSortKey(r[dim]) != null).length;
  if (hits < Math.ceil(sample.length * 0.6)) return rows;
  return [...rows].sort((a, b) => compareTimeCategory(String(a[dim] ?? ""), String(b[dim] ?? "")));
}

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
  const orderedCats =
    cats.filter((c) => timeSortKey(c) != null).length >= Math.ceil(cats.length * 0.6)
      ? [...cats].sort(compareTimeCategory)
      : cats;
  return {
    rawCats: orderedCats,
    cats: orderedCats,
    series,
    values: series.map((s) => orderedCats.map((c) => map.get(`${c}\0${s}`) ?? 0)),
  };
}

export function Chart({ type = "bar", columns = [], rows = [], measures, height, onClick, config = {} }: ChartProps) {
  const { theme } = useTheme();
  const chrome = chartChrome(theme);
  const stringDim = columns.find((c) => typeof rows[0]?.[c] === "string" || rows[0]?.[c] instanceof Date);
  const dim = stringDim || columns[0];
  const chartRows = sortRowsByTimeCategory(rows, dim);
  const rawCatsAll = chartRows.map((r) => String(r[dim] ?? ""));
  const measCols = resolveMeasureColumns(columns, chartRows, stringDim, measures);
  const pieByMeasures = type === "pie" && measCols.length > 1 && (!stringDim || chartRows.length <= 1);
  const seriesDim = type === "pie" || pieByMeasures ? undefined : columns.find((c) => c !== dim && !measCols.includes(c));
  const pivoted = seriesDim && measCols[0] ? pivotSeries(chartRows, dim, seriesDim, measCols[0]) : null;
  const rawCats = pieByMeasures ? measCols : pivoted ? pivoted.rawCats : rawCatsAll;
  const cats = pieByMeasures ? measCols : (pivoted ? pivoted.cats : rawCatsAll).map((s) => formatCategory(s));
  const seriesNames = pieByMeasures ? measCols : pivoted ? pivoted.series : measCols;
  const seriesValues = pieByMeasures
    ? [measCols.map((m) => chartRows.reduce((s, r) => s + Number(r[m] ?? 0), 0))]
    : pivoted
      ? pivoted.values
      : measCols.map((m) => chartRows.map((r) => Number(r[m] ?? 0)));
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
  const pieTotals = pieByMeasures ? seriesValues[0] || [] : [];
  const data: any =
    type === "pie"
      ? pieByMeasures
        ? {
            labels: cats,
            datasets: [
              {
                data: pieTotals,
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
            datasets: measCols.map((m, di) => ({
              label: String(m),
              data: cats.map((_, i) => Number(rows[i]?.[m] ?? 0)),
              backgroundColor: cats.map((_, i) => palette[(di + i) % palette.length]),
              borderColor: chrome.surface,
              borderWidth: 2,
              hoverOffset: di === 0 ? 6 : 4,
              cutout: measCols.length > 1 ? "32%" : "62%",
              weight: 1,
            })),
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
                  const name = pieByMeasures ? cats[ctx.dataIndex] : ctx.dataset.label;
                  return `${name ? `${name}: ` : ""}${formatNumber(n, config)} · ${pct}%`;
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
          if (type === "pie") {
            if (pieByMeasures) return;
            onClick({ dimension: dim, value: rawCats[idx] ?? cats[idx] ?? "" });
          } else onClick({ dimension: dim, value: rawCats[idx] ?? "" });
        }
      : undefined,
  } as any;

  return (
    <div className="h-full w-full min-h-0" style={height == null ? undefined : { height }}>
      <ChartJsCanvas type={chartType} data={data} options={options} />
    </div>
  );
}

function resolveMeasureColumns(
  columns: string[],
  rows: Record<string, any>[],
  dim: string | undefined,
  preferred?: string[],
) {
  const row = rows[0] || {};
  const keys = columns.length ? columns : Object.keys(row);
  const match = (name: string) =>
    keys.find((c) => c === name) || keys.find((c) => c.toLowerCase() === name.toLowerCase().replace(/\s+/g, "_"));
  const fromPref = (preferred || []).map(match).filter((c): c is string => !!c && c !== dim);
  if (fromPref.length) return [...new Set(fromPref)];
  const numericCols = keys.filter((c) => c !== dim && typeof row[c] === "number");
  if (numericCols.length) return numericCols;
  return [keys.find((c) => c !== dim) || keys[1] || keys[0]].filter(Boolean);
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
  icon,
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
  icon?: string;
}) {
  const pos = delta === undefined ? null : delta >= 0;
  const body = (
    <>
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
    </>
  );
  return (
    <Card className={cn("flex h-full flex-col justify-between gap-1 p-4", align === "center" && "text-center")}>
      {icon ? (
        <div className={cn("flex items-start gap-3", align === "center" && "flex-col items-center")}>
          <KpiIconBadge value={icon} color={color} align={align} />
          <div className="min-w-0 flex-1 space-y-1">{body}</div>
        </div>
      ) : (
        body
      )}
    </Card>
  );
}

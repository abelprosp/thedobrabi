"use client";

import ReactECharts from "echarts-for-react";
import { Card } from "@/components/ui";
import { cn } from "@/lib/cn";
import {
  chartChrome,
  chartPalette,
  echartsTooltip,
  formatNumber,
  hexToRgba,
  legendOption,
  type LegendPosition,
} from "@/lib/widget-config";
import { useTheme } from "@/components/theme-provider";

type ChartProps = {
  type?: "line" | "bar" | "area" | "pie";
  title?: string;
  columns?: string[];
  rows?: Record<string, any>[];
  height?: number;
  config?: Record<string, any>;
  onClick?: (payload: { dimension: string; value: string }) => void;
};

const PRIMARY = "#2563EB";

const ISO_DATE = /^\d{4}-\d{2}-\d{2}(T\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:?\d{2})?)?$/;

/** Formata categorias que sejam datas ISO (ex.: 2025-10-26T00:00:00Z → 26/10/2025). */
function formatCategory(v: unknown) {
  const s = String(v ?? "");
  if (!ISO_DATE.test(s)) return s;
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return s;
  const hasTime = s.includes("T") && !/T00:00(:00(\.0+)?)?(Z|[+-]00:?00)?$/.test(s);
  return d.toLocaleDateString("pt-BR", hasTime ? { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit" } : { day: "2-digit", month: "2-digit", year: "numeric" });
}

function pivotSeries(
  rows: Record<string, any>[],
  catCol: string,
  seriesCol: string,
  valueCol: string,
) {
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

export function Chart({ type = "bar", title, columns = [], rows = [], height = 280, onClick, config = {} }: ChartProps) {
  const { theme } = useTheme();
  const chrome = chartChrome();
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
  const color = config.color || PRIMARY;
  const showLegend = type === "pie" ? config.showLegend !== false : !!config.showLegend || seriesNames.length > 1;
  const showLabels = type === "pie" ? config.showDataLabels !== false : !!config.showDataLabels;
  const showTooltip = config.showTooltip !== false;
  const showGrid = config.showGrid !== false;
  const showX = config.showXAxis !== false;
  const showY = config.showYAxis !== false;
  const stacked = !!config.stacked;
  const horizontal = !!config.horizontal && type === "bar";
  const smooth = config.smooth !== false && (type === "line" || type === "area");
  const legendPos = (config.legendPosition || "top") as LegendPosition;
  const axisFmt = (v: number) => formatNumber(v, { ...config, decimals: config.decimals ?? 0 });

  const handleClick = (params: any) => {
    if (!onClick) return;
    if (type === "pie") {
      onClick({ dimension: dim, value: params?.name ?? "" });
    } else {
      const idx = params?.dataIndex ?? 0;
      onClick({ dimension: dim, value: rawCats[idx] ?? "" });
    }
  };

  const legend = legendOption(showLegend, legendPos);
  const legendPad = showLegend && (legendPos === "top" || legendPos === "bottom") ? 22 : 0;
  const sidePad = showLegend && (legendPos === "left" || legendPos === "right") ? 72 : 0;

  const option =
    type === "pie"
      ? {
          backgroundColor: "transparent",
          tooltip: showTooltip ? { ...echartsTooltip(), trigger: "item", formatter: (p: any) => `${p.name}: ${formatNumber(p.value, config)} (${p.percent}%)` } : { show: false },
          legend,
          color: palette,
          series: [
            {
              type: "pie",
              radius: ["45%", "70%"],
              label: { show: showLabels, fontSize: 11, color: chrome.ink, formatter: (p: any) => `${p.name}\n${formatNumber(p.value, config)}` },
              data: cats.map((n, i) => ({ name: n, value: Number(rows[i]?.[measCols[0]] ?? 0) })),
            },
          ],
        }
      : (() => {
          const categoryAxis = {
            type: "category" as const,
            show: horizontal ? showY : showX,
            data: cats,
            name: config.xAxisLabel || undefined,
            nameLocation: "middle" as const,
            nameGap: 28,
            nameTextStyle: { color: chrome.mute, fontSize: 10 },
            axisLine: { lineStyle: { color: chrome.line } },
            axisTick: { show: false },
            axisLabel: { color: chrome.mute, fontSize: 11, rotate: horizontal ? 0 : (config.xAxisRotate ?? 0) },
          };
          const valueAxis = {
            type: "value" as const,
            show: horizontal ? showX : showY,
            name: config.yAxisLabel || undefined,
            nameLocation: "middle" as const,
            nameGap: 40,
            nameTextStyle: { color: chrome.mute, fontSize: 10 },
            splitLine: { show: showGrid, lineStyle: { color: chrome.surface2 } },
            axisLabel: { color: chrome.mute, fontSize: 11, formatter: axisFmt },
          };
          const series = seriesNames.map((m, i) => {
            const c = palette[i % palette.length];
            return {
              name: m,
              type: type === "area" ? "line" : type,
              data: seriesValues[i] || [],
              stack: stacked ? "total" : undefined,
              smooth,
              barMaxWidth: 36,
              label: {
                show: showLabels,
                position: horizontal ? "right" : "top",
                fontSize: 10,
                color: chrome.ink,
                formatter: (p: any) => formatNumber(p.value, config),
              },
              areaStyle:
                type === "area" || type === "line"
                  ? {
                      color: {
                        type: "linear",
                        x: 0,
                        y: 0,
                        x2: 0,
                        y2: 1,
                        colorStops: [
                          { offset: 0, color: hexToRgba(c, 0.22) },
                          { offset: 1, color: hexToRgba(c, 0.02) },
                        ],
                      },
                    }
                  : undefined,
              lineStyle: { color: c, width: 2 },
              itemStyle: { color: c, borderRadius: type === "bar" ? (horizontal ? [0, 6, 6, 0] : [6, 6, 0, 0]) : 0 },
            };
          });
          return {
            backgroundColor: "transparent",
            title: title ? { text: title, left: 0, textStyle: { color: chrome.mute, fontSize: 12, fontWeight: 500 } } : undefined,
            tooltip: showTooltip
              ? {
                  ...echartsTooltip(),
                  trigger: "axis",
                  formatter: (params: any) => {
                    const list = Array.isArray(params) ? params : [params];
                    const head = list[0]?.axisValueLabel ?? list[0]?.name ?? "";
                    const lines = list.map((p: any) => `${p.marker} ${p.seriesName}: ${formatNumber(p.value, config)}`);
                    return `<div class="text-xs font-medium">${head}</div>${lines.map((l: string) => `<div class="text-xs">${l}</div>`).join("")}`;
                  },
                }
              : { show: false },
            legend,
            grid: {
              left: (horizontal ? 16 : 52) + (legendPos === "left" ? sidePad : 0),
              right: 16 + (legendPos === "right" ? sidePad : 0),
              top: (title ? 28 : 12) + (legendPos === "top" ? legendPad : 0),
              bottom: 36 + (legendPos === "bottom" ? legendPad : 0),
            },
            xAxis: horizontal ? valueAxis : categoryAxis,
            yAxis: horizontal ? categoryAxis : valueAxis,
            series,
          };
        })();

  return <ReactECharts key={theme} option={option} style={{ height }} notMerge onEvents={{ click: handleClick }} />;
}

const KPI_SIZE = { sm: "text-2xl", md: "text-3xl", lg: "text-4xl" } as const;

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
    <Card className={cn("flex h-full flex-col justify-between", align === "center" && "text-center")}>
      {showTitle !== false && <div className="text-[12px] uppercase tracking-wide text-mute">{label}</div>}
      <div className={cn("mt-2 font-semibold tracking-tight text-ink", KPI_SIZE[fontSize || "md"])} style={color ? { color } : undefined}>
        {value}
      </div>
      {goalLabel && <div className="mt-1 text-[11px] text-mute">{goalLabel}</div>}
      {progress != null && (
        <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-surface-2">
          <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${Math.min(100, Math.max(0, progress))}%`, backgroundColor: color }} />
        </div>
      )}
      {delta !== undefined && (
        <div className={`mt-2 text-[12px] ${pos ? "text-ok" : "text-danger"}`}>
          {pos ? "+" : ""}
          {delta.toFixed(1)}% {comparisonLabel || "vs. período anterior"}
        </div>
      )}
    </Card>
  );
}

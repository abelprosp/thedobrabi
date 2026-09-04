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

export function EChart({
  option,
  onEvents,
}: {
  option: Record<string, unknown>;
  onEvents?: Record<string, (params: any) => void>;
}) {
  const { theme } = useTheme();
  return (
    <ReactECharts
      key={theme}
      option={option}
      notMerge
      lazyUpdate
      style={{ height: "100%", width: "100%" }}
      opts={{ renderer: "canvas" }}
      autoResize
      onEvents={onEvents}
    />
  );
}

export function Chart({ type = "bar", columns = [], rows = [], height, onClick, config = {} }: ChartProps) {
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
  const legendPad = showLegend && (legendPos === "top" || legendPos === "bottom") ? 28 : 0;
  const sidePad = showLegend && (legendPos === "left" || legendPos === "right") ? 80 : 0;

  const option =
    type === "pie"
      ? {
          backgroundColor: "transparent",
          animationDuration: 450,
          tooltip: showTooltip
            ? {
                ...echartsTooltip(),
                trigger: "item",
                formatter: (p: any) =>
                  `<div style="font-weight:600;margin-bottom:2px">${p.name}</div>${formatNumber(p.value, config)} · ${p.percent}%`,
              }
            : { show: false },
          legend,
          color: palette,
          series: [
            {
              type: "pie",
              radius: ["52%", "74%"],
              padAngle: 2,
              itemStyle: { borderRadius: 8, borderColor: chrome.surface, borderWidth: 2 },
              label: {
                show: showLabels,
                fontSize: 11,
                color: chrome.ink,
                formatter: (p: any) => `${p.name}\n${formatNumber(p.value, config)}`,
              },
              emphasis: { scale: true, scaleSize: 6 },
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
            nameGap: 30,
            nameTextStyle: { color: chrome.mute, fontSize: 11 },
            axisLine: { show: false },
            axisTick: { show: false },
            axisLabel: {
              color: chrome.mute,
              fontSize: 11,
              hideOverlap: true,
              rotate: horizontal ? 0 : (config.xAxisRotate ?? 0),
            },
          };
          const valueAxis = {
            type: "value" as const,
            show: horizontal ? showX : showY,
            name: config.yAxisLabel || undefined,
            nameLocation: "middle" as const,
            nameGap: 44,
            nameTextStyle: { color: chrome.mute, fontSize: 11 },
            splitLine: {
              show: showGrid,
              lineStyle: { color: chrome.line, type: "dashed" as const },
            },
            axisLine: { show: false },
            axisTick: { show: false },
            axisLabel: { color: chrome.mute, fontSize: 11, formatter: axisFmt },
          };
          const series = seriesNames.map((m, i) => {
            const c = palette[i % palette.length];
            const isBar = type === "bar";
            return {
              name: m,
              type: type === "area" ? "line" : type,
              data: seriesValues[i] || [],
              stack: stacked ? "total" : undefined,
              smooth,
              showSymbol: type !== "bar",
              symbol: "circle",
              symbolSize: 7,
              barMaxWidth: 52,
              barCategoryGap: "32%",
              emphasis: { focus: "series" },
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
                          { offset: 0, color: hexToRgba(c, type === "area" ? 0.32 : 0.16) },
                          { offset: 1, color: hexToRgba(c, 0.02) },
                        ],
                      },
                    }
                  : undefined,
              lineStyle: { color: c, width: 2.5 },
              itemStyle: {
                color: c,
                borderRadius: isBar ? (horizontal ? [0, 8, 8, 0] : [8, 8, 2, 2]) : 0,
              },
            };
          });
          return {
            backgroundColor: "transparent",
            animationDuration: 450,
            tooltip: showTooltip
              ? {
                  ...echartsTooltip(),
                  trigger: "axis",
                  axisPointer: { type: type === "bar" ? "shadow" : "line", shadowStyle: { color: "rgba(37,99,235,0.08)" } },
                  formatter: (params: any) => {
                    const list = Array.isArray(params) ? params : [params];
                    const head = list[0]?.axisValueLabel ?? list[0]?.name ?? "";
                    const lines = list.map(
                      (p: any) =>
                        `<div style="display:flex;gap:8px;align-items:center;margin-top:4px">${p.marker}<span>${p.seriesName}</span><span style="margin-left:auto;font-weight:600">${formatNumber(p.value, config)}</span></div>`,
                    );
                    return `<div style="font-weight:600;margin-bottom:4px">${head}</div>${lines.join("")}`;
                  },
                }
              : { show: false },
            legend,
            grid: {
              containLabel: true,
              left: 8 + (legendPos === "left" ? sidePad : 0),
              right: 12 + (legendPos === "right" ? sidePad : 0),
              top: 10 + (legendPos === "top" ? legendPad : 0),
              bottom: 8 + (legendPos === "bottom" ? legendPad : 0),
            },
            xAxis: horizontal ? valueAxis : categoryAxis,
            yAxis: horizontal ? categoryAxis : valueAxis,
            series,
          };
        })();

  const chart = <EChart option={option} onEvents={onClick ? { click: handleClick } : undefined} />;
  return (
    <div className="h-full w-full min-h-0" style={height == null ? undefined : { height }}>
      {chart}
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

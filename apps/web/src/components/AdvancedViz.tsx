"use client";

import { Card } from "@/components/ui";
import { useMemo } from "react";
import { BarChart3, Globe, Layers } from "lucide-react";
import { chartChrome, chartPalette, echartsTooltip, formatNumber, hexToRgba, legendOption } from "@/lib/widget-config";
import { useTheme } from "@/components/theme-provider";
import { EChart, formatCategory } from "@/components/viz";

export type Rows = Record<string, any>[];
export type Config = Record<string, any>;
export { formatNumber };

const PALETTE = ["#2563EB", "#6366F1", "#0EA5E9", "#F59E0B", "#8B5CF6", "#10B981", "#EF4444"];

function pickColumns(rows: Rows, columns: string[], config?: Config) {
  if (!rows.length || !columns.length) return { dim: undefined, measure: undefined, numericCols: [] as string[] };
  const numericCols = columns.filter((c) => typeof rows[0][c] === "number");
  const strCols = columns.filter((c) => typeof rows[0][c] === "string");
  const dim = config?.dimension || strCols[0] || columns[0];
  const measure = config?.measure || numericCols[0] || columns[1] || columns[0];
  return { dim, measure, numericCols };
}

function ChartFill({ option, height }: { option: Record<string, unknown>; height?: number }) {
  const chart = <EChart option={option} />;
  if (height == null) return chart;
  return <div style={{ height }}>{chart}</div>;
}

export function AdvancedChart({
  type,
  rows = [],
  columns = [],
  height,
  config = {},
  title: _title,
}: {
  type: "gauge" | "waterfall" | "funnel" | "scatter" | "treemap" | "heatmap";
  rows?: Rows;
  columns?: string[];
  height?: number;
  config?: Config;
  title?: string;
}) {
  const { theme } = useTheme();
  const option = useMemo(() => {
    const chrome = chartChrome();
    const tooltip = echartsTooltip();
    const palette = chartPalette(config.color);
    const showTooltip = config.showTooltip !== false;
    const showLabels = ["funnel", "treemap", "heatmap"].includes(type) ? config.showDataLabels !== false : !!config.showDataLabels;
    const showGrid = config.showGrid !== false;
    const showX = config.showXAxis !== false;
    const showY = config.showYAxis !== false;
    const axisFmt = (v: number) => formatNumber(v, config);
    const dashed = { color: chrome.line, type: "dashed" as const };

    if (type === "gauge") {
      const { measure } = pickColumns(rows, columns, config);
      const val = rows[0] ? Number(rows[0][measure ?? columns[0]] ?? 0) : 0;
      const min = Number(config.min ?? 0);
      const max = Number(config.max ?? Math.max(val * 1.2, 100));
      const target = Number(config.target ?? max * 0.8);
      const color = config.color || PALETTE[0];
      return {
        backgroundColor: "transparent",
        animationDuration: 450,
        series: [
          {
            type: "gauge",
            startAngle: 210,
            endAngle: -30,
            min,
            max,
            splitNumber: 5,
            itemStyle: { color },
            progress: { show: true, width: 18, roundCap: true, itemStyle: { color } },
            pointer: { show: true, length: "58%", width: 5, itemStyle: { color } },
            axisLine: { roundCap: true, lineStyle: { width: 18, color: [[1, chrome.line]] } },
            axisTick: { show: false },
            splitLine: { length: 10, distance: 8, lineStyle: { color: chrome.mute, width: 1 } },
            axisLabel: { show: showX, distance: 22, color: chrome.mute, fontSize: 10, formatter: axisFmt },
            anchor: { show: true, size: 12, itemStyle: { color, borderColor: chrome.surface, borderWidth: 2 } },
            title: { show: true, offsetCenter: [0, "38%"], color: chrome.mute, fontSize: 11 },
            detail: {
              valueAnimation: true,
              fontSize: 24,
              fontWeight: 600,
              offsetCenter: [0, "62%"],
              formatter: (v: number) => formatNumber(v, config),
              color: chrome.ink,
            },
            data: [{ value: val, name: config.gaugeLabel || "Valor" }],
            markLine:
              target != null
                ? {
                    silent: true,
                    data: [{ yAxis: target }],
                    lineStyle: { color: "#F59E0B", width: 2, type: "dashed" },
                    symbol: ["none", "none"],
                  }
                : undefined,
          },
        ],
      };
    }

    if (type === "waterfall") {
      const { dim, measure } = pickColumns(rows, columns, config);
      const negatives = new Set((config.waterfallNegativeCategories || "").split(",").map((s: string) => s.trim()).filter(Boolean));
      const cats = rows.map((r) => formatCategory(r[dim ?? columns[0]] ?? ""));
      const values = rows.map((r) => Number(r[measure ?? columns[1]] ?? 0));
      let sum = 0;
      const helper: number[] = [];
      const series: number[] = [];
      const colors: string[] = [];
      const pos = config.color || "#10B981";
      const neg = config.colorNegative || "#EF4444";
      values.forEach((v, i) => {
        const isNeg = v < 0 || negatives.has(cats[i]);
        const val = isNeg ? -Math.abs(v) : Math.abs(v);
        helper.push(sum);
        series.push(val);
        colors.push(isNeg ? neg : pos);
        sum += val;
      });
      helper.push(sum);
      series.push(0);
      colors.push("transparent");
      cats.push("Total");
      return {
        backgroundColor: "transparent",
        animationDuration: 450,
        tooltip: showTooltip
          ? {
              ...tooltip,
              trigger: "axis",
              axisPointer: { type: "shadow", shadowStyle: { color: "rgba(37,99,235,0.08)" } },
              formatter: (params: any) => {
                const p = params[0];
                const i = p.dataIndex;
                return `<div style="font-weight:600;margin-bottom:2px">${cats[i]}</div><div>${formatNumber(series[i], config)}</div>`;
              },
            }
          : { show: false },
        grid: { containLabel: true, left: 8, right: 12, top: 16, bottom: 8 },
        xAxis: {
          type: "category",
          show: showX,
          data: cats,
          name: config.xAxisLabel,
          nameTextStyle: { color: chrome.mute, fontSize: 11 },
          axisLine: { show: false },
          axisTick: { show: false },
          axisLabel: { color: chrome.mute, fontSize: 11, rotate: config.xAxisRotate ?? 0, hideOverlap: true },
        },
        yAxis: {
          type: "value",
          show: showY,
          name: config.yAxisLabel,
          nameTextStyle: { color: chrome.mute, fontSize: 11 },
          splitLine: { show: showGrid, lineStyle: dashed },
          axisLine: { show: false },
          axisTick: { show: false },
          axisLabel: { color: chrome.mute, fontSize: 11, formatter: axisFmt },
        },
        series: [
          { type: "bar", stack: "Total", itemStyle: { borderColor: "transparent", color: "transparent" }, emphasis: { itemStyle: { borderColor: "transparent", color: "transparent" } }, data: helper },
          {
            type: "bar",
            stack: "Total",
            data: series,
            itemStyle: { color: (p: any) => colors[p.dataIndex], borderRadius: [8, 8, 2, 2] },
            barMaxWidth: 48,
            barCategoryGap: "32%",
            label: { show: showLabels, position: "top", fontSize: 10, color: chrome.ink, formatter: (p: any) => formatNumber(p.value, config) },
          },
        ],
      };
    }

    if (type === "funnel") {
      const { dim, measure } = pickColumns(rows, columns, config);
      const data = rows
        .map((r) => ({ name: formatCategory(r[dim ?? columns[0]] ?? ""), value: Number(r[measure ?? columns[1]] ?? 0) }))
        .sort((a, b) => b.value - a.value);
      return {
        backgroundColor: "transparent",
        animationDuration: 450,
        tooltip: showTooltip ? { ...tooltip, trigger: "item", formatter: (p: any) => `${p.name}: ${formatNumber(p.value, config)}` } : { show: false },
        legend: legendOption(!!config.showLegend, config.legendPosition || "top"),
        color: palette,
        series: [
          {
            type: "funnel",
            left: "8%",
            top: 28,
            bottom: 16,
            width: "84%",
            minSize: "8%",
            maxSize: "100%",
            sort: "descending",
            gap: 6,
            label: { show: showLabels, color: chrome.ink, fontSize: 11, formatter: (p: any) => `${p.name}: ${formatNumber(p.value, config)}` },
            labelLine: { length: 12, lineStyle: { width: 1 } },
            itemStyle: { borderColor: chrome.surface, borderWidth: 2, borderRadius: 6 },
            emphasis: { label: { fontSize: 12 } },
            data,
          },
        ],
      };
    }

    if (type === "scatter") {
      const xCol = columns.includes(config.xMeasure) ? config.xMeasure : config.xMeasure || columns.find((c) => typeof rows[0]?.[c] === "number") || columns[0];
      const yCol = columns.includes(config.yMeasure) ? config.yMeasure : config.yMeasure || columns.find((c) => c !== xCol && typeof rows[0]?.[c] === "number") || columns[1] || xCol;
      const dim = config.dimension || columns.find((c) => typeof rows[0]?.[c] === "string" && c !== xCol && c !== yCol);
      const color = config.color || PALETTE[0];
      const data = rows.map((r) => [Number(r[xCol] ?? 0), Number(r[yCol] ?? 0), dim ? String(r[dim]) : ""]);
      return {
        backgroundColor: "transparent",
        animationDuration: 450,
        tooltip: showTooltip
          ? {
              ...tooltip,
              trigger: "item",
              formatter: (p: any) =>
                `<div style="font-weight:600;margin-bottom:2px">${p.data[2] || p.name}</div><div>${xCol}: ${formatNumber(p.data[0], config)}</div><div>${yCol}: ${formatNumber(p.data[1], config)}</div>`,
            }
          : { show: false },
        grid: { containLabel: true, left: 8, right: 16, top: 16, bottom: 8 },
        xAxis: {
          type: "value",
          show: showX,
          name: config.xAxisLabel || xCol,
          splitLine: { show: showGrid, lineStyle: dashed },
          axisLine: { show: false },
          axisTick: { show: false },
          axisLabel: { color: chrome.mute, fontSize: 11, formatter: axisFmt },
          nameTextStyle: { color: chrome.mute, fontSize: 11 },
        },
        yAxis: {
          type: "value",
          show: showY,
          name: config.yAxisLabel || yCol,
          splitLine: { show: showGrid, lineStyle: dashed },
          axisLine: { show: false },
          axisTick: { show: false },
          axisLabel: { color: chrome.mute, fontSize: 11, formatter: axisFmt },
          nameTextStyle: { color: chrome.mute, fontSize: 11 },
        },
        series: [
          {
            type: "scatter",
            symbolSize: 14,
            itemStyle: { color: hexToRgba(color, 0.78), borderColor: color, borderWidth: 1.5, shadowBlur: 8, shadowColor: hexToRgba(color, 0.25) },
            emphasis: { scale: 1.2 },
            label: { show: showLabels, fontSize: 10, color: chrome.ink, formatter: (p: any) => p.data[2] || formatNumber(p.data[1], config) },
            data,
          },
        ],
      };
    }

    if (type === "treemap") {
      const { dim, measure } = pickColumns(rows, columns, config);
      const data = rows.map((r) => ({ name: formatCategory(r[dim ?? columns[0]] ?? ""), value: Number(r[measure ?? columns[1]] ?? 0) }));
      return {
        backgroundColor: "transparent",
        animationDuration: 450,
        tooltip: showTooltip ? { ...tooltip, formatter: (p: any) => `${p.name}: ${formatNumber(p.value, config)}` } : { show: false },
        color: palette,
        series: [
          {
            type: "treemap",
            width: "100%",
            height: "100%",
            roam: false,
            nodeClick: false,
            breadcrumb: { show: false },
            label: { show: showLabels, fontSize: 11, formatter: (p: any) => `${p.name}\n${formatNumber(p.value, config)}` },
            itemStyle: { borderColor: chrome.surface, borderWidth: 3, gapWidth: 4, borderRadius: 8 },
            data,
          },
        ],
      };
    }

    if (type === "heatmap") {
      const xCol = columns.find((c) => typeof rows[0]?.[c] === "string") || columns[0];
      const yCol = columns.find((c) => c !== xCol && typeof rows[0]?.[c] === "string") || columns[1] || xCol;
      const valCol = columns.find((c) => typeof rows[0]?.[c] === "number") || columns[2] || columns[0];
      const xSet = Array.from(new Set(rows.map((r) => formatCategory(r[xCol]))));
      const ySet = Array.from(new Set(rows.map((r) => formatCategory(r[yCol]))));
      const data = rows.map((r) => [xSet.indexOf(formatCategory(r[xCol])), ySet.indexOf(formatCategory(r[yCol])), Number(r[valCol] ?? 0)]);
      const max = Math.max(...data.map((d) => d[2]), 0);
      const accent = config.color || "#0ea5e9";
      return {
        backgroundColor: "transparent",
        animationDuration: 450,
        tooltip: showTooltip
          ? {
              ...tooltip,
              position: "top",
              formatter: (p: any) =>
                `<div style="font-weight:600;margin-bottom:2px">${xSet[p.data[0]]} / ${ySet[p.data[1]]}</div><div>${formatNumber(p.data[2], config)}</div>`,
            }
          : { show: false },
        grid: { containLabel: true, left: 8, right: 16, top: 12, bottom: 36 },
        xAxis: {
          type: "category",
          show: showX,
          data: xSet,
          name: config.xAxisLabel,
          splitArea: { show: true },
          axisLine: { show: false },
          axisTick: { show: false },
          axisLabel: { color: chrome.mute, fontSize: 10, rotate: config.xAxisRotate ?? 30 },
        },
        yAxis: {
          type: "category",
          show: showY,
          data: ySet,
          name: config.yAxisLabel,
          splitArea: { show: true },
          axisLine: { show: false },
          axisTick: { show: false },
          axisLabel: { color: chrome.mute, fontSize: 10 },
        },
        visualMap: {
          min: 0,
          max,
          calculable: true,
          orient: "horizontal",
          left: "center",
          bottom: 0,
          inRange: { color: ["#e0f2fe", accent, "#1e40af"] },
          textStyle: { color: chrome.mute, fontSize: 10 },
        },
        series: [
          {
            type: "heatmap",
            itemStyle: { borderRadius: 4, borderColor: chrome.surface, borderWidth: 2 },
            label: { show: showLabels, fontSize: 10, formatter: (p: any) => formatNumber(p.data[2], config) },
            data,
          },
        ],
      };
    }

    return {};
  }, [type, rows, columns, config, theme]);

  return <ChartFill option={option} height={height} />;
}

export function Sparkline({ rows = [], columns = [], height, config = {} }: { rows?: Rows; columns?: string[]; height?: number; config?: Config }) {
  const { theme } = useTheme();
  const dim = columns.find((c) => typeof rows[0]?.[c] === "string") || columns[0];
  const meas = columns.find((c) => typeof rows[0]?.[c] === "number") || columns[1] || columns[0];
  const data = rows.map((r) => Number(r[meas] ?? 0));
  const color = config.color || PALETTE[0];
  const tooltip = echartsTooltip();
  const option = {
    backgroundColor: "transparent",
    animationDuration: 350,
    grid: { left: 4, right: 4, top: 8, bottom: 4 },
    xAxis: { type: "category", show: false, data: rows.map((r) => formatCategory(r[dim ?? ""])) },
    yAxis: { type: "value", show: false },
    tooltip:
      config.showTooltip === false
        ? { show: false }
        : { ...tooltip, trigger: "axis", formatter: (p: any) => formatNumber(p?.[0]?.value, config) },
    series: [
      {
        type: "line",
        data,
        smooth: config.smooth !== false,
        showSymbol: false,
        lineStyle: { color, width: 2.5 },
        areaStyle: {
          color: {
            type: "linear",
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: hexToRgba(color, 0.28) },
              { offset: 1, color: hexToRgba(color, 0.02) },
            ],
          },
        },
      },
    ],
  };
  void theme;
  return <ChartFill option={option} height={height} />;
}

export function KpiGoal({ label, value, goal, variance, config = {} }: { label: string; value: any; goal?: any; variance?: any; config?: Config }) {
  const val = Number(value ?? 0);
  const g = Number(goal ?? config.goal ?? 0);
  const v = variance !== undefined ? Number(variance) : config.variance !== undefined ? Number(config.variance) : g !== 0 ? ((val - g) / g) * 100 : 0;
  const pct = g > 0 ? Math.min(100, Math.max(0, (val / g) * 100)) : 0;
  const positive = v >= 0;
  const size = config.fontSize === "sm" ? "text-2xl" : config.fontSize === "lg" ? "text-4xl" : "text-3xl";
  return (
    <Card className={`flex h-full flex-col justify-between p-4 ${config.kpiAlign === "center" ? "text-center" : ""}`}>
      {config.showTitle !== false && <div className="text-[12px] uppercase tracking-wide text-mute">{label}</div>}
      <div className={`mt-2 font-semibold tracking-tight text-ink ${size}`} style={config.color ? { color: config.color } : undefined}>
        {formatNumber(value, config)}
      </div>
      <div className="mt-3 space-y-1.5">
        <div className={`flex items-center justify-between text-[11px] text-mute ${config.kpiAlign === "center" ? "justify-center gap-3" : ""}`}>
          <span>Meta: {formatNumber(g, config)}</span>
          {config.showTrend !== false && (
            <span className={positive ? "text-ok" : "text-danger"}>
              {positive ? "+" : ""}
              {v.toFixed(1)}% {config.comparisonLabel || ""}
            </span>
          )}
        </div>
        <div className="h-2 w-full overflow-hidden rounded-full bg-surface-2">
          <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${pct}%`, backgroundColor: config.color }} />
        </div>
      </div>
    </Card>
  );
}

export function MetricGroup({ label, rows = [], columns = [], config = {} }: { label: string; rows?: Rows; columns?: string[]; config?: Config }) {
  const numericCols = columns.filter((c) => rows.length && typeof rows[0][c] === "number");
  const cols = numericCols.slice(0, 4);
  if (cols.length === 0 && rows.length) cols.push(columns[0]);
  return (
    <Card className="flex h-full flex-col p-4">
      <div className="text-[12px] uppercase tracking-wide text-mute">{label}</div>
      <div className="mt-3 grid grid-cols-2 gap-3">
        {cols.map((c) => (
          <div key={c}>
            <div className="text-[11px] text-mute">{c}</div>
            <div className="mt-1 text-xl font-semibold text-ink">{formatNumber(rows[0]?.[c] ?? 0, config)}</div>
          </div>
        ))}
      </div>
    </Card>
  );
}

export function DecompositionTree({
  rows = [],
  columns = [],
  hierarchy = [] as string[],
  drillPath = [] as string[],
  onDrill,
  config = {},
}: {
  rows?: Rows;
  columns?: string[];
  hierarchy?: string[];
  drillPath?: string[];
  onDrill?: (value: string) => void;
  config?: Config;
}) {
  const level = drillPath.length < hierarchy.length ? drillPath.length : hierarchy.length - 1;
  const dim = hierarchy[level] || columns[0];
  const measure = columns.find((c) => typeof rows[0]?.[c] === "number") || columns[1] || columns[0];
  const values = useMemo(() => {
    const map = new Map<string, number>();
    rows.forEach((r) => {
      const k = String(r[dim] ?? "Outros");
      map.set(k, (map.get(k) || 0) + Number(r[measure] ?? 0));
    });
    return Array.from(map.entries()).sort((a, b) => b[1] - a[1]).slice(0, 12);
  }, [rows, dim, measure]);
  const max = values[0]?.[1] || 1;
  return (
    <div className="flex h-full flex-col rounded-2xl border border-line bg-surface p-4 shadow-sm">
      <div className="mb-2 text-[13px] font-medium text-ink">
        {dim}
        {drillPath.length > 0 && (
          <button className="ml-2 text-[11px] text-accent" onClick={() => onDrill?.("up")}>
            Subir
          </button>
        )}
      </div>
      <div className="flex-1 space-y-1.5 overflow-auto">
        {values.length === 0 ? (
          <p className="text-xs text-mute">Sem dados.</p>
        ) : (
          values.map(([name, val]) => (
            <button
              key={name}
              type="button"
              className="relative flex w-full items-center justify-between overflow-hidden rounded-lg px-2 py-1.5 text-left hover:bg-surface-2"
              onClick={() => onDrill?.(name)}
            >
              <span
                className="absolute inset-y-0 left-0 bg-primary/10"
                style={{ width: `${Math.max(4, (val / max) * 100)}%` }}
              />
              <div className="relative z-[1] flex items-center gap-2">
                {drillPath.length < hierarchy.length - 1 ? <Layers size={14} className="text-primary" /> : <BarChart3 size={14} className="text-mute" />}
                <span className="text-[12px] text-ink">{name}</span>
              </div>
              <span className="relative z-[1] text-[12px] font-medium tabular-nums text-ink">{formatNumber(val, config)}</span>
            </button>
          ))
        )}
      </div>
    </div>
  );
}

export function IframeWidget({ url, title }: { url?: string; title?: string }) {
  const origin = typeof window !== "undefined" ? window.location.origin : "";
  const isExternal = !!(url && origin && !url.startsWith(origin) && !url.startsWith("/"));
  return (
    <div className="flex h-full flex-col rounded-2xl border border-line bg-surface shadow-sm">
      <div className="flex items-center justify-between border-b border-line px-3 py-2">
        <span className="text-[13px] font-medium text-ink">{title || "Embed"}</span>
        {isExternal && <span className="text-[10px] text-warn">conteúdo externo</span>}
      </div>
      <div className="min-h-0 flex-1 p-2">
        {url ? (
          <iframe src={url} title={title || "embed"} className="h-full w-full rounded-xl" sandbox="allow-scripts allow-same-origin" />
        ) : (
          <div className="flex h-full flex-col items-center justify-center gap-2 text-mute">
            <Globe size={28} />
            <p className="text-xs">Insira uma URL no painel lateral.</p>
          </div>
        )}
      </div>
      {isExternal && (
        <div className="border-t border-line px-3 py-2 text-[10px] text-mute">
          Aviso: websites externos podem definir cookies ou recolher dados. Use apenas fontes confiáveis.
        </div>
      )}
    </div>
  );
}

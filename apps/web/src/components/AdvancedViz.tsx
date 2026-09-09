"use client";

import { Card } from "@/components/ui";
import { useMemo } from "react";
import { BarChart3, Globe, Layers } from "lucide-react";
import { chartChrome, chartPalette, formatNumber, hexToRgba } from "@/lib/widget-config";
import { chartTooltip } from "@/lib/chartjs";
import { ChartJsCanvas } from "@/components/chartjs-canvas";
import { useTheme } from "@/components/theme-provider";
import { formatCategory } from "@/components/viz";
import type { ChartType } from "chart.js";

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

function Fill({ type, data, options, height }: { type: ChartType; data: any; options?: any; height?: number }) {
  const chart = <ChartJsCanvas type={type} data={data} options={options} />;
  if (height == null) return chart;
  return <div style={{ height }}>{chart}</div>;
}

export function AdvancedChart({
  type,
  rows = [],
  columns = [],
  height,
  config = {},
}: {
  type: "gauge" | "waterfall" | "funnel" | "scatter" | "treemap" | "heatmap";
  rows?: Rows;
  columns?: string[];
  height?: number;
  config?: Config;
  title?: string;
}) {
  const { theme } = useTheme();
  const chrome = chartChrome(theme);
  const palette = chartPalette(config.color);
  const showTooltip = config.showTooltip !== false;
  const showGrid = config.showGrid !== false;
  const showX = config.showXAxis !== false;
  const showY = config.showYAxis !== false;
  const axisFmt = (v: string | number) => formatNumber(Number(v), config);
  const tip = showTooltip ? chartTooltip(theme) : { enabled: false };

  if (type === "gauge") {
    const { measure } = pickColumns(rows, columns, config);
    const val = rows[0] ? Number(rows[0][measure ?? columns[0]] ?? 0) : 0;
    const min = Number(config.min ?? 0);
    const max = Number(config.max ?? Math.max(val * 1.2, 100));
    const color = config.color || PALETTE[0];
    const span = Math.max(max - min, 1);
    const filled = Math.min(span, Math.max(0, val - min));
    const rest = Math.max(0, span - filled);
    return (
      <div className="relative h-full w-full" style={height ? { height } : undefined}>
        <ChartJsCanvas
          type="doughnut"
          data={{
            labels: [config.gaugeLabel || "Valor", "Restante"],
            datasets: [
              {
                data: [filled, rest],
                backgroundColor: [color, chrome.line],
                borderWidth: 0,
                circumference: 270,
                rotation: 225,
                cutout: "72%",
              },
            ],
          }}
          options={{
            plugins: { legend: { display: false }, tooltip: { enabled: false } },
          }}
        />
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center pt-4">
          <div className="text-[11px] text-mute">{config.gaugeLabel || "Valor"}</div>
          <div className="text-2xl font-semibold text-ink">{formatNumber(val, config)}</div>
        </div>
      </div>
    );
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
      helper.push(isNeg ? sum + val : sum);
      series.push(Math.abs(val));
      colors.push(isNeg ? neg : pos);
      sum += val;
    });
    helper.push(0);
    series.push(Math.abs(sum));
    colors.push("#6366F1");
    cats.push("Total");
    return (
      <Fill
        height={height}
        type="bar"
        data={{
          labels: cats,
          datasets: [
            { label: "base", data: helper, backgroundColor: "transparent", stack: "wf", borderWidth: 0 },
            { label: "valor", data: series, backgroundColor: colors, stack: "wf", borderRadius: 6, maxBarThickness: 48 },
          ],
        }}
        options={{
          plugins: {
            legend: { display: false },
            tooltip: {
              ...tip,
              callbacks: { label: (ctx: any) => (ctx.datasetIndex === 0 ? "" : formatNumber(ctx.parsed.y, config)) },
            },
          },
          scales: {
            x: { stacked: true, display: showX, ticks: { color: chrome.mute, maxRotation: config.xAxisRotate ?? 0 }, grid: { display: false }, border: { display: false } },
            y: { stacked: true, display: showY, ticks: { color: chrome.mute, callback: (v: any) => axisFmt(v as number) }, grid: { display: showGrid, color: chrome.line }, border: { display: false } },
          },
        }}
      />
    );
  }

  if (type === "funnel") {
    const { dim, measure } = pickColumns(rows, columns, config);
    const items = rows
      .map((r) => ({ name: formatCategory(r[dim ?? columns[0]] ?? ""), value: Number(r[measure ?? columns[1]] ?? 0) }))
      .sort((a, b) => b.value - a.value);
    const max = Math.max(...items.map((d) => d.value), 1);
    return (
      <Fill
        height={height}
        type="bar"
        data={{
          labels: items.map((d) => d.name),
          datasets: [
            { label: "pad", data: items.map((d) => (max - d.value) / 2), backgroundColor: "transparent", stack: "fn", borderWidth: 0 },
            {
              label: "valor",
              data: items.map((d) => d.value),
              backgroundColor: items.map((_, i) => palette[i % palette.length]),
              stack: "fn",
              borderRadius: 6,
            },
          ],
        }}
        options={{
          indexAxis: "y",
          plugins: {
            legend: { display: false },
            tooltip: { ...tip, callbacks: { label: (ctx: any) => (ctx.datasetIndex === 0 ? "" : formatNumber(ctx.parsed.x, config)) } },
          },
          scales: {
            x: { stacked: true, display: false, border: { display: false }, grid: { display: false } },
            y: { stacked: true, ticks: { color: chrome.mute }, grid: { display: false }, border: { display: false } },
          },
        }}
      />
    );
  }

  if (type === "scatter") {
    const xCol = columns.includes(config.xMeasure) ? config.xMeasure : config.xMeasure || columns.find((c) => typeof rows[0]?.[c] === "number") || columns[0];
    const yCol = columns.includes(config.yMeasure) ? config.yMeasure : config.yMeasure || columns.find((c) => c !== xCol && typeof rows[0]?.[c] === "number") || columns[1] || xCol;
    const dim = config.dimension || columns.find((c) => typeof rows[0]?.[c] === "string" && c !== xCol && c !== yCol);
    const color = config.color || PALETTE[0];
    return (
      <Fill
        height={height}
        type="scatter"
        data={{
          datasets: [
            {
              label: yCol,
              data: rows.map((r) => ({ x: Number(r[xCol] ?? 0), y: Number(r[yCol] ?? 0), label: dim ? String(r[dim]) : "" })),
              backgroundColor: hexToRgba(color, 0.78),
              borderColor: color,
              pointRadius: 6,
            },
          ],
        }}
        options={{
          plugins: {
            legend: { display: false },
            tooltip: {
              ...tip,
              callbacks: {
                label: (ctx: any) => {
                  const p = ctx.raw as { x: number; y: number; label?: string };
                  return `${p.label || ""} ${xCol}: ${formatNumber(p.x, config)} · ${yCol}: ${formatNumber(p.y, config)}`;
                },
              },
            },
          },
          scales: {
            x: {
              display: showX,
              title: { display: true, text: config.xAxisLabel || xCol, color: chrome.mute },
              ticks: { color: chrome.mute, callback: (v: any) => axisFmt(v as number) },
              grid: { display: showGrid, color: chrome.line },
              border: { display: false },
            },
            y: {
              display: showY,
              title: { display: true, text: config.yAxisLabel || yCol, color: chrome.mute },
              ticks: { color: chrome.mute, callback: (v: any) => axisFmt(v as number) },
              grid: { display: showGrid, color: chrome.line },
              border: { display: false },
            },
          },
        }}
      />
    );
  }

  if (type === "treemap") {
    const { dim, measure } = pickColumns(rows, columns, config);
    const items = rows.map((r) => ({ name: formatCategory(r[dim ?? columns[0]] ?? ""), value: Number(r[measure ?? columns[1]] ?? 0) }));
    const total = items.reduce((s, d) => s + d.value, 0) || 1;
    return (
      <div className="grid h-full min-h-[8rem] grid-cols-6 grid-rows-4 gap-1" style={height ? { height } : undefined}>
        {items.slice(0, 12).map((d, i) => {
          const share = d.value / total;
          const span = Math.max(1, Math.round(share * 12));
          return (
            <div
              key={`${d.name}-${i}`}
              className="flex flex-col justify-end overflow-hidden rounded-xl p-2 text-white"
              style={{
                backgroundColor: palette[i % palette.length],
                gridColumn: `span ${Math.min(3, Math.max(1, Math.ceil(span / 2)))}`,
                gridRow: `span ${span > 4 ? 2 : 1}`,
              }}
            >
              <span className="truncate text-[11px] font-medium">{d.name}</span>
              <span className="text-[10px] opacity-90">{formatNumber(d.value, config)}</span>
            </div>
          );
        })}
      </div>
    );
  }

  if (type === "heatmap") {
    const xCol = columns.find((c) => typeof rows[0]?.[c] === "string") || columns[0];
    const yCol = columns.find((c) => c !== xCol && typeof rows[0]?.[c] === "string") || columns[1] || xCol;
    const valCol = columns.find((c) => typeof rows[0]?.[c] === "number") || columns[2] || columns[0];
    const xSet = Array.from(new Set(rows.map((r) => formatCategory(r[xCol]))));
    const ySet = Array.from(new Set(rows.map((r) => formatCategory(r[yCol]))));
    const max = Math.max(...rows.map((r) => Number(r[valCol] ?? 0)), 1);
    const accent = config.color || "#0ea5e9";
    return (
      <Fill
        height={height}
        type="bubble"
        data={{
          datasets: [
            {
              label: valCol,
              data: rows.map((r) => ({
                x: xSet.indexOf(formatCategory(r[xCol])),
                y: ySet.indexOf(formatCategory(r[yCol])),
                r: 6 + (Number(r[valCol] ?? 0) / max) * 14,
                v: Number(r[valCol] ?? 0),
                xl: formatCategory(r[xCol]),
                yl: formatCategory(r[yCol]),
              })),
              backgroundColor: hexToRgba(accent, 0.7),
              borderColor: accent,
            },
          ],
        }}
        options={{
          plugins: {
            legend: { display: false },
            tooltip: {
              ...tip,
              callbacks: {
                label: (ctx: any) => {
                  const p = ctx.raw as { xl: string; yl: string; v: number };
                  return `${p.xl} / ${p.yl}: ${formatNumber(p.v, config)}`;
                },
              },
            },
          },
          scales: {
            x: { type: "linear", min: -0.5, max: Math.max(xSet.length - 0.5, 0.5), ticks: { color: chrome.mute, callback: (v: any) => xSet[Number(v)] || "" }, grid: { color: chrome.line }, border: { display: false } },
            y: { type: "linear", min: -0.5, max: Math.max(ySet.length - 0.5, 0.5), ticks: { color: chrome.mute, callback: (v: any) => ySet[Number(v)] || "" }, grid: { color: chrome.line }, border: { display: false } },
          },
        }}
      />
    );
  }

  return null;
}

export function Sparkline({ rows = [], columns = [], height, config = {} }: { rows?: Rows; columns?: string[]; height?: number; config?: Config }) {
  const { theme } = useTheme();
  const meas = columns.find((c) => typeof rows[0]?.[c] === "number") || columns[1] || columns[0];
  const dim = columns.find((c) => typeof rows[0]?.[c] === "string") || columns[0];
  const color = config.color || PALETTE[0];
  return (
    <Fill
      height={height}
      type="line"
      data={{
        labels: rows.map((r) => formatCategory(r[dim ?? ""])),
        datasets: [
          {
            data: rows.map((r) => Number(r[meas] ?? 0)),
            borderColor: color,
            backgroundColor: hexToRgba(color, 0.28),
            fill: true,
            tension: config.smooth !== false ? 0.4 : 0,
            pointRadius: 0,
            borderWidth: 2.5,
          },
        ],
      }}
      options={{
        plugins: {
          legend: { display: false },
          tooltip: config.showTooltip === false ? { enabled: false } : { ...chartTooltip(theme), callbacks: { label: (ctx: any) => formatNumber(ctx.parsed.y, config) } },
        },
        scales: { x: { display: false }, y: { display: false } },
      }}
    />
  );
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
              <span className="absolute inset-y-0 left-0 bg-primary/10" style={{ width: `${Math.max(4, (val / max) * 100)}%` }} />
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

"use client";

import ReactECharts from "echarts-for-react";
import { Card } from "@/components/ui";
import { useMemo } from "react";
import { BarChart3, Globe, Layers } from "lucide-react";

export type Rows = Record<string, any>[];
export type Config = Record<string, any>;

const PALETTE = ["#2563EB", "#6366F1", "#0EA5E9", "#F59E0B", "#8B5CF6", "#10B981", "#EF4444"];

const tooltip = {
  backgroundColor: "#ffffff",
  borderColor: "#e2e8f0",
  textStyle: { color: "#0f172a", fontSize: 12 },
  extraCssText: "box-shadow: 0 8px 24px rgba(15,23,42,0.08); border-radius: 10px;",
};

export function formatNumber(value: any, config?: Config) {
  const n = Number(value);
  if (Number.isNaN(n) || value == null) return "—";
  const d = config?.decimals ?? 0;
  const s = n.toLocaleString("pt-BR", { minimumFractionDigits: d, maximumFractionDigits: d });
  return `${config?.prefix || ""}${s}${config?.suffix || ""}`;
}

function pickColumns(rows: Rows, columns: string[], config?: Config) {
  if (!rows.length || !columns.length) return { dim: undefined, measure: undefined, numericCols: [] as string[] };
  const numericCols = columns.filter((c) => typeof rows[0][c] === "number");
  const strCols = columns.filter((c) => typeof rows[0][c] === "string");
  const dim = config?.dimension || strCols[0] || columns[0];
  const measure = config?.measure || numericCols[0] || columns[1] || columns[0];
  return { dim, measure, numericCols };
}

export function AdvancedChart({
  type,
  rows = [],
  columns = [],
  height = 280,
  config = {},
  title,
}: {
  type: "gauge" | "waterfall" | "funnel" | "scatter" | "treemap" | "heatmap";
  rows?: Rows;
  columns?: string[];
  height?: number;
  config?: Config;
  title?: string;
}) {
  const option = useMemo(() => {
    if (type === "gauge") {
      const { measure } = pickColumns(rows, columns, config);
      const val = rows[0] ? Number(rows[0][measure ?? columns[0]] ?? 0) : 0;
      const min = Number(config.min ?? 0);
      const max = Number(config.max ?? Math.max(val * 1.2, 100));
      const target = Number(config.target ?? max * 0.8);
      const color = config.color || PALETTE[0];
      return {
        backgroundColor: "transparent",
        title: title ? { text: title, left: "center", top: 8, textStyle: { color: "#5b6470", fontSize: 12, fontWeight: 500 } } : undefined,
        series: [
          {
            type: "gauge",
            startAngle: 200,
            endAngle: -20,
            min,
            max,
            splitNumber: 5,
            itemStyle: { color },
            progress: { show: true, width: 18 },
            pointer: { show: true, width: 4 },
            axisLine: { lineStyle: { width: 18, color: [[1, "#e2e8f0"]] } },
            axisTick: { show: false },
            splitLine: { length: 8, lineStyle: { color: "#cbd5e1" } },
            axisLabel: { distance: 20, color: "#5b6470", fontSize: 10 },
            anchor: { show: true, size: 10, itemStyle: { color } },
            title: { show: true, offsetCenter: [0, "40%"], color: "#5b6470", fontSize: 11 },
            detail: {
              valueAnimation: true,
              fontSize: 22,
              offsetCenter: [0, "60%"],
              formatter: (v: number) => formatNumber(v, config),
              color: "#0f172a",
            },
            data: [{ value: val, name: config.gaugeLabel || "Valor" }],
            markLine: target != null ? {
              data: [{ yAxis: target }],
              lineStyle: { color: "#F59E0B", width: 2 },
              symbol: ["none", "none"],
            } : undefined,
          },
        ],
      };
    }

    if (type === "waterfall") {
      const { dim, measure } = pickColumns(rows, columns, config);
      const negatives = new Set((config.waterfallNegativeCategories || "").split(",").map((s: string) => s.trim()).filter(Boolean));
      const cats = rows.map((r) => String(r[dim ?? columns[0]] ?? ""));
      const values = rows.map((r) => Number(r[measure ?? columns[1]] ?? 0));
      let sum = 0;
      const helper: number[] = [];
      const series: number[] = [];
      const colors: string[] = [];
      values.forEach((v, i) => {
        const isNeg = v < 0 || negatives.has(cats[i]);
        const val = isNeg ? -Math.abs(v) : Math.abs(v);
        helper.push(sum);
        series.push(val);
        colors.push(isNeg ? "#EF4444" : "#10B981");
        sum += val;
      });
      helper.push(sum); series.push(0); colors.push("transparent");
      cats.push("Total");
      return {
        backgroundColor: "transparent",
        title: title ? { text: title, left: 0, textStyle: { color: "#5b6470", fontSize: 12, fontWeight: 500 } } : undefined,
        tooltip: { ...tooltip, trigger: "axis", axisPointer: { type: "shadow" }, formatter: (params: any) => {
          const p = params[0];
          const i = p.dataIndex;
          return `<div class="font-medium text-xs">${cats[i]}</div><div class="text-xs">${formatNumber(series[i], config)}</div>`;
        }},
        grid: { left: 52, right: 16, top: title ? 36 : 16, bottom: 36 },
        xAxis: { type: "category", data: cats, axisLine: { lineStyle: { color: "#e2e8f0" } }, axisLabel: { color: "#5b6470", fontSize: 11 } },
        yAxis: { type: "value", splitLine: { lineStyle: { color: "#f1f5f9" } }, axisLabel: { color: "#5b6470", fontSize: 11 } },
        series: [
          { type: "bar", stack: "Total", itemStyle: { borderColor: "transparent", color: "transparent" }, emphasis: { itemStyle: { borderColor: "transparent", color: "transparent" } }, data: helper },
          { type: "bar", stack: "Total", data: series, itemStyle: { color: (p: any) => colors[p.dataIndex] }, barMaxWidth: 36 },
        ],
      };
    }

    if (type === "funnel") {
      const { dim, measure } = pickColumns(rows, columns, config);
      const data = rows.map((r) => ({ name: String(r[dim ?? columns[0]] ?? ""), value: Number(r[measure ?? columns[1]] ?? 0) })).sort((a, b) => b.value - a.value);
      return {
        backgroundColor: "transparent",
        title: title ? { text: title, left: 0, textStyle: { color: "#5b6470", fontSize: 12, fontWeight: 500 } } : undefined,
        tooltip: { ...tooltip, trigger: "item", formatter: "{b}: {c}" },
        color: PALETTE,
        series: [{ type: "funnel", left: "10%", top: 24, bottom: 16, width: "80%", minSize: "0%", maxSize: "100%", sort: "descending", gap: 2, label: { show: true, color: "#0f172a", fontSize: 11 }, labelLine: { length: 10, lineStyle: { width: 1, type: "solid" } }, itemStyle: { borderColor: "#fff", borderWidth: 1 }, emphasis: { label: { fontSize: 12 } }, data }],
      };
    }

    if (type === "scatter") {
      const xCol = columns.includes(config.xMeasure) ? config.xMeasure : config.xMeasure || columns.find((c) => typeof rows[0]?.[c] === "number") || columns[0];
      const yCol = columns.includes(config.yMeasure) ? config.yMeasure : config.yMeasure || columns.find((c) => c !== xCol && typeof rows[0]?.[c] === "number") || columns[1] || xCol;
      const dim = config.dimension || columns.find((c) => typeof rows[0]?.[c] === "string" && c !== xCol && c !== yCol);
      const data = rows.map((r) => [Number(r[xCol] ?? 0), Number(r[yCol] ?? 0), dim ? String(r[dim]) : ""]);
      return {
        backgroundColor: "transparent",
        title: title ? { text: title, left: 0, textStyle: { color: "#5b6470", fontSize: 12, fontWeight: 500 } } : undefined,
        tooltip: { ...tooltip, trigger: "item", formatter: (p: any) => `<div class="text-xs font-medium">${p.data[2] || p.name}</div><div class="text-xs">${xCol}: ${formatNumber(p.data[0], config)}</div><div class="text-xs">${yCol}: ${formatNumber(p.data[1], config)}</div>` },
        grid: { left: 52, right: 16, top: title ? 36 : 16, bottom: 36 },
        xAxis: { type: "value", splitLine: { lineStyle: { color: "#f1f5f9" } }, axisLabel: { color: "#5b6470", fontSize: 11 }, name: xCol, nameTextStyle: { color: "#5b6470", fontSize: 10 } },
        yAxis: { type: "value", splitLine: { lineStyle: { color: "#f1f5f9" } }, axisLabel: { color: "#5b6470", fontSize: 11 }, name: yCol, nameTextStyle: { color: "#5b6470", fontSize: 10 } },
        series: [{ type: "scatter", symbolSize: 12, itemStyle: { color: config.color || PALETTE[0] }, data }],
      };
    }

    if (type === "treemap") {
      const { dim, measure } = pickColumns(rows, columns, config);
      const data = rows.map((r) => ({ name: String(r[dim ?? columns[0]] ?? ""), value: Number(r[measure ?? columns[1]] ?? 0) }));
      return {
        backgroundColor: "transparent",
        title: title ? { text: title, left: 0, textStyle: { color: "#5b6470", fontSize: 12, fontWeight: 500 } } : undefined,
        tooltip: { ...tooltip, formatter: "{b}: {c}" },
        color: PALETTE,
        series: [{ type: "treemap", width: "100%", height: "100%", roam: false, nodeClick: false, breadcrumb: { show: false }, label: { show: true, fontSize: 11 }, itemStyle: { borderColor: "#fff", borderWidth: 2, gapWidth: 2 }, data }],
      };
    }

    if (type === "heatmap") {
      const xCol = columns.find((c) => typeof rows[0]?.[c] === "string") || columns[0];
      const yCol = columns.find((c) => c !== xCol && typeof rows[0]?.[c] === "string") || columns[1] || xCol;
      const valCol = columns.find((c) => typeof rows[0]?.[c] === "number") || columns[2] || columns[0];
      const xSet = Array.from(new Set(rows.map((r) => String(r[xCol]))));
      const ySet = Array.from(new Set(rows.map((r) => String(r[yCol]))));
      const data = rows.map((r) => [xSet.indexOf(String(r[xCol])), ySet.indexOf(String(r[yCol])), Number(r[valCol] ?? 0)]);
      const max = Math.max(...data.map((d) => d[2]), 0);
      return {
        backgroundColor: "transparent",
        title: title ? { text: title, left: 0, textStyle: { color: "#5b6470", fontSize: 12, fontWeight: 500 } } : undefined,
        tooltip: { ...tooltip, position: "top", formatter: (p: any) => `<div class="text-xs font-medium">${xSet[p.data[0]]} / ${ySet[p.data[1]]}</div><div class="text-xs">${formatNumber(p.data[2], config)}</div>` },
        grid: { left: 80, right: 24, top: title ? 36 : 16, bottom: 48 },
        xAxis: { type: "category", data: xSet, splitArea: { show: true }, axisLabel: { color: "#5b6470", fontSize: 10, rotate: 30 } },
        yAxis: { type: "category", data: ySet, splitArea: { show: true }, axisLabel: { color: "#5b6470", fontSize: 10 } },
        visualMap: { min: 0, max, calculable: true, orient: "horizontal", left: "center", bottom: 4, inRange: { color: ["#e0f2fe", "#0ea5e9", "#1e40af"] }, textStyle: { color: "#5b6470", fontSize: 10 } },
        series: [{ type: "heatmap", label: { show: true, fontSize: 10 }, data }],
      };
    }

    return {};
  }, [type, rows, columns, config, title]);

  return <ReactECharts option={option} style={{ height }} notMerge />;
}

export function Sparkline({ rows = [], columns = [], height = 60, config = {} }: { rows?: Rows; columns?: string[]; height?: number; config?: Config }) {
  const dim = columns.find((c) => typeof rows[0]?.[c] === "string") || columns[0];
  const meas = columns.find((c) => typeof rows[0]?.[c] === "number") || columns[1] || columns[0];
  const data = rows.map((r) => Number(r[meas] ?? 0));
  const option = {
    backgroundColor: "transparent",
    grid: { left: 0, right: 0, top: 4, bottom: 4 },
    xAxis: { type: "category", show: false, data: rows.map((r) => r[dim ?? ""]) },
    yAxis: { type: "value", show: false },
    tooltip: { show: false },
    series: [{ type: "line", data, smooth: true, showSymbol: false, lineStyle: { color: config.color || PALETTE[0], width: 2 }, areaStyle: { color: { type: "linear", x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: "rgba(37,99,235,0.2)" }, { offset: 1, color: "rgba(37,99,235,0.02)" }] } } }],
  };
  return <ReactECharts option={option} style={{ height }} notMerge />;
}

export function KpiGoal({ label, value, goal, variance, config = {} }: { label: string; value: any; goal?: any; variance?: any; config?: Config }) {
  const val = Number(value ?? 0);
  const g = Number(goal ?? 0);
  const v = variance !== undefined ? Number(variance) : g !== 0 ? ((val - g) / g) * 100 : 0;
  const pct = g > 0 ? Math.min(100, Math.max(0, (val / g) * 100)) : 0;
  const positive = v >= 0;
  return (
    <Card className="flex h-full flex-col justify-between p-4">
      <div className="text-[12px] uppercase tracking-wide text-mute">{label}</div>
      <div className="mt-2 text-3xl font-semibold tracking-tight text-ink">{formatNumber(value, config)}</div>
      <div className="mt-3 space-y-1.5">
        <div className="flex items-center justify-between text-[11px] text-mute">
          <span>Meta: {formatNumber(g, config)}</span>
          <span className={positive ? "text-ok" : "text-danger"}>{positive ? "+" : ""}{v.toFixed(1)}%</span>
        </div>
        <div className="h-2 w-full overflow-hidden rounded-full bg-surface-2">
          <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${pct}%` }} />
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
}: {
  rows?: Rows;
  columns?: string[];
  hierarchy?: string[];
  drillPath?: string[];
  onDrill?: (value: string) => void;
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
  return (
    <div className="flex h-full flex-col rounded-2xl border border-line bg-surface p-4 shadow-sm">
      <div className="mb-2 text-[13px] font-medium text-ink">
        {dim}
        {drillPath.length > 0 && (
          <button className="ml-2 text-[11px] text-accent" onClick={() => onDrill?.("up")}>Subir</button>
        )}
      </div>
      <div className="flex-1 space-y-1 overflow-auto">
        {values.length === 0 ? (
          <p className="text-xs text-mute">Sem dados.</p>
        ) : (
          values.map(([name, val]) => (
            <div key={name} className="flex items-center justify-between rounded-lg px-2 py-1.5 hover:bg-surface-2">
              <div className="flex items-center gap-2">
                {drillPath.length < hierarchy.length - 1 ? <Layers size={14} className="text-primary" /> : <BarChart3 size={14} className="text-mute" />}
                <button className="text-[12px] text-ink" onClick={() => onDrill?.(name)}>{name}</button>
              </div>
              <span className="text-[12px] font-medium text-ink">{formatNumber(val)}</span>
            </div>
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

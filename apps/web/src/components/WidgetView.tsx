"use client";

import { useMemo, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Chart, Kpi } from "@/components/viz";
import { AdvancedChart, Sparkline, KpiGoal, MetricGroup, DecompositionTree, IframeWidget, formatNumber } from "@/components/AdvancedViz";
import { api } from "@/lib/api";
import { cn } from "@/lib/cn";
import { titleAlignClass } from "@/lib/widget-config";
import { Image as ImageIcon } from "lucide-react";

export type Widget = {
  id: string;
  type: WidgetType;
  title: string;
  layout: { x: number; y: number; w: number; h: number };
  query?: QuerySpec;
  text?: string;
  hierarchy?: string[];
  drillPath?: string[];
  config?: WidgetConfig;
};

export type WidgetType =
  | "kpi"
  | "line"
  | "bar"
  | "area"
  | "pie"
  | "table"
  | "text"
  | "slicer"
  | "image"
  | "markdown"
  | "gauge"
  | "waterfall"
  | "funnel"
  | "scatter"
  | "treemap"
  | "heatmap"
  | "kpi_goal"
  | "sparkline"
  | "decomposition_tree"
  | "metric_group"
  | "iframe";

export type QuerySpec = {
  dataset_id?: string;
  measures?: string[];
  dimensions?: string[];
  filters?: FilterSpec[];
  limit?: number;
  time_range?: { start?: string; end?: string };
};

export type FilterSpec = { dimension: string; op: "eq" | "in" | "neq"; value: any };

export type WidgetConfig = {
  color?: string;
  colorNegative?: string;
  showLegend?: boolean;
  legendPosition?: "top" | "bottom" | "left" | "right";
  stacked?: boolean;
  prefix?: string;
  suffix?: string;
  decimals?: number;
  compact?: "none" | "auto" | "k" | "m" | "b";
  currency?: "" | "BRL" | "USD" | "EUR";
  imageUrl?: string;
  markdown?: string;
  url?: string;
  min?: number;
  max?: number;
  target?: number;
  goal?: number;
  variance?: number;
  gaugeLabel?: string;
  waterfallNegativeCategories?: string;
  xMeasure?: string;
  yMeasure?: string;
  dimension?: string;
  measure?: string;
  showTitle?: boolean;
  titleAlign?: "left" | "center" | "right";
  fontSize?: "sm" | "md" | "lg";
  showXAxis?: boolean;
  showYAxis?: boolean;
  showGrid?: boolean;
  xAxisLabel?: string;
  yAxisLabel?: string;
  xAxisRotate?: number;
  horizontal?: boolean;
  smooth?: boolean;
  showDataLabels?: boolean;
  showTooltip?: boolean;
  showTrend?: boolean;
  comparisonLabel?: string;
  kpiAlign?: "left" | "center";
  showTotals?: boolean;
  zebra?: boolean;
  freezeHeader?: boolean;
  rowLimit?: number;
  multiSelect?: boolean;
  slicerSearch?: boolean;
  slicerStyle?: "list" | "dropdown" | "buttons";
};

export type DashboardFilter = { dimension: string; op: "eq" | "in"; value: any; dataset_id?: string };

const NO_QUERY = ["text", "image", "markdown", "iframe"];
const KPI_TYPES = ["kpi", "kpi_goal", "gauge", "metric_group"];

export function WidgetView({
  w,
  globalFilters,
  timeRange,
  onFilter,
  onDrill,
  isPreview,
}: {
  w: Widget;
  globalFilters: DashboardFilter[];
  timeRange?: { start?: string; end?: string };
  onFilter: (dim: string, value: any, op?: "eq" | "in", datasetId?: string) => void;
  onDrill: (widgetId: string, value: string) => void;
  isPreview?: boolean;
}) {
  void isPreview;
  const cfg = w.config || {};
  const emitFilter = (dim: string, value: any, op?: "eq" | "in") => onFilter(dim, value, op, w.query?.dataset_id);
  const scopedFilters = useMemo(
    () => globalFilters.filter((f) => !f.dataset_id || f.dataset_id === w.query?.dataset_id),
    [globalFilters, w.query?.dataset_id],
  );
  const body = useMemo(() => {
    const b: QuerySpec = { ...w.query };
    if (timeRange?.start || timeRange?.end) {
      b.time_range = { start: timeRange.start, end: timeRange.end };
    } else {
      delete b.time_range;
    }
    let filters = [...(b.filters || []), ...scopedFilters];
    if (w.type === "slicer") {
      const own = w.query?.dimensions?.[0];
      if (own) filters = filters.filter((f) => f.dimension !== own);
    }
    if (w.drillPath && w.drillPath.length > 0 && w.hierarchy && w.hierarchy.length > w.drillPath.length) {
      const dim = w.hierarchy[w.drillPath.length];
      b.dimensions = [dim];
      for (let i = 0; i < w.drillPath.length; i++) {
        filters.push({ dimension: w.hierarchy[i], op: "eq", value: w.drillPath[i] });
      }
    }
    if (KPI_TYPES.includes(w.type)) {
      b.dimensions = [];
    }
    if (filters.length > 0) b.filters = filters;
    else delete b.filters;
    return b;
  }, [w, scopedFilters, timeRange]);

  const q = useQuery({
    queryKey: ["widget", w.id, body],
    queryFn: () => api<any>("/api/v1/queries", { method: "POST", body: JSON.stringify(body) }),
    enabled: !!w.query?.dataset_id && !NO_QUERY.includes(w.type),
  });

  const rows = q.data?.rows || [];
  const columns = q.data?.columns || [];
  const showTitle = cfg.showTitle !== false;
  const titleCls = titleAlignClass(cfg.titleAlign);

  if (w.type === "text") {
    return <div className="h-full overflow-auto rounded-2xl border border-line bg-surface p-4 text-sm text-ink shadow-sm">{w.text || w.title}</div>;
  }
  if (w.type === "image") {
    return (
      <div className="flex h-full items-center justify-center rounded-2xl border border-line bg-surface p-4 shadow-sm">
        {cfg.imageUrl ? (
          <img src={cfg.imageUrl} alt={w.title} className="max-h-full max-w-full object-contain" />
        ) : (
          <div className="text-center text-mute">
            <ImageIcon size={32} className="mx-auto mb-2" />
            <p className="text-xs">{w.title}</p>
          </div>
        )}
      </div>
    );
  }
  if (w.type === "markdown") {
    return (
      <div className="h-full overflow-auto rounded-2xl border border-line bg-surface p-4 text-sm text-ink shadow-sm">
        <div className="prose prose-sm max-w-none" dangerouslySetInnerHTML={{ __html: renderMarkdown(cfg.markdown || w.text || "") }} />
      </div>
    );
  }
  if (w.type === "iframe") {
    return <IframeWidget url={cfg.url} title={w.title} />;
  }

  if (w.query && !w.query.dataset_id) {
    return (
      <div className="flex h-full items-center rounded-2xl border border-line bg-surface p-4 text-xs text-mute shadow-sm">
        Este visual não tem conjunto. Escolha um conjunto no inspector.
      </div>
    );
  }

  if (q.isLoading) {
    return <div className="flex h-full items-center justify-center rounded-2xl border border-line bg-surface p-4 shadow-sm text-xs text-mute">A carregar…</div>;
  }
  if (q.isError) {
    const msg = (q.error as Error).message || "";
    const friendly = /dataset not found/i.test(msg) ? "O conjunto deste visual foi excluído." : msg;
    return <div className="flex h-full items-center rounded-2xl border border-line bg-surface p-4 text-xs text-danger shadow-sm">{friendly}</div>;
  }

  if (w.type === "slicer") {
    return <SlicerView w={w} rows={rows} columns={columns} globalFilters={scopedFilters} onFilter={emitFilter} />;
  }

  if (w.type === "kpi") {
    const val = rows[0] ? Number(Object.values(rows[0])[0] ?? 0) : 0;
    const formatted = formatNumber(val, cfg);
    const goal = cfg.goal != null ? Number(cfg.goal) : undefined;
    const progress = goal && goal > 0 ? (val / goal) * 100 : undefined;
    return (
      <Kpi
        label={w.title}
        value={formatted}
        delta={cfg.showTrend && cfg.variance != null ? Number(cfg.variance) : undefined}
        comparisonLabel={cfg.comparisonLabel}
        align={cfg.kpiAlign}
        fontSize={cfg.fontSize}
        showTitle={showTitle}
        color={cfg.color}
        goalLabel={goal != null ? `Meta: ${formatNumber(goal, cfg)}` : undefined}
        progress={progress}
      />
    );
  }
  if (w.type === "table") {
    return <TableView w={w} rows={rows} columns={columns} onDrill={onDrill} />;
  }

  if (w.type === "kpi_goal") {
    const val = rows[0] ? Number(Object.values(rows[0])[0] ?? 0) : 0;
    return <KpiGoal label={w.title} value={val} goal={cfg.goal} variance={cfg.variance} config={cfg} />;
  }

  if (w.type === "metric_group") {
    return <MetricGroup label={w.title} rows={rows} columns={columns} config={cfg} />;
  }

  if (w.type === "sparkline") {
    return (
      <ChartCard title={w.title} showTitle={showTitle} align={cfg.titleAlign} drill={drillChrome(w, onDrill)}>
        <Sparkline rows={rows} columns={columns} height={Math.max(40, (w.layout.h || 2) * 70 - 40)} config={cfg} />
      </ChartCard>
    );
  }

  if (w.type === "decomposition_tree") {
    return (
      <DecompositionTree
        rows={rows}
        columns={columns}
        hierarchy={w.hierarchy || []}
        drillPath={w.drillPath || []}
        onDrill={(value) => onDrill(w.id, value)}
        config={cfg}
      />
    );
  }

  if (["gauge", "waterfall", "funnel", "scatter", "treemap", "heatmap"].includes(w.type)) {
    return (
      <ChartCard title={w.title} showTitle={showTitle} align={cfg.titleAlign} drill={drillChrome(w, onDrill)}>
        <AdvancedChart
          type={w.type as any}
          title={w.title}
          columns={columns}
          rows={rows}
          height={Math.max(120, (w.layout.h || 4) * 70 - 28)}
          config={cfg}
        />
      </ChartCard>
    );
  }

  return (
    <ChartCard title={w.title} showTitle={showTitle} align={cfg.titleAlign} drill={drillChrome(w, onDrill)}>
      <Chart
        type={w.type === "line" || w.type === "area" ? w.type : w.type === "pie" ? "pie" : "bar"}
        columns={columns}
        rows={rows}
        height={Math.max(120, (w.layout.h || 4) * 70 - 28)}
        config={cfg}
        onClick={({ value, dimension }) => {
          if (w.hierarchy) onDrill(w.id, value);
          else if (dimension) emitFilter(dimension, value);
        }}
      />
    </ChartCard>
  );
}

function ChartCard({
  title,
  showTitle,
  align,
  drill,
  children,
}: {
  title: string;
  showTitle: boolean;
  align?: "left" | "center" | "right";
  drill?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="flex h-full flex-col rounded-2xl border border-line bg-surface p-3 shadow-sm">
      {showTitle && (
        <div className={cn("mb-1 flex items-center justify-between text-[13px] text-mute", titleAlignClass(align))}>
          <span className="font-medium">{title}</span>
          {drill}
        </div>
      )}
      <div className="min-h-0 flex-1">{children}</div>
    </div>
  );
}

function drillChrome(w: Widget, onDrill: (id: string, value: string) => void) {
  if (!(w.hierarchy && w.hierarchy.length > 1 && w.drillPath && w.drillPath.length > 0)) return null;
  return (
    <button className="text-xs text-accent" onClick={() => onDrill(w.id, "up")}>
      Subir
    </button>
  );
}

function TableView({ w, rows, columns, onDrill }: { w: Widget; rows: any[]; columns: string[]; onDrill: (id: string, value: string) => void }) {
  const cfg = w.config || {};
  const limit = cfg.rowLimit ?? 20;
  const visible = rows.slice(0, limit);
  const numeric = new Set(columns.filter((c) => visible.length && typeof visible[0][c] === "number"));
  const totals = cfg.showTotals
    ? Object.fromEntries(
        columns.map((c) => [c, numeric.has(c) ? visible.reduce((s, r) => s + Number(r[c] ?? 0), 0) : ""]),
      )
    : null;

  return (
    <div className="flex h-full flex-col overflow-hidden rounded-2xl border border-line bg-surface shadow-sm">
      {cfg.showTitle !== false && (
        <div className={cn("flex items-center justify-between border-b border-line px-3 py-2", titleAlignClass(cfg.titleAlign))}>
          <span className="text-[13px] font-medium text-ink">{w.title}</span>
          {w.hierarchy && w.hierarchy.length > 1 && w.drillPath && w.drillPath.length > 0 && (
            <button className="text-xs text-accent" onClick={() => onDrill(w.id, "up")}>Subir</button>
          )}
        </div>
      )}
      <div className="flex-1 overflow-auto p-3">
        <table className="w-full text-left text-[12px]">
          <thead className={cfg.freezeHeader !== false ? "sticky top-0 bg-surface z-[1]" : undefined}>
            <tr className="text-mute">
              {columns.map((c) => (
                <th key={c} className={cn("py-1 pr-3", numeric.has(c) && "text-right")}>{c}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.map((r: any, i: number) => (
              <tr key={i} className={cn("border-t border-line hover:bg-surface-2", cfg.zebra && i % 2 === 1 && "bg-surface-2/80")}>
                {columns.map((c: string, ci: number) => (
                  <td key={c} className={cn("py-1 pr-3", numeric.has(c) && "text-right tabular-nums")}>
                    {ci === 0 && w.hierarchy ? (
                      <button className="text-accent hover:underline" onClick={() => onDrill(w.id, String(r[c]))}>{String(r[c] ?? "")}</button>
                    ) : numeric.has(c) ? (
                      formatNumber(r[c], cfg)
                    ) : (
                      String(r[c] ?? "")
                    )}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
          {totals && (
            <tfoot>
              <tr className="border-t-2 border-line font-medium text-ink">
                {columns.map((c, i) => (
                  <td key={c} className={cn("py-1.5 pr-3", numeric.has(c) && "text-right tabular-nums")}>
                    {i === 0 && !numeric.has(c) ? "Total" : numeric.has(c) ? formatNumber(totals[c], cfg) : ""}
                  </td>
                ))}
              </tr>
            </tfoot>
          )}
        </table>
      </div>
    </div>
  );
}

function SlicerView({
  w,
  rows,
  columns,
  globalFilters,
  onFilter,
}: {
  w: Widget;
  rows: any[];
  columns: string[];
  globalFilters: DashboardFilter[];
  onFilter: (dim: string, value: any, op?: "eq" | "in") => void;
}) {
  const cfg = w.config || {};
  const dim = w.query?.dimensions?.[0] || columns[0] || "";
  const [search, setSearch] = useState("");
  const values = useMemo(() => {
    const set = new Set<string>();
    rows.forEach((r) => { if (r[dim] != null) set.add(String(r[dim])); });
    return Array.from(set).slice(0, 200).sort((a, b) => a.localeCompare(b, "pt"));
  }, [rows, dim]);
  const current = globalFilters.find((f) => f.dimension === dim && (!f.dataset_id || f.dataset_id === w.query?.dataset_id));
  const selected = useMemo(() => {
    if (!current) return [] as string[];
    return Array.isArray(current.value) ? current.value.map(String) : [String(current.value)];
  }, [current]);
  const filtered = values.filter((v) => !search || v.toLowerCase().includes(search.toLowerCase()));
  const multi = !!cfg.multiSelect;
  const style = cfg.slicerStyle || "list";
  const accent = cfg.color || "#2563EB";

  const toggle = (v: string) => {
    if (!multi) {
      if (selected.length === 1 && selected[0] === v) onFilter(dim, [], "in");
      else onFilter(dim, v, "eq");
      return;
    }
    const next = selected.includes(v) ? selected.filter((x) => x !== v) : [...selected, v];
    onFilter(dim, next, "in");
  };

  const itemCls = (v: string, kind: "list" | "buttons") =>
    cn(
      kind === "list" ? "block w-full truncate rounded-lg px-2 py-1.5 text-left text-[12px]" : "rounded-full border px-2.5 py-1 text-[11px]",
      selected.includes(v) ? "font-medium text-white" : "text-mute hover:bg-surface-2",
    );

  return (
    <div className="flex h-full flex-col rounded-2xl border border-line bg-surface p-4 shadow-sm">
      {cfg.showTitle !== false && <div className={cn("mb-2 text-[13px] font-medium text-ink", titleAlignClass(cfg.titleAlign))}>{w.title}</div>}
      {cfg.slicerSearch !== false && (
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Pesquisar…"
          className="mb-2 w-full rounded-lg border border-line bg-white px-2.5 py-1.5 text-[12px] text-ink outline-none focus:border-primary/50"
        />
      )}
      {style === "dropdown" ? (
        <select
          multiple={multi}
          className="w-full rounded-xl border border-line bg-white px-2 py-1.5 text-[12px] text-ink"
          value={multi ? selected : selected[0] || ""}
          onChange={(e) => {
            if (multi) {
              const next = Array.from(e.target.selectedOptions).map((o) => o.value);
              onFilter(dim, next, "in");
            } else {
              onFilter(dim, e.target.value, "eq");
            }
          }}
        >
          {!multi && <option value="">Todos</option>}
          {filtered.map((v) => (
            <option key={v} value={v}>{v}</option>
          ))}
        </select>
      ) : (
        <div className={cn("flex-1 overflow-auto", style === "buttons" ? "flex flex-wrap content-start gap-1.5" : "space-y-1")}>
          {filtered.length === 0 ? (
            <p className="text-xs text-mute">Sem valores para filtrar.</p>
          ) : (
            filtered.map((v) => (
              <button
                key={v}
                className={itemCls(v, style === "buttons" ? "buttons" : "list")}
                style={selected.includes(v) ? { backgroundColor: accent, borderColor: accent } : style === "buttons" ? { borderColor: "#e2e8f0" } : undefined}
                onClick={() => toggle(v)}
              >
                {v}
              </button>
            ))
          )}
        </div>
      )}
    </div>
  );
}

function renderMarkdown(md: string) {
  return md
    .replace(/^### (.*$)/gim, "<h3>$1</h3>")
    .replace(/^## (.*$)/gim, "<h2>$1</h2>")
    .replace(/^# (.*$)/gim, "<h1>$1</h1>")
    .replace(/\*\*(.*)\*\*/gim, "<b>$1</b>")
    .replace(/\*(.*)\*\*/gim, "<i>$1</i>")
    .replace(/\n/gim, "<br>");
}

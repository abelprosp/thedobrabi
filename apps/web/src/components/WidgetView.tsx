"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Chart, Kpi } from "@/components/viz";
import { AdvancedChart, Sparkline, KpiGoal, MetricGroup, DecompositionTree, IframeWidget, formatNumber } from "@/components/AdvancedViz";
import { api } from "@/lib/api";
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
  showLegend?: boolean;
  stacked?: boolean;
  prefix?: string;
  suffix?: string;
  decimals?: number;
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
};

export type DashboardFilter = { dimension: string; op: "eq" | "in"; value: any };


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
  onFilter: (dim: string, value: any) => void;
  onDrill: (widgetId: string, value: string) => void;
  isPreview?: boolean;
}) {
  const body = useMemo(() => {
    const b: QuerySpec = { ...w.query };
    if (timeRange?.start || timeRange?.end) {
      b.time_range = { start: timeRange.start, end: timeRange.end };
    } else {
      delete b.time_range;
    }
    const filters = [...(b.filters || []), ...globalFilters];
    if (w.drillPath && w.drillPath.length > 0 && w.hierarchy && w.hierarchy.length > w.drillPath.length) {
      const dim = w.hierarchy[w.drillPath.length];
      b.dimensions = [dim];
      for (let i = 0; i < w.drillPath.length; i++) {
        filters.push({ dimension: w.hierarchy[i], op: "eq", value: w.drillPath[i] });
      }
    }
    if (filters.length > 0) b.filters = filters;
    else delete b.filters;
    return b;
  }, [w, globalFilters, timeRange]);

  const q = useQuery({
    queryKey: ["widget", w.id, body],
    queryFn: () => api<any>("/api/v1/queries", { method: "POST", body: JSON.stringify(body) }),
    enabled: !!w.query && !["text", "slicer", "image", "markdown", "iframe"].includes(w.type),
  });

  const rows = q.data?.rows || [];
  const columns = q.data?.columns || [];

  if (w.type === "text") {
    return <div className="h-full overflow-auto rounded-2xl border border-line bg-surface p-4 text-sm text-ink shadow-sm">{w.text || w.title}</div>;
  }
  if (w.type === "image") {
    return (
      <div className="flex h-full items-center justify-center rounded-2xl border border-line bg-surface p-4 shadow-sm">
        {w.config?.imageUrl ? (
          <img src={w.config.imageUrl} alt={w.title} className="max-h-full max-w-full object-contain" />
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
        <div className="prose prose-sm max-w-none" dangerouslySetInnerHTML={{ __html: renderMarkdown(w.config?.markdown || w.text || "") }} />
      </div>
    );
  }
  if (w.type === "slicer") {
    return <SlicerView w={w} rows={rows} columns={columns} onFilter={onFilter} />;
  }

  if (q.isLoading) {
    return <div className="flex h-full items-center justify-center rounded-2xl border border-line bg-surface p-4 shadow-sm text-xs text-mute">A carregar…</div>;
  }
  if (q.isError) {
    return <div className="flex h-full items-center rounded-2xl border border-line bg-surface p-4 text-xs text-danger shadow-sm">{(q.error as Error).message}</div>;
  }

  if (w.type === "kpi") {
    const val = rows[0] ? Number(Object.values(rows[0])[0] ?? 0) : 0;
    const formatted = formatNumber(val, w.config);
    return <Kpi label={w.title} value={formatted} />;
  }
  if (w.type === "table") {
    return (
      <div className="flex h-full flex-col overflow-hidden rounded-2xl border border-line bg-surface shadow-sm">
        <div className="flex items-center justify-between border-b border-line px-3 py-2">
          <span className="text-[13px] font-medium text-ink">{w.title}</span>
          {w.hierarchy && w.hierarchy.length > 1 && w.drillPath && w.drillPath.length > 0 && (
            <button className="text-xs text-accent" onClick={() => onDrill(w.id, "up")}>Subir</button>
          )}
        </div>
        <div className="flex-1 overflow-auto p-3">
          <table className="w-full text-left text-[12px]">
            <thead>
              <tr className="text-mute">
                {columns.map((c: string) => <th key={c} className="py-1 pr-3">{c}</th>)}
              </tr>
            </thead>
            <tbody>
              {rows.slice(0, 20).map((r: any, i: number) => (
                <tr key={i} className="border-t border-line hover:bg-surface-2">
                  {columns.map((c: string, ci: number) => (
                    <td key={c} className="py-1 pr-3">
                      {ci === 0 && w.hierarchy ? (
                        <button className="text-accent hover:underline" onClick={() => onDrill(w.id, String(r[c]))}>{String(r[c] ?? "")}</button>
                      ) : (
                        String(r[c] ?? "")
                      )}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    );
  }

  if (w.type === "iframe") {
    return <IframeWidget url={w.config?.url} title={w.title} />;
  }

  if (w.type === "kpi_goal") {
    const val = rows[0] ? Number(Object.values(rows[0])[0] ?? 0) : 0;
    return <KpiGoal label={w.title} value={val} goal={w.config?.goal} variance={w.config?.variance} config={w.config} />;
  }

  if (w.type === "metric_group") {
    return <MetricGroup label={w.title} rows={rows} columns={columns} config={w.config} />;
  }

  if (w.type === "sparkline") {
    return (
      <div className="flex h-full flex-col rounded-2xl border border-line bg-surface p-3 shadow-sm">
        <div className="mb-1 text-[13px] font-medium text-mute">{w.title}</div>
        <div className="min-h-0 flex-1">
          <Sparkline rows={rows} columns={columns} height={Math.max(40, (w.layout.h || 2) * 70 - 40)} config={w.config} />
        </div>
      </div>
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
      />
    );
  }

  if (["gauge", "waterfall", "funnel", "scatter", "treemap", "heatmap"].includes(w.type)) {
    return (
      <div className="flex h-full flex-col rounded-2xl border border-line bg-surface p-3 shadow-sm">
        <div className="mb-1 flex items-center justify-between text-[13px] text-mute">
          <span className="font-medium">{w.title}</span>
          {w.hierarchy && w.hierarchy.length > 1 && w.drillPath && w.drillPath.length > 0 && (
            <button className="text-xs text-accent" onClick={() => onDrill(w.id, "up")}>Subir</button>
          )}
        </div>
        <div className="min-h-0 flex-1">
          <AdvancedChart
            type={w.type as any}
            title={w.title}
            columns={columns}
            rows={rows}
            height={Math.max(120, (w.layout.h || 4) * 70 - 28)}
            config={w.config}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col rounded-2xl border border-line bg-surface p-3 shadow-sm">
      <div className="mb-1 flex items-center justify-between text-[13px] text-mute">
        <span className="font-medium">{w.title}</span>
        {w.hierarchy && w.hierarchy.length > 1 && w.drillPath && w.drillPath.length > 0 && (
          <button className="text-xs text-accent" onClick={() => onDrill(w.id, "up")}>Subir</button>
        )}
      </div>
      <div className="min-h-0 flex-1">
        <Chart
          type={w.type === "line" || w.type === "area" ? w.type : w.type === "pie" ? "pie" : "bar"}
          title={w.title}
          columns={columns}
          rows={rows}
          height={Math.max(120, (w.layout.h || 4) * 70 - 28)}
          onClick={({ value, dimension }) => {
            if (w.hierarchy) onDrill(w.id, value);
            else if (dimension) onFilter(dimension, value);
          }}
        />
      </div>
    </div>
  );
}

function SlicerView({ w, rows, columns, onFilter }: { w: Widget; rows: any[]; columns: string[]; onFilter: (dim: string, value: any) => void }) {
  const dim = w.query?.dimensions?.[0] || "";
  const values = useMemo(() => {
    const set = new Set<string>();
    rows.forEach((r) => { if (r[dim] != null) set.add(String(r[dim])); });
    return Array.from(set).slice(0, 50).sort();
  }, [rows, dim]);

  return (
    <div className="flex h-full flex-col rounded-2xl border border-line bg-surface p-4 shadow-sm">
      <div className="mb-2 text-[13px] font-medium text-ink">{w.title}</div>
      <div className="flex-1 space-y-1 overflow-auto">
        {values.length === 0 ? (
          <p className="text-xs text-mute">Sem valores para filtrar.</p>
        ) : (
          values.map((v) => (
            <button key={v} className="block w-full truncate rounded-lg px-2 py-1.5 text-left text-[12px] text-mute hover:bg-surface-2" onClick={() => onFilter(dim, v)}>
              {v}
            </button>
          ))
        )}
      </div>
    </div>
  );
}

function renderMarkdown(md: string) {
  return md
    .replace(/^### (.*$)/gim, "<h3>$1</h3>")
    .replace(/^## (.*$)/gim, "<h2>$1</h2>")
    .replace(/^# (.*$)/gim, "<h1>$1</h1>")
    .replace(/\*\*(.*)\*\*/gim, "<b>$1</b>")
    .replace(/\*(.*)\*/gim, "<i>$1</i>")
    .replace(/\n/gim, "<br>");
}

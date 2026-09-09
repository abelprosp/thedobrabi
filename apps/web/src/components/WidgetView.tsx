"use client";

import { useMemo, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Chart, Kpi } from "@/components/viz";
import { AdvancedChart, Sparkline, KpiGoal, MetricGroup, DecompositionTree, IframeWidget, formatNumber } from "@/components/AdvancedViz";
import {
  BubbleCard,
  HexmapCard,
  NetworkSalesCard,
  RadarCard,
  RadialCard,
  RidgelineCard,
  SalesReportCard,
  SankeyCard,
  StatSparkCard,
} from "@/components/hyper-charts";
import { api, getAccess } from "@/lib/api";
import { toast } from "sonner";
import { cn } from "@/lib/cn";
import { DEFAULT_QUERY_LIMIT, MAX_QUERY_LIMIT, titleAlignClass, widgetCrossBy } from "@/lib/widget-config";
import { diagnoseQueryValue, firstNumericEntry } from "@/lib/widget-errors";
import { AlertCircle, ChevronLeft, ChevronRight, Download, Image as ImageIcon } from "lucide-react";
import { DataIntelligenceCard } from "@/components/data-intelligence-card";
import { RankingCard } from "@/components/ranking-card";

export type GridPos = { x: number; y: number; w: number; h: number };

export type Widget = {
  id: string;
  type: WidgetType;
  title: string;
  layout: GridPos;
  layoutMobile?: GridPos;
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
  | "big_table"
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
  | "iframe"
  | "radar"
  | "sankey"
  | "ridgeline"
  | "sales_report"
  | "network_sales"
  | "radial"
  | "stat_spark"
  | "bubble"
  | "hexmap"
  | "data_intelligence"
  | "ranking";

export type QuerySpec = {
  dataset_id?: string;
  measures?: string[];
  dimensions?: string[];
  filters?: FilterSpec[];
  limit?: number;
  time_range?: { start?: string; end?: string };
  joins?: QueryJoin[];
  order_by?: { field: string; dir: "asc" | "desc" | "ASC" | "DESC" }[];
};

export type QueryJoin = {
  dataset_id: string;
  from_column: string;
  to_column: string;
  match?: "both" | "all_left";
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
  pageSize?: number;
  sparkStyle?: "area" | "candle";
  mirrored?: boolean;
  multiSelect?: boolean;
  slicerSearch?: boolean;
  slicerStyle?: "list" | "dropdown" | "buttons";
  icon?: string;
  focusPrompt?: string;
  crossBy?: "columns" | "measures";
  rankOrder?: "asc" | "desc";
  rankLimit?: number;
};

export type DashboardFilter = { dimension: string; op: "eq" | "in"; value: any; dataset_id?: string };

const NO_QUERY = ["text", "image", "markdown", "iframe", "data_intelligence"];
const KPI_TYPES = ["kpi", "kpi_goal", "gauge", "metric_group"];

export function WidgetView({
  w,
  globalFilters,
  timeRange,
  onFilter,
  onDrill,
  isPreview,
  queryPath,
  siblingWidgets,
  dashboardId,
}: {
  w: Widget;
  globalFilters: DashboardFilter[];
  timeRange?: { start?: string; end?: string };
  onFilter: (dim: string, value: any, op?: "eq" | "in", datasetId?: string) => void;
  onDrill: (widgetId: string, value: string) => void;
  isPreview?: boolean;
  queryPath?: string;
  siblingWidgets?: Widget[];
  dashboardId?: string;
}) {
  void isPreview;
  const queriesURL = queryPath || "/api/v1/queries";
  const isPublicQuery = queriesURL.includes("/public/");
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
    if (!KPI_TYPES.includes(w.type) && w.type !== "slicer" && w.type !== "scatter" && w.type !== "ranking") {
      const crossBy = widgetCrossBy(w.type, cfg, w.query);
      if (crossBy === "columns") {
        b.measures = (b.measures || []).slice(0, 1);
      } else {
        const dimKeep = w.type === "heatmap" ? 2 : 1;
        b.dimensions = (b.dimensions || []).slice(0, dimKeep);
      }
    }
    if (w.type === "ranking") {
      const field = (b.measures || [])[0];
      if (field) b.order_by = [{ field, dir: cfg.rankOrder === "asc" ? "ASC" : "DESC" }];
      const n = Number(cfg.rankLimit || 10);
      b.limit = Math.max(3, Math.min(50, Number.isFinite(n) ? n : 10));
    }
    const joins = (b.joins || []).filter((j) => j.dataset_id && j.from_column && j.to_column);
    if (joins.length) {
      b.joins = joins;
    } else {
      delete b.joins;
      b.measures = (b.measures || []).filter((m) => !m.startsWith("join."));
      b.dimensions = (b.dimensions || []).filter((d) => !d.startsWith("join."));
    }
    if (filters.length > 0) b.filters = filters;
    else delete b.filters;
    if (!b.limit || b.limit <= 0) b.limit = w.type === "big_table" ? MAX_QUERY_LIMIT : DEFAULT_QUERY_LIMIT;
    return b;
  }, [w, scopedFilters, timeRange]);

  const q = useQuery({
    queryKey: ["widget", w.id, queriesURL, body],
    queryFn: () => api<any>(queriesURL, { method: "POST", body: JSON.stringify(body) }),
    enabled: !!w.query?.dataset_id && !NO_QUERY.includes(w.type),
    retry: (count, err) => {
      const status = (err as { status?: number })?.status || 0;
      if (status && status < 500) return false;
      return count < 1;
    },
    refetchOnWindowFocus: false,
  });

  const rows = q.data?.rows || [];
  const columns = q.data?.columns || [];
  const showTitle = cfg.showTitle !== false;
  const issue = !NO_QUERY.includes(w.type) && !q.isLoading && !q.isError
    ? diagnoseQueryValue({
        rows,
        columns,
        measures: w.query?.measures,
        dimensions: w.query?.dimensions,
        kind: KPI_TYPES.includes(w.type) ? "kpi" : w.type === "table" || w.type === "big_table" || w.type === "slicer" || w.type === "ranking" ? "table" : "chart",
      })
    : null;

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
  if (w.type === "data_intelligence") {
    return (
      <DataIntelligenceCard
        title={w.title}
        focusPrompt={cfg.focusPrompt}
        siblings={siblingWidgets || []}
        dashboardId={dashboardId}
        globalFilters={globalFilters}
        timeRange={timeRange}
        isPublic={isPublicQuery}
      />
    );
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
    const friendly = !isPublicQuery && /unauthorized|token|sessão expirada/i.test(msg)
      ? "Sessão expirada. A página vai renovar o acesso; se continuar, entre outra vez."
      : /dataset not found/i.test(msg)
        ? "O conjunto deste visual foi excluído. Escolha outro conjunto no inspector."
        : msg;
    return <div className="flex h-full items-center rounded-2xl border border-line bg-surface p-4 text-xs text-danger shadow-sm">{friendly}</div>;
  }

  if (w.type === "slicer") {
    return <SlicerView w={w} rows={rows} columns={columns} globalFilters={scopedFilters} onFilter={emitFilter} />;
  }

  if (w.type === "kpi") {
    const picked = firstNumericEntry(rows[0], w.query?.measures);
    const val = isFiniteNumber(picked.value) ? Number(picked.value) : NaN;
    const formatted = issue ? "—" : formatNumber(val, cfg);
    const goal = cfg.goal != null ? Number(cfg.goal) : undefined;
    const progress = goal && goal > 0 && Number.isFinite(val) ? (val / goal) * 100 : undefined;
    return (
      <div className="relative h-full">
        <Kpi
          label={w.title}
          value={formatted}
          delta={!issue && cfg.showTrend && cfg.variance != null ? Number(cfg.variance) : undefined}
          comparisonLabel={cfg.comparisonLabel}
          align={cfg.kpiAlign}
          fontSize={cfg.fontSize}
          showTitle={showTitle}
          color={cfg.color}
          icon={cfg.icon}
          goalLabel={!issue && goal != null ? `Meta: ${formatNumber(goal, cfg)}` : undefined}
          progress={!issue ? progress : undefined}
        />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }
  if (w.type === "ranking") {
    return (
      <div className="relative h-full">
        <RankingCard
          title={w.title}
          rows={rows}
          columns={columns}
          measures={w.query?.measures}
          config={cfg}
          showTitle={showTitle}
          onSelect={(dim, value) => emitFilter(dim, value)}
        />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }
  if (w.type === "table") {
    return (
      <div className="relative h-full">
        <TableView w={w} rows={rows} columns={columns} onDrill={onDrill} />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }
  if (w.type === "big_table") {
    return (
      <div className="relative h-full">
        <BigTableView w={w} rows={rows} columns={columns} onDrill={onDrill} />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }

  if (w.type === "kpi_goal") {
    const picked = firstNumericEntry(rows[0], w.query?.measures);
    const val = isFiniteNumber(picked.value) ? Number(picked.value) : 0;
    return (
      <div className="relative h-full">
        <KpiGoal label={w.title} value={issue ? null : val} goal={cfg.goal} variance={cfg.variance} config={cfg} />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }

  if (w.type === "metric_group") {
    return (
      <div className="relative h-full">
        <MetricGroup label={w.title} rows={rows} columns={columns} config={cfg} />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }

  if (w.type === "sparkline") {
    return (
      <ChartCard title={w.title} showTitle={showTitle} align={cfg.titleAlign} drill={drillChrome(w, onDrill)} issue={issue}>
        <Sparkline rows={rows} columns={columns} config={cfg} />
      </ChartCard>
    );
  }

  if (w.type === "decomposition_tree") {
    return (
      <div className="relative h-full">
        <DecompositionTree
          rows={rows}
          columns={columns}
          hierarchy={w.hierarchy || []}
          drillPath={w.drillPath || []}
          onDrill={(value) => onDrill(w.id, value)}
          config={cfg}
        />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }

  if (w.type === "stat_spark") {
    return (
      <div className="relative h-full">
        <StatSparkCard title={w.title} rows={rows} columns={columns} config={cfg} />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }
  if (w.type === "radar") {
    return (
      <div className="relative h-full">
        <RadarCard title={w.title} rows={rows} columns={columns} config={cfg} />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }
  if (w.type === "ridgeline") {
    return (
      <div className="relative h-full">
        <RidgelineCard title={w.title} rows={rows} columns={columns} config={cfg} />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }
  if (w.type === "sankey") {
    return (
      <div className="relative h-full">
        <SankeyCard title={w.title} rows={rows} columns={columns} config={cfg} />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }
  if (w.type === "sales_report") {
    return (
      <div className="relative h-full">
        <SalesReportCard title={w.title} rows={rows} columns={columns} config={cfg} />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }
  if (w.type === "network_sales") {
    return (
      <div className="relative h-full">
        <NetworkSalesCard title={w.title} rows={rows} columns={columns} config={cfg} />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }
  if (w.type === "bubble") {
    return (
      <div className="relative h-full">
        <BubbleCard title={w.title} rows={rows} columns={columns} config={cfg} />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }
  if (w.type === "hexmap") {
    return (
      <div className="relative h-full">
        <HexmapCard title={w.title} rows={rows} columns={columns} config={cfg} />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }
  if (w.type === "radial") {
    return (
      <div className="relative h-full">
        <RadialCard title={w.title} rows={rows} columns={columns} config={cfg} />
        {issue && <IssueHint issue={issue} />}
      </div>
    );
  }
  if (["gauge", "waterfall", "funnel", "scatter", "treemap", "heatmap"].includes(w.type)) {
    return (
      <ChartCard title={w.title} showTitle={showTitle} align={cfg.titleAlign} drill={drillChrome(w, onDrill)} issue={issue}>
        <AdvancedChart
          type={w.type as any}
          title={w.title}
          columns={columns}
          rows={rows}
          config={cfg}
        />
      </ChartCard>
    );
  }

  return (
    <ChartCard title={w.title} showTitle={showTitle} align={cfg.titleAlign} drill={drillChrome(w, onDrill)} issue={issue}>
      <Chart
        type={w.type === "line" || w.type === "area" ? w.type : w.type === "pie" ? "pie" : "bar"}
        columns={columns}
        rows={rows}
        measures={w.query?.measures}
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
  issue,
  children,
}: {
  title: string;
  showTitle: boolean;
  align?: "left" | "center" | "right";
  drill?: ReactNode;
  issue?: { code: string; message: string } | null;
  children: ReactNode;
}) {
  return (
    <div className="relative flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-3 shadow-sm">
      {showTitle && (
        <div className={cn("mb-1 flex shrink-0 items-center justify-between text-[13px] text-mute", titleAlignClass(align))}>
          <span className="font-medium">{title}</span>
          {drill}
        </div>
      )}
      <div className="relative min-h-0 flex-1">{children}</div>
      {issue && <IssueHint issue={issue} />}
    </div>
  );
}

function IssueHint({ issue }: { issue: { message: string } }) {
  return (
    <div className="pointer-events-none absolute inset-x-2 bottom-2 z-10 flex items-start gap-1.5 rounded-xl border border-amber-200 bg-amber-50/95 px-2.5 py-2 text-[11px] leading-snug text-amber-950 shadow-sm">
      <AlertCircle size={14} className="mt-0.5 shrink-0 text-amber-700" />
      <span>{issue.message}</span>
    </div>
  );
}

function isFiniteNumber(v: unknown) {
  const n = typeof v === "number" ? v : Number(v);
  return Number.isFinite(n);
}

function drillChrome(w: Widget, onDrill: (id: string, value: string) => void) {
  if (!(w.hierarchy && w.hierarchy.length > 1 && w.drillPath && w.drillPath.length > 0)) return null;
  return (
    <button className="text-xs text-accent" onClick={() => onDrill(w.id, "up")}>
      Subir
    </button>
  );
}

function BigTableView({ w, rows, columns, onDrill }: { w: Widget; rows: any[]; columns: string[]; onDrill: (id: string, value: string) => void }) {
  const cfg = w.config || {};
  const [page, setPage] = useState(0);
  const [q, setQ] = useState("");
  const [sort, setSort] = useState<{ col: string; dir: "asc" | "desc" } | null>(null);
  const [pageSize, setPageSize] = useState(() => Math.max(10, Math.min(200, Number(cfg.pageSize || cfg.rowLimit || 50))));

  const numeric = useMemo(
    () => new Set(columns.filter((c) => rows.some((r) => typeof r[c] === "number" && Number.isFinite(r[c])))),
    [columns, rows],
  );

  const filtered = useMemo(() => {
    const term = q.trim().toLowerCase();
    let next = rows;
    if (term) {
      next = rows.filter((r) => columns.some((c) => String(r[c] ?? "").toLowerCase().includes(term)));
    }
    if (sort) {
      const { col, dir } = sort;
      const mul = dir === "asc" ? 1 : -1;
      next = [...next].sort((a, b) => {
        const av = a[col];
        const bv = b[col];
        if (typeof av === "number" && typeof bv === "number") return (av - bv) * mul;
        return String(av ?? "").localeCompare(String(bv ?? ""), "pt", { numeric: true }) * mul;
      });
    }
    return next;
  }, [rows, columns, q, sort]);

  const pages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const safePage = Math.min(page, pages - 1);
  const start = safePage * pageSize;
  const visible = filtered.slice(start, start + pageSize);
  const totals = cfg.showTotals
    ? Object.fromEntries(columns.map((c) => [c, numeric.has(c) ? filtered.reduce((s, r) => s + Number(r[c] ?? 0), 0) : ""]))
    : null;

  return (
    <div className="flex h-full flex-col overflow-hidden rounded-2xl border border-line bg-surface shadow-sm">
      <div className="flex flex-wrap items-center gap-2 border-b border-line px-3 py-2">
        {cfg.showTitle !== false ? <span className="min-w-0 flex-1 truncate text-[13px] font-medium text-ink">{w.title}</span> : <span className="flex-1" />}
        <input
          value={q}
          onChange={(e) => { setQ(e.target.value); setPage(0); }}
          placeholder="Pesquisar na tabela…"
          className="h-9 w-full max-w-[14rem] rounded-lg border border-line bg-surface px-2.5 text-[12px] text-ink outline-none focus:border-primary/50 sm:w-44"
        />
        {rows.length > 0 && (
          <button type="button" className="inline-flex min-h-9 items-center gap-1 px-1 text-[11px] text-mute hover:text-ink" onClick={() => downloadRows(w.title, columns, filtered, "csv")}>
            <Download size={12} /> CSV
          </button>
        )}
        {w.query?.dataset_id && (
          <button
            type="button"
            className="inline-flex min-h-9 items-center gap-1 px-1 text-[11px] text-mute hover:text-ink"
            onClick={() => downloadDatasetXlsx(w.query!.dataset_id!, w.title).catch((e: Error) => toast.error(e.message || "Falha ao exportar Excel"))}
          >
            <Download size={12} /> Excel
          </button>
        )}
      </div>
      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full min-w-max text-left text-[12px]">
          <thead className={cfg.freezeHeader !== false ? "sticky top-0 z-[1] bg-surface" : undefined}>
            <tr className="text-mute">
              {columns.map((c) => (
                <th key={c} className={cn("whitespace-nowrap px-3 py-2 font-medium", numeric.has(c) && "text-right")}>
                  <button
                    type="button"
                    className="inline-flex items-center gap-1 hover:text-ink"
                    onClick={() => setSort((s) => s?.col === c ? { col: c, dir: s.dir === "asc" ? "desc" : "asc" } : { col: c, dir: numeric.has(c) ? "desc" : "asc" })}
                  >
                    {c}
                    {sort?.col === c ? (sort.dir === "asc" ? " ↑" : " ↓") : ""}
                  </button>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.length === 0 ? (
              <tr><td colSpan={Math.max(columns.length, 1)} className="px-3 py-8 text-center text-mute">Sem linhas nesta página.</td></tr>
            ) : (
              visible.map((r: any, i: number) => (
                <tr key={start + i} className={cn("border-t border-line hover:bg-surface-2", cfg.zebra && i % 2 === 1 && "bg-surface-2/80")}>
                  {columns.map((c: string, ci: number) => (
                    <td key={c} className={cn("whitespace-nowrap px-3 py-1.5", numeric.has(c) && "text-right tabular-nums")}>
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
              ))
            )}
          </tbody>
          {totals && (
            <tfoot>
              <tr className="border-t-2 border-line font-medium text-ink">
                {columns.map((c, i) => (
                  <td key={c} className={cn("px-3 py-2", numeric.has(c) && "text-right tabular-nums")}>
                    {i === 0 && !numeric.has(c) ? "Total" : numeric.has(c) ? formatNumber(totals[c], cfg) : ""}
                  </td>
                ))}
              </tr>
            </tfoot>
          )}
        </table>
      </div>
      <div className="flex flex-wrap items-center justify-between gap-2 border-t border-line px-3 py-2 text-[12px] text-mute">
        <span>
          {filtered.length === 0 ? "0 linhas" : `${start + 1}–${Math.min(start + pageSize, filtered.length)} de ${filtered.length.toLocaleString("pt-BR")}`}
          {rows.length >= (w.query?.limit || MAX_QUERY_LIMIT) ? " (limite da consulta)" : ""}
        </span>
        <div className="flex items-center gap-1">
          <select
            className="h-9 rounded-lg border border-line bg-surface px-2 text-[12px] text-ink"
            value={pageSize}
            onChange={(e) => { setPageSize(Number(e.target.value)); setPage(0); }}
            aria-label="Linhas por página"
          >
            {[25, 50, 100, 200].map((n) => (
              <option key={n} value={n}>{n} / página</option>
            ))}
          </select>
          <button type="button" className="flex h-9 w-9 items-center justify-center rounded-lg hover:bg-surface-2 disabled:opacity-40" disabled={safePage <= 0} onClick={() => setPage((p) => Math.max(0, p - 1))} aria-label="Página anterior">
            <ChevronLeft size={16} />
          </button>
          <span className="min-w-[4.5rem] text-center tabular-nums">{safePage + 1} / {pages}</span>
          <button type="button" className="flex h-9 w-9 items-center justify-center rounded-lg hover:bg-surface-2 disabled:opacity-40" disabled={safePage >= pages - 1} onClick={() => setPage((p) => p + 1)} aria-label="Página seguinte">
            <ChevronRight size={16} />
          </button>
        </div>
      </div>
    </div>
  );
}

function TableView({ w, rows, columns, onDrill }: { w: Widget; rows: any[]; columns: string[]; onDrill: (id: string, value: string) => void }) {
  const cfg = w.config || {};
  const limit = cfg.rowLimit ?? 20;
  const visible = rows.slice(0, limit);
  const numeric = new Set(
    columns.filter((c) => visible.some((r) => typeof r[c] === "number" && Number.isFinite(r[c]))),
  );
  const totals = cfg.showTotals
    ? Object.fromEntries(
        columns.map((c) => [c, numeric.has(c) ? visible.reduce((s, r) => s + Number(r[c] ?? 0), 0) : ""]),
      )
    : null;

  return (
    <div className="flex h-full flex-col overflow-hidden rounded-2xl border border-line bg-surface shadow-sm">
      {(cfg.showTitle !== false || rows.length > 0) && (
        <div className={cn("flex items-center justify-between border-b border-line px-3 py-2", titleAlignClass(cfg.titleAlign))}>
          {cfg.showTitle !== false ? <span className="text-[13px] font-medium text-ink">{w.title}</span> : <span />}
          <div className="flex items-center gap-2">
            {(rows.length > 0 || w.query?.dataset_id) && (
              <>
                {rows.length > 0 && (
                  <button type="button" className="inline-flex min-h-9 items-center gap-1 px-1 text-[11px] text-mute hover:text-ink sm:min-h-0" onClick={() => downloadRows(w.title, columns, rows, "csv")}>
                    <Download size={12} className="inline" /> CSV
                  </button>
                )}
                {w.query?.dataset_id && (
                  <button
                    type="button"
                    className="inline-flex min-h-9 items-center gap-1 px-1 text-[11px] text-mute hover:text-ink sm:min-h-0"
                    onClick={() => {
                      downloadDatasetXlsx(w.query!.dataset_id!, w.title).catch((e: Error) => toast.error(e.message || "Falha ao exportar Excel"));
                    }}
                  >
                    <Download size={12} className="inline" /> Excel
                  </button>
                )}
              </>
            )}
            {w.hierarchy && w.hierarchy.length > 1 && w.drillPath && w.drillPath.length > 0 && (
              <button className="text-xs text-accent" onClick={() => onDrill(w.id, "up")}>Subir</button>
            )}
          </div>
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
    return Array.from(set).slice(0, 2000).sort((a, b) => a.localeCompare(b, "pt"));
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
          className="mb-2 w-full rounded-lg border border-line bg-surface px-2.5 py-1.5 text-[12px] text-ink outline-none focus:border-primary/50"
        />
      )}
      {style === "dropdown" ? (
        <select
          multiple={multi}
          className="w-full rounded-xl border border-line bg-surface px-2 py-1.5 text-[12px] text-ink"
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

function downloadRows(title: string, columns: string[], rows: any[], format: "csv" | "xlsx") {
  const name = (title || "tabela").replace(/[^\w\-]+/g, "_");
  const esc = (v: unknown) => `"${String(v ?? "").replace(/"/g, '""')}"`;
  const body = [columns.map(esc).join(","), ...rows.map((r) => columns.map((c) => esc(r[c])).join(","))].join("\n");
  const blob = new Blob(["\uFEFF" + body], { type: "text/csv;charset=utf-8" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = `${name}.csv`;
  a.click();
  URL.revokeObjectURL(a.href);
  void format;
}

async function downloadDatasetXlsx(datasetId: string, title: string) {
  const token = getAccess();
  const ws = typeof window !== "undefined" ? localStorage.getItem("thedobra.workspace") : "";
  const res = await fetch(`/api/v1/datasets/${datasetId}/export?format=xlsx`, {
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(ws ? { "X-Workspace-Id": ws } : {}),
    },
  });
  if (!res.ok) {
    throw new Error("Falha ao exportar o conjunto");
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `${(title || "conjunto").replace(/[^\w\-]+/g, "_")}.xlsx`;
  a.click();
  URL.revokeObjectURL(url);
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

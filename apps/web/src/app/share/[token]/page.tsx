"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import GridLayout, { WidthProvider, type Layout } from "react-grid-layout";
import "react-grid-layout/css/styles.css";
import "react-resizable/css/styles.css";
import { api } from "@/lib/api";
import { Logo } from "@/components/brand";
import { ErrorState, PageSkeleton } from "@/components/ui";
import { ThemeSegmented } from "@/components/theme-toggle";
import { parseLayoutTheme, useTheme } from "@/components/theme-provider";
import { WidgetView, type DashboardFilter, type Widget } from "@/components/WidgetView";

const Grid = WidthProvider(GridLayout);

type PublicDashboard = {
  name: string;
  description: string;
  layout: { widgets: Widget[]; theme?: string };
};

function normalizeWidgets(raw: Widget[] | undefined): Widget[] {
  return (raw || []).map((w, i) => ({
    ...w,
    id: w.id || `w-${i}`,
    title: w.title || "",
    type: w.type || "bar",
    layout: {
      x: Number(w.layout?.x ?? (i * 6) % 12),
      y: Number(w.layout?.y ?? Math.floor(i / 2) * 4),
      w: Math.max(2, Number(w.layout?.w ?? 6)),
      h: Math.max(2, Number(w.layout?.h ?? 4)),
    },
  }));
}

export default function SharePage() {
  const { token } = useParams<{ token: string }>();
  const { theme, setTheme } = useTheme();
  const [globalFilters, setGlobalFilters] = useState<DashboardFilter[]>([]);
  const [widgets, setWidgets] = useState<Widget[]>([]);
  const q = useQuery({
    queryKey: ["public-dash", token],
    queryFn: () => api<PublicDashboard>(`/api/v1/public/dashboards/${token}`),
  });

  useEffect(() => {
    const saved = parseLayoutTheme(q.data?.layout);
    if (saved) setTheme(saved);
  }, [q.data, setTheme]);

  useEffect(() => {
    if (!q.data) return;
    setWidgets(normalizeWidgets(q.data.layout?.widgets));
    setGlobalFilters([]);
  }, [q.data]);

  const applyFilter = useCallback((dim: string, value: any, op?: "eq" | "in", datasetId?: string) => {
    setGlobalFilters((prev) => {
      const same = (f: DashboardFilter) => f.dimension === dim && (f.dataset_id || "") === (datasetId || "");
      if (value == null || value === "" || (Array.isArray(value) && value.length === 0)) {
        return prev.filter((f) => !same(f));
      }
      const nextOp = op || (Array.isArray(value) ? "in" : "eq");
      if (prev.some(same)) {
        return prev.map((f) => (same(f) ? { ...f, value, op: nextOp, dataset_id: datasetId } : f));
      }
      return [...prev, { dimension: dim, op: nextOp, value, dataset_id: datasetId }];
    });
  }, []);

  const drill = useCallback((widgetId: string, value: string) => {
    setWidgets((prev) =>
      prev.map((w) => {
        if (w.id !== widgetId) return w;
        const path = w.drillPath || [];
        if (value === "up") return { ...w, drillPath: path.slice(0, -1) };
        return { ...w, drillPath: [...path, value] };
      }),
    );
  }, []);

  const layout = useMemo<Layout[]>(
    () => widgets.map((w) => ({ i: w.id, x: w.layout.x, y: w.layout.y, w: w.layout.w, h: w.layout.h, minW: 2, minH: 2 })),
    [widgets],
  );

  const queryPath = `/api/v1/public/dashboards/${token}/queries`;

  if (q.isError) return <ErrorState message="Partilha inválida ou expirada." />;
  if (q.isLoading || !q.data) return <div className="p-8"><PageSkeleton /></div>;

  return (
    <div className="min-h-screen bg-bg">
      <div className="border-b border-line px-6 py-4">
        <div className="mb-4 flex items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <Logo variant={theme === "dark" ? "dark" : "light"} size={28} />
            <span className="text-sm text-mute">· partilha</span>
          </div>
          <ThemeSegmented value={theme} onChange={setTheme} />
        </div>
        <h1 className="text-2xl font-semibold text-ink">{q.data.name}</h1>
        {q.data.description && <p className="mt-1 text-sm text-mute">{q.data.description}</p>}
        {globalFilters.length > 0 && (
          <div className="mt-3 flex flex-wrap items-center gap-1">
            {globalFilters.map((f) => (
              <span key={`${f.dataset_id || ""}:${f.dimension}`} className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-1 text-[11px] text-primary-600">
                {f.dimension} = {Array.isArray(f.value) ? f.value.join(", ") : String(f.value)}
                <button
                  className="text-primary-700"
                  onClick={() => setGlobalFilters(globalFilters.filter((x) => !(x.dimension === f.dimension && (x.dataset_id || "") === (f.dataset_id || ""))))}
                >
                  ×
                </button>
              </span>
            ))}
            <button className="text-[11px] text-mute hover:text-ink" onClick={() => setGlobalFilters([])}>
              Limpar filtros
            </button>
          </div>
        )}
      </div>
      <div className="min-h-[calc(100vh-8rem)] pb-10">
        {widgets.length === 0 ? (
          <p className="px-6 py-10 text-sm text-mute">Este dashboard ainda não tem widgets.</p>
        ) : (
          <Grid
            className="layout min-h-full"
            layout={layout}
            cols={12}
            rowHeight={96}
            margin={[14, 14]}
            containerPadding={[16, 16]}
            isDraggable={false}
            isResizable={false}
            compactType="vertical"
          >
            {widgets.map((w) => (
              <div key={w.id} className="widget-grid-item relative">
                <WidgetView
                  w={w}
                  globalFilters={globalFilters}
                  onFilter={(dim, value, op) => applyFilter(dim, value, op, w.query?.dataset_id)}
                  onDrill={drill}
                  queryPath={queryPath}
                />
              </div>
            ))}
          </Grid>
        )}
      </div>
    </div>
  );
}

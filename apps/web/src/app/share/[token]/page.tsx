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
import { AppearanceScope, parseLayoutTheme } from "@/components/theme-provider";
import { WidgetView, type DashboardFilter, type Widget } from "@/components/WidgetView";
import { readStoredDashboardAppearance, writeStoredDashboardAppearance, type Appearance } from "@/lib/theme";
import { useMediaQuery } from "@/lib/use-media-query";
import { MOBILE_COLS, resolveDesktopLayout, resolveMobileLayout } from "@/lib/dashboard-layout";

const Grid = WidthProvider(GridLayout);

type PublicDashboard = {
  name: string;
  description: string;
  layout: { widgets: Widget[]; theme?: string };
  brand_name?: string;
  brand_logo_url?: string;
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
    layoutMobile: w.layoutMobile
      ? {
          x: Number(w.layoutMobile.x ?? 0),
          y: Number(w.layoutMobile.y ?? 0),
          w: Math.max(2, Number(w.layoutMobile.w ?? MOBILE_COLS)),
          h: Math.max(2, Number(w.layoutMobile.h ?? 3)),
        }
      : undefined,
  }));
}

export default function SharePage() {
  const { token } = useParams<{ token: string }>();
  const [dashTheme, setDashTheme] = useState<Appearance>("light");
  const [globalFilters, setGlobalFilters] = useState<DashboardFilter[]>([]);
  const [widgets, setWidgets] = useState<Widget[]>([]);
  const isNarrow = useMediaQuery("(max-width: 767px)");
  const q = useQuery({
    queryKey: ["public-dash", token],
    queryFn: () => api<PublicDashboard>(`/api/v1/public/dashboards/${token}`),
  });

  useEffect(() => {
    if (!q.data) return;
    const saved = parseLayoutTheme(q.data.layout);
    setDashTheme(saved || readStoredDashboardAppearance());
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
    () => (isNarrow ? resolveMobileLayout(widgets) : resolveDesktopLayout(widgets)),
    [widgets, isNarrow],
  );

  const queryPath = `/api/v1/public/dashboards/${token}/queries`;

  if (q.isError) return <ErrorState message="Partilha inválida ou expirada." />;
  if (q.isLoading || !q.data) return <div className="p-8"><PageSkeleton /></div>;

  return (
    <AppearanceScope appearance={dashTheme} className="min-h-screen bg-bg">
      <div className="border-b border-line px-4 py-3 sm:px-6 sm:py-4">
        <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 items-center gap-2">
            <Logo
              variant={dashTheme === "dark" ? "dark" : "light"}
              size={28}
              brandName={q.data.brand_name}
              brandLogoUrl={q.data.brand_logo_url}
            />
            <span className="text-sm text-mute">· partilha</span>
          </div>
          <ThemeSegmented
            label="Tema do dashboard"
            value={dashTheme}
            onChange={(next) => {
              setDashTheme(next);
              writeStoredDashboardAppearance(next);
            }}
          />
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
      <div className="min-h-[calc(100dvh-8rem)] pb-10">
        {widgets.length === 0 ? (
          <p className="px-6 py-10 text-sm text-mute">Este dashboard ainda não tem widgets.</p>
        ) : (
          <Grid
            key={isNarrow ? "mobile" : "desktop"}
            className="layout min-h-full"
            layout={layout}
            cols={isNarrow ? MOBILE_COLS : 12}
            rowHeight={isNarrow ? 88 : 96}
            margin={isNarrow ? [10, 10] : [14, 14]}
            containerPadding={isNarrow ? [12, 12] : [16, 16]}
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
    </AppearanceScope>
  );
}

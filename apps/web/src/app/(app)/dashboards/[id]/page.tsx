"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, apiStatus, normalizeArray } from "@/lib/api";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { WidgetView, type Widget, type DashboardFilter, type WidgetType } from "@/components/WidgetView";
import { WidgetInspector } from "@/components/widget-inspector";
import { toast } from "sonner";
import GridLayout, { WidthProvider, type Layout } from "react-grid-layout";
import "react-grid-layout/css/styles.css";
import "react-resizable/css/styles.css";
import {
  Button,
  Card,
  EmptyState,
  ErrorState,
  FieldLabel,
  Input,
  PageSkeleton,
  Select,
  Textarea,
  Skeleton,
  cn,
} from "@/components/ui";
import {
  LayoutDashboard,
  Wand2,
  Sparkles,
  Undo2,
  Redo2,
  Plus,
  Trash2,
  Settings2,
  BarChart3,
  LineChart,
  PieChart,
  Table2,
  Type,
  Filter,
  Image as ImageIcon,
  Monitor,
  Smartphone,
  Grid3X3,
  Share2,
  Save,
  Eye,
  Maximize2,
  Copy,
  X,
  AlertCircle,
  Loader2,
  Gauge,
  ArrowDownUp,
  Funnel,
  ScatterChart,
  LayoutGrid,
  Grid2X2,
  Target,
  Signal,
  Network,
  LayoutTemplate,
  AppWindow,
  Plug,
  Database,
} from "lucide-react";
import Link from "next/link";
import {
  type DatasetListItem,
  modelForDataset,
  modelIdForDataset,
  rebindQueryToLiveDataset,
  widgetFieldDefaults,
} from "@/lib/semantic";
import { DASHBOARD_TEMPLATES, instantiateTemplate, prepareTemplateModel } from "@/lib/dashboard-templates";
import { DEFAULT_QUERY_LIMIT } from "@/lib/widget-config";
import { ThemeSegmented } from "@/components/theme-toggle";
import { parseLayoutTheme, useTheme } from "@/components/theme-provider";
import { readStoredAppearance, type Appearance } from "@/lib/theme";
const WIDGET_CATALOG: { type: WidgetType; label: string; icon: any; description: string; defaultW: number; defaultH: number }[] = [
  { type: "kpi", label: "KPI", icon: Monitor, description: "Métrica principal", defaultW: 3, defaultH: 2 },
  { type: "kpi_goal", label: "KPI com meta", icon: Target, description: "Valor, meta e progresso", defaultW: 4, defaultH: 3 },
  { type: "metric_group", label: "Grupo de KPIs", icon: LayoutTemplate, description: "2–4 métricas pequenas", defaultW: 4, defaultH: 3 },
  { type: "line", label: "Linha", icon: LineChart, description: "Tendência ao longo do tempo", defaultW: 6, defaultH: 4 },
  { type: "bar", label: "Barras", icon: BarChart3, description: "Comparar categorias", defaultW: 6, defaultH: 4 },
  { type: "area", label: "Área", icon: LineChart, description: "Volume acumulado", defaultW: 6, defaultH: 4 },
  { type: "pie", label: "Pizza", icon: PieChart, description: "Partes de um todo", defaultW: 4, defaultH: 4 },
  { type: "gauge", label: "Gauge", icon: Gauge, description: "Medidor circular", defaultW: 4, defaultH: 4 },
  { type: "waterfall", label: "Cascata", icon: ArrowDownUp, description: "Receitas e despesas", defaultW: 6, defaultH: 4 },
  { type: "funnel", label: "Funil", icon: Funnel, description: "Etapas e valores", defaultW: 4, defaultH: 5 },
  { type: "scatter", label: "Dispersão", icon: ScatterChart, description: "Correlação X/Y", defaultW: 6, defaultH: 4 },
  { type: "treemap", label: "Treemap", icon: LayoutGrid, description: "Hierarquia proporcional", defaultW: 5, defaultH: 4 },
  { type: "heatmap", label: "Heatmap", icon: Grid2X2, description: "Duas dimensões e medida", defaultW: 6, defaultH: 4 },
  { type: "sparkline", label: "Sparkline", icon: Signal, description: "Mini gráfico compacto", defaultW: 3, defaultH: 2 },
  { type: "decomposition_tree", label: "Árvore de decomposição", icon: Network, description: "Explorar hierarquias", defaultW: 5, defaultH: 5 },
  { type: "table", label: "Tabela", icon: Table2, description: "Dados tabulares", defaultW: 6, defaultH: 4 },
  { type: "text", label: "Texto", icon: Type, description: "Título ou anotação", defaultW: 4, defaultH: 2 },
  { type: "slicer", label: "Slicer", icon: Filter, description: "Filtro interativo", defaultW: 3, defaultH: 3 },
  { type: "image", label: "Imagem", icon: ImageIcon, description: "Logótipo ou ilustração", defaultW: 4, defaultH: 3 },
  { type: "markdown", label: "Markdown", icon: Type, description: "Texto formatado", defaultW: 4, defaultH: 3 },
  { type: "iframe", label: "Iframe", icon: AppWindow, description: "Embed de URL", defaultW: 6, defaultH: 4 },
];

function useDashboardHistory(initial: Widget[]) {
  const [history, setHistory] = useState<{ past: Widget[][]; present: Widget[]; future: Widget[][] }>({
    past: [],
    present: initial,
    future: [],
  });

  const push = useCallback((widgets: Widget[]) => {
    setHistory((h) => ({
      past: [...h.past, h.present].slice(-20),
      present: widgets,
      future: [],
    }));
  }, []);

  const undo = useCallback(() => {
    setHistory((h) => {
      if (h.past.length === 0) return h;
      const previous = h.past[h.past.length - 1];
      return { past: h.past.slice(0, -1), present: previous, future: [h.present, ...h.future] };
    });
  }, []);

  const redo = useCallback(() => {
    setHistory((h) => {
      if (h.future.length === 0) return h;
      const next = h.future[0];
      return { past: [...h.past, h.present], present: next, future: h.future.slice(1) };
    });
  }, []);

  const set = useCallback((widgets: Widget[]) => {
    setHistory((h) => ({ ...h, present: widgets }));
  }, []);

  return { widgets: history.present, set, push, undo, redo, canUndo: history.past.length > 0, canRedo: history.future.length > 0 };
}

const Grid = WidthProvider(GridLayout);


export default function DashboardEditorPage() {
  return (
    <Suspense fallback={<PageSkeleton />}>
      <DashboardEditorInner />
    </Suspense>
  );
}

function DashboardEditorInner() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const searchParams = useSearchParams();
  const qc = useQueryClient();
  const [edit, setEdit] = useState(true);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [selected, setSelected] = useState<string | null>(null);
  const [globalFilters, setGlobalFilters] = useState<DashboardFilter[]>([]);
  const [timeRange, setTimeRange] = useState<{ start?: string; end?: string }>({});
  const [aiPrompt, setAiPrompt] = useState("");
  const [aiOpen, setAiOpen] = useState(false);
  const [aiCompleteOpen, setAiCompleteOpen] = useState(false);
  const [aiCompletePrompt, setAiCompletePrompt] = useState("");
  const [aiCompleteDataset, setAiCompleteDataset] = useState("");
  const [aiCompleteStep, setAiCompleteStep] = useState(0);
  const [mobilePreview, setMobilePreview] = useState(false);
  const [hydrated, setHydrated] = useState(false);
  const [preferredDatasetId, setPreferredDatasetId] = useState(searchParams.get("dataset_id") || "");
  const [sourceFilter, setSourceFilter] = useState("");
  const { theme, setTheme } = useTheme();
  const [dashTheme, setDashTheme] = useState<Appearance>("light");

  const me = useQuery({ queryKey: ["me"], queryFn: () => api<{ role?: string }>("/api/v1/auth/me") });
  const canDelete = !me.data || me.data.role !== "viewer";
  const datasets = useQuery({ queryKey: ["datasets"], queryFn: () => api<any>("/api/v1/datasets") });
  const datasetList = normalizeArray<DatasetListItem>(datasets.data);
  const sourceOptions = useMemo(() => {
    const seen = new Map<string, string>();
    for (const d of datasetList) {
      if (d.source_id && d.source_name) seen.set(d.source_id, d.source_name);
    }
    return Array.from(seen, ([id, name]) => ({ id, name }));
  }, [datasetList]);
  const visibleDatasets = useMemo(
    () => (sourceFilter ? datasetList.filter((d) => d.source_id === sourceFilter) : datasetList),
    [datasetList, sourceFilter],
  );
  const aiConfig = useQuery({ queryKey: ["ai-config"], queryFn: () => api<{ openai_configured: boolean }>("/api/v1/ai/config") });
  const semantic = useQuery({ queryKey: ["semantic"], queryFn: () => api<any>("/api/v1/semantic-models") });
  const semanticModels = useMemo((): any[] => {
    const d: any = semantic.data;
    if (Array.isArray(d)) return d;
    if (Array.isArray(d?.data)) return d.data;
    return [];
  }, [semantic.data]);
  const d = useQuery({
    queryKey: ["dashboard", id],
    queryFn: () => api<{ name: string; description: string; layout: { widgets: Widget[]; theme?: Appearance }; workspace_id?: string }>(`/api/v1/dashboards/${id}`),
    enabled: !!id,
    retry: (count, err) => apiStatus(err) !== 404 && count < 1,
  });

  const history = useDashboardHistory(d.data?.layout?.widgets || []);
  const widgets = history.widgets;

  useEffect(() => {
    if (d.data && !hydrated) {
      setName(d.data.name);
      setDescription(d.data.description || "");
      history.set(d.data.layout?.widgets || []);
      const saved = parseLayoutTheme(d.data.layout);
      const next = saved || readStoredAppearance();
      setDashTheme(next);
      if (saved) setTheme(saved);
      setHydrated(true);
    }
    const ws = d.data?.workspace_id;
    if (ws && typeof window !== "undefined" && localStorage.getItem("thedobra.workspace") !== ws) {
      localStorage.setItem("thedobra.workspace", ws);
    }
  }, [d.data, hydrated, history]);

  useEffect(() => {
    if (!hydrated) return;
    setDashTheme(theme);
  }, [theme, hydrated]);

  useEffect(() => {
    if (!d.isError) return;
    if (apiStatus(d.error) === 404 || /não encontrado|not found/i.test((d.error as Error).message || "")) {
      toast.error("Este dashboard já não existe.");
      router.replace("/dashboards");
    }
  }, [d.isError, d.error, router]);

  useEffect(() => {
    if (!hydrated || datasets.isLoading || !datasetList.length) return;
    const liveIds = new Set(datasetList.map((ds) => ds.id));
    let changed = false;
    const next = widgets.map((w) => {
      if (!w.query || ["text", "image", "markdown", "iframe"].includes(w.type)) return w;
      if (w.query.dataset_id && liveIds.has(w.query.dataset_id)) return w;
      const query = rebindQueryToLiveDataset(w.query, w.type, datasetList, semanticModels, preferredDatasetId);
      if (!query || query.dataset_id === w.query.dataset_id) return w;
      changed = true;
      return { ...w, query };
    });
    if (changed) {
      history.set(next);
      const fallback = datasetList.find((ds) => next.some((w) => w.query?.dataset_id === ds.id));
      toast.message(`Visuais ligados ao conjunto «${fallback?.name || datasetList[0].name}».`);
    }
  }, [hydrated, datasets.isLoading, datasetList, semanticModels, preferredDatasetId, widgets, history.set]);

  const save = useMutation({
    mutationFn: () =>
      api(`/api/v1/dashboards/${id}`, {
        method: "PUT",
        body: JSON.stringify({ name, description, layout: { widgets, theme: dashTheme } }),
      }),
    onSuccess: () => {
      toast.success("Dashboard guardado");
      qc.invalidateQueries({ queryKey: ["dashboard", id] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const remove = useMutation({
    mutationFn: () => api(`/api/v1/dashboards/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success("Dashboard excluído");
      qc.invalidateQueries({ queryKey: ["dashboards"] });
      router.replace("/dashboards");
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const aiWidget = useMutation({
    mutationFn: () =>
      api<{ type: string; dimension: string; measure: string; sort: string; color: string }>("/api/v1/ai/generate-visual", {
        method: "POST",
        body: JSON.stringify({ prompt: aiPrompt, dataset_id: preferredDatasetId || datasetList[0]?.id }),
      }),
    onSuccess: (res) => {
      const ds = preferredDatasetId || datasetList[0]?.id;
      if (!ds) return;
      const type: WidgetType = (["kpi", "line", "bar", "area", "pie", "table", "slicer"] as string[]).includes(res.type) ? (res.type as WidgetType) : "bar";
      const w: Widget = {
        id: crypto.randomUUID(),
        type,
        title: `${res.measure} por ${res.dimension}`,
        layout: { x: (widgets.length * 4) % 12, y: 100, w: type === "kpi" ? 3 : 6, h: type === "kpi" ? 2 : 4 },
        query: { dataset_id: ds, measures: [res.measure], dimensions: res.dimension ? [res.dimension] : [], limit: DEFAULT_QUERY_LIMIT },
      };
      add(w);
      setAiOpen(false);
      setAiPrompt("");
      toast.success("Widget gerado");
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const aiComplete = useMutation({
    mutationFn: () =>
      api<{ id: string; name: string; widgets: any[]; source: string }>("/api/v1/ai/generate-dashboard", {
        method: "POST",
        body: JSON.stringify({ prompt: aiCompletePrompt, dataset_id: aiCompleteDataset || undefined }),
      }),
    onMutate: () => setAiCompleteStep(0),
    onSuccess: (res) => {
      const generated = (res.widgets || []).map((w: any): Widget => ({
        id: crypto.randomUUID(),
        type: (["kpi", "line", "bar", "area", "pie", "table", "slicer", "text"] as string[]).includes(w.type) ? (w.type as WidgetType) : "bar",
        title: w.title || "Widget",
        layout: {
          x: Number(w.layout?.x ?? 0),
          y: Number(w.layout?.y ?? 0) + 100 + widgets.length * 2,
          w: Number(w.layout?.w ?? 6),
          h: Number(w.layout?.h ?? 4),
        },
        query: w.query ? { ...w.query, dataset_id: w.query.dataset_id || datasetList[0]?.id } : undefined,
        text: w.text,
      }));
      if (generated.length > 0) {
        history.push([...widgets, ...generated]);
        toast.success(`${generated.length} widgets sugeridos adicionados`);
      } else {
        toast.error("A IA não sugeriu widgets compatíveis");
      }
      setAiCompleteOpen(false);
      setAiCompletePrompt("");
      setAiCompleteDataset("");
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const add = useCallback((w: Widget) => {
    history.push([...widgets, w]);
    setSelected(w.id);
  }, [history, widgets]);

  const updateWidgets = useCallback((fn: (prev: Widget[]) => Widget[]) => {
    history.push(fn(widgets));
  }, [history, widgets]);

  const applyTemplate = async (templateId: string) => {
    const tpl = DASHBOARD_TEMPLATES.find((t) => t.id === templateId);
    if (!tpl) return;
    const ds = preferredDatasetId || visibleDatasets[0]?.id || datasetList[0]?.id;
    if (!ds) {
      toast.error("Ligue um conector ou carregue um conjunto antes de aplicar um template.");
      return;
    }
    try {
      const mdl = await prepareTemplateModel(ds, tpl, semanticModels);
      history.push(instantiateTemplate(tpl, ds, mdl));
      setSelected(null);
      qc.invalidateQueries({ queryKey: ["semantic"] });
      toast.success(`Painel «${tpl.name}» aplicado`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Não foi possível aplicar o template.");
    }
  };

  const layout = useMemo<Layout[]>(() => widgets.map((w) => ({ i: w.id, x: w.layout.x, y: w.layout.y, w: w.layout.w, h: w.layout.h, minW: 2, minH: 2 })), [widgets]);

  const onLayoutChange = useCallback((next: Layout[]) => {
    const mapped = widgets.map((w) => {
      const l = next.find((x) => x.i === w.id);
      return l ? { ...w, layout: { x: l.x, y: l.y, w: l.w, h: l.h } } : w;
    });
    history.push(mapped);
  }, [history, widgets]);

  const addWidget = (type: WidgetType) => {
    const ds = preferredDatasetId || visibleDatasets[0]?.id || datasetList[0]?.id;
    const catalog = WIDGET_CATALOG.find((t) => t.type === type)!;
    const x = (widgets.length * 4) % 12;
    const noQueryTypes = ["text", "image", "markdown", "iframe"];
    const mdl = modelForDataset(semanticModels, ds);
    const fields = widgetFieldDefaults(type, mdl);
    if (!ds && !noQueryTypes.includes(type)) {
      toast.error("Ainda sem conjuntos. Vá a Conectores ou carregue a demo.");
      return;
    }
    const w: Widget = {
      id: crypto.randomUUID(),
      type,
      title: catalog.label,
      layout: { x, y: 100, w: catalog.defaultW, h: catalog.defaultH },
      query: ds && !noQueryTypes.includes(type) ? { dataset_id: ds, measures: fields.measures, dimensions: fields.dimensions, limit: DEFAULT_QUERY_LIMIT } : undefined,
      text: type === "text" ? "Novo texto" : undefined,
      hierarchy: type === "decomposition_tree" ? fields.dimensions : undefined,
      config: (() => {
        if (type === "image") return { imageUrl: "" };
        if (type === "markdown") return { markdown: "## Nota\nEdite aqui." };
        if (type === "gauge") return { min: 0, max: 100, target: 80, color: "#2563EB", gaugeLabel: "Valor" };
        if (type === "waterfall") return { waterfallNegativeCategories: "" };
        if (type === "scatter") return { xMeasure: fields.measures[1], yMeasure: fields.measures[0], dimension: fields.dimensions[0] };
        if (type === "kpi_goal") return { goal: 100, color: "#2563EB" };
        if (type === "sparkline") return { color: "#2563EB" };
        if (type === "iframe") return { url: "" };
        return {};
      })(),
    };
    add(w);
  };

  const removeWidget = (id: string) => updateWidgets((prev) => prev.filter((w) => w.id !== id));
  const duplicateWidget = (id: string) => {
    const w = widgets.find((x) => x.id === id);
    if (!w) return;
    add({ ...w, id: crypto.randomUUID(), layout: { ...w.layout, x: (w.layout.x + 2) % 12, y: w.layout.y } });
  };

  const applyFilter = useCallback((dim: string, value: any, op?: "eq" | "in", datasetId?: string) => {
    setGlobalFilters((prev) => {
      const same = (f: DashboardFilter) => f.dimension === dim && (f.dataset_id || "") === (datasetId || "");
      if (value == null || value === "" || (Array.isArray(value) && value.length === 0)) {
        return prev.filter((f) => !same(f));
      }
      const nextOp = op || (Array.isArray(value) ? "in" : "eq");
      const existing = prev.find(same);
      if (existing) {
        return prev.map((f) => (same(f) ? { ...f, value, op: nextOp, dataset_id: datasetId } : f));
      }
      return [...prev, { dimension: dim, op: nextOp, value, dataset_id: datasetId }];
    });
  }, []);

  const drill = useCallback((widgetId: string, value: string) => {
    updateWidgets((prev) =>
      prev.map((w) => {
        if (w.id !== widgetId) return w;
        const path = w.drillPath || [];
        if (value === "up") return { ...w, drillPath: path.slice(0, -1) };
        return { ...w, drillPath: [...path, value] };
      })
    );
  }, [updateWidgets]);

  useEffect(() => {
    if (!edit || !selected) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setSelected(null);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [edit, selected]);

  const current = widgets.find((w) => w.id === selected);
  const currentDataset = current?.query?.dataset_id;
  const model = useMemo(() => modelForDataset(semanticModels, currentDataset), [currentDataset, semanticModels]);
  const semanticModelId = useMemo(() => modelIdForDataset(semanticModels, currentDataset), [currentDataset, semanticModels]);


  if (d.isError) {
    if (apiStatus(d.error) === 404) return <PageSkeleton />;
    return <ErrorState message={(d.error as Error).message} onRetry={() => d.refetch()} />;
  }
  if (!d.data && widgets.length === 0) return <PageSkeleton />;

  return (
    <div className="-m-4 flex h-[calc(100vh-3.5rem)] min-h-0 flex-col bg-surface-2 sm:-m-6">
      <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-line bg-surface px-3 py-2 sm:px-4">
        <nav className="flex min-w-0 items-center gap-1.5 text-[12px] text-mute">
          <Link href="/dashboards" className="hover:text-ink">Dashboards</Link>
          <span className="text-line">/</span>
        </nav>
        {edit ? (
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Nome do dashboard"
            className="h-8 max-w-[260px] border-0 bg-transparent px-1 text-[15px] font-semibold shadow-none"
          />
        ) : (
          <h1 className="truncate text-[15px] font-semibold text-ink">{name || "Dashboard"}</h1>
        )}
        {edit && (
          <Input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Descrição"
            className="hidden h-8 max-w-xs border-0 bg-transparent px-1 text-[12px] text-mute shadow-none lg:block"
          />
        )}
        <div className="ml-auto flex flex-wrap items-center gap-1.5">
          <ThemeSegmented
            value={dashTheme}
            onChange={(next) => {
              setDashTheme(next);
              setTheme(next);
            }}
          />
          {edit && (
            <>
              <Button variant="ghost" size="icon" onClick={history.undo} disabled={!history.canUndo} title="Desfazer">
                <Undo2 size={16} />
              </Button>
              <Button variant="ghost" size="icon" onClick={history.redo} disabled={!history.canRedo} title="Refazer">
                <Redo2 size={16} />
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setEdit(false)}>
                <Eye size={14} /> Pré-visualizar
              </Button>
            </>
          )}
          {!edit && (
            <Button variant="secondary" size="sm" onClick={() => setEdit(true)}>
              <Settings2 size={14} /> Editar
            </Button>
          )}
          <Button variant="secondary" size="sm" onClick={() => setAiOpen(!aiOpen)}>
            <Wand2 size={14} /> Gerar
          </Button>
          <Button variant="secondary" size="sm" onClick={() => setAiCompleteOpen(true)}>
            <Sparkles size={14} /> Completar
          </Button>
          <Button variant="secondary" size="sm" onClick={async () => {
            try {
              const d = await api<{ url: string }>(`/api/v1/dashboards/${id}/share`, { method: "POST" });
              await navigator.clipboard?.writeText(d.url);
              toast.success("Ligação de partilha copiada");
            } catch (e: any) { toast.error(e.message); }
          }}>
            <Share2 size={14} /> Partilhar
          </Button>
          {canDelete && (
            <Button
              variant="danger"
              size="sm"
              onClick={() => {
                if (confirm(`Excluir o dashboard «${name || d.data?.name}»? Esta ação não pode ser desfeita.`)) {
                  remove.mutate();
                }
              }}
              busy={remove.isPending}
            >
              <Trash2 size={14} /> Excluir
            </Button>
          )}
          <Button size="sm" onClick={() => save.mutate()} busy={save.isPending}>
            <Save size={14} /> Guardar
          </Button>
        </div>
      </div>

      {edit && (
        <div className="flex shrink-0 items-center gap-1.5 overflow-x-auto border-b border-line bg-surface px-3 py-1.5">
          <span className="shrink-0 text-[10px] font-semibold uppercase tracking-wide text-mute">Adicionar</span>
          {WIDGET_CATALOG.map((t) => {
            const Icon = t.icon;
            return (
              <button
                key={t.type}
                type="button"
                title={t.description}
                onClick={() => addWidget(t.type)}
                className="flex shrink-0 items-center gap-1.5 rounded-lg border border-line bg-surface-2 px-2 py-1 text-[11px] text-ink transition hover:border-primary hover:bg-primary/5"
              >
                <Icon size={13} className="text-primary" />
                {t.label}
              </button>
            );
          })}
          <div className="mx-1 h-5 w-px shrink-0 bg-line" />
          <Link href="/store" className="shrink-0 text-[11px] text-primary hover:underline">Loja</Link>
          {DASHBOARD_TEMPLATES.map((t) => (
            <button
              key={t.id}
              type="button"
              onClick={() => applyTemplate(t.id)}
              className="flex shrink-0 items-center gap-1 rounded-lg px-2 py-1 text-[11px] text-mute hover:bg-surface-2 hover:text-ink"
            >
              <Grid3X3 size={12} className="text-primary" />
              {t.name}
            </button>
          ))}
        </div>
      )}

      <div className="flex shrink-0 flex-wrap items-end gap-2 border-b border-line bg-surface px-3 py-1.5 sm:px-4">
        <FieldLabel label="Período início">
          <Input type="date" value={timeRange.start || ""} onChange={(e) => setTimeRange({ ...timeRange, start: e.target.value })} className="h-8 w-[10.5rem]" />
        </FieldLabel>
        <FieldLabel label="Período fim">
          <Input type="date" value={timeRange.end || ""} onChange={(e) => setTimeRange({ ...timeRange, end: e.target.value })} className="h-8 w-[10.5rem]" />
        </FieldLabel>
        {edit && (
          <Button variant={mobilePreview ? "primary" : "secondary"} size="sm" onClick={() => setMobilePreview(!mobilePreview)}>
            <Smartphone size={14} /> Mobile
          </Button>
        )}
        {globalFilters.length > 0 && (
          <div className="flex flex-wrap items-center gap-1 pb-0.5">
            {globalFilters.map((f) => (
              <span key={`${f.dataset_id || ""}:${f.dimension}`} className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-1 text-[11px] text-primary-600">
                {f.dimension} = {Array.isArray(f.value) ? f.value.join(", ") : String(f.value)}
                <button className="text-primary-700" onClick={() => setGlobalFilters(globalFilters.filter((x) => !(x.dimension === f.dimension && (x.dataset_id || "") === (f.dataset_id || ""))))}>×</button>
              </span>
            ))}
            <button className="text-[11px] text-mute hover:text-ink" onClick={() => setGlobalFilters([])}>Limpar filtros</button>
          </div>
        )}
      </div>

      {edit && datasetList.length === 0 && (
        <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-amber-200 bg-amber-50 px-4 py-2 text-[12px] text-amber-950">
          <Database size={14} />
          <span>Ainda sem conjuntos. Ligue um conector ou carregue a demo.</span>
          <Link href="/connectors"><Button size="sm"><Plug size={14} /> Conectores</Button></Link>
          <Link href="/data"><Button variant="secondary" size="sm">Carregar demo</Button></Link>
        </div>
      )}

      {aiOpen && (
        <div className="shrink-0 border-b border-line bg-surface px-4 py-2">
          <div className="flex gap-2">
            <Input value={aiPrompt} onChange={(e) => setAiPrompt(e.target.value)} placeholder="Ex.: vendas por região em barras" />
            <Button size="sm" onClick={() => aiWidget.mutate()} busy={aiWidget.isPending} disabled={!aiPrompt.trim()}>Gerar</Button>
          </div>
        </div>
      )}

      <div className="relative min-h-0 flex-1">
        <div
          className={cn("h-full overflow-auto pb-10", edit && current && "pr-80")}
          onClick={() => edit && setSelected(null)}
        >
          {widgets.length === 0 ? (
            <EmptyState
              icon={LayoutDashboard}
              title="Canvas vazio"
              description={
                datasetList.length === 0
                  ? "Primeiro precisa de dados. Sincronize um conector ou carregue a demo."
                  : edit
                    ? "Escolha um componente na faixa acima para começar."
                    : "Este dashboard ainda não tem widgets."
              }
              action={
                datasetList.length === 0 ? (
                  <Link href="/connectors">
                    <Button>
                      <Plug size={14} /> Ir a Conectores
                    </Button>
                  </Link>
                ) : edit ? (
                  <Button onClick={() => addWidget("kpi")}>
                    <Plus size={14} /> Adicionar KPI
                  </Button>
                ) : undefined
              }
            />
          ) : (
            <Grid
              className="layout min-h-full"
              layout={layout}
              cols={mobilePreview ? 4 : 12}
              rowHeight={mobilePreview ? 110 : 96}
              margin={[14, 14]}
              containerPadding={[12, 12]}
              isDraggable={edit}
              isResizable={edit}
              onLayoutChange={onLayoutChange}
              draggableHandle=".drag-handle"
              compactType="vertical"
            >
              {widgets.map((w) => (
                <div
                  key={w.id}
                  className={`widget-grid-item relative ${selected === w.id && edit ? "ring-2 ring-primary/40" : ""}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    if (edit) setSelected(w.id);
                  }}
                >
                  {edit && (
                    <div className="absolute right-2 top-2 z-10 flex items-center gap-1">
                      <div className="drag-handle flex h-8 cursor-move items-center gap-1 rounded-lg bg-surface/95 px-2 text-[10px] text-mute shadow-sm">
                        <Maximize2 size={12} /> Mover
                      </div>
                      <button className="flex h-8 items-center rounded-lg bg-surface/95 p-2 text-mute shadow-sm hover:text-accent" onClick={() => duplicateWidget(w.id)} title="Duplicar">
                        <Copy size={12} />
                      </button>
                      <button className="flex h-8 items-center rounded-lg bg-surface/95 p-2 text-mute shadow-sm hover:text-danger" onClick={() => removeWidget(w.id)} title="Remover">
                        <Trash2 size={12} />
                      </button>
                    </div>
                  )}
                  <WidgetView w={w} globalFilters={globalFilters} timeRange={timeRange} onFilter={(dim, value, op) => applyFilter(dim, value, op, w.query?.dataset_id)} onDrill={drill} />
                </div>
              ))}
            </Grid>
          )}
        </div>

        {edit && current && (
          <div className="absolute inset-y-0 right-0 z-20">
            <WidgetInspector
              widget={current}
              catalog={WIDGET_CATALOG}
              model={model}
              visibleDatasets={visibleDatasets}
              sourceOptions={sourceOptions}
              sourceFilter={sourceFilter}
              onSourceFilter={setSourceFilter}
              onPreferredDataset={setPreferredDatasetId}
              semanticModels={semanticModels}
              semanticModelId={semanticModelId}
              onUpdate={(fn) => updateWidgets((p) => p.map((w) => (w.id === current.id ? fn(w) : w)))}
              onDrillUp={() => drill(current.id, "up")}
              onClose={() => setSelected(null)}
            />
          </div>
        )}
      </div>


      {aiCompleteOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
          <Card className="w-full max-w-lg space-y-4">
            <div className="flex items-start justify-between">
              <div>
                <h3 className="text-lg font-medium text-ink">Completar com IA</h3>
                <p className="text-[13px] text-mute">Descreva o que adicionar ao dashboard actual.</p>
              </div>
              <button onClick={() => setAiCompleteOpen(false)} className="rounded-lg p-1 text-mute hover:bg-surface-2">
                <X size={18} />
              </button>
            </div>

            {aiConfig.data?.openai_configured === false && (
              <div className="flex items-start gap-2 rounded-xl border border-amber-200 bg-amber-50 p-3 text-[13px] text-amber-800">
                <AlertCircle size={16} className="mt-0.5 shrink-0" />
                <span>Configure OPENAI_API_KEY para geração inteligente; até lá usa sugestões automáticas.</span>
              </div>
            )}

            {aiComplete.isPending ? (
              <div className="space-y-4 py-4">
                {["A analisar dados…", "A escolher visualizações…", "A montar widgets…"].map((s, i) => (
                  <div key={s} className="flex items-center gap-3">
                    <div className={cn("flex h-6 w-6 items-center justify-center rounded-full text-[11px] font-medium", i <= aiCompleteStep ? "bg-primary text-white" : "bg-surface-2 text-mute")}>
                      {i < aiCompleteStep ? "✓" : i + 1}
                    </div>
                    <span className={cn("text-sm", i <= aiCompleteStep ? "text-ink" : "text-mute")}>{s}</span>
                    {i === aiCompleteStep && <Loader2 size={14} className="ml-auto animate-spin text-primary" />}
                  </div>
                ))}
                <Skeleton className="h-32 w-full" />
              </div>
            ) : (
              <>
                <div className="space-y-3">
                  <label className="block text-[13px] font-medium text-ink">
                    O que adicionar?
                    <Textarea value={aiCompletePrompt} onChange={(e) => setAiCompletePrompt(e.target.value)} placeholder="Ex.: adiciona KPI de margem, gráfico de tendência e tabela por região" className="mt-1.5 min-h-24" />
                  </label>
                  <label className="block text-[13px] font-medium text-ink">
                    Conjunto de dados (opcional)
                    <Select value={aiCompleteDataset} onChange={(e) => setAiCompleteDataset(e.target.value)} className="mt-1.5">
                      <option value="">Automático</option>
                      {datasetList.map((ds) => <option key={ds.id} value={ds.id}>{ds.name}</option>)}
                    </Select>
                  </label>
                </div>
                <div className="flex justify-end gap-2">
                  <Button variant="secondary" onClick={() => setAiCompleteOpen(false)}>Cancelar</Button>
                  <Button onClick={() => aiComplete.mutate()} disabled={!aiCompletePrompt.trim()} busy={aiComplete.isPending}>
                    <Wand2 size={14} /> Adicionar sugestões
                  </Button>
                </div>
              </>
            )}
          </Card>
        </div>
      )}
    </div>
  );
}

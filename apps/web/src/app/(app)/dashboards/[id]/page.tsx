"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { useParams, useSearchParams } from "next/navigation";
import { WidgetView, type Widget, type DashboardFilter, type WidgetType } from "@/components/WidgetView";
import { toast } from "sonner";
import GridLayout, { WidthProvider, type Layout } from "react-grid-layout";
import "react-grid-layout/css/styles.css";
import "react-resizable/css/styles.css";
import {
  Button,
  Card,
  CardTitle,
  EmptyState,
  ErrorState,
  FieldLabel,
  Input,
  PageHeader,
  PageSkeleton,
  Select,
  Textarea,
  Badge,
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
  EyeOff,
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
  remapQueryToModel,
  widgetFieldDefaults,
} from "@/lib/semantic";
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

const TEMPLATES = [
  {
    id: "executive",
    name: "Executivo",
    widgets: (): Widget[] => [
      { id: crypto.randomUUID(), type: "kpi", title: "Receita", layout: { x: 0, y: 0, w: 3, h: 2 }, query: { measures: ["revenue"] } },
      { id: crypto.randomUUID(), type: "kpi", title: "Margem", layout: { x: 3, y: 0, w: 3, h: 2 }, query: { measures: ["margin"] } },
      { id: crypto.randomUUID(), type: "kpi", title: "Clientes", layout: { x: 6, y: 0, w: 3, h: 2 }, query: { measures: ["customers"] } },
      { id: crypto.randomUUID(), type: "kpi", title: "Pedidos", layout: { x: 9, y: 0, w: 3, h: 2 }, query: { measures: ["orders"] } },
      { id: crypto.randomUUID(), type: "line", title: "Receita ao longo do tempo", layout: { x: 0, y: 2, w: 8, h: 5 }, query: { measures: ["revenue"], dimensions: ["date"], limit: 90 } },
      { id: crypto.randomUUID(), type: "bar", title: "Receita por região", layout: { x: 8, y: 2, w: 4, h: 5 }, query: { measures: ["revenue"], dimensions: ["region"], limit: 20 } },
    ],
  },
  {
    id: "sales",
    name: "Vendas",
    widgets: (): Widget[] => [
      { id: crypto.randomUUID(), type: "kpi", title: "Total de vendas", layout: { x: 0, y: 0, w: 4, h: 2 }, query: { measures: ["revenue"] } },
      { id: crypto.randomUUID(), type: "slicer", title: "Região", layout: { x: 4, y: 0, w: 4, h: 2 }, query: { dimensions: ["region"], measures: [] } },
      { id: crypto.randomUUID(), type: "bar", title: "Vendas por vendedor", layout: { x: 0, y: 2, w: 6, h: 5 }, query: { measures: ["revenue"], dimensions: ["sales_rep"], limit: 20 } },
      { id: crypto.randomUUID(), type: "pie", title: "Mix de produtos", layout: { x: 6, y: 2, w: 6, h: 5 }, query: { measures: ["revenue"], dimensions: ["product"], limit: 10 } },
    ],
  },
  {
    id: "operations",
    name: "Operações",
    widgets: (): Widget[] => [
      { id: crypto.randomUUID(), type: "kpi", title: "Pedidos hoje", layout: { x: 0, y: 0, w: 3, h: 2 }, query: { measures: ["orders"] } },
      { id: crypto.randomUUID(), type: "kpi", title: "Tempo médio", layout: { x: 3, y: 0, w: 3, h: 2 }, query: { measures: ["avg_time"] } },
      { id: crypto.randomUUID(), type: "table", title: "Status dos pedidos", layout: { x: 0, y: 2, w: 6, h: 5 }, query: { measures: ["orders"], dimensions: ["status"], limit: 20 } },
      { id: crypto.randomUUID(), type: "area", title: "Volume por hora", layout: { x: 6, y: 2, w: 6, h: 5 }, query: { measures: ["orders"], dimensions: ["hour"], limit: 24 } },
    ],
  },
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
  const { id } = useParams<{ id: string }>();
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
  const [activeTab, setActiveTab] = useState("data");
  const [hydrated, setHydrated] = useState(false);
  const [preferredDatasetId, setPreferredDatasetId] = useState(searchParams.get("dataset_id") || "");
  const [sourceFilter, setSourceFilter] = useState("");

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
    queryFn: () => api<{ name: string; description: string; layout: { widgets: Widget[] } }>(`/api/v1/dashboards/${id}`),
  });

  const history = useDashboardHistory(d.data?.layout?.widgets || []);
  const widgets = history.widgets;

  useEffect(() => {
    if (d.data && !hydrated) {
      setName(d.data.name);
      setDescription(d.data.description || "");
      history.set(d.data.layout?.widgets || []);
      setHydrated(true);
    }
  }, [d.data, hydrated, history]);

  const save = useMutation({
    mutationFn: () =>
      api(`/api/v1/dashboards/${id}`, {
        method: "PUT",
        body: JSON.stringify({ name, description, layout: { widgets } }),
      }),
    onSuccess: () => {
      toast.success("Dashboard guardado");
      qc.invalidateQueries({ queryKey: ["dashboard", id] });
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
        query: { dataset_id: ds, measures: [res.measure], dimensions: res.dimension ? [res.dimension] : [], limit: 20 },
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

  const applyTemplate = (templateId: string) => {
    const tpl = TEMPLATES.find((t) => t.id === templateId);
    if (!tpl) return;
    const ds = preferredDatasetId || visibleDatasets[0]?.id || datasetList[0]?.id;
    if (!ds) {
      toast.error("Ligue um conector ou carregue um conjunto antes de aplicar um template.");
      return;
    }
    const mdl = modelForDataset(semanticModels, ds);
    const newWidgets = tpl.widgets().map((w) => ({
      ...w,
      id: crypto.randomUUID(),
      query: { ...remapQueryToModel(w.query, w.type, mdl), dataset_id: ds },
    }));
    history.push(newWidgets);
    setSelected(null);
    toast.success(`Template ${tpl.name} aplicado`);
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
      query: ds && !noQueryTypes.includes(type) ? { dataset_id: ds, measures: fields.measures, dimensions: fields.dimensions, limit: 20 } : undefined,
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

  const applyFilter = useCallback((dim: string, value: any) => {
    setGlobalFilters((prev) => {
      const existing = prev.find((f) => f.dimension === dim);
      if (existing) {
        return prev.map((f) => (f.dimension === dim ? { ...f, value } : f));
      }
      return [...prev, { dimension: dim, op: "eq", value }];
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

  const current = widgets.find((w) => w.id === selected);
  const currentDataset = current?.query?.dataset_id;
  const model = useMemo(() => modelForDataset(semanticModels, currentDataset), [currentDataset, semanticModels]);

  if (d.isError) return <ErrorState message={(d.error as Error).message} onRetry={() => d.refetch()} />;
  if (!d.data && widgets.length === 0) return <PageSkeleton />;

  return (
    <div className="flex h-[calc(100vh-7rem)] gap-3">
      {edit && (
        <aside className="flex w-56 flex-col gap-3 overflow-y-auto rounded-2xl border border-line bg-surface p-3 shadow-sm">
          <div className="flex items-center justify-between">
            <span className="text-[12px] font-semibold uppercase text-mute">Componentes</span>
            <Button variant="ghost" size="icon" onClick={() => setEdit(false)} title="Fechar painel">
              <EyeOff size={16} />
            </Button>
          </div>
          <div className="grid grid-cols-2 gap-2">
            {WIDGET_CATALOG.map((t) => {
              const Icon = t.icon;
              return (
                <button
                  key={t.type}
                  onClick={() => addWidget(t.type)}
                  className="flex flex-col items-center gap-1 rounded-xl border border-line bg-white p-3 text-center transition hover:border-primary hover:shadow-sm"
                >
                  <Icon size={18} className="text-primary" />
                  <span className="text-[11px] font-medium text-ink">{t.label}</span>
                  <span className="text-[10px] text-mute">{t.description}</span>
                </button>
              );
            })}
          </div>
          <div className="mt-2 border-t border-line pt-3">
            <span className="text-[12px] font-semibold uppercase text-mute">Templates</span>
            <div className="mt-2 space-y-1">
              {TEMPLATES.map((t) => (
                <button
                  key={t.id}
                  onClick={() => applyTemplate(t.id)}
                  className="flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left text-[12px] text-ink hover:bg-surface-2"
                >
                  <Grid3X3 size={14} className="text-primary" />
                  {t.name}
                </button>
              ))}
            </div>
          </div>
        </aside>
      )}

      <div className="flex min-w-0 flex-1 flex-col gap-3">
        <PageHeader
          title={edit ? name || "Dashboard" : name}
          description={edit ? description : undefined}
          crumbs={[{ href: "/dashboards", label: "Dashboards" }]}
          actions={
            <div className="flex flex-wrap items-center gap-2">
              {edit && (
                <>
                  <Button variant="ghost" size="icon" onClick={history.undo} disabled={!history.canUndo} title="Desfazer">
                    <Undo2 size={16} />
                  </Button>
                  <Button variant="ghost" size="icon" onClick={history.redo} disabled={!history.canRedo} title="Refazer">
                    <Redo2 size={16} />
                  </Button>
                  <Button variant="secondary" onClick={() => setEdit(false)}>
                    <Eye size={14} /> Pré-visualizar
                  </Button>
                </>
              )}
              {!edit && (
                <Button variant="secondary" onClick={() => setEdit(true)}>
                  <Settings2 size={14} /> Editar
                </Button>
              )}
              <Button variant="secondary" onClick={() => setAiOpen(!aiOpen)}>
                <Wand2 size={14} /> Gerar com IA
              </Button>
              <Button variant="secondary" onClick={() => setAiCompleteOpen(true)}>
                <Sparkles size={14} /> Completar com IA
              </Button>
              <Button variant="secondary" onClick={async () => {
                try {
                  const d = await api<{ url: string }>(`/api/v1/dashboards/${id}/share`, { method: "POST" });
                  await navigator.clipboard?.writeText(d.url);
                  toast.success("Ligação de partilha copiada");
                } catch (e: any) { toast.error(e.message); }
              }}>
                <Share2 size={14} /> Partilhar
              </Button>
              <Button onClick={() => save.mutate()} busy={save.isPending}>
                <Save size={14} /> Guardar
              </Button>
            </div>
          }
        />

        {edit && datasetList.length === 0 && (
          <EmptyState
            icon={Database}
            title="Ainda sem conjuntos de dados"
            description="O editor precisa de um conjunto sincronizado. Ligue um conector (Inflação, CSV, Asaas…) ou carregue a demo de vendas."
            action={
              <div className="flex flex-wrap gap-2">
                <Link href="/connectors">
                  <Button>
                    <Plug size={14} /> Ir a Conectores
                  </Button>
                </Link>
                <Link href="/data">
                  <Button variant="secondary">Carregar demo</Button>
                </Link>
              </div>
            }
          />
        )}

        {edit && (
          <div className="flex items-center gap-2">
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Nome do dashboard" className="max-w-xs" />
            <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Descrição" className="max-w-md" />
            <Button variant={mobilePreview ? "primary" : "secondary"} size="sm" onClick={() => setMobilePreview(!mobilePreview)}>
              <Smartphone size={14} /> Mobile
            </Button>
          </div>
        )}

        {aiOpen && (
          <Card className="space-y-3">
            <CardTitle>Gerar widget com IA</CardTitle>
            <div className="flex gap-2">
              <Input value={aiPrompt} onChange={(e) => setAiPrompt(e.target.value)} placeholder="Ex.: vendas por região em barras" />
              <Button onClick={() => aiWidget.mutate()} busy={aiWidget.isPending} disabled={!aiPrompt.trim()}>Gerar</Button>
            </div>
          </Card>
        )}

        <Card className="space-y-3">
          <div className="flex flex-wrap items-end gap-3">
            <FieldLabel label="Período início">
              <Input type="date" value={timeRange.start || ""} onChange={(e) => setTimeRange({ ...timeRange, start: e.target.value })} />
            </FieldLabel>
            <FieldLabel label="Período fim">
              <Input type="date" value={timeRange.end || ""} onChange={(e) => setTimeRange({ ...timeRange, end: e.target.value })} />
            </FieldLabel>
            {globalFilters.length > 0 && (
              <div className="flex flex-wrap gap-1">
                {globalFilters.map((f) => (
                  <span key={f.dimension} className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-1 text-[11px] text-primary-600">
                    {f.dimension} = {String(f.value)}
                    <button className="text-primary-700" onClick={() => setGlobalFilters(globalFilters.filter((x) => x.dimension !== f.dimension))}>×</button>
                  </span>
                ))}
                <button className="text-[11px] text-mute hover:text-ink" onClick={() => setGlobalFilters([])}>Limpar filtros</button>
              </div>
            )}
          </div>
        </Card>

        <div className="relative flex-1 overflow-hidden rounded-2xl border border-line bg-surface shadow-sm">
          {widgets.length === 0 ? (
            <EmptyState
              icon={LayoutDashboard}
              title="Canvas vazio"
              description={
                datasetList.length === 0
                  ? "Primeiro precisa de dados. Sincronize um conector ou carregue a demo."
                  : edit
                    ? "Escolha um componente ou template à esquerda para começar."
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
              rowHeight={mobilePreview ? 90 : 70}
              isDraggable={edit}
              isResizable={edit}
              onLayoutChange={onLayoutChange}
              draggableHandle=".drag-handle"
              compactType="vertical"
            >
              {widgets.map((w) => (
                <div
                  key={w.id}
                  className={`relative ${selected === w.id && edit ? "ring-2 ring-primary/30" : ""}`}
                  onClick={() => edit && setSelected(w.id)}
                >
                  {edit && (
                    <div className="absolute right-2 top-2 z-10 flex items-center gap-1">
                      <div className="drag-handle flex h-8 cursor-move items-center gap-1 rounded-lg bg-white/95 px-2 text-[10px] text-mute shadow-sm">
                        <Maximize2 size={12} /> Mover
                      </div>
                      <button className="flex h-8 items-center rounded-lg bg-white/95 p-2 text-mute shadow-sm hover:text-accent" onClick={() => duplicateWidget(w.id)} title="Duplicar">
                        <Copy size={12} />
                      </button>
                      <button className="flex h-8 items-center rounded-lg bg-white/95 p-2 text-mute shadow-sm hover:text-danger" onClick={() => removeWidget(w.id)} title="Remover">
                        <Trash2 size={12} />
                      </button>
                    </div>
                  )}
                  <WidgetView w={w} globalFilters={globalFilters} timeRange={timeRange} onFilter={applyFilter} onDrill={drill} />
                </div>
              ))}
            </Grid>
          )}
        </div>
      </div>

      {edit && current && (
        <aside className="w-80 shrink-0 overflow-y-auto rounded-2xl border border-line bg-surface p-4 shadow-sm">
          <div className="mb-3 flex items-center justify-between">
            <span className="text-[13px] font-semibold text-ink">Propriedades</span>
            <Badge tone="accent">{WIDGET_CATALOG.find((t) => t.type === current.type)?.label}</Badge>
          </div>
          <Tabs value={activeTab} onValueChange={setActiveTab}>
            <TabsList className="grid w-full grid-cols-3">
              <TabsTrigger value="data" active={activeTab === "data"} onClick={() => setActiveTab("data")}>Dados</TabsTrigger>
              <TabsTrigger value="format" active={activeTab === "format"} onClick={() => setActiveTab("format")}>Formato</TabsTrigger>
              <TabsTrigger value="filters" active={activeTab === "filters"} onClick={() => setActiveTab("filters")}>Filtros</TabsTrigger>
            </TabsList>
            <TabsContent value="data" activeValue={activeTab} className="space-y-3 pt-3">
              <FieldLabel label="Título">
                <Input value={current.title} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, title: e.target.value } : w)))} />
              </FieldLabel>
              {current.type === "text" && (
                <FieldLabel label="Texto">
                  <Textarea value={current.text} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, text: e.target.value } : w)))} />
                </FieldLabel>
              )}
              {current.type === "iframe" && (
                <FieldLabel label="URL do embed" hint="Use apenas fontes confiáveis. Conteúdo externo pode definir cookies.">
                  <Input value={current.config?.url || ""} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, url: e.target.value } } : w)))} placeholder="https://..." />
                </FieldLabel>
              )}
              {(current.type === "image" || current.type === "markdown") && (
                <>
                  {current.type === "image" && (
                    <FieldLabel label="URL da imagem">
                      <Input value={current.config?.imageUrl || ""} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, imageUrl: e.target.value } } : w)))} />
                    </FieldLabel>
                  )}
                  {current.type === "markdown" && (
                    <FieldLabel label="Markdown">
                      <Textarea value={current.config?.markdown || ""} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, markdown: e.target.value } } : w)))} />
                    </FieldLabel>
                  )}
                </>
              )}
              {!["text", "image", "markdown", "iframe"].includes(current.type) && (
                <>
                  {sourceOptions.length > 0 && (
                    <FieldLabel label="Origem">
                      <Select
                        value={sourceFilter}
                        onChange={(e) => setSourceFilter(e.target.value)}
                      >
                        <option value="">Todos os conjuntos</option>
                        {sourceOptions.map((s) => (
                          <option key={s.id} value={s.id}>
                            Conector {s.name}
                          </option>
                        ))}
                      </Select>
                    </FieldLabel>
                  )}
                  <FieldLabel label="Conjunto">
                    <Select
                      value={current.query?.dataset_id || ""}
                      onChange={(e) => {
                        const nextId = e.target.value;
                        setPreferredDatasetId(nextId);
                        const nextModel = modelForDataset(semanticModels, nextId);
                        updateWidgets((p) =>
                          p.map((w) => {
                            if (w.id !== current.id) return w;
                            const q = remapQueryToModel({ ...w.query, dataset_id: nextId }, w.type, nextModel);
                            return { ...w, query: q };
                          }),
                        );
                      }}
                    >
                      <option value="">—</option>
                      {visibleDatasets.map((ds) => (
                        <option key={ds.id} value={ds.id}>
                          {ds.name}
                          {ds.source_name ? ` · ${ds.source_name}` : ""}
                        </option>
                      ))}
                    </Select>
                  </FieldLabel>
                  {current.type !== "slicer" && current.type !== "metric_group" && (
                    <FieldLabel label={current.type === "scatter" ? "Métrica X" : current.type === "funnel" || current.type === "treemap" || current.type === "waterfall" ? "Medida" : "Métrica"}>
                      <Select
                        value={current.query?.measures?.[0] || ""}
                        onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, query: { ...w.query, measures: e.target.value ? [e.target.value, ...(w.query?.measures || []).slice(1)] : (w.query?.measures || []).slice(1) }, config: current.type === "scatter" ? { ...w.config, xMeasure: e.target.value } : w.config } : w)))}
                      >
                        <option value="">—</option>
                        {(model?.measures || []).map((m: any) => <option key={m.name} value={m.name}>{m.name}</option>)}
                      </Select>
                    </FieldLabel>
                  )}
                  {current.type === "scatter" && (
                    <FieldLabel label="Métrica Y">
                      <Select
                        value={current.query?.measures?.[1] || ""}
                        onChange={(e) => {
                          const first = current.query?.measures?.[0] || "";
                          updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, query: { ...w.query, measures: [first, e.target.value].filter(Boolean) }, config: { ...w.config, xMeasure: first, yMeasure: e.target.value } } : w)));
                        }}
                      >
                        <option value="">—</option>
                        {(model?.measures || []).map((m: any) => <option key={m.name} value={m.name}>{m.name}</option>)}
                      </Select>
                    </FieldLabel>
                  )}
                  <FieldLabel label={current.type === "slicer" ? "Dimensão do slicer" : current.type === "heatmap" ? "Dimensão X" : current.type === "sparkline" ? "Dimensão temporal" : "Dimensão"}>
                    <Select
                      value={current.query?.dimensions?.[0] || ""}
                      onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, query: { ...w.query, dimensions: e.target.value ? [e.target.value, ...(w.query?.dimensions || []).slice(1)] : (w.query?.dimensions || []).slice(1) } } : w)))}
                    >
                      <option value="">Nenhuma</option>
                      {(model?.dimensions || []).map((d: any) => <option key={d.column || d.name} value={d.column || d.name}>{d.name || d.column}</option>)}
                    </Select>
                  </FieldLabel>
                  {current.type === "heatmap" && (
                    <FieldLabel label="Dimensão Y">
                      <Select
                        value={current.query?.dimensions?.[1] || ""}
                        onChange={(e) => {
                          const first = current.query?.dimensions?.[0] || "";
                          updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, query: { ...w.query, dimensions: [first, e.target.value].filter(Boolean) } } : w)));
                        }}
                      >
                        <option value="">Nenhuma</option>
                        {(model?.dimensions || []).map((d: any) => <option key={d.column || d.name} value={d.column || d.name}>{d.name || d.column}</option>)}
                      </Select>
                    </FieldLabel>
                  )}
                  {!["kpi", "kpi_goal", "metric_group", "gauge", "sparkline", "slicer"].includes(current.type) && (
                    <FieldLabel label={current.type === "decomposition_tree" ? "Hierarquia (níveis separados por vírgula)" : "Hierarquia de drill-down (separada por vírgula)"} hint="Ex.: regiao, cidade, loja">
                      <Input
                        value={(current.hierarchy || []).join(",")}
                        onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, hierarchy: e.target.value.split(",").map((s) => s.trim()).filter(Boolean) } : w)))}
                        placeholder="regiao, cidade, loja"
                      />
                    </FieldLabel>
                  )}
                  {current.drillPath && current.drillPath.length > 0 && (
                    <div className="flex items-center gap-2 text-xs text-mute">
                      <span>Drill:</span>
                      {current.hierarchy?.slice(0, current.drillPath.length).map((h, i) => (
                        <span key={h} className="rounded bg-surface-2 px-1.5 py-0.5">{h}={current.drillPath?.[i]}</span>
                      ))}
                      <button className="text-accent" onClick={() => drill(current.id, "up")}>Subir</button>
                    </div>
                  )}
                </>
              )}
            </TabsContent>
            <TabsContent value="format" activeValue={activeTab} className="space-y-3 pt-3">
              <FieldLabel label="Tipo de visualização">
                <Select
                  value={current.type}
                  onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, type: e.target.value as WidgetType } : w)))}
                >
                  {WIDGET_CATALOG.filter((t) => !["text", "image", "markdown", "iframe"].includes(t.type)).map((t) => <option key={t.type} value={t.type}>{t.label}</option>)}
                </Select>
              </FieldLabel>
              {current.type !== "slicer" && (
                <>
                  <FieldLabel label="Cor principal">
                    <div className="flex flex-wrap gap-2">
                      {["#2563EB", "#6366F1", "#0EA5E9", "#F59E0B", "#10B981", "#8B5CF6", "#EF4444"].map((c) => (
                        <button
                          key={c}
                          className={`h-7 w-7 rounded-full border-2 ${current.config?.color === c ? "border-ink" : "border-transparent"}`}
                          style={{ backgroundColor: c }}
                          onClick={() => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, color: c } } : w)))}
                        />
                      ))}
                    </div>
                  </FieldLabel>
                </>
              )}
              {["kpi", "kpi_goal", "metric_group", "gauge", "waterfall", "funnel", "scatter", "treemap", "heatmap", "sparkline", "table"].includes(current.type) && (
                <>
                  <FieldLabel label="Prefixo">
                    <Input value={current.config?.prefix || ""} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, prefix: e.target.value } } : w)))} placeholder="R$ " />
                  </FieldLabel>
                  <FieldLabel label="Sufixo">
                    <Input value={current.config?.suffix || ""} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, suffix: e.target.value } } : w)))} placeholder="%" />
                  </FieldLabel>
                  <FieldLabel label="Casas decimais">
                    <Input type="number" min={0} max={6} value={current.config?.decimals ?? 0} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, decimals: Number(e.target.value) } } : w)))} />
                  </FieldLabel>
                </>
              )}
              {current.type === "gauge" && (
                <>
                  <FieldLabel label="Etiqueta do medidor">
                    <Input value={current.config?.gaugeLabel || ""} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, gaugeLabel: e.target.value } } : w)))} placeholder="Valor" />
                  </FieldLabel>
                  <FieldLabel label="Mínimo">
                    <Input type="number" value={current.config?.min ?? 0} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, min: Number(e.target.value) } } : w)))} />
                  </FieldLabel>
                  <FieldLabel label="Máximo">
                    <Input type="number" value={current.config?.max ?? 100} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, max: Number(e.target.value) } } : w)))} />
                  </FieldLabel>
                  <FieldLabel label="Meta">
                    <Input type="number" value={current.config?.target ?? 80} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, target: Number(e.target.value) } } : w)))} />
                  </FieldLabel>
                </>
              )}
              {current.type === "waterfall" && (
                <FieldLabel label="Categorias negativas (separadas por vírgula)" hint="Valores destas categorias serão subtraídos. Deixe vazio para usar o sinal do valor.">
                  <Input value={current.config?.waterfallNegativeCategories || ""} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, waterfallNegativeCategories: e.target.value } } : w)))} placeholder="Despesas, Custos, Impostos" />
                </FieldLabel>
              )}
              {current.type === "scatter" && (
                <>
                  <FieldLabel label="Eixo X (medida ou dimensão)">
                    <Input value={current.config?.xMeasure || ""} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, xMeasure: e.target.value } } : w)))} placeholder="revenue" />
                  </FieldLabel>
                  <FieldLabel label="Eixo Y (medida ou dimensão)">
                    <Input value={current.config?.yMeasure || ""} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, yMeasure: e.target.value } } : w)))} placeholder="orders" />
                  </FieldLabel>
                </>
              )}
              {current.type === "kpi_goal" && (
                <FieldLabel label="Meta">
                  <Input type="number" value={current.config?.goal ?? 100} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, goal: Number(e.target.value) } } : w)))} />
                </FieldLabel>
              )}
              {current.type === "metric_group" && (
                <FieldLabel label="Métricas do grupo">
                  <div className="grid grid-cols-1 gap-1.5">
                    {(model?.measures || []).map((m: any) => {
                      const checked = current.query?.measures?.includes(m.name) || false;
                      return (
                        <label key={m.name} className="flex items-center gap-2 text-[12px] text-ink">
                          <input
                            type="checkbox"
                            checked={checked}
                            onChange={() => {
                              const ms = current.query?.measures || [];
                              const next = checked ? ms.filter((x) => x !== m.name) : [...ms, m.name];
                              updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, query: { ...w.query, measures: next } } : w)));
                            }}
                          />
                          {m.name}
                        </label>
                      );
                    })}
                  </div>
                </FieldLabel>
              )}
              <FieldLabel label="Largura (cols)">
                <Input type="number" min={2} max={12} value={current.layout.w} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, layout: { ...w.layout, w: Number(e.target.value) } } : w)))} />
              </FieldLabel>
              <FieldLabel label="Altura (rows)">
                <Input type="number" min={2} max={20} value={current.layout.h} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, layout: { ...w.layout, h: Number(e.target.value) } } : w)))} />
              </FieldLabel>
            </TabsContent>
            <TabsContent value="filters" activeValue={activeTab} className="space-y-3 pt-3">
              <p className="text-xs text-mute">Filtros aplicados a este widget. Os slicers e cliques no canvas aplicam filtros globais automaticamente.</p>
              {(current.query?.filters || []).map((f, i) => (
                <div key={i} className="flex items-center gap-2 rounded-lg border border-line p-2">
                  <Input value={f.dimension} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, query: { ...w.query, filters: (w.query?.filters || []).map((x, j) => (j === i ? { ...x, dimension: e.target.value } : x)) } } : w)))} className="w-24" />
                  <Select value={f.op} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, query: { ...w.query, filters: (w.query?.filters || []).map((x, j) => (j === i ? { ...x, op: e.target.value as any } : x)) } } : w)))}>
                    <option value="eq">=</option>
                    <option value="neq">≠</option>
                    <option value="in">em</option>
                  </Select>
                  <Input value={String(f.value)} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, query: { ...w.query, filters: (w.query?.filters || []).map((x, j) => (j === i ? { ...x, value: e.target.value } : x)) } } : w)))} />
                  <Button variant="ghost" size="icon" onClick={() => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, query: { ...w.query, filters: (w.query?.filters || []).filter((_, j) => j !== i) } } : w)))}>
                    <Trash2 size={14} />
                  </Button>
                </div>
              ))}
              <Button
                variant="secondary"
                size="sm"
                onClick={() => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, query: { ...w.query, filters: [...(w.query?.filters || []), { dimension: "", op: "eq", value: "" }] } } : w)))}
              >
                <Plus size={14} /> Adicionar filtro
              </Button>
            </TabsContent>
          </Tabs>
        </aside>
      )}

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

// Tabs componentes locais simples
function Tabs({ value, onValueChange, children }: { value: string; onValueChange: (v: string) => void; children: React.ReactNode }) {
  return <div className="w-full" data-value={value} data-onchange={String(onValueChange)}>{children}</div>;
}
function TabsList({ children, className }: { children: React.ReactNode; className?: string }) {
  return <div className={cn("flex gap-1 rounded-xl bg-surface-2 p-1", className)}>{children}</div>;
}
function TabsTrigger({ value, active, children, onClick }: { value: string; active?: boolean; children: React.ReactNode; onClick?: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex-1 rounded-lg px-3 py-1.5 text-[12px] font-medium transition",
        active ? "bg-white text-ink shadow-sm" : "text-mute hover:text-ink hover:bg-white/50",
      )}
    >
      {children}
    </button>
  );
}
function TabsContent({ value, activeValue, children, className }: { value: string; activeValue: string; children: React.ReactNode; className?: string }) {
  if (value !== activeValue) return null;
  return <div className={cn("animate-in fade-in duration-200", className)}>{children}</div>;
}

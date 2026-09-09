"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, getAccess, normalizeArray } from "@/lib/api";
import { WidgetView, type Widget } from "@/components/WidgetView";
import { modelIdForDataset, relationshipsToJoins } from "@/lib/semantic";
import { DEFAULT_QUERY_LIMIT } from "@/lib/widget-config";
import { toast } from "sonner";
import GridLayout, { WidthProvider, type Layout } from "react-grid-layout";
import "react-grid-layout/css/styles.css";
import "react-resizable/css/styles.css";
import { Button, Card, CardTitle, EmptyState, ErrorState, FieldLabel, Input, PageHeader, PageSkeleton, Select, Textarea, Badge, cn } from "@/components/ui";
import { AutoRefreshCard } from "@/components/auto-refresh-card";
import { LineChart, BarChart3, PieChart, Table2, Type, Image as ImageIcon, Plus, Trash2, Eye, EyeOff, Save, FileDown, Calendar, Share2, X, ChevronLeft, Monitor, Printer, MoreHorizontal } from "lucide-react";
import { useMediaQuery } from "@/lib/use-media-query";

const Grid = WidthProvider(GridLayout);

type ReportPage = { name: string; widgets: Widget[] };

const WIDGET_CATALOG: { type: Widget["type"]; label: string; icon: any; defaultW: number; defaultH: number }[] = [
  { type: "kpi", label: "KPI", icon: Monitor, defaultW: 3, defaultH: 2 },
  { type: "line", label: "Linha", icon: LineChart, defaultW: 6, defaultH: 4 },
  { type: "bar", label: "Barras", icon: BarChart3, defaultW: 6, defaultH: 4 },
  { type: "area", label: "Área", icon: LineChart, defaultW: 6, defaultH: 4 },
  { type: "pie", label: "Pizza", icon: PieChart, defaultW: 4, defaultH: 4 },
  { type: "table", label: "Tabela", icon: Table2, defaultW: 6, defaultH: 4 },
  { type: "big_table", label: "Tabela grande", icon: Table2, defaultW: 12, defaultH: 6 },
  { type: "text", label: "Texto", icon: Type, defaultW: 4, defaultH: 2 },
  { type: "image", label: "Imagem", icon: ImageIcon, defaultW: 4, defaultH: 3 },
  { type: "markdown", label: "Markdown", icon: Type, defaultW: 4, defaultH: 3 },
];

export default function ReportEditorPage() {
  const { id } = useParams<{ id: string }>();
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [cadence, setCadence] = useState("weekly");
  const [pages, setPages] = useState<ReportPage[]>([{ name: "Página 1", widgets: [] }]);
  const [activePage, setActivePage] = useState(0);
  const [selected, setSelected] = useState<string | null>(null);
  const [edit, setEdit] = useState(true);
  const [activeTab, setActiveTab] = useState("data");
  const [hydrated, setHydrated] = useState(false);
  const [scheduleOpen, setScheduleOpen] = useState(false);
  const [moreOpen, setMoreOpen] = useState(false);
  const [emailTo, setEmailTo] = useState("");
  const [whatsappTo, setWhatsappTo] = useState("");
  const canvasRef = useRef<HTMLDivElement>(null);
  const isNarrow = useMediaQuery("(max-width: 767px)");

  const q = useQuery({
    queryKey: ["report", id],
    queryFn: () => api<{ id: string; name: string; cadence: string; pages: ReportPage[]; last_generated_at?: string; email_to?: string; whatsapp_to?: string }>(`/api/v1/reports/${id}`),
  });

  const datasets = useQuery({ queryKey: ["datasets"], queryFn: () => api<any>("/api/v1/datasets") });
  const datasetList = normalizeArray<{ id: string; name: string }>(datasets.data);
  const semantic = useQuery({ queryKey: ["semantic"], queryFn: () => api<any>("/api/v1/semantic-models") });
  const semanticModels = useMemo((): any[] => {
    const d: any = semantic.data;
    if (Array.isArray(d)) return d;
    if (Array.isArray(d?.data)) return d.data;
    return [];
  }, [semantic.data]);

  useEffect(() => {
    if (q.data && !hydrated) {
      setName(q.data.name || "");
      setCadence(q.data.cadence || "weekly");
      setEmailTo(q.data.email_to || "");
      setWhatsappTo(q.data.whatsapp_to || "");
      setPages(q.data.pages && q.data.pages.length > 0 ? q.data.pages : [{ name: "Página 1", widgets: [] }]);
      setHydrated(true);
    }
  }, [q.data, hydrated]);

  useEffect(() => {
    if (!hydrated || !semanticModels.length) return;
    let cancelled = false;
    (async () => {
      const cache = new Map<string, ReturnType<typeof relationshipsToJoins>>();
      let changed = false;
      const nextPages = [];
      for (const page of pages) {
        const widgets = [];
        for (const w of page.widgets) {
          if (!w.query?.dataset_id || (w.query.joins && w.query.joins.length > 0)) {
            widgets.push(w);
            continue;
          }
          const modelId = modelIdForDataset(semanticModels, w.query.dataset_id);
          if (!modelId) {
            widgets.push(w);
            continue;
          }
          if (!cache.has(modelId)) {
            try {
              cache.set(modelId, relationshipsToJoins(await api(`/api/v1/semantic-models/${modelId}/relationships`)));
            } catch {
              cache.set(modelId, []);
            }
          }
          const joins = cache.get(modelId) || [];
          if (!joins.length) {
            widgets.push(w);
            continue;
          }
          changed = true;
          widgets.push({ ...w, query: { ...w.query, joins } });
        }
        nextPages.push({ ...page, widgets });
      }
      if (!cancelled && changed) setPages(nextPages);
    })();
    return () => {
      cancelled = true;
    };
  }, [hydrated, semanticModels, pages]);

  const currentPage = pages[activePage] || { name: "", widgets: [] };
  const widgets = currentPage.widgets;
  const current = widgets.find((w) => w.id === selected);
  const currentDataset = current?.query?.dataset_id;
  const model = useMemo(() => {
    if (!currentDataset) return null;
    return semanticModels.find((m: any) => m.dataset_id === currentDataset)?.model || null;
  }, [currentDataset, semanticModels]);

  const save = useMutation({
    mutationFn: () => api(`/api/v1/reports/${id}`, { method: "PUT", body: JSON.stringify({ name, cadence, pages, email_to: emailTo, whatsapp_to: whatsappTo }) }),
    onSuccess: () => {
      toast.success("Relatório guardado");
      qc.invalidateQueries({ queryKey: ["report", id] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const generateBackend = useMutation({
    mutationFn: () => api(`/api/v1/reports/${id}/generate`, { method: "POST" }),
    onSuccess: () => {
      toast.success("Geração agendada");
      qc.invalidateQueries({ queryKey: ["report", id] });
      qc.invalidateQueries({ queryKey: ["reports"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const exportPdf = useCallback(async () => {
    if (!canvasRef.current) {
      window.print();
      return;
    }
    try {
      const { jsPDF } = await import("jspdf");
      const { default: html2canvas } = await import("html2canvas");
      const canvas = await html2canvas(canvasRef.current, { scale: 2, useCORS: true, backgroundColor: "#ffffff" });
      const imgData = canvas.toDataURL("image/png");
      const pdf = new jsPDF({ orientation: "landscape", unit: "px", format: [canvas.width, canvas.height] });
      pdf.addImage(imgData, "PNG", 0, 0, canvas.width, canvas.height);
      pdf.save(`${name || "relatorio"}.pdf`);
      toast.success("PDF da página actual exportado");
    } catch (e: any) {
      toast.error("Falha ao exportar PDF, a abrir impressão: " + (e?.message || ""));
      window.print();
    }
  }, [name]);

  const downloadServerPdf = useCallback(async () => {
    try {
      const token = getAccess();
      const ws = localStorage.getItem("thedobra.workspace") || "";
      const res = await fetch(`/api/v1/reports/${id}/pdf`, {
        headers: {
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
          ...(ws ? { "X-Workspace-Id": ws } : {}),
        },
      });
      if (!res.ok) throw new Error("Falha ao gerar o PDF das páginas");
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${name || "relatorio"}.pdf`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (e: any) {
      toast.error(e.message || "Falha ao descarregar o PDF do servidor");
    }
  }, [id, name]);

  const updatePages = useCallback((fn: (prev: ReportPage[]) => ReportPage[]) => {
    setPages((prev) => fn(prev));
  }, []);

  const updateWidgets = useCallback((fn: (prev: Widget[]) => Widget[]) => {
    setPages((prev) => {
      const next = [...prev];
      next[activePage] = { ...next[activePage], widgets: fn(next[activePage].widgets) };
      return next;
    });
  }, [activePage]);

  const addPage = () => {
    updatePages((prev) => [...prev, { name: `Página ${prev.length + 1}`, widgets: [] }]);
    setActivePage(pages.length);
    setSelected(null);
  };

  const removePage = (idx: number) => {
    if (pages.length <= 1) return;
    updatePages((prev) => prev.filter((_, i) => i !== idx));
    if (activePage >= idx && activePage > 0) setActivePage(activePage - 1);
    setSelected(null);
  };

  const renamePage = (idx: number, name: string) => {
    updatePages((prev) => prev.map((p, i) => (i === idx ? { ...p, name } : p)));
  };

  const addWidget = (type: Widget["type"]) => {
    const catalog = WIDGET_CATALOG.find((t) => t.type === type)!;
    const ds = datasetList[0]?.id;
    const w: Widget = {
      id: crypto.randomUUID(),
      type,
      title: catalog.label,
      layout: { x: (widgets.length * 4) % 12, y: 100, w: catalog.defaultW, h: catalog.defaultH },
      query: ds && !["text", "image", "markdown"].includes(type) ? { dataset_id: ds, measures: ["revenue"], dimensions: type === "kpi" ? [] : ["region"], limit: type === "big_table" ? 10000 : DEFAULT_QUERY_LIMIT } : undefined,
      text: type === "text" ? "Novo texto" : undefined,
      config: type === "image" ? { imageUrl: "" } : type === "markdown" ? { markdown: "## Nota\nEdite aqui." } : type === "big_table" ? { pageSize: 50, zebra: true, freezeHeader: true } : undefined,
    };
    updateWidgets((prev) => [...prev, w]);
    setSelected(w.id);
  };

  const removeWidget = (wid: string) => updateWidgets((prev) => prev.filter((w) => w.id !== wid));

  const layout = useMemo<Layout[]>(() => {
    if (isNarrow) {
      let y = 0;
      return widgets.map((w) => {
        const h = Math.max(3, Math.min(w.layout.h || 4, 6));
        const item = { i: w.id, x: 0, y, w: 4, h, minW: 2, minH: 2 };
        y += h;
        return item;
      });
    }
    return widgets.map((w) => ({ i: w.id, x: w.layout.x, y: w.layout.y, w: w.layout.w, h: w.layout.h, minW: 2, minH: 2 }));
  }, [widgets, isNarrow]);

  const onLayoutChange = useCallback((next: Layout[]) => {
    if (isNarrow) return;
    updateWidgets((prev) =>
      prev.map((w) => {
        const l = next.find((x) => x.i === w.id);
        return l ? { ...w, layout: { x: l.x, y: l.y, w: l.w, h: l.h } } : w;
      })
    );
  }, [updateWidgets, isNarrow]);

  if (q.isError) return <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />;
  if (!q.data && !hydrated) return <PageSkeleton />;

  return (
    <div className="flex h-[calc(100dvh-7rem)] min-h-0 flex-col gap-3 md:flex-row">
      {edit && (
        <aside className="hidden w-56 min-h-0 flex-col gap-3 overflow-y-auto rounded-2xl border border-line bg-surface p-3 shadow-sm print:hidden md:flex">
          <div className="flex items-center justify-between">
            <span className="text-[12px] font-semibold uppercase text-mute">Componentes</span>
            <Button variant="ghost" size="icon" onClick={() => setEdit(false)} title="Fechar painel"><EyeOff size={16} /></Button>
          </div>
          <div className="grid grid-cols-2 gap-2">
            {WIDGET_CATALOG.map((t) => {
              const Icon = t.icon;
              return (
                <button key={t.type} onClick={() => addWidget(t.type)} className="flex flex-col items-center gap-1 rounded-xl border border-line bg-white p-3 text-center transition hover:border-primary hover:shadow-sm">
                  <Icon size={18} className="text-primary" />
                  <span className="text-[11px] font-medium text-ink">{t.label}</span>
                </button>
              );
            })}
          </div>
        </aside>
      )}

      <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3 overflow-hidden">
        <div className="print:hidden">
          <PageHeader
            title={name || "Relatório"}
            description={edit ? "Editor multi-página" : undefined}
            crumbs={[{ href: "/reports", label: "Relatórios" }]}
            actions={
              <div className="flex flex-wrap items-center gap-2">
                {edit ? (
                  <Button variant="secondary" onClick={() => setEdit(false)}><Eye size={14} /> <span className="hidden sm:inline">Pré-visualizar</span></Button>
                ) : (
                  <Button variant="secondary" onClick={() => setEdit(true)}><EyeOff size={14} /> <span className="hidden sm:inline">Editar</span></Button>
                )}
                <Button onClick={() => save.mutate()} busy={save.isPending}><Save size={14} /> <span className="hidden sm:inline">Guardar</span></Button>
                <div className="relative">
                  <Button variant="secondary" size="icon" className="md:hidden" onClick={() => setMoreOpen((v) => !v)} title="Mais acções" aria-expanded={moreOpen}>
                    <MoreHorizontal size={16} />
                  </Button>
                  {moreOpen && (
                    <div className="absolute right-0 z-30 mt-1 w-52 overflow-hidden rounded-xl border border-line bg-surface py-1 shadow-lg md:hidden">
                      <button className="flex min-h-10 w-full items-center gap-2 px-3 text-left text-[13px] text-ink hover:bg-bg" onClick={() => { setMoreOpen(false); exportPdf(); }}><FileDown size={14} /> PDF da página</button>
                      <button className="flex min-h-10 w-full items-center gap-2 px-3 text-left text-[13px] text-ink hover:bg-bg" onClick={() => { setMoreOpen(false); downloadServerPdf(); }}><FileDown size={14} /> PDF servidor</button>
                      <button className="flex min-h-10 w-full items-center gap-2 px-3 text-left text-[13px] text-ink hover:bg-bg" onClick={() => { setMoreOpen(false); window.print(); }}><Printer size={14} /> Imprimir</button>
                      <button className="flex min-h-10 w-full items-center gap-2 px-3 text-left text-[13px] text-ink hover:bg-bg" onClick={() => { setMoreOpen(false); setScheduleOpen(true); }}><Calendar size={14} /> Agendar</button>
                      <button className="flex min-h-10 w-full items-center gap-2 px-3 text-left text-[13px] text-ink hover:bg-bg" onClick={() => { setMoreOpen(false); navigator.clipboard?.writeText(window.location.href); toast.success("Link copiado"); }}><Share2 size={14} /> Partilhar</button>
                    </div>
                  )}
                </div>
                <div className="hidden flex-wrap items-center gap-2 md:flex">
                  <Button variant="secondary" onClick={exportPdf}><FileDown size={14} /> PDF da página</Button>
                  <Button variant="secondary" onClick={downloadServerPdf}><FileDown size={14} /> PDF servidor</Button>
                  <Button variant="secondary" onClick={() => window.print()}><Printer size={14} /> Imprimir</Button>
                  <Button variant="secondary" onClick={() => setScheduleOpen(true)}><Calendar size={14} /> Agendar</Button>
                  <Button variant="secondary" onClick={() => { navigator.clipboard?.writeText(window.location.href); toast.success("Link copiado"); }}><Share2 size={14} /> Partilhar</Button>
                </div>
              </div>
            }
          />
        </div>

        {edit && (
          <div className="flex shrink-0 items-center gap-1.5 overflow-x-auto rounded-2xl border border-line bg-surface px-3 py-2 print:hidden md:hidden">
            <span className="shrink-0 text-[10px] font-semibold uppercase tracking-wide text-mute">Adicionar</span>
            {WIDGET_CATALOG.map((t) => {
              const Icon = t.icon;
              return (
                <button
                  key={t.type}
                  type="button"
                  onClick={() => addWidget(t.type)}
                  className="flex h-10 shrink-0 items-center gap-1.5 rounded-lg border border-line bg-surface-2 px-2.5 text-[11px] text-ink transition hover:border-primary hover:bg-primary/5"
                >
                  <Icon size={13} className="text-primary" />
                  {t.label}
                </button>
              );
            })}
          </div>
        )}

        <Card className="space-y-3 print:hidden">
          <div className="grid grid-cols-1 gap-3 sm:flex sm:flex-wrap sm:items-end">
            <FieldLabel label="Nome"><Input value={name} onChange={(e) => setName(e.target.value)} className="sm:max-w-xs" /></FieldLabel>
            <FieldLabel label="Cadência">
              <Select value={cadence} onChange={(e) => setCadence(e.target.value)} className="sm:w-40">
                <option value="daily">Diário</option>
                <option value="weekly">Semanal</option>
                <option value="monthly">Mensal</option>
              </Select>
            </FieldLabel>
          </div>
          <div className="-mx-1 flex items-center gap-2 overflow-x-auto border-t border-line px-1 pt-3">
            {pages.map((p, i) => (
              <div
                key={i}
                onClick={() => { setActivePage(i); setSelected(null); }}
                className={cn("flex min-h-10 shrink-0 cursor-pointer items-center gap-1 rounded-lg border px-3 py-1.5 text-[12px]", activePage === i ? "border-primary bg-primary/10 text-primary-600" : "border-line bg-white text-ink")}
              >
                {edit ? (
                  <input
                    value={p.name}
                    onChange={(e) => renamePage(i, e.target.value)}
                    onClick={(e) => e.stopPropagation()}
                    className="w-24 bg-transparent outline-none"
                  />
                ) : (
                  <span className={activePage === i ? "font-medium" : ""}>{p.name}</span>
                )}
                {edit && pages.length > 1 && (
                  <button className="flex h-8 w-8 items-center justify-center text-mute hover:text-danger" onClick={(e) => { e.stopPropagation(); removePage(i); }}><X size={12} /></button>
                )}
              </div>
            ))}
            {edit && <Button variant="ghost" size="sm" className="shrink-0" onClick={addPage}><Plus size={14} /> Página</Button>}
          </div>
        </Card>

        <div ref={canvasRef} className="relative min-h-0 flex-1 overflow-auto rounded-2xl border border-line bg-surface p-3 pb-8 shadow-sm sm:p-4 print:overflow-visible print:shadow-none">
          <h2 className="mb-3 hidden text-lg font-semibold text-ink print:block">{name} — {currentPage.name}</h2>
          {widgets.length === 0 ? (
            <EmptyState
              icon={FileDown}
              title={edit ? "Página vazia" : "Sem conteúdo"}
              description={edit ? "Adicione widgets à página." : "Esta página ainda não tem conteúdo."}
              action={edit ? <Button onClick={() => addWidget("kpi")} className="print:hidden"><Plus size={14} /> Adicionar KPI</Button> : undefined}
            />
          ) : (
            <Grid
              key={isNarrow ? "mobile" : "desktop"}
              className="layout min-h-full"
              layout={layout}
              cols={isNarrow ? 4 : 12}
              rowHeight={isNarrow ? 72 : 96}
              margin={isNarrow ? [10, 10] : [14, 14]}
              containerPadding={isNarrow ? [8, 8] : [12, 12]}
              isDraggable={edit && !isNarrow}
              isResizable={edit && !isNarrow}
              onLayoutChange={onLayoutChange}
              draggableHandle=".drag-handle"
              compactType="vertical"
            >
              {widgets.map((w) => (
                <div key={w.id} className={`widget-grid-item relative ${selected === w.id && edit ? "ring-2 ring-primary/30" : ""}`} onClick={() => edit && setSelected(w.id)}>
                  {edit && (
                    <div className="absolute right-2 top-2 z-10 flex items-center gap-1 print:hidden">
                      {!isNarrow && <div className="drag-handle flex h-8 cursor-move items-center gap-1 rounded-lg bg-white/95 px-2 text-[10px] text-mute shadow-sm"><ChevronLeft size={12} /> Mover</div>}
                      <button className="flex h-10 w-10 items-center justify-center rounded-lg bg-white/95 text-mute shadow-sm hover:text-danger sm:h-8 sm:w-8" onClick={() => removeWidget(w.id)}><Trash2 size={12} /></button>
                    </div>
                  )}
                  <WidgetView
                    w={w}
                    globalFilters={[]}
                    onFilter={() => {}}
                    onDrill={() => {}}
                    siblingWidgets={widgets.filter((x) => x.id !== w.id)}
                  />
                </div>
              ))}
            </Grid>
          )}
        </div>
      </div>

      {edit && current && isNarrow && (
        <div className="fixed inset-0 z-40 print:hidden">
          <div className="absolute inset-0 bg-ink/30" onClick={() => setSelected(null)} />
        </div>
      )}
      {edit && current && (
        <aside
          className={cn(
            "relative min-h-0 overflow-y-auto border-line bg-surface p-4 print:hidden",
            isNarrow
              ? "fixed inset-x-0 bottom-0 z-50 max-h-[min(70dvh,36rem)] w-full rounded-t-2xl border-t pb-[max(1rem,env(safe-area-inset-bottom))] shadow-[0_-16px_40px_rgba(15,23,42,0.16)]"
              : "w-80 shrink-0 rounded-2xl border shadow-sm",
          )}
        >
          {isNarrow && <span className="absolute left-1/2 top-1.5 h-1 w-10 -translate-x-1/2 rounded-full bg-line" aria-hidden />}
          <div className={cn("mb-3 flex items-center justify-between", isNarrow && "pt-2")}>
            <span className="text-[13px] font-semibold text-ink">Propriedades</span>
            <div className="flex items-center gap-2">
              <Badge tone="accent">{WIDGET_CATALOG.find((t) => t.type === current.type)?.label}</Badge>
              {isNarrow && (
                <button type="button" onClick={() => setSelected(null)} className="flex h-10 w-10 items-center justify-center rounded-lg text-mute hover:bg-surface-2 hover:text-ink" aria-label="Fechar painel">
                  <X size={16} />
                </button>
              )}
            </div>
          </div>
          <Tabs value={activeTab} onValueChange={setActiveTab}>
            <TabsList className="grid w-full grid-cols-2">
              <TabsTrigger value="data" active={activeTab === "data"} onClick={() => setActiveTab("data")}>Dados</TabsTrigger>
              <TabsTrigger value="format" active={activeTab === "format"} onClick={() => setActiveTab("format")}>Formato</TabsTrigger>
            </TabsList>
            <TabsContent value="data" activeValue={activeTab} className="space-y-3 pt-3">
              <FieldLabel label="Título"><Input value={current.title} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, title: e.target.value } : w)))} /></FieldLabel>
              {current.type === "text" && (
                <FieldLabel label="Texto"><Textarea value={current.text} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, text: e.target.value } : w)))} /></FieldLabel>
              )}
              {(current.type === "image" || current.type === "markdown") && (
                <>
                  {current.type === "image" && <FieldLabel label="URL da imagem"><Input value={current.config?.imageUrl || ""} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, imageUrl: e.target.value } } : w)))} /></FieldLabel>}
                  {current.type === "markdown" && <FieldLabel label="Markdown"><Textarea value={current.config?.markdown || ""} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, markdown: e.target.value } } : w)))} /></FieldLabel>}
                </>
              )}
              {!["text", "image", "markdown"].includes(current.type) && (
                <>
                  <FieldLabel label="Conjunto de dados">
                    <Select value={current.query?.dataset_id || ""} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, query: { ...w.query, dataset_id: e.target.value } } : w)))}>
                      <option value="">—</option>
                      {datasetList.map((ds) => <option key={ds.id} value={ds.id}>{ds.name}</option>)}
                    </Select>
                  </FieldLabel>
                  <FieldLabel label="Métrica">
                    <Select value={current.query?.measures?.[0] || ""} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, query: { ...w.query, measures: e.target.value ? [e.target.value] : [] } } : w)))}>
                      <option value="">—</option>
                      {(model?.measures || []).map((m: any) => <option key={m.name} value={m.name}>{m.name}</option>)}
                    </Select>
                  </FieldLabel>
                  <FieldLabel label="Dimensão">
                    <Select value={current.query?.dimensions?.[0] || ""} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, query: { ...w.query, dimensions: e.target.value ? [e.target.value] : [] } } : w)))}>
                      <option value="">Nenhuma</option>
                      {(model?.dimensions || []).map((d: any) => <option key={d.column || d.name} value={d.column || d.name}>{d.name || d.column}</option>)}
                    </Select>
                  </FieldLabel>
                </>
              )}
            </TabsContent>
            <TabsContent value="format" activeValue={activeTab} className="space-y-3 pt-3">
              {!isNarrow && (
                <>
                  <FieldLabel label="Largura (cols)"><Input type="number" min={2} max={12} value={current.layout.w} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, layout: { ...w.layout, w: Number(e.target.value) } } : w)))} /></FieldLabel>
                  <FieldLabel label="Altura (rows)"><Input type="number" min={2} max={20} value={current.layout.h} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, layout: { ...w.layout, h: Number(e.target.value) } } : w)))} /></FieldLabel>
                </>
              )}
              {current.type !== "text" && current.type !== "image" && current.type !== "markdown" && (
                <FieldLabel label="Cor principal">
                  <div className="flex flex-wrap gap-2">
                    {["#2563EB", "#6366F1", "#0EA5E9", "#F59E0B", "#10B981", "#8B5CF6", "#EF4444"].map((c) => (
                      <button key={c} className={`h-9 w-9 rounded-full border-2 sm:h-7 sm:w-7 ${current.config?.color === c ? "border-ink" : "border-transparent"}`} style={{ backgroundColor: c }} onClick={() => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, color: c } } : w)))} />
                    ))}
                  </div>
                </FieldLabel>
              )}
            </TabsContent>
          </Tabs>
        </aside>
      )}

      {scheduleOpen && (
        <div className="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4">
          <Card className="max-h-[92dvh] w-full max-w-lg space-y-4 overflow-y-auto rounded-b-none pb-[max(1.25rem,env(safe-area-inset-bottom))] sm:rounded-2xl sm:pb-5">
            <CardTitle>Distribuir relatório</CardTitle>
            <p className="text-[13px] text-mute">Gera o briefing no servidor e envia para os destinatários. O cron usa a actualização automática abaixo.</p>
            <FieldLabel label="E-mails (separados por vírgula)">
              <Input value={emailTo} onChange={(e) => setEmailTo(e.target.value)} placeholder="ana@empresa.com, cfo@empresa.com" />
            </FieldLabel>
            <FieldLabel label="WhatsApp (números, separados por vírgula)">
              <Input value={whatsappTo} onChange={(e) => setWhatsappTo(e.target.value)} placeholder="+5511999999999" />
            </FieldLabel>
            <FieldLabel label="Cadência do relatório">
              <Select value={cadence} onChange={(e) => setCadence(e.target.value)}>
                <option value="daily">Diário</option>
                <option value="weekly">Semanal</option>
                <option value="monthly">Mensal</option>
              </Select>
            </FieldLabel>
            <AutoRefreshCard kind="report" targetId={id} />
            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button variant="secondary" onClick={() => setScheduleOpen(false)}>Fechar</Button>
              <Button
                onClick={() => {
                  save.mutate();
                  generateBackend.mutate();
                }}
                busy={generateBackend.isPending || save.isPending}
              >
                Guardar e gerar agora
              </Button>
            </div>
          </Card>
        </div>
      )}
    </div>
  );
}

function Tabs({ value, onValueChange, children }: { value: string; onValueChange: (v: string) => void; children: React.ReactNode }) {
  return <div className="w-full" data-value={value} data-onchange={String(onValueChange)}>{children}</div>;
}
function TabsList({ children, className }: { children: React.ReactNode; className?: string }) {
  return <div className={cn("flex gap-1 rounded-xl bg-surface-2 p-1", className)}>{children}</div>;
}
function TabsTrigger({ value, active, children, onClick }: { value: string; active?: boolean; children: React.ReactNode; onClick?: () => void }) {
  return (
    <button type="button" onClick={onClick} className={cn("flex min-h-10 flex-1 items-center justify-center rounded-lg px-3 py-1.5 text-[12px] font-medium transition sm:min-h-0", active ? "bg-white text-ink shadow-sm" : "text-mute hover:text-ink hover:bg-white/50")}>
      {children}
    </button>
  );
}
function TabsContent({ value, activeValue, children, className }: { value: string; activeValue: string; children: React.ReactNode; className?: string }) {
  if (value !== activeValue) return null;
  return <div className={cn("animate-in fade-in duration-200", className)}>{children}</div>;
}

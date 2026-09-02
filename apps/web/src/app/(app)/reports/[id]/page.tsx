"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { WidgetView, type Widget } from "@/components/WidgetView";
import { toast } from "sonner";
import GridLayout, { WidthProvider, type Layout } from "react-grid-layout";
import "react-grid-layout/css/styles.css";
import "react-resizable/css/styles.css";
import { Button, Card, CardTitle, EmptyState, ErrorState, FieldLabel, Input, PageHeader, PageSkeleton, Select, Textarea, Badge, cn } from "@/components/ui";
import { LineChart, BarChart3, PieChart, Table2, Type, Image as ImageIcon, Plus, Trash2, Eye, EyeOff, Save, FileDown, Calendar, Share2, X, ChevronLeft, Monitor, Printer } from "lucide-react";

const Grid = WidthProvider(GridLayout);

type ReportPage = { name: string; widgets: Widget[] };

const WIDGET_CATALOG: { type: Widget["type"]; label: string; icon: any; defaultW: number; defaultH: number }[] = [
  { type: "kpi", label: "KPI", icon: Monitor, defaultW: 3, defaultH: 2 },
  { type: "line", label: "Linha", icon: LineChart, defaultW: 6, defaultH: 4 },
  { type: "bar", label: "Barras", icon: BarChart3, defaultW: 6, defaultH: 4 },
  { type: "area", label: "Área", icon: LineChart, defaultW: 6, defaultH: 4 },
  { type: "pie", label: "Pizza", icon: PieChart, defaultW: 4, defaultH: 4 },
  { type: "table", label: "Tabela", icon: Table2, defaultW: 6, defaultH: 4 },
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
  const canvasRef = useRef<HTMLDivElement>(null);

  const q = useQuery({
    queryKey: ["report", id],
    queryFn: () => api<{ id: string; name: string; cadence: string; pages: ReportPage[]; last_generated_at?: string }>(`/api/v1/reports/${id}`),
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
      setPages(q.data.pages && q.data.pages.length > 0 ? q.data.pages : [{ name: "Página 1", widgets: [] }]);
      setHydrated(true);
    }
  }, [q.data, hydrated]);

  const currentPage = pages[activePage] || { name: "", widgets: [] };
  const widgets = currentPage.widgets;
  const current = widgets.find((w) => w.id === selected);
  const currentDataset = current?.query?.dataset_id;
  const model = useMemo(() => {
    if (!currentDataset) return null;
    return semanticModels.find((m: any) => m.dataset_id === currentDataset)?.model || null;
  }, [currentDataset, semanticModels]);

  const save = useMutation({
    mutationFn: () => api(`/api/v1/reports/${id}`, { method: "PUT", body: JSON.stringify({ name, cadence, pages }) }),
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
      query: ds && !["text", "image", "markdown"].includes(type) ? { dataset_id: ds, measures: ["revenue"], dimensions: type === "kpi" ? [] : ["region"], limit: 20 } : undefined,
      text: type === "text" ? "Novo texto" : undefined,
      config: type === "image" ? { imageUrl: "" } : type === "markdown" ? { markdown: "## Nota\nEdite aqui." } : undefined,
    };
    updateWidgets((prev) => [...prev, w]);
    setSelected(w.id);
  };

  const removeWidget = (wid: string) => updateWidgets((prev) => prev.filter((w) => w.id !== wid));

  const layout = useMemo<Layout[]>(() => widgets.map((w) => ({ i: w.id, x: w.layout.x, y: w.layout.y, w: w.layout.w, h: w.layout.h, minW: 2, minH: 2 })), [widgets]);

  const onLayoutChange = useCallback((next: Layout[]) => {
    updateWidgets((prev) =>
      prev.map((w) => {
        const l = next.find((x) => x.i === w.id);
        return l ? { ...w, layout: { x: l.x, y: l.y, w: l.w, h: l.h } } : w;
      })
    );
  }, [updateWidgets]);

  if (q.isError) return <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />;
  if (!q.data && !hydrated) return <PageSkeleton />;

  return (
    <div className="flex h-[calc(100vh-7rem)] min-h-0 gap-3">
      {edit && (
        <aside className="flex w-56 min-h-0 flex-col gap-3 overflow-y-auto rounded-2xl border border-line bg-surface p-3 shadow-sm print:hidden">
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
                  <Button variant="secondary" onClick={() => setEdit(false)}><Eye size={14} /> Pré-visualizar</Button>
                ) : (
                  <Button variant="secondary" onClick={() => setEdit(true)}><EyeOff size={14} /> Editar</Button>
                )}
                <Button variant="secondary" onClick={exportPdf}><FileDown size={14} /> PDF</Button>
                <Button variant="secondary" onClick={() => window.print()}><Printer size={14} /> Imprimir</Button>
                <Button variant="secondary" onClick={() => setScheduleOpen(true)}><Calendar size={14} /> Agendar</Button>
                <Button variant="secondary" onClick={() => { navigator.clipboard?.writeText(window.location.href); toast.success("Link copiado"); }}><Share2 size={14} /> Partilhar</Button>
                <Button onClick={() => save.mutate()} busy={save.isPending}><Save size={14} /> Guardar</Button>
              </div>
            }
          />
        </div>

        <Card className="space-y-3 print:hidden">
          <div className="flex flex-wrap items-end gap-3">
            <FieldLabel label="Nome"><Input value={name} onChange={(e) => setName(e.target.value)} className="max-w-xs" /></FieldLabel>
            <FieldLabel label="Cadência">
              <Select value={cadence} onChange={(e) => setCadence(e.target.value)} className="w-40">
                <option value="daily">Diário</option>
                <option value="weekly">Semanal</option>
                <option value="monthly">Mensal</option>
              </Select>
            </FieldLabel>
          </div>
          <div className="flex items-center gap-2 border-t border-line pt-3">
            {pages.map((p, i) => (
              <div
                key={i}
                onClick={() => { setActivePage(i); setSelected(null); }}
                className={cn("flex cursor-pointer items-center gap-1 rounded-lg border px-3 py-1.5 text-[12px]", activePage === i ? "border-primary bg-primary/10 text-primary-600" : "border-line bg-white text-ink")}
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
                  <button className="text-mute hover:text-danger" onClick={(e) => { e.stopPropagation(); removePage(i); }}><X size={12} /></button>
                )}
              </div>
            ))}
            {edit && <Button variant="ghost" size="sm" onClick={addPage}><Plus size={14} /> Página</Button>}
          </div>
        </Card>

        <div ref={canvasRef} className="relative min-h-0 flex-1 overflow-auto rounded-2xl border border-line bg-surface p-4 pb-8 shadow-sm print:overflow-visible print:shadow-none">
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
              className="layout min-h-full"
              layout={layout}
              cols={12}
              rowHeight={70}
              isDraggable={edit}
              isResizable={edit}
              onLayoutChange={onLayoutChange}
              draggableHandle=".drag-handle"
              compactType="vertical"
            >
              {widgets.map((w) => (
                <div key={w.id} className={`relative ${selected === w.id && edit ? "ring-2 ring-primary/30" : ""}`} onClick={() => edit && setSelected(w.id)}>
                  {edit && (
                    <div className="absolute right-2 top-2 z-10 flex items-center gap-1 print:hidden">
                      <div className="drag-handle flex h-8 cursor-move items-center gap-1 rounded-lg bg-white/95 px-2 text-[10px] text-mute shadow-sm"><ChevronLeft size={12} /> Mover</div>
                      <button className="flex h-8 items-center rounded-lg bg-white/95 p-2 text-mute shadow-sm hover:text-danger" onClick={() => removeWidget(w.id)}><Trash2 size={12} /></button>
                    </div>
                  )}
                  <WidgetView w={w} globalFilters={[]} onFilter={() => {}} onDrill={() => {}} />
                </div>
              ))}
            </Grid>
          )}
        </div>
      </div>

      {edit && current && (
        <aside className="w-80 min-h-0 shrink-0 overflow-y-auto rounded-2xl border border-line bg-surface p-4 shadow-sm print:hidden">
          <div className="mb-3 flex items-center justify-between">
            <span className="text-[13px] font-semibold text-ink">Propriedades</span>
            <Badge tone="accent">{WIDGET_CATALOG.find((t) => t.type === current.type)?.label}</Badge>
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
              <FieldLabel label="Largura (cols)"><Input type="number" min={2} max={12} value={current.layout.w} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, layout: { ...w.layout, w: Number(e.target.value) } } : w)))} /></FieldLabel>
              <FieldLabel label="Altura (rows)"><Input type="number" min={2} max={20} value={current.layout.h} onChange={(e) => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, layout: { ...w.layout, h: Number(e.target.value) } } : w)))} /></FieldLabel>
              {current.type !== "text" && current.type !== "image" && current.type !== "markdown" && (
                <FieldLabel label="Cor principal">
                  <div className="flex flex-wrap gap-2">
                    {["#2563EB", "#6366F1", "#0EA5E9", "#F59E0B", "#10B981", "#8B5CF6", "#EF4444"].map((c) => (
                      <button key={c} className={`h-7 w-7 rounded-full border-2 ${current.config?.color === c ? "border-ink" : "border-transparent"}`} style={{ backgroundColor: c }} onClick={() => updateWidgets((p) => p.map((w) => (w.id === current.id ? { ...w, config: { ...w.config, color: c } } : w)))} />
                    ))}
                  </div>
                </FieldLabel>
              )}
            </TabsContent>
          </Tabs>
        </aside>
      )}

      {scheduleOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
          <Card className="w-full max-w-md space-y-4">
            <CardTitle>Agendar geração</CardTitle>
            <p className="text-[13px] text-mute">A cadência define quando o relatório será regenerado. O envio por email é um placeholder.</p>
            <FieldLabel label="Cadência">
              <Select value={cadence} onChange={(e) => setCadence(e.target.value)}>
                <option value="daily">Diário</option>
                <option value="weekly">Semanal</option>
                <option value="monthly">Mensal</option>
              </Select>
            </FieldLabel>
            <div className="flex justify-end gap-2">
              <Button variant="secondary" onClick={() => setScheduleOpen(false)}>Fechar</Button>
              <Button onClick={() => { generateBackend.mutate(); setScheduleOpen(false); }} busy={generateBackend.isPending}>Gerar agora</Button>
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
    <button type="button" onClick={onClick} className={cn("flex-1 rounded-lg px-3 py-1.5 text-[12px] font-medium transition", active ? "bg-white text-ink shadow-sm" : "text-mute hover:text-ink hover:bg-white/50")}>
      {children}
    </button>
  );
}
function TabsContent({ value, activeValue, children, className }: { value: string; activeValue: string; children: React.ReactNode; className?: string }) {
  if (value !== activeValue) return null;
  return <div className={cn("animate-in fade-in duration-200", className)}>{children}</div>;
}

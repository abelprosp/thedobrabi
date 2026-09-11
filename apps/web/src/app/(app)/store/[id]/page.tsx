"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import {
  ArrowLeft,
  BarChart3,
  Check,
  Database,
  Eye,
  Filter,
  Layers3,
  Sparkles,
  Table2,
  Zap,
} from "lucide-react";
import { api, normalizeArray } from "@/lib/api";
import { Badge, Button, Card, EmptyState, FieldLabel, PageSkeleton, Select } from "@/components/ui";
import {
  CATEGORY_LABEL,
  DASHBOARD_TEMPLATES,
  instantiateTemplate,
  prepareTemplateModel,
  type DashboardTemplate,
} from "@/lib/dashboard-templates";
import { modelFromSemanticRow, type DatasetListItem, type SemanticModel } from "@/lib/semantic";

const widgetLabels: Record<string, string> = {
  kpi: "Indicador",
  bar: "Barras",
  line: "Linha",
  area: "Área",
  pie: "Composição",
  table: "Tabela",
  funnel: "Funil",
  combo: "Comparação",
};

export default function StoreModelPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const template = DASHBOARD_TEMPLATES.find((item) => item.id === params.id);
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<{ role?: string }>("/api/v1/auth/me") });
  const datasets = useQuery({ queryKey: ["datasets"], queryFn: () => api<any>("/api/v1/datasets") });
  const datasetList = normalizeArray<DatasetListItem>(datasets.data);
  const [datasetId, setDatasetId] = useState(datasetList[0]?.id || "");
  const canActivate = !me.data || me.data.role !== "viewer";

  const activate = useMutation({
    mutationFn: async () => {
      if (!template) throw new Error("Modelo não encontrado");
      if (!canActivate) throw new Error("Viewers não podem criar dashboards.");
      const dsId = datasetId || datasetList[0]?.id;
      if (!dsId) throw new Error("Ligue um conjunto de dados primeiro");
      const ds = await api<{ name: string; semantic_model?: SemanticModel | { model?: SemanticModel } }>(`/api/v1/datasets/${dsId}`);
      const model =
        (await prepareTemplateModel(dsId, template)) ||
        (ds.semantic_model && "measures" in ds.semantic_model ? ds.semantic_model : null) ||
        modelFromSemanticRow(ds.semantic_model);
      const widgets = instantiateTemplate(template, dsId, model);
      return api<{ id: string }>("/api/v1/dashboards", {
        method: "POST",
        body: JSON.stringify({
          name: template.name,
          description: `Painel pronto · ${CATEGORY_LABEL[template.category]} · ${ds.name || "conjunto"}`,
          layout: { widgets },
        }),
      });
    },
    onSuccess: (data) => {
      toast.success("Modelo instalado com os seus dados");
      router.push(`/dashboards/${data.id}`);
    },
    onError: (error: Error) => toast.error(error.message),
  });

  if (!template) {
    return (
      <div className="mx-auto max-w-6xl">
        <EmptyState title="Modelo não encontrado" description="Este modelo pode ter sido removido da loja." action={<Link href="/store"><Button>Voltar à loja</Button></Link>} />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-6xl space-y-6 pb-8">
      <Link href="/store" className="inline-flex items-center gap-2 rounded-lg text-[13px] text-mute transition hover:text-ink">
        <ArrowLeft size={15} /> Voltar à loja
      </Link>

      <section className="overflow-hidden rounded-3xl border border-line bg-surface shadow-[var(--shadow-card)]">
        <div className="panel-gradient relative px-5 py-8 text-white sm:px-8 sm:py-10">
          <div className="grid-fade pointer-events-none absolute inset-0 opacity-50" />
          <div className="relative z-10 max-w-3xl">
            <div className="flex flex-wrap items-center gap-2">
              <Badge tone="accent">{CATEGORY_LABEL[template.category]}</Badge>
              {template.popular && <span className="rounded-full bg-white/15 px-2.5 py-1 text-[11px] font-medium">Mais usado</span>}
            </div>
            <h1 className="mt-4 text-3xl font-semibold tracking-tight sm:text-4xl">{template.name}</h1>
            <p className="mt-3 max-w-2xl text-sm leading-relaxed text-white/80 sm:text-base">{template.description}</p>
            <div className="mt-6 flex flex-wrap gap-2 text-[12px] text-white/80">
              <span className="rounded-full border border-white/15 bg-white/10 px-3 py-1.5">{template.widgets.length} visualizações</span>
              <span className="rounded-full border border-white/15 bg-white/10 px-3 py-1.5">{template.measures?.length || 0} métricas prontas</span>
              <span className="rounded-full border border-white/15 bg-white/10 px-3 py-1.5">Instalação em minutos</span>
            </div>
          </div>
        </div>
        <div className="grid gap-4 p-5 sm:grid-cols-3 sm:p-7">
          <InfoItem icon={Eye} title="Veja antes de instalar" text="Explore a estrutura e os indicadores do painel." />
          <InfoItem icon={Database} title="Use os seus dados" text="Escolha o conjunto que alimentará o dashboard." />
          <InfoItem icon={Sparkles} title="Pronto para adaptar" text="Edite filtros, métricas e visualizações depois." />
        </div>
      </section>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_340px] lg:items-start">
        <div className="space-y-6">
          <Card>
            <div className="mb-5 flex items-start justify-between gap-3">
              <div>
                <p className="text-sm font-semibold text-ink">Preview do dashboard</p>
                <p className="mt-1 text-[13px] text-mute">Uma visão da composição que será criada.</p>
              </div>
              <span className="inline-flex items-center gap-1.5 rounded-full bg-primary/10 px-2.5 py-1 text-[11px] font-medium text-primary"><Eye size={13} /> Preview</span>
            </div>
            <div className="rounded-2xl border border-line bg-bg p-3 sm:p-4">
              <div className="mb-4 flex items-center justify-between">
                <div>
                  <p className="text-[12px] font-semibold text-ink">{template.name}</p>
                  <p className="text-[10px] text-mute">Visão geral · atualização automática</p>
                </div>
                <div className="flex gap-1.5"><span className="h-7 w-7 rounded-lg bg-surface" /><span className="h-7 w-7 rounded-lg bg-surface" /></div>
              </div>
              <div className="grid gap-3 sm:grid-cols-2">
                {template.widgets.slice(0, 6).map((widget, index) => <PreviewWidget key={`${widget.title}-${index}`} widget={widget} index={index} />)}
              </div>
            </div>
          </Card>

          <Card>
            <SectionHeading icon={Layers3} title="O que vem neste modelo" subtitle="Estrutura preparada para responder às perguntas mais importantes da área." />
            <div className="mt-4 grid gap-2 sm:grid-cols-2">
              {template.widgets.map((widget, index) => (
                <div key={`${widget.title}-${index}`} className="flex items-center gap-3 rounded-xl border border-line p-3">
                  <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                    {widget.type === "table" ? <Table2 size={15} /> : widget.type === "kpi" ? <Zap size={15} /> : <BarChart3 size={15} />}
                  </div>
                  <div className="min-w-0">
                    <p className="truncate text-[13px] font-medium text-ink">{widget.title}</p>
                    <p className="text-[11px] text-mute">{widgetLabels[widget.type] || "Visualização"}</p>
                  </div>
                </div>
              ))}
            </div>
          </Card>
        </div>

        <aside className="space-y-4 lg:sticky lg:top-5">
          <Card className="border-primary/20 shadow-md">
            <p className="text-base font-semibold text-ink">Instalar este modelo</p>
            <p className="mt-1 text-[13px] leading-relaxed text-mute">Escolha os dados e a TheDobra cria o dashboard no seu espaço.</p>
            <div className="mt-5 space-y-4">
              {datasets.isLoading ? <PageSkeleton cards={1} /> : datasetList.length === 0 ? (
                <div className="rounded-xl border border-amber-200 bg-amber-50 p-3 text-[13px] text-amber-900">
                  Ligue um conjunto de dados antes de instalar.
                  <div className="mt-3 flex gap-2"><Link href="/data"><Button size="sm" variant="secondary">Ir para Dados</Button></Link><Link href="/connectors"><Button size="sm" variant="secondary">Conectores</Button></Link></div>
                </div>
              ) : (
                <FieldLabel label="Conjunto de dados" hint="As medidas e dimensões serão mapeadas para este conjunto.">
                  <Select value={datasetId || datasetList[0]?.id} onChange={(event) => setDatasetId(event.target.value)}>
                    {datasetList.map((dataset) => <option key={dataset.id} value={dataset.id}>{dataset.name}</option>)}
                  </Select>
                </FieldLabel>
              )}
              <Button className="w-full" busy={activate.isPending} disabled={!datasetList.length || !canActivate} onClick={() => activate.mutate()}>
                <Zap size={15} /> Instalar dashboard
              </Button>
              {!canActivate && <p className="text-center text-[11px] text-mute">O seu perfil não pode criar dashboards.</p>}
            </div>
          </Card>
          <Card>
            <SectionHeading icon={Filter} title="Necessidades do modelo" subtitle="Para obter o melhor resultado, os seus dados devem conter:" />
            <ul className="mt-4 space-y-2.5">
              {template.needs.map((need) => <li key={need} className="flex items-start gap-2 text-[13px] text-mute"><Check size={15} className="mt-0.5 shrink-0 text-emerald-500" />{need}</li>)}
            </ul>
          </Card>
        </aside>
      </div>
    </div>
  );
}

function InfoItem({ icon: Icon, title, text }: { icon: typeof Eye; title: string; text: string }) {
  return <div className="flex gap-3"><div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary"><Icon size={16} /></div><div><p className="text-[13px] font-medium text-ink">{title}</p><p className="mt-0.5 text-[12px] leading-relaxed text-mute">{text}</p></div></div>;
}

function SectionHeading({ icon: Icon, title, subtitle }: { icon: typeof Layers3; title: string; subtitle: string }) {
  return <div className="flex items-start gap-3"><div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary"><Icon size={17} /></div><div><h2 className="text-sm font-semibold text-ink">{title}</h2><p className="mt-1 text-[12px] leading-relaxed text-mute">{subtitle}</p></div></div>;
}

function PreviewWidget({ widget, index }: { widget: DashboardTemplate["widgets"][number]; index: number }) {
  const bars = [58, 78, 42, 88, 66, 74];
  return (
    <div className={`min-h-28 rounded-xl border border-line bg-surface p-3 ${widget.type === "kpi" ? "sm:col-span-1" : widget.type === "table" ? "sm:col-span-2" : ""}`}>
      <div className="flex items-center justify-between gap-2"><p className="truncate text-[11px] font-medium text-ink">{widget.title}</p><span className="text-[9px] text-mute">{widgetLabels[widget.type] || "Gráfico"}</span></div>
      {widget.type === "kpi" ? <div className="mt-4"><div className="h-5 w-24 rounded bg-primary/10" /><div className="mt-2 h-2 w-14 rounded bg-emerald-500/30" /></div> :
        widget.type === "table" ? <div className="mt-4 space-y-2">{[1, 2, 3].map((row) => <div key={row} className="flex gap-2"><span className="h-2 flex-1 rounded bg-surface-2" /><span className="h-2 w-16 rounded bg-primary/10" /><span className="h-2 w-10 rounded bg-surface-2" /></div>)}</div> :
        <div className="mt-4 flex h-14 items-end gap-1.5">{bars.map((height, bar) => <span key={bar} className={`flex-1 rounded-t ${bar === index + 2 ? "bg-accent" : "bg-primary/25"}`} style={{ height: `${height}%` }} />)}</div>}
    </div>
  );
}

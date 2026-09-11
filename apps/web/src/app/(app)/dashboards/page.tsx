"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import Link from "next/link";
import { toast } from "sonner";
import { useRouter, useSearchParams } from "next/navigation";
import {
  LayoutDashboard,
  Search,
  Sparkles,
  Store,
  Trash2,
  Wand2,
  X,
  AlertCircle,
  Loader2,
} from "lucide-react";
import {
  Button,
  Card,
  EmptyState,
  ErrorState,
  PageHeader,
  PageSkeleton,
  Input,
  Select,
  Textarea,
  Skeleton,
  cn,
} from "@/components/ui";
import { Suspense, useEffect, useMemo, useRef, useState } from "react";
import { starterDashboardWidgets, type DatasetListItem } from "@/lib/semantic";

type Dash = {
  id: string;
  name: string;
  description: string;
  updated_at: string;
};

const STEPS = [
  "A analisar dados…",
  "A escolher visualizações…",
  "A montar dashboard…",
];

export default function DashboardsPage() {
  return (
    <Suspense fallback={<PageSkeleton />}>
      <DashboardsPageInner />
    </Suspense>
  );
}

function DashboardsPageInner() {
  const router = useRouter();
  const qc = useQueryClient();
  const searchParams = useSearchParams();
  const seedDatasetId = searchParams.get("dataset_id") || "";
  const seeded = useRef(false);
  const me = useQuery({
    queryKey: ["me"],
    queryFn: () => api<{ role?: string }>("/api/v1/auth/me"),
  });
  const canDelete = !me.data || me.data.role !== "viewer";
  const q = useQuery({
    queryKey: ["dashboards"],
    queryFn: () => api<any>("/api/v1/dashboards"),
  });
  const dashboards = normalizeArray<Dash>(q.data);
  const datasets = useQuery({
    queryKey: ["datasets"],
    queryFn: () => api<any>("/api/v1/datasets"),
  });
  const datasetList = normalizeArray<DatasetListItem>(datasets.data);
  const aiConfig = useQuery({
    queryKey: ["ai-config"],
    queryFn: () => api<{ openai_configured: boolean }>("/api/v1/ai/config"),
  });

  const [aiOpen, setAiOpen] = useState(false);
  const [aiPrompt, setAiPrompt] = useState("");
  const [aiDataset, setAiDataset] = useState("");
  const [step, setStep] = useState(0);
  const [search, setSearch] = useState("");
  const visibleDashboards = useMemo(() => {
    const query = search.trim().toLowerCase();
    if (!query) return dashboards;
    return dashboards.filter((dashboard) =>
      `${dashboard.name} ${dashboard.description}`
        .toLowerCase()
        .includes(query),
    );
  }, [dashboards, search]);

  const create = useMutation({
    mutationFn: () =>
      api<{ id: string }>("/api/v1/dashboards", {
        method: "POST",
        body: JSON.stringify({
          name: "Dashboard sem título",
          layout: { widgets: [] },
        }),
      }),
    onSuccess: (d) => router.push(`/dashboards/${d.id}`),
    onError: (e: Error) => toast.error(e.message),
  });

  const remove = useMutation({
    mutationFn: (id: string) =>
      api(`/api/v1/dashboards/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success("Dashboard excluído");
      qc.invalidateQueries({ queryKey: ["dashboards"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const generateDashboard = useMutation({
    mutationFn: () =>
      api<{ id: string; name: string; url: string; source: string }>(
        "/api/v1/ai/generate-dashboard",
        {
          method: "POST",
          body: JSON.stringify({
            prompt: aiPrompt,
            dataset_id: aiDataset || undefined,
          }),
        },
      ),
    onMutate: () => setStep(0),
    onSuccess: (d) => {
      toast.success(`Dashboard "${d.name}" criado`);
      setAiOpen(false);
      setAiPrompt("");
      setAiDataset("");
      router.push(d.url || `/dashboards/${d.id}`);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  useEffect(() => {
    if (!seedDatasetId || seeded.current) return;
    seeded.current = true;
    (async () => {
      try {
        const ds = await api<{ name: string; semantic_model?: any }>(
          `/api/v1/datasets/${seedDatasetId}`,
        );
        const widgets = starterDashboardWidgets(
          seedDatasetId,
          ds.semantic_model,
        );
        const dash = await api<{ id: string }>("/api/v1/dashboards", {
          method: "POST",
          body: JSON.stringify({
            name: ds.name || "Dashboard",
            description: `A partir do conjunto ${ds.name || ""}`.trim(),
            layout: { widgets },
          }),
        });
        router.replace(`/dashboards/${dash.id}`);
      } catch (e: any) {
        seeded.current = false;
        toast.error(
          e.message || "Não foi possível abrir o conjunto no dashboard",
        );
      }
    })();
  }, [seedDatasetId, router]);

  return (
    <div className="mx-auto max-w-5xl space-y-5">
      <PageHeader
        title="Dashboards"
        description="Crie, acompanhe e partilhe as métricas que movem o seu negócio."
        actions={
          <>
            <Link href="/store">
              <Button variant="secondary">
                <Store size={16} /> Modelos
              </Button>
            </Link>
            <Button
              variant="secondary"
              data-onboarding="new-dashboard"
              onClick={() => create.mutate()}
              busy={create.isPending}
            >
              Em branco
            </Button>
            <Button onClick={() => setAiOpen(true)}>
              <Sparkles size={16} /> Criar com DobraAI
            </Button>
          </>
        }
      />
      {q.isLoading && <PageSkeleton cards={4} />}
      {q.isError && (
        <ErrorState
          message={(q.error as Error).message}
          onRetry={() => q.refetch()}
        />
      )}
      {dashboards.length > 0 && (
        <div className="flex flex-col gap-3 rounded-2xl border border-line bg-surface p-3 shadow-[var(--shadow-card)] sm:flex-row sm:items-center">
          <div className="relative min-w-0 flex-1">
            <Search
              size={15}
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-mute"
            />
            <Input
              aria-label="Procurar dashboard"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Procurar por nome ou descrição…"
              className="pl-9"
            />
          </div>
          <span className="text-[12px] text-mute">
            {visibleDashboards.length} de {dashboards.length} painéis
          </span>
        </div>
      )}
      {dashboards.length === 0 && !q.isLoading && !q.isError && (
        <div className="space-y-4">
          <EmptyState
            icon={LayoutDashboard}
            title="Ainda sem dashboards"
            description="A DobraAI transforma os seus dados num painel com KPIs, gráficos, filtros e análises."
            action={
              <div className="flex flex-wrap gap-2">
                <Button onClick={() => setAiOpen(true)}>
                  <Sparkles size={16} /> Criar com DobraAI
                </Button>
                <Button
                  variant="secondary"
                  data-onboarding="new-dashboard"
                  onClick={() => create.mutate()}
                  busy={create.isPending}
                >
                  Começar em branco
                </Button>
              </div>
            }
          />
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <EducationalCard
              title="1. Descreva"
              body="Diga à DobraAI o que precisa acompanhar."
            />
            <EducationalCard
              title="2. Ajuste"
              body="Edite métricas, gráficos, filtros e layout."
            />
            <EducationalCard
              title="3. Partilhe"
              body="Envie o link ou incorpore no seu sistema."
            />
          </div>
        </div>
      )}
      {!!dashboards.length && visibleDashboards.length === 0 && (
        <EmptyState
          icon={Search}
          title="Nenhum dashboard encontrado"
          description="Tente outro nome ou limpe a pesquisa."
          action={
            <Button variant="secondary" onClick={() => setSearch("")}>
              Limpar pesquisa
            </Button>
          }
        />
      )}
      {!!visibleDashboards.length && (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {visibleDashboards.map((d) => (
            <Card
              key={d.id}
              className="group h-full transition-all hover:-translate-y-0.5 hover:border-primary/25 hover:shadow-lg"
            >
              <div className="flex items-start justify-between gap-2">
                <Link
                  href={`/dashboards/${d.id}`}
                  className="min-w-0 flex-1 py-1"
                >
                  <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary/10 text-primary">
                    <LayoutDashboard size={17} />
                  </div>
                  <div className="mt-4 text-base font-semibold tracking-tight text-ink">
                    {d.name}
                  </div>
                  <div className="mt-1 line-clamp-2 min-h-9 text-[12px] leading-relaxed text-mute">
                    {d.description || "Dashboard sem descrição"}
                  </div>
                  <div className="mt-4 text-[11px] text-mute">
                    Atualizado{" "}
                    {new Date(d.updated_at).toLocaleDateString("pt-BR")}
                  </div>
                </Link>
                {canDelete && (
                  <Button
                    size="icon"
                    variant="ghost"
                    className="text-mute opacity-70 hover:text-danger sm:opacity-0 sm:group-hover:opacity-100"
                    onClick={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      if (
                        confirm(
                          `Excluir o dashboard «${d.name}»? Esta ação não pode ser desfeita.`,
                        )
                      ) {
                        remove.mutate(d.id);
                      }
                    }}
                    busy={remove.isPending}
                  >
                    <Trash2 size={14} />
                  </Button>
                )}
              </div>
            </Card>
          ))}
        </div>
      )}

      {aiOpen && (
        <div className="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4">
          <Card className="max-h-[92dvh] w-full max-w-lg space-y-4 overflow-y-auto rounded-b-none pb-[max(1.25rem,env(safe-area-inset-bottom))] sm:rounded-2xl sm:pb-5">
            <div className="flex items-start justify-between">
              <div>
                <div className="mb-1 flex items-center gap-2">
                  <span className="flex h-8 w-8 items-center justify-center rounded-xl bg-primary/10 text-primary">
                    <Sparkles size={15} />
                  </span>
                  <h3 className="text-lg font-semibold tracking-tight text-ink">
                    Criar com DobraAI
                  </h3>
                </div>
                <p className="text-[13px] text-mute">
                  Descreva a decisão que precisa tomar. A DobraAI escolhe
                  métricas, gráficos e filtros.
                </p>
              </div>
              <button
                onClick={() => setAiOpen(false)}
                className="rounded-lg p-1 text-mute hover:bg-surface-2"
              >
                <X size={18} />
              </button>
            </div>

            {aiConfig.data?.openai_configured === false && (
              <div className="flex items-start gap-2 rounded-xl border border-amber-200 bg-amber-50 p-3 text-[13px] text-amber-800">
                <AlertCircle size={16} className="mt-0.5 shrink-0" />
                <span>
                  Configure OPENAI_API_KEY para geração inteligente; até lá usa
                  sugestões automáticas.
                </span>
              </div>
            )}

            {generateDashboard.isPending ? (
              <div className="space-y-4 py-4">
                {STEPS.map((s, i) => (
                  <div key={s} className="flex items-center gap-3">
                    <div
                      className={cn(
                        "flex h-6 w-6 items-center justify-center rounded-full text-[11px] font-medium",
                        i <= step
                          ? "bg-primary text-white"
                          : "bg-surface-2 text-mute",
                      )}
                    >
                      {i < step ? "✓" : i + 1}
                    </div>
                    <span
                      className={cn(
                        "text-sm",
                        i <= step ? "text-ink" : "text-mute",
                      )}
                    >
                      {s}
                    </span>
                    {i === step && (
                      <Loader2
                        size={14}
                        className="ml-auto animate-spin text-primary"
                      />
                    )}
                  </div>
                ))}
                <Skeleton className="h-32 w-full" />
              </div>
            ) : (
              <>
                <div className="space-y-3">
                  <label className="block text-[13px] font-medium text-ink">
                    O que pretende ver?
                    <Textarea
                      value={aiPrompt}
                      onChange={(e) => setAiPrompt(e.target.value)}
                      placeholder="Ex.: executivo de vendas com receita, margem, regionais e tendência"
                      className="mt-1.5 min-h-24"
                    />
                  </label>
                  <label className="block text-[13px] font-medium text-ink">
                    Conjunto de dados (opcional)
                    <Select
                      value={aiDataset}
                      onChange={(e) => setAiDataset(e.target.value)}
                      className="mt-1.5"
                    >
                      <option value="">Automático (último usado)</option>
                      {datasetList.map((ds) => (
                        <option key={ds.id} value={ds.id}>
                          {ds.name}
                        </option>
                      ))}
                    </Select>
                  </label>
                </div>
                <div className="flex justify-end gap-2">
                  <Button variant="secondary" onClick={() => setAiOpen(false)}>
                    Cancelar
                  </Button>
                  <Button
                    onClick={() => generateDashboard.mutate()}
                    disabled={!aiPrompt.trim()}
                    busy={generateDashboard.isPending}
                  >
                    <Wand2 size={14} /> Gerar dashboard
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

function EducationalCard({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-line bg-surface p-4 shadow-[var(--shadow-card)]">
      <div className="text-sm font-medium text-ink">{title}</div>
      <p className="mt-1 text-[12px] text-mute">{body}</p>
    </div>
  );
}

"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import Link from "next/link";
import { toast } from "sonner";
import { useRouter, useSearchParams } from "next/navigation";
import { LayoutDashboard, Sparkles, Trash2, Wand2, X, AlertCircle, Loader2 } from "lucide-react";
import { Button, Card, EmptyState, ErrorState, PageHeader, PageSkeleton, Input, Select, Textarea, Skeleton, cn } from "@/components/ui";
import { Suspense, useEffect, useRef, useState } from "react";
import { starterDashboardWidgets, type DatasetListItem } from "@/lib/semantic";

type Dash = { id: string; name: string; description: string; updated_at: string };

const STEPS = ["A analisar dados…", "A escolher visualizações…", "A montar dashboard…"];

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
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<{ role?: string }>("/api/v1/auth/me") });
  const canDelete = !me.data || me.data.role !== "viewer";
  const q = useQuery({ queryKey: ["dashboards"], queryFn: () => api<any>("/api/v1/dashboards") });
  const dashboards = normalizeArray<Dash>(q.data);
  const datasets = useQuery({ queryKey: ["datasets"], queryFn: () => api<any>("/api/v1/datasets") });
  const datasetList = normalizeArray<DatasetListItem>(datasets.data);
  const aiConfig = useQuery({ queryKey: ["ai-config"], queryFn: () => api<{ openai_configured: boolean }>("/api/v1/ai/config") });

  const [aiOpen, setAiOpen] = useState(false);
  const [aiPrompt, setAiPrompt] = useState("");
  const [aiDataset, setAiDataset] = useState("");
  const [step, setStep] = useState(0);

  const create = useMutation({
    mutationFn: () =>
      api<{ id: string }>("/api/v1/dashboards", {
        method: "POST",
        body: JSON.stringify({ name: "Dashboard sem título", layout: { widgets: [] } }),
      }),
    onSuccess: (d) => router.push(`/dashboards/${d.id}`),
    onError: (e: Error) => toast.error(e.message),
  });

  const ai = useMutation({
    mutationFn: () =>
      api<{ id: string }>("/api/v1/dashboards/ai", {
        method: "POST",
        body: JSON.stringify({ prompt: "Criar um dashboard executivo de vendas" }),
      }),
    onSuccess: (d) => {
      toast.success("Dashboard gerado a partir do modelo semântico");
      router.push(`/dashboards/${d.id}`);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const remove = useMutation({
    mutationFn: (id: string) => api(`/api/v1/dashboards/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success("Dashboard excluído");
      qc.invalidateQueries({ queryKey: ["dashboards"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const generateDashboard = useMutation({
    mutationFn: () =>
      api<{ id: string; name: string; url: string; source: string }>("/api/v1/ai/generate-dashboard", {
        method: "POST",
        body: JSON.stringify({ prompt: aiPrompt, dataset_id: aiDataset || undefined }),
      }),
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
        const ds = await api<{ name: string; semantic_model?: any }>(`/api/v1/datasets/${seedDatasetId}`);
        const widgets = starterDashboardWidgets(seedDatasetId, ds.semantic_model);
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
        toast.error(e.message || "Não foi possível abrir o conjunto no dashboard");
      }
    })();
  }, [seedDatasetId, router]);

  return (
    <div className="mx-auto max-w-5xl space-y-5">
      <PageHeader
        title="Dashboards"
        description="Visualize o negócio em painéis que usa todos os dias."
        actions={
          <>
            <Button variant="secondary" onClick={() => ai.mutate()} busy={ai.isPending} className="text-accent">
              Construir com IA
            </Button>
            <Button variant="secondary" onClick={() => setAiOpen(true)}>
              <Sparkles size={16} /> Novo dashboard com IA
            </Button>
            <Button data-onboarding="new-dashboard" onClick={() => create.mutate()} busy={create.isPending}>
              Novo dashboard
            </Button>
          </>
        }
      />
      {q.isLoading && <PageSkeleton cards={4} />}
      {q.isError && <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />}
      {dashboards.length === 0 && (
        <div className="space-y-4">
          <EmptyState
            icon={LayoutDashboard}
            title="Ainda sem dashboards"
            description="Depois de ter dados e métricas, crie um painel executivo. Use a IA para gerar automaticamente KPIs, gráficos e tabelas."
            action={
              <div className="flex flex-wrap gap-2">
                <Button data-onboarding="new-dashboard" onClick={() => create.mutate()} busy={create.isPending}>
                  Criar primeiro dashboard
                </Button>
                <Button variant="secondary" onClick={() => ai.mutate()} busy={ai.isPending}>
                  Construir com IA
                </Button>
                <Button onClick={() => setAiOpen(true)}>
                  <Sparkles size={16} /> Novo dashboard com IA
                </Button>
              </div>
            }
          />
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <EducationalCard title="1. Crie" body="Novo dashboard em branco." />
            <EducationalCard title="2. Adicione widgets" body="KPI, linha, barras, pizza, tabela e slicers." />
            <EducationalCard title="3. Partilhe" body="Copie a ligação pública ou convide a equipa." />
          </div>
        </div>
      )}
      {!!dashboards.length && (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {dashboards.map((d) => (
            <Card key={d.id} className="h-full transition hover:border-accent/30">
              <div className="flex items-start justify-between gap-2">
                <Link href={`/dashboards/${d.id}`} className="min-w-0 flex-1">
                  <div className="text-sm font-medium text-ink">{d.name}</div>
                  <div className="mt-1 text-[12px] text-mute">{d.description || "Sem descrição"}</div>
                </Link>
                {canDelete && (
                  <Button
                    size="sm"
                    variant="danger"
                    onClick={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      if (confirm(`Excluir o dashboard «${d.name}»? Esta ação não pode ser desfeita.`)) {
                        remove.mutate(d.id);
                      }
                    }}
                    busy={remove.isPending}
                  >
                    <Trash2 size={12} /> Excluir
                  </Button>
                )}
              </div>
            </Card>
          ))}
        </div>
      )}

      {aiOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
          <Card className="w-full max-w-lg space-y-4">
            <div className="flex items-start justify-between">
              <div>
                <h3 className="text-lg font-medium text-ink">Novo dashboard com IA</h3>
                <p className="text-[13px] text-mute">Descreva o que precisa e a IA monta o painel.</p>
              </div>
              <button onClick={() => setAiOpen(false)} className="rounded-lg p-1 text-mute hover:bg-surface-2">
                <X size={18} />
              </button>
            </div>

            {aiConfig.data?.openai_configured === false && (
              <div className="flex items-start gap-2 rounded-xl border border-amber-200 bg-amber-50 p-3 text-[13px] text-amber-800">
                <AlertCircle size={16} className="mt-0.5 shrink-0" />
                <span>Configure OPENAI_API_KEY para geração inteligente; até lá usa sugestões automáticas.</span>
              </div>
            )}

            {generateDashboard.isPending ? (
              <div className="space-y-4 py-4">
                {STEPS.map((s, i) => (
                  <div key={s} className="flex items-center gap-3">
                    <div className={cn("flex h-6 w-6 items-center justify-center rounded-full text-[11px] font-medium", i <= step ? "bg-primary text-white" : "bg-surface-2 text-mute")}>
                      {i < step ? "✓" : i + 1}
                    </div>
                    <span className={cn("text-sm", i <= step ? "text-ink" : "text-mute")}>{s}</span>
                    {i === step && <Loader2 size={14} className="ml-auto animate-spin text-primary" />}
                  </div>
                ))}
                <Skeleton className="h-32 w-full" />
              </div>
            ) : (
              <>
                <div className="space-y-3">
                  <label className="block text-[13px] font-medium text-ink">
                    O que pretende ver?
                    <Textarea value={aiPrompt} onChange={(e) => setAiPrompt(e.target.value)} placeholder="Ex.: executivo de vendas com receita, margem, regionais e tendência" className="mt-1.5 min-h-24" />
                  </label>
                  <label className="block text-[13px] font-medium text-ink">
                    Conjunto de dados (opcional)
                    <Select value={aiDataset} onChange={(e) => setAiDataset(e.target.value)} className="mt-1.5">
                      <option value="">Automático (último usado)</option>
                      {datasetList.map((ds) => <option key={ds.id} value={ds.id}>{ds.name}</option>)}
                    </Select>
                  </label>
                </div>
                <div className="flex justify-end gap-2">
                  <Button variant="secondary" onClick={() => setAiOpen(false)}>Cancelar</Button>
                  <Button onClick={() => generateDashboard.mutate()} disabled={!aiPrompt.trim()} busy={generateDashboard.isPending}>
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
    <div className="rounded-2xl border border-line bg-white p-4 shadow-sm">
      <div className="text-sm font-medium text-ink">{title}</div>
      <p className="mt-1 text-[12px] text-mute">{body}</p>
    </div>
  );
}

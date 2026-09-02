"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Kpi } from "@/components/viz";
import { toast } from "sonner";
import { Button, Card, CardTitle, ErrorState, PageSkeleton } from "@/components/ui";
import { OnboardingChecklist } from "@/components/onboarding";
import Link from "next/link";
import { Store } from "lucide-react";

type Brief = {
  headline: string;
  major_changes: Item[];
  risks: Item[];
  opportunities: Item[];
  recommended_actions: string[];
};
type Item = { title: string; body: string; kind: string; severity: string };

export default function OverviewPage() {
  const q = useQuery({
    queryKey: ["overview"],
    queryFn: () => api<{ datasets: number; brief?: Brief }>("/api/v1/overview"),
  });
  const demo = useMutation({
    mutationFn: () => api("/api/v1/datasets/demo", { method: "POST" }),
    onSuccess: () => {
      toast.success("Conjunto de vendas de demonstração ingerido");
      q.refetch();
    },
    onError: (e: Error) => toast.error(e.message),
  });
  const brief = q.data?.brief;

  if (q.isLoading) return <PageSkeleton />;
  if (q.isError) return <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />;

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <section className="panel-gradient relative overflow-hidden rounded-3xl px-6 py-8 text-white sm:px-8">
        <div className="grid-fade pointer-events-none absolute inset-0 opacity-50" />
        <div className="pointer-events-none absolute -top-24 -right-16 h-64 w-64 rounded-full bg-indigo-500/30 blur-3xl" />
        <div className="pointer-events-none absolute -bottom-28 left-1/3 h-64 w-64 rounded-full bg-sky-500/20 blur-3xl" />
        <div className="relative z-10 flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
          <div className="max-w-2xl">
            <p className="text-[12px] uppercase tracking-[0.18em] text-indigo-200/90">Analista digital de negócio</p>
            <h1 className="mt-2 text-2xl font-semibold tracking-tight sm:text-3xl">
              {brief?.headline || "Analisei o seu negócio. Eis o que importa."}
            </h1>
            <p className="mt-2 text-sm text-white/80">O analista digital do seu negócio — métricas oficiais, sem fórmulas inventadas.</p>
          </div>
          {(!q.data?.datasets || q.data.datasets === 0) && (
            <Button
              onClick={() => demo.mutate()}
              busy={demo.isPending}
              className="shrink-0 bg-white text-panel hover:bg-white/90"
            >
              {demo.isPending ? "A ingerir…" : "Carregar dados de demonstração"}
            </Button>
          )}
        </div>
        <div className="relative z-10 mt-6 grid gap-3 sm:grid-cols-3">
          <GlassStat label="Mudanças" value={String(brief?.major_changes?.length ?? 0)} />
          <GlassStat label="Riscos" value={String(brief?.risks?.length ?? 0)} />
          <GlassStat label="Oportunidades" value={String(brief?.opportunities?.length ?? 0)} />
        </div>
      </section>
      <OnboardingChecklist />
      <Link href="/store" className="block">
        <Card className="flex items-center justify-between gap-4 transition hover:border-primary/30">
          <div>
            <div className="mb-1 text-[13px] font-medium text-mute">Loja de painéis</div>
            <p className="text-sm text-ink">Financeiro, comercial, RH, e-commerce e mais — ative um painel pronto e só ligue o conjunto.</p>
          </div>
          <span className="inline-flex shrink-0 items-center gap-2 rounded-xl border border-line bg-white px-4 py-2 text-sm font-medium text-ink">
            <Store size={14} /> Abrir loja
          </span>
        </Card>
      </Link>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <Kpi label="Mudanças principais" value={String(brief?.major_changes?.length ?? 0)} />
        <Kpi label="Riscos" value={String(brief?.risks?.length ?? 0)} />
        <Kpi label="Oportunidades" value={String(brief?.opportunities?.length ?? 0)} />
      </div>
      <section className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Card>
          <CardTitle>O que aconteceu</CardTitle>
          {(brief?.major_changes || []).map((i) => (
            <Row key={i.title} title={i.title} body={i.body} />
          ))}
          {!brief?.major_changes?.length && <p className="text-sm text-mute">Ligue dados para gerar o briefing.</p>}
        </Card>
        <Card>
          <CardTitle>Porque importa</CardTitle>
          {(brief?.risks || []).concat(brief?.opportunities || []).map((i) => (
            <Row key={i.title} title={i.title} body={i.body} />
          ))}
          {!brief?.risks?.length && !brief?.opportunities?.length && (
            <p className="text-sm text-mute">Ainda sem riscos nem oportunidades.</p>
          )}
        </Card>
      </section>
      <Card>
        <CardTitle>O que deve fazer</CardTitle>
        {(brief?.recommended_actions || []).length === 0 ? (
          <p className="text-sm text-mute">Sem acções recomendadas por agora.</p>
        ) : (
          <ol className="list-decimal space-y-2 pl-4">
            {(brief?.recommended_actions || []).map((a) => (
              <li key={a} className="text-sm text-ink">
                {a}
              </li>
            ))}
          </ol>
        )}
      </Card>
    </div>
  );
}

function GlassStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="glass-card rounded-2xl px-4 py-3">
      <div className="text-[11px] uppercase tracking-wide text-white/70">{label}</div>
      <div className="mt-1 text-2xl font-semibold">{value}</div>
    </div>
  );
}

function Row({ title, body }: { title: string; body: string }) {
  return (
    <div className="border-t border-line py-3 first:border-0 first:pt-0">
      <div className="text-sm text-ink">{title}</div>
      <div className="mt-1 text-[12px] text-mute">{body}</div>
    </div>
  );
}

"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorState,
  PageHeader,
  PageSkeleton,
} from "@/components/ui";
import { BarChart3 } from "lucide-react";
import Link from "next/link";

export default function MetricsPage() {
  const q = useQuery({
    queryKey: ["semantic"],
    queryFn: () => api<any>("/api/v1/semantic-models"),
  });
  const goals = useQuery({
    queryKey: ["metric-goals"],
    queryFn: () => api<any>("/api/v1/goals"),
  });
  const models = useMemo(() => normalizeArray(q.data), [q.data]);
  const goalList = useMemo(() => normalizeArray<any>(goals.data), [goals.data]);
  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <PageHeader
        title="Métricas"
        description="A camada semântica é a fonte da verdade. A TheDobra nunca inventa uma fórmula."
      />
      {q.isLoading && <PageSkeleton cards={2} />}
      {q.isError && (
        <ErrorState
          message={(q.error as Error).message}
          onRetry={() => q.refetch()}
        />
      )}
      {models.length === 0 && !q.isLoading && !q.isError && (
        <div className="space-y-4">
          <EmptyState
            icon={BarChart3}
            title="Ainda sem métricas"
            description="As métricas nascem do modelo semântico. Carregue um conjunto, defina medidas e dimensões, e elas aparecem aqui."
            action={
              <Link href="/data">
                <Button>Ir para Dados</Button>
              </Link>
            }
          />
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <EducationalCard
              title="1. Dados"
              body="Carregue um conjunto em /data."
            />
            <EducationalCard
              title="2. Modelo semântico"
              body="Defina medidas, dimensões e coluna de tempo."
            />
            <EducationalCard
              title="3. Métricas oficiais"
              body="Use-as em dashboards, perguntas e alertas."
            />
          </div>
        </div>
      )}
      {models.map((m) => (
        <Card key={m.id}>
          <div className="text-sm font-medium text-ink">{m.name}</div>
          <div className="mt-3 space-y-2">
            {(m.model?.measures || []).map((x: any) => (
              <div
                key={x.name}
                className="flex items-baseline justify-between gap-3 text-sm"
              >
                <span className="flex items-center gap-2">
                  {x.name}
                  {goalList.some(
                    (goal) =>
                      goal.dataset_id === m.dataset_id &&
                      goal.metric_name === x.name,
                  ) && <Badge tone="accent">meta</Badge>}
                </span>
                <span className="font-mono text-[11px] text-accent">
                  {x.expression}
                </span>
              </div>
            ))}
          </div>
        </Card>
      ))}
      {goalList.length > 0 && (
        <Card>
          <div className="text-sm font-medium text-ink">
            Metas em acompanhamento
          </div>
          <div className="mt-3 grid gap-2 sm:grid-cols-2">
            {goalList.map((goal) => (
              <div
                key={goal.id}
                className="rounded-xl border border-line bg-bg px-3 py-2"
              >
                <div className="flex items-center justify-between gap-2 text-sm">
                  <span className="font-medium text-ink">{goal.name}</span>
                  <Badge tone={goal.status === "active" ? "ok" : "neutral"}>
                    {goal.status}
                  </Badge>
                </div>
                <p className="mt-1 text-[12px] text-mute">
                  {goal.metric_name} · meta{" "}
                  {Number(goal.target_value).toLocaleString("pt-BR")} ·{" "}
                  {goal.period}
                </p>
              </div>
            ))}
          </div>
        </Card>
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

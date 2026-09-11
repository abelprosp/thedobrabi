"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
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
import { LineChart } from "lucide-react";
import Link from "next/link";
import { toast } from "sonner";

export default function InsightsPage() {
  const qc = useQueryClient();
  const q = useQuery({
    queryKey: ["insights"],
    queryFn: () => api<any>("/api/v1/insights"),
  });
  const datasets = useQuery({
    queryKey: ["datasets"],
    queryFn: () => api<any>("/api/v1/datasets"),
  });
  const insights = normalizeArray(q.data);
  const dataset = normalizeArray<{ id: string; name: string }>(
    datasets.data,
  )[0];
  const forecast = useQuery({
    queryKey: ["dataset-forecast", dataset?.id],
    queryFn: () =>
      api<any>(`/api/v1/datasets/${dataset.id}/forecast?horizon=6`),
    enabled: Boolean(dataset?.id),
    retry: false,
  });
  const refresh = useMutation({
    mutationFn: () => api("/api/v1/insights/refresh", { method: "POST" }),
    onSuccess: () => {
      toast.success("Análise actualizada");
      qc.invalidateQueries({ queryKey: ["insights"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });
  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <PageHeader
        title="Insights"
        description="Um analista de dados lê o seu conjunto: tendências no tempo, concentração e o que fazer a seguir."
        actions={
          <Button
            variant="secondary"
            onClick={() => refresh.mutate()}
            busy={refresh.isPending}
          >
            Reanalisar
          </Button>
        }
      />
      {q.isLoading && <PageSkeleton cards={3} />}
      {q.isError && (
        <ErrorState
          message={(q.error as Error).message}
          onRetry={() => q.refetch()}
        />
      )}
      {insights.length === 0 && !q.isLoading && (
        <div className="space-y-4">
          <EmptyState
            icon={LineChart}
            title="Ainda sem insights"
            description="Clique em Reanalisar para o analista ler o conjunto mais recente. Distingue série temporal de ranking e aponta riscos."
            action={
              <Link href="/data">
                <Button>Ir para Dados</Button>
              </Link>
            }
          />
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <EducationalCard
              title="1. Dados"
              body="Carregue um conjunto em Dados."
            />
            <EducationalCard
              title="2. Reanalisar"
              body="O analista usa as métricas e dimensões reais do modelo, não nomes genéricos."
            />
            <EducationalCard
              title="3. Decidir"
              body="Leia alterações, concentração e recomendações."
            />
          </div>
        </div>
      )}
      {insights.map((i) => (
        <Card key={i.title + i.body}>
          <div className="flex gap-2">
            <Badge>{i.kind}</Badge>
            <Badge
              tone={
                i.severity === "high" ||
                i.severity === "critical" ||
                i.severity === "warn"
                  ? i.severity === "warn"
                    ? "warn"
                    : "danger"
                  : i.severity === "medium"
                    ? "warn"
                    : "neutral"
              }
            >
              {i.severity}
            </Badge>
          </div>
          <div className="mt-2 text-sm font-medium text-ink">{i.title}</div>
          <p className="mt-1 text-[13px] text-mute">{i.body}</p>
        </Card>
      ))}
      {forecast.data?.forecast?.forecast?.length > 0 && (
        <Card>
          <div className="flex items-center justify-between gap-3">
            <div>
              <div className="text-sm font-medium text-ink">
                Próximos períodos
              </div>
              <p className="mt-1 text-[12px] text-mute">
                Previsão de {forecast.data.measure} para{" "}
                {forecast.data.dataset_name}, baseada no histórico disponível.
              </p>
            </div>
            <Badge tone="accent">forecast</Badge>
          </div>
          <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-6">
            {forecast.data.forecast.forecast.map(
              (value: number, index: number) => (
                <div
                  key={index}
                  className="rounded-xl border border-line bg-bg px-2 py-2 text-center"
                >
                  <div className="text-[10px] text-mute">P{index + 1}</div>
                  <div className="mt-1 text-sm font-semibold text-ink">
                    {Number(value).toLocaleString("pt-BR", {
                      maximumFractionDigits: 2,
                    })}
                  </div>
                </div>
              ),
            )}
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

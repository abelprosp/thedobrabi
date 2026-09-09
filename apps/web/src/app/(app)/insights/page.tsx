"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { Badge, Button, Card, EmptyState, ErrorState, PageHeader, PageSkeleton } from "@/components/ui";
import { LineChart } from "lucide-react";
import Link from "next/link";
import { toast } from "sonner";

export default function InsightsPage() {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["insights"], queryFn: () => api<any>("/api/v1/insights") });
  const insights = normalizeArray(q.data);
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
          <Button variant="secondary" onClick={() => refresh.mutate()} busy={refresh.isPending}>
            Reanalisar
          </Button>
        }
      />
      {q.isLoading && <PageSkeleton cards={3} />}
      {q.isError && <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />}
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
            <EducationalCard title="1. Dados" body="Carregue um conjunto em Dados." />
            <EducationalCard title="2. Reanalisar" body="O analista usa as métricas e dimensões reais do modelo, não nomes genéricos." />
            <EducationalCard title="3. Decidir" body="Leia alterações, concentração e recomendações." />
          </div>
        </div>
      )}
      {insights.map((i) => (
        <Card key={i.title + i.body}>
          <div className="flex gap-2">
            <Badge>{i.kind}</Badge>
            <Badge tone={i.severity === "high" || i.severity === "critical" || i.severity === "warn" ? (i.severity === "warn" ? "warn" : "danger") : i.severity === "medium" ? "warn" : "neutral"}>
              {i.severity}
            </Badge>
          </div>
          <div className="mt-2 text-sm font-medium text-ink">{i.title}</div>
          <p className="mt-1 text-[13px] text-mute">{i.body}</p>
        </Card>
      ))}
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

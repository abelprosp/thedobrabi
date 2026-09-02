"use client";

import { useQuery } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { Badge, Button, Card, EmptyState, ErrorState, PageHeader, PageSkeleton } from "@/components/ui";
import { LineChart } from "lucide-react";
import Link from "next/link";

export default function InsightsPage() {
  const q = useQuery({ queryKey: ["insights"], queryFn: () => api<any>("/api/v1/insights") });
  const insights = normalizeArray(q.data);
  const refresh = useQuery({
    queryKey: ["insights-refresh"],
    queryFn: () => api("/api/v1/insights/refresh", { method: "POST" }),
    enabled: false,
  });
  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <PageHeader
        title="Insights"
        description="Alterações, riscos e oportunidades detectados nos seus dados."
        actions={
          <Button variant="secondary" onClick={() => refresh.refetch()} busy={refresh.isFetching}>
            Reanalisar
          </Button>
        }
      />
      {q.isLoading && <PageSkeleton cards={3} />}
      {q.isError && <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />}
      {insights.length === 0 && (
        <div className="space-y-4">
          <EmptyState
            icon={LineChart}
            title="Ainda sem insights"
            description="Os insights aparecem depois de analisar um conjunto de dados. Detetam alterações, riscos e oportunidades."
            action={
              <Link href="/data">
                <Button>Ir para Dados</Button>
              </Link>
            }
          />
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <EducationalCard title="1. Dados" body="Carregue um conjunto em /data." />
            <EducationalCard title="2. Reanalisar" body="Clique em Reanalisar na página Insights." />
            <EducationalCard title="3. Decidir" body="Leia alterações, riscos e recomendações." />
          </div>
        </div>
      )}
      {insights.map((i) => (
        <Card key={i.title + i.body}>
          <div className="flex gap-2">
            <Badge>{i.kind}</Badge>
            <Badge tone={i.severity === "high" || i.severity === "critical" ? "danger" : i.severity === "medium" ? "warn" : "neutral"}>
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

"use client";

import { Suspense, useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useRouter, useSearchParams } from "next/navigation";
import { Workflow } from "lucide-react";
import Link from "next/link";
import { api, normalizeArray } from "@/lib/api";
import { Badge, Button, Card, EmptyState, ErrorState, PageHeader, PageSkeleton } from "@/components/ui";
import { statusLabel } from "@/lib/labels";
import { NewFlowWizard } from "@/components/flows/new-flow-wizard";
import { canEditFlows, type Flow } from "@/lib/flows";

export default function FlowsPage() {
  return (
    <Suspense fallback={<PageSkeleton />}>
      <FlowsPageInner />
    </Suspense>
  );
}

function FlowsPageInner() {
  const search = useSearchParams();
  const router = useRouter();
  const [wizard, setWizard] = useState(false);
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<{ role?: string }>("/api/v1/auth/me") });
  const flows = useQuery({ queryKey: ["flows"], queryFn: () => api<any>("/api/v1/flows") });
  const flowList = normalizeArray<Flow>(flows.data);
  const writable = !me.data || canEditFlows(me.data.role);

  useEffect(() => {
    if (search.get("new") === "1" && writable) {
      setWizard(true);
      router.replace("/flows");
    }
  }, [search, writable, router]);

  if (flows.isLoading) return <PageSkeleton />;
  if (flows.isError) return <ErrorState message={(flows.error as Error).message} onRetry={() => flows.refetch()} />;

  return (
    <div className="mx-auto max-w-6xl space-y-5">
      <PageHeader
        title="Flows"
        description="Pipelines visuais (ETL/ELT) até ao ClickHouse. Escolha um modelo e o canvas abre já com os nós ligados."
        actions={
          writable ? (
            <Button data-onboarding="new-flow" onClick={() => setWizard(true)}>
              Novo flow
            </Button>
          ) : undefined
        }
      />

      {flowList.length === 0 && (
        <div className="space-y-4">
          <EmptyState
            icon={Workflow}
            title="Ainda sem flows"
            description="Crie o primeiro pipeline em segundos: origem → transformação → ClickHouse. O canvas abre já preenchido."
            action={
              writable ? (
                <Button data-onboarding="new-flow" onClick={() => setWizard(true)}>
                  Novo flow
                </Button>
              ) : undefined
            }
          />
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <Hint title="CSV → ClickHouse" body="O caminho mais rápido: um ficheiro já carregado em Dados vira tabela." />
            <Hint title="SQL → transformar" body="Lê um conjunto, aplica filtro ou SQL, e materializa." />
            <Hint title="Conector → CH" body="Usa dados já sincronizados de um conector existente." />
            <Hint title="Juntar 2 fontes" body="Dois conjuntos, um join e o resultado no ClickHouse." />
          </div>
        </div>
      )}

      {flowList.length > 0 && (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {flowList.map((f) => (
            <Link key={f.id} href={`/flows/${f.id}`}>
              <Card className="h-full transition hover:border-primary/40">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium text-ink">{f.name}</div>
                    <p className="mt-1 line-clamp-2 text-[12px] text-mute">{f.description || "Sem descrição"}</p>
                  </div>
                  <Badge tone={f.output_dataset_id ? "ok" : "accent"}>
                    {f.output_dataset_id ? "Materializado" : statusLabel(f.status)}
                  </Badge>
                </div>
                {f.output_dataset_id && <div className="mt-2 text-[12px] text-mute">Output disponível em Dados</div>}
              </Card>
            </Link>
          ))}
        </div>
      )}

      <NewFlowWizard open={wizard} onClose={() => setWizard(false)} />
    </div>
  );
}

function Hint({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-line bg-white p-4 shadow-sm">
      <div className="text-sm font-medium text-ink">{title}</div>
      <p className="mt-1 text-[12px] text-mute">{body}</p>
    </div>
  );
}

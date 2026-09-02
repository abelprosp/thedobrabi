"use client";

import { useQuery, useMutation } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { toast } from "sonner";
import { useRouter } from "next/navigation";
import { Button, EmptyState, ErrorState, PageHeader, PageSkeleton } from "@/components/ui";
import { GitBranch } from "lucide-react";

type Graph = {
  nodes: { id: string; kind: string; name: string; ref_id?: string; meta?: any }[];
  edges: { from: string; to: string; relation: string }[];
};

const kindLabel: Record<string, string> = {
  source: "Fonte",
  dataset: "Conjunto",
  transformation: "Transformação",
  metric: "Métrica",
  dashboard: "Dashboard",
  report: "Relatório",
  query: "Consulta",
};

export default function LineagePage() {
  const router = useRouter();
  const g = useQuery({ queryKey: ["lineage"], queryFn: () => api<Graph>("/api/v1/lineage") });
  const cdc = useQuery({ queryKey: ["cdc"], queryFn: () => api<any>("/api/v1/cdc") });
  const sources = useQuery({ queryKey: ["sources"], queryFn: () => api<any>("/api/v1/data-sources") });
  const cdcList = normalizeArray(cdc.data);
  const sourceList = normalizeArray(sources.data);
  const enable = useMutation({
    mutationFn: () => {
      const src = sourceList[0]?.id;
      if (!src) throw new Error("Ligue uma fonte PostgreSQL primeiro");
      return api("/api/v1/cdc/enable", { method: "POST", body: JSON.stringify({ data_source_id: src, table: "public.sales" }) });
    },
    onSuccess: () => {
      toast.success("CDC ativado");
      cdc.refetch();
    },
    onError: (e: Error) => toast.error(e.message),
  });
  const nodes = g.data?.nodes || [];
  const edges = g.data?.edges || [];
  const col: Record<string, number> = { source: 0, transformation: 1, dataset: 2, metric: 3, dashboard: 4, report: 5 };
  return (
    <div className="space-y-6">
      <PageHeader
        title="Linha de origem"
        description="Fonte → conjunto → métrica → dashboard → relatório. Cada número tem proveniência."
        actions={
          <Button variant="secondary" onClick={() => enable.mutate()} busy={enable.isPending}>
            Ativar CDC PostgreSQL
          </Button>
        }
      />
      {g.isLoading && <PageSkeleton />}
      {g.isError && <ErrorState message={(g.error as Error).message} onRetry={() => g.refetch()} />}
      {!g.isLoading && !g.isError && nodes.length === 0 && (
        <div className="space-y-4">
          <EmptyState
            icon={GitBranch}
            title="Sem linhagem"
            description="A linhagem mostra a proveniência de cada número: fonte → conjunto → métrica → dashboard → relatório."
          />
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <EducationalCard title="1. Fonte" body="Ligue PostgreSQL/MySQL ou carregue CSV." />
            <EducationalCard title="2. Conjunto" body="Sincronize ou carregue os dados." />
            <EducationalCard title="3. Proveniência" body="Veja o grafo de dependências dos números." />
          </div>
        </div>
      )}
      {!g.isLoading && nodes.length > 0 && (
      <div className="relative min-h-[320px] overflow-x-auto rounded-2xl border border-line bg-surface p-6 shadow-sm">
        <svg className="absolute inset-0 h-full w-full" aria-hidden>
          {edges.map((e, i) => {
            const a = nodes.find((n) => n.id === e.from);
            const b = nodes.find((n) => n.id === e.to);
            if (!a || !b) return null;
            const xa = (col[a.kind] ?? 0) * 180 + 80;
            const xb = (col[b.kind] ?? 0) * 180 + 80;
            const ya = 40 + nodes.filter((n) => n.kind === a.kind).indexOf(a) * 88 + 24;
            const yb = 40 + nodes.filter((n) => n.kind === b.kind).indexOf(b) * 88 + 24;
            return <line key={i} x1={xa} y1={ya} x2={xb} y2={yb} stroke="rgba(37,99,235,0.45)" strokeWidth="1.5" />;
          })}
        </svg>
        {nodes.map((n) => {
          const idx = nodes.filter((x) => x.kind === n.kind).indexOf(n);
          const href =
            n.kind === "dataset" && n.ref_id
              ? `/data/${n.ref_id}`
              : n.kind === "dashboard" && n.ref_id
                ? `/dashboards/${n.ref_id}`
                : n.kind === "source"
                  ? "/data"
                  : n.kind === "metric"
                    ? "/metrics"
                    : n.kind === "report"
                      ? "/reports"
                      : null;
          return (
            <div
              key={n.id}
              role={href ? "link" : undefined}
              className={`absolute w-40 rounded-xl border border-line bg-white p-3 shadow-sm ${href ? "cursor-pointer hover:border-accent/40" : ""}`}
              style={{ left: (col[n.kind] ?? 0) * 180, top: 40 + idx * 88 }}
              onClick={() => href && router.push(href)}
            >
              <div className="text-[10px] uppercase text-accent">{kindLabel[n.kind] || n.kind}</div>
              <div className="mt-1 truncate text-[13px] text-ink">{n.name}</div>
              {n.meta?.expression && <div className="mt-1 truncate font-mono text-[10px] text-mute">{n.meta.expression}</div>}
            </div>
          );
        })}
      </div>
      )}
      <div>
        <h2 className="mb-2 text-[13px] text-mute">Checkpoints CDC</h2>
        {cdcList.map((c) => (
          <div key={c.id} className="mb-2 rounded-xl border border-line bg-surface px-4 py-3 text-sm shadow-sm">
            {c.table} · {c.status} · LSN {c.cursor || "—"}
          </div>
        ))}
      </div>
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

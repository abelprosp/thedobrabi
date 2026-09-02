"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import Link from "next/link";
import { toast } from "sonner";
import { Database, Plug } from "lucide-react";
import { Badge, Button, Card, EmptyState, ErrorState, PageHeader, PageSkeleton, Table, TableWrap, Td, Th, formatPt } from "@/components/ui";
import { statusLabel } from "@/lib/labels";

type Dataset = {
  id: string;
  name: string;
  status: string;
  row_count: number;
  quality_score?: number;
  storage_mode?: string;
  source_id?: string | null;
  source_type?: string | null;
  source_name?: string | null;
};
type Source = { id: string; name: string; type: string; status: string; last_sync_at?: string; preview?: boolean };
type Lake = { id: string; stage: string; key: string; bytes: number; created_at: string };

export default function DataPage() {
  const q = useQuery({ queryKey: ["datasets"], queryFn: () => api<any>("/api/v1/datasets") });
  const sources = useQuery({ queryKey: ["sources"], queryFn: () => api<any>("/api/v1/data-sources") });
  const lake = useQuery({ queryKey: ["lake"], queryFn: () => api<any>("/api/v1/lake") });
  const datasetList = normalizeArray<Dataset>(q.data);
  const sourceList = normalizeArray<Source>(sources.data);
  const lakeList = normalizeArray<Lake>(lake.data);
  const demo = useMutation({
    mutationFn: () => api("/api/v1/datasets/demo", { method: "POST" }),
    onSuccess: () => {
      toast.success("Conjunto pronto");
      q.refetch();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  async function onUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    const fd = new FormData();
    fd.append("file", file);
    fd.append("name", file.name.replace(/\.[^.]+$/, ""));
    try {
      await api("/api/v1/datasets/upload", { method: "POST", body: fd });
      toast.success("Ingerido " + file.name);
      q.refetch();
    } catch (err: any) {
      toast.error(err.message);
    }
  }

  return (
    <div className="mx-auto max-w-5xl space-y-5">
      <PageHeader
        title="Dados"
        description="Conjuntos, qualidade, lake e modelo semântico. Ligue fontes no hub de conectores."
        actions={
          <>
            <Link href="/connectors">
              <Button variant="secondary">
                <Plug size={14} /> Ver conectores
              </Button>
            </Link>
            <label className="inline-flex min-h-10 cursor-pointer items-center rounded-xl border border-line bg-white px-3 text-sm hover:bg-bg">
              Carregar CSV / XLSX / JSON
              <input type="file" accept=".csv,.xlsx,.xls,.json,.ndjson" className="hidden" onChange={onUpload} />
            </label>
            <Button data-onboarding="demo" onClick={() => demo.mutate()} busy={demo.isPending}>
              Carregar demo
            </Button>
          </>
        }
      />

      <Card>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 className="text-[13px] font-medium text-mute">Fontes de dados</h2>
            <p className="mt-1 max-w-lg text-[13px] text-mute">
              Todos os conectores do hub sincronizam dados para um conjunto ClickHouse. Use Testar ligação e Sync em cada fonte.
            </p>
          </div>
          <Link href="/connectors">
            <Button>
              <Plug size={14} /> Abrir hub de conectores
            </Button>
          </Link>
        </div>
        {sourceList.length > 0 && (
          <div className="mt-4 space-y-2">
            {sourceList.slice(0, 6).map((s) => (
              <Link
                key={s.id}
                href={`/connectors/${s.id}`}
                className="flex items-center justify-between rounded-xl border border-line px-3 py-2 text-sm hover:bg-bg"
              >
                <span>
                  {s.name} · {s.type}
                </span>
                <Badge tone={s.preview || s.status === "preview" ? "warn" : s.status === "synced" ? "ok" : "neutral"}>
                  {s.preview ? "Preview" : statusLabel(s.status)}
                </Badge>
              </Link>
            ))}
            {sourceList.length > 6 && (
              <Link href="/connectors" className="text-[12px] text-accent hover:underline">
                Ver todas as {sourceList.length} fontes
              </Link>
            )}
          </div>
        )}
      </Card>

      {q.isLoading && <PageSkeleton cards={2} />}
      {q.isError && <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />}
      {datasetList.length === 0 && (
        <div className="space-y-4">
          <EmptyState
            icon={Database}
            title="Ainda sem conjuntos"
            description="Comece por dados. Pode carregar a demo de vendas, fazer upload de CSV/XLSX/JSON ou ligar PostgreSQL/MySQL no hub de conectores."
            action={
              <Button data-onboarding="demo" onClick={() => demo.mutate()} busy={demo.isPending}>
                Carregar demo de vendas
              </Button>
            }
          />
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <EducationalCard title="1. Demo" body="50 mil linhas de vendas prontas para explorar." />
            <EducationalCard title="2. CSV / XLSX / JSON" body="Arraste um ficheiro e a TheDobra infere o esquema." />
            <EducationalCard title="3. Conectores" body="Hub com bases de dados, APIs, SaaS e streaming." />
          </div>
        </div>
      )}
      {!!datasetList.length && (
      <TableWrap>
        <Table>
          <thead className="bg-bg">
            <tr>
              <Th>Conjunto</Th>
              <Th>Origem</Th>
              <Th>Estado</Th>
              <Th numeric>Linhas</Th>
              <Th numeric>Qualidade</Th>
            </tr>
          </thead>
          <tbody>
            {datasetList.map((d) => (
              <tr key={d.id} className="cursor-pointer border-t border-line hover:bg-bg" onClick={() => (window.location.href = `/data/${d.id}`)}>
                <Td>
                  <Link href={`/data/${d.id}`} className="font-medium text-accent hover:underline" onClick={(e) => e.stopPropagation()}>
                    {d.name}
                  </Link>
                </Td>
                <Td>
                  {d.source_name ? (
                    <Link href={`/connectors/${d.source_id}`} className="text-accent hover:underline" onClick={(e) => e.stopPropagation()}>
                      {d.source_name}
                    </Link>
                  ) : (
                    <span className="text-mute">{d.storage_mode === "import" ? "Upload / demo" : d.storage_mode || "—"}</span>
                  )}
                </Td>
                <Td>
                  <Badge tone={d.status === "ready" || d.status === "ready_ok" ? "ok" : d.status === "failed" || d.status === "error" ? "danger" : "neutral"}>
                    {statusLabel(d.status)}
                  </Badge>
                </Td>
                <Td numeric>{formatPt(d.row_count)}</Td>
                <Td numeric>{d.quality_score != null ? `${d.quality_score}/100` : "—"}</Td>
              </tr>
            ))}
          </tbody>
        </Table>
      </TableWrap>
      )}

      {lakeList.length > 0 && (
        <div className="rounded-2xl border border-line bg-surface p-5 shadow-sm">
          <h2 className="mb-3 text-[13px] text-mute">Lake (bronze / silver / gold)</h2>
          <div className="space-y-1 text-[12px] text-mute">
            {lakeList.map((o) => (
              <div key={o.id} className="flex justify-between font-mono">
                <span>
                  {o.stage} · {o.key}
                </span>
                <span>{o.bytes} B</span>
              </div>
            ))}
          </div>
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

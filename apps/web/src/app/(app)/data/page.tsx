"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import Link from "next/link";
import { toast } from "sonner";
import { Database, Plug, Trash2, Upload } from "lucide-react";
import { Badge, Button, EmptyState, ErrorState, PageHeader, PageSkeleton, Table, TableWrap, Td, Th, formatPt } from "@/components/ui";
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
type Lake = { id: string; stage: string; key: string; bytes: number; created_at: string };

export default function DataPage() {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["datasets"], queryFn: () => api<any>("/api/v1/datasets") });
  const lake = useQuery({ queryKey: ["lake"], queryFn: () => api<any>("/api/v1/lake") });
  const datasetList = normalizeArray<Dataset>(q.data);
  const lakeList = normalizeArray<Lake>(lake.data);
  const demo = useMutation({
    mutationFn: () => api("/api/v1/datasets/demo", { method: "POST" }),
    onSuccess: () => {
      toast.success("Conjunto pronto");
      q.refetch();
    },
    onError: (e: Error) => toast.error(e.message),
  });
  const removeDataset = useMutation({
    mutationFn: (id: string) => api(`/api/v1/datasets/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success("Conjunto excluído");
      qc.invalidateQueries({ queryKey: ["datasets"] });
      qc.invalidateQueries({ queryKey: ["lake"] });
      qc.invalidateQueries({ queryKey: ["lineage"] });
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
        title="Conjuntos de dados"
        description="Consulte a qualidade, explore os campos e atualize os dados de cada conjunto."
        actions={
          <>
            <Link href="/connectors">
              <Button>
                <Plug size={14} /> Conectar dados
              </Button>
            </Link>
            <label className="inline-flex min-h-10 cursor-pointer items-center gap-2 rounded-xl border border-line bg-surface px-3 text-sm font-medium text-ink shadow-sm transition hover:-translate-y-px hover:border-primary/25 hover:bg-surface-2">
              <Upload size={14} /> Importar ficheiro
              <input type="file" accept=".csv,.xlsx,.xls,.json,.ndjson" className="hidden" onChange={onUpload} />
            </label>
            <Button variant="ghost" data-onboarding="demo" onClick={() => demo.mutate()} busy={demo.isPending}>
              Usar dados de exemplo
            </Button>
          </>
        }
      />

      {q.isLoading && <PageSkeleton cards={2} />}
      {q.isError && <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />}
      {datasetList.length === 0 && !q.isLoading && !q.isError && (
        <div className="space-y-4">
          <EmptyState
            icon={Database}
            title="Ainda sem conjuntos"
            description="Ligue uma fonte, importe um ficheiro ou use os dados de exemplo para começar."
            action={
              <div className="flex flex-wrap justify-center gap-2">
                <Link href="/connectors"><Button><Plug size={14} /> Conectar dados</Button></Link>
                <Button variant="secondary" data-onboarding="demo" onClick={() => demo.mutate()} busy={demo.isPending}>
                  Usar exemplo
                </Button>
              </div>
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
              <Th> </Th>
            </tr>
          </thead>
          <tbody>
            {datasetList.map((d) => (
              <tr key={d.id} className="border-t border-line hover:bg-bg">
                <Td>
                  <Link href={`/data/${d.id}`} className="font-medium text-accent hover:underline">
                    {d.name}
                  </Link>
                </Td>
                <Td>
                  {d.source_name ? (
                    <Link href={`/connectors/${d.source_id}`} className="text-accent hover:underline">
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
                <Td>
                  <Button
                    size="sm"
                    variant="danger"
                    onClick={() => {
                      if (confirm(`Excluir o conjunto «${d.name}»? Esta acção não se desfaz.`)) {
                        removeDataset.mutate(d.id);
                      }
                    }}
                    busy={removeDataset.isPending}
                  >
                    <Trash2 size={12} /> Excluir
                  </Button>
                </Td>
              </tr>
            ))}
          </tbody>
        </Table>
      </TableWrap>
      )}

      {lakeList.length > 0 && (
        <details className="rounded-[1.125rem] border border-line bg-surface px-5 py-4 text-sm shadow-[var(--shadow-card)]">
          <summary className="cursor-pointer font-medium text-ink">Armazenamento avançado</summary>
          <p className="mb-3 mt-2 text-[12px] text-mute">Objetos ativos no data lake. Esta informação é útil para administração técnica.</p>
          <div className="max-h-48 space-y-1 overflow-auto text-[11px] text-mute">
            {lakeList.map((o) => (
              <div key={o.id} className="flex justify-between font-mono">
                <span>
                  {o.stage} · {o.key}
                </span>
                <span>{o.bytes} B</span>
              </div>
            ))}
          </div>
        </details>
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

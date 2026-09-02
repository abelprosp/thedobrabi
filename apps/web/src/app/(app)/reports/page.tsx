"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { toast } from "sonner";
import { Badge, Button, Card, EmptyState, ErrorState, PageHeader, PageSkeleton } from "@/components/ui";
import { FileText, Trash2, Edit3, Play, Clock } from "lucide-react";
import Link from "next/link";
import { useState } from "react";

type Report = { id: string; name: string; cadence: string; last_generated_at?: string; updated_at?: string };

export default function ReportsPage() {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["reports"], queryFn: () => api<any>("/api/v1/reports") });
  const reports = normalizeArray<Report>(q.data);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);

  const create = useMutation({
    mutationFn: () => api<{ id: string }>("/api/v1/reports", { method: "POST", body: JSON.stringify({ name: "Novo relatório", cadence: "weekly", pages: [{ name: "Página 1", widgets: [] }] }) }),
    onSuccess: (res) => {
      toast.success("Relatório criado");
      qc.invalidateQueries({ queryKey: ["reports"] });
      window.location.href = `/reports/${res.id}`;
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const remove = useMutation({
    mutationFn: (id: string) => api(`/api/v1/reports/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success("Relatório removido");
      setDeleteId(null);
      qc.invalidateQueries({ queryKey: ["reports"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  async function generate(id: string) {
    setBusyId(id);
    try {
      const content = await api<any>(`/api/v1/reports/${id}/generate`, { method: "POST" });
      toast.success(content.executive_summary || "Relatório gerado");
      qc.invalidateQueries({ queryKey: ["reports"] });
    } catch (e: any) {
      toast.error(e.message);
    } finally {
      setBusyId(null);
    }
  }

  const cadence: Record<string, string> = { weekly: "Semanal", daily: "Diário", monthly: "Mensal" };

  return (
    <div className="mx-auto max-w-5xl space-y-5">
      <PageHeader
        title="Relatórios"
        description="Crie relatórios multi-página com KPIs, tabelas, gráficos e texto, e exporte para PDF."
        actions={
          <Button onClick={() => create.mutate()} busy={create.isPending}>
            Novo relatório
          </Button>
        }
      />
      {q.isLoading && <PageSkeleton cards={2} />}
      {q.isError && <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />}
      {reports.length === 0 && !q.isLoading && (
        <EmptyState
          icon={FileText}
          title="Ainda sem relatórios"
          description="Os relatórios permitem agrupar widgets em páginas e partilhá-los como PDF."
          action={<Button onClick={() => create.mutate()} busy={create.isPending}>Novo relatório</Button>}
        />
      )}
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {reports.map((r) => (
          <Card key={r.id} className="flex flex-col gap-3">
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="truncate text-sm font-medium text-ink">{r.name}</div>
                <div className="mt-1.5 flex flex-wrap items-center gap-2">
                  <Badge>{cadence[r.cadence] || r.cadence}</Badge>
                  {r.last_generated_at && (
                    <span className="inline-flex items-center gap-1 text-[11px] text-mute">
                      <Clock size={12} /> {new Date(r.last_generated_at).toLocaleString("pt-BR")}
                    </span>
                  )}
                </div>
              </div>
              <div className="flex items-center gap-1">
                <Button variant="ghost" size="icon" title="Editar" onClick={() => window.location.href = `/reports/${r.id}`}>
                  <Edit3 size={16} />
                </Button>
                <Button variant="ghost" size="icon" title="Gerar" busy={busyId === r.id} onClick={() => generate(r.id)}>
                  <Play size={16} />
                </Button>
                <Button variant="ghost" size="icon" title="Remover" onClick={() => setDeleteId(r.id)}>
                  <Trash2 size={16} />
                </Button>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Link href={`/reports/${r.id}`} className="flex-1">
                <Button variant="secondary" className="w-full">Abrir editor</Button>
              </Link>
            </div>
            {deleteId === r.id && (
              <div className="rounded-xl border border-rose-100 bg-rose-50 p-3 text-[12px] text-rose-800">
                Tem a certeza? Esta ação não pode ser desfeita.
                <div className="mt-2 flex gap-2">
                  <Button variant="danger" size="sm" onClick={() => remove.mutate(r.id)} busy={remove.isPending}>Apagar</Button>
                  <Button variant="secondary" size="sm" onClick={() => setDeleteId(null)}>Cancelar</Button>
                </div>
              </div>
            )}
          </Card>
        ))}
      </div>
    </div>
  );
}

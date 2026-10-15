"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { loadActions, reviewAction } from "@/lib/manage-client";
import { Badge, Button, Card, EmptyState, ErrorState, PageHeader, PageSkeleton, Textarea } from "@/components/ui";
import { ActionDialog } from "@/components/manage/action-dialog";
import type { ActionReview, ManageAction } from "@/components/manage/types";
import { ListChecks } from "lucide-react";
import { toast } from "sonner";

export default function ActionsPage() {
  const qc = useQueryClient();
  const q = useQuery({
    queryKey: ["manage-actions"],
    queryFn: loadActions,
  });
  const items = normalizeArray<ManageAction>(q.data);
  const [open, setOpen] = useState(false);
  const [reviewing, setReviewing] = useState<string | null>(null);
  const [note, setNote] = useState("");
  const [history, setHistory] = useState<Record<string, ActionReview[]>>({});

  const review = useMutation({
    mutationFn: (id: string) => reviewAction(id, note),
    onSuccess: () => {
      toast.success("Revisão registada. A evolução do indicador ficou separada do efeito da acção.");
      setReviewing(null);
      setNote("");
      qc.invalidateQueries({ queryKey: ["manage-actions"] });
      qc.invalidateQueries({ queryKey: ["overview"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  async function openReview(id: string) {
    setReviewing(id);
    try {
      const res = await api<{ action: ManageAction; reviews: ActionReview[] }>(`/api/v1/manage/actions/${id}`);
      setHistory((h) => ({ ...h, [id]: res.reviews || [] }));
    } catch {
      /* empty */
    }
  }

  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <PageHeader
        title="Acções"
        description="Cada insight vira uma decisão com responsável, prazo e revisão. Volte para conferir o resultado — não só o movimento do indicador."
        actions={<Button onClick={() => setOpen(true)}>Nova acção</Button>}
      />
      {q.isLoading && <PageSkeleton cards={3} />}
      {q.isError && <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />}
      {items.length === 0 && !q.isLoading && !q.isError && (
        <EmptyState
          icon={ListChecks}
          title="Ainda sem acções acompanhadas"
          description="Crie uma acção a partir do início ou do radar. A TheDobra avisa quando chegar a data de revisão."
          action={<Button onClick={() => setOpen(true)}>Criar acção</Button>}
        />
      )}
      {items.map((a) => (
        <Card key={a.id}>
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="text-sm font-semibold text-ink">{a.title}</h2>
                <Badge tone={a.review_due ? "warn" : a.status === "reviewed" || a.status === "done" ? "ok" : "neutral"}>
                  {a.review_due ? "rever agora" : statusLabel(a.status)}
                </Badge>
              </div>
              {a.body && <p className="mt-1 text-[13px] text-mute">{a.body}</p>}
              <p className="mt-2 text-[12px] text-mute">
                {a.owner_name || "Sem responsável"}
                {a.due_at ? ` · prazo ${fmtDate(a.due_at)}` : ""}
                {a.review_at ? ` · revisão ${fmtDate(a.review_at)}` : ""}
                {a.measure ? ` · ${a.measure}` : ""}
              </p>
              {a.expected_result && <p className="mt-1 text-[12px] text-ink">Esperado: {a.expected_result}</p>}
            </div>
            {a.status !== "cancelled" && (
              <Button variant="secondary" size="sm" onClick={() => openReview(a.id)}>
                Rever resultado
              </Button>
            )}
          </div>
          {reviewing === a.id && (
            <div className="mt-4 space-y-3 rounded-2xl bg-bg p-4">
              <p className="text-[13px] font-medium text-ink">O que foi executado?</p>
              <Textarea value={note} onChange={(e) => setNote(e.target.value)} placeholder="Contactámos 18 de 20 clientes. Dois recusaram." />
              <div className="flex gap-2">
                <Button size="sm" onClick={() => review.mutate(a.id)} busy={review.isPending}>
                  Guardar revisão
                </Button>
                <Button size="sm" variant="ghost" onClick={() => setReviewing(null)}>
                  Cancelar
                </Button>
              </div>
              {(history[a.id] || []).map((r) => (
                <div key={r.id} className="border-t border-line pt-3 text-[13px]">
                  <p className="text-ink">{r.indicator_narrative}</p>
                  <p className="mt-1 text-mute">{r.effect_narrative}</p>
                  {r.executed_note && <p className="mt-1 text-ink">Executado: {r.executed_note}</p>}
                </div>
              ))}
            </div>
          )}
        </Card>
      ))}
      <ActionDialog open={open} onClose={() => setOpen(false)} />
    </div>
  );
}

function statusLabel(s: string) {
  return { open: "aberta", in_progress: "em curso", reviewed: "revista", done: "concluída", cancelled: "cancelada" }[s] || s;
}

function fmtDate(s: string) {
  const [y, m, d] = s.split("-");
  if (!d) return s;
  return `${d}/${m}/${y}`;
}

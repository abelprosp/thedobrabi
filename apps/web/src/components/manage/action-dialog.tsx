"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { Button, FieldLabel, Input, Select, Textarea } from "@/components/ui";
import { toast } from "sonner";
import type { Member } from "./types";

export type ActionPreset = {
  title?: string;
  body?: string;
  discoveryId?: string;
  datasetId?: string;
  measure?: string;
};

export function ActionDialog({
  open,
  onClose,
  preset,
}: {
  open: boolean;
  onClose: () => void;
  preset?: ActionPreset;
}) {
  const qc = useQueryClient();
  const members = useQuery({
    queryKey: ["members"],
    queryFn: () => api<Member[]>("/api/v1/members"),
    enabled: open,
  });
  const datasets = useQuery({
    queryKey: ["datasets"],
    queryFn: () => api<any>("/api/v1/datasets"),
    enabled: open,
  });
  const semantic = useQuery({
    queryKey: ["semantic"],
    queryFn: () => api<any>("/api/v1/semantic-models"),
    enabled: open,
  });
  const dsList = normalizeArray<{ id: string; name: string }>(datasets.data);
  const models = normalizeArray<any>(semantic.data);
  const people = normalizeArray<Member>(members.data).filter((m) => m.active !== false);

  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [ownerId, setOwnerId] = useState("");
  const [dueAt, setDueAt] = useState("");
  const [reviewAt, setReviewAt] = useState("");
  const [datasetId, setDatasetId] = useState("");
  const [measure, setMeasure] = useState("");
  const [expected, setExpected] = useState("");

  useEffect(() => {
    if (!open) return;
    setTitle(preset?.title || "");
    setBody(preset?.body || "");
    setDatasetId(preset?.datasetId || dsList[0]?.id || "");
    setMeasure(preset?.measure || "");
    setExpected("");
    const due = new Date();
    due.setDate(due.getDate() + 5);
    const review = new Date();
    review.setDate(review.getDate() + 12);
    setDueAt(due.toISOString().slice(0, 10));
    setReviewAt(review.toISOString().slice(0, 10));
  }, [open, preset?.title, preset?.body, preset?.datasetId, preset?.measure, dsList[0]?.id]);

  const measures = useMemo(() => {
    const model = models.find((m) => m.dataset_id === datasetId);
    return (model?.model?.measures || []) as { name: string }[];
  }, [models, datasetId]);

  const create = useMutation({
    mutationFn: () =>
      api("/api/v1/manage/actions", {
        method: "POST",
        body: JSON.stringify({
          discovery_id: preset?.discoveryId || "",
          dataset_id: datasetId,
          owner_id: ownerId,
          owner_name: people.find((p) => p.id === ownerId)?.name || "",
          title,
          body,
          measure,
          expected_result: expected,
          due_at: dueAt,
          review_at: reviewAt,
        }),
      }),
    onSuccess: () => {
      toast.success("Acção criada. A revisão fica agendada.");
      qc.invalidateQueries({ queryKey: ["manage-actions"] });
      qc.invalidateQueries({ queryKey: ["overview"] });
      qc.invalidateQueries({ queryKey: ["manage-brief"] });
      onClose();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 bg-ink/30 p-3 pt-[max(0.75rem,env(safe-area-inset-top))] sm:p-10" onClick={onClose}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Criar acção"
        className="card-float mx-auto max-w-lg overflow-hidden rounded-3xl p-5 shadow-[var(--shadow-float)] sm:p-6"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="text-lg font-semibold tracking-tight text-ink">Criar acção</h2>
        <p className="mt-1 text-[13px] text-mute">Registe responsável, prazo e o indicador que vai acompanhar. A TheDobra volta a pedir a revisão na data combinada.</p>
        <div className="mt-4 grid gap-3">
          <FieldLabel label="O que vai fazer" required>
            <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Contatar 20 clientes inativos até sexta-feira" />
          </FieldLabel>
          <FieldLabel label="Contexto">
            <Textarea value={body} onChange={(e) => setBody(e.target.value)} rows={3} />
          </FieldLabel>
          <div className="grid gap-3 sm:grid-cols-2">
            <FieldLabel label="Responsável">
              <Select value={ownerId} onChange={(e) => setOwnerId(e.target.value)}>
                <option value="">Eu / a definir</option>
                {people.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </Select>
            </FieldLabel>
            <FieldLabel label="Resultado esperado">
              <Input value={expected} onChange={(e) => setExpected(e.target.value)} placeholder="Quantos voltaram a comprar" />
            </FieldLabel>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <FieldLabel label="Prazo">
              <Input type="date" value={dueAt} onChange={(e) => setDueAt(e.target.value)} />
            </FieldLabel>
            <FieldLabel label="Data de revisão">
              <Input type="date" value={reviewAt} onChange={(e) => setReviewAt(e.target.value)} />
            </FieldLabel>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <FieldLabel label="Conjunto">
              <Select value={datasetId} onChange={(e) => setDatasetId(e.target.value)}>
                <option value="">Sem indicador</option>
                {dsList.map((d) => (
                  <option key={d.id} value={d.id}>
                    {d.name}
                  </option>
                ))}
              </Select>
            </FieldLabel>
            <FieldLabel label="Indicador acompanhado">
              <Select value={measure} onChange={(e) => setMeasure(e.target.value)} disabled={!datasetId}>
                <option value="">—</option>
                {measures.map((m) => (
                  <option key={m.name} value={m.name}>
                    {m.name}
                  </option>
                ))}
              </Select>
            </FieldLabel>
          </div>
        </div>
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>
            Cancelar
          </Button>
          <Button onClick={() => create.mutate()} busy={create.isPending} disabled={!title.trim()}>
            Guardar acção
          </Button>
        </div>
      </div>
    </div>
  );
}

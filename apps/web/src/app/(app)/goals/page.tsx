"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { createGoal, loadGoals } from "@/lib/manage-client";
import { Badge, Button, Card, EmptyState, ErrorState, FieldLabel, Input, PageHeader, PageSkeleton, Select } from "@/components/ui";
import { ActionDialog, type ActionPreset } from "@/components/manage/action-dialog";
import { money, type GoalPace } from "@/components/manage/types";
import { Target } from "lucide-react";
import Link from "next/link";
import { toast } from "sonner";

export default function GoalsPage() {
  const qc = useQueryClient();
  const q = useQuery({
    queryKey: ["manage-goals"],
    queryFn: loadGoals,
  });
  const datasets = useQuery({ queryKey: ["datasets"], queryFn: () => api<any>("/api/v1/datasets") });
  const semantic = useQuery({ queryKey: ["semantic"], queryFn: () => api<any>("/api/v1/semantic-models") });
  const goals = normalizeArray<GoalPace>(q.data);
  const dsList = normalizeArray<{ id: string; name: string }>(datasets.data);
  const models = normalizeArray<any>(semantic.data);
  const [open, setOpen] = useState(false);
  const [preset, setPreset] = useState<ActionPreset | undefined>();
  const [form, setForm] = useState({ name: "", dataset_id: "", metric_name: "", target_value: "", period: "monthly" });

  const measures = useMemo(() => {
    const model = models.find((m) => m.dataset_id === form.dataset_id);
    return (model?.model?.measures || []) as { name: string }[];
  }, [models, form.dataset_id]);

  const create = useMutation({
    mutationFn: () =>
      createGoal({
          ...form,
          target_value: Number(String(form.target_value).replace(",", ".")),
        }),
    onSuccess: () => {
      toast.success("Meta criada. O ritmo necessário é calculado pelo sistema.");
      qc.invalidateQueries({ queryKey: ["manage-goals"] });
      setForm({ ...form, name: "", target_value: "" });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <PageHeader
        title="Metas"
        description="Não basta a barra de progresso. A TheDobra calcula o ritmo por dia útil restante e marca hipóteses como hipóteses."
      />
      {q.isLoading && <PageSkeleton cards={2} />}
      {q.isError && <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />}
      {goals.length === 0 && !q.isLoading && !q.isError && (
        <EmptyState
          icon={Target}
          title="Ainda sem metas"
          description="Defina um alvo sobre uma métrica real do modelo. O ritmo necessário sai dos dados, não de um palpite."
        />
      )}
      {goals.map((g) => (
        <Card key={g.id}>
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-[17px] font-semibold text-ink">{g.name}</h2>
                <Badge tone={g.reached ? "ok" : g.pct < 50 ? "warn" : "accent"}>{Math.round(g.pct)}%</Badge>
              </div>
              <p className="mt-1 text-[13px] text-mute">
                {g.metric_name} · {money(g.current_value, g.currency)} de {money(g.target_value, g.currency)} · {g.remaining_business_days} dias úteis
              </p>
            </div>
          </div>
          <div className="mt-3 h-2 overflow-hidden rounded-full bg-surface-2">
            <div className="h-full rounded-full bg-primary" style={{ width: `${Math.min(100, Math.max(0, g.pct))}%` }} />
          </div>
          <p className="mt-3 text-sm text-ink">{g.narrative}</p>
          {g.hypothesis && <p className="mt-1 text-[13px] italic text-mute">{g.hypothesis}</p>}
          <div className="mt-4 flex flex-wrap gap-2">
            <Link href={`/ask?q=${encodeURIComponent("Do que é composta a métrica " + g.metric_name + "?")}`}>
              <Button variant="secondary" size="sm">
                Ver composição
              </Button>
            </Link>
            <Button
              size="sm"
              onClick={() => {
                setPreset({
                  title: `Acelerar ${g.name}`,
                  body: g.narrative,
                  datasetId: g.dataset_id,
                  measure: g.metric_name,
                });
                setOpen(true);
              }}
            >
              Definir acção
            </Button>
            <Link href="/actions">
              <Button variant="ghost" size="sm">
                Acompanhar meta
              </Button>
            </Link>
          </div>
        </Card>
      ))}

      <Card>
        <h2 className="text-[17px] font-semibold text-ink">Nova meta</h2>
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          <FieldLabel label="Nome">
            <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="Receita do mês" />
          </FieldLabel>
          <FieldLabel label="Alvo">
            <Input value={form.target_value} onChange={(e) => setForm({ ...form, target_value: e.target.value })} placeholder="100000" />
          </FieldLabel>
          <FieldLabel label="Conjunto">
            <Select value={form.dataset_id} onChange={(e) => setForm({ ...form, dataset_id: e.target.value, metric_name: "" })}>
              <option value="">Escolher</option>
              {dsList.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                </option>
              ))}
            </Select>
          </FieldLabel>
          <FieldLabel label="Métrica">
            <Select value={form.metric_name} onChange={(e) => setForm({ ...form, metric_name: e.target.value })}>
              <option value="">Escolher</option>
              {measures.map((m) => (
                <option key={m.name} value={m.name}>
                  {m.name}
                </option>
              ))}
            </Select>
          </FieldLabel>
          <FieldLabel label="Período">
            <Select value={form.period} onChange={(e) => setForm({ ...form, period: e.target.value })}>
              <option value="weekly">Semanal</option>
              <option value="monthly">Mensal</option>
              <option value="quarterly">Trimestral</option>
              <option value="yearly">Anual</option>
            </Select>
          </FieldLabel>
        </div>
        <Button className="mt-4" onClick={() => create.mutate()} busy={create.isPending} disabled={!form.name || !form.dataset_id || !form.metric_name || !form.target_value}>
          Criar meta
        </Button>
      </Card>
      <ActionDialog open={open} onClose={() => setOpen(false)} preset={preset} />
    </div>
  );
}

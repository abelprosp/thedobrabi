"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { Button, Card, ErrorState, Input, PageHeader, PageSkeleton, Select, Textarea } from "@/components/ui";
import { copyWithToast } from "@/lib/clipboard";
import type { Discovery, ManageAction, Member, WeeklyReview } from "@/components/manage/types";
import { toast } from "sonner";

const STEPS = [
  "Conferir os principais resultados",
  "Revisar acções anteriores",
  "Escolher até três prioridades",
  "Atribuir responsáveis",
  "Agendar a próxima revisão",
];

export default function ReviewPage() {
  const qc = useQueryClient();
  const home = useQuery({ queryKey: ["overview"], queryFn: () => api<any>("/api/v1/overview") });
  const actions = useQuery({ queryKey: ["manage-actions"], queryFn: () => api<ManageAction[]>("/api/v1/manage/actions") });
  const radar = useQuery({ queryKey: ["manage-radar"], queryFn: () => api<Discovery[]>("/api/v1/manage/radar") });
  const members = useQuery({ queryKey: ["members"], queryFn: () => api<Member[]>("/api/v1/members") });
  const past = useQuery({ queryKey: ["manage-reviews"], queryFn: () => api<WeeklyReview[]>("/api/v1/manage/reviews") });
  const people = normalizeArray<Member>(members.data);
  const discoveries = normalizeArray<Discovery>(radar.data).filter((d) => d.kind !== "stable").slice(0, 5);
  const prior = normalizeArray<ManageAction>(actions.data).slice(0, 8);
  const reviews = normalizeArray<WeeklyReview>(past.data);

  const [step, setStep] = useState(0);
  const [notes, setNotes] = useState("");
  const [nextAt, setNextAt] = useState(() => {
    const d = new Date();
    d.setDate(d.getDate() + 7);
    return d.toISOString().slice(0, 10);
  });
  const [picked, setPicked] = useState<{ title: string; owner_id: string; owner_name: string }[]>([
    { title: "", owner_id: "", owner_name: "" },
    { title: "", owner_id: "", owner_name: "" },
    { title: "", owner_id: "", owner_name: "" },
  ]);
  const [done, setDone] = useState<WeeklyReview | null>(null);

  const results = useMemo(
    () =>
      (home.data?.what_changed || discoveries).slice(0, 5).map((d: Discovery) => ({
        title: d.title,
        body: d.body,
      })),
    [home.data, discoveries],
  );

  const save = useMutation({
    mutationFn: () =>
      api<WeeklyReview>("/api/v1/manage/reviews", {
        method: "POST",
        body: JSON.stringify({
          results,
          prior_actions: prior.map((a) => ({ id: a.id, title: a.title, status: a.status })),
          priorities: picked.filter((p) => p.title.trim()),
          notes,
          next_review_at: nextAt,
        }),
      }),
    onSuccess: (rev) => {
      setDone(rev);
      qc.invalidateQueries({ queryKey: ["manage-reviews"] });
      qc.invalidateQueries({ queryKey: ["manage-actions"] });
      qc.invalidateQueries({ queryKey: ["overview"] });
      toast.success("Revisão da semana concluída");
    },
    onError: (e: Error) => toast.error(e.message),
  });

  if (home.isLoading) return <PageSkeleton />;
  if (home.isError) return <ErrorState message={(home.error as Error).message} onRetry={() => home.refetch()} />;

  if (done) {
    const url = typeof window !== "undefined" ? `${window.location.origin}${done.share_url}` : done.share_url;
    return (
      <div className="mx-auto max-w-3xl space-y-4">
        <PageHeader title="Pauta pronta" description="Partilhe com a equipa. As decisões, responsáveis e a próxima revisão ficaram registados." />
        <Card>
          <p className="text-sm text-ink">Período {done.period}</p>
          <ul className="mt-3 space-y-2 text-sm">
            {(done.agenda?.priorities || []).map((p) => (
              <li key={p.title}>
                <span className="font-medium">{p.title}</span>
                {p.owner_name ? ` · ${p.owner_name}` : ""}
              </li>
            ))}
          </ul>
          <p className="mt-3 text-[13px] text-mute">Próxima revisão: {done.next_review_at}</p>
          <div className="mt-4 flex flex-wrap gap-2">
            <Button onClick={() => copyWithToast(url || "", "Ligação da pauta copiada")}>Copiar ligação</Button>
            <a href={done.share_url} target="_blank" rel="noreferrer">
              <Button variant="secondary">Abrir pauta</Button>
            </a>
            <Button variant="ghost" onClick={() => setDone(null)}>
              Nova revisão
            </Button>
          </div>
        </Card>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl space-y-4">
      <PageHeader
        title="Revisão da semana"
        description="Uma sequência curta: resultados, acções, três prioridades, responsáveis e a próxima data."
      />
      <ol className="flex flex-wrap gap-2 text-[11px]">
        {STEPS.map((label, i) => (
          <li key={label} className={`rounded-full px-3 py-1 ${i === step ? "bg-primary text-white" : "bg-surface-2 text-mute"}`}>
            {i + 1}. {label}
          </li>
        ))}
      </ol>

      <Card>
        {step === 0 && (
          <div className="space-y-3">
            <h2 className="text-[17px] font-semibold text-ink">Principais resultados</h2>
            {results.length === 0 && <p className="text-sm text-mute">Sem alterações materiais neste recorte.</p>}
            {results.map((r: { title: string; body: string }) => (
              <div key={r.title} className="border-t border-line py-2 first:border-0 first:pt-0">
                <div className="text-sm font-medium text-ink">{r.title}</div>
                <p className="text-[13px] text-mute">{r.body}</p>
              </div>
            ))}
          </div>
        )}
        {step === 1 && (
          <div className="space-y-3">
            <h2 className="text-[17px] font-semibold text-ink">Acções anteriores</h2>
            {prior.length === 0 && <p className="text-sm text-mute">Ainda não há acções registadas.</p>}
            {prior.map((a) => (
              <div key={a.id} className="flex items-start justify-between gap-3 border-t border-line py-2 first:border-0 first:pt-0">
                <div>
                  <div className="text-sm text-ink">{a.title}</div>
                  <p className="text-[12px] text-mute">{a.owner_name || "—"} · {a.status}{a.review_due ? " · rever" : ""}</p>
                </div>
              </div>
            ))}
          </div>
        )}
        {step === 2 && (
          <div className="space-y-3">
            <h2 className="text-[17px] font-semibold text-ink">Até três prioridades</h2>
            {picked.map((p, i) => (
              <Input
                key={i}
                value={p.title}
                placeholder={`Prioridade ${i + 1}`}
                onChange={(e) => {
                  const next = [...picked];
                  next[i] = { ...next[i], title: e.target.value };
                  setPicked(next);
                }}
              />
            ))}
            <div className="flex flex-wrap gap-2">
              {discoveries.slice(0, 4).map((d) => (
                <button
                  key={d.id}
                  type="button"
                  className="rounded-full bg-surface-2 px-3 py-1 text-[12px] text-ink"
                  onClick={() => {
                    const idx = picked.findIndex((x) => !x.title);
                    if (idx < 0) return;
                    const next = [...picked];
                    next[idx] = { ...next[idx], title: d.next_step || d.title };
                    setPicked(next);
                  }}
                >
                  Usar: {d.title}
                </button>
              ))}
            </div>
          </div>
        )}
        {step === 3 && (
          <div className="space-y-3">
            <h2 className="text-[17px] font-semibold text-ink">Responsáveis</h2>
            {picked.filter((p) => p.title.trim()).length === 0 && <p className="text-sm text-mute">Defina prioridades no passo anterior.</p>}
            {picked.map((p, i) =>
              p.title.trim() ? (
                <label key={i} className="block text-[13px]">
                  <span className="text-ink">{p.title}</span>
                  <Select
                    className="mt-1"
                    value={p.owner_id}
                    onChange={(e) => {
                      const id = e.target.value;
                      const next = [...picked];
                      next[i] = { ...next[i], owner_id: id, owner_name: people.find((m) => m.id === id)?.name || "" };
                      setPicked(next);
                    }}
                  >
                    <option value="">Sem responsável</option>
                    {people.map((m) => (
                      <option key={m.id} value={m.id}>
                        {m.name}
                      </option>
                    ))}
                  </Select>
                </label>
              ) : null,
            )}
          </div>
        )}
        {step === 4 && (
          <div className="space-y-3">
            <h2 className="text-[17px] font-semibold text-ink">Próxima revisão</h2>
            <Input type="date" value={nextAt} onChange={(e) => setNextAt(e.target.value)} />
            <Textarea value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Notas para a pauta (opcional)" />
          </div>
        )}
        <div className="mt-5 flex justify-between">
          <Button variant="ghost" onClick={() => setStep((s) => Math.max(0, s - 1))} disabled={step === 0}>
            Voltar
          </Button>
          {step < 4 ? (
            <Button onClick={() => setStep((s) => s + 1)}>Continuar</Button>
          ) : (
            <Button onClick={() => save.mutate()} busy={save.isPending}>
              Fechar revisão e gerar pauta
            </Button>
          )}
        </div>
      </Card>

      {reviews.length > 0 && (
        <Card>
          <h2 className="text-sm font-semibold text-ink">Revisões anteriores</h2>
          <ul className="mt-2 space-y-2 text-[13px]">
            {reviews.map((r) => (
              <li key={r.id} className="flex items-center justify-between gap-2">
                <span>
                  {r.period}
                  {r.agenda?.priorities?.[0]?.title ? ` · ${r.agenda.priorities[0].title}` : ""}
                </span>
                {r.share_url && (
                  <a className="text-primary hover:underline" href={r.share_url} target="_blank" rel="noreferrer">
                    Pauta
                  </a>
                )}
              </li>
            ))}
          </ul>
        </Card>
      )}
    </div>
  );
}

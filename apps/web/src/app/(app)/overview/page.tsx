"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { Button, ErrorState, PageSkeleton, Select, cn } from "@/components/ui";
import { OnboardingChecklist } from "@/components/onboarding";
import { ActionDialog, type ActionPreset } from "@/components/manage/action-dialog";
import { cadenceLabel, type Discovery, type Home } from "@/components/manage/types";
import Link from "next/link";
import { ArrowUpRight, CalendarCheck, ListChecks, Search, Sparkles } from "lucide-react";

const CYAN = "#0ea5e9";

export default function OverviewPage() {
  const qc = useQueryClient();
  const q = useQuery({
    queryKey: ["overview"],
    queryFn: () => api<Home>("/api/v1/overview"),
  });
  const demo = useMutation({
    mutationFn: () => api("/api/v1/datasets/demo", { method: "POST" }),
    onSuccess: () => {
      toast.success("Conjunto de vendas de demonstração ingerido");
      q.refetch();
    },
    onError: (e: Error) => toast.error(e.message),
  });
  const cadence = useMutation({
    mutationFn: (value: string) =>
      api("/api/v1/manage/cadence", {
        method: "PATCH",
        body: JSON.stringify({ cadence: value, watch_measures: q.data?.watch_measures || [] }),
      }),
    onSuccess: () => {
      toast.success("Frequência actualizada");
      qc.invalidateQueries({ queryKey: ["overview"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });
  const [preset, setPreset] = useState<ActionPreset | undefined>();
  const [open, setOpen] = useState(false);
  const home = q.data;
  const brief = home?.brief;
  const whatChanged =
    home?.what_changed?.length
      ? home.what_changed
      : (brief?.major_changes || []).map((i, idx) => ({
          id: `change-${idx}`,
          kind: i.kind || "change",
          title: i.title,
          body: i.body,
          next_step: "Abrir a análise e decidir se vale uma acção",
          impact_score: 0,
          fingerprint: "",
          feedback: "",
          watching: false,
        }));
  const attention =
    home?.attention?.length
      ? home.attention
      : [...(brief?.risks || []), ...(brief?.opportunities || [])].map((i, idx) => ({
          id: `att-${idx}`,
          kind: i.kind || "risk",
          title: i.title,
          body: i.body,
          next_step: "",
          impact_score: 0,
          fingerprint: "",
          feedback: "",
          watching: false,
        }));

  if (q.isLoading) return <PageSkeleton />;
  if (q.isError) return <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />;

  function openAction(d?: Discovery) {
    setPreset({
      title: d?.next_step || d?.title,
      body: d ? `${d.title}. ${d.body}` : brief?.headline,
      discoveryId: d?.id,
      datasetId: d?.dataset_id || brief?.dataset_id,
      measure: typeof d?.evidence?.metric === "string" ? d.evidence.metric : undefined,
    });
    setOpen(true);
  }

  return (
    <div className="mx-auto max-w-6xl space-y-6 pb-8">
      <header className="flex flex-col gap-4 pt-2 sm:flex-row sm:items-end sm:justify-between">
        <div className="max-w-2xl">
          <p className="text-[12px] font-medium text-mute">Seu negócio em 2 minutos</p>
          <h1 className="mt-1 text-3xl font-bold tracking-[-0.035em] text-ink sm:text-[2.6rem] sm:leading-[1.05]">
            {brief?.headline || "Toda semana, saiba o que mudou, decida o que fazer e acompanhe o resultado."}
          </h1>
          <p className="mt-2 max-w-xl text-sm leading-relaxed text-mute">
            O que mudou, o que merece atenção e qual acção tomar — no ritmo em que os seus dados actualizam.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Select
            value={home?.cadence_mode || "auto"}
            onChange={(e) => cadence.mutate(e.target.value)}
            aria-label="Frequência do resumo"
            className="w-auto min-w-[10rem]"
          >
            <option value="auto">Frequência automática</option>
            <option value="daily">Diária</option>
            <option value="weekly">Semanal</option>
            <option value="monthly">Mensal</option>
          </Select>
          {(!home?.datasets || home.datasets === 0) && (
            <Button onClick={() => demo.mutate()} busy={demo.isPending} className="shrink-0 rounded-full px-5">
              {demo.isPending ? "A preparar…" : "Adicionar dados de demonstração"}
            </Button>
          )}
        </div>
      </header>

      <p className="text-[12px] text-mute">
        Resumo {cadenceLabel(home?.cadence || "weekly").toLowerCase()}
        {home?.suggested_cadence && home.cadence_mode === "auto" ? ` · sugerida pela actualização dos dados (${cadenceLabel(home.suggested_cadence).toLowerCase()})` : ""}
        {home?.data_updated_at ? ` · dados de ${new Date(home.data_updated_at).toLocaleDateString("pt-BR")}` : ""}
      </p>

      {(home?.week_review_ready || (home?.due_reviews && home.due_reviews.length > 0)) && (
        <div className="grid gap-3 md:grid-cols-2">
          {home.week_review_ready && (
            <Link href="/review" className="card-float flex items-center gap-3 p-4 hover:ring-1 hover:ring-primary/30">
              <CalendarCheck size={18} className="text-primary" />
              <div>
                <div className="text-sm font-medium text-ink">A sua revisão semanal está pronta</div>
                <p className="text-[12px] text-mute">Abra o resumo, escolha até três prioridades e partilhe a pauta.</p>
              </div>
            </Link>
          )}
          {(home.due_reviews?.length ?? 0) > 0 && (
            <Link href="/actions" className="card-float flex items-center gap-3 p-4 hover:ring-1 hover:ring-primary/30">
              <ListChecks size={18} className="text-primary" />
              <div>
                <div className="text-sm font-medium text-ink">Chegou o momento de conferir o resultado da sua acção</div>
                <p className="text-[12px] text-mute">{home.due_reviews?.[0]?.title}</p>
              </div>
            </Link>
          )}
        </div>
      )}

      <section className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
        <div className="grid grid-cols-3 gap-6 sm:gap-12">
          <Stat label="O que mudou" value={whatChanged.length} />
          <Stat label="Atenção" value={attention.length} />
          <Stat label="Acções abertas" value={home?.open_actions || 0} />
        </div>
        <AskPill />
      </section>

      <OnboardingChecklist />

      {home?.quiet && (
        <div className="rounded-3xl bg-surface px-5 py-4 text-sm text-mute ring-1 ring-line/70">
          {home.quiet_message}
        </div>
      )}

      <section className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <Panel eyebrow="O que mudou" title="Alterações com evidência" href="/radar" className="lg:col-span-2">
          {whatChanged.length === 0 ? (
            <p className="text-sm text-mute">Ligue dados para gerar o briefing.</p>
          ) : (
            <ol className="relative space-y-4 border-l border-dashed border-line pl-5">
              {whatChanged.map((i) => (
                <li key={i.id} className="relative">
                  <span className="absolute -left-[26px] top-1 h-3 w-3 rounded-full ring-4 ring-surface" style={{ background: i.kind === "risk" ? "#e2552b" : CYAN }} />
                  <div className="text-sm font-medium text-ink">{i.title}</div>
                  <div className="mt-0.5 text-[12px] leading-relaxed text-mute">{i.body}</div>
                  <div className="mt-2 flex flex-wrap gap-2">
                    <Link href="/radar" className="text-[12px] font-medium text-primary hover:underline">
                      Ver análise
                    </Link>
                    <button type="button" className="text-[12px] font-medium text-ink hover:underline" onClick={() => openAction(i)}>
                      Criar acção
                    </button>
                  </div>
                </li>
              ))}
            </ol>
          )}
        </Panel>

        <Panel eyebrow="O que merece atenção" title="Prioridades" href="/radar">
          {attention.slice(0, 4).map((i) => (
            <div key={i.id} className="border-t border-line py-2.5 first:border-0 first:pt-0">
              <div className="text-sm text-ink">{i.title}</div>
              <div className="mt-0.5 line-clamp-2 text-[12px] text-mute">{i.body}</div>
            </div>
          ))}
          {attention.length === 0 && <p className="text-sm text-mute">Nada a destacar neste recorte.</p>}
        </Panel>
      </section>

      <Panel title="Qual acção posso tomar?">
        {home?.action_now ? (
          <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <p className="text-sm font-medium text-ink">{home.action_now.title}</p>
              <p className="mt-1 text-[13px] leading-relaxed text-mute">{home.action_now.body}</p>
              <p className="mt-2 text-[12px] text-ink">Próximo passo: {home.action_now.next_step}</p>
            </div>
            <div className="flex shrink-0 flex-wrap gap-2">
              <Link href="/radar">
                <Button variant="secondary">Ver análise</Button>
              </Link>
              <Button onClick={() => openAction(home.action_now)}>Criar acção</Button>
            </div>
          </div>
        ) : (
          <p className="text-sm text-mute">{home?.quiet_message || "Sem acção recomendada por agora."}</p>
        )}
      </Panel>

      {home?.past_decisions && home.past_decisions.length > 0 && (
        <Panel title="O que decidimos quando esse problema apareceu antes?">
          <ul className="space-y-2">
            {home.past_decisions.map((d) => (
              <li key={d.period + d.title} className="text-sm text-ink">
                <span className="text-mute">{d.when} · {d.period}</span>
                {" — "}
                {d.title}
                {d.owner_name ? ` · ${d.owner_name}` : ""}
              </li>
            ))}
          </ul>
        </Panel>
      )}

      <MemoryBox onSaved={() => q.refetch()} />

      <ActionDialog open={open} onClose={() => setOpen(false)} preset={preset} />
    </div>
  );
}

function MemoryBox({ onSaved }: { onSaved: () => void }) {
  const [note, setNote] = useState("");
  const [occurred, setOccurred] = useState(() => new Date().toISOString().slice(0, 10));
  const save = useMutation({
    mutationFn: () =>
      api("/api/v1/manage/memories", {
        method: "POST",
        body: JSON.stringify({ note, occurred_on: occurred }),
      }),
    onSuccess: () => {
      toast.success("Memória registada. Vai aparecer nas comparações e revisões.");
      setNote("");
      onSaved();
    },
    onError: (e: Error) => toast.error(e.message),
  });
  const memories = useQuery({
    queryKey: ["manage-memories"],
    queryFn: () => api<any[]>("/api/v1/manage/memories"),
  });
  return (
    <Panel title="Memória de gestão">
      <p className="mb-3 text-[13px] text-mute">Anote o que os gráficos não mostram: promoção, loja fechada, venda excepcional.</p>
      <div className="flex flex-col gap-2 sm:flex-row">
        <input
          value={note}
          onChange={(e) => setNote(e.target.value)}
          placeholder="Fizemos uma promoção nesta semana."
          className="min-h-11 flex-1 rounded-2xl bg-surface-2 px-4 text-sm text-ink outline-none"
        />
        <input type="date" value={occurred} onChange={(e) => setOccurred(e.target.value)} className="min-h-11 rounded-2xl bg-surface-2 px-3 text-sm text-ink" />
        <Button onClick={() => save.mutate()} busy={save.isPending} disabled={!note.trim()}>
          Registar
        </Button>
      </div>
      {(memories.data || []).slice(0, 5).length > 0 && (
        <ul className="mt-4 space-y-1.5 text-[13px] text-mute">
          {(memories.data || []).slice(0, 5).map((m: any) => (
            <li key={m.id}>
              <span className="text-ink">{m.occurred_on}</span> — {m.note}
            </li>
          ))}
        </ul>
      )}
    </Panel>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <div className="text-[13px] text-mute">{label}</div>
      <div className="mt-1 text-2xl font-bold tracking-tight text-ink sm:text-[1.7rem]">{value}</div>
    </div>
  );
}

function AskPill() {
  const router = useRouter();
  const [text, setText] = useState("");
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        const query = text.trim();
        router.push(query ? `/ask?q=${encodeURIComponent(query)}` : "/ask");
      }}
      className="flex h-11 w-full items-center gap-2 rounded-full bg-surface px-4 shadow-[var(--shadow-card)] ring-1 ring-line/60 lg:w-80"
    >
      <Search size={15} className="shrink-0 text-mute" />
      <input
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder="Pergunte sobre os dados…"
        aria-label="Perguntar aos dados"
        className="min-w-0 flex-1 bg-transparent text-[13px] text-ink outline-none placeholder:text-mute"
      />
      <button type="submit" aria-label="Perguntar aos dados" className="text-ink">
        <Sparkles size={15} />
      </button>
    </form>
  );
}

function Panel({
  eyebrow,
  title,
  href,
  className,
  children,
}: {
  eyebrow?: string;
  title: string;
  href?: string;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <section className={cn("card-float p-5 sm:p-6", className)}>
      <div className="mb-4 flex items-start justify-between gap-3">
        <div>
          {eyebrow && <p className="text-[11px] text-mute">{eyebrow}</p>}
          <h2 className="mt-0.5 text-[17px] font-semibold tracking-tight text-ink">{title}</h2>
        </div>
        {href && (
          <Link href={href} aria-label={title} className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-line text-ink transition hover:bg-bg">
            <ArrowUpRight size={15} />
          </Link>
        )}
      </div>
      {children}
    </section>
  );
}

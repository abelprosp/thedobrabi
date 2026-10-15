"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useMutation, useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { Button, ErrorState, PageSkeleton, cn } from "@/components/ui";
import { OnboardingChecklist } from "@/components/onboarding";
import Link from "next/link";
import { ArrowUpRight, LayoutDashboard, MessageSquare, Plug, Search, Sparkles, Store } from "lucide-react";

type Brief = {
  headline: string;
  major_changes: Item[];
  risks: Item[];
  opportunities: Item[];
  recommended_actions: string[];
};
type Item = { title: string; body: string; kind: string; severity: string };

// Cores da marca: ciano para oportunidades, índigo para pontos de atenção.
const CYAN = "#0ea5e9";
const INDIGO = "#4f46e5";

const QUICK_ACTIONS = [
  { href: "/dashboards", icon: LayoutDashboard, title: "Criar dashboard", description: "Monte um painel para acompanhar o que importa." },
  { href: "/ask", icon: MessageSquare, title: "Perguntar aos dados", description: "Encontre respostas usando os seus números." },
  { href: "/connectors", icon: Plug, title: "Adicionar dados", description: "Ligue bases, ficheiros, APIs ou ERPs." },
  { href: "/store", icon: Store, title: "Explorar modelos", description: "Comece com um painel pronto para adaptar." },
];

export default function OverviewPage() {
  const q = useQuery({
    queryKey: ["overview"],
    queryFn: () => api<{ datasets: number; brief?: Brief }>("/api/v1/overview"),
  });
  const demo = useMutation({
    mutationFn: () => api("/api/v1/datasets/demo", { method: "POST" }),
    onSuccess: () => {
      toast.success("Conjunto de vendas de demonstração ingerido");
      q.refetch();
    },
    onError: (e: Error) => toast.error(e.message),
  });
  const brief = q.data?.brief;

  if (q.isLoading) return <PageSkeleton />;
  if (q.isError) return <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />;

  const changes = brief?.major_changes || [];
  const risks = brief?.risks || [];
  const opportunities = brief?.opportunities || [];
  const actions = brief?.recommended_actions || [];

  return (
    <div className="mx-auto max-w-6xl space-y-6 pb-8">
      <header className="flex flex-col gap-4 pt-2 sm:flex-row sm:items-end sm:justify-between">
        <div className="max-w-2xl">
          <p className="text-[12px] font-medium text-mute">Visão geral</p>
          <h1 className="mt-1 text-3xl font-bold tracking-[-0.035em] text-ink sm:text-[2.6rem] sm:leading-[1.05]">
            {brief?.headline || "Seu negócio em um só lugar"}
          </h1>
          <p className="mt-2 max-w-xl text-sm leading-relaxed text-mute">Acompanhe os principais números, entenda o que mudou e escolha o próximo passo.</p>
        </div>
        {(!q.data?.datasets || q.data.datasets === 0) && (
          <Button onClick={() => demo.mutate()} busy={demo.isPending} className="shrink-0 rounded-full px-5">
            {demo.isPending ? "A preparar…" : "Adicionar dados de demonstração"}
          </Button>
        )}
      </header>

      <section className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
        <div className="grid grid-cols-3 gap-6 sm:gap-12">
          <Stat label="Alterações" value={changes.length} />
          <Stat label="Pontos de atenção" value={risks.length} />
          <Stat label="Oportunidades" value={opportunities.length} />
        </div>
        <AskPill />
      </section>

      <OnboardingChecklist />

      <section className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <Panel eyebrow="Alterações" title="O que aconteceu" href="/insights" className="lg:col-span-2">
          <div className="grid gap-5 md:grid-cols-[minmax(0,1fr)_260px]">
            <div>
              {changes.length === 0 ? (
                <p className="text-sm text-mute">Ligue dados para gerar o briefing.</p>
              ) : (
                <ol className="relative space-y-4 border-l border-dashed border-line pl-5">
                  {changes.map((i) => (
                    <li key={i.title} className="relative">
                      <span
                        className="absolute -left-[26px] top-1 h-3 w-3 rounded-full ring-4 ring-surface"
                        style={{ background: severityColor(i.severity) }}
                      />
                      <div className="text-sm font-medium text-ink">{i.title}</div>
                      <div className="mt-0.5 text-[12px] leading-relaxed text-mute">{i.body}</div>
                    </li>
                  ))}
                </ol>
              )}
            </div>
            <div className="rounded-3xl bg-surface p-4 shadow-[var(--shadow-float)] ring-1 ring-line/70">
              <p className="text-[11px] text-mute">Acesso rápido</p>
              <h3 className="mt-0.5 text-[17px] font-semibold tracking-tight text-ink">O que quer fazer agora?</h3>
              <div className="mt-3 space-y-1.5">
                {QUICK_ACTIONS.map((a) => (
                  <Link
                    key={a.href}
                    href={a.href}
                    title={a.description}
                    className="group flex items-center gap-2.5 rounded-full bg-surface-2 px-3 py-1.5 text-[12px] text-ink transition hover:bg-primary/10 hover:text-primary-600"
                  >
                    <a.icon size={13} className="text-mute group-hover:text-ink" />
                    <span className="truncate">{a.title}</span>
                    <ArrowUpRight size={12} className="ml-auto text-mute opacity-0 transition group-hover:opacity-100" />
                  </Link>
                ))}
              </div>
            </div>
          </div>
        </Panel>

        <Panel eyebrow="Pontos de atenção · Oportunidades" title="Porque importa" href="/insights">
          <ImpactBars risks={risks.length} opportunities={opportunities.length} />
          <div className="mt-4">
            {risks.concat(opportunities).slice(0, 4).map((i) => (
              <div key={i.title} className="border-t border-line py-2.5 first:border-0 first:pt-0">
                <div className="text-sm text-ink">{i.title}</div>
                <div className="mt-0.5 line-clamp-2 text-[12px] text-mute">{i.body}</div>
              </div>
            ))}
            {!risks.length && !opportunities.length && <p className="text-sm text-mute">Ainda sem riscos nem oportunidades.</p>}
          </div>
        </Panel>
      </section>

      <Panel title="O que deve fazer" href="/ask">
        {actions.length === 0 ? (
          <p className="text-sm text-mute">Sem acções recomendadas por agora.</p>
        ) : (
          <ol className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {actions.map((a, idx) => (
              <li key={a} className="flex gap-3 rounded-2xl bg-bg p-4">
                <span
                  className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-[12px] font-semibold"
                  style={{ background: idx === 0 ? CYAN : INDIGO, color: "#fff" }}
                >
                  {idx + 1}
                </span>
                <span className="text-sm leading-relaxed text-ink">{a}</span>
              </li>
            ))}
          </ol>
        )}
      </Panel>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <div className="text-[13px] text-mute">{label}</div>
      <div className="mt-1 flex items-center gap-2">
        <span className="text-2xl font-bold tracking-tight text-ink sm:text-[1.7rem]">{value}</span>
      </div>
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
          <Link
            href={href}
            aria-label={title}
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-line text-ink transition hover:bg-bg"
          >
            <ArrowUpRight size={15} />
          </Link>
        )}
      </div>
      {children}
    </section>
  );
}

function ImpactBars({ risks, opportunities }: { risks: number; opportunities: number }) {
  const total = risks + opportunities;
  const bars = [
    { label: "Oportunidades", value: opportunities, color: CYAN },
    { label: "Pontos de atenção", value: risks, color: INDIGO },
  ];
  return (
    <div>
      <div className="mb-3 text-right text-5xl font-bold tracking-[-0.04em] text-ink">{total}</div>
      <div className="flex h-36 items-end gap-3">
        {bars.map((b) => {
          const pct = total ? b.value / total : 0;
          return (
            <div key={b.label} className="relative flex h-full flex-1 flex-col justify-end">
              <span className="absolute left-2 top-0 text-[12px] text-mute">{b.label}</span>
              <div className="absolute bottom-0 left-0 w-[3px] rounded-full" style={{ height: `${Math.max(18, pct * 100)}%`, background: b.color }} />
              <div
                className="ml-[3px] rounded-2xl transition-all"
                style={{
                  height: `${Math.max(10, pct * 72)}%`,
                  background: `linear-gradient(180deg, ${b.color}, ${b.color}cc)`,
                  boxShadow: `0 10px 30px ${b.color}55`,
                }}
              />
            </div>
          );
        })}
      </div>
      <div className="mt-3 flex justify-around text-[12px] text-ink">
        {bars.map((b) => (
          <span key={b.label} className="flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full" style={{ background: b.color }} />
            {total ? Math.round((b.value / total) * 100) : 0}%
          </span>
        ))}
      </div>
    </div>
  );
}

function severityColor(severity: string) {
  if (/high|critical|alta/i.test(severity)) return "#e2552b";
  if (/medium|warn|média/i.test(severity)) return CYAN;
  return INDIGO;
}

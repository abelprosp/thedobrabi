"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMutation, useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Button, Card } from "@/components/ui";
import { useOnboarding, type OnboardingStep } from "@/components/onboarding";
import { toast } from "sonner";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  CheckCircle2,
  Database,
  LayoutDashboard,
  MessageSquare,
  Plug,
  Sparkles,
  Store,
  Upload,
  Users,
} from "lucide-react";

type WizardStep = Exclude<OnboardingStep, "done">;

const STEP_META: Record<WizardStep, { label: string; title: string; description: string; icon: typeof Sparkles }> = {
  welcome: {
    label: "Início",
    title: "Bem-vindo à TheDobra",
    description: "Vamos deixar tudo pronto em poucos minutos. Cada passo é opcional e pode voltar a ele depois na página Início.",
    icon: Sparkles,
  },
  data: {
    label: "Dados",
    title: "De onde vêm os seus dados?",
    description: "Comece pela demo para ver a plataforma a funcionar, ou traga já os dados reais.",
    icon: Database,
  },
  dashboard: {
    label: "Painel",
    title: "O seu primeiro painel",
    description: "A IA monta um painel executivo com os seus indicadores, ou escolha um modelo pronto para o seu setor.",
    icon: LayoutDashboard,
  },
  ask: {
    label: "IA",
    title: "Pergunte aos seus dados",
    description: "Escreva como falaria com um analista. A resposta usa as métricas oficiais do modelo.",
    icon: MessageSquare,
  },
  team: {
    label: "Equipa",
    title: "Convide a equipa",
    description: "Partilhe os painéis com quem acompanha os números. Pode fazer isto mais tarde nas Definições.",
    icon: Users,
  },
};

const suggestedQuestions = ["Quanto vendi este mês?", "Quais são os produtos mais vendidos?", "Comparar vendas por região."];

export default function WelcomePage() {
  const router = useRouter();
  const ob = useOnboarding();
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<{ name?: string; role?: string }>("/api/v1/auth/me") });
  const role = me.data?.role ?? "viewer";
  const canEdit = role !== "viewer";
  const isAdmin = role === "owner" || role === "admin";

  const steps = useMemo<WizardStep[]>(() => {
    const s: WizardStep[] = ["welcome"];
    if (canEdit) s.push("data", "dashboard");
    s.push("ask");
    if (isAdmin) s.push("team");
    return s;
  }, [canEdit, isAdmin]);

  const [current, setCurrent] = useState<WizardStep>("welcome");
  const resumed = useRef(false);
  const done = (id: string) => ob.tasks.some((t) => t.id === id && t.done);

  // Entrar no assistente conta como "visto": nos próximos logins vai direto para a app.
  useEffect(() => {
    if (ob.isLoading || me.isLoading || resumed.current) return;
    resumed.current = true;
    if (ob.completed) {
      router.replace("/overview");
      return;
    }
    ob.dismiss();
    const saved = ob.step as WizardStep;
    if (steps.includes(saved)) setCurrent(saved);
  }, [ob.isLoading, me.isLoading, ob.completed, ob.step, steps, ob, router]);

  const idx = Math.max(0, steps.indexOf(current));
  const isLast = idx === steps.length - 1;

  const goTo = (step: WizardStep) => {
    setCurrent(step);
    ob.setStep(step).catch(() => {});
  };

  const finish = async () => {
    try {
      await ob.complete();
    } catch {}
    router.replace("/overview");
  };

  const next = () => (isLast ? finish() : goTo(steps[idx + 1]));
  const back = () => idx > 0 && goTo(steps[idx - 1]);

  const [question, setQuestion] = useState("");
  const [answer, setAnswer] = useState("");
  const [dashboard, setDashboard] = useState<{ id: string; name: string } | null>(null);
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState("analyst");
  const [invited, setInvited] = useState<string[]>([]);

  const demo = useMutation({
    mutationFn: () => api<{ dataset_id: string; name: string }>("/api/v1/datasets/demo", { method: "POST" }),
    onSuccess: (res) => {
      toast.success(`Dados de demonstração prontos: ${res.name}`);
      ob.refresh();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const aiDashboard = useMutation({
    mutationFn: () =>
      api<{ id: string; name: string }>("/api/v1/dashboards/ai", {
        method: "POST",
        body: JSON.stringify({ prompt: "Dashboard executivo com os principais indicadores do negócio" }),
      }),
    onSuccess: (res) => {
      setDashboard(res);
      toast.success("Painel criado");
      ob.refresh();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const ask = useMutation({
    mutationFn: (q: string) => api<{ answer: string }>("/api/v1/ai/ask", { method: "POST", body: JSON.stringify({ message: q }) }),
    onSuccess: (res) => {
      setAnswer(res.answer);
      ob.refresh();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const invite = useMutation({
    mutationFn: () =>
      api<{ invite_url: string }>("/api/v1/members/invite", {
        method: "POST",
        body: JSON.stringify({ email: inviteEmail.trim(), role: inviteRole }),
      }),
    onSuccess: () => {
      setInvited((l) => [...l, inviteEmail.trim()]);
      setInviteEmail("");
      toast.success("Convite enviado");
      ob.refresh();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  if (ob.isLoading || me.isLoading) {
    return <div className="flex min-h-[60vh] items-center justify-center text-sm text-mute">A preparar…</div>;
  }

  const meta = STEP_META[current];
  const Icon = meta.icon;
  const hasData = done("data");

  return (
    <div className="mx-auto max-w-4xl px-4 py-8 sm:py-12">
      <div className="mb-6 flex items-center justify-between">
        <div className="flex items-center gap-2 text-sm font-semibold text-ink">
          <Sparkles size={16} className="text-primary" /> TheDobra
        </div>
        <button onClick={finish} className="text-[13px] text-mute underline-offset-4 hover:text-ink hover:underline">
          Saltar configuração
        </button>
      </div>

      <div className="grid gap-6 md:grid-cols-[200px_minmax(0,1fr)]">
        <ol className="hidden space-y-1 md:block">
          {steps.map((s, i) => {
            const stepDone = i < idx || (s !== "welcome" && done(s));
            const active = s === current;
            return (
              <li key={s}>
                <button
                  onClick={() => goTo(s)}
                  className={`flex w-full items-center gap-2.5 rounded-xl px-3 py-2 text-left text-sm transition ${
                    active ? "bg-primary/10 font-medium text-primary" : "text-mute hover:bg-surface-2 hover:text-ink"
                  }`}
                >
                  <span
                    className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-full border text-[10px] ${
                      stepDone ? "border-primary bg-primary text-white" : active ? "border-primary text-primary" : "border-line"
                    }`}
                  >
                    {stepDone ? <Check size={11} /> : i + 1}
                  </span>
                  {STEP_META[s].label}
                </button>
              </li>
            );
          })}
        </ol>

        <Card className="space-y-6 p-6 sm:p-8">
          <div className="flex gap-1.5 md:hidden">
            {steps.map((s, i) => (
              <div key={s} className={`h-1.5 flex-1 rounded-full ${i <= idx ? "bg-primary" : "bg-surface-2"}`} />
            ))}
          </div>

          <div className="flex items-start gap-4">
            <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary">
              <Icon size={22} />
            </div>
            <div>
              <h1 className="text-xl font-semibold tracking-tight text-ink sm:text-2xl">
                {current === "welcome" && me.data?.name ? `Olá, ${me.data.name.split(" ")[0]}!` : meta.title}
              </h1>
              <p className="mt-1.5 text-sm leading-relaxed text-mute">{meta.description}</p>
            </div>
          </div>

          {current === "welcome" && (
            <div className="grid gap-3 sm:grid-cols-2">
              {steps
                .filter((s) => s !== "welcome")
                .map((s) => {
                  const M = STEP_META[s];
                  return (
                    <div key={s} className="flex items-center gap-3 rounded-xl border border-line bg-bg p-3">
                      <M.icon size={18} className="text-primary" />
                      <span className="text-sm text-ink">{M.title}</span>
                      {done(s) && <CheckCircle2 size={16} className="ml-auto text-ok" />}
                    </div>
                  );
                })}
            </div>
          )}

          {current === "data" && (
            <div className="space-y-4">
              <div className="grid gap-3 sm:grid-cols-2">
                <Choice
                  icon={Database}
                  title="Usar dados de demonstração"
                  description="Vendas de exemplo, prontas em segundos."
                  onClick={() => demo.mutate()}
                  busy={demo.isPending}
                  recommended
                />
                <Choice icon={Store} title="Modelo do meu setor" description="Varejo, moda, oficinas, finanças e mais." href="/store" />
                <Choice icon={Upload} title="Enviar ficheiro" description="CSV ou Excel do seu sistema." href="/data" />
                <Choice icon={Plug} title="Ligar base ou ERP" description="PostgreSQL, MySQL, APIs, ERPs." href="/connectors" />
              </div>
              {hasData && <Success>Já tem dados prontos. Pode avançar.</Success>}
            </div>
          )}

          {current === "dashboard" && (
            <div className="space-y-4">
              <div className="grid gap-3 sm:grid-cols-2">
                <Choice
                  icon={Sparkles}
                  title="Criar com IA"
                  description={hasData ? "Painel executivo a partir dos seus dados." : "Precisa de dados primeiro (passo anterior)."}
                  onClick={() => aiDashboard.mutate()}
                  busy={aiDashboard.isPending}
                  disabled={!hasData || !!dashboard}
                  recommended
                />
                <Choice icon={Store} title="Escolher na Loja" description="Modelos prontos com métricas e dados mock." href="/store" />
              </div>
              {dashboard && (
                <Success>
                  Painel criado: <strong>{dashboard.name}</strong> ·{" "}
                  <Link href={`/dashboards/${dashboard.id}`} className="underline">
                    abrir
                  </Link>
                </Success>
              )}
              {!dashboard && done("dashboard") && <Success>Já tem um painel na conta.</Success>}
            </div>
          )}

          {current === "ask" && (
            <div className="space-y-4">
              <div className="flex flex-wrap gap-2">
                {suggestedQuestions.map((q) => (
                  <button
                    key={q}
                    onClick={() => {
                      setQuestion(q);
                      ask.mutate(q);
                    }}
                    disabled={ask.isPending}
                    className="rounded-full border border-line bg-surface px-3 py-1.5 text-[12px] text-mute hover:border-primary/40 hover:text-ink disabled:opacity-50"
                  >
                    {q}
                  </button>
                ))}
              </div>
              <form
                className="flex flex-col gap-2 sm:flex-row"
                onSubmit={(e) => {
                  e.preventDefault();
                  if (question.trim()) ask.mutate(question.trim());
                }}
              >
                <input
                  className="min-h-11 flex-1 rounded-xl border border-line bg-surface px-3.5 text-sm text-ink outline-none focus:border-primary/50"
                  value={question}
                  onChange={(e) => setQuestion(e.target.value)}
                  placeholder="Ex.: qual foi a minha melhor semana?"
                />
                <Button type="submit" busy={ask.isPending} disabled={!question.trim()}>
                  <MessageSquare size={14} /> Perguntar
                </Button>
              </form>
              {!hasData && canEdit && <p className="text-[12px] text-mute">Dica: carregue dados no passo «Dados» para respostas sobre o seu negócio.</p>}
              {answer && <div className="whitespace-pre-wrap rounded-xl bg-bg p-4 text-sm text-ink">{answer}</div>}
            </div>
          )}

          {current === "team" && (
            <div className="space-y-4">
              <form
                className="flex flex-col gap-2 sm:flex-row"
                onSubmit={(e) => {
                  e.preventDefault();
                  if (inviteEmail.trim()) invite.mutate();
                }}
              >
                <input
                  type="email"
                  className="min-h-11 flex-1 rounded-xl border border-line bg-surface px-3.5 text-sm text-ink outline-none focus:border-primary/50"
                  value={inviteEmail}
                  onChange={(e) => setInviteEmail(e.target.value)}
                  placeholder="colega@empresa.com"
                  aria-label="E-mail do colega"
                />
                <select
                  className="min-h-11 rounded-xl border border-line bg-surface px-3 text-sm text-ink"
                  value={inviteRole}
                  onChange={(e) => setInviteRole(e.target.value)}
                  aria-label="Função"
                >
                  <option value="viewer">Leitor</option>
                  <option value="analyst">Analista</option>
                  <option value="admin">Admin</option>
                </select>
                <Button type="submit" busy={invite.isPending} disabled={!inviteEmail.trim()}>
                  Convidar
                </Button>
              </form>
              {invited.length > 0 && <Success>Convites enviados: {invited.join(", ")}</Success>}
            </div>
          )}

          <div className="flex flex-wrap items-center justify-between gap-2 border-t border-line pt-5">
            {idx > 0 ? (
              <Button variant="ghost" onClick={back}>
                <ArrowLeft size={14} /> Voltar
              </Button>
            ) : (
              <span />
            )}
            <div className="flex gap-2">
              {current !== "welcome" && !isLast && (
                <Button variant="secondary" onClick={next}>
                  Pular
                </Button>
              )}
              <Button onClick={next}>
                {current === "welcome" ? "Começar" : isLast ? "Concluir e ir para a app" : "Continuar"} <ArrowRight size={14} />
              </Button>
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
}

function Choice({
  icon: Icon,
  title,
  description,
  href,
  onClick,
  busy,
  disabled,
  recommended,
}: {
  icon: typeof Sparkles;
  title: string;
  description: string;
  href?: string;
  onClick?: () => void;
  busy?: boolean;
  disabled?: boolean;
  recommended?: boolean;
}) {
  const body = (
    <>
      <div className="flex items-center justify-between">
        <Icon size={20} className="text-primary" />
        {recommended && <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-semibold text-primary">Recomendado</span>}
      </div>
      <div className="mt-3 text-sm font-medium text-ink">{busy ? "A preparar…" : title}</div>
      <div className="mt-0.5 text-[12px] text-mute">{description}</div>
    </>
  );
  const cls = "card-float block p-4 text-left transition hover:-translate-y-0.5 hover:shadow-[var(--shadow-float)] disabled:cursor-not-allowed disabled:opacity-50";
  if (href) {
    return (
      <Link href={href} className={cls}>
        {body}
      </Link>
    );
  }
  return (
    <button type="button" onClick={onClick} disabled={disabled || busy} className={cls}>
      {body}
    </button>
  );
}

function Success({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-2 rounded-xl bg-emerald-50/70 p-3 text-sm text-ok">
      <CheckCircle2 size={16} className="shrink-0" />
      <span>{children}</span>
    </div>
  );
}

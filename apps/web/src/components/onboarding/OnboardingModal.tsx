"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Button } from "@/components/ui";
import { X, ArrowRight, ArrowLeft, Sparkles, Database, BarChart3, LayoutDashboard, MessageSquare, Users, CheckCircle, Check } from "lucide-react";
import { useOnboarding, ONBOARDING_STEPS, type OnboardingStep } from "./useOnboarding";

const stepContent: Record<OnboardingStep, { eyebrow: string; title: string; description: string; tip: string; icon: typeof Sparkles; cta?: string; href?: string }> = {
  welcome: {
    eyebrow: "Primeiros passos",
    title: "Vamos preparar a sua operação",
    description: "Em poucos minutos, ligue os seus dados, organize os indicadores e deixe o primeiro painel pronto para a equipa.",
    tip: "Pode avançar no seu ritmo. Este guia fica disponível na visão geral.",
    icon: Sparkles,
  },
  connect_data: {
    eyebrow: "Passo 1 · Dados",
    title: "Ligue os seus dados",
    description: "Carregue a demo de vendas, um CSV ou conecte uma base PostgreSQL/MySQL para começar.",
    tip: "Comece com um ficheiro ou dados de demonstração. Pode trocar a fonte depois.",
    icon: Database,
    cta: "Ir para Dados",
    href: "/data",
  },
  create_metric: {
    eyebrow: "Passo 2 · Modelo",
    title: "Crie uma métrica semântica",
    description: "No modelo semântico, defina medidas e dimensões oficiais. A TheDobra nunca inventa fórmulas.",
    tip: "Métricas oficiais tornam os números consistentes em todos os dashboards.",
    icon: BarChart3,
    cta: "Abrir modelo",
    href: "/data",
  },
  build_dashboard: {
    eyebrow: "Passo 3 · Visualização",
    title: "Crie o primeiro dashboard",
    description: "Construa um painel executivo ou peça à IA para gerar widgets a partir do modelo.",
    tip: "Use um modelo pronto se quiser começar com uma estrutura já organizada.",
    icon: LayoutDashboard,
    cta: "Novo dashboard",
    href: "/dashboards",
  },
  ask_ai: {
    eyebrow: "Passo 4 · Exploração",
    title: "Pergunte à TheDobra",
    description: "Faça uma pergunta em linguagem natural e receba respostas com evidência e métricas oficiais.",
    tip: "Pergunte sobre tendências, comparações ou os maiores contribuidores de um resultado.",
    icon: MessageSquare,
    cta: "Perguntar",
    href: "/ask",
  },
  invite_team: {
    eyebrow: "Passo 5 · Equipa",
    title: "Convide a equipa",
    description: "Partilhe a plataforma com colegas. Cada função tem permissões ajustadas.",
    tip: "Comece por convidar quem precisa acompanhar ou validar os indicadores.",
    icon: Users,
    cta: "Convidar",
    href: "/settings",
  },
  done: {
    eyebrow: "Configuração concluída",
    title: "Está pronto",
    description: "Já tem dados, métricas, dashboards e a equipa. Explore a visão geral para mais insights.",
    tip: "A qualquer momento, pode voltar às definições para ajustar a equipa e as integrações.",
    icon: CheckCircle,
    cta: "Começar",
    href: "/overview",
  },
};

export function OnboardingModal() {
  const { step, completed, seen, isLoading, next, markSeen } = useOnboarding();
  const [open, setOpen] = useState(false);
  const [current, setCurrent] = useState(step);

  useEffect(() => {
    if (!isLoading && !completed && step !== "done" && !seen) {
      setOpen(true);
    }
  }, [isLoading, completed, step, seen]);

  useEffect(() => {
    setCurrent(step);
  }, [step]);

  if (!open || isLoading || completed || step === "done") return null;

  const content = stepContent[current] || stepContent.welcome;
  const Icon = content.icon;
  const idx = ONBOARDING_STEPS.indexOf(current);
  const total = ONBOARDING_STEPS.length - 1;

  const close = () => {
    setOpen(false);
    markSeen();
  };

  const handleNext = () => {
    if (current === "done") {
      close();
      return;
    }
    const nextStep = next(current);
    setCurrent(nextStep);
    if (nextStep === "done") {
      setTimeout(close, 300);
    }
  };

  const handleBack = () => {
    const prev = Math.max(0, idx - 1);
    setCurrent(ONBOARDING_STEPS[prev]);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center p-0 sm:items-center sm:p-4">
      <div className="absolute inset-0 bg-ink/40 backdrop-blur-sm" onClick={close} />
      <div className="relative max-h-[92dvh] w-full max-w-2xl overflow-y-auto rounded-t-3xl border border-line bg-surface shadow-2xl sm:rounded-3xl">
        <div className="grid md:grid-cols-[190px_minmax(0,1fr)]">
        <div className="brand-gradient p-6 text-white md:p-7">
          <div className="flex items-start justify-between">
            <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-white/20">
              <Icon size={24} />
            </div>
            <button
              onClick={close}
              className="flex h-9 w-9 items-center justify-center rounded-full text-white/80 hover:bg-white/20"
              aria-label="Fechar"
            >
              <X size={18} />
            </button>
          </div>
          <p className="mt-5 text-[10px] font-semibold uppercase tracking-[0.16em] text-white/65">{content.eyebrow}</p>
          <h2 className="mt-2 text-2xl font-semibold tracking-tight">{content.title}</h2>
          <p className="mt-2 text-sm text-white/85">{content.description}</p>
          <div className="mt-6 hidden space-y-2 md:block">
            {ONBOARDING_STEPS.slice(1, -1).map((stepName, stepIndex) => {
              const stepDone = idx > stepIndex;
              const stepActive = current === stepName;
              return (
                <div key={stepName} className={`flex items-center gap-2 rounded-lg px-2 py-1.5 text-[11px] ${stepActive ? "bg-white/15 text-white" : stepDone ? "text-white/75" : "text-white/45"}`}>
                  <span className={`flex h-4 w-4 items-center justify-center rounded-full border ${stepDone ? "border-white bg-white text-primary" : "border-white/30"}`}>
                    {stepDone && <Check size={10} />}
                  </span>
                  <span>{stepContent[stepName].eyebrow.split(" · ")[1]}</span>
                </div>
              );
            })}
          </div>
        </div>
        <div className="p-6 sm:p-7">
          <div className="mb-4 flex items-center gap-1.5">
            {ONBOARDING_STEPS.slice(0, -1).map((s, i) => (
              <div
                key={s}
                className={`h-1.5 flex-1 rounded-full transition ${i <= idx ? "bg-primary" : "bg-surface-2"}`}
              />
            ))}
          </div>
          <div className="mb-6 rounded-2xl border border-primary/10 bg-primary/[0.045] p-4">
            <p className="text-[11px] font-semibold uppercase tracking-wide text-primary">Dica para esta etapa</p>
            <p className="mt-1.5 text-[13px] leading-relaxed text-mute">{content.tip}</p>
          </div>
          <div className="flex flex-wrap items-center justify-end gap-2">
            {idx > 0 && (
              <Button variant="ghost" onClick={handleBack}>
                <ArrowLeft size={14} /> Voltar
              </Button>
            )}
            <Button variant="secondary" onClick={close}>
              Continuar depois
            </Button>
            {content.href && content.cta && (
              <Link href={content.href} onClick={handleNext}>
                <Button variant="primary">
                  {content.cta} <ArrowRight size={14} />
                </Button>
              </Link>
            )}
            {!content.href && (
              <Button onClick={handleNext}>
                {current === "done" ? "Terminar" : "Próximo"} <ArrowRight size={14} />
              </Button>
            )}
          </div>
          <div className="mt-3 text-right text-[11px] text-mute">
            Passo {idx + 1} de {total}
          </div>
        </div>
        </div>
      </div>
    </div>
  );
}

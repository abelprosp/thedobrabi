"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Button } from "@/components/ui";
import { X, ArrowRight, ArrowLeft, Sparkles, Database, BarChart3, LayoutDashboard, MessageSquare, Users, CheckCircle } from "lucide-react";
import { useOnboarding, ONBOARDING_STEPS, type OnboardingStep } from "./useOnboarding";

const stepContent: Record<OnboardingStep, { title: string; description: string; icon: typeof Sparkles; cta?: string; href?: string }> = {
  welcome: {
    title: "Bem-vindo à TheDobra",
    description: "O analista digital do seu negócio. Vamos dar os primeiros passos para transformar dados em decisões.",
    icon: Sparkles,
  },
  connect_data: {
    title: "Ligue os seus dados",
    description: "Carregue a demo de vendas, um CSV ou conecte uma base PostgreSQL/MySQL para começar.",
    icon: Database,
    cta: "Ir para Dados",
    href: "/data",
  },
  create_metric: {
    title: "Crie uma métrica semântica",
    description: "No modelo semântico, defina medidas e dimensões oficiais. A TheDobra nunca inventa fórmulas.",
    icon: BarChart3,
    cta: "Abrir modelo",
    href: "/data",
  },
  build_dashboard: {
    title: "Crie o primeiro dashboard",
    description: "Construa um painel executivo ou peça à IA para gerar widgets a partir do modelo.",
    icon: LayoutDashboard,
    cta: "Novo dashboard",
    href: "/dashboards",
  },
  ask_ai: {
    title: "Pergunte à TheDobra",
    description: "Faça uma pergunta em linguagem natural e receba respostas com evidência e métricas oficiais.",
    icon: MessageSquare,
    cta: "Perguntar",
    href: "/ask",
  },
  invite_team: {
    title: "Convide a equipa",
    description: "Partilhe a plataforma com colegas. Cada função tem permissões ajustadas.",
    icon: Users,
    cta: "Convidar",
    href: "/settings",
  },
  done: {
    title: "Está pronto",
    description: "Já tem dados, métricas, dashboards e a equipa. Explore a visão geral para mais insights.",
    icon: CheckCircle,
    cta: "Começar",
    href: "/overview",
  },
};

export function OnboardingModal() {
  const { step, completed, seen, isLoading, next, skip, markSeen } = useOnboarding();
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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-ink/40 backdrop-blur-sm" onClick={close} />
      <div className="relative w-full max-w-lg overflow-hidden rounded-3xl border border-line bg-white shadow-2xl">
        <div className="brand-gradient p-6 text-white">
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
          <h2 className="mt-4 text-2xl font-semibold tracking-tight">{content.title}</h2>
          <p className="mt-2 text-sm text-white/85">{content.description}</p>
        </div>
        <div className="p-6">
          <div className="mb-4 flex items-center gap-1.5">
            {ONBOARDING_STEPS.slice(0, -1).map((s, i) => (
              <div
                key={s}
                className={`h-1.5 flex-1 rounded-full transition ${i <= idx ? "bg-primary" : "bg-surface-2"}`}
              />
            ))}
          </div>
          <div className="flex flex-wrap items-center justify-end gap-2">
            {idx > 0 && (
              <Button variant="ghost" onClick={handleBack}>
                <ArrowLeft size={14} /> Voltar
              </Button>
            )}
            <Button variant="secondary" onClick={skip}>
              Pular
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
  );
}

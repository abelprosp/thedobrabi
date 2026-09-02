"use client";

import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { useOnboarding, type OnboardingStep, SpotlightTooltip, type SpotlightStep } from "./";

const spotlightSteps: Record<string, SpotlightStep[]> = {
  "/overview": [
    { target: "nav a[href='/data']", title: "Dados", description: "Comece por ligar uma fonte de dados ou carregar a demo.", position: "right" },
    { target: "header a[href='/ask']", title: "Perguntar", description: "Clique aqui para fazer perguntas à IA em qualquer altura.", position: "bottom" },
  ],
  "/data": [
    { target: "[data-onboarding='demo']", title: "Demo rápida", description: "Carregue dados de vendas de demonstração para explorar.", position: "bottom" },
  ],
  "/dashboards": [
    { target: "[data-onboarding='new-dashboard']", title: "Novo dashboard", description: "Crie o seu primeiro painel executivo.", position: "bottom" },
  ],
  "/ask": [
    { target: "input[aria-label='Pergunta']", title: "Pergunte", description: "Escreva uma pergunta sobre os seus dados e pressione Enter.", position: "top" },
  ],
  "/settings": [
    { target: "input[aria-label='E-mail do novo membro']", title: "Convidar", description: "Adicione o e-mail do colega e clique em Convidar.", position: "top" },
  ],
};

export function OnboardingSpotlight() {
  const pathname = usePathname();
  const { step, completed, markSeen } = useOnboarding();
  const [open, setOpen] = useState(false);
  const [tourStep, setTourStep] = useState(0);
  const [steps, setSteps] = useState<SpotlightStep[]>([]);

  useEffect(() => {
    if (completed || step === "done" || !pathname) {
      setOpen(false);
      return;
    }
    const candidates = spotlightSteps[pathname] || [];
    // Only show the spotlight if the current onboarding step matches a relevant page
    const relevant = candidates.filter((s) => isRelevant(pathname, step));
    if (relevant.length > 0) {
      setSteps(relevant);
      setTourStep(0);
      setOpen(true);
    } else {
      setOpen(false);
    }
  }, [pathname, step, completed]);

  const onNext = () => {
    if (tourStep >= steps.length - 1) {
      setOpen(false);
      markSeen();
    } else {
      setTourStep((s) => s + 1);
    }
  };

  const onClose = () => {
    setOpen(false);
    markSeen();
  };

  return <SpotlightTooltip steps={steps} step={tourStep} open={open} onNext={onNext} onClose={onClose} />;
}

function isRelevant(path: string, step: OnboardingStep) {
  if (step === "connect_data" && path === "/data") return true;
  if (step === "build_dashboard" && path === "/dashboards") return true;
  if (step === "ask_ai" && path === "/ask") return true;
  if (step === "invite_team" && path === "/settings") return true;
  if (step === "welcome" && path === "/overview") return true;
  return false;
}

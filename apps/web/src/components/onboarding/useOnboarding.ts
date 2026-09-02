"use client";

import { useCallback, useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

export const ONBOARDING_STEPS = [
  "welcome",
  "connect_data",
  "create_metric",
  "build_dashboard",
  "ask_ai",
  "invite_team",
  "done",
] as const;

export type OnboardingStep = (typeof ONBOARDING_STEPS)[number];

export const CHECKLIST_TASKS = [
  { id: "connect_data", label: "Ligar uma fonte de dados", href: "/data" },
  { id: "create_metric", label: "Criar uma métrica semântica", href: "/data" },
  { id: "ask_ai", label: "Fazer uma pergunta à IA", href: "/ask" },
  { id: "build_dashboard", label: "Criar um dashboard", href: "/dashboards" },
  { id: "add_widget", label: "Adicionar um widget", href: "/dashboards" },
  { id: "invite_team", label: "Convidar um membro da equipa", href: "/settings" },
] as const;

export type OnboardingState = {
  step: OnboardingStep;
  completed: boolean;
  seen: boolean;
  localStep?: OnboardingStep;
};

const LS_KEY = "thedobra.onboarding";

function loadLocal(): Partial<OnboardingState> {
  if (typeof window === "undefined") return {};
  try {
    const raw = localStorage.getItem(LS_KEY);
    if (raw) return JSON.parse(raw);
  } catch {}
  return {};
}

function saveLocal(state: Partial<OnboardingState>) {
  if (typeof window === "undefined") return;
  try {
    localStorage.setItem(LS_KEY, JSON.stringify(state));
  } catch {}
}

export function useOnboarding() {
  const qc = useQueryClient();
  const [local, setLocal] = useState<Partial<OnboardingState>>(loadLocal);

  const { data: me, isLoading } = useQuery({
    queryKey: ["me"],
    queryFn: () =>
      api<{
        user_id: string;
        onboarding_step?: OnboardingStep;
        onboarding_completed?: boolean;
      }>("/api/v1/auth/me"),
  });

  const step = me?.onboarding_step || local.step || "welcome";
  const completed = me?.onboarding_completed ?? local.completed ?? false;

  useEffect(() => {
    if (me?.onboarding_step) {
      setLocal((prev) => {
        const next = { ...prev, step: me.onboarding_step, completed: me.onboarding_completed };
        saveLocal(next);
        return next;
      });
    }
  }, [me?.onboarding_step, me?.onboarding_completed]);

  const nextMutation = useMutation({
    mutationFn: async (target?: OnboardingStep) => {
      const res = await api<{ onboarding_step: OnboardingStep; onboarding_completed: boolean }>("/api/v1/onboarding/next", {
        method: "POST",
        body: JSON.stringify({ step: target || step }),
      });
      return res;
    },
    onSuccess: (res) => {
      setLocal({ step: res.onboarding_step, completed: res.onboarding_completed });
      saveLocal({ step: res.onboarding_step, completed: res.onboarding_completed });
      qc.invalidateQueries({ queryKey: ["me"] });
    },
  });

  const completeMutation = useMutation({
    mutationFn: () => api<{ onboarding_step: OnboardingStep; onboarding_completed: boolean }>("/api/v1/onboarding/complete", { method: "POST" }),
    onSuccess: (res) => {
      setLocal({ step: res.onboarding_step, completed: res.onboarding_completed });
      saveLocal({ step: res.onboarding_step, completed: res.onboarding_completed });
      qc.invalidateQueries({ queryKey: ["me"] });
    },
  });

  const next = useCallback(
    (target?: OnboardingStep) => {
      const currentIdx = ONBOARDING_STEPS.indexOf(target || step);
      const nextStep = ONBOARDING_STEPS[Math.min(currentIdx + 1, ONBOARDING_STEPS.length - 1)];
      setLocal((prev) => {
        const s = { ...prev, step: nextStep, completed: nextStep === "done" };
        saveLocal(s);
        return s;
      });
      nextMutation.mutate(target || step);
      return nextStep;
    },
    [step, nextMutation],
  );

  const complete = useCallback(() => {
    setLocal({ step: "done", completed: true });
    saveLocal({ step: "done", completed: true });
    completeMutation.mutate();
  }, [completeMutation]);

  const skip = useCallback(() => {
    complete();
  }, [complete]);

  const markSeen = useCallback(() => {
    setLocal((prev) => {
      const s = { ...prev, seen: true };
      saveLocal(s);
      return s;
    });
  }, []);

  const taskCompleted = useCallback(
    (id: string) => {
      const stepIdx = ONBOARDING_STEPS.indexOf(step);
      // Map checklist task id to the onboarding step that completes it.
      const stepForTask: Record<string, string> = {
        connect_data: "create_metric",
        create_metric: "build_dashboard",
        ask_ai: "build_dashboard", // ask happens before dashboard in flow, but both are unlocked by this point
        build_dashboard: "invite_team",
        add_widget: "invite_team",
        invite_team: "done",
      };
      const requiredStep = stepForTask[id];
      if (!requiredStep) return false;
      return stepIdx >= ONBOARDING_STEPS.indexOf(requiredStep as OnboardingStep);
    },
    [step],
  );

  return {
    step,
    completed,
    seen: local.seen ?? false,
    isLoading,
    next,
    complete,
    skip,
    markSeen,
    taskCompleted,
    isBusy: nextMutation.isPending || completeMutation.isPending,
  };
}

export function stepIndex(step: OnboardingStep) {
  return ONBOARDING_STEPS.indexOf(step);
}

"use client";

import { useCallback } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

export const ONBOARDING_STEPS = ["welcome", "data", "dashboard", "ask", "team", "done"] as const;

export type OnboardingStep = (typeof ONBOARDING_STEPS)[number];

export type TaskId = "data" | "dashboard" | "ask" | "automate" | "team";

export const CHECKLIST_TASKS: Record<TaskId, { label: string; description: string; href: string; cta: string }> = {
  data: {
    label: "Adicionar dados",
    description: "Demo, CSV/Excel ou ligação a base de dados e ERP.",
    href: "/connectors",
    cta: "Adicionar",
  },
  dashboard: {
    label: "Ter o primeiro painel",
    description: "Ative um modelo pronto da Loja ou crie com IA.",
    href: "/store",
    cta: "Ver modelos",
  },
  ask: {
    label: "Fazer uma pergunta à IA",
    description: "Pergunte em português, como a um analista.",
    href: "/ask",
    cta: "Perguntar",
  },
  automate: {
    label: "Automatizar",
    description: "Crie um alerta, um relatório ou agende a atualização.",
    href: "/alerts",
    cta: "Configurar",
  },
  team: {
    label: "Convidar a equipa",
    description: "Partilhe os painéis com quem decide.",
    href: "/settings",
    cta: "Convidar",
  },
};

export type OnboardingStatus = {
  onboarding_step: OnboardingStep;
  onboarding_completed: boolean;
  onboarding_dismissed: boolean;
  tasks?: { id: TaskId; done: boolean }[];
  just_completed?: boolean;
};

export const ONBOARDING_QUERY_KEY = ["onboarding"];

export type OnboardingUser = { onboarding_step?: string; onboarding_completed?: boolean; onboarding_dismissed?: boolean };

/** Para onde mandar o utilizador depois do login: o assistente só aparece na primeira vez. */
export function postLoginPath(user?: OnboardingUser) {
  if (!user || user.onboarding_completed || user.onboarding_dismissed) return "/overview";
  return "/welcome";
}

export function useOnboarding() {
  const qc = useQueryClient();

  const query = useQuery({
    queryKey: ONBOARDING_QUERY_KEY,
    queryFn: () => api<OnboardingStatus>("/api/v1/onboarding"),
    staleTime: 30_000,
    refetchOnWindowFocus: true,
  });

  const apply = (res: Partial<OnboardingStatus>) => {
    qc.setQueryData<OnboardingStatus>(ONBOARDING_QUERY_KEY, (prev) => ({ ...(prev as OnboardingStatus), ...res }));
    qc.invalidateQueries({ queryKey: ["me"] });
  };

  const post = (path: string, body?: unknown) =>
    api<OnboardingStatus>(`/api/v1/onboarding/${path}`, { method: "POST", body: body ? JSON.stringify(body) : undefined });

  const stepMutation = useMutation({ mutationFn: (step: OnboardingStep) => post("step", { step }), onSuccess: apply });
  const dismissMutation = useMutation({ mutationFn: () => post("dismiss"), onSuccess: apply });
  const completeMutation = useMutation({ mutationFn: () => post("complete"), onSuccess: apply });
  const resetMutation = useMutation({
    mutationFn: () => post("reset"),
    onSuccess: (res) => {
      apply(res);
      qc.invalidateQueries({ queryKey: ONBOARDING_QUERY_KEY });
    },
  });

  const data = query.data;
  const completed = data?.onboarding_completed ?? false;
  const dismissed = data?.onboarding_dismissed ?? false;
  const tasks = data?.tasks ?? [];

  const setStep = useCallback((step: OnboardingStep) => stepMutation.mutateAsync(step), [stepMutation]);
  const dismiss = useCallback(() => {
    if (dismissed) return;
    qc.setQueryData<OnboardingStatus>(ONBOARDING_QUERY_KEY, (prev) => (prev ? { ...prev, onboarding_dismissed: true } : prev));
    dismissMutation.mutate();
  }, [dismissed, dismissMutation, qc]);
  const complete = useCallback(() => completeMutation.mutateAsync(), [completeMutation]);
  const reset = useCallback(() => resetMutation.mutateAsync(), [resetMutation]);

  return {
    isLoading: query.isLoading,
    step: data?.onboarding_step ?? "welcome",
    completed,
    dismissed,
    justCompleted: data?.just_completed ?? false,
    tasks,
    doneCount: tasks.filter((t) => t.done).length,
    refresh: () => query.refetch(),
    setStep,
    dismiss,
    complete,
    reset,
    isBusy: stepMutation.isPending || dismissMutation.isPending || completeMutation.isPending || resetMutation.isPending,
  };
}

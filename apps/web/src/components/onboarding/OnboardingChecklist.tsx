"use client";

import Link from "next/link";
import { useOnboarding, CHECKLIST_TASKS, stepIndex } from "./useOnboarding";
import { Button, Card, CardTitle } from "@/components/ui";
import { CheckCircle2, Circle, PartyPopper } from "lucide-react";

export function OnboardingChecklist() {
  const { step, completed, taskCompleted, next, complete } = useOnboarding();

  if (completed || step === "done") {
    return (
      <Card className="flex items-center gap-3 border-emerald-200 bg-emerald-50/60 py-4">
        <PartyPopper className="text-ok" size={22} />
        <div className="text-sm font-medium text-ok">Parabéns! Completou o onboarding da TheDobra.</div>
      </Card>
    );
  }

  const currentStepIdx = stepIndex(step);
  // Map onboarding step to the current checklist task index (connect_data=1 -> task 0, ...)
  const currentTaskIdx = Math.max(0, currentStepIdx - 1);
  const total = CHECKLIST_TASKS.length;
  const completedCount = CHECKLIST_TASKS.filter((t) => taskCompleted(t.id)).length;
  const progress = Math.round((completedCount / total) * 100);

  return (
    <Card>
      <CardTitle>Primeiros passos</CardTitle>
      <div className="mb-3 flex items-center gap-3">
        <div className="h-2 flex-1 overflow-hidden rounded-full bg-surface-2">
          <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${progress}%` }} />
        </div>
        <span className="text-[12px] text-mute">
          {completedCount}/{total}
        </span>
      </div>
      <div className="space-y-2">
        {CHECKLIST_TASKS.map((t, i) => {
          const done = taskCompleted(t.id);
          const active = i === currentTaskIdx;
          return (
            <div
              key={t.id}
              className={`flex items-center justify-between rounded-xl border px-3 py-2 transition ${
                done
                  ? "border-emerald-200 bg-emerald-50/40"
                  : active
                    ? "border-primary/30 bg-primary/5"
                    : "border-line bg-white"
              }`}
            >
              <div className="flex items-center gap-2.5">
                {done ? <CheckCircle2 size={16} className="text-ok" /> : <Circle size={16} className={active ? "text-primary" : "text-slate-300"} />}
                <span className={`text-sm ${done ? "text-ok line-through" : active ? "font-medium text-ink" : "text-mute"}`}>{t.label}</span>
              </div>
              <Link
                href={t.href}
                onClick={() => {
                  if (active) next(step);
                }}
              >
                <Button size="sm" variant={active ? "primary" : "secondary"}>
                  {done ? "Feito" : active ? "Começar" : "Ver"}
                </Button>
              </Link>
            </div>
          );
        })}
      </div>
      {completedCount === total - 1 && (
        <div className="mt-4 text-right">
          <Button size="sm" onClick={complete}>
            Concluir onboarding
          </Button>
        </div>
      )}
    </Card>
  );
}

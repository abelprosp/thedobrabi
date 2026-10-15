"use client";

import { useState } from "react";
import Link from "next/link";
import { useOnboarding, CHECKLIST_TASKS } from "./useOnboarding";
import { Button, Card } from "@/components/ui";
import { CheckCircle2, Circle, PartyPopper, X } from "lucide-react";

export function OnboardingChecklist() {
  const { isLoading, completed, justCompleted, tasks, doneCount, complete, isBusy } = useOnboarding();
  const [hidden, setHidden] = useState(false);

  if (isLoading || hidden) return null;

  if (completed) {
    if (!justCompleted) return null;
    return (
      <Card className="flex items-center gap-3 border-emerald-200 bg-emerald-50/60 py-4">
        <PartyPopper className="text-ok" size={22} />
        <div className="flex-1 text-sm font-medium text-ok">Tudo pronto! A configuração inicial foi concluída e este guia não volta a aparecer.</div>
        <button onClick={() => setHidden(true)} className="text-ok/70 hover:text-ok" aria-label="Fechar">
          <X size={16} />
        </button>
      </Card>
    );
  }

  if (tasks.length === 0) return null;

  const total = tasks.length;
  const progress = Math.round((doneCount / total) * 100);
  const nextId = tasks.find((t) => !t.done)?.id;

  return (
    <Card>
      <div className="mb-3 flex items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-ink">Primeiros passos</h2>
          <p className="mt-0.5 text-[12px] text-mute">Cada passo marca-se sozinho quando o fizer.</p>
        </div>
        <Button size="sm" variant="ghost" onClick={() => complete()} busy={isBusy}>
          Ocultar guia
        </Button>
      </div>
      <div className="mb-4 flex items-center gap-3">
        <div className="h-2 flex-1 overflow-hidden rounded-full bg-surface-2">
          <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${progress}%` }} />
        </div>
        <span className="text-[12px] text-mute">
          {doneCount}/{total}
        </span>
      </div>
      <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
        {tasks.map((t) => {
          const meta = CHECKLIST_TASKS[t.id];
          if (!meta) return null;
          const active = t.id === nextId;
          return (
            <div
              key={t.id}
              className={`flex flex-col justify-between gap-3 rounded-xl border p-3 transition ${
                t.done ? "border-emerald-200 bg-emerald-50/40" : active ? "border-primary/30 bg-primary/5" : "border-line bg-surface"
              }`}
            >
              <div className="flex items-start gap-2.5">
                {t.done ? (
                  <CheckCircle2 size={16} className="mt-0.5 shrink-0 text-ok" />
                ) : (
                  <Circle size={16} className={`mt-0.5 shrink-0 ${active ? "text-primary" : "text-mute/40"}`} />
                )}
                <div>
                  <div className={`text-sm ${t.done ? "text-ok line-through" : "font-medium text-ink"}`}>{meta.label}</div>
                  {!t.done && <div className="mt-0.5 text-[12px] text-mute">{meta.description}</div>}
                </div>
              </div>
              {!t.done && (
                <Link href={meta.href} className="self-end">
                  <Button size="sm" variant={active ? "primary" : "secondary"}>
                    {meta.cta}
                  </Button>
                </Link>
              )}
            </div>
          );
        })}
      </div>
    </Card>
  );
}

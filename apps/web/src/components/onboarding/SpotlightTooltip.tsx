"use client";

import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui";
import { X } from "lucide-react";

export type SpotlightStep = {
  target: string;
  title: string;
  description: string;
  position?: "top" | "bottom" | "left" | "right";
};

export function SpotlightTooltip({
  steps,
  step,
  onNext,
  onClose,
  open,
}: {
  steps: SpotlightStep[];
  step: number;
  onNext: () => void;
  onClose: () => void;
  open: boolean;
}) {
  const tooltipRef = useRef<HTMLDivElement | null>(null);
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);
  const current = steps[step];

  useEffect(() => {
    if (!open || !current) return;
    const target = document.querySelector(current.target);
    if (!target) {
      setPos(null);
      return;
    }
    const rect = target.getBoundingClientRect();
    const gap = 12;
    let left = rect.left + rect.width / 2;
    let top = rect.bottom + gap;
    if (current.position === "top") top = rect.top - gap;
    if (current.position === "left") left = rect.left - gap;
    if (current.position === "right") left = rect.right + gap;
    setPos({ left, top });
  }, [open, current, step]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open || !current || pos === null) return null;

  return (
    <div className="fixed inset-0 z-[60]">
      <div className="absolute inset-0 bg-ink/20" onClick={onClose} />
      <div
        ref={tooltipRef}
        className="absolute w-80 rounded-2xl border border-line bg-white p-4 shadow-xl"
        style={{ left: Math.max(12, Math.min(window.innerWidth - 320, pos.left - 160)), top: Math.max(12, pos.top) }}
      >
        <div className="flex items-start justify-between">
          <h3 className="text-sm font-semibold text-ink">{current.title}</h3>
          <button onClick={onClose} className="text-mute hover:text-ink" aria-label="Fechar">
            <X size={14} />
          </button>
        </div>
        <p className="mt-1 text-[13px] text-mute">{current.description}</p>
        <div className="mt-3 flex justify-between">
          <span className="text-[11px] text-mute">
            {step + 1} / {steps.length}
          </span>
          <Button size="sm" onClick={onNext}>
            {step < steps.length - 1 ? "Próximo" : "Concluir"}
          </Button>
        </div>
      </div>
    </div>
  );
}

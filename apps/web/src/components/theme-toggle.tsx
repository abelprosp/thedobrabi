"use client";

import { Moon, Sun } from "lucide-react";
import { useSystemTheme } from "@/components/theme-provider";
import { cn } from "@/lib/cn";
import type { Appearance } from "@/lib/theme";

export function ThemeToggle({
  className,
  withLabel,
}: {
  className?: string;
  withLabel?: boolean;
}) {
  const { theme, toggle } = useSystemTheme();
  const dark = theme === "dark";
  return (
    <button
      type="button"
      onClick={toggle}
      aria-label={dark ? "Mudar para tema claro" : "Mudar para tema escuro"}
      title={dark ? "Tema claro" : "Tema escuro"}
      className={cn(
        "flex h-10 items-center justify-center gap-1.5 rounded-lg text-mute hover:bg-surface-2 hover:text-ink sm:h-9",
        withLabel ? "px-2.5 text-[12px]" : "w-10 sm:w-9",
        className,
      )}
    >
      {dark ? <Sun size={16} aria-hidden /> : <Moon size={16} aria-hidden />}
      {withLabel && <span>{dark ? "Claro" : "Escuro"}</span>}
    </button>
  );
}

export function ThemeSegmented({
  value,
  onChange,
  label = "Tema",
}: {
  value: Appearance;
  onChange: (theme: Appearance) => void;
  label?: string;
}) {
  return (
    <div className="inline-flex rounded-xl border border-line bg-surface p-0.5" role="group" aria-label={label}>
      <button
        type="button"
        onClick={() => onChange("light")}
        className={cn(
          "inline-flex min-h-10 items-center gap-1.5 rounded-lg px-3 py-1.5 text-[12px] font-medium transition sm:min-h-0",
          value === "light" ? "bg-primary text-white shadow-sm" : "text-mute hover:text-ink",
        )}
        aria-pressed={value === "light"}
      >
        <Sun size={14} aria-hidden />
        Claro
      </button>
      <button
        type="button"
        onClick={() => onChange("dark")}
        className={cn(
          "inline-flex min-h-10 items-center gap-1.5 rounded-lg px-3 py-1.5 text-[12px] font-medium transition sm:min-h-0",
          value === "dark" ? "bg-primary text-white shadow-sm" : "text-mute hover:text-ink",
        )}
        aria-pressed={value === "dark"}
      >
        <Moon size={14} aria-hidden />
        Escuro
      </button>
    </div>
  );
}

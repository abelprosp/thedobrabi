"use client";

import Link from "next/link";
import { AlertCircle, Inbox, Loader2, type LucideIcon } from "lucide-react";
import { cn, formatPt, isNumericValue } from "@/lib/cn";
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";

export { cn, formatPt, isNumericValue };

const focusRing = "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-2 focus-visible:ring-offset-bg";

export function Button({
  variant = "primary",
  size = "md",
  busy,
  className,
  children,
  disabled,
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "ghost" | "danger";
  size?: "sm" | "md" | "lg" | "icon";
  busy?: boolean;
}) {
  const variants = {
    primary: "bg-primary text-white shadow-sm shadow-primary/20 hover:-translate-y-px hover:bg-primary-600 hover:shadow-md hover:shadow-primary/20 disabled:opacity-60 disabled:hover:translate-y-0",
    secondary: "border border-line bg-surface text-ink shadow-sm hover:-translate-y-px hover:border-primary/25 hover:bg-surface-2 disabled:hover:translate-y-0",
    ghost: "text-mute hover:bg-surface-2 hover:text-ink",
    danger: "border border-line bg-surface text-danger hover:border-danger/20 hover:bg-rose-50 dark:hover:bg-rose-500/10",
  };
  const sizes = {
    sm: "min-h-10 px-3 text-[12px] sm:min-h-9",
    md: "min-h-11 px-4 text-sm sm:min-h-10",
    lg: "min-h-11 px-5 text-sm",
    icon: "h-10 w-10 min-h-10 p-0 sm:h-9 sm:w-9 sm:min-h-9",
  };
  return (
    <button
      type={type}
      disabled={disabled || busy}
      className={cn(
        "inline-flex items-center justify-center gap-2 rounded-xl font-medium transition-all duration-150 disabled:cursor-not-allowed",
        focusRing,
        variants[variant],
        sizes[size],
        className,
      )}
      {...props}
    >
      {busy && <Loader2 size={14} className="animate-spin" />}
      {children}
    </button>
  );
}

export function Card({ className, children }: { className?: string; children: ReactNode }) {
  return <div className={cn("rounded-[1.125rem] border border-line/90 bg-surface p-4 shadow-[var(--shadow-card)] sm:p-5", className)}>{children}</div>;
}

export function CardTitle({ children }: { children: ReactNode }) {
  return <h2 className="mb-3 text-sm font-semibold tracking-tight text-ink">{children}</h2>;
}

const fieldCls =
  "w-full min-h-11 rounded-xl border border-line bg-surface px-3.5 py-2 text-sm text-ink shadow-sm placeholder:text-mute/80 outline-none transition focus:border-primary/50 focus:ring-3 focus:ring-primary/10 disabled:cursor-not-allowed disabled:bg-bg disabled:text-mute sm:min-h-10";

export function Input({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={cn(fieldCls, className)} {...props} />;
}

export function Select({ className, children, ...props }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select className={cn(fieldCls, className)} {...props}>
      {children}
    </select>
  );
}

export function Textarea({ className, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={cn(fieldCls, "min-h-24 py-2.5", className)} {...props} />;
}

export function FieldLabel({
  label,
  hint,
  error,
  required,
  children,
}: {
  label: string;
  hint?: string;
  error?: string;
  required?: boolean;
  children: ReactNode;
}) {
  return (
    <label className="block text-[13px] font-medium text-ink">
      {label}
      {required && <span className="ml-0.5 text-accent">*</span>}
      <span className="mt-1.5 block">{children}</span>
      {error ? <span className="mt-1 block text-[12px] font-normal text-danger">{error}</span> : hint ? <span className="mt-1 block text-[12px] font-normal text-mute">{hint}</span> : null}
    </label>
  );
}

export function PageHeader({
  title,
  description,
  actions,
  crumbs,
}: {
  title: string;
  description?: string;
  actions?: ReactNode;
  crumbs?: { href: string; label: string }[];
}) {
  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:flex-wrap sm:items-start sm:justify-between">
      <div className="min-w-0">
        {crumbs && crumbs.length > 0 && (
          <nav aria-label="Navegação estrutural" className="mb-1.5 flex flex-wrap items-center gap-1.5 text-[12px] text-mute">
            {crumbs.map((c, i) => (
              <span key={c.href} className="flex items-center gap-1.5">
                {i > 0 && <span className="text-line">/</span>}
                <Link href={c.href} className={cn("hover:text-ink", focusRing, "rounded")}>
                  {c.label}
                </Link>
              </span>
            ))}
            <span className="text-line">/</span>
            <span className="truncate text-ink">{title}</span>
          </nav>
        )}
        <h1 className="text-2xl font-semibold tracking-[-0.025em] text-ink sm:text-[1.75rem]">{title}</h1>
        {description && <p className="mt-1.5 max-w-2xl text-sm leading-relaxed text-mute">{description}</p>}
      </div>
      {actions && <div className="page-actions flex flex-wrap items-center gap-2 sm:justify-end">{actions}</div>}
    </div>
  );
}

export function EmptyState({
  title,
  description,
  action,
  icon: Icon = Inbox,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
  icon?: LucideIcon;
}) {
  return (
    <Card className="flex flex-col items-center border-dashed py-14 text-center shadow-none">
      <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-primary/10 text-primary ring-8 ring-primary/[0.035]">
        <Icon size={21} />
      </div>
      <p className="text-base font-semibold tracking-tight text-ink">{title}</p>
      {description && <p className="mt-1.5 max-w-md text-[13px] leading-relaxed text-mute">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </Card>
  );
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={cn("animate-pulse rounded-xl bg-gradient-to-r from-surface-2 via-primary/5 to-surface-2", className)} />;
}

export function PageSkeleton({ cards = 3 }: { cards?: number }) {
  return (
    <div className="space-y-4">
      <Skeleton className="h-8 w-48" />
      <Skeleton className="h-4 w-80" />
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        {Array.from({ length: cards }).map((_, i) => (
          <Skeleton key={i} className="h-28" />
        ))}
      </div>
      <Skeleton className="h-48" />
    </div>
  );
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <Card className="flex flex-col items-center py-10 text-center">
      <AlertCircle className="mb-2 text-danger" size={22} />
      <p className="text-sm font-medium text-ink">Não foi possível carregar</p>
      <p className="mt-1 text-[13px] text-mute">{message}</p>
      {onRetry && (
        <Button className="mt-4" variant="secondary" onClick={onRetry}>
          Tentar novamente
        </Button>
      )}
    </Card>
  );
}

export function Toggle({
  checked,
  onChange,
  label,
  disabled,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  label?: string;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        "relative inline-flex h-7 w-11 shrink-0 items-center rounded-full transition sm:h-5 sm:w-9",
        focusRing,
        checked ? "bg-primary" : "bg-slate-300",
        disabled && "cursor-not-allowed opacity-50",
      )}
    >
      <span
        className={cn(
          "inline-block h-5 w-5 rounded-full bg-white shadow-sm transition-transform sm:h-4 sm:w-4",
          checked ? "translate-x-5 sm:translate-x-4" : "translate-x-1 sm:translate-x-0.5",
        )}
      />
    </button>
  );
}

export function Badge({
  children,
  tone = "neutral",
}: {
  children: ReactNode;
  tone?: "neutral" | "accent" | "warn" | "danger" | "ok";
}) {
  const tones = {
    neutral: "bg-surface-2 text-mute",
    accent: "bg-primary/10 text-primary-600",
    warn: "bg-amber-50 text-warn dark:bg-amber-400/10",
    danger: "bg-rose-50 text-danger dark:bg-rose-400/10",
    ok: "bg-emerald-50 text-ok dark:bg-emerald-400/10",
  };
  return <span className={cn("inline-flex items-center rounded-full px-2.5 py-1 text-[11px] font-medium leading-none", tones[tone])}>{children}</span>;
}

export function TableWrap({ children }: { children: ReactNode }) {
  return <div className="overflow-x-auto rounded-[1.125rem] border border-line/90 bg-surface shadow-[var(--shadow-card)]">{children}</div>;
}

export function Table({ children, className }: { children: ReactNode; className?: string }) {
  return <table className={cn("w-full text-left text-sm", className)}>{children}</table>;
}

export function Th({ children, numeric }: { children: ReactNode; numeric?: boolean }) {
  return <th className={cn("whitespace-nowrap px-4 py-3 text-[11px] font-semibold uppercase tracking-[0.06em] text-mute", numeric && "text-right")}>{children}</th>;
}

export function Td({ children, numeric }: { children: ReactNode; numeric?: boolean }) {
  return <td className={cn("px-4 py-3.5", numeric && "text-right tabular-nums")}>{children}</td>;
}

export function cellValue(v: unknown) {
  if (v == null || v === "") return "—";
  if (typeof v === "number") return formatPt(v);
  if (isNumericValue(v)) return formatPt(Number(String(v).replace(",", ".")));
  return String(v);
}

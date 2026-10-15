"use client";

import Link from "next/link";
import { AlertCircle, Inbox, Loader2, type LucideIcon } from "lucide-react";
import { cn, formatPt, isNumericValue } from "@/lib/cn";
import type { ButtonHTMLAttributes, HTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";

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
    primary: "bg-primary text-white shadow-[0_8px_20px_-8px_rgba(37,99,235,0.6)] hover:bg-primary-600 disabled:opacity-60",
    secondary: "bg-surface-2 text-ink ring-1 ring-inset ring-line/70 hover:bg-surface hover:ring-primary/30",
    ghost: "text-mute hover:bg-surface-2 hover:text-ink",
    danger: "bg-rose-50 text-danger ring-1 ring-inset ring-rose-200/60 hover:bg-rose-100 dark:bg-rose-500/10 dark:ring-rose-400/20 dark:hover:bg-rose-500/20",
  };
  const sizes = {
    sm: "min-h-10 px-3.5 text-[12px] sm:min-h-9",
    md: "min-h-11 px-5 text-sm sm:min-h-10",
    lg: "min-h-12 px-6 text-sm",
    icon: "h-10 w-10 min-h-10 p-0 sm:h-9 sm:w-9 sm:min-h-9",
  };
  return (
    <button
      type={type}
      disabled={disabled || busy}
      className={cn(
        "inline-flex items-center justify-center gap-2 rounded-full font-medium transition-all duration-150 active:scale-[0.98] disabled:cursor-not-allowed",
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

export function Card({ className, children, ...props }: HTMLAttributes<HTMLDivElement> & { children: ReactNode }) {
  return <div className={cn("card-float border border-transparent p-5 transition-colors duration-200 sm:p-6", className)} {...props}>{children}</div>;
}

export function CardTitle({ children }: { children: ReactNode }) {
  return <h2 className="mb-4 text-[17px] font-semibold tracking-tight text-ink">{children}</h2>;
}

const fieldCls =
  "w-full min-h-11 rounded-2xl border border-transparent bg-surface-2 px-4 py-2 text-sm text-ink placeholder:text-mute/80 outline-none transition-[border-color,box-shadow,background-color] focus:border-primary/40 focus:bg-surface focus:ring-4 focus:ring-primary/10 disabled:cursor-not-allowed disabled:opacity-60 sm:min-h-10 dark:border-line/60";

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
      <span className="flex items-center gap-1">{label}
      {required && <span className="ml-0.5 text-accent">*</span>}
      </span>
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
        <h1 className="text-[1.75rem] font-bold tracking-[-0.035em] text-ink sm:text-[2.25rem] sm:leading-[1.1]">{title}</h1>
        {description && <p className="mt-2 max-w-2xl text-sm leading-relaxed text-mute">{description}</p>}
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
    <Card className="flex flex-col items-center border-dashed border-line bg-transparent py-14 text-center shadow-none">
      <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-primary/10 text-primary ring-8 ring-primary/[0.05]">
        <Icon size={21} />
      </div>
      <p className="text-base font-semibold tracking-tight text-ink">{title}</p>
      {description && <p className="mt-1.5 max-w-md text-[13px] leading-relaxed text-mute">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </Card>
  );
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={cn("animate-pulse rounded-2xl bg-gradient-to-r from-surface via-primary/5 to-surface", className)} />;
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
        checked ? "bg-primary" : "bg-line",
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
    accent: "bg-primary/10 text-primary-600 dark:text-primary",
    warn: "bg-amber-50 text-warn dark:bg-amber-400/10",
    danger: "bg-rose-50 text-danger dark:bg-rose-400/10",
    ok: "bg-emerald-50 text-ok dark:bg-emerald-400/10",
  };
  return <span className={cn("inline-flex items-center rounded-full px-2.5 py-1 text-[11px] font-medium leading-none", tones[tone])}>{children}</span>;
}

export function TableWrap({ children }: { children: ReactNode }) {
  return <div className="card-float overflow-x-auto">{children}</div>;
}

export function Table({ children, className }: { children: ReactNode; className?: string }) {
  return <table className={cn("w-full text-left text-sm", className)}>{children}</table>;
}

export function Th({ children, numeric }: { children: ReactNode; numeric?: boolean }) {
  return <th className={cn("whitespace-nowrap bg-surface-2/60 px-5 py-3 text-[11px] font-semibold uppercase tracking-[0.06em] text-mute first:rounded-tl-[var(--radius-lg)] last:rounded-tr-[var(--radius-lg)]", numeric && "text-right")}>{children}</th>;
}

export function Td({ children, numeric }: { children: ReactNode; numeric?: boolean }) {
  return <td className={cn("border-t border-line/70 px-5 py-3.5 text-ink", numeric && "text-right tabular-nums")}>{children}</td>;
}

export function cellValue(v: unknown) {
  if (v == null || v === "") return "—";
  if (typeof v === "number") return formatPt(v);
  if (isNumericValue(v)) return formatPt(Number(String(v).replace(",", ".")));
  return String(v);
}

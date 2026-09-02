"use client";

import Link from "next/link";
import { useState, type ReactNode } from "react";
import { Eye, EyeOff, Lock, Mail, Building2, User } from "lucide-react";
import { Logo } from "@/components/brand";

export function AuthSplit({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen bg-white">
      <div className="flex w-full flex-col lg:w-1/2">{children}</div>
      <MarketingPanel />
    </div>
  );
}

export function AuthFormShell({
  mode,
  children,
}: {
  mode: "login" | "signup";
  children: ReactNode;
}) {
  return (
    <AuthSplit>
      <div className="flex min-h-screen flex-col px-8 py-8 sm:px-12 lg:px-16">
        <Link href="/login" aria-label="TheDobra" className="inline-flex w-fit">
          <Logo variant="light" size={32} />
        </Link>
        <div className="mx-auto flex w-full max-w-[400px] flex-1 flex-col justify-center py-10">
          <h1 className="text-center text-[28px] font-semibold tracking-tight text-ink">Bem-vindo à TheDobra</h1>
          <p className="mt-2 text-center text-[13px] leading-relaxed text-mute">
            Comece a sua experiência na TheDobra — entre ou crie uma conta.
          </p>
          <div className="mx-auto mt-6 mb-7 inline-flex rounded-full border border-line bg-surface-2/80 p-1">
            <Link
              href="/login"
              className={`rounded-full px-5 py-1.5 text-[13px] font-medium transition ${
                mode === "login" ? "bg-white text-ink shadow-sm" : "text-mute hover:text-ink"
              }`}
            >
              Entrar
            </Link>
            <Link
              href="/signup"
              className={`rounded-full px-5 py-1.5 text-[13px] font-medium transition ${
                mode === "signup" ? "bg-white text-ink shadow-sm" : "text-mute hover:text-ink"
              }`}
            >
              Criar conta
            </Link>
          </div>
          {children}
        </div>
      </div>
    </AuthSplit>
  );
}

export function AuthCard({ title, subtitle, children }: { title: string; subtitle: string; children: ReactNode }) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-bg px-4">
      <div className="w-full max-w-sm rounded-2xl border border-line bg-surface p-6 shadow-sm">
        <Link href="/login" className="mb-6 inline-flex">
          <Logo variant="light" size={28} />
        </Link>
        <h1 className="text-xl font-semibold text-ink">{title}</h1>
        <p className="mt-1 mb-5 text-[13px] text-mute">{subtitle}</p>
        {children}
      </div>
    </div>
  );
}

const icons = {
  email: Mail,
  lock: Lock,
  user: User,
  org: Building2,
};

export function Field({
  label,
  value,
  onChange,
  type = "text",
  placeholder,
  required,
  icon,
  error,
  minLength,
  autoComplete,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  placeholder?: string;
  required?: boolean;
  icon?: keyof typeof icons;
  error?: string;
  minLength?: number;
  autoComplete?: string;
}) {
  const [show, setShow] = useState(false);
  const isPassword = type === "password";
  const Icon = icon ? icons[icon] : null;
  return (
    <label className="block text-[13px] font-medium text-ink">
      {label}
      {required && <span className="ml-0.5 text-accent">*</span>}
      <span className="relative mt-1.5 block">
        {Icon && (
          <Icon size={16} className="pointer-events-none absolute top-1/2 left-3.5 -translate-y-1/2 text-slate-400" />
        )}
        <input
          type={isPassword && show ? "text" : type}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          required={required}
          minLength={minLength}
          autoComplete={autoComplete}
          aria-invalid={error ? true : undefined}
          className={`min-h-11 w-full rounded-xl border bg-white py-2.5 text-sm text-ink outline-none placeholder:text-slate-400 focus:ring-2 focus:ring-accent/15 ${
            error ? "border-danger focus:border-danger" : "border-line focus:border-accent/50"
          } ${Icon ? "pl-10" : "px-3.5"} ${isPassword ? "pr-11" : "pr-3.5"}`}
        />
        {isPassword && (
          <button
            type="button"
            onClick={() => setShow((s) => !s)}
            className="absolute top-1/2 right-3 flex h-9 w-9 -translate-y-1/2 items-center justify-center text-slate-400 hover:text-ink"
            aria-label={show ? "Ocultar senha" : "Mostrar senha"}
          >
            {show ? <EyeOff size={16} /> : <Eye size={16} />}
          </button>
        )}
      </span>
      {error && <span className="mt-1 block text-[12px] font-normal text-danger">{error}</span>}
    </label>
  );
}

export function PrimaryButton({
  children,
  busy,
  disabled,
}: {
  children: ReactNode;
  busy?: boolean;
  disabled?: boolean;
}) {
  return (
    <button
      type="submit"
      disabled={busy || disabled}
      className="w-full min-h-11 rounded-xl bg-accent py-3 text-sm font-medium text-white transition hover:bg-accent-2 disabled:opacity-60"
    >
      {children}
    </button>
  );
}

export function MarketingPanel() {
  return (
    <aside className="panel-gradient relative hidden overflow-hidden px-12 py-14 lg:flex lg:w-1/2 lg:flex-col lg:items-center lg:justify-between">
      <div className="grid-fade pointer-events-none absolute inset-0 opacity-70" />
      <div className="pointer-events-none absolute -top-32 -right-24 h-96 w-96 rounded-full bg-indigo-500/25 blur-3xl" />
      <div className="pointer-events-none absolute -bottom-32 -left-24 h-96 w-96 rounded-full bg-sky-500/20 blur-3xl" />
      <div className="pointer-events-none absolute -top-24 -right-16 h-72 w-72 rounded-3xl border border-white/8" />
      <div className="pointer-events-none absolute bottom-40 -left-10 h-56 w-56 rounded-3xl border border-white/8" />

      <div className="relative z-10 mt-6 h-[380px] w-full max-w-lg">
        <div className="glass-card absolute top-0 right-6 w-[280px] rotate-2 rounded-2xl p-4">
          <div className="text-[11px] font-medium text-white/70">Receita este mês</div>
          <div className="mt-3 flex items-center gap-4">
            <Donut />
            <div>
              <div className="text-xl font-semibold text-white">R$ 248.320</div>
              <div className="mt-1 text-[11px] text-emerald-300">+12,4% vs. mês anterior</div>
              <div className="mt-2 space-y-1 text-[10px] text-white/70">
                <div className="flex items-center gap-1.5">
                  <span className="h-1.5 w-1.5 rounded-full bg-indigo-400" /> Directo
                </div>
                <div className="flex items-center gap-1.5">
                  <span className="h-1.5 w-1.5 rounded-full bg-sky-400" /> Parceiros
                </div>
                <div className="flex items-center gap-1.5">
                  <span className="h-1.5 w-1.5 rounded-full bg-amber-400" /> Recorrente
                </div>
              </div>
            </div>
          </div>
        </div>

        <div className="glass-card absolute top-36 left-0 w-[250px] -rotate-3 rounded-2xl p-4">
          <div className="text-[11px] font-medium text-white/70">Dashboards activos</div>
          <div className="mt-1 text-xl font-semibold text-white">17</div>
          <div className="mt-3 space-y-2 text-[12px] text-white/85">
            <Row name="Vendas executivo" value="+8,2%" />
            <Row name="Funil comercial" value="+3,1%" />
            <Row name="Churn e retenção" value="-1,4%" down />
          </div>
        </div>

        <div className="glass-card absolute top-52 right-0 w-[220px] rotate-3 rounded-2xl p-4">
          <div className="text-[11px] font-medium text-white/70">Previsões</div>
          <div className="mt-3 space-y-3">
            <Bar label="Receita Q4" pct={78} color="#818cf8" />
            <Bar label="Pipeline" pct={54} color="#38bdf8" />
            <Bar label="Meta anual" pct={91} color="#fbbf24" />
          </div>
        </div>
      </div>

      <div className="relative z-10 mt-auto flex flex-col items-center text-center">
        <Logo variant="dark" size={44} />
        <p className="mt-6 max-w-sm text-[22px] leading-snug font-semibold text-white">
          O analista digital do seu negócio
        </p>
      </div>
    </aside>
  );
}

function Donut() {
  return (
    <div
      className="h-16 w-16 shrink-0 rounded-full"
      style={{
        background:
          "conic-gradient(#818cf8 0 48%, #38bdf8 48% 74%, #fbbf24 74% 100%)",
        mask: "radial-gradient(farthest-side, transparent 58%, #000 59%)",
        WebkitMask: "radial-gradient(farthest-side, transparent 58%, #000 59%)",
      }}
    />
  );
}

function Row({ name, value, down }: { name: string; value: string; down?: boolean }) {
  return (
    <div className="flex items-center justify-between">
      <span className="truncate">{name}</span>
      <span className={down ? "text-rose-300" : "text-emerald-300"}>{value}</span>
    </div>
  );
}

function Bar({ label, pct, color }: { label: string; pct: number; color: string }) {
  return (
    <div>
      <div className="mb-1 flex justify-between text-[11px] text-white/80">
        <span>{label}</span>
        <span>{pct}%</span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-white/15">
        <div className="h-full rounded-full" style={{ width: `${pct}%`, background: color }} />
      </div>
    </div>
  );
}

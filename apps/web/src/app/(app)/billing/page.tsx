"use client";

import { useQuery, useMutation } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { Kpi } from "@/components/viz";
import { toast } from "sonner";
import { Button, ErrorState, PageHeader, PageSkeleton, formatPt, cn } from "@/components/ui";
import { planLabel } from "@/lib/labels";
import { Check } from "lucide-react";

const fmtUsage = (used: number | undefined, max: number | undefined) =>
  `${formatPt(used ?? 0)}${typeof max === "number" && max >= 0 ? ` / ${formatPt(max)}` : ""}`;

function fmtBytes(n?: number) {
  if (n == null) return "—";
  if (n < 0) return "Ilimitado";
  if (n < 1024 * 1024) return `${n} B`;
  if (n < 1024 * 1024 * 1024) {
    const mb = n / (1024 * 1024);
    return Number.isInteger(mb) ? `${mb} MB` : `${mb.toFixed(1)} MB`;
  }
  const gb = n / (1024 * 1024 * 1024);
  return Number.isInteger(gb) ? `${gb} GB` : `${gb.toFixed(1)} GB`;
}

function fmtLimit(n?: number, unlimited = "Ilimitado") {
  if (n == null) return "—";
  if (n < 0) return unlimited;
  return formatPt(n);
}

export default function BillingPage() {
  const usage = useQuery({ queryKey: ["usage"], queryFn: () => api<any>("/api/v1/usage") });
  const cfg = useQuery({ queryKey: ["billcfg"], queryFn: () => api<any>("/api/v1/billing/config") });
  const st = useQuery({ queryKey: ["billst"], queryFn: () => api<any>("/api/v1/billing/status") });
  const checkout = useMutation({
    mutationFn: (plan: string) => api<{ url: string }>("/api/v1/billing/checkout", { method: "POST", body: JSON.stringify({ plan }) }),
    onSuccess: (d) => {
      window.location.href = d.url;
    },
    onError: (e: Error) => toast.error(e.message),
  });
  const portal = useMutation({
    mutationFn: () => api<{ url: string }>("/api/v1/billing/portal", { method: "POST" }),
    onSuccess: (d) => {
      window.location.href = d.url;
    },
    onError: (e: Error) => toast.error(e.message),
  });
  const u = usage.data;
  const plans = normalizeArray(cfg.data?.plans);
  const current = st.data?.plan || u?.plan || "gratis";
  if (usage.isLoading) return <PageSkeleton cards={4} />;
  if (usage.isError) return <ErrorState message={(usage.error as Error).message} onRetry={() => usage.refetch()} />;
  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <PageHeader
        title="Faturação"
        description={`Plano ${planLabel(current)}${cfg.data?.enabled ? " · Stripe ligado" : " · defina STRIPE_SECRET_KEY para checkout vivo"}`}
      />
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <Kpi label="Créditos de IA" value={fmtUsage(u?.ai_messages, u?.limits?.ai)} />
        <Kpi label="Dashboards" value={fmtUsage(u?.dashboards, u?.limits?.dashboards)} />
        <Kpi label="Bases de dados" value={fmtUsage(u?.datasets, u?.limits?.datasets)} />
        <Kpi label="Armazenamento" value={`${fmtBytes(u?.storage_bytes)} / ${fmtBytes(u?.limits?.storage_bytes)}`} />
      </div>
      {u?.limits && (
        <div className="card-float space-y-2 p-5 text-[12px] text-mute">
          <Quota label="Créditos de IA" used={u.ai_messages} max={u.limits.ai} />
          <Quota label="Dashboards" used={u.dashboards} max={u.limits.dashboards} />
          <Quota label="Bases de dados" used={u.datasets} max={u.limits.datasets} />
          <Quota label="Conectores externos" used={u.connectors} max={u.limits.connectors} />
          <Quota label="Armazenamento" used={u.storage_bytes} max={u.limits.storage_bytes} format={fmtBytes} />
          <Quota label="Utilizadores" used={u.users} max={u.limits.users} />
        </div>
      )}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
        {plans.map((p: any) => {
          const active = p.id === current;
          const free = !p.price_brl;
          return (
            <div key={p.id} className={cn("card-float flex flex-col p-6", active && "ring-2 ring-primary")}>
              <div className="text-[11px] font-semibold uppercase tracking-wide text-mute">{p.name}</div>
              <div className="mt-1 text-2xl font-bold tracking-tight text-ink">
                {free ? "R$ 0" : `R$ ${formatPt(p.price_brl)}`}
                <span className="ml-1 text-sm font-medium text-mute">/ mês</span>
              </div>
              <ul className="mt-4 flex-1 space-y-2">
                {(p.highlights || []).map((h: string) => (
                  <li key={h} className="flex items-start gap-2 text-[13px] text-ink">
                    <Check size={14} className="mt-0.5 shrink-0 text-primary" />
                    <span>{h}</span>
                  </li>
                ))}
                {!(p.highlights || []).length && (
                  <li className="text-[13px] text-mute">
                    {fmtLimit(p.users)} utilizadores · {fmtLimit(p.datasets)} bases · {fmtBytes(p.storage_bytes)}
                  </li>
                )}
              </ul>
              {active ? (
                <Button size="sm" className="mt-5" variant="secondary" disabled>
                  Plano actual
                </Button>
              ) : free ? (
                <p className="mt-5 text-[12px] text-mute">Incluído em todas as contas novas.</p>
              ) : (
                <Button size="sm" className="mt-5" onClick={() => checkout.mutate(p.id)} busy={checkout.isPending}>
                  Assinar {p.name}
                </Button>
              )}
            </div>
          );
        })}
      </div>
      {st.data?.stripe_customer && (
        <Button variant="secondary" onClick={() => portal.mutate()} busy={portal.isPending}>
          Abrir portal Stripe
        </Button>
      )}
    </div>
  );
}

function Quota({
  label,
  used,
  max,
  format,
}: {
  label: string;
  used: number;
  max: number;
  format?: (n?: number) => string;
}) {
  const pct = max < 0 ? 0 : Math.min(100, Math.round(((used || 0) / Math.max(max, 1)) * 100));
  const shown = format
    ? `${format(used)} · ${max < 0 ? "ilimitado" : format(max)}`
    : max < 0
      ? `${formatPt(used ?? 0)} · ilimitado`
      : fmtUsage(used, max);
  return (
    <div>
      <div className="mb-1 flex justify-between">
        <span>{label}</span>
        <span>{shown}</span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-surface-2">
        <div className="brand-gradient h-full rounded-full" style={{ width: max < 0 ? "8%" : `${pct}%` }} />
      </div>
    </div>
  );
}

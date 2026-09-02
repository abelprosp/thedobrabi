"use client";

import { useQuery, useMutation } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { Kpi } from "@/components/viz";
import { toast } from "sonner";
import { Button, ErrorState, PageHeader, PageSkeleton, formatPt } from "@/components/ui";
import { planLabel } from "@/lib/labels";

const fmtUsage = (used: number | undefined, max: number | undefined) =>
  `${formatPt(used ?? 0)}${typeof max === "number" && max >= 0 ? ` / ${formatPt(max)}` : ""}`;

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
  if (usage.isLoading) return <PageSkeleton cards={4} />;
  if (usage.isError) return <ErrorState message={(usage.error as Error).message} onRetry={() => usage.refetch()} />;
  return (
    <div className="mx-auto max-w-4xl space-y-5">
      <PageHeader
        title="Faturação"
        description={`Plano ${planLabel(st.data?.plan || u?.plan || "starter")}${cfg.data?.enabled ? " · Stripe ligado" : " · defina STRIPE_SECRET_KEY para checkout vivo"}`}
      />
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <Kpi label="Consultas" value={fmtUsage(u?.queries, u?.limits?.queries)} />
        <Kpi label="Conjuntos" value={fmtUsage(u?.datasets, u?.limits?.datasets)} />
        <Kpi label="Mensagens IA" value={fmtUsage(u?.ai_messages, u?.limits?.ai)} />
        <Kpi label="Utilizadores" value={fmtUsage(u?.users, u?.limits?.users)} />
      </div>
      {u?.limits && (
        <div className="space-y-2 rounded-2xl border border-line bg-surface p-5 text-[12px] text-mute shadow-sm">
          <Quota label="Consultas" used={u.queries} max={u.limits.queries} />
          <Quota label="Conjuntos" used={u.datasets} max={u.limits.datasets} />
          <Quota label="IA" used={u.ai_messages} max={u.limits.ai} />
          <Quota label="Utilizadores" used={u.users} max={u.limits.users} />
        </div>
      )}
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        {plans.map((p: any) => (
          <div key={p.id} className="rounded-2xl border border-line bg-surface p-5 shadow-sm">
            <div className="text-sm font-medium text-ink">{p.name}</div>
            <div className="mt-2 text-[12px] text-mute">
              {p.users < 0 ? "Ilimitado" : `${p.users} utilizadores`} · {p.queries < 0 ? "consultas ilimitadas" : `${p.queries.toLocaleString("pt-BR")} consultas`}
            </div>
            <Button size="sm" className="mt-4" onClick={() => checkout.mutate(p.id)} busy={checkout.isPending}>
              Assinar {p.name}
            </Button>
          </div>
        ))}
      </div>
      {st.data?.stripe_customer && (
        <Button variant="secondary" onClick={() => portal.mutate()} busy={portal.isPending}>
          Abrir portal Stripe
        </Button>
      )}
    </div>
  );
}

function Quota({ label, used, max }: { label: string; used: number; max: number }) {
  const pct = max < 0 ? 0 : Math.min(100, Math.round(((used || 0) / Math.max(max, 1)) * 100));
  return (
    <div>
      <div className="mb-1 flex justify-between">
        <span>{label}</span>
        <span>{max < 0 ? `${formatPt(used ?? 0)} · ilimitado` : fmtUsage(used, max)}</span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-surface-2">
        <div className="brand-gradient h-full rounded-full" style={{ width: max < 0 ? "8%" : `${pct}%` }} />
      </div>
    </div>
  );
}

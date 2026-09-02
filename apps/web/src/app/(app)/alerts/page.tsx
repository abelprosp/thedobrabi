"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { toast } from "sonner";
import { useState } from "react";
import { AlertTriangle } from "lucide-react";
import { Badge, Button, Card, EmptyState, ErrorState, FieldLabel, Input, PageHeader, PageSkeleton, Select } from "@/components/ui";

const channelLabel: Record<string, string> = {
  realtime: "tempo real",
  email: "e-mail",
  slack: "Slack",
  webhook: "webhook",
};

export default function AlertsPage() {
  const datasets = useQuery({ queryKey: ["datasets"], queryFn: () => api<any>("/api/v1/datasets") });
  const q = useQuery({ queryKey: ["alerts"], queryFn: () => api<any>("/api/v1/alerts") });
  const alerts = normalizeArray(q.data);
  const [name, setName] = useState("Queda de receita");
  const [measure, setMeasure] = useState("revenue");
  const [op, setOp] = useState("<");
  const [value, setValue] = useState("1");
  const [channels, setChannels] = useState<string[]>(["realtime", "email"]);
  const [busyId, setBusyId] = useState<string | null>(null);

  function toggle(ch: string) {
    setChannels((p) => (p.includes(ch) ? p.filter((x) => x !== ch) : [...p, ch]));
  }

  const create = useMutation({
    mutationFn: async () => {
      const ds = datasets.data?.[0]?.id;
      if (!ds) throw new Error("Carregue um conjunto primeiro");
      return api("/api/v1/alerts", {
        method: "POST",
        body: JSON.stringify({
          name,
          condition: { dataset_id: ds, measure, op, value: Number(value) },
          channels,
        }),
      });
    },
    onSuccess: () => {
      toast.success("Alerta criado");
      q.refetch();
    },
    onError: (e: Error) => toast.error(e.message),
  });
  async function evalAlert(id: string) {
    setBusyId(id);
    try {
      const r = await api<any>(`/api/v1/alerts/${id}/evaluate`, { method: "POST" });
      toast.message(r.triggered ? "Disparado" : "Não disparou", { description: `valor=${r.value}` });
    } catch (e: any) {
      toast.error(e.message);
    } finally {
      setBusyId(null);
    }
  }
  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <PageHeader title="Alertas" description="Receba avisos quando uma métrica cruzar um limiar." />
      <Card className="space-y-3">
        <FieldLabel label="Nome" required>
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Ex.: Queda de receita" />
        </FieldLabel>
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
          <FieldLabel label="Métrica">
            <Input value={measure} onChange={(e) => setMeasure(e.target.value)} />
          </FieldLabel>
          <FieldLabel label="Operador">
            <Select value={op} onChange={(e) => setOp(e.target.value)}>
              <option value="<">&lt;</option>
              <option value=">">&gt;</option>
              <option value="<=">&lt;=</option>
              <option value=">=">&gt;=</option>
            </Select>
          </FieldLabel>
          <FieldLabel label="Valor">
            <Input value={value} onChange={(e) => setValue(e.target.value)} inputMode="decimal" />
          </FieldLabel>
        </div>
        <div className="flex flex-wrap gap-3 text-[12px] text-mute">
          {["realtime", "email", "slack", "webhook"].map((ch) => (
            <label key={ch} className="flex min-h-9 items-center gap-1.5">
              <input type="checkbox" checked={channels.includes(ch)} onChange={() => toggle(ch)} />
              {channelLabel[ch]}
            </label>
          ))}
        </div>
        <p className="text-[11px] text-mute">E-mail, Slack e webhook usam SMTP_HOST / SLACK_WEBHOOK_URL / ALERT_WEBHOOK_URL no servidor.</p>
        <Button onClick={() => create.mutate()} busy={create.isPending}>
          Criar alerta
        </Button>
      </Card>
      {q.isLoading && <PageSkeleton cards={2} />}
      {q.isError && <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />}
      {alerts.length === 0 && (
        <div className="space-y-4">
          <EmptyState
            icon={AlertTriangle}
            title="Ainda sem alertas"
            description="Os alertas monitorizam métricas e disparam quando cruzam um limiar. Comece por carregar dados e criar uma métrica."
          />
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <Card className="space-y-1 p-4">
              <div className="text-[13px] font-medium text-ink">1. Dados</div>
              <div className="text-[12px] text-mute">Carregue um conjunto em /data.</div>
            </Card>
            <Card className="space-y-1 p-4">
              <div className="text-[13px] font-medium text-ink">2. Métrica</div>
              <div className="text-[12px] text-mute">Defina uma medida no modelo semântico.</div>
            </Card>
            <Card className="space-y-1 p-4">
              <div className="text-[13px] font-medium text-ink">3. Limiar</div>
              <div className="text-[12px] text-mute">Escolha um operador e um valor de disparo.</div>
            </Card>
          </div>
        </div>
      )}
      {alerts.map((a) => (
        <Card key={a.id} className="flex items-center justify-between gap-3">
          <div>
            <div className="text-sm text-ink">{a.name}</div>
            <div className="mt-1 flex flex-wrap gap-1.5">
              <Badge tone={a.enabled ? "ok" : "neutral"}>{a.enabled ? "activo" : "inactivo"}</Badge>
              {a.channels &&
                (typeof a.channels === "string" ? a.channels.split(",") : a.channels).map((ch: string) => (
                  <Badge key={ch}>{channelLabel[ch] || ch}</Badge>
                ))}
            </div>
          </div>
          <Button variant="secondary" size="sm" busy={busyId === a.id} onClick={() => evalAlert(a.id)}>
            Avaliar
          </Button>
        </Card>
      ))}
    </div>
  );
}

function EducationalCard({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-line bg-white p-4 shadow-sm">
      <div className="text-sm font-medium text-ink">{title}</div>
      <p className="mt-1 text-[12px] text-mute">{body}</p>
    </div>
  );
}

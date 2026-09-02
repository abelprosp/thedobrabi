"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Clock, Pause, Play, RefreshCw, Trash2 } from "lucide-react";
import { api, normalizeArray } from "@/lib/api";
import { Badge, Button, Card, CardTitle, FieldLabel, Input, Select } from "@/components/ui";
import {
  FREQUENCY_OPTIONS,
  TIMEZONE_OPTIONS,
  WEEKDAY_OPTIONS,
  canEditSchedules,
  cdcCapable,
  formatRelativePt,
  frequencyLabel,
  type ScheduleKind,
  type SyncSchedule,
  type SyncScheduleRun,
} from "@/lib/schedules";
import { statusLabel } from "@/lib/labels";

export function AutoRefreshCard({
  kind,
  targetId,
  targetType,
  canEdit,
}: {
  kind: ScheduleKind;
  targetId: string;
  targetType?: string;
  canEdit?: boolean;
}) {
  const qc = useQueryClient();
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<{ role?: string }>("/api/v1/auth/me") });
  const writable = canEdit ?? canEditSchedules(me.data?.role);
  const list = useQuery({
    queryKey: ["sync-schedules", kind, targetId],
    queryFn: () => api<SyncSchedule[]>(`/api/v1/sync-schedules?kind=${kind}&target_id=${targetId}`),
    enabled: Boolean(targetId),
  });
  const schedule = useMemo(() => {
    const rows = normalizeArray<SyncSchedule>(list.data);
    return rows[0] || null;
  }, [list.data]);

  const runs = useQuery({
    queryKey: ["sync-schedule-runs", schedule?.id],
    queryFn: () => api<SyncScheduleRun[]>(`/api/v1/sync-schedules/${schedule!.id}/runs`),
    enabled: Boolean(schedule?.id),
  });
  const recent = normalizeArray<SyncScheduleRun>(runs.data)[0];

  const [frequency, setFrequency] = useState("hourly");
  const [timezone, setTimezone] = useState("America/Sao_Paulo");
  const [hour, setHour] = useState(6);
  const [weekday, setWeekday] = useState(1);
  const [incremental, setIncremental] = useState(true);

  useEffect(() => {
    if (!schedule) return;
    setFrequency(schedule.frequency || "hourly");
    setTimezone(schedule.timezone || "America/Sao_Paulo");
    setHour(schedule.hour_local ?? 6);
    setWeekday(schedule.weekday ?? 1);
    setIncremental(schedule.incremental !== false);
  }, [schedule?.id]);

  const save = useMutation({
    mutationFn: () =>
      api<SyncSchedule>("/api/v1/sync-schedules", {
        method: "POST",
        body: JSON.stringify({
          kind,
          target_id: targetId,
          enabled: schedule?.enabled ?? true,
          frequency,
          timezone,
          hour_local: hour,
          weekday,
          incremental: cdcCapable(targetType) ? incremental : false,
        }),
      }),
    onSuccess: () => {
      toast.success("Actualização automática guardada");
      qc.invalidateQueries({ queryKey: ["sync-schedules"] });
      qc.invalidateQueries({ queryKey: ["sources"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const pause = useMutation({
    mutationFn: (id: string) => api<SyncSchedule>(`/api/v1/sync-schedules/${id}/pause`, { method: "POST" }),
    onSuccess: () => {
      toast.success("Em pausa");
      qc.invalidateQueries({ queryKey: ["sync-schedules"] });
      qc.invalidateQueries({ queryKey: ["sources"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const resume = useMutation({
    mutationFn: (id: string) => api<SyncSchedule>(`/api/v1/sync-schedules/${id}/resume`, { method: "POST" }),
    onSuccess: () => {
      toast.success("Retomada");
      qc.invalidateQueries({ queryKey: ["sync-schedules"] });
      qc.invalidateQueries({ queryKey: ["sources"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const runNow = useMutation({
    mutationFn: (id: string) => api<SyncSchedule>(`/api/v1/sync-schedules/${id}/run`, { method: "POST" }),
    onSuccess: (sc) => {
      if (sc.last_status === "error") toast.error(sc.last_error || "A sincronização falhou");
      else toast.success("Sincronização concluída");
      qc.invalidateQueries({ queryKey: ["sync-schedules"] });
      qc.invalidateQueries({ queryKey: ["sync-schedule-runs"] });
      qc.invalidateQueries({ queryKey: ["sources"] });
      qc.invalidateQueries({ queryKey: ["datasets"] });
      qc.invalidateQueries({ queryKey: ["source"] });
      qc.invalidateQueries({ queryKey: ["dataset"] });
      qc.invalidateQueries({ queryKey: ["flows"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const remove = useMutation({
    mutationFn: (id: string) => api(`/api/v1/sync-schedules/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success("Actualização automática removida");
      qc.invalidateQueries({ queryKey: ["sync-schedules"] });
      qc.invalidateQueries({ queryKey: ["sources"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const statusTone =
    schedule?.last_status === "ok" ? "ok" : schedule?.last_status === "error" ? "danger" : schedule?.last_status === "running" ? "warn" : "neutral";
  const showTimeFields = frequency === "daily" || frequency === "weekly";
  const incrAvailable = cdcCapable(targetType);

  return (
    <Card className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <CardTitle>Actualização automática</CardTitle>
          <p className="text-[13px] text-mute">
            O processo da API dispara syncs de conectores, materializações de flows e refresh de conjuntos no horário definido.
          </p>
        </div>
        {schedule ? (
          <Badge tone={schedule.enabled ? (statusTone as "ok" | "danger" | "warn" | "neutral") : "neutral"}>
            {schedule.enabled ? statusLabel(schedule.last_status || "idle") : "Em pausa"}
          </Badge>
        ) : (
          <Badge tone="neutral">Desligada</Badge>
        )}
      </div>

      {schedule && (
        <div className="grid grid-cols-1 gap-2 rounded-xl border border-line bg-bg px-3 py-3 text-[13px] sm:grid-cols-3">
          <div>
            <div className="text-[11px] font-medium uppercase tracking-wide text-mute">Último run</div>
            <div className="mt-0.5 font-medium text-ink">{formatRelativePt(schedule.last_run_at)}</div>
            {schedule.last_mode && <div className="text-[12px] text-mute">{schedule.last_mode === "incremental" ? "incremental (CDC)" : "carga completa"}</div>}
          </div>
          <div>
            <div className="text-[11px] font-medium uppercase tracking-wide text-mute">Próximo run</div>
            <div className="mt-0.5 font-medium text-ink">{schedule.enabled ? formatRelativePt(schedule.next_run_at) : "—"}</div>
            <div className="text-[12px] text-mute">{frequencyLabel(schedule.frequency)}</div>
          </div>
          <div>
            <div className="text-[11px] font-medium uppercase tracking-wide text-mute">Estado</div>
            {schedule.last_status === "error" && schedule.last_error ? (
              <div className="mt-0.5 text-danger">{schedule.last_error}</div>
            ) : (
              <div className="mt-0.5 font-medium text-ink">{recent?.rows_affected != null ? `${recent.rows_affected.toLocaleString("pt-PT")} linhas` : statusLabel(schedule.last_status)}</div>
            )}
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <FieldLabel label="Frequência">
          <Select value={frequency} onChange={(e) => setFrequency(e.target.value)} disabled={!writable}>
            {FREQUENCY_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </Select>
        </FieldLabel>
        <FieldLabel label="Fuso horário">
          <Select value={timezone} onChange={(e) => setTimezone(e.target.value)} disabled={!writable}>
            {TIMEZONE_OPTIONS.map((tz) => (
              <option key={tz} value={tz}>
                {tz}
              </option>
            ))}
          </Select>
        </FieldLabel>
        {showTimeFields && (
          <FieldLabel label="Hora local" hint="Usada em cadências diárias e semanais.">
            <Input type="number" min={0} max={23} value={hour} onChange={(e) => setHour(Number(e.target.value))} disabled={!writable} />
          </FieldLabel>
        )}
        {frequency === "weekly" && (
          <FieldLabel label="Dia da semana">
            <Select value={String(weekday)} onChange={(e) => setWeekday(Number(e.target.value))} disabled={!writable}>
              {WEEKDAY_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </Select>
          </FieldLabel>
        )}
      </div>

      {incrAvailable && (
        <label className="flex items-start gap-2 text-[13px] text-ink">
          <input
            type="checkbox"
            className="mt-0.5"
            checked={incremental}
            disabled={!writable}
            onChange={(e) => setIncremental(e.target.checked)}
          />
          <span>
            Incremental quando o CDC estiver disponível (PostgreSQL / MySQL). Caso contrário a carga é completa e substitui o conjunto existente.
          </span>
        </label>
      )}

      {writable && (
        <div className="flex flex-wrap gap-2">
          <Button onClick={() => save.mutate()} busy={save.isPending}>
            <Clock size={14} /> {schedule ? "Guardar" : "Activar"}
          </Button>
          {schedule && (
            <>
              {schedule.enabled ? (
                <Button variant="secondary" onClick={() => pause.mutate(schedule.id)} busy={pause.isPending}>
                  <Pause size={14} /> Pausar
                </Button>
              ) : (
                <Button variant="secondary" onClick={() => resume.mutate(schedule.id)} busy={resume.isPending}>
                  <Play size={14} /> Retomar
                </Button>
              )}
              <Button variant="secondary" onClick={() => runNow.mutate(schedule.id)} busy={runNow.isPending}>
                <RefreshCw size={14} /> Sincronizar agora
              </Button>
              <Button
                variant="ghost"
                className="text-danger"
                onClick={() => {
                  if (confirm("Remover a actualização automática?")) remove.mutate(schedule.id);
                }}
                busy={remove.isPending}
              >
                <Trash2 size={14} /> Remover
              </Button>
            </>
          )}
        </div>
      )}
      {!writable && <p className="text-[12px] text-mute">Leitores só consultam o calendário. Peça a um analista para alterar a frequência.</p>}
    </Card>
  );
}

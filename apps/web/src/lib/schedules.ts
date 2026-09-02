export type ScheduleKind = "connector" | "flow" | "dataset";

export type SyncSchedule = {
  id: string;
  org_id?: string;
  workspace_id?: string;
  kind: ScheduleKind;
  target_id: string;
  enabled: boolean;
  frequency: "15m" | "hourly" | "daily" | "weekly" | string;
  hour_local: number;
  weekday: number;
  timezone: string;
  incremental: boolean;
  table_name?: string;
  last_run_at?: string | null;
  next_run_at?: string | null;
  last_status: string;
  last_error?: string | null;
  last_mode?: string;
  target_name?: string;
  target_type?: string;
  created_at?: string;
  updated_at?: string;
};

export type SyncScheduleRun = {
  id: string;
  schedule_id: string;
  status: string;
  mode: string;
  error?: string | null;
  rows_affected?: number | null;
  started_at: string;
  finished_at?: string | null;
};

export const FREQUENCY_OPTIONS = [
  { value: "15m", label: "A cada 15 minutos" },
  { value: "hourly", label: "De hora a hora" },
  { value: "daily", label: "Diário" },
  { value: "weekly", label: "Semanal" },
] as const;

export const WEEKDAY_OPTIONS = [
  { value: 0, label: "Domingo" },
  { value: 1, label: "Segunda" },
  { value: 2, label: "Terça" },
  { value: 3, label: "Quarta" },
  { value: 4, label: "Quinta" },
  { value: 5, label: "Sexta" },
  { value: 6, label: "Sábado" },
] as const;

export const TIMEZONE_OPTIONS = [
  "America/Sao_Paulo",
  "America/Fortaleza",
  "America/Recife",
  "America/Manaus",
  "America/Belem",
  "America/Noronha",
  "UTC",
  "Europe/Lisbon",
] as const;

export function frequencyLabel(v?: string | null) {
  return FREQUENCY_OPTIONS.find((o) => o.value === v)?.label || v || "—";
}

export function weekdayLabel(v?: number | null) {
  return WEEKDAY_OPTIONS.find((o) => o.value === v)?.label ?? "—";
}

export function formatRelativePt(iso?: string | null) {
  if (!iso) return "nunca";
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return iso;
  const diffMs = t - Date.now();
  const rtf = new Intl.RelativeTimeFormat("pt-PT", { numeric: "auto" });
  const abs = Math.abs(diffMs);
  const min = Math.round(abs / 60_000);
  if (min < 1) return diffMs >= 0 ? "dentro de instantes" : "agora mesmo";
  if (min < 60) return rtf.format(Math.round(diffMs / 60_000), "minute");
  const hr = Math.round(min / 60);
  if (hr < 48) return rtf.format(Math.round(diffMs / 3_600_000), "hour");
  return rtf.format(Math.round(diffMs / 86_400_000), "day");
}

export function cdcCapable(type?: string | null) {
  const t = (type || "").toLowerCase();
  return t === "postgres" || t === "postgresql" || t === "pg" || t === "mysql" || t === "mariadb" || t === "supabase";
}

export function canEditSchedules(role?: string | null) {
  return role === "owner" || role === "admin" || role === "analyst";
}

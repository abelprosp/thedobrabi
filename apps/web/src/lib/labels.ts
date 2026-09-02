/** Rótulos em português para valores técnicos devolvidos pela API. */

export const ROLE_LABELS: Record<string, string> = {
  owner: "Proprietário",
  admin: "Administrador",
  analyst: "Analista",
  viewer: "Leitor",
};

export const PLAN_LABELS: Record<string, string> = {
  essencial: "Essencial",
  pro: "Pro",
  completo: "Completo",
  starter: "Essencial",
  growth: "Pro",
  business: "Completo",
  enterprise: "Completo",
};

export const STATUS_LABELS: Record<string, string> = {
  ready: "Pronto",
  ready_ok: "Pronto",
  pending: "Pendente",
  processing: "A processar",
  ingesting: "A ingerir",
  syncing: "A sincronizar",
  failed: "Falhou",
  error: "Erro",
  active: "Activo",
  inactive: "Inactivo",
  paused: "Em pausa",
  draft: "Rascunho",
  completed: "Concluído",
  idle: "Em espera",
  ok: "Sucesso",
  running: "A executar",
  synced: "Sincronizado",
  preview: "Preview",
  disabled: "Desactivado",
};

export function roleLabel(v?: string | null) {
  return (v && ROLE_LABELS[v]) || v || "—";
}

export function planLabel(v?: string | null) {
  return (v && PLAN_LABELS[v]) || v || "—";
}

export function statusLabel(v?: string | null) {
  return (v && STATUS_LABELS[v]) || v || "—";
}

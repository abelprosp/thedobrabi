import { api, apiStatus, normalizeArray } from "@/lib/api";
import type { Discovery, GoalPace, ManageAction, Memory, WeeklyReview } from "@/components/manage/types";

function missing(err: unknown) {
  const s = apiStatus(err);
  return s === 404 || s === 501;
}

async function tryGet<T>(path: string): Promise<T | null> {
  try {
    return await api<T>(path);
  } catch (err) {
    if (missing(err)) return null;
    throw err;
  }
}

function workspaceKey() {
  if (typeof window === "undefined") return "local";
  return localStorage.getItem("thedobra.workspace") || "local";
}

function storeKey(kind: string) {
  return `thedobra.manage.${workspaceKey()}.${kind}`;
}

function readStore<T>(kind: string, fallback: T): T {
  if (typeof window === "undefined") return fallback;
  try {
    const raw = localStorage.getItem(storeKey(kind));
    return raw ? (JSON.parse(raw) as T) : fallback;
  } catch {
    return fallback;
  }
}

function writeStore(kind: string, value: unknown) {
  if (typeof window === "undefined") return;
  localStorage.setItem(storeKey(kind), JSON.stringify(value));
}

function nextStep(kind: string, title: string) {
  const blob = `${kind} ${title}`.toLowerCase();
  if (blob.includes("despesa") || blob.includes("custo")) return "Investigar lançamentos";
  if (kind === "opportunity") return "Conferir estoque e disponibilidade";
  if (kind === "risk") return "Investigar a causa e definir um responsável";
  return "Abrir a análise e decidir se vale uma acção";
}

function insightsToDiscoveries(raw: unknown): Discovery[] {
  return normalizeArray<any>(raw).map((i, idx) => ({
    id: String(i.id || `insight-${idx}-${i.title || ""}`),
    dataset_id: i.dataset_id,
    kind: i.kind === "trend" ? "change" : i.kind || "change",
    title: i.title || "Alteração",
    body: i.body || "",
    next_step: nextStep(i.kind || "", i.title || ""),
    impact_score: Number(i.evidence?.delta_pct || i.evidence?.share_pct || 0),
    evidence: i.evidence,
    fingerprint: "",
    feedback: "",
    watching: false,
  }));
}

export async function loadRadar(): Promise<Discovery[]> {
  const managed = await tryGet<Discovery[]>("/api/v1/manage/radar");
  if (managed) return normalizeArray<Discovery>(managed);
  return insightsToDiscoveries(await api("/api/v1/insights"));
}

export async function sendRadarFeedback(id: string, feedback: string) {
  try {
    await api(`/api/v1/manage/radar/${id}/feedback`, {
      method: "POST",
      body: JSON.stringify({ feedback }),
    });
  } catch (err) {
    if (!missing(err)) throw err;
  }
}

export async function loadActions(): Promise<ManageAction[]> {
  const managed = await tryGet<ManageAction[]>("/api/v1/manage/actions");
  if (managed) return normalizeArray<ManageAction>(managed);
  return readStore<ManageAction[]>("actions", []);
}

export async function createAction(payload: Record<string, unknown>): Promise<ManageAction> {
  try {
    return await api<ManageAction>("/api/v1/manage/actions", {
      method: "POST",
      body: JSON.stringify(payload),
    });
  } catch (err) {
    if (!missing(err)) throw err;
    const row: ManageAction = {
      id: crypto.randomUUID(),
      discovery_id: String(payload.discovery_id || ""),
      dataset_id: String(payload.dataset_id || ""),
      owner_id: String(payload.owner_id || ""),
      owner_name: String(payload.owner_name || ""),
      title: String(payload.title || ""),
      body: String(payload.body || ""),
      measure: String(payload.measure || ""),
      expected_result: String(payload.expected_result || ""),
      status: "open",
      due_at: String(payload.due_at || ""),
      review_at: String(payload.review_at || ""),
      created_at: new Date().toISOString(),
    };
    writeStore("actions", [row, ...loadActionsSync()]);
    return row;
  }
}

function loadActionsSync() {
  return readStore<ManageAction[]>("actions", []);
}

export async function reviewAction(id: string, executed_note: string) {
  try {
    return await api(`/api/v1/manage/actions/${id}/review`, {
      method: "POST",
      body: JSON.stringify({ executed_note }),
    });
  } catch (err) {
    if (!missing(err)) throw err;
    const list = loadActionsSync().map((a) => (a.id === id ? { ...a, status: "reviewed" } : a));
    writeStore("actions", list);
    return {
      id: crypto.randomUUID(),
      executed_note,
      indicator_narrative: "Sem leitura de indicador nesta revisão local.",
      effect_narrative: "A evolução do indicador só é calculada quando a API de gestão estiver no ar. Esta nota ficou registada neste browser.",
      created_at: new Date().toISOString(),
    };
  }
}

function toPace(g: any): GoalPace {
  const target = Number(g.target_value) || 0;
  const current = Number(g.current_value) || 0;
  const remaining = target - current;
  const pct = target ? (current / Math.abs(target)) * 100 : 0;
  return {
    id: g.id,
    name: g.name,
    metric_name: g.metric_name,
    dataset_id: g.dataset_id,
    target_value: target,
    current_value: current,
    remaining,
    pct: g.pct ?? pct,
    period: g.period || "monthly",
    period_start: g.period_start || "",
    period_end: g.period_end || "",
    remaining_business_days: g.remaining_business_days || 0,
    daily_pace: g.daily_pace || 0,
    reached: Boolean(g.reached),
    currency: Boolean(g.currency),
    status: g.status || "active",
    narrative: g.narrative || `Meta de ${target.toLocaleString("pt-BR")} (${g.period || "mensal"}).`,
    hypothesis: g.hypothesis,
  };
}

export async function loadGoals(): Promise<GoalPace[]> {
  const managed = await tryGet<GoalPace[]>("/api/v1/manage/goals");
  if (managed) return normalizeArray<GoalPace>(managed).map(toPace);
  return normalizeArray<any>(await api("/api/v1/goals")).map(toPace);
}

export async function createGoal(payload: Record<string, unknown>) {
  try {
    return await api("/api/v1/manage/goals", {
      method: "POST",
      body: JSON.stringify(payload),
    });
  } catch (err) {
    if (!missing(err)) throw err;
    return api("/api/v1/goals", {
      method: "POST",
      body: JSON.stringify(payload),
    });
  }
}

export async function loadMemories(): Promise<Memory[]> {
  const managed = await tryGet<Memory[]>("/api/v1/manage/memories");
  if (managed) return normalizeArray<Memory>(managed);
  return readStore<Memory[]>("memories", []);
}

export async function createMemory(payload: { note: string; occurred_on: string; dataset_id?: string }) {
  try {
    return await api<Memory>("/api/v1/manage/memories", {
      method: "POST",
      body: JSON.stringify(payload),
    });
  } catch (err) {
    if (!missing(err)) throw err;
    const row: Memory = {
      id: crypto.randomUUID(),
      note: payload.note,
      occurred_on: payload.occurred_on,
      dataset_id: payload.dataset_id,
      created_at: new Date().toISOString(),
    };
    writeStore("memories", [row, ...readStore<Memory[]>("memories", [])]);
    return row;
  }
}

export async function loadReviews(): Promise<WeeklyReview[]> {
  const managed = await tryGet<WeeklyReview[]>("/api/v1/manage/reviews");
  if (managed) return normalizeArray<WeeklyReview>(managed);
  return readStore<WeeklyReview[]>("reviews", []);
}

export async function saveReview(payload: Record<string, unknown>): Promise<WeeklyReview> {
  try {
    return await api<WeeklyReview>("/api/v1/manage/reviews", {
      method: "POST",
      body: JSON.stringify(payload),
    });
  } catch (err) {
    if (!missing(err)) throw err;
    const token = crypto.randomUUID().replace(/-/g, "").slice(0, 24);
    const row: WeeklyReview = {
      id: crypto.randomUUID(),
      period: String(payload.period || new Date().toISOString().slice(0, 10)),
      agenda: {
        results: payload.results as WeeklyReview["agenda"]["results"],
        prior_actions: payload.prior_actions as WeeklyReview["agenda"]["prior_actions"],
        priorities: payload.priorities as WeeklyReview["agenda"]["priorities"],
        notes: String(payload.notes || ""),
      },
      share_token: token,
      share_url: `/share/review/${token}`,
      next_review_at: String(payload.next_review_at || ""),
      created_at: new Date().toISOString(),
    };
    writeStore("reviews", [row, ...readStore<WeeklyReview[]>("reviews", [])]);
    return row;
  }
}

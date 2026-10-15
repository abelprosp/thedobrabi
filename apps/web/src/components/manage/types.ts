export type Discovery = {
  id: string;
  dataset_id?: string;
  kind: string;
  title: string;
  body: string;
  next_step: string;
  impact_score: number;
  evidence?: Record<string, unknown>;
  fingerprint: string;
  feedback: string;
  watching: boolean;
};

export type ManageAction = {
  id: string;
  discovery_id?: string;
  dataset_id?: string;
  owner_id?: string;
  owner_name: string;
  title: string;
  body: string;
  measure: string;
  expected_result: string;
  baseline?: Record<string, unknown>;
  status: string;
  due_at?: string;
  review_at?: string;
  created_at: string;
  due_soon?: boolean;
  review_due?: boolean;
  latest_review?: ActionReview;
};

export type ActionReview = {
  id: string;
  executed_note: string;
  indicator_value?: number;
  indicator_delta_pct?: number;
  indicator_narrative: string;
  effect_narrative: string;
  created_at: string;
};

export type Memory = {
  id: string;
  dataset_id?: string;
  note: string;
  chart_key?: string;
  occurred_on: string;
  created_at: string;
};

export type PastDecision = {
  period: string;
  title: string;
  owner_name?: string;
  when: string;
};

export type BriefItem = { title: string; body: string; kind: string; severity: string };

export type Home = {
  datasets: number;
  brief?: {
    headline: string;
    major_changes: BriefItem[];
    risks: BriefItem[];
    opportunities: BriefItem[];
    recommended_actions: string[];
    dataset_id?: string;
  };
  cadence: string;
  cadence_mode: string;
  suggested_cadence: string;
  data_updated_at?: string;
  what_changed: Discovery[];
  attention: Discovery[];
  action_now?: Discovery;
  quiet: boolean;
  quiet_message?: string;
  open_actions: number;
  due_reviews: ManageAction[];
  week_review_ready: boolean;
  memories: Memory[];
  past_decisions: PastDecision[];
  watch_measures: string[];
};

export type GoalPace = {
  id: string;
  name: string;
  metric_name: string;
  dataset_id: string;
  target_value: number;
  current_value: number;
  remaining: number;
  pct: number;
  period: string;
  period_start: string;
  period_end: string;
  remaining_business_days: number;
  daily_pace: number;
  reached: boolean;
  currency: boolean;
  status: string;
  narrative: string;
  hypothesis?: string;
};

export type Member = { id: string; name: string; email: string; role: string; active: boolean };

export type WeeklyReview = {
  id: string;
  period: string;
  agenda: {
    results?: { title?: string; body?: string }[];
    prior_actions?: { id?: string; title?: string; status?: string }[];
    priorities?: { title?: string; owner_id?: string; owner_name?: string }[];
    notes?: string;
  };
  share_token?: string;
  share_url?: string;
  next_review_at?: string;
  created_at: string;
};

export function cadenceLabel(v: string) {
  return { auto: "Automática", daily: "Diária", weekly: "Semanal", monthly: "Mensal" }[v] || v;
}

export function money(n: number, currency?: boolean) {
  const formatted = n.toLocaleString("pt-BR", { maximumFractionDigits: n >= 100 ? 0 : 1 });
  return currency ? `R$ ${formatted}` : formatted;
}

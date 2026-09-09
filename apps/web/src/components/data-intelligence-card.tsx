"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { AlertTriangle, Brain, RefreshCw, Sparkles } from "lucide-react";
import { toast } from "sonner";
import { api, apiStatus } from "@/lib/api";
import { Badge, Button } from "@/components/ui";
import type { DashboardFilter, QuerySpec, Widget } from "@/components/WidgetView";

export type DashboardInsight = {
  kind: string;
  title: string;
  body: string;
  severity: string;
  widget_id?: string;
};

export type AlertSuggestion = {
  name: string;
  rationale?: string;
  widget_id?: string;
  severity?: string;
  condition: { dataset_id: string; measure: string; op: string; value: number };
};

export type DashboardIntelResult = {
  headline: string;
  insights: DashboardInsight[];
  alert_suggestions: AlertSuggestion[];
  recommended_actions: string[];
  generated_at?: string;
  source?: string;
  analyzed_widgets?: number;
};

const KIND_LABEL: Record<string, string> = {
  trend: "Tendência",
  risk: "Risco",
  opportunity: "Oportunidade",
  anomaly: "Anomalia",
  concentration: "Concentração",
  alert: "Alerta",
  info: "Info",
};

const SEVERITY_LABEL: Record<string, string> = {
  low: "Baixo",
  medium: "Médio",
  high: "Alto",
  critical: "Crítico",
};

const SKIP_TYPES = new Set(["text", "image", "markdown", "iframe", "data_intelligence", "slicer"]);

function severityTone(sev: string): "danger" | "warn" | "ok" | "neutral" {
  if (sev === "high" || sev === "critical") return "danger";
  if (sev === "medium") return "warn";
  if (sev === "low") return "ok";
  return "neutral";
}

function analyzableWidgets(siblings: Widget[]): Widget[] {
  return siblings.filter((w) => !SKIP_TYPES.has(w.type) && !!w.query?.dataset_id).slice(0, 12);
}

export function DataIntelligenceCard({
  title,
  focusPrompt,
  siblings,
  dashboardId,
  globalFilters,
  timeRange,
  isPublic,
  analyzePath,
}: {
  title: string;
  focusPrompt?: string;
  siblings: Widget[];
  dashboardId?: string;
  globalFilters: DashboardFilter[];
  timeRange?: { start?: string; end?: string };
  isPublic?: boolean;
  analyzePath?: string;
}) {
  const targets = useMemo(() => analyzableWidgets(siblings), [siblings]);
  const [created, setCreated] = useState<Set<string>>(new Set());
  const analyzeURL = analyzePath || "/api/v1/ai/analyze-dashboard-widgets";

  const analysis = useQuery({
    queryKey: [
      "data-intelligence",
      dashboardId,
      analyzeURL,
      focusPrompt,
      timeRange,
      globalFilters,
      targets.map((w) => ({
        id: w.id,
        type: w.type,
        title: w.title,
        dataset: w.query?.dataset_id,
        measures: w.query?.measures,
        dimensions: w.query?.dimensions,
        filters: w.query?.filters,
      })),
    ],
    enabled: targets.length > 0,
    queryFn: () =>
      api<DashboardIntelResult>(analyzeURL, {
        method: "POST",
        body: JSON.stringify({
          dashboard_id: dashboardId,
          focus_prompt: focusPrompt || undefined,
          time_range: timeRange?.start || timeRange?.end ? timeRange : undefined,
          global_filters: globalFilters,
          widgets: targets.map((w) => ({
            id: w.id,
            type: w.type,
            title: w.title,
            query: slimQuery(w.query),
          })),
        }),
      }),
    retry: (count, err) => apiStatus(err) !== 402 && apiStatus(err) !== 401 && count < 1,
    refetchOnWindowFocus: false,
    staleTime: 60_000,
  });

  const createAlert = useMutation({
    mutationFn: (sug: AlertSuggestion) =>
      api("/api/v1/alerts", {
        method: "POST",
        body: JSON.stringify({
          name: sug.name,
          condition: sug.condition,
          channels: ["realtime"],
        }),
      }),
    onSuccess: (_data, sug) => {
      setCreated((prev) => new Set(prev).add(sug.name));
      toast.success("Alerta criado");
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const data = analysis.data;
  const insights = data?.insights || [];
  const alerts = data?.alert_suggestions || [];
  const actions = data?.recommended_actions || [];
  const widgetTitle = (id?: string) => siblings.find((w) => w.id === id)?.title;

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface shadow-sm">
      <div className="flex items-start justify-between gap-2 border-b border-line px-4 py-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-mute">
            <Brain size={14} className="text-primary" />
            Inteligência dados
          </div>
          <div className="mt-0.5 truncate text-sm font-semibold text-ink">{title || "Inteligência dados"}</div>
        </div>
        <Button
          size="sm"
          variant="secondary"
          className="shrink-0"
          busy={analysis.isFetching}
          onClick={() => analysis.refetch()}
        >
          <RefreshCw size={12} />
          Analisar
        </Button>
      </div>

      <div className="min-h-0 flex-1 overflow-auto px-4 py-3">
        {analysis.isLoading && (
          <p className="text-[13px] text-mute">A analisar os visuais deste dashboard…</p>
        )}

        {analysis.isError && (
          <div className="rounded-xl border border-rose-100 bg-rose-50 px-3 py-2 text-[13px] text-danger">
            {apiStatus(analysis.error) === 402
              ? "A quota de IA deste espaço esgotou. Tente mais tarde."
              : (analysis.error as Error).message || "Não foi possível analisar o dashboard."}
          </div>
        )}

        {!analysis.isLoading && !analysis.isError && targets.length === 0 && (
          <p className="text-[13px] text-mute">
            {isPublic
              ? "Não há visuais suficientes neste dashboard para analisar."
              : "Adicione KPIs, gráficos ou tabelas a este dashboard. Este cartão analisa os visuais que estão ao lado e gera insights e alertas."}
          </p>
        )}

        {!analysis.isLoading && !analysis.isError && data && (
          <div className="space-y-3">
            <p className="text-[13px] font-medium leading-snug text-ink">{data.headline}</p>
            <div className="flex flex-wrap items-center gap-1.5">
              <Badge tone={data.source === "openai" ? "accent" : "neutral"}>
                {data.source === "openai" ? (
                  <span className="inline-flex items-center gap-1">
                    <Sparkles size={10} /> IA
                  </span>
                ) : (
                  "Análise local"
                )}
              </Badge>
              {typeof data.analyzed_widgets === "number" && (
                <Badge>{data.analyzed_widgets} visuais</Badge>
              )}
            </div>

            {insights.length > 0 && (
              <div className="space-y-2">
                {insights.map((ins, idx) => (
                  <div key={`${ins.title}-${idx}`} className="rounded-xl border border-line bg-surface-2/60 px-3 py-2">
                    <div className="flex flex-wrap items-center gap-1.5">
                      <Badge>{KIND_LABEL[ins.kind] || ins.kind}</Badge>
                      <Badge tone={severityTone(ins.severity)}>{SEVERITY_LABEL[ins.severity] || ins.severity}</Badge>
                      {ins.widget_id && widgetTitle(ins.widget_id) && (
                        <span className="truncate text-[11px] text-mute">{widgetTitle(ins.widget_id)}</span>
                      )}
                    </div>
                    <div className="mt-1 text-[13px] font-medium text-ink">{ins.title}</div>
                    <p className="mt-0.5 text-[12px] leading-snug text-mute">{ins.body}</p>
                  </div>
                ))}
              </div>
            )}

            {alerts.length > 0 && (
              <div className="space-y-2">
                <div className="flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide text-mute">
                  <AlertTriangle size={12} />
                  Alertas sugeridos
                </div>
                {alerts.map((al) => {
                  const done = created.has(al.name);
                  return (
                    <div key={al.name} className="rounded-xl border border-amber-100 bg-amber-50/70 px-3 py-2 dark:border-amber-500/20 dark:bg-amber-500/10">
                      <div className="text-[13px] font-medium text-ink">{al.name}</div>
                      {al.rationale && <p className="mt-0.5 text-[12px] text-mute">{al.rationale}</p>}
                      {!isPublic && (
                        <div className="mt-2">
                          <Button
                            size="sm"
                            variant={done ? "ghost" : "secondary"}
                            disabled={done || createAlert.isPending}
                            busy={createAlert.isPending && createAlert.variables?.name === al.name}
                            onClick={() => createAlert.mutate(al)}
                          >
                            {done ? "Alerta criado" : "Criar alerta"}
                          </Button>
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            )}

            {actions.length > 0 && (
              <ul className="list-disc space-y-1 pl-4 text-[12px] text-mute">
                {actions.map((a) => (
                  <li key={a}>{a}</li>
                ))}
              </ul>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function slimQuery(query?: QuerySpec): QuerySpec {
  return {
    dataset_id: query?.dataset_id,
    measures: query?.measures,
    dimensions: query?.dimensions,
    filters: query?.filters,
    limit: query?.limit,
    time_range: query?.time_range,
    joins: query?.joins,
  };
}

"use client";

import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { Button, Select, cn } from "@/components/ui";
import {
  AlertTriangle,
  ArrowUp,
  CheckCircle2,
  Loader2,
  Sparkles,
  X,
} from "lucide-react";
import type {
  DashboardFilter,
  Widget,
  WidgetType,
} from "@/components/WidgetView";
import { DEFAULT_QUERY_LIMIT } from "@/lib/widget-config";

type PlanItem = {
  title: string;
  chart: string;
  why: string;
  measure?: string;
  dimension?: string;
};

export type DobraReply = {
  conversation_id: string;
  reply: string;
  plan?: PlanItem[];
  apply?: boolean;
  replace?: boolean;
  widgets?: any[];
  filters?: { dimension: string; op?: string; value?: any }[];
  time_range?: { start?: string; end?: string };
  dashboard_name?: string;
  source?: string;
  dataset_id?: string;
  dataset_name?: string;
  validated?: boolean;
  confidence?: "low" | "medium" | "high";
  warnings?: string[];
};

type Msg = {
  role: "user" | "assistant";
  text: string;
  plan?: PlanItem[];
  applied?: number;
  warnings?: string[];
  validated?: boolean;
  confidence?: string;
};

const ALLOWED: WidgetType[] = [
  "kpi",
  "kpi_goal",
  "line",
  "bar",
  "area",
  "pie",
  "table",
  "ranking",
  "slicer",
  "data_intelligence",
  "text",
  "gauge",
  "sparkline",
  "heatmap",
  "treemap",
  "funnel",
  "big_table",
];

const SUGGESTIONS = [
  "Monta um dashboard completo com KPIs, tendência, ranking e filtros",
  "Adiciona um ranking das maiores categorias",
  "Coloca um gráfico de linha do valor ao longo do tempo",
  "Inclui um slicer e a análise automática",
];

function mapWidgets(
  raw: any[],
  fallbackDataset?: string,
  yOffset = 0,
): Widget[] {
  return (raw || []).map((w): Widget => {
    const type: WidgetType = ALLOWED.includes(w.type) ? w.type : "bar";
    const q = w.query
      ? { ...w.query, dataset_id: w.query.dataset_id || fallbackDataset }
      : undefined;
    if (q && q.limit == null)
      q.limit =
        type === "ranking"
          ? 10
          : type === "table" || type === "big_table"
            ? 200
            : DEFAULT_QUERY_LIMIT;
    return {
      id: typeof w.id === "string" && w.id ? w.id : crypto.randomUUID(),
      type,
      title: w.title || "Widget",
      layout: {
        x: Number(w.layout?.x ?? 0),
        y: Number(w.layout?.y ?? 0) + yOffset,
        w: Number(w.layout?.w ?? 6),
        h: Number(w.layout?.h ?? 4),
      },
      query: q,
      text: w.text,
      config: w.config,
    };
  });
}

export function DobraAIChat({
  open,
  onClose,
  dashboardName,
  dashboardId,
  widgets,
  datasets,
  datasetId,
  onDatasetId,
  onApply,
}: {
  open: boolean;
  onClose: () => void;
  dashboardName: string;
  dashboardId: string;
  widgets: Widget[];
  datasets: { id: string; name: string }[];
  datasetId: string;
  onDatasetId: (id: string) => void;
  onApply: (next: {
    widgets: Widget[];
    replace: boolean;
    name?: string;
    filters?: DashboardFilter[];
    timeRange?: { start?: string; end?: string };
  }) => void;
}) {
  const [q, setQ] = useState("");
  const [busy, setBusy] = useState(false);
  const [convId, setConvId] = useState("");
  const [msgs, setMsgs] = useState<Msg[]>([
    {
      role: "assistant",
      text: "Sou a DobraAI. Diga o que o painel deve mostrar — eu proponho KPIs, gráficos, filtros e análises e monto tudo no canvas.",
    },
  ]);
  const bottom = useRef<HTMLDivElement>(null);
  const chatScroll = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (open && msgs.length > 1) {
      inputRef.current?.focus();
      const container = chatScroll.current;
      if (container) {
        requestAnimationFrame(() => container.scrollTo({ top: container.scrollHeight, behavior: "smooth" }));
      }
    } else if (open) {
      inputRef.current?.focus();
    }
  }, [open, msgs.length]);

  async function send(text?: string) {
    const message = (text ?? q).trim();
    if (!message || busy) return;
    setQ("");
    setBusy(true);
    setMsgs((m) => [...m, { role: "user", text: message }]);
    try {
      const history = msgs
        .slice(-8)
        .map((m) => ({ role: m.role, text: m.text }));
      const res = await api<DobraReply>("/api/v1/ai/dobra", {
        method: "POST",
        body: JSON.stringify({
          conversation_id: convId || undefined,
          message,
          dataset_id: datasetId || undefined,
          dashboard_id: dashboardId,
          dashboard_name: dashboardName,
          history,
          widgets: widgets.slice(0, 24).map((w) => ({
            type: w.type,
            title: w.title,
            query: w.query
              ? { measures: w.query.measures, dimensions: w.query.dimensions }
              : undefined,
          })),
        }),
      });
      if (res.conversation_id) setConvId(res.conversation_id);
      const mapped = mapWidgets(
        res.widgets || [],
        res.dataset_id || datasetId,
        res.replace ? 0 : maxY(widgets),
      );
      if (res.apply && mapped.length > 0) {
        onApply({
          widgets: mapped,
          replace: Boolean(res.replace),
          name: res.dashboard_name,
          filters: (res.filters || [])
            .filter((f) => f.dimension)
            .map((f) => ({
              dimension: f.dimension,
              op: (f.op === "in" ? "in" : "eq") as "eq" | "in",
              value: f.value,
              dataset_id: res.dataset_id || datasetId,
            })),
          timeRange: res.time_range,
        });
      }
      setMsgs((m) => [
        ...m,
        {
          role: "assistant",
          text: res.reply || "Pronto.",
          plan: res.plan,
          applied: res.apply ? mapped.length : 0,
          warnings: res.warnings,
          validated: res.validated,
          confidence: res.confidence,
        },
      ]);
    } catch (e: any) {
      setMsgs((m) => [
        ...m,
        {
          role: "assistant",
          text: e.message || "Não consegui montar o dashboard.",
        },
      ]);
    } finally {
      setBusy(false);
    }
  }

  if (!open) return null;

  return (
    <aside className="absolute inset-0 z-40 flex w-full flex-col border-line bg-surface pb-[env(safe-area-inset-bottom)] shadow-xl xl:inset-y-0 xl:right-0 xl:left-auto xl:w-[min(100%,24rem)] xl:border-l">
      <div className="flex items-center gap-2 border-b border-line px-3 py-2.5">
        <Sparkles size={16} className="text-primary" />
        <div className="min-w-0 flex-1">
          <div className="text-[13px] font-semibold text-ink">DobraAI</div>
          <div className="truncate text-[11px] text-mute">
            Planeia e monta o dashboard
          </div>
        </div>
        <button
          type="button"
          className="rounded-lg p-1 text-mute hover:bg-bg hover:text-ink"
          onClick={onClose}
          aria-label="Fechar DobraAI"
        >
          <X size={16} />
        </button>
      </div>
      {datasets.length > 0 && (
        <div className="border-b border-line px-3 py-2">
          <Select
            value={datasetId}
            onChange={(e) => onDatasetId(e.target.value)}
            className="h-8 text-[12px]"
          >
            <option value="">Conjunto automático</option>
            {datasets.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
              </option>
            ))}
          </Select>
        </div>
      )}
      <div ref={chatScroll} className="min-h-0 flex-1 space-y-3 overflow-y-auto overscroll-contain px-3 py-3 [scrollbar-gutter:stable]">
        {msgs.map((m, i) => (
          <div
            key={i}
            className={cn(
              "max-w-[95%] rounded-2xl px-3 py-2 text-[13px] leading-relaxed",
              m.role === "user"
                ? "ml-auto bg-primary text-white"
                : "bg-bg text-ink",
            )}
          >
            <p className="whitespace-pre-wrap">{m.text}</p>
            {m.plan && m.plan.length > 0 && (
              <ul className="mt-2 space-y-1.5 border-t border-white/20 pt-2 text-[12px]">
                {m.plan.map((p, j) => (
                  <li
                    key={j}
                    className={cn(
                      m.role === "user" ? "text-white/90" : "text-mute",
                    )}
                  >
                    <span
                      className={cn(
                        "font-medium",
                        m.role === "assistant" && "text-ink",
                      )}
                    >
                      {p.title}
                    </span>
                    <span> · {p.chart}</span>
                    {p.why ? (
                      <div className="text-[11px] opacity-80">{p.why}</div>
                    ) : null}
                  </li>
                ))}
              </ul>
            )}
            {!!m.applied && (
              <p
                className={cn(
                  "mt-2 text-[11px] font-medium",
                  m.role === "assistant" ? "text-primary" : "text-white/90",
                )}
              >
                {m.applied} visual{m.applied === 1 ? "" : "is"} aplicado
                {m.applied === 1 ? "" : "s"} no canvas
              </p>
            )}
            {m.warnings && m.warnings.length > 0 && (
              <div className="mt-2 space-y-1 border-t border-amber-500/20 pt-2 text-[11px] text-amber-700">
                {m.warnings.map((warning, j) => (
                  <p key={j} className="flex items-start gap-1.5">
                    <AlertTriangle size={12} className="mt-0.5 shrink-0" />
                    <span>{warning}</span>
                  </p>
                ))}
              </div>
            )}
            {m.role === "assistant" && m.validated && (
              <p className="mt-2 flex items-center gap-1 text-[10px] text-mute">
                <CheckCircle2 size={11} className="text-emerald-600" />
                Campos validados no modelo · confiança{" "}
                {m.confidence === "high"
                  ? "alta"
                  : m.confidence === "low"
                    ? "baixa"
                    : "média"}
              </p>
            )}
          </div>
        ))}
        {busy && (
          <div className="flex items-center gap-2 text-[12px] text-mute">
            <Loader2 size={14} className="animate-spin" /> A desenhar o plano…
          </div>
        )}
        <div ref={bottom} />
      </div>
      {msgs.length < 3 && (
        <div className="flex flex-wrap gap-1.5 border-t border-line px-3 py-2">
          {SUGGESTIONS.map((s) => (
            <button
              key={s}
              type="button"
              disabled={busy}
              onClick={() => send(s)}
              className="rounded-full border border-line bg-bg px-2.5 py-1 text-left text-[11px] text-mute hover:border-primary hover:text-ink"
            >
              {s}
            </button>
          ))}
        </div>
      )}
      <form
        className="flex items-end gap-2 border-t border-line p-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))]"
        onSubmit={(e) => {
          e.preventDefault();
          void send();
        }}
      >
        <textarea
          ref={inputRef}
          value={q}
          rows={2}
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              void send();
            }
          }}
          placeholder="Ex.: monta um painel de vendas com filtros"
          className="min-h-16 flex-1 resize-none rounded-xl border border-line bg-bg px-3 py-2 text-[13px] text-ink outline-none focus:border-primary"
        />
        <Button
          type="submit"
          size="icon"
          disabled={busy || !q.trim()}
          title="Enviar"
        >
          {busy ? (
            <Loader2 size={16} className="animate-spin" />
          ) : (
            <ArrowUp size={16} />
          )}
        </Button>
      </form>
    </aside>
  );
}

function maxY(widgets: Widget[]) {
  return widgets.reduce(
    (m, w) => Math.max(m, (w.layout?.y ?? 0) + (w.layout?.h ?? 0)),
    0,
  );
}

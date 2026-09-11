"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { Chart, Kpi } from "@/components/viz";
import { toast } from "sonner";
import {
  Button,
  Card,
  EmptyState,
  PageSkeleton,
  Select,
  cn,
} from "@/components/ui";
import {
  AlertTriangle,
  ArrowUp,
  BarChart3,
  Calculator,
  CheckCircle2,
  Database,
  Lightbulb,
  Loader2,
  MessageCircle,
  RotateCcw,
  Sparkles,
  Wand2,
} from "lucide-react";
import Link from "next/link";

type Answer = {
  conversation_id?: string;
  answer: string;
  key_metric?: { label: string; value: number; delta_pct?: number };
  chart?: { type: string; title: string; columns: string[]; rows: any[] };
  explanation?: string;
  drivers?: string[];
  recommendation?: string;
  evidence?: Record<string, any>;
  insufficient_data?: boolean;
  source?: string;
  confidence?: "low" | "medium" | "high";
  warnings?: string[];
};

type Msg = {
  role: "user" | "assistant";
  text: string;
  answer?: Answer;
  meta?: any;
};
type Dataset = { id: string; name: string };

const examples = [
  "Qual o valor total?",
  "Valor por categoria",
  "Como está o resultado por natureza?",
  "Evolução do valor por mês",
  "Quais as maiores linhas?",
];

function formatMetric(label: string, value: number) {
  const money = /valor|receita|despesa|amount|revenue|total|montante/i.test(
    label,
  );
  return value.toLocaleString("pt-BR", {
    minimumFractionDigits: money ? 2 : 0,
    maximumFractionDigits: money ? 2 : 0,
  });
}

function Evidence({ evidence }: { evidence: Record<string, any> }) {
  const sql = String(evidence.sql || evidence.SQL || "");
  const source = String(evidence.source || evidence.dataset || "");
  const metric = String(evidence.metric || evidence.calculation || "");
  const period = String(evidence.period || "");
  return (
    <details className="rounded-xl border border-line bg-bg/80">
      <summary className="cursor-pointer px-3 py-2 text-[12px] font-medium text-mute hover:text-ink">
        Como cheguei aqui
      </summary>
      <div className="space-y-2 border-t border-line px-3 py-3 text-[12px] text-mute">
        {source && (
          <div>
            <span className="font-medium text-ink">Conjunto</span> · {source}
          </div>
        )}
        {metric && !/^linhas$/i.test(metric) && (
          <div>
            <span className="font-medium text-ink">Métrica</span> · {metric}
          </div>
        )}
        {period && (
          <div>
            <span className="font-medium text-ink">Período</span> · {period}
          </div>
        )}
        {sql && (
          <pre className="overflow-x-auto rounded-lg bg-slate-950 p-3 font-mono text-[11px] leading-relaxed text-slate-100">
            {sql}
          </pre>
        )}
        {!sql && (
          <pre className="overflow-x-auto text-[11px]">
            {JSON.stringify(evidence, null, 2)}
          </pre>
        )}
      </div>
    </details>
  );
}

export default function AskPage() {
  const [q, setQ] = useState("");
  const [msgs, setMsgs] = useState<Msg[]>([]);
  const [busy, setBusy] = useState(false);
  const [datasetId, setDatasetId] = useState("");
  const [conversationId, setConversationId] = useState("");
  const bottom = useRef<HTMLDivElement>(null);
  const chatScroll = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const [showLatest, setShowLatest] = useState(false);

  const datasets = useQuery({
    queryKey: ["datasets"],
    queryFn: () => api<any>("/api/v1/datasets"),
  });
  const datasetList = normalizeArray<Dataset>(datasets.data);
  const activeId = datasetId || datasetList[0]?.id || "";
  const activeName = datasetList.find((d) => d.id === activeId)?.name;

  const examplesForData = useMemo(() => {
    if (/financeiro|redorai|p.?l|dre/i.test(activeName || "")) return examples;
    return [
      "Porque caiu a receita este mês?",
      "Quais são os 10 maiores clientes?",
      "Compare agosto e setembro.",
      "Que produtos estão a perder margem?",
      "Preveja o valor para o próximo mês.",
    ];
  }, [activeName]);

  useEffect(() => {
    const container = chatScroll.current;
    if (!container) return;
    requestAnimationFrame(() => {
      container.scrollTo({ top: container.scrollHeight, behavior: "smooth" });
    });
  }, [msgs, busy]);

  async function ask(text: string) {
    const prompt = text.trim();
    if (!prompt || busy) return;
    setBusy(true);
    setQ("");
    setMsgs((m) => [...m, { role: "user", text: prompt }]);
    try {
      const res = await api<Answer>("/api/v1/ai/ask", {
        method: "POST",
        body: JSON.stringify({
          conversation_id: conversationId || undefined,
          message: prompt,
          dataset_id: activeId || undefined,
          history: msgs
            .slice(-6)
            .map((message) => ({ role: message.role, text: message.text })),
        }),
      });
      if (res.conversation_id) setConversationId(res.conversation_id);
      setMsgs((m) => [
        ...m,
        { role: "assistant", text: res.answer, answer: res },
      ]);
    } catch (e: any) {
      toast.error(e.message);
      setMsgs((m) => [
        ...m,
        {
          role: "assistant",
          text: "Não consegui responder agora. Tente de novo em instantes.",
        },
      ]);
    } finally {
      setBusy(false);
      inputRef.current?.focus();
    }
  }

  const generateSQL = useMutation({
    mutationFn: (prompt: string) =>
      api<{ sql: string; explanation: string }>("/api/v1/ai/generate-sql", {
        method: "POST",
        body: JSON.stringify({ prompt, dataset_id: activeId || undefined }),
      }),
    onSuccess: (res, prompt) =>
      setMsgs((m) => [
        ...m,
        { role: "assistant", text: "SQL para: " + prompt, meta: res },
      ]),
    onError: (e: Error) => toast.error(e.message),
  });

  const generateMeasure = useMutation({
    mutationFn: (prompt: string) =>
      api<{ name: string; expression: string; explanation: string }>(
        "/api/v1/ai/generate-measure",
        {
          method: "POST",
          body: JSON.stringify({ prompt, dataset_id: activeId || undefined }),
        },
      ),
    onSuccess: (res, prompt) =>
      setMsgs((m) => [
        ...m,
        { role: "assistant", text: "Medida para: " + prompt, meta: res },
      ]),
    onError: (e: Error) => toast.error(e.message),
  });

  if (datasets.isLoading) return <PageSkeleton cards={2} />;

  if (datasetList.length === 0) {
    return (
      <div className="mx-auto max-w-3xl space-y-5">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-ink">
            Analisar com IA
          </h1>
          <p className="mt-1.5 text-sm text-mute">
            Conecte os seus dados para receber respostas com métricas oficiais e
            evidências.
          </p>
        </div>
        <EmptyState
          icon={Database}
          title="A DobraAI precisa de dados"
          description="Conecte uma base, importe um ficheiro ou use os dados de exemplo. Depois poderá perguntar em português e conferir como cada resposta foi calculada."
          action={
            <div className="flex flex-wrap justify-center gap-2">
              <Link href="/connectors">
                <Button>Conectar dados</Button>
              </Link>
              <Link href="/data">
                <Button variant="secondary">Importar ficheiro</Button>
              </Link>
            </div>
          }
        />
      </div>
    );
  }

  return (
    <div className="mx-auto flex h-[calc(100dvh-12rem)] min-h-[34rem] max-w-5xl flex-col lg:h-[calc(100dvh-9rem)]">
      <div className="mb-4 overflow-hidden rounded-3xl border border-primary/15 bg-gradient-to-br from-primary/[0.08] via-surface to-accent/[0.06] p-5 shadow-sm sm:p-6">
        <div className="flex flex-col gap-5 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex items-start gap-3">
            <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-primary text-white shadow-lg shadow-primary/20">
              <Sparkles size={20} />
            </div>
            <div>
              <div className="flex flex-wrap items-center gap-2">
                <h1 className="text-xl font-semibold tracking-tight text-ink sm:text-2xl">Analisar com IA</h1>
                <span className="rounded-full border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[10px] font-medium text-emerald-700">Dados verificados</span>
              </div>
              <p className="mt-1 max-w-xl text-[13px] leading-relaxed text-mute">
                Faça perguntas em linguagem natural. A DobraAI encontra métricas, explica variações e mostra de onde veio cada número.
              </p>
            </div>
          </div>
          <div className="flex w-full gap-2 lg:w-auto">
          {datasetList.length > 0 && (
            <Select
              aria-label="Conjunto"
              value={activeId}
              onChange={(e) => {
                setDatasetId(e.target.value);
                setConversationId("");
                setMsgs([]);
              }}
              className="min-w-0 flex-1 sm:w-[220px]"
            >
              {datasetList.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                </option>
              ))}
            </Select>
          )}
          {msgs.length > 0 && (
            <Button
              variant="ghost"
              size="icon"
              aria-label="Nova conversa"
              title="Nova conversa"
              onClick={() => {
                setConversationId("");
                setMsgs([]);
              }}
            >
              <RotateCcw size={15} />
            </Button>
          )}
          </div>
        </div>
        <div className="mt-5 grid grid-cols-1 gap-2 border-t border-primary/10 pt-4 sm:grid-cols-3">
          <Capability icon={BarChart3} title="Explore" text="Veja tendências e comparações" />
          <Capability icon={Calculator} title="Calcule" text="Crie métricas sem SQL" />
          <Capability icon={Lightbulb} title="Decida" text="Encontre os próximos passos" />
        </div>
      </div>
      <div className="mb-4 flex items-center gap-2 rounded-xl border border-line bg-surface px-3 py-2 text-[11px] text-mute shadow-sm">
        <CheckCircle2 size={13} className="text-emerald-600" />
        <span>
          Respostas calculadas com métricas oficiais e evidências do conjunto
          selecionado.
        </span>
        <span className="hidden text-line sm:inline">·</span>
        <span>Faça perguntas de continuação como “e por região?”</span>
      </div>

      <div className="relative min-h-0 flex-1">
      <div
        ref={chatScroll}
        onScroll={(event) => {
          const el = event.currentTarget;
          const distance = el.scrollHeight - el.scrollTop - el.clientHeight;
          setShowLatest(distance > 180);
        }}
        aria-label="Histórico da conversa"
        className="h-full space-y-4 overflow-y-auto overscroll-contain scroll-smooth px-0.5 pb-5 pt-2 pr-1 [scrollbar-gutter:stable]"
      >
        {msgs.length === 0 && (
          <div className="flex min-h-full flex-col items-center justify-center px-2 py-8 text-center">
            <div className="flex h-16 w-16 items-center justify-center rounded-3xl bg-primary/10 text-primary ring-8 ring-primary/[0.035]">
              <MessageCircle size={28} />
            </div>
            <h2 className="mt-4 text-lg font-semibold tracking-tight text-ink">Por onde começamos?</h2>
            <p className="mt-2 max-w-md text-[13px] leading-relaxed text-mute">
              Pergunte em português sobre <strong className="font-medium text-ink">{activeName || "o seu negócio"}</strong>. Comece por uma sugestão ou escreva a sua própria pergunta.
            </p>
            <div className="mt-6 grid w-full max-w-2xl gap-2 sm:grid-cols-2">
              {examplesForData.slice(0, 4).map((ex) => (
                <button key={ex} type="button" onClick={() => ask(ex)} className="group flex items-center gap-3 rounded-xl border border-line bg-surface px-3.5 py-3 text-left text-[12px] text-ink shadow-sm transition hover:-translate-y-px hover:border-primary/35 hover:shadow-md">
                  <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-primary/8 text-primary"><Lightbulb size={14} /></span>
                  <span className="flex-1">{ex}</span>
                  <ArrowUp size={13} className="-rotate-45 text-mute transition group-hover:text-primary" />
                </button>
              ))}
            </div>
          </div>
        )}

        {msgs.map((m, i) =>
          m.role === "user" ? (
            <div key={i} className="flex justify-end">
              <div className="max-w-[85%] rounded-2xl rounded-br-md bg-primary px-4 py-2.5 text-sm leading-relaxed text-white">
                {m.text}
              </div>
            </div>
          ) : (
            <div key={i} className="flex gap-2.5">
              <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
                <Sparkles size={14} />
              </div>
              <Card
                className={cn(
                  "min-w-0 flex-1 space-y-3",
                  m.answer?.insufficient_data &&
                    "border-amber-200 bg-amber-50/40",
                )}
              >
                <p className="text-sm leading-relaxed text-ink">{m.text}</p>
                {m.answer?.key_metric && (
                  <Kpi
                    label={m.answer.key_metric.label}
                    value={formatMetric(
                      m.answer.key_metric.label,
                      m.answer.key_metric.value,
                    )}
                    delta={m.answer.key_metric.delta_pct}
                  />
                )}
                {m.answer?.chart && (
                  <Chart
                    type={(m.answer.chart.type as any) || "bar"}
                    title={m.answer.chart.title}
                    columns={m.answer.chart.columns}
                    rows={m.answer.chart.rows}
                    height={280}
                  />
                )}
                {m.answer?.drivers && m.answer.drivers.length > 0 && (
                  <div>
                    <div className="text-[11px] font-medium uppercase tracking-wide text-mute">
                      O que puxa o número
                    </div>
                    <ul className="mt-2 space-y-1.5 text-sm text-ink">
                      {m.answer.drivers.map((d) => (
                        <li key={d} className="flex gap-2">
                          <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                          {d}
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
                {m.answer?.explanation && (
                  <p className="text-[13px] leading-relaxed text-mute">
                    {m.answer.explanation}
                  </p>
                )}
                {m.answer?.recommendation && (
                  <div className="rounded-xl border border-primary/15 bg-primary/5 px-3 py-2.5 text-[13px] leading-relaxed text-primary-700">
                    {m.answer.recommendation}
                  </div>
                )}
                {m.answer && !m.answer.insufficient_data && (
                  <div className="flex flex-wrap gap-1.5 border-t border-line pt-3">
                    {[
                      "E por região?",
                      "Compare com o período anterior",
                      "O que devo fazer agora?",
                    ].map((suggestion) => (
                      <button
                        key={suggestion}
                        type="button"
                        onClick={() => ask(suggestion)}
                        disabled={busy}
                        className="rounded-full border border-line bg-surface px-2.5 py-1.5 text-[11px] text-mute transition hover:border-primary/30 hover:text-primary disabled:opacity-50"
                      >
                        {suggestion}
                      </button>
                    ))}
                  </div>
                )}
                {m.answer?.warnings && m.answer.warnings.length > 0 && (
                  <div className="space-y-1 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-[12px] text-amber-800">
                    {m.answer.warnings.map((warning) => (
                      <p key={warning} className="flex items-start gap-1.5">
                        <AlertTriangle size={13} className="mt-0.5 shrink-0" />
                        {warning}
                      </p>
                    ))}
                  </div>
                )}
                {m.answer?.evidence &&
                  Object.keys(m.answer.evidence).length > 0 && (
                    <Evidence evidence={m.answer.evidence} />
                  )}
                {m.answer && !m.answer.insufficient_data && (
                  <p className="flex items-center gap-1 text-[10px] text-mute">
                    <CheckCircle2 size={11} className="text-emerald-600" />
                    Calculado no conjunto selecionado · confiança{" "}
                    {m.answer.confidence === "low"
                      ? "baixa"
                      : m.answer.confidence === "medium"
                        ? "média"
                        : "alta"}
                  </p>
                )}
                {m.meta && (
                  <div className="space-y-2 rounded-xl border border-line bg-bg p-3 text-[13px]">
                    {m.meta.sql && (
                      <pre className="overflow-x-auto font-mono text-[11px] text-ink">
                        {m.meta.sql}
                      </pre>
                    )}
                    {m.meta.expression && (
                      <p>
                        <span className="font-medium">{m.meta.name}</span> ={" "}
                        {m.meta.expression}
                      </p>
                    )}
                    {m.meta.explanation && (
                      <p className="text-mute">{m.meta.explanation}</p>
                    )}
                  </div>
                )}
              </Card>
            </div>
          ),
        )}
        {busy && (
          <div
            className="flex items-center gap-2 text-sm text-mute"
            aria-live="polite"
          >
            <Loader2 size={14} className="animate-spin text-primary" /> A
            consultar o conjunto…
          </div>
        )}
        <div ref={bottom} />
      </div>
      {showLatest && (
        <button
          type="button"
          onClick={() => chatScroll.current?.scrollTo({ top: chatScroll.current.scrollHeight, behavior: "smooth" })}
          className="absolute bottom-3 left-1/2 z-10 inline-flex -translate-x-1/2 items-center gap-1.5 rounded-full border border-primary/20 bg-surface px-3 py-2 text-[11px] font-medium text-primary shadow-lg shadow-slate-900/10 transition hover:-translate-y-px"
        >
          <ArrowUp size={13} className="rotate-180" /> Ver mensagens recentes
        </button>
      )}
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          ask(q);
        }}
        className="mt-3 shrink-0 rounded-2xl border border-line bg-surface p-2 pb-[calc(0.5rem+env(safe-area-inset-bottom))] shadow-[var(--shadow-card)]"
      >
        <textarea
          ref={inputRef}
          value={q}
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              ask(q);
            }
          }}
          placeholder={
            activeName
              ? `Pergunte sobre «${activeName}»…`
              : "Pergunte qualquer coisa sobre o seu negócio…"
          }
          aria-label="Pergunta"
          disabled={busy}
          rows={2}
          className="w-full resize-none bg-transparent px-2 py-1.5 text-sm text-ink outline-none placeholder:text-slate-400 disabled:opacity-60"
        />
        <div className="flex items-center justify-between gap-2 px-1 pb-0.5">
          <div className="flex gap-1">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => generateSQL.mutate(q || "valor por categoria")}
              busy={generateSQL.isPending}
              disabled={busy}
            >
              <Wand2 size={14} /> SQL
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => generateMeasure.mutate(q || "resultado")}
              busy={generateMeasure.isPending}
              disabled={busy}
            >
              <Wand2 size={14} /> Medida
            </Button>
          </div>
          <Button
            type="submit"
            size="icon"
            busy={busy}
            disabled={!q.trim()}
            aria-label="Enviar"
          >
            <ArrowUp size={16} />
          </Button>
        </div>
      </form>
    </div>
  );
}

function Capability({ icon: Icon, title, text }: { icon: typeof BarChart3; title: string; text: string }) {
  return (
    <div className="flex items-center gap-2.5 rounded-xl px-2 py-1.5">
      <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10 text-primary"><Icon size={15} /></div>
      <div><p className="text-[12px] font-medium text-ink">{title}</p><p className="text-[11px] text-mute">{text}</p></div>
    </div>
  );
}

"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { Chart, Kpi } from "@/components/viz";
import { toast } from "sonner";
import { Button, Card, Select, cn } from "@/components/ui";
import { ArrowUp, Loader2, Sparkles, Wand2 } from "lucide-react";

type Answer = {
  answer: string;
  key_metric?: { label: string; value: number; delta_pct?: number };
  chart?: { type: string; title: string; columns: string[]; rows: any[] };
  explanation?: string;
  drivers?: string[];
  recommendation?: string;
  evidence?: Record<string, any>;
  insufficient_data?: boolean;
};

type Msg = { role: "user" | "assistant"; text: string; answer?: Answer; meta?: any };
type Dataset = { id: string; name: string };

const examples = [
  "Qual o valor total?",
  "Valor por categoria",
  "Como está o resultado por natureza?",
  "Evolução do valor por mês",
  "Quais as maiores linhas?",
];

function formatMetric(label: string, value: number) {
  const money = /valor|receita|despesa|amount|revenue|total|montante/i.test(label);
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
      <summary className="cursor-pointer px-3 py-2 text-[12px] font-medium text-mute hover:text-ink">Como cheguei aqui</summary>
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
          <pre className="overflow-x-auto rounded-lg bg-slate-950 p-3 font-mono text-[11px] leading-relaxed text-slate-100">{sql}</pre>
        )}
        {!sql && <pre className="overflow-x-auto text-[11px]">{JSON.stringify(evidence, null, 2)}</pre>}
      </div>
    </details>
  );
}

export default function AskPage() {
  const [q, setQ] = useState("");
  const [msgs, setMsgs] = useState<Msg[]>([]);
  const [busy, setBusy] = useState(false);
  const [datasetId, setDatasetId] = useState("");
  const bottom = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  const datasets = useQuery({ queryKey: ["datasets"], queryFn: () => api<any>("/api/v1/datasets") });
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
    bottom.current?.scrollIntoView({ behavior: "smooth" });
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
        body: JSON.stringify({ message: prompt, dataset_id: activeId || undefined }),
      });
      setMsgs((m) => [...m, { role: "assistant", text: res.answer, answer: res }]);
    } catch (e: any) {
      toast.error(e.message);
      setMsgs((m) => [...m, { role: "assistant", text: "Não consegui responder agora. Tente de novo em instantes." }]);
    } finally {
      setBusy(false);
      inputRef.current?.focus();
    }
  }

  const generateSQL = useMutation({
    mutationFn: (prompt: string) =>
      api<{ sql: string; explanation: string }>("/api/v1/ai/generate-sql", { method: "POST", body: JSON.stringify({ prompt, dataset_id: activeId || undefined }) }),
    onSuccess: (res, prompt) => setMsgs((m) => [...m, { role: "assistant", text: "SQL para: " + prompt, meta: res }]),
    onError: (e: Error) => toast.error(e.message),
  });

  const generateMeasure = useMutation({
    mutationFn: (prompt: string) =>
      api<{ name: string; expression: string; explanation: string }>("/api/v1/ai/generate-measure", { method: "POST", body: JSON.stringify({ prompt, dataset_id: activeId || undefined }) }),
    onSuccess: (res, prompt) => setMsgs((m) => [...m, { role: "assistant", text: "Medida para: " + prompt, meta: res }]),
    onError: (e: Error) => toast.error(e.message),
  });

  return (
    <div className="mx-auto flex h-[calc(100vh-7rem)] max-w-3xl flex-col">
      <div className="mb-4 flex items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-ink">Perguntar à TheDobra</h1>
          <p className="mt-1 text-[13px] text-mute">Respostas com as métricas do seu conjunto — sem inventar fórmulas.</p>
        </div>
        {datasetList.length > 0 && (
          <Select
            aria-label="Conjunto"
            value={activeId}
            onChange={(e) => setDatasetId(e.target.value)}
            className="max-w-[220px] shrink-0"
          >
            {datasetList.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
              </option>
            ))}
          </Select>
        )}
      </div>

      <div className="min-h-0 flex-1 space-y-4 overflow-y-auto pr-1">
        {msgs.length === 0 && (
          <div className="flex min-h-full flex-col items-center justify-center px-2 py-8 text-center">
            <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary/10 text-primary">
              <Sparkles size={26} />
            </div>
            <h2 className="mt-4 text-lg font-medium text-ink">O analista do seu negócio</h2>
            <p className="mt-2 max-w-md text-[13px] leading-relaxed text-mute">
              Pergunte em português. A TheDobra usa o modelo semântico do conjunto
              {activeName ? ` «${activeName}»` : ""} e devolve números com evidência.
            </p>
            <div className="mt-6 flex max-w-lg flex-wrap justify-center gap-2">
              {examplesForData.map((ex) => (
                <button
                  key={ex}
                  type="button"
                  onClick={() => ask(ex)}
                  className="rounded-full border border-line bg-white px-3.5 py-2 text-[12px] text-ink shadow-sm transition hover:border-primary/40 hover:text-primary"
                >
                  {ex}
                </button>
              ))}
            </div>
          </div>
        )}

        {msgs.map((m, i) =>
          m.role === "user" ? (
            <div key={i} className="flex justify-end">
              <div className="max-w-[85%] rounded-2xl rounded-br-md bg-primary px-4 py-2.5 text-sm leading-relaxed text-white">{m.text}</div>
            </div>
          ) : (
            <div key={i} className="flex gap-2.5">
              <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
                <Sparkles size={14} />
              </div>
              <Card className={cn("min-w-0 flex-1 space-y-3", m.answer?.insufficient_data && "border-amber-200 bg-amber-50/40")}>
                <p className="text-sm leading-relaxed text-ink">{m.text}</p>
                {m.answer?.key_metric && (
                  <Kpi
                    label={m.answer.key_metric.label}
                    value={formatMetric(m.answer.key_metric.label, m.answer.key_metric.value)}
                    delta={m.answer.key_metric.delta_pct}
                  />
                )}
                {m.answer?.chart && (
                  <Chart type={(m.answer.chart.type as any) || "bar"} title={m.answer.chart.title} columns={m.answer.chart.columns} rows={m.answer.chart.rows} />
                )}
                {m.answer?.drivers && m.answer.drivers.length > 0 && (
                  <div>
                    <div className="text-[11px] font-medium uppercase tracking-wide text-mute">O que puxa o número</div>
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
                {m.answer?.explanation && <p className="text-[13px] leading-relaxed text-mute">{m.answer.explanation}</p>}
                {m.answer?.recommendation && (
                  <div className="rounded-xl border border-primary/15 bg-primary/5 px-3 py-2.5 text-[13px] leading-relaxed text-primary-700">
                    {m.answer.recommendation}
                  </div>
                )}
                {m.answer?.evidence && Object.keys(m.answer.evidence).length > 0 && <Evidence evidence={m.answer.evidence} />}
                {m.meta && (
                  <div className="space-y-2 rounded-xl border border-line bg-bg p-3 text-[13px]">
                    {m.meta.sql && <pre className="overflow-x-auto font-mono text-[11px] text-ink">{m.meta.sql}</pre>}
                    {m.meta.expression && (
                      <p>
                        <span className="font-medium">{m.meta.name}</span> = {m.meta.expression}
                      </p>
                    )}
                    {m.meta.explanation && <p className="text-mute">{m.meta.explanation}</p>}
                  </div>
                )}
              </Card>
            </div>
          ),
        )}
        {busy && (
          <div className="flex items-center gap-2 text-sm text-mute" aria-live="polite">
            <Loader2 size={14} className="animate-spin text-primary" /> A consultar o conjunto…
          </div>
        )}
        <div ref={bottom} />
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          ask(q);
        }}
        className="mt-3 shrink-0 rounded-2xl border border-line bg-white p-2 shadow-sm"
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
          placeholder={activeName ? `Pergunte sobre «${activeName}»…` : "Pergunte qualquer coisa sobre o seu negócio…"}
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
          <Button type="submit" size="icon" busy={busy} disabled={!q.trim()} aria-label="Enviar">
            <ArrowUp size={16} />
          </Button>
        </div>
      </form>
    </div>
  );
}

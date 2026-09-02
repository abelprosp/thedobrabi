"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Chart, Kpi } from "@/components/viz";
import { toast } from "sonner";
import { Button, Card, Input, PageHeader, Textarea } from "@/components/ui";
import { ChevronDown, Sparkles, Wand2 } from "lucide-react";

type Answer = {
  answer: string;
  key_metric?: { label: string; value: number; delta_pct?: number };
  chart?: { type: string; title: string; columns: string[]; rows: any[] };
  explanation?: string;
  drivers?: string[];
  recommendation?: string;
  evidence?: Record<string, any>;
};

type Msg = { role: "user" | "assistant"; text: string; answer?: Answer; meta?: any };

const examples = [
  "Porque caiu a receita este mês?",
  "Quais são os meus 10 maiores clientes?",
  "Compare vendas de agosto e setembro.",
  "Que produtos estão a perder margem?",
  "Preveja a receita para dezembro.",
];

export default function AskPage() {
  const [q, setQ] = useState("");
  const [msgs, setMsgs] = useState<Msg[]>([]);
  const [busy, setBusy] = useState(false);

  async function ask(text: string) {
    const prompt = text.trim();
    if (!prompt || busy) return;
    setBusy(true);
    setQ("");
    setMsgs((m) => [...m, { role: "user", text: prompt }]);
    try {
      const res = await api<Answer>("/api/v1/ai/ask", { method: "POST", body: JSON.stringify({ message: prompt }) });
      setMsgs((m) => [...m, { role: "assistant", text: res.answer, answer: res }]);
    } catch (e: any) {
      toast.error(e.message);
      setMsgs((m) => [...m, { role: "assistant", text: "Não consegui responder agora. Tente novamente." }]);
    } finally {
      setBusy(false);
    }
  }

  const generateSQL = useMutation({
    mutationFn: (prompt: string) => api<{ sql: string; explanation: string }>("/api/v1/ai/generate-sql", { method: "POST", body: JSON.stringify({ prompt }) }),
    onSuccess: (res, prompt) => setMsgs((m) => [...m, { role: "assistant", text: "SQL gerado para: " + prompt, meta: res }]),
    onError: (e: Error) => toast.error(e.message),
  });

  const generateMeasure = useMutation({
    mutationFn: (prompt: string) => api<{ name: string; expression: string; explanation: string }>("/api/v1/ai/generate-measure", { method: "POST", body: JSON.stringify({ prompt }) }),
    onSuccess: (res, prompt) => setMsgs((m) => [...m, { role: "assistant", text: "Medida gerada para: " + prompt, meta: res }]),
    onError: (e: Error) => toast.error(e.message),
  });

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <PageHeader title="Perguntar à TheDobra" description="As respostas usam métricas oficiais e consultas reais. Gere SQL e DAX-like quando precisar." />
      {msgs.length === 0 && (
        <div className="space-y-4">
          <div className="rounded-2xl border border-line bg-white p-4 shadow-sm">
            <div className="text-sm font-medium text-ink">Como funciona</div>
            <p className="mt-1 text-[13px] text-mute">
              Pergunte em linguagem natural. A TheDobra traduz para a camada semântica, executa a consulta e devolve a resposta com evidência.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            {examples.map((ex) => (
              <button
                key={ex}
                type="button"
                onClick={() => ask(ex)}
                className="min-h-9 rounded-full border border-line bg-white px-3 py-1.5 text-[12px] text-mute hover:border-accent/40 hover:text-ink"
              >
                {ex}
              </button>
            ))}
          </div>
        </div>
      )}
      <div className="space-y-4">
        {msgs.map((m, i) =>
          m.role === "user" ? (
            <div key={i} className="flex justify-end">
              <div className="max-w-[85%] rounded-2xl rounded-br-md bg-accent px-4 py-2.5 text-sm text-white">{m.text}</div>
            </div>
          ) : (
            <div key={i} className="flex gap-2">
              <div className="mt-1 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-accent/10 text-accent">
                <Sparkles size={14} />
              </div>
              <Card className="min-w-0 flex-1 space-y-3">
                <p className="text-sm leading-relaxed text-ink">{m.text}</p>
                {m.answer?.key_metric && (
                  <Kpi
                    label={m.answer.key_metric.label}
                    value={m.answer.key_metric.value.toLocaleString("pt-BR", { maximumFractionDigits: 0 })}
                    delta={m.answer.key_metric.delta_pct}
                  />
                )}
                {m.answer?.chart && <Chart type={(m.answer.chart.type as any) || "bar"} title={m.answer.chart.title} columns={m.answer.chart.columns} rows={m.answer.chart.rows} />}
                {m.answer?.drivers && m.answer.drivers.length > 0 && (
                  <div>
                    <div className="text-[12px] uppercase text-mute">Factores</div>
                    <ul className="mt-2 list-disc space-y-1 pl-4 text-sm">
                      {m.answer.drivers.map((d) => <li key={d}>{d}</li>)}
                    </ul>
                  </div>
                )}
                {m.answer?.explanation && <p className="text-sm text-mute">{m.answer.explanation}</p>}
                {m.answer?.recommendation && <div className="rounded-xl bg-accent/10 p-3 text-sm text-accent-2">Recomendação: {m.answer.recommendation}</div>}
                {m.answer?.evidence && (
                  <details className="rounded-xl bg-bg">
                    <summary className="flex min-h-10 cursor-pointer items-center gap-2 px-3 text-[12px] font-medium text-mute">
                      <ChevronDown size={14} /> Evidência e SQL
                    </summary>
                    <pre className="overflow-x-auto p-3 text-[11px] text-mute">{JSON.stringify(m.answer.evidence, null, 2)}</pre>
                  </details>
                )}
                {m.meta && (
                  <details className="rounded-xl bg-bg">
                    <summary className="flex min-h-10 cursor-pointer items-center gap-2 px-3 text-[12px] font-medium text-mute">
                      <ChevronDown size={14} /> Resultado da IA
                    </summary>
                    <pre className="overflow-x-auto p-3 text-[11px] text-mute">{JSON.stringify(m.meta, null, 2)}</pre>
                  </details>
                )}
              </Card>
            </div>
          ),
        )}
        {busy && <p className="text-sm text-mute" aria-live="polite">A analisar…</p>}
      </div>
      <form
        onSubmit={(e) => { e.preventDefault(); ask(q); }}
        className="sticky bottom-0 flex gap-2 bg-bg/90 py-2 backdrop-blur"
      >
        <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Pergunte qualquer coisa sobre o seu negócio…" aria-label="Pergunta" disabled={busy} />
        <Button type="submit" busy={busy} disabled={!q.trim()}>Perguntar</Button>
      </form>
      <div className="flex gap-2">
        <Button variant="secondary" size="sm" onClick={() => generateSQL.mutate(q || "receita por região")} busy={generateSQL.isPending}>
          <Wand2 size={14} /> Gerar SQL
        </Button>
        <Button variant="secondary" size="sm" onClick={() => generateMeasure.mutate(q || "margem")} busy={generateMeasure.isPending}>
          <Wand2 size={14} /> Gerar medida
        </Button>
      </div>
    </div>
  );
}

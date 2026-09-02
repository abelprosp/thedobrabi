"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useMutation, useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Button, Card, CardTitle, PageHeader } from "@/components/ui";
import { useOnboarding } from "@/components/onboarding";
import { toast } from "sonner";
import { Database, MessageSquare, LayoutDashboard, Upload, Server, Sparkles, ArrowRight, ArrowLeft } from "lucide-react";

const suggestedQuestions = [
  "Quanto vendi este mês?",
  "Quais são os produtos mais vendidos?",
  "Comparar vendas por região.",
];

export default function WelcomePage() {
  const router = useRouter();
  const { complete } = useOnboarding();
  const [step, setStep] = useState(1);
  const [dataset, setDataset] = useState<{ id: string; name: string } | null>(null);
  const [question, setQuestion] = useState("");
  const [answer, setAnswer] = useState("");
  const [dashboard, setDashboard] = useState<{ id: string; name: string } | null>(null);

  const demo = useMutation({
    mutationFn: () => api<{ dataset_id: string; name: string }>("/api/v1/datasets/demo", { method: "POST" }),
    onSuccess: (res) => {
      setDataset({ id: res.dataset_id, name: res.name });
      toast.success("Dados de demonstração carregados");
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const ask = useMutation({
    mutationFn: () => api<{ answer: string }>("/api/v1/ai/ask", { method: "POST", body: JSON.stringify({ message: question }) }),
    onSuccess: (res) => {
      setAnswer(res.answer);
      toast.success("A TheDobra respondeu");
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const aiDashboard = useMutation({
    mutationFn: () => api<{ id: string; name: string }>("/api/v1/dashboards/ai", { method: "POST", body: JSON.stringify({ prompt: "Dashboard executivo de vendas", dataset_id: dataset?.id }) }),
    onSuccess: (res) => {
      setDashboard(res);
      toast.success("Dashboard criado");
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const finish = () => {
    complete();
    router.replace("/overview");
  };

  return (
    <div className="mx-auto max-w-3xl space-y-6 py-8">
      <div className="text-center">
        <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/10 text-primary">
          <Sparkles size={28} />
        </div>
        <h1 className="text-3xl font-semibold tracking-tight text-ink">Bem-vindo à TheDobra</h1>
        <p className="mt-2 text-sm text-mute">Três passos para começar a analisar o seu negócio.</p>
      </div>

      <div className="mb-6 flex items-center justify-center gap-2">
        {[1, 2, 3].map((s) => (
          <div key={s} className={`h-2 w-12 rounded-full ${s <= step ? "bg-primary" : "bg-surface-2"}`} />
        ))}
      </div>

      {step === 1 && (
        <Card className="space-y-4">
          <CardTitle>1. Dados</CardTitle>
          <p className="text-sm text-mute">Comece com dados de demonstração para explorar a plataforma.</p>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <button
              onClick={() => demo.mutate()}
              className="flex flex-col items-center gap-3 rounded-2xl border border-line bg-surface p-5 text-center hover:border-primary/40"
            >
              <Database className="text-primary" size={24} />
              <span className="text-sm font-medium">Carregar demo</span>
            </button>
            <button
              onClick={() => router.push("/data")}
              className="flex flex-col items-center gap-3 rounded-2xl border border-line bg-surface p-5 text-center hover:border-primary/40"
            >
              <Upload className="text-primary" size={24} />
              <span className="text-sm font-medium">Upload CSV</span>
            </button>
            <button
              onClick={() => router.push("/data")}
              className="flex flex-col items-center gap-3 rounded-2xl border border-line bg-surface p-5 text-center hover:border-primary/40"
            >
              <Server className="text-primary" size={24} />
              <span className="text-sm font-medium">Ligar SQL</span>
            </button>
          </div>
          {dataset && (
            <div className="rounded-xl bg-primary/5 p-3 text-sm text-primary">
              Conjunto pronto: <strong>{dataset.name}</strong>
            </div>
          )}
          <div className="flex justify-end">
            <Button onClick={() => setStep(2)} disabled={!dataset}>
              Próximo <ArrowRight size={14} />
            </Button>
          </div>
        </Card>
      )}

      {step === 2 && (
        <Card className="space-y-4">
          <CardTitle>2. Perguntar à TheDobra</CardTitle>
          <p className="text-sm text-mute">Escolha uma sugestão ou escreva a sua pergunta.</p>
          <div className="flex flex-wrap gap-2">
            {suggestedQuestions.map((q) => (
              <button
                key={q}
                onClick={() => setQuestion(q)}
                className="rounded-full border border-line bg-white px-3 py-1.5 text-[12px] text-mute hover:border-primary/40 hover:text-ink"
              >
                {q}
              </button>
            ))}
          </div>
          <div className="flex gap-2">
            <input
              className="min-h-10 flex-1 rounded-xl border border-line bg-white px-3.5 text-sm text-ink outline-none focus:border-primary/50"
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
              placeholder="Pergunte sobre os dados…"
            />
            <Button onClick={() => ask.mutate()} busy={ask.isPending} disabled={!question.trim()}>
              <MessageSquare size={14} /> Perguntar
            </Button>
          </div>
          {answer && (
            <div className="rounded-xl bg-bg p-4 text-sm text-ink">
              <strong>Resposta:</strong> {answer}
            </div>
          )}
          <div className="flex justify-between">
            <Button variant="ghost" onClick={() => setStep(1)}>
              <ArrowLeft size={14} /> Voltar
            </Button>
            <Button onClick={() => setStep(3)} disabled={!answer}>
              Próximo <ArrowRight size={14} />
            </Button>
          </div>
        </Card>
      )}

      {step === 3 && (
        <Card className="space-y-4">
          <CardTitle>3. Primeiro dashboard</CardTitle>
          <p className="text-sm text-mute">A IA cria automaticamente um painel executivo com KPI, linha, barras e tabela.</p>
          {!dashboard ? (
            <Button onClick={() => aiDashboard.mutate()} busy={aiDashboard.isPending}>
              <LayoutDashboard size={14} /> Criar dashboard com IA
            </Button>
          ) : (
            <div className="rounded-xl bg-primary/5 p-3 text-sm text-primary">
              Dashboard criado: <strong>{dashboard.name}</strong>
            </div>
          )}
          <div className="flex justify-between">
            <Button variant="ghost" onClick={() => setStep(2)}>
              <ArrowLeft size={14} /> Voltar
            </Button>
            <Button onClick={finish} disabled={!dashboard}>
              Concluir <ArrowRight size={14} />
            </Button>
          </div>
        </Card>
      )}
    </div>
  );
}

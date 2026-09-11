"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api, normalizeArray } from "@/lib/api";
import { Button, FieldLabel, Input, Textarea, cn } from "@/components/ui";
import {
  CheckCircle2,
  XCircle,
  Loader2,
  X,
  Code2,
  Blocks,
  Sparkles,
} from "lucide-react";
import type { SemanticModel, SemanticMeasure } from "@/lib/semantic";
import { MeasureBlockBuilder } from "@/components/measure-block-builder";
import {
  compileBlock,
  expressionToBlocks,
  type MeasureBlock,
} from "@/lib/measure-blocks";

type ValidationResult =
  | { valid: true; sql: string; func: string }
  | { valid: false; error: string };

type EditorMode = "blocks" | "sql";

export function CustomMeasureModal({
  semanticModelId,
  model,
  datasetId,
  onClose,
  onAdded,
}: {
  semanticModelId: string;
  model: SemanticModel;
  datasetId?: string;
  onClose: () => void;
  onAdded: (measure: SemanticMeasure) => void;
}) {
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [mode, setMode] = useState<EditorMode>("blocks");
  const [expression, setExpression] = useState("");
  const [blocks, setBlocks] = useState<MeasureBlock | null>(null);
  const [validation, setValidation] = useState<ValidationResult | null>(null);
  const [aiPrompt, setAiPrompt] = useState("");
  const [aiNote, setAiNote] = useState<string | null>(null);
  const resolvedDatasetId = datasetId || model.dataset_id;

  const datasets = useQuery({
    queryKey: ["datasets"],
    queryFn: () => api<any>("/api/v1/datasets"),
  });
  const aiConfig = useQuery({
    queryKey: ["ai-config"],
    queryFn: () => api<{ openai_configured: boolean }>("/api/v1/ai/config"),
  });
  const tables = normalizeArray<{ id: string; name: string }>(
    datasets.data,
  ).map((d) => ({ name: d.name }));

  const columns = useMemo(() => {
    const seen = new Set<string>();
    const out: { name: string }[] = [];
    for (const m of model.measures || []) {
      const col = (m.column || "").trim();
      if (col && col !== "*" && !seen.has(col)) {
        seen.add(col);
        out.push({ name: col });
      }
    }
    for (const d of model.dimensions || []) {
      const col = (d.column || d.name || "").trim();
      if (col && !seen.has(col)) {
        seen.add(col);
        out.push({ name: col });
      }
    }
    return out;
  }, [model.measures, model.dimensions]);

  const existingMeasures = (model.measures || []).filter((m) =>
    (m.name || "").trim(),
  );

  const expressionFromBlocks = compileBlock(blocks);
  const activeExpression =
    mode === "blocks" ? expressionFromBlocks : expression;

  const validate = useMutation({
    mutationFn: () =>
      api<ValidationResult>(
        `/api/v1/semantic-models/${semanticModelId}/validate-measure`,
        {
          method: "POST",
          body: JSON.stringify({ expression: activeExpression }),
        },
      ),
    onSuccess: (res) => setValidation(res),
    onError: (e: Error) => setValidation({ valid: false, error: e.message }),
  });

  const save = useMutation({
    mutationFn: async () => {
      const newMeasure: SemanticMeasure = {
        name: name.trim(),
        expression: activeExpression,
        aggregation: "expression",
        editor_mode: mode,
        expression_blocks: mode === "blocks" ? blocks || undefined : undefined,
      };
      const updatedModel: SemanticModel = {
        ...model,
        measures: [...(model.measures || []), newMeasure],
      };
      await api(`/api/v1/semantic-models/${semanticModelId}`, {
        method: "PUT",
        body: JSON.stringify(updatedModel),
      });
      return newMeasure;
    },
    onSuccess: (newMeasure) => {
      qc.invalidateQueries({ queryKey: ["semantic"] });
      qc.invalidateQueries({ queryKey: ["datasets"] });
      onAdded(newMeasure);
      onClose();
    },
  });

  const generate = useMutation({
    mutationFn: (prompt: string) =>
      api<{
        name: string;
        expression: string;
        explanation: string;
        source?: string;
        validated?: boolean;
        confidence?: string;
        referenced_fields?: string[];
        warnings?: string[];
      }>("/api/v1/ai/generate-measure", {
        method: "POST",
        body: JSON.stringify({
          prompt,
          dataset_id: resolvedDatasetId || undefined,
        }),
      }),
    onSuccess: (res) => {
      const expr = (res.expression || "").trim();
      if (res.name?.trim()) setName(res.name.trim());
      setExpression(expr);
      setValidation(null);
      const tree = expressionToBlocks(expr);
      const compiled = tree ? compileBlock(tree) : "";
      const norm = (s: string) =>
        s
          .replace(/\s/g, "")
          .replace(/\bAVERAGE\b/gi, "AVG")
          .toUpperCase();
      if (tree && compiled && norm(compiled) === norm(expr)) {
        setBlocks(tree);
        setMode("blocks");
      } else {
        setBlocks(null);
        setMode("sql");
      }
      const fields = res.referenced_fields?.length
        ? ` Campos usados: ${res.referenced_fields.join(", ")}.`
        : "";
      const warnings = res.warnings?.length ? ` ${res.warnings.join(" ")}` : "";
      setAiNote(
        `${res.explanation || "Medida preenchida."}${fields}${warnings}`,
      );
      toast.success(
        res.validated
          ? "Medida gerada com campos validados."
          : "Medida gerada — reveja e valide.",
      );
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const canValidate = activeExpression.trim().length > 0;
  const canSave =
    validation?.valid === true && name.trim().length > 0 && !save.isPending;

  const handleSqlChange = (v: string) => {
    setExpression(v);
    setValidation(null);
  };

  const handleBlocksChange = (next: MeasureBlock | null) => {
    setBlocks(next);
    setValidation(null);
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4"
      onClick={onClose}
    >
      <div
        className="flex max-h-[92dvh] w-full max-w-4xl flex-col overflow-hidden rounded-t-2xl border border-line bg-surface shadow-xl sm:rounded-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-3 border-b border-line px-5 py-4">
          <div>
            <h3 className="text-[15px] font-semibold text-ink">Nova medida</h3>
            <p className="text-[12px] text-mute">
              Escreva SQL, monte com blocos, ou peça à IA para preencher a
              fórmula.
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="flex h-10 w-10 items-center justify-center rounded-lg text-mute hover:bg-surface-2 hover:text-ink"
            aria-label="Fechar"
          >
            <X size={16} />
          </button>
        </div>

        <div className="space-y-4 overflow-y-auto px-5 py-4">
          <div className="space-y-3 rounded-xl border border-line bg-surface-2/50 p-3">
            <div className="flex items-center gap-2 text-[13px] font-medium text-ink">
              <Sparkles size={14} className="text-primary" />
              Criar com IA
            </div>
            <Textarea
              value={aiPrompt}
              onChange={(e) => setAiPrompt(e.target.value)}
              placeholder="Ex: ticket médio por cliente, receita líquida, variação face ao ano anterior"
              className="min-h-[72px] text-[13px]"
              onKeyDown={(e) => {
                if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                  e.preventDefault();
                  const prompt = aiPrompt.trim();
                  if (prompt && !generate.isPending) generate.mutate(prompt);
                }
              }}
            />
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-[11px] text-mute">
                {aiConfig.data?.openai_configured === false
                  ? "Sem chave OpenAI usa as colunas deste conjunto para uma sugestão. Valide sempre."
                  : "A IA preenche o nome e a expressão. Valide antes de guardar."}
              </p>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => generate.mutate(aiPrompt.trim())}
                disabled={!aiPrompt.trim()}
                busy={generate.isPending}
              >
                <Sparkles size={14} /> Gerar com IA
              </Button>
            </div>
            {aiNote && <p className="text-[12px] text-ink">{aiNote}</p>}
          </div>

          <FieldLabel
            label="Nome da medida"
            hint="Ex: Ticket médio, Receita líquida"
          >
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Ex: Ticket médio"
            />
          </FieldLabel>

          <div className="inline-flex rounded-xl border border-line bg-bg p-1">
            <button
              type="button"
              onClick={() => {
                if (!blocks && expression) {
                  const tree = expressionToBlocks(expression);
                  if (tree) setBlocks(tree);
                }
                setMode("blocks");
                setValidation(null);
              }}
              className={cn(
                "inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-[12px] font-medium",
                mode === "blocks"
                  ? "bg-surface text-ink shadow-sm"
                  : "text-mute hover:text-ink",
              )}
            >
              <Blocks size={14} /> Blocos
            </button>
            <button
              type="button"
              onClick={() => {
                if (mode === "blocks" && expressionFromBlocks)
                  setExpression(expressionFromBlocks);
                setMode("sql");
                setValidation(null);
              }}
              className={cn(
                "inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-[12px] font-medium",
                mode === "sql"
                  ? "bg-surface text-ink shadow-sm"
                  : "text-mute hover:text-ink",
              )}
            >
              <Code2 size={14} /> SQL
            </button>
          </div>

          {mode === "blocks" ? (
            <div className="space-y-2">
              <p className="text-[12px] text-mute">
                Arraste agregações, contas e buscas para o canvas. A medida é
                compilada para a mesma linguagem do motor.
              </p>
              <MeasureBlockBuilder
                value={blocks}
                onChange={handleBlocksChange}
                columns={columns}
                measures={existingMeasures.map((m) => ({ name: m.name }))}
                dimensions={model.dimensions || []}
                tables={tables}
              />
            </div>
          ) : (
            <>
              <FieldLabel
                label="Expressão SQL"
                hint="Só a expressão — sem SELECT, FROM ou AS. SUM, AVG, COUNT, DIVIDE, CASE WHEN, CALCULATE, LOOKUPVALUE, RELATED"
              >
                <Textarea
                  value={expression}
                  onChange={(e) => handleSqlChange(e.target.value)}
                  placeholder={
                    "Exemplos:\nSUM(valor_mensal)\nDIVIDE(SUM(valor_mensal), COUNT(DISTINCT cliente))\nLOOKUPVALUE(Clientes[região], Clientes[id], cliente_id)"
                  }
                  className="min-h-[120px] font-mono text-[12px]"
                />
              </FieldLabel>
              {columns.length > 0 && (
                <div className="rounded-xl border border-line bg-surface-2/60 p-3">
                  <p className="mb-1.5 text-[11px] font-medium text-mute">
                    Colunas (clique para inserir):
                  </p>
                  <div className="flex flex-wrap gap-1">
                    {columns.map((c) => (
                      <code
                        key={c.name}
                        className="cursor-pointer rounded bg-surface px-1.5 py-0.5 text-[10px] text-ink hover:bg-primary/10 hover:text-primary"
                        onClick={() =>
                          handleSqlChange(
                            expression +
                              (expression && !expression.endsWith(" ")
                                ? " "
                                : "") +
                              c.name,
                          )
                        }
                      >
                        {c.name}
                      </code>
                    ))}
                  </div>
                </div>
              )}
            </>
          )}

          {validation && (
            <div
              className={cn(
                "flex items-start gap-2 rounded-xl border p-3 text-[12px]",
                validation.valid
                  ? "border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                  : "border-red-200 bg-red-50 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-300",
              )}
            >
              {validation.valid ? (
                <CheckCircle2
                  size={14}
                  className="mt-0.5 shrink-0 text-emerald-600"
                />
              ) : (
                <XCircle size={14} className="mt-0.5 shrink-0 text-red-600" />
              )}
              <div>
                {validation.valid ? (
                  <>
                    <span className="font-medium">Expressão válida.</span>
                    <div className="mt-0.5 font-mono text-[11px] opacity-75">
                      SQL: {validation.sql}
                    </div>
                  </>
                ) : (
                  <>
                    <span className="font-medium">Expressão inválida.</span>
                    <div className="mt-0.5">{validation.error}</div>
                  </>
                )}
              </div>
            </div>
          )}
        </div>

        <div className="flex flex-col-reverse gap-2 border-t border-line px-5 py-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] sm:flex-row sm:justify-end">
          <Button variant="secondary" onClick={onClose}>
            Cancelar
          </Button>
          <Button
            variant="secondary"
            onClick={() => validate.mutate()}
            disabled={!canValidate || validate.isPending}
          >
            {validate.isPending && (
              <Loader2 size={14} className="animate-spin" />
            )}
            Validar
          </Button>
          <Button
            onClick={() => save.mutate()}
            disabled={!canSave}
            busy={save.isPending}
          >
            Adicionar medida
          </Button>
        </div>
      </div>
    </div>
  );
}

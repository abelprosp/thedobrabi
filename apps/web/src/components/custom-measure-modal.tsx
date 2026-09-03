"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Button, FieldLabel, Input, Textarea, cn } from "@/components/ui";
import { CheckCircle2, XCircle, Loader2, X, Code2 } from "lucide-react";
import type { SemanticModel, SemanticMeasure } from "@/lib/semantic";

type ValidationResult =
  | { valid: true; sql: string; func: string }
  | { valid: false; error: string };

export function CustomMeasureModal({
  semanticModelId,
  model,
  onClose,
  onAdded,
}: {
  semanticModelId: string;
  model: SemanticModel;
  onClose: () => void;
  onAdded: (measure: SemanticMeasure) => void;
}) {
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [expression, setExpression] = useState("");
  const [validation, setValidation] = useState<ValidationResult | null>(null);

  const validate = useMutation({
    mutationFn: () =>
      api<ValidationResult>(`/api/v1/semantic-models/${semanticModelId}/validate-measure`, {
        method: "POST",
        body: JSON.stringify({ expression }),
      }),
    onSuccess: (res) => setValidation(res),
    onError: (e: Error) => setValidation({ valid: false, error: e.message }),
  });

  const save = useMutation({
    mutationFn: async () => {
      const newMeasure: SemanticMeasure = {
        name: name.trim(),
        expression,
        aggregation: "expression",
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
      onAdded(newMeasure);
      onClose();
    },
  });

  const canValidate = expression.trim().length > 0;
  const canSave = validation?.valid === true && name.trim().length > 0 && !save.isPending;

  const handleExpressionChange = (v: string) => {
    setExpression(v);
    setValidation(null);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" onClick={onClose}>
      <div
        className="w-full max-w-lg rounded-2xl border border-line bg-surface p-5 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-4 flex items-start justify-between gap-3">
          <div className="flex items-center gap-2">
            <Code2 size={18} className="text-primary" />
            <div>
              <h3 className="text-[15px] font-semibold text-ink">Nova medida SQL</h3>
              <p className="text-[11px] text-mute">Escreva uma expressão SQL agregada para criar uma métrica personalizada.</p>
            </div>
          </div>
          <button onClick={onClose} className="rounded-lg p-1 text-mute hover:bg-surface-2 hover:text-ink">
            <X size={16} />
          </button>
        </div>

        <div className="space-y-4">
          <FieldLabel label="Nome da medida" hint="Ex: Ticket Médio, Receita Líquida">
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Ex: Ticket Médio" />
          </FieldLabel>

          <FieldLabel
            label="Expressão SQL"
            hint="Use funções de agregação: SUM, AVG, COUNT, MIN, MAX"
          >
            <Textarea
              value={expression}
              onChange={(e) => handleExpressionChange(e.target.value)}
              placeholder={"Exemplos:\nSUM(valor_mensal)\nSUM(valor_mensal) / NULLIF(COUNT(DISTINCT cliente), 0)\nSUM(CASE WHEN vendedor = 'RICARDO' THEN valor_mensal ELSE 0 END)"}
              className="min-h-[120px] font-mono text-[12px]"
            />
          </FieldLabel>

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
                <CheckCircle2 size={14} className="mt-0.5 shrink-0 text-emerald-600" />
              ) : (
                <XCircle size={14} className="mt-0.5 shrink-0 text-red-600" />
              )}
              <div>
                {validation.valid ? (
                  <>
                    <span className="font-medium">Expressão válida.</span>
                    <div className="mt-0.5 font-mono text-[11px] opacity-75">SQL: {validation.sql}</div>
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

          {(model.measures || []).length > 0 && (
            <div className="rounded-xl border border-line bg-surface-2/60 p-3">
              <p className="mb-1.5 text-[11px] font-medium text-mute">Colunas numéricas disponíveis (clique para inserir):</p>
              <div className="flex flex-wrap gap-1">
                {(model.measures || []).map((m) => (
                  <code
                    key={m.name}
                    className="cursor-pointer rounded bg-surface px-1.5 py-0.5 text-[10px] text-ink hover:bg-primary/10 hover:text-primary"
                    onClick={() =>
                      handleExpressionChange(
                        expression + (expression && !expression.endsWith(" ") ? " " : "") + (m.column || m.name || ""),
                      )
                    }
                  >
                    {m.column || m.name}
                  </code>
                ))}
              </div>
            </div>
          )}
        </div>

        <div className="mt-5 flex justify-end gap-2">
          <Button variant="secondary" onClick={onClose}>
            Cancelar
          </Button>
          <Button variant="secondary" onClick={() => validate.mutate()} disabled={!canValidate || validate.isPending}>
            {validate.isPending && <Loader2 size={14} className="animate-spin" />}
            Validar
          </Button>
          <Button onClick={() => save.mutate()} disabled={!canSave} busy={save.isPending}>
            Adicionar medida
          </Button>
        </div>
      </div>
    </div>
  );
}

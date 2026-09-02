"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { ArrowRight, Database, FileSpreadsheet, GitMerge, Plug, Workflow, X } from "lucide-react";
import { api, normalizeArray } from "@/lib/api";
import { Button, FieldLabel, Input, Select, cn } from "@/components/ui";
import {
  FLOW_TEMPLATES,
  type DatasetOption,
  type FlowTemplateId,
  suggestedSource,
  templateSteps,
} from "@/lib/flows";

const TEMPLATE_ICON = {
  csv_clickhouse: FileSpreadsheet,
  sql_transform: Database,
  connector_ch: Plug,
  join_sources: GitMerge,
} as const;

export function NewFlowWizard({ open, onClose }: { open: boolean; onClose: () => void }) {
  const router = useRouter();
  const datasets = useQuery({
    queryKey: ["datasets"],
    queryFn: () => api<any>("/api/v1/datasets"),
    enabled: open,
  });
  const datasetList = normalizeArray<DatasetOption>(datasets.data);
  const [templateId, setTemplateId] = useState<FlowTemplateId>("csv_clickhouse");
  const [name, setName] = useState("CSV para ClickHouse");
  const [sourceId, setSourceId] = useState("");
  const [source2Id, setSource2Id] = useState("");

  const template = FLOW_TEMPLATES.find((t) => t.id === templateId) || FLOW_TEMPLATES[0];

  useEffect(() => {
    if (!open) return;
    setTemplateId("csv_clickhouse");
    setName("CSV para ClickHouse");
    setSourceId("");
    setSource2Id("");
  }, [open]);

  useEffect(() => {
    if (!open || !datasetList.length) return;
    setSourceId((prev) => prev || suggestedSource(datasetList, template.preferConnector));
    setSource2Id((prev) => prev || datasetList[1]?.id || "");
  }, [open, datasetList, template.preferConnector]);

  const sourceOptions = useMemo(() => {
    if (!template.preferConnector) return datasetList;
    const preferred = datasetList.filter((d) => d.source_id);
    return preferred.length ? preferred : datasetList;
  }, [datasetList, template.preferConnector]);

  const create = useMutation({
    mutationFn: async () => {
      const trimmed = name.trim() || template.defaultName;
      const src = template.needsSource ? sourceId : "";
      if (template.needsSource && !src) {
        throw new Error("Escolha o conjunto de origem");
      }
      if (template.needsSecondSource && !source2Id) {
        throw new Error("Escolha a segunda origem");
      }
      if (template.needsSecondSource && source2Id === src) {
        throw new Error("Escolha dois conjuntos diferentes");
      }
      const steps = templateSteps({ template: template.id, name: trimmed, sourceId: src, source2Id });
      return api<{ id: string }>("/api/v1/flows", {
        method: "POST",
        body: JSON.stringify({
          name: trimmed,
          description: template.description,
          source_dataset_id: src || undefined,
          steps,
        }),
      });
    },
    onSuccess: (d) => {
      toast.success("Flow criado com o pipeline inicial");
      onClose();
      router.push(`/flows/${d.id}`);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const createBlank = useMutation({
    mutationFn: () =>
      api<{ id: string }>("/api/v1/flows", {
        method: "POST",
        body: JSON.stringify({
          name: name.trim() || "Novo flow",
          description: "",
          steps: [
            { name: "Origem", kind: "extract", subkind: "extract", step_order: 1, config: { dataset_id: sourceId || "" } },
            {
              name: "ClickHouse",
              kind: "load",
              subkind: "load",
              step_order: 2,
              config: { target: "clickhouse", table_name: "flow_output" },
            },
          ],
        }),
      }),
    onSuccess: (d) => {
      toast.success("Flow criado");
      onClose();
      router.push(`/flows/${d.id}`);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  if (!open) return null;

  const busy = create.isPending || createBlank.isPending;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
      <div className="max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-2xl border border-line bg-white p-5 shadow-2xl">
        <div className="flex items-start justify-between gap-3">
          <div>
            <h3 className="text-lg font-semibold text-ink">Novo flow</h3>
            <p className="mt-1 text-[13px] text-mute">Escolha um modelo. O canvas abre já com origem, transformação e destino ligados.</p>
          </div>
          <button type="button" onClick={onClose} className="rounded-lg p-1 text-mute hover:bg-surface-2" aria-label="Fechar">
            <X size={18} />
          </button>
        </div>

        <div className="mt-4 grid grid-cols-1 gap-2 sm:grid-cols-2">
          {FLOW_TEMPLATES.map((t) => {
            const Icon = TEMPLATE_ICON[t.id];
            const selected = templateId === t.id;
            return (
              <button
                key={t.id}
                type="button"
                onClick={() => {
                  setTemplateId(t.id);
                  setName(t.defaultName);
                }}
                className={cn(
                  "rounded-2xl border p-3 text-left transition",
                  selected ? "border-primary bg-primary/5 ring-1 ring-primary/30" : "border-line bg-white hover:border-primary/40",
                )}
              >
                <div className="flex items-center gap-2">
                  <span className="flex h-8 w-8 items-center justify-center rounded-xl bg-primary/10 text-primary">
                    <Icon size={16} />
                  </span>
                  <span className="text-sm font-medium text-ink">{t.title}</span>
                </div>
                <p className="mt-2 text-[12px] text-mute">{t.description}</p>
              </button>
            );
          })}
        </div>

        <div className="mt-4 space-y-3">
          <FieldLabel label="Nome" required>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder={template.defaultName} />
          </FieldLabel>
          {template.needsSource && (
            <FieldLabel
              label={template.preferConnector ? "Conjunto do conector" : template.id === "csv_clickhouse" ? "Conjunto de origem (CSV ou ficheiro)" : "Origem"}
              hint={datasetList.length === 0 ? "Ainda não há conjuntos — carregue um CSV em Dados." : undefined}
            >
              <Select value={sourceId} onChange={(e) => setSourceId(e.target.value)} disabled={!sourceOptions.length}>
                <option value="">{sourceOptions.length ? "Escolher conjunto…" : "Sem conjuntos disponíveis"}</option>
                {sourceOptions.map((d) => (
                  <option key={d.id} value={d.id}>
                    {d.name}
                    {d.source_name ? ` · ${d.source_name}` : ""}
                    {d.row_count ? ` · ${d.row_count} linhas` : ""}
                  </option>
                ))}
              </Select>
            </FieldLabel>
          )}
          {template.needsSecondSource && (
            <FieldLabel label="Segunda origem">
              <Select value={source2Id} onChange={(e) => setSource2Id(e.target.value)} disabled={datasetList.length < 2}>
                <option value="">{datasetList.length < 2 ? "Precisa de dois conjuntos" : "Escolher segundo conjunto…"}</option>
                {datasetList
                  .filter((d) => d.id !== sourceId)
                  .map((d) => (
                    <option key={d.id} value={d.id}>
                      {d.name}
                    </option>
                  ))}
              </Select>
            </FieldLabel>
          )}
          <div className="flex items-center gap-2 rounded-xl border border-line bg-bg px-3 py-2 text-[13px] text-mute">
            <span>Destino</span>
            <ArrowRight size={14} />
            <span className="font-medium text-ink">{template.destination}</span>
          </div>
        </div>

        {datasetList.length === 0 && (
          <p className="mt-3 text-[12px] text-mute">
            Sem dados ainda?{" "}
            <Link href="/data" className="text-accent hover:underline">
              Carregue um CSV
            </Link>{" "}
            ou{" "}
            <Link href="/connectors" className="text-accent hover:underline">
              ligue um conector
            </Link>
            .
          </p>
        )}

        <div className="mt-5 flex flex-wrap items-center justify-between gap-2">
          <Button variant="ghost" onClick={() => createBlank.mutate()} busy={createBlank.isPending} disabled={busy}>
            <Workflow size={14} /> Começar em branco
          </Button>
          <div className="flex gap-2">
            <Button variant="secondary" onClick={onClose} disabled={busy}>
              Cancelar
            </Button>
            <Button onClick={() => create.mutate()} busy={create.isPending} disabled={busy || (template.needsSource && !sourceId) || (template.needsSecondSource && !source2Id)}>
              Criar e abrir canvas
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

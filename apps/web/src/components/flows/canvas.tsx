"use client";

import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import Link from "next/link";
import { Database, Play, Plus, Settings, Trash2, Workflow } from "lucide-react";
import { api, normalizeArray } from "@/lib/api";
import { Button, Card, FieldLabel, Input, Select, Textarea, cn } from "@/components/ui";
import {
  STEP_SUBKINDS,
  nodeTone,
  parseFlowLayout,
  slugTable,
  type DatasetOption,
  type Flow,
  type FlowEdge,
  type FlowLayout,
  type FlowNode,
  type Step,
} from "@/lib/flows";

export function FlowCanvasEditor({ flow, initialSteps }: { flow: Flow; initialSteps?: Step[] }) {
  const qc = useQueryClient();
  const steps = useQuery({
    queryKey: ["flow-steps", flow.id],
    queryFn: () => api<any>(`/api/v1/flows/${flow.id}/steps`),
    initialData: initialSteps,
  });
  const datasets = useQuery({ queryKey: ["datasets"], queryFn: () => api<any>("/api/v1/datasets") });
  const stepList = normalizeArray<Step>(steps.data);
  const datasetList = normalizeArray<DatasetOption>(datasets.data);
  const [nodes, setNodes] = useState<FlowNode[]>([]);
  const [edges, setEdges] = useState<FlowEdge[]>([]);
  const [selectedNode, setSelectedNode] = useState<string | null>(null);
  const [dragging, setDragging] = useState<string | null>(null);
  const [connectFrom, setConnectFrom] = useState<string | null>(null);
  const canvasRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const layout = parseFlowLayout(flow.layout);
    const sorted = [...stepList].sort((a, b) => a.step_order - b.step_order);
    if (!sorted.length) {
      setNodes([]);
      setEdges([]);
      return;
    }
    setNodes((prev) => {
      const prevByStep = new Map(prev.map((n) => [n.stepId, n]));
      return sorted.map((s, i) => {
        const existing = prevByStep.get(s.id) || layout?.nodes.find((n) => n.stepId === s.id || n.id === s.id);
        return {
          id: existing?.id || s.id,
          stepId: s.id,
          kind: s.kind,
          subkind: s.subkind,
          name: s.name || s.subkind,
          x: existing?.x ?? 40 + i * 220,
          y: existing?.y ?? 100 + (i % 2) * 80,
          config: s.config || existing?.config || {},
        };
      });
    });
    setEdges((prev) => {
      if (prev.length) return prev;
      if (layout?.edges?.length) return layout.edges;
      const generated: FlowEdge[] = [];
      for (let i = 0; i < sorted.length - 1; i++) {
        generated.push({ from: sorted[i].id, to: sorted[i + 1].id });
      }
      return generated;
    });
    setSelectedNode((cur) => cur || sorted[0]?.id || null);
  }, [stepList, flow.layout]);

  const updateLayout = useMutation({
    mutationFn: (layout: FlowLayout) =>
      api(`/api/v1/flows/${flow.id}`, {
        method: "PUT",
        body: JSON.stringify({
          name: flow.name,
          description: flow.description,
          status: flow.status,
          source_dataset_id: flow.source_dataset_id || undefined,
          target_dataset_id: flow.target_dataset_id || undefined,
          output_dataset_id: flow.output_dataset_id || undefined,
          layout,
        }),
      }),
    onError: (e: Error) => toast.error(e.message),
  });

  const persistLayout = (nextNodes: FlowNode[], nextEdges: FlowEdge[]) => {
    updateLayout.mutate({ nodes: nextNodes, edges: nextEdges });
  };

  const run = useMutation({
    mutationFn: () => api<{ run_id: string }>(`/api/v1/flows/${flow.id}/runs`, { method: "POST" }),
    onSuccess: () => {
      toast.success("Execução iniciada — a materializar no ClickHouse");
      qc.invalidateQueries({ queryKey: ["flow-runs", flow.id] });
      qc.invalidateQueries({ queryKey: ["flows"] });
      qc.invalidateQueries({ queryKey: ["flow", flow.id] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const addStep = useMutation({
    mutationFn: (payload: Record<string, unknown>) => api<{ id: string }>(`/api/v1/flows/${flow.id}/steps`, { method: "POST", body: JSON.stringify(payload) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["flow-steps", flow.id] }),
    onError: (e: Error) => toast.error(e.message),
  });

  const updateStep = useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: Record<string, unknown> }) =>
      api(`/api/v1/flows/${flow.id}/steps/${id}`, { method: "PUT", body: JSON.stringify(payload) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["flow-steps", flow.id] }),
    onError: (e: Error) => toast.error(e.message),
  });

  const deleteStep = useMutation({
    mutationFn: (id: string) => api(`/api/v1/flows/${flow.id}/steps/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["flow-steps", flow.id] }),
    onError: (e: Error) => toast.error(e.message),
  });

  const defaultConfig = (subkind: string) => {
    if (subkind === "extract") {
      return { dataset_id: datasetList[0]?.id || flow.source_dataset_id || "" };
    }
    if (subkind === "load") {
      return { target: "clickhouse", table_name: slugTable(flow.name) };
    }
    if (subkind === "filter") {
      return { column: "", op: "eq", value: "" };
    }
    if (subkind === "join") {
      return { left_key: "id", right_key: "id" };
    }
    return {};
  };

  const addNode = (subkind: string, kind: string) => {
    const id = `node-${Date.now()}`;
    const x = kind === "extract" ? 36 : kind === "load" ? 520 : 278;
    const y = 40 + (nodes.filter((n) => n.kind === kind).length % 4) * 120;
    const name = STEP_SUBKINDS.find((s) => s.value === subkind)?.label || subkind;
    const config = defaultConfig(subkind);
    addStep.mutate({ name, kind, subkind, step_order: nodes.length + 1, config }, {
      onSuccess: (res) => {
        const newNode: FlowNode = { id, stepId: res.id, kind, subkind, name, x, y, config };
        const prev = nodes[nodes.length - 1];
        const nextNodes = [...nodes, newNode];
        const nextEdges = [...edges, ...(prev ? [{ from: prev.id, to: id }] : [])];
        setNodes(nextNodes);
        setEdges(nextEdges);
        setSelectedNode(id);
        persistLayout(nextNodes, nextEdges);
      },
    });
  };

  const seedStarter = () => {
    addNode("extract", "extract");
  };

  const handleMouseDown = (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    setSelectedNode(id);
    setDragging(id);
  };

  const handleMouseMove = (e: React.MouseEvent) => {
    if (!dragging || !canvasRef.current) return;
    const rect = canvasRef.current.getBoundingClientRect();
    const x = Math.max(8, e.clientX - rect.left - 80);
    const y = Math.max(8, e.clientY - rect.top - 24);
    setNodes((prev) => prev.map((n) => (n.id === dragging ? { ...n, x, y } : n)));
  };

  const handleMouseUp = () => {
    if (dragging) {
      setDragging(null);
      persistLayout(nodes, edges);
    }
  };

  const handleConnect = (id: string) => {
    if (!connectFrom) {
      setConnectFrom(id);
      return;
    }
    if (connectFrom === id) {
      setConnectFrom(null);
      return;
    }
    const nextEdges = [...edges, { from: connectFrom, to: id }];
    setEdges(nextEdges);
    setConnectFrom(null);
    persistLayout(nodes, nextEdges);
  };

  const selected = nodes.find((n) => n.id === selectedNode);
  const extractReady = nodes.some((n) => n.kind === "extract" && String(n.config?.dataset_id || flow.source_dataset_id || ""));
  const hasLoad = nodes.some((n) => n.kind === "load");
  const readyToRun = extractReady && (hasLoad || nodes.length > 0);

  return (
    <div className="grid grid-cols-1 gap-3 lg:grid-cols-4">
      <div className="space-y-3 lg:col-span-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="text-[13px] font-medium text-mute">Editor visual</div>
          <div className="flex gap-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setConnectFrom(connectFrom ? null : selectedNode)}
              disabled={!selectedNode}
            >
              {connectFrom ? "A ligar… clique no destino" : "Ligar nós"}
            </Button>
            <Button size="sm" onClick={() => run.mutate()} busy={run.isPending} disabled={!nodes.length}>
              <Play className="h-4 w-4" /> Executar e materializar
            </Button>
          </div>
        </div>
        {readyToRun && extractReady && (
          <p className="rounded-xl border border-emerald-200 bg-emerald-50 px-3 py-2 text-[12px] text-emerald-800">
            Pronto a correr: origem definida, destino ClickHouse. O primeiro run grava um conjunto materializado.
          </p>
        )}
        {!extractReady && nodes.length > 0 && (
          <p className="rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-[12px] text-amber-800">
            Escolha um conjunto na origem (nó à esquerda) para o primeiro run.
          </p>
        )}
        <div
          ref={canvasRef}
          className="relative h-[28rem] w-full overflow-hidden rounded-xl border border-line bg-slate-50"
          onMouseMove={handleMouseMove}
          onMouseUp={handleMouseUp}
          onMouseLeave={handleMouseUp}
          onClick={() => setConnectFrom(null)}
        >
          <svg className="pointer-events-none absolute inset-0 h-full w-full">
            {edges.map((e, i) => {
              const from = nodes.find((n) => n.id === e.from);
              const to = nodes.find((n) => n.id === e.to);
              if (!from || !to) return null;
              return (
                <line
                  key={i}
                  x1={from.x + 80}
                  y1={from.y + 30}
                  x2={to.x + 80}
                  y2={to.y + 30}
                  stroke="#6366f1"
                  strokeWidth={2}
                />
              );
            })}
          </svg>
          {nodes.map((n) => (
            <div
              key={n.id}
              className={cn(
                "absolute cursor-grab select-none rounded-xl border px-3 py-2 shadow-sm transition",
                nodeTone(n.kind),
                selectedNode === n.id ? "ring-1 ring-primary" : "",
                connectFrom === n.id ? "ring-2 ring-primary" : "",
              )}
              style={{ left: n.x, top: n.y, width: 160 }}
              onMouseDown={(e) => handleMouseDown(e, n.id)}
              onClick={(e) => {
                e.stopPropagation();
                if (connectFrom) handleConnect(n.id);
              }}
            >
              <div className="flex items-center justify-between">
                <span className="text-[11px] font-semibold uppercase text-primary-600">{n.subkind}</span>
                {connectFrom === n.id && <span className="text-[10px] text-accent">origem</span>}
              </div>
              <div className="truncate text-[13px] font-medium text-ink">{n.name}</div>
            </div>
          ))}
          {nodes.length === 0 && (
            <div className="absolute inset-0 flex items-center justify-center p-6">
              <div className="max-w-md rounded-2xl border border-line bg-white p-6 text-center shadow-sm">
                <div className="mx-auto mb-3 flex h-11 w-11 items-center justify-center rounded-2xl bg-primary/10 text-primary">
                  <Workflow size={20} />
                </div>
                <p className="text-sm font-medium text-ink">Canvas vazio</p>
                <p className="mt-1 text-[13px] text-mute">Adicione uma origem para começar, ou volte à lista e escolha um modelo (CSV → ClickHouse).</p>
                <div className="mt-4 flex flex-wrap justify-center gap-2">
                  <Button size="sm" onClick={seedStarter} busy={addStep.isPending}>
                    <Database size={14} /> Adicionar origem
                  </Button>
                  <Button size="sm" variant="secondary" onClick={() => addNode("load", "load")} busy={addStep.isPending}>
                    Adicionar ClickHouse
                  </Button>
                  <Link href="/flows?new=1">
                    <Button size="sm" variant="ghost">
                      Escolher modelo
                    </Button>
                  </Link>
                </div>
              </div>
            </div>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          {STEP_SUBKINDS.map((s) => (
            <Button key={s.value} variant="secondary" size="sm" onClick={() => addNode(s.value, s.kind)} busy={addStep.isPending}>
              <Plus className="h-3 w-3" /> {s.label}
            </Button>
          ))}
        </div>
      </div>
      <div className="space-y-3">
        <Card className="space-y-3">
          <div className="flex items-center gap-2 text-[13px] font-medium text-mute">
            <Settings className="h-4 w-4" /> Propriedades
          </div>
          {selected ? (
            <NodeProperties
              node={selected}
              datasets={datasetList}
              onChange={(patch) => {
                setNodes((prev) => prev.map((n) => (n.id === selected.id ? { ...n, ...patch } : n)));
              }}
              onSave={(node) => {
                updateStep.mutate({
                  id: node.stepId,
                  payload: { name: node.name, kind: node.kind, subkind: node.subkind, config: node.config, step_order: stepList.find((s) => s.id === node.stepId)?.step_order || 1 },
                });
                persistLayout(nodes, edges);
              }}
              onDelete={(node) => {
                deleteStep.mutate(node.stepId);
                const nextNodes = nodes.filter((n) => n.id !== node.id);
                const nextEdges = edges.filter((e) => e.from !== node.id && e.to !== node.id);
                setNodes(nextNodes);
                setEdges(nextEdges);
                setSelectedNode(nextNodes[0]?.id || null);
                persistLayout(nextNodes, nextEdges);
              }}
            />
          ) : (
            <p className="text-[12px] text-mute">Seleccione um nó para editar, ou adicione uma origem.</p>
          )}
        </Card>
        <RunList flowId={flow.id} outputDatasetId={flow.output_dataset_id} />
      </div>
    </div>
  );
}

function NodeProperties({
  node,
  datasets,
  onChange,
  onSave,
  onDelete,
}: {
  node: FlowNode;
  datasets: DatasetOption[];
  onChange: (p: Partial<FlowNode>) => void;
  onSave: (n: FlowNode) => void;
  onDelete: (n: FlowNode) => void;
}) {
  return (
    <div className="space-y-3">
      <FieldLabel label="Nome">
        <Input value={node.name} onChange={(e) => onChange({ name: e.target.value })} />
      </FieldLabel>
      <div className="text-[12px] text-mute">
        {node.kind} · {node.subkind}
      </div>
      <StepConfigFields subkind={node.subkind} config={node.config} datasets={datasets} onChange={(c) => onChange({ config: c })} />
      <div className="flex gap-2">
        <Button size="sm" className="flex-1" onClick={() => onSave(node)}>
          Guardar
        </Button>
        <Button variant="danger" size="sm" onClick={() => onDelete(node)}>
          <Trash2 className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}

function StepConfigFields({
  subkind,
  config,
  datasets,
  onChange,
}: {
  subkind: string;
  config: Record<string, unknown>;
  datasets: DatasetOption[];
  onChange: (c: Record<string, unknown>) => void;
}) {
  const update = (key: string, value: unknown) => onChange({ ...config, [key]: value });
  const str = (key: string) => String(config[key] ?? "");
  switch (subkind) {
    case "extract":
      return (
        <FieldLabel label="Conjunto de origem" hint="CSV, ficheiro ou dados já sincronizados de um conector.">
          <Select value={str("dataset_id")} onChange={(e) => update("dataset_id", e.target.value)}>
            <option value="">Escolher conjunto…</option>
            {datasets.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
                {d.source_name ? ` · ${d.source_name}` : ""}
              </option>
            ))}
          </Select>
        </FieldLabel>
      );
    case "load":
      return (
        <>
          <FieldLabel label="Destino">
            <Select value={str("target") || "clickhouse"} onChange={(e) => update("target", e.target.value)}>
              <option value="clickhouse">ClickHouse</option>
            </Select>
          </FieldLabel>
          <FieldLabel label="Nome da tabela" hint="Usado como referência; o run cria um conjunto materializado.">
            <Input value={str("table_name")} onChange={(e) => update("table_name", e.target.value)} placeholder="flow_output" />
          </FieldLabel>
        </>
      );
    case "sql":
      return (
        <FieldLabel label="SQL" hint="Opcional neste MVP — se vazio, as linhas passam sem alteração.">
          <Textarea value={str("query")} onChange={(e) => update("query", e.target.value)} placeholder="SELECT * FROM origem" />
        </FieldLabel>
      );
    case "join":
      return (
        <>
          <FieldLabel label="Chave esquerda">
            <Input value={str("left_key")} onChange={(e) => update("left_key", e.target.value)} placeholder="id" />
          </FieldLabel>
          <FieldLabel label="Chave direita">
            <Input value={str("right_key")} onChange={(e) => update("right_key", e.target.value)} placeholder="id" />
          </FieldLabel>
          <FieldLabel label="Segundo conjunto">
            <Select value={str("right_dataset_id")} onChange={(e) => update("right_dataset_id", e.target.value)}>
              <option value="">Escolher…</option>
              {datasets.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                </option>
              ))}
            </Select>
          </FieldLabel>
        </>
      );
    case "rename":
      return (
        <>
          <FieldLabel label="De">
            <Input value={str("from")} onChange={(e) => update("from", e.target.value)} placeholder="coluna antiga" />
          </FieldLabel>
          <FieldLabel label="Para">
            <Input value={str("to")} onChange={(e) => update("to", e.target.value)} placeholder="coluna nova" />
          </FieldLabel>
        </>
      );
    case "filter":
      return (
        <>
          <FieldLabel label="Coluna" hint="Deixe vazio para passar os dados sem filtrar no primeiro run.">
            <Input value={str("column")} onChange={(e) => update("column", e.target.value)} />
          </FieldLabel>
          <FieldLabel label="Operador">
            <Select value={str("op") || "eq"} onChange={(e) => update("op", e.target.value)}>
              <option value="eq">=</option>
              <option value="gt">&gt;</option>
              <option value="lt">&lt;</option>
              <option value="contains">contém</option>
            </Select>
          </FieldLabel>
          <FieldLabel label="Valor">
            <Input value={str("value")} onChange={(e) => update("value", e.target.value)} />
          </FieldLabel>
        </>
      );
    case "change_type":
      return (
        <>
          <FieldLabel label="Coluna">
            <Input value={str("column")} onChange={(e) => update("column", e.target.value)} />
          </FieldLabel>
          <FieldLabel label="Tipo">
            <Select value={str("type") || "int"} onChange={(e) => update("type", e.target.value)}>
              <option value="int">Inteiro</option>
              <option value="float">Decimal</option>
              <option value="string">Texto</option>
              <option value="bool">Booleano</option>
            </Select>
          </FieldLabel>
        </>
      );
    case "fill_null":
      return (
        <>
          <FieldLabel label="Coluna">
            <Input value={str("column")} onChange={(e) => update("column", e.target.value)} />
          </FieldLabel>
          <FieldLabel label="Valor">
            <Input value={str("value")} onChange={(e) => update("value", e.target.value)} />
          </FieldLabel>
        </>
      );
    case "dedup":
      return (
        <FieldLabel label="Colunas (separadas por vírgula)">
          <Input
            value={Array.isArray(config.columns) ? (config.columns as string[]).join(",") : str("columns")}
            onChange={(e) => update("columns", e.target.value.split(",").map((s) => s.trim()).filter(Boolean))}
          />
        </FieldLabel>
      );
    case "aggregate":
      return (
        <>
          <FieldLabel label="Agrupar por">
            <Input
              value={Array.isArray(config.group_by) ? (config.group_by as string[]).join(",") : str("group_by")}
              onChange={(e) => update("group_by", e.target.value.split(",").map((s) => s.trim()).filter(Boolean))}
            />
          </FieldLabel>
          <FieldLabel label="Coluna agregada">
            <Input value={str("agg_column")} onChange={(e) => update("agg_column", e.target.value)} />
          </FieldLabel>
          <FieldLabel label="Nome da métrica">
            <Input value={str("agg_name")} onChange={(e) => update("agg_name", e.target.value)} />
          </FieldLabel>
          <FieldLabel label="Função">
            <Select value={str("agg_fn") || "sum"} onChange={(e) => update("agg_fn", e.target.value)}>
              <option value="sum">Soma</option>
              <option value="avg">Média</option>
              <option value="min">Mín</option>
              <option value="max">Máx</option>
            </Select>
          </FieldLabel>
        </>
      );
    default:
      return (
        <FieldLabel label="Config (JSON)">
          <Textarea
            value={JSON.stringify(config)}
            onChange={(e) => {
              try {
                onChange(JSON.parse(e.target.value));
              } catch {}
            }}
          />
        </FieldLabel>
      );
  }
}

function RunList({ flowId, outputDatasetId }: { flowId: string; outputDatasetId?: string | null }) {
  const [runId, setRunId] = useState<string | null>(null);
  const runs = useQuery({ queryKey: ["flow-runs", flowId], queryFn: () => api<any>(`/api/v1/flows/${flowId}/runs`) });
  const logs = useQuery({ queryKey: ["flow-run-logs", runId], queryFn: () => api<any>(`/api/v1/flows/runs/${runId}/logs`), enabled: !!runId });
  const runList = normalizeArray(runs.data);
  const logList = normalizeArray(logs.data);

  return (
    <div className="space-y-2">
      <div className="text-[13px] font-medium text-mute">Execuções</div>
      {outputDatasetId && (
        <Link href={`/data/${outputDatasetId}`} className="block rounded-xl border border-emerald-200 bg-emerald-50 px-3 py-2 text-[12px] text-emerald-800 hover:underline">
          Ver conjunto materializado
        </Link>
      )}
      {runList.length === 0 && <p className="text-[12px] text-mute">Ainda sem execuções. Clique em Executar e materializar.</p>}
      {runList.map((r: any) => (
        <div key={r.id} className="rounded-xl border border-line bg-white px-3 py-2 text-sm">
          <button type="button" className="flex w-full items-center justify-between" onClick={() => setRunId(runId === r.id ? null : r.id)}>
            <span>
              {r.id.slice(0, 8)} · {r.status} · {r.rows_processed ?? "—"} linhas
            </span>
            <span className="text-[11px] text-mute">{new Date(r.created_at).toLocaleString("pt-BR")}</span>
          </button>
          {runId === r.id && (
            <div className="mt-2 space-y-1 border-t border-line pt-2">
              {logList.map((l: any) => (
                <div key={l.id} className={cn("text-[11px]", l.level === "error" ? "text-danger" : "text-mute")}>
                  [{l.level}] {l.message}
                </div>
              ))}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}

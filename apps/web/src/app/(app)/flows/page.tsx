"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import Link from "next/link";
import { toast } from "sonner";
import { Button, Card, CardTitle, EmptyState, ErrorState, FieldLabel, Input, PageHeader, PageSkeleton, Select, Textarea } from "@/components/ui";
import { Workflow, Play, Trash2, Plus, Settings } from "lucide-react";

type Flow = {
  id: string;
  name: string;
  description: string;
  status: string;
  updated_at: string;
  output_dataset_id?: string;
  layout?: FlowLayout;
};

type Step = { id: string; step_order: number; kind: string; subkind: string; name: string; config: any };

type FlowNode = {
  id: string;
  stepId: string;
  kind: string;
  subkind: string;
  name: string;
  x: number;
  y: number;
  config: any;
};

type FlowEdge = { from: string; to: string };

type FlowLayout = {
  nodes: FlowNode[];
  edges: FlowEdge[];
};

const STEP_SUBKINDS = [
  { value: "extract", label: "Extract", kind: "extract" },
  { value: "rename", label: "Renomear", kind: "transform" },
  { value: "filter", label: "Filtrar", kind: "transform" },
  { value: "change_type", label: "Mudar tipo", kind: "transform" },
  { value: "fill_null", label: "Preencher nulos", kind: "transform" },
  { value: "dedup", label: "Remover duplicados", kind: "transform" },
  { value: "conditional", label: "Condicional", kind: "transform" },
  { value: "aggregate", label: "Agregar", kind: "transform" },
  { value: "validate", label: "Validar", kind: "validate" },
  { value: "load", label: "Load", kind: "load" },
];

export default function FlowsPage() {
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [desc, setDesc] = useState("");
  const [selected, setSelected] = useState<string | null>(null);
  const flows = useQuery({ queryKey: ["flows"], queryFn: () => api<any>("/api/v1/flows") });
  const flowList = normalizeArray<Flow>(flows.data);

  const create = useMutation({
    mutationFn: () => api<{ id: string }>("/api/v1/flows", { method: "POST", body: JSON.stringify({ name, description: desc }) }),
    onSuccess: () => {
      toast.success("Flow criado");
      setName("");
      setDesc("");
      qc.invalidateQueries({ queryKey: ["flows"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  if (flows.isLoading) return <PageSkeleton />;
  if (flows.isError) return <ErrorState message={(flows.error as Error).message} onRetry={() => flows.refetch()} />;

  return (
    <div className="mx-auto max-w-6xl space-y-5">
      <PageHeader
        title="Flows"
        description="Pipelines de transformação (ETL/ELT) visual — editor de nós."
        actions={
          <Button onClick={() => create.mutate()} busy={create.isPending} disabled={!name.trim()}>
            Criar flow
          </Button>
        }
      />
      <Card className="space-y-3">
        <CardTitle>Novo flow</CardTitle>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <FieldLabel label="Nome">
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="ex. Limpeza vendas" />
          </FieldLabel>
          <FieldLabel label="Descrição">
            <Textarea value={desc} onChange={(e) => setDesc(e.target.value)} placeholder="O que este flow faz…" />
          </FieldLabel>
        </div>
      </Card>
      {flowList.length === 0 && (
        <EmptyState icon={Workflow} title="Ainda sem flows" description="Crie o primeiro pipeline de transformação acima." />
      )}
      <div className="space-y-2">
        {flowList.map((f) => (
          <div
            key={f.id}
            className={`cursor-pointer rounded-2xl border border-line bg-surface p-4 shadow-sm transition hover:border-primary/40 ${selected === f.id ? "ring-1 ring-primary/40" : ""}`}
            onClick={() => setSelected(selected === f.id ? null : f.id)}
          >
            <div className="flex items-center justify-between">
              <div>
                <div className="font-medium text-ink">
                  <Link href={`/flows/${f.id}`} className="text-accent hover:underline" onClick={(e) => e.stopPropagation()}>
                    {f.name}
                  </Link>
                </div>
                <div className="text-[12px] text-mute">
                  {f.description || "Sem descrição"}
                  {f.output_dataset_id && (
                    <span className="ml-2">
                      · Output: <Link href={`/data/${f.output_dataset_id}`} className="text-accent hover:underline" onClick={(e) => e.stopPropagation()}>dataset</Link>
                    </span>
                  )}
                </div>
              </div>
              <span className={`rounded-full px-2 py-0.5 text-[11px] ${f.output_dataset_id ? "bg-emerald-50 text-emerald-700" : "bg-primary/10 text-primary-600"}`}>
                {f.output_dataset_id ? "Materializado" : f.status}
              </span>
            </div>
            {selected === f.id && <FlowCanvasEditor flow={f} />}
          </div>
        ))}
      </div>
    </div>
  );
}

function FlowCanvasEditor({ flow }: { flow: Flow }) {
  const qc = useQueryClient();
  const steps = useQuery({ queryKey: ["flow-steps", flow.id], queryFn: () => api<any>(`/api/v1/flows/${flow.id}/steps`) });
  const stepList = normalizeArray<Step>(steps.data);
  const [nodes, setNodes] = useState<FlowNode[]>([]);
  const [edges, setEdges] = useState<FlowEdge[]>([]);
  const [selectedNode, setSelectedNode] = useState<string | null>(null);
  const [dragging, setDragging] = useState<string | null>(null);
  const [connectFrom, setConnectFrom] = useState<string | null>(null);
  const canvasRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!stepList.length) return;
    const layout = flow.layout?.nodes ? flow.layout : undefined;
    const sorted = [...stepList].sort((a, b) => a.step_order - b.step_order);
    const nextNodes: FlowNode[] = sorted.map((s, i) => {
      const existing = layout?.nodes.find((n) => n.stepId === s.id);
      return {
        id: existing?.id || s.id,
        stepId: s.id,
        kind: s.kind,
        subkind: s.subkind,
        name: s.name || s.subkind,
        x: existing?.x ?? 40 + i * 220,
        y: existing?.y ?? 100 + (i % 2) * 80,
        config: s.config || {},
      };
    });
    const nextEdges: FlowEdge[] = layout?.edges || [];
    if (!layout?.edges) {
      for (let i = 0; i < sorted.length - 1; i++) {
        nextEdges.push({ from: nextNodes[i].id, to: nextNodes[i + 1].id });
      }
    }
    setNodes(nextNodes);
    setEdges(nextEdges);
  }, [stepList, flow.layout]);

  const updateLayout = useMutation({
    mutationFn: (layout: FlowLayout) =>
      api(`/api/v1/flows/${flow.id}`, {
        method: "PUT",
        body: JSON.stringify({
          name: flow.name,
          description: flow.description,
          status: flow.status,
          layout,
        }),
      }),
    onError: (e: Error) => toast.error(e.message),
  });

  const run = useMutation({
    mutationFn: () => api<{ run_id: string }>(`/api/v1/flows/${flow.id}/runs`, { method: "POST" }),
    onSuccess: (d) => toast.success(`Execução iniciada: ${d.run_id}`),
    onError: (e: Error) => toast.error(e.message),
  });

  const addStep = useMutation({
    mutationFn: (payload: any) => api(`/api/v1/flows/${flow.id}/steps`, { method: "POST", body: JSON.stringify(payload) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["flow-steps", flow.id] }),
    onError: (e: Error) => toast.error(e.message),
  });

  const updateStep = useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: any }) => api(`/api/v1/flows/${flow.id}/steps/${id}`, { method: "PUT", body: JSON.stringify(payload) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["flow-steps", flow.id] }),
    onError: (e: Error) => toast.error(e.message),
  });

  const deleteStep = useMutation({
    mutationFn: (id: string) => api(`/api/v1/flows/${flow.id}/steps/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["flow-steps", flow.id] }),
    onError: (e: Error) => toast.error(e.message),
  });

  const addNode = (subkind: string, kind: string) => {
    const id = `node-${Date.now()}`;
    const x = 40 + (nodes.length * 220) % 800;
    const y = 100 + (nodes.length % 2) * 80;
    const name = STEP_SUBKINDS.find((s) => s.value === subkind)?.label || subkind;
    addStep.mutate({ name, kind, subkind, step_order: nodes.length + 1, config: {} }, {
      onSuccess: (res: any) => {
        const stepId = res.id;
        const newNode: FlowNode = { id, stepId, kind, subkind, name, x, y, config: {} };
        const nextNodes = [...nodes, newNode];
        const nextEdges = [...edges, ...(nodes.length > 0 ? [{ from: nodes[nodes.length - 1].id, to: id }] : [])];
        setNodes(nextNodes);
        setEdges(nextEdges);
        updateLayout.mutate({ nodes: nextNodes, edges: nextEdges });
      },
    });
  };

  const handleMouseDown = (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    setSelectedNode(id);
    setDragging(id);
  };

  const handleMouseMove = (e: React.MouseEvent) => {
    if (!dragging || !canvasRef.current) return;
    const rect = canvasRef.current.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    setNodes((prev) => prev.map((n) => (n.id === dragging ? { ...n, x, y } : n)));
  };

  const handleMouseUp = () => {
    if (dragging) {
      setDragging(null);
      updateLayout.mutate({ nodes, edges });
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
    updateLayout.mutate({ nodes, edges: nextEdges });
  };

  const selected = nodes.find((n) => n.id === selectedNode);

  return (
    <div className="mt-4 grid grid-cols-1 gap-3 lg:grid-cols-4">
      <div className="lg:col-span-3 space-y-3">
        <div className="flex items-center justify-between">
          <div className="text-[13px] font-medium text-mute">Editor visual</div>
          <div className="flex gap-2">
            <Button variant="secondary" size="sm" onClick={() => run.mutate()} busy={run.isPending}>
              <Play className="mr-1 h-4 w-4" /> Executar
            </Button>
          </div>
        </div>
        <div
          ref={canvasRef}
          className="relative h-96 w-full overflow-hidden rounded-xl border border-line bg-slate-50"
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
                  stroke="#94a3b8"
                  strokeWidth={2}
                />
              );
            })}
          </svg>
          {nodes.map((n) => (
            <div
              key={n.id}
              className={`absolute cursor-grab select-none rounded-xl border bg-white px-3 py-2 shadow-sm transition ${selectedNode === n.id ? "border-accent ring-1 ring-accent" : "border-line"} ${connectFrom === n.id ? "ring-2 ring-primary" : ""}`}
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
        </div>
        <div className="flex flex-wrap gap-2">
          {STEP_SUBKINDS.map((s) => (
            <Button key={s.value} variant="secondary" size="sm" onClick={() => addNode(s.value, s.kind)} busy={addStep.isPending}>
              <Plus className="mr-1 h-3 w-3" /> {s.label}
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
              onChange={(patch) => {
                setNodes((prev) => prev.map((n) => (n.id === selected.id ? { ...n, ...patch } : n)));
              }}
              onSave={(node) => {
                updateStep.mutate({
                  id: node.stepId,
                  payload: { name: node.name, kind: node.kind, subkind: node.subkind, config: node.config },
                });
                updateLayout.mutate({ nodes, edges });
              }}
              onDelete={(node) => {
                deleteStep.mutate(node.stepId);
                setNodes((prev) => prev.filter((n) => n.id !== node.id));
                setEdges((prev) => prev.filter((e) => e.from !== node.id && e.to !== node.id));
              }}
            />
          ) : (
            <p className="text-[12px] text-mute">Seleciona um nó para editar.</p>
          )}
        </Card>
        <RunList flowId={flow.id} />
      </div>
    </div>
  );
}

function NodeProperties({
  node,
  onChange,
  onSave,
  onDelete,
}: {
  node: FlowNode;
  onChange: (p: Partial<FlowNode>) => void;
  onSave: (n: FlowNode) => void;
  onDelete: (n: FlowNode) => void;
}) {
  const updateConfig = (key: string, value: any) => onChange({ config: { ...node.config, [key]: value } });
  return (
    <div className="space-y-3">
      <FieldLabel label="Nome">
        <Input value={node.name} onChange={(e) => onChange({ name: e.target.value })} />
      </FieldLabel>
      <div className="text-[12px] text-mute">Kind: {node.kind}</div>
      <div className="text-[12px] text-mute">Subkind: {node.subkind}</div>
      <StepConfigFields subkind={node.subkind} config={node.config} onChange={(c) => onChange({ config: c })} />
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

function StepConfigFields({ subkind, config, onChange }: { subkind: string; config: any; onChange: (c: any) => void }) {
  const update = (key: string, value: any) => onChange({ ...config, [key]: value });
  switch (subkind) {
    case "rename":
      return (
        <>
          <FieldLabel label="De">
            <Input value={config.from || ""} onChange={(e) => update("from", e.target.value)} placeholder="coluna antiga" />
          </FieldLabel>
          <FieldLabel label="Para">
            <Input value={config.to || ""} onChange={(e) => update("to", e.target.value)} placeholder="coluna nova" />
          </FieldLabel>
        </>
      );
    case "filter":
      return (
        <>
          <FieldLabel label="Coluna">
            <Input value={config.column || ""} onChange={(e) => update("column", e.target.value)} />
          </FieldLabel>
          <FieldLabel label="Operador">
            <Select value={config.op || "eq"} onChange={(e) => update("op", e.target.value)}>
              <option value="eq">=</option>
              <option value="gt">&gt;</option>
              <option value="lt">&lt;</option>
              <option value="contains">contém</option>
            </Select>
          </FieldLabel>
          <FieldLabel label="Valor">
            <Input value={config.value || ""} onChange={(e) => update("value", e.target.value)} />
          </FieldLabel>
        </>
      );
    case "change_type":
      return (
        <>
          <FieldLabel label="Coluna">
            <Input value={config.column || ""} onChange={(e) => update("column", e.target.value)} />
          </FieldLabel>
          <FieldLabel label="Tipo">
            <Select value={config.type || "int"} onChange={(e) => update("type", e.target.value)}>
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
            <Input value={config.column || ""} onChange={(e) => update("column", e.target.value)} />
          </FieldLabel>
          <FieldLabel label="Valor">
            <Input value={config.value || ""} onChange={(e) => update("value", e.target.value)} />
          </FieldLabel>
        </>
      );
    case "dedup":
      return (
        <FieldLabel label="Colunas (separadas por vírgula)">
          <Input value={(config.columns || []).join(",")} onChange={(e) => update("columns", e.target.value.split(",").map((s) => s.trim()))} />
        </FieldLabel>
      );
    case "aggregate":
      return (
        <>
          <FieldLabel label="Agrupar por">
            <Input value={(config.group_by || []).join(",")} onChange={(e) => update("group_by", e.target.value.split(",").map((s) => s.trim()))} />
          </FieldLabel>
          <FieldLabel label="Coluna agregada">
            <Input value={config.agg_column || ""} onChange={(e) => update("agg_column", e.target.value)} />
          </FieldLabel>
          <FieldLabel label="Nome da métrica">
            <Input value={config.agg_name || ""} onChange={(e) => update("agg_name", e.target.value)} />
          </FieldLabel>
          <FieldLabel label="Função">
            <Select value={config.agg_fn || "sum"} onChange={(e) => update("agg_fn", e.target.value)}>
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

function RunList({ flowId }: { flowId: string }) {
  const [runId, setRunId] = useState<string | null>(null);
  const runs = useQuery({ queryKey: ["flow-runs", flowId], queryFn: () => api<any>(`/api/v1/flows/${flowId}/runs`) });
  const logs = useQuery({ queryKey: ["flow-run-logs", runId], queryFn: () => api<any>(`/api/v1/flows/runs/${runId}/logs`), enabled: !!runId });
  const runList = normalizeArray(runs.data);
  const logList = normalizeArray(logs.data);

  return (
    <div className="space-y-2">
      <div className="text-[13px] font-medium text-mute">Execuções</div>
      {runList.length === 0 && <p className="text-[12px] text-mute">Sem execuções.</p>}
      {runList.map((r: any) => (
        <div key={r.id} className="rounded-xl border border-line bg-white px-3 py-2 text-sm">
          <button className="flex w-full items-center justify-between" onClick={() => setRunId(runId === r.id ? null : r.id)}>
            <span>
              {r.id.slice(0, 8)} · {r.status} · {r.rows_processed ?? "—"} linhas
            </span>
            <span className="text-[11px] text-mute">{new Date(r.created_at).toLocaleString("pt-BR")}</span>
          </button>
          {runId === r.id && (
            <div className="mt-2 space-y-1 border-t border-line pt-2">
              {logList.map((l: any) => (
                <div key={l.id} className={`text-[11px] ${l.level === "error" ? "text-danger" : "text-mute"}`}>
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

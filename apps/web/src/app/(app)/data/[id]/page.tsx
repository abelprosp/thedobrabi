"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { useParams, useRouter } from "next/navigation";
import { Chart } from "@/components/viz";
import { toast } from "sonner";
import { Trash2 } from "lucide-react";
import { Button, Card, CardTitle, ErrorState, FieldLabel, Input, PageHeader, PageSkeleton, Select, Table, Td, Textarea, Th, cellValue, isNumericValue } from "@/components/ui";
import { AutoRefreshCard } from "@/components/auto-refresh-card";

export default function DatasetPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const qc = useQueryClient();
  const [tab, setTab] = useState<"schema" | "quality" | "model" | "relationships" | "measures" | "security">("schema");
  const ds = useQuery({
    queryKey: ["dataset", id],
    queryFn: () => api<any>(`/api/v1/datasets/${id}`),
  });
  const preview = useQuery({
    queryKey: ["preview", id],
    queryFn: () => api<any>(`/api/v1/datasets/${id}/preview`),
  });
  const remove = useMutation({
    mutationFn: () => api(`/api/v1/datasets/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success("Conjunto excluído");
      qc.invalidateQueries({ queryKey: ["datasets"] });
      router.replace("/data");
    },
    onError: (e: Error) => toast.error(e.message),
  });

  if (ds.isLoading) return <PageSkeleton />;
  if (ds.isError) return <ErrorState message={(ds.error as Error).message} onRetry={() => ds.refetch()} />;
  if (!ds.data) return <ErrorState message="Conjunto não encontrado" />;

  const schema = ds.data.schema || [];
  const quality = ds.data.quality || {};
  const model = ds.data.semantic_model || {};

  const tabs = [
    { key: "schema", label: "Esquema" },
    { key: "quality", label: "Qualidade" },
    { key: "model", label: "Modelo Semântico" },
    { key: "relationships", label: "Relacionamentos" },
    { key: "measures", label: "Medidas" },
    { key: "security", label: "Segurança" },
  ] as const;

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <PageHeader
        title={ds.data.name}
        description={`${ds.data.row_count?.toLocaleString("pt-BR")} linhas · qualidade ${ds.data.quality_score ?? "—"}/100 · ${ds.data.clickhouse_table} · ${ds.data.storage_mode || "import"}`}
        crumbs={[{ href: "/data", label: "Dados" }]}
        actions={
          <Button
            variant="danger"
            onClick={() => {
              if (confirm(`Excluir o conjunto «${ds.data.name}»? Esta acção não se desfaz.`)) remove.mutate();
            }}
            busy={remove.isPending}
          >
            <Trash2 size={14} /> Excluir conjunto
          </Button>
        }
      />
      {ds.data.source_id && (
        <AutoRefreshCard kind="dataset" targetId={id} targetType={ds.data.source_type} />
      )}
      <div className="border-b border-line">
        <div className="flex gap-1 overflow-x-auto pb-1">
          {tabs.map((t) => (
            <button
              key={t.key}
              onClick={() => setTab(t.key as any)}
              className={`rounded-t-lg px-3 py-2 text-[13px] font-medium transition ${
                tab === t.key ? "border-b-2 border-primary text-primary-600" : "text-mute hover:text-ink"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>
      {tab === "schema" && <SchemaTab schema={schema} preview={preview} />}
      {tab === "quality" && <QualityTab quality={quality} />}
      {tab === "model" && <ModelTab datasetId={id} model={model} schema={schema} />}
      {tab === "relationships" && <RelationshipsTab datasetId={id} model={model} />}
      {tab === "measures" && <MeasuresTab datasetId={id} model={model} />}
      {tab === "security" && <SecurityTab datasetId={id} />}
    </div>
  );
}

function SchemaTab({ schema, preview }: { schema: any[]; preview: any }) {
  return (
    <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
      <Card>
        <CardTitle>Colunas</CardTitle>
        {schema.map((c: any) => (
          <div key={c.name} className="flex justify-between border-t border-line py-2 text-sm first:border-0">
            <span>{c.name}</span>
            <span className="text-mute">
              {c.type} · {c.role}
            </span>
          </div>
        ))}
      </Card>
      <Card>
        <CardTitle>Pré-visualização</CardTitle>
        {preview.data?.rows ? (
          <div className="overflow-x-auto">
            <Table className="text-[12px]">
              <thead>
                <tr>
                  {(preview.data.columns || []).slice(0, 8).map((c: string) => (
                    <Th key={c} numeric={preview.data.rows?.[0] && isNumericValue(preview.data.rows[0][c])}>
                      {c}
                    </Th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {(preview.data.rows || []).slice(0, 12).map((r: any, i: number) => (
                  <tr key={i} className="border-t border-line hover:bg-bg">
                    {(preview.data.columns || []).slice(0, 8).map((c: string) => (
                      <Td key={c} numeric={isNumericValue(r[c])}>
                        {cellValue(r[c])}
                      </Td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </Table>
          </div>
        ) : (
          <p className="text-[13px] text-mute">A carregar pré-visualização…</p>
        )}
      </Card>
      {preview.data?.rows?.length > 0 && (
        <div className="rounded-2xl border border-line bg-surface p-4 shadow-sm md:col-span-2">
          <Chart type="bar" rows={preview.data.rows.slice(0, 12)} columns={preview.data.columns} />
        </div>
      )}
    </div>
  );
}

function QualityTab({ quality }: { quality: any }) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
      <Stat label="Nulos" value={`${quality.null_pct ?? 0}%`} />
      <Stat label="Duplicados" value={`${quality.duplicate_pct ?? 0}%`} />
      <Stat label="Inválidos" value={`${quality.invalid_pct ?? 0}%`} />
    </div>
  );
}

function ModelTab({ datasetId, model, schema }: { datasetId: string; model: any; schema: any[] }) {
  const qc = useQueryClient();
  const [timeCol, setTimeCol] = useState(model.time_column || "");
  const [dims, setDims] = useState<any[]>(model.dimensions || []);
  const [measures, setMeasures] = useState<any[]>(model.measures || []);
  const save = useMutation({
    mutationFn: () =>
      api(`/api/v1/semantic-models/${model.id || datasetId}`, {
        method: "PUT",
        body: JSON.stringify({
          ...model,
          dataset_id: datasetId,
          time_column: timeCol,
          dimensions: dims,
          measures,
        }),
      }),
    onSuccess: () => {
      toast.success("Modelo guardado");
      qc.invalidateQueries({ queryKey: ["dataset", datasetId] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  return (
    <div className="space-y-4">
      <Card className="space-y-3">
        <CardTitle>Coluna de tempo</CardTitle>
        <Select value={timeCol} onChange={(e) => setTimeCol(e.target.value)}>
          <option value="">Nenhuma</option>
          {schema.map((c: any) => (
            <option key={c.name} value={c.name}>
              {c.name} ({c.type})
            </option>
          ))}
        </Select>
      </Card>
      <Card className="space-y-3">
        <CardTitle>Dimensões</CardTitle>
        {dims.map((d: any, i: number) => (
          <div key={i} className="grid grid-cols-3 gap-2">
            <Input value={d.name} placeholder="Nome" onChange={(e) => { const copy = [...dims]; copy[i].name = e.target.value; setDims(copy); }} />
            <Select value={d.column} onChange={(e) => { const copy = [...dims]; copy[i].column = e.target.value; setDims(copy); }}>
              {schema.map((c: any) => <option key={c.name} value={c.name}>{c.name}</option>)}
            </Select>
            <Input value={d.type || ""} placeholder="tipo" onChange={(e) => { const copy = [...dims]; copy[i].type = e.target.value; setDims(copy); }} />
          </div>
        ))}
        <Button variant="secondary" size="sm" onClick={() => setDims([...dims, { name: "", column: "", type: "string" }])}>
          + Dimensão
        </Button>
      </Card>
      <Card className="space-y-3">
        <CardTitle>Medidas</CardTitle>
        {measures.map((m: any, i: number) => (
          <div key={i} className="grid grid-cols-2 gap-2">
            <Input value={m.name} placeholder="Nome" onChange={(e) => { const copy = [...measures]; copy[i].name = e.target.value; setMeasures(copy); }} />
            <Select value={m.column} onChange={(e) => { const copy = [...measures]; copy[i].column = e.target.value; setMeasures(copy); }}>
              {schema.map((c: any) => <option key={c.name} value={c.name}>{c.name}</option>)}
              <option value="*">COUNT(*)</option>
            </Select>
          </div>
        ))}
        <Button variant="secondary" size="sm" onClick={() => setMeasures([...measures, { name: "", column: "", aggregation: "sum", expression: "" }])}>
          + Medida
        </Button>
      </Card>
      <Button onClick={() => save.mutate()} busy={save.isPending}>
        Guardar modelo
      </Button>
    </div>
  );
}

function RelationshipsTab({ datasetId, model }: { datasetId: string; model: any }) {
  const qc = useQueryClient();
  const rels = useQuery({ queryKey: ["relationships", model.id], queryFn: () => api<any>(`/api/v1/semantic-models/${model.id || datasetId}/relationships`), enabled: !!model.id });
  const relsList = normalizeArray(rels.data);
  const [toDS, setToDS] = useState("");
  const [fromCol, setFromCol] = useState("");
  const [toCol, setToCol] = useState("");
  const [type, setType] = useState("many_to_one");
  const create = useMutation({
    mutationFn: () => api(`/api/v1/semantic-models/${model.id || datasetId}/relationships`, { method: "POST", body: JSON.stringify({ to_dataset_id: toDS, from_column: fromCol, to_column: toCol, type }) }),
    onSuccess: () => { toast.success("Relacionamento adicionado"); qc.invalidateQueries({ queryKey: ["relationships", model.id] }); },
    onError: (e: Error) => toast.error(e.message),
  });

  return (
    <div className="space-y-4">
      <Card className="space-y-3">
        <CardTitle>Novo relacionamento</CardTitle>
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-4">
          <Input value={fromCol} onChange={(e) => setFromCol(e.target.value)} placeholder="Coluna origem" />
          <Input value={toDS} onChange={(e) => setToDS(e.target.value)} placeholder="Dataset destino" />
          <Input value={toCol} onChange={(e) => setToCol(e.target.value)} placeholder="Coluna destino" />
          <Select value={type} onChange={(e) => setType(e.target.value)}>
            <option value="many_to_one">muitos-para-um</option>
            <option value="one_to_many">um-para-muitos</option>
            <option value="one_to_one">um-para-um</option>
          </Select>
        </div>
        <Button onClick={() => create.mutate()} busy={create.isPending} disabled={!toDS || !fromCol || !toCol}>
          Adicionar
        </Button>
      </Card>
      <Card>
        <CardTitle>Relacionamentos</CardTitle>
        {relsList.length === 0 && <p className="text-[13px] text-mute">Sem relacionamentos.</p>}
        {relsList.map((r: any) => (
          <div key={r.id} className="border-t border-line py-2 text-sm first:border-0">
            {r.from_column} → {r.to_dataset_id}.{r.to_column} ({r.type})
          </div>
        ))}
      </Card>
    </div>
  );
}

function MeasuresTab({ datasetId, model }: { datasetId: string; model: any }) {
  const qc = useQueryClient();
  const [expr, setExpr] = useState("");
  const [result, setResult] = useState<any>(null);
  const validate = useMutation({
    mutationFn: () => api<any>(`/api/v1/semantic-models/${model.id || datasetId}/validate-measure`, { method: "POST", body: JSON.stringify({ expression: expr }) }),
    onSuccess: (d) => { setResult(d); toast.success(d.valid ? "Válida" : "Inválida"); },
    onError: (e: Error) => toast.error(e.message),
  });

  return (
    <div className="space-y-4">
      <Card className="space-y-3">
        <CardTitle>Validador DAX-like</CardTitle>
        <Textarea value={expr} onChange={(e) => setExpr(e.target.value)} placeholder="SUM(revenue) / COUNT(*)" />
        <Button onClick={() => validate.mutate()} busy={validate.isPending} disabled={!expr.trim()}>
          Validar
        </Button>
        {result && (
          <pre className="rounded-xl bg-bg p-3 text-[12px] text-mute">
            {JSON.stringify(result, null, 2)}
          </pre>
        )}
      </Card>
      <Card>
        <CardTitle>Medidas existentes</CardTitle>
        {(model.measures || []).map((m: any) => (
          <div key={m.name} className="border-t border-line py-2 text-sm first:border-0">
            <div className="font-medium">{m.name}</div>
            <div className="font-mono text-[11px] text-accent">{m.expression || `${m.aggregation}(${m.column})`}</div>
          </div>
        ))}
      </Card>
    </div>
  );
}

function SecurityTab({ datasetId }: { datasetId: string }) {
  const qc = useQueryClient();
  const rules = useQuery({ queryKey: ["rls", datasetId], queryFn: () => api<any>(`/api/v1/datasets/${datasetId}/rls`) });
  const rulesList = normalizeArray(rules.data);
  const [role, setRole] = useState("viewer");
  const [column, setColumn] = useState("");
  const [expr, setExpr] = useState("");
  const create = useMutation({
    mutationFn: () => api(`/api/v1/datasets/${datasetId}/rls`, { method: "POST", body: JSON.stringify({ role, column, expression: expr }) }),
    onSuccess: () => { toast.success("Regra adicionada"); qc.invalidateQueries({ queryKey: ["rls", datasetId] }); setColumn(""); setExpr(""); },
    onError: (e: Error) => toast.error(e.message),
  });

  return (
    <div className="space-y-4">
      <Card className="space-y-3">
        <CardTitle>Row-Level Security (RLS)</CardTitle>
        <p className="text-[13px] text-mute">Exemplo: user_id = current_user_id() ou tenant_id = current_org_id()</p>
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
          <Select value={role} onChange={(e) => setRole(e.target.value)}>
            <option value="viewer">viewer</option>
            <option value="analyst">analyst</option>
            <option value="admin">admin</option>
            <option value="owner">owner</option>
          </Select>
          <Input value={column} onChange={(e) => setColumn(e.target.value)} placeholder="Coluna" />
          <Input value={expr} onChange={(e) => setExpr(e.target.value)} placeholder="Expressão" />
        </div>
        <Button onClick={() => create.mutate()} busy={create.isPending} disabled={!column || !expr}>
          Adicionar regra
        </Button>
      </Card>
      <Card>
        <CardTitle>Regras</CardTitle>
        {rulesList.length === 0 && <p className="text-[13px] text-mute">Sem regras.</p>}
        {rulesList.map((r: any) => (
          <div key={r.id} className="border-t border-line py-2 text-sm first:border-0">
            <span className="font-medium">{r.role}</span>: <code className="text-accent">{r.column_name}</code> {r.expression}
          </div>
        ))}
      </Card>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <Card className="p-4">
      <div className="text-[11px] uppercase text-mute">{label}</div>
      <div className="mt-1 text-xl text-ink">{value}</div>
    </Card>
  );
}

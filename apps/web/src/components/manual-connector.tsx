"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Pencil, Plus, Save, Trash2 } from "lucide-react";
import { api } from "@/lib/api";
import {
  DEFAULT_MANUAL_COLUMNS,
  MANUAL_COL_TYPES,
  cellDisplay,
  emptyManualColumn,
  type ManualColumn,
  type ManualRow,
  type ManualTable,
} from "@/lib/manual";
import { Badge, Button, Card, CardTitle, EmptyState, FieldLabel, Input, Select, Table, TableWrap, Td, Th } from "@/components/ui";

export function ManualSchemaEditor({
  columns,
  onChange,
}: {
  columns: ManualColumn[];
  onChange: (next: ManualColumn[]) => void;
}) {
  function patch(i: number, part: Partial<ManualColumn>) {
    onChange(columns.map((c, idx) => (idx === i ? { ...c, ...part } : c)));
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-2">
        <p className="text-[13px] text-mute">Cada coluna vira um campo do formulário e uma coluna da planilha.</p>
        <Button
          size="sm"
          variant="secondary"
          onClick={() => onChange([...columns, emptyManualColumn()])}
        >
          <Plus size={14} /> Coluna
        </Button>
      </div>
      <div className="space-y-2">
        {columns.map((c, i) => (
          <div key={`${c.key || "new"}-${i}`} className="rounded-xl border border-line bg-bg p-3">
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-12">
              <div className="sm:col-span-4">
                <FieldLabel label="Nome" required>
                  <Input value={c.label} placeholder="Ex.: Cliente" onChange={(e) => patch(i, { label: e.target.value })} />
                </FieldLabel>
              </div>
              <div className="sm:col-span-3">
                <FieldLabel label="Tipo">
                  <Select value={c.type} onChange={(e) => patch(i, { type: e.target.value })}>
                    {MANUAL_COL_TYPES.map((t) => (
                      <option key={t.value} value={t.value}>
                        {t.label}
                      </option>
                    ))}
                  </Select>
                </FieldLabel>
              </div>
              <div className="flex items-end gap-2 sm:col-span-5">
                <label className="mb-2 flex items-center gap-2 text-[13px] text-ink">
                  <input type="checkbox" checked={Boolean(c.required)} onChange={(e) => patch(i, { required: e.target.checked })} />
                  Obrigatório
                </label>
                <Button
                  size="sm"
                  variant="ghost"
                  className="ml-auto text-danger"
                  onClick={() => onChange(columns.filter((_, idx) => idx !== i))}
                >
                  <Trash2 size={14} />
                </Button>
              </div>
            </div>
            {c.type === "select" && (
              <div className="mt-2">
                <FieldLabel label="Opções" hint="Separadas por vírgula.">
                  <Input
                    value={(c.options || []).join(", ")}
                    placeholder="Pendente, Pago, Cancelado"
                    onChange={(e) =>
                      patch(i, {
                        options: e.target.value
                          .split(",")
                          .map((s) => s.trim())
                          .filter(Boolean),
                      })
                    }
                  />
                </FieldLabel>
              </div>
            )}
          </div>
        ))}
      </div>
      {columns.length === 0 && <p className="text-[13px] text-mute">Adicione a primeira coluna para criar a base.</p>}
    </div>
  );
}

function ManualField({
  col,
  value,
  onChange,
}: {
  col: ManualColumn;
  value: string;
  onChange: (v: string) => void;
}) {
  if (col.type === "boolean") {
    return (
      <label className="flex items-center gap-2 text-sm text-ink">
        <input type="checkbox" checked={value === "true" || value === "1"} onChange={(e) => onChange(e.target.checked ? "true" : "false")} />
        {col.label}
        {col.required ? <span className="text-accent">*</span> : null}
      </label>
    );
  }
  if (col.type === "select") {
    return (
      <FieldLabel label={col.label} required={col.required} hint={col.hint}>
        <Select value={value} onChange={(e) => onChange(e.target.value)}>
          <option value="">Escolher…</option>
          {(col.options || []).map((o) => (
            <option key={o} value={o}>
              {o}
            </option>
          ))}
        </Select>
      </FieldLabel>
    );
  }
  const inputType = col.type === "number" ? "number" : col.type === "date" ? "date" : col.type === "datetime" ? "datetime-local" : "text";
  return (
    <FieldLabel label={col.label} required={col.required} hint={col.hint}>
      <Input type={inputType} value={value} step={col.type === "number" ? "any" : undefined} onChange={(e) => onChange(e.target.value)} />
    </FieldLabel>
  );
}

function valuesToForm(cols: ManualColumn[], values?: Record<string, unknown>) {
  const out: Record<string, string> = {};
  for (const c of cols) {
    const v = values?.[c.key];
    if (v == null) out[c.key] = c.type === "boolean" ? "false" : "";
    else if (typeof v === "boolean") out[c.key] = v ? "true" : "false";
    else out[c.key] = String(v);
  }
  return out;
}

export function ManualWorkbook({ sourceId }: { sourceId: string }) {
  const qc = useQueryClient();
  const table = useQuery({
    queryKey: ["manual-table", sourceId],
    queryFn: () => api<ManualTable>(`/api/v1/data-sources/${sourceId}/manual`),
  });
  const [draftCols, setDraftCols] = useState<ManualColumn[] | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<Record<string, string>>({});

  const columns = draftCols ?? table.data?.columns ?? [];
  const rows = table.data?.rows ?? [];

  const schemaDirty = useMemo(() => {
    if (!draftCols || !table.data) return false;
    return JSON.stringify(draftCols) !== JSON.stringify(table.data.columns);
  }, [draftCols, table.data]);

  function startEdit(row: ManualRow) {
    setEditingId(row.id);
    setForm(valuesToForm(columns, row.values));
  }

  function resetForm() {
    setEditingId(null);
    setForm(valuesToForm(columns));
  }

  const saveSchema = useMutation({
    mutationFn: () =>
      api<ManualTable>(`/api/v1/data-sources/${sourceId}/manual/schema`, {
        method: "PUT",
        body: JSON.stringify({ columns }),
      }),
    onSuccess: (data) => {
      toast.success("Colunas guardadas");
      setDraftCols(null);
      qc.setQueryData(["manual-table", sourceId], data);
      qc.invalidateQueries({ queryKey: ["source", sourceId] });
      qc.invalidateQueries({ queryKey: ["datasets"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const saveRow = useMutation({
    mutationFn: async () => {
      const values: Record<string, unknown> = {};
      for (const c of columns) {
        const v = form[c.key];
        if (v === undefined || v === "") continue;
        if (c.type === "number") values[c.key] = v.replace(",", ".");
        else if (c.type === "boolean") values[c.key] = v === "true" || v === "1";
        else values[c.key] = v;
      }
      if (editingId) {
        return api<ManualRow>(`/api/v1/data-sources/${sourceId}/manual/rows/${editingId}`, {
          method: "PATCH",
          body: JSON.stringify({ values }),
        });
      }
      return api<ManualRow>(`/api/v1/data-sources/${sourceId}/manual/rows`, {
        method: "POST",
        body: JSON.stringify({ values }),
      });
    },
    onSuccess: () => {
      toast.success(editingId ? "Linha actualizada" : "Linha adicionada");
      resetForm();
      qc.invalidateQueries({ queryKey: ["manual-table", sourceId] });
      qc.invalidateQueries({ queryKey: ["source", sourceId] });
      qc.invalidateQueries({ queryKey: ["datasets"] });
      qc.invalidateQueries({ queryKey: ["semantic"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const removeRow = useMutation({
    mutationFn: (rowId: string) => api(`/api/v1/data-sources/${sourceId}/manual/rows/${rowId}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success("Linha excluída");
      if (editingId) resetForm();
      qc.invalidateQueries({ queryKey: ["manual-table", sourceId] });
      qc.invalidateQueries({ queryKey: ["source", sourceId] });
      qc.invalidateQueries({ queryKey: ["datasets"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  if (table.isLoading) return <p className="text-[13px] text-mute">A carregar a planilha…</p>;
  if (table.isError) return <p className="text-[13px] text-danger">{(table.error as Error).message}</p>;

  const hasCols = columns.length > 0;

  return (
    <div className="space-y-5">
      <Card className="space-y-3">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div>
            <CardTitle>Colunas da planilha</CardTitle>
            <p className="text-[13px] text-mute">Isto é a sua base de dados. Alterar colunas recria o conjunto usado nos dashboards.</p>
          </div>
          <Button onClick={() => saveSchema.mutate()} busy={saveSchema.isPending} disabled={columns.length === 0}>
            <Save size={14} /> Guardar colunas
          </Button>
        </div>
        <ManualSchemaEditor
          columns={columns}
          onChange={(next) => {
            setDraftCols(next);
            setForm(valuesToForm(next, form));
          }}
        />
      </Card>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-5">
        <Card className="space-y-3 lg:col-span-2">
          <CardTitle>{editingId ? "Editar linha" : "Formulário de preenchimento"}</CardTitle>
          {!hasCols && <p className="text-[13px] text-mute">Guarde as colunas para activar o formulário.</p>}
          {hasCols && (
            <>
              <div className="space-y-3">
                {columns.map((c) => (
                  <ManualField key={c.key || c.label} col={c} value={form[c.key] ?? ""} onChange={(v) => setForm((p) => ({ ...p, [c.key]: v }))} />
                ))}
              </div>
              <div className="flex flex-wrap gap-2">
                <Button onClick={() => saveRow.mutate()} busy={saveRow.isPending} disabled={!hasCols || schemaDirty}>
                  <Plus size={14} /> {editingId ? "Guardar linha" : "Adicionar linha"}
                </Button>
                {editingId && (
                  <Button variant="secondary" onClick={resetForm}>
                    Cancelar
                  </Button>
                )}
              </div>
              {schemaDirty && <p className="text-[12px] text-amber-800">Guarde as colunas antes de preencher novas linhas.</p>}
            </>
          )}
        </Card>

        <Card className="lg:col-span-3">
          <div className="mb-3 flex items-center justify-between gap-2">
            <CardTitle>Dados</CardTitle>
            <Badge tone="neutral">{rows.length} linha(s)</Badge>
          </div>
          {rows.length === 0 && (
            <EmptyState title="Planilha vazia" description="Use o formulário à esquerda para adicionar a primeira linha. Os dados entram logo no conjunto analítico." />
          )}
          {rows.length > 0 && (
            <TableWrap>
              <Table>
                <thead className="bg-bg">
                  <tr>
                    {columns.map((c) => (
                      <Th key={c.key}>{c.label}</Th>
                    ))}
                    <Th>Acções</Th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((row) => (
                    <tr key={row.id} className="border-t border-line hover:bg-bg">
                      {columns.map((c) => (
                        <Td key={c.key}>{cellDisplay(row.values?.[c.key])}</Td>
                      ))}
                      <Td>
                        <div className="flex gap-1">
                          <Button size="sm" variant="ghost" onClick={() => startEdit(row)}>
                            <Pencil size={12} /> Editar
                          </Button>
                          <Button
                            size="sm"
                            variant="ghost"
                            className="text-danger"
                            onClick={() => {
                              if (confirm("Excluir esta linha?")) removeRow.mutate(row.id);
                            }}
                          >
                            <Trash2 size={12} />
                          </Button>
                        </div>
                      </Td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            </TableWrap>
          )}
        </Card>
      </div>
    </div>
  );
}

export function defaultManualColumns() {
  return DEFAULT_MANUAL_COLUMNS.map((c) => ({ ...c, options: [...(c.options || [])] }));
}

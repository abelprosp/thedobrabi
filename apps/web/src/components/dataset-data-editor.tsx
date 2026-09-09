"use client";

import { useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { FileSpreadsheet, Plus, Save, Trash2, Upload } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { Button, Card, CardTitle, FieldLabel, Input, Table, Td, Textarea, Th } from "@/components/ui";

type Col = { name: string; type?: string; role?: string; source_name?: string };
type Dataset = {
  id: string;
  name: string;
  row_count?: number;
  storage_mode?: string;
  source_id?: string | null;
  source_type?: string | null;
  schema?: Col[];
};

const EDIT_LIMIT = 2000;
const FILE_ACCEPT = ".csv,.xlsx,.xls,.json,.ndjson";

function dataCols(schema: Col[]) {
  return (schema || []).filter((c) => c?.name && !String(c.name).startsWith("_"));
}

function cellToInput(type: string | undefined, v: unknown): string {
  if (v == null) return "";
  const s = String(v).trim();
  if (!s) return "";
  const t = (type || "").toLowerCase();
  if (t === "date") return s.slice(0, 10);
  if (t === "datetime") {
    const iso = s.includes("T") ? s : s.replace(" ", "T");
    return iso.slice(0, 16);
  }
  if (t === "bool") return s === "true" || s === "1" ? "true" : "false";
  return s;
}

function inputType(type?: string) {
  const t = (type || "").toLowerCase();
  if (t === "int" || t === "float") return "number";
  if (t === "date") return "date";
  if (t === "datetime") return "datetime-local";
  return "text";
}

function emptyRow(cols: Col[]): Record<string, string> {
  return Object.fromEntries(cols.map((c) => [c.name, c.type === "bool" ? "false" : ""]));
}

function filled(cols: Col[], row: Record<string, string>) {
  return cols.some((c) => {
    const v = String(row[c.name] ?? "").trim();
    if (!v) return false;
    if (c.type === "bool" && v === "false") return false;
    return true;
  });
}

export function DatasetDataEditor({ dataset }: { dataset: Dataset }) {
  const qc = useQueryClient();
  const id = dataset.id;
  const cols = useMemo(() => dataCols(dataset.schema || []), [dataset.schema]);
  const live = dataset.storage_mode === "direct_query";
  const manual = dataset.source_type === "manual";
  const canEdit = !live && !manual && cols.length > 0;
  const smallEnough = (dataset.row_count || 0) <= EDIT_LIMIT;
  const appendRef = useRef<HTMLInputElement>(null);
  const replaceRef = useRef<HTMLInputElement>(null);
  const [draft, setDraft] = useState<Record<string, string>>(() => emptyRow(cols));
  const [paste, setPaste] = useState("");
  const [grid, setGrid] = useState<Record<string, string>[] | null>(null);

  function refreshAll() {
    qc.invalidateQueries({ queryKey: ["dataset", id] });
    qc.invalidateQueries({ queryKey: ["preview", id] });
    qc.invalidateQueries({ queryKey: ["dataset-rows", id] });
    qc.invalidateQueries({ queryKey: ["datasets"] });
  }

  const fileMut = useMutation({
    mutationFn: async ({ file, mode }: { file: File; mode: "append" | "replace" }) => {
      const fd = new FormData();
      fd.append("file", file);
      fd.append("mode", mode);
      return api<{ added: number; row_count: number }>(`/api/v1/datasets/${id}/file`, { method: "POST", body: fd });
    },
    onSuccess: (d, vars) => {
      toast.success(vars.mode === "replace" ? `Conjunto actualizado · ${d.row_count?.toLocaleString("pt-BR")} linhas` : `${d.added?.toLocaleString("pt-BR")} linhas acrescentadas`);
      refreshAll();
      setGrid(null);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const rowsMut = useMutation({
    mutationFn: (body: { mode: "append" | "replace"; rows: Record<string, string>[] }) =>
      api<{ added: number; row_count: number }>(`/api/v1/datasets/${id}/rows`, { method: "POST", body: JSON.stringify(body) }),
    onSuccess: (d, vars) => {
      toast.success(vars.mode === "replace" ? "Linhas guardadas" : `${d.added?.toLocaleString("pt-BR")} linhas acrescentadas`);
      refreshAll();
      setDraft(emptyRow(cols));
      setPaste("");
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const loaded = useQuery({
    queryKey: ["dataset-rows", id],
    queryFn: () => api<{ columns: string[]; rows: Record<string, any>[] }>(`/api/v1/datasets/${id}/rows?limit=${EDIT_LIMIT}`),
    enabled: canEdit && smallEnough,
  });

  function onFile(mode: "append" | "replace", e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    if (mode === "replace" && !confirm(`Substituir todas as linhas de «${dataset.name}» pelo ficheiro ${file.name}?`)) return;
    fileMut.mutate({ file, mode });
  }

  function addDraft() {
    if (!filled(cols, draft)) {
      toast.error("Preencha pelo menos um campo");
      return;
    }
    rowsMut.mutate({ mode: "append", rows: [draft] });
  }

  function addPaste() {
    const blob = new Blob([paste], { type: "text/csv" });
    const file = new File([blob], "colar.csv", { type: "text/csv" });
    fileMut.mutate({ file, mode: "append" });
  }

  function startGrid() {
    if (loaded.isError) {
      toast.error((loaded.error as Error)?.message || "Não foi possível carregar as linhas");
      return;
    }
    const source = loaded.data?.rows || [];
    if ((dataset.row_count || 0) > 0 && source.length === 0) {
      toast.error("Não foi possível carregar as linhas para editar");
      return;
    }
    const rows = source.map((r) => {
      const next: Record<string, string> = {};
      for (const c of cols) next[c.name] = cellToInput(c.type, r[c.name]);
      return next;
    });
    setGrid(rows.length ? rows : [emptyRow(cols)]);
  }

  function saveGrid() {
    if (!grid) return;
    const rows = grid.filter((r) => filled(cols, r));
    if (!rows.length) {
      toast.error("Não há linhas para guardar");
      return;
    }
    if (!confirm("Isto substitui todas as linhas do conjunto pelas da tabela. Continuar?")) return;
    rowsMut.mutate({ mode: "replace", rows }, { onSuccess: () => setGrid(null) });
  }

  if (live) {
    return (
      <Card>
        <CardTitle>Dados</CardTitle>
        <p className="text-[13px] text-mute">Este conjunto consulta a fonte em tempo real. Altere os dados na origem e sincronize.</p>
      </Card>
    );
  }
  if (manual) {
    return (
      <Card>
        <CardTitle>Dados</CardTitle>
        <p className="text-[13px] text-mute">Planilha manual: acrescente e edite linhas no conector.</p>
        {dataset.source_id && (
          <Link href={`/connectors/${dataset.source_id}`} className="mt-3 inline-block text-[13px] text-accent hover:underline">
            Abrir planilha
          </Link>
        )}
      </Card>
    );
  }
  if (!canEdit) {
    return (
      <Card>
        <CardTitle>Dados</CardTitle>
        <p className="text-[13px] text-mute">Este conjunto ainda não tem colunas para receber dados.</p>
      </Card>
    );
  }

  const busy = fileMut.isPending || rowsMut.isPending;

  return (
    <div className="space-y-4">
      <Card className="space-y-3">
        <CardTitle>Atualizar arquivo</CardTitle>
        <p className="text-[13px] text-mute">
          Envie outro CSV, Excel ou JSON com as mesmas colunas. Acrescentar junta linhas; substituir troca o conteúdo inteiro.
        </p>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" onClick={() => appendRef.current?.click()} busy={busy}>
            <Upload size={14} /> Acrescentar arquivo
          </Button>
          <Button variant="secondary" onClick={() => replaceRef.current?.click()} busy={busy}>
            <FileSpreadsheet size={14} /> Substituir arquivo
          </Button>
          <input ref={appendRef} type="file" accept={FILE_ACCEPT} className="hidden" onChange={(e) => onFile("append", e)} />
          <input ref={replaceRef} type="file" accept={FILE_ACCEPT} className="hidden" onChange={(e) => onFile("replace", e)} />
        </div>
      </Card>

      <Card className="space-y-3">
        <CardTitle>Adicionar linhas</CardTitle>
        <p className="text-[13px] text-mute">Preencha uma linha ou cole CSV com cabeçalho igual ao conjunto.</p>
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
          {cols.map((c) => (
            <FieldLabel key={c.name} label={c.source_name || c.name} hint={c.type}>
              {c.type === "bool" ? (
                <label className="flex min-h-11 items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={draft[c.name] === "true"}
                    onChange={(e) => setDraft((d) => ({ ...d, [c.name]: e.target.checked ? "true" : "false" }))}
                  />
                  Sim
                </label>
              ) : (
                <Input
                  type={inputType(c.type)}
                  step={c.type === "float" || c.type === "int" ? "any" : undefined}
                  value={draft[c.name] ?? ""}
                  onChange={(e) => setDraft((d) => ({ ...d, [c.name]: e.target.value }))}
                />
              )}
            </FieldLabel>
          ))}
        </div>
        <Button onClick={addDraft} busy={rowsMut.isPending} disabled={busy}>
          <Plus size={14} /> Adicionar linha
        </Button>
        <FieldLabel label="Colar CSV" hint="A primeira linha deve ser o cabeçalho.">
          <Textarea value={paste} onChange={(e) => setPaste(e.target.value)} placeholder={"empresa,valor\nVIVO,100"} />
        </FieldLabel>
        <Button variant="secondary" onClick={addPaste} busy={fileMut.isPending} disabled={busy || !paste.trim()}>
          Acrescentar colado
        </Button>
      </Card>

      <Card className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle>Editar na plataforma</CardTitle>
          {grid ? (
            <div className="flex gap-2">
              <Button size="sm" variant="secondary" onClick={() => setGrid((g) => [...(g || []), emptyRow(cols)])} disabled={busy}>
                <Plus size={14} /> Linha
              </Button>
              <Button size="sm" onClick={saveGrid} busy={rowsMut.isPending}>
                <Save size={14} /> Guardar
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setGrid(null)} disabled={busy}>
                Cancelar
              </Button>
            </div>
          ) : (
            <Button size="sm" variant="secondary" onClick={startGrid} disabled={!smallEnough || loaded.isLoading || busy}>
              Abrir tabela
            </Button>
          )}
        </div>
        {!smallEnough && (
          <p className="text-[13px] text-mute">
            Este conjunto tem {(dataset.row_count || 0).toLocaleString("pt-BR")} linhas. Para editar tudo de uma vez, exporte, altere o arquivo e use Substituir arquivo. Pode sempre acrescentar linhas acima.
          </p>
        )}
        {smallEnough && !grid && (
          <p className="text-[13px] text-mute">Abra a tabela para corrigir valores, apagar linhas ou acrescentar várias de uma vez. Ao guardar, o conjunto passa a ter exactamente o que está na tabela.</p>
        )}
        {grid && (
          <div className="overflow-x-auto">
            <Table className="text-[12px]">
              <thead>
                <tr>
                  {cols.map((c) => (
                    <Th key={c.name}>{c.source_name || c.name}</Th>
                  ))}
                  <Th></Th>
                </tr>
              </thead>
              <tbody>
                {grid.map((row, i) => (
                  <tr key={i} className="border-t border-line">
                    {cols.map((c) => (
                      <Td key={c.name}>
                        {c.type === "bool" ? (
                          <input
                            type="checkbox"
                            checked={row[c.name] === "true"}
                            onChange={(e) =>
                              setGrid((g) => (g || []).map((r, idx) => (idx === i ? { ...r, [c.name]: e.target.checked ? "true" : "false" } : r)))
                            }
                          />
                        ) : (
                          <input
                            className="min-h-9 w-full min-w-[7rem] rounded-lg border border-line bg-surface px-2 py-1 text-[12px]"
                            type={inputType(c.type)}
                            step={c.type === "float" || c.type === "int" ? "any" : undefined}
                            value={row[c.name] ?? ""}
                            onChange={(e) =>
                              setGrid((g) => (g || []).map((r, idx) => (idx === i ? { ...r, [c.name]: e.target.value } : r)))
                            }
                          />
                        )}
                      </Td>
                    ))}
                    <Td>
                      <button
                        type="button"
                        className="text-danger"
                        onClick={() => setGrid((g) => (g || []).filter((_, idx) => idx !== i))}
                        aria-label="Apagar linha"
                      >
                        <Trash2 size={14} />
                      </button>
                    </Td>
                  </tr>
                ))}
              </tbody>
            </Table>
          </div>
        )}
      </Card>
    </div>
  );
}

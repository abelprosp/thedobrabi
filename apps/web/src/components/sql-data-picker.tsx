"use client";

import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Check, ChevronDown, ChevronRight, Link2, Plus, Search, Sparkles, Table2, X } from "lucide-react";
import { api } from "@/lib/api";
import { Button, FieldLabel, Input, Select, Toggle } from "@/components/ui";
import { cn } from "@/lib/cn";
import {
  type DiscoverCatalog,
  type InspectTable,
  type SelectedJoin,
  type SourceSelection,
  catalogFromDiscover,
  formatRowCount,
  joinSentence,
  suggestJoins,
  summaryPhrase,
  tableByKey,
  toApiSelection,
} from "@/lib/sql-picker";

type Step = "tables" | "columns" | "joins" | "summary";

export function SqlDataPicker({
  sourceId,
  sourceName,
  storageMode = "import",
  initialSelection,
  onCancel,
  onDone,
  compact,
}: {
  sourceId: string;
  sourceName?: string;
  storageMode?: string;
  initialSelection?: SourceSelection | null;
  onCancel?: () => void;
  onDone: (datasetId?: string) => void;
  compact?: boolean;
}) {
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [tables, setTables] = useState<InspectTable[]>([]);
  const [fks, setFks] = useState<DiscoverCatalog["foreign_keys"]>([]);
  const [q, setQ] = useState("");
  const [selected, setSelected] = useState<Record<string, string[]>>({});
  const [openTable, setOpenTable] = useState<string | null>(null);
  const [joins, setJoins] = useState<SelectedJoin[]>([]);
  const [step, setStep] = useState<Step>("tables");
  const [autoJoined, setAutoJoined] = useState(false);

  async function load() {
    setLoading(true);
    setError("");
    try {
      const d = await api<DiscoverCatalog>(`/api/v1/data-sources/${sourceId}/discover`, { method: "POST" });
      const cat = catalogFromDiscover(d);
      setTables(cat);
      setFks(d.foreign_keys || []);
      if (Object.keys(selected).length === 0) {
        hydrate(cat, d.foreign_keys || [], initialSelection);
      }
    } catch (e: any) {
      setError(e.message || "Não foi possível ler as listas desta base.");
    } finally {
      setLoading(false);
    }
  }

  function hydrate(cat: InspectTable[], fkList: NonNullable<DiscoverCatalog["foreign_keys"]>, init?: SourceSelection | null) {
    if (init?.tables?.length) {
      const next: Record<string, string[]> = {};
      for (const t of init.tables) {
        const full = t.schema ? `${t.schema}.${t.name}` : t.name;
        const found = tableByKey(cat, full) || tableByKey(cat, t.name);
        if (!found) continue;
        const cols = t.columns?.length ? t.columns : found.columns.map((c) => c.name);
        next[found.full_name] = cols;
      }
      setSelected(next);
      setJoins(init.joins || []);
      return;
    }
    if (cat.length === 1) {
      setSelected({ [cat[0].full_name]: cat[0].columns.map((c) => c.name) });
    }
    void fkList;
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sourceId]);

  const selectedKeys = useMemo(() => Object.keys(selected), [selected]);
  const suggestions = useMemo(() => suggestJoins(tables, fks || [], selectedKeys), [tables, fks, selectedKeys]);

  useEffect(() => {
    if (autoJoined || joins.length > 0 || selectedKeys.length < 2) return;
    const auto = suggestions.filter((s) => s.reason === "fk");
    if (auto.length) {
      setJoins(auto.map(({ reason: _r, ...j }) => j));
      setAutoJoined(true);
    }
  }, [suggestions, selectedKeys.length, joins.length, autoJoined]);

  const filtered = useMemo(() => {
    const s = q.trim().toLowerCase();
    if (!s) return tables;
    return tables.filter((t) => `${t.label} ${t.name} ${t.full_name}`.toLowerCase().includes(s));
  }, [tables, q]);

  const hasColumns = selectedKeys.some((k) => (tableByKey(tables, k)?.columns.length || 0) > 0);
  const needJoins = selectedKeys.length > 1;

  const steps: { id: Step; label: string; hidden?: boolean }[] = [
    { id: "tables", label: "Listas" },
    { id: "columns", label: "Campos", hidden: !hasColumns },
    { id: "joins", label: "Cruzamentos", hidden: !needJoins },
    { id: "summary", label: "Resumo" },
  ];
  const visibleSteps = steps.filter((s) => !s.hidden);
  const stepIndex = Math.max(0, visibleSteps.findIndex((s) => s.id === step));

  function goNext() {
    if (step === "tables") {
      if (selectedKeys.length === 0) {
        toast.error("Escolha pelo menos uma lista");
        return;
      }
      setStep(hasColumns ? "columns" : needJoins ? "joins" : "summary");
      return;
    }
    if (step === "columns") {
      const empty = selectedKeys.filter((k) => (selected[k] || []).length === 0);
      if (empty.length) {
        toast.error("Cada lista precisa de pelo menos um campo ligado");
        return;
      }
      setStep(needJoins ? "joins" : "summary");
      return;
    }
    if (step === "joins") setStep("summary");
  }

  function goBack() {
    if (step === "summary") {
      setStep(needJoins ? "joins" : hasColumns ? "columns" : "tables");
      return;
    }
    if (step === "joins") {
      setStep(hasColumns ? "columns" : "tables");
      return;
    }
    if (step === "columns") setStep("tables");
  }

  function toggleTable(t: InspectTable) {
    setSelected((prev) => {
      const next = { ...prev };
      if (next[t.full_name]) {
        delete next[t.full_name];
        setJoins((js) => js.filter((j) => j.left_table !== t.full_name && j.right_table !== t.full_name));
      } else {
        next[t.full_name] = t.columns.map((c) => c.name);
      }
      return next;
    });
  }

  function toggleCol(table: string, col: string) {
    setSelected((prev) => {
      const cur = new Set(prev[table] || []);
      if (cur.has(col)) cur.delete(col);
      else cur.add(col);
      return { ...prev, [table]: Array.from(cur) };
    });
  }

  function setAllCols(table: string, on: boolean) {
    const t = tableByKey(tables, table);
    setSelected((prev) => ({ ...prev, [table]: on ? (t?.columns.map((c) => c.name) || []) : [] }));
  }

  const apiSel = toApiSelection(tables, selected, joins);

  async function bringData() {
    if (selectedKeys.length === 0) {
      toast.error("Escolha pelo menos uma lista");
      return;
    }
    setBusy(true);
    try {
      await api(`/api/v1/data-sources/${sourceId}`, {
        method: "PATCH",
        body: JSON.stringify({ selection: apiSel }),
      });
      const res = await api<{ dataset_id?: string }>(`/api/v1/data-sources/${sourceId}/sync`, {
        method: "POST",
        body: JSON.stringify({
          selection: apiSel,
          storage_mode: storageMode,
          name: sourceName || undefined,
        }),
      });
      toast.success("Dados trazidos");
      onDone(res.dataset_id);
    } catch (e: any) {
      toast.error(e.message || "Falha ao trazer os dados");
    } finally {
      setBusy(false);
    }
  }

  const unusedSuggestions = suggestions.filter(
    (s) =>
      !joins.some(
        (j) =>
          (j.left_table === s.left_table && j.right_table === s.right_table && j.left_column === s.left_column) ||
          (j.left_table === s.right_table && j.right_table === s.left_table),
      ),
  );

  return (
    <div className={cn("flex flex-col", compact ? "" : "min-h-[28rem]")}>
      <ol className="mb-4 flex flex-wrap gap-1.5" aria-label="Passos">
        {visibleSteps.map((s, i) => (
          <li key={s.id}>
            <button
              type="button"
              onClick={() => selectedKeys.length > 0 && setStep(s.id)}
              className={cn(
                "rounded-full px-3 py-1 text-[12px] font-medium transition",
                s.id === step ? "bg-primary text-white" : i < stepIndex ? "bg-primary/10 text-primary" : "bg-surface-2 text-mute",
              )}
            >
              {i + 1}. {s.label}
            </button>
          </li>
        ))}
      </ol>

      {loading && <p className="py-10 text-center text-sm text-mute">A ler as listas desta base…</p>}
      {error && !loading && (
        <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-[13px] text-amber-900">
          {error}
          <Button className="mt-2" size="sm" variant="secondary" onClick={load}>
            Tentar novamente
          </Button>
        </div>
      )}

      {!loading && !error && step === "tables" && (
        <div className="space-y-3">
          <p className="text-sm text-ink">Que informação queres trazer?</p>
          <p className="text-[12px] text-mute">Escolhe as listas. Os nomes técnicos ficam em letra pequena.</p>
          <div className="relative">
            <Search size={14} className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-mute" />
            <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Procurar listas…" className="pl-8" />
          </div>
          <div className="max-h-[20rem] space-y-1.5 overflow-y-auto pr-1">
            {filtered.map((t) => {
              const on = Boolean(selected[t.full_name]);
              return (
                <label
                  key={t.full_name}
                  className={cn(
                    "flex cursor-pointer items-start gap-3 rounded-xl border px-3 py-2.5 transition",
                    on ? "border-primary/40 bg-primary/5" : "border-line bg-white hover:border-primary/30",
                  )}
                >
                  <input type="checkbox" className="mt-1" checked={on} onChange={() => toggleTable(t)} />
                  <span className="min-w-0 flex-1">
                    <span className="block text-sm font-medium text-ink">{t.label}</span>
                    <span className="block truncate text-[11px] text-mute">{t.full_name}</span>
                  </span>
                  {t.row_count != null && <span className="shrink-0 text-[11px] text-mute">{formatRowCount(t.row_count)}</span>}
                </label>
              );
            })}
            {filtered.length === 0 && <p className="py-6 text-center text-[13px] text-mute">Nenhuma lista corresponde à pesquisa.</p>}
          </div>
        </div>
      )}

      {!loading && step === "columns" && (
        <div className="space-y-3">
          <p className="text-sm text-ink">Quais campos queres em cada lista?</p>
          <p className="text-[12px] text-mute">Tudo vem ligado. Desliga o que não precisas.</p>
          <div className="max-h-[22rem] space-y-2 overflow-y-auto pr-1">
            {selectedKeys.map((key) => {
              const t = tableByKey(tables, key);
              if (!t) return null;
              const cols = selected[key] || [];
              const open = openTable === key || selectedKeys.length === 1;
              return (
                <div key={key} className="rounded-xl border border-line bg-white">
                  <button
                    type="button"
                    className="flex w-full items-center gap-2 px-3 py-2.5 text-left"
                    onClick={() => setOpenTable(open && selectedKeys.length > 1 ? null : key)}
                  >
                    {open ? <ChevronDown size={14} className="text-mute" /> : <ChevronRight size={14} className="text-mute" />}
                    <Table2 size={14} className="text-primary" />
                    <span className="flex-1 text-sm font-medium text-ink">{t.label}</span>
                    <span className="text-[11px] text-mute">
                      {cols.length}/{t.columns.length || cols.length} campos
                    </span>
                  </button>
                  {open && (
                    <div className="border-t border-line px-3 py-2">
                      <div className="mb-2 flex gap-2">
                        <button type="button" className="text-[12px] text-accent hover:underline" onClick={() => setAllCols(key, true)}>
                          Ligar todos
                        </button>
                        <button type="button" className="text-[12px] text-accent hover:underline" onClick={() => setAllCols(key, false)}>
                          Desligar todos
                        </button>
                      </div>
                      {t.columns.length === 0 && <p className="text-[12px] text-mute">Vamos trazer todos os campos desta lista.</p>}
                      <ul className="grid grid-cols-1 gap-1 sm:grid-cols-2">
                        {t.columns.map((c) => (
                          <li key={c.name} className="flex items-center justify-between gap-2 rounded-lg px-1 py-1">
                            <span>
                              <span className="block text-[13px] text-ink">{c.label}</span>
                              <span className="block text-[10px] text-mute">{c.name}</span>
                            </span>
                            <Toggle checked={cols.includes(c.name)} onChange={() => toggleCol(key, c.name)} label={c.label} />
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}

      {!loading && step === "joins" && (
        <div className="space-y-3">
          <p className="text-sm text-ink">Como se cruzam estas listas?</p>
          <p className="text-[12px] text-mute">Diz que campo de uma lista combina com a outra. Sem SQL.</p>

          {unusedSuggestions.length > 0 && (
            <div className="space-y-1.5">
              <p className="flex items-center gap-1 text-[12px] font-medium text-primary">
                <Sparkles size={12} /> Sugestões
              </p>
              {unusedSuggestions.map((s) => (
                <button
                  key={`${s.left_table}-${s.left_column}-${s.right_table}`}
                  type="button"
                  onClick={() => setJoins((js) => [...js, { left_table: s.left_table, left_column: s.left_column, right_table: s.right_table, right_column: s.right_column, match: s.match }])}
                  className="flex w-full items-center gap-2 rounded-xl border border-primary/20 bg-primary/5 px-3 py-2 text-left text-[13px] text-ink hover:border-primary/40"
                >
                  <Link2 size={14} className="text-primary" />
                  <span className="flex-1">{joinSentence(tables, s)}</span>
                  <span className="text-[11px] text-primary">{s.reason === "fk" ? "ligação conhecida" : "pelo nome"}</span>
                </button>
              ))}
            </div>
          )}

          {joins.map((j, idx) => (
            <JoinCard
              key={`${j.left_table}-${idx}`}
              tables={tables.filter((t) => selectedKeys.includes(t.full_name))}
              join={j}
              onChange={(next) => setJoins((js) => js.map((x, i) => (i === idx ? next : x)))}
              onRemove={() => setJoins((js) => js.filter((_, i) => i !== idx))}
            />
          ))}

          <Button
            size="sm"
            variant="secondary"
            onClick={() => {
              const a = selectedKeys[0];
              const b = selectedKeys[1] || selectedKeys[0];
              const ta = tableByKey(tables, a);
              const tb = tableByKey(tables, b);
              setJoins((js) => [
                ...js,
                {
                  left_table: a,
                  left_column: ta?.columns[0]?.name || "id",
                  right_table: b,
                  right_column: tb?.columns.find((c) => c.name.toLowerCase() === "id")?.name || tb?.columns[0]?.name || "id",
                  match: "both",
                },
              ]);
            }}
          >
            <Plus size={12} /> Combinar outras listas
          </Button>
          {joins.length === 0 && (
            <p className="text-[12px] text-mute">Sem cruzamento, cada lista vem num conjunto à parte.</p>
          )}
        </div>
      )}

      {!loading && step === "summary" && (
        <div className="space-y-3">
          <p className="text-sm leading-relaxed text-ink">{summaryPhrase(tables, apiSel)}</p>
          <ul className="space-y-2">
            {selectedKeys.map((key) => {
              const t = tableByKey(tables, key);
              const cols = selected[key] || [];
              return (
                <li key={key} className="rounded-xl border border-line bg-white px-3 py-2">
                  <div className="text-sm font-medium text-ink">{t?.label || key}</div>
                  <div className="mt-1 flex flex-wrap gap-1">
                    {(cols.length ? cols : ["todos"]).slice(0, 12).map((c) => (
                      <span key={c} className="rounded-full bg-surface-2 px-2 py-0.5 text-[11px] text-mute">
                        {t?.columns.find((x) => x.name === c)?.label || c}
                      </span>
                    ))}
                    {cols.length > 12 && <span className="text-[11px] text-mute">+{cols.length - 12}</span>}
                  </div>
                </li>
              );
            })}
          </ul>
          {joins.map((j, i) => (
            <p key={i} className="flex items-center gap-2 text-[13px] text-ink">
              <Check size={14} className="text-primary" />
              {joinSentence(tables, j)}
              <span className="text-[12px] text-mute">
                {j.match === "all_left" ? "— manter todos mesmo sem correspondência" : "— só quando existe nos dois"}
              </span>
            </p>
          ))}
        </div>
      )}

      <div className="mt-5 flex flex-wrap justify-between gap-2">
        <div className="flex gap-2">
          {onCancel && (
            <Button variant="ghost" onClick={onCancel}>
              Agora não
            </Button>
          )}
          {step !== "tables" && (
            <Button variant="secondary" onClick={goBack}>
              Voltar
            </Button>
          )}
        </div>
        <div className="flex gap-2">
          {step !== "summary" ? (
            <Button onClick={goNext} disabled={loading || selectedKeys.length === 0}>
              Continuar
            </Button>
          ) : (
            <Button onClick={bringData} busy={busy}>
              Trazer dados
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}

function JoinCard({
  tables,
  join,
  onChange,
  onRemove,
}: {
  tables: InspectTable[];
  join: SelectedJoin;
  onChange: (j: SelectedJoin) => void;
  onRemove: () => void;
}) {
  const left = tableByKey(tables, join.left_table);
  const right = tableByKey(tables, join.right_table);
  return (
    <div className="rounded-xl border border-line bg-white p-3">
      <div className="mb-2 flex items-center justify-between">
        <p className="text-[13px] font-medium text-ink">{joinSentence(tables, join)}</p>
        <button type="button" onClick={onRemove} className="text-mute hover:text-danger" aria-label="Remover cruzamento">
          <X size={14} />
        </button>
      </div>
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        <FieldLabel label="Esta lista">
          <Select value={join.left_table} onChange={(e) => onChange({ ...join, left_table: e.target.value })}>
            {tables.map((t) => (
              <option key={t.full_name} value={t.full_name}>
                {t.label}
              </option>
            ))}
          </Select>
        </FieldLabel>
        <FieldLabel label="combina com">
          <Select value={join.right_table} onChange={(e) => onChange({ ...join, right_table: e.target.value })}>
            {tables.map((t) => (
              <option key={t.full_name} value={t.full_name}>
                {t.label}
              </option>
            ))}
          </Select>
        </FieldLabel>
        <FieldLabel label="pelo campo">
          <Select value={join.left_column} onChange={(e) => onChange({ ...join, left_column: e.target.value })}>
            {(left?.columns || []).map((c) => (
              <option key={c.name} value={c.name}>
                {c.label}
              </option>
            ))}
          </Select>
        </FieldLabel>
        <FieldLabel label="e o campo">
          <Select value={join.right_column} onChange={(e) => onChange({ ...join, right_column: e.target.value })}>
            {(right?.columns || []).map((c) => (
              <option key={c.name} value={c.name}>
                {c.label}
              </option>
            ))}
          </Select>
        </FieldLabel>
      </div>
      <div className="mt-3 space-y-1.5">
        <label className="flex items-center gap-2 text-[13px] text-ink">
          <input type="radio" checked={join.match !== "all_left"} onChange={() => onChange({ ...join, match: "both" })} />
          Só quando existe nos dois
        </label>
        <label className="flex items-center gap-2 text-[13px] text-ink">
          <input type="radio" checked={join.match === "all_left"} onChange={() => onChange({ ...join, match: "all_left" })} />
          Manter todos os {left?.label || "da primeira lista"}, mesmo sem correspondência
        </label>
      </div>
    </div>
  );
}

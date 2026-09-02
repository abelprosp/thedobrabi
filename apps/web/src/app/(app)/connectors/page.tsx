"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Cloud, Database, FileSpreadsheet, Globe, Landmark, Megaphone, MessagesSquare, Plug, Radio, RefreshCw, Search, Share2, Trash2, TrendingUp, Unplug } from "lucide-react";
import { api, normalizeArray } from "@/lib/api";
import { statusLabel } from "@/lib/labels";
import {
  type CatalogItem,
  type CatalogResponse,
  type DataSource,
  connectorByType,
  connectorIconSrc,
  connectorLabel,
  formatSyncAt,
  isGoogleSheetsType,
  isGuidedSQLType,
  isManualType,
} from "@/lib/connectors";
import { ConnectorIcon } from "@/components/connector-icon";
import { SqlDataPicker } from "@/components/sql-data-picker";
import { ManualSchemaEditor, defaultManualColumns } from "@/components/manual-connector";
import type { ManualColumn } from "@/lib/manual";
import { Badge, Button, Card, EmptyState, ErrorState, FieldLabel, Input, PageHeader, PageSkeleton, Select, Table, TableWrap, Td, Textarea, Th, cn } from "@/components/ui";
import { frequencyLabel, formatRelativePt } from "@/lib/schedules";

const groupIcon = {
  databases: Database,
  files: FileSpreadsheet,
  negocio_brasil: Landmark,
  publicidade: Megaphone,
  redes_crm: MessagesSquare,
  dados_economicos: TrendingUp,
  cloud: Cloud,
  web: Globe,
  streaming: Radio,
} as const;

export default function ConnectorsPage() {
  const qc = useQueryClient();
  const catalog = useQuery({
    queryKey: ["connectors-catalog"],
    queryFn: () => api<CatalogResponse>("/api/v1/connectors/catalog"),
  });
  const sources = useQuery({
    queryKey: ["sources"],
    queryFn: () => api<any>("/api/v1/data-sources"),
  });
  const items = catalog.data?.items ?? [];
  const groups = catalog.data?.groups ?? [];
  const sourceList = normalizeArray<DataSource>(sources.data);
  const [q, setQ] = useState("");
  const [picked, setPicked] = useState<CatalogItem | null>(null);

  const filtered = useMemo(() => {
    const s = q.trim().toLowerCase();
    if (!s) return items;
    return items.filter((it) => `${it.label} ${it.id} ${it.description} ${(it.aliases || []).join(" ")}`.toLowerCase().includes(s));
  }, [items, q]);

  const remove = useMutation({
    mutationFn: (id: string) => api(`/api/v1/data-sources/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success("Fonte excluída");
      qc.invalidateQueries({ queryKey: ["sources"] });
      qc.invalidateQueries({ queryKey: ["datasets"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  async function discover(id: string) {
    try {
      const d = await api<{ tables: string[]; message?: string; preview?: boolean }>(`/api/v1/data-sources/${id}/discover`, { method: "POST" });
      if (d.message) toast.message(d.message);
      else toast.success(`${(d.tables || []).length} tabela(s) encontradas`);
    } catch (e: any) {
      toast.error(e.message);
    }
  }

  async function sync(id: string) {
    try {
      await api(`/api/v1/data-sources/${id}/sync`, { method: "POST", body: JSON.stringify({ storage_mode: "import" }) });
      toast.success("Sincronização concluída");
      qc.invalidateQueries({ queryKey: ["sources"] });
      qc.invalidateQueries({ queryKey: ["datasets"] });
    } catch (e: any) {
      toast.error(e.message);
    }
  }

  if (catalog.isLoading) return <PageSkeleton cards={4} />;
  if (catalog.isError) return <ErrorState message={(catalog.error as Error).message} onRetry={() => catalog.refetch()} />;

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <PageHeader
        title="Conectores"
        description="Hub de dados da TheDobra — ligue bases de dados, ficheiros, APIs e fontes cloud. Configure a actualização automática em cada conector."
        actions={
          <div className="relative">
            <Search size={14} className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-mute" />
            <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Filtrar catálogo…" className="w-56 pl-8" />
          </div>
        }
      />

      <Card>
        <div className="mb-3 flex items-center justify-between gap-2">
          <h2 className="text-[13px] font-medium text-mute">Instâncias ligadas</h2>
          <span className="text-[12px] text-mute">{sourceList.length} fonte(s)</span>
        </div>
        {sources.isError && <ErrorState message={(sources.error as Error).message} onRetry={() => sources.refetch()} />}
        {sourceList.length === 0 && !sources.isLoading && (
          <EmptyState icon={Unplug} title="Nenhuma fonte ligada" description="Escolha um conector no catálogo abaixo para criar a primeira ligação." />
        )}
        {sourceList.length > 0 && (
          <TableWrap>
            <Table>
              <thead className="bg-bg">
                <tr>
                  <Th>Nome</Th>
                  <Th>Tipo</Th>
                  <Th>Estado</Th>
                  <Th>Último sync</Th>
                  <Th>Actualização</Th>
                  <Th>Acções</Th>
                </tr>
              </thead>
              <tbody>
                {sourceList.map((s) => (
                  <tr key={s.id} className="border-t border-line hover:bg-bg">
                    <Td>
                      <Link href={`/connectors/${s.id}`} className="font-medium text-accent hover:underline">
                        {s.name}
                      </Link>
                    </Td>
                    <Td>
                      <span className="inline-flex items-center gap-2">
                        <ConnectorIcon
                          src={connectorIconSrc(connectorByType(items, s.type), s.type)}
                          className="h-5 w-5 object-contain"
                          boxClassName="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-white ring-1 ring-line"
                        />
                        {connectorLabel(items, s.type)}
                      </span>
                    </Td>
                    <Td>
                      <Badge tone={s.preview || s.status === "preview" || s.status === "failed" ? (s.status === "failed" ? "danger" : "warn") : s.status === "synced" ? "ok" : "neutral"}>
                        {s.preview && s.status === "preview" ? "Aguardando sync" : statusLabel(s.status)}
                      </Badge>
                    </Td>
                    <Td>{formatSyncAt(s.last_sync_at)}</Td>
                    <Td>
                      {s.schedule ? (
                        <span className="text-[12px] text-ink">
                          {s.schedule.enabled ? frequencyLabel(s.schedule.frequency) : "Em pausa"}
                          {s.schedule.enabled && s.schedule.next_run_at ? (
                            <span className="block text-mute">próximo {formatRelativePt(s.schedule.next_run_at)}</span>
                          ) : null}
                        </span>
                      ) : (
                        <span className="text-[12px] text-mute">Manual</span>
                      )}
                    </Td>
                    <Td>
                      <div className="flex flex-wrap gap-1">
                        <Button size="sm" variant="ghost" onClick={() => discover(s.id)}>
                          Descobrir
                        </Button>
                        <Button size="sm" variant="ghost" onClick={() => sync(s.id)}>
                          <RefreshCw size={12} /> Sync
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          className="text-danger"
                          onClick={() => {
                            if (confirm(`Excluir «${s.name}» e os conjuntos sincronizados a partir desta fonte?`)) remove.mutate(s.id);
                          }}
                        >
                          <Trash2 size={12} /> Excluir
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

      {groups.map((g) => {
        const list = filtered.filter((it) => it.group === g.id);
        if (list.length === 0) return null;
        const Icon = groupIcon[g.id as keyof typeof groupIcon] || Plug;
        return (
          <section key={g.id}>
            <div className="mb-3 flex items-center gap-2">
              <Icon size={16} className="text-primary" />
              <h2 className="text-sm font-semibold text-ink">{g.label}</h2>
              <span className="text-[12px] text-mute">{list.length}</span>
            </div>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {list.map((it) => {
                return (
                  <button
                    key={it.id}
                    type="button"
                    onClick={() => setPicked(it)}
                    className="rounded-2xl border border-line bg-white p-4 text-left shadow-sm transition hover:border-primary/40 hover:shadow"
                  >
                    <div className="flex items-start justify-between gap-2">
                      <ConnectorIcon src={connectorIconSrc(it)} />
                      <Badge tone={it.implemented && !it.preview ? "ok" : "warn"}>{it.implemented && !it.preview ? "Sync real" : "Preview"}</Badge>
                    </div>
                    <div className="mt-3 text-sm font-medium text-ink">{it.label}</div>
                    <p className="mt-1 line-clamp-2 text-[12px] text-mute">{it.description}</p>
                  </button>
                );
              })}
            </div>
          </section>
        );
      })}

      {filtered.length === 0 && <p className="text-sm text-mute">Nenhum conector corresponde à pesquisa.</p>}

      {picked && (
        <ConnectWizard
          item={picked}
          onClose={() => setPicked(null)}
          onSaved={() => {
            qc.invalidateQueries({ queryKey: ["sources"] });
            qc.invalidateQueries({ queryKey: ["datasets"] });
            setPicked(null);
          }}
        />
      )}
    </div>
  );
}

function ConnectWizard({ item, onClose, onSaved }: { item: CatalogItem; onClose: () => void; onSaved: () => void }) {
  const router = useRouter();
  const [values, setValues] = useState<Record<string, string>>(() => {
    const init: Record<string, string> = {};
    for (const f of item.fields) {
      if (f.default) init[f.key] = f.default;
    }
    if (!init.name) init.name = item.label;
    return init;
  });
  const [file, setFile] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [ssl, setSsl] = useState(false);
  const [sourceId, setSourceId] = useState<string | null>(null);
  const [columns, setColumns] = useState<ManualColumn[]>(() => (isManualType(item.id) ? defaultManualColumns() : []));
  const [sheetsError, setSheetsError] = useState("");
  const [sheetsSourceId, setSheetsSourceId] = useState<string | null>(null);
  const sheets = isGoogleSheetsType(item.id);

  function set(key: string, v: string) {
    setValues((p) => ({ ...p, [key]: v }));
  }

  async function importSheets(id: string) {
    await api(`/api/v1/data-sources/${id}/sync`, {
      method: "POST",
      body: JSON.stringify({ storage_mode: "import", name: values.name || item.label, table: values.table || "" }),
    });
  }

  async function save() {
    setBusy(true);
    setSheetsError("");
    try {
      if (sheets && !values.url?.trim()) {
        setSheetsError("Cole o link da planilha.");
        return;
      }
      if (sheets && sheetsSourceId) {
        await importSheets(sheetsSourceId);
        toast.success("Planilha importada");
        onSaved();
        return;
      }

      const config: Record<string, unknown> = { ssl };
      for (const f of item.fields) {
        if (f.key === "name" || f.key === "file" || f.type === "file") continue;
        if (sheets && (f.key === "api_key" || f.key === "token")) continue;
        const v = values[f.key];
        if (v === undefined || v === "") continue;
        if (f.type === "number") config[f.key] = Number(v) || 0;
        else if (f.type === "checkbox") config[f.key] = v === "true" || v === "1";
        else config[f.key] = v;
      }
      if (ssl && !config.ssl_mode) config.ssl_mode = "require";
      if (file) config.file_name = file.name;
      if (isManualType(item.id)) config.columns = columns;

      const res = await api<{ id: string; preview?: boolean; message?: string }>("/api/v1/data-sources", {
        method: "POST",
        body: JSON.stringify({ name: values.name || item.label, type: item.id, config }),
      });

      const canUpload = file && ["csv", "xlsx", "json", "parquet", "pdf", "contabilidade"].includes(item.id);
      if (canUpload) {
        const fd = new FormData();
        fd.append("file", file);
        fd.append("name", values.name || file.name.replace(/\.[^.]+$/, ""));
        fd.append("data_source_id", res.id);
        await api("/api/v1/datasets/upload", { method: "POST", body: fd });
        toast.success("Ficheiro ingerido");
        onSaved();
        return;
      }
      if (isManualType(item.id)) {
        toast.success("Planilha criada. Agora preencha o formulário.");
        router.push(`/connectors/${res.id}`);
        onSaved();
        return;
      }
      if (item.preview || res.preview) {
        toast.message(res.message || item.message || "Conector guardado em preview.");
        onSaved();
        return;
      }
      if (isGuidedSQLType(item.id)) {
        toast.success("Ligação feita. Agora escolha o que trazer.");
        setSourceId(res.id);
        return;
      }
      if (sheets) {
        setSheetsSourceId(res.id);
        await importSheets(res.id);
        toast.success("Planilha importada");
        onSaved();
        return;
      }
      toast.success("Conector ligado");
      onSaved();
    } catch (e: any) {
      if (sheets) {
        setSheetsError(e.message || "Não foi possível ler a planilha.");
      }
      toast.error(e.message);
    } finally {
      setBusy(false);
    }
  }

  const picking = Boolean(sourceId);

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-ink/30 p-4 sm:items-center" onClick={picking ? undefined : onClose}>
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="connect-title"
        className={cn(
          "max-h-[90vh] w-full overflow-y-auto rounded-2xl border border-line bg-white p-5 shadow-2xl",
          picking || isManualType(item.id) ? "max-w-2xl" : "max-w-lg",
        )}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-3">
          <ConnectorIcon src={connectorIconSrc(item)} />
          <h2 id="connect-title" className="text-lg font-semibold text-ink">
            {picking ? `O que trazer de ${item.label}` : `Ligar ${item.label}`}
          </h2>
        </div>
        {picking && sourceId ? (
          <>
            <p className="mt-1 text-[13px] text-mute">Ligação guardada. Escolha as listas, os campos e como se cruzam — sem SQL.</p>
            <div className="mt-4">
              <SqlDataPicker
                sourceId={sourceId}
                sourceName={values.name || item.label}
                onCancel={onSaved}
                onDone={() => onSaved()}
              />
            </div>
          </>
        ) : (
          <>
        <p className="mt-1 text-[13px] text-mute">{item.description}</p>
        {sheets && <SheetsShareGuide />}
        {sheetsError && (
          <p className="mt-3 rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-[12px] text-red-800">{sheetsError}</p>
        )}
        {(item.preview || item.message) && (
          <p className="mt-3 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-[12px] text-amber-800">
            {item.message || "Conector em preview. A sincronização directa requer o gateway ou CSV."}
          </p>
        )}
        <div className="mt-4 space-y-3">
          {item.fields.map((f) => {
            if (isGuidedSQLType(item.id) && (f.key === "query" || f.key === "table")) return null;
            if (sheets && (f.key === "api_key" || f.key === "token" || f.key === "query" || f.key === "limit")) return null;
            if (f.type === "file") {
              return (
                <FieldLabel key={f.key} label={f.label} required={f.required} hint={f.hint}>
                  <input
                    type="file"
                    accept={f.accept}
                    className="block w-full text-sm text-ink file:mr-3 file:rounded-lg file:border file:border-line file:bg-white file:px-3 file:py-1.5"
                    onChange={(e) => setFile(e.target.files?.[0] || null)}
                  />
                </FieldLabel>
              );
            }
            if (f.type === "checkbox") {
              if (f.key === "ssl") {
                return (
                  <label key={f.key} className="flex items-center gap-2 text-sm text-ink">
                    <input type="checkbox" checked={ssl} onChange={(e) => setSsl(e.target.checked)} />
                    {f.label}
                  </label>
                );
              }
              return (
                <label key={f.key} className="flex items-center gap-2 text-sm text-ink">
                  <input
                    type="checkbox"
                    checked={values[f.key] === "true"}
                    onChange={(e) => set(f.key, e.target.checked ? "true" : "false")}
                  />
                  {f.label}
                </label>
              );
            }
            if (f.type === "select") {
              return (
                <FieldLabel key={f.key} label={f.label} hint={f.hint} required={f.required}>
                  <Select value={values[f.key] || ""} onChange={(e) => set(f.key, e.target.value)}>
                    {(f.options || []).map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </Select>
                </FieldLabel>
              );
            }
            if (f.type === "textarea") {
              return (
                <FieldLabel key={f.key} label={f.label} hint={f.hint} required={f.required}>
                  <Textarea value={values[f.key] || ""} placeholder={f.placeholder} onChange={(e) => set(f.key, e.target.value)} />
                </FieldLabel>
              );
            }
            return (
              <FieldLabel key={f.key} label={f.label} hint={f.hint} required={f.required}>
                <Input
                  type={f.type === "password" ? "password" : f.type === "number" ? "number" : f.type === "url" ? "url" : "text"}
                  value={values[f.key] || ""}
                  placeholder={f.placeholder}
                  onChange={(e) => set(f.key, e.target.value)}
                />
              </FieldLabel>
            );
          })}
        </div>
        {isManualType(item.id) && (
          <div className="mt-4">
            <p className="mb-2 text-[13px] font-medium text-ink">Colunas da planilha</p>
            <ManualSchemaEditor columns={columns} onChange={setColumns} />
          </div>
        )}
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="secondary" onClick={onClose}>
            Cancelar
          </Button>
          <Button onClick={save} busy={busy}>
            <Plug size={14} />{" "}
            {isGuidedSQLType(item.id)
              ? "Ligar e escolher dados"
              : isManualType(item.id)
                ? "Criar planilha"
                : sheets
                  ? sheetsSourceId
                    ? "Tentar importar de novo"
                    : "Importar planilha"
                  : "Guardar ligação"}
          </Button>
        </div>
          </>
        )}
      </div>
    </div>
  );
}

function SheetsShareGuide() {
  const steps = [
    "Abra a planilha no Google Sheets",
    "Clique em Partilhar (canto superior direito)",
    "Em Acesso geral, escolha Qualquer pessoa com o link",
    "Deixe a permissão em Leitor e copie o link",
  ];
  return (
    <div className="mt-3 rounded-xl border border-emerald-200 bg-emerald-50/70 px-3 py-3">
      <div className="flex items-center gap-2 text-[12px] font-medium text-emerald-900">
        <Share2 size={14} />
        Sem chave de API — basta partilhar o link
      </div>
      <ol className="mt-2 space-y-1 pl-1 text-[12px] text-emerald-900/90">
        {steps.map((s, i) => (
          <li key={s} className="flex gap-2">
            <span className="mt-px flex h-4 w-4 shrink-0 items-center justify-center rounded-full bg-emerald-200 text-[10px] font-semibold">{i + 1}</span>
            <span>{s}</span>
          </li>
        ))}
      </ol>
    </div>
  );
}

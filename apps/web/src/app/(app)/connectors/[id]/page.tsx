"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import { Plug, RefreshCw, Trash2, Unplug, LayoutDashboard } from "lucide-react";
import { api, normalizeArray } from "@/lib/api";
import { statusLabel } from "@/lib/labels";
import { type CatalogResponse, type DataSource, connectorIconSrc, formatSyncAt, isGoogleSheetsType, isGuidedSQLType, isManualType } from "@/lib/connectors";
import { ConnectorIcon } from "@/components/connector-icon";
import { SqlDataPicker } from "@/components/sql-data-picker";
import { ManualWorkbook } from "@/components/manual-connector";
import { Badge, Button, Card, CardTitle, EmptyState, ErrorState, FieldLabel, PageHeader, PageSkeleton, Select } from "@/components/ui";
import { starterDashboardWidgets } from "@/lib/semantic";
import { AutoRefreshCard } from "@/components/auto-refresh-card";
import type { SourceSelection } from "@/lib/sql-picker";

export default function ConnectorDetailPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const qc = useQueryClient();
  const [storageMode, setStorageMode] = useState("import");
  const [tables, setTables] = useState<string[]>([]);
  const [discoverMsg, setDiscoverMsg] = useState("");

  const [lastSyncedId, setLastSyncedId] = useState<string | null>(null);

  const catalog = useQuery({
    queryKey: ["connectors-catalog"],
    queryFn: () => api<CatalogResponse>("/api/v1/connectors/catalog"),
  });
  const src = useQuery({
    queryKey: ["source", id],
    queryFn: () => api<DataSource>(`/api/v1/data-sources/${id}`),
  });

  const item = catalog.data?.items.find((i) => i.id === src.data?.type);

  const remove = useMutation({
    mutationFn: () => api(`/api/v1/data-sources/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success("Fonte excluída");
      qc.invalidateQueries({ queryKey: ["sources"] });
      qc.invalidateQueries({ queryKey: ["datasets"] });
      router.push("/connectors");
    },
    onError: (e: Error) => toast.error(e.message),
  });

  async function test() {
    try {
      const res = await api<{ ok: boolean; implemented: boolean; message: string }>(`/api/v1/data-sources/${id}/test`, { method: "POST" });
      if (res.ok) toast.success(res.message || "Ligação OK");
      else toast.error(res.message || "Falha no teste de ligação");
    } catch (e: any) {
      toast.error(e.message);
    }
  }

  async function discover() {
    try {
      const d = await api<{ tables: string[]; message?: string; preview?: boolean }>(`/api/v1/data-sources/${id}/discover`, { method: "POST" });
      setTables(d.tables || []);
      setDiscoverMsg(d.message || "");
      if (d.message) toast.message(d.message);
      else toast.success(`${(d.tables || []).length} tabela(s)`);
    } catch (e: any) {
      toast.error(e.message);
    }
  }

  async function sync(table?: string) {
    try {
      const res = await api<{ dataset_id: string }>(`/api/v1/data-sources/${id}/sync`, {
        method: "POST",
        body: JSON.stringify({ table: table || "", name: table || src.data?.name, storage_mode: storageMode }),
      });
      toast.success("Sincronizado");
      if (res.dataset_id) setLastSyncedId(res.dataset_id);
      qc.invalidateQueries({ queryKey: ["source", id] });
      qc.invalidateQueries({ queryKey: ["datasets"] });
      qc.invalidateQueries({ queryKey: ["semantic"] });
    } catch (e: any) {
      toast.error(e.message);
    }
  }

  async function openInDashboard(datasetId: string, datasetName: string) {
    try {
      const ds = await api<{ semantic_model?: any }>(`/api/v1/datasets/${datasetId}`);
      const widgets = starterDashboardWidgets(datasetId, ds.semantic_model);
      const dash = await api<{ id: string }>("/api/v1/dashboards", {
        method: "POST",
        body: JSON.stringify({
          name: datasetName,
          description: `A partir do conector ${src.data?.name || datasetName}`,
          layout: { widgets },
        }),
      });
      router.push(`/dashboards/${dash.id}`);
    } catch (e: any) {
      toast.error(e.message);
    }
  }

  if (src.isLoading) return <PageSkeleton />;
  if (src.isError) return <ErrorState message={(src.error as Error).message} onRetry={() => src.refetch()} />;
  if (!src.data) return <ErrorState message="Conector não encontrado" />;

  const s = src.data;
  const cfg = s.config || {};
  const datasets = normalizeArray(s.datasets);
  const preview = Boolean(s.preview && !s.implemented);

  const configRows: { label: string; value: string }[] = [
    { label: "Anfitrião", value: str(cfg.host) },
    { label: "Porta", value: cfg.port ? String(cfg.port) : "" },
    { label: "Base de dados", value: str(cfg.database) },
    { label: "Utilizador", value: str(cfg.user) },
    { label: "SSL", value: cfg.ssl ? "sim" : str(cfg.ssl_mode) },
    { label: "URL", value: str(cfg.url) },
    { label: "URL do projecto", value: str(cfg.project_url) },
    { label: "Conta", value: str(cfg.account) },
    { label: "Warehouse", value: str(cfg.warehouse) },
    { label: "Projecto", value: str(cfg.project) },
    { label: "Dataset", value: str(cfg.dataset) },
    { label: "Esquema", value: str(cfg.schema) },
    { label: "Broker", value: str(cfg.broker) },
    { label: "Tópico", value: str(cfg.topic) },
    { label: "Ficheiro", value: str(cfg.file_name) },
    { label: "Tabela", value: str(cfg.table) },
    { label: "Senha", value: cfg.password_set ? "••••••••" : "" },
    { label: "Token", value: cfg.token_set ? "••••••••" : "" },
    { label: "Ambiente", value: str(cfg.environment) },
    { label: "Webhook", value: str(cfg.webhook_url) },
    { label: "Domínio", value: str(cfg.domain) },
    { label: "Customer ID", value: str(cfg.customer_id) },
    { label: "Ad account", value: str(cfg.ad_account_id) },
    { label: "Page ID", value: str(cfg.page_id) },
    { label: "Instagram ID", value: str(cfg.instagram_id) },
    { label: "Location ID", value: str(cfg.location_id) },
    { label: "Seller ID", value: str(cfg.seller_id) },
    { label: "Séries", value: str(cfg.series) },
    { label: "Property ID", value: str(cfg.property_id) },
    { label: "Client ID", value: str(cfg.client_id) },
    { label: "Access token", value: cfg.access_token_set ? "••••••••" : "" },
    { label: "App key", value: cfg.app_key_set ? "••••••••" : "" },
    { label: "Chave API", value: cfg.api_key_set ? "••••••••" : "" },
    { label: "Service role", value: cfg.service_role_key_set ? "••••••••" : "" },
    { label: "Anon key", value: cfg.anon_key_set ? "••••••••" : "" },
  ].filter((r) => r.value);

  return (
    <div className="mx-auto max-w-4xl space-y-5">
      <div className="flex items-start gap-3">
        <ConnectorIcon src={connectorIconSrc(item, s.type)} className="h-8 w-8 object-contain" boxClassName="mt-1 flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-white ring-1 ring-line" />
        <div className="min-w-0 flex-1">
      <PageHeader
        title={s.name}
        description={`${item?.label || s.type} · último sync ${formatSyncAt(s.last_sync_at)}`}
        crumbs={[{ href: "/connectors", label: "Conectores" }]}
        actions={
          <>
            <Button variant="secondary" onClick={test}>
              Testar ligação
            </Button>
            <Button onClick={() => sync()} variant="primary">
              <RefreshCw size={14} /> Sincronizar
            </Button>
            {(lastSyncedId || datasets[0]?.id) && (
              <Button
                variant="secondary"
                onClick={() => {
                  const dsId = lastSyncedId || datasets[0].id;
                  const dsName = datasets.find((d: any) => d.id === dsId)?.name || s.name;
                  openInDashboard(dsId, dsName);
                }}
              >
                <LayoutDashboard size={14} /> Abrir no dashboard
              </Button>
            )}
            <Button
              variant="danger"
              onClick={() => {
                if (confirm(`Excluir «${s.name}» e os conjuntos sincronizados a partir desta fonte?`)) remove.mutate();
              }}
              busy={remove.isPending}
            >
              <Trash2 size={14} /> Excluir fonte
            </Button>
          </>
        }
      />
        </div>
      </div>

      {preview && (
        <p className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-[13px] text-amber-800">
          {s.message || item?.message || "Conector em preview. A sincronização directa requer o gateway ou importação CSV/XLSX."}
        </p>
      )}

      {isGoogleSheetsType(s.type) && (
        <p className="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-[13px] text-emerald-900">
          Esta ligação não usa chave de API. A planilha tem de estar partilhada com <strong>qualquer pessoa com o link</strong> (Leitor). Depois sincronize de novo.
        </p>
      )}

      {isManualType(s.type) && <ManualWorkbook sourceId={id} />}

      {!isManualType(s.type) && (
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Card>
          <CardTitle>Configuração</CardTitle>
          {configRows.length === 0 && <p className="text-[13px] text-mute">Sem parâmetros visíveis.</p>}
          {configRows.map((r) => (
            <div key={r.label} className="flex justify-between gap-3 border-t border-line py-2 text-sm first:border-0">
              <span className="text-mute">{r.label}</span>
              <span className="max-w-[60%] truncate text-right font-medium text-ink">{r.value}</span>
            </div>
          ))}
          <div className="mt-3 flex items-center gap-2">
            <ConnectorIcon
              src={connectorIconSrc(item, s.type)}
              className="h-5 w-5 object-contain"
              boxClassName="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-white ring-1 ring-line"
            />
            <Badge tone={preview ? "warn" : s.status === "synced" ? "ok" : "neutral"}>{preview ? "Preview" : statusLabel(s.status)}</Badge>
            <span className="text-[12px] text-mute">{s.type}</span>
          </div>
        </Card>

        <Card className="space-y-3">
          <CardTitle>{isGuidedSQLType(s.type) ? "O que trazer" : "Sincronização"}</CardTitle>
          <FieldLabel label="Modo de armazenamento" hint="Importar copia os dados. Consulta directa marca o conjunto sem materializar (PostgreSQL/MySQL).">
            <Select value={storageMode} onChange={(e) => setStorageMode(e.target.value)}>
              <option value="import">Importar</option>
              <option value="direct_query">Consulta directa</option>
            </Select>
          </FieldLabel>
          {isGuidedSQLType(s.type) ? (
            <SqlDataPicker
              sourceId={id}
              sourceName={s.name}
              storageMode={storageMode}
              initialSelection={readSelection(cfg)}
              compact
              onDone={(dsId) => {
                if (dsId) setLastSyncedId(dsId);
                qc.invalidateQueries({ queryKey: ["source", id] });
                qc.invalidateQueries({ queryKey: ["datasets"] });
                qc.invalidateQueries({ queryKey: ["semantic"] });
              }}
            />
          ) : (
            <>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" onClick={discover}>
              Descobrir tabelas
            </Button>
            <Button onClick={() => sync()}>
              <Plug size={14} /> Sync
            </Button>
          </div>
          {discoverMsg && <p className="text-[12px] text-amber-800">{discoverMsg}</p>}
          {tables.length === 0 && !discoverMsg && <p className="text-[13px] text-mute">Ainda sem tabelas descobertas.</p>}
          <div className="flex flex-wrap gap-1.5">
            {tables.map((t) => (
              <button
                key={t}
                type="button"
                onClick={() => sync(t)}
                className="rounded-lg border border-line bg-white px-2 py-1 text-[12px] text-ink hover:border-primary/40 hover:bg-primary/5"
              >
                Sync {t}
              </button>
            ))}
          </div>
            </>
          )}
        </Card>
      </div>
      )}

      {!isManualType(s.type) && <AutoRefreshCard kind="connector" targetId={id} targetType={s.type} />}

      <Card>
        <CardTitle>Conjuntos gerados</CardTitle>
        {datasets.length === 0 && (
          <EmptyState icon={Unplug} title="Nenhum conjunto" description={isManualType(s.type) ? "Guarde as colunas e adicione a primeira linha no formulário." : "Sincronize uma tabela ou carregue um ficheiro para materializar dados."} />
        )}
        {datasets.map((d: any) => (
          <div key={d.id} className="flex items-center justify-between gap-3 border-t border-line py-2 text-sm first:border-0">
            <Link href={`/data/${d.id}`} className="font-medium text-accent hover:underline">
              {d.name}
            </Link>
            <div className="flex items-center gap-2">
              <span className="text-[12px] text-mute">
                {statusLabel(d.status)} · {d.storage_mode || "import"} · {d.row_count?.toLocaleString("pt-PT")} linhas
              </span>
              <Button size="sm" variant="ghost" onClick={() => openInDashboard(d.id, d.name)}>
                <LayoutDashboard size={12} /> Abrir no dashboard
              </Button>
            </div>
          </div>
        ))}
      </Card>
    </div>
  );
}

function str(v: unknown) {
  if (v == null || v === "" || v === 0 || v === false) return "";
  return String(v);
}

function readSelection(cfg: Record<string, unknown>): SourceSelection | null {
  const sel = cfg.selection;
  if (!sel || typeof sel !== "object") return null;
  const s = sel as SourceSelection;
  if (!Array.isArray(s.tables)) return null;
  return s;
}

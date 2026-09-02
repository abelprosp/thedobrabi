"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { toast } from "sonner";
import {
  Badge,
  Button,
  Card,
  CardTitle,
  EmptyState,
  ErrorState,
  FieldLabel,
  Input,
  PageHeader,
  PageSkeleton,
  Select,
  Textarea,
} from "@/components/ui";
import { Box, Copy, ExternalLink, Globe, LayoutDashboard, FileText, Rocket, Save } from "lucide-react";
import Link from "next/link";

type AppRecord = {
  id: string;
  name: string;
  description: string;
  status: "draft" | "published";
  theme?: string;
  cover_url?: string;
  public_token?: string;
  permissions?: { viewer: boolean; analyst: boolean };
};

type Ref = { id: string; name: string; order?: number; section?: string };
type Dash = { id: string; name: string; description?: string };
type Report = { id: string; name: string; cadence?: string };

type ContentItem = { id: string; section: string };

export default function AppEditorPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const qc = useQueryClient();

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [theme, setTheme] = useState("indigo");
  const [coverUrl, setCoverUrl] = useState("");
  const [viewer, setViewer] = useState(true);
  const [analyst, setAnalyst] = useState(true);
  const [dashboards, setDashboards] = useState<ContentItem[]>([]);
  const [reports, setReports] = useState<ContentItem[]>([]);
  const [hydrated, setHydrated] = useState(false);
  const [publicToken, setPublicToken] = useState<string | undefined>();
  const [status, setStatus] = useState<"draft" | "published">("draft");

  const q = useQuery({
    queryKey: ["app", id],
    queryFn: () =>
      api<{ app: AppRecord; dashboards: Ref[]; reports: Ref[] }>(`/api/v1/apps/${id}`),
  });
  const dashList = useQuery({ queryKey: ["dashboards"], queryFn: () => api<any>("/api/v1/dashboards") });
  const reportList = useQuery({ queryKey: ["reports"], queryFn: () => api<any>("/api/v1/reports") });
  const allDashboards = normalizeArray<Dash>(dashList.data);
  const allReports = normalizeArray<Report>(reportList.data);

  useEffect(() => {
    if (q.data && !hydrated) {
      const a = q.data.app;
      setName(a.name || "");
      setDescription(a.description || "");
      setTheme(a.theme || "indigo");
      setCoverUrl(a.cover_url || "");
      setViewer(a.permissions?.viewer !== false);
      setAnalyst(a.permissions?.analyst !== false);
      setPublicToken(a.public_token);
      setStatus(a.status || "draft");
      setDashboards((q.data.dashboards || []).map((d) => ({ id: d.id, section: d.section || "" })));
      setReports((q.data.reports || []).map((r) => ({ id: r.id, section: r.section || "" })));
      setHydrated(true);
    }
  }, [q.data, hydrated]);

  const publicUrl = useMemo(() => {
    if (!publicToken || typeof window === "undefined") return "";
    return `${window.location.origin}/apps/public/${publicToken}`;
  }, [publicToken]);

  const save = useMutation({
    mutationFn: async () => {
      await api(`/api/v1/apps/${id}`, {
        method: "PUT",
        body: JSON.stringify({
          name,
          description,
          theme,
          cover_url: coverUrl,
          status,
          permissions: { viewer, analyst },
        }),
      });
      await api(`/api/v1/apps/${id}/content`, {
        method: "POST",
        body: JSON.stringify({
          dashboards: dashboards.map((d) => ({ id: d.id, section: d.section })),
          reports: reports.map((r) => ({ id: r.id, section: r.section })),
        }),
      });
    },
    onSuccess: () => {
      toast.success("App guardada");
      qc.invalidateQueries({ queryKey: ["app", id] });
      qc.invalidateQueries({ queryKey: ["apps"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const publish = useMutation({
    mutationFn: async () => {
      await save.mutateAsync();
      return api<{ id: string; public_token: string; public_url: string }>(`/api/v1/apps/${id}/publish`, { method: "POST" });
    },
    onSuccess: (res) => {
      setPublicToken(res.public_token);
      setStatus("published");
      toast.success("App publicada");
      qc.invalidateQueries({ queryKey: ["app", id] });
      qc.invalidateQueries({ queryKey: ["apps"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  function toggleDash(did: string) {
    setDashboards((prev) => (prev.some((d) => d.id === did) ? prev.filter((d) => d.id !== did) : [...prev, { id: did, section: "" }]));
  }
  function toggleReport(rid: string) {
    setReports((prev) => (prev.some((r) => r.id === rid) ? prev.filter((r) => r.id !== rid) : [...prev, { id: rid, section: "" }]));
  }
  function setDashSection(did: string, section: string) {
    setDashboards((prev) => prev.map((d) => (d.id === did ? { ...d, section } : d)));
  }
  function setReportSection(rid: string, section: string) {
    setReports((prev) => prev.map((r) => (r.id === rid ? { ...r, section } : r)));
  }

  if (q.isError) return <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />;
  if (!q.data && !hydrated) return <PageSkeleton />;

  return (
    <div className="mx-auto max-w-5xl space-y-5">
      <PageHeader
        title={name || "App"}
        description="Agrupe dashboards e relatórios e publique um link para viewers."
        crumbs={[{ href: "/apps", label: "Apps" }]}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            {publicToken && (
              <Link href={`/apps/public/${publicToken}`} target="_blank">
                <Button variant="secondary">
                  <Globe size={14} /> Pré-visualizar
                </Button>
              </Link>
            )}
            <Button variant="secondary" onClick={() => publish.mutate()} busy={publish.isPending}>
              <Rocket size={14} /> {status === "published" ? "Republicar" : "Publicar"}
            </Button>
            <Button onClick={() => save.mutate()} busy={save.isPending}>
              <Save size={14} /> Guardar
            </Button>
          </div>
        }
      />

      <Card className="space-y-4">
        <CardTitle>Detalhes</CardTitle>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <FieldLabel label="Nome" required>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Nome da app" />
          </FieldLabel>
          <FieldLabel label="Tema">
            <Select value={theme} onChange={(e) => setTheme(e.target.value)}>
              <option value="indigo">Indigo</option>
              <option value="blue">Azul</option>
              <option value="slate">Ardósia</option>
            </Select>
          </FieldLabel>
        </div>
        <FieldLabel label="Descrição">
          <Textarea value={description} onChange={(e) => setDescription(e.target.value)} placeholder="O que esta app mostra aos viewers" />
        </FieldLabel>
        <FieldLabel label="URL da capa" hint="Opcional. Imagem de cabeçalho na página pública.">
          <Input value={coverUrl} onChange={(e) => setCoverUrl(e.target.value)} placeholder="https://..." />
        </FieldLabel>
        <div className="flex flex-wrap gap-4 text-[13px] text-ink">
          <label className="inline-flex items-center gap-2">
            <input type="checkbox" checked={viewer} onChange={(e) => setViewer(e.target.checked)} />
            Visível para viewers
          </label>
          <label className="inline-flex items-center gap-2">
            <input type="checkbox" checked={analyst} onChange={(e) => setAnalyst(e.target.checked)} />
            Visível para analistas
          </label>
        </div>
      </Card>

      {status === "published" && publicToken && (
        <Card className="space-y-3">
          <div className="flex items-center justify-between gap-3">
            <CardTitle>Link público</CardTitle>
            <Badge tone="ok">Publicada</Badge>
          </div>
          <div className="flex items-center gap-2 rounded-xl border border-line bg-surface-2 px-3 py-2">
            <input readOnly value={publicUrl} className="min-w-0 flex-1 bg-transparent text-[12px] text-mute outline-none" />
            <Button
              variant="ghost"
              size="icon"
              title="Copiar link"
              onClick={() => {
                navigator.clipboard.writeText(publicUrl);
                toast.success("Link copiado");
              }}
            >
              <Copy size={14} />
            </Button>
            <Link href={publicUrl} target="_blank">
              <Button variant="ghost" size="icon" title="Abrir">
                <ExternalLink size={14} />
              </Button>
            </Link>
          </div>
        </Card>
      )}

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card className="space-y-3">
          <div className="flex items-center justify-between">
            <CardTitle>Dashboards</CardTitle>
            <Badge>{dashboards.length} seleccionados</Badge>
          </div>
          {allDashboards.length === 0 ? (
            <EmptyState
              icon={LayoutDashboard}
              title="Sem dashboards"
              description="Crie um dashboard primeiro para o incluir nesta app."
              action={
                <Button variant="secondary" onClick={() => router.push("/dashboards")}>
                  Ir para dashboards
                </Button>
              }
            />
          ) : (
            <ul className="space-y-2">
              {allDashboards.map((d) => {
                const selected = dashboards.find((x) => x.id === d.id);
                return (
                  <li key={d.id} className="rounded-xl border border-line p-3">
                    <label className="flex items-start gap-2 text-[13px] text-ink">
                      <input type="checkbox" className="mt-0.5" checked={!!selected} onChange={() => toggleDash(d.id)} />
                      <span className="min-w-0">
                        <span className="block font-medium">{d.name}</span>
                        {d.description && <span className="block text-[12px] text-mute">{d.description}</span>}
                      </span>
                    </label>
                    {selected && (
                      <div className="mt-2 pl-6">
                        <Input
                          value={selected.section}
                          onChange={(e) => setDashSection(d.id, e.target.value)}
                          placeholder="Secção (ex.: Executivo)"
                        />
                      </div>
                    )}
                  </li>
                );
              })}
            </ul>
          )}
        </Card>

        <Card className="space-y-3">
          <div className="flex items-center justify-between">
            <CardTitle>Relatórios</CardTitle>
            <Badge>{reports.length} seleccionados</Badge>
          </div>
          {allReports.length === 0 ? (
            <EmptyState
              icon={FileText}
              title="Sem relatórios"
              description="Crie um relatório para o incluir nesta app."
              action={
                <Button variant="secondary" onClick={() => router.push("/reports")}>
                  Ir para relatórios
                </Button>
              }
            />
          ) : (
            <ul className="space-y-2">
              {allReports.map((r) => {
                const selected = reports.find((x) => x.id === r.id);
                return (
                  <li key={r.id} className="rounded-xl border border-line p-3">
                    <label className="flex items-start gap-2 text-[13px] text-ink">
                      <input type="checkbox" className="mt-0.5" checked={!!selected} onChange={() => toggleReport(r.id)} />
                      <span className="min-w-0">
                        <span className="block font-medium">{r.name}</span>
                        {r.cadence && <span className="block text-[12px] text-mute">{r.cadence}</span>}
                      </span>
                    </label>
                    {selected && (
                      <div className="mt-2 pl-6">
                        <Input
                          value={selected.section}
                          onChange={(e) => setReportSection(r.id, e.target.value)}
                          placeholder="Secção (ex.: Mensal)"
                        />
                      </div>
                    )}
                  </li>
                );
              })}
            </ul>
          )}
        </Card>
      </div>

      {dashboards.length === 0 && reports.length === 0 && (
        <EmptyState icon={Box} title="Nenhum conteúdo seleccionado" description="Escolha pelo menos um dashboard ou relatório antes de publicar." />
      )}
    </div>
  );
}

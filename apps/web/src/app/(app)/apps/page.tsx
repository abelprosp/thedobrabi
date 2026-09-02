"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, normalizeArray } from "@/lib/api";
import { toast } from "sonner";
import { Badge, Button, Card, EmptyState, ErrorState, FieldLabel, Input, PageHeader, PageSkeleton, Textarea } from "@/components/ui";
import { Box, Globe, ExternalLink, Trash2, Edit3, Rocket, Copy } from "lucide-react";
import Link from "next/link";

type App = {
  id: string;
  name: string;
  description: string;
  status: "draft" | "published";
  public_token?: string;
  theme?: string;
  cover_url?: string;
  permissions?: { viewer: boolean; analyst: boolean };
};

export default function AppsPage() {
  const qc = useQueryClient();
  const apps = useQuery({ queryKey: ["apps"], queryFn: () => api<any>("/api/v1/apps") });
  const appList = normalizeArray<App>(apps.data);
  const [name, setName] = useState("");
  const [desc, setDesc] = useState("");

  const create = useMutation({
    mutationFn: () => api<{ id: string }>("/api/v1/apps", { method: "POST", body: JSON.stringify({ name: name || "Nova app", description: desc }) }),
    onSuccess: (res) => {
      toast.success("App criada");
      setName("");
      setDesc("");
      qc.invalidateQueries({ queryKey: ["apps"] });
      window.location.href = `/apps/${res.id}`;
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const remove = useMutation({
    mutationFn: (id: string) => api(`/api/v1/apps/${id}`, { method: "DELETE" }),
    onSuccess: () => { toast.success("App removida"); qc.invalidateQueries({ queryKey: ["apps"] }); },
    onError: (e: Error) => toast.error(e.message),
  });

  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
  const [origin, setOrigin] = useState("");
  useEffect(() => { setOrigin(window.location.origin); }, []);

  return (
    <div className="mx-auto max-w-5xl space-y-5">
      <PageHeader
        title="Apps"
        description="Agrupe dashboards e relatórios em aplicações empacotadas para viewers."
        actions={
          <Button onClick={() => create.mutate()} busy={create.isPending}>
            <Rocket size={14} /> Nova app
          </Button>
        }
      />
      <Card className="space-y-3">
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <FieldLabel label="Nome"><Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Nome da app" /></FieldLabel>
          <FieldLabel label="Descrição"><Textarea value={desc} onChange={(e) => setDesc(e.target.value)} placeholder="Descrição pública" /></FieldLabel>
        </div>
      </Card>
      {apps.isLoading && <PageSkeleton cards={2} />}
      {apps.isError && <ErrorState message={(apps.error as Error).message} onRetry={() => apps.refetch()} />}
      {appList.length === 0 && !apps.isLoading && (
        <EmptyState icon={Box} title="Ainda sem apps" description="Crie a primeira app empacotada acima." />
      )}
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {appList.map((app) => (
          <Card key={app.id} className="flex flex-col gap-3">
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="truncate text-sm font-medium text-ink">{app.name}</div>
                <div className="mt-1 text-[12px] text-mute">{app.description || "Sem descrição"}</div>
                <div className="mt-2 flex flex-wrap items-center gap-2">
                  <Badge tone={app.status === "published" ? "ok" : "neutral"}>{app.status === "published" ? "Publicada" : "Rascunho"}</Badge>
                  {app.public_token && (
                    <Badge tone="accent">
                      <Globe size={10} className="mr-1" /> Pública
                    </Badge>
                  )}
                </div>
              </div>
              <div className="flex items-center gap-1">
                <Link href={`/apps/${app.id}`}><Button variant="ghost" size="icon" title="Editar"><Edit3 size={16} /></Button></Link>
                <Button
                  variant="ghost"
                  size="icon"
                  title="Abrir app"
                  onClick={() => window.open(app.public_token ? `/apps/public/${app.public_token}` : `/apps/${app.id}`, "_blank")}
                >
                  <ExternalLink size={16} />
                </Button>
                <Button variant="ghost" size="icon" title="Remover" onClick={() => setConfirmDelete(app.id)}><Trash2 size={16} /></Button>
              </div>
            </div>
            {app.public_token && (
              <div className="flex items-center gap-2 rounded-xl border border-line bg-surface-2 px-3 py-2">
                <input readOnly value={`${origin}/apps/public/${app.public_token}`} className="min-w-0 flex-1 bg-transparent text-[11px] text-mute outline-none" />
                <Button variant="ghost" size="icon" onClick={() => { navigator.clipboard.writeText(`${origin}/apps/public/${app.public_token}`); toast.success("Link copiado"); }}><Copy size={14} /></Button>
                <Link href={`/apps/public/${app.public_token}`} target="_blank"><Button variant="ghost" size="icon"><Globe size={14} /></Button></Link>
              </div>
            )}
            {confirmDelete === app.id && (
              <div className="rounded-xl border border-rose-100 bg-rose-50 p-3 text-[12px] text-rose-800">
                Tem a certeza?
                <div className="mt-2 flex gap-2">
                  <Button variant="danger" size="sm" onClick={() => remove.mutate(app.id)} busy={remove.isPending}>Apagar</Button>
                  <Button variant="secondary" size="sm" onClick={() => setConfirmDelete(null)}>Cancelar</Button>
                </div>
              </div>
            )}
            <div className="flex gap-2">
              <Link href={`/apps/${app.id}`} className="flex-1"><Button variant="secondary" className="w-full">Editar</Button></Link>
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}

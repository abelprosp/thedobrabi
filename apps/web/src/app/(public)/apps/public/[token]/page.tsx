"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { useParams } from "next/navigation";
import { Logo } from "@/components/brand";
import { Badge, Card, ErrorState, PageSkeleton } from "@/components/ui";
import { ExternalLink, FileText, LayoutDashboard } from "lucide-react";

type PublicDash = { id: string; name: string; description?: string; section?: string; public_url?: string };
type PublicReport = { id: string; name: string; section?: string; cadence?: string; last_generated_at?: string };
type PublicApp = {
  name: string;
  description: string;
  theme?: string;
  cover_url?: string;
  status?: string;
};

export default function PublicAppPage() {
  const { token } = useParams<{ token: string }>();
  const q = useQuery({
    queryKey: ["public-app", token],
    queryFn: () =>
      api<{ app: PublicApp; dashboards: PublicDash[]; reports: PublicReport[] }>(`/api/v1/apps/public/${token}`),
  });

  if (q.isError) return (
    <div className="mx-auto max-w-3xl p-8">
      <ErrorState message="Esta app não está publicada ou o link é inválido." />
    </div>
  );
  if (q.isLoading || !q.data) return <div className="p-8"><PageSkeleton /></div>;

  const app = q.data.app;
  const dashboards = q.data.dashboards || [];
  const reports = q.data.reports || [];
  const sections = Array.from(
    new Set([...dashboards.map((d) => d.section || ""), ...reports.map((r) => r.section || "")]),
  );

  return (
    <div className="min-h-screen bg-bg">
      {app.cover_url && (
        <div className="h-40 w-full overflow-hidden bg-surface-2">
          <img src={app.cover_url} alt="" className="h-full w-full object-cover" />
        </div>
      )}
      <div className="mx-auto max-w-5xl px-6 py-8">
        <div className="mb-6 flex items-center gap-2">
          <Logo variant="light" size={28} />
          <span className="text-sm text-mute">· app pública</span>
        </div>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h1 className="text-2xl font-semibold text-ink">{app.name}</h1>
            {app.description && <p className="mt-1 max-w-2xl text-sm text-mute">{app.description}</p>}
          </div>
          <Badge tone="ok">Publicada</Badge>
        </div>

        {dashboards.length === 0 && reports.length === 0 && (
          <Card className="mt-8 text-center text-sm text-mute">Esta app ainda não tem conteúdo.</Card>
        )}

        {(sections.length === 0 ? [""] : sections).map((section) => {
          const ds = dashboards.filter((d) => (d.section || "") === section);
          const rs = reports.filter((r) => (r.section || "") === section);
          if (ds.length === 0 && rs.length === 0) return null;
          return (
            <section key={section || "default"} className="mt-8 space-y-3">
              {section && <h2 className="text-sm font-medium uppercase tracking-wide text-mute">{section}</h2>}
              {ds.length > 0 && (
                <div className="grid gap-3 sm:grid-cols-2">
                  {ds.map((d) => (
                    <a
                      key={d.id}
                      href={d.public_url || "#"}
                      className="block rounded-2xl border border-line bg-surface p-5 shadow-sm transition hover:border-primary"
                    >
                      <div className="flex items-start justify-between gap-2">
                        <div className="flex items-center gap-2 text-ink">
                          <LayoutDashboard size={16} className="text-primary" />
                          <span className="text-sm font-medium">{d.name}</span>
                        </div>
                        <ExternalLink size={14} className="text-mute" />
                      </div>
                      {d.description && <p className="mt-2 text-[13px] text-mute">{d.description}</p>}
                      <p className="mt-3 text-[12px] text-primary">Abrir dashboard</p>
                    </a>
                  ))}
                </div>
              )}
              {rs.length > 0 && (
                <div className="grid gap-3 sm:grid-cols-2">
                  {rs.map((r) => (
                    <Card key={r.id} className="flex flex-col gap-2">
                      <div className="flex items-center gap-2 text-ink">
                        <FileText size={16} className="text-primary" />
                        <span className="text-sm font-medium">{r.name}</span>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        {r.cadence && <Badge>{r.cadence}</Badge>}
                        {r.last_generated_at && (
                          <span className="text-[11px] text-mute">
                            Gerado {new Date(r.last_generated_at).toLocaleString("pt-BR")}
                          </span>
                        )}
                      </div>
                    </Card>
                  ))}
                </div>
              )}
            </section>
          );
        })}
      </div>
    </div>
  );
}

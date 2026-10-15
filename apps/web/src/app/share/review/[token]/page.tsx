"use client";

import { useQuery } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import { Logo } from "@/components/brand";
import { ErrorState, PageSkeleton } from "@/components/ui";

type PublicAgenda = {
  period: string;
  created_at: string;
  next_review_at?: string;
  brand_name?: string;
  brand_logo_url?: string;
  agenda?: {
    results?: { title?: string; body?: string }[];
    prior_actions?: { title?: string; status?: string }[];
    priorities?: { title?: string; owner_name?: string }[];
    notes?: string;
  };
};

export default function PublicWeekReviewPage() {
  const { token } = useParams<{ token: string }>();
  const q = useQuery({
    queryKey: ["public-week-review", token],
    queryFn: () => api<PublicAgenda>(`/api/v1/public/week-review/${token}`),
    enabled: Boolean(token),
  });
  if (q.isLoading) return <PageSkeleton />;
  if (q.isError) return <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />;
  const d = q.data;
  const agenda = d?.agenda || {};
  return (
    <div className="mx-auto max-w-2xl px-4 py-10">
      <div className="mb-6 flex items-center gap-3">
        {d?.brand_logo_url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={d.brand_logo_url} alt="" className="h-8 w-auto" />
        ) : (
          <Logo />
        )}
        <div>
          <p className="text-[12px] text-mute">{d?.brand_name || "TheDobra"}</p>
          <h1 className="text-2xl font-bold tracking-tight text-ink">Revisão da semana · {d?.period}</h1>
        </div>
      </div>
      <section className="card-float space-y-4 p-6">
        <Block title="Resultados">
          {(agenda.results || []).length === 0 && <p className="text-sm text-mute">Sem pontos registados.</p>}
          {(agenda.results || []).map((r) => (
            <div key={r.title} className="py-2">
              <div className="text-sm font-medium text-ink">{r.title}</div>
              <p className="text-[13px] text-mute">{r.body}</p>
            </div>
          ))}
        </Block>
        <Block title="Acções revistas">
          {(agenda.prior_actions || []).map((a) => (
            <p key={a.title} className="text-sm text-ink">
              {a.title} <span className="text-mute">· {a.status}</span>
            </p>
          ))}
        </Block>
        <Block title="Prioridades">
          {(agenda.priorities || []).map((p) => (
            <p key={p.title} className="text-sm text-ink">
              {p.title}
              {p.owner_name ? ` · ${p.owner_name}` : ""}
            </p>
          ))}
        </Block>
        {agenda.notes && (
          <Block title="Notas">
            <p className="text-sm text-ink">{agenda.notes}</p>
          </Block>
        )}
        {d?.next_review_at && <p className="text-[13px] text-mute">Próxima revisão: {d.next_review_at}</p>}
      </section>
    </div>
  );
}

function Block({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h2 className="text-[13px] font-semibold uppercase tracking-wide text-mute">{title}</h2>
      <div className="mt-1">{children}</div>
    </div>
  );
}

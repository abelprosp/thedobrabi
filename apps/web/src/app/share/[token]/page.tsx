"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { useParams } from "next/navigation";
import { Chart, Kpi } from "@/components/viz";
import { Logo } from "@/components/brand";
import { ErrorState, PageSkeleton } from "@/components/ui";
import { ThemeSegmented } from "@/components/theme-toggle";
import { parseLayoutTheme, useTheme } from "@/components/theme-provider";
import { useEffect } from "react";

type Widget = {
  id: string;
  type: string;
  title: string;
  layout?: { w: number; h: number };
  text?: string;
  result?: { columns: string[]; rows: any[] };
  error?: string;
};

export default function SharePage() {
  const { token } = useParams<{ token: string }>();
  const { theme, setTheme } = useTheme();
  const q = useQuery({
    queryKey: ["public-dash", token],
    queryFn: () => api<{ name: string; description: string; layout: { widgets: Widget[]; theme?: string } }>(`/api/v1/public/dashboards/${token}`),
  });

  useEffect(() => {
    const saved = parseLayoutTheme(q.data?.layout);
    if (saved) setTheme(saved);
  }, [q.data, setTheme]);

  if (q.isError) return <ErrorState message="Partilha inválida ou expirada." />;
  if (q.isLoading || !q.data) return <div className="p-8"><PageSkeleton /></div>;

  const widgets = q.data.layout?.widgets || [];
  return (
    <div className="min-h-screen bg-bg p-8">
      <div className="mb-6 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Logo variant={theme === "dark" ? "dark" : "light"} size={28} />
          <span className="text-sm text-mute">· partilha</span>
        </div>
        <ThemeSegmented value={theme} onChange={setTheme} />
      </div>
      <h1 className="text-2xl font-semibold text-ink">{q.data.name}</h1>
      {q.data.description && <p className="mt-1 text-sm text-mute">{q.data.description}</p>}
      <div className="mt-6 grid gap-4 md:grid-cols-2">
        {widgets.map((w) => (
          <PublicWidget key={w.id} w={w} />
        ))}
      </div>
    </div>
  );
}

function PublicWidget({ w }: { w: Widget }) {
  const rows = w.result?.rows || [];
  const columns = w.result?.columns || [];
  if (w.type === "text") {
    return <div className="rounded-2xl border border-line bg-surface p-4 text-sm shadow-sm">{w.text || w.title}</div>;
  }
  if (w.error) {
    return (
      <div className="rounded-2xl border border-line bg-surface p-4 text-sm text-danger shadow-sm">
        {w.title}: {w.error}
      </div>
    );
  }
  if (w.type === "kpi") {
    const val = rows[0] ? Number(Object.values(rows[0])[0] ?? 0) : 0;
    return <Kpi label={w.title} value={val.toLocaleString("pt-BR", { maximumFractionDigits: 0 })} />;
  }
  if (w.type === "table") {
    return (
      <div className="overflow-auto rounded-2xl border border-line bg-surface p-4 shadow-sm">
        <div className="mb-2 text-[13px] text-mute">{w.title}</div>
        <table className="w-full text-left text-[12px]">
          <thead>
            <tr className="text-mute">
              {columns.map((c) => (
                <th key={c} className="py-1 pr-3">
                  {c}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.slice(0, 12).map((r, i) => (
              <tr key={i} className="border-t border-line">
                {columns.map((c) => (
                  <td key={c} className="py-1 pr-3">
                    {String(r[c] ?? "")}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  }
  return (
    <div className="rounded-2xl border border-line bg-surface p-3 shadow-sm">
      <Chart
        type={w.type === "line" || w.type === "area" ? w.type : w.type === "pie" ? "pie" : "bar"}
        title={w.title}
        columns={columns}
        rows={rows}
        height={240}
      />
    </div>
  );
}

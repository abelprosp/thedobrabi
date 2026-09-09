"use client";

import { formatNumber, titleAlignClass } from "@/lib/widget-config";
import { cn } from "@/lib/cn";
import { formatCategory } from "@/components/viz";

type Rows = Record<string, any>[];

const MEDAL = ["#F59E0B", "#94A3B8", "#B45309"];

export function RankingCard({
  title,
  rows = [],
  columns = [],
  measures = [],
  config = {},
  showTitle = true,
  onSelect,
}: {
  title: string;
  rows?: Rows;
  columns?: string[];
  measures?: string[];
  config?: Record<string, any>;
  showTitle?: boolean;
  onSelect?: (dimension: string, value: string) => void;
}) {
  const stringDim = columns.find((c) => typeof rows[0]?.[c] === "string") || columns[0];
  const dim = config.dimension || stringDim;
  const numeric = columns.filter((c) => c !== dim && typeof rows[0]?.[c] === "number");
  const measure =
    (measures || []).find((m) => columns.includes(m) || Object.keys(rows[0] || {}).includes(m)) ||
    config.measure ||
    numeric[0] ||
    columns.find((c) => c !== dim) ||
    columns[0];
  const extras = (measures || []).filter((m) => m !== measure).filter((m) => m in (rows[0] || {}) || columns.includes(m));
  const dir = config.rankOrder === "asc" ? 1 : -1;
  const limit = Math.max(3, Math.min(50, Number(config.rankLimit || 10)));
  const sorted = [...rows]
    .sort((a, b) => (Number(a[measure] ?? 0) - Number(b[measure] ?? 0)) * dir)
    .slice(0, limit);
  const max = Math.max(1, ...sorted.map((r) => Math.abs(Number(r[measure] ?? 0))));
  const color = config.color || "#2563EB";
  const align = config.titleAlign as "left" | "center" | "right" | undefined;

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface shadow-sm">
      {showTitle && (
        <div className={cn("shrink-0 border-b border-line px-4 py-2.5 text-[13px] font-medium text-ink", titleAlignClass(align))}>
          {title}
        </div>
      )}
      <div className="min-h-0 flex-1 overflow-auto px-3 py-2">
        {sorted.length === 0 ? (
          <p className="px-1 py-6 text-center text-[12px] text-mute">Sem linhas para o ranking neste recorte.</p>
        ) : (
          <ol className="space-y-1.5">
            {sorted.map((row, i) => {
              const raw = String(row[dim] ?? "");
              const label = formatCategory(raw) || "—";
              const val = Number(row[measure] ?? 0);
              const width = Math.max(4, Math.round((Math.abs(val) / max) * 100));
              const medal = MEDAL[i];
              return (
                <li key={`${raw}-${i}`}>
                  <button
                    type="button"
                    className="flex w-full items-center gap-2.5 rounded-xl px-1 py-1 text-left hover:bg-surface-2"
                    onClick={() => dim && raw && onSelect?.(dim, raw)}
                  >
                    <span
                      className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-[11px] font-semibold"
                      style={
                        medal
                          ? { backgroundColor: `${medal}22`, color: medal }
                          : { backgroundColor: "transparent", color: "inherit" }
                      }
                    >
                      <span className={medal ? "" : "text-mute"}>{i + 1}</span>
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-baseline justify-between gap-2">
                        <span className="truncate text-[13px] font-medium text-ink">{label}</span>
                        <span className="shrink-0 tabular-nums text-[13px] font-semibold text-ink">{formatNumber(val, config)}</span>
                      </div>
                      <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-surface-2">
                        <div className="h-full rounded-full" style={{ width: `${width}%`, backgroundColor: medal || color }} />
                      </div>
                      {extras.length > 0 && (
                        <div className="mt-0.5 flex flex-wrap gap-x-2 text-[11px] text-mute">
                          {extras.map((m) => (
                            <span key={m}>
                              {m}: {formatNumber(Number(row[m] ?? 0), config)}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                  </button>
                </li>
              );
            })}
          </ol>
        )}
      </div>
    </div>
  );
}

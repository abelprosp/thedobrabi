"use client";

import { useId, useMemo } from "react";
import { chartChrome, formatAxisTick, formatNumber, hexToRgba } from "@/lib/widget-config";
import { chartTooltip } from "@/lib/chartjs";
import { ChartJsCanvas } from "@/components/chartjs-canvas";
import { formatCategory, pivotSeries } from "@/components/viz";
import { useTheme } from "@/components/theme-provider";
import { cn } from "@/lib/cn";

type Rows = Record<string, any>[];
type Cfg = Record<string, any>;

const RAINBOW = ["#EF4444", "#F97316", "#F59E0B", "#84CC16", "#10B981", "#06B6D4", "#6366F1", "#8B5CF6"];

function numCols(rows: Rows, columns: string[]) {
  return columns.filter((c) => rows.length && typeof rows[0][c] === "number");
}
function strCols(rows: Rows, columns: string[]) {
  return columns.filter((c) => rows.length && typeof rows[0][c] === "string");
}
function measureOf(rows: Rows, columns: string[], cfg: Cfg) {
  return cfg.measure || numCols(rows, columns)[0] || columns[1] || columns[0];
}
function dimOf(rows: Rows, columns: string[], cfg: Cfg) {
  return cfg.dimension || strCols(rows, columns)[0] || columns[0];
}
function pctDelta(curr: number, prev?: number) {
  if (prev == null || !Number.isFinite(prev) || prev === 0) return curr ? 100 : 0;
  return ((curr - prev) / Math.abs(prev)) * 100;
}
function Delta({ v }: { v: number }) {
  const ok = v >= 0;
  return (
    <span className={cn("inline-flex rounded-md px-1.5 py-0.5 text-[11px] font-semibold", ok ? "bg-emerald-50 text-emerald-600" : "bg-red-50 text-danger")}>
      {ok ? "+" : ""}
      {v.toFixed(1)}%
    </span>
  );
}

function EmptyViz() {
  return <div className="flex h-full min-h-[6rem] items-center justify-center text-[12px] text-mute">Sem dados para este visual.</div>;
}

function seriesByDim(rows: Rows, dim: string, meas: string) {
  const order: string[] = [];
  const map = new Map<string, number>();
  for (const r of rows) {
    const k = String(r[dim] ?? "");
    if (!map.has(k)) order.push(k);
    map.set(k, (map.get(k) || 0) + Number(r[meas] ?? 0));
  }
  return { labels: order.map((k) => formatCategory(k)), raw: order, values: order.map((k) => map.get(k) || 0) };
}

/** Last n periods vs the n periods immediately before — only when those windows exist in the data. */
function trailingWindows(values: number[]) {
  const sum = (n: number, offset: number) => {
    const end = values.length - offset;
    const start = end - n;
    if (start < 0 || end <= 0) return null;
    return values.slice(start, end).reduce((s, v) => s + v, 0);
  };
  const out: { label: string; value: number; prev?: number }[] = [];
  const last = sum(1, 0);
  const prev1 = sum(1, 1);
  if (last != null) out.push({ label: "Último período", value: last, prev: prev1 ?? undefined });
  if (values.length >= 6) {
    const a = sum(3, 0);
    const b = sum(3, 3);
    if (a != null) out.push({ label: "3 períodos", value: a, prev: b ?? undefined });
  }
  if (values.length >= 24) {
    const a = sum(12, 0);
    const b = sum(12, 12);
    if (a != null) out.push({ label: "12 períodos", value: a, prev: b ?? undefined });
  }
  return out;
}

function measureKpis(rows: Rows, columns: string[], cfg: Cfg) {
  const measures = numCols(rows, columns);
  const dim = dimOf(rows, columns, cfg);
  const timeish = strCols(rows, columns).includes(dim) && rows.length > 1;
  if (measures.length >= 2) {
    return measures.slice(0, 3).map((m) => {
      const s = seriesByDim(rows, dim, m);
      const last = s.values[s.values.length - 1] || 0;
      const prev = s.values.length > 1 ? s.values[s.values.length - 2] : undefined;
      return { label: m, value: last, prev };
    });
  }
  const meas = measures[0] || measureOf(rows, columns, cfg);
  const s = seriesByDim(rows, dim, meas);
  if (timeish && s.values.length > 1) {
    return trailingWindows(s.values).map((w) => ({ ...w, last: w.value }));
  }
  const last = s.values[s.values.length - 1] || 0;
  const prev = s.values.length > 1 ? s.values[s.values.length - 2] : undefined;
  return [{ label: meas || "Valor", value: last, prev }];
}

export function StatSparkCard({
  title,
  rows = [],
  columns = [],
  config = {},
}: {
  title: string;
  rows?: Rows;
  columns?: string[];
  config?: Cfg;
}) {
  const { theme } = useTheme();
  const meas = measureOf(rows, columns, config);
  const dim = dimOf(rows, columns, config);
  const s = seriesByDim(rows, dim, meas);
  const last = s.values[s.values.length - 1] || 0;
  const prev = s.values.length > 1 ? s.values[s.values.length - 2] : undefined;
  const avg = s.values.length ? s.values.reduce((n, v) => n + v, 0) / s.values.length : 0;
  const color = config.color || "#8B5CF6";
  const candle = config.sparkStyle === "candle";
  if (!rows.length) return <div className="rounded-2xl border border-line bg-surface p-4 shadow-sm"><EmptyViz /></div>;

  return (
    <div className="flex h-full min-h-0 items-stretch gap-3 overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
      <div className="min-w-0 flex-1">
        {config.showTitle !== false && <div className="text-[13px] font-medium text-ink">{title}</div>}
        <div className="mt-1 flex items-baseline gap-2">
          <span className="text-2xl font-semibold tracking-tight text-ink">{formatNumber(last, config)}</span>
          {prev != null && <Delta v={pctDelta(last, prev)} />}
        </div>
        <p className="mt-1 text-[11px] text-mute">Média {formatNumber(avg, config)}</p>
      </div>
      <div className="h-[4.5rem] w-[7.5rem] shrink-0">
        {candle ? (
          <MiniCandles values={s.values} />
        ) : (
          <ChartJsCanvas
            type="line"
            data={{
              labels: s.labels,
              datasets: [{ data: s.values, borderColor: color, backgroundColor: hexToRgba(color, 0.28), fill: true, tension: 0.45, pointRadius: 0, borderWidth: 2 }],
            }}
            options={{
              plugins: { legend: { display: false }, tooltip: { ...chartTooltip(theme), callbacks: { label: (c: any) => formatNumber(c.parsed.y, config) } } },
              scales: { x: { display: false }, y: { display: false } },
            }}
          />
        )}
      </div>
    </div>
  );
}

function MiniCandles({ values }: { values: number[] }) {
  const slice = values.slice(-10);
  const min = Math.min(...slice, 0);
  const max = Math.max(...slice, 1);
  const span = max - min || 1;
  return (
    <svg viewBox="0 0 120 72" className="h-full w-full">
      {slice.map((v, i) => {
        const open = slice[i - 1] ?? v;
        const high = Math.max(open, v);
        const low = Math.min(open, v);
        const x = 8 + i * 11;
        const yHigh = 64 - ((high - min) / span) * 52;
        const yLow = 64 - ((low - min) / span) * 52;
        const yOpen = 64 - ((open - min) / span) * 52;
        const yClose = 64 - ((v - min) / span) * 52;
        const up = v >= open;
        const fill = up ? "#3B82F6" : "#F43F5E";
        return (
          <g key={i}>
            <line x1={x} x2={x} y1={yHigh} y2={yLow} stroke={fill} strokeWidth="1" />
            <rect x={x - 3} y={Math.min(yOpen, yClose)} width="6" height={Math.max(2, Math.abs(yClose - yOpen))} rx="1" fill={fill} />
          </g>
        );
      })}
    </svg>
  );
}

export function RadarCard({ title, rows = [], columns = [], config = {} }: { title: string; rows?: Rows; columns?: string[]; config?: Cfg }) {
  const { theme } = useTheme();
  const chrome = chartChrome(theme);
  const meas = measureOf(rows, columns, config);
  const dim = dimOf(rows, columns, config);
  const s = seriesByDim(rows, dim, meas);
  const labels = s.labels.slice(0, 12);
  const data = s.values.slice(0, 12);
  const last = data[data.length - 1] || 0;
  const prev = data.length > 1 ? data[data.length - 2] : undefined;
  const color = config.color || "#8B5CF6";
  if (!data.length) return <div className="rounded-2xl border border-line bg-surface p-4 shadow-sm"><EmptyViz /></div>;
  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
      <HeaderBlock title={title} value={last} prev={prev} config={config} showTitle={config.showTitle !== false} />
      <div className="min-h-0 flex-1">
        <ChartJsCanvas
          type="radar"
          data={{
            labels,
            datasets: [{ label: meas, data, backgroundColor: hexToRgba(color, 0.28), borderColor: color, borderWidth: 2, pointBackgroundColor: color, pointRadius: 3 }],
          }}
          options={{
            plugins: { legend: { display: false }, tooltip: chartTooltip(theme) },
            scales: {
              r: {
                ticks: { display: false, backdropColor: "transparent" },
                grid: { color: chrome.line },
                angleLines: { color: chrome.line },
                pointLabels: { color: chrome.mute, font: { size: 10 } },
              },
            },
          }}
        />
      </div>
    </div>
  );
}

function ridgeGroups(rows: Rows, columns: string[], cfg: Cfg) {
  const meas = measureOf(rows, columns, cfg);
  const dim = dimOf(rows, columns, cfg);
  const seriesCol = strCols(rows, columns).find((c) => c !== dim);
  if (!seriesCol) {
    const s = seriesByDim(rows, dim, meas);
    return { labels: s.labels, groups: [{ name: meas, data: s.values }] };
  }
  const pivoted = pivotSeries(rows, dim, seriesCol, meas);
  return {
    labels: pivoted.cats.map((c) => formatCategory(c)),
    groups: pivoted.series.slice(0, 6).map((name, i) => ({ name, data: pivoted.values[i] || [] })),
  };
}

export function RidgelineCard({ title, rows = [], columns = [], config = {} }: { title: string; rows?: Rows; columns?: string[]; config?: Cfg }) {
  const { theme } = useTheme();
  const chrome = chartChrome(theme);
  const { labels, groups } = useMemo(() => ridgeGroups(rows, columns, config), [rows, columns, config]);
  const last = groups[0]?.data[groups[0].data.length - 1] || 0;
  const prev = (groups[0]?.data.length || 0) > 1 ? groups[0].data[groups[0].data.length - 2] : undefined;
  const mirrored = config.mirrored !== false;
  if (!groups.length || !labels.length) return <div className="rounded-2xl border border-line bg-surface p-4 shadow-sm"><EmptyViz /></div>;
  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
      <HeaderBlock title={title} value={last} prev={prev} config={config} showTitle={config.showTitle !== false} />
      <div className="min-h-0 flex-1">
        <RidgePlot labels={labels} groups={groups} mirrored={mirrored} config={config} chrome={chrome} theme={theme} />
      </div>
    </div>
  );
}

function RidgePlot({
  labels,
  groups,
  mirrored,
  config,
  chrome,
  theme,
}: {
  labels: string[];
  groups: { name: string; data: number[] }[];
  mirrored: boolean;
  config: Cfg;
  chrome: ReturnType<typeof chartChrome>;
  theme: "light" | "dark" | undefined;
}) {
  return (
    <ChartJsCanvas
      type="line"
      data={{
        labels,
        datasets: groups.flatMap((g, i) => {
          const c = RAINBOW[i % RAINBOW.length];
          const up = { label: g.name, data: g.data, borderColor: c, backgroundColor: hexToRgba(c, 0.22), fill: true, tension: 0.45, pointRadius: 0, borderWidth: 2 };
          if (!mirrored) return [up];
          return [up, { ...up, label: `${g.name} (espelho)`, data: g.data.map((n) => -n) }];
        }),
      }}
      options={{
        plugins: { legend: { display: false }, tooltip: { ...chartTooltip(theme), callbacks: { label: (c: any) => `${c.dataset.label}: ${formatNumber(Math.abs(Number(c.parsed?.y ?? 0)), config)}` } } },
        scales: {
          x: { ticks: { color: chrome.mute, maxRotation: 0 }, grid: { display: false }, border: { display: false } },
          y: { ticks: { color: chrome.mute, callback: (v: any) => formatAxisTick(Math.abs(Number(v)), config) }, grid: { color: chrome.line }, border: { display: false } },
        },
      }}
    />
  );
}

export function SankeyCard({ title, rows = [], columns = [], config = {} }: { title: string; rows?: Rows; columns?: string[]; config?: Cfg }) {
  const gid = useId().replace(/:/g, "");
  const dims = strCols(rows, columns);
  const meas = measureOf(rows, columns, config);
  const leftKey = dims[0] || columns[0];
  const rightKey = dims[1] || "";
  const pairs = useMemo(() => {
    if (!rightKey || rightKey === leftKey) return [];
    const map = new Map<string, { left: string; right: string; value: number }>();
    for (const r of rows) {
      const left = formatCategory(r[leftKey] ?? "Origem");
      const right = formatCategory(r[rightKey] ?? "Destino");
      const k = `${left}\0${right}`;
      map.set(k, { left, right, value: (map.get(k)?.value || 0) + Number(r[meas] ?? 0) });
    }
    return Array.from(map.values()).sort((a, b) => b.value - a.value).slice(0, 16);
  }, [rows, leftKey, rightKey, meas]);
  const left = summarize(rows, leftKey, meas).slice(0, 6);
  const right = rightKey ? summarize(rows, rightKey, meas).slice(0, 6) : [];
  const last = left.reduce((s, x) => s + x.value, 0);
  const maxL = Math.max(...left.map((x) => x.value), 1);
  const maxR = Math.max(...right.map((x) => x.value), 1);
  const h = 160;
  const leftBars = layoutBars(left, maxL, h, 18);
  const rightBars = layoutBars(right, maxR, h, 240);
  const leftPos = new Map(leftBars.map((b) => [b.item.name, b]));
  const rightPos = new Map(rightBars.map((b) => [b.item.name, b]));
  const maxFlow = Math.max(...pairs.map((p) => p.value), 1);

  if (!rows.length) return <div className="rounded-2xl border border-line bg-surface p-4 shadow-sm"><EmptyViz /></div>;

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
      {config.showTitle !== false && <div className="text-[13px] font-medium text-ink">{title}</div>}
      <div className="min-h-0 flex-1">
        {!rightKey || rightKey === leftKey ? (
          <p className="p-2 text-[12px] text-mute">Defina duas dimensões (origem e destino) para o fluxo. A mostrar totais da dimensão actual.</p>
        ) : null}
        <svg viewBox="0 0 280 170" className="h-full w-full">
          <defs>
            <linearGradient id={`flow-${gid}`} x1="0" x2="1">
              <stop offset="0" stopColor="#38bdf8" />
              <stop offset="0.5" stopColor="#818cf8" />
              <stop offset="1" stopColor="#34d399" />
            </linearGradient>
          </defs>
          {pairs.map((p, i) => {
            const a = leftPos.get(p.left);
            const b = rightPos.get(p.right);
            if (!a || !b) return null;
            const d = `M ${a.x + 10} ${a.y + a.h / 2} C 110 ${a.y + a.h / 2}, 150 ${b.y + b.h / 2}, ${b.x} ${b.y + b.h / 2}`;
            return <path key={i} d={d} fill="none" stroke={`url(#flow-${gid})`} strokeWidth={Math.max(2, (p.value / maxFlow) * 18)} opacity="0.45" />;
          })}
          {leftBars.map((b) => (
            <g key={`l-${b.item.name}`}>
              <rect x={b.x} y={b.y} width="10" height={b.h} rx="3" fill="#22c55e" />
              <text x={b.x + 16} y={b.y + b.h / 2 + 4} fontSize="10" fill="#64748b">{b.item.name.slice(0, 12)} {formatNumber(b.item.value, { ...config, compact: "auto" })}</text>
            </g>
          ))}
          {rightBars.map((b) => (
            <g key={`r-${b.item.name}`}>
              <rect x={b.x} y={b.y} width="10" height={b.h} rx="3" fill="#22c55e" />
              <text x={b.x - 4} y={b.y + b.h / 2 + 4} fontSize="10" fill="#64748b" textAnchor="end">{formatNumber(b.item.value, { ...config, compact: "auto" })}</text>
            </g>
          ))}
        </svg>
      </div>
      <div className="mt-1">
        <div className="text-[11px] text-mute">Total</div>
        <div className="flex items-baseline gap-2">
          <span className="text-xl font-semibold text-ink">{formatNumber(last, config)}</span>
        </div>
      </div>
    </div>
  );
}

export function SalesReportCard({ title, rows = [], columns = [], config = {} }: { title: string; rows?: Rows; columns?: string[]; config?: Cfg }) {
  const { theme } = useTheme();
  const chrome = chartChrome(theme);
  const dim = dimOf(rows, columns, config);
  const measures = numCols(rows, columns);
  const kpis = measureKpis(rows, columns, config);
  const cols = measures.length ? measures : [measureOf(rows, columns, config)];
  const tableRows = summarizeMany(rows, dim, cols).slice(0, 8);
  const ridge = ridgeGroups(rows, columns, config);
  if (!rows.length) return <div className="rounded-2xl border border-line bg-surface p-4 shadow-sm"><EmptyViz /></div>;

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
      {config.showTitle !== false && <div className="text-[15px] font-semibold text-ink">{title || "Relatório"}</div>}
      <div className={cn("mt-3 grid gap-2", kpis.length === 1 ? "grid-cols-1" : kpis.length === 2 ? "grid-cols-2" : "grid-cols-3")}>
        {kpis.map((k) => (
          <div key={k.label}>
            <div className="text-[11px] text-mute">{k.label}</div>
            <div className="mt-0.5 flex flex-wrap items-baseline gap-1.5">
              <span className="text-lg font-semibold text-ink">{formatNumber(k.value, config)}</span>
              {k.prev != null && <Delta v={pctDelta(k.value, k.prev)} />}
            </div>
            {k.prev != null && (
              <p className="mt-0.5 text-[10px] text-mute">Comparado a {formatNumber(k.prev, config)} no período anterior</p>
            )}
          </div>
        ))}
      </div>
      <div className="mt-2 min-h-0 flex-1">
        {ridge.labels.length ? (
          <RidgePlot labels={ridge.labels} groups={ridge.groups} mirrored={config.mirrored !== false} config={config} chrome={chrome} theme={theme} />
        ) : (
          <EmptyViz />
        )}
      </div>
      <div className="mt-2 space-y-1 border-t border-line pt-2">
        {tableRows.map((r) => (
          <div key={r.name} className={cn("grid gap-2 text-[12px]", `grid-cols-${Math.min(cols.length + 1, 4)}`)} style={{ gridTemplateColumns: `minmax(0,1.4fr) repeat(${cols.length}, minmax(0,1fr))` }}>
            <span className="truncate text-ink">{r.name}</span>
            {r.values.map((v, i) => (
              <span key={cols[i] || i} className="text-right tabular-nums text-mute">{formatNumber(v, config)}</span>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}

export function NetworkSalesCard({ title, rows = [], columns = [], config = {} }: { title: string; rows?: Rows; columns?: string[]; config?: Cfg }) {
  const dims = strCols(rows, columns);
  const measures = numCols(rows, columns);
  const meas = measures[0] || measureOf(rows, columns, config);
  const dim = dims[0] || dimOf(rows, columns, config);
  const kpis = measureKpis(rows, columns, config);
  const tableRows = summarizeMany(rows, dim, measures.length ? measures : [meas]).slice(0, 6);
  const left = summarize(rows, dims[0] || dim, meas).slice(0, 4);
  const right = dims[1] ? summarize(rows, dims[1], meas).slice(0, 4) : [];
  const pairs = dims[1]
    ? (() => {
        const map = new Map<string, number>();
        for (const r of rows) {
          const a = formatCategory(r[dims[0]]);
          const b = formatCategory(r[dims[1]]);
          map.set(`${a}\0${b}`, (map.get(`${a}\0${b}`) || 0) + Number(r[meas] ?? 0));
        }
        return Array.from(map.entries()).map(([k, value]) => {
          const [a, b] = k.split("\0");
          return { a, b, value };
        });
      })()
    : [];
  if (!rows.length) return <div className="rounded-2xl border border-line bg-surface p-4 shadow-sm"><EmptyViz /></div>;

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
      {config.showTitle !== false && <div className="text-[15px] font-semibold text-ink">{title}</div>}
      <div className={cn("mt-2 grid gap-2", kpis.length >= 3 ? "grid-cols-3" : kpis.length === 2 ? "grid-cols-2" : "grid-cols-1")}>
        {kpis.slice(0, 3).map((k) => (
          <div key={k.label}>
            <div className="text-[11px] text-mute">{k.label}</div>
            <div className="flex items-baseline gap-1">
              <span className="text-base font-semibold">{formatNumber(k.value, config)}</span>
              {k.prev != null && <Delta v={pctDelta(k.value, k.prev)} />}
            </div>
          </div>
        ))}
      </div>
      <svg viewBox="0 0 280 120" className="mx-auto my-2 h-28 w-full max-w-[280px]">
        {pairs.slice(0, 12).map((p, i) => {
          const li = Math.max(0, left.findIndex((x) => x.name === p.a));
          const ri = Math.max(0, right.findIndex((x) => x.name === p.b));
          const y1 = 18 + li * 24;
          const y2 = 18 + ri * 24;
          return <line key={i} x1="70" y1={y1} x2="210" y2={y2} stroke="#c4b5fd" strokeWidth={Math.max(1, (p.value / Math.max(...pairs.map((x) => x.value), 1)) * 6)} />;
        })}
        {left.map((n, i) => (
          <g key={`l-${n.name}`}>
            <circle cx="48" cy={18 + i * 24} r="10" fill="#ede9fe" />
            <text x="8" y={22 + i * 24} fontSize="9" fill="#64748b">{n.name.slice(0, 10)}</text>
          </g>
        ))}
        {right.map((n, i) => (
          <g key={`r-${n.name}`}>
            <circle cx="232" cy={18 + i * 24} r="10" fill="#ede9fe" />
            <text x="246" y={22 + i * 24} fontSize="9" fill="#64748b">{n.name.slice(0, 10)}</text>
          </g>
        ))}
      </svg>
      <div className="space-y-1 border-t border-line pt-2">
        {tableRows.map((r) => (
          <div key={r.name} className="grid gap-2 text-[11px]" style={{ gridTemplateColumns: `minmax(0,1.3fr) repeat(${r.values.length}, minmax(0,1fr))` }}>
            <span className="truncate text-ink">{r.name}</span>
            {r.values.map((v, i) => (
              <span key={i} className="text-right tabular-nums text-mute">{formatNumber(v, config)}</span>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}

export function BubbleCard({
  title,
  rows = [],
  columns = [],
  config = {},
  measures,
}: {
  title: string;
  rows?: Rows;
  columns?: string[];
  config?: Cfg;
  measures?: string[];
}) {
  const { theme } = useTheme();
  const chrome = chartChrome(theme);
  const nums = numCols(rows, columns);
  const requested = (measures || []).filter((m) => m && columns.includes(m));
  const xCol =
    (config.xMeasure && columns.includes(config.xMeasure) ? config.xMeasure : "") || requested[0] || nums[0] || "";
  const yCol =
    (config.yMeasure && columns.includes(config.yMeasure) && config.yMeasure !== xCol ? config.yMeasure : "") ||
    requested.find((m) => m !== xCol) ||
    nums.find((c) => c !== xCol) ||
    "";
  const xyMode = Boolean(xCol && yCol && nums.includes(xCol) && nums.includes(yCol));
  const sizeCandidate =
    (config.measure && columns.includes(config.measure) ? config.measure : "") || requested[2] || "";
  const sizeCol = sizeCandidate && nums.includes(sizeCandidate) ? sizeCandidate : yCol || measureOf(rows, columns, config);
  const dim =
    (config.dimension && columns.includes(config.dimension) ? config.dimension : "") || strCols(rows, columns)[0] || "";
  const color = config.color || "#F97316";
  const kpis = measureKpis(rows, columns, config);
  if (!rows.length) return <div className="rounded-2xl border border-line bg-surface p-4 shadow-sm"><EmptyViz /></div>;

  if (xyMode) {
    const maxSize = Math.max(...rows.map((r) => Math.abs(Number(r[sizeCol] ?? 0))), 1);
    const points = rows.map((r) => ({
      x: Number(r[xCol] ?? 0),
      y: Number(r[yCol] ?? 0),
      r: 6 + (Math.abs(Number(r[sizeCol] ?? 0)) / maxSize) * 22,
      v: Number(r[sizeCol] ?? 0),
      label: dim ? formatCategory(r[dim]) : "",
    }));
    const cats = dim ? summarize(rows, dim, sizeCol).slice(0, 5) : [];
    return (
      <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
        {config.showTitle !== false && <div className="text-[13px] font-medium text-ink">{title}</div>}
        <div className="min-h-0 flex-1">
          <ChartJsCanvas
            type="bubble"
            data={{
              datasets: [{
                label: yCol,
                data: points,
                backgroundColor: hexToRgba(color, 0.55),
                borderColor: color,
              }],
            }}
            options={{
              plugins: {
                legend: { display: false },
                tooltip: {
                  ...chartTooltip(theme),
                  callbacks: {
                    label: (ctx: any) => {
                      const p = ctx.raw as { x: number; y: number; v: number; label?: string };
                      const bits = [
                        p.label,
                        `${xCol}: ${formatNumber(p.x, config)}`,
                        `${yCol}: ${formatNumber(p.y, config)}`,
                      ];
                      if (sizeCol && sizeCol !== xCol && sizeCol !== yCol) bits.push(`${sizeCol}: ${formatNumber(p.v, config)}`);
                      return bits.filter(Boolean).join(" · ");
                    },
                  },
                },
              },
              scales: {
                x: {
                  title: { display: true, text: config.xAxisLabel || xCol, color: chrome.mute },
                  ticks: { color: chrome.mute, callback: (v: any) => formatAxisTick(v, config) },
                  grid: { color: chrome.line, display: config.showGrid !== false },
                  border: { display: false },
                },
                y: {
                  title: { display: true, text: config.yAxisLabel || yCol, color: chrome.mute },
                  ticks: { color: chrome.mute, callback: (v: any) => formatAxisTick(v, config) },
                  grid: { color: chrome.line, display: config.showGrid !== false },
                  border: { display: false },
                },
              },
            }}
          />
        </div>
        <div className="mt-2 grid grid-cols-2 gap-2 text-[12px]">
          {kpis.slice(0, 2).map((k) => (
            <div key={k.label}>
              <div className="text-mute">{k.label}</div>
              <div className="flex items-baseline gap-1.5">
                <span className="font-semibold">{formatNumber(k.value, config)}</span>
                {k.prev != null && <Delta v={pctDelta(k.value, k.prev)} />}
              </div>
            </div>
          ))}
        </div>
        {cats.length > 0 && (
          <div className="mt-2 space-y-1 border-t border-line pt-2">
            {cats.map((r) => (
              <div key={r.name} className="flex items-center justify-between text-[12px]">
                <span className="truncate text-ink">{r.name}</span>
                <span className="tabular-nums text-mute">{formatNumber(r.value, config)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    );
  }

  const meas = sizeCol || measureOf(rows, columns, config);
  const dims = strCols(rows, columns);
  const catX = dims[0] || columns[0];
  const catY = dims[1] || dims[0];
  const xSet = Array.from(new Set(rows.map((r) => formatCategory(r[catX]))));
  const ySet = Array.from(new Set(rows.map((r) => formatCategory(r[catY]))));
  const max = Math.max(...rows.map((r) => Number(r[meas] ?? 0)), 1);
  const cats = summarize(rows, catY || catX, meas).slice(0, 8);
  const points = rows.map((r) => ({
    x: xSet.indexOf(formatCategory(r[catX])),
    y: ySet.indexOf(formatCategory(r[catY])),
    r: 4 + (Number(r[meas] ?? 0) / max) * 14,
    v: Number(r[meas] ?? 0),
    xl: formatCategory(r[catX]),
    yl: formatCategory(r[catY]),
  }));

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
      {config.showTitle !== false && <div className="text-[13px] font-medium text-ink">{title}</div>}
      <div className="min-h-0 flex-1">
        <ChartJsCanvas
          type="bubble"
          data={{
            datasets: [{
              label: meas,
              data: points,
              backgroundColor: hexToRgba(color, 0.55),
              borderColor: color,
            }],
          }}
          options={{
            plugins: {
              legend: { display: false },
              tooltip: { ...chartTooltip(theme), callbacks: { label: (ctx: any) => `${ctx.raw.xl} / ${ctx.raw.yl}: ${formatNumber(ctx.raw.v, config)}` } },
            },
            scales: {
              x: { min: -0.5, max: Math.max(xSet.length - 0.5, 0.5), ticks: { color: chrome.mute, callback: (v: any) => xSet[Number(v)] || "" }, grid: { color: chrome.line }, border: { display: false } },
              y: { min: -0.5, max: Math.max(ySet.length - 0.5, 0.5), ticks: { color: chrome.mute, callback: (v: any) => ySet[Number(v)] || "" }, grid: { color: chrome.line }, border: { display: false } },
            },
          }}
        />
      </div>
      <div className="mt-2 grid grid-cols-2 gap-2 text-[12px]">
        {kpis.slice(0, 2).map((k) => (
          <div key={k.label}>
            <div className="text-mute">{k.label}</div>
            <div className="flex items-baseline gap-1.5">
              <span className="font-semibold">{formatNumber(k.value, config)}</span>
              {k.prev != null && <Delta v={pctDelta(k.value, k.prev)} />}
            </div>
          </div>
        ))}
      </div>
      <div className="mt-2 space-y-1 border-t border-line pt-2">
        {cats.slice(0, 5).map((r) => (
          <div key={r.name} className="flex items-center justify-between text-[12px]">
            <span className="truncate text-ink">{r.name}</span>
            <span className="tabular-nums text-mute">{formatNumber(r.value, config)}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

export function HexmapCard({ title, rows = [], columns = [], config = {} }: { title: string; rows?: Rows; columns?: string[]; config?: Cfg }) {
  const meas = measureOf(rows, columns, config);
  const dim = dimOf(rows, columns, config);
  const cats = summarize(rows, dim, meas).slice(0, 19);
  const max = Math.max(...cats.map((c) => c.value), 1);
  const colors = ["#93C5FD", "#A78BFA", "#C084FC", "#F472B6", "#FB7185", "#FB923C"];
  const cells = hexLayout(cats.length || 1);
  if (!cats.length) return <div className="rounded-2xl border border-line bg-surface p-3 shadow-sm"><EmptyViz /></div>;
  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-3 shadow-sm">
      {config.showTitle !== false && <div className="mb-1 text-[12px] font-medium text-ink">{title}</div>}
      <svg viewBox="0 0 200 180" className="min-h-0 flex-1">
        {cells.map((c, i) => {
          const item = cats[i];
          if (!item) return null;
          const t = item.value / max;
          return (
            <g key={item.name}>
              <polygon points={hexPoints(c.x, c.y, 12)} fill={colors[Math.min(colors.length - 1, Math.floor(t * (colors.length - 1)))]} opacity="0.95">
                <title>{`${item.name}: ${item.value}`}</title>
              </polygon>
            </g>
          );
        })}
      </svg>
    </div>
  );
}

export function RadialCard({ title, rows = [], columns = [], config = {} }: { title: string; rows?: Rows; columns?: string[]; config?: Cfg }) {
  const { theme } = useTheme();
  const meas = measureOf(rows, columns, config);
  const dim = dimOf(rows, columns, config);
  const s = seriesByDim(rows, dim, meas);
  const labels = s.labels.slice(0, 10);
  const values = s.values.slice(0, 10);
  const last = values.reduce((n, v) => n + v, 0);
  const color = config.color || "#8B5CF6";
  if (!values.length) return <div className="rounded-2xl border border-line bg-surface p-4 shadow-sm"><EmptyViz /></div>;
  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
      <div className="relative min-h-0 flex-1">
        <ChartJsCanvas
          type="polarArea"
          data={{
            labels,
            datasets: [{ data: values, backgroundColor: values.map((_, i) => hexToRgba(color, 0.18 + (i % 5) * 0.12)), borderColor: color, borderWidth: 1 }],
          }}
          options={{
            plugins: { legend: { display: false }, tooltip: chartTooltip(theme) },
            scales: { r: { ticks: { display: false }, grid: { color: "#e2e8f0" } } },
          }}
        />
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <div className="text-2xl font-semibold text-ink">{formatNumber(last, { ...config, compact: "auto", decimals: 0 })}</div>
        </div>
      </div>
      {config.showTitle !== false && <div className="mt-1 text-[13px] font-medium text-ink">{title}</div>}
    </div>
  );
}

function HeaderBlock({ title, value, prev, config, showTitle }: { title: string; value: number; prev?: number; config: Cfg; showTitle: boolean }) {
  return (
    <div className="mb-1">
      {showTitle && <div className="text-[13px] font-medium text-ink">{title}</div>}
      <div className="flex items-baseline gap-2">
        <span className="text-2xl font-semibold tracking-tight text-ink">{formatNumber(value, config)}</span>
        {prev != null && <Delta v={pctDelta(value, prev)} />}
      </div>
      {prev != null && <p className="text-[11px] text-mute">Comparado a {formatNumber(prev, config)} no período anterior</p>}
    </div>
  );
}

function summarize(rows: Rows, key: string, meas: string) {
  const map = new Map<string, number>();
  for (const r of rows) {
    const k = formatCategory(r[key] ?? "Outros");
    map.set(k, (map.get(k) || 0) + Number(r[meas] ?? 0));
  }
  return Array.from(map.entries()).map(([name, value]) => ({ name, value })).sort((a, b) => b.value - a.value);
}
function summarizeMany(rows: Rows, key: string, measures: string[]) {
  const map = new Map<string, number[]>();
  for (const r of rows) {
    const k = formatCategory(r[key] ?? "Outros");
    const cur = map.get(k) || measures.map(() => 0);
    measures.forEach((m, i) => {
      cur[i] += Number(r[m] ?? 0);
    });
    map.set(k, cur);
  }
  return Array.from(map.entries())
    .map(([name, values]) => ({ name, values }))
    .sort((a, b) => (b.values[0] || 0) - (a.values[0] || 0));
}
function layoutBars(items: { name: string; value: number }[], max: number, height: number, x: number) {
  let y = 8;
  return items.map((item) => {
    const h = Math.max(12, (item.value / max) * (height / Math.max(items.length, 1)));
    const bar = { item, x, y, h };
    y += h + 8;
    return bar;
  });
}
function hexPoints(cx: number, cy: number, r: number) {
  return Array.from({ length: 6 }, (_, i) => {
    const a = (Math.PI / 180) * (60 * i - 30);
    return `${cx + r * Math.cos(a)},${cy + r * Math.sin(a)}`;
  }).join(" ");
}
function hexLayout(n: number) {
  const out: { x: number; y: number }[] = [];
  out.push({ x: 100, y: 90 });
  for (let ring = 1; ring <= 3 && out.length < n; ring++) {
    for (let i = 0; i < 6; i++) {
      for (let j = 0; j < ring && out.length < n; j++) {
        const a = (Math.PI / 3) * i - Math.PI / 6;
        const step = (Math.PI / 3) * (i + 2);
        out.push({
          x: 100 + Math.cos(a) * 22 * ring + Math.cos(step) * 22 * j,
          y: 90 + Math.sin(a) * 22 * ring + Math.sin(step) * 22 * j,
        });
      }
    }
  }
  return out.slice(0, n);
}

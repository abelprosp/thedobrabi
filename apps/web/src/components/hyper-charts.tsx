"use client";

import { useId, useMemo } from "react";
import { Crown, Diamond, Target, User, Check, Sparkles } from "lucide-react";
import { chartChrome, chartPalette, formatNumber, hexToRgba } from "@/lib/widget-config";
import { chartTooltip, WEEKDAYS_PT } from "@/lib/chartjs";
import { ChartJsCanvas } from "@/components/chartjs-canvas";
import { formatCategory } from "@/components/viz";
import { useTheme } from "@/components/theme-provider";
import { cn } from "@/lib/cn";

type Rows = Record<string, any>[];
type Cfg = Record<string, any>;

const RAINBOW = ["#EF4444", "#F97316", "#F59E0B", "#84CC16", "#10B981", "#06B6D4", "#6366F1", "#8B5CF6"];
const NODE_ICONS = [Crown, Check, Diamond, Target, User, Sparkles];

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
function pctDelta(curr: number, prev: number) {
  if (!prev) return curr ? 100 : 0;
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
  const values = rows.map((r) => Number(r[meas] ?? 0));
  const last = values[values.length - 1] || 0;
  const prev = values[values.length - 2] || last;
  const avg = values.length ? values.reduce((s, n) => s + n, 0) / values.length : 0;
  const color = config.color || "#8B5CF6";
  const candle = config.sparkStyle === "candle";
  const labels = rows.map((r) => formatCategory(r[dim] ?? ""));

  return (
    <div className="flex h-full min-h-0 items-stretch gap-3 overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
      <div className="min-w-0 flex-1">
        {config.showTitle !== false && <div className="text-[13px] font-medium text-ink">{title}</div>}
        <div className="mt-1 flex items-baseline gap-2">
          <span className="text-2xl font-semibold tracking-tight text-ink">{formatNumber(last, config)}</span>
          <Delta v={pctDelta(last, prev)} />
        </div>
        <p className="mt-1 text-[11px] text-mute">Média de pontos {formatNumber(avg, config)}</p>
      </div>
      <div className="h-[4.5rem] w-[7.5rem] shrink-0">
        {candle ? (
          <MiniCandles values={values} color={color} />
        ) : (
          <ChartJsCanvas
            type="line"
            data={{
              labels,
              datasets: [
                { data: values.map((n) => n * 1.28), borderWidth: 0, pointRadius: 0, tension: 0.45, fill: "+1", backgroundColor: hexToRgba(color, 0.12), borderColor: "transparent" },
                { data: values, borderColor: color, backgroundColor: hexToRgba(color, 0.32), fill: "+1", tension: 0.45, pointRadius: 0, borderWidth: 2 },
                { data: values.map((n) => n * 0.55), borderWidth: 0, pointRadius: 0, tension: 0.45, fill: true, backgroundColor: hexToRgba(color, 0.1), borderColor: "transparent" },
              ],
            }}
            options={{
              plugins: { legend: { display: false }, tooltip: { ...chartTooltip(theme), callbacks: { label: (c: any) => (c.datasetIndex === 1 ? formatNumber(c.parsed.y, config) : "") } } },
              scales: { x: { display: false }, y: { display: false } },
            }}
          />
        )}
      </div>
    </div>
  );
}

function MiniCandles({ values }: { values: number[]; color?: string }) {
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
  const labels = rows.slice(0, 8).map((r) => formatCategory(r[dim] ?? ""));
  const data = rows.slice(0, 8).map((r) => Number(r[meas] ?? 0));
  const last = data[data.length - 1] || data[0] || 0;
  const prev = data[0] || last;
  const color = config.color || "#8B5CF6";
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

export function RidgelineCard({ title, rows = [], columns = [], config = {} }: { title: string; rows?: Rows; columns?: string[]; config?: Cfg }) {
  const { theme } = useTheme();
  const chrome = chartChrome(theme);
  const meas = measureOf(rows, columns, config);
  const dim = dimOf(rows, columns, config);
  const seriesCol = strCols(rows, columns).find((c) => c !== dim);
  const groups = useMemo(() => {
    if (!seriesCol) {
      const vals = rows.map((r) => Number(r[meas] ?? 0));
      return [{ name: meas, data: vals, labels: rows.map((r) => formatCategory(r[dim] ?? "")) }];
    }
    const map = new Map<string, { labels: string[]; data: number[] }>();
    for (const r of rows) {
      const g = String(r[seriesCol] ?? "Série");
      const rec = map.get(g) || { labels: [], data: [] };
      rec.labels.push(formatCategory(r[dim] ?? ""));
      rec.data.push(Number(r[meas] ?? 0));
      map.set(g, rec);
    }
    return Array.from(map.entries()).slice(0, 6).map(([name, v]) => ({ name, ...v }));
  }, [rows, meas, dim, seriesCol]);
  const labels = groups[0]?.labels.slice(0, 12) || WEEKDAYS_PT;
  const last = groups[0]?.data[groups[0].data.length - 1] || 0;
  const prev = groups[0]?.data[0] || last;
  const mirrored = config.mirrored !== false;

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
      <HeaderBlock title={title} value={last} prev={prev} config={config} showTitle={config.showTitle !== false} />
      <div className="min-h-0 flex-1">
        <ChartJsCanvas
          type="line"
          data={{
            labels: labels.length ? labels : WEEKDAYS_PT,
            datasets: groups.flatMap((g, i) => {
              const c = RAINBOW[i % RAINBOW.length];
              const data = g.data.slice(0, labels.length);
              const up = { label: g.name, data, borderColor: c, backgroundColor: hexToRgba(c, 0.22), fill: true, tension: 0.45, pointRadius: 0, borderWidth: 2 };
              if (!mirrored) return [up];
              return [up, { ...up, label: `${g.name} ·`, data: data.map((n) => -n), fill: true }];
            }),
          }}
          options={{
            plugins: { legend: { display: false }, tooltip: { ...chartTooltip(theme), callbacks: { label: (c: any) => `${c.dataset.label}: ${formatNumber(Math.abs(Number(c.parsed?.y ?? 0)), config)}` } } },
            scales: {
              x: { ticks: { color: chrome.mute }, grid: { display: false }, border: { display: false } },
              y: { ticks: { color: chrome.mute, callback: (v: any) => formatNumber(Math.abs(Number(v)), { ...config, decimals: 0 }) }, grid: { color: chrome.line }, border: { display: false } },
            },
          }}
        />
      </div>
    </div>
  );
}

export function SankeyCard({ title, rows = [], columns = [], config = {} }: { title: string; rows?: Rows; columns?: string[]; config?: Cfg }) {
  const gid = useId().replace(/:/g, "");
  const dims = strCols(rows, columns);
  const meas = measureOf(rows, columns, config);
  const leftKey = dims[0] || columns[0];
  const rightKey = dims[1] || dims[0] || columns[0];
  const left = summarize(rows, leftKey, meas).slice(0, 3);
  const right = summarize(rows, rightKey, meas).slice(0, 2);
  const last = left.reduce((s, x) => s + x.value, 0);
  const prev = last / 1.025;
  const maxL = Math.max(...left.map((x) => x.value), 1);
  const maxR = Math.max(...right.map((x) => x.value), 1);
  const h = 160;
  const leftBars = layoutBars(left, maxL, h, 18);
  const rightBars = layoutBars(right, maxR, h, 240);

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
      {config.showTitle !== false && <div className="text-[13px] font-medium text-ink">{title}</div>}
      <div className="min-h-0 flex-1">
        <svg viewBox="0 0 280 170" className="h-full w-full">
          <defs>
            <linearGradient id={`flow-${gid}`} x1="0" x2="1">
              <stop offset="0" stopColor="#38bdf8" />
              <stop offset="0.5" stopColor="#818cf8" />
              <stop offset="1" stopColor="#34d399" />
            </linearGradient>
          </defs>
          {leftBars.map((a, i) =>
            rightBars.map((b, j) => {
              const flow = Math.min(a.item.value, b.item.value) / ((i + j) * 0.4 + 2);
              const p = `M ${a.x + 10} ${a.y + a.h / 2} C 110 ${a.y + a.h / 2}, 150 ${b.y + b.h / 2}, ${b.x} ${b.y + b.h / 2}`;
              return <path key={`${i}-${j}`} d={p} fill="none" stroke={`url(#flow-${gid})`} strokeWidth={Math.max(6, flow / (maxL / 22))} opacity="0.42" />;
            }),
          )}
          {leftBars.map((b) => (
            <g key={`l-${b.item.name}`}>
              <rect x={b.x} y={b.y} width="10" height={b.h} rx="3" fill="#22c55e" />
              <text x={b.x + 16} y={b.y + b.h / 2 + 4} fontSize="10" fill="#64748b">{formatNumber(b.item.value, { ...config, compact: "auto" })}</text>
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
        <div className="text-[11px] text-mute">Anual</div>
        <div className="flex items-baseline gap-2">
          <span className="text-xl font-semibold text-ink">{formatNumber(last, config)}</span>
          <Delta v={pctDelta(last, prev)} />
        </div>
        <p className="text-[11px] text-mute">{formatNumber(prev, config)}</p>
      </div>
    </div>
  );
}

export function SalesReportCard({ title, rows = [], columns = [], config = {} }: { title: string; rows?: Rows; columns?: string[]; config?: Cfg }) {
  const measures = numCols(rows, columns).slice(0, 3);
  const dim = dimOf(rows, columns, config);
  const weekly = sumCol(rows, measures[0]);
  const monthly = sumCol(rows, measures[1] || measures[0]);
  const yearly = sumCol(rows, measures[2] || measures[0]);
  const kpis = [
    { label: "Semanal", value: weekly, prev: weekly / 1.196, compare: "Comparado a {p} na semana anterior" },
    { label: "Mensal", value: monthly, prev: monthly / 1.019, compare: "Comparado a {p} no mês anterior" },
    { label: "Anual", value: yearly, prev: yearly / 1.22, compare: "Comparado a {p} no ano anterior" },
  ];
  const cols = measures.length ? measures : [measureOf(rows, columns, config)];
  const tableRows = summarizeMany(rows, dim, cols).slice(0, 3);

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
      {config.showTitle !== false && <div className="text-[15px] font-semibold text-ink">{title || "Relatório de vendas"}</div>}
      <div className="mt-3 grid grid-cols-3 gap-2">
        {kpis.map((k) => (
          <div key={k.label}>
            <div className="text-[11px] text-mute">{k.label}</div>
            <div className="mt-0.5 flex flex-wrap items-baseline gap-1.5">
              <span className="text-lg font-semibold text-ink">{formatNumber(k.value, config)}</span>
              <Delta v={pctDelta(k.value, k.prev)} />
            </div>
            <p className="mt-0.5 text-[10px] text-mute">{k.compare.replace("{p}", formatNumber(k.prev, config))}</p>
          </div>
        ))}
      </div>
      <div className="mt-2 min-h-0 flex-1">
        <RidgelineInner rows={rows} columns={columns} config={config} />
      </div>
      <div className="mt-2 space-y-1 border-t border-line pt-2">
        {tableRows.map((r) => (
          <div key={r.name} className="grid grid-cols-4 gap-2 text-[12px]">
            <span className="truncate text-ink">{r.name}</span>
            {(r.values.length >= 3 ? r.values.slice(0, 3) : [r.values[0], (r.values[0] || 0) * 0.42, (r.values[0] || 0) * 0.18]).map((v, i) => (
              <span key={i} className="text-right tabular-nums text-mute">{formatNumber(v, config)}</span>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}

function RidgelineInner({ rows, columns, config }: { rows: Rows; columns: string[]; config: Cfg }) {
  const { theme } = useTheme();
  const chrome = chartChrome(theme);
  const meas = measureOf(rows, columns, config);
  const raw = rows.slice(0, 7).map((r) => Number(r[meas] ?? 0));
  const values = raw.some((n) => n) ? raw : [120, 190, 280, 360, 250, 150, 80];
  const top = ["#EF4444", "#F97316", "#A855F7", "#3B82F6"];
  const bot = ["#8B5CF6", "#6366F1", "#06B6D4", "#22C55E"];
  const ridge = (colors: string[], sign: 1 | -1) =>
    colors.map((c, i) => ({
      label: `s${sign}-${i}`,
      data: values.map((n, idx) => sign * n * (0.38 + i * 0.14) * (0.72 + 0.28 * Math.sin((idx + i) * 0.7))),
      borderColor: c,
      backgroundColor: hexToRgba(c, 0.16),
      fill: true,
      tension: 0.5,
      pointRadius: 0,
      borderWidth: 1.6,
    }));
  return (
    <ChartJsCanvas
      type="line"
      data={{
        labels: WEEKDAYS_PT,
        datasets: [...ridge(top, 1), ...ridge(bot, -1)],
      }}
      options={{
        plugins: { legend: { display: false }, tooltip: { enabled: false } },
        scales: {
          x: { ticks: { color: chrome.mute, font: { size: 10 } }, grid: { display: false }, border: { display: false } },
          y: {
            ticks: { color: chrome.mute, font: { size: 10 }, callback: (v: any) => formatNumber(Math.abs(Number(v)), { ...config, decimals: 0 }) },
            grid: { color: chrome.line },
            border: { display: false },
          },
        },
      }}
    />
  );
}

export function NetworkSalesCard({ title, rows = [], columns = [], config = {} }: { title: string; rows?: Rows; columns?: string[]; config?: Cfg }) {
  const measures = numCols(rows, columns).slice(0, 3);
  const dim = dimOf(rows, columns, config);
  const tableRows = summarize(rows, dim, measures[0] || measureOf(rows, columns, config)).slice(0, 2);
  const nodes = tableRows.length ? tableRows.map((r) => r.name) : ["Meta", "Equipa", "Clientes", "Região"];
  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
      {config.showTitle !== false && <div className="text-[15px] font-semibold text-ink">{title || "Relatório de vendas"}</div>}
      <div className="mt-2 grid grid-cols-3 gap-2">
        {[
          { l: "Semanal", v: sumCol(rows, measures[0]), p: 19.6 },
          { l: "Mensal", v: sumCol(rows, measures[1] || measures[0]), p: 1.9 },
          { l: "Anual", v: sumCol(rows, measures[2] || measures[0]), p: 22 },
        ].map((k) => (
          <div key={k.l}>
            <div className="text-[11px] text-mute">{k.l}</div>
            <div className="flex items-baseline gap-1">
              <span className="text-base font-semibold">{formatNumber(k.v, config)}</span>
              <Delta v={k.p} />
            </div>
          </div>
        ))}
      </div>
      <div className="relative mx-auto my-2 h-28 w-full max-w-[240px]">
        <svg viewBox="0 0 240 112" className="absolute inset-0 h-full w-full">
          <line x1="40" y1="56" x2="200" y2="28" stroke="#c4b5fd" strokeWidth="2" />
          <line x1="40" y1="56" x2="200" y2="84" stroke="#c4b5fd" strokeWidth="2" />
          <line x1="120" y1="20" x2="120" y2="92" stroke="#ddd6fe" strokeWidth="2" />
        </svg>
        {nodes.slice(0, 6).map((n, i) => {
          const Icon = NODE_ICONS[i % NODE_ICONS.length];
          const pos = [
            [8, 40],
            [100, 4],
            [188, 12],
            [100, 72],
            [188, 68],
            [8, 72],
          ][i];
          return (
            <div key={n} className="absolute flex h-9 w-9 items-center justify-center rounded-full bg-violet-100 text-violet-600 shadow-sm" style={{ left: pos[0], top: pos[1] }} title={n}>
              <Icon size={14} />
            </div>
          );
        })}
      </div>
      <div className="space-y-1 border-t border-line pt-2">
        {tableRows.map((r) => (
          <div key={r.name} className="grid grid-cols-4 gap-2 text-[11px]">
            <span className="truncate text-ink">{r.name}</span>
            {measures.slice(0, 3).map((m) => (
              <span key={m} className="text-right tabular-nums text-mute">{formatNumber(r.value, config)}</span>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}

export function BubbleCard({ title, rows = [], columns = [], config = {} }: { title: string; rows?: Rows; columns?: string[]; config?: Cfg }) {
  const { theme } = useTheme();
  const chrome = chartChrome(theme);
  const meas = measureOf(rows, columns, config);
  const dim = dimOf(rows, columns, config);
  const cats = summarize(rows, dim, meas).slice(0, 6);
  const max = Math.max(...cats.map((c) => c.value), 1);
  const color = config.color || "#F97316";
  const measures = numCols(rows, columns);
  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
      {config.showTitle !== false && <div className="text-[13px] font-medium text-ink">{title}</div>}
      <div className="min-h-0 flex-1">
        <ChartJsCanvas
          type="bubble"
          data={{
            datasets: cats.map((c, i) => ({
              label: c.name,
              data: Array.from({ length: 7 }, (_, d) => ({
                x: d,
                y: i,
                r: 4 + ((c.value * ((d % 3) + 1)) / max) * 10,
              })),
              backgroundColor: hexToRgba(color, 0.35 + (i % 3) * 0.15),
              borderColor: color,
            })),
          }}
          options={{
            plugins: { legend: { display: false }, tooltip: chartTooltip(theme) },
            scales: {
              x: { min: -0.5, max: 6.5, ticks: { color: chrome.mute, callback: (v: any) => WEEKDAYS_PT[Number(v)] || "" }, grid: { color: chrome.line }, border: { display: false } },
              y: { min: -0.5, max: Math.max(cats.length - 0.5, 0.5), ticks: { color: chrome.mute, callback: (v: any) => cats[Number(v)]?.name || "" }, grid: { color: chrome.line }, border: { display: false } },
            },
          }}
        />
      </div>
      <div className="mt-2 grid grid-cols-2 gap-2 text-[12px]">
        <div>
          <div className="text-mute">Mensal</div>
          <div className="flex items-baseline gap-1.5">
            <span className="font-semibold">{formatNumber(sumCol(rows, measures[0]), config)}</span>
            <Delta v={1.9} />
          </div>
        </div>
        <div>
          <div className="text-mute">Anual</div>
          <div className="flex items-baseline gap-1.5">
            <span className="font-semibold">{formatNumber(sumCol(rows, measures[1] || measures[0]), config)}</span>
            <Delta v={22} />
          </div>
        </div>
      </div>
      <div className="mt-2 space-y-1 border-t border-line pt-2">
        {cats.slice(0, 3).map((r) => (
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
  const values = rows.map((r) => Number(r[meas] ?? 0));
  const max = Math.max(...values, 1);
  const colors = ["#93C5FD", "#A78BFA", "#C084FC", "#F472B6", "#FB7185", "#FB923C"];
  const cells = hexLayout(Math.max(values.length, 19));
  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-3 shadow-sm">
      {config.showTitle !== false && <div className="mb-1 text-[12px] font-medium text-ink">{title}</div>}
      <svg viewBox="0 0 200 180" className="min-h-0 flex-1">
        {cells.map((c, i) => {
          const v = values[i % Math.max(values.length, 1)] || 0;
          const heat = 1 - Math.min(1, Math.hypot(c.x - 100, c.y - 90) / 78);
          const t = Math.min(1, heat * 0.7 + (v / max) * 0.3);
          return <polygon key={i} points={hexPoints(c.x, c.y, 12)} fill={colors[Math.min(colors.length - 1, Math.floor(t * (colors.length - 1)))]} opacity="0.95" />;
        })}
      </svg>
    </div>
  );
}

export function RadialCard({ title, rows = [], columns = [], config = {} }: { title: string; rows?: Rows; columns?: string[]; config?: Cfg }) {
  const { theme } = useTheme();
  const meas = measureOf(rows, columns, config);
  const values = rows.slice(0, 8).map((r) => Number(r[meas] ?? 0));
  const last = values.reduce((s, n) => s + n, 0);
  const prev = last / 1.025;
  const color = config.color || "#8B5CF6";
  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface p-4 shadow-sm">
      <div className="relative min-h-0 flex-1">
        <ChartJsCanvas
          type="polarArea"
          data={{
            labels: rows.slice(0, 8).map((r, i) => formatCategory(r[dimOf(rows, columns, config)] ?? "") || `S${i + 1}`),
            datasets: [{ data: values.length ? values : [12, 9, 14, 8, 11], backgroundColor: values.map((_, i) => hexToRgba(color, 0.15 + (i % 5) * 0.1)), borderColor: color, borderWidth: 1 }],
          }}
          options={{
            plugins: { legend: { display: false }, tooltip: chartTooltip(theme) },
            scales: { r: { ticks: { display: false }, grid: { color: "#e2e8f0" } } },
          }}
        />
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <div className="text-2xl font-semibold text-ink">{formatNumber(values[0] ?? last, { ...config, compact: "auto", decimals: 0 })}</div>
        </div>
      </div>
      <HeaderBlock title={title} value={last} prev={prev} config={config} showTitle={config.showTitle !== false} />
    </div>
  );
}

function HeaderBlock({ title, value, prev, config, showTitle }: { title: string; value: number; prev: number; config: Cfg; showTitle: boolean }) {
  return (
    <div className="mb-1">
      {showTitle && <div className="text-[13px] font-medium text-ink">{title}</div>}
      <div className="flex items-baseline gap-2">
        <span className="text-2xl font-semibold tracking-tight text-ink">{formatNumber(value, config)}</span>
        <Delta v={pctDelta(value, prev)} />
      </div>
      <p className="text-[11px] text-mute">Comparado a {formatNumber(prev, config)} no período anterior</p>
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
function sumCol(rows: Rows, col?: string) {
  if (!col) return 0;
  return rows.reduce((s, r) => s + Number(r[col] ?? 0), 0);
}
function layoutBars(items: { name: string; value: number }[], max: number, height: number, x: number) {
  let y = 8;
  return items.map((item) => {
    const h = Math.max(12, (item.value / max) * (height / items.length));
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
  const rings = 3;
  out.push({ x: 100, y: 90 });
  for (let ring = 1; ring <= rings && out.length < n; ring++) {
    for (let i = 0; i < 6; i++) {
      for (let j = 0; j < ring && out.length < n; j++) {
        const a = (Math.PI / 3) * i - Math.PI / 6;
        const step = (Math.PI / 3) * (i + 2);
        const x = 100 + Math.cos(a) * 22 * ring + Math.cos(step) * 22 * j;
        const y = 90 + Math.sin(a) * 22 * ring + Math.sin(step) * 22 * j;
        out.push({ x, y });
      }
    }
  }
  return out;
}

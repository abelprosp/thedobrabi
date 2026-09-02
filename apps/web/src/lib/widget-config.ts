export const BRAND_PALETTE = [
  "#2563EB",
  "#1D4ED8",
  "#1E40AF",
  "#6366F1",
  "#4F46E5",
  "#0EA5E9",
  "#0284C7",
  "#F59E0B",
  "#8B5CF6",
  "#10B981",
  "#EF4444",
];

export type CompactMode = "none" | "auto" | "k" | "m" | "b";
export type CurrencyCode = "" | "BRL" | "USD" | "EUR";
export type LegendPosition = "top" | "bottom" | "left" | "right";
export type TitleAlign = "left" | "center" | "right";

const CURRENCY_PREFIX: Record<string, string> = {
  BRL: "R$ ",
  USD: "US$ ",
  EUR: "€ ",
};

export function formatNumber(value: any, config?: Record<string, any> | null) {
  const n = Number(value);
  if (value == null || Number.isNaN(n)) return "—";
  const d = Math.max(0, Math.min(6, Number(config?.decimals ?? 0)));
  const compact = (config?.compact as CompactMode) || "none";
  let scaled = n;
  let compactSuffix = "";
  const abs = Math.abs(n);
  if (compact === "auto") {
    if (abs >= 1e9) {
      scaled = n / 1e9;
      compactSuffix = " B";
    } else if (abs >= 1e6) {
      scaled = n / 1e6;
      compactSuffix = " M";
    } else if (abs >= 1e3) {
      scaled = n / 1e3;
      compactSuffix = " K";
    }
  } else if (compact === "k") {
    scaled = n / 1e3;
    compactSuffix = " K";
  } else if (compact === "m") {
    scaled = n / 1e6;
    compactSuffix = " M";
  } else if (compact === "b") {
    scaled = n / 1e9;
    compactSuffix = " B";
  }
  const currency = config?.currency as string | undefined;
  const prefix = (currency && CURRENCY_PREFIX[currency]) || config?.prefix || "";
  const suffix = `${config?.suffix || ""}${compactSuffix}`;
  const s = scaled.toLocaleString("pt-BR", { minimumFractionDigits: d, maximumFractionDigits: d });
  return `${prefix}${s}${suffix}`;
}

export function hexToRgba(hex: string, alpha: number) {
  const raw = (hex || "#2563EB").replace("#", "");
  const full = raw.length === 3 ? raw.split("").map((c) => c + c).join("") : raw.padEnd(6, "0").slice(0, 6);
  const n = Number.parseInt(full, 16);
  if (Number.isNaN(n)) return `rgba(37,99,235,${alpha})`;
  const r = (n >> 16) & 255;
  const g = (n >> 8) & 255;
  const b = n & 255;
  return `rgba(${r},${g},${b},${alpha})`;
}

export function chartPalette(color?: string) {
  if (!color) return BRAND_PALETTE;
  return [color, ...BRAND_PALETTE.filter((c) => c.toLowerCase() !== color.toLowerCase())];
}

export type ChartChrome = {
  ink: string;
  mute: string;
  line: string;
  surface: string;
  surface2: string;
};

const LIGHT_CHROME: ChartChrome = {
  ink: "#0f172a",
  mute: "#5b6470",
  line: "#e2e8f0",
  surface: "#ffffff",
  surface2: "#f1f5f9",
};

export function chartChrome(): ChartChrome {
  if (typeof window === "undefined") return LIGHT_CHROME;
  const s = getComputedStyle(document.documentElement);
  const v = (name: string, fb: string) => s.getPropertyValue(name).trim() || fb;
  return {
    ink: v("--color-ink", LIGHT_CHROME.ink),
    mute: v("--color-mute", LIGHT_CHROME.mute),
    line: v("--color-line", LIGHT_CHROME.line),
    surface: v("--color-surface", LIGHT_CHROME.surface),
    surface2: v("--color-surface-2", LIGHT_CHROME.surface2),
  };
}

export function legendOption(show: boolean, position: LegendPosition = "top") {
  if (!show) return undefined;
  const c = chartChrome();
  const base = { textStyle: { color: c.mute, fontSize: 11 } };
  if (position === "bottom") return { ...base, bottom: 0, left: "center" };
  if (position === "left") return { ...base, left: 0, top: "middle", orient: "vertical" };
  if (position === "right") return { ...base, right: 0, top: "middle", orient: "vertical" };
  return { ...base, top: 0, left: "center" };
}

export function titleAlignClass(align?: TitleAlign) {
  if (align === "center") return "text-center justify-center";
  if (align === "right") return "text-right justify-end";
  return "text-left justify-start";
}

export type InspectorCaps = {
  query: boolean;
  color: boolean;
  axes: boolean;
  legend: boolean;
  dataLabels: boolean;
  format: boolean;
  interaction: boolean;
  kpi: boolean;
  table: boolean;
  slicer: boolean;
  gauge: boolean;
  waterfall: boolean;
  cartesian: boolean;
  pie: boolean;
  scatter: boolean;
};

const NO_QUERY = new Set(["text", "image", "markdown", "iframe"]);
const NO_COLOR = new Set(["text", "markdown", "iframe", "decomposition_tree", "table", "image"]);
const AXES = new Set(["bar", "line", "area", "scatter", "waterfall", "heatmap"]);
const LEGEND = new Set(["bar", "line", "area", "pie", "funnel", "treemap"]);
const DATA_LABELS = new Set(["bar", "line", "area", "pie", "funnel", "treemap", "heatmap", "waterfall", "scatter"]);
const NO_FORMAT = new Set(["text", "image", "markdown", "iframe", "slicer"]);
const CARTESIAN = new Set(["bar", "line", "area"]);

export function inspectorCaps(type: string): InspectorCaps {
  return {
    query: !NO_QUERY.has(type),
    color: !NO_COLOR.has(type),
    axes: AXES.has(type),
    legend: LEGEND.has(type),
    dataLabels: DATA_LABELS.has(type),
    format: !NO_FORMAT.has(type),
    interaction: !NO_QUERY.has(type),
    kpi: type === "kpi" || type === "kpi_goal",
    table: type === "table",
    slicer: type === "slicer",
    gauge: type === "gauge",
    waterfall: type === "waterfall",
    cartesian: CARTESIAN.has(type),
    pie: type === "pie",
    scatter: type === "scatter",
  };
}

export function echartsTooltip() {
  const c = chartChrome();
  return {
    backgroundColor: c.surface,
    borderColor: c.line,
    textStyle: { color: c.ink, fontSize: 12 },
    extraCssText: "box-shadow: 0 8px 24px rgba(15,23,42,0.18); border-radius: 10px;",
  };
}

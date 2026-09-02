import { isNumericValue } from "@/lib/cn";

export type WidgetIssue = {
  code: string;
  message: string;
};

const JOIN_PREFIX = "join.";

export function isJoinField(name: string) {
  return name.startsWith(JOIN_PREFIX);
}

export function joinFieldName(name: string) {
  return name.startsWith(JOIN_PREFIX) ? name.slice(JOIN_PREFIX.length) : name;
}

export function asJoinField(name: string) {
  return name.startsWith(JOIN_PREFIX) ? name : `${JOIN_PREFIX}${name}`;
}

function preview(raw: unknown) {
  const s = String(raw ?? "").trim();
  if (!s) return "vazio";
  return s.length > 48 ? `${s.slice(0, 45)}…` : s;
}

export function firstNumericEntry(row: Record<string, unknown> | undefined, prefer?: string[]) {
  if (!row) return { key: "", value: undefined as unknown };
  for (const key of prefer || []) {
    if (key && key in row && isNumericValue(row[key])) return { key, value: row[key] };
    const alt = key.replace(".", "_");
    if (alt && alt in row && isNumericValue(row[alt])) return { key: alt, value: row[alt] };
  }
  const entries = Object.entries(row);
  const num = entries.find(([, v]) => isNumericValue(v));
  if (num) return { key: num[0], value: num[1] };
  return { key: entries[0]?.[0] || "", value: entries[0]?.[1] };
}

export function diagnoseQueryValue(opts: {
  rows: Record<string, unknown>[];
  columns: string[];
  measures?: string[];
  dimensions?: string[];
  kind: "kpi" | "chart" | "table";
}): WidgetIssue | null {
  const { rows, columns, measures = [], dimensions = [], kind } = opts;
  if (!measures.length && kind === "kpi") {
    return {
      code: "no_measure",
      message: "Este KPI não tem medida. Nas propriedades, escolha uma métrica numérica (valor, salário, quantidade…).",
    };
  }
  if (rows.length === 0) {
    return {
      code: "empty",
      message: "A consulta não devolveu linhas. O filtro, o período ou o cruzamento deixou o conjunto vazio.",
    };
  }
  if (kind === "table") return null;

  const prefer = measures.map((m) => m.replace(".", "_"));
  const { key, value } = firstNumericEntry(rows[0], prefer);

  if (value == null || value === "") {
    return {
      code: "null",
      message: `A medida «${key || measures[0] || "métrica"}» veio nula. Não há valores para agregar neste recorte.`,
    };
  }
  if (typeof value === "number" && Number.isNaN(value)) {
    return {
      code: "nan",
      message: `A medida «${key}» devolveu NaN (divisão por zero, média sem valores ou dados inválidos). Confira a métrica e os filtros.`,
    };
  }
  if (typeof value === "number" && !Number.isFinite(value)) {
    return {
      code: "infinity",
      message: `A medida «${key}» devolveu infinito. Há uma divisão por zero ou um valor fora da escala.`,
    };
  }
  if (!isNumericValue(value)) {
    const looksLikeDim = dimensions.some((d) => d === key || d.toLowerCase() === key.toLowerCase());
    return {
      code: "not_numeric",
      message: looksLikeDim
        ? `«${key}» é uma dimensão, não uma medida. O visual tentou somar «${preview(value)}». Escolha uma métrica numérica nas propriedades.`
        : `«${key}» não é numérico (recebido: ${preview(value)}). Escolha uma medida como valor, quantidade ou salário.`,
    };
  }

  if (kind === "chart") {
    const numericCols = columns.filter((c) => rows.some((r) => isNumericValue(r[c])));
    if (numericCols.length === 0) {
      return {
        code: "chart_no_number",
        message: "Este gráfico não encontrou nenhuma coluna numérica. Cruze ou escolha uma medida nas propriedades.",
      };
    }
  }
  return null;
}

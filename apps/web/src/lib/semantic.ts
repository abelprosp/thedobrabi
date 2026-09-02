export type SemanticMeasure = {
  name: string;
  column?: string;
  aggregation?: string;
  expression?: string;
};

export type SemanticDimension = {
  name: string;
  column?: string;
  type?: string;
};

export type SemanticModel = {
  dataset_id?: string;
  name?: string;
  time_column?: string;
  measures?: SemanticMeasure[];
  dimensions?: SemanticDimension[];
};

export type DatasetListItem = {
  id: string;
  name: string;
  slug?: string;
  status?: string;
  row_count?: number;
  quality_score?: number;
  storage_mode?: string;
  source_id?: string | null;
  source_type?: string | null;
  source_name?: string | null;
};

export function measureKey(m: SemanticMeasure) {
  return (m.name || m.column || "").trim();
}

export function dimensionKey(d: SemanticDimension) {
  return (d.column || d.name || "").trim();
}

export function modelFromSemanticRow(row: any): SemanticModel | null {
  if (!row) return null;
  if (row.model && typeof row.model === "object") return row.model as SemanticModel;
  if (row.model_json && typeof row.model_json === "object") return row.model_json as SemanticModel;
  return null;
}

export function modelForDataset(models: any[], datasetId?: string | null): SemanticModel | null {
  if (!datasetId) return null;
  const row = models.find((m: any) => m.dataset_id === datasetId);
  return modelFromSemanticRow(row);
}

export function pickMeasures(model: SemanticModel | null | undefined, n: number): string[] {
  const all = (model?.measures || []).map(measureKey).filter(Boolean);
  const real = all.filter((name) => !isRowCountMeasure(model, name));
  return (real.length ? real : all).slice(0, Math.max(0, n));
}

function isRowCountMeasure(model: SemanticModel | null | undefined, name: string) {
  const m = model?.measures?.find((x) => measureKey(x) === name);
  const expr = (m?.expression || "").replace(/\s/g, "").toLowerCase();
  return m?.column === "*" || expr === "count(*)" || (m?.aggregation === "count" && (!m.column || m.column === "*"));
}

export function pickDimensions(model: SemanticModel | null | undefined, n: number): string[] {
  return (model?.dimensions || []).map(dimensionKey).filter(Boolean).slice(0, Math.max(0, n));
}

export function timeDimension(model: SemanticModel | null | undefined): string {
  const dims = pickDimensions(model, 50);
  const time = model?.time_column || "";
  if (time && dims.includes(time)) return time;
  const byName = dims.find((d) => /date|time|periodo|period|month|year|data|dia/.test(d.toLowerCase()));
  return byName || dims[0] || "";
}

export function categoryDimension(model: SemanticModel | null | undefined): string {
  const time = timeDimension(model);
  const dims = pickDimensions(model, 50);
  return dims.find((d) => d !== time) || dims[0] || "";
}

/** Defaults for a new widget from the dataset's real semantic model (not revenue/region). */
export function widgetFieldDefaults(type: string, model: SemanticModel | null | undefined): { measures: string[]; dimensions: string[] } {
  const measuresAll = pickMeasures(model, 8);
  const m0 = measuresAll[0] || "";
  const m1 = measuresAll[1] || m0;
  const time = timeDimension(model);
  const cat = categoryDimension(model);
  const cat2 = pickDimensions(model, 8).find((d) => d !== cat && d !== time) || cat;

  switch (type) {
    case "kpi":
    case "kpi_goal":
    case "gauge":
      return { measures: m0 ? [m0] : [], dimensions: [] };
    case "metric_group":
      return { measures: measuresAll.slice(0, 4), dimensions: [] };
    case "slicer":
      return { measures: [], dimensions: cat ? [cat] : [] };
    case "sparkline":
    case "line":
    case "area":
      return { measures: m0 ? [m0] : [], dimensions: time ? [time] : cat ? [cat] : [] };
    case "heatmap":
    case "decomposition_tree":
      return { measures: m0 ? [m0] : [], dimensions: [cat, cat2].filter(Boolean) };
    case "scatter":
      return { measures: [m0, m1].filter(Boolean), dimensions: cat ? [cat] : [] };
    default:
      return { measures: m0 ? [m0] : [], dimensions: cat ? [cat] : [] };
  }
}

export function starterDashboardWidgets(datasetId: string, model: SemanticModel | null | undefined) {
  const kpi = widgetFieldDefaults("kpi", model);
  const bar = widgetFieldDefaults("bar", model);
  const measureLabel = kpi.measures[0] || "KPI";
  const dimLabel = bar.dimensions[0] || "";
  return [
    {
      id: crypto.randomUUID(),
      type: "kpi" as const,
      title: measureLabel,
      layout: { x: 0, y: 0, w: 3, h: 2 },
      query: { dataset_id: datasetId, measures: kpi.measures, dimensions: [], limit: 20 },
    },
    {
      id: crypto.randomUUID(),
      type: "bar" as const,
      title: dimLabel ? `${measureLabel} por ${dimLabel}` : measureLabel,
      layout: { x: 3, y: 0, w: 9, h: 4 },
      query: { dataset_id: datasetId, measures: bar.measures, dimensions: bar.dimensions, limit: 20 },
    },
  ];
}

export type QueryJoinSpec = {
  dataset_id: string;
  from_column: string;
  to_column: string;
  match?: "both" | "all_left";
};

export type RemappedQuery = {
  dataset_id?: string;
  measures: string[];
  dimensions: string[];
  filters?: any[];
  joins?: QueryJoinSpec[];
  limit?: number;
  time_range?: { start?: string; end?: string };
};

function normalizeJoins(
  joins: { dataset_id?: string; from_column?: string; to_column?: string; match?: string }[] | undefined,
): QueryJoinSpec[] {
  return (joins || [])
    .filter((j): j is { dataset_id: string; from_column?: string; to_column?: string; match?: string } =>
      typeof j?.dataset_id === "string" && j.dataset_id.length > 0,
    )
    .map((j) => ({
      dataset_id: j.dataset_id,
      from_column: j.from_column || "",
      to_column: j.to_column || "",
      match: j.match === "all_left" ? "all_left" : j.match === "both" ? "both" : undefined,
    }));
}

export function remapQueryToModel(
  query: {
    measures?: string[];
    dimensions?: string[];
    dataset_id?: string;
    limit?: number;
    filters?: any[];
    time_range?: { start?: string; end?: string };
    joins?: { dataset_id?: string; from_column?: string; to_column?: string; match?: string }[];
  } | undefined,
  type: string,
  model: SemanticModel | null | undefined,
): RemappedQuery {
  const next = widgetFieldDefaults(type, model);
  const joins = normalizeJoins(query?.joins);
  const hasJoin = joins.length > 0;
  const split = (names: string[] | undefined, resolve: (n: string) => string) => {
    const local: string[] = [];
    const joined: string[] = [];
    for (const n of names || []) {
      if (n.startsWith("join.")) {
        if (hasJoin) joined.push(n);
      } else {
        const r = resolve(n);
        if (r) local.push(r);
      }
    }
    return { local, joined };
  };
  const measures = split(query?.measures, (n) => resolveMeasureName(model, n));
  const dimensions = split(query?.dimensions, (n) => resolveDimensionName(model, n));
  const kpiNoDims = type === "kpi" || type === "kpi_goal" || type === "metric_group" || type === "gauge";
  const filters = (query?.filters || [])
    .map((f) => {
      if (typeof f?.dimension === "string" && f.dimension.startsWith("join.")) return hasJoin ? f : null;
      const dimension = resolveDimensionName(model, f.dimension);
      return dimension ? { ...f, dimension } : null;
    })
    .filter((f): f is NonNullable<typeof f> => f != null);
  return {
    dataset_id: query?.dataset_id,
    measures: (measures.local.length ? measures.local : next.measures).concat(measures.joined),
    dimensions: kpiNoDims ? [] : (dimensions.local.length ? dimensions.local : next.dimensions).concat(dimensions.joined),
    filters: filters.length ? filters : undefined,
    joins: hasJoin ? joins : undefined,
    limit: query?.limit,
    time_range: query?.time_range,
  };
}

export function pickLiveDatasetId(live: DatasetListItem[], preferred?: string, currentName?: string) {
  const ids = new Set(live.map((d) => d.id));
  if (preferred && ids.has(preferred)) return preferred;
  if (currentName) {
    const n = norm(currentName);
    const hit = live.find((d) => norm(d.name || "") === n || norm(d.name || "").includes(n) || n.includes(norm(d.name || "")));
    if (hit) return hit.id;
  }
  return live[0]?.id || "";
}

export function rebindQueryToLiveDataset(
  query: { measures?: string[]; dimensions?: string[]; dataset_id?: string; limit?: number; filters?: any[] } | undefined,
  type: string,
  live: DatasetListItem[],
  models: any[],
  preferred?: string,
) {
  if (!query) return query;
  const ids = new Set(live.map((d) => d.id));
  if (query.dataset_id && ids.has(query.dataset_id)) return query;
  const staleName = models.find((m: any) => m.dataset_id === query.dataset_id)?.name as string | undefined;
  const nextId = pickLiveDatasetId(live, preferred, staleName);
  if (!nextId) return query;
  return remapQueryToModel({ ...query, dataset_id: nextId }, type, modelForDataset(models, nextId));
}

function norm(s: string) {
  return s
    .toLowerCase()
    .trim()
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/\s+/g, "_");
}

const MEASURE_ALIASES: string[][] = [
  ["revenue", "receita", "sales", "vendas", "amount", "valor", "faturamento", "gmv", "billing", "turnover", "net_sales", "total"],
  ["cost", "custo", "despesa", "expense", "custos", "despesas", "valor", "amount"],
  ["margin", "margem", "lucro", "profit", "resultado", "ebitda", "result"],
  ["orders", "pedidos", "order_count", "qty", "quantidade", "volume", "units", "unidades"],
  ["customers", "clientes", "users", "usuarios", "accounts", "contas"],
  ["headcount", "funcionarios", "employees", "colaboradores", "fte", "pessoas"],
  ["salary", "salario", "folha", "payroll", "remuneracao"],
  ["sessions", "sessoes", "visits", "visitas", "traffic", "trafego"],
  ["conversion", "conversao", "cvr"],
  ["churn", "cancelamentos", "churn_rate"],
  ["mrr", "arr", "recorrencia", "recurring"],
  ["inventory", "estoque", "stock", "on_hand"],
  ["freight", "frete", "shipping", "logistica"],
];

const DIMENSION_ALIASES: string[][] = [
  ["date", "data", "dia", "period", "periodo", "month", "mes", "year", "ano", "week", "semana", "ordem_mes"],
  ["region", "regiao", "uf", "estado", "cidade", "city", "country", "pais", "territorio"],
  ["product", "produto", "sku", "item", "item_name"],
  ["category", "categoria", "linha", "rubrica", "classificacao", "segmento", "segment"],
  ["customer", "cliente", "account", "conta"],
  ["sales_rep", "vendedor", "seller", "representante", "owner", "agente"],
  ["channel", "canal", "origem", "source", "utm"],
  ["status", "situacao", "etapa", "stage", "pipeline"],
  ["natureza", "tipo", "type", "kind"],
  ["company", "empresa", "filial", "loja", "store", "unidade", "branch"],
  ["department", "departamento", "area", "cargo", "role", "funcao", "team", "time"],
  ["campaign", "campanha", "adset", "ad"],
  ["warehouse", "deposito", "armazem", "cd"],
  ["supplier", "fornecedor", "vendor"],
];

function aliasGroup(groups: string[][], name: string): string[] | undefined {
  const n = norm(name);
  return groups.find((g) => g.some((a) => norm(a) === n));
}

function keysMatch(candidate: string, wanted: string, group?: string[]) {
  const c = norm(candidate);
  const w = norm(wanted);
  if (!c) return false;
  if (c === w) return true;
  if (group?.some((a) => norm(a) === c)) return true;
  if (c.includes(w) || (w.includes(c) && c.length >= 3)) return true;
  if (group?.some((a) => {
    const an = norm(a);
    return an.length >= 3 && (c.includes(an) || an.includes(c));
  })) return true;
  return false;
}

export function resolveMeasureName(model: SemanticModel | null | undefined, name: string): string {
  if (!name || !model?.measures?.length) return "";
  const n = norm(name);
  const exact = model.measures.find((m) => norm(measureKey(m)) === n || norm(m.column || "") === n);
  if (exact) return measureKey(exact);
  const group = aliasGroup(MEASURE_ALIASES, name);
  const ranked = [...model.measures].sort((a, b) => Number(isRowCountMeasure(model, measureKey(a))) - Number(isRowCountMeasure(model, measureKey(b))));
  const hit = ranked.find((m) => keysMatch(measureKey(m), name, group) || keysMatch(m.column || "", name, group));
  return hit ? measureKey(hit) : "";
}

export function resolveDimensionName(model: SemanticModel | null | undefined, name: string): string {
  if (!name || !model?.dimensions?.length) return "";
  const n = norm(name);
  const exact = model.dimensions.find((d) => norm(dimensionKey(d)) === n || norm(d.name || "") === n);
  if (exact) return dimensionKey(exact);
  if (aliasGroup(DIMENSION_ALIASES, name)?.includes("date") || ["date", "data", "mes", "month"].includes(n)) {
    const time = timeDimension(model);
    if (time) return time;
  }
  const group = aliasGroup(DIMENSION_ALIASES, name);
  const hit = model.dimensions.find((d) => keysMatch(dimensionKey(d), name, group) || keysMatch(d.name || "", name, group));
  return hit ? dimensionKey(hit) : "";
}

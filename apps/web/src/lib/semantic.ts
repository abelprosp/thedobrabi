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
  return (model?.measures || []).map(measureKey).filter(Boolean).slice(0, Math.max(0, n));
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

export function remapQueryToModel(
  query: { measures?: string[]; dimensions?: string[]; dataset_id?: string; limit?: number; filters?: any[] } | undefined,
  type: string,
  model: SemanticModel | null | undefined,
) {
  const next = widgetFieldDefaults(type, model);
  const measures = (query?.measures || []).map((m) => resolveMeasureName(model, m)).filter(Boolean);
  const dimensions = (query?.dimensions || []).map((d) => resolveDimensionName(model, d)).filter(Boolean);
  const kpiNoDims = type === "kpi" || type === "kpi_goal" || type === "metric_group" || type === "gauge";
  return {
    ...query,
    dataset_id: query?.dataset_id,
    measures: measures.length ? measures : next.measures,
    dimensions: kpiNoDims ? [] : dimensions.length ? dimensions : next.dimensions,
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
  return s.toLowerCase().trim().replace(/\s+/g, "_");
}

function resolveMeasureName(model: SemanticModel | null | undefined, name: string): string {
  if (!name || !model?.measures?.length) return "";
  const hit = model.measures.find((m) => norm(measureKey(m)) === norm(name) || norm(m.column || "") === norm(name));
  return hit ? measureKey(hit) : "";
}

function resolveDimensionName(model: SemanticModel | null | undefined, name: string): string {
  if (!name || !model?.dimensions?.length) return "";
  const hit = model.dimensions.find((d) => norm(dimensionKey(d)) === norm(name) || norm(d.name || "") === norm(name));
  return hit ? dimensionKey(hit) : "";
}

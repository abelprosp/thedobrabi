export type Flow = {
  id: string;
  name: string;
  description: string;
  status: string;
  updated_at: string;
  source_dataset_id?: string | null;
  target_dataset_id?: string | null;
  output_dataset_id?: string | null;
  schedule?: string;
  layout?: FlowLayout | string | null;
};

export type Step = {
  id: string;
  step_order: number;
  kind: string;
  subkind: string;
  name: string;
  config: Record<string, unknown>;
};

export type FlowNode = {
  id: string;
  stepId: string;
  kind: string;
  subkind: string;
  name: string;
  x: number;
  y: number;
  config: Record<string, unknown>;
};

export type FlowEdge = { from: string; to: string };

export type FlowLayout = {
  nodes: FlowNode[];
  edges: FlowEdge[];
};

export type DatasetOption = {
  id: string;
  name: string;
  row_count?: number;
  source_id?: string | null;
  source_type?: string | null;
  source_name?: string | null;
};

export type StepDraft = {
  name: string;
  kind: string;
  subkind: string;
  step_order: number;
  config: Record<string, unknown>;
};

export const STEP_SUBKINDS = [
  { value: "extract", label: "Origem", kind: "extract" },
  { value: "rename", label: "Renomear", kind: "transform" },
  { value: "filter", label: "Filtrar", kind: "transform" },
  { value: "change_type", label: "Mudar tipo", kind: "transform" },
  { value: "fill_null", label: "Preencher nulos", kind: "transform" },
  { value: "dedup", label: "Remover duplicados", kind: "transform" },
  { value: "conditional", label: "Condicional", kind: "transform" },
  { value: "aggregate", label: "Agregar", kind: "transform" },
  { value: "sql", label: "SQL", kind: "transform" },
  { value: "join", label: "Juntar", kind: "transform" },
  { value: "validate", label: "Validar", kind: "validate" },
  { value: "load", label: "ClickHouse", kind: "load" },
] as const;

export type FlowTemplateId = "csv_clickhouse" | "sql_transform" | "connector_ch" | "join_sources";

export type FlowTemplate = {
  id: FlowTemplateId;
  title: string;
  description: string;
  defaultName: string;
  needsSource: boolean;
  needsSecondSource: boolean;
  preferConnector: boolean;
  destination: string;
};

export const FLOW_TEMPLATES: FlowTemplate[] = [
  {
    id: "csv_clickhouse",
    title: "CSV → ClickHouse",
    description: "Lê um conjunto (CSV ou ficheiro) e materializa no ClickHouse.",
    defaultName: "CSV para ClickHouse",
    needsSource: true,
    needsSecondSource: false,
    preferConnector: false,
    destination: "ClickHouse",
  },
  {
    id: "sql_transform",
    title: "SQL → transformar → CH",
    description: "Lê um conjunto SQL, aplica uma transformação e grava no ClickHouse.",
    defaultName: "SQL com transformação",
    needsSource: true,
    needsSecondSource: false,
    preferConnector: false,
    destination: "ClickHouse",
  },
  {
    id: "connector_ch",
    title: "Conector → ClickHouse",
    description: "Usa um conjunto já sincronizado de um conector e materializa no ClickHouse.",
    defaultName: "Conector para ClickHouse",
    needsSource: true,
    needsSecondSource: false,
    preferConnector: true,
    destination: "ClickHouse",
  },
  {
    id: "join_sources",
    title: "Juntar 2 fontes",
    description: "Junta dois conjuntos e materializa o resultado no ClickHouse.",
    defaultName: "Junção de fontes",
    needsSource: true,
    needsSecondSource: true,
    preferConnector: false,
    destination: "ClickHouse",
  },
];

export function canEditFlows(role?: string | null) {
  return role === "owner" || role === "admin" || role === "analyst";
}

export function slugTable(name: string) {
  const slug = name
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_|_$/g, "")
    .slice(0, 48);
  return slug || "flow_output";
}

export function parseFlowLayout(raw: Flow["layout"]): FlowLayout | undefined {
  if (!raw) return undefined;
  let value: unknown = raw;
  if (typeof raw === "string") {
    try {
      value = JSON.parse(raw);
    } catch {
      return undefined;
    }
  }
  if (!value || typeof value !== "object") return undefined;
  const layout = value as FlowLayout;
  if (!Array.isArray(layout.nodes) || layout.nodes.length === 0) return undefined;
  return {
    nodes: layout.nodes,
    edges: Array.isArray(layout.edges) ? layout.edges : [],
  };
}

export function templateSteps(opts: {
  template: FlowTemplateId;
  name: string;
  sourceId: string;
  source2Id?: string;
}): StepDraft[] {
  const table = slugTable(opts.name);
  const extract = (label: string, datasetId: string, order: number): StepDraft => ({
    name: label,
    kind: "extract",
    subkind: "extract",
    step_order: order,
    config: { dataset_id: datasetId },
  });
  const load = (order: number): StepDraft => ({
    name: "ClickHouse",
    kind: "load",
    subkind: "load",
    step_order: order,
    config: { target: "clickhouse", table_name: table },
  });

  switch (opts.template) {
    case "csv_clickhouse":
      return [extract("Origem CSV", opts.sourceId, 1), load(2)];
    case "sql_transform":
      return [
        extract("Origem SQL", opts.sourceId, 1),
        {
          name: "Transformar",
          kind: "transform",
          subkind: "filter",
          step_order: 2,
          config: { column: "", op: "eq", value: "" },
        },
        load(3),
      ];
    case "connector_ch":
      return [extract("Conector", opts.sourceId, 1), load(2)];
    case "join_sources":
      return [
        extract("Fonte A", opts.sourceId, 1),
        extract("Fonte B", opts.source2Id || "", 2),
        {
          name: "Juntar",
          kind: "transform",
          subkind: "join",
          step_order: 3,
          config: {
            left_dataset_id: opts.sourceId,
            right_dataset_id: opts.source2Id || "",
            left_key: "id",
            right_key: "id",
          },
        },
        load(4),
      ];
  }
}

export function nodeTone(kind: string) {
  switch (kind) {
    case "extract":
      return "border-sky-300 bg-sky-50";
    case "load":
      return "border-emerald-300 bg-emerald-50";
    case "validate":
      return "border-amber-300 bg-amber-50";
    default:
      return "border-indigo-200 bg-white";
  }
}

export function suggestedSource(datasets: DatasetOption[], preferConnector: boolean) {
  if (preferConnector) {
    const fromConnector = datasets.find((d) => d.source_id);
    if (fromConnector) return fromConnector.id;
  }
  return datasets[0]?.id || "";
}

export type ManualColType = "text" | "number" | "date" | "datetime" | "boolean" | "select";

export type ManualColumn = {
  key: string;
  label: string;
  type: ManualColType | string;
  required?: boolean;
  options?: string[];
  hint?: string;
};

export type ManualRow = {
  id: string;
  values: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type ManualTable = {
  source_id: string;
  name: string;
  columns: ManualColumn[];
  rows: ManualRow[];
};

export const MANUAL_COL_TYPES: { value: ManualColType; label: string }[] = [
  { value: "text", label: "Texto" },
  { value: "number", label: "Número" },
  { value: "date", label: "Data" },
  { value: "datetime", label: "Data e hora" },
  { value: "boolean", label: "Sim / Não" },
  { value: "select", label: "Lista" },
];

export const DEFAULT_MANUAL_COLUMNS: ManualColumn[] = [
  { key: "data", label: "Data", type: "date", required: true, options: [] },
  { key: "descricao", label: "Descrição", type: "text", required: true, options: [] },
  { key: "valor", label: "Valor", type: "number", required: true, options: [] },
];

export function emptyManualColumn(): ManualColumn {
  return { key: "", label: "", type: "text", required: false, options: [] };
}

export function cellDisplay(v: unknown) {
  if (v == null || v === "") return "—";
  if (typeof v === "boolean") return v ? "Sim" : "Não";
  return String(v);
}

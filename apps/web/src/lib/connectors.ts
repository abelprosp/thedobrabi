export type CatalogField = {
  key: string;
  label: string;
  type: "text" | "password" | "number" | "url" | "textarea" | "checkbox" | "file" | "select" | string;
  required?: boolean;
  placeholder?: string;
  default?: string;
  hint?: string;
  options?: { value: string; label: string }[];
  accept?: string;
};

export type CatalogItem = {
  id: string;
  label: string;
  group: string;
  group_label: string;
  description: string;
  implemented: boolean;
  preview: boolean;
  message?: string;
  aliases?: string[];
  icon?: string;
  fields: CatalogField[];
};

export type CatalogGroup = { id: string; label: string };

export type CatalogResponse = {
  groups: CatalogGroup[];
  items: CatalogItem[];
};

export type DataSource = {
  id: string;
  name: string;
  type: string;
  status: string;
  last_sync_at?: string | null;
  created_at?: string;
  implemented?: boolean;
  preview?: boolean;
  message?: string;
  config?: Record<string, unknown>;
  datasets?: {
    id: string;
    name: string;
    status: string;
    row_count: number;
    storage_mode: string;
  }[];
};

export function connectorByType(items: CatalogItem[], type: string) {
  const t = (type || "").toLowerCase();
  return items.find((i) => i.id === t || i.id === type || (i.aliases || []).some((a) => a.toLowerCase() === t));
}

export function connectorLabel(items: CatalogItem[], type: string) {
  return connectorByType(items, type)?.label || type;
}

const PNG_ICON_IDS = new Set(["asaas", "conta_azul", "ibge_censo", "odata", "inflacao", "expectativas", "cambio"]);

export function connectorIconSrc(item?: Pick<CatalogItem, "id" | "icon"> | null, type?: string) {
  if (item?.icon?.startsWith("/")) return item.icon;
  const id = item?.id || type || "";
  if (!id) return "";
  return `/connectors/${id}.${PNG_ICON_IDS.has(id) ? "png" : "svg"}`;
}

export function formatSyncAt(iso?: string | null) {
  if (!iso) return "Nunca";
  try {
    return new Date(iso).toLocaleString("pt-PT", { dateStyle: "short", timeStyle: "short" });
  } catch {
    return iso;
  }
}

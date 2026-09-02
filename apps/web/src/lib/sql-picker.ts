export type InspectColumn = {
  name: string;
  type?: string;
  label: string;
};

export type InspectTable = {
  schema?: string;
  name: string;
  full_name: string;
  label: string;
  row_count?: number | null;
  columns: InspectColumn[];
};

export type InspectFK = {
  from_table: string;
  from_column: string;
  to_table: string;
  to_column: string;
};

export type DiscoverCatalog = {
  tables: string[];
  catalog?: InspectTable[];
  foreign_keys?: InspectFK[];
  message?: string;
  preview?: boolean;
};

export type SelectedTable = {
  schema?: string;
  name: string;
  columns: string[];
};

export type SelectedJoin = {
  left_table: string;
  left_column: string;
  right_table: string;
  right_column: string;
  match: "both" | "all_left";
};

export type SourceSelection = {
  tables: SelectedTable[];
  joins: SelectedJoin[];
};

const WORDS: Record<string, string> = {
  customer: "Cliente",
  customers: "Clientes",
  cliente: "Cliente",
  clientes: "Clientes",
  order: "Pedido",
  orders: "Pedidos",
  pedido: "Pedido",
  pedidos: "Pedidos",
  sale: "Venda",
  sales: "Vendas",
  venda: "Venda",
  vendas: "Vendas",
  product: "Produto",
  products: "Produtos",
  produto: "Produto",
  produtos: "Produtos",
  user: "Utilizador",
  users: "Utilizadores",
  invoice: "Fatura",
  invoices: "Faturas",
  fatura: "Fatura",
  faturas: "Faturas",
  payment: "Pagamento",
  payments: "Pagamentos",
  item: "Item",
  items: "Itens",
  id: "Código",
  name: "Nome",
  nome: "Nome",
  email: "E-mail",
  phone: "Telefone",
  created: "Criado",
  updated: "Actualizado",
  at: "em",
  total: "Total",
  amount: "Valor",
  valor: "Valor",
  date: "Data",
  status: "Estado",
  description: "Descrição",
};

const DROP = ["fct_", "fact_", "dim_", "stg_", "raw_", "vw_", "tb_", "tbl_"];

export function humanizeIdent(raw: string) {
  let s = (raw || "").trim();
  const dot = s.lastIndexOf(".");
  if (dot >= 0) s = s.slice(dot + 1);
  let lower = s.toLowerCase();
  for (const p of DROP) {
    if (lower.startsWith(p)) {
      s = s.slice(p.length);
      lower = s.toLowerCase();
      break;
    }
  }
  if (WORDS[lower]) return WORDS[lower];
  const parts = s.split(/[_\-\s]+/).filter(Boolean);
  return parts
    .map((p, i) => {
      const lp = p.toLowerCase();
      const mapped = WORDS[lp];
      if (mapped) return i === 0 ? mapped : mapped.toLowerCase();
      return p.charAt(0).toUpperCase() + p.slice(1).toLowerCase();
    })
    .join(" ");
}

export function catalogFromDiscover(d: DiscoverCatalog): InspectTable[] {
  if (d.catalog && d.catalog.length > 0) return d.catalog;
  return (d.tables || []).map((full) => {
    const i = full.lastIndexOf(".");
    const schema = i > 0 ? full.slice(0, i) : "";
    const name = i > 0 ? full.slice(i + 1) : full;
    return { schema, name, full_name: full, label: humanizeIdent(name), columns: [] };
  });
}

export function tableByKey(tables: InspectTable[], key: string) {
  return tables.find((t) => t.full_name === key || t.name === key);
}

function stemFromIdColumn(col: string) {
  const c = col.toLowerCase();
  if (c.endsWith("_id") && c !== "id") return c.slice(0, -3);
  if (c.startsWith("id_") && c !== "id") return c.slice(3);
  return "";
}

function namesMatch(tableName: string, stem: string) {
  const a = tableName.toLowerCase().replace(/_/g, "");
  const b = stem.toLowerCase().replace(/_/g, "");
  if (a === b) return true;
  if (a === b + "s" || a + "s" === b) return true;
  if (a === b + "es" || a + "es" === b) return true;
  return humanizeIdent(tableName).toLowerCase() === humanizeIdent(stem).toLowerCase();
}

export type JoinSuggestion = SelectedJoin & { reason: "fk" | "name" };

export function suggestJoins(tables: InspectTable[], fks: InspectFK[], selectedKeys: string[]): JoinSuggestion[] {
  const selected = new Set(selectedKeys);
  const out: JoinSuggestion[] = [];
  const seen = new Set<string>();
  const keyOf = (a: string, ac: string, b: string, bc: string) => `${a}|${ac}|${b}|${bc}`;
  const push = (j: JoinSuggestion) => {
    const k = keyOf(j.left_table, j.left_column, j.right_table, j.right_column);
    const k2 = keyOf(j.right_table, j.right_column, j.left_table, j.left_column);
    if (seen.has(k) || seen.has(k2)) return;
    seen.add(k);
    out.push(j);
  };

  for (const fk of fks) {
    if (selected.has(fk.from_table) && selected.has(fk.to_table)) {
      push({
        left_table: fk.from_table,
        left_column: fk.from_column,
        right_table: fk.to_table,
        right_column: fk.to_column,
        match: "both",
        reason: "fk",
      });
    }
  }

  const sel = tables.filter((t) => selected.has(t.full_name));
  for (const a of sel) {
    for (const b of sel) {
      if (a.full_name === b.full_name) continue;
      for (const col of a.columns) {
        const stem = stemFromIdColumn(col.name);
        if (!stem || !namesMatch(b.name, stem)) continue;
        const right =
          b.columns.find((c) => c.name.toLowerCase() === "id") ||
          b.columns.find((c) => c.name.toLowerCase() === `${stem}_id`) ||
          b.columns.find((c) => c.name.toLowerCase() === `id_${stem}`);
        if (!right) continue;
        push({
          left_table: a.full_name,
          left_column: col.name,
          right_table: b.full_name,
          right_column: right.name,
          match: "both",
          reason: "name",
        });
      }
    }
  }
  return out;
}

export function joinSentence(tables: InspectTable[], j: SelectedJoin) {
  const left = tableByKey(tables, j.left_table);
  const right = tableByKey(tables, j.right_table);
  const a = left?.label || humanizeIdent(j.left_table);
  const b = right?.label || humanizeIdent(j.right_table);
  const field = humanizeIdent(j.left_column.replace(/_id$/i, "")) || humanizeIdent(j.left_column);
  return `${a} e ${b} ligam-se por ${field}`;
}

export function summaryPhrase(tables: InspectTable[], sel: SourceSelection) {
  const picked = sel.tables
    .map((t) => tableByKey(tables, t.schema ? `${t.schema}.${t.name}` : t.name) || tableByKey(tables, t.name))
    .filter(Boolean) as InspectTable[];
  const labels = picked.map((t) => t.label);
  const colCount = sel.tables.reduce((n, t) => n + (t.columns?.length || 0), 0);
  const colBit = colCount > 0 ? ` e trazer ${colCount} campo${colCount === 1 ? "" : "s"}` : "";

  if (labels.length === 0) return "Escolha pelo menos uma lista de dados.";
  if (labels.length === 1) return `Vamos trazer ${labels[0]}${colBit}.`;
  if (sel.joins.length === 0) {
    return `Vamos trazer ${joinPt(labels)} em separado (sem cruzamento)${colBit}.`;
  }
  const links = sel.joins.map((j) => {
    const a = tableByKey(tables, j.left_table)?.label || humanizeIdent(j.left_table);
    const b = tableByKey(tables, j.right_table)?.label || humanizeIdent(j.right_table);
    const field = humanizeIdent(j.left_column.replace(/_id$/i, ""));
    return `${a} com ${b} pelo ${field.toLowerCase()}`;
  });
  return `Vamos juntar ${links.join(", ")}${colBit}.`;
}

function joinPt(items: string[]) {
  if (items.length === 1) return items[0];
  if (items.length === 2) return `${items[0]} e ${items[1]}`;
  return `${items.slice(0, -1).join(", ")} e ${items[items.length - 1]}`;
}

export function toApiSelection(tables: InspectTable[], selected: Record<string, string[]>, joins: SelectedJoin[]): SourceSelection {
  const keys = Object.keys(selected);
  return {
    tables: keys.map((full) => {
      const t = tableByKey(tables, full);
      const name = t?.name || full.split(".").pop() || full;
      const schema = t?.schema;
      return { schema: schema || undefined, name, columns: selected[full] || [] };
    }),
    joins,
  };
}

export function formatRowCount(n?: number | null) {
  if (n == null || n < 0) return "";
  return `${Math.round(n).toLocaleString("pt-PT")} linha${n === 1 ? "" : "s"}`;
}

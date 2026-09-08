export type BlockKind =
  | "sum"
  | "avg"
  | "count"
  | "distinctcount"
  | "min"
  | "max"
  | "divide"
  | "add"
  | "sub"
  | "mul"
  | "nullif"
  | "if"
  | "eq"
  | "neq"
  | "gt"
  | "lt"
  | "calculate"
  | "lookup"
  | "related"
  | "yoy"
  | "tomonth"
  | "column"
  | "measure"
  | "number"
  | "text"
  | "star";

export type SlotKey =
  | "arg"
  | "left"
  | "right"
  | "cond"
  | "then"
  | "else"
  | "expr"
  | "filterCol"
  | "filterVal"
  | "column"
  | "matchCol"
  | "matchVal";

export type MeasureBlock = {
  id: string;
  kind: BlockKind;
  arg?: MeasureBlock | null;
  left?: MeasureBlock | null;
  right?: MeasureBlock | null;
  cond?: MeasureBlock | null;
  then?: MeasureBlock | null;
  else?: MeasureBlock | null;
  expr?: MeasureBlock | null;
  filterCol?: MeasureBlock | null;
  filterVal?: MeasureBlock | null;
  column?: MeasureBlock | null;
  matchCol?: MeasureBlock | null;
  matchVal?: MeasureBlock | null;
  name?: string;
  value?: string;
  table?: string;
};

export const BLOCK_MIME = "application/x-dobra-block";

const SLOT_KEYS: SlotKey[] = [
  "arg",
  "left",
  "right",
  "cond",
  "then",
  "else",
  "expr",
  "filterCol",
  "filterVal",
  "column",
  "matchCol",
  "matchVal",
];

export function newBlockId() {
  return `b-${Math.random().toString(36).slice(2, 10)}`;
}

export function createBlock(kind: BlockKind, extra: Partial<MeasureBlock> = {}): MeasureBlock {
  return { id: newBlockId(), kind, ...extra };
}

export type PaletteItem = {
  kind: BlockKind;
  label: string;
  group: "agregar" | "calcular" | "logica" | "buscar" | "valores";
  name?: string;
  value?: string;
};

export const PALETTE_OPS: PaletteItem[] = [
  { kind: "sum", label: "Soma", group: "agregar" },
  { kind: "avg", label: "Média", group: "agregar" },
  { kind: "count", label: "Contar", group: "agregar" },
  { kind: "distinctcount", label: "Distintos", group: "agregar" },
  { kind: "min", label: "Mínimo", group: "agregar" },
  { kind: "max", label: "Máximo", group: "agregar" },
  { kind: "divide", label: "Dividir", group: "calcular" },
  { kind: "add", label: "Somar (+)", group: "calcular" },
  { kind: "sub", label: "Subtrair (−)", group: "calcular" },
  { kind: "mul", label: "Multiplicar (×)", group: "calcular" },
  { kind: "nullif", label: "Se zero, vazio", group: "calcular" },
  { kind: "if", label: "Se / então", group: "logica" },
  { kind: "eq", label: "Igual a", group: "logica" },
  { kind: "neq", label: "Diferente de", group: "logica" },
  { kind: "gt", label: "Maior que", group: "logica" },
  { kind: "lt", label: "Menor que", group: "logica" },
  { kind: "calculate", label: "Filtrar cálculo", group: "buscar" },
  { kind: "lookup", label: "Procurar noutro conjunto", group: "buscar" },
  { kind: "related", label: "Campo relacionado", group: "buscar" },
  { kind: "yoy", label: "Ano anterior", group: "buscar" },
  { kind: "tomonth", label: "Mês da data", group: "buscar" },
  { kind: "number", label: "Número", group: "valores" },
  { kind: "text", label: "Texto", group: "valores" },
  { kind: "star", label: "Tudo (*)", group: "valores" },
];

export const GROUP_LABELS: Record<PaletteItem["group"], string> = {
  agregar: "Agregar",
  calcular: "Calcular",
  logica: "Lógica",
  buscar: "Buscar dados",
  valores: "Valores",
};

export function blockLabel(kind: BlockKind) {
  return PALETTE_OPS.find((p) => p.kind === kind)?.label || kind;
}

export function compileBlock(b?: MeasureBlock | null): string {
  if (!b) return "";
  const a = (x?: MeasureBlock | null) => compileBlock(x);
  switch (b.kind) {
    case "sum":
      return `SUM(${a(b.arg) || "*"})`;
    case "avg":
      return `AVG(${a(b.arg)})`;
    case "count":
      return `COUNT(${a(b.arg) || "*"})`;
    case "distinctcount":
      return `DISTINCTCOUNT(${a(b.arg)})`;
    case "min":
      return `MIN(${a(b.arg)})`;
    case "max":
      return `MAX(${a(b.arg)})`;
    case "divide":
      return `DIVIDE(${a(b.left)}, ${a(b.right)})`;
    case "add":
      return `(${a(b.left)} + ${a(b.right)})`;
    case "sub":
      return `(${a(b.left)} - ${a(b.right)})`;
    case "mul":
      return `(${a(b.left)} * ${a(b.right)})`;
    case "nullif":
      return `NULLIF(${a(b.left)}, ${a(b.right) || "0"})`;
    case "if":
      return `CASE WHEN ${a(b.cond)} THEN ${a(b.then)} ELSE ${a(b.else) || "0"} END`;
    case "eq":
      return `${a(b.left)} = ${a(b.right)}`;
    case "neq":
      return `${a(b.left)} <> ${a(b.right)}`;
    case "gt":
      return `${a(b.left)} > ${a(b.right)}`;
    case "lt":
      return `${a(b.left)} < ${a(b.right)}`;
    case "calculate":
      return `CALCULATE(${a(b.expr)}, ${a(b.filterCol)} = ${a(b.filterVal)})`;
    case "lookup": {
      const t = (b.table || "Tabela").replace(/[\[\]]/g, "");
      return `LOOKUPVALUE(${t}[${a(b.column)}], ${t}[${a(b.matchCol)}], ${a(b.matchVal)})`;
    }
    case "related": {
      const t = (b.table || "Tabela").replace(/[\[\]]/g, "");
      return `RELATED(${t}[${a(b.column)}])`;
    }
    case "yoy":
      return `YOY(${a(b.arg)})`;
    case "tomonth":
      return `TOMONTH(${a(b.arg)})`;
    case "column":
      return b.name || "";
    case "measure":
      return b.name ? `[${b.name}]` : "";
    case "number":
      return b.value?.trim() || "0";
    case "text":
      return `'${(b.value || "").replace(/'/g, "''")}'`;
    case "star":
      return "*";
    default:
      return "";
  }
}

export function slotsFor(kind: BlockKind): { key: SlotKey; label: string }[] {
  switch (kind) {
    case "sum":
    case "avg":
    case "count":
    case "distinctcount":
    case "min":
    case "max":
    case "yoy":
    case "tomonth":
      return [{ key: "arg", label: kind === "count" ? "o quê" : "campo ou expressão" }];
    case "divide":
      return [
        { key: "left", label: "numerador" },
        { key: "right", label: "denominador" },
      ];
    case "add":
    case "sub":
    case "mul":
    case "nullif":
    case "eq":
    case "neq":
    case "gt":
    case "lt":
      return [
        { key: "left", label: "esquerda" },
        { key: "right", label: "direita" },
      ];
    case "if":
      return [
        { key: "cond", label: "se" },
        { key: "then", label: "então" },
        { key: "else", label: "senão" },
      ];
    case "calculate":
      return [
        { key: "expr", label: "cálculo" },
        { key: "filterCol", label: "coluna" },
        { key: "filterVal", label: "valor" },
      ];
    case "lookup":
      return [
        { key: "column", label: "trazer campo" },
        { key: "matchCol", label: "quando este campo" },
        { key: "matchVal", label: "igual a" },
      ];
    case "related":
      return [{ key: "column", label: "campo" }];
    default:
      return [];
  }
}

export function getSlot(block: MeasureBlock, key: SlotKey): MeasureBlock | null | undefined {
  return block[key];
}

export function setSlotOn(block: MeasureBlock, key: SlotKey, child: MeasureBlock | null): MeasureBlock {
  return { ...block, [key]: child };
}

export function mapTree(root: MeasureBlock | null, fn: (b: MeasureBlock) => MeasureBlock): MeasureBlock | null {
  if (!root) return null;
  const next = fn({ ...root });
  for (const key of SLOT_KEYS) {
    const child = next[key];
    if (child) next[key] = mapTree(child, fn);
  }
  return next;
}

export function findBlock(root: MeasureBlock | null, id: string): MeasureBlock | null {
  if (!root) return null;
  if (root.id === id) return root;
  for (const key of SLOT_KEYS) {
    const found = findBlock(root[key] || null, id);
    if (found) return found;
  }
  return null;
}

export function extractBlock(root: MeasureBlock | null, id: string): { tree: MeasureBlock | null; extracted: MeasureBlock | null } {
  if (!root) return { tree: null, extracted: null };
  if (root.id === id) return { tree: null, extracted: root };
  let extracted: MeasureBlock | null = null;
  const next: MeasureBlock = { ...root };
  for (const key of SLOT_KEYS) {
    const child = root[key];
    if (!child) continue;
    const res = extractBlock(child, id);
    next[key] = res.tree;
    if (res.extracted) extracted = res.extracted;
  }
  return { tree: next, extracted };
}

export function placeBlock(
  root: MeasureBlock | null,
  target: { parentId: string | null; slot: SlotKey | "root" },
  incoming: MeasureBlock,
): MeasureBlock | null {
  if (target.slot === "root" || target.parentId == null) return incoming;
  if (!root) return incoming;
  return mapTree(root, (b) => {
    if (b.id !== target.parentId) return b;
    return setSlotOn(b, target.slot as SlotKey, incoming);
  });
}

export function patchBlock(root: MeasureBlock | null, id: string, patch: Partial<MeasureBlock>): MeasureBlock | null {
  return mapTree(root, (b) => (b.id === id ? { ...b, ...patch } : b));
}

export function containsId(root: MeasureBlock | null, id: string): boolean {
  return !!findBlock(root, id);
}

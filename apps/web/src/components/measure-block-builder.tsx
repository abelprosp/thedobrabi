"use client";

import { useMemo, useState, type ReactNode } from "react";
import { Blocks, GripVertical, Trash2 } from "lucide-react";
import { cn } from "@/components/ui";
import {
  BLOCK_MIME,
  GROUP_LABELS,
  PALETTE_OPS,
  type BlockKind,
  type MeasureBlock,
  type PaletteItem,
  type SlotKey,
  blockLabel,
  compileBlock,
  containsId,
  createBlock,
  extractBlock,
  getSlot,
  placeBlock,
  patchBlock,
  slotsFor,
} from "@/lib/measure-blocks";

type DropTarget = { parentId: string | null; slot: SlotKey | "root" };

const KIND_TONE: Record<string, string> = {
  agregar: "bg-indigo-50 border-indigo-200 text-indigo-950 dark:bg-indigo-950/50 dark:border-indigo-800 dark:text-indigo-100",
  calcular: "bg-cyan-50 border-cyan-200 text-cyan-950 dark:bg-cyan-950/40 dark:border-cyan-800 dark:text-cyan-100",
  logica: "bg-amber-50 border-amber-200 text-amber-950 dark:bg-amber-950/40 dark:border-amber-800 dark:text-amber-100",
  buscar: "bg-violet-50 border-violet-200 text-violet-950 dark:bg-violet-950/40 dark:border-violet-800 dark:text-violet-100",
  valores: "bg-slate-100 border-slate-300 text-slate-800 dark:bg-slate-800 dark:border-slate-600 dark:text-slate-100",
};

function groupOf(kind: BlockKind): PaletteItem["group"] {
  return PALETTE_OPS.find((p) => p.kind === kind)?.group || "valores";
}

function parseDrag(e: React.DragEvent): MeasureBlock | null {
  const raw = e.dataTransfer.getData(BLOCK_MIME);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as MeasureBlock;
  } catch {
    return null;
  }
}

export function MeasureBlockBuilder({
  value,
  onChange,
  columns,
  measures,
  dimensions,
  tables,
}: {
  value: MeasureBlock | null;
  onChange: (next: MeasureBlock | null) => void;
  columns: { name: string }[];
  measures: { name: string }[];
  dimensions: { name: string; column?: string }[];
  tables?: { name: string }[];
}) {
  const [over, setOver] = useState<string | null>(null);
  const expression = useMemo(() => compileBlock(value), [value]);

  const dropAt = (target: DropTarget, incoming: MeasureBlock) => {
    let tree = value;
    if (incoming.id && containsId(tree, incoming.id)) {
      const extracted = extractBlock(tree, incoming.id);
      tree = extracted.tree;
      incoming = extracted.extracted || incoming;
    }
    if (!incoming.id) incoming = { ...incoming, id: createBlock(incoming.kind).id };
    onChange(placeBlock(tree, target, incoming));
  };

  const onPaletteDrag = (e: React.DragEvent, item: PaletteItem) => {
    e.dataTransfer.setData(BLOCK_MIME, JSON.stringify(createBlock(item.kind, { name: item.name, value: item.value })));
    e.dataTransfer.effectAllowed = "copy";
  };

  return (
    <div className="grid gap-3 lg:grid-cols-[220px_minmax(0,1fr)]">
      <div className="max-h-[420px] space-y-3 overflow-auto rounded-xl border border-line bg-bg p-3">
        <p className="text-[11px] font-medium text-mute">Arraste para o canvas</p>
        {(Object.keys(GROUP_LABELS) as PaletteItem["group"][]).map((group) => (
          <div key={group}>
            <p className="mb-1 text-[10px] font-semibold uppercase tracking-wide text-mute">{GROUP_LABELS[group]}</p>
            <div className="flex flex-wrap gap-1">
              {PALETTE_OPS.filter((p) => p.group === group).map((item) => (
                <button
                  key={item.kind}
                  type="button"
                  draggable
                  onDragStart={(e) => onPaletteDrag(e, item)}
                  onClick={() => {
                    if (!value) onChange(createBlock(item.kind));
                  }}
                  className={cn(
                    "min-h-10 cursor-grab rounded-lg border px-2.5 py-1.5 text-[11px] font-medium active:cursor-grabbing sm:min-h-0 sm:py-1",
                    KIND_TONE[group],
                  )}
                >
                  {item.label}
                </button>
              ))}
            </div>
          </div>
        ))}
        {columns.length > 0 && (
          <div>
            <p className="mb-1 text-[10px] font-semibold uppercase tracking-wide text-mute">Colunas</p>
            <div className="flex flex-wrap gap-1">
              {columns.slice(0, 24).map((c) => (
                <button
                  key={c.name}
                  type="button"
                  draggable
                  onDragStart={(e) => {
                    e.dataTransfer.setData(BLOCK_MIME, JSON.stringify(createBlock("column", { name: c.name })));
                    e.dataTransfer.effectAllowed = "copy";
                  }}
                  onClick={() => {
                    if (!value) onChange(createBlock("column", { name: c.name }));
                  }}
                  className="min-h-10 cursor-grab rounded-lg border border-line bg-surface px-2.5 py-1.5 font-mono text-[10px] text-ink active:cursor-grabbing sm:min-h-0 sm:py-1"
                >
                  {c.name}
                </button>
              ))}
            </div>
          </div>
        )}
        {measures.length > 0 && (
          <div>
            <p className="mb-1 text-[10px] font-semibold uppercase tracking-wide text-mute">Medidas já criadas</p>
            <div className="flex flex-wrap gap-1">
              {measures.slice(0, 16).map((m) => (
                <button
                  key={m.name}
                  type="button"
                  draggable
                  onDragStart={(e) => {
                    e.dataTransfer.setData(BLOCK_MIME, JSON.stringify(createBlock("measure", { name: m.name })));
                    e.dataTransfer.effectAllowed = "copy";
                  }}
                  className="min-h-10 cursor-grab rounded-lg border border-primary/20 bg-primary/5 px-2.5 py-1.5 text-[10px] text-primary active:cursor-grabbing sm:min-h-0 sm:py-1"
                >
                  [{m.name}]
                </button>
              ))}
            </div>
          </div>
        )}
        {dimensions.length > 0 && (
          <div>
            <p className="mb-1 text-[10px] font-semibold uppercase tracking-wide text-mute">Dimensões (filtros)</p>
            <div className="flex flex-wrap gap-1">
              {dimensions.slice(0, 16).map((d) => {
                const col = d.column || d.name;
                return (
                  <button
                    key={col}
                    type="button"
                    draggable
                    onDragStart={(e) => {
                      e.dataTransfer.setData(BLOCK_MIME, JSON.stringify(createBlock("column", { name: col })));
                      e.dataTransfer.effectAllowed = "copy";
                    }}
                    className="min-h-10 cursor-grab rounded-lg border border-line bg-surface px-2.5 py-1.5 text-[10px] text-ink active:cursor-grabbing sm:min-h-0 sm:py-1"
                  >
                    {d.name}
                  </button>
                );
              })}
            </div>
          </div>
        )}
      </div>

      <div className="space-y-2">
        <DropSlot
          label="Monte a lógica aqui"
          active={over === "root"}
          empty={!value}
          onDragOver={(e) => {
            e.preventDefault();
            setOver("root");
          }}
          onDragLeave={() => setOver(null)}
          onDrop={(e) => {
            e.preventDefault();
            setOver(null);
            const incoming = parseDrag(e);
            if (incoming) dropAt({ parentId: null, slot: "root" }, incoming);
          }}
        >
          {value ? (
            <BlockView
              block={value}
              tables={tables}
              over={over}
              setOver={setOver}
              onDropAt={dropAt}
              onPatch={(id, patch) => onChange(patchBlock(value, id, patch))}
              onRemove={(id) => onChange(extractBlock(value, id).tree)}
            />
          ) : (
            <div className="flex flex-col items-center gap-1 py-8 text-center text-[12px] text-mute">
              <Blocks size={22} />
              Arraste blocos da esquerda. Clique num bloco se o canvas estiver vazio.
            </div>
          )}
        </DropSlot>
        {value && (
          <div className="flex items-start justify-between gap-2">
            <code className="block flex-1 overflow-x-auto rounded-lg bg-bg px-2 py-1.5 font-mono text-[11px] text-mute">{expression || "—"}</code>
            <button
              type="button"
              className="shrink-0 rounded-lg p-1.5 text-mute hover:bg-rose-50 hover:text-danger"
              onClick={() => onChange(null)}
              title="Limpar"
            >
              <Trash2 size={14} />
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

function DropSlot({
  label,
  empty,
  active,
  children,
  onDrop,
  onDragOver,
  onDragLeave,
}: {
  label: string;
  empty?: boolean;
  active?: boolean;
  children: ReactNode;
  onDrop: (e: React.DragEvent) => void;
  onDragOver: (e: React.DragEvent) => void;
  onDragLeave: () => void;
}) {
  return (
    <div
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
      className={cn(
        "min-h-[44px] rounded-xl border-2 border-dashed p-2 transition",
        active ? "border-primary bg-primary/5" : empty ? "border-line bg-surface-2/40" : "border-transparent bg-transparent p-0",
      )}
    >
      {empty && <span className="sr-only">{label}</span>}
      {children}
    </div>
  );
}

function BlockView({
  block,
  tables,
  over,
  setOver,
  onDropAt,
  onPatch,
  onRemove,
}: {
  block: MeasureBlock;
  tables?: { name: string }[];
  over: string | null;
  setOver: (id: string | null) => void;
  onDropAt: (target: DropTarget, incoming: MeasureBlock) => void;
  onPatch: (id: string, patch: Partial<MeasureBlock>) => void;
  onRemove: (id: string) => void;
}) {
  const slots = slotsFor(block.kind);
  const tone = KIND_TONE[groupOf(block.kind)];
  const leaf = slots.length === 0;

  return (
    <div
      draggable
      onDragStart={(e) => {
        e.dataTransfer.setData(BLOCK_MIME, JSON.stringify(block));
        e.dataTransfer.effectAllowed = "move";
        e.stopPropagation();
      }}
      className={cn("rounded-xl border px-2 py-1.5 shadow-sm", tone)}
    >
      <div className="mb-1 flex items-center gap-1">
        <GripVertical size={12} className="shrink-0 opacity-50" />
        <span className="text-[11px] font-semibold">{blockLabel(block.kind)}</span>
        <button type="button" className="ml-auto rounded p-0.5 opacity-60 hover:bg-black/5 hover:opacity-100" onClick={() => onRemove(block.id)}>
          <Trash2 size={11} />
        </button>
      </div>
      {block.kind === "column" && (
        <input
          className="mb-1 w-full rounded-md border border-white/40 bg-white/70 px-1.5 py-0.5 font-mono text-[11px] text-ink outline-none dark:bg-black/20 dark:text-white"
          value={block.name || ""}
          onChange={(e) => onPatch(block.id, { name: e.target.value })}
          placeholder="coluna"
        />
      )}
      {block.kind === "measure" && (
        <input
          className="mb-1 w-full rounded-md border border-white/40 bg-white/70 px-1.5 py-0.5 text-[11px] outline-none dark:bg-black/20"
          value={block.name || ""}
          onChange={(e) => onPatch(block.id, { name: e.target.value })}
          placeholder="nome da medida"
        />
      )}
      {(block.kind === "number" || block.kind === "text") && (
        <input
          className="mb-1 w-full rounded-md border border-white/40 bg-white/70 px-1.5 py-0.5 font-mono text-[11px] outline-none dark:bg-black/20"
          value={block.value || ""}
          onChange={(e) => onPatch(block.id, { value: e.target.value })}
          placeholder={block.kind === "number" ? "0" : "texto"}
        />
      )}
      {(block.kind === "lookup" || block.kind === "related") && (
        <div className="mb-1">
          {tables && tables.length > 0 ? (
            <select
              className="w-full rounded-md border border-white/40 bg-white/70 px-1.5 py-0.5 text-[11px] outline-none dark:bg-black/20"
              value={block.table || ""}
              onChange={(e) => onPatch(block.id, { table: e.target.value })}
            >
              <option value="">Conjunto…</option>
              {tables.map((t) => (
                <option key={t.name} value={t.name}>
                  {t.name}
                </option>
              ))}
            </select>
          ) : (
            <input
              className="w-full rounded-md border border-white/40 bg-white/70 px-1.5 py-0.5 text-[11px] outline-none dark:bg-black/20"
              value={block.table || ""}
              onChange={(e) => onPatch(block.id, { table: e.target.value })}
              placeholder="nome do conjunto"
            />
          )}
        </div>
      )}
      {!leaf && (
        <div className={cn("grid gap-1.5", slots.length > 1 ? "sm:grid-cols-2" : "")}>
          {slots.map((slot) => {
            const child = getSlot(block, slot.key);
            const key = `${block.id}:${slot.key}`;
            return (
              <div key={slot.key}>
                <p className="mb-0.5 text-[9px] font-medium uppercase tracking-wide opacity-70">{slot.label}</p>
                <DropSlot
                  label={slot.label}
                  empty={!child}
                  active={over === key}
                  onDragOver={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    setOver(key);
                  }}
                  onDragLeave={() => setOver(null)}
                  onDrop={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    setOver(null);
                    const incoming = parseDrag(e);
                    if (incoming) onDropAt({ parentId: block.id, slot: slot.key }, incoming);
                  }}
                >
                  {child ? (
                    <BlockView
                      block={child}
                      tables={tables}
                      over={over}
                      setOver={setOver}
                      onDropAt={onDropAt}
                      onPatch={onPatch}
                      onRemove={onRemove}
                    />
                  ) : (
                    <p className="px-1 py-2 text-center text-[10px] opacity-50">solte aqui</p>
                  )}
                </DropSlot>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

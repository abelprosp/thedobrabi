"use client";

import { useMemo, useState } from "react";
import { Search, X } from "lucide-react";
import { Button, FieldLabel, Input, cn } from "@/components/ui";
import { KPI_ICON_CATALOG, KPI_ICON_GROUPS, isKpiIconUrl, kpiIconUrl } from "@/lib/kpi-icons";
import { KpiIcon } from "@/components/kpi-icon";

export function KpiIconPicker({
  value,
  onChange,
}: {
  value?: string;
  onChange: (icon: string | undefined) => void;
}) {
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const query = q.trim().toLowerCase();

  const grouped = useMemo(() => {
    const items = KPI_ICON_CATALOG.filter((i) => {
      if (!query) return true;
      return (
        i.label.toLowerCase().includes(query) ||
        i.name.includes(query) ||
        i.group.toLowerCase().includes(query) ||
        (i.aliases || []).some((a) => a.toLowerCase().includes(query))
      );
    });
    return KPI_ICON_GROUPS.map((group) => ({
      group,
      items: items.filter((i) => i.group === group),
    })).filter((g) => g.items.length);
  }, [query]);

  const customUrl = value && isKpiIconUrl(value) ? kpiIconUrl(value) : "";
  const customEmoji = value && !isKpiIconUrl(value) && !KPI_ICON_CATALOG.some((i) => i.name === value) ? value : "";

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <div className="flex h-11 w-11 items-center justify-center rounded-xl border border-line bg-surface-2 text-ink">
          {value ? <KpiIcon value={value} size={20} /> : <span className="text-[10px] text-mute">Nenhum</span>}
        </div>
        <Button type="button" size="sm" variant="secondary" onClick={() => setOpen((v) => !v)}>
          {open ? "Fechar biblioteca" : "Biblioteca de ícones"}
        </Button>
        {value && (
          <button type="button" className="text-[12px] text-mute hover:text-ink" onClick={() => onChange(undefined)}>
            Remover
          </button>
        )}
      </div>

      {open && (
        <div className="space-y-3 rounded-xl border border-line bg-surface-2/60 p-3">
          <div className="relative">
            <Search size={14} className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-mute" />
            <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Procurar: receita, clientes, meta…" className="pl-8" />
            {q && (
              <button type="button" className="absolute right-2 top-1/2 -translate-y-1/2 text-mute" onClick={() => setQ("")} aria-label="Limpar">
                <X size={14} />
              </button>
            )}
          </div>
          <div className="max-h-56 space-y-3 overflow-y-auto pr-1">
            {grouped.length === 0 && <p className="text-[12px] text-mute">Nenhum ícone com esse nome. Use emoji ou um URL abaixo.</p>}
            {grouped.map((g) => (
              <div key={g.group}>
                <div className="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-mute">{g.group}</div>
                <div className="grid grid-cols-6 gap-1">
                  {g.items.map((i) => {
                    const selected = value === i.name;
                    return (
                      <button
                        key={i.name}
                        type="button"
                        title={i.label}
                        onClick={() => {
                          onChange(i.name);
                          setOpen(false);
                        }}
                        className={cn(
                          "flex h-9 w-full items-center justify-center rounded-lg border text-ink hover:bg-surface",
                          selected ? "border-primary bg-primary/10 text-primary" : "border-transparent bg-surface",
                        )}
                      >
                        <i.Icon size={16} />
                      </button>
                    );
                  })}
                </div>
              </div>
            ))}
          </div>
          <FieldLabel label="Personalizado" hint="Emoji (💰) ou URL de uma imagem">
            <Input
              value={customUrl || customEmoji}
              placeholder="💰  ou  https://…"
              onChange={(e) => {
                const v = e.target.value.trim();
                onChange(v || undefined);
              }}
            />
          </FieldLabel>
        </div>
      )}
    </div>
  );
}

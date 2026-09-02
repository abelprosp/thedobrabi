"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";

const items = [
  { href: "/overview", label: "Visão geral" },
  { href: "/dashboards", label: "Dashboards" },
  { href: "/apps", label: "Apps" },
  { href: "/ask", label: "Perguntar à TheDobra" },
  { href: "/data", label: "Dados" },
  { href: "/connectors", label: "Conectores" },
  { href: "/flows", label: "Flows" },
  { href: "/metrics", label: "Métricas" },
  { href: "/insights", label: "Insights" },
  { href: "/lineage", label: "Linha de origem" },
  { href: "/reports", label: "Relatórios" },
  { href: "/alerts", label: "Alertas" },
  { href: "/settings", label: "Definições" },
  { href: "/billing", label: "Faturação" },
];

export function CommandPalette({ open, onClose }: { open: boolean; onClose: () => void }) {
  const router = useRouter();
  const [q, setQ] = useState("");
  const [active, setActive] = useState(0);
  const filtered = useMemo(
    () => items.filter((it) => it.label.toLowerCase().includes(q.trim().toLowerCase())),
    [q],
  );

  useEffect(() => {
    if (open) {
      setQ("");
      setActive(0);
    }
  }, [open]);

  useEffect(() => {
    setActive(0);
  }, [q]);

  if (!open) return null;

  function go(href: string) {
    router.push(href);
    onClose();
  }

  return (
    <div className="fixed inset-0 z-50 bg-ink/30 p-6 sm:p-24" onClick={onClose}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Procurar"
        className="mx-auto max-w-lg overflow-hidden rounded-2xl border border-line bg-white shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <input
          autoFocus
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Ir para…"
          aria-label="Procurar páginas"
          className="w-full bg-transparent px-4 py-3.5 text-sm text-ink outline-none placeholder:text-mute"
          onKeyDown={(e) => {
            if (e.key === "Escape") onClose();
            if (e.key === "ArrowDown") {
              e.preventDefault();
              setActive((i) => Math.min(i + 1, Math.max(filtered.length - 1, 0)));
            }
            if (e.key === "ArrowUp") {
              e.preventDefault();
              setActive((i) => Math.max(i - 1, 0));
            }
            if (e.key === "Enter" && filtered[active]) go(filtered[active].href);
          }}
        />
        <div className="border-t border-line py-1">
          {filtered.length === 0 && <p className="px-4 py-3 text-sm text-mute">Nenhum resultado.</p>}
          {filtered.map((it, i) => (
            <button
              key={it.href}
              className={`block min-h-10 w-full px-4 py-2.5 text-left text-sm ${i === active ? "bg-accent/10 text-accent" : "text-ink hover:bg-bg"}`}
              onMouseEnter={() => setActive(i)}
              onClick={() => go(it.href)}
            >
              {it.label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

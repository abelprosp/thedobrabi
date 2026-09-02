"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  Bell,
  Box,
  CreditCard,
  LayoutDashboard,
  Store,
  LineChart,
  MessageSquare,
  Search,
  Settings,
  Sparkles,
  Database,
  AlertTriangle,
  FileText,
  BarChart3,
  LogOut,
  GitBranch,
  Menu,
  Workflow,
  X,
  Plug,
  PanelLeftClose,
  PanelLeftOpen,
} from "lucide-react";
import { useEffect, useRef, useState, lazy, Suspense } from "react";
import { api, setTokens, clearTokens, getAccess } from "@/lib/api";
import { CommandPalette } from "@/components/command-palette";
import { Logo } from "@/components/brand";

const OnboardingModal = lazy(() => import("@/components/onboarding").then((m) => ({ default: m.OnboardingModal })));
const OnboardingSpotlight = lazy(() => import("@/components/onboarding").then((m) => ({ default: m.OnboardingSpotlight })));

const SIDEBAR_COLLAPSED_KEY = "thedobra.sidebar-collapsed";

const nav = [
  { href: "/overview", label: "Visão geral", icon: Sparkles },
  { href: "/dashboards", label: "Dashboards", icon: LayoutDashboard },
  { href: "/store", label: "Loja", icon: Store },
  { href: "/apps", label: "Apps", icon: Box },
  { href: "/ask", label: "Perguntar à TheDobra", icon: MessageSquare },
  { href: "/data", label: "Dados", icon: Database },
  { href: "/connectors", label: "Conectores", icon: Plug },
  { href: "/flows", label: "Flows", icon: Workflow },
  { href: "/metrics", label: "Métricas", icon: BarChart3 },
  { href: "/insights", label: "Insights", icon: LineChart },
  { href: "/lineage", label: "Linha de origem", icon: GitBranch },
  { href: "/reports", label: "Relatórios", icon: FileText },
  { href: "/alerts", label: "Alertas", icon: AlertTriangle },
];

export function AppShell({ children }: { children: React.ReactNode }) {
  const path = usePathname();
  const router = useRouter();
  const [me, setMe] = useState<{ name: string; email: string; org_name?: string; role?: string; workspace_id?: string } | null>(null);
  const [workspaces, setWorkspaces] = useState<{ id: string; name: string }[]>([]);
  const [wsId, setWsId] = useState("");
  const [open, setOpen] = useState(false);
  const [mobile, setMobile] = useState(false);
  const [menu, setMenu] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!getAccess()) {
      router.replace("/login");
      return;
    }
    Promise.all([
      api<any>("/api/v1/auth/me"),
      api<{ id: string; name: string }[]>("/api/v1/workspaces").catch(() => [] as { id: string; name: string }[]),
    ])
      .then(([u, list]) => {
        setMe(u);
        const ws = Array.isArray(list) ? list : [];
        setWorkspaces(ws);
        const stored = localStorage.getItem("thedobra.workspace") || "";
        const valid = ws.some((w) => w.id === stored) ? stored : u.workspace_id || ws[0]?.id || "";
        if (valid) {
          localStorage.setItem("thedobra.workspace", valid);
          setWsId(valid);
        }
      })
      .catch(() => router.replace("/login"));
  }, [router]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen(true);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  useEffect(() => {
    setMobile(false);
    setMenu(false);
  }, [path]);

  useEffect(() => {
    try {
      setCollapsed(localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === "true");
    } catch {
      /* ignore */
    }
  }, []);

  const toggleCollapsed = () => {
    setCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(next));
      } catch {
        /* ignore */
      }
      return next;
    });
  };

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setMenu(false);
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);

  const navLinkClass = (active: boolean, iconOnly: boolean) =>
    `flex min-h-10 items-center rounded-xl py-2 text-[13px] transition ${
      iconOnly ? "justify-center px-2" : "gap-2.5 px-3"
    } ${active ? "bg-primary/10 font-medium text-primary-600" : "text-mute hover:bg-surface-2 hover:text-ink"}`;

  const navItem = (item: (typeof nav)[number], iconOnly: boolean) => {
    const active = path === item.href || path.startsWith(item.href + "/");
    const Icon = item.icon;
    return (
      <Link
        key={item.href}
        href={item.href}
        title={iconOnly ? item.label : undefined}
        aria-current={active ? "page" : undefined}
        aria-label={iconOnly ? item.label : undefined}
        className={navLinkClass(active, iconOnly)}
      >
        <Icon size={16} className="shrink-0" aria-hidden />
        <span className={iconOnly ? "sr-only" : "truncate"}>{item.label}</span>
      </Link>
    );
  };

  const sidebarContent = (opts: { iconOnly: boolean; showCollapse: boolean }) => {
    const { iconOnly, showCollapse } = opts;
    const settingsActive = path.startsWith("/settings");
    const billingActive = path.startsWith("/billing");
    return (
      <>
        <div
          className={`flex shrink-0 ${
            iconOnly ? "flex-col items-center gap-1 px-2 py-3" : "items-center justify-between gap-1 px-3 py-4"
          }`}
        >
          <Link
            href="/overview"
            className={`flex items-center ${iconOnly ? "justify-center" : "px-2"}`}
            aria-label="TheDobra — visão geral"
          >
            <Logo variant="light" size={28} markOnly={iconOnly} />
          </Link>
          {showCollapse && (
            <button
              type="button"
              onClick={toggleCollapsed}
              aria-label={iconOnly ? "Expandir menu" : "Recolher menu"}
              aria-expanded={!iconOnly}
              title={iconOnly ? "Expandir menu" : "Recolher menu"}
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-mute hover:bg-surface-2 hover:text-ink"
            >
              {iconOnly ? <PanelLeftOpen size={18} aria-hidden /> : <PanelLeftClose size={18} aria-hidden />}
            </button>
          )}
        </div>
        <nav className={`flex-1 space-y-0.5 ${iconOnly ? "px-2" : "px-3"}`} aria-label="Principal">
          {nav.map((item) => navItem(item, iconOnly))}
        </nav>
        <div className={`space-y-0.5 pb-4 ${iconOnly ? "px-2" : "px-3"}`}>
          <Link
            href="/settings"
            title={iconOnly ? "Definições" : undefined}
            aria-label={iconOnly ? "Definições" : undefined}
            aria-current={settingsActive ? "page" : undefined}
            className={navLinkClass(settingsActive, iconOnly)}
          >
            <Settings size={16} className="shrink-0" aria-hidden />
            <span className={iconOnly ? "sr-only" : "truncate"}>Definições</span>
          </Link>
          <Link
            href="/billing"
            title={iconOnly ? "Faturação" : undefined}
            aria-label={iconOnly ? "Faturação" : undefined}
            aria-current={billingActive ? "page" : undefined}
            className={navLinkClass(billingActive, iconOnly)}
          >
            <CreditCard size={16} className="shrink-0" aria-hidden />
            <span className={iconOnly ? "sr-only" : "truncate"}>Faturação</span>
          </Link>
        </div>
      </>
    );
  };

  return (
    <div className="flex min-h-screen bg-bg">
      <aside
        className={`hidden flex-col overflow-x-hidden border-r border-line bg-white print:hidden transition-[width] duration-200 ease-in-out lg:flex ${
          collapsed ? "w-[72px]" : "w-60"
        }`}
      >
        {sidebarContent({ iconOnly: collapsed, showCollapse: true })}
      </aside>
      {mobile && (
        <div className="fixed inset-0 z-40 lg:hidden">
          <div className="absolute inset-0 bg-ink/30" onClick={() => setMobile(false)} />
          <aside className="relative z-10 flex h-full w-64 flex-col bg-white shadow-xl">
            <button
              className="absolute top-3 right-3 flex h-9 w-9 items-center justify-center rounded-lg text-mute hover:bg-surface-2"
              onClick={() => setMobile(false)}
              aria-label="Fechar menu"
            >
              <X size={18} />
            </button>
            {sidebarContent({ iconOnly: false, showCollapse: false })}
          </aside>
        </div>
      )}
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 items-center justify-between border-b border-line bg-white px-4 print:hidden sm:px-6">
          <div className="flex min-w-0 items-center gap-2 text-[13px] text-mute">
            <button
              className="flex h-9 w-9 items-center justify-center rounded-lg text-mute hover:bg-surface-2 lg:hidden"
              onClick={() => setMobile(true)}
              aria-label="Abrir menu"
            >
              <Menu size={18} />
            </button>
            <span className="hidden truncate sm:inline">{me?.org_name || "Organização"}</span>
            <span className="hidden text-line sm:inline">/</span>
            {workspaces.length > 0 ? (
              <select
                aria-label="Espaço de trabalho"
                className="max-w-[160px] rounded-lg border border-line bg-white px-2 py-1.5 text-[13px] text-ink outline-none"
                value={wsId || workspaces[0].id}
                onChange={async (e) => {
                  const id = e.target.value;
                  if (!id || id === wsId) return;
                  try {
                    const res = await api<{ tokens: { access_token: string; refresh_token: string; expires_in: number } }>(
                      `/api/v1/workspaces/${id}/switch`,
                      { method: "POST" },
                    );
                    setTokens(res.tokens);
                    localStorage.setItem("thedobra.workspace", id);
                    setWsId(id);
                    window.location.reload();
                  } catch (err: any) {
                    console.error(err);
                  }
                }}
              >
                {workspaces.map((w) => (
                  <option key={w.id} value={w.id}>
                    {w.name}
                  </option>
                ))}
              </select>
            ) : (
              <span>…</span>
            )}
          </div>
          <div className="flex items-center gap-1.5 sm:gap-2">
            <button
              onClick={() => setOpen(true)}
              aria-label="Procurar"
              className="flex h-9 items-center gap-2 rounded-lg border border-line bg-bg px-2.5 text-[12px] text-mute sm:px-3"
            >
              <Search size={14} />
              <span className="hidden sm:inline">Procurar</span>
              <kbd className="ml-2 hidden text-[10px] text-slate-400 sm:inline">⌘K</kbd>
            </button>
            <Link href="/ask" className="flex h-9 items-center rounded-lg bg-primary px-3 text-[12px] font-medium text-white hover:bg-primary-600">
              Perguntar
            </Link>
            <Link
              href="/alerts"
              className="flex h-9 w-9 items-center justify-center rounded-lg text-mute hover:bg-surface-2"
              aria-label="Alertas"
            >
              <Bell size={16} />
            </Link>
            <div className="relative" ref={menuRef}>
              <button
                onClick={() => setMenu((v) => !v)}
                className="ml-1 flex h-9 w-9 items-center justify-center rounded-full brand-gradient text-[12px] font-semibold text-white"
                aria-label="Menu da conta"
                aria-expanded={menu}
              >
                {(me?.name || "U").slice(0, 1).toUpperCase()}
              </button>
              {menu && (
                <div className="absolute right-0 z-20 mt-2 w-52 overflow-hidden rounded-xl border border-line bg-white py-1 shadow-lg">
                  <div className="border-b border-line px-3 py-2">
                    <div className="truncate text-[13px] font-medium text-ink">{me?.name}</div>
                    <div className="truncate text-[11px] text-mute">{me?.email}</div>
                  </div>
                  <Link href="/settings" className="flex min-h-10 items-center gap-2 px-3 text-sm text-ink hover:bg-bg">
                    <Settings size={14} /> Definições
                  </Link>
                  <button
                    className="flex min-h-10 w-full items-center gap-2 px-3 text-sm text-danger hover:bg-rose-50"
                    onClick={() => {
                      clearTokens();
                      router.replace("/login");
                    }}
                  >
                    <LogOut size={14} /> Sair
                  </button>
                </div>
              )}
            </div>
          </div>
        </header>
        <main className="min-w-0 flex-1 p-4 sm:p-6">{children}</main>
      </div>
      <CommandPalette open={open} onClose={() => setOpen(false)} />
      <Suspense fallback={null}>
        <OnboardingModal />
        <OnboardingSpotlight />
      </Suspense>
    </div>
  );
}

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
  ChevronDown,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState, lazy, Suspense, type ComponentType } from "react";
import { api, setTokens, clearTokens, getAccess } from "@/lib/api";
import { CommandPalette } from "@/components/command-palette";
import { Logo } from "@/components/brand";
import { ThemeToggle } from "@/components/theme-toggle";
import { useSystemTheme } from "@/components/theme-provider";

const OnboardingModal = lazy(() => import("@/components/onboarding").then((m) => ({ default: m.OnboardingModal })));
const OnboardingSpotlight = lazy(() => import("@/components/onboarding").then((m) => ({ default: m.OnboardingSpotlight })));

const SIDEBAR_COLLAPSED_KEY = "thedobra.sidebar-collapsed";
const NAV_GROUPS_KEY = "thedobra.nav-groups";

type NavItem = { href: string; label: string; icon: ComponentType<{ size?: number; className?: string; "aria-hidden"?: boolean }> };
type NavGroup = { id: string; label: string; items: NavItem[] };

const pinnedNav: NavItem[] = [
  { href: "/overview", label: "Início", icon: Sparkles },
  { href: "/ask", label: "Analisar com IA", icon: MessageSquare },
];

const mobilePrimaryNav: NavItem[] = [
  { href: "/overview", label: "Início", icon: Sparkles },
  { href: "/dashboards", label: "Painéis", icon: LayoutDashboard },
  { href: "/ask", label: "DobraAI", icon: MessageSquare },
  { href: "/connectors", label: "Dados", icon: Plug },
];

const navGroups: NavGroup[] = [
  {
    id: "analise",
    label: "Criar e analisar",
    items: [
      { href: "/dashboards", label: "Dashboards", icon: LayoutDashboard },
      { href: "/store", label: "Loja", icon: Store },
      { href: "/reports", label: "Relatórios", icon: FileText },
      { href: "/apps", label: "Apps", icon: Box },
    ],
  },
  {
    id: "dados",
    label: "Fontes e dados",
    items: [
      { href: "/connectors", label: "Conectores", icon: Plug },
      { href: "/data", label: "Conjuntos de dados", icon: Database },
      { href: "/flows", label: "Flows", icon: Workflow },
      { href: "/lineage", label: "Linhagem", icon: GitBranch },
    ],
  },
  {
    id: "monitor",
    label: "Monitoramento",
    items: [
      { href: "/metrics", label: "Métricas", icon: BarChart3 },
      { href: "/insights", label: "Insights", icon: LineChart },
      { href: "/alerts", label: "Alertas", icon: AlertTriangle },
    ],
  },
];

function pathMatches(path: string, href: string) {
  return path === href || path.startsWith(href + "/");
}

function groupIdForPath(path: string) {
  return navGroups.find((g) => g.items.some((i) => pathMatches(path, i.href)))?.id;
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const path = usePathname();
  const router = useRouter();
  const { theme } = useSystemTheme();
  const [me, setMe] = useState<{ name: string; email: string; org_name?: string; role?: string; workspace_id?: string } | null>(null);
  const [brand, setBrand] = useState<{ brand_name?: string; brand_logo_url?: string }>({});
  const [workspaces, setWorkspaces] = useState<{ id: string; name: string }[]>([]);
  const [wsId, setWsId] = useState("");
  const [open, setOpen] = useState(false);
  const [mobile, setMobile] = useState(false);
  const [menu, setMenu] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const [openGroups, setOpenGroups] = useState<string[]>(["analise"]);
  const menuRef = useRef<HTMLDivElement>(null);
  const prevPathRef = useRef<string | null>(null);
  const activeGroupId = useMemo(() => groupIdForPath(path), [path]);

  useEffect(() => {
    if (!getAccess()) {
      router.replace("/login");
      return;
    }
    Promise.all([
      api<any>("/api/v1/auth/me"),
      api<{ id: string; name: string }[]>("/api/v1/workspaces").catch(() => [] as { id: string; name: string }[]),
      api<{ brand_name?: string; brand_logo_url?: string }>("/api/v1/organizations/current").catch(() => ({})),
    ])
      .then(([u, list, org]) => {
        setMe(u);
        setBrand(org || {});
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
      const raw = localStorage.getItem(NAV_GROUPS_KEY);
      let saved: string[] | null = null;
      if (raw) {
        const parsed = JSON.parse(raw);
        if (Array.isArray(parsed) && parsed.every((x) => typeof x === "string")) saved = parsed;
      }
      const current = groupIdForPath(path);
      if (saved) {
        setOpenGroups(current && !saved.includes(current) ? [...saved, current] : saved);
      } else if (current) {
        setOpenGroups([current]);
      }
    } catch {
      /* ignore */
    }
  }, []);

  useEffect(() => {
    if (prevPathRef.current === null) {
      prevPathRef.current = path;
      return;
    }
    if (prevPathRef.current === path) return;
    prevPathRef.current = path;
    if (!activeGroupId) return;
    setOpenGroups((prev) => (prev.includes(activeGroupId) ? prev : [...prev, activeGroupId]));
  }, [path, activeGroupId]);

  const persistGroups = (next: string[]) => {
    setOpenGroups(next);
    try {
      localStorage.setItem(NAV_GROUPS_KEY, JSON.stringify(next));
    } catch {
      /* ignore */
    }
  };

  const toggleGroup = (id: string) => {
    persistGroups(openGroups.includes(id) ? openGroups.filter((g) => g !== id) : [...openGroups, id]);
  };

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
    `group flex min-h-10 items-center rounded-xl py-2 text-[13px] transition-all ${
      iconOnly ? "justify-center px-2" : "gap-2.5 px-3"
    } ${active ? "bg-primary/10 font-semibold text-primary-600 shadow-[inset_3px_0_0_var(--color-primary)]" : "text-mute hover:translate-x-0.5 hover:bg-surface-2 hover:text-ink"}`;

  const navItem = (item: NavItem, iconOnly: boolean) => {
    const active = pathMatches(path, item.href);
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
            aria-label={`${brand.brand_name || "TheDobra"} — visão geral`}
          >
            <Logo
              variant={theme === "dark" ? "dark" : "light"}
              size={28}
              markOnly={iconOnly}
              brandName={brand.brand_name}
              brandLogoUrl={brand.brand_logo_url}
            />
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
        <nav className={`min-h-0 flex-1 overflow-y-auto ${iconOnly ? "space-y-0.5 px-2" : "space-y-4 px-3"}`} aria-label="Principal">
          <div className="space-y-0.5">
            {pinnedNav.map((item) => navItem(item, iconOnly))}
          </div>
          {navGroups.map((group) => {
            const groupActive = group.items.some((i) => pathMatches(path, i.href));
            const expanded = iconOnly || openGroups.includes(group.id);
            return (
              <div key={group.id} className="space-y-0.5">
                {iconOnly ? (
                  <div className="mx-2 my-1.5 h-px bg-line" aria-hidden />
                ) : (
                  <button
                    type="button"
                    onClick={() => toggleGroup(group.id)}
                    aria-expanded={expanded}
                    className={`flex w-full items-center justify-between rounded-lg px-2 py-1.5 text-[10px] font-semibold uppercase tracking-[0.1em] transition ${
                      groupActive ? "text-primary-600" : "text-mute hover:text-ink"
                    }`}
                  >
                    {group.label}
                    <ChevronDown
                      size={14}
                      className={`shrink-0 transition-transform ${expanded ? "" : "-rotate-90"}`}
                      aria-hidden
                    />
                  </button>
                )}
                {expanded && (
                  <div className={iconOnly ? "space-y-0.5" : "ml-1.5 space-y-0.5 border-l border-line pl-1.5"}>
                    {group.items.map((item) => navItem(item, iconOnly))}
                  </div>
                )}
              </div>
            );
          })}
        </nav>
        <div className={`shrink-0 space-y-0.5 border-t border-line pb-4 pt-2 ${iconOnly ? "px-2" : "px-3"}`}>
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
    <div className="flex min-h-screen bg-transparent">
      <a
        href="#main-content"
        className="fixed left-3 top-3 z-[100] -translate-y-20 rounded-xl bg-primary px-4 py-2 text-sm font-medium text-white shadow-lg transition focus:translate-y-0"
      >
        Ir para o conteúdo
      </a>
      <aside
        className={`hidden min-h-0 flex-col overflow-hidden border-r border-line/80 bg-surface/95 print:hidden transition-[width] duration-200 ease-in-out lg:flex ${
          collapsed ? "w-[72px]" : "w-64"
        }`}
      >
        {sidebarContent({ iconOnly: collapsed, showCollapse: true })}
      </aside>
      {mobile && (
        <div className="fixed inset-0 z-40 lg:hidden">
          <div className="absolute inset-0 bg-ink/30" onClick={() => setMobile(false)} />
          <aside className="relative z-10 flex h-full min-h-0 w-[min(18rem,88vw)] flex-col overflow-hidden bg-surface pb-[env(safe-area-inset-bottom)] pt-[env(safe-area-inset-top)] shadow-xl">
            <button
              className="absolute top-3 right-3 flex h-10 w-10 items-center justify-center rounded-lg text-mute hover:bg-surface-2"
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
        <header className="sticky top-0 z-30 flex h-16 items-center justify-between gap-2 border-b border-line/80 bg-surface/90 px-3 backdrop-blur-xl print:hidden sm:px-6">
          <div className="flex min-w-0 items-center gap-2 text-[13px] text-mute">
            <button
              className="flex h-10 w-10 items-center justify-center rounded-lg text-mute hover:bg-surface-2 lg:hidden"
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
                className="max-w-[7.5rem] rounded-lg border border-line bg-surface px-2 py-1.5 text-[13px] text-ink outline-none sm:max-w-[160px]"
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
              className="flex h-10 items-center gap-2 rounded-xl border border-line bg-bg/80 px-2.5 text-[12px] text-mute shadow-sm transition hover:border-primary/25 hover:text-ink sm:h-9 sm:min-w-36 sm:px-3"
            >
              <Search size={14} />
              <span className="hidden sm:inline">Procurar</span>
              <kbd className="ml-2 hidden text-[10px] text-slate-400 sm:inline">⌘K</kbd>
            </button>
            <div className="hidden sm:block">
              <ThemeToggle />
            </div>
            <Link href="/ask" className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary text-white shadow-sm shadow-primary/20 transition hover:-translate-y-px hover:bg-primary-600 sm:h-9 sm:w-auto sm:px-3 sm:text-[12px] sm:font-medium">
              <MessageSquare size={16} className="sm:hidden" />
              <span className="hidden sm:inline">Perguntar</span>
            </Link>
            <Link
              href="/alerts"
              className="hidden h-9 w-9 items-center justify-center rounded-lg text-mute hover:bg-surface-2 sm:flex"
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
                <div className="absolute right-0 z-20 mt-2 w-52 overflow-hidden rounded-xl border border-line bg-surface py-1 shadow-lg">
                  <div className="border-b border-line px-3 py-2">
                    <div className="truncate text-[13px] font-medium text-ink">{me?.name}</div>
                    <div className="truncate text-[11px] text-mute">{me?.email}</div>
                  </div>
                  <Link href="/settings" className="flex min-h-10 items-center gap-2 px-3 text-sm text-ink hover:bg-bg">
                    <Settings size={14} /> Definições
                  </Link>
                  <button
                    className="flex min-h-10 w-full items-center gap-2 px-3 text-sm text-danger hover:bg-rose-50 dark:hover:bg-rose-500/10"
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
        <main id="main-content" className="min-w-0 flex-1 px-3 py-5 pb-[calc(5.5rem+env(safe-area-inset-bottom))] sm:p-7 lg:pb-7">{children}</main>
        <nav
          aria-label="Navegação rápida"
          className="fixed inset-x-0 bottom-0 z-30 grid grid-cols-4 border-t border-line/80 bg-surface/95 px-2 pb-[env(safe-area-inset-bottom)] shadow-[0_-8px_30px_rgba(15,23,42,0.08)] backdrop-blur-xl lg:hidden"
        >
          {mobilePrimaryNav.map((item) => {
            const active = pathMatches(path, item.href);
            const Icon = item.icon;
            return (
              <Link
                key={item.href}
                href={item.href}
                aria-current={active ? "page" : undefined}
                className={`flex min-h-16 flex-col items-center justify-center gap-1 rounded-xl text-[10px] font-medium transition ${
                  active ? "text-primary" : "text-mute"
                }`}
              >
                <span className={`flex h-8 w-10 items-center justify-center rounded-xl ${active ? "bg-primary/10" : ""}`}>
                  <Icon size={17} aria-hidden />
                </span>
                {item.label}
              </Link>
            );
          })}
        </nav>
      </div>
      <CommandPalette open={open} onClose={() => setOpen(false)} />
      <Suspense fallback={null}>
        <OnboardingModal />
        <OnboardingSpotlight />
      </Suspense>
    </div>
  );
}

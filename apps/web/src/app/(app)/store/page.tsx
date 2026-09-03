"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import {
  AlertTriangle,
  Headphones,
  Megaphone,
  Package,
  Percent,
  Repeat,
  Search,
  ShoppingBag,
  ShoppingCart,
  Store,
  Target,
  TrendingUp,
  Truck,
  Users,
  Wallet,
  X,
  Zap,
} from "lucide-react";
import { api, normalizeArray } from "@/lib/api";
import { Badge, Button, Card, EmptyState, FieldLabel, PageSkeleton, Select } from "@/components/ui";
import {
  CATEGORY_LABEL,
  DASHBOARD_TEMPLATES,
  STORE_CATEGORIES,
  instantiateTemplate,
  prepareTemplateModel,
  type DashboardTemplate,
  type StoreCategory,
  type StoreIcon,
} from "@/lib/dashboard-templates";
import { modelFromSemanticRow, type DatasetListItem, type SemanticModel } from "@/lib/semantic";

const ICONS: Record<StoreIcon, typeof Wallet> = {
  wallet: Wallet,
  trending: TrendingUp,
  target: Target,
  cart: ShoppingCart,
  users: Users,
  package: Package,
  megaphone: Megaphone,
  truck: Truck,
  repeat: Repeat,
  shopping: ShoppingBag,
  headset: Headphones,
  alert: AlertTriangle,
  percent: Percent,
};

export default function StorePage() {
  const router = useRouter();
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<{ role?: string }>("/api/v1/auth/me") });
  const canActivate = !me.data || me.data.role !== "viewer";
  const datasets = useQuery({ queryKey: ["datasets"], queryFn: () => api<any>("/api/v1/datasets") });
  const datasetList = normalizeArray<DatasetListItem>(datasets.data);

  const [category, setCategory] = useState<StoreCategory | "todos">("todos");
  const [q, setQ] = useState("");
  const [picked, setPicked] = useState<DashboardTemplate | null>(null);
  const [datasetId, setDatasetId] = useState("");

  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return DASHBOARD_TEMPLATES.filter((t) => {
      if (category !== "todos" && t.category !== category) return false;
      if (!needle) return true;
      const hay = `${t.name} ${t.description} ${t.pain} ${CATEGORY_LABEL[t.category]} ${t.needs.join(" ")}`.toLowerCase();
      return hay.includes(needle);
    });
  }, [category, q]);

  const featured = DASHBOARD_TEMPLATES.filter((t) => t.popular);

  const activate = useMutation({
    mutationFn: async () => {
      if (!picked) throw new Error("Escolha um painel");
      const dsId = datasetId || datasetList[0]?.id;
      if (!dsId) throw new Error("Ligue um conjunto de dados primeiro");
      const ds = await api<{ name: string; semantic_model?: SemanticModel | { model?: SemanticModel } }>(`/api/v1/datasets/${dsId}`);
      const model =
        (await prepareTemplateModel(dsId, picked)) ||
        (ds.semantic_model && "measures" in ds.semantic_model ? ds.semantic_model : null) ||
        modelFromSemanticRow(ds.semantic_model);
      const widgets = instantiateTemplate(picked, dsId, model);
      return api<{ id: string }>("/api/v1/dashboards", {
        method: "POST",
        body: JSON.stringify({
          name: picked.name,
          description: `Painel pronto · ${CATEGORY_LABEL[picked.category]} · ${ds.name || "conjunto"}`,
          layout: { widgets },
        }),
      });
    },
    onSuccess: (d) => {
      toast.success("Painel ativado com os seus dados");
      setPicked(null);
      router.push(`/dashboards/${d.id}`);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  useEffect(() => {
    if (!picked) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !activate.isPending) setPicked(null);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [picked, activate.isPending]);

  function openActivate(tpl: DashboardTemplate) {
    if (!canActivate) {
      toast.error("Viewers não podem criar dashboards.");
      return;
    }
    setPicked(tpl);
    setDatasetId(datasetList[0]?.id || "");
  }

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <section className="panel-gradient relative overflow-hidden rounded-3xl px-6 py-8 text-white sm:px-8">
        <div className="grid-fade pointer-events-none absolute inset-0 opacity-50" />
        <div className="pointer-events-none absolute -top-24 -right-16 h-64 w-64 rounded-full bg-indigo-500/30 blur-3xl" />
        <div className="relative z-10 grid gap-8 lg:grid-cols-[1.1fr_0.9fr] lg:items-center">
          <div>
            <p className="inline-flex items-center gap-1.5 rounded-full bg-white/10 px-3 py-1 text-[11px] font-medium uppercase tracking-[0.16em] text-indigo-100">
              <Store size={12} /> Loja TheDobra
            </p>
            <h1 className="mt-3 text-2xl font-semibold tracking-tight sm:text-3xl">Painéis prontos que ativa em minutos</h1>
            <p className="mt-2 max-w-xl text-sm text-white/80">
              Não construa do zero. Escolha o rito da área — financeiro, comercial, RH, e-commerce — ligue o conjunto e o dashboard já nasce com as suas colunas.
            </p>
            <div className="mt-5 grid max-w-md grid-cols-2 gap-2">
              <div className="rounded-2xl border border-white/15 bg-white/5 px-4 py-3">
                <div className="text-lg font-semibold">Semanas</div>
                <div className="text-[12px] text-white/70">a desenhar widgets à mão</div>
              </div>
              <div className="rounded-2xl border border-white/25 bg-white px-4 py-3 text-ink shadow-lg">
                <div className="text-lg font-semibold text-primary">Minutos</div>
                <div className="text-[12px] text-mute">a ativar um painel pronto</div>
              </div>
            </div>
          </div>
          <div className="hidden rounded-2xl border border-white/15 bg-white/10 p-3 lg:block">
            <div className="mb-2 flex items-center justify-between px-1 text-[12px] text-white/80">
              <span className="font-medium">Em destaque</span>
              <span>{DASHBOARD_TEMPLATES.length} painéis</span>
            </div>
            <div className="space-y-2">
              {featured.slice(0, 3).map((t) => {
                const Icon = ICONS[t.icon];
                return (
                  <button
                    key={t.id}
                    type="button"
                    onClick={() => openActivate(t)}
                    className="flex w-full items-center gap-3 rounded-xl bg-white px-3 py-2.5 text-left text-ink shadow-sm transition hover:bg-indigo-50"
                  >
                    <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
                      <Icon size={16} />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-[13px] font-medium">{t.name}</span>
                      <span className="text-[11px] text-mute">
                        {CATEGORY_LABEL[t.category]} · {t.needs[0]}
                      </span>
                    </span>
                    <span className="rounded-lg bg-primary/10 px-2.5 py-1 text-[11px] font-medium text-primary">Ativar</span>
                  </button>
                );
              })}
            </div>
          </div>
        </div>
      </section>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 flex-1 items-center gap-2 rounded-xl border border-line bg-white px-3 py-2">
          <Search size={16} className="shrink-0 text-mute" />
          <input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Procurar por área, dor ou métrica…"
            aria-label="Procurar painéis"
            className="min-w-0 flex-1 bg-transparent text-sm text-ink outline-none placeholder:text-mute"
          />
        </div>
        <p className="text-[12px] text-mute">{filtered.length} disponíveis</p>
      </div>

      <div className="flex flex-wrap gap-1.5">
        {STORE_CATEGORIES.map((c) => (
          <button
            key={c.id}
            type="button"
            onClick={() => setCategory(c.id)}
            className={`rounded-full px-3.5 py-1.5 text-[13px] font-medium transition ${
              category === c.id ? "bg-primary text-white shadow-sm" : "border border-line bg-white text-mute hover:border-primary/40 hover:text-ink"
            }`}
          >
            {c.label}
          </button>
        ))}
      </div>

      {datasets.isLoading && <PageSkeleton cards={6} />}

      {!datasets.isLoading && filtered.length === 0 && (
        <EmptyState icon={Store} title="Nenhum painel nesta pesquisa" description="Tente outra área ou um termo mais curto." />
      )}

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
        {filtered.map((t) => {
          const Icon = ICONS[t.icon];
          return (
            <Card key={t.id} className="flex h-full flex-col gap-3 transition hover:border-primary/30">
              <div className="flex items-start gap-3">
                <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                  <Icon size={18} />
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <h2 className="text-sm font-medium text-ink">{t.name}</h2>
                    {t.popular && <Badge tone="accent">Popular</Badge>}
                  </div>
                  <p className="mt-0.5 text-[12px] text-mute">{CATEGORY_LABEL[t.category]}</p>
                </div>
              </div>
              <p className="text-[13px] text-ink/80">{t.description}</p>
              <p className="text-[12px] text-mute">{t.pain}</p>
              <div className="mt-auto flex flex-wrap gap-1">
                {t.needs.map((n) => (
                  <span key={n} className="rounded-full bg-surface-2 px-2 py-0.5 text-[10px] text-mute">
                    {n}
                  </span>
                ))}
              </div>
              <Button className="w-full" onClick={() => openActivate(t)} disabled={!canActivate}>
                <Zap size={14} /> Ativar
              </Button>
            </Card>
          );
        })}
      </div>

      {picked && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-ink/40 p-4" onClick={() => !activate.isPending && setPicked(null)}>
          <div
            role="dialog"
            aria-modal="true"
            aria-label={`Ativar ${picked.name}`}
            className="w-full max-w-lg"
            onClick={(e) => e.stopPropagation()}
          >
          <Card className="space-y-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <h3 className="text-lg font-medium text-ink">Ativar «{picked.name}»</h3>
                <p className="mt-1 text-[13px] text-mute">
                  {picked.measures?.length
                    ? `Liga o conjunto com ${picked.needs.join(", ")}. As ${picked.measures.length} medidas SQL são gravadas no modelo automaticamente.`
                    : "Só precisa de ligar um conjunto. As colunas (valor, categoria, mês, etc.) são mapeadas sozinhas."}
                </p>
              </div>
              <button type="button" className="rounded-lg p-1 text-mute hover:bg-surface-2" onClick={() => setPicked(null)} aria-label="Fechar">
                <X size={18} />
              </button>
            </div>
            <div className="space-y-3">
              {datasetList.length === 0 ? (
                <div className="rounded-xl border border-amber-200 bg-amber-50 p-3 text-[13px] text-amber-900">
                  Ainda não há conjuntos neste espaço. Carregue um CSV em Dados ou ligue um conector, e volte aqui.
                  <div className="mt-2 flex gap-2">
                    <Link href="/data">
                      <Button size="sm" variant="secondary">Dados</Button>
                    </Link>
                    <Link href="/connectors">
                      <Button size="sm" variant="secondary">Conectores</Button>
                    </Link>
                  </div>
                </div>
              ) : (
                <FieldLabel label="Conjunto de dados" hint="O painel usa as medidas e dimensões deste conjunto.">
                  <Select value={datasetId} onChange={(e) => setDatasetId(e.target.value)}>
                    {datasetList.map((ds) => (
                      <option key={ds.id} value={ds.id}>
                        {ds.name}
                      </option>
                    ))}
                  </Select>
                </FieldLabel>
              )}
              <div className="rounded-xl border border-line bg-surface-2 px-3 py-2 text-[12px] text-mute">
                Widgets: {picked.widgets.length} · Área: {CATEGORY_LABEL[picked.category]}
              </div>
              <div className="flex justify-end gap-2">
                <Button variant="secondary" onClick={() => setPicked(null)} disabled={activate.isPending}>
                  Cancelar
                </Button>
                <Button onClick={() => activate.mutate()} busy={activate.isPending} disabled={!datasetList.length}>
                  <Zap size={14} /> Ativar painel
                </Button>
              </div>
            </div>
          </Card>
          </div>
        </div>
      )}
    </div>
  );
}

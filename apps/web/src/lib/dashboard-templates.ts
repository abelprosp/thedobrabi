import type { Widget, WidgetType } from "@/components/WidgetView";
import { api } from "@/lib/api";
import {
  measureKey,
  modelForDataset,
  modelIdForDataset,
  remapQueryToModel,
  type SemanticMeasure,
  type SemanticModel,
} from "@/lib/semantic";

export type StoreCategory =
  | "financeiro"
  | "comercial"
  | "ecommerce"
  | "rh"
  | "operacoes"
  | "marketing"
  | "logistica"
  | "saas"
  | "compras"
  | "atendimento";

export type StoreIcon =
  | "wallet"
  | "trending"
  | "target"
  | "cart"
  | "users"
  | "package"
  | "megaphone"
  | "truck"
  | "repeat"
  | "shopping"
  | "headset"
  | "alert"
  | "percent";

export type TemplateWidget = Omit<Widget, "id">;

export type DashboardTemplate = {
  id: string;
  name: string;
  category: StoreCategory;
  description: string;
  pain: string;
  icon: StoreIcon;
  popular?: boolean;
  needs: string[];
  /** Custom SQL measures written onto the semantic model when the template is applied. */
  measures?: SemanticMeasure[];
  widgets: TemplateWidget[];
};

export const STORE_CATEGORIES: { id: StoreCategory | "todos"; label: string }[] = [
  { id: "todos", label: "Todos" },
  { id: "financeiro", label: "Financeiro" },
  { id: "comercial", label: "Comercial" },
  { id: "ecommerce", label: "E-commerce" },
  { id: "rh", label: "RH" },
  { id: "operacoes", label: "Operações" },
  { id: "marketing", label: "Marketing" },
  { id: "logistica", label: "Logística" },
  { id: "saas", label: "SaaS" },
  { id: "compras", label: "Compras" },
  { id: "atendimento", label: "Atendimento" },
];

export const CATEGORY_LABEL: Record<StoreCategory, string> = Object.fromEntries(
  STORE_CATEGORIES.filter((c) => c.id !== "todos").map((c) => [c.id, c.label]),
) as Record<StoreCategory, string>;

type Q = NonNullable<Widget["query"]>;

function w(
  type: WidgetType,
  title: string,
  layout: Widget["layout"],
  query?: Partial<Q>,
  extra?: Partial<TemplateWidget>,
): TemplateWidget {
  return {
    type,
    title,
    layout,
    query: query
      ? {
          measures: query.measures || [],
          dimensions: query.dimensions || [],
          filters: query.filters,
          limit: query.limit,
        }
      : undefined,
    ...extra,
  };
}

const brl = { currency: "BRL" as const, compact: "auto" as const };
const brlFull = { currency: "BRL" as const, compact: "none" as const, decimals: 0 };
const brlTicket = { currency: "BRL" as const, compact: "none" as const, decimals: 2 };
const pct1 = { suffix: "%", decimals: 1 };

const CONTRATOS_MEASURES: SemanticMeasure[] = [
  { name: "Receita", expression: "SUM(valor_mensal)", aggregation: "expression" },
  { name: "Contratos", expression: "COUNT(*)", aggregation: "expression" },
  { name: "Ticket médio", expression: "AVERAGE(valor_mensal)", aggregation: "expression" },
  { name: "Clientes", expression: "DISTINCTCOUNT(cliente)", aggregation: "expression" },
  {
    name: "Variação da receita",
    expression:
      "(SUM(CASE WHEN mes = '2026-08' THEN valor_mensal ELSE 0 END) - SUM(CASE WHEN mes = '2026-07' THEN valor_mensal ELSE 0 END)) / NULLIF(SUM(CASE WHEN mes = '2026-07' THEN valor_mensal ELSE 0 END), 0) * 100",
    aggregation: "expression",
  },
  {
    name: "Variação do ticket",
    expression:
      "(AVG(CASE WHEN mes = '2026-08' THEN valor_mensal END) - AVG(CASE WHEN mes = '2026-07' THEN valor_mensal END)) / NULLIF(AVG(CASE WHEN mes = '2026-07' THEN valor_mensal END), 0) * 100",
    aggregation: "expression",
  },
  { name: "Linhas julho", expression: "SUM(CASE WHEN mes = '2026-07' THEN 1 ELSE 0 END)", aggregation: "expression" },
  { name: "Linhas agosto", expression: "SUM(CASE WHEN mes = '2026-08' THEN 1 ELSE 0 END)", aggregation: "expression" },
  { name: "Clientes julho", expression: "COUNT(DISTINCT CASE WHEN mes = '2026-07' THEN cliente END)", aggregation: "expression" },
  { name: "Clientes agosto", expression: "COUNT(DISTINCT CASE WHEN mes = '2026-08' THEN cliente END)", aggregation: "expression" },
];

export const DASHBOARD_TEMPLATES: DashboardTemplate[] = [
  {
    id: "financeiro-dre",
    name: "P&L sob controlo",
    category: "financeiro",
    description: "DRE executivo: receita, despesa e resultado por categoria, linha e empresa.",
    pain: "Receita e despesa misturadas no mesmo total — o P&L fica ilegível.",
    icon: "wallet",
    popular: true,
    needs: ["valor", "categoria ou linha", "mês ou data"],
    widgets: [
      w("kpi", "Receita", { x: 0, y: 0, w: 3, h: 2 }, { measures: ["revenue"], filters: [{ dimension: "natureza", op: "eq", value: "Receita" }] }, { config: brl }),
      w("kpi", "Despesa", { x: 3, y: 0, w: 3, h: 2 }, { measures: ["cost"], filters: [{ dimension: "natureza", op: "eq", value: "Despesa" }] }, { config: brl }),
      w("kpi", "Resultado", { x: 6, y: 0, w: 3, h: 2 }, { measures: ["valor"] }, { config: brl }),
      w("slicer", "Empresa", { x: 9, y: 0, w: 3, h: 2 }, { dimensions: ["company"], measures: [], limit: 200 }),
      w("line", "Evolução no tempo", { x: 0, y: 2, w: 8, h: 5 }, { measures: ["valor"], dimensions: ["date"], limit: 24 }),
      w("pie", "Por natureza", { x: 8, y: 2, w: 4, h: 5 }, { measures: ["valor"], dimensions: ["natureza"], limit: 8 }),
      w("bar", "Por categoria", { x: 0, y: 7, w: 6, h: 5 }, { measures: ["valor"], dimensions: ["category"], limit: 12 }),
      w("table", "Por linha do DRE", { x: 6, y: 7, w: 6, h: 5 }, { measures: ["valor"], dimensions: ["linha"], limit: 40 }),
    ],
  },
  {
    id: "financeiro-inadimplencia",
    name: "Inadimplência sob controlo",
    category: "financeiro",
    description: "Atraso, cobrança e concentração de risco por cliente e status.",
    pain: "Só descobre o atraso quando o caixa já apertou.",
    icon: "alert",
    popular: true,
    needs: ["valor", "cliente ou status"],
    widgets: [
      w("kpi", "Em aberto", { x: 0, y: 0, w: 3, h: 2 }, { measures: ["valor"] }, { config: brl }),
      w("kpi", "Clientes", { x: 3, y: 0, w: 3, h: 2 }, { measures: ["customers"] }),
      w("slicer", "Status", { x: 6, y: 0, w: 6, h: 2 }, { dimensions: ["status"], measures: [], limit: 200 }),
      w("bar", "Por cliente", { x: 0, y: 2, w: 7, h: 5 }, { measures: ["valor"], dimensions: ["customer"], limit: 15 }),
      w("pie", "Por status", { x: 7, y: 2, w: 5, h: 5 }, { measures: ["valor"], dimensions: ["status"], limit: 8 }),
      w("table", "Detalhe", { x: 0, y: 7, w: 12, h: 5 }, { measures: ["valor"], dimensions: ["customer"], limit: 50 }),
    ],
  },
  {
    id: "financeiro-orcamento",
    name: "Orçamento vs realizado",
    category: "financeiro",
    description: "Desvios por categoria e período para fechar o mês sem surpresa.",
    pain: "O realizado foge do budget e ninguém vê a tempo.",
    icon: "percent",
    needs: ["valor", "categoria", "mês"],
    widgets: [
      w("kpi", "Realizado", { x: 0, y: 0, w: 4, h: 2 }, { measures: ["valor"] }, { config: brl }),
      w("kpi", "Despesa", { x: 4, y: 0, w: 4, h: 2 }, { measures: ["cost"] }, { config: brl }),
      w("sparkline", "Tendência", { x: 8, y: 0, w: 4, h: 2 }, { measures: ["valor"], dimensions: ["date"] }),
      w("bar", "Por categoria", { x: 0, y: 2, w: 6, h: 5 }, { measures: ["valor"], dimensions: ["category"], limit: 12 }),
      w("line", "Mês a mês", { x: 6, y: 2, w: 6, h: 5 }, { measures: ["valor"], dimensions: ["date"], limit: 24 }),
      w("table", "Linhas", { x: 0, y: 7, w: 12, h: 5 }, { measures: ["valor"], dimensions: ["linha"], limit: 40 }),
    ],
  },
  {
    id: "comercial-contratos-jul-ago",
    name: "Contratos julho vs agosto",
    category: "comercial",
    description: "Receita, ticket, linhas e clientes lado a lado — julho contra agosto, já com as medidas SQL.",
    pain: "Os dois meses estão no mesmo CSV e ninguém vê o que caiu sem montar a conta à mão.",
    icon: "repeat",
    popular: true,
    needs: ["valor_mensal", "mes", "cliente", "vendedor"],
    measures: CONTRATOS_MEASURES,
    widgets: [
      w("kpi", "Receita", { x: 0, y: 0, w: 3, h: 2 }, { measures: ["Receita"] }, { config: brlFull }),
      w("kpi", "Contratos", { x: 3, y: 0, w: 3, h: 2 }, { measures: ["Contratos"] }),
      w("kpi", "Ticket médio", { x: 6, y: 0, w: 3, h: 2 }, { measures: ["Ticket médio"] }, { config: brlTicket }),
      w("kpi", "Clientes", { x: 9, y: 0, w: 3, h: 2 }, { measures: ["Clientes"] }),
      w("kpi", "Variação da receita", { x: 0, y: 2, w: 3, h: 2 }, { measures: ["Variação da receita"] }, { config: pct1 }),
      w("kpi", "Variação do ticket", { x: 3, y: 2, w: 3, h: 2 }, { measures: ["Variação do ticket"] }, { config: pct1 }),
      w("kpi", "Linhas julho", { x: 6, y: 2, w: 3, h: 2 }, { measures: ["Linhas julho"] }),
      w("kpi", "Linhas agosto", { x: 9, y: 2, w: 3, h: 2 }, { measures: ["Linhas agosto"] }),
      w("slicer", "Mês", { x: 0, y: 4, w: 3, h: 2 }, { dimensions: ["mes"], measures: [], limit: 200 }, { config: { slicerStyle: "buttons" } }),
      w("slicer", "Vendedor", { x: 3, y: 4, w: 3, h: 2 }, { dimensions: ["vendedor"], measures: [], limit: 200 }, { config: { slicerStyle: "dropdown" } }),
      w("slicer", "Cliente", { x: 6, y: 4, w: 3, h: 2 }, { dimensions: ["cliente"], measures: [], limit: 200 }, { config: { slicerStyle: "dropdown", slicerSearch: true } }),
      w("slicer", "Luxus", { x: 9, y: 4, w: 3, h: 2 }, { dimensions: ["cliente_luxus"], measures: [], limit: 200 }, { config: { slicerStyle: "buttons" } }),
      w("bar", "Receita por mês", { x: 0, y: 6, w: 6, h: 4 }, { measures: ["Receita"], dimensions: ["mes"], limit: 12 }, { config: { ...brlFull, showDataLabels: true } }),
      w("bar", "Contratos por mês", { x: 6, y: 6, w: 6, h: 4 }, { measures: ["Contratos"], dimensions: ["mes"], limit: 12 }, { config: { showDataLabels: true } }),
      w("line", "Receita por dia", { x: 0, y: 10, w: 12, h: 4 }, { measures: ["Receita"], dimensions: ["data_venda"], limit: 90 }, { config: brlFull }),
      w("bar", "Receita por vendedor", { x: 0, y: 14, w: 6, h: 4 }, { measures: ["Receita"], dimensions: ["vendedor", "mes"], limit: 15 }, { config: brlFull }),
      w("bar", "Ticket por vendedor", { x: 6, y: 14, w: 6, h: 4 }, { measures: ["Ticket médio"], dimensions: ["vendedor"], limit: 15 }, { config: brlTicket }),
      w("pie", "Mix de clientes", { x: 0, y: 18, w: 4, h: 4 }, { measures: ["Receita"], dimensions: ["cliente"], limit: 8 }, { config: brlFull }),
      w("treemap", "Peso dos clientes", { x: 4, y: 18, w: 8, h: 4 }, { measures: ["Receita"], dimensions: ["cliente"], limit: 20 }, { config: brlFull }),
      w("table", "Detalhe", { x: 0, y: 22, w: 12, h: 5 }, { measures: ["Receita", "Contratos"], dimensions: ["cliente", "vendedor", "mes"], limit: 20 }, { config: { ...brlFull, showTotals: true, zebra: true } }),
    ],
  },
  {
    id: "comercial-performance",
    name: "Performance comercial",
    category: "comercial",
    description: "Receita, vendedores, mix de produto e tendência — o ritual semanal pronto.",
    pain: "Cada gestor olha vendas à sua hora, com critério diferente.",
    icon: "trending",
    popular: true,
    needs: ["receita ou valor", "vendedor ou região", "produto"],
    widgets: [
      w("kpi", "Receita", { x: 0, y: 0, w: 3, h: 2 }, { measures: ["revenue"] }, { config: brl }),
      w("kpi", "Pedidos", { x: 3, y: 0, w: 3, h: 2 }, { measures: ["orders"] }),
      w("kpi", "Clientes", { x: 6, y: 0, w: 3, h: 2 }, { measures: ["customers"] }),
      w("slicer", "Região", { x: 9, y: 0, w: 3, h: 2 }, { dimensions: ["region"], measures: [], limit: 200 }),
      w("line", "Receita no tempo", { x: 0, y: 2, w: 8, h: 5 }, { measures: ["revenue"], dimensions: ["date"], limit: 90 }),
      w("pie", "Mix de produto", { x: 8, y: 2, w: 4, h: 5 }, { measures: ["revenue"], dimensions: ["product"], limit: 10 }),
      w("bar", "Por vendedor", { x: 0, y: 7, w: 6, h: 5 }, { measures: ["revenue"], dimensions: ["sales_rep"], limit: 15 }),
      w("bar", "Por região", { x: 6, y: 7, w: 6, h: 5 }, { measures: ["revenue"], dimensions: ["region"], limit: 15 }),
    ],
  },
  {
    id: "comercial-pipeline",
    name: "Pipeline e follow-up",
    category: "comercial",
    description: "Funil, etapas paradas e concentração de oportunidades.",
    pain: "Oportunidades esfriam no pipeline e só alguém repara semanas depois.",
    icon: "target",
    needs: ["valor", "status ou etapa"],
    widgets: [
      w("kpi", "Pipeline", { x: 0, y: 0, w: 4, h: 2 }, { measures: ["revenue"] }, { config: brl }),
      w("kpi", "Oportunidades", { x: 4, y: 0, w: 4, h: 2 }, { measures: ["orders"] }),
      w("slicer", "Etapa", { x: 8, y: 0, w: 4, h: 2 }, { dimensions: ["status"], measures: [], limit: 200 }),
      w("funnel", "Funil", { x: 0, y: 2, w: 5, h: 6 }, { measures: ["revenue"], dimensions: ["status"], limit: 10 }),
      w("bar", "Por vendedor", { x: 5, y: 2, w: 7, h: 6 }, { measures: ["revenue"], dimensions: ["sales_rep"], limit: 15 }),
      w("table", "Detalhe", { x: 0, y: 8, w: 12, h: 4 }, { measures: ["revenue"], dimensions: ["customer"], limit: 40 }),
    ],
  },
  {
    id: "ecommerce-loja",
    name: "E-commerce",
    category: "ecommerce",
    description: "GMV, pedidos, canais e produtos que puxam a loja.",
    pain: "O site vende, mas não se vê o que cresce e o que está a cair.",
    icon: "cart",
    popular: true,
    needs: ["receita ou valor", "produto ou canal"],
    widgets: [
      w("kpi", "GMV", { x: 0, y: 0, w: 3, h: 2 }, { measures: ["revenue"] }, { config: brl }),
      w("kpi", "Pedidos", { x: 3, y: 0, w: 3, h: 2 }, { measures: ["orders"] }),
      w("kpi", "Ticket", { x: 6, y: 0, w: 3, h: 2 }, { measures: ["valor"] }, { config: brl }),
      w("slicer", "Canal", { x: 9, y: 0, w: 3, h: 2 }, { dimensions: ["channel"], measures: [], limit: 200 }),
      w("area", "Vendas no tempo", { x: 0, y: 2, w: 8, h: 5 }, { measures: ["revenue"], dimensions: ["date"], limit: 90 }),
      w("pie", "Por canal", { x: 8, y: 2, w: 4, h: 5 }, { measures: ["revenue"], dimensions: ["channel"], limit: 8 }),
      w("bar", "Top produtos", { x: 0, y: 7, w: 6, h: 5 }, { measures: ["revenue"], dimensions: ["product"], limit: 12 }),
      w("treemap", "Categorias", { x: 6, y: 7, w: 6, h: 5 }, { measures: ["revenue"], dimensions: ["category"], limit: 20 }),
    ],
  },
  {
    id: "rh-pessoas",
    name: "RH e pessoas",
    category: "rh",
    description: "Quadro, folha e distribuição por área — o painel de people ops.",
    pain: "Headcount e custo de pessoal espalhados em planilhas.",
    icon: "users",
    needs: ["pessoas ou valor", "área ou cargo"],
    widgets: [
      w("kpi", "Pessoas", { x: 0, y: 0, w: 4, h: 2 }, { measures: ["headcount"] }),
      w("kpi", "Folha", { x: 4, y: 0, w: 4, h: 2 }, { measures: ["salary"] }, { config: brl }),
      w("slicer", "Área", { x: 8, y: 0, w: 4, h: 2 }, { dimensions: ["department"], measures: [], limit: 200 }),
      w("bar", "Por departamento", { x: 0, y: 2, w: 6, h: 5 }, { measures: ["headcount"], dimensions: ["department"], limit: 15 }),
      w("pie", "Por cargo", { x: 6, y: 2, w: 6, h: 5 }, { measures: ["headcount"], dimensions: ["cargo"], limit: 10 }),
      w("line", "Evolução", { x: 0, y: 7, w: 7, h: 5 }, { measures: ["headcount"], dimensions: ["date"], limit: 24 }),
      w("table", "Detalhe", { x: 7, y: 7, w: 5, h: 5 }, { measures: ["salary"], dimensions: ["department"], limit: 30 }),
    ],
  },
  {
    id: "rh-desempenho",
    name: "Desempenho de pessoas",
    category: "rh",
    description: "Notas, faixas e atingimento de metas por área, gestor e colaborador.",
    pain: "A avaliação fecha e o mapa de desempenho fica numa planilha que ninguém abre.",
    icon: "target",
    popular: true,
    needs: ["nota ou atingimento", "colaborador ou área"],
    widgets: [
      w("kpi", "Pessoas avaliadas", { x: 0, y: 0, w: 3, h: 2 }, { measures: ["headcount"] }),
      w("kpi", "Nota média", { x: 3, y: 0, w: 3, h: 2 }, { measures: ["nota"] }, { config: { decimals: 1 } }),
      w("kpi", "Atingimento", { x: 6, y: 0, w: 3, h: 2 }, { measures: ["atingimento"] }, { config: { suffix: "%", decimals: 0 } }),
      w("slicer", "Ciclo", { x: 9, y: 0, w: 3, h: 2 }, { dimensions: ["ciclo"], measures: [], limit: 200 }),
      w("bar", "Nota por área", { x: 0, y: 2, w: 7, h: 5 }, { measures: ["nota"], dimensions: ["department"], limit: 15 }),
      w("pie", "Por faixa", { x: 7, y: 2, w: 5, h: 5 }, { measures: ["headcount"], dimensions: ["faixa"], limit: 8 }),
      w("bar", "Por colaborador", { x: 0, y: 7, w: 6, h: 5 }, { measures: ["nota"], dimensions: ["colaborador"], limit: 15 }),
      w("table", "Detalhe", { x: 6, y: 7, w: 6, h: 5 }, { measures: ["nota", "atingimento"], dimensions: ["colaborador"], limit: 40 }),
    ],
  },
  {
    id: "operacoes-ruptura",
    name: "Ruptura zero",
    category: "operacoes",
    description: "Estoque, ruptura e itens parados — operação que avisa antes de faltar.",
    pain: "Produto some da prateleira e o comercial só sabe pelo cliente.",
    icon: "package",
    popular: true,
    needs: ["estoque ou quantidade", "produto ou status"],
    widgets: [
      w("kpi", "Estoque", { x: 0, y: 0, w: 4, h: 2 }, { measures: ["inventory"] }),
      w("kpi", "Volume", { x: 4, y: 0, w: 4, h: 2 }, { measures: ["orders"] }),
      w("slicer", "Status", { x: 8, y: 0, w: 4, h: 2 }, { dimensions: ["status"], measures: [], limit: 200 }),
      w("bar", "Por produto", { x: 0, y: 2, w: 7, h: 5 }, { measures: ["inventory"], dimensions: ["product"], limit: 15 }),
      w("pie", "Por armazém", { x: 7, y: 2, w: 5, h: 5 }, { measures: ["inventory"], dimensions: ["warehouse"], limit: 8 }),
      w("table", "Itens", { x: 0, y: 7, w: 12, h: 5 }, { measures: ["inventory"], dimensions: ["product"], limit: 50 }),
    ],
  },
  {
    id: "operacoes-producao",
    name: "Performance de produção",
    category: "operacoes",
    description: "Volume, eficiência e qualidade ao longo do turno.",
    pain: "A linha cai de rendimento e o relatório só chega no dia seguinte.",
    icon: "package",
    needs: ["volume ou valor", "status ou produto"],
    widgets: [
      w("kpi", "Volume", { x: 0, y: 0, w: 4, h: 2 }, { measures: ["orders"] }),
      w("kpi", "Valor", { x: 4, y: 0, w: 4, h: 2 }, { measures: ["valor"] }, { config: brl }),
      w("sparkline", "Ritmo", { x: 8, y: 0, w: 4, h: 2 }, { measures: ["orders"], dimensions: ["date"] }),
      w("area", "Volume no tempo", { x: 0, y: 2, w: 8, h: 5 }, { measures: ["orders"], dimensions: ["date"], limit: 48 }),
      w("pie", "Por status", { x: 8, y: 2, w: 4, h: 5 }, { measures: ["orders"], dimensions: ["status"], limit: 8 }),
      w("bar", "Por produto", { x: 0, y: 7, w: 12, h: 4 }, { measures: ["orders"], dimensions: ["product"], limit: 12 }),
    ],
  },
  {
    id: "marketing-aquisicao",
    name: "Aquisição e campanhas",
    category: "marketing",
    description: "Tráfego, conversão e performance por campanha e canal.",
    pain: "Gasta-se em ads sem ver qual campanha realmente converte.",
    icon: "megaphone",
    needs: ["receita ou sessões", "campanha ou canal"],
    widgets: [
      w("kpi", "Receita", { x: 0, y: 0, w: 3, h: 2 }, { measures: ["revenue"] }, { config: brl }),
      w("kpi", "Sessões", { x: 3, y: 0, w: 3, h: 2 }, { measures: ["sessions"] }),
      w("kpi", "Conversão", { x: 6, y: 0, w: 3, h: 2 }, { measures: ["conversion"] }),
      w("slicer", "Canal", { x: 9, y: 0, w: 3, h: 2 }, { dimensions: ["channel"], measures: [], limit: 200 }),
      w("line", "Tendência", { x: 0, y: 2, w: 8, h: 5 }, { measures: ["revenue"], dimensions: ["date"], limit: 90 }),
      w("pie", "Por canal", { x: 8, y: 2, w: 4, h: 5 }, { measures: ["revenue"], dimensions: ["channel"], limit: 8 }),
      w("bar", "Por campanha", { x: 0, y: 7, w: 12, h: 5 }, { measures: ["revenue"], dimensions: ["campaign"], limit: 15 }),
    ],
  },
  {
    id: "logistica-entregas",
    name: "Logística e frete",
    category: "logistica",
    description: "Custo de frete, volume de entregas e desempenho por transportadora.",
    pain: "O frete sobe em silêncio e come a margem do pedido.",
    icon: "truck",
    needs: ["frete ou valor", "status ou região"],
    widgets: [
      w("kpi", "Frete", { x: 0, y: 0, w: 4, h: 2 }, { measures: ["freight"] }, { config: brl }),
      w("kpi", "Entregas", { x: 4, y: 0, w: 4, h: 2 }, { measures: ["orders"] }),
      w("slicer", "Status", { x: 8, y: 0, w: 4, h: 2 }, { dimensions: ["status"], measures: [], limit: 200 }),
      w("bar", "Por região", { x: 0, y: 2, w: 6, h: 5 }, { measures: ["freight"], dimensions: ["region"], limit: 15 }),
      w("line", "Custo no tempo", { x: 6, y: 2, w: 6, h: 5 }, { measures: ["freight"], dimensions: ["date"], limit: 60 }),
      w("table", "Detalhe", { x: 0, y: 7, w: 12, h: 5 }, { measures: ["freight"], dimensions: ["supplier"], limit: 40 }),
    ],
  },
  {
    id: "saas-recorrencia",
    name: "SaaS e recorrência",
    category: "saas",
    description: "MRR, churn e contas — o pulso da receita recorrente.",
    pain: "O churn aparece no relatório do mês seguinte, tarde demais.",
    icon: "repeat",
    needs: ["receita ou MRR", "status ou plano"],
    widgets: [
      w("kpi", "MRR", { x: 0, y: 0, w: 4, h: 2 }, { measures: ["mrr"] }, { config: brl }),
      w("kpi", "Contas", { x: 4, y: 0, w: 4, h: 2 }, { measures: ["customers"] }),
      w("kpi", "Churn", { x: 8, y: 0, w: 4, h: 2 }, { measures: ["churn"] }),
      w("area", "Receita recorrente", { x: 0, y: 2, w: 8, h: 5 }, { measures: ["mrr"], dimensions: ["date"], limit: 24 }),
      w("pie", "Por status", { x: 8, y: 2, w: 4, h: 5 }, { measures: ["mrr"], dimensions: ["status"], limit: 8 }),
      w("bar", "Por plano", { x: 0, y: 7, w: 12, h: 5 }, { measures: ["mrr"], dimensions: ["category"], limit: 12 }),
    ],
  },
  {
    id: "compras-fornecedores",
    name: "Compras e fornecedores",
    category: "compras",
    description: "Spend, concentração e desempenho de fornecedor.",
    pain: "O custo de insumo sobe e a renegociação chega tarde.",
    icon: "shopping",
    needs: ["valor", "fornecedor ou categoria"],
    widgets: [
      w("kpi", "Spend", { x: 0, y: 0, w: 4, h: 2 }, { measures: ["cost"] }, { config: brl }),
      w("kpi", "Pedidos", { x: 4, y: 0, w: 4, h: 2 }, { measures: ["orders"] }),
      w("slicer", "Fornecedor", { x: 8, y: 0, w: 4, h: 2 }, { dimensions: ["supplier"], measures: [], limit: 200 }),
      w("bar", "Por fornecedor", { x: 0, y: 2, w: 7, h: 5 }, { measures: ["cost"], dimensions: ["supplier"], limit: 12 }),
      w("pie", "Por categoria", { x: 7, y: 2, w: 5, h: 5 }, { measures: ["cost"], dimensions: ["category"], limit: 8 }),
      w("table", "Detalhe", { x: 0, y: 7, w: 12, h: 5 }, { measures: ["cost"], dimensions: ["product"], limit: 40 }),
    ],
  },
  {
    id: "atendimento-cs",
    name: "Atendimento e CS",
    category: "atendimento",
    description: "Volume de tickets, filas e satisfação por canal.",
    pain: "A fila cresce e o tempo de resposta só se vê no fim da semana.",
    icon: "headset",
    needs: ["volume ou valor", "status ou canal"],
    widgets: [
      w("kpi", "Tickets", { x: 0, y: 0, w: 4, h: 2 }, { measures: ["orders"] }),
      w("kpi", "Volume", { x: 4, y: 0, w: 4, h: 2 }, { measures: ["valor"] }),
      w("slicer", "Canal", { x: 8, y: 0, w: 4, h: 2 }, { dimensions: ["channel"], measures: [], limit: 200 }),
      w("bar", "Por status", { x: 0, y: 2, w: 6, h: 5 }, { measures: ["orders"], dimensions: ["status"], limit: 12 }),
      w("line", "Entrada no tempo", { x: 6, y: 2, w: 6, h: 5 }, { measures: ["orders"], dimensions: ["date"], limit: 60 }),
      w("table", "Fila", { x: 0, y: 7, w: 12, h: 5 }, { measures: ["orders"], dimensions: ["customer"], limit: 40 }),
    ],
  },
];

export function getTemplate(id: string) {
  return DASHBOARD_TEMPLATES.find((t) => t.id === id);
}

export function mergeTemplateMeasures(model: SemanticModel | null | undefined, extra?: SemanticMeasure[]): SemanticModel {
  const measures = [...(model?.measures || [])];
  const seen = new Set(measures.map((m) => measureKey(m).toLowerCase()));
  for (const m of extra || []) {
    const key = (m.name || "").trim().toLowerCase();
    if (!key || seen.has(key)) continue;
    measures.push(m);
    seen.add(key);
  }
  return { ...(model || {}), measures };
}

function semanticRows(raw: unknown): any[] {
  if (Array.isArray(raw)) return raw;
  if (raw && typeof raw === "object" && Array.isArray((raw as { data?: unknown }).data)) {
    return (raw as { data: any[] }).data;
  }
  return [];
}

/** Installs template measures on the dataset model, then returns the merged model for remap. */
export async function prepareTemplateModel(
  datasetId: string,
  tpl: DashboardTemplate,
  knownRows?: any[],
): Promise<SemanticModel | null> {
  const rows = knownRows?.length ? knownRows : semanticRows(await api<any>("/api/v1/semantic-models"));
  const current = modelForDataset(rows, datasetId);
  const merged = mergeTemplateMeasures(current, tpl.measures);
  const id = modelIdForDataset(rows, datasetId);
  if (id && tpl.measures?.length) {
    await api(`/api/v1/semantic-models/${id}`, {
      method: "PUT",
      body: JSON.stringify({ ...merged, dataset_id: datasetId }),
    });
  }
  return merged;
}

export function instantiateTemplate(tpl: DashboardTemplate, datasetId: string, model: SemanticModel | null | undefined): Widget[] {
  return tpl.widgets.map((widget) => {
    const query = widget.query
      ? {
          ...remapQueryToModel({ ...widget.query, dataset_id: datasetId }, widget.type, model),
          dataset_id: datasetId,
          limit: widget.query.limit ?? (widget.type === "slicer" ? 200 : widget.type === "table" ? 50 : 20),
        }
      : undefined;
    return {
      ...widget,
      id: crypto.randomUUID(),
      query,
    };
  });
}

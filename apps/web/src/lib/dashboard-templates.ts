import type { Widget, WidgetType } from "@/components/WidgetView";
import { api } from "@/lib/api";
import { DEFAULT_QUERY_LIMIT } from "@/lib/widget-config";
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
  | "atendimento"
  | "imobiliario"
  | "advocacia";

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
  | "percent"
  | "building"
  | "home"
  | "key"
  | "scale";

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
  { id: "imobiliario", label: "Imobiliário" },
  { id: "advocacia", label: "Advocacia" },
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
          limit: query.limit ?? DEFAULT_QUERY_LIMIT,
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
  { name: "Ticket médio", expression: "DIVIDE(SUM(valor_mensal), COUNT(*))", aggregation: "expression" },
  { name: "Clientes", expression: "DISTINCTCOUNT(cliente)", aggregation: "expression" },
  {
    name: "Variação da receita",
    expression:
      "(SUM(CASE WHEN TOMONTH(data_venda) = '2026-08' THEN valor_mensal ELSE 0 END) - SUM(CASE WHEN TOMONTH(data_venda) = '2026-07' THEN valor_mensal ELSE 0 END)) / NULLIF(SUM(CASE WHEN TOMONTH(data_venda) = '2026-07' THEN valor_mensal ELSE 0 END), 0) * 100",
    aggregation: "expression",
  },
  {
    name: "Variação do ticket",
    expression:
      "(AVG(CASE WHEN TOMONTH(data_venda) = '2026-08' THEN valor_mensal END) - AVG(CASE WHEN TOMONTH(data_venda) = '2026-07' THEN valor_mensal END)) / NULLIF(AVG(CASE WHEN TOMONTH(data_venda) = '2026-07' THEN valor_mensal END), 0) * 100",
    aggregation: "expression",
  },
  { name: "Linhas julho", expression: "SUM(CASE WHEN TOMONTH(data_venda) = '2026-07' THEN 1 ELSE 0 END)", aggregation: "expression" },
  { name: "Linhas agosto", expression: "SUM(CASE WHEN TOMONTH(data_venda) = '2026-08' THEN 1 ELSE 0 END)", aggregation: "expression" },
  { name: "Clientes julho", expression: "COUNT(DISTINCT CASE WHEN TOMONTH(data_venda) = '2026-07' THEN cliente END)", aggregation: "expression" },
  { name: "Clientes agosto", expression: "COUNT(DISTINCT CASE WHEN TOMONTH(data_venda) = '2026-08' THEN cliente END)", aggregation: "expression" },
];

const IMOBILIARIO_VENDAS_MEASURES: SemanticMeasure[] = [
  { name: "VGV", expression: "SUM(valor)", aggregation: "expression" },
  { name: "Unidades", expression: "COUNT(*)", aggregation: "expression" },
  { name: "Ticket médio", expression: "DIVIDE(SUM(valor), COUNT(*))", aggregation: "expression" },
  { name: "Comissão", expression: "SUM(comissao)", aggregation: "expression" },
  { name: "Área vendida", expression: "SUM(area_m2)", aggregation: "expression" },
  { name: "Preço por m²", expression: "DIVIDE(SUM(valor), NULLIF(SUM(area_m2), 0))", aggregation: "expression" },
];

const IMOBILIARIO_LOCACAO_MEASURES: SemanticMeasure[] = [
  { name: "Aluguel", expression: "SUM(valor_aluguel)", aggregation: "expression" },
  { name: "Contratos", expression: "COUNT(*)", aggregation: "expression" },
  { name: "Unidades locadas", expression: "COUNT(DISTINCT imovel)", aggregation: "expression" },
  { name: "Inadimplência", expression: "SUM(CASE WHEN status IN ('atraso','inadimplente','vencido') THEN valor_aluguel ELSE 0 END)", aggregation: "expression" },
  { name: "Taxa de ocupação", expression: "DIVIDE(SUM(CASE WHEN status IN ('locado','ocupado','ativo') THEN 1 ELSE 0 END), COUNT(*)) * 100", aggregation: "expression" },
];

const IMOBILIARIO_ESTOQUE_MEASURES: SemanticMeasure[] = [
  { name: "Imóveis", expression: "COUNT(*)", aggregation: "expression" },
  { name: "Valor de estoque", expression: "SUM(valor)", aggregation: "expression" },
  { name: "Dias em estoque", expression: "AVG(dias_estoque)", aggregation: "expression" },
  { name: "Disponíveis", expression: "SUM(CASE WHEN status IN ('disponivel','ativo','captação','captacao') THEN 1 ELSE 0 END)", aggregation: "expression" },
];

const IMOBILIARIO_LEADS_MEASURES: SemanticMeasure[] = [
  { name: "Leads", expression: "COUNT(*)", aggregation: "expression" },
  { name: "Pipeline", expression: "SUM(valor)", aggregation: "expression" },
  { name: "Convertidos", expression: "SUM(CASE WHEN status IN ('ganho','vendido','fechado','locado') THEN 1 ELSE 0 END)", aggregation: "expression" },
  { name: "Conversão", expression: "DIVIDE(SUM(CASE WHEN status IN ('ganho','vendido','fechado','locado') THEN 1 ELSE 0 END), COUNT(*)) * 100", aggregation: "expression" },
];

const ADVOCACIA_PROCESSOS_MEASURES: SemanticMeasure[] = [
  { name: "Processos", expression: "COUNT(*)", aggregation: "expression" },
  { name: "Clientes", expression: "DISTINCTCOUNT(cliente)", aggregation: "expression" },
  { name: "Valor da causa", expression: "SUM(valor_causa)", aggregation: "expression" },
  { name: "Honorários previstos", expression: "SUM(honorarios_previstos)", aggregation: "expression" },
  { name: "Honorários recebidos", expression: "SUM(honorarios_recebidos)", aggregation: "expression" },
  { name: "Horas trabalhadas", expression: "SUM(horas_trabalhadas)", aggregation: "expression" },
  { name: "Taxa de êxito", expression: "DIVIDE(SUM(CASE WHEN resultado = 'Ganho' THEN 1 ELSE 0 END), COUNT(*)) * 100", aggregation: "expression" },
];

const ADVOCACIA_PRAZOS_MEASURES: SemanticMeasure[] = [
  { name: "Prazos", expression: "COUNT(*)", aggregation: "expression" },
  { name: "Prazos vencidos", expression: "SUM(CASE WHEN status_prazo = 'Vencido' THEN 1 ELSE 0 END)", aggregation: "expression" },
  { name: "Prazos próximos", expression: "SUM(CASE WHEN status_prazo = 'Próximo' THEN 1 ELSE 0 END)", aggregation: "expression" },
  { name: "Valor em risco", expression: "SUM(CASE WHEN risco = 'Alto' THEN valor_causa ELSE 0 END)", aggregation: "expression" },
  { name: "Taxa de cumprimento", expression: "DIVIDE(SUM(CASE WHEN status_prazo = 'Concluído' THEN 1 ELSE 0 END), COUNT(*)) * 100", aggregation: "expression" },
];

const ADVOCACIA_FINANCEIRO_MEASURES: SemanticMeasure[] = [
  { name: "Honorários previstos", expression: "SUM(honorarios_previstos)", aggregation: "expression" },
  { name: "Honorários recebidos", expression: "SUM(honorarios_recebidos)", aggregation: "expression" },
  { name: "Em aberto", expression: "SUM(honorarios_previstos) - SUM(honorarios_recebidos)", aggregation: "expression" },
  { name: "Clientes faturados", expression: "DISTINCTCOUNT(cliente)", aggregation: "expression" },
  { name: "Horas trabalhadas", expression: "SUM(horas_trabalhadas)", aggregation: "expression" },
  { name: "Ticket médio", expression: "DIVIDE(SUM(honorarios_previstos), COUNT(*))", aggregation: "expression" },
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
      w("slicer", "Mês", { x: 0, y: 4, w: 3, h: 2 }, { dimensions: ["mes"], measures: [] }, { config: { slicerStyle: "buttons" } }),
      w("slicer", "Vendedor", { x: 3, y: 4, w: 3, h: 2 }, { dimensions: ["vendedor"], measures: [] }, { config: { slicerStyle: "dropdown" } }),
      w("slicer", "Cliente", { x: 6, y: 4, w: 3, h: 2 }, { dimensions: ["cliente"], measures: [] }, { config: { slicerStyle: "dropdown", slicerSearch: true } }),
      w("slicer", "Luxus", { x: 9, y: 4, w: 3, h: 2 }, { dimensions: ["cliente_luxus"], measures: [] }, { config: { slicerStyle: "buttons" } }),
      w("bar", "Receita por mês", { x: 0, y: 6, w: 6, h: 4 }, { measures: ["Receita"], dimensions: ["mes"] }, { config: { ...brlFull, showDataLabels: true } }),
      w("bar", "Contratos por mês", { x: 6, y: 6, w: 6, h: 4 }, { measures: ["Contratos"], dimensions: ["mes"] }, { config: { showDataLabels: true } }),
      w("line", "Receita por dia", { x: 0, y: 10, w: 12, h: 4 }, { measures: ["Receita"], dimensions: ["data_venda"] }, { config: brlFull }),
      w("bar", "Receita por vendedor", { x: 0, y: 14, w: 6, h: 4 }, { measures: ["Receita"], dimensions: ["vendedor", "mes"] }, { config: brlFull }),
      w("bar", "Ticket por vendedor", { x: 6, y: 14, w: 6, h: 4 }, { measures: ["Ticket médio"], dimensions: ["vendedor"] }, { config: brlTicket }),
      w("pie", "Mix de clientes", { x: 0, y: 18, w: 4, h: 4 }, { measures: ["Receita"], dimensions: ["cliente"] }, { config: brlFull }),
      w("treemap", "Peso dos clientes", { x: 4, y: 18, w: 8, h: 4 }, { measures: ["Receita"], dimensions: ["cliente"] }, { config: brlFull }),
      w("table", "Detalhe", { x: 0, y: 22, w: 12, h: 5 }, { measures: ["Receita", "Contratos"], dimensions: ["cliente", "vendedor", "mes"] }, { config: { ...brlFull, showTotals: true, zebra: true } }),
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
  {
    id: "imobiliario-vendas",
    name: "Vendas e VGV",
    category: "imobiliario",
    description: "VGV, unidades, ticket e preço/m² por corretor, tipologia e bairro.",
    pain: "O VGV fecha no Excel do comercial e a diretoria só vê o consolidado no mês seguinte.",
    icon: "building",
    popular: true,
    needs: ["valor ou VGV", "corretor ou bairro", "tipologia"],
    measures: IMOBILIARIO_VENDAS_MEASURES,
    widgets: [
      w("kpi", "VGV", { x: 0, y: 0, w: 3, h: 2 }, { measures: ["VGV"] }, { config: brl }),
      w("kpi", "Unidades", { x: 3, y: 0, w: 3, h: 2 }, { measures: ["Unidades"] }),
      w("kpi", "Ticket médio", { x: 6, y: 0, w: 3, h: 2 }, { measures: ["Ticket médio"] }, { config: brlTicket }),
      w("kpi", "Preço por m²", { x: 9, y: 0, w: 3, h: 2 }, { measures: ["Preço por m²"] }, { config: brlFull }),
      w("slicer", "Tipologia", { x: 0, y: 2, w: 3, h: 2 }, { dimensions: ["tipologia"], measures: [] }, { config: { slicerStyle: "buttons" } }),
      w("slicer", "Bairro", { x: 3, y: 2, w: 3, h: 2 }, { dimensions: ["bairro"], measures: [] }, { config: { slicerStyle: "dropdown", slicerSearch: true } }),
      w("slicer", "Corretor", { x: 6, y: 2, w: 3, h: 2 }, { dimensions: ["corretor"], measures: [] }, { config: { slicerStyle: "dropdown" } }),
      w("slicer", "Status", { x: 9, y: 2, w: 3, h: 2 }, { dimensions: ["status"], measures: [] }, { config: { slicerStyle: "buttons" } }),
      w("area", "VGV no tempo", { x: 0, y: 4, w: 8, h: 5 }, { measures: ["VGV"], dimensions: ["date"], limit: 24 }, { config: brl }),
      w("pie", "Mix de tipologia", { x: 8, y: 4, w: 4, h: 5 }, { measures: ["VGV"], dimensions: ["tipologia"], limit: 8 }, { config: brl }),
      w("bar", "Por corretor", { x: 0, y: 9, w: 6, h: 5 }, { measures: ["VGV"], dimensions: ["corretor"], limit: 12 }, { config: brl }),
      w("bar", "Por bairro", { x: 6, y: 9, w: 6, h: 5 }, { measures: ["VGV"], dimensions: ["bairro"], limit: 12 }, { config: brl }),
      w("table", "Detalhe das vendas", { x: 0, y: 14, w: 12, h: 5 }, { measures: ["VGV", "Unidades", "Comissão"], dimensions: ["imovel", "corretor", "tipologia"], limit: 50 }, { config: { ...brlFull, showTotals: true, zebra: true } }),
    ],
  },
  {
    id: "imobiliario-locacao",
    name: "Locação e ocupação",
    category: "imobiliario",
    description: "Aluguel, ocupação, vacância e inadimplência do portfólio locado.",
    pain: "Unidade vaga e aluguel em atraso só aparecem quando o proprietário liga.",
    icon: "key",
    popular: true,
    needs: ["aluguel ou valor", "status ou imóvel"],
    measures: IMOBILIARIO_LOCACAO_MEASURES,
    widgets: [
      w("kpi", "Aluguel", { x: 0, y: 0, w: 3, h: 2 }, { measures: ["Aluguel"] }, { config: brl }),
      w("kpi", "Contratos", { x: 3, y: 0, w: 3, h: 2 }, { measures: ["Contratos"] }),
      w("kpi", "Ocupação", { x: 6, y: 0, w: 3, h: 2 }, { measures: ["Taxa de ocupação"] }, { config: pct1 }),
      w("kpi", "Inadimplência", { x: 9, y: 0, w: 3, h: 2 }, { measures: ["Inadimplência"] }, { config: brl }),
      w("slicer", "Status", { x: 0, y: 2, w: 4, h: 2 }, { dimensions: ["status"], measures: [] }, { config: { slicerStyle: "buttons" } }),
      w("slicer", "Tipologia", { x: 4, y: 2, w: 4, h: 2 }, { dimensions: ["tipologia"], measures: [] }),
      w("slicer", "Bairro", { x: 8, y: 2, w: 4, h: 2 }, { dimensions: ["bairro"], measures: [] }),
      w("line", "Aluguel no tempo", { x: 0, y: 4, w: 8, h: 5 }, { measures: ["Aluguel"], dimensions: ["date"], limit: 24 }, { config: brl }),
      w("pie", "Por status", { x: 8, y: 4, w: 4, h: 5 }, { measures: ["Contratos"], dimensions: ["status"], limit: 8 }),
      w("bar", "Por tipologia", { x: 0, y: 9, w: 6, h: 5 }, { measures: ["Aluguel"], dimensions: ["tipologia"], limit: 10 }, { config: brl }),
      w("bar", "Por bairro", { x: 6, y: 9, w: 6, h: 5 }, { measures: ["Aluguel"], dimensions: ["bairro"], limit: 12 }, { config: brl }),
      w("table", "Contratos", { x: 0, y: 14, w: 12, h: 5 }, { measures: ["Aluguel", "Inadimplência"], dimensions: ["imovel", "inquilino", "status"], limit: 50 }, { config: { ...brlFull, showTotals: true, zebra: true } }),
    ],
  },
  {
    id: "imobiliario-estoque",
    name: "Estoque e captação",
    category: "imobiliario",
    description: "Imóveis em carteira, dias em estoque e mix por tipologia e empreendimento.",
    pain: "Ninguém sabe o que está parado na vitrine e o que precisa de recaptação.",
    icon: "home",
    popular: true,
    needs: ["imóvel ou valor", "status ou tipologia"],
    measures: IMOBILIARIO_ESTOQUE_MEASURES,
    widgets: [
      w("kpi", "Imóveis", { x: 0, y: 0, w: 3, h: 2 }, { measures: ["Imóveis"] }),
      w("kpi", "Valor de estoque", { x: 3, y: 0, w: 3, h: 2 }, { measures: ["Valor de estoque"] }, { config: brl }),
      w("kpi", "Disponíveis", { x: 6, y: 0, w: 3, h: 2 }, { measures: ["Disponíveis"] }),
      w("kpi", "Dias em estoque", { x: 9, y: 0, w: 3, h: 2 }, { measures: ["Dias em estoque"] }, { config: { decimals: 0 } }),
      w("slicer", "Status", { x: 0, y: 2, w: 4, h: 2 }, { dimensions: ["status"], measures: [] }, { config: { slicerStyle: "buttons" } }),
      w("slicer", "Tipologia", { x: 4, y: 2, w: 4, h: 2 }, { dimensions: ["tipologia"], measures: [] }),
      w("slicer", "Empreendimento", { x: 8, y: 2, w: 4, h: 2 }, { dimensions: ["empreendimento"], measures: [] }),
      w("bar", "Por tipologia", { x: 0, y: 4, w: 6, h: 5 }, { measures: ["Imóveis"], dimensions: ["tipologia"], limit: 12 }),
      w("treemap", "Por bairro", { x: 6, y: 4, w: 6, h: 5 }, { measures: ["Valor de estoque"], dimensions: ["bairro"], limit: 20 }, { config: brl }),
      w("pie", "Por status", { x: 0, y: 9, w: 4, h: 5 }, { measures: ["Imóveis"], dimensions: ["status"], limit: 8 }),
      w("bar", "Por empreendimento", { x: 4, y: 9, w: 8, h: 5 }, { measures: ["Imóveis"], dimensions: ["empreendimento"], limit: 12 }),
      w("table", "Carteira", { x: 0, y: 14, w: 12, h: 5 }, { measures: ["Valor de estoque", "Dias em estoque"], dimensions: ["imovel", "tipologia", "status"], limit: 50 }, { config: { ...brlFull, zebra: true } }),
    ],
  },
  {
    id: "imobiliario-leads",
    name: "Funil de leads",
    category: "imobiliario",
    description: "Captação, conversão e valor em pipeline por etapa, canal e corretor.",
    pain: "Lead esfria no WhatsApp e só o corretor sabe em que etapa está.",
    icon: "target",
    needs: ["lead ou valor", "status ou canal"],
    measures: IMOBILIARIO_LEADS_MEASURES,
    widgets: [
      w("kpi", "Leads", { x: 0, y: 0, w: 3, h: 2 }, { measures: ["Leads"] }),
      w("kpi", "Pipeline", { x: 3, y: 0, w: 3, h: 2 }, { measures: ["Pipeline"] }, { config: brl }),
      w("kpi", "Convertidos", { x: 6, y: 0, w: 3, h: 2 }, { measures: ["Convertidos"] }),
      w("kpi", "Conversão", { x: 9, y: 0, w: 3, h: 2 }, { measures: ["Conversão"] }, { config: pct1 }),
      w("slicer", "Etapa", { x: 0, y: 2, w: 4, h: 2 }, { dimensions: ["status"], measures: [] }, { config: { slicerStyle: "buttons" } }),
      w("slicer", "Canal", { x: 4, y: 2, w: 4, h: 2 }, { dimensions: ["channel"], measures: [] }),
      w("slicer", "Corretor", { x: 8, y: 2, w: 4, h: 2 }, { dimensions: ["corretor"], measures: [] }),
      w("funnel", "Funil", { x: 0, y: 4, w: 5, h: 6 }, { measures: ["Leads"], dimensions: ["status"], limit: 10 }),
      w("bar", "Por corretor", { x: 5, y: 4, w: 7, h: 6 }, { measures: ["Pipeline"], dimensions: ["corretor"], limit: 12 }, { config: brl }),
      w("pie", "Por canal", { x: 0, y: 10, w: 5, h: 5 }, { measures: ["Leads"], dimensions: ["channel"], limit: 8 }),
      w("line", "Entrada no tempo", { x: 5, y: 10, w: 7, h: 5 }, { measures: ["Leads"], dimensions: ["date"], limit: 60 }),
      w("table", "Oportunidades", { x: 0, y: 15, w: 12, h: 5 }, { measures: ["Pipeline"], dimensions: ["customer", "corretor", "status"], limit: 40 }, { config: { ...brlFull, zebra: true } }),
    ],
  },
  {
    id: "imobiliario-corretores",
    name: "Performance de corretores",
    category: "imobiliario",
    description: "Ranking de VGV, unidades e comissão — o ritual semanal da equipe.",
    pain: "Cada gerente mede o time com critério diferente e o ranking vira discussão.",
    icon: "trending",
    needs: ["valor ou VGV", "corretor"],
    measures: IMOBILIARIO_VENDAS_MEASURES,
    widgets: [
      w("kpi", "VGV", { x: 0, y: 0, w: 3, h: 2 }, { measures: ["VGV"] }, { config: brl }),
      w("kpi", "Unidades", { x: 3, y: 0, w: 3, h: 2 }, { measures: ["Unidades"] }),
      w("kpi", "Comissão", { x: 6, y: 0, w: 3, h: 2 }, { measures: ["Comissão"] }, { config: brl }),
      w("kpi", "Ticket médio", { x: 9, y: 0, w: 3, h: 2 }, { measures: ["Ticket médio"] }, { config: brlTicket }),
      w("slicer", "Corretor", { x: 0, y: 2, w: 4, h: 2 }, { dimensions: ["corretor"], measures: [] }),
      w("slicer", "Equipe", { x: 4, y: 2, w: 4, h: 2 }, { dimensions: ["department"], measures: [] }),
      w("slicer", "Período", { x: 8, y: 2, w: 4, h: 2 }, { dimensions: ["date"], measures: [] }),
      w("bar", "VGV por corretor", { x: 0, y: 4, w: 7, h: 5 }, { measures: ["VGV"], dimensions: ["corretor"], limit: 15 }, { config: { ...brl, showDataLabels: true } }),
      w("bar", "Unidades por corretor", { x: 7, y: 4, w: 5, h: 5 }, { measures: ["Unidades"], dimensions: ["corretor"], limit: 15 }),
      w("line", "Ritmo da equipe", { x: 0, y: 9, w: 8, h: 5 }, { measures: ["VGV"], dimensions: ["date"], limit: 24 }, { config: brl }),
      w("pie", "Peso da comissão", { x: 8, y: 9, w: 4, h: 5 }, { measures: ["Comissão"], dimensions: ["corretor"], limit: 8 }, { config: brl }),
      w("table", "Ranking", { x: 0, y: 14, w: 12, h: 5 }, { measures: ["VGV", "Unidades", "Comissão", "Ticket médio"], dimensions: ["corretor"], limit: 40 }, { config: { ...brlFull, showTotals: true, zebra: true } }),
    ],
  },
  {
    id: "imobiliario-inadimplencia",
    name: "Inadimplência de aluguel",
    category: "imobiliario",
    description: "Atraso, concentração por inquilino e status da cobrança.",
    pain: "O atraso acumula e a cobrança só entra quando o caixa do proprietário aperta.",
    icon: "alert",
    needs: ["aluguel ou valor", "inquilino ou status"],
    measures: IMOBILIARIO_LOCACAO_MEASURES,
    widgets: [
      w("kpi", "Em aberto", { x: 0, y: 0, w: 4, h: 2 }, { measures: ["Inadimplência"] }, { config: brl }),
      w("kpi", "Contratos", { x: 4, y: 0, w: 4, h: 2 }, { measures: ["Contratos"] }),
      w("slicer", "Status", { x: 8, y: 0, w: 4, h: 2 }, { dimensions: ["status"], measures: [] }, { config: { slicerStyle: "buttons" } }),
      w("bar", "Por inquilino", { x: 0, y: 2, w: 7, h: 5 }, { measures: ["Inadimplência"], dimensions: ["inquilino"], limit: 15 }, { config: brl }),
      w("pie", "Por status", { x: 7, y: 2, w: 5, h: 5 }, { measures: ["Inadimplência"], dimensions: ["status"], limit: 8 }, { config: brl }),
      w("bar", "Por bairro", { x: 0, y: 7, w: 6, h: 5 }, { measures: ["Inadimplência"], dimensions: ["bairro"], limit: 12 }, { config: brl }),
      w("table", "Cobrança", { x: 6, y: 7, w: 6, h: 5 }, { measures: ["Inadimplência", "Aluguel"], dimensions: ["inquilino", "imovel"], limit: 40 }, { config: { ...brlFull, zebra: true } }),
    ],
  },
  {
    id: "advocacia-processos",
    name: "Carteira processual",
    category: "advocacia",
    description: "Visão executiva da carteira: processos, clientes, valor da causa, êxito e carga de trabalho.",
    pain: "A carteira está espalhada entre sistemas e a gestão não consegue priorizar os casos.",
    icon: "scale",
    popular: true,
    needs: ["cliente", "processo", "status ou fase", "valor da causa"],
    measures: ADVOCACIA_PROCESSOS_MEASURES,
    widgets: [
      w("kpi", "Processos", { x: 0, y: 0, w: 3, h: 2 }, { measures: ["Processos"] }),
      w("kpi", "Clientes", { x: 3, y: 0, w: 3, h: 2 }, { measures: ["Clientes"] }),
      w("kpi", "Valor da causa", { x: 6, y: 0, w: 3, h: 2 }, { measures: ["Valor da causa"] }, { config: brl }),
      w("kpi", "Taxa de êxito", { x: 9, y: 0, w: 3, h: 2 }, { measures: ["Taxa de êxito"] }, { config: pct1 }),
      w("slicer", "Área jurídica", { x: 0, y: 2, w: 3, h: 2 }, { dimensions: ["area"], measures: [] }, { config: { slicerStyle: "buttons" } }),
      w("slicer", "Fase", { x: 3, y: 2, w: 3, h: 2 }, { dimensions: ["fase"], measures: [] }),
      w("slicer", "Risco", { x: 6, y: 2, w: 3, h: 2 }, { dimensions: ["risco"], measures: [] }),
      w("slicer", "Advogado", { x: 9, y: 2, w: 3, h: 2 }, { dimensions: ["advogado"], measures: [] }),
      w("line", "Processos distribuídos", { x: 0, y: 4, w: 7, h: 5 }, { measures: ["Processos"], dimensions: ["data_distribuicao"], limit: 24 }),
      w("pie", "Por área", { x: 7, y: 4, w: 5, h: 5 }, { measures: ["Processos"], dimensions: ["area"], limit: 10 }),
      w("bar", "Valor por advogado", { x: 0, y: 9, w: 6, h: 5 }, { measures: ["Valor da causa"], dimensions: ["advogado"], limit: 12 }, { config: brl }),
      w("bar", "Processos por status", { x: 6, y: 9, w: 6, h: 5 }, { measures: ["Processos"], dimensions: ["status"], limit: 10 }),
      w("table", "Detalhe da carteira", { x: 0, y: 14, w: 12, h: 5 }, { measures: ["Valor da causa", "Honorários previstos", "Horas trabalhadas"], dimensions: ["processo", "cliente", "advogado", "status"], limit: 50 }, { config: { ...brlFull, zebra: true } }),
    ],
  },
  {
    id: "advocacia-prazos-riscos",
    name: "Prazos e riscos jurídicos",
    category: "advocacia",
    description: "Controle de prazos, tarefas críticas e exposição financeira por processo e responsável.",
    pain: "Um prazo perdido pode gerar custo, risco para o cliente e desgaste para o escritório.",
    icon: "alert",
    popular: true,
    needs: ["processo", "prazo", "responsável", "risco ou status do prazo"],
    measures: ADVOCACIA_PRAZOS_MEASURES,
    widgets: [
      w("kpi", "Prazos", { x: 0, y: 0, w: 3, h: 2 }, { measures: ["Prazos"] }),
      w("kpi", "Vencidos", { x: 3, y: 0, w: 3, h: 2 }, { measures: ["Prazos vencidos"] }),
      w("kpi", "Próximos", { x: 6, y: 0, w: 3, h: 2 }, { measures: ["Prazos próximos"] }),
      w("kpi", "Valor em risco", { x: 9, y: 0, w: 3, h: 2 }, { measures: ["Valor em risco"] }, { config: brl }),
      w("slicer", "Status do prazo", { x: 0, y: 2, w: 4, h: 2 }, { dimensions: ["status_prazo"], measures: [] }, { config: { slicerStyle: "buttons" } }),
      w("slicer", "Responsável", { x: 4, y: 2, w: 4, h: 2 }, { dimensions: ["responsavel"], measures: [] }),
      w("slicer", "Risco", { x: 8, y: 2, w: 4, h: 2 }, { dimensions: ["risco"], measures: [] }),
      w("bar", "Prazos por responsável", { x: 0, y: 4, w: 7, h: 5 }, { measures: ["Prazos"], dimensions: ["responsavel"], limit: 15 }),
      w("pie", "Por status", { x: 7, y: 4, w: 5, h: 5 }, { measures: ["Prazos"], dimensions: ["status_prazo"], limit: 8 }),
      w("line", "Agenda de prazos", { x: 0, y: 9, w: 6, h: 5 }, { measures: ["Prazos"], dimensions: ["data_prazo"], limit: 60 }),
      w("bar", "Exposição por área", { x: 6, y: 9, w: 6, h: 5 }, { measures: ["Valor em risco"], dimensions: ["area"], limit: 12 }, { config: brl }),
      w("table", "Prazos críticos", { x: 0, y: 14, w: 12, h: 5 }, { measures: ["Valor em risco"], dimensions: ["processo", "cliente", "data_prazo", "responsavel", "risco"], limit: 50 }, { config: { ...brlFull, zebra: true } }),
    ],
  },
  {
    id: "advocacia-financeiro",
    name: "Financeiro do escritório",
    category: "advocacia",
    description: "Honorários previstos, recebimentos, horas trabalhadas e rentabilidade por cliente e área.",
    pain: "O escritório fatura, mas não sabe quais clientes e áreas geram margem de verdade.",
    icon: "wallet",
    needs: ["cliente", "honorários", "recebimentos", "horas trabalhadas"],
    measures: ADVOCACIA_FINANCEIRO_MEASURES,
    widgets: [
      w("kpi", "Honorários previstos", { x: 0, y: 0, w: 3, h: 2 }, { measures: ["Honorários previstos"] }, { config: brl }),
      w("kpi", "Honorários recebidos", { x: 3, y: 0, w: 3, h: 2 }, { measures: ["Honorários recebidos"] }, { config: brl }),
      w("kpi", "Em aberto", { x: 6, y: 0, w: 3, h: 2 }, { measures: ["Em aberto"] }, { config: brl }),
      w("kpi", "Horas trabalhadas", { x: 9, y: 0, w: 3, h: 2 }, { measures: ["Horas trabalhadas"] }),
      w("slicer", "Cliente", { x: 0, y: 2, w: 4, h: 2 }, { dimensions: ["cliente"], measures: [] }, { config: { slicerStyle: "dropdown", slicerSearch: true } }),
      w("slicer", "Área", { x: 4, y: 2, w: 4, h: 2 }, { dimensions: ["area"], measures: [] }),
      w("slicer", "Status de recebimento", { x: 8, y: 2, w: 4, h: 2 }, { dimensions: ["status_recebimento"], measures: [] }, { config: { slicerStyle: "buttons" } }),
      w("bar", "Honorários por cliente", { x: 0, y: 4, w: 7, h: 5 }, { measures: ["Honorários previstos"], dimensions: ["cliente"], limit: 15 }, { config: brl }),
      w("pie", "Por área", { x: 7, y: 4, w: 5, h: 5 }, { measures: ["Honorários previstos"], dimensions: ["area"], limit: 10 }, { config: brl }),
      w("line", "Previsto x recebido", { x: 0, y: 9, w: 8, h: 5 }, { measures: ["Honorários previstos", "Honorários recebidos"], dimensions: ["mes"], limit: 24 }, { config: brl }),
      w("bar", "Horas por advogado", { x: 8, y: 9, w: 4, h: 5 }, { measures: ["Horas trabalhadas"], dimensions: ["advogado"], limit: 12 }),
      w("table", "Carteira financeira", { x: 0, y: 14, w: 12, h: 5 }, { measures: ["Honorários previstos", "Honorários recebidos", "Horas trabalhadas", "Ticket médio"], dimensions: ["cliente", "processo", "advogado"], limit: 50 }, { config: { ...brlFull, zebra: true } }),
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
          limit: widget.query.limit ?? DEFAULT_QUERY_LIMIT,
        }
      : undefined;
    return {
      ...widget,
      id: crypto.randomUUID(),
      query,
    };
  });
}

"use client";

import type { WidgetConfig } from "@/components/WidgetView";
import type { DatasetListItem, SemanticModel } from "@/lib/semantic";
import { dimensionKey, measureKey, modelForDataset, remapQueryToModel } from "@/lib/semantic";
import { asJoinField } from "@/lib/widget-errors";
import {
  BRAND_PALETTE,
  inspectorCaps,
  type CompactMode,
  type CurrencyCode,
  type LegendPosition,
  type TitleAlign,
} from "@/lib/widget-config";
import { Badge, FieldLabel, Input, Select, Textarea, Toggle, Button, cn } from "@/components/ui";
import { ChevronDown, Plus, Trash2 } from "lucide-react";
import { useState, type ReactNode } from "react";
import type { Widget, WidgetType, QueryJoin } from "@/components/WidgetView";

type CatalogItem = { type: WidgetType; label: string };

export function WidgetInspector({
  widget,
  catalog,
  model,
  visibleDatasets,
  sourceOptions,
  sourceFilter,
  onSourceFilter,
  onPreferredDataset,
  semanticModels,
  onUpdate,
  onDrillUp,
}: {
  widget: Widget;
  catalog: CatalogItem[];
  model: SemanticModel | null;
  visibleDatasets: DatasetListItem[];
  sourceOptions: { id: string; name: string }[];
  sourceFilter: string;
  onSourceFilter: (v: string) => void;
  onPreferredDataset: (id: string) => void;
  semanticModels: any[];
  onUpdate: (fn: (w: Widget) => Widget) => void;
  onDrillUp: () => void;
}) {
  const caps = inspectorCaps(widget.type);
  const cfg = widget.config || {};
  const setCfg = (partial: Partial<WidgetConfig>) =>
    onUpdate((w) => ({ ...w, config: { ...w.config, ...partial } }));

  return (
    <aside className="w-80 min-h-0 shrink-0 overflow-y-auto rounded-2xl border border-line bg-surface p-4 shadow-sm">
      <div className="mb-2 flex items-center justify-between">
        <span className="text-[13px] font-semibold text-ink">Propriedades</span>
        <Badge tone="accent">{catalog.find((t) => t.type === widget.type)?.label || widget.type}</Badge>
      </div>
      <p className="mb-3 text-[11px] text-mute">As alterações aplicam-se imediatamente no canvas.</p>

      <Section title="Dados" defaultOpen>
        <FieldLabel label="Título">
          <Input value={widget.title} onChange={(e) => onUpdate((w) => ({ ...w, title: e.target.value }))} />
        </FieldLabel>
        {caps.query && (
          <FieldLabel label="Tipo de visualização">
            <Select
              value={widget.type}
              onChange={(e) => onUpdate((w) => ({ ...w, type: e.target.value as WidgetType }))}
            >
              {catalog
                .filter((t) => !["text", "image", "markdown", "iframe"].includes(t.type))
                .map((t) => (
                  <option key={t.type} value={t.type}>
                    {t.label}
                  </option>
                ))}
            </Select>
          </FieldLabel>
        )}
        {widget.type === "text" && (
          <FieldLabel label="Texto">
            <Textarea value={widget.text} onChange={(e) => onUpdate((w) => ({ ...w, text: e.target.value }))} />
          </FieldLabel>
        )}
        {widget.type === "iframe" && (
          <FieldLabel label="URL do embed" hint="Use apenas fontes confiáveis. Conteúdo externo pode definir cookies.">
            <Input value={cfg.url || ""} onChange={(e) => setCfg({ url: e.target.value })} placeholder="https://..." />
          </FieldLabel>
        )}
        {widget.type === "image" && (
          <FieldLabel label="URL da imagem">
            <Input value={cfg.imageUrl || ""} onChange={(e) => setCfg({ imageUrl: e.target.value })} />
          </FieldLabel>
        )}
        {widget.type === "markdown" && (
          <FieldLabel label="Markdown">
            <Textarea value={cfg.markdown || ""} onChange={(e) => setCfg({ markdown: e.target.value })} />
          </FieldLabel>
        )}
        {caps.query && (
          <QueryFields
            widget={widget}
            model={model}
            visibleDatasets={visibleDatasets}
            sourceOptions={sourceOptions}
            sourceFilter={sourceFilter}
            onSourceFilter={onSourceFilter}
            onPreferredDataset={onPreferredDataset}
            semanticModels={semanticModels}
            onUpdate={onUpdate}
            onDrillUp={onDrillUp}
          />
        )}
      </Section>

      <Section title="Estilo" defaultOpen>
        <ToggleRow label="Mostrar título" checked={cfg.showTitle !== false} onChange={(v) => setCfg({ showTitle: v })} />
        <FieldLabel label="Alinhamento do título">
          <Select value={cfg.titleAlign || "left"} onChange={(e) => setCfg({ titleAlign: e.target.value as TitleAlign })}>
            <option value="left">Esquerda</option>
            <option value="center">Centro</option>
            <option value="right">Direita</option>
          </Select>
        </FieldLabel>
        {caps.color && (
          <FieldLabel label="Cor principal">
            <ColorSwatches value={cfg.color} onChange={(color) => setCfg({ color })} />
          </FieldLabel>
        )}
        {caps.waterfall && (
          <FieldLabel label="Cor negativa">
            <ColorSwatches value={cfg.colorNegative || "#EF4444"} onChange={(colorNegative) => setCfg({ colorNegative })} />
          </FieldLabel>
        )}
        {caps.kpi && (
          <FieldLabel label="Tamanho do valor">
            <Select value={cfg.fontSize || "md"} onChange={(e) => setCfg({ fontSize: e.target.value as WidgetConfig["fontSize"] })}>
              <option value="sm">Pequeno</option>
              <option value="md">Médio</option>
              <option value="lg">Grande</option>
            </Select>
          </FieldLabel>
        )}
        {caps.kpi && (
          <FieldLabel label="Alinhamento do KPI">
            <Select value={cfg.kpiAlign || "left"} onChange={(e) => setCfg({ kpiAlign: e.target.value as WidgetConfig["kpiAlign"] })}>
              <option value="left">Esquerda</option>
              <option value="center">Centro</option>
            </Select>
          </FieldLabel>
        )}
        {caps.cartesian && widget.type === "bar" && (
          <ToggleRow label="Barras horizontais" checked={!!cfg.horizontal} onChange={(v) => setCfg({ horizontal: v })} />
        )}
        {caps.cartesian && (
          <ToggleRow label="Empilhado" checked={!!cfg.stacked} onChange={(v) => setCfg({ stacked: v })} />
        )}
        {(widget.type === "line" || widget.type === "area") && (
          <ToggleRow label="Linha suave" checked={cfg.smooth !== false} onChange={(v) => setCfg({ smooth: v })} />
        )}
        {caps.table && (
          <>
            <ToggleRow label="Linhas zebradas" checked={!!cfg.zebra} onChange={(v) => setCfg({ zebra: v })} />
            <ToggleRow label="Fixar cabeçalho" checked={cfg.freezeHeader !== false} onChange={(v) => setCfg({ freezeHeader: v })} />
          </>
        )}
        <div className="grid grid-cols-2 gap-2">
          <FieldLabel label="Largura">
            <Input
              type="number"
              min={2}
              max={12}
              value={widget.layout.w}
              onChange={(e) => onUpdate((w) => ({ ...w, layout: { ...w.layout, w: Number(e.target.value) } }))}
            />
          </FieldLabel>
          <FieldLabel label="Altura">
            <Input
              type="number"
              min={2}
              max={20}
              value={widget.layout.h}
              onChange={(e) => onUpdate((w) => ({ ...w, layout: { ...w.layout, h: Number(e.target.value) } }))}
            />
          </FieldLabel>
        </div>
      </Section>

      {caps.axes && (
        <Section title="Eixos">
          <ToggleRow label="Eixo X" checked={cfg.showXAxis !== false} onChange={(v) => setCfg({ showXAxis: v })} />
          <ToggleRow label="Eixo Y" checked={cfg.showYAxis !== false} onChange={(v) => setCfg({ showYAxis: v })} />
          <ToggleRow label="Grelha" checked={cfg.showGrid !== false} onChange={(v) => setCfg({ showGrid: v })} />
          <FieldLabel label="Etiqueta do eixo X">
            <Input value={cfg.xAxisLabel || ""} onChange={(e) => setCfg({ xAxisLabel: e.target.value })} placeholder="Automático" />
          </FieldLabel>
          <FieldLabel label="Etiqueta do eixo Y">
            <Input value={cfg.yAxisLabel || ""} onChange={(e) => setCfg({ yAxisLabel: e.target.value })} placeholder="Automático" />
          </FieldLabel>
          <FieldLabel label="Rotação das categorias" hint="0 a 90 graus">
            <Input
              type="number"
              min={0}
              max={90}
              value={cfg.xAxisRotate ?? 0}
              onChange={(e) => setCfg({ xAxisRotate: Number(e.target.value) })}
            />
          </FieldLabel>
        </Section>
      )}

      {(caps.legend || caps.dataLabels) && (
        <Section title="Legenda">
          {caps.legend && (
            <>
              <ToggleRow
                label="Mostrar legenda"
                checked={widget.type === "pie" ? cfg.showLegend !== false : !!cfg.showLegend}
                onChange={(v) => setCfg({ showLegend: v })}
              />
              <FieldLabel label="Posição">
                <Select
                  value={cfg.legendPosition || "top"}
                  onChange={(e) => setCfg({ legendPosition: e.target.value as LegendPosition })}
                >
                  <option value="top">Topo</option>
                  <option value="bottom">Base</option>
                  <option value="left">Esquerda</option>
                  <option value="right">Direita</option>
                </Select>
              </FieldLabel>
            </>
          )}
          {caps.dataLabels && (
            <ToggleRow
              label="Rótulos de dados"
              checked={
                ["funnel", "treemap", "heatmap", "pie"].includes(widget.type)
                  ? cfg.showDataLabels !== false
                  : !!cfg.showDataLabels
              }
              onChange={(v) => setCfg({ showDataLabels: v })}
            />
          )}
          <ToggleRow label="Tooltip" checked={cfg.showTooltip !== false} onChange={(v) => setCfg({ showTooltip: v })} />
        </Section>
      )}

      {caps.format && (
        <Section title="Formato">
          {caps.kpi && (
            <>
              <FieldLabel label="Meta" hint={widget.type === "kpi" ? "Opcional. Mostra progresso vs. objectivo." : undefined}>
                <Input
                  type="number"
                  value={widget.type === "kpi_goal" ? (cfg.goal ?? 100) : (cfg.goal ?? "")}
                  onChange={(e) => setCfg({ goal: e.target.value === "" ? undefined : Number(e.target.value) })}
                  placeholder="Ex.: 100000"
                />
              </FieldLabel>
              <ToggleRow label="Mostrar tendência" checked={!!cfg.showTrend} onChange={(v) => setCfg({ showTrend: v })} />
              {cfg.showTrend && (
                <>
                  <FieldLabel label="Variação (%)" hint="Valor manual até haver comparação automática de períodos.">
                    <Input
                      type="number"
                      value={cfg.variance ?? ""}
                      onChange={(e) => setCfg({ variance: e.target.value === "" ? undefined : Number(e.target.value) })}
                      placeholder="Ex.: 4.2"
                    />
                  </FieldLabel>
                  <FieldLabel label="Etiqueta de comparação">
                    <Input
                      value={cfg.comparisonLabel || ""}
                      onChange={(e) => setCfg({ comparisonLabel: e.target.value })}
                      placeholder="vs. período anterior"
                    />
                  </FieldLabel>
                </>
              )}
            </>
          )}
          {caps.gauge && (
            <>
              <FieldLabel label="Etiqueta do medidor">
                <Input value={cfg.gaugeLabel || ""} onChange={(e) => setCfg({ gaugeLabel: e.target.value })} placeholder="Valor" />
              </FieldLabel>
              <div className="grid grid-cols-3 gap-2">
                <FieldLabel label="Mín.">
                  <Input type="number" value={cfg.min ?? 0} onChange={(e) => setCfg({ min: Number(e.target.value) })} />
                </FieldLabel>
                <FieldLabel label="Máx.">
                  <Input type="number" value={cfg.max ?? 100} onChange={(e) => setCfg({ max: Number(e.target.value) })} />
                </FieldLabel>
                <FieldLabel label="Meta">
                  <Input type="number" value={cfg.target ?? 80} onChange={(e) => setCfg({ target: Number(e.target.value) })} />
                </FieldLabel>
              </div>
            </>
          )}
          {caps.waterfall && (
            <FieldLabel label="Categorias negativas" hint="Separadas por vírgula. Vazio usa o sinal do valor.">
              <Input
                value={cfg.waterfallNegativeCategories || ""}
                onChange={(e) => setCfg({ waterfallNegativeCategories: e.target.value })}
                placeholder="Despesas, Custos"
              />
            </FieldLabel>
          )}
          {caps.table && (
            <>
              <ToggleRow label="Linha de totais" checked={!!cfg.showTotals} onChange={(v) => setCfg({ showTotals: v })} />
              <FieldLabel label="Limite de linhas visíveis">
                <Input
                  type="number"
                  min={5}
                  max={500}
                  value={cfg.rowLimit ?? 20}
                  onChange={(e) => setCfg({ rowLimit: Number(e.target.value) })}
                />
              </FieldLabel>
            </>
          )}
          <FieldLabel label="Moeda">
            <Select value={cfg.currency || ""} onChange={(e) => setCfg({ currency: e.target.value as CurrencyCode })}>
              <option value="">Nenhuma</option>
              <option value="BRL">Real (R$)</option>
              <option value="USD">Dólar (US$)</option>
              <option value="EUR">Euro (€)</option>
            </Select>
          </FieldLabel>
          <div className="grid grid-cols-2 gap-2">
            <FieldLabel label="Prefixo">
              <Input value={cfg.prefix || ""} onChange={(e) => setCfg({ prefix: e.target.value })} placeholder="R$ " />
            </FieldLabel>
            <FieldLabel label="Sufixo">
              <Input value={cfg.suffix || ""} onChange={(e) => setCfg({ suffix: e.target.value })} placeholder="%" />
            </FieldLabel>
          </div>
          <FieldLabel label="Casas decimais">
            <Input
              type="number"
              min={0}
              max={6}
              value={cfg.decimals ?? 0}
              onChange={(e) => setCfg({ decimals: Number(e.target.value) })}
            />
          </FieldLabel>
          <FieldLabel label="Compactação">
            <Select value={cfg.compact || "none"} onChange={(e) => setCfg({ compact: e.target.value as CompactMode })}>
              <option value="none">Completo</option>
              <option value="auto">Automático (K/M/B)</option>
              <option value="k">Milhares (K)</option>
              <option value="m">Milhões (M)</option>
              <option value="b">Mil milhões (B)</option>
            </Select>
          </FieldLabel>
        </Section>
      )}

      {caps.interaction && (
        <Section title="Interação" defaultOpen={widget.type === "slicer"}>
          {caps.slicer && (
            <>
              <ToggleRow label="Selecção múltipla" checked={!!cfg.multiSelect} onChange={(v) => setCfg({ multiSelect: v })} />
              <ToggleRow label="Campo de pesquisa" checked={cfg.slicerSearch !== false} onChange={(v) => setCfg({ slicerSearch: v })} />
              <FieldLabel label="Estilo">
                <Select
                  value={cfg.slicerStyle || "list"}
                  onChange={(e) => setCfg({ slicerStyle: e.target.value as WidgetConfig["slicerStyle"] })}
                >
                  <option value="list">Lista</option>
                  <option value="buttons">Botões</option>
                  <option value="dropdown">Dropdown</option>
                </Select>
              </FieldLabel>
            </>
          )}
          <p className="text-[11px] text-mute">
            Filtros deste widget. Slicers e cliques no canvas aplicam filtros globais automaticamente.
          </p>
          {(widget.query?.filters || []).map((f, i) => (
            <div key={i} className="flex items-center gap-2 rounded-lg border border-line p-2">
              <Input
                value={f.dimension}
                onChange={(e) =>
                  onUpdate((w) => ({
                    ...w,
                    query: {
                      ...w.query,
                      filters: (w.query?.filters || []).map((x, j) => (j === i ? { ...x, dimension: e.target.value } : x)),
                    },
                  }))
                }
                className="w-24"
              />
              <Select
                value={f.op}
                onChange={(e) =>
                  onUpdate((w) => ({
                    ...w,
                    query: {
                      ...w.query,
                      filters: (w.query?.filters || []).map((x, j) => (j === i ? { ...x, op: e.target.value as any } : x)),
                    },
                  }))
                }
              >
                <option value="eq">=</option>
                <option value="neq">≠</option>
                <option value="in">em</option>
              </Select>
              <Input
                value={String(f.value)}
                onChange={(e) =>
                  onUpdate((w) => ({
                    ...w,
                    query: {
                      ...w.query,
                      filters: (w.query?.filters || []).map((x, j) => (j === i ? { ...x, value: e.target.value } : x)),
                    },
                  }))
                }
              />
              <Button
                variant="ghost"
                size="icon"
                onClick={() =>
                  onUpdate((w) => ({
                    ...w,
                    query: { ...w.query, filters: (w.query?.filters || []).filter((_, j) => j !== i) },
                  }))
                }
              >
                <Trash2 size={14} />
              </Button>
            </div>
          ))}
          <Button
            variant="secondary"
            size="sm"
            onClick={() =>
              onUpdate((w) => ({
                ...w,
                query: { ...w.query, filters: [...(w.query?.filters || []), { dimension: "", op: "eq", value: "" }] },
              }))
            }
          >
            <Plus size={14} /> Adicionar filtro
          </Button>
        </Section>
      )}
    </aside>
  );
}

function modelColumns(model: SemanticModel | null | undefined) {
  const seen = new Set<string>();
  const out: { key: string; label: string }[] = [];
  const add = (key: string, label?: string) => {
    const k = (key || "").trim();
    if (!k || seen.has(k)) return;
    seen.add(k);
    out.push({ key: k, label: (label || k).trim() || k });
  };
  for (const d of model?.dimensions || []) add(dimensionKey(d), d.name || d.column);
  if (model?.time_column) add(model.time_column);
  for (const m of model?.measures || []) {
    if (m.column && m.column !== "*") add(m.column, measureKey(m));
  }
  return out;
}

function MeasureOptions({
  model,
  joinModel,
  joinName,
}: {
  model: SemanticModel | null;
  joinModel: SemanticModel | null;
  joinName?: string;
}) {
  return (
    <>
      <optgroup label="Este conjunto">
        {(model?.measures || []).map((m) => (
          <option key={measureKey(m)} value={measureKey(m)}>
            {measureKey(m)}
          </option>
        ))}
      </optgroup>
      {joinModel && (
        <optgroup label={joinName || "Conjunto cruzado"}>
          {(joinModel.measures || []).map((m) => (
            <option key={asJoinField(measureKey(m))} value={asJoinField(measureKey(m))}>
              {measureKey(m)}
            </option>
          ))}
        </optgroup>
      )}
    </>
  );
}

function DimensionOptions({
  model,
  joinModel,
  joinName,
}: {
  model: SemanticModel | null;
  joinModel: SemanticModel | null;
  joinName?: string;
}) {
  return (
    <>
      <optgroup label="Este conjunto">
        {(model?.dimensions || []).map((d) => (
          <option key={dimensionKey(d)} value={dimensionKey(d)}>
            {d.name || d.column}
          </option>
        ))}
      </optgroup>
      {joinModel && (
        <optgroup label={joinName || "Conjunto cruzado"}>
          {(joinModel.dimensions || []).map((d) => (
            <option key={asJoinField(dimensionKey(d))} value={asJoinField(dimensionKey(d))}>
              {d.name || d.column}
            </option>
          ))}
        </optgroup>
      )}
    </>
  );
}

function patchJoin(onUpdate: (fn: (w: Widget) => Widget) => void, partial: Partial<QueryJoin> | null) {
  onUpdate((w) => {
    if (partial === null) {
      return {
        ...w,
        query: {
          ...w.query,
          joins: undefined,
          measures: (w.query?.measures || []).filter((m) => !m.startsWith("join.")),
          dimensions: (w.query?.dimensions || []).filter((d) => !d.startsWith("join.")),
        },
      };
    }
    const prev: QueryJoin = w.query?.joins?.[0] || { dataset_id: "", from_column: "", to_column: "", match: "both" };
    const next: QueryJoin = { ...prev, ...partial };
    return { ...w, query: { ...w.query, joins: next.dataset_id ? [next] : undefined } };
  });
}

function QueryFields({
  widget,
  model,
  visibleDatasets,
  sourceOptions,
  sourceFilter,
  onSourceFilter,
  onPreferredDataset,
  semanticModels,
  onUpdate,
  onDrillUp,
}: {
  widget: Widget;
  model: SemanticModel | null;
  visibleDatasets: DatasetListItem[];
  sourceOptions: { id: string; name: string }[];
  sourceFilter: string;
  onSourceFilter: (v: string) => void;
  onPreferredDataset: (id: string) => void;
  semanticModels: any[];
  onUpdate: (fn: (w: Widget) => Widget) => void;
  onDrillUp: () => void;
}) {
  const measureLabel =
    widget.type === "scatter"
      ? "Métrica X"
      : widget.type === "funnel" || widget.type === "treemap" || widget.type === "waterfall"
        ? "Medida"
        : "Métrica";
  const dimLabel =
    widget.type === "slicer"
      ? "Dimensão do slicer"
      : widget.type === "heatmap"
        ? "Dimensão X"
        : widget.type === "sparkline"
          ? "Dimensão temporal"
          : "Dimensão";
  const join = widget.query?.joins?.[0];
  const joinModel = join?.dataset_id ? modelForDataset(semanticModels, join.dataset_id) : null;
  const joinName = visibleDatasets.find((d) => d.id === join?.dataset_id)?.name;
  const relatedDatasets = visibleDatasets.filter((d) => d.id && d.id !== widget.query?.dataset_id);
  const extraStart = widget.type === "heatmap" ? 2 : 1;
  const canBreak = !["kpi", "kpi_goal", "metric_group", "gauge", "sparkline", "slicer"].includes(widget.type);
  const extraDims = (widget.query?.dimensions || []).slice(extraStart);
  const primaryCols = modelColumns(model);
  const joinCols = modelColumns(joinModel);
  const setJoin = (partial: Partial<QueryJoin> | null) => patchJoin(onUpdate, partial);

  return (
    <>
      {sourceOptions.length > 0 && (
        <FieldLabel label="Origem">
          <Select value={sourceFilter} onChange={(e) => onSourceFilter(e.target.value)}>
            <option value="">Todos os conjuntos</option>
            {sourceOptions.map((s) => (
              <option key={s.id} value={s.id}>
                Conector {s.name}
              </option>
            ))}
          </Select>
        </FieldLabel>
      )}
      <FieldLabel label="Conjunto">
        <Select
          value={widget.query?.dataset_id || ""}
          onChange={(e) => {
            const nextId = e.target.value;
            onPreferredDataset(nextId);
            const nextModel = modelForDataset(semanticModels, nextId);
            onUpdate((w) => ({
              ...w,
              query: remapQueryToModel(
                {
                  ...w.query,
                  dataset_id: nextId,
                  joins: undefined,
                  measures: (w.query?.measures || []).filter((m) => !m.startsWith("join.")),
                  dimensions: (w.query?.dimensions || []).filter((d) => !d.startsWith("join.")),
                },
                w.type,
                nextModel,
              ),
            }));
          }}
        >
          <option value="">—</option>
          {visibleDatasets.map((ds) => (
            <option key={ds.id} value={ds.id}>
              {ds.name}
              {ds.source_name ? ` · ${ds.source_name}` : ""}
            </option>
          ))}
        </Select>
      </FieldLabel>
      {widget.query?.dataset_id && relatedDatasets.length === 0 && (
        <p className="text-[11px] leading-snug text-mute">
          Para cruzar com outro conjunto, importe um segundo conjunto neste espaço de trabalho.
        </p>
      )}
      {widget.query?.dataset_id && relatedDatasets.length > 0 && (
        <div className="space-y-2 rounded-xl border border-line bg-surface-2/60 p-2.5">
          <div className="flex items-start justify-between gap-2">
            <div>
              <div className="text-[13px] font-medium text-ink">Cruzar com outro conjunto</div>
              <div className="text-[11px] text-mute">Use campos do conjunto relacionado nas métricas e nas quebras.</div>
            </div>
            {join?.dataset_id && (
              <button type="button" className="shrink-0 text-[11px] text-danger" onClick={() => setJoin(null)}>
                Remover
              </button>
            )}
          </div>
          <FieldLabel label="Conjunto relacionado">
            <Select
              value={join?.dataset_id || ""}
              onChange={(e) =>
                setJoin(e.target.value ? { dataset_id: e.target.value, from_column: "", to_column: "" } : null)
              }
            >
              <option value="">Nenhum</option>
              {relatedDatasets.map((ds) => (
                <option key={ds.id} value={ds.id}>
                  {ds.name}
                  {ds.source_name ? ` · ${ds.source_name}` : ""}
                </option>
              ))}
            </Select>
          </FieldLabel>
          {join?.dataset_id && (
            <>
              <FieldLabel label="Coluna deste conjunto" hint="A chave à esquerda, por exemplo empresa">
                <Select value={join.from_column || ""} onChange={(e) => setJoin({ from_column: e.target.value })}>
                  <option value="">—</option>
                  {primaryCols.map((c) => (
                    <option key={c.key} value={c.key}>
                      {c.label}
                    </option>
                  ))}
                </Select>
              </FieldLabel>
              <FieldLabel label="Coluna do conjunto cruzado" hint="A mesma chave no outro conjunto">
                <Select value={join.to_column || ""} onChange={(e) => setJoin({ to_column: e.target.value })}>
                  <option value="">—</option>
                  {joinCols.map((c) => (
                    <option key={c.key} value={c.key}>
                      {c.label}
                    </option>
                  ))}
                </Select>
              </FieldLabel>
              <FieldLabel label="Como cruzar">
                <Select
                  value={join.match || "both"}
                  onChange={(e) => setJoin({ match: e.target.value as QueryJoin["match"] })}
                >
                  <option value="both">Só linhas que existem nos dois</option>
                  <option value="all_left">Manter todas as linhas deste conjunto</option>
                </Select>
              </FieldLabel>
            </>
          )}
        </div>
      )}
      {widget.type !== "slicer" && widget.type !== "metric_group" && (
        <FieldLabel label={measureLabel}>
          <Select
            value={widget.query?.measures?.[0] || ""}
            onChange={(e) => {
              const liveId = visibleDatasets.some((d) => d.id === widget.query?.dataset_id)
                ? widget.query?.dataset_id
                : visibleDatasets[0]?.id || widget.query?.dataset_id;
              if (liveId && liveId !== widget.query?.dataset_id) onPreferredDataset(liveId);
              onUpdate((w) => {
                const base =
                  liveId && liveId !== w.query?.dataset_id
                    ? remapQueryToModel({ ...w.query, dataset_id: liveId }, w.type, modelForDataset(semanticModels, liveId))
                    : w.query;
                return {
                  ...w,
                  query: {
                    ...base,
                    dataset_id: liveId || base?.dataset_id,
                    measures: e.target.value
                      ? ["scatter"].includes(widget.type)
                        ? [e.target.value, ...(base?.measures || []).slice(1)]
                        : [e.target.value]
                      : ["scatter"].includes(widget.type)
                        ? (base?.measures || []).slice(1)
                        : [],
                  },
                  config: widget.type === "scatter" ? { ...w.config, xMeasure: e.target.value } : w.config,
                };
              });
            }}
          >
            <option value="">—</option>
            <MeasureOptions model={model} joinModel={joinModel} joinName={joinName} />
          </Select>
        </FieldLabel>
      )}
      {widget.type === "scatter" && (
        <FieldLabel label="Métrica Y">
          <Select
            value={widget.query?.measures?.[1] || ""}
            onChange={(e) => {
              const first = widget.query?.measures?.[0] || "";
              onUpdate((w) => ({
                ...w,
                query: { ...w.query, measures: [first, e.target.value].filter(Boolean) },
                config: { ...w.config, xMeasure: first, yMeasure: e.target.value },
              }));
            }}
          >
            <option value="">—</option>
            <MeasureOptions model={model} joinModel={joinModel} joinName={joinName} />
          </Select>
        </FieldLabel>
      )}
      {widget.type === "metric_group" && (
        <FieldLabel label="Métricas do grupo">
          <div className="grid grid-cols-1 gap-1.5">
            {(model?.measures || []).map((m) => {
              const name = measureKey(m);
              const checked = widget.query?.measures?.includes(name) || false;
              return (
                <label key={name} className="flex items-center gap-2 text-[12px] text-ink">
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => {
                      const ms = widget.query?.measures || [];
                      const next = checked ? ms.filter((x) => x !== name) : [...ms, name];
                      onUpdate((w) => ({ ...w, query: { ...w.query, measures: next } }));
                    }}
                  />
                  {name}
                </label>
              );
            })}
            {(joinModel?.measures || []).map((m) => {
              const name = asJoinField(measureKey(m));
              const checked = widget.query?.measures?.includes(name) || false;
              return (
                <label key={name} className="flex items-center gap-2 text-[12px] text-ink">
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => {
                      const ms = widget.query?.measures || [];
                      const next = checked ? ms.filter((x) => x !== name) : [...ms, name];
                      onUpdate((w) => ({ ...w, query: { ...w.query, measures: next } }));
                    }}
                  />
                  {joinName ? `${measureKey(m)} · ${joinName}` : measureKey(m)}
                </label>
              );
            })}
          </div>
        </FieldLabel>
      )}
      <FieldLabel label={dimLabel}>
        <Select
          value={widget.query?.dimensions?.[0] || ""}
          onChange={(e) =>
            onUpdate((w) => ({
              ...w,
              query: {
                ...w.query,
                dimensions: e.target.value ? [e.target.value, ...(w.query?.dimensions || []).slice(1)] : (w.query?.dimensions || []).slice(1),
              },
            }))
          }
        >
          <option value="">Nenhuma</option>
          <DimensionOptions model={model} joinModel={joinModel} joinName={joinName} />
        </Select>
      </FieldLabel>
      {widget.type === "heatmap" && (
        <FieldLabel label="Dimensão Y">
          <Select
            value={widget.query?.dimensions?.[1] || ""}
            onChange={(e) => {
              const first = widget.query?.dimensions?.[0] || "";
              onUpdate((w) => ({
                ...w,
                query: { ...w.query, dimensions: [first, e.target.value].filter(Boolean) },
              }));
            }}
          >
            <option value="">Nenhuma</option>
            <DimensionOptions model={model} joinModel={joinModel} joinName={joinName} />
          </Select>
        </FieldLabel>
      )}
      {canBreak &&
        extraDims.map((dim, i) => {
          const idx = extraStart + i;
          return (
            <FieldLabel
              key={`break-${idx}`}
              label={i === 0 ? "Quebrar também por" : `Quebra ${i + 2}`}
              hint={i === 0 ? "No gráfico de barras ou linhas, esta dimensão vira séries." : undefined}
            >
              <div className="flex items-center gap-1.5">
                <Select
                  className="flex-1"
                  value={dim}
                  onChange={(e) =>
                    onUpdate((w) => {
                      const dims = [...(w.query?.dimensions || [])];
                      dims[idx] = e.target.value;
                      return { ...w, query: { ...w.query, dimensions: dims.filter(Boolean) } };
                    })
                  }
                >
                  <option value="">Nenhuma</option>
                  <DimensionOptions model={model} joinModel={joinModel} joinName={joinName} />
                </Select>
                <button
                  type="button"
                  className="rounded-lg p-1.5 text-mute hover:bg-surface-2 hover:text-danger"
                  title="Remover quebra"
                  onClick={() =>
                    onUpdate((w) => {
                      const dims = [...(w.query?.dimensions || [])];
                      dims.splice(idx, 1);
                      return { ...w, query: { ...w.query, dimensions: dims } };
                    })
                  }
                >
                  <Trash2 size={14} />
                </button>
              </div>
            </FieldLabel>
          );
        })}
      {canBreak && (widget.query?.dimensions?.length || 0) >= extraStart && (widget.query?.dimensions?.length || 0) < extraStart + 3 && (
        <button
          type="button"
          className="inline-flex items-center gap-1 text-[12px] font-medium text-accent"
          onClick={() => {
            const used = new Set(widget.query?.dimensions || []);
            const local = (model?.dimensions || []).map(dimensionKey).find((k) => k && !used.has(k));
            const related = (joinModel?.dimensions || [])
              .map((d) => asJoinField(dimensionKey(d)))
              .find((k) => k && !used.has(k));
            const next = local || related;
            if (!next) return;
            onUpdate((w) => ({ ...w, query: { ...w.query, dimensions: [...(w.query?.dimensions || []), next] } }));
          }}
        >
          <Plus size={14} /> Adicionar quebra
        </button>
      )}
      {!["kpi", "kpi_goal", "metric_group", "gauge", "sparkline", "slicer"].includes(widget.type) && (
        <FieldLabel
          label={widget.type === "decomposition_tree" ? "Hierarquia (níveis separados por vírgula)" : "Hierarquia de drill-down"}
          hint="Ex.: regiao, cidade, loja"
        >
          <Input
            value={(widget.hierarchy || []).join(",")}
            onChange={(e) =>
              onUpdate((w) => ({ ...w, hierarchy: e.target.value.split(",").map((s) => s.trim()).filter(Boolean) }))
            }
            placeholder="regiao, cidade, loja"
          />
        </FieldLabel>
      )}
      {widget.drillPath && widget.drillPath.length > 0 && (
        <div className="flex items-center gap-2 text-xs text-mute">
          <span>Drill:</span>
          {widget.hierarchy?.slice(0, widget.drillPath.length).map((h, i) => (
            <span key={h} className="rounded bg-surface-2 px-1.5 py-0.5">
              {h}={widget.drillPath?.[i]}
            </span>
          ))}
          <button className="text-accent" onClick={onDrillUp}>
            Subir
          </button>
        </div>
      )}
      <FieldLabel label="Limite de linhas (consulta)">
        <Input
          type="number"
          min={1}
          max={1000}
          value={widget.query?.limit ?? 20}
          onChange={(e) => onUpdate((w) => ({ ...w, query: { ...w.query, limit: Number(e.target.value) } }))}
        />
      </FieldLabel>
    </>
  );
}

function Section({ title, defaultOpen = false, children }: { title: string; defaultOpen?: boolean; children: ReactNode }) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div className="border-b border-line last:border-b-0">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center justify-between py-2.5 text-[11px] font-semibold uppercase tracking-wide text-mute"
      >
        {title}
        <ChevronDown size={14} className={cn("transition-transform", open && "rotate-180")} />
      </button>
      {open && <div className="space-y-3 pb-3">{children}</div>}
    </div>
  );
}

function ToggleRow({ label, checked, onChange, hint }: { label: string; checked: boolean; onChange: (v: boolean) => void; hint?: string }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <div className="min-w-0">
        <div className="text-[13px] font-medium text-ink">{label}</div>
        {hint && <div className="text-[11px] font-normal text-mute">{hint}</div>}
      </div>
      <Toggle checked={checked} onChange={onChange} label={label} />
    </div>
  );
}

function ColorSwatches({ value, onChange }: { value?: string; onChange: (c: string) => void }) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      {BRAND_PALETTE.map((c) => (
        <button
          key={c}
          type="button"
          title={c}
          className={cn("h-7 w-7 rounded-full border-2 transition", value === c ? "border-ink scale-110" : "border-transparent")}
          style={{ backgroundColor: c }}
          onClick={() => onChange(c)}
        />
      ))}
      <input
        type="color"
        value={value || "#2563EB"}
        onChange={(e) => onChange(e.target.value)}
        className="h-7 w-7 cursor-pointer rounded-full border border-line bg-white p-0"
        title="Cor personalizada"
      />
    </div>
  );
}

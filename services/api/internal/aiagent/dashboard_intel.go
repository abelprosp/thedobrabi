package aiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/queryeng"
)

const (
	maxIntelWidgets = 12
	maxIntelRows    = 36
	maxIntelPayload = 16
)

var skipIntelTypes = map[string]bool{
	"text":              true,
	"image":             true,
	"markdown":          true,
	"iframe":            true,
	"data_intelligence": true,
	"slicer":            true,
}

var kpiIntelTypes = map[string]bool{
	"kpi":          true,
	"kpi_goal":     true,
	"gauge":        true,
	"metric_group": true,
}

// AnalyzeDashboardRequest is the payload from the Inteligência dados widget.
type AnalyzeDashboardRequest struct {
	DashboardID   string                 `json:"dashboard_id,omitempty"`
	FocusPrompt   string                 `json:"focus_prompt,omitempty"`
	TimeRange     *queryeng.TimeRange    `json:"time_range,omitempty"`
	GlobalFilters []DashboardIntelFilter `json:"global_filters,omitempty"`
	Widgets       []DashboardWidgetSpec  `json:"widgets"`
}

// DashboardIntelFilter retains dataset scope from multi-source dashboards.
type DashboardIntelFilter struct {
	DatasetID string `json:"dataset_id,omitempty"`
	Dimension string `json:"dimension"`
	Op        string `json:"op"`
	Value     any    `json:"value"`
}

// DashboardWidgetSpec is a sibling visual to analyse.
type DashboardWidgetSpec struct {
	ID     string           `json:"id"`
	Type   string           `json:"type"`
	Title  string           `json:"title"`
	Query  queryeng.Request `json:"query"`
	Config map[string]any   `json:"config,omitempty"`
}

// DashboardIntelResult is insights + alert suggestions for the dashboard canvas.
type DashboardIntelResult struct {
	Headline           string             `json:"headline"`
	Insights           []DashboardInsight `json:"insights"`
	AlertSuggestions   []AlertSuggestion  `json:"alert_suggestions"`
	RecommendedActions []string           `json:"recommended_actions"`
	GeneratedAt        string             `json:"generated_at"`
	Source             string             `json:"source"`
	AnalyzedWidgets    int                `json:"analyzed_widgets"`
}

// DashboardInsight is one finding tied optionally to a widget.
type DashboardInsight struct {
	Kind     string         `json:"kind"`
	Title    string         `json:"title"`
	Body     string         `json:"body"`
	Severity string         `json:"severity"`
	Evidence map[string]any `json:"evidence,omitempty"`
	WidgetID string         `json:"widget_id,omitempty"`
}

// AlertSuggestion can be saved as a real alert from the dashboard card.
type AlertSuggestion struct {
	Name      string         `json:"name"`
	Rationale string         `json:"rationale"`
	WidgetID  string         `json:"widget_id,omitempty"`
	Severity  string         `json:"severity,omitempty"`
	Condition AlertCondition `json:"condition"`
}

// AlertCondition matches POST /api/v1/alerts.
type AlertCondition struct {
	DatasetID string  `json:"dataset_id"`
	Measure   string  `json:"measure"`
	Op        string  `json:"op"`
	Value     float64 `json:"value"`
}

type widgetSnapshot struct {
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	Type       string           `json:"type"`
	DatasetID  string           `json:"dataset_id,omitempty"`
	Measures   []string         `json:"measures,omitempty"`
	Dimensions []string         `json:"dimensions,omitempty"`
	Columns    []string         `json:"columns,omitempty"`
	Rows       []map[string]any `json:"rows,omitempty"`
	Stats      *seriesStats     `json:"stats,omitempty"`
	Overlay    *overlayInfo     `json:"overlay,omitempty"`
	Error      string           `json:"error,omitempty"`
}

type seriesStats struct {
	Measure     string  `json:"measure,omitempty"`
	First       float64 `json:"first"`
	Last        float64 `json:"last"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Sum         float64 `json:"sum"`
	ChangePct   float64 `json:"change_pct"`
	RowCount    int     `json:"row_count"`
	TopCategory string  `json:"top_category,omitempty"`
	TopShare    float64 `json:"top_share,omitempty"`
	LabelColumn string  `json:"label_column,omitempty"`
	TimeSeries  bool    `json:"time_series,omitempty"`
}

type overlayInfo struct {
	Kind  string   `json:"kind,omitempty"`
	Label string   `json:"label,omitempty"`
	Value *float64 `json:"value,omitempty"`
}

// AnalyzeDashboardWidgets re-runs sibling widget queries and returns insights/alerts.
func (a *Agent) AnalyzeDashboardWidgets(ctx context.Context, orgID, wsID, userID uuid.UUID, role string, req AnalyzeDashboardRequest) (DashboardIntelResult, error) {
	snaps := make([]widgetSnapshot, 0, maxIntelWidgets)
	for _, spec := range req.Widgets {
		if len(snaps) >= maxIntelWidgets {
			break
		}
		if skipIntelTypes[spec.Type] {
			continue
		}
		dsID := strings.TrimSpace(spec.Query.DatasetID)
		if dsID == "" {
			continue
		}
		q := spec.Query
		if req.TimeRange != nil && (req.TimeRange.Start != "" || req.TimeRange.End != "") {
			q.TimeRange = req.TimeRange
		}
		if len(req.GlobalFilters) > 0 {
			q.Filters = append(q.Filters, scopedIntelFilters(req.GlobalFilters, dsID)...)
		}
		if kpiIntelTypes[spec.Type] {
			q.Dimensions = nil
		}
		if q.Limit <= 0 || q.Limit > maxIntelRows {
			q.Limit = maxIntelRows
		}
		snap := widgetSnapshot{
			ID:         spec.ID,
			Title:      strings.TrimSpace(spec.Title),
			Type:       spec.Type,
			DatasetID:  dsID,
			Measures:   q.Measures,
			Dimensions: q.Dimensions,
		}
		if snap.Title == "" {
			snap.Title = spec.Type
		}
		res, err := a.query.Execute(ctx, orgID, wsID, userID, role, q)
		if err != nil {
			snap.Error = err.Error()
			snaps = append(snaps, snap)
			continue
		}
		snap.Columns = res.Columns
		snap.Rows = compactIntelRows(res.Rows, res.Columns, maxIntelPayload)
		snap.Stats = computeSeriesStats(res.Columns, res.Rows, q.Measures)
		snap.Overlay = overlayFromConfig(spec.Config, snap.Stats)
		snaps = append(snaps, snap)
	}

	out := analyzeDashboardFallback(snaps, req.FocusPrompt)
	if a.cfg.OpenAIKey != "" && len(snaps) > 0 {
		if llm, err := a.analyzeDashboardWithLLM(ctx, snaps, req.FocusPrompt); err == nil && strings.TrimSpace(llm.Headline) != "" {
			llm.AnalyzedWidgets = len(snaps)
			llm.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
			llm.Source = "openai"
			if len(llm.Insights) == 0 {
				llm.Insights = out.Insights
			}
			if len(llm.AlertSuggestions) == 0 {
				llm.AlertSuggestions = out.AlertSuggestions
			}
			if len(llm.RecommendedActions) == 0 {
				llm.RecommendedActions = out.RecommendedActions
			}
			return llm, nil
		}
	}
	return out, nil
}

func scopedIntelFilters(filters []DashboardIntelFilter, datasetID string) []queryeng.Filter {
	scoped := make([]queryeng.Filter, 0, len(filters))
	for _, filter := range filters {
		if filter.DatasetID != "" && filter.DatasetID != datasetID {
			continue
		}
		scoped = append(scoped, queryeng.Filter{
			Dimension: filter.Dimension,
			Op:        filter.Op,
			Value:     filter.Value,
		})
	}
	return scoped
}

func (a *Agent) analyzeDashboardWithLLM(ctx context.Context, snaps []widgetSnapshot, focus string) (DashboardIntelResult, error) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"headline": map[string]any{"type": "string"},
			"insights": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"kind":      map[string]any{"type": "string"},
						"title":     map[string]any{"type": "string"},
						"body":      map[string]any{"type": "string"},
						"severity":  map[string]any{"type": "string"},
						"widget_id": map[string]any{"type": "string"},
					},
					"required": []string{"kind", "title", "body", "severity"},
				},
			},
			"alert_suggestions": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":       map[string]any{"type": "string"},
						"rationale":  map[string]any{"type": "string"},
						"widget_id":  map[string]any{"type": "string"},
						"dataset_id": map[string]any{"type": "string"},
						"measure":    map[string]any{"type": "string"},
						"op":         map[string]any{"type": "string"},
						"value":      map[string]any{"type": "number"},
						"severity":   map[string]any{"type": "string"},
					},
					"required": []string{"name", "dataset_id", "measure", "op", "value"},
				},
			},
			"recommended_actions": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
		"required": []string{"headline", "insights"},
	}
	payload, _ := json.Marshal(snaps)
	sys := `És um especialista sénior em análise de dados (analytics), não um assistente genérico de chat. Analisas um dashboard de BI da TheDobra.

Método:
1. Usa só os números do resumo (stats + amostra de linhas). Nunca inventes valores, períodos ou categorias.
2. Distingue série temporal (mês, data, ano) de ranking (empresa, produto, canal). Em ranking, o primeiro e o último ponto NÃO são tendência — são categorias.
3. Se o visual tiver overlay (meta, média ou linha complementar), compara as barras com esse limiar: quantos períodos ficaram abaixo, desvio percentual, se a meta é realista.
4. Quantifica: variação %, concentração, pico, mínimo, anomalia. Explica o «então o quê» em linguagem de negócio.
5. Recomenda a próxima acção concreta (filtro, alerta, investigação de mix/preço/volume).

Regras de formato:
- Português do Brasil, concreto e executivo.
- Cada insight deve citar widget_id quando fizer sentido.
- kind: trend, risk, opportunity, anomaly, concentration ou alert.
- severity: low, medium, high ou critical.
- Máximo 8 insights e 4 alert_suggestions.
- alert_suggestions: limiares acionáveis (dataset_id, measure, op < ou >, value) com base nos valores observados. Preferir alerta de queda (op "<") quando a métrica desce.
- recommended_actions: 2 a 4 frases curtas.
- headline: uma frase a resumir o estado do dashboard.`
	user := fmt.Sprintf("Visuais do dashboard:\n%s", string(payload))
	if strings.TrimSpace(focus) != "" {
		user += "\nFoco pedido pelo utilizador: " + strings.TrimSpace(focus)
	}
	raw, err := a.callOpenAIJSON(ctx, sys, user, schema)
	if err != nil {
		return DashboardIntelResult{}, err
	}
	return parseDashboardIntelLLM(raw, snaps)
}

type llmDashboardIntel struct {
	Headline string `json:"headline"`
	Insights []struct {
		Kind     string `json:"kind"`
		Title    string `json:"title"`
		Body     string `json:"body"`
		Severity string `json:"severity"`
		WidgetID string `json:"widget_id"`
	} `json:"insights"`
	AlertSuggestions []struct {
		Name      string  `json:"name"`
		Rationale string  `json:"rationale"`
		WidgetID  string  `json:"widget_id"`
		DatasetID string  `json:"dataset_id"`
		Measure   string  `json:"measure"`
		Op        string  `json:"op"`
		Value     float64 `json:"value"`
		Severity  string  `json:"severity"`
	} `json:"alert_suggestions"`
	RecommendedActions []string `json:"recommended_actions"`
}

func parseDashboardIntelLLM(raw []byte, snaps []widgetSnapshot) (DashboardIntelResult, error) {
	var parsed llmDashboardIntel
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return DashboardIntelResult{}, fmt.Errorf("invalid llm json: %w", err)
	}
	byID := map[string]widgetSnapshot{}
	for _, s := range snaps {
		byID[s.ID] = s
	}
	out := DashboardIntelResult{
		Headline:           strings.TrimSpace(parsed.Headline),
		RecommendedActions: parsed.RecommendedActions,
		Insights:           make([]DashboardInsight, 0, len(parsed.Insights)),
		AlertSuggestions:   make([]AlertSuggestion, 0, len(parsed.AlertSuggestions)),
	}
	for i, in := range parsed.Insights {
		if i >= 8 {
			break
		}
		title := strings.TrimSpace(in.Title)
		body := strings.TrimSpace(in.Body)
		if title == "" || body == "" {
			continue
		}
		out.Insights = append(out.Insights, DashboardInsight{
			Kind:     normalizeIntelKind(in.Kind),
			Title:    title,
			Body:     body,
			Severity: normalizeIntelSeverity(in.Severity),
			WidgetID: in.WidgetID,
		})
	}
	for i, al := range parsed.AlertSuggestions {
		if i >= 4 {
			break
		}
		ds := strings.TrimSpace(al.DatasetID)
		measure := strings.TrimSpace(al.Measure)
		if snap, ok := byID[al.WidgetID]; ok {
			if ds == "" {
				ds = snap.DatasetID
			}
			if measure == "" && len(snap.Measures) > 0 {
				measure = snap.Measures[0]
			}
		}
		if ds == "" || measure == "" {
			continue
		}
		name := strings.TrimSpace(al.Name)
		if name == "" {
			name = "Alerta: " + measure
		}
		out.AlertSuggestions = append(out.AlertSuggestions, AlertSuggestion{
			Name:      name,
			Rationale: strings.TrimSpace(al.Rationale),
			WidgetID:  al.WidgetID,
			Severity:  normalizeIntelSeverity(al.Severity),
			Condition: AlertCondition{
				DatasetID: ds,
				Measure:   measure,
				Op:        normalizeAlertOp(al.Op),
				Value:     al.Value,
			},
		})
	}
	if out.Headline == "" {
		return DashboardIntelResult{}, fmt.Errorf("empty llm headline")
	}
	return out, nil
}

func analyzeDashboardFallback(snaps []widgetSnapshot, focus string) DashboardIntelResult {
	now := time.Now().UTC().Format(time.RFC3339)
	out := DashboardIntelResult{
		GeneratedAt:        now,
		Source:             "fallback",
		AnalyzedWidgets:    len(snaps),
		Insights:           []DashboardInsight{},
		AlertSuggestions:   []AlertSuggestion{},
		RecommendedActions: []string{},
	}
	if len(snaps) == 0 {
		out.Headline = "Ainda não há visuais com dados para analisar."
		out.Insights = append(out.Insights, DashboardInsight{
			Kind:     "info",
			Title:    "Adicione gráficos ou KPIs",
			Body:     "Este cartão lê os outros componentes do dashboard. Coloque um KPI, gráfico ou tabela ao lado para gerar insights e alertas.",
			Severity: "low",
		})
		return out
	}

	var worstDrop *widgetSnapshot
	var worstPct float64
	highCount := 0
	for i := range snaps {
		s := snaps[i]
		if s.Error != "" {
			out.Insights = append(out.Insights, DashboardInsight{
				Kind:     "risk",
				Title:    s.Title + " sem dados",
				Body:     "Não consegui consultar este visual: " + s.Error,
				Severity: "medium",
				WidgetID: s.ID,
			})
			continue
		}
		st := s.Stats
		if st == nil {
			continue
		}
		label := s.Title
		if st.RowCount <= 1 || !st.TimeSeries {
			if st.RowCount <= 1 {
				out.Insights = append(out.Insights, DashboardInsight{
					Kind:     "trend",
					Title:    label + " está em " + formatIntelNum(st.Last),
					Body:     fmt.Sprintf("Valor actual de %s: %s.", measureLabel(st, s), formatIntelNum(st.Last)),
					Severity: "low",
					WidgetID: s.ID,
					Evidence: map[string]any{"value": st.Last, "measure": st.Measure},
				})
			} else if st.TopCategory != "" {
				out.Insights = append(out.Insights, DashboardInsight{
					Kind:     "concentration",
					Title:    fmt.Sprintf("%s: %s lidera", label, st.TopCategory),
					Body:     fmt.Sprintf("«%s» é a maior fatia visível de %s (%.0f%% do total). Isto é um ranking, não uma tendência no tempo.", st.TopCategory, measureLabel(st, s), st.TopShare*100),
					Severity: "medium",
					WidgetID: s.ID,
					Evidence: map[string]any{"category": st.TopCategory, "share": st.TopShare},
				})
			}
			if s.Overlay != nil && s.Overlay.Value != nil && st.Last < *s.Overlay.Value {
				out.Insights = append(out.Insights, DashboardInsight{
					Kind:     "alert",
					Title:    label + " abaixo da linha complementar",
					Body:     fmt.Sprintf("O valor actual (%s) ficou abaixo de %s (%s).", formatIntelNum(st.Last), overlayLabel(s.Overlay), formatIntelNum(*s.Overlay.Value)),
					Severity: "high",
					WidgetID: s.ID,
				})
			}
			continue
		}
		sev := "low"
		kind := "trend"
		body := fmt.Sprintf("%s passou de %s para %s (%.1f%%).", measureLabel(st, s), formatIntelNum(st.First), formatIntelNum(st.Last), st.ChangePct)
		if st.ChangePct <= -10 {
			sev = "high"
			kind = "risk"
			highCount++
			body = fmt.Sprintf("%s caiu %.1f%% neste visual (de %s para %s).", measureLabel(st, s), math.Abs(st.ChangePct), formatIntelNum(st.First), formatIntelNum(st.Last))
			if worstDrop == nil || st.ChangePct < worstPct {
				cp := s
				worstDrop = &cp
				worstPct = st.ChangePct
			}
		} else if st.ChangePct >= 15 {
			sev = "medium"
			kind = "opportunity"
			body = fmt.Sprintf("%s subiu %.1f%% neste visual (de %s para %s).", measureLabel(st, s), st.ChangePct, formatIntelNum(st.First), formatIntelNum(st.Last))
		}
		out.Insights = append(out.Insights, DashboardInsight{
			Kind:     kind,
			Title:    label + variationTitle(st.ChangePct),
			Body:     body,
			Severity: sev,
			WidgetID: s.ID,
			Evidence: map[string]any{"change_pct": st.ChangePct, "first": st.First, "last": st.Last, "measure": st.Measure},
		})
		if s.Overlay != nil && s.Overlay.Value != nil {
			below := 0
			if st.Last < *s.Overlay.Value {
				below = 1
			}
			if st.Max < *s.Overlay.Value {
				below = st.RowCount
			}
			if st.Last < *s.Overlay.Value || st.Max < *s.Overlay.Value {
				out.Insights = append(out.Insights, DashboardInsight{
					Kind:     "alert",
					Title:    label + " vs " + overlayLabel(s.Overlay),
					Body:     fmt.Sprintf("A linha complementar está em %s. O último ponto da série é %s.", formatIntelNum(*s.Overlay.Value), formatIntelNum(st.Last)),
					Severity: "high",
					WidgetID: s.ID,
					Evidence: map[string]any{"overlay": *s.Overlay.Value, "last": st.Last, "below": below},
				})
			}
		}
		if st.ChangePct <= -10 && s.DatasetID != "" && st.Measure != "" {
			threshold := st.Last
			if threshold == 0 {
				threshold = st.First * 0.9
			}
			out.AlertSuggestions = append(out.AlertSuggestions, AlertSuggestion{
				Name:      "Queda em " + label,
				Rationale: fmt.Sprintf("Avisar se %s ficar abaixo de %s, o último valor observado.", st.Measure, formatIntelNum(threshold)),
				WidgetID:  s.ID,
				Severity:  "high",
				Condition: AlertCondition{DatasetID: s.DatasetID, Measure: st.Measure, Op: "<", Value: threshold},
			})
		}
	}

	if len(out.Insights) == 0 {
		out.Headline = fmt.Sprintf("Revisei %d visuais. Ainda não há variação clara nos números.", len(snaps))
		out.Insights = append(out.Insights, DashboardInsight{
			Kind:     "info",
			Title:    "Sem sinais fortes",
			Body:     "Os visuais devolveram dados, mas não encontrei quedas, picos ou concentrações evidentes. Volte a analisar depois de filtrar o período.",
			Severity: "low",
		})
	} else if worstDrop != nil {
		out.Headline = fmt.Sprintf("Atenção: %s caiu %.1f%%. Revisei %d visuais do dashboard.", worstDrop.Title, math.Abs(worstPct), len(snaps))
	} else {
		out.Headline = fmt.Sprintf("Revisei %d visuais do dashboard. Sem quedas graves no recorte actual.", len(snaps))
	}

	if highCount > 0 {
		out.RecommendedActions = append(out.RecommendedActions, "Abra os visuais com queda e confirme se o recorte de tempo e os filtros estão correctos.")
		out.RecommendedActions = append(out.RecommendedActions, "Crie um alerta no limiar sugerido para não perder a próxima variação.")
	} else {
		out.RecommendedActions = append(out.RecommendedActions, "Mantenha este cartão no dashboard e volte a analisar depois de aplicar filtros ou um novo período.")
	}
	if strings.TrimSpace(focus) != "" {
		out.RecommendedActions = append(out.RecommendedActions, "Foquei a leitura em: "+strings.TrimSpace(focus)+".")
	}
	if len(out.Insights) > 8 {
		out.Insights = out.Insights[:8]
	}
	if len(out.AlertSuggestions) > 4 {
		out.AlertSuggestions = out.AlertSuggestions[:4]
	}
	return out
}

func computeSeriesStats(columns []string, rows []map[string]any, measures []string) *seriesStats {
	if len(rows) == 0 {
		return nil
	}
	valueKey := firstNumericKey(columns, rows, measures)
	if valueKey == "" {
		return nil
	}
	labelKey := firstLabelKey(columns, rows, valueKey)
	st := &seriesStats{Measure: valueKey, Min: math.Inf(1), Max: math.Inf(-1), RowCount: 0}
	var topCat string
	var topVal float64
	for _, row := range rows {
		v, ok := asFloat(row[valueKey])
		if !ok {
			continue
		}
		st.RowCount++
		if st.RowCount == 1 {
			st.First = v
		}
		st.Last = v
		if v < st.Min {
			st.Min = v
		}
		if v > st.Max {
			st.Max = v
		}
		st.Sum += v
		if labelKey != "" {
			cat := stringifyIntel(row[labelKey])
			if cat != "" && v >= topVal {
				topVal = v
				topCat = cat
			}
		}
	}
	if st.RowCount == 0 {
		return nil
	}
	if st.First != 0 {
		st.ChangePct = (st.Last - st.First) / math.Abs(st.First) * 100
	}
	if topCat != "" && st.Sum != 0 {
		st.TopCategory = topCat
		st.TopShare = topVal / math.Abs(st.Sum)
		st.LabelColumn = labelKey
	}
	if len(measures) > 0 && strings.TrimSpace(measures[0]) != "" {
		st.Measure = measures[0]
	}
	st.TimeSeries = labelLooksTime(labelKey)
	return st
}

func labelLooksTime(key string) bool {
	n := strings.ToLower(strings.TrimSpace(key))
	n = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ã", "a", "ê", "e", "ç", "c", " ", "_", "-", "_").Replace(n)
	if n == "" {
		return false
	}
	parts := strings.FieldsFunc(n, func(r rune) bool { return r == '_' })
	for _, p := range parts {
		switch p {
		case "date", "data", "datetime", "mes", "month", "ano", "year", "dia", "day", "semana", "week", "periodo", "period":
			return true
		}
	}
	return false
}

func overlayFromConfig(cfg map[string]any, st *seriesStats) *overlayInfo {
	if len(cfg) == 0 {
		return nil
	}
	kind, _ := cfg["overlayLine"].(string)
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" || kind == "off" {
		return nil
	}
	label, _ := cfg["overlayLineLabel"].(string)
	info := &overlayInfo{Kind: kind, Label: strings.TrimSpace(label)}
	switch kind {
	case "value":
		if v, ok := asFloat(cfg["overlayLineValue"]); ok {
			info.Value = &v
		}
		if info.Label == "" {
			info.Label = "Meta"
		}
	case "average":
		if st != nil && st.RowCount > 0 {
			avg := st.Sum / float64(st.RowCount)
			info.Value = &avg
		}
		if info.Label == "" {
			info.Label = "Média"
		}
	default:
		if info.Label == "" {
			info.Label = "Linha complementar"
		}
	}
	return info
}

func overlayLabel(o *overlayInfo) string {
	if o == nil || strings.TrimSpace(o.Label) == "" {
		return "a linha complementar"
	}
	return o.Label
}

func firstNumericKey(columns []string, rows []map[string]any, measures []string) string {
	if len(rows) == 0 {
		return ""
	}
	for _, m := range measures {
		key := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(m), " ", "_"))
		for _, row := range rows {
			if _, ok := asFloat(row[m]); ok {
				return m
			}
			if _, ok := asFloat(row[key]); ok {
				return key
			}
		}
	}
	keys := columns
	if len(keys) == 0 {
		for k := range rows[0] {
			keys = append(keys, k)
		}
	}
	for _, k := range keys {
		hits := 0
		for _, row := range rows {
			if _, ok := asFloat(row[k]); ok {
				hits++
			}
		}
		if hits > 0 {
			return k
		}
	}
	return ""
}

func firstLabelKey(columns []string, rows []map[string]any, valueKey string) string {
	keys := columns
	if len(keys) == 0 && len(rows) > 0 {
		for k := range rows[0] {
			keys = append(keys, k)
		}
	}
	for _, k := range keys {
		if k == valueKey {
			continue
		}
		for _, row := range rows {
			if _, isNum := asFloat(row[k]); !isNum {
				if stringifyIntel(row[k]) != "" {
					return k
				}
			}
		}
	}
	return ""
}

func compactIntelRows(rows []map[string]any, columns []string, limit int) []map[string]any {
	if len(rows) > limit {
		rows = rows[:limit]
	}
	cols := columns
	if len(cols) > 6 {
		cols = cols[:6]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if len(cols) == 0 {
			out = append(out, row)
			continue
		}
		slim := map[string]any{}
		for _, c := range cols {
			if v, ok := row[c]; ok {
				slim[c] = v
			}
		}
		out = append(out, slim)
	}
	return out
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case nil:
		return 0, false
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return 0, false
		}
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case bool:
		if n {
			return 1, true
		}
		return 0, true
	case []byte:
		return asFloat(string(n))
	case string:
		s := strings.ReplaceAll(strings.TrimSpace(n), ",", ".")
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func stringifyIntel(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []byte:
		return strings.TrimSpace(string(t))
	default:
		if _, ok := asFloat(v); ok {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func formatIntelNum(v float64) string {
	if math.Abs(v) >= 1000 {
		return fmt.Sprintf("%.0f", v)
	}
	if math.Abs(v-math.Round(v)) < 0.005 {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func measureLabel(st *seriesStats, s widgetSnapshot) string {
	if st != nil && st.Measure != "" {
		return st.Measure
	}
	if len(s.Measures) > 0 {
		return s.Measures[0]
	}
	return "a métrica"
}

func variationTitle(pct float64) string {
	if pct <= -10 {
		return " em queda"
	}
	if pct >= 15 {
		return " em alta"
	}
	return ": variação estável"
}

func normalizeIntelKind(k string) string {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "trend", "risk", "opportunity", "anomaly", "concentration", "alert", "info":
		return strings.ToLower(strings.TrimSpace(k))
	default:
		return "trend"
	}
}

func normalizeIntelSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low", "medium", "high", "critical":
		return strings.ToLower(strings.TrimSpace(s))
	case "warn", "warning":
		return "medium"
	case "danger", "error":
		return "high"
	default:
		return "medium"
	}
}

func normalizeAlertOp(op string) string {
	switch strings.TrimSpace(op) {
	case ">", ">=", "<", "<=":
		return strings.TrimSpace(op)
	default:
		return "<"
	}
}

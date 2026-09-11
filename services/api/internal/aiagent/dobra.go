package aiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/semantic"
)

type DobraTurn struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type DobraRequest struct {
	ConversationID string           `json:"conversation_id"`
	Message        string           `json:"message"`
	DatasetID      string           `json:"dataset_id,omitempty"`
	DashboardID    string           `json:"dashboard_id,omitempty"`
	DashboardName  string           `json:"dashboard_name,omitempty"`
	History        []DobraTurn      `json:"history,omitempty"`
	Widgets        []map[string]any `json:"widgets,omitempty"`
}

type DobraPlanItem struct {
	Title     string `json:"title"`
	Chart     string `json:"chart"`
	Why       string `json:"why"`
	Measure   string `json:"measure,omitempty"`
	Dimension string `json:"dimension,omitempty"`
}

type DobraFilter struct {
	Dimension string `json:"dimension"`
	Op        string `json:"op,omitempty"`
	Value     any    `json:"value,omitempty"`
}

type DobraReply struct {
	ConversationID string            `json:"conversation_id"`
	Reply          string            `json:"reply"`
	Plan           []DobraPlanItem   `json:"plan,omitempty"`
	Apply          bool              `json:"apply"`
	Replace        bool              `json:"replace"`
	Widgets        []map[string]any  `json:"widgets,omitempty"`
	Filters        []DobraFilter     `json:"filters,omitempty"`
	TimeRange      map[string]string `json:"time_range,omitempty"`
	DashboardName  string            `json:"dashboard_name,omitempty"`
	Source         string            `json:"source"`
	DatasetID      string            `json:"dataset_id"`
	DatasetName    string            `json:"dataset_name"`
	Validated      bool              `json:"validated"`
	Confidence     string            `json:"confidence"`
	Warnings       []string          `json:"warnings,omitempty"`
}

func (a *Agent) DobraCompose(ctx context.Context, orgID, wsID, userID uuid.UUID, req DobraRequest) (DobraReply, error) {
	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		return DobraReply{}, fmt.Errorf("escreva o que quer no dashboard")
	}
	convID, err := a.ensureConv(ctx, orgID, wsID, userID, AskRequest{ConversationID: req.ConversationID, Message: msg})
	if err != nil {
		return DobraReply{}, err
	}
	_, _ = a.pg.Exec(ctx, `INSERT INTO ai_messages (conversation_id, role, content) VALUES ($1,'user',$2)`,
		convID, mustJSON(map[string]any{"text": msg, "intent": "dobra_ai", "dashboard_id": req.DashboardID}))

	dsID := req.DatasetID
	if dsID == "" {
		dsID, err = a.defaultDataset(ctx, orgID, wsID)
		if err != nil {
			out := DobraReply{
				ConversationID: convID.String(),
				Reply:          "Ainda não há conjuntos neste espaço. Ligue um conector ou carregue um ficheiro e volte a pedir-me o dashboard.",
				Source:         "empty",
			}
			a.storeAssistant(ctx, convID, out)
			return out, nil
		}
	}
	model, dsName, err := a.loadModel(ctx, orgID, wsID, dsID)
	if err != nil {
		return DobraReply{}, err
	}

	var out DobraReply
	if a.cfg.OpenAIKey != "" {
		out, err = a.dobraWithLLM(ctx, req, dsID, dsName, model)
		if err != nil {
			out = a.dobraFallback(req, dsID, dsName, model)
		}
	} else {
		out = a.dobraFallback(req, dsID, dsName, model)
	}
	out.ConversationID = convID.String()
	out.DatasetID = dsID
	out.DatasetName = dsName
	out.Widgets, out.Warnings = a.validateDobraWidgets(out.Widgets, dsID, model, out.Warnings)
	var filterWarnings []string
	out.Filters, filterWarnings = validateDobraFilters(out.Filters, model)
	out.Warnings = append(out.Warnings, filterWarnings...)
	var timeWarnings []string
	out.TimeRange, timeWarnings = validateDobraTimeRange(out.TimeRange, model)
	out.Warnings = append(out.Warnings, timeWarnings...)
	if out.Apply && len(out.Widgets) == 0 && !proposeOnly(msg) {
		fb := a.dobraFallback(req, dsID, dsName, model)
		out.Widgets = fb.Widgets
		out.Plan = fb.Plan
		out.Replace = fb.Replace
		out.Warnings = append(out.Warnings, "O plano original não passou na validação; foi aplicado um plano seguro com os campos disponíveis.")
		out.Widgets, out.Warnings = a.validateDobraWidgets(out.Widgets, dsID, model, out.Warnings)
		if out.Reply == "" {
			out.Reply = fb.Reply
		}
	}
	out.Widgets = applyDobraScope(out.Widgets, out.Filters, out.TimeRange)
	out.Validated = true
	if out.Source == "openai" && len(out.Warnings) == 0 {
		out.Confidence = "high"
	} else {
		out.Confidence = "medium"
	}
	a.storeAssistant(ctx, convID, out)
	return out, nil
}

func (a *Agent) validateDobraWidgets(widgets []map[string]any, dsID string, model semantic.Model, warnings []string) ([]map[string]any, []string) {
	fixed := make([]map[string]any, 0, len(widgets))
	for _, widget := range widgets {
		validated, widgetWarnings := a.validateAndFixWidgetDetailed(widget, dsID, model)
		warnings = append(warnings, widgetWarnings...)
		if validated != nil {
			fixed = append(fixed, validated)
		}
	}
	return fixed, warnings
}

func applyDobraScope(widgets []map[string]any, filters []DobraFilter, timeRange map[string]string) []map[string]any {
	for _, widget := range widgets {
		query, ok := widget["query"].(map[string]any)
		if !ok {
			continue
		}
		if len(filters) > 0 {
			query["filters"] = filters
		}
		if len(timeRange) > 0 {
			query["time_range"] = timeRange
		}
	}
	return widgets
}

func validateDobraFilters(filters []DobraFilter, model semantic.Model) ([]DobraFilter, []string) {
	valid := make([]DobraFilter, 0, len(filters))
	warnings := []string{}
	for _, filter := range filters {
		dimension, ok := semantic.ResolveDimension(model, filter.Dimension)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("O filtro %q foi ignorado porque a dimensão não existe no modelo.", filter.Dimension))
			continue
		}
		filter.Dimension = dimension.Column
		if filter.Op != "in" {
			filter.Op = "eq"
		}
		valid = append(valid, filter)
	}
	return valid, warnings
}

func validateDobraTimeRange(value map[string]string, model semantic.Model) (map[string]string, []string) {
	if len(value) == 0 {
		return nil, nil
	}
	if strings.TrimSpace(model.TimeColumn) == "" {
		return nil, []string{"O período foi ignorado porque o conjunto não possui uma dimensão de tempo."}
	}
	start, startErr := time.Parse("2006-01-02", value["start"])
	end, endErr := time.Parse("2006-01-02", value["end"])
	if startErr != nil || endErr != nil || !end.After(start) {
		return nil, []string{"O período sugerido foi ignorado porque as datas eram inválidas."}
	}
	return map[string]string{"start": start.Format("2006-01-02"), "end": end.Format("2006-01-02")}, nil
}

func proposeOnly(msg string) bool {
	q := strings.ToLower(msg)
	return strings.Contains(q, "só sugere") || strings.Contains(q, "so sugere") ||
		strings.Contains(q, "não apliques") || strings.Contains(q, "nao apliques") ||
		strings.Contains(q, "não montes") || strings.Contains(q, "nao montes") ||
		(strings.Contains(q, "plano") && !strings.Contains(q, "monta") && !strings.Contains(q, "aplica") && !strings.Contains(q, "cria"))
}

func (a *Agent) dobraWithLLM(ctx context.Context, req DobraRequest, dsID, dsName string, model semantic.Model) (DobraReply, error) {
	schemaJSON := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"reply":          map[string]any{"type": "string"},
			"dashboard_name": map[string]any{"type": "string"},
			"apply":          map[string]any{"type": "boolean"},
			"replace":        map[string]any{"type": "boolean"},
			"time_range": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"start": map[string]any{"type": "string"},
					"end":   map[string]any{"type": "string"},
				},
			},
			"filters": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"dimension": map[string]any{"type": "string"},
						"op":        map[string]any{"type": "string", "enum": []string{"eq", "in"}},
						"value":     map[string]any{},
					},
					"required": []string{"dimension"},
				},
			},
			"plan": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title":     map[string]any{"type": "string"},
						"chart":     map[string]any{"type": "string"},
						"why":       map[string]any{"type": "string"},
						"measure":   map[string]any{"type": "string"},
						"dimension": map[string]any{"type": "string"},
					},
					"required": []string{"title", "chart", "why"},
				},
			},
			"widgets": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type":  map[string]any{"type": "string", "enum": []string{"kpi", "kpi_goal", "line", "bar", "area", "pie", "table", "ranking", "slicer", "data_intelligence", "text", "gauge", "sparkline", "heatmap", "treemap", "funnel"}},
						"title": map[string]any{"type": "string"},
						"text":  map[string]any{"type": "string"},
						"layout": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"x": map[string]any{"type": "integer"},
								"y": map[string]any{"type": "integer"},
								"w": map[string]any{"type": "integer"},
								"h": map[string]any{"type": "integer"},
							},
							"required": []string{"x", "y", "w", "h"},
						},
						"query": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"dataset_id": map[string]any{"type": "string"},
								"measures":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
								"dimensions": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
								"limit":      map[string]any{"type": "integer"},
							},
						},
					},
					"required": []string{"type", "title", "layout"},
				},
			},
		},
		"required": []string{"reply", "apply", "plan"},
	}

	modelJSON, _ := json.Marshal(summarizeModelForMeasure(model))
	current, _ := json.Marshal(summarizeWidgets(req.Widgets))
	hist := ""
	for _, t := range req.History {
		if len(hist) > 2500 {
			break
		}
		hist += fmt.Sprintf("%s: %s\n", t.Role, t.Text)
	}
	sys := `És a DobraAI, designer sénior de dashboards da TheDobra.
Ajuda a montar o dashboard: propõe o plano (KPIs, gráficos, filtros, análises) e DEVOLVE os widgets prontos a aplicar no canvas.
Regras:
- Usa APENAS medidas e dimensões do modelo semântico. Nunca inventes colunas.
- Copia os nomes exatamente como aparecem no modelo. Se o pedido depender de um campo ausente, explica a limitação e não cries esse visual.
- Por omissão apply=true e monta o dashboard. Só apply=false se o utilizador pedir explicitamente um plano sem aplicar.
- replace=true quando o canvas está vazio, quando pedem para refazer/substituir, ou quando o plano é um dashboard completo. replace=false para "adiciona X".
- Escolhe o gráfico certo: kpi para totais, line para tempo, ranking/bar para categorias, pie só com poucas fatias, slicer para filtros, data_intelligence para análise automática, table para detalhe.
- Para pedidos de alteração, preserva os visuais atuais que não foram mencionados e devolve apenas os novos visuais com replace=false.
- Não cries KPI sem medida; não cries line/area sem dimensão temporal; não cries ranking, pie, slicer ou barras sem dimensão categórica.
- Usa no máximo 8 visuais num dashboard completo, sem repetir a mesma combinação de medida e dimensão.
- Grelha 12 colunas. Sem sobreposições. KPIs na primeira fila.
- Inclui dataset_id em cada query.
- Responde em português do Brasil. No reply, explica o plano em 3–6 frases e diz o que foi montado.`
	user := fmt.Sprintf("Conjunto: %s (id=%s)\nModelo: %s\nDashboard actual: %s\nWidgets actuais: %s\nHistórico:\n%s\nPedido: %s",
		dsName, dsID, modelJSON, req.DashboardName, current, hist, req.Message)
	raw, err := a.callOpenAIJSON(ctx, sys, user, schemaJSON)
	if err != nil {
		return DobraReply{}, err
	}
	var parsed DobraReply
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return DobraReply{}, err
	}
	parsed.Source = "openai"
	if parsed.Reply == "" {
		parsed.Reply = "Montei o dashboard com o plano abaixo."
	}
	return parsed, nil
}

func (a *Agent) dobraFallback(req DobraRequest, dsID, dsName string, model semantic.Model) DobraReply {
	q := strings.ToLower(req.Message)
	only := proposeOnly(req.Message)
	appendOnly := strings.Contains(q, "adiciona") || strings.Contains(q, "acrescenta") || strings.Contains(q, "inclui")
	replace := !appendOnly && (len(req.Widgets) == 0 || strings.Contains(q, "refaz") || strings.Contains(q, "substitui") || strings.Contains(q, "monta") || strings.Contains(q, "completa") || strings.Contains(q, "cria"))
	if len(req.Widgets) == 0 {
		replace = true
	}

	meas := semantic.PrimaryMeasure(model)
	if requested := pickMeasure(model, q); requested != "" {
		meas = requested
	}
	if meas == "" && len(model.Measures) > 0 {
		meas = model.Measures[0].Name
	}
	extra := []string{}
	for _, m := range model.Measures {
		if m.Name == meas {
			continue
		}
		if isRowCountMeasure(m) {
			continue
		}
		extra = append(extra, m.Name)
		if len(extra) >= 2 {
			break
		}
	}
	timeDim := pickModelTime(model)
	cats := pickModelCategories(model, timeDim, 3)
	if requested := pickDimension(model, q); requested != "" && !strings.EqualFold(requested, timeDim) {
		reordered := []string{requested}
		for _, category := range cats {
			if !strings.EqualFold(category, requested) {
				reordered = append(reordered, category)
			}
		}
		cats = reordered
		if len(cats) > 3 {
			cats = cats[:3]
		}
	}

	want := detectWantedCharts(q)
	widgets := []map[string]any{}
	plan := []DobraPlanItem{}
	y := 0

	add := func(w map[string]any, item DobraPlanItem) {
		widgets = append(widgets, w)
		plan = append(plan, item)
	}

	if want["kpi"] && meas != "" {
		kpis := append([]string{meas}, extra...)
		if len(kpis) > 3 {
			kpis = kpis[:3]
		}
		x := 0
		w := 12 / max(1, len(kpis))
		for i, m := range kpis {
			add(map[string]any{
				"id":     uuid.New().String(),
				"type":   "kpi",
				"title":  m,
				"layout": map[string]int{"x": x, "y": y, "w": w, "h": 2},
				"query":  map[string]any{"dataset_id": dsID, "measures": []string{m}, "limit": 1},
			}, DobraPlanItem{Title: m, Chart: "kpi", Why: "Número de topo para ler em 2 segundos.", Measure: m})
			x += w
			_ = i
		}
		y += 2
	}

	if want["line"] && meas != "" && timeDim != "" {
		add(map[string]any{
			"id":     uuid.New().String(),
			"type":   "line",
			"title":  meas + " ao longo do tempo",
			"layout": map[string]int{"x": 0, "y": y, "w": 8, "h": 4},
			"query":  map[string]any{"dataset_id": dsID, "measures": []string{meas}, "dimensions": []string{timeDim}, "limit": 90},
		}, DobraPlanItem{Title: meas + " no tempo", Chart: "line", Why: "Mostra tendência e sazonalidade.", Measure: meas, Dimension: timeDim})
	}

	if want["ranking"] && meas != "" && len(cats) > 0 {
		x, ww := 8, 4
		if !want["line"] || timeDim == "" {
			x, ww = 0, 6
		}
		add(map[string]any{
			"id":     uuid.New().String(),
			"type":   "ranking",
			"title":  "Top " + humanDim(cats[0]),
			"layout": map[string]int{"x": x, "y": y, "w": ww, "h": 4},
			"query":  map[string]any{"dataset_id": dsID, "measures": []string{meas}, "dimensions": []string{cats[0]}, "limit": 10, "order_by": []map[string]string{{"field": meas, "dir": "desc"}}},
		}, DobraPlanItem{Title: "Ranking " + cats[0], Chart: "ranking", Why: "Concentração: quem puxa o resultado.", Measure: meas, Dimension: cats[0]})
		if want["line"] && timeDim != "" {
			y += 4
		} else {
			y += 4
		}
	} else if want["bar"] && meas != "" && len(cats) > 0 {
		add(map[string]any{
			"id":     uuid.New().String(),
			"type":   "bar",
			"title":  meas + " por " + humanDim(cats[0]),
			"layout": map[string]int{"x": 0, "y": y, "w": 6, "h": 4},
			"query":  map[string]any{"dataset_id": dsID, "measures": []string{meas}, "dimensions": []string{cats[0]}, "limit": 12},
		}, DobraPlanItem{Title: meas + " por " + cats[0], Chart: "bar", Why: "Compara categorias lado a lado.", Measure: meas, Dimension: cats[0]})
		y += 4
	}

	if want["pie"] && meas != "" && len(cats) > 1 {
		add(map[string]any{
			"id":     uuid.New().String(),
			"type":   "pie",
			"title":  "Composição por " + humanDim(cats[1]),
			"layout": map[string]int{"x": 0, "y": y, "w": 4, "h": 4},
			"query":  map[string]any{"dataset_id": dsID, "measures": []string{meas}, "dimensions": []string{cats[1]}, "limit": 8},
		}, DobraPlanItem{Title: "Composição", Chart: "pie", Why: "Parte do todo — só com poucas fatias.", Measure: meas, Dimension: cats[1]})
	}

	if want["slicer"] && len(cats) > 0 {
		add(map[string]any{
			"id":     uuid.New().String(),
			"type":   "slicer",
			"title":  humanDim(cats[0]),
			"layout": map[string]int{"x": 8, "y": y, "w": 4, "h": 4},
			"query":  map[string]any{"dataset_id": dsID, "dimensions": []string{cats[0]}, "limit": 50},
		}, DobraPlanItem{Title: "Filtro " + cats[0], Chart: "slicer", Why: "Filtra o dashboard sem sair do ecrã.", Dimension: cats[0]})
	}

	if want["table"] && meas != "" {
		dims := cats
		if timeDim != "" {
			dims = append([]string{timeDim}, cats...)
		}
		if len(dims) > 2 {
			dims = dims[:2]
		}
		ms := []string{meas}
		if len(extra) > 0 {
			ms = append(ms, extra[0])
		}
		add(map[string]any{
			"id":     uuid.New().String(),
			"type":   "table",
			"title":  "Detalhe",
			"layout": map[string]int{"x": 0, "y": y + 4, "w": 12, "h": 4},
			"query":  map[string]any{"dataset_id": dsID, "measures": ms, "dimensions": dims, "limit": 200},
		}, DobraPlanItem{Title: "Tabela de detalhe", Chart: "table", Why: "Permite inspeccionar as linhas por detrás dos gráficos.", Measure: meas})
	}

	if want["intel"] {
		add(map[string]any{
			"id":     uuid.New().String(),
			"type":   "data_intelligence",
			"title":  "Análise DobraAI",
			"layout": map[string]int{"x": 0, "y": y + 8, "w": 12, "h": 5},
		}, DobraPlanItem{Title: "Análise automática", Chart: "data_intelligence", Why: "Lê os visuais e aponta quedas, concentração e próximos passos."})
	}

	name := req.DashboardName
	if name == "" || strings.EqualFold(name, "novo dashboard") {
		name = "Painel · " + dsName
	}
	reply := fmt.Sprintf("Plano para %s: KPIs de topo, tendência, ranking/categorias, filtro e uma análise automática. Usei o conjunto «%s».", name, dsName)
	if only {
		reply = "Aqui vai o plano. Diga «monta» para eu aplicar no dashboard."
	}
	if appendOnly {
		reply = "Vou acrescentar os visuais pedidos sem apagar o que já está no canvas."
		replace = false
	}

	return DobraReply{
		Reply:         reply,
		Plan:          plan,
		Apply:         !only,
		Replace:       replace && !only,
		Widgets:       widgets,
		DashboardName: name,
		Source:        "fallback",
		TimeRange:     defaultTimeRange(q),
	}
}

func detectWantedCharts(q string) map[string]bool {
	w := map[string]bool{"kpi": true, "line": true, "ranking": true, "slicer": true, "table": true, "intel": true, "pie": false, "bar": false}
	full := strings.Contains(q, "dashboard") || strings.Contains(q, "painel") ||
		strings.Contains(q, "monta") || strings.Contains(q, "completa") ||
		strings.Contains(q, "refaz") || strings.Contains(q, "substitui") ||
		strings.Contains(q, "cria")
	if full {
		if strings.Contains(q, "pizza") || strings.Contains(q, "pie") || strings.Contains(q, "composição") || strings.Contains(q, "composicao") {
			w["pie"] = true
		}
		if strings.Contains(q, "barra") {
			w["bar"] = true
		}
		return w
	}
	specific := false
	set := func(k string) {
		if !specific {
			for x := range w {
				w[x] = false
			}
			specific = true
		}
		w[k] = true
	}
	if strings.Contains(q, "kpi") || strings.Contains(q, "indicador") {
		set("kpi")
	}
	if strings.Contains(q, "linha") || strings.Contains(q, "tendência") || strings.Contains(q, "tendencia") || strings.Contains(q, "evolução") || strings.Contains(q, "evolucao") || strings.Contains(q, "tempo") {
		set("line")
	}
	if strings.Contains(q, "ranking") || strings.Contains(q, "top") || strings.Contains(q, "maiores") {
		set("ranking")
	}
	if strings.Contains(q, "barra") || strings.Contains(q, "bar ") {
		set("bar")
	}
	if strings.Contains(q, "pizza") || strings.Contains(q, "pie") || strings.Contains(q, "composição") || strings.Contains(q, "composicao") {
		set("pie")
	}
	if strings.Contains(q, "filtro") || strings.Contains(q, "slicer") {
		set("slicer")
	}
	if strings.Contains(q, "tabela") || strings.Contains(q, "detalhe") {
		set("table")
	}
	if strings.Contains(q, "análise") || strings.Contains(q, "analise") || strings.Contains(q, "inteligência") || strings.Contains(q, "inteligencia") {
		set("intel")
	}
	return w
}

func defaultTimeRange(q string) map[string]string {
	now := time.Now().UTC()
	end := now.AddDate(0, 0, 1).Format("2006-01-02")
	start := now.AddDate(0, 0, -90).Format("2006-01-02")
	if strings.Contains(q, "este mês") || strings.Contains(q, "este mes") {
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	}
	if strings.Contains(q, "este ano") {
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	}
	if strings.Contains(q, "30 dias") || strings.Contains(q, "último mês") || strings.Contains(q, "ultimo mes") {
		start = now.AddDate(0, 0, -30).Format("2006-01-02")
	}
	return map[string]string{"start": start, "end": end}
}

func pickModelTime(model semantic.Model) string {
	if model.TimeColumn != "" {
		return model.TimeColumn
	}
	for _, d := range model.Dimensions {
		n := strings.ToLower(d.Column + " " + d.Name)
		if strings.Contains(n, "mes") || strings.Contains(n, "mês") || strings.Contains(n, "month") ||
			strings.Contains(n, "data") || strings.Contains(n, "date") || strings.Contains(n, "ano") || strings.Contains(n, "year") {
			if d.Column != "" {
				return d.Column
			}
			return d.Name
		}
	}
	return ""
}

func pickModelCategories(model semantic.Model, timeDim string, maxN int) []string {
	out := []string{}
	for _, d := range model.Dimensions {
		col := d.Column
		if col == "" {
			col = d.Name
		}
		if col == "" || strings.EqualFold(col, timeDim) || strings.EqualFold(d.Name, timeDim) {
			continue
		}
		n := strings.ToLower(col + " " + d.Name)
		if strings.Contains(n, "mes") || strings.Contains(n, "mês") || strings.Contains(n, "data") || strings.Contains(n, "date") || strings.Contains(n, "ano") {
			continue
		}
		out = append(out, col)
		if len(out) >= maxN {
			break
		}
	}
	return out
}

func measureNames(model semantic.Model) []string {
	out := make([]string, 0, len(model.Measures))
	for _, m := range model.Measures {
		out = append(out, m.Name)
	}
	return out
}

func dimensionNames(model semantic.Model) []string {
	out := make([]string, 0, len(model.Dimensions))
	for _, d := range model.Dimensions {
		if d.Column != "" {
			out = append(out, d.Column)
		} else {
			out = append(out, d.Name)
		}
	}
	return out
}

func summarizeWidgets(widgets []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(widgets))
	for _, w := range widgets {
		item := map[string]any{
			"type":   w["type"],
			"title":  w["title"],
			"layout": w["layout"],
			"config": w["config"],
		}
		if q, ok := w["query"].(map[string]any); ok {
			item["dataset_id"] = q["dataset_id"]
			item["measures"] = q["measures"]
			item["dimensions"] = q["dimensions"]
			item["filters"] = q["filters"]
			item["time_range"] = q["time_range"]
		}
		out = append(out, item)
		if len(out) >= 24 {
			break
		}
	}
	return out
}

func humanDim(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

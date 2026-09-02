package aiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/thedobra/thedobra/services/api/internal/config"
	"github.com/thedobra/thedobra/services/api/internal/intelligence"
	"github.com/thedobra/thedobra/services/api/internal/mljobs"
	"github.com/thedobra/thedobra/services/api/internal/queryeng"
	"github.com/thedobra/thedobra/services/api/internal/schemax"
	"github.com/thedobra/thedobra/services/api/internal/semantic"
)

type Agent struct {
	pg    *pgxpool.Pool
	query *queryeng.Engine
	intel *intelligence.Engine
	cfg   config.Config
	rdb   *redis.Client
}

func New(pg *pgxpool.Pool, q *queryeng.Engine, intel *intelligence.Engine, cfg config.Config, rdb *redis.Client) *Agent {
	return &Agent{pg: pg, query: q, intel: intel, cfg: cfg, rdb: rdb}
}

type AskRequest struct {
	ConversationID string `json:"conversation_id"`
	Message        string `json:"message"`
	DatasetID      string `json:"dataset_id"`
}

type Answer struct {
	ConversationID string         `json:"conversation_id"`
	Answer         string         `json:"answer"`
	KeyMetric      *Metric        `json:"key_metric,omitempty"`
	Chart          *Chart         `json:"chart,omitempty"`
	Explanation    string         `json:"explanation,omitempty"`
	Drivers        []string       `json:"drivers,omitempty"`
	Recommendation string         `json:"recommendation,omitempty"`
	Evidence       map[string]any `json:"evidence"`
	Insufficient   bool           `json:"insufficient_data"`
}

type Metric struct {
	Label string   `json:"label"`
	Value float64  `json:"value"`
	Delta *float64 `json:"delta_pct,omitempty"`
}

type Chart struct {
	Type    string           `json:"type"`
	Title   string           `json:"title"`
	Columns []string         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
}

func (a *Agent) Ask(ctx context.Context, orgID, wsID, userID uuid.UUID, role string, req AskRequest) (Answer, error) {
	convID, err := a.ensureConv(ctx, orgID, wsID, userID, req)
	if err != nil {
		return Answer{}, err
	}
	_, _ = a.pg.Exec(ctx, `INSERT INTO ai_messages (conversation_id, role, content) VALUES ($1,'user',$2)`,
		convID, mustJSON(map[string]any{"text": req.Message}))

	dsID := req.DatasetID
	if dsID == "" {
		dsID, err = a.defaultDataset(ctx, orgID, wsID)
		if err != nil {
			ans := Answer{ConversationID: convID.String(), Insufficient: true,
				Answer:   "Não tenho dados suficientes para responder com fiabilidade. Ligue um conjunto primeiro.",
				Evidence: map[string]any{}}
			a.storeAssistant(ctx, convID, ans)
			return ans, nil
		}
	}

	model, dsName, err := a.loadModel(ctx, orgID, wsID, dsID)
	if err != nil {
		return Answer{}, err
	}

	var ans Answer
	if a.cfg.OpenAIKey != "" {
		ans, err = a.askLLM(ctx, orgID, wsID, userID, role, dsID, dsName, model, req.Message)
		if err != nil {
			ans, err = a.askDeterministic(ctx, orgID, wsID, userID, role, dsID, dsName, model, req.Message)
		}
	} else {
		ans, err = a.askDeterministic(ctx, orgID, wsID, userID, role, dsID, dsName, model, req.Message)
	}
	if err != nil {
		return Answer{}, err
	}
	ans.ConversationID = convID.String()
	a.storeAssistant(ctx, convID, ans)
	return ans, nil
}

func (a *Agent) askDeterministic(ctx context.Context, orgID, wsID, userID uuid.UUID, role string, dsID, dsName string, model semantic.Model, msg string) (Answer, error) {
	q := strings.ToLower(msg)
	measure := pickMeasure(model, q)
	if measure == "" {
		return Answer{
			Answer:       "Não encontrei uma métrica oficial que corresponda a esta pergunta. Não vou inventar uma fórmula.",
			Insufficient: true,
			Evidence:     map[string]any{"dataset": dsName},
		}, nil
	}
	meas, _ := semantic.ResolveMeasure(model, measure)

	now := time.Now().UTC()
	rangeCur := &queryeng.TimeRange{Start: now.AddDate(0, 0, -30).Format("2006-01-02"), End: now.AddDate(0, 0, 1).Format("2006-01-02")}
	if strings.Contains(q, "agosto") || strings.Contains(q, "august") {
		rangeCur = &queryeng.TimeRange{Start: fmt.Sprintf("%d-08-01", now.Year()), End: fmt.Sprintf("%d-09-01", now.Year())}
	}
	if (strings.Contains(q, "setembro") || strings.Contains(q, "september")) && !strings.Contains(q, "agosto") && !strings.Contains(q, "august") {
		rangeCur = &queryeng.TimeRange{Start: fmt.Sprintf("%d-09-01", now.Year()), End: fmt.Sprintf("%d-10-01", now.Year())}
	}
	if (strings.Contains(q, "este mês") || strings.Contains(q, "este mes") || strings.Contains(q, "this month")) && now.Day() > 5 {
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		rangeCur = &queryeng.TimeRange{Start: start.Format("2006-01-02"), End: now.AddDate(0, 0, 1).Format("2006-01-02")}
	}

	dim := pickDimension(model, q)
	limit := 10
	if strings.Contains(q, "top") {
		limit = 10
	}

	req := queryeng.Request{DatasetID: dsID, Measures: []string{measure}, Limit: limit, TimeRange: rangeCur}
	if dim != "" && (strings.Contains(q, "top") || strings.Contains(q, "maiores") || strings.Contains(q, "por ") || strings.Contains(q, "by ") || strings.Contains(q, "quais") || strings.Contains(q, "which") || strings.Contains(q, "região") || strings.Contains(q, "regiao") || strings.Contains(q, "region") || strings.Contains(q, "produto") || strings.Contains(q, "product") || strings.Contains(q, "cliente") || strings.Contains(q, "customer")) {
		req.Dimensions = []string{dim}
	}

	cmp := strings.Contains(q, "why") || strings.Contains(q, "porque") || strings.Contains(q, "por que") || strings.Contains(q, "caiu") || strings.Contains(q, "queda") || strings.Contains(q, "fall") || strings.Contains(q, "fell") || strings.Contains(q, "drop") || strings.Contains(q, "decreas") || strings.Contains(q, "compar")

	res, err := a.query.Execute(ctx, orgID, wsID, userID, role, req)
	if err != nil {
		return Answer{Answer: "Não tenho dados suficientes para responder com fiabilidade.", Insufficient: true, Evidence: map[string]any{"error": err.Error()}}, nil
	}
	if len(res.Rows) == 0 {
		return Answer{Answer: "Não tenho dados suficientes para responder com fiabilidade.", Insufficient: true, Evidence: map[string]any{"sql": res.SQL}}, nil
	}

	total := 0.0
	for _, r := range res.Rows {
		total += toF(firstNum(r, alias(measure)))
	}
	if dim == "" {
		total = toF(firstNum(res.Rows[0], alias(measure)))
	}

	ans := Answer{
		KeyMetric: &Metric{Label: meas.Name, Value: total},
		Evidence: map[string]any{
			"source":      dsName,
			"metric":      meas.Name,
			"calculation": meas.Expression,
			"period":      rangeCur.Start + " → " + rangeCur.End,
			"sql":         res.SQL,
		},
	}

	if dim != "" {
		ans.Chart = &Chart{Type: "bar", Title: meas.Name + " por " + dim, Columns: res.Columns, Rows: res.Rows}
	} else if model.TimeColumn != "" && (strings.Contains(q, "trend") || strings.Contains(q, "tendência") || strings.Contains(q, "tendencia") || strings.Contains(q, "over time") || strings.Contains(q, "ao longo") || strings.Contains(q, "forecast") || strings.Contains(q, "previs")) {
		ts, err := a.query.Execute(ctx, orgID, wsID, userID, role, queryeng.Request{
			DatasetID: dsID, Measures: []string{measure}, Dimensions: []string{model.TimeColumn}, Limit: 90, TimeRange: rangeCur,
		})
		if err == nil {
			ans.Chart = &Chart{Type: "line", Title: meas.Name + " ao longo do tempo", Columns: ts.Columns, Rows: ts.Rows}
		}
	}

	if cmp {
		prev := &queryeng.TimeRange{Start: addDays(rangeCur.Start, -30), End: rangeCur.Start}
		pres, err := a.query.Execute(ctx, orgID, wsID, userID, role, queryeng.Request{
			DatasetID: dsID, Measures: []string{measure}, Limit: 1, TimeRange: prev,
		})
		if err == nil && len(pres.Rows) > 0 {
			pv := toF(firstNum(pres.Rows[0], alias(measure)))
			if pv != 0 {
				d := (total - pv) / pv * 100
				ans.KeyMetric.Delta = &d
				ans.Evidence["compared_with"] = prev.Start + " → " + prev.End
				if d < 0 {
					ans.Answer = fmt.Sprintf("%s caiu %.1f%%.", meas.Name, abs(d))
				} else {
					ans.Answer = fmt.Sprintf("%s subiu %.1f%%.", meas.Name, d)
				}
			}
		}
		drivers := []string{}
		for _, dname := range []string{"region", "product", "segment", "channel"} {
			if _, ok := semantic.ResolveDimension(model, dname); !ok {
				continue
			}
			curD, err1 := a.query.Execute(ctx, orgID, wsID, userID, role, queryeng.Request{
				DatasetID: dsID, Measures: []string{measure}, Dimensions: []string{dname}, Limit: 20, TimeRange: rangeCur,
			})
			prevD, err2 := a.query.Execute(ctx, orgID, wsID, userID, role, queryeng.Request{
				DatasetID: dsID, Measures: []string{measure}, Dimensions: []string{dname}, Limit: 20, TimeRange: prev,
			})
			if err1 != nil || err2 != nil {
				continue
			}
			pm := map[string]float64{}
			for _, r := range prevD.Rows {
				pm[fmt.Sprint(r[dname])] = toF(firstNum(r, alias(measure)))
			}
			bestK, bestD := "", 0.0
			for _, r := range curD.Rows {
				k := fmt.Sprint(r[dname])
				cv := toF(firstNum(r, alias(measure)))
				pv := pm[k]
				if pv == 0 {
					continue
				}
				dl := (cv - pv) / pv * 100
				if abs(dl) > abs(bestD) {
					bestD, bestK = dl, k
				}
			}
			if bestK != "" {
				drivers = append(drivers, fmt.Sprintf("%s · %s: %+.1f%%", dname, bestK, bestD))
			}
		}
		ans.Drivers = drivers
		ans.Explanation = "Os factores vêm da métrica oficial da camada semântica, comparando o período escolhido com a janela anterior de igual duração."
		if ans.KeyMetric.Delta != nil && *ans.KeyMetric.Delta < 0 {
			ans.Recommendation = "Investigue as fatias com maior queda e crie um alerta de variação semanal de " + meas.Name + "."
		} else {
			ans.Recommendation = "Proteja as fatias em crescimento e replique o que funciona nos segmentos em atraso."
		}
	} else if (strings.Contains(q, "top") || strings.Contains(q, "maiores")) && dim != "" {
		ans.Answer = fmt.Sprintf("Principais %s por %s.", dim, meas.Name)
		ans.Explanation = "Ordenado com a definição oficial " + meas.Expression + "."
		ans.Recommendation = "Concentre cobertura nas contas do topo e inspecione a cauda longa para expansão."
		for i, r := range res.Rows {
			if i >= 5 {
				break
			}
			ans.Drivers = append(ans.Drivers, fmt.Sprintf("%s: %.2f", r[alias(dim)], toF(firstNum(r, alias(measure)))))
		}
	} else if strings.Contains(q, "forecast") || strings.Contains(q, "previs") {
		freq := queryeng.Request{DatasetID: dsID, Measures: []string{measure}, Limit: 90, TimeRange: rangeCur}
		if model.TimeColumn != "" {
			freq.Dimensions = []string{model.TimeColumn}
		}
		ts, err := a.query.Execute(ctx, orgID, wsID, userID, role, freq)
		series := []float64{}
		if err == nil {
			for _, row := range ts.Rows {
				series = append(series, toF(firstNum(row, alias(measure))))
			}
			ans.Chart = &Chart{Type: "line", Title: meas.Name + " + previsão", Columns: ts.Columns, Rows: ts.Rows}
		}
		fc := mljobs.Forecast(ctx, a.rdb, series, 12)
		if fc.Error != "" {
			ans.Answer = fmt.Sprintf("Não consegui prever %s: %s", meas.Name, fc.Error)
		} else {
			src := "worker de analytics"
			if !fc.FromWorker {
				src = "baseline local (mínimos quadrados)"
			}
			last := 0.0
			if len(fc.Forecast) > 0 {
				last = fc.Forecast[len(fc.Forecast)-1]
			}
			ans.Answer = fmt.Sprintf("Previsão de %s (horizonte 12) via %s: último ponto estimado %.2f.", meas.Name, src, last)
			ans.Evidence["forecast"] = fc
			ans.Evidence["assumptions"] = fc.Assumptions
		}
		ans.Explanation = strings.Join(fc.Assumptions, " ")
		ans.Recommendation = "Compare o intervalo [low, high] com o pipeline comercial antes de comprometer meta."
	} else {
		ans.Answer = fmt.Sprintf("%s é %.2f no período seleccionado.", meas.Name, total)
		ans.Explanation = "Calculado como " + meas.Expression + " no conjunto " + dsName + "."
		ans.Recommendation = "Pergunte porque mudou, ou desdobre por região, produto ou cliente."
	}
	if ans.Answer == "" {
		ans.Answer = fmt.Sprintf("%s é %.2f no período seleccionado.", meas.Name, total)
	}

	if ans.Chart == nil && dim != "" {
		ans.Chart = &Chart{Type: "bar", Title: meas.Name + " por " + dim, Columns: res.Columns, Rows: res.Rows}
	}
	return ans, nil
}

func (a *Agent) askLLM(ctx context.Context, orgID, wsID, userID uuid.UUID, role string, dsID, dsName string, model semantic.Model, msg string) (Answer, error) {
	schema, _ := json.Marshal(model)
	sys := `És a TheDobra, analista de negócio nativa em IA. DEVE usar apenas métricas oficiais da camada semântica.
Nunca inventes uma fórmula. Se os dados forem insuficientes, di-lo. Se as métricas conflitarem, pede a definição oficial.
Responde em português do Brasil, em JSON: {"answer":"", "explanation":"", "recommendation":"", "drivers":[], "measure":"", "dimension":"", "chart_type":"bar|line|none"}`
	user := fmt.Sprintf("Conjunto: %s\nModelo semântico: %s\nPergunta: %s", dsName, schema, msg)
	raw, err := a.callOpenAI(ctx, sys, user)
	if err != nil {
		return Answer{}, err
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(raw, &parsed) != nil || len(parsed.Choices) == 0 {
		return Answer{}, fmt.Errorf("invalid llm response")
	}
	var plan struct {
		Answer         string   `json:"answer"`
		Explanation    string   `json:"explanation"`
		Recommendation string   `json:"recommendation"`
		Drivers        []string `json:"drivers"`
		Measure        string   `json:"measure"`
		Dimension      string   `json:"dimension"`
		ChartType      string   `json:"chart_type"`
	}
	if json.Unmarshal([]byte(parsed.Choices[0].Message.Content), &plan) != nil {
		return a.askDeterministic(ctx, orgID, wsID, userID, role, dsID, dsName, model, msg)
	}
	base, err := a.askDeterministic(ctx, orgID, wsID, userID, role, dsID, dsName, model, msg)
	if err != nil {
		return Answer{}, err
	}
	if plan.Answer != "" {
		base.Answer = plan.Answer
	}
	if plan.Explanation != "" {
		base.Explanation = plan.Explanation
	}
	if plan.Recommendation != "" {
		base.Recommendation = plan.Recommendation
	}
	if len(plan.Drivers) > 0 {
		base.Drivers = plan.Drivers
	}
	return base, nil
}

func (a *Agent) loadModel(ctx context.Context, orgID, wsID uuid.UUID, dsID string) (semantic.Model, string, error) {
	id, err := uuid.Parse(dsID)
	if err != nil {
		return semantic.Model{}, "", err
	}
	var name string
	var schema, raw []byte
	err = a.pg.QueryRow(ctx, `
		SELECT d.name, d.schema_json, COALESCE(s.model_json, '{}'::jsonb) FROM datasets d
		LEFT JOIN semantic_models s ON s.dataset_id=d.id
		WHERE d.id=$1 AND d.org_id=$2 AND d.workspace_id=$3
	`, id, orgID, wsID).Scan(&name, &schema, &raw)
	if err != nil {
		return semantic.Model{}, "", fmt.Errorf("conjunto não encontrado")
	}
	var m semantic.Model
	_ = json.Unmarshal(raw, &m)
	var cols []schemax.Column
	_ = json.Unmarshal(schema, &cols)
	m = semantic.Hydrate(m, cols)
	return m, name, nil
}

// LoadModel exposes the model loader for other handlers.
func (a *Agent) LoadModel(ctx context.Context, orgID, wsID uuid.UUID, dsID string) (semantic.Model, string, error) {
	return a.loadModel(ctx, orgID, wsID, dsID)
}

func (a *Agent) defaultDataset(ctx context.Context, orgID, wsID uuid.UUID) (string, error) {
	var id uuid.UUID
	err := a.pg.QueryRow(ctx, `SELECT id FROM datasets WHERE org_id=$1 AND workspace_id=$2 AND status='ready' ORDER BY updated_at DESC LIMIT 1`,
		orgID, wsID).Scan(&id)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func (a *Agent) ensureConv(ctx context.Context, orgID, wsID, userID uuid.UUID, req AskRequest) (uuid.UUID, error) {
	if req.ConversationID != "" {
		id, err := uuid.Parse(req.ConversationID)
		if err == nil {
			return id, nil
		}
	}
	id := uuid.New()
	title := req.Message
	if len(title) > 80 {
		title = title[:80]
	}
	_, err := a.pg.Exec(ctx, `INSERT INTO ai_conversations (id, org_id, workspace_id, user_id, title) VALUES ($1,$2,$3,$4,$5)`,
		id, orgID, wsID, userID, title)
	return id, err
}

func (a *Agent) storeAssistant(ctx context.Context, convID uuid.UUID, content any) {
	_, _ = a.pg.Exec(ctx, `INSERT INTO ai_messages (conversation_id, role, content) VALUES ($1,'assistant',$2)`,
		convID, mustJSON(content))
}

// GenerateDashboardRequest is the input for AI dashboard generation.
type GenerateDashboardRequest struct {
	Prompt    string `json:"prompt"`
	DatasetID string `json:"dataset_id,omitempty"`
}

// GeneratedDashboard is the result of AI dashboard generation.
type GeneratedDashboard struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Widgets     []map[string]any `json:"widgets"`
	DatasetID   string           `json:"dataset_id"`
	DatasetName string           `json:"dataset_name"`
	Source      string           `json:"source"`
}

// GenerateDashboard builds a dashboard layout from a natural-language prompt.
// It prefers OpenAI when configured and falls back to a deterministic template
// builder otherwise. The request is stored in ai_conversations/ai_messages.
func (a *Agent) GenerateDashboard(ctx context.Context, orgID, wsID, userID uuid.UUID, req GenerateDashboardRequest) (GeneratedDashboard, error) {
	convID, err := a.ensureConv(ctx, orgID, wsID, userID, AskRequest{Message: req.Prompt})
	if err != nil {
		return GeneratedDashboard{}, err
	}
	_, _ = a.pg.Exec(ctx, `INSERT INTO ai_messages (conversation_id, role, content) VALUES ($1,'user',$2)`,
		convID, mustJSON(map[string]any{"text": req.Prompt, "intent": "generate_dashboard"}))

	dsID := req.DatasetID
	if dsID == "" {
		dsID, err = a.defaultDataset(ctx, orgID, wsID)
		if err != nil {
			return GeneratedDashboard{}, fmt.Errorf("nenhum conjunto disponível")
		}
	}
	model, dsName, err := a.loadModel(ctx, orgID, wsID, dsID)
	if err != nil {
		return GeneratedDashboard{}, err
	}

	var out GeneratedDashboard
	if a.cfg.OpenAIKey != "" {
		out, err = a.generateDashboardWithLLM(ctx, req.Prompt, dsID, dsName, model)
		if err != nil {
			out = a.generateDashboardFallback(req.Prompt, dsID, dsName, model)
		}
	} else {
		out = a.generateDashboardFallback(req.Prompt, dsID, dsName, model)
	}
	out.DatasetID = dsID
	out.DatasetName = dsName

	a.storeAssistant(ctx, convID, map[string]any{
		"intent":              "generate_dashboard",
		"generated_dashboard": out,
	})
	return out, nil
}

func (a *Agent) generateDashboardWithLLM(ctx context.Context, prompt, dsID, dsName string, model semantic.Model) (GeneratedDashboard, error) {
	schemaJSON := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":        map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"widgets": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type":  map[string]any{"type": "string", "enum": []string{"kpi", "line", "bar", "area", "pie", "table", "text"}},
						"title": map[string]any{"type": "string"},
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
								"time_range": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"start": map[string]any{"type": "string"},
										"end":   map[string]any{"type": "string"},
									},
								},
							},
						},
					},
					"required": []string{"type", "title", "layout", "query"},
				},
			},
		},
		"required": []string{"name", "widgets"},
	}
	modelJSON, _ := json.Marshal(model)
	sys := `És a TheDobra, especialista em dashboards de negócio. Gere um layout de dashboard em JSON a partir do pedido do utilizador.
Regras:
- Usa APENAS medidas e dimensões do modelo semântico fornecido.
- Nunca inventes nomes de colunas ou fórmulas.
- Para widgets de texto, o campo "text" deve estar ao mesmo nível de "title".
- Escolhe visualizações adequadas: kpi para métricas isoladas, line para tendências temporais, bar/pie para categorias, table para detalhes.
- Distribui os widgets numa grelha 12 colunas (x: 0..11, w: 1..12, h: 2..8). Não deixes sobreposições.
- Inclui sempre dataset_id na query.`
	user := fmt.Sprintf("Conjunto: %s (id=%s)\nModelo semântico: %s\nPedido: %s", dsName, dsID, modelJSON, prompt)
	raw, err := a.callOpenAIJSON(ctx, sys, user, schemaJSON)
	if err != nil {
		return GeneratedDashboard{}, err
	}
	var parsed struct {
		Name        string           `json:"name"`
		Description string           `json:"description"`
		Widgets     []map[string]any `json:"widgets"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return GeneratedDashboard{}, err
	}
	out := GeneratedDashboard{
		Name:        parsed.Name,
		Description: parsed.Description,
		Source:      "openai",
	}
	if out.Name == "" {
		out.Name = "Dashboard gerado pela IA"
	}
	for _, w := range parsed.Widgets {
		fixed := a.validateAndFixWidget(w, dsID, model)
		if fixed != nil {
			out.Widgets = append(out.Widgets, fixed)
		}
	}
	if len(out.Widgets) == 0 {
		return GeneratedDashboard{}, fmt.Errorf("nenhum widget válido gerado")
	}
	return out, nil
}

func (a *Agent) generateDashboardFallback(prompt, dsID, dsName string, model semantic.Model) GeneratedDashboard {
	name := "Dashboard gerado"
	if prompt != "" {
		name = "IA · " + prompt
		if len(name) > 60 {
			name = name[:60]
		}
	}
	out := GeneratedDashboard{Name: name, Description: "Gerado pela TheDobra a partir do conjunto " + dsName, Source: "fallback"}

	measures := []string{}
	for i, m := range model.Measures {
		if i >= 3 {
			break
		}
		measures = append(measures, m.Name)
	}
	dims := []string{}
	for _, d := range model.Dimensions {
		switch d.Column {
		case "region", "product", "channel", "segment", "customer":
			dims = append(dims, d.Column)
		}
	}
	if len(dims) == 0 && len(model.Dimensions) > 0 {
		dims = append(dims, model.Dimensions[0].Column)
	}

	x := 0
	for i, m := range measures {
		out.Widgets = append(out.Widgets, map[string]any{
			"id":     fmt.Sprintf("kpi-%d", i),
			"type":   "kpi",
			"title":  m,
			"layout": map[string]int{"x": x, "y": 0, "w": 4, "h": 2},
			"query":  map[string]any{"dataset_id": dsID, "measures": []string{m}, "limit": 1},
		})
		x += 4
	}
	if len(dims) > 0 && len(measures) > 0 {
		out.Widgets = append(out.Widgets, map[string]any{
			"id":     "bar-1",
			"type":   "bar",
			"title":  measures[0] + " por " + dims[0],
			"layout": map[string]int{"x": 0, "y": 2, "w": 6, "h": 4},
			"query":  map[string]any{"dataset_id": dsID, "measures": []string{measures[0]}, "dimensions": []string{dims[0]}, "limit": 12},
		})
	}
	if model.TimeColumn != "" && len(measures) > 0 {
		out.Widgets = append(out.Widgets, map[string]any{
			"id":     "line-1",
			"type":   "line",
			"title":  measures[0] + " ao longo do tempo",
			"layout": map[string]int{"x": 6, "y": 2, "w": 6, "h": 4},
			"query":  map[string]any{"dataset_id": dsID, "measures": []string{measures[0]}, "dimensions": []string{model.TimeColumn}, "limit": 90},
		})
	}
	if len(dims) > 1 && len(measures) > 0 {
		out.Widgets = append(out.Widgets, map[string]any{
			"id":     "table-1",
			"type":   "table",
			"title":  "Detalhado",
			"layout": map[string]int{"x": 0, "y": 6, "w": 12, "h": 4},
			"query":  map[string]any{"dataset_id": dsID, "measures": measures[:min(2, len(measures))], "dimensions": dims[:min(2, len(dims))], "limit": 20},
		})
	}
	return out
}

func (a *Agent) validateAndFixWidget(w map[string]any, dsID string, model semantic.Model) map[string]any {
	typ, _ := w["type"].(string)
	if typ == "" {
		typ = "bar"
	}
	title, _ := w["title"].(string)
	if title == "" {
		title = "Widget"
	}
	query, ok := w["query"].(map[string]any)
	if !ok {
		query = map[string]any{"dataset_id": dsID}
	}
	query["dataset_id"] = dsID

	measures := []string{}
	if raw, ok := query["measures"].([]any); ok && len(raw) > 0 {
		for _, m := range raw {
			name, _ := m.(string)
			if name == "" {
				continue
			}
			if _, ok := semantic.ResolveMeasure(model, name); ok {
				measures = append(measures, name)
			} else if found := a.findClosestMeasure(model, name); found != "" {
				measures = append(measures, found)
			}
		}
	}
	if len(measures) == 0 && len(model.Measures) > 0 {
		measures = []string{model.Measures[0].Name}
	}
	query["measures"] = measures

	dimensions := []string{}
	if raw, ok := query["dimensions"].([]any); ok && len(raw) > 0 {
		for _, d := range raw {
			name, _ := d.(string)
			if name == "" {
				continue
			}
			if _, ok := semantic.ResolveDimension(model, name); ok {
				dimensions = append(dimensions, name)
			} else if found := a.findClosestDimension(model, name); found != "" {
				dimensions = append(dimensions, found)
			}
		}
	}
	query["dimensions"] = dimensions

	layout, ok := w["layout"].(map[string]any)
	if !ok {
		layout = map[string]any{"x": 0, "y": 0, "w": 6, "h": 4}
	}
	ensureInt := func(k string, def int) int {
		v, ok := layout[k].(float64)
		if ok {
			return int(v)
		}
		i, ok := layout[k].(int)
		if ok {
			return i
		}
		return def
	}
	layout["x"] = ensureInt("x", 0)
	layout["y"] = ensureInt("y", 0)
	layout["w"] = max(2, min(12, ensureInt("w", 6)))
	layout["h"] = max(2, min(8, ensureInt("h", 4)))

	out := map[string]any{
		"id":     w["id"],
		"type":   typ,
		"title":  title,
		"layout": layout,
		"query":  query,
	}
	if typ == "text" {
		out["text"] = w["text"]
	}
	if out["id"] == nil || out["id"] == "" {
		out["id"] = uuid.New().String()
	}
	return out
}

func (a *Agent) findClosestMeasure(model semantic.Model, name string) string {
	want := alias(name)
	if want == "linhas" || want == "orders" || want == "count" {
		if p := semantic.PrimaryMeasure(model); p != "" {
			return p
		}
	}
	for _, m := range model.Measures {
		if strings.Contains(alias(m.Name), want) || strings.Contains(want, alias(m.Name)) {
			return m.Name
		}
	}
	if p := semantic.PrimaryMeasure(model); p != "" {
		return p
	}
	if len(model.Measures) > 0 {
		return model.Measures[0].Name
	}
	return ""
}

func (a *Agent) findClosestDimension(model semantic.Model, name string) string {
	want := alias(name)
	for _, d := range model.Dimensions {
		if strings.Contains(alias(d.Name), want) || strings.Contains(want, alias(d.Column)) || strings.Contains(alias(d.Column), want) {
			return d.Column
		}
	}
	if model.TimeColumn != "" {
		return model.TimeColumn
	}
	if len(model.Dimensions) > 0 {
		return model.Dimensions[0].Column
	}
	return ""
}

func pickMeasure(model semantic.Model, q string) string {
	for _, m := range model.Measures {
		if isRowCountMeasure(m) && !asksForCount(q) {
			continue
		}
		n := strings.ToLower(m.Name)
		c := strings.ToLower(m.Column)
		if strings.Contains(q, n) || strings.Contains(q, strings.ReplaceAll(n, " ", "_")) || (c != "*" && strings.Contains(q, c)) {
			return m.Name
		}
	}
	if strings.Contains(q, "revenue") || strings.Contains(q, "sales") || strings.Contains(q, "receita") || strings.Contains(q, "vendas") || strings.Contains(q, "valor") {
		if name := semantic.PrimaryMeasure(model); name != "" {
			return name
		}
	}
	if strings.Contains(q, "profit") || strings.Contains(q, "lucro") {
		if _, ok := semantic.ResolveMeasure(model, "profit"); ok {
			return "profit"
		}
	}
	if strings.Contains(q, "margin") || strings.Contains(q, "margem") {
		if _, ok := semantic.ResolveMeasure(model, "margin"); ok {
			return "margin"
		}
	}
	if asksForCount(q) {
		for _, m := range model.Measures {
			if isRowCountMeasure(m) {
				return m.Name
			}
		}
	}
	if name := semantic.PrimaryMeasure(model); name != "" {
		return name
	}
	if len(model.Measures) > 0 {
		return model.Measures[0].Name
	}
	return ""
}

func isRowCountMeasure(m semantic.Measure) bool {
	expr := strings.ToLower(strings.ReplaceAll(m.Expression, " ", ""))
	return m.Column == "*" || expr == "count(*)"
}

func asksForCount(q string) bool {
	return strings.Contains(q, "quantos registos") || strings.Contains(q, "número de linhas") || strings.Contains(q, "numero de linhas") || strings.Contains(q, "contagem")
}

func pickDimension(model semantic.Model, q string) string {
	cands := []string{"customer", "product", "region", "seller", "channel", "segment", "cliente", "produto", "região", "regiao", "vendedor", "canal", "segmento", "categoria", "linha", "natureza", "empresa", "mes", "mês"}
	for _, c := range cands {
		if strings.Contains(q, c) {
			key := c
			switch c {
			case "cliente":
				key = "customer"
			case "produto":
				key = "product"
			case "região", "regiao":
				key = "region"
			case "vendedor":
				key = "seller"
			case "canal":
				key = "channel"
			case "segmento":
				key = "segment"
			}
			if d, ok := semantic.ResolveDimension(model, key); ok {
				return d.Column
			}
		}
	}
	if strings.Contains(q, "top") || strings.Contains(q, "maiores") {
		for _, c := range []string{"customer", "product", "region"} {
			if d, ok := semantic.ResolveDimension(model, c); ok {
				return d.Column
			}
		}
	}
	return ""
}

func firstNum(row map[string]any, key string) any {
	if v, ok := row[key]; ok {
		return v
	}
	for k, v := range row {
		if k != key && (strings.Contains(k, key) || true) {
			switch v.(type) {
			case float64, int64, float32, int:
				return v
			}
		}
	}
	return 0
}

func toF(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case float32:
		return float64(t)
	default:
		var f float64
		fmt.Sscanf(fmt.Sprint(t), "%f", &f)
		return f
	}
}

func alias(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", "_"))
}

func addDays(iso string, days int) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.AddDate(0, 0, days).Format("2006-01-02")
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

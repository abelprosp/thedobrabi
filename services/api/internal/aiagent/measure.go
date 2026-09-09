package aiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/semantic"
	"github.com/thedobra/thedobra/services/api/internal/semanticxpr"
)

// GenerateMeasureRequest is the input for AI measure generation.
type GenerateMeasureRequest struct {
	Prompt    string `json:"prompt"`
	DatasetID string `json:"dataset_id,omitempty"`
}

// GeneratedMeasure is a DAX-like measure suggested from a natural-language prompt.
type GeneratedMeasure struct {
	Name        string `json:"name"`
	Expression  string `json:"expression"`
	Explanation string `json:"explanation"`
	Source      string `json:"source"`
	DatasetID   string `json:"dataset_id,omitempty"`
}

// GenerateMeasure builds a custom measure from a natural-language prompt.
// It prefers OpenAI when configured and falls back to a deterministic template
// using columns from the semantic model.
func (a *Agent) GenerateMeasure(ctx context.Context, orgID, wsID, userID uuid.UUID, req GenerateMeasureRequest) (GeneratedMeasure, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return GeneratedMeasure{}, fmt.Errorf("prompt obrigatório")
	}

	dsID := req.DatasetID
	var err error
	if dsID == "" {
		dsID, err = a.defaultDataset(ctx, orgID, wsID)
		if err != nil {
			return GeneratedMeasure{}, fmt.Errorf("nenhum conjunto disponível")
		}
	}
	model, dsName, err := a.loadModel(ctx, orgID, wsID, dsID)
	if err != nil {
		return GeneratedMeasure{}, err
	}

	out := generateMeasureFallback(req.Prompt, model, dsName)
	if a.cfg.OpenAIKey != "" {
		if llm, err := a.generateMeasureWithLLM(ctx, req.Prompt, dsName, model); err == nil {
			out = llm
		}
	}
	out.DatasetID = dsID
	_ = userID
	return out, nil
}

func (a *Agent) generateMeasureWithLLM(ctx context.Context, prompt, dsName string, model semantic.Model) (GeneratedMeasure, error) {
	schemaJSON := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":        map[string]any{"type": "string"},
			"expression":  map[string]any{"type": "string"},
			"explanation": map[string]any{"type": "string"},
		},
		"required": []string{"name", "expression"},
	}
	modelJSON, _ := json.Marshal(summarizeModelForMeasure(model))
	sys := `És a TheDobra, especialista em medidas de negócio. Gera UMA medida DAX-like em JSON.
Regras:
- A expressão é só a fórmula (ex.: SUM(valor), DIVIDE(SUM(receita), COUNT(*)), AVG(preco)). Sem SELECT, FROM, AS, GROUP BY ou ponto e vírgula.
- Funções permitidas: SUM, AVG, AVERAGE, COUNT, DISTINCTCOUNT, MIN, MAX, DIVIDE, NULLIF, CALCULATE, LOOKUPVALUE, RELATED, YOY, TOMONTH, CASE WHEN, SAMEPERIODLASTYEAR, DATEADD, TOTALYTD, TOTALMTD, TOTALQTD.
- Usa APENAS colunas, medidas e dimensões do modelo semântico. Medidas existentes referenciam-se como [Nome].
- Não inventes nomes de colunas.
- O nome da medida deve ser curto e em português, adequado a um dashboard.
- explanation: uma frase a explicar a fórmula.`
	user := fmt.Sprintf("Conjunto: %s\nModelo: %s\nPedido: %s", dsName, string(modelJSON), prompt)
	raw, err := a.callOpenAIJSON(ctx, sys, user, schemaJSON)
	if err != nil {
		return GeneratedMeasure{}, err
	}
	var parsed struct {
		Name        string `json:"name"`
		Expression  string `json:"expression"`
		Explanation string `json:"explanation"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return GeneratedMeasure{}, err
	}
	expr := sanitizeMeasureExpression(parsed.Expression)
	if _, err := semanticxpr.Parse(expr); err != nil {
		return GeneratedMeasure{}, fmt.Errorf("expressão gerada inválida: %w", err)
	}
	name := strings.TrimSpace(strings.ReplaceAll(parsed.Name, "\n", " "))
	if name == "" {
		name = "Nova métrica"
	}
	if len(name) > 80 {
		name = strings.TrimSpace(name[:80])
	}
	expl := strings.TrimSpace(parsed.Explanation)
	if expl == "" {
		expl = "Medida gerada pela IA a partir do pedido e das colunas do conjunto."
	}
	return GeneratedMeasure{
		Name:        name,
		Expression:  expr,
		Explanation: expl,
		Source:      "openai",
	}, nil
}

func generateMeasureFallback(prompt string, model semantic.Model, dsName string) GeneratedMeasure {
	q := strings.ToLower(prompt)
	col := pickMeasureColumn(model, q)
	name := "Nova métrica"
	expr := "SUM(" + col + ")"
	expl := "Soma de " + col

	switch {
	case strings.Contains(q, "ticket"):
		name = "Ticket médio"
		expr = "DIVIDE(SUM(" + col + "), COUNT(*))"
		expl = "Valor médio por linha a partir de " + col
	case strings.Contains(q, "média") || strings.Contains(q, "media") || strings.Contains(q, "average") || strings.Contains(q, "avg"):
		name = "Média"
		expr = "AVG(" + col + ")"
		expl = "Média de " + col
	case strings.Contains(q, "contagem") || strings.Contains(q, "contar") || strings.Contains(q, "count") || strings.Contains(q, "número de") || strings.Contains(q, "numero de") || strings.Contains(q, "quantos"):
		name = "Contagem"
		expr = "COUNT(*)"
		expl = "Número de linhas"
	case strings.Contains(q, "yoy") || strings.Contains(q, "ano anterior") || strings.Contains(q, "variação anual") || strings.Contains(q, "variacao anual"):
		name = "YoY"
		expr = "YOY(" + col + ")"
		expl = "Variação face ao mesmo período do ano anterior em " + col
	case strings.Contains(q, "mín") || strings.Contains(q, "minimo") || strings.Contains(q, "mínimo") || strings.Contains(q, "min("):
		name = "Mínimo"
		expr = "MIN(" + col + ")"
		expl = "Valor mínimo de " + col
	case strings.Contains(q, "máx") || strings.Contains(q, "maximo") || strings.Contains(q, "máximo") || strings.Contains(q, "max("):
		name = "Máximo"
		expr = "MAX(" + col + ")"
		expl = "Valor máximo de " + col
	default:
		name = titleFromPrompt(prompt, "Nova métrica")
		if matched := matchMeasureInPrompt(model, q); matched != nil {
			if matched.Expression != "" {
				expr = matched.Expression
				expl = "Medida existente do modelo: " + matched.Name
			} else if matched.Column != "" && matched.Column != "*" {
				agg := strings.ToUpper(strings.TrimSpace(matched.Aggregation))
				if agg == "" || agg == "EXPRESSION" {
					agg = "SUM"
				}
				if agg == "COUNT_DISTINCT" {
					expr = "DISTINCTCOUNT(" + matched.Column + ")"
				} else if agg == "COUNT" {
					expr = "COUNT(" + matched.Column + ")"
				} else if agg == "AVG" || agg == "AVERAGE" {
					expr = "AVG(" + matched.Column + ")"
				} else {
					expr = agg + "(" + matched.Column + ")"
				}
				expl = "Agregação de " + matched.Column
			}
		}
	}

	if dsName != "" {
		expl += " no conjunto " + dsName + "."
	} else {
		expl += "."
	}
	return GeneratedMeasure{
		Name:        name,
		Expression:  expr,
		Explanation: expl,
		Source:      "fallback",
	}
}

func sanitizeMeasureExpression(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```sql")
	s = strings.TrimPrefix(s, "```dax")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, ";")
	return strings.TrimSpace(s)
}

func summarizeModelForMeasure(model semantic.Model) map[string]any {
	measures := make([]map[string]string, 0, len(model.Measures))
	for _, m := range model.Measures {
		item := map[string]string{"name": m.Name, "column": m.Column, "aggregation": m.Aggregation}
		if m.Expression != "" {
			item["expression"] = m.Expression
		}
		measures = append(measures, item)
	}
	dims := make([]map[string]string, 0, len(model.Dimensions))
	for _, d := range model.Dimensions {
		dims = append(dims, map[string]string{"name": d.Name, "column": d.Column, "type": d.Type})
	}
	return map[string]any{
		"time_column": model.TimeColumn,
		"measures":    measures,
		"dimensions":  dims,
	}
}

func pickMeasureColumn(model semantic.Model, q string) string {
	if m := matchMeasureInPrompt(model, q); m != nil && m.Column != "" && m.Column != "*" {
		return m.Column
	}
	for _, m := range model.Measures {
		if m.Column != "" && m.Column != "*" {
			return m.Column
		}
	}
	return "revenue"
}

func titleFromPrompt(prompt, fallback string) string {
	n := strings.TrimSpace(strings.Split(prompt, "\n")[0])
	if n == "" {
		return fallback
	}
	runes := []rune(n)
	if len(runes) > 48 {
		n = string(runes[:48])
	}
	return n
}

func matchMeasureInPrompt(model semantic.Model, q string) *semantic.Measure {
	for i := range model.Measures {
		m := &model.Measures[i]
		ln := strings.ToLower(m.Name)
		lc := strings.ToLower(m.Column)
		if ln != "" && strings.Contains(q, ln) {
			return m
		}
		if lc != "" && lc != "*" && strings.Contains(q, lc) {
			return m
		}
		if ln != "" && strings.Contains(q, strings.ReplaceAll(ln, " ", "_")) {
			return m
		}
	}
	return nil
}

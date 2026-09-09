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

// GenerateDimensionRequest is the input for AI dimension generation.
type GenerateDimensionRequest struct {
	Prompt    string `json:"prompt"`
	DatasetID string `json:"dataset_id,omitempty"`
}

// GeneratedDimension is a row-level SQL expression suggested from a prompt.
type GeneratedDimension struct {
	Name        string `json:"name"`
	Expression  string `json:"expression"`
	Explanation string `json:"explanation"`
	Source      string `json:"source"`
	DatasetID   string `json:"dataset_id,omitempty"`
}

// GenerateDimension builds a calculated dimension from a natural-language prompt.
func (a *Agent) GenerateDimension(ctx context.Context, orgID, wsID, userID uuid.UUID, req GenerateDimensionRequest) (GeneratedDimension, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return GeneratedDimension{}, fmt.Errorf("prompt obrigatório")
	}
	dsID := req.DatasetID
	var err error
	if dsID == "" {
		dsID, err = a.defaultDataset(ctx, orgID, wsID)
		if err != nil {
			return GeneratedDimension{}, fmt.Errorf("nenhum conjunto disponível")
		}
	}
	model, dsName, err := a.loadModel(ctx, orgID, wsID, dsID)
	if err != nil {
		return GeneratedDimension{}, err
	}
	out := generateDimensionFallback(req.Prompt, model, dsName)
	if a.cfg.OpenAIKey != "" {
		if llm, err := a.generateDimensionWithLLM(ctx, req.Prompt, dsName, model); err == nil {
			out = llm
		}
	}
	out.DatasetID = dsID
	_ = userID
	return out, nil
}

func (a *Agent) generateDimensionWithLLM(ctx context.Context, prompt, dsName string, model semantic.Model) (GeneratedDimension, error) {
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
	sys := `És a TheDobra, especialista em dimensões de negócio. Gera UMA dimensão calculada em JSON.
Regras:
- A expressão é só a fórmula ao nível da linha. Sem SELECT, FROM, AS, GROUP BY, ponto e vírgula ou agregações.
- Proibido: SUM, AVG, COUNT, DISTINCTCOUNT, MIN, MAX, CALCULATE, YOY.
- Permitido: CASE WHEN … THEN … ELSE … END, TOMONTH(coluna), YEAR(coluna), COALESCE, comparações (=, IN), colunas do modelo.
- Usa APENAS colunas e dimensões existentes. Não inventes nomes.
- O nome deve ser curto e em português, adequado a um eixo ou slicer.
- explanation: uma frase a explicar o agrupamento.`
	user := fmt.Sprintf("Conjunto: %s\nModelo: %s\nPedido: %s", dsName, string(modelJSON), prompt)
	raw, err := a.callOpenAIJSON(ctx, sys, user, schemaJSON)
	if err != nil {
		return GeneratedDimension{}, err
	}
	var parsed struct {
		Name        string `json:"name"`
		Expression  string `json:"expression"`
		Explanation string `json:"explanation"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return GeneratedDimension{}, err
	}
	expr := sanitizeMeasureExpression(parsed.Expression)
	parsedExpr, err := semanticxpr.Parse(expr)
	if err != nil {
		return GeneratedDimension{}, fmt.Errorf("expressão gerada inválida: %w", err)
	}
	if parsedExpr.IsAggregate() {
		return GeneratedDimension{}, fmt.Errorf("a dimensão não pode usar agregações")
	}
	name := strings.TrimSpace(strings.ReplaceAll(parsed.Name, "\n", " "))
	if name == "" {
		name = "Nova dimensão"
	}
	if len(name) > 80 {
		name = strings.TrimSpace(name[:80])
	}
	expl := strings.TrimSpace(parsed.Explanation)
	if expl == "" {
		expl = "Dimensão gerada pela IA a partir do pedido e das colunas do conjunto."
	}
	return GeneratedDimension{
		Name:        name,
		Expression:  expr,
		Explanation: expl,
		Source:      "openai",
	}, nil
}

func generateDimensionFallback(prompt string, model semantic.Model, dsName string) GeneratedDimension {
	q := strings.ToLower(prompt)
	timeCol := strings.TrimSpace(model.TimeColumn)
	if timeCol == "" {
		for _, d := range model.Dimensions {
			n := strings.ToLower(d.Name + " " + d.Column)
			if strings.Contains(n, "data") || strings.Contains(n, "date") || strings.Contains(n, "mes") || strings.Contains(n, "mês") {
				timeCol = d.Column
				break
			}
		}
	}
	col := pickDimensionColumn(model, q)
	name := titleFromPrompt(prompt, "Nova dimensão")
	expr := col
	expl := "Agrupa pela coluna " + col

	switch {
	case strings.Contains(q, "mês") || strings.Contains(q, "mes") || strings.Contains(q, "month") || strings.Contains(q, "ano-mês"):
		if timeCol != "" {
			name = "Mês"
			expr = "TOMONTH(" + timeCol + ")"
			expl = "Mês extraído de " + timeCol
		}
	case strings.Contains(q, "ano") || strings.Contains(q, "year"):
		if timeCol != "" {
			name = "Ano"
			expr = "YEAR(" + timeCol + ")"
			expl = "Ano extraído de " + timeCol
		}
	case strings.Contains(q, "caso") || strings.Contains(q, "quando") || strings.Contains(q, "se ") || strings.Contains(q, "grupo") || strings.Contains(q, "faixa"):
		name = titleFromPrompt(prompt, "Grupo")
		expr = "CASE WHEN " + col + " = '' THEN 'Vazio' ELSE " + col + " END"
		expl = "Agrupamento condicional a partir de " + col
	}

	if dsName != "" {
		expl += " no conjunto " + dsName + "."
	} else {
		expl += "."
	}
	return GeneratedDimension{
		Name:        name,
		Expression:  expr,
		Explanation: expl,
		Source:      "fallback",
	}
}

func pickDimensionColumn(model semantic.Model, q string) string {
	for _, d := range model.Dimensions {
		ln := strings.ToLower(d.Name)
		lc := strings.ToLower(d.Column)
		if ln != "" && strings.Contains(q, ln) && d.Column != "" {
			return d.Column
		}
		if lc != "" && strings.Contains(q, lc) {
			return d.Column
		}
	}
	for _, d := range model.Dimensions {
		if d.Column != "" && strings.TrimSpace(d.Expression) == "" {
			return d.Column
		}
	}
	return "categoria"
}

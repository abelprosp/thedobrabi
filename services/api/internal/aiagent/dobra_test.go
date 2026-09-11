package aiagent

import (
	"strings"
	"testing"

	"github.com/thedobra/thedobra/services/api/internal/semantic"
)

func sampleModel() semantic.Model {
	return semantic.Model{
		TimeColumn: "mes",
		Measures: []semantic.Measure{
			{Name: "Receita", Column: "valor", Expression: "SUM(valor)"},
			{Name: "Linhas", Column: "*", Expression: "COUNT(*)"},
		},
		Dimensions: []semantic.Dimension{
			{Name: "Mês", Column: "mes"},
			{Name: "Empresa", Column: "empresa"},
			{Name: "Natureza", Column: "natureza"},
		},
	}
}

func TestDobraFallbackMountsDashboard(t *testing.T) {
	a := &Agent{}
	out := a.dobraFallback(DobraRequest{Message: "monta um dashboard comercial com filtros e análise"}, "ds-1", "Vendas", sampleModel())
	if !out.Apply || !out.Replace {
		t.Fatalf("apply/replace: %+v", out)
	}
	if len(out.Widgets) < 4 {
		t.Fatalf("widgets=%d plan=%d", len(out.Widgets), len(out.Plan))
	}
	types := map[string]bool{}
	for _, w := range out.Widgets {
		typ, _ := w["type"].(string)
		types[typ] = true
	}
	for _, need := range []string{"kpi", "line", "ranking", "slicer", "data_intelligence"} {
		if !types[need] {
			t.Fatalf("missing %s in %v", need, types)
		}
	}
	if !strings.Contains(strings.ToLower(out.Reply), "vendas") {
		t.Fatalf("reply=%s", out.Reply)
	}
}

func TestDobraFallbackProposeOnly(t *testing.T) {
	a := &Agent{}
	out := a.dobraFallback(DobraRequest{Message: "só sugere um plano"}, "ds-1", "Vendas", sampleModel())
	if out.Apply {
		t.Fatal("should not apply")
	}
}

func TestDobraFallbackAppend(t *testing.T) {
	a := &Agent{}
	out := a.dobraFallback(DobraRequest{
		Message: "adiciona um ranking",
		Widgets: []map[string]any{{"type": "kpi", "title": "Receita"}},
	}, "ds-1", "Vendas", sampleModel())
	if out.Replace {
		t.Fatal("append should not replace")
	}
	if !out.Apply {
		t.Fatal("should apply")
	}
}

func TestValidateSlicerAndIntelligence(t *testing.T) {
	a := &Agent{}
	m := sampleModel()
	slicer := a.validateAndFixWidget(map[string]any{
		"type":  "slicer",
		"title": "Empresa",
		"query": map[string]any{"dimensions": []any{"empresa"}},
	}, "ds-1", m)
	q := slicer["query"].(map[string]any)
	dims := asStringSlice(q["dimensions"])
	if len(dims) != 1 || dims[0] != "empresa" {
		t.Fatalf("slicer dims=%v", q["dimensions"])
	}
	intel := a.validateAndFixWidget(map[string]any{"type": "data_intelligence", "title": "Análise"}, "ds-1", m)
	if _, ok := intel["query"]; ok {
		t.Fatal("intelligence should not require query")
	}
}

func TestValidateWidgetRejectsInventedSemanticFields(t *testing.T) {
	a := &Agent{}
	fixed, warnings := a.validateAndFixWidgetDetailed(map[string]any{
		"type":  "bar",
		"title": "Campo inventado",
		"query": map[string]any{
			"measures":   []any{"Receita imaginária"},
			"dimensions": []any{"Planeta"},
		},
	}, "ds-1", sampleModel())
	if fixed != nil {
		t.Fatalf("invented fields should reject widget: %#v", fixed)
	}
	if len(warnings) == 0 {
		t.Fatal("expected a validation warning")
	}
}

func TestValidateDobraFiltersCanonicalizesDimension(t *testing.T) {
	filters, warnings := validateDobraFilters([]DobraFilter{
		{Dimension: "Mês", Op: "eq", Value: "2026-09"},
		{Dimension: "Inexistente", Op: "eq", Value: "x"},
	}, sampleModel())
	if len(filters) != 1 || filters[0].Dimension != "mes" {
		t.Fatalf("filters: %#v", filters)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings: %#v", warnings)
	}
}

func TestContextualAskMessageKeepsFollowUpContext(t *testing.T) {
	got := contextualAskMessage("E por empresa?", []AskTurn{{Role: "user", Text: "Mostre a receita deste mês"}})
	if !strings.Contains(got, "Mostre a receita") || !strings.Contains(got, "E por empresa?") {
		t.Fatalf("context: %q", got)
	}
}

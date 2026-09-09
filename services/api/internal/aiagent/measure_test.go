package aiagent

import (
	"testing"

	"github.com/thedobra/thedobra/services/api/internal/semantic"
	"github.com/thedobra/thedobra/services/api/internal/semanticxpr"
)

func testModel() semantic.Model {
	return semantic.Model{
		Name: "Vendas",
		Measures: []semantic.Measure{
			{Name: "Receita", Column: "valor_mensal", Aggregation: "sum"},
			{Name: "Clientes", Column: "cliente_id", Aggregation: "count_distinct"},
		},
	}
}

func TestGenerateMeasureFallbackUsesModelColumn(t *testing.T) {
	out := generateMeasureFallback("soma da receita", testModel(), "Vendas")
	if out.Expression != "SUM(valor_mensal)" {
		t.Fatalf("expression: %s", out.Expression)
	}
	if out.Name != "soma da receita" {
		t.Fatalf("name: %s", out.Name)
	}
	if out.Source != "fallback" {
		t.Fatalf("source: %s", out.Source)
	}
	if _, err := semanticxpr.Parse(out.Expression); err != nil {
		t.Fatalf("parse: %v", err)
	}
}

func TestGenerateMeasureFallbackTicket(t *testing.T) {
	out := generateMeasureFallback("ticket médio", testModel(), "Vendas")
	if out.Expression != "DIVIDE(SUM(valor_mensal), COUNT(*))" {
		t.Fatalf("expression: %s", out.Expression)
	}
	if _, err := semanticxpr.Parse(out.Expression); err != nil {
		t.Fatalf("parse: %v", err)
	}
}

func TestGenerateMeasureFallbackAverageAndCount(t *testing.T) {
	avg := generateMeasureFallback("média de valor_mensal", testModel(), "")
	if avg.Expression != "AVG(valor_mensal)" {
		t.Fatalf("avg: %s", avg.Expression)
	}
	cnt := generateMeasureFallback("quantos registos", testModel(), "")
	if cnt.Expression != "COUNT(*)" {
		t.Fatalf("count: %s", cnt.Expression)
	}
}

func TestSanitizeMeasureExpression(t *testing.T) {
	got := sanitizeMeasureExpression("```sql\nSUM(valor);\n```")
	if got != "SUM(valor)" {
		t.Fatalf("got %q", got)
	}
}

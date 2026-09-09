package aiagent

import (
	"testing"

	"github.com/thedobra/thedobra/services/api/internal/semantic"
)

func TestGenerateDimensionFallbackMonth(t *testing.T) {
	model := semantic.Model{
		TimeColumn: "data_venda",
		Dimensions: []semantic.Dimension{{Name: "Data", Column: "data_venda"}},
	}
	out := generateDimensionFallback("mês da venda", model, "Vendas")
	if out.Expression != "TOMONTH(data_venda)" {
		t.Fatalf("got %q", out.Expression)
	}
	if out.Name != "Mês" {
		t.Fatalf("name %q", out.Name)
	}
}

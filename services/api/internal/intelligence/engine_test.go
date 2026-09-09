package intelligence

import (
	"testing"

	"github.com/thedobra/thedobra/services/api/internal/semantic"
)

func TestPickTimeAndCategoryDimensions(t *testing.T) {
	model := semantic.Model{
		TimeColumn: "data_venda",
		Dimensions: []semantic.Dimension{
			{Name: "Mês", Column: "mes"},
			{Name: "Empresa", Column: "empresa"},
			{Name: "Produto", Column: "produto"},
		},
		Measures: []semantic.Measure{{Name: "Receita", Expression: "SUM(valor)"}},
	}
	timeDim := pickTimeDimension(model)
	if timeDim == "" {
		t.Fatal("expected time dimension")
	}
	cats := pickCategoryDimensions(model, timeDim, 3)
	if len(cats) == 0 {
		t.Fatal("expected category dimensions")
	}
	for _, c := range cats {
		if c == timeDim {
			t.Fatalf("time dim leaked into categories: %v", cats)
		}
	}
}

func TestDimLooksTimeSkipsEmpresa(t *testing.T) {
	if dimLooksTime(semantic.Dimension{Name: "Empresa", Column: "empresa"}, "data_venda") {
		t.Fatal("empresa is not time")
	}
	if !dimLooksTime(semantic.Dimension{Name: "Mês", Column: "mes"}, "") {
		t.Fatal("mês should be time")
	}
}

func TestMeasureValueAlias(t *testing.T) {
	row := map[string]any{"Receita": 10.0, "empresa": "VIVO"}
	if measureValue(row, "Receita") != 10 {
		t.Fatal("exact")
	}
	if measureValue(map[string]any{"receita": 12.0}, "Receita") != 12 {
		t.Fatal("folded")
	}
}

func TestPrimaryMeasureUsedInsteadOfRevenue(t *testing.T) {
	model := semantic.Model{Measures: []semantic.Measure{{Name: "Receita", Expression: "SUM(valor)"}}}
	if semantic.PrimaryMeasure(model) != "Receita" {
		t.Fatal(semantic.PrimaryMeasure(model))
	}
}

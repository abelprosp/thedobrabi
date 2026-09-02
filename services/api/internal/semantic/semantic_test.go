package semantic

import (
	"testing"

	"github.com/thedobra/thedobra/services/api/internal/schemax"
)

func TestSuggestRevenue(t *testing.T) {
	cols := []schemax.Column{
		{Name: "order_date", Type: schemax.TypeDate, Role: "time"},
		{Name: "region", Type: schemax.TypeString, Role: "dimension"},
		{Name: "revenue", Type: schemax.TypeFloat, Role: "measure"},
	}
	m := Suggest("sales", cols)
	if _, ok := ResolveMeasure(m, "revenue"); !ok {
		t.Fatal("expected revenue measure")
	}
	if m.TimeColumn != "order_date" {
		t.Fatalf("time column: %s", m.TimeColumn)
	}
}

func TestSuggestIBGEStringOnlyHasCount(t *testing.T) {
	cols := []schemax.Column{
		{Name: "id", Type: schemax.TypeInt, Role: "id"},
		{Name: "nome", Type: schemax.TypeString},
		{Name: "uf", Type: schemax.TypeString},
	}
	m := Suggest("Municípios", cols)
	if _, ok := ResolveMeasure(m, "linhas"); !ok {
		t.Fatal("expected Linhas COUNT(*) for datasets without numeric measures")
	}
	if _, ok := ResolveDimension(m, "nome"); !ok {
		t.Fatal("expected nome dimension")
	}
	if _, ok := ResolveDimension(m, "uf"); !ok {
		t.Fatal("expected uf dimension")
	}
}

func TestSuggestInflacaoNumericSum(t *testing.T) {
	cols := []schemax.Column{
		{Name: "data", Type: schemax.TypeDate, Role: "time"},
		{Name: "indice", Type: schemax.TypeString},
		{Name: "valor", Type: schemax.TypeFloat},
	}
	m := Suggest("IPCA", cols)
	if _, ok := ResolveMeasure(m, "valor"); !ok {
		t.Fatal("expected SUM(valor)")
	}
	if _, ok := ResolveDimension(m, "indice"); !ok {
		t.Fatal("expected indice dimension")
	}
}

func TestHydrateEmptyModel(t *testing.T) {
	cols := []schemax.Column{
		{Name: "categoria", Type: schemax.TypeString},
		{Name: "montante", Type: schemax.TypeFloat},
	}
	m := Hydrate(Model{Name: "Asaas pagamentos"}, cols)
	if _, ok := ResolveMeasure(m, "montante"); !ok {
		t.Fatal("expected montante measure after hydrate")
	}
	if _, ok := ResolveDimension(m, "categoria"); !ok {
		t.Fatal("expected categoria dimension after hydrate")
	}
}

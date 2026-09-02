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
	if _, ok := ResolveMeasure(m, "n.º de registos"); !ok {
		if PrimaryMeasure(m) == "" {
			t.Fatal("expected a row-count measure for datasets without numeric measures")
		}
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

func TestPreferValorOverLinhas(t *testing.T) {
	cols := []schemax.Column{
		{Name: "data", Type: schemax.TypeDate, Role: "time"},
		{Name: "categoria", Type: schemax.TypeString},
		{Name: "linha", Type: schemax.TypeString},
		{Name: "natureza", Type: schemax.TypeString},
		{Name: "valor", Type: schemax.TypeFloat},
		{Name: "empresa", Type: schemax.TypeString},
	}
	m := Suggest("redorai-financeiro", cols)
	if PrimaryMeasure(m) != "Valor" {
		t.Fatalf("primary measure want Valor, got %q among %+v", PrimaryMeasure(m), m.Measures)
	}
	if _, ok := ResolveDimension(m, "linha"); !ok {
		t.Fatal("expected linha as dimension")
	}
	for _, meas := range m.Measures {
		if isRowCount(meas) {
			t.Fatalf("did not expect row-count measure when valor exists: %+v", meas)
		}
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

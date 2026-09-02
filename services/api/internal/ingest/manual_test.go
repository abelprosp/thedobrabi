package ingest

import "testing"

func TestNormalizeManualColumns(t *testing.T) {
	cols, err := NormalizeManualColumns([]ManualColumn{
		{Label: "Data", Type: "date", Required: true},
		{Label: "Valor", Type: "number"},
		{Label: "Categoria", Type: "select", Options: []string{"A", "A", " B "}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 3 {
		t.Fatalf("len %d", len(cols))
	}
	if cols[0].Key != "data" || cols[0].Type != "date" || !cols[0].Required {
		t.Fatalf("data: %+v", cols[0])
	}
	if cols[1].Key != "valor" || cols[1].Type != "number" {
		t.Fatalf("valor: %+v", cols[1])
	}
	if cols[2].Key != "categoria" || len(cols[2].Options) != 2 || cols[2].Options[1] != "B" {
		t.Fatalf("categoria: %+v", cols[2])
	}
}

func TestNormalizeManualColumnsRejectsEmpty(t *testing.T) {
	if _, err := NormalizeManualColumns(nil); err == nil {
		t.Fatal("expected error")
	}
	if _, err := NormalizeManualColumns([]ManualColumn{{Label: "  "}}); err == nil {
		t.Fatal("expected error for blank labels")
	}
	if _, err := NormalizeManualColumns([]ManualColumn{{Label: "Estado", Type: "select"}}); err == nil {
		t.Fatal("select without options")
	}
}

func TestValidateManualRow(t *testing.T) {
	cols := []ManualColumn{
		{Key: "cliente", Label: "Cliente", Type: "text", Required: true},
		{Key: "valor", Label: "Valor", Type: "number", Required: true},
		{Key: "activo", Label: "Activo", Type: "boolean"},
	}
	got, err := validateManualRow(cols, map[string]any{"cliente": " Ana ", "valor": "12,5", "activo": "sim"})
	if err != nil {
		t.Fatal(err)
	}
	if got["cliente"] != "Ana" {
		t.Fatalf("cliente %v", got["cliente"])
	}
	n, ok := got["valor"].(float64)
	if !ok || n != 12.5 {
		t.Fatalf("valor %v", got["valor"])
	}
	if got["activo"] != true {
		t.Fatalf("activo %v", got["activo"])
	}
	if _, err := validateManualRow(cols, map[string]any{"valor": 1}); err == nil {
		t.Fatal("missing required cliente")
	}
	if _, err := validateManualRow(cols, map[string]any{"cliente": "x", "valor": "abc"}); err == nil {
		t.Fatal("invalid number")
	}
}

func TestSlugKeyAccents(t *testing.T) {
	if slugKey("Descrição") != "descricao" {
		t.Fatalf("got %q", slugKey("Descrição"))
	}
	if slugKey("  Valor (€)  ") != "valor" {
		t.Fatalf("got %q", slugKey("  Valor (€)  "))
	}
}

func TestManualColType(t *testing.T) {
	if manualColType("number") != "float" {
		t.Fatal("number -> float")
	}
	if manualColType("date") != "date" {
		t.Fatal("date")
	}
	if manualColType("text") != "string" {
		t.Fatal("text")
	}
}

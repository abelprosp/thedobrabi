package ingest

import (
	"testing"

	"github.com/thedobra/thedobra/services/api/internal/schemax"
)

func TestAlignToSchemaMatchesNameAndSource(t *testing.T) {
	cols := []schemax.Column{
		{Name: "empresa", SourceName: "Empresa"},
		{Name: "valor", SourceName: "Valor mensal"},
		{Name: "data_venda", SourceName: "Data"},
	}
	headers := []string{"Empresa", "Valor mensal", "extra"}
	rows := [][]string{{"VIVO", "10", "x"}, {"TIM", "20", "y"}}
	got, mapped := alignToSchema(cols, headers, rows)
	if mapped != 2 {
		t.Fatalf("mapped=%d", mapped)
	}
	if len(got) != 2 || got[0][0] != "VIVO" || got[0][1] != "10" || got[0][2] != "" {
		t.Fatalf("got %#v", got)
	}
}

func TestAlignToSchemaNoMatch(t *testing.T) {
	cols := []schemax.Column{{Name: "empresa"}, {Name: "valor"}}
	_, mapped := alignToSchema(cols, []string{"foo", "bar"}, [][]string{{"a", "1"}})
	if mapped != 0 {
		t.Fatalf("mapped=%d", mapped)
	}
}

func TestMapsToTableSkipsEmptyAndSystemCols(t *testing.T) {
	cols := []schemax.Column{{Name: "empresa"}, {Name: "valor"}}
	headers, rows := mapsToTable(cols, []map[string]any{
		{"_ingested_at": "2026-01-01", "empresa": "VIVO", "valor": 10},
		{"Empresa": "TIM", "valor": "20"},
		{"_ingested_at": "x"},
		nil,
	})
	if len(headers) != 2 || headers[0] != "empresa" {
		t.Fatalf("headers=%v", headers)
	}
	if len(rows) != 2 || rows[0][0] != "VIVO" || rows[0][1] != "10" || rows[1][0] != "TIM" {
		t.Fatalf("rows=%v", rows)
	}
}

func TestParseUploadedBytesCSV(t *testing.T) {
	raw := []byte("empresa,valor\nVIVO,10\nTIM,20\n")
	kind, headers, rows, err := parseUploadedBytes("vendas.csv", raw)
	if err != nil {
		t.Fatal(err)
	}
	if kind != "csv" || len(headers) != 2 || len(rows) != 2 {
		t.Fatalf("kind=%s headers=%v rows=%d", kind, headers, len(rows))
	}
}

func TestParseUploadedBytesEmpty(t *testing.T) {
	if _, _, _, err := parseUploadedBytes("x.csv", []byte("")); err == nil {
		t.Fatal("expected error")
	}
}

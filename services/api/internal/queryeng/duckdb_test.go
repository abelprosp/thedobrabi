package queryeng

import (
	"context"
	"testing"
)

func TestDuckDBKPIAggregatesWithoutGroupBy(t *testing.T) {
	rows := []map[string]any{
		{"headcount": 1, "salario": 1000.0, "ano": 2025},
		{"headcount": 1, "salario": 2500.0, "ano": 2025},
		{"headcount": 0, "salario": 1800.0, "ano": 2026},
	}
	exec := NewDuckDBExecutor(func(context.Context, int) ([]string, []map[string]any, error) {
		return []string{"ano", "headcount", "salario"}, rows, nil
	})
	res, err := exec.Execute(context.Background(), "SELECT SUM(`salario`) AS `salario` FROM rows", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("want 1 aggregated row, got %d", len(res.Rows))
	}
	if got := toF(res.Rows[0]["salario"]); got != 5300 {
		t.Fatalf("salario sum: got %v want 5300", got)
	}
	if _, ok := res.Rows[0]["headcount"]; ok {
		t.Fatal("raw headcount should not leak into the KPI row")
	}

	hc, err := exec.Execute(context.Background(), "SELECT SUM(`headcount`) AS `headcount` FROM rows", 20)
	if err != nil {
		t.Fatal(err)
	}
	if got := toF(hc.Rows[0]["headcount"]); got != 2 {
		t.Fatalf("headcount sum: got %v want 2", got)
	}
}

func TestDuckDBAvgWithoutGroupBy(t *testing.T) {
	rows := []map[string]any{
		{"nota": 4.0},
		{"nota": 2.0},
	}
	exec := NewDuckDBExecutor(func(context.Context, int) ([]string, []map[string]any, error) {
		return []string{"nota"}, rows, nil
	})
	res, err := exec.Execute(context.Background(), "SELECT AVG(`nota`) AS `nota` FROM rows", 20)
	if err != nil {
		t.Fatal(err)
	}
	if got := toF(res.Rows[0]["nota"]); got != 3 {
		t.Fatalf("nota avg: got %v want 3", got)
	}
}

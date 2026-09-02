package queryeng

import (
	"context"
	"fmt"
	"testing"
)

func TestDashboardSlicerFilter(t *testing.T) {
	rows := []map[string]any{
		{"region": "Norte", "product": "A", "sales": 100.0},
		{"region": "Sul", "product": "B", "sales": 200.0},
		{"region": "Norte", "product": "C", "sales": 150.0},
	}
	loader := func(ctx context.Context, limit int) ([]string, []map[string]any, error) {
		return []string{"region", "product", "sales"}, rows, nil
	}
	exec := NewDuckDBExecutor(loader)
	// Widget query with a global filter from a slicer: region = Norte
	sql := "SELECT `product`, SUM(`sales`) AS `TotalSales` FROM rows WHERE `region` = 'Norte' GROUP BY `product`"
	res, err := exec.Execute(context.Background(), sql, 100)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("expected 2 rows after slicer filter, got %d", len(res.Rows))
	}
	for _, r := range res.Rows {
		if r["region"] != nil && r["region"] != "Norte" {
			t.Fatalf("unexpected region: %v", r["region"])
		}
	}
}

func TestDrillDownHierarchy(t *testing.T) {
	rows := []map[string]any{
		{"year": "2024", "quarter": "Q1", "month": "Jan", "sales": 100.0},
		{"year": "2024", "quarter": "Q1", "month": "Feb", "sales": 120.0},
		{"year": "2024", "quarter": "Q2", "month": "Apr", "sales": 90.0},
		{"year": "2023", "quarter": "Q4", "month": "Dec", "sales": 200.0},
	}
	loader := func(ctx context.Context, limit int) ([]string, []map[string]any, error) {
		return []string{"year", "quarter", "month", "sales"}, rows, nil
	}
	exec := NewDuckDBExecutor(loader)

	// Level 1: group by year
	sql1 := "SELECT `year`, SUM(`sales`) AS `TotalSales` FROM rows GROUP BY `year` ORDER BY `year` ASC"
	res1, err := exec.Execute(context.Background(), sql1, 100)
	if err != nil {
		t.Fatalf("level1 execute: %v", err)
	}
	if len(res1.Rows) != 2 {
		t.Fatalf("expected 2 years, got %d", len(res1.Rows))
	}

	// Level 2: drill into 2024, group by quarter
	sql2 := "SELECT `quarter`, SUM(`sales`) AS `TotalSales` FROM rows WHERE `year` = '2024' GROUP BY `quarter` ORDER BY `quarter` ASC"
	res2, err := exec.Execute(context.Background(), sql2, 100)
	if err != nil {
		t.Fatalf("level2 execute: %v", err)
	}
	if len(res2.Rows) != 2 {
		t.Fatalf("expected 2 quarters for 2024, got %d", len(res2.Rows))
	}
	for _, r := range res2.Rows {
		if fmt.Sprint(r["quarter"]) != "Q1" && fmt.Sprint(r["quarter"]) != "Q2" {
			t.Fatalf("unexpected quarter: %v", r["quarter"])
		}
	}

	// Level 3: drill into Q1, group by month
	sql3 := "SELECT `month`, SUM(`sales`) AS `TotalSales` FROM rows WHERE `year` = '2024' AND `quarter` = 'Q1' GROUP BY `month` ORDER BY `month` ASC"
	res3, err := exec.Execute(context.Background(), sql3, 100)
	if err != nil {
		t.Fatalf("level3 execute: %v", err)
	}
	if len(res3.Rows) != 2 {
		t.Fatalf("expected 2 months for Q1, got %d", len(res3.Rows))
	}
}

func TestCrossFilterSimulation(t *testing.T) {
	rows := []map[string]any{
		{"region": "Norte", "product": "A", "sales": 100.0},
		{"region": "Sul", "product": "B", "sales": 200.0},
		{"region": "Norte", "product": "C", "sales": 150.0},
	}
	loader := func(ctx context.Context, limit int) ([]string, []map[string]any, error) {
		return []string{"region", "product", "sales"}, rows, nil
	}
	exec := NewDuckDBExecutor(loader)

	// Simulate user clicking on a bar chart segment for region=Norte. The cross-filter
	// is applied to a second widget (e.g. product table).
	sql := "SELECT `product`, SUM(`sales`) AS `TotalSales` FROM rows WHERE `region` = 'Norte' GROUP BY `product`"
	res, err := exec.Execute(context.Background(), sql, 100)
	if err != nil {
		t.Fatalf("cross-filter execute: %v", err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("expected 2 products after cross-filter, got %d", len(res.Rows))
	}
	var total float64
	for _, r := range res.Rows {
		total += toF(r["TotalSales"])
	}
	if total != 250.0 {
		t.Fatalf("expected total sales 250 after cross-filter, got %f", total)
	}
}

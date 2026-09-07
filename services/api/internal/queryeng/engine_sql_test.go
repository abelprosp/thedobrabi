package queryeng

import (
	"strings"
	"testing"

	"github.com/thedobra/thedobra/services/api/internal/semantic"
)

func TestQualifyIdentExpr(t *testing.T) {
	got := qualifyIdentExpr("SUM(`valor`)", "a")
	want := "SUM(a.`valor`)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got = qualifyIdentExpr("SUM(`valor`) / NULLIF(SUM(`qtd`), 0)", "b")
	want = "SUM(b.`valor`) / NULLIF(SUM(b.`qtd`), 0)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTimeFilterIncludesEndDate(t *testing.T) {
	w := timeFilterSQL("`data_venda`", "2026-07-01", "2026-08-31")
	if len(w) != 2 {
		t.Fatalf("want 2 clauses, got %v", w)
	}
	if !strings.Contains(w[1], "INTERVAL 1 DAY") {
		t.Fatalf("end date should be inclusive via +1 day, got %q", w[1])
	}
}

func TestDimensionExprLeavesDateColumn(t *testing.T) {
	d := semantic.Dimension{Name: "Data", Column: "data_venda", Type: "date"}
	got := dimensionExpr(d, "data_venda", "a.`data_venda`")
	if got != "a.`data_venda`" {
		t.Fatalf("date columns should group as-is, got %s", got)
	}
}

func TestDimensionExprTruncatesDateTime(t *testing.T) {
	d := semantic.Dimension{Name: "Data", Column: "data_venda", Type: "datetime"}
	got := dimensionExpr(d, "data_venda", "a.`data_venda`")
	if !strings.Contains(got, "toDate(") || !strings.Contains(got, "a.`data_venda`") {
		t.Fatalf("expected date truncation of table-qualified datetime, got %s", got)
	}
}

func TestSQLOutAlias(t *testing.T) {
	if got := sqlOutAlias("join.salario", "salario"); got != "join_salario" {
		t.Fatalf("join field: %q", got)
	}
	if got := sqlOutAlias("join.1.regiao", "regiao"); got != "join1_regiao" {
		t.Fatalf("second join field: %q", got)
	}
	if got := sqlOutAlias("valor", "valor"); got != "valor" {
		t.Fatalf("local field: %q", got)
	}
}

func TestParseJoinRef(t *testing.T) {
	idx, raw, ok := parseJoinRef("join.salario")
	if !ok || idx != 0 || raw != "salario" {
		t.Fatalf("join.salario → %d %q %v", idx, raw, ok)
	}
	idx, raw, ok = parseJoinRef("join.0.salario")
	if !ok || idx != 0 || raw != "salario" {
		t.Fatalf("join.0.salario → %d %q %v", idx, raw, ok)
	}
	idx, raw, ok = parseJoinRef("join.2.regiao")
	if !ok || idx != 2 || raw != "regiao" {
		t.Fatalf("join.2.regiao → %d %q %v", idx, raw, ok)
	}
	idx, raw, ok = parseJoinRef("valor")
	if ok || idx != -1 || raw != "valor" {
		t.Fatalf("valor → %d %q %v", idx, raw, ok)
	}
}

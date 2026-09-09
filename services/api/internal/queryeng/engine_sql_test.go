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

func TestLooksLikeTimeName(t *testing.T) {
	for _, ok := range []string{"mes", "mês", "data_venda", "ano", "TOMONTH", "year_month"} {
		d := semantic.Dimension{Name: ok, Column: ok}
		if ok == "TOMONTH" {
			d = semantic.Dimension{Name: "Grupo", Expression: "TOMONTH(data_venda)"}
		}
		if !isTimeDimension(d, "data_venda") {
			t.Fatalf("expected time dimension: %+v", d)
		}
	}
	if isTimeDimension(semantic.Dimension{Name: "Empresa", Column: "empresa"}, "data_venda") {
		t.Fatal("empresa should not be a time dimension")
	}
}

func TestRequestToSimpleSQLOrdersTimeDimension(t *testing.T) {
	meta := datasetInfo{
		Name:  "vendas",
		Table: "vendas",
		Model: semantic.Model{
			TimeColumn: "data_venda",
			Dimensions: []semantic.Dimension{{Name: "Mês", Column: "mes"}},
			Measures:   []semantic.Measure{{Name: "Receita", Expression: "SUM(valor)"}},
		},
	}
	sql, _ := requestToSimpleSQL(meta, Request{Dimensions: []string{"mes"}, Measures: []string{"Receita"}, Limit: 50})
	if !strings.Contains(sql, "ORDER BY `mes` DESC") {
		t.Fatalf("expected chronological time order, got %s", sql)
	}
	if strings.Contains(sql, "ORDER BY `Receita`") {
		t.Fatalf("time charts should not order by the metric, got %s", sql)
	}

	company, _ := requestToSimpleSQL(meta, Request{
		Dimensions: []string{"empresa"},
		Measures:   []string{"Receita"},
		Limit:      50,
	})
	if strings.Contains(company, "ORDER BY `mes`") {
		t.Fatalf("company dimension should not order by month, got %s", company)
	}
}

func TestTimeDimensionOrderField(t *testing.T) {
	model := semantic.Model{
		TimeColumn: "data_venda",
		Dimensions: []semantic.Dimension{{Name: "Mês", Column: "mes"}, {Name: "Empresa", Column: "empresa"}},
	}
	got := timeDimensionOrderField(Request{Dimensions: []string{"mes"}, Measures: []string{"Receita"}}, model)
	if got == "" {
		t.Fatal("expected order field for month")
	}
	if timeDimensionOrderField(Request{Dimensions: []string{"empresa"}, Measures: []string{"Receita"}}, model) != "" {
		t.Fatal("company chart should not switch to time order")
	}
}

func TestCompileDimensionSQLCase(t *testing.T) {
	d := semantic.Dimension{Name: "Grupo", Expression: "CASE WHEN empresa = 'VIVO' THEN 'Telecom' ELSE 'Outros' END"}
	got, err := compileDimensionSQL(d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "CASE") || !strings.Contains(got, "`empresa`") {
		t.Fatalf("unexpected sql %s", got)
	}
}

func TestCompileDimensionSQLRejectsAggregate(t *testing.T) {
	d := semantic.Dimension{Name: "Total", Expression: "SUM(valor)"}
	if _, err := compileDimensionSQL(d); err == nil {
		t.Fatal("expected aggregate dimension to fail")
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

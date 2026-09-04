package semanticxpr

import (
	"strings"
	"testing"
)

func q(s string) string { return "`" + s + "`" }

func TestParseBasicAggregation(t *testing.T) {
	expr, err := Parse("SUM(revenue)")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if expr.Func != "SUM" {
		t.Fatalf("expected SUM, got %s", expr.Func)
	}
	sql, err := expr.ToSQL(q)
	if err != nil {
		t.Fatalf("sql: %v", err)
	}
	if sql != "SUM(toFloat64OrZero(toString(`revenue`)))" {
		t.Fatalf("unexpected sql: %s", sql)
	}
}

func TestParseBracketMeasure(t *testing.T) {
	expr, err := Parse("[Receita]")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if expr.Func != "MEASURE" || expr.Column != "Receita" {
		t.Fatalf("unexpected expr: %+v", expr)
	}
}

func TestDependentMeasure(t *testing.T) {
	expr, err := Parse("[Receita] - [Custo]")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if expr.Func != "OP" || expr.Op != "-" {
		t.Fatalf("expected OP -, got %+v", expr)
	}
	resolve := func(name string) (Expr, error) {
		switch name {
		case "Receita":
			return Parse("SUM(revenue)")
		case "Custo":
			return Parse("SUM(cost)")
		}
		return Expr{}, nil
	}
	sql, err := expr.ToSQLWithResolver(q, resolve)
	if err != nil {
		t.Fatalf("sql: %v", err)
	}
	want := "(SUM(toFloat64OrZero(toString(`revenue`))) - SUM(toFloat64OrZero(toString(`cost`))))"
	if sql != want {
		t.Fatalf("expected %s, got %s", want, sql)
	}
}

func TestCalculatePredicate(t *testing.T) {
	expr, err := Parse("CALCULATE([Receita], Regiao = 'Norte')")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	resolve := func(name string) (Expr, error) {
		if name == "Receita" {
			return Parse("SUM(revenue)")
		}
		return Expr{}, nil
	}
	ctx := evalContext{measures: map[string]Expr{}}
	if err := ctx.collectDependencies(expr, resolve); err != nil {
		t.Fatalf("collect: %v", err)
	}
	sql, err := expr.toSQLWithContext(q, &ctx)
	if err != nil {
		t.Fatalf("sql: %v", err)
	}
	if sql != "sumIf(toFloat64OrZero(toString(`revenue`)), `Regiao` = 'Norte')" {
		t.Fatalf("unexpected sql: %s", sql)
	}
	if len(ctx.filterMods) != 1 || ctx.filterMods[0] != "`Regiao` = 'Norte'" {
		t.Fatalf("unexpected filter mods: %v", ctx.filterMods)
	}
}

func TestRejectBadIdentifier(t *testing.T) {
	_, err := Parse("SUM(foo; drop table)")
	if err == nil {
		t.Fatalf("expected error for invalid identifier")
	}
}

func TestYoYUsesTimeColumn(t *testing.T) {
	expr, err := Parse("YOY(valor_mensal)")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = expr.ToSQL(q)
	if err == nil {
		t.Fatal("expected error without time column")
	}
	sql, err := expr.ToSQLWithOptions(q, nil, SQLOptions{TimeColumn: "data_venda", RangeStart: "2026-07-01", RangeEnd: "2026-08-31"})
	if err != nil {
		t.Fatalf("sql: %v", err)
	}
	if !strings.Contains(sql, "sumIf") || !strings.Contains(sql, "addYears") || !strings.Contains(sql, "`data_venda`") {
		t.Fatalf("unexpected yoy sql: %s", sql)
	}
}

func TestCaseWhenAnd(t *testing.T) {
	expr, err := Parse("SUM(CASE WHEN mes = '2026-07' AND ano = 2026 THEN valor_mensal ELSE 0 END)")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sql, err := expr.ToSQL(q)
	if err != nil {
		t.Fatalf("sql: %v", err)
	}
	if !strings.Contains(sql, "AND") || !strings.Contains(sql, "`ano` = 2026") {
		t.Fatalf("unexpected sql: %s", sql)
	}
}

func TestDivideAndTomonth(t *testing.T) {
	expr, err := Parse("DIVIDE(SUM(valor_mensal), COUNT(DISTINCT cliente))")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sql, err := expr.ToSQL(q)
	if err != nil {
		t.Fatalf("sql: %v", err)
	}
	if !strings.Contains(sql, "divide") || !strings.Contains(sql, "uniqExact") {
		t.Fatalf("unexpected sql: %s", sql)
	}
	tm, err := Parse("SUM(CASE WHEN TOMONTH(data_venda) = '2026-07' THEN valor_mensal ELSE 0 END)")
	if err != nil {
		t.Fatalf("parse tomonth: %v", err)
	}
	sql, err = tm.ToSQL(q)
	if err != nil {
		t.Fatalf("sql tomonth: %v", err)
	}
	if !strings.Contains(sql, "%Y-%m") && !strings.Contains(sql, "formatDateTime") {
		t.Fatalf("unexpected tomonth sql: %s", sql)
	}
}

func TestRelated(t *testing.T) {
	expr, err := Parse("RELATED(DimRegiao[Regiao])")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sql, err := expr.ToSQL(q)
	if err != nil {
		t.Fatalf("sql: %v", err)
	}
	want := "(SELECT `Regiao` FROM DimRegiao AS r WHERE r._tenant = _tenant LIMIT 1)"
	if sql != want {
		t.Fatalf("expected %s, got %s", want, sql)
	}
}

func TestCaseWhenSum(t *testing.T) {
	expr, err := Parse("SUM(CASE WHEN mes = '2026-08' THEN valor_mensal ELSE 0 END)")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sql, err := expr.ToSQL(q)
	if err != nil {
		t.Fatalf("sql: %v", err)
	}
	want := "SUM(CASE WHEN `mes` = '2026-08' THEN `valor_mensal` ELSE 0 END)"
	if sql != want {
		t.Fatalf("expected %s, got %s", want, sql)
	}
}

func TestVariacaoReceita(t *testing.T) {
	src := `(SUM(CASE WHEN mes = '2026-08' THEN valor_mensal ELSE 0 END) - SUM(CASE WHEN mes = '2026-07' THEN valor_mensal ELSE 0 END)) / NULLIF(SUM(CASE WHEN mes = '2026-07' THEN valor_mensal ELSE 0 END), 0) * 100`
	expr, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sql, err := expr.ToSQL(q)
	if err != nil {
		t.Fatalf("sql: %v", err)
	}
	if !strings.Contains(sql, "NULLIF") || !strings.Contains(sql, "`mes` = '2026-08'") {
		t.Fatalf("unexpected sql: %s", sql)
	}
}

func TestRejectSelectFrom(t *testing.T) {
	_, err := Parse("SUM(valor_mensal) FROM contratos")
	if err == nil {
		t.Fatalf("expected error for SELECT/FROM")
	}
}

func TestCountDistinctCaseWhen(t *testing.T) {
	expr, err := Parse("COUNT(DISTINCT CASE WHEN mes = '2026-07' THEN cliente END)")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sql, err := expr.ToSQL(q)
	if err != nil {
		t.Fatalf("sql: %v", err)
	}
	want := "uniqExact(CASE WHEN `mes` = '2026-07' THEN `cliente` END)"
	if sql != want {
		t.Fatalf("expected %s, got %s", want, sql)
	}
}

func TestCountStar(t *testing.T) {
	expr, err := Parse("COUNT(*)")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sql, err := expr.ToSQL(q)
	if err != nil {
		t.Fatalf("sql: %v", err)
	}
	if sql != "COUNT(*)" {
		t.Fatalf("unexpected sql: %s", sql)
	}
}

func TestCountDistinct(t *testing.T) {
	expr, err := Parse("COUNT(DISTINCT cliente)")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sql, err := expr.ToSQL(q)
	if err != nil {
		t.Fatalf("sql: %v", err)
	}
	if sql != "uniqExact(`cliente`)" {
		t.Fatalf("unexpected sql: %s", sql)
	}
}

func TestEvaluateDependent(t *testing.T) {
	expr, err := Parse("[Margem]")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	resolve := func(name string) (Expr, error) {
		if name == "Margem" {
			return Parse("[Receita] - [Custo]")
		}
		if name == "Receita" {
			return Parse("SUM(revenue)")
		}
		if name == "Custo" {
			return Parse("SUM(cost)")
		}
		return Expr{}, nil
	}
	rows := []map[string]any{
		{"revenue": 100.0, "cost": 40.0},
		{"revenue": 200.0, "cost": 80.0},
	}
	val, err := Evaluate(expr, rows, resolve)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if val != 180.0 {
		t.Fatalf("expected 180, got %f", val)
	}
}

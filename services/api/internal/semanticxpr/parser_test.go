package semanticxpr

import (
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
	if sql != "SUM(`revenue`)" {
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
	want := "(SUM(`revenue`) - SUM(`cost`))"
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
	if sql != "SUM(`revenue`)" {
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

func TestTimeFunctions(t *testing.T) {
	for _, fn := range []string{"SAMEPERIODLASTYEAR", "TOTALYTD", "TOTALMTD", "TOTALQTD"} {
		expr, err := Parse(fn + "(revenue)")
		if err != nil {
			t.Fatalf("parse %s: %v", fn, err)
		}
		if expr.Func != fn {
			t.Fatalf("expected %s, got %s", fn, expr.Func)
		}
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

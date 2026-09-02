package queryeng

import "testing"

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

func TestSQLOutAlias(t *testing.T) {
	if got := sqlOutAlias("join.salario", "salario"); got != "join_salario" {
		t.Fatalf("join field: %q", got)
	}
	if got := sqlOutAlias("valor", "valor"); got != "valor" {
		t.Fatalf("local field: %q", got)
	}
}

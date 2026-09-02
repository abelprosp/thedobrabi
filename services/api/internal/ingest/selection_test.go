package ingest

import "testing"

func TestHumanizeIdent(t *testing.T) {
	cases := map[string]string{
		"public.fct_sales": "Vendas",
		"customers":        "Clientes",
		"customer_id":      "Cliente código",
		"orders":           "Pedidos",
		"created_at":       "Criado em",
		"dim_product":      "Produto",
	}
	for in, want := range cases {
		if got := HumanizeIdent(in); got != want {
			t.Fatalf("%s: got %q want %q", in, got, want)
		}
	}
}

func TestBuildSelectionSQLSingleTable(t *testing.T) {
	q, err := buildSelectionSQL("postgres", SourceSelection{
		Tables: []SelectedTable{{Schema: "public", Name: "orders", Columns: []string{"id", "total"}}},
	}, 500)
	if err != nil {
		t.Fatal(err)
	}
	want := `SELECT "orders"."id" AS "id", "orders"."total" AS "total" FROM "public"."orders" AS "orders" LIMIT 500`
	if q != want {
		t.Fatalf("got %s", q)
	}
}

func TestBuildSelectionSQLJoin(t *testing.T) {
	q, err := buildSelectionSQL("postgres", SourceSelection{
		Tables: []SelectedTable{
			{Schema: "public", Name: "orders", Columns: []string{"id", "customer_id", "total"}},
			{Schema: "public", Name: "customers", Columns: []string{"id", "name"}},
		},
		Joins: []SelectedJoin{{
			LeftTable: "public.orders", LeftColumn: "customer_id",
			RightTable: "public.customers", RightColumn: "id",
			Match: "both",
		}},
	}, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(q, []string{
		`INNER JOIN "public"."customers" AS "customers"`,
		`"orders"."customer_id" = "customers"."id"`,
		`AS "orders_id"`,
		`AS "customers_name"`,
		`LIMIT 1000`,
	}) {
		t.Fatalf("unexpected SQL: %s", q)
	}
}

func TestBuildSelectionSQLLeftJoinAndSQLServer(t *testing.T) {
	q, err := buildSelectionSQL("sqlserver", SourceSelection{
		Tables: []SelectedTable{
			{Schema: "dbo", Name: "orders", Columns: []string{"id"}},
			{Schema: "dbo", Name: "customers", Columns: []string{"id"}},
		},
		Joins: []SelectedJoin{{
			LeftTable: "dbo.orders", LeftColumn: "customer_id",
			RightTable: "dbo.customers", RightColumn: "id",
			Match: "all_left",
		}},
	}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(q, []string{"SELECT TOP (10)", "LEFT JOIN", "[dbo].[orders]", "[customers]"}) {
		t.Fatalf("unexpected SQL: %s", q)
	}
}

func TestBuildSelectionSQLRejectsInjection(t *testing.T) {
	_, err := buildSelectionSQL("postgres", SourceSelection{
		Tables: []SelectedTable{{Name: "orders;drop", Columns: []string{"id"}}},
	}, 10)
	if err == nil {
		t.Fatal("expected invalid ident")
	}
	_, err = buildSelectionSQL("mysql", SourceSelection{
		Tables: []SelectedTable{{Name: "orders", Columns: []string{"id;drop"}}},
	}, 10)
	if err == nil {
		t.Fatal("expected invalid column")
	}
}

func TestBuildSelectionSQLNoJoinsMulti(t *testing.T) {
	_, err := buildSelectionSQL("postgres", SourceSelection{
		Tables: []SelectedTable{
			{Name: "a", Columns: []string{"id"}},
			{Name: "b", Columns: []string{"id"}},
		},
	}, 10)
	if err == nil {
		t.Fatal("expected multi-without-join to fail at SQL build")
	}
}

func TestApplySelectionNoop(t *testing.T) {
	cfg := SQLConfig{Table: "orders", Query: "SELECT 1"}
	got, err := applySelection("postgres", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.Query != "SELECT 1" || got.Table != "orders" {
		t.Fatalf("%+v", got)
	}
}

func TestJoinInMemory(t *testing.T) {
	headers, rows, err := joinInMemory(
		[]string{"id", "customer_id"},
		[][]string{{"1", "10"}, {"2", "99"}},
		"customer_id",
		[]string{"id", "name"},
		[][]string{{"10", "Ana"}},
		"id",
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 4 || len(rows) != 1 || rows[0][3] != "Ana" {
		t.Fatalf("headers=%v rows=%v", headers, rows)
	}
	_, leftRows, err := joinInMemory(
		[]string{"id", "customer_id"},
		[][]string{{"1", "10"}, {"2", "99"}},
		"customer_id",
		[]string{"id", "name"},
		[][]string{{"10", "Ana"}},
		"id",
		true,
	)
	if err != nil || len(leftRows) != 2 {
		t.Fatalf("left join rows=%d err=%v", len(leftRows), err)
	}
}

func TestExtractOpenAPIColumns(t *testing.T) {
	raw := []byte(`{"definitions":{"users":{"properties":{"id":{"type":"integer"},"email":{"type":"string"}}}}}`)
	got := extractOpenAPIColumns(raw)
	if len(got["users"]) != 2 {
		t.Fatalf("%+v", got)
	}
}

func containsAll(s string, parts []string) bool {
	for _, p := range parts {
		if !containsStr(s, p) {
			return false
		}
	}
	return true
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

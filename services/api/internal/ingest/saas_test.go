package ingest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchAsaasCustomers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("access_token") != "key-test" {
			http.Error(w, "no auth", 401)
			return
		}
		if r.URL.Path != "/customers" {
			http.Error(w, r.URL.Path, 404)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"hasMore": false,
			"data": []map[string]any{
				{"id": "cus_1", "name": "Ana"},
				{"id": "cus_2", "name": "Bruno"},
			},
		})
	}))
	defer srv.Close()
	e := &Engine{}
	h, rows, err := e.fetchAsaas(t.Context(), SQLConfig{APIKey: "key-test", URL: srv.URL, Limit: 50}, "customers")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !contains(h, "name") {
		t.Fatalf("headers=%v rows=%v", h, rows)
	}
}

func TestFetchOmieClientes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["app_key"] != "k" || body["call"] != "ListarClientes" {
			http.Error(w, "bad body", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"clientes_cadastro": []map[string]any{{"codigo_cliente_omie": 1, "nome_fantasia": "Loja"}},
		})
	}))
	defer srv.Close()
	e := &Engine{}
	h, rows, err := e.fetchOmie(t.Context(), SQLConfig{AppKey: "k", AppSecret: "s", URL: srv.URL, Limit: 10}, "clientes")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !contains(h, "nome_fantasia") {
		t.Fatalf("headers=%v rows=%v", h, rows)
	}
}

func TestPickJSONArrayMeta(t *testing.T) {
	raw := []byte(`{"data":[{"campaign_name":"Camp A","impressions":"10"}]}`)
	maps, err := pickJSONArray(raw, "data")
	if err != nil || len(maps) != 1 {
		t.Fatalf("%v %v", maps, err)
	}
}

func TestParseODataPageNextLink(t *testing.T) {
	raw := []byte(`{"value":[{"id":1},{"id":2}],"@odata.nextLink":"https://example.com/next"}`)
	page, next, err := parseODataPage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || next != "https://example.com/next" {
		t.Fatalf("page=%v next=%s", page, next)
	}
}

func TestSelectStarSQL(t *testing.T) {
	if got := selectStarSQL("sqlserver", "dbo.Vendas", 10); got != "SELECT TOP (10) * FROM dbo.Vendas" {
		t.Fatal(got)
	}
	if got := selectStarSQL("postgres", "public.t", 5); got != "SELECT * FROM public.t LIMIT 5" {
		t.Fatal(got)
	}
}

func TestDetectODBCType(t *testing.T) {
	if detectODBCType(SQLConfig{URL: "mysql://u:p@h/db"}) != "mysql" {
		t.Fatal("mysql")
	}
	if detectODBCType(SQLConfig{URL: "postgres://localhost/db"}) != "postgres" {
		t.Fatal("postgres")
	}
	if detectODBCType(SQLConfig{URL: "sqlserver://h:1433"}) != "sqlserver" {
		t.Fatal("sqlserver")
	}
}

package ingest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSupabaseDBHostFromProject(t *testing.T) {
	got := supabaseDBHostFromProject("https://abcd1234.supabase.co")
	if got != "db.abcd1234.supabase.co" {
		t.Fatalf("host=%q", got)
	}
	if supabaseDBHostFromProject("https://db.abcd1234.supabase.co") != "db.abcd1234.supabase.co" {
		t.Fatal("já prefixado")
	}
}

func TestSupabaseUsePostgresVsREST(t *testing.T) {
	pg := SQLConfig{Host: "db.xxxx.supabase.co", Password: "secret"}
	if !supabaseUsePostgres(pg) {
		t.Fatal("host+password deveria usar Postgres")
	}
	rest := SQLConfig{ProjectURL: "https://xxxx.supabase.co", ServiceRoleKey: "eyJ"}
	if supabaseUsePostgres(rest) || !supabaseUseREST(rest) {
		t.Fatal("só URL+key deveria usar REST")
	}
	both := SQLConfig{Host: "db.xxxx.supabase.co", Password: "secret", ProjectURL: "https://xxxx.supabase.co", ServiceRoleKey: "eyJ"}
	if !supabaseUsePostgres(both) {
		t.Fatal("com host+password a preferência é Postgres")
	}
	derived := SQLConfig{ProjectURL: "https://xxxx.supabase.co", Password: "secret"}
	if !supabaseUsePostgres(derived) {
		t.Fatal("URL do projecto + senha deveria derivar o anfitrião Postgres")
	}
}

func TestParsePostgRESTOpenAPI(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"paths": map[string]any{
			"/users":        map[string]any{"get": map[string]any{}},
			"/orders":       map[string]any{"get": map[string]any{}},
			"/rpc/do_thing": map[string]any{"post": map[string]any{}},
			"/{table_name}": map[string]any{"get": map[string]any{}},
			"/nested/path":  map[string]any{"get": map[string]any{}},
		},
	})
	got := parsePostgRESTOpenAPI(raw)
	if len(got) != 2 || got[0] != "orders" || got[1] != "users" {
		t.Fatalf("tables=%v", got)
	}
}

func TestFetchSupabaseREST(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("apikey") != "sr-key" || r.Header.Get("Authorization") != "Bearer sr-key" {
			http.Error(w, "no auth", 401)
			return
		}
		if r.URL.Path != "/rest/v1/users" {
			http.Error(w, r.URL.Path, 404)
			return
		}
		if r.URL.Query().Get("select") != "*" {
			http.Error(w, "select", 400)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "email": "ana@ex.com"},
			{"id": 2, "email": "bruno@ex.com"},
		})
	}))
	defer srv.Close()
	e := &Engine{}
	h, rows, err := e.fetchSupabase(t.Context(), SQLConfig{
		ProjectURL:     srv.URL,
		ServiceRoleKey: "sr-key",
		Table:          "public.users",
		Limit:          500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !contains(h, "email") {
		t.Fatalf("headers=%v rows=%v", h, rows)
	}
}

func TestDiscoverSupabaseRESTOpenAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/v1/" && r.URL.Path != "/rest/v1" {
			http.Error(w, r.URL.Path, 404)
			return
		}
		if r.Header.Get("apikey") == "" {
			http.Error(w, "no key", 401)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"paths": map[string]any{"/clientes": map[string]any{"get": map[string]any{}}},
		})
	}))
	defer srv.Close()
	e := &Engine{}
	tables, err := e.discoverSupabase(t.Context(), SQLConfig{ProjectURL: srv.URL, AnonKey: "anon"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 || tables[0] != "clientes" {
		t.Fatalf("tables=%v", tables)
	}
}

func TestPingSupabaseREST(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			http.Error(w, "no auth", 401)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"swagger":"2.0","paths":{}}`))
	}))
	defer srv.Close()
	e := &Engine{}
	if err := e.pingSupabase(t.Context(), SQLConfig{ProjectURL: srv.URL, ServiceRoleKey: "k"}); err != nil {
		t.Fatal(err)
	}
}

func TestSupabasePreparedSQLDefaults(t *testing.T) {
	cfg := supabasePreparedSQL(SQLConfig{ProjectURL: "https://abc.supabase.co", Password: "p"})
	if cfg.Host != "db.abc.supabase.co" || cfg.Port != 5432 || cfg.Database != "postgres" || cfg.User != "postgres" || cfg.EffectiveSSLMode() != "require" {
		t.Fatalf("%+v ssl=%s", cfg, cfg.EffectiveSSLMode())
	}
}

func TestSupabaseRESTTableStripSchema(t *testing.T) {
	if supabaseRESTTable("public.vendas") != "vendas" {
		t.Fatal(supabaseRESTTable("public.vendas"))
	}
}

package ingest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseGoogleSheetRef(t *testing.T) {
	cases := []struct {
		url, table, a1 string
		id, gid, title string
		pub            bool
	}{
		{
			url: "https://docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit#gid=0",
			id:  "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms", gid: "0",
		},
		{
			url: "https://docs.google.com/spreadsheets/d/abcDEF-_0123456789/edit?usp=sharing&gid=123#gid=123",
			id:  "abcDEF-_0123456789", gid: "123",
		},
		{
			url:   "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
			table: "Vendas",
			id:    "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms", title: "Vendas",
		},
		{
			url:   "https://docs.google.com/spreadsheets/d/1abcDEFGHIJ0123456789/edit",
			table: "42",
			id:    "1abcDEFGHIJ0123456789", gid: "42",
		},
		{
			url: "https://docs.google.com/spreadsheets/d/e/2PACX-1vPubSheetID01234567890/pubhtml",
			id:  "2PACX-1vPubSheetID01234567890", pub: true,
		},
	}
	for _, c := range cases {
		got, err := parseGoogleSheetRef(c.url, c.table, c.a1)
		if err != nil {
			t.Fatalf("%s: %v", c.url, err)
		}
		if got.ID != c.id || got.GID != c.gid || got.Title != c.title || got.Published != c.pub {
			t.Fatalf("%s: %+v want id=%s gid=%s title=%s pub=%v", c.url, got, c.id, c.gid, c.title, c.pub)
		}
	}
	if _, err := parseGoogleSheetRef("", "", ""); err == nil {
		t.Fatal("URL vazio deveria falhar")
	}
}

func TestGoogleSheetsA1(t *testing.T) {
	if googleSheetsA1("Página1", "") != "'Página1'" {
		t.Fatalf("quoted: %q", googleSheetsA1("Página1", ""))
	}
	if googleSheetsA1("Sheet1", "A1:C10") != "Sheet1!A1:C10" {
		t.Fatal("plain range")
	}
	if googleSheetsA1("A B", "Sheet1!A1:B") != "Sheet1!A1:B" {
		t.Fatal("already full range")
	}
}

func TestSheetsValuesToTable(t *testing.T) {
	h, rows, err := sheetsValuesToTable([][]any{
		{"Nome", "Idade", ""},
		{"Ana", float64(30), "x"},
		{"", "", ""},
		{"Bruno", float64(18.5)},
	}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 3 || h[2] != "col_3" {
		t.Fatalf("headers=%v", h)
	}
	if len(rows) != 2 || rows[0][1] != "30" || rows[1][1] != "18.5" {
		t.Fatalf("rows=%v", rows)
	}
}

func TestFetchGoogleSheetsAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/spreadsheets/abc1234567", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "no auth", 401)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sheets": []map[string]any{
				{"properties": map[string]any{"sheetId": 0, "title": "Vendas"}},
			},
		})
	})
	mux.HandleFunc("/spreadsheets/abc1234567/values/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "Vendas") {
			http.Error(w, r.URL.Path, 404)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"values": [][]any{{"produto", "qtd"}, {"café", 2}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	prev := googleSheetsAPIBase
	googleSheetsAPIBase = srv.URL
	defer func() { googleSheetsAPIBase = prev }()

	e := &Engine{}
	h, rows, err := e.fetchGoogleSheets(t.Context(), SQLConfig{
		URL: "abc1234567", Token: "tok", Limit: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0][0] != "café" || !contains(h, "produto") {
		t.Fatalf("headers=%v rows=%v", h, rows)
	}
}

func TestFetchGoogleSheetsPublicCSV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte("mes,valor\njan,10\nfev,20\n"))
	}))
	defer srv.Close()
	prev := googleSheetsDocsBase
	googleSheetsDocsBase = srv.URL
	defer func() { googleSheetsDocsBase = prev }()

	e := &Engine{}
	h, rows, err := e.fetchGoogleSheets(t.Context(), SQLConfig{
		URL: "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms", Limit: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1][0] != "fev" || !contains(h, "valor") {
		t.Fatalf("headers=%v rows=%v", h, rows)
	}
}

func TestFetchGoogleSheetsPublicHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><html><body>login</body></html>"))
	}))
	defer srv.Close()
	prev := googleSheetsDocsBase
	googleSheetsDocsBase = srv.URL
	defer func() { googleSheetsDocsBase = prev }()

	e := &Engine{}
	_, _, err := e.fetchGoogleSheets(t.Context(), SQLConfig{
		URL: "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms",
	})
	if err == nil || !strings.Contains(err.Error(), "pública") {
		t.Fatalf("esperava erro de planilha pública, got %v", err)
	}
}

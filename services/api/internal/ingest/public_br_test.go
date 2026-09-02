package ingest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchIBGEMunicipios(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 3550308, "nome": "São Paulo", "microrregiao": map[string]any{"id": 1, "nome": "São Paulo"}},
			{"id": 3304557, "nome": "Rio de Janeiro"},
		})
	}))
	defer srv.Close()
	e := &Engine{}
	h, rows, err := e.fetchIBGE(t.Context(), SQLConfig{URL: srv.URL, Limit: 50}, "municipios")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !contains(h, "nome") {
		t.Fatalf("headers=%v rows=%d", h, len(rows))
	}
}

func TestFetchIBGESIDRA(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"id": "6579",
			"resultados": []any{map[string]any{
				"series": []any{map[string]any{
					"localidade": map[string]any{"id": "35", "nome": "São Paulo"},
					"serie":      map[string]any{"2022": "44420459"},
				}},
			}},
		}})
	}))
	defer srv.Close()
	e := &Engine{}
	h, rows, err := e.fetchIBGE(t.Context(), SQLConfig{URL: srv.URL, Limit: 10}, "populacao")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !contains(h, "valor") {
		t.Fatalf("headers=%v rows=%v", h, rows)
	}
}

func TestFetchInflacaoSGS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"data": "01/01/2024", "valor": "0.42"},
			{"data": "01/02/2024", "valor": "0.83"},
		})
	}))
	defer srv.Close()
	e := &Engine{}
	h, rows, err := e.fetchInflacao(t.Context(), SQLConfig{URL: srv.URL, Series: "433", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !contains(h, "valor") {
		t.Fatalf("headers=%v rows=%v", h, rows)
	}
}

func TestFetchExpectativas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{
				{"Indicador": "IPCA", "Data": "2024-01-01", "Mediana": 3.5},
			},
		})
	}))
	defer srv.Close()
	e := &Engine{}
	h, rows, err := e.fetchExpectativas(t.Context(), SQLConfig{URL: srv.URL, Limit: 10}, "anuais")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !contains(h, "Indicador") {
		t.Fatalf("headers=%v rows=%v", h, rows)
	}
}

func TestFetchCambioAwesomeAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"USDBRL": map[string]any{"code": "USD", "bid": "5.40", "ask": "5.41"},
			"EURBRL": map[string]any{"code": "EUR", "bid": "5.90", "ask": "5.91"},
		})
	}))
	defer srv.Close()
	e := &Engine{}
	h, rows, err := e.fetchCambio(t.Context(), SQLConfig{URL: srv.URL, Limit: 10}, "ultima")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !contains(h, "bid") {
		t.Fatalf("headers=%v rows=%v", h, rows)
	}
}

func TestParseOFX(t *testing.T) {
	raw := []byte(`OFXHEADER:100
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20240115
<TRNAMT>-12.50
<FITID>1
<MEMO>Cafe
</STMTTRN>
<STMTTRN>
<TRNTYPE>CREDIT
<DTPOSTED>20240116
<TRNAMT>100
<MEMO>Pix
</STMTTRN>`)
	h, rows, err := parseOFX(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !contains(h, "amount") || rows[0][2] != "-12.50" {
		t.Fatalf("headers=%v rows=%v", h, rows)
	}
}

func TestFetchMercadoLivreMe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "no auth", 401)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/users/me") {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 99, "nickname": "LOJA"})
			return
		}
		http.Error(w, r.URL.Path, 404)
	}))
	defer srv.Close()
	e := &Engine{}
	h, rows, err := e.fetchMercadoLivre(t.Context(), SQLConfig{AccessToken: "tok", URL: srv.URL, Limit: 10}, "me")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !contains(h, "nickname") {
		t.Fatalf("headers=%v rows=%v", h, rows)
	}
}

func TestFetchInstagramOverrideURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"id": "m1", "caption": "olá"}},
		})
	}))
	defer srv.Close()
	e := &Engine{}
	h, rows, err := e.fetchInstagram(t.Context(), SQLConfig{
		AccessToken: "t", InstagramID: "178", URL: srv.URL, Limit: 10,
	}, "media")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !contains(h, "caption") {
		t.Fatalf("headers=%v rows=%v", h, rows)
	}
}

func TestFetchSalesforceAccounts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sf" {
			http.Error(w, "no auth", 401)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"records": []map[string]any{{"Id": "001", "Name": "Acme"}},
		})
	}))
	defer srv.Close()
	e := &Engine{}
	h, rows, err := e.fetchSalesforce(t.Context(), SQLConfig{URL: srv.URL, Token: "sf", Limit: 10}, "Account")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !contains(h, "Name") {
		t.Fatalf("headers=%v rows=%v", h, rows)
	}
}

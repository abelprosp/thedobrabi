package ingest

import (
	"os"
	"strings"
	"testing"
)

func livePublic(t *testing.T) *Engine {
	t.Helper()
	if os.Getenv("LIVE_PUBLIC") != "1" {
		t.Skip("defina LIVE_PUBLIC=1 para chamar as APIs oficiais")
	}
	return &Engine{}
}

func TestLiveIBGEMunicipios(t *testing.T) {
	e := livePublic(t)
	h, rows, err := e.fetchIBGE(t.Context(), SQLConfig{Limit: 80}, "municipios")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 50 || !contains(h, "nome") {
		t.Fatalf("headers=%v n=%d", h, len(rows))
	}
}

func TestLiveIBGEPopulacao(t *testing.T) {
	e := livePublic(t)
	h, rows, err := e.fetchIBGE(t.Context(), SQLConfig{Limit: 800}, "populacao")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 27 || !contains(h, "valor") || !contains(h, "periodo") {
		t.Fatalf("headers=%v n=%d", h, len(rows))
	}
	var last string
	pi := indexOf(h, "periodo")
	for _, r := range rows {
		if pi >= 0 && pi < len(r) && r[pi] > last {
			last = r[pi]
		}
	}
	if last < "2024" {
		t.Fatalf("população SIDRA deveria incluir anos recentes, último período=%s", last)
	}
}

func TestLiveIPCA(t *testing.T) {
	e := livePublic(t)
	h, rows, err := e.fetchInflacao(t.Context(), SQLConfig{Series: "433", Limit: 2000})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 100 || !contains(h, "valor") || !contains(h, "data") {
		t.Fatalf("headers=%v n=%d", h, len(rows))
	}
	di := indexOf(h, "data")
	last := rows[len(rows)-1][di]
	if last < "01/01/2024" {
		t.Fatalf("IPCA último ponto demasiado antigo: %s", last)
	}
}

func TestLiveFocus(t *testing.T) {
	e := livePublic(t)
	h, rows, err := e.fetchExpectativas(t.Context(), SQLConfig{Limit: 400}, "anuais")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 50 || !contains(h, "Indicador") {
		t.Fatalf("headers=%v n=%d", h, len(rows))
	}
	foundIPCA := false
	ii := indexOf(h, "Indicador")
	di := indexOf(h, "Data")
	for _, r := range rows {
		if ii >= 0 && r[ii] == "IPCA" {
			foundIPCA = true
		}
		if di >= 0 && r[di] < "2020-01-01" {
			t.Fatalf("Focus deveria vir ordenado do mais recente; vi Data=%s", r[di])
		}
	}
	if !foundIPCA {
		t.Fatal("Focus sem indicador IPCA nas linhas mais recentes")
	}
}

func TestLiveCambioAwesome(t *testing.T) {
	e := livePublic(t)
	h, rows, err := e.fetchCambio(t.Context(), SQLConfig{Limit: 10}, "ultima")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 || !contains(h, "bid") || !contains(h, "_fonte") {
		t.Fatalf("headers=%v rows=%v", h, rows)
	}
	fi := indexOf(h, "_fonte")
	if rows[0][fi] != "awesomeapi" {
		t.Fatalf("fonte=%s", rows[0][fi])
	}
	if contains(h, "_fallback") {
		t.Fatal("AwesomeAPI real não deveria marcar fallback")
	}
}

func TestLiveCambioPTAX(t *testing.T) {
	e := livePublic(t)
	h, rows, err := e.fetchCambio(t.Context(), SQLConfig{Limit: 50}, "ptax")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 1 {
		t.Fatal("PTAX sem linhas")
	}
	fi := indexOf(h, "_fonte")
	if fi < 0 {
		t.Fatalf("sem _fonte: %v", h)
	}
	fonte := rows[0][fi]
	if fonte != "ptax" && fonte != "awesomeapi" && fonte != "bcb_sgs" {
		t.Fatalf("fonte inesperada %s", fonte)
	}
}

func TestLiveFetchSaaSDispatch(t *testing.T) {
	e := livePublic(t)
	h, rows, err := e.fetchSaaS(t.Context(), "inflacao", SQLConfig{Series: "433", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 5 || !contains(h, "valor") {
		t.Fatalf("sync inflacao via fetchSaaS falhou: headers=%v n=%d", h, len(rows))
	}
}

func indexOf(ss []string, v string) int {
	for i, s := range ss {
		if s == v {
			return i
		}
	}
	return -1
}

func TestLiveFocusURLHitsOlinda(t *testing.T) {
	livePublic(t)
	u := focusOfficialURL("anuais", 5)
	if !strings.Contains(u, "ExpectativasMercadoAnuais") {
		t.Fatal(u)
	}
}

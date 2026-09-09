package ingest

import (
	"strings"
	"testing"
)

func TestIBGEUnknownResource(t *testing.T) {
	_, err := ibgeLookup("nao_existe")
	if err == nil || !strings.Contains(err.Error(), "desconhecido") {
		t.Fatalf("esperado erro de recurso, got %v", err)
	}
	e := &Engine{}
	_, _, err = e.fetchIBGE(t.Context(), SQLConfig{Limit: 10}, "nao_existe")
	if err == nil {
		t.Fatal("fetch deveria recusar recurso desconhecido")
	}
}

func TestIBGEOfficialURLs(t *testing.T) {
	cases := map[string][]string{
		"populacao":      {"6579", "periodos/all", "9324"},
		"desocupacao":    {"6468", "4099"},
		"horas":          {"6371", "8190", "classificacao="},
		"rendimento":     {"6472", "5933"},
		"informalidade":  {"8529", "12466"},
		"valor_adicionado": {"5938", "513"},
		"pib_municipios": {"5938", "N6"},
		"pib_brasil":     {"6784", "9812"},
	}
	for res, needles := range cases {
		u := ibgeOfficialURL(res)
		for _, n := range needles {
			if !strings.Contains(u, n) {
				t.Fatalf("%s URL %s deveria conter %s", res, u, n)
			}
		}
		if strings.Contains(u, "/2022/") {
			t.Fatalf("%s não deveria cravar o ano 2022: %s", res, u)
		}
	}
}

func TestFlattenSIDRAClassificacao(t *testing.T) {
	raw := []byte(`[
	  {"id":"8190","variavel":"Horas","unidade":"Horas","resultados":[{
	    "classificacoes":[{"id":"2","nome":"Sexo","categoria":{"5":"Mulheres"}}],
	    "series":[{"localidade":{"id":"35","nome":"São Paulo","nivel":{"id":"N3","nome":"UF"}},
	      "serie":{"202401":"37.5","202402":"-"}}]
	  }]}
	]`)
	h, rows, err := flattenSIDRA(raw, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("deveria ignorar SIDRA '-', rows=%v", rows)
	}
	if !contains(h, "categoria") || !contains(h, "variavel") || !contains(h, "valor") {
		t.Fatalf("headers=%v", h)
	}
}

func TestAttachYoY(t *testing.T) {
	maps := attachYoY([]map[string]any{
		{"localidade_id": "35", "variavel_id": "9324", "categoria_id": "", "periodo": "2021", "valor": "100"},
		{"localidade_id": "35", "variavel_id": "9324", "categoria_id": "", "periodo": "2022", "valor": "110"},
	})
	var grew string
	for _, m := range maps {
		if m["periodo"] == "2022" {
			grew = fmtString(m["crescimento_pct"])
		}
	}
	if !strings.HasPrefix(grew, "10") {
		t.Fatalf("crescimento 2022=%s want ~10", grew)
	}
}

func TestJoinPIBPerCapita(t *testing.T) {
	out := joinPIBPerCapita(
		[]map[string]any{{"localidade_id": "35", "localidade_nome": "São Paulo", "periodo": "2021", "valor": "1000"}},
		[]map[string]any{{"localidade_id": "35", "localidade_nome": "São Paulo", "periodo": "2021", "valor": "2000"}},
	)
	if len(out) != 1 {
		t.Fatalf("rows=%d", len(out))
	}
	// 1000 mil reais / 2000 pessoas * 1000 = 500
	if fmtString(out[0]["pib_per_capita"]) != "500.00" {
		t.Fatalf("per capita=%v", out[0])
	}
}

func TestIBGECatalogResourcesCoverPNAD(t *testing.T) {
	need := []string{"horas", "rendimento", "ocupacao", "desocupacao", "informalidade", "forca_trabalho", "empregados", "escolaridade", "sexo", "setor", "pib_estados", "pib_municipios", "pib_per_capita", "valor_adicionado", "populacao", "crescimento_populacional"}
	got := map[string]bool{}
	for _, n := range ibgeResourceNames() {
		got[n] = true
	}
	for _, n := range need {
		if !got[n] {
			t.Fatalf("falta recurso %s", n)
		}
	}
}

func fmtString(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(v.(string))
}

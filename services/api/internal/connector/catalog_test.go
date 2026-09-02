package connector

import (
	"strings"
	"testing"
)

func TestCatalogCoverage(t *testing.T) {
	want := []string{
		"postgres", "supabase", "mysql", "sqlserver", "oracle", "mariadb", "snowflake", "redshift",
		"bigquery", "databricks", "mongodb", "odbc",
		"csv", "xlsx", "manual", "google_sheets", "json", "parquet", "pdf",
		"rest", "odata", "url",
		"asaas", "conta_azul", "bitrix24", "omie", "google_ads", "meta_ads",
		"instagram", "facebook", "google_business", "salesforce", "mercado_livre",
		"ibge_censo", "contabilidade", "inflacao", "expectativas", "cambio",
		"google_analytics", "github", "stripe",
		"kafka", "mqtt", "webhook",
	}
	got := map[string]Item{}
	for _, it := range Catalog() {
		if it.ID == "" || it.Label == "" || it.Group == "" {
			t.Fatalf("item incompleto: %+v", it)
		}
		if it.GroupLabel == "" {
			t.Fatalf("group_label vazio em %s", it.ID)
		}
		if it.Implemented == it.Preview {
			t.Fatalf("%s: implemented e preview devem ser opostos", it.ID)
		}
		if !it.Implemented || it.Preview {
			t.Fatalf("%s deveria ter sync real (implemented=true)", it.ID)
		}
		if len(it.Fields) == 0 {
			t.Fatalf("%s sem campos", it.ID)
		}
		if !strings.HasPrefix(it.Icon, "/connectors/"+it.ID+".") {
			t.Fatalf("%s icon %q deveria ser /connectors/%s.{svg|png}", it.ID, it.Icon, it.ID)
		}
		got[it.ID] = it
	}
	if len(got) != len(want) {
		t.Fatalf("catálogo tem %d itens, esperados %d", len(got), len(want))
	}
	for _, id := range want {
		if _, ok := got[id]; !ok {
			t.Fatalf("falta %s no catálogo", id)
		}
	}
	if got["xlsx"].Label != "Excel" {
		t.Fatalf("xlsx deveria chamar-se Excel, got %q", got["xlsx"].Label)
	}
	if got["postgres"].Label != "PostgreSQL" {
		t.Fatalf("postgres label %q", got["postgres"].Label)
	}
	if got["sqlserver"].Label != "SQL Server" {
		t.Fatalf("sqlserver label %q", got["sqlserver"].Label)
	}
	sb := got["supabase"]
	if sb.Group != GroupDatabases || !sb.Implemented {
		t.Fatalf("supabase deveria estar em bases de dados com sync real")
	}
	keys := map[string]bool{}
	for _, f := range sb.Fields {
		keys[f.Key] = true
	}
	for _, k := range []string{"project_url", "host", "port", "database", "user", "password", "ssl_mode", "service_role_key", "anon_key", "table"} {
		if !keys[k] {
			t.Fatalf("supabase sem campo %s", k)
		}
	}
}

func TestImplementedFlags(t *testing.T) {
	for _, it := range Catalog() {
		if !Implemented(it.ID) {
			t.Fatalf("%s deveria estar implementado", it.ID)
		}
	}
	if Known("nonesuch") || ByID("nonesuch") != nil {
		t.Fatal("tipo desconhecido deveria falhar")
	}
	if len(Groups()) != 9 {
		t.Fatalf("esperados 9 grupos, got %d", len(Groups()))
	}
}

func TestCanonicalAliases(t *testing.T) {
	cases := map[string]string{
		"SQL":             "sqlserver",
		"sql":             "sqlserver",
		"mssql":           "sqlserver",
		"Excel":           "xlsx",
		"excel":           "xlsx",
		"postgresql":      "postgres",
		"assas":           "asaas",
		"contaazul":       "conta_azul",
		"metaads":         "meta_ads",
		"googleads":       "google_ads",
		"postgres":        "postgres",
		"supabase.co":     "supabase",
		"ibge":            "ibge_censo",
		"ipca":            "inflacao",
		"ml":              "mercado_livre",
		"gmb":             "google_business",
		"gsheets":         "google_sheets",
		"planilha":        "google_sheets",
		"sheets":          "google_sheets",
		"googlesheets":    "google_sheets",
		"formulario":      "manual",
		"planilha_manual": "manual",
		"focus":           "expectativas",
		"ptax":            "cambio",
	}
	for in, want := range cases {
		if Canonical(in) != want {
			t.Fatalf("Canonical(%q)=%q want %q", in, Canonical(in), want)
		}
		if !Known(in) {
			t.Fatalf("Known(%q) deveria ser true", in)
		}
		if ByID(in) == nil || ByID(in).ID != want {
			t.Fatalf("ByID(%q) deveria resolver para %s", in, want)
		}
	}
}

func TestRequestedLabels(t *testing.T) {
	want := map[string]string{
		"asaas":           "Asaas",
		"xlsx":            "Excel",
		"csv":             "CSV",
		"sqlserver":       "SQL Server",
		"postgres":        "PostgreSQL",
		"supabase":        "Supabase",
		"conta_azul":      "Conta Azul",
		"bitrix24":        "Bitrix24",
		"omie":            "Omie",
		"google_ads":      "Google Ads",
		"meta_ads":        "Meta Ads",
		"instagram":       "Instagram",
		"facebook":        "Facebook",
		"google_business": "Google Meu Negócio",
		"mercado_livre":   "Mercado Livre",
		"ibge_censo":      "Censo IBGE",
		"inflacao":        "Inflação (IPCA)",
		"expectativas":    "Expectativa de mercado",
		"cambio":          "Câmbio em tempo real",
		"google_sheets":   "Google Sheets",
		"manual":          "Planilha manual",
	}
	for id, label := range want {
		it := ByID(id)
		if it == nil {
			t.Fatalf("falta %s", id)
		}
		if it.Label != label {
			t.Fatalf("%s label %q want %q", id, it.Label, label)
		}
	}
}

func TestGoogleSheetsLinkOnly(t *testing.T) {
	it := ByID("google_sheets")
	if it == nil {
		t.Fatal("falta google_sheets")
	}
	if !strings.Contains(strings.ToLower(it.Description), "sem chave de api") {
		t.Fatalf("descrição deveria dizer que não precisa de API: %q", it.Description)
	}
	keys := map[string]bool{}
	for _, f := range it.Fields {
		switch f.Key {
		case "api_key", "token", "query":
			t.Fatalf("Google Sheets não deveria pedir %s", f.Key)
		}
		keys[f.Key] = true
	}
	for _, k := range []string{"name", "url"} {
		if !keys[k] {
			t.Fatalf("google_sheets sem campo %s", k)
		}
	}
}

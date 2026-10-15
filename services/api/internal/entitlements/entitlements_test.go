package entitlements

import "testing"

func TestForPlan(t *testing.T) {
	free := ForPlan("gratis")
	if free.Plan != PlanGratis || free.AI != 10 || free.Dashboards != 2 || free.Datasets != 2 || free.StorageBytes != 10*MB || free.PriceBRL != 0 {
		t.Fatalf("plano grátis inesperado: %+v", free)
	}
	starter := ForPlan("starter")
	if starter.Plan != PlanEssencial || starter.AI != 150 || starter.Dashboards != -1 || starter.Datasets != 20 || starter.Connectors != 4 || starter.StorageBytes != 5*GB/2 || starter.Reports != -1 {
		t.Fatalf("starter inesperado: %+v", starter)
	}
	if ForPlan("enterprise").Queries != -1 || ForPlan("enterprise").Plan != PlanCompleto {
		t.Fatal("enterprise deveria mapear para Completo ilimitado")
	}
	pro := ForPlan("growth")
	if pro.AI != 1500 || pro.Plan != PlanPro || pro.Datasets != 160 || pro.StorageBytes != 25*GB {
		t.Fatalf("growth deveria mapear para Pro exponencial: %+v", pro)
	}
	if ForPlan(PlanPro).PriceBRL != 197 {
		t.Fatal("preço Pro")
	}
	if ForPlan("").Plan != PlanGratis {
		t.Fatal("plano vazio deveria ser Grátis")
	}
}

func TestConnectorAllowedGoogleSheets(t *testing.T) {
	if ConnectorAllowed(TierBasic, "google_sheets") {
		t.Fatal("Google Sheets não está no Grátis")
	}
	if !ConnectorAllowed(TierPlus, "google_sheets") {
		t.Fatal("Google Sheets deveria estar no Starter/Pro")
	}
	if !ConnectorAllowed(TierAll, "google_sheets") {
		t.Fatal("Google Sheets deveria estar no Completo")
	}
	if !ConnectorAllowed(TierBasic, "csv") {
		t.Fatal("CSV no Grátis")
	}
	if !ConnectorAllowed(TierBasic, "manual") {
		t.Fatal("planilha manual no Grátis")
	}
	if IsExternalConnector("csv") || IsExternalConnector("xlsx") {
		t.Fatal("ficheiros não são conectores externos")
	}
	if !IsExternalConnector("postgres") {
		t.Fatal("postgres é conector externo")
	}
}

func TestFormatBytes(t *testing.T) {
	if FormatBytes(10*MB) != "10 MB" {
		t.Fatal(FormatBytes(10 * MB))
	}
	if FormatBytes(5*GB/2) != "2.5 GB" {
		t.Fatal(FormatBytes(5 * GB / 2))
	}
}

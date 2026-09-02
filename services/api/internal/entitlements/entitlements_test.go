package entitlements

import "testing"

func TestForPlan(t *testing.T) {
	if ForPlan("starter").Users != 3 || ForPlan("starter").Plan != PlanEssencial {
		t.Fatal("starter deveria mapear para Essencial")
	}
	if ForPlan("enterprise").Queries != -1 || ForPlan("enterprise").Plan != PlanCompleto {
		t.Fatal("enterprise deveria mapear para Completo ilimitado")
	}
	if ForPlan("growth").AI != 1500 || ForPlan("growth").Plan != PlanPro {
		t.Fatal("growth deveria mapear para Pro com 1500 créditos de IA")
	}
	if ForPlan(PlanPro).PriceBRL != 129 {
		t.Fatal("preço Pro")
	}
}

func TestConnectorAllowedGoogleSheets(t *testing.T) {
	if ConnectorAllowed(TierBasic, "google_sheets") {
		t.Fatal("Google Sheets não está no Essencial")
	}
	if !ConnectorAllowed(TierPlus, "google_sheets") {
		t.Fatal("Google Sheets deveria estar no Pro")
	}
	if !ConnectorAllowed(TierAll, "google_sheets") {
		t.Fatal("Google Sheets deveria estar no Completo")
	}
	if !ConnectorAllowed(TierBasic, "csv") {
		t.Fatal("CSV no Essencial")
	}
}

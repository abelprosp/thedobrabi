package ingest

import (
	"strings"
	"testing"
)

func TestErpResourcePath(t *testing.T) {
	if got := erpResourcePath("totvs_protheus", "customers"); got != "customerCustomer" {
		t.Fatalf("protheus customers: %s", got)
	}
	if got := erpResourcePath("sap_b1", "Orders"); got != "Orders" {
		t.Fatalf("sap orders: %s", got)
	}
	if got := erpResourcePath("senior", "employees"); got != "hcm/v1/employees" {
		t.Fatalf("senior employees: %s", got)
	}
}

func TestErpEndpointPagination(t *testing.T) {
	u := erpEndpoint("sap_b1", "https://b1.local:50000", "Orders", 200, 100, "")
	if !strings.Contains(u, "/b1s/v1/Orders") || !(strings.Contains(u, "$top=100") || strings.Contains(u, "%24top=100")) || !(strings.Contains(u, "$skip=200") || strings.Contains(u, "%24skip=200")) {
		t.Fatalf("sap endpoint: %s", u)
	}
	p := erpEndpoint("totvs_protheus", "https://protheus.local/rest", "customers", 0, 50, "")
	if !strings.Contains(p, "customerCustomer") || !strings.Contains(p, "page=1") || !strings.Contains(p, "pageSize=50") {
		t.Fatalf("protheus endpoint: %s", p)
	}
}

func TestSaasHasNativeURLIncludesERP(t *testing.T) {
	for _, typ := range []string{"totvs_protheus", "sap_b1", "senior"} {
		if !saasHasNativeURL(typ) {
			t.Fatalf("%s should use native ERP fetcher", typ)
		}
	}
}

package apihttp

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestLookupShareSQLRequiresTenantMatch(t *testing.T) {
	if !strings.Contains(shareLookupSQL, "d.org_id=s.org_id") {
		t.Fatal("lookupShare JOIN must require d.org_id=s.org_id")
	}
	if !strings.Contains(shareLookupSQL, "d.workspace_id=s.workspace_id") {
		t.Fatal("lookupShare JOIN must require d.workspace_id=s.workspace_id")
	}
	if !strings.Contains(shareLookupSQL, "s.token=$1 OR s.token=$2") {
		t.Fatal("lookupShare must dual-read hash and legacy plaintext")
	}
	// Mismatched share.org vs dashboard.org must not resolve via id-only join.
	if strings.Contains(shareLookupSQL, "JOIN dashboards d ON d.id=s.dashboard_id\n") ||
		strings.Contains(shareLookupSQL, "JOIN dashboards d ON d.id=s.dashboard_id WHERE") {
		t.Fatal("lookupShare must not join dashboards by id alone")
	}
}

func TestPublicAppQueriesRequireTenantMatch(t *testing.T) {
	for name, sql := range map[string]string{
		"dashboard": publicAppDashboardSQL,
		"report":    publicAppReportSQL,
	} {
		if !strings.Contains(sql, "org_id=$2") || !strings.Contains(sql, "workspace_id=$3") {
			t.Fatalf("publicApp %s query must filter by org_id and workspace_id", name)
		}
		if strings.Contains(sql, "WHERE id=$1\n") || strings.HasSuffix(strings.TrimSpace(sql), "WHERE id=$1") {
			t.Fatalf("publicApp %s query must not load by id alone", name)
		}
	}
}

func TestPublicAppDoesNotLeakForeignLayout(t *testing.T) {
	orgA, orgB := uuid.New(), uuid.New()
	wsA, wsB := uuid.New(), uuid.New()

	if !dashboardBelongsToAppTenant(orgA, wsA, orgA, wsA) {
		t.Fatal("same-tenant dashboard should be included")
	}
	if dashboardBelongsToAppTenant(orgA, wsA, orgB, wsB) {
		t.Fatal("publicApp must not leak layout from foreign org/ws")
	}
	if dashboardBelongsToAppTenant(orgA, wsA, orgA, wsB) {
		t.Fatal("publicApp must not leak layout from foreign workspace")
	}
	if dashboardBelongsToAppTenant(orgA, wsA, orgB, wsA) {
		t.Fatal("publicApp must not leak layout from foreign org")
	}
}

func TestLookupShareMismatchedOrgWsFailsGuard(t *testing.T) {
	// Semantic stand-in for the SQL JOIN: share row with attacker's org pointing
	// at victim dashboard_id must not match when dashboard tenant differs.
	shareOrg, shareWs := uuid.New(), uuid.New()
	dashOrg, dashWs := uuid.New(), uuid.New()
	if dashboardBelongsToAppTenant(shareOrg, shareWs, dashOrg, dashWs) {
		t.Fatal("lookupShare with mismatched org/ws must fail tenant match")
	}
}

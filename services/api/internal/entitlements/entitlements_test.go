package entitlements

import "testing"

func TestForPlan(t *testing.T) {
	if ForPlan("starter").Users != 5 {
		t.Fatal("starter")
	}
	if ForPlan("enterprise").Queries != -1 {
		t.Fatal("enterprise unlimited")
	}
	if ForPlan("growth").AI != 5000 {
		t.Fatal("growth ai")
	}
}

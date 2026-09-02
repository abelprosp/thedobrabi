package schemax

import "testing"

func TestSanitizeAndInfer(t *testing.T) {
	if got := SanitizeIdent("Revenue (USD)"); got != "revenue_usd" {
		t.Fatalf("sanitize: %s", got)
	}
	if InferType([]string{"1", "2", "3", ""}) != TypeInt {
		t.Fatal("expected int")
	}
	if InferType([]string{"1.5", "2.0", "9"}) != TypeFloat {
		t.Fatal("expected float")
	}
	if InferType([]string{"2026-01-01", "2026-02-02"}) != TypeDate {
		t.Fatal("expected date")
	}
}

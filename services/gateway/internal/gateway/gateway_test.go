package gateway

import (
	"strings"
	"testing"
)

func TestDriverNameFor(t *testing.T) {
	got, err := driverNameFor("postgresql")
	if err != nil || got != "pgx" {
		t.Fatalf("postgresql -> %q %v", got, err)
	}
	got, err = driverNameFor("postgres")
	if err != nil || got != "pgx" {
		t.Fatalf("postgres -> %q %v", got, err)
	}
	got, err = driverNameFor("mysql")
	if err != nil || got != "mysql" {
		t.Fatalf("mysql -> %q %v", got, err)
	}
	if _, err := driverNameFor("mssql"); err == nil {
		t.Fatal("mssql should fail")
	}
}

func TestSafeSelectLimit(t *testing.T) {
	q, err := safeSelect("SELECT * FROM t", 50)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(strings.ToUpper(q), "LIMIT 50") {
		t.Fatalf("expected appended LIMIT, got %q", q)
	}

	already := "SELECT * FROM t LIMIT 10"
	q, err = safeSelect(already, 50)
	if err != nil {
		t.Fatal(err)
	}
	if q != already {
		t.Fatalf("should not duplicate LIMIT: %q", q)
	}

	multiline := "select id from t\nlimit 5"
	q, err = safeSelect(multiline, 100)
	if err != nil {
		t.Fatal(err)
	}
	if q != multiline {
		t.Fatalf("multiline LIMIT should be kept: %q", q)
	}

	if _, err := safeSelect("DELETE FROM t", 10); err == nil {
		t.Fatal("non-select should fail")
	}
}

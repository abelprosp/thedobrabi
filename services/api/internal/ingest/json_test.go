package ingest

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestParseJSONArray(t *testing.T) {
	headers, rows, err := parseJSON([]byte(`[{"id":1,"name":"A"},{"id":2,"name":"B","extra":true}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) < 2 || len(rows) != 2 {
		t.Fatalf("headers=%v rows=%d", headers, len(rows))
	}
}

func TestParseJSONWrapped(t *testing.T) {
	headers, rows, err := parseJSON([]byte(`{"value":[{"sku":"x","qty":3}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(headers, "sku") || len(rows) != 1 {
		t.Fatalf("headers=%v rows=%v", headers, rows)
	}
}

func TestParseNDJSON(t *testing.T) {
	headers, rows, err := parseJSON([]byte("{\"a\":1}\n{\"a\":2,\"b\":\"z\"}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !contains(headers, "a") {
		t.Fatalf("headers=%v rows=%v", headers, rows)
	}
}

func TestParseJSONRejectsScalar(t *testing.T) {
	_, _, err := parseJSON([]byte(`"hello"`))
	if err == nil {
		t.Fatal("esperava erro")
	}
}

func TestHeaderMapUnmarshal(t *testing.T) {
	var cfg SQLConfig
	if err := json.Unmarshal([]byte(`{"headers":"X-A: 1\nAuthorization: Bearer t"}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Headers["X-A"] != "1" || cfg.Headers["Authorization"] != "Bearer t" {
		t.Fatalf("%v", cfg.Headers)
	}
	var cfg2 SQLConfig
	if err := json.Unmarshal([]byte(`{"headers":{"Accept":"application/json"}}`), &cfg2); err != nil {
		t.Fatal(err)
	}
	if cfg2.Headers["Accept"] != "application/json" {
		t.Fatalf("%v", cfg2.Headers)
	}
}

func TestRedactConfigHidesSecrets(t *testing.T) {
	m := RedactConfig(SQLConfig{Host: "db", Password: "secret", Token: "tok", APIKey: "k", Headers: HeaderMap{"X": "y"}})
	if m["password"] != nil || m["token"] != nil || m["api_key"] != nil {
		t.Fatalf("secrets leaked: %v", m)
	}
	if m["password_set"] != true || m["host"] != "db" {
		t.Fatalf("%v", m)
	}
	keys, _ := m["header_keys"].([]string)
	if !reflect.DeepEqual(keys, []string{"X"}) && (len(keys) != 1 || keys[0] != "X") {
		t.Fatalf("header keys %v", m["header_keys"])
	}
}

func TestAssertHTTPURL(t *testing.T) {
	if err := assertHTTPURL("https://api.example.com/v1"); err != nil {
		t.Fatal(err)
	}
	if err := assertHTTPURL("file:///etc/passwd"); err == nil {
		t.Fatal("file deveria falhar")
	}
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

package apihttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsLocalOrigin(t *testing.T) {
	if !isLocalOrigin("http://localhost:3056") {
		t.Fatal("expected localhost")
	}
	if !isLocalOrigin("http://127.0.0.1:3010") {
		t.Fatal("expected 127.0.0.1")
	}
	if isLocalOrigin("https://app.thedobra.cc") {
		t.Fatal("production origin must not be local")
	}
}

func TestRequestPublicOriginFromForwardedHost(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:2003/api/v1/dashboards/x/share", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-Host", "app.thedobra.cc")
	got := requestPublicOrigin(r)
	if got != "https://app.thedobra.cc" {
		t.Fatalf("got %q", got)
	}
}

func TestRequestPublicOriginFromBrowserOrigin(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:2003/api/v1/dashboards/x/share", nil)
	r.Header.Set("Origin", "https://app.thedobra.cc")
	got := requestPublicOrigin(r)
	if got != "https://app.thedobra.cc" {
		t.Fatalf("got %q", got)
	}
}

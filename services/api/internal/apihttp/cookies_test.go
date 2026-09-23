package apihttp

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thedobra/thedobra/services/api/internal/authn"
)

func TestCookieSecure(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	r := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
	if cookieSecure(r) {
		t.Fatal("http + development must not be Secure")
	}

	r.Header.Set("X-Forwarded-Proto", "https")
	if !cookieSecure(r) {
		t.Fatal("X-Forwarded-Proto=https must be Secure")
	}

	r2 := httptest.NewRequest(http.MethodGet, "https://example/", nil)
	r2.TLS = &tls.ConnectionState{}
	if !cookieSecure(r2) {
		t.Fatal("TLS request must be Secure")
	}

	t.Setenv("APP_ENV", "production")
	r3 := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
	if !cookieSecure(r3) {
		t.Fatal("APP_ENV=production must be Secure")
	}
}

func TestSetAndClearAuthCookies(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "http://localhost/api/v1/auth/login", nil)
	setAuthCookies(rec, r, authn.TokenPair{
		AccessToken:  "access-xyz",
		RefreshToken: "refresh-xyz",
		ExpiresIn:    3600,
	})
	cookies := rec.Result().Cookies()
	var sawAccess, sawRefresh bool
	for _, c := range cookies {
		if c.Name == cookieAccess {
			sawAccess = true
			if !c.HttpOnly || c.Path != "/" || c.SameSite != http.SameSiteLaxMode || c.Value != "access-xyz" {
				t.Fatalf("bad access cookie: %+v", c)
			}
		}
		if c.Name == cookieRefresh {
			sawRefresh = true
			if !c.HttpOnly || c.Value != "refresh-xyz" {
				t.Fatalf("bad refresh cookie: %+v", c)
			}
		}
	}
	if !sawAccess || !sawRefresh {
		t.Fatal("expected both auth cookies")
	}

	rec2 := httptest.NewRecorder()
	clearAuthCookies(rec2, r)
	for _, c := range rec2.Result().Cookies() {
		if c.MaxAge >= 0 && c.Value != "" {
			t.Fatalf("clear should expire cookie: %+v", c)
		}
	}
}

func TestAccessTokenFromRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer from-header")
	r.AddCookie(&http.Cookie{Name: cookieAccess, Value: "from-cookie"})
	if got := accessTokenFromRequest(r); got != "from-header" {
		t.Fatalf("Bearer must win, got %q", got)
	}

	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.AddCookie(&http.Cookie{Name: cookieAccess, Value: "from-cookie"})
	if got := accessTokenFromRequest(r2); got != "from-cookie" {
		t.Fatalf("cookie fallback, got %q", got)
	}
}

package apihttp

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/thedobra/thedobra/services/api/internal/authn"
)

const (
	cookieAccess  = "thedobra_access"
	cookieRefresh = "thedobra_refresh"
	refreshMaxAge = 30 * 24 * 3600 // 30 days, matches refresh token TTL
)

func cookieSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	return strings.EqualFold(os.Getenv("APP_ENV"), "production")
}

func setAuthCookies(w http.ResponseWriter, r *http.Request, tok authn.TokenPair) {
	secure := cookieSecure(r)
	accessMax := tok.ExpiresIn
	if accessMax <= 0 {
		accessMax = int((8 * time.Hour).Seconds())
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieAccess,
		Value:    tok.AccessToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   accessMax,
	})
	if tok.RefreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     cookieRefresh,
			Value:    tok.RefreshToken,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   secure,
			MaxAge:   refreshMaxAge,
		})
	}
}

func clearAuthCookies(w http.ResponseWriter, r *http.Request) {
	secure := cookieSecure(r)
	for _, name := range []string{cookieAccess, cookieRefresh} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   secure,
			MaxAge:   -1,
		})
	}
}

// accessTokenFromRequest prefers Authorization Bearer, then thedobra_access cookie.
func accessTokenFromRequest(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		if t := strings.TrimSpace(strings.TrimPrefix(h, "Bearer ")); t != "" {
			return t
		}
	}
	if c, err := r.Cookie(cookieAccess); err == nil {
		return strings.TrimSpace(c.Value)
	}
	return ""
}

func refreshTokenFromRequest(r *http.Request, bodyToken string) string {
	if t := strings.TrimSpace(bodyToken); t != "" {
		return t
	}
	if c, err := r.Cookie(cookieRefresh); err == nil {
		return strings.TrimSpace(c.Value)
	}
	return ""
}

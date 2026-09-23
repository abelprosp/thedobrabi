package apihttp

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/thedobra/thedobra/services/api/internal/httpx"
)

// securityHeaders centralizes browser-side protections so every API route gets
// the same baseline, including public embeds and authentication endpoints.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimit is Redis-backed so limits are shared by all API replicas.
// Auth routes fail closed (503) if Redis is unavailable to avoid brute-force
// during an outage; other routes still fail open so a cache outage does not
// take down the whole API.
func (s *Server) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		key := requestIP(r)
		bucket := "general"
		limit := int64(300)
		authRoute := strings.HasPrefix(r.URL.Path, "/api/v1/auth/")
		if authRoute {
			bucket = "auth"
			limit = 30
		} else if strings.HasPrefix(r.URL.Path, "/api/v1/public/") {
			bucket = "public"
			limit = 60
		} else if strings.Contains(r.URL.Path, "/ai/") || strings.HasPrefix(r.URL.Path, "/api/v1/queries") {
			bucket = "analytics"
			limit = 120
		}
		window := time.Now().UTC().Truncate(time.Minute).Unix()
		redisKey := "thedobra:ratelimit:" + key + ":" + bucket + ":" + formatInt(window)
		ctx := r.Context()
		count, err := s.deps.Redis.Incr(ctx, redisKey).Result()
		if err != nil {
			if authRoute {
				httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "serviço temporariamente indisponível")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if count == 1 {
			_ = s.deps.Redis.Expire(ctx, redisKey, 70*time.Second).Err()
		}
		w.Header().Set("X-RateLimit-Limit", formatInt(limit))
		w.Header().Set("X-RateLimit-Remaining", formatInt(maxInt64(0, limit-count)))
		if count > limit {
			w.Header().Set("Retry-After", "60")
			httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "muitas solicitações; tente novamente em instantes")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host == "" {
		return "unknown"
	}
	// Behind Nginx/local proxy, RemoteAddr is loopback — use first X-Forwarded-For hop
	// so rate limits are per client, not shared across everyone as 127.0.0.1.
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first := strings.TrimSpace(strings.Split(xff, ",")[0])
			if first != "" {
				return first
			}
		}
	}
	return host
}

func formatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

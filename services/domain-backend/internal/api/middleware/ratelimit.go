package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/auth/ratelimit"
)

// RateLimitMiddleware aplica token bucket rate limit por key org+IP.
type RateLimitMiddleware struct {
	Limiter *ratelimit.Limiter
	KeyFunc func(r *http.Request) string
}

func (m *RateLimitMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := m.KeyFunc(r)
		if !m.Limiter.Allow(key) {
			retryAfter := m.Limiter.RetryAfter(key, 1)
			reset := time.Now().Add(retryAfter).Unix()
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			// HU-28.5: status ya escrito → best effort sobre el body. Loggeamos
			// errores de escritura en vez de tragarlos.
			if _, err := w.Write([]byte(`{"error":{"code":"rate_limited","message":"too many requests"}}`)); err != nil {
				slog.Warn("ratelimit response write failed", "error", err, "key", key)
			}
			return
		}
		remaining := m.Limiter.Tokens(key)
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(int(remaining)))
		next.ServeHTTP(w, r)
	})
}

// DefaultKeyFunc usa API key ID si autenticado, sino IP.
func DefaultKeyFunc(r *http.Request) string {
	if p, ok := apikey.FromContext(r.Context()); ok && p != nil {
		return "org:" + p.OrganizationID + ":key:" + p.APIKeyID
	}
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}
	return "ip:" + ip
}

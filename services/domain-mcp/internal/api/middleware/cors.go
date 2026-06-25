// Package middleware — CORS configurable (issue-32.2).
//
// CORS habilitado para /api/v1/* via env var DOMAIN_CORS_ORIGINS (CSV).
// Default deny: sin env var, ningún header CORS se agrega.
//
// Spec en .claude/rules/api.md sección CORS + design de issue-32.2.
package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// CORS middleware con allowlist explícito.
//
// Construcción canónica: NewCORS(origins, logger).
//   - origins == nil/empty → default deny: no agrega headers, no rompe req
//   - origins == ["*"]     → wildcard mode (dev only): * sin credentials + warn
//   - origins == [a, b]    → allowlist estricto + Vary: Origin + credentials
type CORS struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAgeSeconds    int
	Logger           *slog.Logger
	wildcardMode     bool
}

// NewCORS construye el middleware desde una lista de origins.
//
// Logger puede ser nil (no se loggea CORS denied).
func NewCORS(origins []string, logger *slog.Logger) *CORS {
	cleaned := make([]string, 0, len(origins))
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		cleaned = append(cleaned, o)
	}
	c := &CORS{
		AllowedOrigins: cleaned,
		AllowedMethods: []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "X-CSRF-Token", "X-Request-ID", "Idempotency-Key"},
		ExposedHeaders: []string{"X-Request-ID"},
		MaxAgeSeconds:  86400,
		Logger:         logger,
	}
	switch {
	case len(cleaned) == 0:

	case len(cleaned) == 1 && cleaned[0] == "*":
		c.wildcardMode = true
		c.AllowCredentials = false
		if logger != nil {
			logger.Warn("CORS wildcard enabled; NOT for production",
				slog.String("origins", "*"))
		}
	default:
		c.AllowCredentials = true
	}
	return c
}

// DefaultCORS construye un CORS sin origins (default deny).
// Mantenido por compat con call-sites existentes; usar NewCORS para nuevas wireos.
func DefaultCORS() *CORS {
	return NewCORS(nil, nil)
}

// Enabled reporta si el middleware tiene al menos un origin configurado.
func (c *CORS) Enabled() bool {
	return len(c.AllowedOrigins) > 0
}

// Wrap envuelve next con la lógica CORS.
//
// Comportamiento:
//   - Sin Origin header o sin origins configurados → pasa transparente.
//   - Origin en allowlist → setea Access-Control-Allow-Origin + Vary + credentials.
//   - Preflight (OPTIONS + Access-Control-Request-Method) → 204 con headers.
//   - Origin fuera de allowlist → loggea warn, NO setea headers (browser bloquea).
func (c *CORS) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		isPreflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""

		if origin == "" || !c.Enabled() {
			if isPreflight {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if !c.matchOrigin(origin) {
			if c.Logger != nil {
				c.Logger.Warn("CORS denied origin",
					slog.String("origin", origin),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path))
			}
			if isPreflight {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if c.wildcardMode {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		if c.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if len(c.ExposedHeaders) > 0 {
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(c.ExposedHeaders, ", "))
		}

		if isPreflight {
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(c.AllowedMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(c.AllowedHeaders, ", "))
			if c.MaxAgeSeconds > 0 {
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(c.MaxAgeSeconds))
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (c *CORS) matchOrigin(origin string) bool {
	for _, o := range c.AllowedOrigins {
		if o == "*" {
			return true
		}
		if o == origin {
			return true
		}
	}
	return false
}

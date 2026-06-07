// HU-02.1 + HU-13.2 — middleware HTTP que extrae API key del header Authorization
// y resuelve user/org context vía Resolver interface.

package apikey

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// Principal datos del caller post-auth.
type Principal struct {
	UserID         string
	OrganizationID string
	APIKeyID       string
	Role           string
}

// principalKey context key.
type principalKey struct{}

// FromContext retorna principal autenticado o false.
func FromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(*Principal)
	return p, ok
}

// Resolver lookup de API key plaintext → Principal.
// Implementaciones: pg adapter (HU-02.1 store).
type Resolver interface {
	Resolve(ctx context.Context, plaintext string) (*Principal, error)
}

// ErrUnauthorized error tipado para 401.
var ErrUnauthorized = errors.New("unauthorized")

// Middleware autentica vía header `Authorization: Bearer domk_*`.
// Skip si path está en allowlist (e.g. /health).
type Middleware struct {
	Resolver  Resolver
	Allowlist []string // paths exactos que no requieren auth
}

// ServeHTTP wraps next con check de auth.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allowlist
		for _, p := range m.Allowlist {
			if r.URL.Path == p {
				next.ServeHTTP(w, r)
				return
			}
		}

		header := r.Header.Get("Authorization")
		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(header, bearerPrefix) {
			writeUnauthorized(w, "missing_bearer", "Authorization header required")
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))
		if !IsAPIKeyFormat(token) {
			writeUnauthorized(w, "invalid_format", "invalid api key format")
			return
		}

		p, err := m.Resolver.Resolve(r.Context(), token)
		if err != nil {
			writeUnauthorized(w, "invalid_credentials", "api key not found or revoked")
			return
		}

		ctx := context.WithValue(r.Context(), principalKey{}, p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeUnauthorized(w http.ResponseWriter, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("WWW-Authenticate", `Bearer realm="domain"`)
	w.WriteHeader(http.StatusUnauthorized)
	// Anti-enumeration: mismo body para todas las causas
	_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"unauthorized"}}`))
	_ = code
	_ = msg
}

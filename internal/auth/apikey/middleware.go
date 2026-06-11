// issue-02.1 + issue-13.2 — middleware HTTP que extrae API key del header Authorization
// y resuelve user/org context vía Resolver interface.
//
// issue-25.14: post-auth, abre una tx con SET LOCAL app.current_org_id y
// app.current_user_id y la inyecta en el ctx via txctx.WithTxContext.
// Esto permite que la RLS de Postgres (issue-25.5) actúe sobre tablas
// observations/sessions/etc. sin necesidad de que cada handler/repo
// conozca el patrón SET LOCAL.

package apikey

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/store/txctx"
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

// WithPrincipal inyecta un Principal en el context. Usado por:
//   - el middleware HTTP (post-auth) para propagar identidad a handlers
//   - tests para inyectar un Principal sin pasar por el flujo de auth
//
// La key privada garantiza que solo este package puede setear/extraer.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// Resolver lookup de API key plaintext → Principal.
// Implementaciones: pg adapter (issue-02.1 store).
type Resolver interface {
	Resolve(ctx context.Context, plaintext string) (*Principal, error)
}

// ErrUnauthorized error tipado para 401.
var ErrUnauthorized = errors.New("unauthorized")

// Middleware autentica vía header `Authorization: Bearer domk_*`.
// Skip si path está en allowlist (e.g. /health).
//
// Si Pool != nil, post-auth abre una tx con SET LOCAL app.current_org_id
// y app.current_user_id (issue-25.14). La tx vive lo que dura el handler;
// al salir del wrapper se hace Rollback si el handler no hizo Commit
// explícitamente. Repos de tablas con RLS extraen la tx con
// txctx.TxFromContext.
type Middleware struct {
	Resolver  Resolver
	Allowlist []string // paths exactos que no requieren auth
	Pool      *pgxpool.Pool // opcional; si nil, NO se abre tx (legacy auth-only)
}

// ServeHTTP wraps next con check de auth + (opcional) tx wireup.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allowlist: match exacto o prefix con trailing "/*"
		for _, p := range m.Allowlist {
			if strings.HasSuffix(p, "/*") {
				prefix := strings.TrimSuffix(p, "/*")
				if strings.HasPrefix(r.URL.Path, prefix) {
					next.ServeHTTP(w, r)
					return
				}
			} else if r.URL.Path == p {
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

		ctx := WithPrincipal(r.Context(), p)

		// issue-25.14: wireup tx con SET LOCAL post-auth.
		// Si m.Pool es nil → modo legacy (solo Principal en ctx, sin tx).
		// Si el Principal no tiene org/user parseable → no abrimos tx
		// (es un caso borde; handler igual corre, queries fallarán si
		// tocan tablas RLS, pero el contrato de auth no exige tx).
		if m.Pool != nil {
			orgID, orgErr := uuid.Parse(p.OrganizationID)
			userID, userErr := uuid.Parse(p.UserID)
			if orgErr == nil && userErr == nil && orgID != uuid.Nil {
				tx, terr := m.openTxWithOrg(ctx, orgID, userID)
				if terr != nil {
					// No exponemos el error técnico al cliente (anti-enum),
					// pero logueable vía metrics. 500 porque es bug nuestro.
					http.Error(w, `{"error":{"code":"internal","message":"wireup failed"}}`,
						http.StatusInternalServerError)
					return
				}
				defer func() { _ = tx.Rollback(ctx) }()
				ctx = txctx.WithTxContext(ctx, tx)
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// openTxWithOrg abre una tx y ejecuta SET LOCAL app.current_org_id +
// app.current_user_id en una sola round-trip via set_config.
//
// Rechaza uuid.Nil (defense: nil org podría bypassear RLS si Postgres
// lo aceptara, aunque current_org_id() ya coerce a NULL con EXCEPTION).
func (m *Middleware) openTxWithOrg(ctx context.Context, orgID, userID uuid.UUID) (pgx.Tx, error) {
	if orgID == uuid.Nil {
		return nil, errors.New("apikey.Middleware: orgID uuid.Nil rejected")
	}
	tx, err := m.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`SELECT set_config('app.current_org_id', $1, true), set_config('app.current_user_id', $2, true)`,
		orgID.String(), userID.String()); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
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

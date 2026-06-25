








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




	SessionResolver SessionResolverFunc
}

// SessionResolverFunc resuelve un session token "sess_*". Devuelve
// (principal, ctxAttach, err). ctxAttach es un closure que el
// middleware llama para inyectar el Active completo en el ctx
// (evitando import circular apikey ↔ session).
type SessionResolverFunc func(ctx context.Context, plainToken string) (*Principal, func(context.Context) context.Context, error)

// ServeHTTP wraps next con check de auth + (opcional) tx wireup.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

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




		var ctx context.Context
		var p *Principal
		if m.SessionResolver != nil && strings.HasPrefix(token, "sess_") {
			pp, attacher, err := m.SessionResolver(r.Context(), token)
			if err != nil || pp == nil {
				writeUnauthorized(w, "invalid_credentials", "session inválida o expirada")
				return
			}
			p = pp
			ctx = r.Context()
			if attacher != nil {
				ctx = attacher(ctx)
			}
		} else {
			if !IsAPIKeyFormat(token) {
				writeUnauthorized(w, "invalid_format", "invalid bearer token format")
				return
			}
			var err error
			p, err = m.Resolver.Resolve(r.Context(), token)
			if err != nil {
				writeUnauthorized(w, "invalid_credentials", "api key not found or revoked")
				return
			}
			ctx = r.Context()
		}
		ctx = WithPrincipal(ctx, p)






		if m.Pool != nil {
			orgID, orgErr := uuid.Parse(p.OrganizationID)
			userID, userErr := uuid.Parse(p.UserID)
			if orgErr == nil && userErr == nil && orgID != uuid.Nil {
				tx, terr := m.openTxWithOrg(ctx, orgID, userID)
				if terr != nil {


					http.Error(w, `{"error":{"code":"internal","message":"wireup failed"}}`,
						http.StatusInternalServerError)
					return
				}




				rec := &statusRecorder{ResponseWriter: w, status: 200}
				defer func() {
					if rec.status >= 500 {
						_ = tx.Rollback(ctx)
						return
					}
					_ = tx.Commit(ctx)
				}()
				ctx = txctx.WithTxContext(ctx, tx)
				w = rec
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

	_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"unauthorized"}}`))
	_ = code
	_ = msg
}

// statusRecorder captura el status code del response para que el wireup
// de tx pueda decidir commit (status<500) vs rollback (status>=500).
//
// Status default = 200 (per http.ResponseWriter contract) si el handler
// nunca llamo WriteHeader explicitamente.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.status = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {

		r.wroteHeader = true
	}
	return r.ResponseWriter.Write(b)
}

// Flush implementa http.Flusher para que SSE (REQ-69) funcione a través
// del middleware. Sin esto, el handler /api/v1/events recibe un writer
// sin Flusher y devuelve "streaming no soportado por el writer".
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack passthrough — útil si en el futuro algún handler quiere usar
// WebSocket. No rompe nada agregarlo.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

package apikey

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

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
	// AllowedTools es el allowlist de tools MCP de la credencial; nil/vacío =
	// full access. Un service token de mínimo privilegio (ej. el mcpServer ACP)
	// lo restringe para excluir agent_run/orchestrate/flow_run (DOMAINSERV-85).
	AllowedTools []string
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

// AuthFailureLogger registra un intento de auth con API key fallido en el
// audit trail (auth_events). Interfaz definida en el consumidor (coupling
// policy); la implementa session.Service. reason es un código + contexto NO
// sensible (nunca el token, policy secrets-redaction). DOMAINSERV-82 H1.
type AuthFailureLogger interface {
	LogAPIKeyAuthFailure(ctx context.Context, reason, ip, userAgent string)
}

// ErrUnauthorized error tipado para 401.
var ErrUnauthorized = errors.New("unauthorized")

// Middleware autentica vía header `Authorization`, aceptando dos schemes:
// `Bearer <api-key|sess_*>` y `DOMAIN-HMAC-SHA256 ...` (firma, DOMAINSERV-129).
// Skip si path está en allowlist (e.g. /health).
//
// ALCANCE: este middleware cubre solo /api/ (ver server_router.go). El
// transporte MCP en /mcp tiene su propio Bearer en
// internal/mcp/httpserver/handler.go y NO pasa por acá: la firma HMAC todavía
// no aplica ahí.
//
// Si Pool != nil, post-auth abre una tx con SET LOCAL app.current_org_id
// y app.current_user_id (issue-25.14). La tx vive lo que dura el handler;
// al salir del wrapper se hace Rollback si el handler no hizo Commit
// explícitamente. Repos de tablas con RLS extraen la tx con
// txctx.TxFromContext.
type Middleware struct {
	Resolver  Resolver
	Allowlist []string      // paths exactos que no requieren auth
	Pool      *pgxpool.Pool // opcional; si nil, NO se abre tx (legacy auth-only)

	SessionResolver SessionResolverFunc

	// FailureLogger registra fallos de auth con API key en auth_events
	// (opcional, nil-safe). DOMAINSERV-82 H1.
	FailureLogger AuthFailureLogger

	// SignedResolver y NonceBurner habilitan el scheme DOMAIN-HMAC-SHA256.
	// Si alguno es nil, una request firmada se rechaza (no hay anti-replay).
	SignedResolver SignedResolver
	NonceBurner    NonceBurner

	// RequireSignature prohíbe el Bearer en claro: solo pasan requests
	// firmadas. Default false para convivir con los clientes actuales.
	RequireSignature bool

	// MaxSignedBodyBytes override del tope de body a bufferear para hashear
	// (0 = DefaultMaxSignedBodyBytes).
	MaxSignedBodyBytes int64

	// Now reloj inyectable para la ventana de skew (nil = time.Now).
	Now func() time.Time
}

// SessionResolverFunc resuelve un session token "sess_*". Devuelve
// (principal, ctxAttach, err). ctxAttach es un closure que el
// middleware llama para inyectar el Active completo en el ctx
// (evitando import circular apikey ↔ session).
type SessionResolverFunc func(ctx context.Context, plainToken string) (*Principal, func(context.Context) context.Context, error)

// ServeHTTP wraps next con check de auth + (opcional) tx wireup.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.allowlisted(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		r, p, ok := m.authenticate(w, r)
		if !ok {
			writeUnauthorized(w)
			return
		}
		ctx := WithPrincipal(r.Context(), p)

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

// allowlisted true si el path no requiere auth. Soporta sufijo "/*" como
// prefijo de subárbol.
func (m *Middleware) allowlisted(path string) bool {
	for _, p := range m.Allowlist {
		if strings.HasSuffix(p, "/*") {
			if strings.HasPrefix(path, strings.TrimSuffix(p, "/*")) {
				return true
			}
		} else if path == p {
			return true
		}
	}
	return false
}

// authenticate despacha por el scheme del header Authorization. Devuelve la
// request (con el body repuesto si hubo que hashearlo, o con el ctx de sesión
// adjunto) y el Principal. Cada rama loguea su propio fallo: el caller solo
// escribe el 401 uniforme.
func (m *Middleware) authenticate(w http.ResponseWriter, r *http.Request) (*http.Request, *Principal, bool) {
	header := r.Header.Get("Authorization")
	if HasHMACScheme(header) {
		return m.authenticateSigned(w, r)
	}
	if m.RequireSignature {
		m.logAPIKeyFailure(r, "signature_required")
		return r, nil, false
	}
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(header, bearerPrefix) {
		return r, nil, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))
	if m.SessionResolver != nil && strings.HasPrefix(token, "sess_") {
		return m.authenticateSession(r, token)
	}
	return m.authenticateBearer(r, token)
}

// authenticateSession resuelve un token "sess_*" y adjunta el Active al ctx.
func (m *Middleware) authenticateSession(r *http.Request, token string) (*http.Request, *Principal, bool) {
	p, attacher, err := m.SessionResolver(r.Context(), token)
	if err != nil || p == nil {
		return r, nil, false
	}
	if attacher != nil {
		r = r.WithContext(attacher(r.Context()))
	}
	return r, p, true
}

// authenticateBearer resuelve una API key que viaja en claro.
func (m *Middleware) authenticateBearer(r *http.Request, token string) (*http.Request, *Principal, bool) {
	if !IsAPIKeyFormat(token) {
		m.logAPIKeyFailure(r, "invalid_format")
		return r, nil, false
	}
	p, err := m.Resolver.Resolve(r.Context(), token)
	if err != nil || p == nil {
		m.logAPIKeyFailure(r, "invalid_credentials")
		return r, nil, false
	}
	return r, p, true
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

// logAPIKeyFailure registra un fallo de auth con API key en auth_events vía el
// FailureLogger (best-effort, nil-safe). Nunca incluye el token: solo el código
// de fallo + path (contexto SOC) e IP/User-Agent del request. DOMAINSERV-82 H1.
func (m *Middleware) logAPIKeyFailure(r *http.Request, code string) {
	if m.FailureLogger == nil {
		return
	}
	reason := code + " path=" + r.URL.Path
	m.FailureLogger.LogAPIKeyAuthFailure(r.Context(), reason, clientIP(r), r.UserAgent())
}

// clientIP extrae la IP del cliente: primer hop de X-Forwarded-For si está, si
// no el host de RemoteAddr (sin puerto).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// writeUnauthorized emite SIEMPRE el mismo 401: qué chequeo falló no se filtra
// al cliente (anti-enumeration issue-02.7). El detalle va al audit trail.
func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("WWW-Authenticate", `Bearer realm="domain"`)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"unauthorized"}}`))
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

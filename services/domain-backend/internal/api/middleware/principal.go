// HU-28.3 — middleware principal-ctx
//
// Post-auth, extrae el Principal del context (lo seteó apikey.Middleware),
// parsea OrganizationID y UserID a uuid.UUID y los inyecta como value objects
// en el context via ctxkeys. Los handlers downstream usan a.orgID(ctx) /
// a.userID(ctx) en vez de repetir `p, _ := principal(r); uuid.Parse(...)`.
//
// Diseño:
//   - Si NO hay Principal en el ctx (path en allowlist o auth no corrió),
//     pasa el request sin tocar nada — no es responsabilidad de este MW
//     bloquear, solo enriquecer ctx.
//   - Si el Principal existe pero los UUIDs son inválidos, igual deja pasar:
//     ctxkeys.OrgID(ctx) devolverá uuid.Nil y el helper authorizeOrg fallará
//     en el handler. Es coherente con anti-enumeration (404 en vez de 401).
//
// Inserción: justo después de apikey.Middleware (que ya resuelve Principal
// y opcionalmente abre tx con SET LOCAL). No abre tx propia.

package middleware

import (
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/api/ctxkeys"
	"nunezlagos/domain/internal/auth/apikey"
)

// PrincipalCtx wraps next con la lógica de extraer Principal del ctx (lo
// dejó apikey.Middleware) y reinyectar OrgID/UserID como uuid.UUID via
// ctxkeys.
func PrincipalCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := apikey.FromContext(r.Context())
		if !ok || p == nil {
			// Allowlist path o auth aún no corrió: no enriquecemos ctx.
			next.ServeHTTP(w, r)
			return
		}
		ctx := r.Context()
		if orgID, err := uuid.Parse(p.OrganizationID); err == nil {
			ctx = ctxkeys.WithOrgID(ctx, orgID)
		}
		if userID, err := uuid.Parse(p.UserID); err == nil {
			ctx = ctxkeys.WithUserID(ctx, userID)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

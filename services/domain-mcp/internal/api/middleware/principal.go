

















package middleware

import (
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/api/ctxkeys"
	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/logging"
)

// PrincipalCtx wraps next con la lógica de extraer Principal del ctx (lo
// dejó apikey.Middleware) y reinyectar OrgID/UserID como uuid.UUID via
// ctxkeys.
func PrincipalCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := apikey.FromContext(r.Context())
		if !ok || p == nil {

			next.ServeHTTP(w, r)
			return
		}
		// ctxkeys y logging son familias de context keys DISTINTAS: los handlers
		// leen de ctxkeys, el ctxEnrichHandler del logger lee de logging. Escribir
		// solo en una dejaba los logs sin user_id ni organization_id (DOMAINSERV-81).
		// El duplicado es deliberado: la alternativa era que logging importara
		// api/ctxkeys, invirtiendo la dirección de dependencia.
		ctx := r.Context()
		if orgID, err := uuid.Parse(p.OrganizationID); err == nil {
			ctx = ctxkeys.WithOrgID(ctx, orgID)
			ctx = logging.WithOrgID(ctx, orgID.String())
		}
		if userID, err := uuid.Parse(p.UserID); err == nil {
			ctx = ctxkeys.WithUserID(ctx, userID)
			ctx = logging.WithUserID(ctx, userID.String())
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

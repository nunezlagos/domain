

















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

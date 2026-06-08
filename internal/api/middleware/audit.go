// Package middleware — HU-02.4 audit metadata injection.
//
// Este middleware extrae IP, User-Agent y RequestID del request y los pone
// en el context.Context para que PGRecorder.Record() los use automáticamente.
package middleware

import (
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/audit"
)

// AuditMiddleware extrae IP/UA/RequestID y los inyecta en el context.
func AuditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}
		ua := r.UserAgent()
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.New().String()
		}
		ctx := audit.WithAuditMetadata(r.Context(), ip, ua, rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

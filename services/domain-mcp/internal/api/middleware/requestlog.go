package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/logging"
)

// headerRequestID es el nombre canónico de facto para propagar el id entre
// servicios; se respeta el entrante para no cortar una traza que ya venía.
const headerRequestID = "X-Request-Id"

type logRecorder struct {
	http.ResponseWriter
	status int
}

func (r *logRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// RequestLog logea cada request y siembra el request_id que el ctxEnrichHandler de
// internal/logging inyecta en TODO log posterior del mismo request. Sin esta
// siembra ese handler no tenía nunca qué inyectar y los logs salían sin un solo
// campo de correlación (DOMAINSERV-81, criterio 2).
func RequestLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			requestID := r.Header.Get(headerRequestID)
			if requestID == "" {
				requestID = uuid.NewString()
			}
			ctx := logging.WithRequestID(r.Context(), requestID)
			// se escribe antes de ServeHTTP: después el handler ya mandó los headers
			w.Header().Set(headerRequestID, requestID)

			rec := &logRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(rec, r.WithContext(ctx))
			logger.InfoContext(ctx, "http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

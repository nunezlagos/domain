package httpserver

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// RecoverMiddleware envuelve un http.Handler para que cualquier panic
// durante el procesamiento del request se recupere y se loggee con
// stack trace (slog nivel Error). El response al cliente es 500.
//
// issue-29.3 T5: evita que un panic en un handler tumbe el server
// silenciosamente (caso real donde un bug en un handler específico
// mata todas las requests concurrentes, no solo esa).
//
// Uso:
//
//	mux := http.NewServeMux()
//	mux.Handle("/foo", fooHandler)
//	handler := httpserver.RecoverMiddleware(logger)(mux)
//
//	logger puede ser nil (no loggea, pero igual responde 500).
func RecoverMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if logger != nil {
					logger.Error("PANIC recovered in HTTP handler",
						slog.Any("panic", rec),
						slog.String("stack", string(debug.Stack())),
						slog.String("path", r.URL.Path),
						slog.String("method", r.Method),
					)
				}
				// Si el handler ya empezó a escribir el response,
				// no podemos cambiar el status code. En ese caso
				// el cliente verá el response parcial.
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

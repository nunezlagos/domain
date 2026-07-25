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
			tracked := &panicTracker{ResponseWriter: w}
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

				// Si el handler YA respondió, un http.Error acá empeora las cosas:
				// el status no se puede cambiar (Go descarta el segundo WriteHeader)
				// pero el texto SÍ se concatena al body ya enviado. El cliente
				// recibía un 200 con "internal server error" pegado al final — JSON
				// corrupto en vez de un error legible. El panic ya quedó en el log;
				// la respuesta a medias se deja como está.
				if tracked.wrote {
					return
				}
				http.Error(tracked, "internal server error", http.StatusInternalServerError)
			}()
			next.ServeHTTP(tracked, r)
		})
	}
}

// panicTracker recuerda si el handler alcanzó a responder, para que el recover no
// pise una respuesta ya enviada.
type panicTracker struct {
	http.ResponseWriter
	wrote bool
}

func (t *panicTracker) WriteHeader(code int) {
	t.wrote = true
	t.ResponseWriter.WriteHeader(code)
}

func (t *panicTracker) Write(b []byte) (int, error) {
	t.wrote = true
	return t.ResponseWriter.Write(b)
}

// Flush y Unwrap replican el patrón de apikey.statusRecorder: sin ellos el SSE de
// /api/v1/events deja de funcionar detrás de este middleware, porque
// runner/streaming hace un type assertion directo a http.Flusher sobre el writer
// que recibe.
func (t *panicTracker) Flush() {
	if f, ok := t.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (t *panicTracker) Unwrap() http.ResponseWriter {
	return t.ResponseWriter
}

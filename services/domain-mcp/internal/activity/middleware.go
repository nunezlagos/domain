// issue-02.6 — middleware HTTP que auto-registra activity en cada mutación
// exitosa, con summaries human-readable consistentes (helper Summarize).
package activity

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// PrincipalFn extrae actor/org del request autenticado. Inyectada para no
// acoplar activity ↔ auth/apikey.
type PrincipalFn func(r *http.Request) (orgID uuid.UUID, actorID *uuid.UUID, ok bool)

// HTTPMiddleware auto-registra mutaciones (POST/PUT/PATCH/DELETE) con
// status 2xx. Lecturas y errores no generan activity.
type HTTPMiddleware struct {
	Recorder  Recorder
	Principal PrincipalFn
	Logger    *slog.Logger
}

// statusWriter captura el status code preservando http.Flusher (SSE).
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Flush() {
	if fl, ok := w.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

func (m *HTTPMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutating := r.Method == http.MethodPost || r.Method == http.MethodPut ||
			r.Method == http.MethodPatch || r.Method == http.MethodDelete
		if !mutating || strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			next.ServeHTTP(w, r)
			return
		}
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)

		if sw.status < 200 || sw.status >= 300 {
			return
		}
		orgID, actorID, ok := m.Principal(r)
		if !ok || m.Recorder == nil {
			return
		}
		action, entityType, entityID, summary := Summarize(r.Method, r.URL.Path)
		if action == "" {
			return
		}
		if _, err := m.Recorder.Record(context.WithoutCancel(r.Context()), Event{
			OrganizationID: orgID,
			ActorID:        actorID,
			Action:         action,
			EntityType:     entityType,
			EntityID:       entityID,
			Summary:        summary,
			Metadata:       map[string]any{"method": r.Method, "path": r.URL.Path},
		}); err != nil && m.Logger != nil {
			m.Logger.Warn("activity auto-record failed",
				slog.String("action", action), slog.String("error", err.Error()))
		}
	})
}

var verbES = map[string]string{
	"created": "creó", "updated": "actualizó", "deleted": "eliminó",
}

// Summarize deriva (action, entity_type, entity_id, summary) de una
// mutación HTTP. Convención: /api/v1/<recurso>[/<uuid>][/<acción>].
// Helper público para summaries consistentes (issue-02.6).
func Summarize(method, path string) (action, entityType string, entityID *uuid.UUID, summary string) {
	p := strings.TrimPrefix(path, "/api/v1/")
	if p == path || p == "" {
		return "", "", nil, "" // fuera del API versionada
	}
	segs := strings.Split(strings.Trim(p, "/"), "/")
	entityType = singularize(segs[0])

	var sub string
	for _, seg := range segs[1:] {
		if id, err := uuid.Parse(seg); err == nil {
			entityID = &id
			continue
		}
		sub = seg
	}

	verb := ""
	switch {
	case sub != "":
		verb = strings.ReplaceAll(sub, "-", "_") // run, pause, import, re-encrypt...
	case method == http.MethodPost:
		verb = "created"
	case method == http.MethodPatch, method == http.MethodPut:
		verb = "updated"
	case method == http.MethodDelete:
		verb = "deleted"
	}
	if verb == "" {
		return "", "", nil, ""
	}
	action = entityType + "." + verb

	human := verbES[verb]
	if human == "" {
		human = "ejecutó " + verb + " sobre"
	}
	if entityID != nil {
		summary = "Se " + human + " " + entityType + " " + shortID(*entityID)
	} else {
		summary = "Se " + human + " " + entityType
	}
	return action, entityType, entityID, summary
}

// singularize convierte el segmento de recurso kebab-plural al entity_type
// snake_case singular ("flow-runs" → "flow_run", "api-keys" → "api_key").
func singularize(resource string) string {
	s := strings.ReplaceAll(resource, "-", "_")
	switch {
	case strings.HasSuffix(s, "ies"):
		return strings.TrimSuffix(s, "ies") + "y"
	case strings.HasSuffix(s, "ses"):
		return strings.TrimSuffix(s, "es")
	case strings.HasSuffix(s, "s"):
		return strings.TrimSuffix(s, "s")
	}
	return s
}

func shortID(id uuid.UUID) string {
	return id.String()[:8]
}

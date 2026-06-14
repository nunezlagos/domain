// Package logging — issue-17.3 structured-logging.
// slog wrapping con campos contextuales (request_id, trace_id, user_id, org_id, project_id).
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
)

// ctxKey clave privada para values en context.
type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota + 1
	ctxKeyUserID
	ctxKeyOrgID
	ctxKeyProjectID
)

// dynamicLevel permite cambiar nivel en runtime (issue-17.3 escenario 2 + issue-27.3).
var dynamicLevel = &slog.LevelVar{}

// Config para Setup.
type Config struct {
	Level     string // debug | info | warn | error
	Format    string // text | json
	Output    string // stdout | stderr
	AddSource bool
}

// Setup crea logger root y lo asigna como default global.
func Setup(cfg Config) *slog.Logger {
	dynamicLevel.Set(parseLevel(cfg.Level))

	var w io.Writer = os.Stdout
	if strings.EqualFold(cfg.Output, "stderr") {
		w = os.Stderr
	}

	opts := &slog.HandlerOptions{
		Level:     dynamicLevel,
		AddSource: cfg.AddSource,
	}
	var handler slog.Handler
	if strings.EqualFold(cfg.Format, "json") {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}
	// envuelve para inyectar campos contextuales en cada log
	logger := slog.New(&ctxEnrichHandler{inner: handler})
	slog.SetDefault(logger)
	return logger
}

// SetLevel cambia nivel runtime (issue-17.3 dynamic level + issue-27.3 hot-reload).
func SetLevel(level string) {
	dynamicLevel.Set(parseLevel(level))
}

// CurrentLevel retorna nivel actual.
func CurrentLevel() string {
	switch dynamicLevel.Level() {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	}
	return "info"
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "err":
		return slog.LevelError
	}
	return slog.LevelInfo
}

// WithRequestID retorna ctx con request_id que se incluirá automáticamente en logs.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// WithUserID retorna ctx con user_id.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, id)
}

// WithOrgID retorna ctx con organization_id.
func WithOrgID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyOrgID, id)
}

// WithProjectID retorna ctx con project_id.
func WithProjectID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyProjectID, id)
}

// FromContext retorna logger por defecto (campos contextuales se inyectan vía handler).
func FromContext(_ context.Context) *slog.Logger {
	return slog.Default()
}

// ctxEnrichHandler agrega atributos request_id/user_id/org_id/project_id si presentes en ctx.
type ctxEnrichHandler struct{ inner slog.Handler }

func (h *ctxEnrichHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return h.inner.Enabled(ctx, lvl)
}

func (h *ctxEnrichHandler) Handle(ctx context.Context, r slog.Record) error {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok && v != "" {
		r.AddAttrs(slog.String("request_id", v))
	}
	if v, ok := ctx.Value(ctxKeyUserID).(string); ok && v != "" {
		r.AddAttrs(slog.String("user_id", v))
	}
	if v, ok := ctx.Value(ctxKeyOrgID).(string); ok && v != "" {
		r.AddAttrs(slog.String("organization_id", v))
	}
	if v, ok := ctx.Value(ctxKeyProjectID).(string); ok && v != "" {
		r.AddAttrs(slog.String("project_id", v))
	}
	return h.inner.Handle(ctx, r)
}

func (h *ctxEnrichHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ctxEnrichHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *ctxEnrichHandler) WithGroup(name string) slog.Handler {
	return &ctxEnrichHandler{inner: h.inner.WithGroup(name)}
}

// hotReloadCount track # de cambios de nivel (telemetry).
var hotReloadCount atomic.Int64

// HotReloadCount devuelve número total de cambios dinámicos.
func HotReloadCount() int64 { return hotReloadCount.Load() }

// ChangeLevel wrapper que incrementa contador + setea nivel.
func ChangeLevel(level string) {
	SetLevel(level)
	hotReloadCount.Add(1)
}

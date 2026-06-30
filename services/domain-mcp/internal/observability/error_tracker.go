// Package observability: este archivo cubre el registro y dedup de errores
// para early-error-reporting. Record categoriza, calcula fingerprint y hace
// upsert con dedup; tras un upsert exitoso dispara el AlertHook.
//
// issue-53.9 early-error-reporting.
package observability

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrorEvent es el payload a persistir/deduplicar en error_events.
type ErrorEvent struct {
	Source      string
	Category    Category
	Severity    string
	Message     string
	StackTrace  string
	Fingerprint []byte
	WorkflowID  string
	ProjectID   string
}

// ErrorEventStore abstrae la persistencia con dedup.
type ErrorEventStore interface {
	UpsertErrorEvent(ctx context.Context, e ErrorEvent) error
}

// AlertHook se invoca tras un upsert exitoso (alerting desacoplado).
type AlertHook func(ctx context.Context, e ErrorEvent)

// ErrorTracker registra errores categorizados con dedup por fingerprint.
type ErrorTracker struct {
	store   ErrorEventStore
	logger  *slog.Logger
	onAlert AlertHook
}

// NewErrorTracker construye el tracker. logger nil -> slog.Default().
func NewErrorTracker(store ErrorEventStore, logger *slog.Logger) *ErrorTracker {
	if logger == nil {
		logger = slog.Default()
	}
	return &ErrorTracker{store: store, logger: logger}
}

// SetAlertHook registra el callback de alerting (idempotente).
func (t *ErrorTracker) SetAlertHook(h AlertHook) { t.onAlert = h }

// Record categoriza el error, calcula su fingerprint y lo deduplica.
// Si el upsert falla, loguea WARN y NO dispara la alerta.
func (t *ErrorTracker) Record(ctx context.Context, err error, source string) {
	if err == nil {
		return
	}
	cat := Categorize(err)
	e := ErrorEvent{
		Source:      source,
		Category:    cat,
		Severity:    defaultSeverity(cat),
		Message:     err.Error(),
		Fingerprint: Fingerprint(cat, err.Error(), source, ""),
		WorkflowID:  WorkflowIDFromContext(ctx).String(),
	}
	if uerr := t.store.UpsertErrorEvent(ctx, e); uerr != nil {
		t.logger.Warn("error event persist failed",
			slog.String("source", source),
			slog.String("category", string(cat)),
			slog.String("error", uerr.Error()))
		return
	}
	if t.onAlert != nil {
		t.onAlert(ctx, e)
	}
}

// defaultSeverity mapea la categoria a una severidad inicial razonable.
// El operador puede sobreescribir por known_error.
func defaultSeverity(cat Category) string {
	switch cat {
	case CategoryPanic:
		return "critical"
	case CategorySQL, CategoryAuth, CategoryExternal:
		return "error"
	default:
		return "warn"
	}
}

// PGErrorEventStore persiste con dedup en error_events.
// Pool puede ser nil al inicio; setear via SetPool post-OpenProduction.
type PGErrorEventStore struct {
	Pool *pgxpool.Pool
}

// SetPool setea el pool (post-init).
func (s *PGErrorEventStore) SetPool(p *pgxpool.Pool) { s.Pool = p }

// UpsertErrorEvent inserta el evento o, si el fingerprint ya existe,
// incrementa dedup_count y refresca last_seen_at.
func (s *PGErrorEventStore) UpsertErrorEvent(ctx context.Context, e ErrorEvent) error {
	if s.Pool == nil {
		return ErrStoreNotReady
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO error_events
			(source, category, severity, message, stack_trace, fingerprint, workflow_id, project_id)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,NULLIF($7,''),NULLIF($8,'')::uuid)
		ON CONFLICT (fingerprint) DO UPDATE
		SET dedup_count = error_events.dedup_count + 1,
		    last_seen_at = now()
	`,
		e.Source, string(e.Category), e.Severity, e.Message,
		e.StackTrace, e.Fingerprint, e.WorkflowID, e.ProjectID,
	)
	return err
}

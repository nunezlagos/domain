// Package audit — issue-02.4 audit-log.
//
// Append-only. Diferenciado de activity_log (issue-02.6) según RFC 0003:
//  - audit_log: technical/compliance, field-level diffs, inmutable a nivel DB
//  - activity_log: product/UX, human summaries, mutable
//
// Esta API no permite UPDATE/DELETE; el role app_user tampoco (REVOKE en issue-25.6).
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Context keys para metadata de audit (issue-02.4). El AuditMiddleware las inyecta.
type ctxKey string

const (
	ctxKeyIP     ctxKey = "audit_ip"
	ctxKeyUA     ctxKey = "audit_ua"
	ctxKeyReqID  ctxKey = "audit_reqid"
)

// WithAuditMetadata inyecta IP/UA/ReqID en el context para audit.Record.
// Usado por el audit middleware.
func WithAuditMetadata(ctx context.Context, ip, ua, reqID string) context.Context {
	ctx = context.WithValue(ctx, ctxKeyIP, ip)
	ctx = context.WithValue(ctx, ctxKeyUA, ua)
	ctx = context.WithValue(ctx, ctxKeyReqID, reqID)
	return ctx
}

func auditIP(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyIP).(string)
	return v
}

func auditUA(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyUA).(string)
	return v
}

func auditReqID(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyReqID).(string)
	return v
}

// AuditEntry representa una fila de audit_log para queries.
type AuditEntry struct {
	ID             int64      `json:"id"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	ActorID        *uuid.UUID `json:"actor_id,omitempty"`
	ActorType      string     `json:"actor_type"`
	Action         string     `json:"action"`
	EntityType     string     `json:"entity_type"`
	EntityID       *uuid.UUID `json:"entity_id,omitempty"`
	OldValues      json.RawMessage `json:"old_values,omitempty"`
	NewValues      json.RawMessage `json:"new_values,omitempty"`
	IPAddress      string     `json:"ip_address,omitempty"`
	UserAgent      string     `json:"user_agent,omitempty"`
	RequestID      string     `json:"request_id,omitempty"`
	TraceID        string     `json:"trace_id,omitempty"`
	OccurredAt     time.Time  `json:"occurred_at"`
}

// AuditFilter filtros opcionales para Query.
type AuditFilter struct {
	OrganizationID *uuid.UUID
	ActorID        *uuid.UUID
	Action         string
	EntityType     string
	EntityID       *uuid.UUID
	Limit          int
	Cursor         int64 // last ID for cursor pagination
}

// ActorType quién ejecutó la acción.
type ActorType string

const (
	ActorUser          ActorType = "user"
	ActorSystem        ActorType = "system"
	ActorAPIKey        ActorType = "api_key"
	ActorPlatformAdmin ActorType = "platform_admin"
)

// Event datos a registrar en una entrada.
type Event struct {
	OrganizationID *uuid.UUID
	OriginOrgID    *uuid.UUID // org where the action originated (for multi-tenant audit)
	ActorID        *uuid.UUID
	ActorType      ActorType
	Action         string // "user.created", "api_key.rotated", etc.
	EntityType     string // "user", "api_key", "observation", etc.
	EntityID       *uuid.UUID
	OldValues      any // marshalled como JSONB; nil OK
	NewValues      any // marshalled como JSONB; nil OK
	IPAddress      string
	UserAgent      string
	RequestID      string
	TraceID        string
}

// Recorder graba eventos en `audit_log`. Implementaciones swappable para tests.
type Recorder interface {
	Record(ctx context.Context, e Event) error
}

// RecordOrLog persiste el evento via recorder y loggea el error si falla.
// Audit es best-effort por diseño: no debe bloquear el flujo principal, pero
// los misses deben quedar visibles en logs para alertas de compliance
// (HU-28.5). Si recorder es nil, no hace nada (cubre tests/dev sin audit
// configurado).
func RecordOrLog(ctx context.Context, recorder Recorder, e Event) {
	if recorder == nil {
		return
	}
	if err := recorder.Record(ctx, e); err != nil {
		entityID := ""
		if e.EntityID != nil {
			entityID = e.EntityID.String()
		}
		slog.WarnContext(ctx, "audit record failed",
			"error", err,
			"action", e.Action,
			"entity_type", e.EntityType,
			"entity_id", entityID,
		)
	}
}

// PGRecorder implementación Postgres con pgxpool.
type PGRecorder struct {
	Pool *pgxpool.Pool
}

// Record persiste el evento. Errores se devuelven al caller para que decida
// (no swallow): un audit miss en operaciones críticas debe loggear loud.
// IPAddress, UserAgent y RequestID se completan automáticamente desde el
// context si el AuditMiddleware está en la cadena (issue-02.4).
func (r *PGRecorder) Record(ctx context.Context, e Event) error {
	if e.Action == "" {
		return fmt.Errorf("audit: action required")
	}
	if e.EntityType == "" {
		return fmt.Errorf("audit: entity_type required")
	}
	if e.ActorType == "" {
		e.ActorType = ActorSystem
	}
	if e.IPAddress == "" {
		e.IPAddress = auditIP(ctx)
	}
	if e.UserAgent == "" {
		e.UserAgent = auditUA(ctx)
	}
	if e.RequestID == "" {
		e.RequestID = auditReqID(ctx)
	}

	var oldJSON, newJSON []byte
	if e.OldValues != nil {
		b, err := json.Marshal(e.OldValues)
		if err != nil {
			return fmt.Errorf("marshal old_values: %w", err)
		}
		oldJSON = b
	}
	if e.NewValues != nil {
		b, err := json.Marshal(e.NewValues)
		if err != nil {
			return fmt.Errorf("marshal new_values: %w", err)
		}
		newJSON = b
	}

	_, err := r.Pool.Exec(ctx, `
		INSERT INTO audit_log (
			organization_id, origin_org_id, actor_id, actor_type, action, entity_type, entity_id,
			old_values, new_values, ip_address, user_agent, request_id, trace_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`,
		e.OrganizationID, e.OriginOrgID, e.ActorID, string(e.ActorType), e.Action,
		e.EntityType, e.EntityID,
		oldJSON, newJSON,
		nullIfEmpty(e.IPAddress), nullIfEmpty(e.UserAgent),
		nullIfEmpty(e.RequestID), nullIfEmpty(e.TraceID),
	)
	if err != nil {
		return fmt.Errorf("audit insert: %w", err)
	}
	return nil
}

// Query retorna entries con filtros opcionales + cursor pagination.
func (r *PGRecorder) Query(ctx context.Context, filter AuditFilter) ([]AuditEntry, error) {
	where := "TRUE"
	args := []any{}
	argN := 0

	addArg := func(v any) string {
		argN++
		args = append(args, v)
		return fmt.Sprintf("$%d", argN)
	}

	if filter.OrganizationID != nil {
		where += " AND organization_id = " + addArg(*filter.OrganizationID)
	}
	if filter.ActorID != nil {
		where += " AND actor_id = " + addArg(*filter.ActorID)
	}
	if filter.Action != "" {
		where += " AND action = " + addArg(filter.Action)
	}
	if filter.EntityType != "" {
		where += " AND entity_type = " + addArg(filter.EntityType)
	}
	if filter.EntityID != nil {
		where += " AND entity_id = " + addArg(*filter.EntityID)
	}
	if filter.Cursor > 0 {
		where += " AND id < " + addArg(filter.Cursor)
	}

	limit := 50
	if filter.Limit > 0 && filter.Limit <= 500 {
		limit = filter.Limit
	}

	rows, err := r.Pool.Query(ctx, fmt.Sprintf(`
		SELECT id, organization_id, actor_id, actor_type, action,
		       entity_type, entity_id, old_values, new_values,
		       ip_address, user_agent, request_id, trace_id, occurred_at
		FROM audit_log
		WHERE %s
		ORDER BY id DESC
		LIMIT %d
	`, where, limit), args...)
	if err != nil {
		return nil, fmt.Errorf("audit query: %w", err)
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(
			&e.ID, &e.OrganizationID, &e.ActorID, &e.ActorType, &e.Action,
			&e.EntityType, &e.EntityID, &e.OldValues, &e.NewValues,
			&e.IPAddress, &e.UserAgent, &e.RequestID, &e.TraceID, &e.OccurredAt,
		); err != nil {
			return nil, fmt.Errorf("audit scan: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// Prune borra entries anteriores a `before`.
func (r *PGRecorder) Prune(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.Pool.Exec(ctx, `DELETE FROM audit_log WHERE occurred_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("audit prune: %w", err)
	}
	return tag.RowsAffected(), nil
}

// nullIfEmpty retorna nil si el string es vacío (para escribir NULL en lugar de '').
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// NopRecorder no-op para tests donde no queremos persistir.
type NopRecorder struct {
	Calls []Event
}

// Record agrega evento al slice in-memory.
func (r *NopRecorder) Record(_ context.Context, e Event) error {
	r.Calls = append(r.Calls, e)
	return nil
}

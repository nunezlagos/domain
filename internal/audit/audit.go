// Package audit — HU-02.4 audit-log.
//
// Append-only. Diferenciado de activity_log (HU-02.6) según RFC 0003:
//  - audit_log: technical/compliance, field-level diffs, inmutable a nivel DB
//  - activity_log: product/UX, human summaries, mutable
//
// Esta API no permite UPDATE/DELETE; el role app_user tampoco (REVOKE en HU-25.6).
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

// PGRecorder implementación Postgres con pgxpool.
type PGRecorder struct {
	Pool *pgxpool.Pool
}

// Record persiste el evento. Errores se devuelven al caller para que decida
// (no swallow): un audit miss en operaciones críticas debe loggear loud.
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
			organization_id, actor_id, actor_type, action, entity_type, entity_id,
			old_values, new_values, ip_address, user_agent, request_id, trace_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`,
		e.OrganizationID, e.ActorID, string(e.ActorType), e.Action,
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

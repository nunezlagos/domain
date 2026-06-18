// Package activity — issue-02.6 activity-log (user-facing).
//
// Diferenciado de audit_log (issue-02.4 / RFC 0003):
//   - activity_log: human summaries para UI (feeds, notifications)
//   - audit_log: technical compliance, field-level diffs, inmutable a DB level
//
// API permite UPDATE (corrección de visibility/metadata) y query con filtros
// por proyecto/usuario/entidad. Sin retention strict (parte del producto).
package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Visibility quién puede leer el evento.
type Visibility string

const (
	VisPublic  Visibility = "public"  // todos los users en org pueden ver
	VisOrg     Visibility = "org"     // members de org (default)
	VisProject Visibility = "project" // solo members del project
	VisPrivate Visibility = "private" // solo actor
)

// Event registro user-facing.
type Event struct {
	OrganizationID uuid.UUID
	ProjectID      *uuid.UUID
	ActorID        *uuid.UUID
	Action         string  // "observation.created", "agent.invited", etc.
	EntityType     string  // "observation", "agent", etc.
	EntityID       *uuid.UUID
	Summary        string  // "Alice creó observation X"
	Metadata       any     // shallow JSONB (sin PII full, ya redacted)
	Visibility     Visibility
}

// Filter para list/query.
type Filter struct {
	OrganizationID uuid.UUID
	ProjectID      *uuid.UUID
	ActorID        *uuid.UUID
	EntityType     string
	EntityID       *uuid.UUID
	Since          *time.Time
	Until          *time.Time
	Limit          int    // 0 = default 50
	BeforeID       string // cursor: filtrar by created_at < lookup(BeforeID).created_at
}

// Entry retorned de query (incluye id + created_at).
type Entry struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	ProjectID      *uuid.UUID
	ActorID        *uuid.UUID
	Action         string
	EntityType     string
	EntityID       *uuid.UUID
	Summary        string
	Metadata       map[string]any
	Visibility     Visibility
	CreatedAt      time.Time
}

// Recorder interface registra eventos.
type Recorder interface {
	Record(ctx context.Context, e Event) (uuid.UUID, error)
}

// Querier lookup con filtros.
type Querier interface {
	List(ctx context.Context, f Filter) ([]Entry, error)
}

// PGStore implementación Postgres.
type PGStore struct {
	Pool *pgxpool.Pool
}

// Record persiste evento. Errores se devuelven al caller (no swallow).
func (s *PGStore) Record(ctx context.Context, e Event) (uuid.UUID, error) {
	if e.Action == "" {
		return uuid.Nil, fmt.Errorf("activity: action required")
	}
	if e.EntityType == "" {
		return uuid.Nil, fmt.Errorf("activity: entity_type required")
	}
	if e.Summary == "" {
		return uuid.Nil, fmt.Errorf("activity: summary required")
	}
	if e.Visibility == "" {
		e.Visibility = VisOrg
	}

	var metaJSON []byte
	if e.Metadata != nil {
		b, err := json.Marshal(e.Metadata)
		if err != nil {
			return uuid.Nil, fmt.Errorf("marshal metadata: %w", err)
		}
		metaJSON = b
	} else {
		metaJSON = []byte(`{}`)
	}

	// ISSUE-21.6: INSERT sin organization_id.
	var id uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO activity_log (
			project_id, actor_id, action, entity_type, entity_id,
			summary, metadata, visibility
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`,
		e.ProjectID, e.ActorID,
		e.Action, e.EntityType, e.EntityID,
		e.Summary, metaJSON, string(e.Visibility),
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("activity insert: %w", err)
	}
	return id, nil
}

// List devuelve entries con filtros aplicados.
func (s *PGStore) List(ctx context.Context, f Filter) ([]Entry, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	// ISSUE-21.6: SELECT sin organization_id.
	_ = f.OrganizationID
	args := []any{}
	q := `
		SELECT id, project_id, actor_id, action, entity_type, entity_id,
		       summary, metadata, visibility, created_at
		FROM activity_log
		WHERE TRUE`

	if f.ProjectID != nil {
		q += fmt.Sprintf(" AND project_id = $%d", len(args)+1)
		args = append(args, *f.ProjectID)
	}
	if f.ActorID != nil {
		args = append(args, *f.ActorID)
		q += fmt.Sprintf(" AND actor_id = $%d", len(args))
	}
	if f.EntityType != "" {
		args = append(args, f.EntityType)
		q += fmt.Sprintf(" AND entity_type = $%d", len(args))
	}
	if f.EntityID != nil {
		args = append(args, *f.EntityID)
		q += fmt.Sprintf(" AND entity_id = $%d", len(args))
	}
	if f.Since != nil {
		args = append(args, *f.Since)
		q += fmt.Sprintf(" AND created_at >= $%d", len(args))
	}
	if f.Until != nil {
		args = append(args, *f.Until)
		q += fmt.Sprintf(" AND created_at < $%d", len(args))
	}
	q += " ORDER BY created_at DESC"
	args = append(args, limit)
	q += fmt.Sprintf(" LIMIT $%d", len(args))

	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("activity list: %w", err)
	}
	defer rows.Close()

	out := make([]Entry, 0, limit)
	for rows.Next() {
		var e Entry
		var metaJSON []byte
		var vis string
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.ActorID,
			&e.Action, &e.EntityType, &e.EntityID,
			&e.Summary, &metaJSON, &vis, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("activity scan: %w", err)
		}
		e.Visibility = Visibility(vis)
		if len(metaJSON) > 0 {
			_ = json.Unmarshal(metaJSON, &e.Metadata)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// NopRecorder in-memory para tests.
type NopRecorder struct {
	Calls []Event
}

// Record agrega evento al slice.
func (r *NopRecorder) Record(_ context.Context, e Event) (uuid.UUID, error) {
	r.Calls = append(r.Calls, e)
	return uuid.New(), nil
}

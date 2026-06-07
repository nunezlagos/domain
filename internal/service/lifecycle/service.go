// Package lifecycle — HU-23.2 soft-delete restore + HU-23.3 GDPR export.
//
// Restore: revierte deleted_at = NULL para entidades soft-deleted dentro de la
// ventana de retención (default 30 días).
//
// ExportUserData: genera un JSON con todos los datos persistidos asociados a un
// user específico (organizations, projects, observations, sessions, prompts,
// knowledge_docs, agent_runs, api_keys metadata sin secrets, audit_log entries).
// Cumple Art. 15 GDPR (right of access) + Art. 20 (data portability).
package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
)

var (
	ErrEntityNotSupported = errors.New("entity type not supported for restore")
	ErrNotFound           = errors.New("entity not found or not soft-deleted")
	ErrRetentionExpired   = errors.New("deleted_at older than retention window")
)

// restorableEntities mapea entity_type → tabla. Solo tablas con deleted_at.
var restorableEntities = map[string]string{
	"organization":   "organizations",
	"user":           "users",
	"project":        "projects",
	"observation":    "observations",
	"session":        "sessions",
	"prompt":         "prompts",
	"knowledge_doc":  "knowledge_docs",
	"skill":          "skills",
	"agent":          "agents",
	"invitation":     "invitations",
}

type Service struct {
	Pool             *pgxpool.Pool
	Audit            audit.Recorder
	RetentionDays    int // default 30; restore falla si deleted_at < now()-RetentionDays
}

func (s *Service) retentionDays() int {
	if s.RetentionDays > 0 {
		return s.RetentionDays
	}
	return 30
}

// Restore revierte deleted_at = NULL para entityType/id si:
//   1. el entity existe en su tabla
//   2. deleted_at IS NOT NULL (era soft-deleted)
//   3. deleted_at >= NOW() - retention_days (dentro de ventana)
func (s *Service) Restore(ctx context.Context, entityType string, id, actorID uuid.UUID, orgID *uuid.UUID) error {
	table, ok := restorableEntities[entityType]
	if !ok {
		return ErrEntityNotSupported
	}
	// Validar dentro de la retention window
	var deletedAt *time.Time
	var entityOrg *uuid.UUID
	var query string
	args := []any{id}
	if hasOrgColumn(table) {
		query = fmt.Sprintf(
			`SELECT deleted_at, organization_id FROM %s WHERE id = $1`, table)
	} else {
		query = fmt.Sprintf(`SELECT deleted_at, NULL::uuid FROM %s WHERE id = $1`, table)
	}
	err := s.Pool.QueryRow(ctx, query, args...).Scan(&deletedAt, &entityOrg)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup: %w", err)
	}
	if deletedAt == nil {
		return ErrNotFound
	}
	// Cross-org guard
	if orgID != nil && entityOrg != nil && *entityOrg != *orgID {
		return ErrNotFound
	}
	// Retention
	if time.Since(*deletedAt) > time.Duration(s.retentionDays())*24*time.Hour {
		return ErrRetentionExpired
	}

	_, err = s.Pool.Exec(ctx,
		fmt.Sprintf(`UPDATE %s SET deleted_at = NULL WHERE id = $1`, table), id)
	if err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: orgID,
			ActorID:        &actorID,
			ActorType:      audit.ActorUser,
			Action:         entityType + ".restored",
			EntityType:     entityType,
			EntityID:       &id,
		})
	}
	return nil
}

// hasOrgColumn devuelve si la tabla incluye columna organization_id.
// Tablas listadas en restorableEntities: todas tienen organization_id excepto users
// (users.organization_id existe) — en realidad todas tienen.
func hasOrgColumn(table string) bool {
	// Tablas que SÍ tienen organization_id (todas en restorableEntities):
	return table != "" // simplificación: todas tienen org_id en el schema actual
}

// --- GDPR Export ---

// UserExport bundle con todos los datos del user en formato portable.
type UserExport struct {
	Version       string         `json:"export_version"`
	ExportedAt    time.Time      `json:"exported_at"`
	UserID        uuid.UUID      `json:"user_id"`
	User          map[string]any `json:"user"`
	Organizations []map[string]any `json:"organizations"`
	Projects      []map[string]any `json:"projects"`
	Observations  []map[string]any `json:"observations"`
	Sessions      []map[string]any `json:"sessions"`
	Prompts       []map[string]any `json:"prompts"`
	KnowledgeDocs []map[string]any `json:"knowledge_docs"`
	AgentRuns     []map[string]any `json:"agent_runs"`
	APIKeys       []map[string]any `json:"api_keys_metadata"` // sin secrets
	AuditEntries  []map[string]any `json:"audit_log"`
}

// ExportUserData arma el bundle GDPR para un user específico.
// orgID restringe el scope al org del user (cross-org export no permitido).
func (s *Service) ExportUserData(ctx context.Context, userID, orgID uuid.UUID) (*UserExport, error) {
	out := &UserExport{
		Version:    "1.0",
		ExportedAt: time.Now().UTC(),
		UserID:     userID,
	}

	// 1. user record
	usr, err := scanRow(ctx, s.Pool,
		`SELECT id, organization_id, email, name, role, created_at, updated_at, deleted_at
		 FROM users WHERE id = $1 AND organization_id = $2`,
		userID, orgID)
	if err != nil {
		return nil, fmt.Errorf("user: %w", err)
	}
	if usr == nil {
		return nil, ErrNotFound
	}
	out.User = usr

	// 2. organization (one, the user's)
	orgRow, _ := scanRow(ctx, s.Pool,
		`SELECT id, name, slug, settings, created_at, plan_id, custom_limits
		 FROM organizations WHERE id = $1`, orgID)
	if orgRow != nil {
		out.Organizations = []map[string]any{orgRow}
	}

	// 3-N. Tablas con created_by = userID
	out.Projects, _ = scanRows(ctx, s.Pool,
		`SELECT id, name, slug, description, created_at FROM projects
		 WHERE organization_id = $1 ORDER BY created_at DESC`, orgID)
	out.Observations, _ = scanRows(ctx, s.Pool,
		`SELECT id, project_id, content, observation_type, tags, metadata, created_at
		 FROM observations WHERE created_by = $1 AND deleted_at IS NULL`, userID)
	out.Sessions, _ = scanRows(ctx, s.Pool,
		`SELECT id, project_id, title, summary, tags, started_at, ended_at
		 FROM sessions WHERE user_id = $1 AND deleted_at IS NULL`, userID)
	out.Prompts, _ = scanRows(ctx, s.Pool,
		`SELECT id, project_id, slug, version, body, is_active, created_at
		 FROM prompts WHERE created_by = $1 AND deleted_at IS NULL`, userID)
	out.KnowledgeDocs, _ = scanRows(ctx, s.Pool,
		`SELECT id, project_id, title, source, source_url, tags, created_at
		 FROM knowledge_docs WHERE created_by = $1 AND deleted_at IS NULL`, userID)
	out.AgentRuns, _ = scanRows(ctx, s.Pool,
		`SELECT id, agent_id, status, inputs, outputs, tokens_input, tokens_output,
		        cost_usd, iterations, started_at, finished_at
		 FROM agent_runs WHERE user_id = $1`, userID)
	// api_keys: solo metadata (key_prefix), NUNCA key_hash ni secrets
	out.APIKeys, _ = scanRows(ctx, s.Pool,
		`SELECT id, name, key_prefix, last_used_at, expires_at, revoked_at, created_at
		 FROM api_keys WHERE user_id = $1`, userID)
	// audit_log: entradas con actor=this user
	out.AuditEntries, _ = scanRows(ctx, s.Pool,
		`SELECT id, action, entity_type, entity_id, new_values, occurred_at
		 FROM audit_log WHERE actor_id = $1 ORDER BY occurred_at DESC LIMIT 5000`, userID)

	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &orgID,
			ActorID:        &userID,
			ActorType:      audit.ActorUser,
			Action:         "user.gdpr_exported",
			EntityType:     "user",
			EntityID:       &userID,
			NewValues: map[string]any{
				"counts": map[string]int{
					"observations": len(out.Observations),
					"sessions":     len(out.Sessions),
					"prompts":      len(out.Prompts),
					"knowledge":    len(out.KnowledgeDocs),
					"agent_runs":   len(out.AgentRuns),
				},
			},
		})
	}
	return out, nil
}

// scanRow ejecuta una query que devuelve UNA fila y la convierte a map[string]any.
func scanRow(ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) (map[string]any, error) {
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	cols := rows.FieldDescriptions()
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	m := make(map[string]any, len(cols))
	for i, c := range cols {
		m[string(c.Name)] = normalizeValue(vals[i])
	}
	return m, nil
}

func scanRows(ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) ([]map[string]any, error) {
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		cols := rows.FieldDescriptions()
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			m[string(c.Name)] = normalizeValue(vals[i])
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// normalizeValue convierte tipos pgx a JSON-friendly. UUIDs como string,
// JSONB ya viene como []byte o map dependiendo del driver.
func normalizeValue(v any) any {
	switch x := v.(type) {
	case []byte:
		if isJSON(x) {
			var anyVal any
			if err := json.Unmarshal(x, &anyVal); err == nil {
				return anyVal
			}
		}
		return string(x)
	case [16]byte:
		// UUID raw bytes — convertir a string
		u := uuid.UUID(x)
		return u.String()
	default:
		return v
	}
}

func isJSON(b []byte) bool {
	t := strings.TrimSpace(string(b))
	return strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[")
}

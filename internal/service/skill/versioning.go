// issue-05.3 skill-versioning — snapshots inmutables de skills con pin/rollback.
//
// Tabla skill_versions ya existe desde migration 000011. issue-25.5 partial
// agregó column pinned_version en skills.
package skill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Version representa un snapshot persistido.
type Version struct {
	ID           uuid.UUID       `json:"id"`
	SkillID      uuid.UUID       `json:"skill_id"`
	Version      int             `json:"version"`
	Content      *string         `json:"content,omitempty"`
	InputSchema  json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema json.RawMessage `json:"output_schema,omitempty"`
	Changelog    *string         `json:"changelog,omitempty"`
	CreatedBy    *uuid.UUID      `json:"created_by,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

var ErrSkillVersionNotFound = errors.New("skill version not found")

// VersionStore CRUD sobre skill_versions.
type VersionStore struct {
	Pool *pgxpool.Pool
}

// Create persiste una nueva versión auto-incrementando version.
func (s *VersionStore) Create(ctx context.Context, skillID uuid.UUID, content *string, inputSchema, outputSchema []byte, changelog *string, createdBy *uuid.UUID) (*Version, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var nextVersion int
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(version), 0) + 1 FROM skill_versions WHERE skill_id = $1`, skillID,
	).Scan(&nextVersion)
	if err != nil {
		return nil, fmt.Errorf("max version: %w", err)
	}

	var v Version
	err = tx.QueryRow(ctx, `
		INSERT INTO skill_versions (skill_id, version, content, input_schema, output_schema, changelog, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, skill_id, version, content, input_schema, output_schema, changelog, created_by, created_at`,
		skillID, nextVersion, content, inputSchema, outputSchema, changelog, createdBy,
	).Scan(&v.ID, &v.SkillID, &v.Version, &v.Content, &v.InputSchema, &v.OutputSchema,
		&v.Changelog, &v.CreatedBy, &v.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &v, nil
}

// Get devuelve una versión específica.
func (s *VersionStore) Get(ctx context.Context, skillID uuid.UUID, version int) (*Version, error) {
	var v Version
	err := s.Pool.QueryRow(ctx, `
		SELECT id, skill_id, version, content, input_schema, output_schema, changelog, created_by, created_at
		FROM skill_versions WHERE skill_id = $1 AND version = $2`,
		skillID, version,
	).Scan(&v.ID, &v.SkillID, &v.Version, &v.Content, &v.InputSchema, &v.OutputSchema,
		&v.Changelog, &v.CreatedBy, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSkillVersionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	return &v, nil
}

// List devuelve todas las versions del skill orden DESC.
func (s *VersionStore) List(ctx context.Context, skillID uuid.UUID) ([]Version, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, skill_id, version, content, input_schema, output_schema, changelog, created_by, created_at
		FROM skill_versions WHERE skill_id = $1 ORDER BY version DESC`,
		skillID,
	)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.SkillID, &v.Version, &v.Content, &v.InputSchema,
			&v.OutputSchema, &v.Changelog, &v.CreatedBy, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// PinVersion setea pinned_version en el skill. Calls que invoquen el skill
// usan esta versión específica en lugar de la latest.
func (s *VersionStore) PinVersion(ctx context.Context, skillID uuid.UUID, version int) error {
	// Valida que la versión existe
	if _, err := s.Get(ctx, skillID, version); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx,
		`UPDATE skills SET pinned_version = $1 WHERE id = $2`, version, skillID,
	)
	if err != nil {
		return fmt.Errorf("pin: %w", err)
	}
	return nil
}

// Unpin remueve el pin para volver a usar la versión latest.
func (s *VersionStore) Unpin(ctx context.Context, skillID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE skills SET pinned_version = NULL WHERE id = $1`, skillID,
	)
	if err != nil {
		return fmt.Errorf("unpin: %w", err)
	}
	return nil
}

// Effective devuelve la version que se usa para invocations: pinned si está,
// sino la latest. Retorna ErrSkillVersionNotFound si no hay ninguna.
func (s *VersionStore) Effective(ctx context.Context, skillID uuid.UUID) (*Version, error) {
	var pinned *int
	err := s.Pool.QueryRow(ctx,
		`SELECT pinned_version FROM skills WHERE id = $1`, skillID,
	).Scan(&pinned)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSkillVersionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lookup skill: %w", err)
	}
	if pinned != nil {
		return s.Get(ctx, skillID, *pinned)
	}
	versions, err := s.List(ctx, skillID)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, ErrSkillVersionNotFound
	}
	return &versions[0], nil
}

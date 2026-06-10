// HU-09.7 workflow-versioning — versiona definitions de flow para que
// runs en curso usen la versión con la que arrancaron y nuevas runs usen
// la latest.
package flow

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

// FlowVersion es un snapshot inmutable de la definition de un flow.
// Se persiste al UPSERT del flow; cada run referencia version_id.
type FlowVersion struct {
	ID         uuid.UUID       `json:"id"`
	FlowID     uuid.UUID       `json:"flow_id"`
	Version    int             `json:"version"`
	Definition json.RawMessage `json:"definition"`
	Hash       string          `json:"hash"`
	CreatedBy  *uuid.UUID      `json:"created_by,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	Note       string          `json:"note,omitempty"`
}

// VersioningStore gestiona snapshots versionados de flow definitions.
type VersioningStore struct {
	Pool *pgxpool.Pool
}

// ErrFlowVersionNotFound se retorna cuando la version solicitada no existe.
var ErrFlowVersionNotFound = errors.New("flow version not found")

// NewVersion crea una nueva versión (versión n+1) snapshot de definition.
// Si hash coincide con la última, no crea nueva (idempotent).
func (s *VersioningStore) NewVersion(ctx context.Context, flowID uuid.UUID, definition []byte, hash, note string, createdBy *uuid.UUID) (*FlowVersion, error) {
	var existing FlowVersion
	err := s.Pool.QueryRow(ctx,
		`SELECT id, flow_id, version, definition, hash, created_by, created_at, COALESCE(note, '')
		 FROM flow_versions WHERE flow_id = $1 ORDER BY version DESC LIMIT 1`, flowID,
	).Scan(&existing.ID, &existing.FlowID, &existing.Version, &existing.Definition,
		&existing.Hash, &existing.CreatedBy, &existing.CreatedAt, &existing.Note)
	if err == nil && existing.Hash == hash {
		return &existing, nil // no-op: misma definition
	}

	nextVersion := 1
	if err == nil {
		nextVersion = existing.Version + 1
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("lookup latest: %w", err)
	}

	var v FlowVersion
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO flow_versions (flow_id, version, definition, hash, created_by, note)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''))
		RETURNING id, flow_id, version, definition, hash, created_by, created_at, COALESCE(note, '')`,
		flowID, nextVersion, definition, hash, createdBy, note,
	).Scan(&v.ID, &v.FlowID, &v.Version, &v.Definition, &v.Hash, &v.CreatedBy, &v.CreatedAt, &v.Note)
	if err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}
	return &v, nil
}

// GetVersion retorna una versión específica.
func (s *VersioningStore) GetVersion(ctx context.Context, flowID uuid.UUID, version int) (*FlowVersion, error) {
	var v FlowVersion
	err := s.Pool.QueryRow(ctx,
		`SELECT id, flow_id, version, definition, hash, created_by, created_at, COALESCE(note, '')
		 FROM flow_versions WHERE flow_id = $1 AND version = $2`,
		flowID, version,
	).Scan(&v.ID, &v.FlowID, &v.Version, &v.Definition, &v.Hash, &v.CreatedBy, &v.CreatedAt, &v.Note)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFlowVersionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	return &v, nil
}

// ListVersions devuelve todas las versions DESC.
func (s *VersioningStore) ListVersions(ctx context.Context, flowID uuid.UUID, limit int) ([]FlowVersion, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT id, flow_id, version, definition, hash, created_by, created_at, COALESCE(note, '')
		 FROM flow_versions WHERE flow_id = $1 ORDER BY version DESC LIMIT $2`,
		flowID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var out []FlowVersion
	for rows.Next() {
		var v FlowVersion
		if err := rows.Scan(&v.ID, &v.FlowID, &v.Version, &v.Definition, &v.Hash,
			&v.CreatedBy, &v.CreatedAt, &v.Note); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetLatest devuelve la versión más reciente.
func (s *VersioningStore) GetLatest(ctx context.Context, flowID uuid.UUID) (*FlowVersion, error) {
	versions, err := s.ListVersions(ctx, flowID, 1)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, ErrFlowVersionNotFound
	}
	return &versions[0], nil
}

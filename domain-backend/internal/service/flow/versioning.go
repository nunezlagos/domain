// issue-09.7 workflow-versioning — versiona definitions de flow para que
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

// FindByHash localiza una versión por hash de definition (cualquier posición,
// no solo la última). Para run pinning idempotente (fv-008).
func (s *VersioningStore) FindByHash(ctx context.Context, flowID uuid.UUID, hash string) (*FlowVersion, error) {
	var v FlowVersion
	err := s.Pool.QueryRow(ctx,
		`SELECT id, flow_id, version, definition, hash, created_by, created_at, COALESCE(note, '')
		 FROM flow_versions WHERE flow_id = $1 AND hash = $2
		 ORDER BY version DESC LIMIT 1`,
		flowID, hash,
	).Scan(&v.ID, &v.FlowID, &v.Version, &v.Definition, &v.Hash, &v.CreatedBy, &v.CreatedAt, &v.Note)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFlowVersionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find by hash: %w", err)
	}
	return &v, nil
}

// ArchiveDeprecated elimina versiones deprecated hace más de retention que
// ningún run referencia (fv-009, cron diario). Nunca toca la default.
func (s *VersioningStore) ArchiveDeprecated(ctx context.Context, retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention)
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM flow_versions v
		WHERE v.status = 'deprecated'
		  AND v.is_default = false
		  AND v.deprecated_at IS NOT NULL
		  AND v.deprecated_at < $1
		  AND NOT EXISTS (
			SELECT 1 FROM flow_runs r WHERE r.flow_version_id = v.id
		  )`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("archive deprecated: %w", err)
	}
	return tag.RowsAffected(), nil
}

var (
	ErrVersionAlreadyPublished  = errors.New("version already published")
	ErrVersionAlreadyDeprecated = errors.New("version already deprecated")
	ErrVersionDraftCannotInvoke = errors.New("draft version cannot be invoked")
	ErrVersionDeprecatedCannotInvoke = errors.New("deprecated version cannot be invoked")
)

// VersionStatus representa el estado de publicación de una versión.
type VersionStatus string

const (
	VersionDraft      VersionStatus = "draft"
	VersionPublished  VersionStatus = "published"
	VersionDeprecated VersionStatus = "deprecated"
)

// Change describe un cambio entre dos versiones.
type Change struct {
	Type     string      `json:"type"`     // "added", "removed", "modified"
	Path     string      `json:"path"`     // ej: "steps.s1", "steps.s1.type"
	OldValue interface{} `json:"old_value,omitempty"`
	NewValue interface{} `json:"new_value,omitempty"`
	Breaking bool        `json:"breaking"`
}

// FlowVersionDiff contiene el resultado de comparar dos versiones.
type FlowVersionDiff struct {
	FromVersion int      `json:"from_version"`
	ToVersion   int      `json:"to_version"`
	Changes     []Change `json:"changes"`
}

// BreakingChange describe un cambio breaking detectado.
type BreakingChange struct {
	Description string `json:"description"`
	Severity    string `json:"severity"` // "major" | "minor"
}

// PublishVersion cambia el estado a published y lo marca como default.
// En la misma tx: quita is_default de todas las versiones del flow,
// luego setea is_default=true + published_at=now() en la target.
func (s *VersioningStore) PublishVersion(ctx context.Context, flowID uuid.UUID, version int) (*FlowVersion, error) {
	var curStatus VersionStatus
	err := s.Pool.QueryRow(ctx,
		`SELECT COALESCE(NULLIF(status, ''), 'draft')::VARCHAR(20) FROM flow_versions
		 WHERE flow_id = $1 AND version = $2`,
		flowID, version,
	).Scan(&curStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFlowVersionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("check status: %w", err)
	}
	switch curStatus {
	case VersionPublished:
		return nil, ErrVersionAlreadyPublished
	case VersionDeprecated:
		return nil, ErrVersionAlreadyDeprecated
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE flow_versions SET is_default = false WHERE flow_id = $1`, flowID,
	); err != nil {
		return nil, fmt.Errorf("unset defaults: %w", err)
	}
	_, err = tx.Exec(ctx,
		`UPDATE flow_versions
		 SET status = 'published', is_default = true, published_at = NOW()
		 WHERE flow_id = $1 AND version = $2`,
		flowID, version,
	)
	if err != nil {
		return nil, fmt.Errorf("publish version: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return s.GetVersion(ctx, flowID, version)
}

// DeprecateVersion marca una versión como deprecated.
func (s *VersioningStore) DeprecateVersion(ctx context.Context, flowID uuid.UUID, version int) (*FlowVersion, error) {
	var curStatus VersionStatus
	err := s.Pool.QueryRow(ctx,
		`SELECT COALESCE(NULLIF(status, ''), 'draft')::VARCHAR(20) FROM flow_versions
		 WHERE flow_id = $1 AND version = $2`,
		flowID, version,
	).Scan(&curStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFlowVersionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("check status: %w", err)
	}
	if curStatus == VersionDeprecated {
		return nil, ErrVersionAlreadyDeprecated
	}

	tag, err := s.Pool.Exec(ctx,
		`UPDATE flow_versions
		 SET status = 'deprecated', deprecated_at = NOW(), is_default = false
		 WHERE flow_id = $1 AND version = $2
		   AND COALESCE(NULLIF(status, ''), 'draft') != 'deprecated'`,
		flowID, version,
	)
	if err != nil {
		return nil, fmt.Errorf("deprecate version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrFlowVersionNotFound
	}
	return s.GetVersion(ctx, flowID, version)
}

// IsVersionInvokable verifica si una versión puede ejecutarse.
func (s *VersioningStore) IsVersionInvokable(ctx context.Context, flowID uuid.UUID, version int) error {
	var status VersionStatus
	err := s.Pool.QueryRow(ctx,
		`SELECT COALESCE(NULLIF(status, ''), 'draft')::VARCHAR(20) FROM flow_versions
		 WHERE flow_id = $1 AND version = $2`,
		flowID, version,
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrFlowVersionNotFound
	}
	if err != nil {
		return fmt.Errorf("check invokable: %w", err)
	}
	switch status {
	case VersionDraft:
		return ErrVersionDraftCannotInvoke
	case VersionDeprecated:
		return ErrVersionDeprecatedCannotInvoke
	case VersionPublished:
		return nil
	default:
		return fmt.Errorf("unknown version status: %s", status)
	}
}

// GetPublishedVersion retorna la versión publicada (default) del flow.
func (s *VersioningStore) GetPublishedVersion(ctx context.Context, flowID uuid.UUID) (*FlowVersion, error) {
	var v FlowVersion
	err := s.Pool.QueryRow(ctx, `
		SELECT id, flow_id, version, definition, hash, created_by, created_at, COALESCE(note, '')
		FROM flow_versions
		WHERE flow_id = $1 AND is_default = true AND status = 'published'
		LIMIT 1`,
		flowID,
	).Scan(&v.ID, &v.FlowID, &v.Version, &v.Definition, &v.Hash,
		&v.CreatedBy, &v.CreatedAt, &v.Note)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFlowVersionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get published: %w", err)
	}
	return &v, nil
}

// DiffVersions compara dos versiones y retorna los cambios estructurales.
func (s *VersioningStore) DiffVersions(ctx context.Context, flowID uuid.UUID, fromVersion, toVersion int) (*FlowVersionDiff, error) {
	fromV, err := s.GetVersion(ctx, flowID, fromVersion)
	if err != nil {
		return nil, fmt.Errorf("from version: %w", err)
	}
	toV, err := s.GetVersion(ctx, flowID, toVersion)
	if err != nil {
		return nil, fmt.Errorf("to version: %w", err)
	}
	return &FlowVersionDiff{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
		Changes:     diffSpecs(fromV.Definition, toV.Definition),
	}, nil
}

// diffSpecs compara dos definiciones JSON de Spec y retorna los cambios.
func diffSpecs(from, to json.RawMessage) []Change {
	var fromSpec, toSpec Spec
	fromOK := json.Unmarshal(from, &fromSpec) == nil
	toOK := json.Unmarshal(to, &toSpec) == nil
	if !fromOK || !toOK {
		if string(from) != string(to) {
			return []Change{{Type: "modified", Path: "(raw)", Breaking: true}}
		}
		return nil
	}

	fromSteps := map[string]Step{}
	for _, s := range fromSpec.Steps {
		fromSteps[s.ID] = s
	}
	toSteps := map[string]Step{}
	for _, s := range toSpec.Steps {
		toSteps[s.ID] = s
	}

	var changes []Change

	for id, s := range fromSteps {
		if _, ok := toSteps[id]; !ok {
			changes = append(changes, Change{
				Type: "removed", Path: "steps." + id,
				OldValue: s, Breaking: true,
			})
		}
	}

	for id, s := range toSteps {
		oldS, ok := fromSteps[id]
		if !ok {
			changes = append(changes, Change{
				Type: "added", Path: "steps." + id,
				NewValue: s, Breaking: false,
			})
		} else if s.Type != oldS.Type {
			changes = append(changes, Change{
				Type: "modified", Path: "steps." + id + ".type",
				OldValue: oldS.Type, NewValue: s.Type, Breaking: true,
			})
		}
	}

	return changes
}

// DetectBreakingChanges analiza un diff y retorna las breaking changes.
func DetectBreakingChanges(diff *FlowVersionDiff) []BreakingChange {
	var out []BreakingChange
	for _, c := range diff.Changes {
		if !c.Breaking {
			continue
		}
		switch c.Type {
		case "removed":
			out = append(out, BreakingChange{
				Description: fmt.Sprintf("Step %s was removed", c.Path),
				Severity:    "major",
			})
		case "modified":
			out = append(out, BreakingChange{
				Description: fmt.Sprintf("Step %s type changed from %v to %v", c.Path, c.OldValue, c.NewValue),
				Severity:    "major",
			})
		}
	}
	return out
}

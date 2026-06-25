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

	"nunezlagos/domain/internal/service/skill/skilldb"
	"nunezlagos/domain/internal/store/txctx"
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

func toVersion(v skilldb.SkillVersion) *Version {
	return &Version{
		ID:           v.ID,
		SkillID:      v.SkillID,
		Version:      int(v.Version),
		Content:      v.Content,
		InputSchema:  json.RawMessage(v.InputSchema),
		OutputSchema: json.RawMessage(v.OutputSchema),
		Changelog:    v.Changelog,
		CreatedBy:    v.CreatedBy,
		CreatedAt:    v.CreatedAt,
	}
}

// VersionStore CRUD sobre skill_versions.
type VersionStore struct {
	Pool *pgxpool.Pool
}

func (s *VersionStore) q(ctx context.Context) *skilldb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return skilldb.New(tx)
	}
	return skilldb.New(s.Pool)
}

// Create persiste una nueva versión auto-incrementando version.
func (s *VersionStore) Create(ctx context.Context, skillID uuid.UUID, content *string, inputSchema, outputSchema []byte, changelog *string, createdBy *uuid.UUID) (*Version, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	q := skilldb.New(tx)

	nextVersion, err := q.VersionMaxVersion(ctx, skillID)
	if err != nil {
		return nil, fmt.Errorf("max version: %w", err)
	}

	v, err := q.VersionCreate(ctx, skilldb.VersionCreateParams{
		SkillID:      skillID,
		Version:      nextVersion,
		Content:      content,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Changelog:    changelog,
		CreatedBy:    createdBy,
	})
	if err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return toVersion(v), nil
}

// Get devuelve una versión específica.
func (s *VersionStore) Get(ctx context.Context, skillID uuid.UUID, version int) (*Version, error) {
	v, err := s.q(ctx).VersionGetBySkillAndVersion(ctx, skilldb.VersionGetBySkillAndVersionParams{
		SkillID: skillID,
		Version: int32(version),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSkillVersionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	return toVersion(v), nil
}

// List devuelve todas las versions del skill orden DESC.
func (s *VersionStore) List(ctx context.Context, skillID uuid.UUID) ([]Version, error) {
	rows, err := s.q(ctx).VersionListBySkill(ctx, skillID)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	out := make([]Version, len(rows))
	for i, r := range rows {
		out[i] = *toVersion(r)
	}
	return out, nil
}

// PinVersion setea pinned_version en el skill. Calls que invoquen el skill
// usan esta versión específica en lugar de la latest.
func (s *VersionStore) PinVersion(ctx context.Context, skillID uuid.UUID, version int) error {
	if _, err := s.Get(ctx, skillID, version); err != nil {
		return err
	}
	v := int32(version)
	if err := s.q(ctx).VersionPin(ctx, skilldb.VersionPinParams{
		Version: &v,
		ID:      skillID,
	}); err != nil {
		return fmt.Errorf("pin: %w", err)
	}
	return nil
}

// Unpin remueve el pin para volver a usar la versión latest.
func (s *VersionStore) Unpin(ctx context.Context, skillID uuid.UUID) error {
	if err := s.q(ctx).VersionUnpin(ctx, skillID); err != nil {
		return fmt.Errorf("unpin: %w", err)
	}
	return nil
}

// Effective devuelve la version que se usa para invocations: pinned si está,
// sino la latest. Retorna ErrSkillVersionNotFound si no hay ninguna.
func (s *VersionStore) Effective(ctx context.Context, skillID uuid.UUID) (*Version, error) {
	pinned, err := s.q(ctx).VersionGetPinned(ctx, skillID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSkillVersionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lookup skill: %w", err)
	}
	if pinned != nil {
		return s.Get(ctx, skillID, int(*pinned))
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

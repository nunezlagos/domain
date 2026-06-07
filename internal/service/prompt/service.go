// Package prompt — HU-03.3 prompt templates versionados.
//
// Cada prompt tiene slug + version (UNIQUE por org+project+slug+version).
// Active = is_active=true AND deleted_at IS NULL. Solo UNA versión activa por
// slug+project a la vez (enforcement en service, no constraint DB).
//
// Variables JSONB define los placeholders esperados:
//
//	[{"name":"username","type":"string","required":true,"default":""}, ...]
package prompt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/saargo/domain/internal/audit"
)

var (
	ErrSlugInvalid    = errors.New("slug must be lowercase ascii, digits, dashes (2-100 chars)")
	ErrBodyRequired   = errors.New("body required")
	ErrNotFound       = errors.New("prompt not found")
	ErrVersionExists  = errors.New("version already exists for this slug")
	ErrNoActiveVersion = errors.New("no active version found for slug")
)

var reSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{0,98}[a-z0-9]$|^[a-z]$`)

type Variable struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
	Default  any    `json:"default,omitempty"`
}

type Prompt struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	ProjectID       *uuid.UUID
	CreatedBy       *uuid.UUID
	Slug            string
	Version         int
	Body            string
	Variables       []Variable
	Description     string
	IsActive        bool
	ParentVersionID *uuid.UUID
	Tags            []string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateInput struct {
	OrganizationID uuid.UUID
	ProjectID      *uuid.UUID
	CreatedBy      *uuid.UUID
	Slug           string
	Body           string
	Variables      []Variable
	Description    string
	Tags           []string
	// SetActive: si true marca esta versión como activa (y desactiva otras del mismo slug+project)
	SetActive bool
}

type SearchResult struct {
	Prompt
	Score    float64
	Headline string
}

type Service struct {
	Pool  *pgxpool.Pool
	Audit audit.Recorder
}

// Create crea siempre una nueva versión. La numeración auto-incrementa por
// (organization_id, COALESCE(project_id, '00000...'), slug). Si SetActive=true,
// dentro de la misma tx desactiva las anteriores y marca esta como activa.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Prompt, error) {
	in.Slug = strings.TrimSpace(in.Slug)
	if !reSlug.MatchString(in.Slug) {
		return nil, ErrSlugInvalid
	}
	if strings.TrimSpace(in.Body) == "" {
		return nil, ErrBodyRequired
	}
	if in.Variables == nil {
		in.Variables = []Variable{}
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	varsJSON, _ := json.Marshal(in.Variables)

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Calcular siguiente version (max + 1 por slug+project)
	var nextVersion int
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(version), 0) + 1
		 FROM prompts
		 WHERE organization_id = $1 AND slug = $2
		   AND project_id IS NOT DISTINCT FROM $3
		   AND deleted_at IS NULL`,
		in.OrganizationID, in.Slug, in.ProjectID,
	).Scan(&nextVersion)
	if err != nil {
		return nil, fmt.Errorf("calc version: %w", err)
	}

	// Si SetActive: desactivar las anteriores
	if in.SetActive {
		_, err = tx.Exec(ctx,
			`UPDATE prompts SET is_active = false
			 WHERE organization_id = $1 AND slug = $2
			   AND project_id IS NOT DISTINCT FROM $3
			   AND is_active = true AND deleted_at IS NULL`,
			in.OrganizationID, in.Slug, in.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("deactivate prior versions: %w", err)
		}
	}

	var p Prompt
	err = tx.QueryRow(ctx,
		`INSERT INTO prompts (organization_id, project_id, created_by, slug, version,
		                      body, variables, description, is_active, tags)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, organization_id, project_id, created_by, slug, version,
		           body, variables, COALESCE(description,''), is_active,
		           parent_version_id, tags, created_at, updated_at`,
		in.OrganizationID, in.ProjectID, in.CreatedBy, in.Slug, nextVersion,
		in.Body, varsJSON, nullStr(in.Description), in.SetActive, in.Tags,
	).Scan(&p.ID, &p.OrganizationID, &p.ProjectID, &p.CreatedBy, &p.Slug, &p.Version,
		&p.Body, &varsRaw{&p.Variables}, &p.Description, &p.IsActive,
		&p.ParentVersionID, &p.Tags, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert prompt: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        in.CreatedBy,
			ActorType:      audit.ActorUser,
			Action:         "prompt.created",
			EntityType:     "prompt",
			EntityID:       &p.ID,
			NewValues:      map[string]any{"slug": p.Slug, "version": p.Version, "is_active": p.IsActive},
		})
	}
	return &p, nil
}

// SetActive marca una versión como activa y desactiva las anteriores del mismo slug+project.
func (s *Service) SetActive(ctx context.Context, id, actorID uuid.UUID) (*Prompt, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var p Prompt
	err = tx.QueryRow(ctx,
		`SELECT id, organization_id, project_id, created_by, slug, version,
		        body, variables, COALESCE(description,''), is_active,
		        parent_version_id, tags, created_at, updated_at
		 FROM prompts WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, id,
	).Scan(&p.ID, &p.OrganizationID, &p.ProjectID, &p.CreatedBy, &p.Slug, &p.Version,
		&p.Body, &varsRaw{&p.Variables}, &p.Description, &p.IsActive,
		&p.ParentVersionID, &p.Tags, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	_, err = tx.Exec(ctx,
		`UPDATE prompts SET is_active = false
		 WHERE organization_id = $1 AND slug = $2
		   AND project_id IS NOT DISTINCT FROM $3
		   AND id <> $4 AND is_active = true AND deleted_at IS NULL`,
		p.OrganizationID, p.Slug, p.ProjectID, p.ID)
	if err != nil {
		return nil, fmt.Errorf("deactivate siblings: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE prompts SET is_active = true WHERE id = $1`, p.ID)
	if err != nil {
		return nil, fmt.Errorf("activate: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	p.IsActive = true

	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &p.OrganizationID,
			ActorID:        &actorID,
			ActorType:      audit.ActorUser,
			Action:         "prompt.set_active",
			EntityType:     "prompt",
			EntityID:       &id,
		})
	}
	return &p, nil
}

// GetActive devuelve la versión activa del slug en el project (o sin project).
func (s *Service) GetActive(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, slug string) (*Prompt, error) {
	p, err := s.queryOne(ctx,
		`WHERE organization_id = $1 AND slug = $2
		   AND project_id IS NOT DISTINCT FROM $3
		   AND is_active = true AND deleted_at IS NULL
		 ORDER BY version DESC LIMIT 1`,
		orgID, slug, projectID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNoActiveVersion
	}
	return p, err
}

// GetByID retorna prompt por UUID (cualquier versión, no-deleted).
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Prompt, error) {
	return s.queryOne(ctx, `WHERE id = $1 AND deleted_at IS NULL`, id)
}

// ListVersions devuelve todas las versiones de un slug (newest first).
func (s *Service) ListVersions(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, slug string) ([]Prompt, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, organization_id, project_id, created_by, slug, version,
		        body, variables, COALESCE(description,''), is_active,
		        parent_version_id, tags, created_at, updated_at
		 FROM prompts
		 WHERE organization_id = $1 AND slug = $2
		   AND project_id IS NOT DISTINCT FROM $3
		   AND deleted_at IS NULL
		 ORDER BY version DESC`,
		orgID, slug, projectID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()
	var out []Prompt
	for rows.Next() {
		var p Prompt
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.ProjectID, &p.CreatedBy, &p.Slug, &p.Version,
			&p.Body, &varsRaw{&p.Variables}, &p.Description, &p.IsActive,
			&p.ParentVersionID, &p.Tags, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Search hace BM25 sobre body_tsv con headline para fragmento destacado.
func (s *Service) Search(ctx context.Context, orgID uuid.UUID, query string, limit int) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.Pool.Query(ctx, `
SELECT p.id, p.organization_id, p.project_id, p.created_by, p.slug, p.version,
       p.body, p.variables, COALESCE(p.description,''), p.is_active,
       p.parent_version_id, p.tags, p.created_at, p.updated_at,
       ts_rank(p.body_tsv, q)::float8 AS score,
       ts_headline('spanish', p.body, q, 'StartSel=<mark>,StopSel=</mark>,MaxWords=20,MinWords=5') AS headline
FROM prompts p, plainto_tsquery('spanish', $2) AS q
WHERE p.organization_id = $1 AND p.deleted_at IS NULL AND p.body_tsv @@ q
ORDER BY score DESC
LIMIT $3
`, orgID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()
	var out []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.OrganizationID, &r.ProjectID, &r.CreatedBy, &r.Slug, &r.Version,
			&r.Body, &varsRaw{&r.Variables}, &r.Description, &r.IsActive,
			&r.ParentVersionID, &r.Tags, &r.CreatedAt, &r.UpdatedAt,
			&r.Score, &r.Headline); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SoftDelete marca deleted_at. Deja inactivo automáticamente.
func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE prompts SET deleted_at = NOW(), is_active = false
		 WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			ActorID:    &actorID,
			ActorType:  audit.ActorUser,
			Action:     "prompt.deleted",
			EntityType: "prompt",
			EntityID:   &id,
		})
	}
	return nil
}

func (s *Service) queryOne(ctx context.Context, where string, args ...any) (*Prompt, error) {
	var p Prompt
	q := `SELECT id, organization_id, project_id, created_by, slug, version,
	        body, variables, COALESCE(description,''), is_active,
	        parent_version_id, tags, created_at, updated_at FROM prompts ` + where
	err := s.Pool.QueryRow(ctx, q, args...).Scan(
		&p.ID, &p.OrganizationID, &p.ProjectID, &p.CreatedBy, &p.Slug, &p.Version,
		&p.Body, &varsRaw{&p.Variables}, &p.Description, &p.IsActive,
		&p.ParentVersionID, &p.Tags, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return &p, nil
}

// varsRaw scan helper: JSONB → []Variable.
type varsRaw struct {
	target *[]Variable
}

func (v *varsRaw) Scan(src any) error {
	if src == nil {
		*v.target = []Variable{}
		return nil
	}
	var raw []byte
	switch s := src.(type) {
	case []byte:
		raw = s
	case string:
		raw = []byte(s)
	default:
		return fmt.Errorf("varsRaw: unsupported type %T", src)
	}
	if len(raw) == 0 {
		*v.target = []Variable{}
		return nil
	}
	return json.Unmarshal(raw, v.target)
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

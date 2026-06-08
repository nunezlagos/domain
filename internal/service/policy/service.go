// Package policy — HU-01.8 platform policies en BD como SOT.
//
// Cada policy es un markdown body + structured JSONB versioned. Los agents
// IA consumen las activas via MCP tool domain_policy_get(slug) para
// asegurar consistencia cross-sesión.
package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUnknown      = errors.New("not_found")
	ErrInvalidSlug  = errors.New("invalid_slug")
	ErrInvalidKind  = errors.New("invalid_kind")
)

var reSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{0,98}[a-z0-9]$|^[a-z]$`)

// Kinds soportados (matchea CHECK constraint).
const (
	KindConvention    = "convention"
	KindSecurityRule  = "security_rule"
	KindArchitecture  = "architecture"
	KindSDDWorkflow   = "sdd_workflow"
	KindObservability = "observability"
	KindMigrationRule = "migration_rule"
	KindLinterConfig  = "linter_config"
)

var validKinds = map[string]bool{
	KindConvention: true, KindSecurityRule: true, KindArchitecture: true,
	KindSDDWorkflow: true, KindObservability: true, KindMigrationRule: true,
	KindLinterConfig: true,
}

// Policy estructura persistida.
type Policy struct {
	ID             uuid.UUID       `json:"id"`
	Slug           string          `json:"slug"`
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	BodyMD         string          `json:"body_md"`
	BodyStructured json.RawMessage `json:"body_structured"`
	Version        int             `json:"version"`
	IsActive       bool            `json:"is_active"`
	SourceFile     string          `json:"source_file,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// CreateInput para POST.
type CreateInput struct {
	Slug           string
	Name           string
	Kind           string
	BodyMD         string
	BodyStructured map[string]any
	SourceFile     string
}

// UpdateInput para PATCH — genera nueva version.
type UpdateInput struct {
	BodyMD         *string
	BodyStructured map[string]any
	ChangedBy      *uuid.UUID
}

type Service struct {
	Pool *pgxpool.Pool
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Policy, error) {
	if !reSlug.MatchString(in.Slug) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidSlug, in.Slug)
	}
	if !validKinds[in.Kind] {
		return nil, fmt.Errorf("%w: %s", ErrInvalidKind, in.Kind)
	}
	if in.BodyStructured == nil {
		in.BodyStructured = map[string]any{}
	}
	bodyJSON, _ := json.Marshal(in.BodyStructured)
	row := s.Pool.QueryRow(ctx,
		`INSERT INTO platform_policies
			(slug, name, kind, body_md, body_structured, source_file)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id, version, created_at, updated_at`,
		in.Slug, in.Name, in.Kind, in.BodyMD, bodyJSON, nullableStr(in.SourceFile))
	p := &Policy{
		Slug: in.Slug, Name: in.Name, Kind: in.Kind,
		BodyMD: in.BodyMD, BodyStructured: bodyJSON,
		SourceFile: in.SourceFile, IsActive: true,
	}
	if err := row.Scan(&p.ID, &p.Version, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}
	return p, nil
}

// GetBySlug retorna la versión active del policy slug.
func (s *Service) GetBySlug(ctx context.Context, slug string) (*Policy, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT id, slug, name, kind, body_md, body_structured, version,
			is_active, COALESCE(source_file,''), created_at, updated_at
		 FROM platform_policies WHERE slug=$1 AND is_active=TRUE`, slug)
	var p Policy
	err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.Kind, &p.BodyMD,
		&p.BodyStructured, &p.Version, &p.IsActive, &p.SourceFile,
		&p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// List por kind opcional. kind="" → todos.
func (s *Service) List(ctx context.Context, kind string) ([]Policy, error) {
	q := `SELECT id, slug, name, kind, body_md, body_structured, version,
		is_active, COALESCE(source_file,''), created_at, updated_at
	   FROM platform_policies WHERE is_active=TRUE`
	args := []any{}
	if kind != "" {
		q += " AND kind=$1"
		args = append(args, kind)
	}
	q += " ORDER BY kind, slug"
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Policy
	for rows.Next() {
		var p Policy
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.Kind, &p.BodyMD,
			&p.BodyStructured, &p.Version, &p.IsActive, &p.SourceFile,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Update genera nueva versión, archiva la anterior en platform_policy_versions.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Policy, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Lock + lookup actual
	var cur Policy
	err = tx.QueryRow(ctx,
		`SELECT id, slug, name, kind, body_md, body_structured, version,
			is_active, COALESCE(source_file,''), created_at, updated_at
		 FROM platform_policies WHERE id=$1 FOR UPDATE`, id).
		Scan(&cur.ID, &cur.Slug, &cur.Name, &cur.Kind, &cur.BodyMD,
			&cur.BodyStructured, &cur.Version, &cur.IsActive, &cur.SourceFile,
			&cur.CreatedAt, &cur.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, err
	}

	// Archivar versión actual
	_, err = tx.Exec(ctx,
		`INSERT INTO platform_policy_versions (policy_id, version, body_md, body_structured, changed_by)
		 VALUES ($1,$2,$3,$4,$5)`,
		cur.ID, cur.Version, cur.BodyMD, cur.BodyStructured, in.ChangedBy)
	if err != nil {
		return nil, fmt.Errorf("archive version: %w", err)
	}

	// Apply update
	newBody := cur.BodyMD
	if in.BodyMD != nil {
		newBody = *in.BodyMD
	}
	newStructured := cur.BodyStructured
	if in.BodyStructured != nil {
		j, _ := json.Marshal(in.BodyStructured)
		newStructured = j
	}
	_, err = tx.Exec(ctx,
		`UPDATE platform_policies
		 SET body_md=$1, body_structured=$2, version=version+1
		 WHERE id=$3`, newBody, newStructured, id)
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetBySlug(ctx, cur.Slug)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	ct, err := s.Pool.Exec(ctx,
		`UPDATE platform_policies SET is_active=FALSE WHERE id=$1 AND is_active=TRUE`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrUnknown
	}
	return nil
}

func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

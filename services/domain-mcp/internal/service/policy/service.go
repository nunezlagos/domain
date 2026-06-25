// Package policy — issue-01.8 platform policies en BD como SOT.
//
// Cada policy es un markdown body + structured JSONB versioned. Los agents
// IA consumen las activas via MCP tool domain_policy_get(slug) para
// asegurar consistencia cross-sesión.
//
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
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

	"nunezlagos/domain/internal/service/policy/policydb"
)

var (
	ErrUnknown     = errors.New("not_found")
	ErrInvalidSlug = errors.New("invalid_slug")
	ErrInvalidKind = errors.New("invalid_kind")
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

func (s *Service) q() *policydb.Queries { return policydb.New(s.Pool) }

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
	row, err := s.q().InsertPolicy(ctx, policydb.InsertPolicyParams{
		Slug:           in.Slug,
		Name:           in.Name,
		Kind:           in.Kind,
		BodyMd:         in.BodyMD,
		BodyStructured: bodyJSON,
		SourceFile:     optStr(in.SourceFile),
	})
	if err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}
	p := toPolicy(row)
	return &p, nil
}

// GetBySlug retorna la versión active del policy slug.
func (s *Service) GetBySlug(ctx context.Context, slug string) (*Policy, error) {
	row, err := s.q().GetActivePolicyBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, err
	}
	p := toPolicy(row)
	return &p, nil
}

// List por kind opcional. kind="" → todos.
func (s *Service) List(ctx context.Context, kind string) ([]Policy, error) {
	rows, err := s.q().ListActivePolicies(ctx, optStr(kind))
	if err != nil {
		return nil, err
	}
	var out []Policy
	for _, row := range rows {
		out = append(out, toPolicy(row))
	}
	return out, nil
}

// Update genera nueva versión, archiva la anterior en platform_policy_versions.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Policy, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	q := policydb.New(tx)

	cur, err := q.GetPolicyForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, err
	}

	if err := q.InsertPolicyVersion(ctx, policydb.InsertPolicyVersionParams{
		PolicyID:       cur.ID,
		Version:        cur.Version,
		BodyMd:         cur.BodyMd,
		BodyStructured: cur.BodyStructured,
		ChangedBy:      in.ChangedBy,
	}); err != nil {
		return nil, fmt.Errorf("archive version: %w", err)
	}

	newBody := cur.BodyMd
	if in.BodyMD != nil {
		newBody = *in.BodyMD
	}
	newStructured := cur.BodyStructured
	if in.BodyStructured != nil {
		j, _ := json.Marshal(in.BodyStructured)
		newStructured = j
	}

	if err := q.UpdatePolicyBody(ctx, policydb.UpdatePolicyBodyParams{
		ID:             id,
		BodyMd:         newBody,
		BodyStructured: newStructured,
	}); err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetBySlug(ctx, cur.Slug)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := s.q().DeactivatePolicy(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUnknown
	}
	return nil
}

func toPolicy(r policydb.PlatformPolicy) Policy {
	src := ""
	if r.SourceFile != nil {
		src = *r.SourceFile
	}
	return Policy{
		ID:             r.ID,
		Slug:           r.Slug,
		Name:           r.Name,
		Kind:           r.Kind,
		BodyMD:         r.BodyMd,
		BodyStructured: json.RawMessage(r.BodyStructured),
		Version:        int(r.Version),
		IsActive:       r.IsActive,
		SourceFile:     src,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

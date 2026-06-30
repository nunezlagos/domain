package projectpolicy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/projectpolicy/projectpolicydb"
	"nunezlagos/domain/internal/store/txctx"
)

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewPgRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (r *pgRepository) q(ctx context.Context) *projectpolicydb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return projectpolicydb.New(tx)
	}
	return projectpolicydb.New(r.pool)
}

func (r *pgRepository) raw(ctx context.Context) querier {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return r.pool
}

func toPolicy(id uuid.UUID, projectID uuid.UUID, slug, name, kind, bodyMD string, bodyStructured []byte, version int32, isActive, overridePlatform bool, source string, createdAt, updatedAt time.Time, deletedAt pgtype.Timestamptz) Policy {
	var bs any
	if bodyStructured != nil {
		_ = json.Unmarshal(bodyStructured, &bs)
	}
	var da *time.Time
	if deletedAt.Valid {
		da = &deletedAt.Time
	}
	return Policy{
		ID: id, ProjectID: projectID, Slug: slug, Name: name, Kind: kind,
		BodyMD: bodyMD, BodyStructured: bs,
		Version: int(version), IsActive: isActive, OverridePlatform: overridePlatform,
		Source: source, CreatedAt: createdAt, UpdatedAt: updatedAt, DeletedAt: da,
	}
}

func toPolicyFromInsert(r projectpolicydb.InsertPolicyRow) Policy {
	return toPolicy(r.ID, r.ProjectID, r.Slug, r.Name, r.Kind, r.BodyMd, r.BodyStructured, r.Version, r.IsActive, r.OverridePlatform, r.Source, r.CreatedAt, r.UpdatedAt, r.DeletedAt)
}

func toPolicyFromList(r projectpolicydb.ListPoliciesRow) Policy {
	return toPolicy(r.ID, r.ProjectID, r.Slug, r.Name, r.Kind, r.BodyMd, r.BodyStructured, r.Version, r.IsActive, r.OverridePlatform, r.Source, r.CreatedAt, r.UpdatedAt, r.DeletedAt)
}

func toPolicyFromGetBySlug(r projectpolicydb.GetPolicyBySlugRow) Policy {
	return toPolicy(r.ID, r.ProjectID, r.Slug, r.Name, r.Kind, r.BodyMd, r.BodyStructured, r.Version, r.IsActive, r.OverridePlatform, r.Source, r.CreatedAt, r.UpdatedAt, r.DeletedAt)
}

func toPolicyFromGet(r projectpolicydb.GetPolicyRow) Policy {
	return toPolicy(r.ID, r.ProjectID, r.Slug, r.Name, r.Kind, r.BodyMd, r.BodyStructured, r.Version, r.IsActive, r.OverridePlatform, r.Source, r.CreatedAt, r.UpdatedAt, r.DeletedAt)
}

func toPolicyFromUpdate(r projectpolicydb.UpdatePolicyRow) Policy {
	return toPolicy(r.ID, r.ProjectID, r.Slug, r.Name, r.Kind, r.BodyMd, r.BodyStructured, r.Version, r.IsActive, r.OverridePlatform, r.Source, r.CreatedAt, r.UpdatedAt, r.DeletedAt)
}

func (r *pgRepository) Insert(ctx context.Context, in CreateInput) (*Policy, error) {
	structuredBytes := []byte("{}")
	if in.BodyStructured != nil {
		if b, err := json.Marshal(in.BodyStructured); err == nil {
			structuredBytes = b
		}
	}
	source := in.Source
	if source == "" {
		source = "manual"
	}

	row, err := r.q(ctx).InsertPolicy(ctx, projectpolicydb.InsertPolicyParams{
		ProjectID:        in.ProjectID,
		Slug:             in.Slug,
		Name:             in.Name,
		Kind:             in.Kind,
		BodyMd:           in.BodyMD,
		BodyStructured:   structuredBytes,
		OverridePlatform: in.OverridePlatform,
		Source:           source,
	})
	if err != nil {
		return nil, fmt.Errorf("insert project_policy: %w", err)
	}
	p := toPolicyFromInsert(row)
	return &p, nil
}

func (r *pgRepository) List(ctx context.Context, _ uuid.UUID, projectID uuid.UUID, kind string) ([]*Policy, error) {
	var kindPtr *string
	if kind != "" {
		kindPtr = &kind
	}

	rows, err := r.q(ctx).ListPolicies(ctx, projectpolicydb.ListPoliciesParams{
		ProjectID: projectID,
		Kind:      kindPtr,
	})
	if err != nil {
		return nil, fmt.Errorf("list project_policies: %w", err)
	}
	out := make([]*Policy, 0, len(rows))
	for _, row := range rows {
		p := toPolicyFromList(row)
		out = append(out, &p)
	}
	return out, nil
}

func (r *pgRepository) GetBySlug(ctx context.Context, _ uuid.UUID, projectID uuid.UUID, slug string) (*Policy, error) {
	row, err := r.q(ctx).GetPolicyBySlug(ctx, projectpolicydb.GetPolicyBySlugParams{
		ProjectID: projectID,
		Slug:      slug,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get project_policy by slug: %w", err)
	}
	p := toPolicyFromGetBySlug(row)
	return &p, nil
}

func (r *pgRepository) Get(ctx context.Context, _ uuid.UUID, id uuid.UUID) (*Policy, error) {
	row, err := r.q(ctx).GetPolicy(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get project_policy: %w", err)
	}
	p := toPolicyFromGet(row)
	return &p, nil
}

func (r *pgRepository) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, changedBy *uuid.UUID) (*Policy, error) {
	curr, err := r.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		curr.Name = *in.Name
	}
	if in.Kind != nil {
		curr.Kind = *in.Kind
	}
	if in.BodyMD != nil {
		curr.BodyMD = *in.BodyMD
	}
	if in.OverridePlatform != nil {
		curr.OverridePlatform = *in.OverridePlatform
	}
	if in.BodyStructured != nil {
		curr.BodyStructured = in.BodyStructured
	}

	structuredBytes := []byte("{}")
	if curr.BodyStructured != nil {
		if b, jerr := json.Marshal(curr.BodyStructured); jerr == nil {
			structuredBytes = b
		}
	}

	newVersion := curr.Version + 1

	if _, err := r.raw(ctx).Exec(ctx,
		`INSERT INTO project_policy_versions
		   (policy_id, version, body_md, body_structured, changed_by)
		 VALUES ($1,$2,$3,$4,$5)`,
		curr.ID, curr.Version, curr.BodyMD, structuredBytes, changedBy,
	); err != nil {
		return nil, fmt.Errorf("snapshot version: %w", err)
	}

	row, err := r.q(ctx).UpdatePolicy(ctx, projectpolicydb.UpdatePolicyParams{
		ID:               id,
		Name:             curr.Name,
		Kind:             curr.Kind,
		BodyMd:           curr.BodyMD,
		BodyStructured:   structuredBytes,
		OverridePlatform: curr.OverridePlatform,
		Version:          int32(newVersion),
	})
	if err != nil {
		return nil, fmt.Errorf("update project_policy: %w", err)
	}
	p := toPolicyFromUpdate(row)
	return &p, nil
}

func (r *pgRepository) SoftDelete(ctx context.Context, _ uuid.UUID, id uuid.UUID) error {
	n, err := r.q(ctx).SoftDeletePolicy(ctx, id)
	if err != nil {
		return fmt.Errorf("soft-delete project_policy: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

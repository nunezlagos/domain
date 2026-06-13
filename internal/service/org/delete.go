package org

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DeleteResult struct {
	OrgID           uuid.UUID
	RowsDeleted     int64
	S3CleanupFailed bool
	S3Configured    bool
	DurationMs      int64
}

type TableCount struct {
	Table string `json:"table"`
	Count int64  `json:"count"`
}

type DeleteService struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
}

func NewDeleteService(pool *pgxpool.Pool, logger *slog.Logger) *DeleteService {
	return &DeleteService{Pool: pool, Logger: logger}
}

func (s *DeleteService) PreCountOrgData(ctx context.Context, orgID uuid.UUID) ([]TableCount, error) {
	query := `
		SELECT 'observations' AS table, COUNT(*)::bigint FROM observations WHERE organization_id = $1
		UNION ALL SELECT 'prompts', COUNT(*)::bigint FROM prompts WHERE organization_id = $1
		UNION ALL SELECT 'knowledge_docs', COUNT(*)::bigint FROM knowledge_docs WHERE organization_id = $1
		UNION ALL SELECT 'skills', COUNT(*)::bigint FROM skills WHERE organization_id = $1
		UNION ALL SELECT 'agents', COUNT(*)::bigint FROM agents WHERE organization_id = $1 AND deleted_at IS NULL
		UNION ALL SELECT 'flows', COUNT(*)::bigint FROM flows WHERE organization_id = $1 AND deleted_at IS NULL
		UNION ALL SELECT 'flow_runs', COUNT(*)::bigint FROM flow_runs WHERE organization_id = $1
		UNION ALL SELECT 'audit_log', COUNT(*)::bigint FROM audit_log WHERE organization_id = $1
	`
	rows, err := s.Pool.Query(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("pre-count: %w", err)
	}
	defer rows.Close()

	var counts []TableCount
	for rows.Next() {
		var tc TableCount
		if err := rows.Scan(&tc.Table, &tc.Count); err != nil {
			return nil, fmt.Errorf("scan count: %w", err)
		}
		counts = append(counts, tc)
	}
	return counts, rows.Err()
}

func (s *DeleteService) DeleteOrg(ctx context.Context, orgID uuid.UUID, actorUserID *uuid.UUID, actorEmail string) (*DeleteResult, error) {
	start := time.Now()

	org, err := s.getOrgByID(ctx, orgID)
	if err != nil {
		return nil, ErrNotFound
	}

	if org.DeletedAt != nil {
		s.Logger.Warn("org already deleted, skipping",
			slog.String("org_id", orgID.String()),
			slog.String("slug", org.Slug),
		)
		return &DeleteResult{OrgID: orgID}, nil
	}

	preCounts, err := s.PreCountOrgData(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("pre-count: %w", err)
	}
	preCountsJSON, _ := json.Marshal(preCounts)

	deleteLogID := uuid.New()
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO org_delete_log (id, organization_id, slug, actor_user_id, actor_email, pre_counts, s3_configured)
		VALUES ($1, $2, $3, $4, $5, $6, false)
	`, deleteLogID, orgID, org.Slug, actorUserID, actorEmail, preCountsJSON)
	if err != nil {
		return nil, fmt.Errorf("insert delete log: %w", err)
	}

	tag, err := s.Pool.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	if err != nil {
		return nil, fmt.Errorf("delete org: %w", err)
	}

	durationMs := time.Since(start).Milliseconds()

	_, _ = s.Pool.Exec(ctx, `
		UPDATE org_delete_log SET duration_ms = $1 WHERE id = $2
	`, durationMs, deleteLogID)

	s.Logger.Info("org deleted",
		slog.String("org_id", orgID.String()),
		slog.String("slug", org.Slug),
		slog.Int64("rows_deleted", tag.RowsAffected()),
		slog.Int64("duration_ms", durationMs),
	)

	return &DeleteResult{
		OrgID:       orgID,
		RowsDeleted: tag.RowsAffected(),
		DurationMs:  durationMs,
	}, nil
}

func (s *DeleteService) getOrgByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	var org Organization
	err := s.Pool.QueryRow(ctx,
		`SELECT id, name, slug, settings, created_at, updated_at, deleted_at
		 FROM organizations WHERE id = $1`, id,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.Settings, &org.CreatedAt, &org.UpdatedAt, &org.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get org: %w", err)
	}
	return &org, nil
}

func (s *DeleteService) GetOrgBySlug(ctx context.Context, slug string) (*Organization, error) {
	var org Organization
	err := s.Pool.QueryRow(ctx,
		`SELECT id, name, slug, settings, created_at, updated_at, deleted_at
		 FROM organizations WHERE slug = $1`, slug,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.Settings, &org.CreatedAt, &org.UpdatedAt, &org.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get org by slug: %w", err)
	}
	return &org, nil
}

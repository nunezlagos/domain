// Package observation — HU-03.1 CRUD de observations + búsqueda híbrida.
//
// Observations son la unidad central de memoria. Cada una vive en un project
// dentro de una organization. Búsqueda híbrida combina:
//   - BM25 (ts_rank con índice GIN sobre content_tsv en español)
//   - cosine (operador <=> de pgvector sobre embedding)
// fusionados con Reciprocal Rank Fusion (RRF): score = sum(1 / (k + rank_i)).
//
// Si el embedding es vector zero (NopEmbedder), search degrada a tsvector-only.
package observation

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

	"github.com/saargo/domain/internal/audit"
	"github.com/saargo/domain/internal/llm"
)

var (
	ErrNotFound          = errors.New("observation not found")
	ErrContentRequired   = errors.New("content required")
	ErrProjectMismatch   = errors.New("project does not belong to organization")
)

type Observation struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	ProjectID       uuid.UUID
	CreatedBy       *uuid.UUID
	SessionID       *uuid.UUID
	Content         string
	ObservationType string
	Tags            []string
	Metadata        map[string]any
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type SaveInput struct {
	OrganizationID  uuid.UUID
	ProjectID       uuid.UUID
	CreatedBy       *uuid.UUID
	SessionID       *uuid.UUID
	Content         string
	ObservationType string // default "note"
	Tags            []string
	Metadata        map[string]any
}

type SearchResult struct {
	Observation
	Score      float64 // RRF combined score
	BM25Rank   int     // 0 si no apareció en BM25
	VectorRank int     // 0 si no apareció en vector
}

type Service struct {
	Pool     *pgxpool.Pool
	Audit    audit.Recorder
	Embedder llm.Embedder
}

// Save crea observation con embedding generado en línea.
func (s *Service) Save(ctx context.Context, in SaveInput) (*Observation, error) {
	if strings.TrimSpace(in.Content) == "" {
		return nil, ErrContentRequired
	}
	if in.ObservationType == "" {
		in.ObservationType = "note"
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}
	metaJSON, _ := json.Marshal(in.Metadata)

	// Generar embedding (puede ser zero vector si NopEmbedder)
	vec, err := s.Embedder.Embed(ctx, in.Content)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	embedLit := vectorLiteral(vec)
	dim := s.Embedder.Dimensions()

	var o Observation
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO observations
		   (organization_id, project_id, created_by, session_id, content,
		    embedding, observation_type, tags, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6::vector, $7, $8, $9)
		 RETURNING id, organization_id, project_id, created_by, session_id,
		           content, observation_type, tags, metadata, created_at, updated_at`,
		in.OrganizationID, in.ProjectID, in.CreatedBy, in.SessionID, in.Content,
		embedLit, in.ObservationType, in.Tags, metaJSON,
	).Scan(&o.ID, &o.OrganizationID, &o.ProjectID, &o.CreatedBy, &o.SessionID,
		&o.Content, &o.ObservationType, &o.Tags, &o.Metadata, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "violates foreign key constraint") &&
			strings.Contains(err.Error(), "project") {
			return nil, ErrProjectMismatch
		}
		return nil, fmt.Errorf("insert observation: %w", err)
	}
	_ = dim
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        in.CreatedBy,
			ActorType:      audit.ActorUser,
			Action:         "observation.saved",
			EntityType:     "observation",
			EntityID:       &o.ID,
		})
	}
	return &o, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Observation, error) {
	var o Observation
	err := s.Pool.QueryRow(ctx,
		`SELECT id, organization_id, project_id, created_by, session_id,
		        content, observation_type, tags, metadata, created_at, updated_at
		 FROM observations WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&o.ID, &o.OrganizationID, &o.ProjectID, &o.CreatedBy, &o.SessionID,
		&o.Content, &o.ObservationType, &o.Tags, &o.Metadata, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get observation: %w", err)
	}
	return &o, nil
}

// List lista observations del project, más recientes primero.
func (s *Service) List(ctx context.Context, projectID uuid.UUID, limit int) ([]Observation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT id, organization_id, project_id, created_by, session_id,
		        content, observation_type, tags, metadata, created_at, updated_at
		 FROM observations
		 WHERE project_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT $2`, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()
	var out []Observation
	for rows.Next() {
		var o Observation
		if err := rows.Scan(&o.ID, &o.OrganizationID, &o.ProjectID, &o.CreatedBy, &o.SessionID,
			&o.Content, &o.ObservationType, &o.Tags, &o.Metadata, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// SoftDelete marca deleted_at.
func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE observations SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
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
			Action:     "observation.deleted",
			EntityType: "observation",
			EntityID:   &id,
		})
	}
	return nil
}

// SearchHybrid combina BM25 y cosine con RRF (Reciprocal Rank Fusion).
// k=60 es el default standard en literatura RRF.
// Si embedder es Nop (zero vector), search degrada a tsvector-only.
func (s *Service) SearchHybrid(ctx context.Context, orgID uuid.UUID, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	vec, err := s.Embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	useVector := !llm.IsZero(vec)

	const rrfK = 60
	const candidates = 100 // por cada modalidad

	// CTE: BM25 ranking + (opcional) vector ranking, después RRF fusion.
	var rows pgx.Rows
	if useVector {
		rows, err = s.Pool.Query(ctx, `
WITH bm25 AS (
  SELECT id, ROW_NUMBER() OVER (ORDER BY ts_rank(content_tsv, query) DESC) AS r
  FROM observations, plainto_tsquery('spanish', $2) AS query
  WHERE organization_id = $1 AND deleted_at IS NULL AND content_tsv @@ query
  LIMIT $4
),
vec AS (
  SELECT id, ROW_NUMBER() OVER (ORDER BY embedding <=> $3::vector ASC) AS r
  FROM observations
  WHERE organization_id = $1 AND deleted_at IS NULL AND embedding IS NOT NULL
  LIMIT $4
),
fused AS (
  SELECT id,
         COALESCE(1.0 / ($5 + bm25.r), 0) + COALESCE(1.0 / ($5 + vec.r), 0) AS score,
         COALESCE(bm25.r, 0) AS bm25_rank,
         COALESCE(vec.r, 0) AS vec_rank
  FROM bm25 FULL OUTER JOIN vec USING (id)
)
SELECT o.id, o.organization_id, o.project_id, o.created_by, o.session_id,
       o.content, o.observation_type, o.tags, o.metadata, o.created_at, o.updated_at,
       f.score, f.bm25_rank, f.vec_rank
FROM fused f
JOIN observations o ON o.id = f.id
ORDER BY f.score DESC
LIMIT $6
`, orgID, query, vectorLiteral(vec), candidates, rrfK, limit)
	} else {
		rows, err = s.Pool.Query(ctx, `
SELECT o.id, o.organization_id, o.project_id, o.created_by, o.session_id,
       o.content, o.observation_type, o.tags, o.metadata, o.created_at, o.updated_at,
       ts_rank(o.content_tsv, query)::float8 AS score,
       0::bigint AS bm25_rank,
       0::bigint AS vec_rank
FROM observations o, plainto_tsquery('spanish', $2) AS query
WHERE o.organization_id = $1 AND o.deleted_at IS NULL AND o.content_tsv @@ query
ORDER BY score DESC
LIMIT $3
`, orgID, query, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("hybrid search: %w", err)
	}
	defer rows.Close()
	var out []SearchResult
	for rows.Next() {
		var r SearchResult
		var bm25Rank, vecRank int64
		if err := rows.Scan(&r.ID, &r.OrganizationID, &r.ProjectID, &r.CreatedBy, &r.SessionID,
			&r.Content, &r.ObservationType, &r.Tags, &r.Metadata, &r.CreatedAt, &r.UpdatedAt,
			&r.Score, &bm25Rank, &vecRank); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		r.BM25Rank = int(bm25Rank)
		r.VectorRank = int(vecRank)
		out = append(out, r)
	}
	return out, rows.Err()
}

// vectorLiteral convierte []float32 a literal '[v1,v2,...]' para pgvector.
func vectorLiteral(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%g", f)
	}
	sb.WriteByte(']')
	return sb.String()
}

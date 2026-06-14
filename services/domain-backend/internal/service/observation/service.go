// Package observation — issue-03.1 CRUD de observations + búsqueda híbrida.
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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/memory/dedup"
	"nunezlagos/domain/internal/memory/privacy"
	"nunezlagos/domain/internal/store/txctx"
)

var (
	ErrNotFound          = errors.New("observation not found")
	ErrContentRequired   = errors.New("content required")
	ErrProjectMismatch   = errors.New("project does not belong to organization")
	ErrDuplicate         = errors.New("duplicate observation (content_hash already exists)")
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

// EventEmitter publica eventos de dominio hacia webhooks outbound
// (issue-10.4 ow-002). Opcional.
type EventEmitter interface {
	EmitEntityEvent(ctx context.Context, orgID uuid.UUID, eventType string, data map[string]any)
}

type Service struct {
	Pool     *pgxpool.Pool
	Audit    audit.Recorder
	Embedder llm.Embedder
	Events   EventEmitter // nil = sin webhooks
}

// querier retorna la tx con SET LOCAL si el middleware HTTP la inyectó
// (issue-25.14), o el pool como fallback. Permite que las mismas queries
// funcionen dentro de una request HTTP (RLS activa) o desde un job/cron
// (que corre como app_admin con BYPASSRLS, sin necesidad de tx).
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (s *Service) q(ctx context.Context) querier {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return s.Pool
}

// Save crea observation con embedding generado en línea.
// Aplica privacy stripping (<private>...</private>) y dedup hash check
// (UNIQUE constraint en DB = defense in depth contra bypass de la app).
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

	// issue-03.6 privacy: stripping de bloques <private>...</private>
	cleanContent, redactedCount := privacy.Strip(in.Content)
	if redactedCount > 0 {
		in.Metadata["privacy_redacted_blocks"] = redactedCount
	}
	if strings.TrimSpace(cleanContent) == "" {
		return nil, ErrContentRequired
	}

	metaJSON, _ := json.Marshal(in.Metadata)

	// issue-03.6 dedup: hash normalizado del fingerprint
	hash := dedup.Hash(dedup.FingerprintInput{
		ProjectID:       in.ProjectID,
		ObservationType: in.ObservationType,
		Title:           "",
		Content:         cleanContent,
	})

	// Generar embedding sobre contenido limpio
	vec, err := s.Embedder.Embed(ctx, cleanContent)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	embedLit := vectorLiteral(vec)

	var o Observation
	err = s.q(ctx).QueryRow(ctx,
		`INSERT INTO observations
		   (organization_id, project_id, created_by, session_id, content,
		    embedding, observation_type, tags, metadata, content_hash)
		 VALUES ($1, $2, $3, $4, $5, $6::vector, $7, $8, $9, $10)
		 RETURNING id, organization_id, project_id, created_by, session_id,
		           content, observation_type, tags, metadata, created_at, updated_at`,
		in.OrganizationID, in.ProjectID, in.CreatedBy, in.SessionID, cleanContent,
		embedLit, in.ObservationType, in.Tags, metaJSON, hash,
	).Scan(&o.ID, &o.OrganizationID, &o.ProjectID, &o.CreatedBy, &o.SessionID,
		&o.Content, &o.ObservationType, &o.Tags, &o.Metadata, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "observations_dedup_hash_uniq") {
			return nil, ErrDuplicate
		}
		if strings.Contains(err.Error(), "violates foreign key constraint") &&
			strings.Contains(err.Error(), "project") {
			return nil, ErrProjectMismatch
		}
		return nil, fmt.Errorf("insert observation: %w", err)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        in.CreatedBy,
			ActorType:      audit.ActorUser,
			Action:         "observation.saved",
			EntityType:     "observation",
			EntityID:       &o.ID,
			NewValues:      map[string]any{"redacted": redactedCount},
		})
	}
	// issue-10.4 ow-002: webhook outbound (solo metadata, nunca el content)
	if s.Events != nil {
		s.Events.EmitEntityEvent(ctx, in.OrganizationID, "observation.created", map[string]any{
			"observation_id": o.ID,
			"project_id":     in.ProjectID,
			"type":           o.ObservationType,
		})
	}
	return &o, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Observation, error) {
	var o Observation
	err := s.q(ctx).QueryRow(ctx,
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
	rows, err := s.q(ctx).Query(ctx,
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

// ListPageInput describe filtros + paginación cursor-based (issue-13.6).
type ListPageInput struct {
	ProjectID      uuid.UUID
	Limit          int
	SortDesc       bool       // true = DESC (default), false = ASC
	CursorTime     *time.Time // si != nil, paginar desde este punto
	CursorID       *uuid.UUID // tie-breaker para estabilidad
}

// ListPaginated implementa keyset pagination estable por (created_at, id).
// Devuelve hasta limit+1 rows internamente para detectar has_more sin extra query.
func (s *Service) ListPaginated(ctx context.Context, in ListPageInput) ([]Observation, bool, error) {
	limit := in.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	dir := "DESC"
	cmp := "<"
	if !in.SortDesc {
		dir = "ASC"
		cmp = ">"
	}

	args := []any{in.ProjectID}
	q := `SELECT id, organization_id, project_id, created_by, session_id,
	            content, observation_type, tags, metadata, created_at, updated_at
	      FROM observations
	      WHERE project_id = $1 AND deleted_at IS NULL`
	if in.CursorTime != nil && in.CursorID != nil {
		args = append(args, *in.CursorTime, *in.CursorID)
		q += fmt.Sprintf(" AND (created_at, id) %s ($2, $3)", cmp)
	}
	args = append(args, limit+1)
	q += fmt.Sprintf(" ORDER BY created_at %s, id %s LIMIT $%d", dir, dir, len(args))

	rows, err := s.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list paginated: %w", err)
	}
	defer rows.Close()
	var out []Observation
	for rows.Next() {
		var o Observation
		if err := rows.Scan(&o.ID, &o.OrganizationID, &o.ProjectID, &o.CreatedBy, &o.SessionID,
			&o.Content, &o.ObservationType, &o.Tags, &o.Metadata, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, false, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return out, hasMore, nil
}

// SoftDelete marca deleted_at.
func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	tag, err := s.q(ctx).Exec(ctx,
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
		rows, err = s.q(ctx).Query(ctx, `
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
		rows, err = s.q(ctx).Query(ctx, `
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

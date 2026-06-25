// Package observation — pg_repository.go: implementación PG del Repository.
//
// Wrappea el pool y honra tx-context (si el middleware HTTP inyectó una tx,
// las queries corren contra esa tx para que RLS aplique).
package observation

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/store/txctx"
)

// pgRepository implementa Repository contra Postgres con pgxpool.
type pgRepository struct {
	pool *pgxpool.Pool
}

// NewPgRepository construye el repository PG. pool puede ser nil para casos
// donde el Service es usado solo con tx-context inyectada — en ese caso TODAS
// las requests deben venir con tx (o las queries fallarán al hacer nil deref
// sobre pool).
func NewPgRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

// querier es el subset de pgx que necesitamos. Tanto *pgxpool.Pool como pgx.Tx
// satisfacen estas firmas.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (r *pgRepository) q(ctx context.Context) querier {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return r.pool
}

func (r *pgRepository) Insert(ctx context.Context, in InsertParams) (*Observation, error) {
	var o Observation
	err := r.q(ctx).QueryRow(ctx,
		`INSERT INTO knowledge_observations
		   (project_id, created_by, session_id, content,
		    embedding, observation_type, tags, metadata, content_hash)
		 VALUES ($1, $2, $3, $4, $5::vector, $6, $7, $8, $9)
		 RETURNING id, project_id, created_by, session_id,
		           content, observation_type, tags, metadata, created_at, updated_at`,
		in.ProjectID, in.CreatedBy, in.SessionID, in.Content,
		in.EmbeddingLit, in.ObservationType, in.Tags, in.MetadataJSON, in.ContentHash,
	).Scan(&o.ID, &o.ProjectID, &o.CreatedBy, &o.SessionID,
		&o.Content, &o.ObservationType, &o.Tags, &o.Metadata, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {

		return nil, err
	}
	return &o, nil
}

func (r *pgRepository) Get(ctx context.Context, id uuid.UUID) (*Observation, error) {
	var o Observation
	err := r.q(ctx).QueryRow(ctx,
		`SELECT id, project_id, created_by, session_id,
		        content, observation_type, tags, metadata, created_at, updated_at
		 FROM knowledge_observations WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&o.ID, &o.ProjectID, &o.CreatedBy, &o.SessionID,
		&o.Content, &o.ObservationType, &o.Tags, &o.Metadata, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get observation: %w", err)
	}
	return &o, nil
}

func (r *pgRepository) List(ctx context.Context, projectID uuid.UUID, limit int) ([]Observation, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT id, project_id, created_by, session_id,
		        content, observation_type, tags, metadata, created_at, updated_at
		 FROM knowledge_observations
		 WHERE project_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT $2`, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()
	var out []Observation
	for rows.Next() {
		var o Observation
		if err := rows.Scan(&o.ID, &o.ProjectID, &o.CreatedBy, &o.SessionID,
			&o.Content, &o.ObservationType, &o.Tags, &o.Metadata, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *pgRepository) ListPaginated(ctx context.Context, in ListPageInput) ([]Observation, bool, error) {
	limit := in.Limit
	dir := "DESC"
	cmp := "<"
	if !in.SortDesc {
		dir = "ASC"
		cmp = ">"
	}
	args := []any{in.ProjectID}
	q := `SELECT id, project_id, created_by, session_id,
	            content, observation_type, tags, metadata, created_at, updated_at
	      FROM knowledge_observations
	      WHERE project_id = $1 AND deleted_at IS NULL`
	if in.CursorTime != nil && in.CursorID != nil {
		args = append(args, *in.CursorTime, *in.CursorID)
		q += fmt.Sprintf(" AND (created_at, id) %s ($2, $3)", cmp)
	}
	args = append(args, limit+1)
	q += fmt.Sprintf(" ORDER BY created_at %s, id %s LIMIT $%d", dir, dir, len(args))

	rows, err := r.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list paginated: %w", err)
	}
	defer rows.Close()
	var out []Observation
	for rows.Next() {
		var o Observation
		if err := rows.Scan(&o.ID, &o.ProjectID, &o.CreatedBy, &o.SessionID,
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

func (r *pgRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q(ctx).Exec(ctx,
		`UPDATE knowledge_observations SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) SearchHybrid(ctx context.Context, in SearchInput) ([]SearchResult, error) {
	var rows pgx.Rows
	var err error
	if in.UseVector {
		rows, err = r.q(ctx).Query(ctx, `
WITH bm25 AS (
  SELECT id, ROW_NUMBER() OVER (ORDER BY ts_rank(content_tsv, query) DESC) AS r
  FROM knowledge_observations, plainto_tsquery('spanish', $1) AS query
  WHERE deleted_at IS NULL AND content_tsv @@ query
  LIMIT $3
),
vec AS (
  SELECT id, ROW_NUMBER() OVER (ORDER BY embedding <=> $2::vector ASC) AS r
  FROM knowledge_observations
  WHERE deleted_at IS NULL AND embedding IS NOT NULL
  LIMIT $3
),
fused AS (
  SELECT id,
         COALESCE(1.0 / ($4 + bm25.r), 0) + COALESCE(1.0 / ($4 + vec.r), 0) AS score,
         COALESCE(bm25.r, 0) AS bm25_rank,
         COALESCE(vec.r, 0) AS vec_rank
  FROM bm25 FULL OUTER JOIN vec USING (id)
)
SELECT o.id, o.project_id, o.created_by, o.session_id,
       o.content, o.observation_type, o.tags, o.metadata, o.created_at, o.updated_at,
       f.score, f.bm25_rank, f.vec_rank
FROM fused f
JOIN knowledge_observations o ON o.id = f.id
ORDER BY f.score DESC
LIMIT $5
`, in.Query, in.EmbeddingLit, in.Candidates, in.RRFK, in.Limit)
	} else {
		rows, err = r.q(ctx).Query(ctx, `
SELECT o.id, o.project_id, o.created_by, o.session_id,
       o.content, o.observation_type, o.tags, o.metadata, o.created_at, o.updated_at,
       ts_rank(o.content_tsv, query)::float8 AS score,
       0::bigint AS bm25_rank,
       0::bigint AS vec_rank
FROM knowledge_observations o, plainto_tsquery('spanish', $1) AS query
WHERE o.deleted_at IS NULL AND o.content_tsv @@ query
ORDER BY score DESC
LIMIT $2
`, in.Query, in.Limit)
	}
	if err != nil {
		return nil, fmt.Errorf("hybrid search: %w", err)
	}
	defer rows.Close()
	var out []SearchResult
	for rows.Next() {
		var r SearchResult
		var bm25Rank, vecRank int64
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.CreatedBy, &r.SessionID,
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

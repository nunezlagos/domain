// Package session — pg_repository.go: implementación PG del Repository.
package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

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

func (r *pgRepository) q(ctx context.Context) querier {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return r.pool
}

func (r *pgRepository) Insert(ctx context.Context, in InsertParams) (*Session, error) {
	var sess Session
	err := r.q(ctx).QueryRow(ctx,
		`INSERT INTO sessions (organization_id, project_id, user_id, title, tags)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, organization_id, project_id, user_id, title, COALESCE(summary,''),
		           tags, started_at, ended_at, created_at`,
		in.OrganizationID, in.ProjectID, in.UserID, in.Title, in.Tags,
	).Scan(&sess.ID, &sess.OrganizationID, &sess.ProjectID, &sess.UserID, &sess.Title, &sess.Summary,
		&sess.Tags, &sess.StartedAt, &sess.EndedAt, &sess.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	return &sess, nil
}

func (r *pgRepository) GetByID(ctx context.Context, id uuid.UUID) (*Session, error) {
	return r.queryOne(ctx, `WHERE id = $1 AND deleted_at IS NULL`, id)
}

func (r *pgRepository) GetActive(ctx context.Context, userID, projectID uuid.UUID) (*Session, error) {
	if projectID == uuid.Nil {
		return r.queryOne(ctx,
			`WHERE user_id = $1 AND ended_at IS NULL AND deleted_at IS NULL
			 ORDER BY started_at DESC LIMIT 1`, userID)
	}
	return r.queryOne(ctx,
		`WHERE user_id = $1 AND project_id = $2 AND ended_at IS NULL
		   AND deleted_at IS NULL
		 ORDER BY started_at DESC LIMIT 1`, userID, projectID)
}

func (r *pgRepository) List(ctx context.Context, userID uuid.UUID, limit int) ([]Session, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT id, organization_id, project_id, user_id, title, COALESCE(summary,''),
		        tags, started_at, ended_at, created_at
		 FROM sessions
		 WHERE user_id = $1 AND deleted_at IS NULL
		 ORDER BY started_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.OrganizationID, &sess.ProjectID, &sess.UserID, &sess.Title, &sess.Summary,
			&sess.Tags, &sess.StartedAt, &sess.EndedAt, &sess.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// EndAndLoad implementa el End() del Service. Reusa tx-context si existe,
// sino abre tx propia con FOR UPDATE para lock pesimista de la fila.
func (r *pgRepository) EndAndLoad(ctx context.Context, id uuid.UUID, summary string, endedAt time.Time) (*Session, error) {
	var tx pgx.Tx
	ownedTx := false
	if existing := txctx.TxFromContext(ctx); existing != nil {
		tx = existing
	} else {
		var err error
		tx, err = r.pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return nil, fmt.Errorf("begin tx: %w", err)
		}
		ownedTx = true
		defer func() { _ = tx.Rollback(ctx) }()
	}

	var sess Session
	err := tx.QueryRow(ctx,
		`SELECT id, organization_id, project_id, user_id, title, COALESCE(summary,''),
		        tags, started_at, ended_at, created_at
		 FROM sessions WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, id,
	).Scan(&sess.ID, &sess.OrganizationID, &sess.ProjectID, &sess.UserID, &sess.Title, &sess.Summary,
		&sess.Tags, &sess.StartedAt, &sess.EndedAt, &sess.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}
	if sess.EndedAt != nil {
		return nil, ErrAlreadyEnded
	}

	_, err = tx.Exec(ctx,
		`UPDATE sessions SET ended_at = $2, summary = $3 WHERE id = $1`,
		id, endedAt, nullStr(summary))
	if err != nil {
		return nil, fmt.Errorf("end session: %w", err)
	}
	if ownedTx {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit: %w", err)
		}
	}
	sess.EndedAt = &endedAt
	sess.Summary = summary
	return &sess, nil
}

func (r *pgRepository) CloseInactive(ctx context.Context, cutoff time.Time, now time.Time) ([]uuid.UUID, error) {
	// NOTE: preservamos el comportamiento original (HU-28.1: behavior-preserving).
	// La query usa $2 (now) tanto para SET ended_at como para WHERE updated_at < $2.
	// $1 (cutoff) queda sin usar — se mantiene en la firma para mantener
	// compatibilidad de invocación con CloseInactive(idle).
	_ = cutoff
	rows, err := r.pool.Query(ctx, `
		UPDATE sessions SET ended_at = $2
		WHERE ended_at IS NULL
		  AND deleted_at IS NULL
		  AND updated_at < $2
		RETURNING id
	`, cutoff, now)
	if err != nil {
		return nil, fmt.Errorf("close inactive: %w", err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *pgRepository) queryOne(ctx context.Context, where string, args ...any) (*Session, error) {
	var sess Session
	q := `SELECT id, organization_id, project_id, user_id, title, COALESCE(summary,''),
	        tags, started_at, ended_at, created_at FROM sessions ` + where
	err := r.q(ctx).QueryRow(ctx, q, args...).Scan(
		&sess.ID, &sess.OrganizationID, &sess.ProjectID, &sess.UserID, &sess.Title, &sess.Summary,
		&sess.Tags, &sess.StartedAt, &sess.EndedAt, &sess.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}
	return &sess, nil
}

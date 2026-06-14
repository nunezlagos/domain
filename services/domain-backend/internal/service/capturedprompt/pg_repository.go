package capturedprompt

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

const selectCols = `id, organization_id, user_id, session_id, project_id,
		content, COALESCE(client_kind,''), COALESCE(model,''),
		char_count, captured_at`

func scanPrompt(row pgx.Row) (*Prompt, error) {
	var p Prompt
	if err := row.Scan(
		&p.ID, &p.OrganizationID, &p.UserID, &p.SessionID, &p.ProjectID,
		&p.Content, &p.ClientKind, &p.Model, &p.CharCount, &p.CapturedAt,
	); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *pgRepository) Insert(ctx context.Context, in InsertParams) (*Prompt, error) {
	row := r.q(ctx).QueryRow(ctx,
		`INSERT INTO captured_prompts
		   (organization_id, user_id, session_id, project_id,
		    content, client_kind, model, char_count)
		 VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),$8)
		 RETURNING `+selectCols,
		in.OrganizationID, in.UserID, in.SessionID, in.ProjectID,
		in.Content, in.ClientKind, in.Model, in.CharCount,
	)
	p, err := scanPrompt(row)
	if err != nil {
		return nil, fmt.Errorf("insert captured_prompt: %w", err)
	}
	return p, nil
}

func (r *pgRepository) Get(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*Prompt, error) {
	row := r.q(ctx).QueryRow(ctx,
		`SELECT `+selectCols+`
		 FROM captured_prompts
		 WHERE organization_id = $1 AND id = $2`,
		orgID, id,
	)
	p, err := scanPrompt(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get captured_prompt: %w", err)
	}
	return p, nil
}

func (r *pgRepository) List(ctx context.Context, orgID uuid.UUID, filter ListFilter) ([]*Prompt, int64, error) {
	conds := []string{"organization_id = $1"}
	args := []any{orgID}
	idx := 2
	if filter.SessionID != nil {
		conds = append(conds, fmt.Sprintf("session_id = $%d", idx))
		args = append(args, *filter.SessionID)
		idx++
	}
	if filter.ProjectID != nil {
		conds = append(conds, fmt.Sprintf("project_id = $%d", idx))
		args = append(args, *filter.ProjectID)
		idx++
	}
	if filter.UserID != nil {
		conds = append(conds, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, *filter.UserID)
		idx++
	}
	where := "WHERE " + joinAnd(conds)

	var total int64
	if err := r.q(ctx).QueryRow(ctx,
		`SELECT COUNT(*) FROM captured_prompts `+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count captured_prompts: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	args = append(args, limit, filter.Offset)
	rows, err := r.q(ctx).Query(ctx,
		`SELECT `+selectCols+`
		 FROM captured_prompts `+where+`
		 ORDER BY captured_at DESC
		 LIMIT $`+itoa(idx)+` OFFSET $`+itoa(idx+1),
		args...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list captured_prompts: %w", err)
	}
	defer rows.Close()
	out := make([]*Prompt, 0, limit)
	for rows.Next() {
		p, err := scanPrompt(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan captured_prompt: %w", err)
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func joinAnd(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " AND "
		}
		out += p
	}
	return out
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

// Package client — pg_repository.go: implementación PG del Repository.
//
// Wrappea el pool y honra tx-context (si el middleware HTTP inyectó una tx,
// las queries corren contra esa tx para que RLS aplique).
package client

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// NewPgRepository construye el repository PG.
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

// selectCols centraliza la lista de columnas para SELECT (Scan también ordena
// por este listado).
const selectCols = `id, organization_id, name, slug,
		COALESCE(tax_id, ''), COALESCE(contact_email, ''),
		COALESCE(contact_phone, ''), COALESCE(address, ''),
		metadata, status, created_at, updated_at, deleted_at`

func scanClient(row pgx.Row) (*Client, error) {
	var c Client
	if err := row.Scan(
		&c.ID, &c.OrganizationID, &c.Name, &c.Slug,
		&c.TaxID, &c.ContactEmail, &c.ContactPhone, &c.Address,
		&c.Metadata, &c.Status, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
	); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *pgRepository) Insert(ctx context.Context, in InsertParams) (*Client, error) {
	row := r.q(ctx).QueryRow(ctx,
		`INSERT INTO clients
		   (organization_id, name, slug, tax_id, contact_email,
		    contact_phone, address, metadata, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING `+selectCols,
		in.OrganizationID, in.Name, in.Slug,
		nullStr(in.TaxID), nullStr(in.ContactEmail),
		nullStr(in.ContactPhone), nullStr(in.Address),
		in.MetadataJSON, in.Status,
	)
	c, err := scanClient(row)
	if err != nil {
		// El service interpreta pgErr para mapear a ErrClientSlugExists.
		return nil, err
	}
	return c, nil
}

func (r *pgRepository) GetByID(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*Client, error) {
	row := r.q(ctx).QueryRow(ctx,
		`SELECT `+selectCols+`
		 FROM clients
		 WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL`,
		orgID, id,
	)
	c, err := scanClient(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get client by id: %w", err)
	}
	return c, nil
}

func (r *pgRepository) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Client, error) {
	row := r.q(ctx).QueryRow(ctx,
		`SELECT `+selectCols+`
		 FROM clients
		 WHERE organization_id = $1 AND slug = $2 AND deleted_at IS NULL`,
		orgID, slug,
	)
	c, err := scanClient(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get client by slug: %w", err)
	}
	return c, nil
}

func (r *pgRepository) List(ctx context.Context, orgID uuid.UUID, f ListFilter) ([]*Client, int64, error) {
	args := []any{orgID}
	conds := []string{"organization_id = $1"}
	if !f.IncludeDeleted {
		conds = append(conds, "deleted_at IS NULL")
	}
	if f.Status != "" {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf("status = $%d", len(args)))
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		args = append(args, "%"+s+"%")
		conds = append(conds, fmt.Sprintf("(name ILIKE $%d OR slug ILIKE $%d)", len(args), len(args)))
	}
	where := strings.Join(conds, " AND ")

	// Count total (sin limit/offset) en query separada — más simple que
	// window function y compatible con los planners de PG sin issues.
	var total int64
	if err := r.q(ctx).QueryRow(ctx,
		`SELECT COUNT(*) FROM clients WHERE `+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count clients: %w", err)
	}

	// Append limit/offset al final.
	args = append(args, f.Limit, f.Offset)
	q := `SELECT ` + selectCols + ` FROM clients WHERE ` + where +
		fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))

	rows, err := r.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list clients: %w", err)
	}
	defer rows.Close()

	var out []*Client
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *pgRepository) Update(ctx context.Context, orgID uuid.UUID, id uuid.UUID, in UpdateParams) (*Client, error) {
	row := r.q(ctx).QueryRow(ctx,
		`UPDATE clients
		 SET name = $3, tax_id = $4, contact_email = $5, contact_phone = $6,
		     address = $7, metadata = $8, status = $9
		 WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL
		 RETURNING `+selectCols,
		orgID, id,
		in.Name, nullStr(in.TaxID), nullStr(in.ContactEmail),
		nullStr(in.ContactPhone), nullStr(in.Address),
		in.MetadataJSON, in.Status,
	)
	c, err := scanClient(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update client: %w", err)
	}
	return c, nil
}

func (r *pgRepository) SoftDelete(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	tag, err := r.q(ctx).Exec(ctx,
		`UPDATE clients SET deleted_at = NOW()
		 WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL`,
		orgID, id,
	)
	if err != nil {
		return fmt.Errorf("soft delete client: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrClientNotFound
	}
	return nil
}

func (r *pgRepository) Restore(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	tag, err := r.q(ctx).Exec(ctx,
		`UPDATE clients SET deleted_at = NULL
		 WHERE organization_id = $1 AND id = $2 AND deleted_at IS NOT NULL`,
		orgID, id,
	)
	if err != nil {
		return fmt.Errorf("restore client: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrClientNotFound
	}
	return nil
}

func (r *pgRepository) SetStatus(ctx context.Context, orgID uuid.UUID, id uuid.UUID, status string) (*Client, error) {
	row := r.q(ctx).QueryRow(ctx,
		`UPDATE clients SET status = $3
		 WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL
		 RETURNING `+selectCols,
		orgID, id, status,
	)
	c, err := scanClient(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("set client status: %w", err)
	}
	return c, nil
}

// nullStr — empty string → NULL para columnas opcionales (tax_id, contact_*,
// address). Mantiene la convención de project/observation.
func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

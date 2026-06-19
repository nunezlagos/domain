// Package txctx — issue-25.5 helper de transacciones con SET LOCAL para RLS.
//
// Cada query sobre tablas con RLS debe ir dentro de un tx con
// `SET LOCAL app.current_org_id = $1` ejecutado primero. Este helper lo automatiza.
//
// Sin WithOrgTx → queries sobre auth_secrets/audit_log/auth_otp_codes/activity_log/auth_api_keys
// devuelven 0 rows (RLS deniega). Es defense-in-depth contra bugs RBAC en app.
package txctx

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WithOrgTx ejecuta fn dentro de una tx con SET LOCAL app.current_org_id seteado.
// fn recibe la tx; si retorna error, rollback; sino commit.
// orgID requerido (NO permite uuid.Nil).
func WithOrgTx(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, fn func(pgx.Tx) error) error {
	if orgID == uuid.Nil {
		return fmt.Errorf("WithOrgTx: orgID required (uuid.Nil rejected)")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgID.String()); err != nil {
		return fmt.Errorf("set_config org: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// WithUserTx setea app.current_user_id (para tablas como auth_otp_codes con RLS user-scoped).
func WithUserTx(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, fn func(pgx.Tx) error) error {
	if userID == uuid.Nil {
		return fmt.Errorf("WithUserTx: userID required")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_user_id', $1, true)", userID.String()); err != nil {
		return fmt.Errorf("set_config user: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// WithOrgUserTx setea ambos contextos (más común en HTTP handlers post-auth).
func WithOrgUserTx(ctx context.Context, pool *pgxpool.Pool, orgID, userID uuid.UUID, fn func(pgx.Tx) error) error {
	if orgID == uuid.Nil {
		return fmt.Errorf("WithOrgUserTx: orgID required")
	}
	if userID == uuid.Nil {
		return fmt.Errorf("WithOrgUserTx: userID required")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`SELECT set_config('app.current_org_id', $1, true), set_config('app.current_user_id', $2, true)`,
		orgID.String(), userID.String()); err != nil {
		return fmt.Errorf("set_config org+user: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

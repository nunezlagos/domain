// Package dbstats — issue-25.2 monitoreo de queries lentas via pg_stat_statements.
//
// Requiere postgresql.conf con shared_preload_libraries='pg_stat_statements'
// y CREATE EXTENSION pg_stat_statements en la DB. Si la extensión no está
// cargada, Available() retorna false y los métodos retornan ErrNotAvailable.
//
// Funcionalidad:
//   - Available(): detecta si pg_stat_statements está disponible
//   - SlowQueries(threshold_ms, limit): top-N queries lentas
//   - Snapshot(): captura estado actual a domain_query_stats_history
//   - Reset(): pg_stat_statements_reset() (caller debe tener privilegio)
//
// El llamado se hace periódicamente desde un cron job (issue-27.1 jobs).
package dbstats

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotAvailable = errors.New("pg_stat_statements extension not available")

type SlowQuery struct {
	QueryID         int64
	QueryText       string
	Calls           int64
	TotalExecTime   float64
	MeanExecTime    float64
	RowsReturned    int64
	SharedBlksHit   int64
	SharedBlksRead  int64
}

type SnapshotResult struct {
	CapturedAt time.Time
	Inserted   int
}

type Service struct {
	Pool *pgxpool.Pool
}

// Available verifica que la extensión pg_stat_statements esté instalada en la DB.
func (s *Service) Available(ctx context.Context) (bool, error) {
	var exists bool
	err := s.Pool.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements'
		 )`,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check extension: %w", err)
	}
	return exists, nil
}

// SlowQueries devuelve top-N queries con mean_exec_time >= threshold_ms.
// Ordenadas por mean_exec_time DESC. Requiere extensión disponible.
func (s *Service) SlowQueries(ctx context.Context, thresholdMS float64, limit int) ([]SlowQuery, error) {
	ok, err := s.Available(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotAvailable
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
SELECT queryid, query, calls, total_exec_time, mean_exec_time,
       rows, shared_blks_hit, shared_blks_read
FROM pg_stat_statements
WHERE mean_exec_time >= $1
ORDER BY mean_exec_time DESC
LIMIT $2`, thresholdMS, limit)
	if err != nil {
		return nil, fmt.Errorf("query pg_stat_statements: %w", err)
	}
	defer rows.Close()
	var out []SlowQuery
	for rows.Next() {
		var q SlowQuery
		var queryIDNullable *int64
		if err := rows.Scan(&queryIDNullable, &q.QueryText, &q.Calls,
			&q.TotalExecTime, &q.MeanExecTime, &q.RowsReturned,
			&q.SharedBlksHit, &q.SharedBlksRead); err != nil {
			return nil, err
		}
		if queryIDNullable != nil {
			q.QueryID = *queryIDNullable
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// Snapshot captura el estado actual de pg_stat_statements a la tabla
// domain_query_stats_history. Útil como cron weekly antes de Reset().
func (s *Service) Snapshot(ctx context.Context) (*SnapshotResult, error) {
	ok, err := s.Available(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotAvailable
	}
	now := time.Now().UTC()
	tag, err := s.Pool.Exec(ctx, `
INSERT INTO domain_query_stats_history
   (captured_at, query_text, queryid, calls, total_exec_time, mean_exec_time,
    rows_returned, shared_blks_hit, shared_blks_read)
SELECT $1, query, queryid, calls, total_exec_time, mean_exec_time,
       rows, shared_blks_hit, shared_blks_read
FROM pg_stat_statements
WHERE calls > 0`, now)
	if err != nil {
		return nil, fmt.Errorf("snapshot insert: %w", err)
	}
	return &SnapshotResult{CapturedAt: now, Inserted: int(tag.RowsAffected())}, nil
}

// Reset llama pg_stat_statements_reset(). Requiere privilegio
// pg_read_all_stats o superuser; en producción Auth pool (app_admin) debe
// tener el GRANT correspondiente.
func (s *Service) Reset(ctx context.Context) error {
	ok, err := s.Available(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotAvailable
	}
	if _, err := s.Pool.Exec(ctx, `SELECT pg_stat_statements_reset()`); err != nil {
		return fmt.Errorf("reset: %w", err)
	}
	return nil
}

// HistorySince devuelve snapshots de history desde un timestamp dado.
func (s *Service) HistorySince(ctx context.Context, since time.Time, limit int) ([]SlowQuery, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
SELECT COALESCE(queryid, 0), query_text, calls, total_exec_time, mean_exec_time,
       rows_returned, shared_blks_hit, shared_blks_read
FROM domain_query_stats_history
WHERE captured_at >= $1
ORDER BY captured_at DESC, mean_exec_time DESC
LIMIT $2`, since, limit)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}
	defer rows.Close()
	var out []SlowQuery
	for rows.Next() {
		var q SlowQuery
		if err := rows.Scan(&q.QueryID, &q.QueryText, &q.Calls,
			&q.TotalExecTime, &q.MeanExecTime, &q.RowsReturned,
			&q.SharedBlksHit, &q.SharedBlksRead); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// ensure pgx import (para futuras queries que retornan pgx.ErrNoRows)
var _ = pgx.ErrNoRows

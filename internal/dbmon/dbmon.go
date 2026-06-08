// Package dbmon — HU-25.12 monitoring de locks, vacuum y conexiones Postgres.
//
// Expone queries periódicas sobre pg_stat_user_tables, pg_stat_activity y
// pg_locks. El consumidor publica las métricas (Prometheus, log, endpoint).
package dbmon

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Snapshot agrupa todas las métricas del cluster en un punto en el tiempo.
type Snapshot struct {
	Connections ConnectionStats `json:"connections"`
	Tables      []TableStats    `json:"tables"`
	Locks       LockStats       `json:"locks"`
}

// ConnectionStats — escenario 5.
type ConnectionStats struct {
	Active               int     `json:"active"`
	Idle                 int     `json:"idle"`
	IdleInTransaction    int     `json:"idle_in_transaction"`
	LongestQuerySeconds  float64 `json:"longest_query_seconds"`
}

// TableStats — escenarios 2 + 3 + 5.
type TableStats struct {
	Schema                string  `json:"schema"`
	Name                  string  `json:"name"`
	LiveTuples            int64   `json:"live_tuples"`
	DeadTuples            int64   `json:"dead_tuples"`
	DeadRatio             float64 `json:"dead_ratio"` // dead/live, 0 si live=0
	LastAutovacuumAgeSecs *int64  `json:"last_autovacuum_age_secs,omitempty"`
	BloatBytes            int64   `json:"bloat_bytes"`
}

// LockStats — escenario 1.
type LockStats struct {
	WaitingCount         int     `json:"waiting_count"`
	LongestWaitSeconds   float64 `json:"longest_wait_seconds"`
}

// Collector consulta el cluster y arma snapshots.
type Collector struct {
	Pool *pgxpool.Pool
}

// Collect retorna un snapshot completo (3 queries: conexiones, tablas, locks).
func (c *Collector) Collect(ctx context.Context) (*Snapshot, error) {
	conns, err := c.collectConnections(ctx)
	if err != nil {
		return nil, fmt.Errorf("connections: %w", err)
	}
	tables, err := c.collectTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("tables: %w", err)
	}
	locks, err := c.collectLocks(ctx)
	if err != nil {
		return nil, fmt.Errorf("locks: %w", err)
	}
	return &Snapshot{Connections: conns, Tables: tables, Locks: locks}, nil
}

func (c *Collector) collectConnections(ctx context.Context) (ConnectionStats, error) {
	var s ConnectionStats
	err := c.Pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN state = 'active' THEN 1 ELSE 0 END), 0) AS active,
			COALESCE(SUM(CASE WHEN state = 'idle' THEN 1 ELSE 0 END), 0) AS idle,
			COALESCE(SUM(CASE WHEN state = 'idle in transaction' THEN 1 ELSE 0 END), 0) AS idle_in_tx,
			COALESCE(MAX(EXTRACT(EPOCH FROM (now() - query_start))) FILTER (WHERE state = 'active'), 0) AS longest
		FROM pg_stat_activity
		WHERE pid <> pg_backend_pid() AND datname = current_database()
	`).Scan(&s.Active, &s.Idle, &s.IdleInTransaction, &s.LongestQuerySeconds)
	return s, err
}

func (c *Collector) collectTables(ctx context.Context) ([]TableStats, error) {
	rows, err := c.Pool.Query(ctx, `
		SELECT schemaname, relname, n_live_tup, n_dead_tup,
			EXTRACT(EPOCH FROM (now() - COALESCE(last_autovacuum, last_analyze)))::bigint AS vacuum_age,
			COALESCE(pg_total_relation_size(schemaname || '.' || relname), 0) AS total_bytes
		FROM pg_stat_user_tables
		WHERE schemaname = 'public'
		ORDER BY n_dead_tup DESC
		LIMIT 50
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TableStats
	for rows.Next() {
		var t TableStats
		var ageSecs *int64
		if err := rows.Scan(&t.Schema, &t.Name, &t.LiveTuples, &t.DeadTuples,
			&ageSecs, &t.BloatBytes); err != nil {
			return nil, err
		}
		if ageSecs != nil {
			t.LastAutovacuumAgeSecs = ageSecs
		}
		if t.LiveTuples > 0 {
			t.DeadRatio = float64(t.DeadTuples) / float64(t.LiveTuples)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (c *Collector) collectLocks(ctx context.Context) (LockStats, error) {
	var s LockStats
	err := c.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE NOT granted) AS waiting,
			COALESCE(MAX(EXTRACT(EPOCH FROM (now() - a.query_start))) FILTER (WHERE NOT l.granted), 0) AS longest
		FROM pg_locks l
		LEFT JOIN pg_stat_activity a ON a.pid = l.pid
	`).Scan(&s.WaitingCount, &s.LongestWaitSeconds)
	return s, err
}

// Alerts evalúa el snapshot y retorna mensajes de warning/critical para acción.
type Alert struct {
	Level   string `json:"level"` // "warning" | "critical"
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Evaluate corre las reglas básicas de alertas (escenarios HU).
func Evaluate(s *Snapshot) []Alert {
	var alerts []Alert
	if s.Connections.IdleInTransaction > 10 {
		alerts = append(alerts, Alert{
			Level:   "warning",
			Code:    "idle_in_transaction_high",
			Message: fmt.Sprintf("%d idle in transaction (umbral 10)", s.Connections.IdleInTransaction),
		})
	}
	if s.Connections.LongestQuerySeconds > 60 {
		alerts = append(alerts, Alert{
			Level:   "warning",
			Code:    "long_running_query",
			Message: fmt.Sprintf("query corriendo %.1fs (umbral 60s)", s.Connections.LongestQuerySeconds),
		})
	}
	if s.Locks.LongestWaitSeconds > 5 {
		alerts = append(alerts, Alert{
			Level:   "warning",
			Code:    "lock_wait_high",
			Message: fmt.Sprintf("lock wait %.1fs (umbral 5s)", s.Locks.LongestWaitSeconds),
		})
	}
	for _, t := range s.Tables {
		if t.LiveTuples > 1000 && t.DeadRatio > 0.5 {
			alerts = append(alerts, Alert{
				Level: "warning",
				Code:  "dead_tuples_high",
				Message: fmt.Sprintf("table %s tiene dead_ratio=%.2f (umbral 0.5) — vacuum no progresa",
					t.Name, t.DeadRatio),
			})
		}
	}
	return alerts
}

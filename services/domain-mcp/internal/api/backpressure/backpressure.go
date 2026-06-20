// Package backpressure — issue-26.6 shed-load + per-org quota.
//
// Antes de encolar un agent_run / flow_run / webhook_delivery, el caller
// consulta CheckQueue() para verificar que la queue global no esté saturada
// y que la org no haya excedido su quota concurrente.
package backpressure

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Errores devueltos por CheckQueue.
var (
	// ErrQueueFull queue global excedida → 429 Retry-After.
	ErrQueueFull = errors.New("queue_full")
	// ErrOrgQuotaExceeded org excedió su límite concurrente → 429 sin retry inmediato.
	ErrOrgQuotaExceeded = errors.New("org_queue_limit_exceeded")
)

// Queue describe una queue con cap global y per-org.
type Queue struct {
	// Name identifica la queue (usado en métricas y errores).
	Name string
	// Table es la tabla SQL que respalda la queue.
	Table string
	// PendingCondition es la WHERE clause que define "pending" (sin trailing AND).
	PendingCondition string
	// GlobalCap máximo total pending. 0 = sin cap global.
	GlobalCap int
	// PerOrgCap máximo pending por organización. 0 = sin cap per-org.
	PerOrgCap int
	// OrgColumn nombre de la columna de organization_id en la tabla.
	OrgColumn string
}

// PredefinedQueues catálogo de queues con sus caps default.
// Los caps reales se overridean por config o per-plan.
var PredefinedQueues = map[string]Queue{
	"agent_runs": {
		Name: "agent_runs", Table: "agent_runs",
		PendingCondition: "status IN ('pending','running')",
		GlobalCap:        5000, PerOrgCap: 100, OrgColumn: "organization_id",
	},
	"flow_runs": {
		Name: "flow_runs", Table: "flow_runs",
		PendingCondition: "status IN ('pending','running')",
		GlobalCap:        5000, PerOrgCap: 100, OrgColumn: "organization_id",
	},
	"webhook_outbound_deliveries": {
		Name: "webhook_outbound_deliveries", Table: "webhook_outbound_deliveries",
		PendingCondition: "status = 'pending'",
		GlobalCap:        20000, PerOrgCap: 2000, OrgColumn: "organization_id",
	},
}

// Limiter consulta queue depths para shed-load decisions.
type Limiter struct {
	Pool *pgxpool.Pool
}

// CheckQueue retorna nil si se puede encolar; ErrQueueFull o ErrOrgQuotaExceeded
// con métricas Retry-After info en el caller layer.
//
// Hace 2 queries paralelas-conceptualmente (1 query con CTE para evitar 2 roundtrips).
func (l *Limiter) CheckQueue(ctx context.Context, q Queue, orgID uuid.UUID) error {
	// Si caps en 0 → skip
	if q.GlobalCap == 0 && q.PerOrgCap == 0 {
		return nil
	}
	sql := fmt.Sprintf(
		`SELECT
			(SELECT COUNT(*) FROM %s WHERE %s) AS global_count,
			(SELECT COUNT(*) FROM %s WHERE %s AND %s = $1) AS org_count`,
		q.Table, q.PendingCondition,
		q.Table, q.PendingCondition, q.OrgColumn)
	var globalCount, orgCount int
	if err := l.Pool.QueryRow(ctx, sql, orgID).Scan(&globalCount, &orgCount); err != nil {
		// En caso de fail del check, permitir (fail-open) — log en caller.
		return fmt.Errorf("check queue: %w", err)
	}
	if q.GlobalCap > 0 && globalCount >= q.GlobalCap {
		return ErrQueueFull
	}
	if q.PerOrgCap > 0 && orgCount >= q.PerOrgCap {
		return ErrOrgQuotaExceeded
	}
	return nil
}

// RetryAfterSeconds heurística de cuánto esperar antes de reintentar.
// 60s para queue_full global, 5s para org-specific (puede liberarse rápido).
func RetryAfterSeconds(err error) int {
	switch {
	case errors.Is(err, ErrQueueFull):
		return 60
	case errors.Is(err, ErrOrgQuotaExceeded):
		return 5
	}
	return 30
}

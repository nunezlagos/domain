// issue-09.6 — worker claim atómico (de-003).
//
// ClaimRun intenta tomar ownership de un flow_run en pending o cuyo
// heartbeat haya expirado (stale). Retorna el runID + cursor si obtuvo
// el lock, nil si no había disponibles.
package flowrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ClaimedRun resultado de ClaimRun.
type ClaimedRun struct {
	RunID        uuid.UUID      `json:"run_id"`
	FlowID       uuid.UUID      `json:"flow_id"`
	Status       string         `json:"status"`
	Cursor       map[string]any `json:"cursor"`
	Outputs      map[string]any `json:"outputs"`
	Inputs       map[string]any `json:"inputs"`
	RecoveryCount int           `json:"recovery_count"`
	IsRecovery   bool           `json:"is_recovery"` // true si fue reclaim de run stale
}

// ClaimRunClaims es el pool de claims. Swappable para tests.
type ClaimRunClaims struct {
	Pool       *pgxpool.Pool
	WorkerID   string
	StaleAfter time.Duration
}

// ClaimRun intenta claim atómico de un flow_run disponible.
// Usa UPDATE ... RETURNING para atomicidad.
func (c *ClaimRunClaims) ClaimRun(ctx context.Context) (*ClaimedRun, error) {
	if c.StaleAfter <= 0 {
		c.StaleAfter = 5 * time.Minute
	}
	if c.WorkerID == "" {
		c.WorkerID = uuid.New().String()
	}

	var (
		runID         uuid.UUID
		flowID        uuid.UUID
		status        string
		cursorRaw     []byte
		outputsRaw    []byte
		inputsRaw     []byte
		recoveryCount int
	)
	// El RETURNING de un UPDATE devuelve los valores NUEVOS — para saber
	// si esto es recovery necesitamos el status PREVIO, capturado en la
	// subquery (sel.prev_status). Con RETURNING fr.status, IsRecovery era
	// SIEMPRE true (status recién seteado a 'running').
	err := c.Pool.QueryRow(ctx, `
		UPDATE flow_runs fr
		SET status = 'running',
		    worker_id = $1,
		    last_heartbeat_at = NOW(),
		    started_at = COALESCE(started_at, NOW())
		FROM (
			SELECT id, status AS prev_status FROM flow_runs
			WHERE (status = 'pending'
			   OR (status = 'running'
			       AND last_heartbeat_at IS NOT NULL
			       AND last_heartbeat_at < NOW() - $2::interval))
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		) sel
		WHERE fr.id = sel.id
		RETURNING fr.id, fr.flow_id, sel.prev_status, fr.cursor, fr.outputs, fr.inputs, fr.recovery_count
	`, c.WorkerID, c.StaleAfter).Scan(&runID, &flowID, &status,
		&cursorRaw, &outputsRaw, &inputsRaw, &recoveryCount)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // no runs available
		}
		return nil, fmt.Errorf("claim run: %w", err)
	}

	isRecovery := status == "running"
	cursor := map[string]any{}
	if len(cursorRaw) > 0 {
		_ = json.Unmarshal(cursorRaw, &cursor)
	}
	outputs := map[string]any{}
	if len(outputsRaw) > 0 {
		_ = json.Unmarshal(outputsRaw, &outputs)
	}
	inputs := map[string]any{}
	if len(inputsRaw) > 0 {
		_ = json.Unmarshal(inputsRaw, &inputs)
	}

	return &ClaimedRun{
		RunID:         runID,
		FlowID:        flowID,
		Status:        status,
		Cursor:        cursor,
		Outputs:       outputs,
		Inputs:        inputs,
		RecoveryCount: recoveryCount,
		IsRecovery:    isRecovery,
	}, nil
}

// ReleaseRun libera un flow_run sin finalizarlo (vuelve a pending).
func ReleaseRun(ctx context.Context, pool *pgxpool.Pool, runID uuid.UUID) error {
	_, err := pool.Exec(ctx,
		`UPDATE flow_runs SET status = 'pending', worker_id = NULL WHERE id = $1`, runID)
	if err != nil {
		return fmt.Errorf("release run: %w", err)
	}
	return nil
}

// IsStepReplaySafe retorna true si el step puede re-ejecutarse en resume.
// Por defecto (ReplaySafe nil) es seguro.
func IsStepReplaySafe(step map[string]any) bool {
	rs, ok := step["replay_safe"]
	if !ok || rs == nil {
		return true
	}
	b, _ := rs.(bool)
	return b
}

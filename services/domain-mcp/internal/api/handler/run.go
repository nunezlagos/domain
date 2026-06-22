package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/api/backpressure"
	agentrunner "nunezlagos/domain/internal/runner/agent"
	"nunezlagos/domain/internal/service/agent"
)

type runAgentBody struct {
	Input     string         `json:"input"`
	Variables map[string]any `json:"variables,omitempty"`
}

// POST /api/v1/agents/{id}/run — ejecucion sincrona del agent.
func (a *API) runAgent(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.AgentRunner == nil {
		writeError(w, http.StatusServiceUnavailable, "runner_disabled",
			"agent runner no configurado (DOMAIN_LLM_PROVIDER missing?)")
		return
	}

	// Existence pre-flight
	if _, err := a.AgentService.GetByID(r.Context(), id); err != nil {
		if errors.Is(err, agent.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}

	var b runAgentBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if b.Input == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "input requerido")
		return
	}
	// issue-26.6 backpressure: rechazar si queue agent_runs saturada
	if a.Backpressure != nil {
		orgID, _ := uuid.Parse(p.OrganizationID)
		if err := a.Backpressure.CheckQueue(r.Context(),
			backpressure.PredefinedQueues["agent_runs"], orgID); err != nil {
			if errors.Is(err, backpressure.ErrQueueFull) || errors.Is(err, backpressure.ErrOrgQuotaExceeded) {
				retry := backpressure.RetryAfterSeconds(err)
				w.Header().Set("Retry-After", strconv.Itoa(retry))
				code := "queue_full"
				if errors.Is(err, backpressure.ErrOrgQuotaExceeded) {
					code = "org_queue_limit_exceeded"
				}
				writeError(w, http.StatusTooManyRequests, code,
					fmt.Sprintf("retry after %d seconds", retry))
				return
			}
		}
	}
	userID, _ := uuid.Parse(p.UserID)
	res, runErr := a.AgentRunner.Run(r.Context(), agentrunner.RunInput{
		AgentID: id, UserID: &userID, UserPrompt: b.Input, Variables: b.Variables,
	})
	if res == nil {
		writeError(w, http.StatusInternalServerError, "run", runErr.Error())
		return
	}
	status := http.StatusOK
	if res.Status == agentrunner.StatusFailed {
		status = http.StatusUnprocessableEntity
	}
	writeData(w, status, map[string]any{
		"run_id":        res.RunID,
		"status":        res.Status,
		"output":        res.Output,
		"error":         res.Error,
		"tokens_input":  res.TokensInput,
		"tokens_output": res.TokensOutput,
		"iterations":    res.Iterations,
		"started_at":    res.StartedAt,
		"finished_at":   res.FinishedAt,
	})
}

// GET /api/v1/agent-runs/{id}/logs — devuelve agent_run_logs ordenados por iteration.
func (a *API) getAgentRunLogs(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	// organization_id eliminado del schema (single-org): se omite el chequeo
	// de ownership por org.

	rows, err := a.AgentService.Pool.Query(r.Context(),
		`SELECT id, iteration, event_type, payload, tokens_input, tokens_output,
		        latency_ms, occurred_at
		 FROM agent_run_logs WHERE agent_run_id = $1
		 ORDER BY iteration ASC, occurred_at ASC LIMIT 1000`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_logs", err.Error())
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var (
			logID      int64
			iteration  int
			eventType  string
			payloadRaw []byte
			tokensIn   int
			tokensOut  int
			latencyMS  int
			occurredAt any
		)
		if err := rows.Scan(&logID, &iteration, &eventType, &payloadRaw,
			&tokensIn, &tokensOut, &latencyMS, &occurredAt); err != nil {
			continue
		}
		var payload any
		if len(payloadRaw) > 0 {
			_ = json.Unmarshal(payloadRaw, &payload)
		}
		out = append(out, map[string]any{
			"id":            logID,
			"iteration":     iteration,
			"event_type":    eventType,
			"payload":       payload,
			"tokens_input":  tokensIn,
			"tokens_output": tokensOut,
			"latency_ms":    latencyMS,
			"occurred_at":   occurredAt,
		})
	}
	writeData(w, http.StatusOK, out)
}

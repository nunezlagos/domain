package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	agentrunner "nunezlagos/domain/internal/runner/agent"
	"nunezlagos/domain/internal/service/agent"
)

type runAgentBody struct {
	Input     string         `json:"input"`
	Variables map[string]any `json:"variables,omitempty"`
}

// POST /api/v1/agents/{id}/run — ejecución síncrona del agent.
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

	// Cross-org guard pre-flight
	ag, err := a.AgentService.GetByID(r.Context(), id)
	if errors.Is(err, agent.ErrNotFound) || (err == nil && ag.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
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

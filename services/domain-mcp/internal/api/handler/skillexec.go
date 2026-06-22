// Handlers de ejecucion de skills (issue-05.5): sync, async y polling.
package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	skillsvc "nunezlagos/domain/internal/service/skill"
)

type executeSkillBody struct {
	Parameters     map[string]any `json:"parameters,omitempty"`
	Mode           string         `json:"mode,omitempty"` // sync (default) | async
	TimeoutSeconds int            `json:"timeout_seconds,omitempty"`
}

// POST /api/v1/skills/{id}/execute
func (a *API) executeSkill(w http.ResponseWriter, r *http.Request) {
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
	if a.SkillExecution == nil {
		writeError(w, http.StatusServiceUnavailable, "execution_disabled", "")
		return
	}
	var b executeSkillBody
	_ = decodeJSON(r, &b)
	if b.Mode != "" && b.Mode != "sync" && b.Mode != "async" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "mode must be sync or async")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)

	exec, err := a.SkillExecution.Execute(r.Context(), skillsvc.ExecuteInput{
		OrganizationID: orgID, SkillID: id,
		Parameters: b.Parameters, Mode: b.Mode, TimeoutSeconds: b.TimeoutSeconds,
	})
	if errors.Is(err, skillsvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if errors.Is(err, skillsvc.ErrInvalidParams) {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "execute", err.Error())
		return
	}
	status := http.StatusOK
	if exec.Mode == "async" {
		status = http.StatusAccepted
		w.Header().Set("Location", "/api/v1/executions/"+exec.ID.String())
	}
	writeData(w, status, exec)
}

// GET /api/v1/executions/{id} — polling de ejecuciones async.
func (a *API) getExecution(w http.ResponseWriter, r *http.Request) {
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
	if a.SkillExecution == nil {
		writeError(w, http.StatusServiceUnavailable, "execution_disabled", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	exec, err := a.SkillExecution.Get(r.Context(), orgID, id)
	if errors.Is(err, skillsvc.ErrExecutionNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	writeData(w, http.StatusOK, exec)
}

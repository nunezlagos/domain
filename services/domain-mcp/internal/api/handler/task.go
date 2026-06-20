package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	tsvc "nunezlagos/domain/internal/service/task"
)

type createTasksBody struct {
	Tasks []tsvc.CreateTaskInput `json:"tasks"`
}

type updateTaskStatusBody struct {
	Status      string `json:"status"`
	CompletedBy string `json:"completed_by,omitempty"`
}

type createVerificationBody struct {
	Result     string `json:"result"`
	Evidence   string `json:"evidence,omitempty"`
	Notes      string `json:"notes,omitempty"`
	VerifiedBy string `json:"verified_by,omitempty"`
}

type createSabotageBody struct {
	Action          string `json:"action"`
	ExpectedFailure string `json:"expected_failure,omitempty"`
	ActualResult    string `json:"actual_result,omitempty"`
	Restored        bool   `json:"restored"`
}

// createTasks POST /api/v1/user-stories/{slug}/tasks
func (a *API) createTasks(w http.ResponseWriter, r *http.Request) {
	if a.TaskService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	huSlug := r.PathValue("slug")
	hu, err := a.HUService.GetBySlug(r.Context(), huSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "hu_not_found", "")
		return
	}
	var b createTasksBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	tasks, err := a.TaskService.CreateTasks(r.Context(), hu.ID, b.Tasks)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_tasks_failed", err.Error())
		return
	}
	writeData(w, http.StatusCreated, tasks)
}

// listTasks GET /api/v1/user-stories/{slug}/tasks
func (a *API) listTasks(w http.ResponseWriter, r *http.Request) {
	if a.TaskService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	huSlug := r.PathValue("slug")
	hu, err := a.HUService.GetBySlug(r.Context(), huSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "hu_not_found", "")
		return
	}
	tasks, err := a.TaskService.ListTasks(r.Context(), hu.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_tasks_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, tasks)
}

// getTask GET /api/v1/tasks/{id}
func (a *API) getTask(w http.ResponseWriter, r *http.Request) {
	if a.TaskService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	taskID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_task_id", "")
		return
	}
	t, err := a.TaskService.GetTask(r.Context(), taskID)
	if errors.Is(err, tsvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_task_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, t)
}

// updateTaskStatus PATCH /api/v1/tasks/{id}/status
func (a *API) updateTaskStatus(w http.ResponseWriter, r *http.Request) {
	if a.TaskService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	taskID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_task_id", "")
		return
	}
	var b updateTaskStatusBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	t, err := a.TaskService.UpdateTaskStatus(r.Context(), taskID, b.Status, b.CompletedBy)
	if err != nil {
		switch {
		case errors.Is(err, tsvc.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "")
		case errors.Is(err, tsvc.ErrInvalidStatus), errors.Is(err, tsvc.ErrInvalidTransition):
			writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "update_status_failed", err.Error())
		}
		return
	}
	writeData(w, http.StatusOK, t)
}

// getProgress GET /api/v1/user-stories/{slug}/progress
func (a *API) getProgress(w http.ResponseWriter, r *http.Request) {
	if a.TaskService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	huSlug := r.PathValue("slug")
	hu, err := a.HUService.GetBySlug(r.Context(), huSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "hu_not_found", "")
		return
	}
	p, err := a.TaskService.GetProgress(r.Context(), hu.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_progress_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, p)
}

// createVerification POST /api/v1/tasks/{id}/verification
func (a *API) createVerification(w http.ResponseWriter, r *http.Request) {
	if a.TaskService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	taskID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_task_id", "")
		return
	}
	var b createVerificationBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	v, err := a.TaskService.CreateVerification(r.Context(), taskID, b.Result, b.Evidence, b.Notes, b.VerifiedBy)
	if err != nil {
		switch {
		case errors.Is(err, tsvc.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "")
		case errors.Is(err, tsvc.ErrNotCompleted):
			writeError(w, http.StatusConflict, "task_not_completed", "verification requires task=completed")
		default:
			writeError(w, http.StatusInternalServerError, "create_verification_failed", err.Error())
		}
		return
	}
	writeData(w, http.StatusCreated, v)
}

// createSabotage POST /api/v1/tasks/{id}/sabotage
func (a *API) createSabotage(w http.ResponseWriter, r *http.Request) {
	if a.TaskService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	taskID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_task_id", "")
		return
	}
	var b createSabotageBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	s, err := a.TaskService.CreateSabotage(r.Context(), taskID, b.Action, b.ExpectedFailure, b.ActualResult, b.Restored)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_sabotage_failed", err.Error())
		return
	}
	writeData(w, http.StatusCreated, s)
}

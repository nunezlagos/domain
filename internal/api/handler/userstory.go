package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	usvc "nunezlagos/domain/internal/service/userstory"
)

type createHUBody struct {
	Slug        string              `json:"slug"`
	Title       string              `json:"title"`
	Description string              `json:"description,omitempty"`
	Status      string              `json:"status,omitempty"`
	Priority    string              `json:"priority,omitempty"`
	ReqSlug     string              `json:"req_slug"`
	Scenarios   []usvc.Scenario    `json:"scenarios,omitempty"`
}

type updateHUBody struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
	Priority    *string `json:"priority,omitempty"`
}

// createUserStory POST /api/v1/user-stories
func (a *API) createUserStory(w http.ResponseWriter, r *http.Request) {
	if a.HUService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	var b createHUBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	hu, err := a.HUService.Create(r.Context(), b.Slug, b.Title, b.Description, b.Status, b.Priority, b.ReqSlug, b.Scenarios)
	if err != nil {
		switch {
		case errors.Is(err, usvc.ErrSlugInvalid), errors.Is(err, usvc.ErrInvalidStatus), errors.Is(err, usvc.ErrInvalidPriority), errors.Is(err, usvc.ErrScenarioInvalid):
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		case errors.Is(err, usvc.ErrSlugTaken):
			writeError(w, http.StatusConflict, "slug_taken", err.Error())
		case errors.Is(err, usvc.ErrReqNotFound):
			writeError(w, http.StatusUnprocessableEntity, "req_not_found", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		}
		return
	}
	writeData(w, http.StatusCreated, hu)
}

// getUserStory GET /api/v1/user-stories/{slug}
func (a *API) getUserStory(w http.ResponseWriter, r *http.Request) {
	if a.HUService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	slug := r.PathValue("slug")
	hu, err := a.HUService.GetBySlug(r.Context(), slug)
	if errors.Is(err, usvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "user story not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, hu)
}

// listUserStories GET /api/v1/user-stories
func (a *API) listUserStories(w http.ResponseWriter, r *http.Request) {
	if a.HUService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	filter := usvc.UserStoryFilter{
		Status:   r.URL.Query().Get("status"),
		Priority: r.URL.Query().Get("priority"),
		ReqSlug:  r.URL.Query().Get("req_slug"),
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			filter.Limit = n
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			filter.Offset = n
		}
	}
	hus, err := a.HUService.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, hus)
}

// updateUserStory PATCH /api/v1/user-stories/{slug}
func (a *API) updateUserStory(w http.ResponseWriter, r *http.Request) {
	if a.HUService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	slug := r.PathValue("slug")
	var b updateHUBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	hu, err := a.HUService.Update(r.Context(), slug, b.Title, b.Description, b.Status, b.Priority)
	if err != nil {
		switch {
		case errors.Is(err, usvc.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, usvc.ErrInvalidStatus), errors.Is(err, usvc.ErrInvalidPriority):
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		}
		return
	}
	writeData(w, http.StatusOK, hu)
}

// deleteUserStory DELETE /api/v1/user-stories/{slug}
func (a *API) deleteUserStory(w http.ResponseWriter, r *http.Request) {
	if a.HUService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	slug := r.PathValue("slug")
	err := a.HUService.Delete(r.Context(), slug)
	if err != nil {
		if errors.Is(err, usvc.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// addScenario POST /api/v1/user-stories/{slug}/scenarios
func (a *API) addScenario(w http.ResponseWriter, r *http.Request) {
	if a.HUService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	slug := r.PathValue("slug")
	var sc usvc.Scenario
	if err := decodeJSON(r, &sc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	inserted, err := a.HUService.AddScenario(r.Context(), slug, sc)
	if err != nil {
		if errors.Is(err, usvc.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "user story not found")
			return
		}
		if errors.Is(err, usvc.ErrScenarioInvalid) {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "add_scenario_failed", err.Error())
		return
	}
	writeData(w, http.StatusCreated, inserted)
}

// removeScenario DELETE /api/v1/user-stories/{slug}/scenarios/{id}
func (a *API) removeScenario(w http.ResponseWriter, r *http.Request) {
	if a.HUService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	scenarioID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_scenario_id", "")
		return
	}
	err = a.HUService.RemoveScenario(r.Context(), scenarioID)
	if err != nil {
		if errors.Is(err, usvc.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "scenario not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "remove_scenario_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/timeline"
)

// GET /api/v1/context?project_slug=
func (a *API) getContext(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	userID, _ := uuid.Parse(p.UserID)
	var projectID uuid.UUID
	if ps := r.URL.Query().Get("project_slug"); ps != "" {
		proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, ps)
		if err != nil {
			writeError(w, http.StatusNotFound, "project_not_found", "")
			return
		}
		projectID = proj.ID
	}
	snap, err := a.TimelineService.Context(r.Context(), orgID, userID, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "context", err.Error())
		return
	}
	writeData(w, http.StatusOK, snap)
}

// GET /api/v1/observations/{id}/timeline?before=&after=
func (a *API) getTimeline(w http.ResponseWriter, r *http.Request) {
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
	orgID, _ := uuid.Parse(p.OrganizationID)
	before, _ := strconv.Atoi(r.URL.Query().Get("before"))
	after, _ := strconv.Atoi(r.URL.Query().Get("after"))
	if before == 0 {
		before = 3
	}
	if after == 0 {
		after = 3
	}
	entries, err := a.TimelineService.Timeline(r.Context(), orgID, id, before, after)
	if errors.Is(err, timeline.ErrObservationNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "timeline", err.Error())
		return
	}
	writeData(w, http.StatusOK, entries)
}

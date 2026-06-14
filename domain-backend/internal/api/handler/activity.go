package handler

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/activity"
	"nunezlagos/domain/internal/auth/apikey"
)

// listActivityLogs GET /api/v1/activity-logs
func (a *API) listActivityLogs(w http.ResponseWriter, r *http.Request) {
	p, ok := apikey.FromContext(r.Context())
	if !ok || p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "auth required")
		return
	}
	if a.ActivityQuerier == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "activity store not configured")
		return
	}
	orgID, err := uuid.Parse(p.OrganizationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_org", "invalid organization")
		return
	}

	filter := activity.Filter{
		OrganizationID: orgID,
		Limit:          50,
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			filter.Limit = n
		}
	}
	if pid := r.URL.Query().Get("project_id"); pid != "" {
		if id, err := uuid.Parse(pid); err == nil {
			filter.ProjectID = &id
		}
	}
	if eid := r.URL.Query().Get("entity_id"); eid != "" {
		if id, err := uuid.Parse(eid); err == nil {
			filter.EntityID = &id
		}
	}
	if et := r.URL.Query().Get("entity_type"); et != "" {
		filter.EntityType = et
	}

	entries, err := a.ActivityQuerier.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "activity_query_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, entries)
}

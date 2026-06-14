// Handlers de la Dead Letter Queue (issue-09.4 escenario 7).
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/flow"
)

// GET /api/v1/dlq — fallos permanentes pendientes de la org.
func (a *API) listDLQ(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	store := &flow.DLQStore{Pool: a.FlowService.Pool}
	entries, err := store.List(r.Context(), orgID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "dlq_list", err.Error())
		return
	}
	if entries == nil {
		entries = []flow.DLQEntry{}
	}
	writeData(w, http.StatusOK, entries)
}

// DELETE /api/v1/dlq/{id} — marca la entry como resuelta.
func (a *API) resolveDLQ(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	store := &flow.DLQStore{Pool: a.FlowService.Pool}
	if err := store.Resolve(r.Context(), orgID, id); err != nil {
		if errors.Is(err, flow.ErrDLQNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "dlq_resolve", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

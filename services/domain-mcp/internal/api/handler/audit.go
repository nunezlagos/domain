package handler

import (
	"net/http"
	"strconv"

	"nunezlagos/domain/internal/audit"
)

// listAuditLogs GET /api/v1/audit-logs.
func (a *API) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "auth required")
		return
	}

	filter := audit.AuditFilter{
		Limit:  50,
		Cursor: 0,
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			filter.Limit = n
		}
	}
	if c := r.URL.Query().Get("cursor"); c != "" {
		if n, err := strconv.Atoi(c); err == nil && n > 0 {
			filter.Cursor = int64(n)
		}
	}
	if e := r.URL.Query().Get("entity_type"); e != "" {
		filter.EntityType = e
	}
	if a := r.URL.Query().Get("action"); a != "" {
		filter.Action = a
	}

	entries, err := a.Audit.Query(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "audit_query_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, entries)
}

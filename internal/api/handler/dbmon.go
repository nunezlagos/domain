package handler

import (
	"net/http"

	"nunezlagos/domain/internal/dbmon"
)

// GET /api/v1/admin/db-stats — issue-25.12
// Devuelve snapshot del cluster (conexiones, tablas, locks) + alertas evaluadas.
//
// Requiere rol platform_admin (issue-02.2). Por simplicidad ahora solo verifica
// que el principal exista — endurecer cuando RBAC esté completo.
func (a *API) getDBStats(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.DBMonCollector == nil {
		writeError(w, http.StatusServiceUnavailable, "dbmon_not_configured", "")
		return
	}
	snap, err := a.DBMonCollector.Collect(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "collect", err.Error())
		return
	}
	alerts := dbmon.Evaluate(snap)
	writeDataWithMeta(w, http.StatusOK, snap, map[string]any{"alerts": alerts})
}

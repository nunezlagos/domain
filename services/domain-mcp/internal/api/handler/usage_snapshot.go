package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	usagesvc "nunezlagos/domain/internal/service/usage"
)

// usageCurrentSnapshot GET /api/v1/usage/current — issue-33.4.
// Devuelve snapshot del día UTC actual scoped a la org del principal.
// Read-only, sin params. POST/PUT/DELETE devuelven 405 vía router (Go 1.22+
// method routing).
func (a *API) usageCurrentSnapshot(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.UsageSnapshot == nil {
		writeError(w, http.StatusServiceUnavailable, "usage_snapshot_not_configured", "")
		return
	}
	orgID, err := uuid.Parse(p.OrganizationID)
	if err != nil || orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid organization_id in principal")
		return
	}
	snap, err := a.UsageSnapshot.Current(r.Context(), orgID)
	if errors.Is(err, usagesvc.ErrOrgNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "snapshot", err.Error())
		return
	}
	writeData(w, http.StatusOK, snap)
}

// usageHistory GET /api/v1/usage/history?days=N — issue-33.4.
// Default days=7, max days=365. Día más reciente primero. Gap-fill con ceros
// en días sin actividad.
func (a *API) usageHistory(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.UsageSnapshot == nil {
		writeError(w, http.StatusServiceUnavailable, "usage_snapshot_not_configured", "")
		return
	}
	orgID, err := uuid.Parse(p.OrganizationID)
	if err != nil || orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid organization_id in principal")
		return
	}
	days := 0
	if raw := r.URL.Query().Get("days"); raw != "" {
		n, perr := strconv.Atoi(raw)
		if perr != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_days", "days must be an integer")
			return
		}
		days = n
	}
	h, err := a.UsageSnapshot.History(r.Context(), orgID, days)
	if errors.Is(err, usagesvc.ErrInvalidDays) {
		writeError(w, http.StatusUnprocessableEntity, "invalid_days", err.Error())
		return
	}
	if errors.Is(err, usagesvc.ErrOrgNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "history", err.Error())
		return
	}
	writeData(w, http.StatusOK, h)
}

package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/usagealerts"
)

type createUsageAlertBody struct {
	Name         string   `json:"name"`
	Metric       string   `json:"metric"`
	Threshold    float64  `json:"threshold"`
	Condition    string   `json:"condition,omitempty"`
	Channel      string   `json:"channel,omitempty"`
	Recipients   []string `json:"recipients,omitempty"`
	CooldownSecs int      `json:"cooldown_secs,omitempty"`
}

// POST /api/v1/usage-alerts
func (a *API) createUsageAlert(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.UsageAlertsService == nil {
		writeError(w, http.StatusServiceUnavailable, "alerts_not_configured", "")
		return
	}
	var in createUsageAlertBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	alert, err := a.UsageAlertsService.Create(r.Context(), orgID,
		usagealerts.CreateInput{
			Name: in.Name, Metric: in.Metric, Threshold: in.Threshold,
			Condition: in.Condition, Channel: in.Channel,
			Recipients: in.Recipients, CooldownSecs: in.CooldownSecs,
		})
	if err != nil {
		switch {
		case errors.Is(err, usagealerts.ErrInvalidMetric):
			writeError(w, http.StatusUnprocessableEntity, "invalid_metric", err.Error())
		case errors.Is(err, usagealerts.ErrInvalidCondition):
			writeError(w, http.StatusUnprocessableEntity, "invalid_condition", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create", err.Error())
		}
		return
	}
	writeData(w, http.StatusCreated, alert)
}

// GET /api/v1/usage-alerts
func (a *API) listUsageAlerts(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	alerts, err := a.UsageAlertsService.ListByOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, alerts)
}

// DELETE /api/v1/usage-alerts/{id}
func (a *API) deleteUsageAlert(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	if err := a.UsageAlertsService.Delete(r.Context(), orgID, id); err != nil {
		if errors.Is(err, usagealerts.ErrUnknown) {
			writeError(w, http.StatusNotFound, "not_found", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

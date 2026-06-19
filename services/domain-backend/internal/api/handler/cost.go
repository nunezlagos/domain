package handler

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

// GET /api/v1/cost/daily?days=N&group_by=org|agent
// issue-15.1 + issue-15.2 — cost analytics
func (a *API) getCostDaily(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.CostService == nil {
		writeError(w, http.StatusServiceUnavailable, "cost_not_configured", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	groupBy := r.URL.Query().Get("group_by")
	if groupBy == "" {
		groupBy = "org"
	}
	switch groupBy {
	case "org":
		out, err := a.CostService.DailyByOrg(r.Context(), orgID, days)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query", err.Error())
			return
		}
		writeData(w, http.StatusOK, out)
	case "agent":
		out, err := a.CostService.DailyByAgent(r.Context(), orgID, days)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query", err.Error())
			return
		}
		writeData(w, http.StatusOK, out)
	default:
		writeError(w, http.StatusUnprocessableEntity, "invalid_group_by",
			"group_by must be org or agent")
	}
}

// GET /api/v1/usage — usage actual del mes para la org del principal (issue-21.3).
func (a *API) getCurrentUsage(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.BillingService == nil {
		writeError(w, http.StatusServiceUnavailable, "billing_not_configured", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	usage, err := a.BillingService.GetUsage(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "usage", err.Error())
		return
	}
	limits, err := a.BillingService.ResolveLimits(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "limits", err.Error())
		return
	}
	resp := map[string]any{
		"usage":  usage,
		"limits": limits,
	}
	writeData(w, http.StatusOK, resp)
}

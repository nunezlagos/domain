// Handlers de cost analytics (issue-15.2): spend, breakdown, forecast,
// budgets y export CSV.
package handler

import (
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/cost"
)

func (a *API) costOrg(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return uuid.Nil, false
	}
	if a.CostService == nil {
		writeError(w, http.StatusServiceUnavailable, "cost_disabled", "")
		return uuid.Nil, false
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	return orgID, true
}

func daysParam(r *http.Request) int {
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 30
}

// GET /api/v1/cost/spend/{granularity} — daily|weekly|monthly
func (a *API) getCostSpend(w http.ResponseWriter, r *http.Request) {
	orgID, ok := a.costOrg(w, r)
	if !ok {
		return
	}
	buckets, err := a.CostService.Spend(r.Context(), orgID, r.PathValue("granularity"), daysParam(r))
	if errors.Is(err, cost.ErrInvalidGranularity) {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "spend", err.Error())
		return
	}
	if buckets == nil {
		buckets = []cost.SpendBucket{}
	}
	writeData(w, http.StatusOK, buckets)
}

// GET /api/v1/cost/breakdown/{dimension} — provider|model|operation|agent|flow
func (a *API) getCostBreakdown(w http.ResponseWriter, r *http.Request) {
	orgID, ok := a.costOrg(w, r)
	if !ok {
		return
	}
	rows, err := a.CostService.Breakdown(r.Context(), orgID, r.PathValue("dimension"), daysParam(r))
	if errors.Is(err, cost.ErrInvalidDimension) {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "breakdown", err.Error())
		return
	}
	if rows == nil {
		rows = []cost.BreakdownRow{}
	}
	writeData(w, http.StatusOK, rows)
}

// GET /api/v1/cost/forecast?window=14
func (a *API) getCostForecast(w http.ResponseWriter, r *http.Request) {
	orgID, ok := a.costOrg(w, r)
	if !ok {
		return
	}
	window := 14
	if v := r.URL.Query().Get("window"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			window = n
		}
	}
	f, err := a.CostService.ForecastSMA(r.Context(), orgID, window)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "forecast", err.Error())
		return
	}
	writeData(w, http.StatusOK, f)
}

type createBudgetBody struct {
	Name                string  `json:"name"`
	AmountUSD           float64 `json:"amount_usd"`
	Period              string  `json:"period,omitempty"`
	WarningThresholdPct int     `json:"warning_threshold_pct,omitempty"`
}

// POST /api/v1/cost/budgets
func (a *API) createBudget(w http.ResponseWriter, r *http.Request) {
	orgID, ok := a.costOrg(w, r)
	if !ok {
		return
	}
	var b createBudgetBody
	if err := decodeJSON(r, &b); err != nil || b.Name == "" || b.AmountUSD <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "name and amount_usd > 0 required")
		return
	}
	budget, err := a.CostService.CreateBudget(r.Context(), orgID, b.Name, b.AmountUSD, b.Period, b.WarningThresholdPct)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	}
	w.Header().Set("Location", "/api/v1/cost/budgets/"+budget.ID.String())
	writeData(w, http.StatusCreated, budget)
}

// GET /api/v1/cost/budgets — con current_spend + status
func (a *API) listBudgets(w http.ResponseWriter, r *http.Request) {
	orgID, ok := a.costOrg(w, r)
	if !ok {
		return
	}
	budgets, err := a.CostService.ListBudgets(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "budgets", err.Error())
		return
	}
	if budgets == nil {
		budgets = []cost.Budget{}
	}
	writeData(w, http.StatusOK, budgets)
}

// DELETE /api/v1/cost/budgets/{id}
func (a *API) deleteBudget(w http.ResponseWriter, r *http.Request) {
	orgID, ok := a.costOrg(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err := a.CostService.DeleteBudget(r.Context(), orgID, id); err != nil {
		if errors.Is(err, cost.ErrBudgetNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// response-shape-lint:allow CSV export — responde text/csv, no JSON envelope
//
// GET /api/v1/cost/export?type=spend|breakdown&days=N
func (a *API) exportCost(w http.ResponseWriter, r *http.Request) {
	orgID, ok := a.costOrg(w, r)
	if !ok {
		return
	}
	exportType := r.URL.Query().Get("type")
	if exportType == "" {
		exportType = "spend"
	}
	rows, err := a.CostService.ExportCSV(r.Context(), orgID, exportType, daysParam(r))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=cost-"+exportType+".csv")
	cw := csv.NewWriter(w)
	_ = cw.WriteAll(rows)
	cw.Flush()
}

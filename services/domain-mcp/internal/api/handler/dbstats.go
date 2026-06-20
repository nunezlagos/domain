package handler

import (
	"net/http"
)

// GET /api/v1/admin/db/slow-queries — issue-25.2
func (a *API) getSlowQueries(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.DBStatsService == nil {
		writeError(w, http.StatusServiceUnavailable, "dbstats_not_configured", "")
		return
	}
	thresholdMS := 100.0
	limit := 50
	if t := r.URL.Query().Get("threshold_ms"); t != "" {
		if v, err := parseInt(t); err == nil && v > 0 {
			thresholdMS = float64(v)
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := parseInt(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}
	queries, err := a.DBStatsService.SlowQueries(r.Context(), thresholdMS, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "slow_queries", err.Error())
		return
	}
	writeData(w, http.StatusOK, queries)
}

func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

package handler

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// skillMetricsShow maneja GET /api/v1/skills/{slug}/metrics?days=7.
//
// Resuelve el slug -> skill_id via SkillService y devuelve la serie diaria de
// skill_metrics_daily de ese skill. Backend de HU-52.2 (lectura para CLI/admin).
func (a *API) skillMetricsShow(w http.ResponseWriter, r *http.Request) {
	if a.SkillMetrics == nil || a.Skills == nil {
		writeError(w, http.StatusServiceUnavailable, "metrics_disabled", "")
		return
	}
	slug := strings.TrimSpace(r.PathValue("slug"))
	if slug == "" {
		writeError(w, http.StatusBadRequest, "invalid_slug", "slug requerido")
		return
	}
	sk, err := a.Skills.GetBySlug(r.Context(), uuid.Nil, slug)
	if err != nil {
		writeError(w, http.StatusNotFound, "skill_not_found", err.Error())
		return
	}
	days := parseIntDefault(r.URL.Query().Get("days"), 7)
	rows, err := a.SkillMetrics.GetBySkill(r.Context(), sk.ID, days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "metrics_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, rows)
}

// skillMetricsTopFailed maneja GET /api/v1/skills/metrics/top-failed?days=7&limit=10.
func (a *API) skillMetricsTopFailed(w http.ResponseWriter, r *http.Request) {
	if a.SkillMetrics == nil {
		writeError(w, http.StatusServiceUnavailable, "metrics_disabled", "")
		return
	}
	days := parseIntDefault(r.URL.Query().Get("days"), 7)
	limit := parseIntDefault(r.URL.Query().Get("limit"), 10)
	rows, err := a.SkillMetrics.ListTopFailed(r.Context(), days, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "metrics_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, rows)
}

// skillMetricsSlowest maneja GET /api/v1/skills/metrics/slowest?days=7&limit=10.
func (a *API) skillMetricsSlowest(w http.ResponseWriter, r *http.Request) {
	if a.SkillMetrics == nil {
		writeError(w, http.StatusServiceUnavailable, "metrics_disabled", "")
		return
	}
	days := parseIntDefault(r.URL.Query().Get("days"), 7)
	limit := parseIntDefault(r.URL.Query().Get("limit"), 10)
	rows, err := a.SkillMetrics.ListSlowest(r.Context(), days, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "metrics_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, rows)
}

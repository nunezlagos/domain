package handler

import (
	"net/http"

	"github.com/google/uuid"
)

type addCodeRefBody struct {
	FilePath string `json:"file_path"`
	Repo     string `json:"repo,omitempty"`
	Branch   string `json:"branch,omitempty"`
}

// getRequirementTrace GET /api/v1/traceability/req/{slug}
func (a *API) getRequirementTrace(w http.ResponseWriter, r *http.Request) {
	if a.TraceService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	slug := r.PathValue("slug")
	trace, err := a.TraceService.GetRequirementTrace(r.Context(), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, "req_not_found", err.Error())
		return
	}
	writeData(w, http.StatusOK, trace)
}

// getCodeTrace GET /api/v1/traceability/code
func (a *API) getCodeTrace(w http.ResponseWriter, r *http.Request) {
	if a.TraceService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	filePath := r.URL.Query().Get("file")
	if filePath == "" {
		writeError(w, http.StatusBadRequest, "missing_file", "")
		return
	}
	trace, err := a.TraceService.GetCodeTrace(r.Context(), filePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "trace_failed", "")
		return
	}
	writeData(w, http.StatusOK, trace)
}

// getCoverageDashboard GET /api/v1/traceability/coverage
func (a *API) getCoverageDashboard(w http.ResponseWriter, r *http.Request) {
	if a.TraceService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	d, err := a.TraceService.GetCoverageDashboard(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "dashboard_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, d)
}

// getProgressReport GET /api/v1/traceability/progress
func (a *API) getProgressReport(w http.ResponseWriter, r *http.Request) {
	if a.TraceService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	report, err := a.TraceService.GetProgressReport(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "progress_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, report)
}

// getConsolidatedReport GET /api/v1/traceability/consolidated
func (a *API) getConsolidatedReport(w http.ResponseWriter, r *http.Request) {
	if a.TraceService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	report, err := a.TraceService.GetConsolidatedReport(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "consolidated_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, report)
}

// getHUsWithoutProposals GET /api/v1/traceability/gaps/no-proposal
func (a *API) getHUsWithoutProposals(w http.ResponseWriter, r *http.Request) {
	if a.TraceService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	gaps, err := a.TraceService.GetHUsWithoutProposals(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "gaps_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, gaps)
}

// getHUsWithoutDesigns GET /api/v1/traceability/gaps/no-design
func (a *API) getHUsWithoutDesigns(w http.ResponseWriter, r *http.Request) {
	if a.TraceService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	gaps, err := a.TraceService.GetHUsWithoutDesigns(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "gaps_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, gaps)
}

// getHUsWithIncompleteTasks GET /api/v1/traceability/gaps/incomplete-tasks
func (a *API) getHUsWithIncompleteTasks(w http.ResponseWriter, r *http.Request) {
	if a.TraceService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	gaps, err := a.TraceService.GetHUsWithIncompleteTasks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "gaps_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, gaps)
}

// addCodeReference POST /api/v1/traceability/code-refs
func (a *API) addCodeReference(w http.ResponseWriter, r *http.Request) {
	if a.TraceService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	huSlug := r.URL.Query().Get("hu_slug")
	if huSlug == "" {
		writeError(w, http.StatusBadRequest, "missing_hu_slug", "")
		return
	}
	hu, err := a.HUService.GetBySlug(r.Context(), huSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "hu_not_found", "")
		return
	}
	var b addCodeRefBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	repo := b.Repo
	if repo == "" {
		repo = "domain"
	}
	cr, err := a.TraceService.AddCodeReference(r.Context(), hu.ID, b.FilePath, repo, b.Branch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "add_code_ref_failed", err.Error())
		return
	}
	writeData(w, http.StatusCreated, cr)
}

// removeCodeReference DELETE /api/v1/traceability/code-refs/{id}
func (a *API) removeCodeReference(w http.ResponseWriter, r *http.Request) {
	if a.TraceService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	refID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "")
		return
	}
	if err := a.TraceService.RemoveCodeReference(r.Context(), refID); err != nil {
		writeError(w, http.StatusInternalServerError, "remove_code_ref_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

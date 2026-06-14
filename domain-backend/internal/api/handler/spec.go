package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	specsvc "nunezlagos/domain/internal/service/spec"
)

type createProposalBody struct {
	Intention    string `json:"intention"`
	Scope        string `json:"scope"`
	Approach     string `json:"approach"`
	Risks        string `json:"risks,omitempty"`
	TestingNotes string `json:"testing_notes,omitempty"`
}

type changePropStatusBody struct {
	Status          string `json:"status"`
	RejectionReason string `json:"rejection_reason,omitempty"`
}

type createDesignBody struct {
	ProposalID      *string `json:"proposal_id,omitempty"`
	ArchDecisions   string  `json:"arch_decisions"`
	Alternatives    string  `json:"alternatives,omitempty"`
	DataFlow        string  `json:"data_flow,omitempty"`
	TDDPlan         string  `json:"tdd_plan,omitempty"`
	RisksMitigation string  `json:"risks_mitigation,omitempty"`
}

type changeDesignStatusBody struct {
	Status string `json:"status"`
}

// createProposal POST /api/v1/user-stories/{slug}/proposals
func (a *API) createProposal(w http.ResponseWriter, r *http.Request) {
	if a.SpecService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	huSlug := r.PathValue("slug")
	hu, err := a.HUService.GetBySlug(r.Context(), huSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "hu_not_found", "user story not found")
		return
	}
	var b createProposalBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	p, err := a.SpecService.CreateProposal(r.Context(), hu.ID, b.Intention, b.Scope, b.Approach, b.Risks, b.TestingNotes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_proposal_failed", err.Error())
		return
	}
	writeData(w, http.StatusCreated, p)
}

// getLatestProposal GET /api/v1/user-stories/{slug}/proposals/latest
func (a *API) getLatestProposal(w http.ResponseWriter, r *http.Request) {
	if a.SpecService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	huSlug := r.PathValue("slug")
	hu, err := a.HUService.GetBySlug(r.Context(), huSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "hu_not_found", "user story not found")
		return
	}
	p, err := a.SpecService.GetLatestProposal(r.Context(), hu.ID)
	if errors.Is(err, specsvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no proposals found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_proposal_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, p)
}

// listProposalVersions GET /api/v1/user-stories/{slug}/proposals
func (a *API) listProposalVersions(w http.ResponseWriter, r *http.Request) {
	if a.SpecService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	huSlug := r.PathValue("slug")
	hu, err := a.HUService.GetBySlug(r.Context(), huSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "hu_not_found", "user story not found")
		return
	}
	versions, err := a.SpecService.ListProposalVersions(r.Context(), hu.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_proposals_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, versions)
}

// changeProposalStatus PATCH /api/v1/proposals/{id}/status
func (a *API) changeProposalStatus(w http.ResponseWriter, r *http.Request) {
	if a.SpecService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	propID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_proposal_id", "")
		return
	}
	var b changePropStatusBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	p, err := a.SpecService.ChangeProposalStatus(r.Context(), propID, b.Status, b.RejectionReason)
	if err != nil {
		switch {
		case errors.Is(err, specsvc.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, specsvc.ErrInvalidStatus), errors.Is(err, specsvc.ErrInvalidTransition):
			writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "change_status_failed", err.Error())
		}
		return
	}
	writeData(w, http.StatusOK, p)
}

// createDesign POST /api/v1/user-stories/{slug}/designs
func (a *API) createDesign(w http.ResponseWriter, r *http.Request) {
	if a.SpecService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	huSlug := r.PathValue("slug")
	hu, err := a.HUService.GetBySlug(r.Context(), huSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "hu_not_found", "user story not found")
		return
	}
	var b createDesignBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	var propID *uuid.UUID
	if b.ProposalID != nil {
		id, err := uuid.Parse(*b.ProposalID)
		if err == nil {
			propID = &id
		}
	}
	d, err := a.SpecService.CreateDesign(r.Context(), hu.ID, propID, b.ArchDecisions, b.Alternatives, b.DataFlow, b.TDDPlan, b.RisksMitigation)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_design_failed", err.Error())
		return
	}
	writeData(w, http.StatusCreated, d)
}

// getLatestDesign GET /api/v1/user-stories/{slug}/designs/latest
func (a *API) getLatestDesign(w http.ResponseWriter, r *http.Request) {
	if a.SpecService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	huSlug := r.PathValue("slug")
	hu, err := a.HUService.GetBySlug(r.Context(), huSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "hu_not_found", "user story not found")
		return
	}
	d, err := a.SpecService.GetLatestDesign(r.Context(), hu.ID)
	if errors.Is(err, specsvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no designs found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_design_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, d)
}

// listDesigns GET /api/v1/user-stories/{slug}/designs
func (a *API) listDesigns(w http.ResponseWriter, r *http.Request) {
	if a.SpecService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	huSlug := r.PathValue("slug")
	hu, err := a.HUService.GetBySlug(r.Context(), huSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "hu_not_found", "user story not found")
		return
	}
	designs, err := a.SpecService.ListDesignsByHU(r.Context(), hu.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_designs_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, designs)
}

// changeDesignStatus PATCH /api/v1/designs/{id}/status
func (a *API) changeDesignStatus(w http.ResponseWriter, r *http.Request) {
	if a.SpecService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	designID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_design_id", "")
		return
	}
	var b changeDesignStatusBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	d, err := a.SpecService.ChangeDesignStatus(r.Context(), designID, b.Status)
	if err != nil {
		switch {
		case errors.Is(err, specsvc.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, specsvc.ErrInvalidStatus):
			writeError(w, http.StatusUnprocessableEntity, "invalid_status", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "change_status_failed", err.Error())
		}
		return
	}
	writeData(w, http.StatusOK, d)
}

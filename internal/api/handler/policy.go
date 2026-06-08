package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/policy"
)

type createPolicyBody struct {
	Slug           string         `json:"slug"`
	Name           string         `json:"name"`
	Kind           string         `json:"kind"`
	BodyMD         string         `json:"body_md"`
	BodyStructured map[string]any `json:"body_structured,omitempty"`
	SourceFile     string         `json:"source_file,omitempty"`
}

// POST /api/v1/platform/policies — HU-01.8 admin only
func (a *API) createPolicy(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.PolicyService == nil {
		writeError(w, http.StatusServiceUnavailable, "policies_not_configured", "")
		return
	}
	var in createPolicyBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	pol, err := a.PolicyService.Create(r.Context(), policy.CreateInput{
		Slug: in.Slug, Name: in.Name, Kind: in.Kind, BodyMD: in.BodyMD,
		BodyStructured: in.BodyStructured, SourceFile: in.SourceFile,
	})
	if err != nil {
		switch {
		case errors.Is(err, policy.ErrInvalidSlug):
			writeError(w, http.StatusUnprocessableEntity, "invalid_slug", err.Error())
		case errors.Is(err, policy.ErrInvalidKind):
			writeError(w, http.StatusUnprocessableEntity, "invalid_kind", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create", err.Error())
		}
		return
	}
	writeData(w, http.StatusCreated, pol)
}

// GET /api/v1/platform/policies?kind=convention
func (a *API) listPolicies(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	kind := r.URL.Query().Get("kind")
	pols, err := a.PolicyService.List(r.Context(), kind)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, pols)
}

// GET /api/v1/platform/policies/{slug}
func (a *API) getPolicyBySlug(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	slug := r.PathValue("slug")
	pol, err := a.PolicyService.GetBySlug(r.Context(), slug)
	if errors.Is(err, policy.ErrUnknown) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get", err.Error())
		return
	}
	writeData(w, http.StatusOK, pol)
}

type updatePolicyBody struct {
	BodyMD         *string        `json:"body_md,omitempty"`
	BodyStructured map[string]any `json:"body_structured,omitempty"`
}

// PATCH /api/v1/platform/policies/{id}
func (a *API) updatePolicy(w http.ResponseWriter, r *http.Request) {
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
	var in updatePolicyBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	userID, _ := uuid.Parse(p.UserID)
	pol, err := a.PolicyService.Update(r.Context(), id, policy.UpdateInput{
		BodyMD: in.BodyMD, BodyStructured: in.BodyStructured, ChangedBy: &userID,
	})
	if errors.Is(err, policy.ErrUnknown) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update", err.Error())
		return
	}
	writeData(w, http.StatusOK, pol)
}

// DELETE /api/v1/platform/policies/{id}
func (a *API) deletePolicy(w http.ResponseWriter, r *http.Request) {
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
	if err := a.PolicyService.Delete(r.Context(), id); err != nil {
		if errors.Is(err, policy.ErrUnknown) {
			writeError(w, http.StatusNotFound, "not_found", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

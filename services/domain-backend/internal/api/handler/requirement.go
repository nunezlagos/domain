package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	reqsvc "nunezlagos/domain/internal/service/requirement"
)

type createReqBody struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
	Priority    string `json:"priority,omitempty"`
	ParentSlug  string `json:"parent_slug,omitempty"`
}

type updateReqBody struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
	Priority    *string `json:"priority,omitempty"`
}

type archiveReqBody struct {
	Recursive bool `json:"recursive"`
}

// createRequirement POST /api/v1/requirements
func (a *API) createRequirement(w http.ResponseWriter, r *http.Request) {
	if a.ReqService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	var b createReqBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	req, err := a.ReqService.Create(r.Context(), b.Slug, b.Title, b.Description, b.Status, b.Priority, b.ParentSlug)
	if err != nil {
		switch {
		case errors.Is(err, reqsvc.ErrSlugInvalid), errors.Is(err, reqsvc.ErrInvalidStatus), errors.Is(err, reqsvc.ErrInvalidPriority):
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		case errors.Is(err, reqsvc.ErrSlugTaken):
			writeError(w, http.StatusConflict, "slug_taken", err.Error())
		case errors.Is(err, reqsvc.ErrParentNotFound):
			writeError(w, http.StatusUnprocessableEntity, "parent_not_found", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		}
		return
	}
	writeData(w, http.StatusCreated, req)
}

// getRequirement GET /api/v1/requirements/{slug}
func (a *API) getRequirement(w http.ResponseWriter, r *http.Request) {
	if a.ReqService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	slug := r.PathValue("slug")
	req, err := a.ReqService.GetBySlug(r.Context(), slug)
	if errors.Is(err, reqsvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "requirement not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, req)
}

// listRequirements GET /api/v1/requirements
func (a *API) listRequirements(w http.ResponseWriter, r *http.Request) {
	if a.ReqService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	filter := reqsvc.RequirementFilter{
		Status:   r.URL.Query().Get("status"),
		Priority: r.URL.Query().Get("priority"),
	}
	if p := r.URL.Query().Get("parent_id"); p != "" {
		id, err := uuid.Parse(p)
		if err == nil {
			filter.ParentID = &id
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			filter.Limit = n
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			filter.Offset = n
		}
	}
	reqs, err := a.ReqService.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, reqs)
}

// updateRequirement PATCH /api/v1/requirements/{slug}
func (a *API) updateRequirement(w http.ResponseWriter, r *http.Request) {
	if a.ReqService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	slug := r.PathValue("slug")
	var b updateReqBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	req, err := a.ReqService.Update(r.Context(), slug, b.Title, b.Description, b.Status, b.Priority)
	if err != nil {
		switch {
		case errors.Is(err, reqsvc.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, reqsvc.ErrInvalidStatus), errors.Is(err, reqsvc.ErrInvalidPriority):
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		}
		return
	}
	writeData(w, http.StatusOK, req)
}

// archiveRequirement POST /api/v1/requirements/{slug}/archive
func (a *API) archiveRequirement(w http.ResponseWriter, r *http.Request) {
	if a.ReqService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	slug := r.PathValue("slug")
	var b archiveReqBody
	if err := decodeJSON(r, &b); err != nil {
		b.Recursive = false
	}
	err := a.ReqService.Archive(r.Context(), slug, b.Recursive)
	if err != nil {
		if errors.Is(err, reqsvc.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "archive_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getRequirementTree GET /api/v1/requirements/{slug}/tree
func (a *API) getRequirementTree(w http.ResponseWriter, r *http.Request) {
	if a.ReqService == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "")
		return
	}
	slug := r.PathValue("slug")
	tree, err := a.ReqService.GetTree(r.Context(), slug)
	if errors.Is(err, reqsvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "requirement not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "tree_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, tree)
}

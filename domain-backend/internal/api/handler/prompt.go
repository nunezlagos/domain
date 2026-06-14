package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/prompt"
)

type createPromptBody struct {
	ProjectSlug string             `json:"project_slug,omitempty"`
	Slug        string             `json:"slug"`
	Body        string             `json:"body"`
	Variables   []prompt.Variable  `json:"variables,omitempty"`
	Description string             `json:"description,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
	SetActive   bool               `json:"set_active,omitempty"`
}

func (a *API) createPrompt(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	userID, _ := uuid.Parse(p.UserID)
	var b createPromptBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	var projectID *uuid.UUID
	if b.ProjectSlug != "" {
		proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, b.ProjectSlug)
		if err != nil {
			writeError(w, http.StatusNotFound, "project_not_found", "")
			return
		}
		projectID = &proj.ID
	}
	out, err := a.PromptService.Create(r.Context(), prompt.CreateInput{
		OrganizationID: orgID,
		ProjectID:      projectID,
		CreatedBy:      &userID,
		Slug:           b.Slug,
		Body:           b.Body,
		Variables:      b.Variables,
		Description:    b.Description,
		Tags:           b.Tags,
		SetActive:      b.SetActive,
	})
	if err != nil {
		switch {
		case errors.Is(err, prompt.ErrSlugInvalid):
			writeError(w, http.StatusUnprocessableEntity, "invalid_slug", err.Error())
		case errors.Is(err, prompt.ErrBodyRequired):
			writeError(w, http.StatusUnprocessableEntity, "body_required", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create", err.Error())
		}
		return
	}
	w.Header().Set("Location", "/api/v1/prompts/"+out.ID.String())
	writeData(w, http.StatusCreated, out)
}

func (a *API) getPrompt(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	out, err := a.PromptService.GetByID(r.Context(), id)
	if errors.Is(err, prompt.ErrNotFound) || (err == nil && out.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get", err.Error())
		return
	}
	writeData(w, http.StatusOK, out)
}

func (a *API) listPromptVersions(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	slug := r.PathValue("slug")
	var projectID *uuid.UUID
	if ps := r.URL.Query().Get("project_slug"); ps != "" {
		proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, ps)
		if err != nil {
			writeError(w, http.StatusNotFound, "project_not_found", "")
			return
		}
		projectID = &proj.ID
	}
	list, err := a.PromptService.ListVersions(r.Context(), orgID, projectID, slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, list)
}

func (a *API) setActivePrompt(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	actorID, _ := uuid.Parse(p.UserID)
	out, err := a.PromptService.SetActive(r.Context(), id, actorID)
	if errors.Is(err, prompt.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "set_active", err.Error())
		return
	}
	if out.OrganizationID.String() != p.OrganizationID {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	writeData(w, http.StatusOK, out)
}

func (a *API) deletePrompt(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	out, err := a.PromptService.GetByID(r.Context(), id)
	if errors.Is(err, prompt.ErrNotFound) || (err == nil && out.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	actorID, _ := uuid.Parse(p.UserID)
	if err := a.PromptService.SoftDelete(r.Context(), id, actorID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) searchPrompts(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "q requerido")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	results, err := a.PromptService.Search(r.Context(), orgID, q, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search", err.Error())
		return
	}
	writeData(w, http.StatusOK, results)
}

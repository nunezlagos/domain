package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	projsvc "github.com/saargo/domain/internal/service/project"
)

type createProjectBody struct {
	Name          string         `json:"name"`
	Slug          string         `json:"slug"`
	Description   string         `json:"description,omitempty"`
	RepositoryURL string         `json:"repository_url,omitempty"`
	Settings      map[string]any `json:"settings,omitempty"`
}

func (a *API) createProject(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, err := uuid.Parse(p.OrganizationID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_principal", "")
		return
	}
	actorID, _ := uuid.Parse(p.UserID)
	var b createProjectBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if b.Name == "" || b.Slug == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "name y slug requeridos")
		return
	}
	proj, err := a.ProjectService.Create(r.Context(), projsvc.CreateInput{
		OrganizationID: orgID,
		Name:           b.Name,
		Slug:           b.Slug,
		Description:    b.Description,
		RepositoryURL:  b.RepositoryURL,
		Settings:       b.Settings,
		ActorID:        actorID,
	})
	if err != nil {
		switch {
		case errors.Is(err, projsvc.ErrSlugInvalid):
			writeError(w, http.StatusUnprocessableEntity, "invalid_slug", err.Error())
		case errors.Is(err, projsvc.ErrSlugTaken):
			writeError(w, http.StatusConflict, "slug_taken", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create_project", err.Error())
		}
		return
	}
	w.Header().Set("Location", "/api/v1/projects/"+proj.Slug)
	writeData(w, http.StatusCreated, proj)
}

func (a *API) listProjects(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	list, err := a.ProjectService.List(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, list)
}

func (a *API) getProject(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, r.PathValue("slug"))
	if errors.Is(err, projsvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "project not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get", err.Error())
		return
	}
	writeData(w, http.StatusOK, proj)
}

type updateProjectBody struct {
	Name          *string        `json:"name,omitempty"`
	Description   *string        `json:"description,omitempty"`
	RepositoryURL *string        `json:"repository_url,omitempty"`
	Settings      map[string]any `json:"settings,omitempty"`
}

func (a *API) updateProject(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	actorID, _ := uuid.Parse(p.UserID)
	proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, r.PathValue("slug"))
	if errors.Is(err, projsvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	var b updateProjectBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	updated, err := a.ProjectService.Update(r.Context(), proj.ID, projsvc.UpdateInput{
		Name:          b.Name,
		Description:   b.Description,
		RepositoryURL: b.RepositoryURL,
		Settings:      b.Settings,
		ActorID:       actorID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update", err.Error())
		return
	}
	writeData(w, http.StatusOK, updated)
}

func (a *API) deleteProject(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	actorID, _ := uuid.Parse(p.UserID)
	proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, r.PathValue("slug"))
	if errors.Is(err, projsvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	if err := a.ProjectService.SoftDelete(r.Context(), proj.ID, actorID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

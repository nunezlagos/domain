package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/projecttemplate"
)

type createTemplateBody struct {
	Slug          string         `json:"slug"`
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	IsDefault     bool           `json:"is_default,omitempty"`
	IsPublic      bool           `json:"is_public,omitempty"`
	Settings      map[string]any `json:"settings,omitempty"`
	DefaultSkills []string       `json:"default_skills,omitempty"`
	DefaultAgents []string       `json:"default_agents,omitempty"`
	DefaultFlows  []string       `json:"default_flows,omitempty"`
}

// POST /api/v1/project-templates
func (a *API) createProjectTemplate(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.ProjectTemplateService == nil {
		writeError(w, http.StatusServiceUnavailable, "templates_not_configured", "")
		return
	}
	var in createTemplateBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	t, err := a.ProjectTemplateService.Create(r.Context(), orgID,
		projecttemplate.CreateInput{
			Slug: in.Slug, Name: in.Name, Description: in.Description,
			IsDefault: in.IsDefault, IsPublic: in.IsPublic,
			Settings: in.Settings, DefaultSkills: in.DefaultSkills,
			DefaultAgents: in.DefaultAgents, DefaultFlows: in.DefaultFlows,
		})
	if err != nil {
		if errors.Is(err, projecttemplate.ErrInvalidSlug) {
			writeError(w, http.StatusUnprocessableEntity, "invalid_slug", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "create", err.Error())
		return
	}
	w.Header().Set("Location", "/api/v1/project-templates/"+t.ID.String())
	writeData(w, http.StatusCreated, t)
}

// GET /api/v1/project-templates
func (a *API) listProjectTemplates(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	ts, err := a.ProjectTemplateService.ListByOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, ts)
}

// GET /api/v1/project-templates/{id}
func (a *API) getProjectTemplate(w http.ResponseWriter, r *http.Request) {
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
	orgID, _ := uuid.Parse(p.OrganizationID)
	t, err := a.ProjectTemplateService.Get(r.Context(), orgID, id)
	if errors.Is(err, projecttemplate.ErrUnknown) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get", err.Error())
		return
	}
	writeData(w, http.StatusOK, t)
}

// DELETE /api/v1/project-templates/{id}
func (a *API) deleteProjectTemplate(w http.ResponseWriter, r *http.Request) {
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
	orgID, _ := uuid.Parse(p.OrganizationID)
	if err := a.ProjectTemplateService.Delete(r.Context(), orgID, id); err != nil {
		if errors.Is(err, projecttemplate.ErrUnknown) {
			writeError(w, http.StatusNotFound, "not_found", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

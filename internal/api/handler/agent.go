package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/saargo/domain/internal/service/agent"
)

type createAgentBody struct {
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Provider      string   `json:"provider"`
	Model         string   `json:"model"`
	SystemPrompt  string   `json:"system_prompt,omitempty"`
	SkillsSlugs   []string `json:"skills_slugs,omitempty"`
	MaxIterations int      `json:"max_iterations,omitempty"`
	TokenBudget   *int64   `json:"token_budget,omitempty"`
	Temperature   *float64 `json:"temperature,omitempty"`
}

func (a *API) createAgent(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	actorID, _ := uuid.Parse(p.UserID)
	var b createAgentBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	out, err := a.AgentService.Create(r.Context(), agent.CreateInput{
		OrganizationID: orgID,
		Slug:           b.Slug, Name: b.Name, Description: b.Description,
		Provider: b.Provider, Model: b.Model, SystemPrompt: b.SystemPrompt,
		SkillsSlugs: b.SkillsSlugs, MaxIterations: b.MaxIterations,
		TokenBudget: b.TokenBudget, Temperature: b.Temperature,
		ActorID: actorID,
	})
	if err != nil {
		switch {
		case errors.Is(err, agent.ErrSlugInvalid),
			errors.Is(err, agent.ErrNameRequired),
			errors.Is(err, agent.ErrModelRequired),
			errors.Is(err, agent.ErrProviderInvalid):
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		case errors.Is(err, agent.ErrSkillNotFound):
			writeError(w, http.StatusUnprocessableEntity, "skill_not_found", err.Error())
		case errors.Is(err, agent.ErrSlugTaken):
			writeError(w, http.StatusConflict, "slug_taken", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create", err.Error())
		}
		return
	}
	w.Header().Set("Location", "/api/v1/agents/"+out.ID.String())
	writeData(w, http.StatusCreated, out)
}

func (a *API) getAgent(w http.ResponseWriter, r *http.Request) {
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
	out, err := a.AgentService.GetByID(r.Context(), id)
	if errors.Is(err, agent.ErrNotFound) || (err == nil && out.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get", err.Error())
		return
	}
	writeData(w, http.StatusOK, out)
}

func (a *API) listAgents(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := a.AgentService.List(r.Context(), orgID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, list)
}

type updateAgentBody struct {
	Name          *string  `json:"name,omitempty"`
	Description   *string  `json:"description,omitempty"`
	Provider      *string  `json:"provider,omitempty"`
	Model         *string  `json:"model,omitempty"`
	SystemPrompt  *string  `json:"system_prompt,omitempty"`
	SkillsSlugs   []string `json:"skills_slugs,omitempty"`
	MaxIterations *int     `json:"max_iterations,omitempty"`
	TokenBudget   *int64   `json:"token_budget,omitempty"`
	Temperature   *float64 `json:"temperature,omitempty"`
}

func (a *API) updateAgent(w http.ResponseWriter, r *http.Request) {
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
	prev, err := a.AgentService.GetByID(r.Context(), id)
	if errors.Is(err, agent.ErrNotFound) || (err == nil && prev.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	actorID, _ := uuid.Parse(p.UserID)
	var b updateAgentBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	out, err := a.AgentService.Update(r.Context(), id, agent.UpdateInput{
		Name: b.Name, Description: b.Description, Provider: b.Provider,
		Model: b.Model, SystemPrompt: b.SystemPrompt, SkillsSlugs: b.SkillsSlugs,
		MaxIterations: b.MaxIterations, TokenBudget: b.TokenBudget,
		Temperature: b.Temperature, ActorID: actorID,
	})
	if err != nil {
		switch {
		case errors.Is(err, agent.ErrSkillNotFound):
			writeError(w, http.StatusUnprocessableEntity, "skill_not_found", err.Error())
		case errors.Is(err, agent.ErrProviderInvalid):
			writeError(w, http.StatusUnprocessableEntity, "invalid_provider", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "update", err.Error())
		}
		return
	}
	writeData(w, http.StatusOK, out)
}

func (a *API) deleteAgent(w http.ResponseWriter, r *http.Request) {
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
	prev, err := a.AgentService.GetByID(r.Context(), id)
	if errors.Is(err, agent.ErrNotFound) || (err == nil && prev.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	actorID, _ := uuid.Parse(p.UserID)
	if err := a.AgentService.SoftDelete(r.Context(), id, actorID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

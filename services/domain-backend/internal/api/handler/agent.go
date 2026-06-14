package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/agent"
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
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	actorID := a.userID(ctx)
	var b createAgentBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	out, err := a.AgentService.Create(ctx, agent.CreateInput{
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
	ctx := r.Context()
	if a.orgID(ctx) == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	out, err := a.AgentService.GetByID(ctx, id)
	if errors.Is(err, agent.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get", err.Error())
		return
	}
	if err := a.authorizeOrg(ctx, out.OrganizationID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	writeData(w, http.StatusOK, out)
}

func (a *API) listAgents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := a.AgentService.List(ctx, orgID, limit)
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
	ctx := r.Context()
	if a.orgID(ctx) == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	prev, err := a.AgentService.GetByID(ctx, id)
	if errors.Is(err, agent.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	if err := a.authorizeOrg(ctx, prev.OrganizationID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	actorID := a.userID(ctx)
	var b updateAgentBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	out, err := a.AgentService.Update(ctx, id, agent.UpdateInput{
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

func (a *API) listAgentVersions(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	ctx := r.Context()
	if a.orgID(ctx) == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	ag, err := a.AgentService.GetByID(ctx, id)
	if errors.Is(err, agent.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	if err := a.authorizeOrg(ctx, ag.OrganizationID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	versions, err := a.AgentService.GetVersions(ctx, id, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "versions", err.Error())
		return
	}
	if versions == nil {
		versions = []agent.AgentVersion{}
	}
	writeData(w, http.StatusOK, versions)
}

func (a *API) deleteAgent(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	ctx := r.Context()
	if a.orgID(ctx) == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	prev, err := a.AgentService.GetByID(ctx, id)
	if errors.Is(err, agent.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	if err := a.authorizeOrg(ctx, prev.OrganizationID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	actorID := a.userID(ctx)
	if err := a.AgentService.SoftDelete(ctx, id, actorID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

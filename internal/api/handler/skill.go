package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/skill"
)

type createSkillBody struct {
	Slug           string         `json:"slug"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	SkillType      string         `json:"type"`
	Content        string         `json:"content"`
	InputSchema    map[string]any `json:"input_schema,omitempty"`
	OutputSchema   map[string]any `json:"output_schema,omitempty"`
	TimeoutSeconds int            `json:"timeout_seconds,omitempty"`
	Idempotent     bool           `json:"idempotent,omitempty"`
	HasSideEffects bool           `json:"has_side_effects,omitempty"`
	DependsOn      []string       `json:"depends_on,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
}

func (a *API) createSkill(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	actorID, _ := uuid.Parse(p.UserID)
	var b createSkillBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	sk, err := a.SkillService.Create(r.Context(), skill.CreateInput{
		OrganizationID: orgID,
		Slug:           b.Slug,
		Name:           b.Name,
		Description:    b.Description,
		SkillType:      b.SkillType,
		Content:        b.Content,
		InputSchema:    b.InputSchema,
		OutputSchema:   b.OutputSchema,
		TimeoutSeconds: b.TimeoutSeconds,
		Idempotent:     b.Idempotent,
		HasSideEffects: b.HasSideEffects,
		DependsOn:      b.DependsOn,
		Tags:           b.Tags,
		ActorID:        actorID,
	})
	if err != nil {
		switch {
		case errors.Is(err, skill.ErrSlugInvalid),
			errors.Is(err, skill.ErrNameRequired),
			errors.Is(err, skill.ErrContentRequired),
			errors.Is(err, skill.ErrInvalidType):
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		case errors.Is(err, skill.ErrInvalidSchema):
			writeError(w, http.StatusUnprocessableEntity, "invalid_schema", err.Error())
		case errors.Is(err, skill.ErrSlugTaken):
			writeError(w, http.StatusConflict, "slug_taken", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create", err.Error())
		}
		return
	}
	w.Header().Set("Location", "/api/v1/skills/"+sk.ID.String())
	writeData(w, http.StatusCreated, sk)
}

func (a *API) getSkill(w http.ResponseWriter, r *http.Request) {
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
	sk, err := a.SkillService.GetByID(r.Context(), id)
	if errors.Is(err, skill.ErrNotFound) || (err == nil && sk.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get", err.Error())
		return
	}
	writeData(w, http.StatusOK, sk)
}

func (a *API) listSkills(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := a.SkillService.List(r.Context(), orgID, skill.ListFilter{
		SkillType: r.URL.Query().Get("type"),
		Tag:       r.URL.Query().Get("tag"),
		Limit:     limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, out)
}

func (a *API) searchSkills(w http.ResponseWriter, r *http.Request) {
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
	results, err := a.SkillService.SearchHybrid(r.Context(), orgID, q, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search", err.Error())
		return
	}
	writeData(w, http.StatusOK, results)
}

type updateSkillBody struct {
	Name           *string        `json:"name,omitempty"`
	Description    *string        `json:"description,omitempty"`
	Content        *string        `json:"content,omitempty"`
	InputSchema    map[string]any `json:"input_schema,omitempty"`
	OutputSchema   map[string]any `json:"output_schema,omitempty"`
	TimeoutSeconds *int           `json:"timeout_seconds,omitempty"`
	Idempotent     *bool          `json:"idempotent,omitempty"`
	HasSideEffects *bool          `json:"has_side_effects,omitempty"`
	DependsOn      []string       `json:"depends_on,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
}

func (a *API) updateSkill(w http.ResponseWriter, r *http.Request) {
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
	prev, err := a.SkillService.GetByID(r.Context(), id)
	if errors.Is(err, skill.ErrNotFound) || (err == nil && prev.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	actorID, _ := uuid.Parse(p.UserID)
	var b updateSkillBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	out, err := a.SkillService.Update(r.Context(), id, skill.UpdateInput{
		Name:           b.Name,
		Description:    b.Description,
		Content:        b.Content,
		InputSchema:    b.InputSchema,
		OutputSchema:   b.OutputSchema,
		TimeoutSeconds: b.TimeoutSeconds,
		Idempotent:     b.Idempotent,
		HasSideEffects: b.HasSideEffects,
		DependsOn:      b.DependsOn,
		Tags:           b.Tags,
		ActorID:        actorID,
	})
	if err != nil {
		if errors.Is(err, skill.ErrInvalidSchema) {
			writeError(w, http.StatusUnprocessableEntity, "invalid_schema", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "update", err.Error())
		return
	}
	writeData(w, http.StatusOK, out)
}

func (a *API) deleteSkill(w http.ResponseWriter, r *http.Request) {
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
	sk, err := a.SkillService.GetByID(r.Context(), id)
	if errors.Is(err, skill.ErrNotFound) || (err == nil && sk.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	actorID, _ := uuid.Parse(p.UserID)
	if err := a.SkillService.SoftDelete(r.Context(), id, actorID); err != nil {
		if errors.Is(err, skill.ErrHasDependencies) {
			writeError(w, http.StatusConflict, "has_dependencies", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/saargo/domain/internal/service/knowledge"
)

type saveKnowledgeBody struct {
	ProjectSlug string         `json:"project_slug"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	Source      string         `json:"source,omitempty"`
	SourceURL   string         `json:"source_url,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

func (a *API) saveKnowledge(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	userID, _ := uuid.Parse(p.UserID)
	var b saveKnowledgeBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if b.ProjectSlug == "" || b.Title == "" || b.Body == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed",
			"project_slug, title y body requeridos")
		return
	}
	proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, b.ProjectSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "project_not_found", "")
		return
	}
	doc, chunks, err := a.KnowledgeService.Save(r.Context(), knowledge.SaveInput{
		OrganizationID: orgID,
		ProjectID:      proj.ID,
		CreatedBy:      &userID,
		Title:          b.Title,
		Body:           b.Body,
		Source:         b.Source,
		SourceURL:      b.SourceURL,
		Tags:           b.Tags,
		Metadata:       b.Metadata,
	})
	if err != nil {
		switch {
		case errors.Is(err, knowledge.ErrTitleRequired),
			errors.Is(err, knowledge.ErrBodyRequired):
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "save", err.Error())
		}
		return
	}
	w.Header().Set("Location", "/api/v1/knowledge/"+doc.ID.String())
	writeData(w, http.StatusCreated, map[string]any{
		"document":     doc,
		"chunks_count": len(chunks),
	})
}

func (a *API) getKnowledge(w http.ResponseWriter, r *http.Request) {
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
	doc, chunks, err := a.KnowledgeService.Get(r.Context(), id)
	if errors.Is(err, knowledge.ErrNotFound) || (err == nil && doc.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"document": doc,
		"chunks":   chunks,
	})
}

func (a *API) listKnowledge(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	slug := r.URL.Query().Get("project_slug")
	if slug == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "project_slug requerido")
		return
	}
	proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, slug)
	if err != nil {
		writeError(w, http.StatusNotFound, "project_not_found", "")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	docs, err := a.KnowledgeService.ListByProject(r.Context(), proj.ID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, docs)
}

func (a *API) searchKnowledge(w http.ResponseWriter, r *http.Request) {
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
	results, err := a.KnowledgeService.SearchHybrid(r.Context(), orgID, q, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search", err.Error())
		return
	}
	writeData(w, http.StatusOK, results)
}

func (a *API) deleteKnowledge(w http.ResponseWriter, r *http.Request) {
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
	doc, _, err := a.KnowledgeService.Get(r.Context(), id)
	if errors.Is(err, knowledge.ErrNotFound) || (err == nil && doc.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	actorID, _ := uuid.Parse(p.UserID)
	if err := a.KnowledgeService.SoftDelete(r.Context(), id, actorID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

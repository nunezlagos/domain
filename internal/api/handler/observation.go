package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/saargo/domain/internal/service/observation"
	searchsvc "github.com/saargo/domain/internal/service/search"
)

type saveObsBody struct {
	ProjectSlug     string         `json:"project_slug"`
	Content         string         `json:"content"`
	ObservationType string         `json:"observation_type,omitempty"`
	Tags            []string       `json:"tags,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	SessionID       string         `json:"session_id,omitempty"`
}

func (a *API) saveObservation(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	userID, _ := uuid.Parse(p.UserID)
	var b saveObsBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if b.ProjectSlug == "" || b.Content == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "project_slug y content requeridos")
		return
	}
	proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, b.ProjectSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "project_not_found", "")
		return
	}
	var sessionID *uuid.UUID
	if b.SessionID != "" {
		s, err := uuid.Parse(b.SessionID)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_session_id", err.Error())
			return
		}
		sessionID = &s
	}
	o, err := a.ObsService.Save(r.Context(), observation.SaveInput{
		OrganizationID:  orgID,
		ProjectID:       proj.ID,
		CreatedBy:       &userID,
		SessionID:       sessionID,
		Content:         b.Content,
		ObservationType: b.ObservationType,
		Tags:            b.Tags,
		Metadata:        b.Metadata,
	})
	if err != nil {
		if errors.Is(err, observation.ErrContentRequired) {
			writeError(w, http.StatusUnprocessableEntity, "content_required", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "save", err.Error())
		return
	}
	w.Header().Set("Location", "/api/v1/observations/"+o.ID.String())
	writeData(w, http.StatusCreated, o)
}

func (a *API) getObservation(w http.ResponseWriter, r *http.Request) {
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
	o, err := a.ObsService.Get(r.Context(), id)
	if errors.Is(err, observation.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get", err.Error())
		return
	}
	// Cross-org leak guard
	if o.OrganizationID.String() != p.OrganizationID {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	writeData(w, http.StatusOK, o)
}

func (a *API) deleteObservation(w http.ResponseWriter, r *http.Request) {
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
	o, err := a.ObsService.Get(r.Context(), id)
	if errors.Is(err, observation.ErrNotFound) || (err == nil && o.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	actorID, _ := uuid.Parse(p.UserID)
	if err := a.ObsService.SoftDelete(r.Context(), id, actorID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listObservations(w http.ResponseWriter, r *http.Request) {
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
	list, err := a.ObsService.List(r.Context(), proj.ID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, list)
}

// GET /api/v1/search — búsqueda global cross-entity (HU-03.7).
// Parámetros: q, limit, entity_type (csv: observation,prompt,session),
// project_slug (csv), tags (csv), date_from, date_to (ISO 8601).
func (a *API) searchObservations(w http.ResponseWriter, r *http.Request) {
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

	filter := searchsvc.Filter{}
	if et := r.URL.Query().Get("entity_type"); et != "" {
		for _, t := range strings.Split(et, ",") {
			filter.EntityTypes = append(filter.EntityTypes, searchsvc.EntityType(strings.TrimSpace(t)))
		}
	}
	if slugs := r.URL.Query().Get("project_slug"); slugs != "" {
		for _, slug := range strings.Split(slugs, ",") {
			proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, strings.TrimSpace(slug))
			if err != nil {
				continue // slug inválido: ignorar
			}
			filter.ProjectIDs = append(filter.ProjectIDs, proj.ID)
		}
	}
	if tags := r.URL.Query().Get("tags"); tags != "" {
		for _, t := range strings.Split(tags, ",") {
			filter.Tags = append(filter.Tags, strings.TrimSpace(t))
		}
	}
	if df := r.URL.Query().Get("date_from"); df != "" {
		if t, err := time.Parse(time.RFC3339, df); err == nil {
			filter.DateFrom = &t
		}
	}
	if dt := r.URL.Query().Get("date_to"); dt != "" {
		if t, err := time.Parse(time.RFC3339, dt); err == nil {
			filter.DateTo = &t
		}
	}

	results, err := a.SearchService.Search(r.Context(), orgID, q, limit, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search", err.Error())
		return
	}
	writeData(w, http.StatusOK, results)
}

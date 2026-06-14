package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/api/cursor"
	"nunezlagos/domain/internal/api/etag"
	"nunezlagos/domain/internal/service/observation"
	searchsvc "nunezlagos/domain/internal/service/search"
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
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	userID := a.userID(ctx)
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
	ctx := r.Context()
	if a.orgID(ctx) == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	o, err := a.ObsService.Get(ctx, id)
	if errors.Is(err, observation.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get", err.Error())
		return
	}
	if err := a.authorizeOrg(ctx, o.OrganizationID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	tag := etag.SetHeaders(w, o.ID.String(), o.UpdatedAt, "private, max-age=30")
	if etag.IsNotModified(r, tag, o.UpdatedAt) {
		w.WriteHeader(http.StatusNotModified)
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
	ctx := r.Context()
	if a.orgID(ctx) == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	o, err := a.ObsService.Get(ctx, id)
	if errors.Is(err, observation.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	if err := a.authorizeOrg(ctx, o.OrganizationID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	actorID := a.userID(ctx)
	if err := a.ObsService.SoftDelete(ctx, id, actorID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listObservations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
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

	// issue-13.6 cursor pagination
	sortDir, err := cursor.NormalizeSort(r.URL.Query().Get("sort"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_sort", "sort must be asc|desc")
		return
	}
	filtersHash := cursor.HashFilters(map[string]string{
		"project_slug": slug,
		"org":          orgID.String(),
	})
	in := observation.ListPageInput{
		ProjectID: proj.ID,
		Limit:     limit,
		SortDesc:  sortDir == "desc",
	}
	if rawCursor := r.URL.Query().Get("cursor"); rawCursor != "" {
		c, err := cursor.Decode(rawCursor, filtersHash, sortDir)
		if err != nil {
			switch {
			case errors.Is(err, cursor.ErrFiltersMismatch):
				writeError(w, http.StatusBadRequest, "cursor_filters_mismatch",
					"el cursor no aplica a los filtros actuales")
			case errors.Is(err, cursor.ErrSortMismatch):
				writeError(w, http.StatusBadRequest, "cursor_sort_mismatch",
					"el cursor no aplica al sort actual")
			default:
				writeError(w, http.StatusBadRequest, "invalid_cursor", "")
			}
			return
		}
		t := c.LastSortValue
		id, _ := uuid.Parse(c.LastID)
		in.CursorTime = &t
		in.CursorID = &id
	}
	// Legacy offset deprecated (issue-13.6 escenario 6)
	if offStr := r.URL.Query().Get("offset"); offStr != "" {
		w.Header().Set("Deprecation", "true")
		w.Header().Set("Sunset", "2026-12-31")
		off, _ := strconv.Atoi(offStr)
		if off > cursor.MaxLegacyOffset {
			writeError(w, http.StatusBadRequest, "offset_too_large",
				"offset legacy capado a 10000; usar cursor")
			return
		}
	}

	list, hasMore, err := a.ObsService.ListPaginated(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}

	pageMeta := cursor.PageMeta{HasMore: hasMore, Limit: len(list)}
	if hasMore && len(list) > 0 {
		last := list[len(list)-1]
		next := cursor.Cursor{
			LastID:        last.ID.String(),
			LastSortValue: last.CreatedAt,
			FiltersHash:   filtersHash,
			SortDir:       sortDir,
		}
		pageMeta.NextCursor = next.Encode()
	}
	writeDataWithMeta(w, http.StatusOK, list, map[string]any{"pagination": pageMeta})
}

// GET /api/v1/search — búsqueda global cross-entity (issue-03.7).
// Parámetros: q, limit, entity_type (csv: observation,prompt,session),
// project_slug (csv), tags (csv), date_from, date_to (ISO 8601).
func (a *API) searchObservations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
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

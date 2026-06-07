package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/session"
)

type startSessionBody struct {
	Title       string   `json:"title"`
	ProjectSlug string   `json:"project_slug,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func (a *API) startSession(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	userID, _ := uuid.Parse(p.UserID)
	var b startSessionBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if b.Title == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "title requerido")
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
	sess, err := a.SessionService.Start(r.Context(), session.StartInput{
		OrganizationID: orgID,
		UserID:         userID,
		ProjectID:      projectID,
		Title:          b.Title,
		Tags:           b.Tags,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "start", err.Error())
		return
	}
	w.Header().Set("Location", "/api/v1/sessions/"+sess.ID.String())
	writeData(w, http.StatusCreated, sess)
}

type endSessionBody struct {
	Summary string `json:"summary,omitempty"`
}

func (a *API) endSession(w http.ResponseWriter, r *http.Request) {
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
	var b endSessionBody
	_ = decodeJSON(r, &b)
	sess, err := a.SessionService.End(r.Context(), id, actorID, b.Summary)
	if err != nil {
		switch {
		case errors.Is(err, session.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "")
		case errors.Is(err, session.ErrAlreadyEnded):
			writeError(w, http.StatusConflict, "already_ended", "")
		default:
			writeError(w, http.StatusInternalServerError, "end", err.Error())
		}
		return
	}
	// Cross-org guard
	if sess.OrganizationID.String() != p.OrganizationID {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	writeData(w, http.StatusOK, sess)
}

func (a *API) getSession(w http.ResponseWriter, r *http.Request) {
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
	sess, err := a.SessionService.GetByID(r.Context(), id)
	if errors.Is(err, session.ErrNotFound) || (err == nil && sess.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get", err.Error())
		return
	}
	writeData(w, http.StatusOK, sess)
}

func (a *API) listSessions(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	userID, _ := uuid.Parse(p.UserID)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := a.SessionService.List(r.Context(), userID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, list)
}

func (a *API) activeSession(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	userID, _ := uuid.Parse(p.UserID)
	orgID, _ := uuid.Parse(p.OrganizationID)

	var projectID uuid.UUID
	if slug := r.URL.Query().Get("project_slug"); slug != "" {
		proj, err := a.ProjectService.GetBySlug(r.Context(), orgID, slug)
		if err != nil {
			writeError(w, http.StatusNotFound, "project_not_found", "")
			return
		}
		projectID = proj.ID
	}
	sess, err := a.SessionService.GetActive(r.Context(), userID, projectID)
	if errors.Is(err, session.ErrNotFound) {
		writeData(w, http.StatusOK, nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "active", err.Error())
		return
	}
	writeData(w, http.StatusOK, sess)
}

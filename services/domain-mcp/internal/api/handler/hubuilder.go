package handler

import (
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/service/issuebuilder"
)

func (a *API) startHubDraft(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode        string `json:"mode"`
		InitialIdea string `json:"initial_idea"`
		ProjectID   string `json:"project_id,omitempty"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, 400, "invalid_json", err.Error())
		return
	}
	p, ok := apikey.FromContext(r.Context())
	if !ok {
		writeError(w, 401, "unauthorized", "auth required")
		return
	}
	userID, _ := uuid.Parse(p.UserID)

	var projectID *uuid.UUID
	if body.ProjectID != "" {
		pp, perr := uuid.Parse(body.ProjectID)
		if perr != nil {
			writeError(w, 400, "invalid_project_id", perr.Error())
			return
		}
		projectID = &pp
	}

	draft, question, err := a.Hubuilder.Start(r.Context(), body.Mode, body.InitialIdea, &userID, projectID)
	if err != nil {
		code, status := "internal_error", 500
		switch {
		case isErr(err, issuebuilder.ErrInvalidMode):
			code, status = "invalid_mode", 422
		case isErr(err, issuebuilder.ErrUnsupportedMode):
			code, status = "unsupported_mode", 422
		}
		writeError(w, status, code, err.Error())
		return
	}
	writeData(w, 201, map[string]any{
		"draft":    draft,
		"question": question,
	})
}

func (a *API) answerHubDraft(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid_id", "UUID required")
		return
	}
	var body struct {
		Answer string `json:"answer"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, 400, "invalid_json", err.Error())
		return
	}
	draft, question, err := a.Hubuilder.Answer(r.Context(), id, body.Answer)
	if err != nil {
		code, status := "internal_error", 500
		switch {
		case isErr(err, issuebuilder.ErrNotFound):
			code, status = "not_found", 404
		case isErr(err, issuebuilder.ErrInvalidAnswer):
			code, status = "invalid_answer", 422
		case isErr(err, issuebuilder.ErrExpired):
			code, status = "expired", 410
		}
		writeError(w, status, code, err.Error())
		return
	}
	writeData(w, 200, map[string]any{
		"draft":    draft,
		"question": question,
	})
}

func (a *API) previewHubDraft(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid_id", "UUID required")
		return
	}
	preview, err := a.Hubuilder.BuildPreview(r.Context(), id)
	if err != nil {
		code, status := "internal_error", 500
		if isErr(err, issuebuilder.ErrNotFound) {
			code, status = "not_found", 404
		}
		writeError(w, status, code, err.Error())
		return
	}
	writeData(w, 200, preview)
}

func (a *API) commitHubDraft(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid_id", "UUID required")
		return
	}
	draft, err := a.Hubuilder.Commit(r.Context(), id)
	if err != nil {
		code, status := "internal_error", 500
		switch {
		case isErr(err, issuebuilder.ErrNotFound):
			code, status = "not_found", 404
		case isErr(err, issuebuilder.ErrInvalidStatus):
			code, status = "invalid_status", 422
		}
		writeError(w, status, code, err.Error())
		return
	}
	writeData(w, 200, draft)
}

func (a *API) abandonHubDraft(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid_id", "UUID required")
		return
	}
	if err := a.Hubuilder.Abandon(r.Context(), id, "abandoned via HTTP"); err != nil {
		code, status := "internal_error", 500
		if isErr(err, issuebuilder.ErrNotFound) {
			code, status = "not_found", 404
		}
		writeError(w, status, code, err.Error())
		return
	}
	writeData(w, 200, map[string]string{"status": "abandoned"})
}

func (a *API) listHubDrafts(w http.ResponseWriter, r *http.Request) {
	drafts, err := a.Hubuilder.List(r.Context(), "")
	if err != nil {
		writeError(w, 500, "internal_error", err.Error())
		return
	}
	writeData(w, 200, drafts)
}

func isErr(err, target error) bool {
	return err != nil && target != nil && err.Error() == target.Error()
}

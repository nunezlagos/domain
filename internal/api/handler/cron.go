// Handlers HTTP de crons — issue-10.1.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	cronsvc "nunezlagos/domain/internal/service/cron"
)

type createCronRequest struct {
	Slug           string         `json:"slug"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	CronExpression string         `json:"cron_expression"`
	Timezone       string         `json:"timezone"`
	TargetType     string         `json:"target_type"`
	TargetID       uuid.UUID      `json:"target_id"`
	Inputs         map[string]any `json:"inputs"`
	Enabled        *bool          `json:"enabled"`
}

func (a *API) createCron(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	var req createCronRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	actorID, _ := uuid.Parse(p.UserID)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	out, err := a.CronService.Create(r.Context(), cronsvc.CreateInput{
		OrganizationID: orgID, CreatedBy: &actorID,
		Slug: req.Slug, Name: req.Name, Description: req.Description,
		CronExpression: req.CronExpression, Timezone: req.Timezone,
		TargetType: req.TargetType, TargetID: req.TargetID,
		Inputs: req.Inputs, Enabled: enabled, ActorID: actorID,
	})
	switch {
	case errors.Is(err, cronsvc.ErrSlugTaken):
		writeError(w, http.StatusConflict, "slug_taken", err.Error())
		return
	case errors.Is(err, cronsvc.ErrSlugInvalid),
		errors.Is(err, cronsvc.ErrInvalidCronExpr),
		errors.Is(err, cronsvc.ErrInvalidTargetType),
		errors.Is(err, cronsvc.ErrInvalidTimezone):
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "create", err.Error())
		return
	}
	w.Header().Set("Location", "/api/v1/crons/"+out.ID.String())
	writeData(w, http.StatusCreated, out)
}

func (a *API) listCrons(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	out, err := a.CronService.List(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	if out == nil {
		out = []cronsvc.Cron{}
	}
	writeData(w, http.StatusOK, out)
}

// lookupCron resuelve el cron del path validando ownership de la org.
func (a *API) lookupCron(w http.ResponseWriter, r *http.Request) *cronsvc.Cron {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return nil
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return nil
	}
	c, err := a.CronService.GetByID(r.Context(), id)
	if errors.Is(err, cronsvc.ErrNotFound) || (err == nil && c.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return nil
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return nil
	}
	return c
}

func (a *API) getCron(w http.ResponseWriter, r *http.Request) {
	if c := a.lookupCron(w, r); c != nil {
		writeData(w, http.StatusOK, c)
	}
}

func (a *API) patchCron(w http.ResponseWriter, r *http.Request) {
	c := a.lookupCron(w, r)
	if c == nil {
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if req.Enabled == nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "enabled required")
		return
	}
	if err := a.CronService.SetEnabled(r.Context(), c.ID, *req.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "update", err.Error())
		return
	}
	c.Enabled = *req.Enabled
	writeData(w, http.StatusOK, c)
}

func (a *API) deleteCron(w http.ResponseWriter, r *http.Request) {
	c := a.lookupCron(w, r)
	if c == nil {
		return
	}
	p, _ := principal(r)
	actorID, _ := uuid.Parse(p.UserID)
	if err := a.CronService.SoftDelete(r.Context(), c.ID, actorID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) cronHistory(w http.ResponseWriter, r *http.Request) {
	c := a.lookupCron(w, r)
	if c == nil {
		return
	}
	out, err := a.CronService.History(r.Context(), c.ID, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "history", err.Error())
		return
	}
	if out == nil {
		out = []cronsvc.Execution{}
	}
	writeData(w, http.StatusOK, out)
}

package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/lifecycle"
)

type restoreBody struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
}

// POST /api/v1/restore — revierte soft-delete de una entidad dentro de retention.
func (a *API) restoreEntity(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	var b restoreBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	id, err := uuid.Parse(b.EntityID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_entity_id", err.Error())
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	actorID, _ := uuid.Parse(p.UserID)
	err = a.LifecycleService.Restore(r.Context(), b.EntityType, id, actorID, &orgID)
	switch {
	case errors.Is(err, lifecycle.ErrEntityNotSupported):
		writeError(w, http.StatusUnprocessableEntity, "entity_not_supported", err.Error())
	case errors.Is(err, lifecycle.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "")
	case errors.Is(err, lifecycle.ErrRetentionExpired):
		writeError(w, http.StatusGone, "retention_expired", err.Error())
	case err != nil:
		writeError(w, http.StatusInternalServerError, "restore", err.Error())
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// GET /api/v1/me/export — GDPR Art. 15+20 data portability.
func (a *API) exportMyData(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	userID, _ := uuid.Parse(p.UserID)
	orgID, _ := uuid.Parse(p.OrganizationID)
	exp, err := a.LifecycleService.ExportUserData(r.Context(), userID, orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export", err.Error())
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="domain-user-export.json"`)
	writeData(w, http.StatusOK, exp)
}

type eraseMyDataBody struct {
	Confirmation string `json:"confirmation"`
	Reason       string `json:"reason,omitempty"`
}

// POST /api/v1/me/erase — issue-23.4 GDPR Art. 17 right to erasure.
// Irreversible. Requiere confirmation="DELETE_ME" en body.
func (a *API) eraseMyData(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	var b eraseMyDataBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if b.Confirmation != "DELETE_ME" {
		writeError(w, http.StatusUnprocessableEntity, "confirmation_required",
			"Send {confirmation: 'DELETE_ME'} to confirm irreversible erase")
		return
	}
	userID, _ := uuid.Parse(p.UserID)
	res, err := a.LifecycleService.EraseUser(r.Context(), userID, userID, b.Reason)
	if err != nil {
		switch {
		case errors.Is(err, lifecycle.ErrAlreadyErased):
			writeError(w, http.StatusConflict, "already_erased", "")
		case errors.Is(err, lifecycle.ErrTransferOwnershipFirst):
			writeError(w, http.StatusConflict, "transfer_ownership_first",
				"Transfer ownership of org(s) before erasing your account")
		case errors.Is(err, lifecycle.ErrUserNotFound):
			writeError(w, http.StatusNotFound, "not_found", "")
		default:
			writeError(w, http.StatusInternalServerError, "erase", err.Error())
		}
		return
	}
	writeData(w, http.StatusOK, res)
}

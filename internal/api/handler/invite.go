package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/saargo/domain/internal/service/invite"
)

type createInviteBody struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (a *API) createInvite(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(r.PathValue("id"))
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
	var b createInviteBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	inv, err := a.InviteService.Create(r.Context(), orgID, actorID, b.Email, b.Role)
	if err != nil {
		switch {
		case errors.Is(err, invite.ErrInvalidRole):
			writeError(w, http.StatusUnprocessableEntity, "invalid_role", err.Error())
		case errors.Is(err, invite.ErrInvalidIdentifier):
			writeError(w, http.StatusUnprocessableEntity, "invalid_email", err.Error())
		case errors.Is(err, invite.ErrAlreadyPending):
			writeError(w, http.StatusConflict, "already_pending", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create_invite", err.Error())
		}
		return
	}
	writeData(w, http.StatusCreated, inv)
}

func (a *API) listInvites(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	list, err := a.InviteService.ListByOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, list)
}

type acceptInviteBody struct {
	Name string `json:"name,omitempty"`
}

func (a *API) acceptInvite(w http.ResponseWriter, r *http.Request) {
	token, err := uuid.Parse(r.PathValue("token"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	// Email del accepter viene del principal autenticado (via OTP previo)
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	var email string
	if err := a.OrgService.Pool.QueryRow(r.Context(),
		`SELECT email FROM users WHERE id = $1`, p.UserID).Scan(&email); err != nil {
		writeError(w, http.StatusInternalServerError, "lookup_user", err.Error())
		return
	}
	var b acceptInviteBody
	_ = decodeJSON(r, &b)
	userID, orgID, role, err := a.InviteService.Accept(r.Context(), token, email, b.Name)
	if err != nil {
		switch {
		case errors.Is(err, invite.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "")
		case errors.Is(err, invite.ErrExpired):
			writeError(w, http.StatusGone, "expired", "invitación expiró")
		case errors.Is(err, invite.ErrEmailMismatch):
			writeError(w, http.StatusForbidden, "email_mismatch", "invitación para otro email")
		case errors.Is(err, invite.ErrNotPending):
			writeError(w, http.StatusConflict, "not_pending", "invitación ya no está pendiente")
		default:
			writeError(w, http.StatusInternalServerError, "accept", err.Error())
		}
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"user_id":         userID,
		"organization_id": orgID,
		"role":            role,
	})
}

func (a *API) declineInvite(w http.ResponseWriter, r *http.Request) {
	token, err := uuid.Parse(r.PathValue("token"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err := a.InviteService.Decline(r.Context(), token); err != nil {
		if errors.Is(err, invite.ErrNotPending) {
			writeError(w, http.StatusConflict, "not_pending", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "decline", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) revokeInvite(w http.ResponseWriter, r *http.Request) {
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
	if err := a.InviteService.Revoke(r.Context(), id, actorID); err != nil {
		if errors.Is(err, invite.ErrNotPending) {
			writeError(w, http.StatusConflict, "not_pending", "")
			return
		}
		writeError(w, http.StatusInternalServerError, "revoke", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

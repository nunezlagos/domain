package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/saargo/domain/internal/auth/apikey"
	orgsvc "github.com/saargo/domain/internal/service/org"
)

func principal(r *http.Request) (*apikey.Principal, bool) {
	return apikey.FromContext(r.Context())
}

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

type createOrgBody struct {
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	OwnerEmail string `json:"owner_email,omitempty"` // si vacío usa el del principal
	OwnerName  string `json:"owner_name,omitempty"`
}

func (a *API) createOrg(w http.ResponseWriter, r *http.Request) {
	var b createOrgBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if b.Name == "" || b.Slug == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "name y slug requeridos")
		return
	}
	ownerEmail, ownerName := b.OwnerEmail, b.OwnerName
	if ownerEmail == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "owner_email requerido")
		return
	}

	org, owner, err := a.OrgService.Create(r.Context(), b.Name, b.Slug, ownerEmail, ownerName)
	if err != nil {
		switch {
		case errors.Is(err, orgsvc.ErrSlugInvalid):
			writeError(w, http.StatusUnprocessableEntity, "invalid_slug", err.Error())
		case errors.Is(err, orgsvc.ErrSlugTaken):
			writeError(w, http.StatusConflict, "slug_taken", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create_org", err.Error())
		}
		return
	}
	w.Header().Set("Location", "/api/v1/organizations/"+org.ID.String())
	writeData(w, http.StatusCreated, map[string]any{
		"organization": org,
		"owner":        owner,
	})
}

func (a *API) getOrg(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}
	org, err := a.OrgService.GetByID(r.Context(), id)
	if errors.Is(err, orgsvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_org", err.Error())
		return
	}
	writeData(w, http.StatusOK, org)
}

type updateOrgBody struct {
	Settings map[string]any `json:"settings"`
}

func (a *API) updateOrg(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	actorID, _ := uuid.Parse(p.UserID)
	var b updateOrgBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	org, err := a.OrgService.UpdateSettings(r.Context(), id, actorID, b.Settings)
	if errors.Is(err, orgsvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update", err.Error())
		return
	}
	writeData(w, http.StatusOK, org)
}

type deleteOrgBody struct {
	Confirm string `json:"confirm"`
}

func (a *API) deleteOrg(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	actorID, _ := uuid.Parse(p.UserID)
	var b deleteOrgBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if err := a.OrgService.SoftDelete(r.Context(), id, actorID, b.Confirm); err != nil {
		switch {
		case errors.Is(err, orgsvc.ErrConfirmMismatch):
			writeError(w, http.StatusUnprocessableEntity, "confirm_mismatch", "slug de confirmación no coincide")
		case errors.Is(err, orgsvc.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "organization not found")
		default:
			writeError(w, http.StatusInternalServerError, "delete", err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listMembers(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	members, err := a.OrgService.ListMembers(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, members)
}

type transferBody struct {
	ToUserID string `json:"to_user_id"`
}

func (a *API) transferOwnership(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	fromID, _ := uuid.Parse(p.UserID)
	var b transferBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	toID, err := parseUUID(b.ToUserID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_user_id", err.Error())
		return
	}
	if err := a.OrgService.TransferOwnership(r.Context(), id, fromID, toID); err != nil {
		switch {
		case errors.Is(err, orgsvc.ErrNotOwner):
			writeError(w, http.StatusForbidden, "not_owner", err.Error())
		case errors.Is(err, orgsvc.ErrTargetNotMember):
			writeError(w, http.StatusUnprocessableEntity, "target_not_eligible", err.Error())
		case errors.Is(err, orgsvc.ErrUserNotFound):
			writeError(w, http.StatusNotFound, "user_not_found", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "transfer", err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

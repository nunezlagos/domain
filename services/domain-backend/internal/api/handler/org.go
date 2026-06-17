package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/auth/apikey"
	orgsvc "nunezlagos/domain/internal/service/org"
)

func principal(r *http.Request) (*apikey.Principal, bool) {
	return apikey.FromContext(r.Context())
}

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
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

// addMemberWithKeyBody body de POST /api/v1/organizations/{id}/members (issue-36.1).
type addMemberWithKeyBody struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
	Role  string `json:"role"`
}

// addMemberWithKey crea user + api_key en una sola tx, sin pasar por
// invitations/OTP/email. Solo accesible para admin/owner de la org del path.
// El plaintext de la key se devuelve UNA SOLA VEZ en la response.
func (a *API) addMemberWithKey(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	// Anti-enumeration: si principal no pertenece a esa org, 404 (igual que
	// "no existe"). No revela existencia de otras orgs.
	if p.OrganizationID != orgID.String() {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}
	if p.Role != orgsvc.RoleOwner && p.Role != orgsvc.RoleAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "owners/admins only")
		return
	}
	actorID, err := uuid.Parse(p.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid actor id")
		return
	}

	var b addMemberWithKeyBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if b.Email == "" || b.Role == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "email and role are required")
		return
	}

	out, err := a.OrgService.AddMemberWithAPIKey(r.Context(), orgID, actorID, b.Email, b.Name, b.Role)
	switch {
	case errors.Is(err, orgsvc.ErrInvalidEmail):
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "email format invalid")
		return
	case errors.Is(err, orgsvc.ErrInvalidRole):
		writeError(w, http.StatusUnprocessableEntity, "invalid_role", err.Error())
		return
	case errors.Is(err, orgsvc.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", "email already in use within the organization")
		return
	case errors.Is(err, orgsvc.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "create_member", err.Error())
		return
	}

	w.Header().Set("Location", "/api/v1/users/"+out.User.UserID.String())
	writeData(w, http.StatusCreated, map[string]any{
		"user": map[string]any{
			"id":         out.User.UserID,
			"email":      out.User.Email,
			"name":       out.User.Name,
			"role":       out.User.Role,
			"joined_at":  out.User.JoinedAt,
		},
		"api_key":    out.APIKey,
		"api_key_id": out.APIKeyID,
		"key_prefix": out.KeyPrefix,
	})
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


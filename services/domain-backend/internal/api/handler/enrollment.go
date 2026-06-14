package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	enrollsvc "nunezlagos/domain/internal/service/enrollment"
	orgsvc "nunezlagos/domain/internal/service/org"
)

// enrollSelfBody body de POST /api/v1/auth/enroll.
type enrollSelfBody struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// enrollSelf POST /api/v1/auth/enroll — sin auth Bearer, gating por
// header X-Enrollment-Token (issue-37.1).
func (a *API) enrollSelf(w http.ResponseWriter, r *http.Request) {
	if a.Enrollment == nil {
		writeError(w, http.StatusServiceUnavailable, "enrollment_not_configured", "")
		return
	}
	token := r.Header.Get("X-Enrollment-Token")
	if token == "" {
		writeError(w, http.StatusUnauthorized, "invalid_token", "missing X-Enrollment-Token header")
		return
	}
	var b enrollSelfBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if b.Email == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "email is required")
		return
	}

	out, err := a.Enrollment.Enroll(r.Context(), token, b.Email, b.Name)
	switch {
	case errors.Is(err, enrollsvc.ErrInvalidToken):
		writeError(w, http.StatusUnauthorized, "invalid_token", "enrollment token invalid or revoked")
		return
	case errors.Is(err, enrollsvc.ErrInvalidEmail):
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "email format invalid")
		return
	case errors.Is(err, enrollsvc.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", "email already in use within the organization")
		return
	case errors.Is(err, enrollsvc.ErrOrgNotFound):
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "enroll", err.Error())
		return
	}

	w.Header().Set("Location", "/api/v1/users/"+out.UserID.String())
	writeData(w, http.StatusCreated, map[string]any{
		"user": map[string]any{
			"id":    out.UserID,
			"email": out.Email,
			"name":  out.Name,
			"role":  out.Role,
		},
		"organization": map[string]any{
			"id":   out.OrganizationID,
			"name": out.OrgName,
			"slug": out.OrgSlug,
		},
		"api_key":    out.APIKey,
		"api_key_id": out.APIKeyID,
		"key_prefix": out.KeyPrefix,
	})
}

// rotateEnrollmentTokenBody body opcional de rotate.
type rotateEnrollmentTokenBody struct {
	RoleOnEnroll string `json:"role_on_enroll,omitempty"`
}

// rotateEnrollmentToken POST /api/v1/organizations/{id}/enrollment-token/rotate.
// Admin/owner-only. Devuelve el plaintext UNA sola vez.
func (a *API) rotateEnrollmentToken(w http.ResponseWriter, r *http.Request) {
	orgID, actorID, ok := a.authEnrollmentAdmin(w, r)
	if !ok {
		return
	}
	var b rotateEnrollmentTokenBody
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &b); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
	}
	out, err := a.Enrollment.Rotate(r.Context(), orgID, actorID, b.RoleOnEnroll)
	switch {
	case errors.Is(err, enrollsvc.ErrInvalidRole):
		writeError(w, http.StatusUnprocessableEntity, "invalid_role", err.Error())
		return
	case errors.Is(err, enrollsvc.ErrOrgNotFound):
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "rotate", err.Error())
		return
	}
	writeData(w, http.StatusCreated, map[string]any{
		"enrollment_token": out.Plaintext,
		"role_on_enroll":   out.RoleOnEnroll,
		"key_prefix":       out.Prefix,
		"created_at":       out.CreatedAt,
	})
}

// getEnrollmentTokenMetadata GET /api/v1/organizations/{id}/enrollment-token.
// Devuelve metadata sin plaintext.
func (a *API) getEnrollmentTokenMetadata(w http.ResponseWriter, r *http.Request) {
	orgID, _, ok := a.authEnrollmentAdmin(w, r)
	if !ok {
		return
	}
	m, err := a.Enrollment.GetMetadata(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_metadata", err.Error())
		return
	}
	resp := map[string]any{
		"exists": m.Exists,
	}
	if m.Exists {
		resp["key_prefix"] = m.Prefix
		resp["role_on_enroll"] = m.RoleOnEnroll
		resp["created_at"] = m.CreatedAt
	}
	writeData(w, http.StatusOK, resp)
}

// deleteEnrollmentToken DELETE /api/v1/organizations/{id}/enrollment-token.
// Revoca el token activo sin crear uno nuevo. 204 si éxito, 404 si no había activo.
func (a *API) deleteEnrollmentToken(w http.ResponseWriter, r *http.Request) {
	orgID, actorID, ok := a.authEnrollmentAdmin(w, r)
	if !ok {
		return
	}
	err := a.Enrollment.Revoke(r.Context(), orgID, actorID)
	switch {
	case errors.Is(err, enrollsvc.ErrNoActive):
		writeError(w, http.StatusNotFound, "no_active_token", "no active enrollment token to revoke")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "revoke", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// authEnrollmentAdmin valida principal + RBAC owner/admin del path's org.
// Devuelve (orgID, actorID, true) si pasa; escribe error y devuelve false si no.
func (a *API) authEnrollmentAdmin(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	if a.Enrollment == nil {
		writeError(w, http.StatusServiceUnavailable, "enrollment_not_configured", "")
		return uuid.Nil, uuid.Nil, false
	}
	orgID, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return uuid.Nil, uuid.Nil, false
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return uuid.Nil, uuid.Nil, false
	}
	if p.OrganizationID != orgID.String() {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return uuid.Nil, uuid.Nil, false
	}
	if p.Role != orgsvc.RoleOwner && p.Role != orgsvc.RoleAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "owners/admins only")
		return uuid.Nil, uuid.Nil, false
	}
	actorID, err := uuid.Parse(p.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid actor id")
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, actorID, true
}

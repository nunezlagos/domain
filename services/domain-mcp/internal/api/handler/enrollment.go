package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/auth/rbac"
	enrollsvc "nunezlagos/domain/internal/service/enrollment"
)

func principal(r *http.Request) (*apikey.Principal, bool) {
	return apikey.FromContext(r.Context())
}

var (
	roleOwner = string(rbac.RoleOwner)
	roleAdmin = string(rbac.RoleAdmin)
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
		writeError(w, http.StatusConflict, "email_taken", "email already in use")
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
		"api_key":    out.APIKey,
		"api_key_id": out.APIKeyID,
		"key_prefix": out.KeyPrefix,
	})
}

// authEnrollmentAdmin valida principal + RBAC owner/admin.
// Single-org: ya no valida pertenencia a una org. Devuelve (actorID, true)
// si pasa; escribe error y devuelve false si no.
func (a *API) authEnrollmentAdmin(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	if a.Enrollment == nil {
		writeError(w, http.StatusServiceUnavailable, "enrollment_not_configured", "")
		return uuid.Nil, false
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return uuid.Nil, false
	}
	if p.Role != roleOwner && p.Role != roleAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "owners/admins only")
		return uuid.Nil, false
	}
	actorID, err := uuid.Parse(p.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid actor id")
		return uuid.Nil, false
	}
	return actorID, true
}

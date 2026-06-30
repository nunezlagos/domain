package handler

import (
	"errors"
	"net/http"

	"nunezlagos/domain/internal/auth/bootstrap"
)

// GET /api/v1/auth/first-run
// Indica si la DB esta vacia (sin users). Helper para que el CLI
// decida si usar bootstrap o member-create antes de pedir email.
func (a *API) authFirstRun(w http.ResponseWriter, r *http.Request) {
	if a.Bootstrap == nil {
		writeError(w, http.StatusServiceUnavailable, "bootstrap_disabled", "bootstrap service not configured")
		return
	}
	isFirstRun, count, err := a.Bootstrap.IsFirstRun(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "first_run_check_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"is_first_run": isFirstRun,
		"user_count":   count,
	})
}

// POST /api/v1/auth/bootstrap
// Auto-crea el primer user de la primera org. SOLO funciona si
// la DB no tiene users. Despues, retorna 400 email_not_in_any_org
// y el caller debe pedirle a un admin que use member-create (HU-36.1)
// o enrollment token (HU-37.1).
func (a *API) authBootstrap(w http.ResponseWriter, r *http.Request) {
	if a.Bootstrap == nil {
		writeError(w, http.StatusServiceUnavailable, "bootstrap_disabled", "bootstrap service not configured")
		return
	}
	var b bootstrapRequest
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "JSON invalido")
		return
	}
	if b.Email == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "email requerido")
		return
	}

	res, err := a.Bootstrap.Bootstrap(r.Context(), bootstrap.BootstrapInput{
		Email:   b.Email,
		KeyName: b.KeyName,
		OrgName: b.OrgName,
	})
	switch {
	case errors.Is(err, bootstrap.ErrNotFirstRun):
		writeError(w, http.StatusBadRequest, "email_not_in_any_org",
			"bootstrap is first-run only; admin debe usar member-create o enrollment token")
		return
	case errors.Is(err, bootstrap.ErrInvalidEmail):
		writeError(w, http.StatusUnprocessableEntity, "invalid_email", "email format invalid")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "bootstrap_failed", err.Error())
		return
	}

	writeData(w, http.StatusOK, map[string]any{
		"user_id":          res.UserID,
		"api_key":          res.APIKey,
		"api_key_id":       res.APIKeyID,
		"email":            res.Email,
		"method":           "bootstrap",
		"enrollment_token": res.EnrollmentToken,
		"enrollment_role":  res.EnrollmentRole,
		"enrollment_note":  "comparti este enrollment_token con tu equipo; cualquiera con el token puede self-enrolarse via POST /api/v1/auth/enroll. Rotalo si se filtra.",
		"note":             "guarda la API key — solo se muestra UNA vez. No expira automaticamente; rotala manualmente con /domain-login.",
	})
}

type bootstrapRequest struct {
	Email   string `json:"email"`
	KeyName string `json:"key_name,omitempty"`
	OrgName string `json:"org_name,omitempty"`
}

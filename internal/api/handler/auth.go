package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/saargo/domain/internal/auth/otp"
)

type requestOTPBody struct {
	Identifier string `json:"identifier"` // RUT o email
}

type verifyOTPBody struct {
	Identifier string `json:"identifier"`
	Code       string `json:"code"`
	KeyName    string `json:"key_name,omitempty"`
}

// POST /api/v1/auth/request-otp
// Anti-enumeration: response 200 incluso si user no existe.
func (a *API) requestOTP(w http.ResponseWriter, r *http.Request) {
	var b requestOTPBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "JSON inválido")
		return
	}
	if b.Identifier == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "identifier requerido")
		return
	}
	if a.OTPService != nil {
		_ = a.OTPService.Request(r.Context(), b.Identifier, r.RemoteAddr, r.UserAgent())
		// Swallow ErrUserNotFound: anti-enumeration
	}
	writeData(w, http.StatusOK, map[string]any{
		"message": "si el identifier corresponde a una cuenta, recibirás un código",
	})
}

// POST /api/v1/auth/verify-otp
// Si OK: emite API key nueva y la devuelve UNA vez junto con user_id y org_id.
func (a *API) verifyOTP(w http.ResponseWriter, r *http.Request) {
	var b verifyOTPBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "JSON inválido")
		return
	}
	if b.Identifier == "" || b.Code == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "identifier y code requeridos")
		return
	}
	if a.OTPService == nil || a.APIKeys == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "auth no configurado")
		return
	}

	result, err := a.OTPService.Verify(r.Context(), b.Identifier, b.Code)
	if err != nil {
		switch {
		case errors.Is(err, otp.ErrInvalidCode):
			writeError(w, http.StatusUnauthorized, "invalid_code", "código inválido")
		case errors.Is(err, otp.ErrOTPExpired):
			writeError(w, http.StatusUnauthorized, "code_expired", "código expirado")
		case errors.Is(err, otp.ErrTooManyAttempts):
			writeError(w, http.StatusTooManyRequests, "too_many_attempts", "demasiados intentos")
		default:
			writeError(w, http.StatusUnauthorized, "invalid_code", "código inválido")
		}
		return
	}

	// Obtener orgID del user
	var orgID uuid.UUID
	if err := a.APIKeys.Pool.QueryRow(r.Context(),
		`SELECT organization_id FROM users WHERE id = $1`, result.UserID).Scan(&orgID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "lookup user")
		return
	}

	name := b.KeyName
	if name == "" {
		name = "domain-cli"
	}
	plaintext, keyID, err := a.APIKeys.Issue(r.Context(), orgID, result.UserID, name, "live")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "issue_key", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"user_id":         result.UserID,
		"organization_id": orgID,
		"email":           result.Email,
		"api_key":         plaintext,
		"api_key_id":      keyID,
		"note":            "guardá la API key — solo se muestra UNA vez",
	})
}

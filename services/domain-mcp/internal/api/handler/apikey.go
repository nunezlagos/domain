package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/auth/apikey"
)

type createAPIKeyBody struct {
	Name string `json:"name"`
	Env  string `json:"env,omitempty"` // dev|staging|live
}

// listAPIKeys GET /api/v1/api-keys
func (a *API) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(r)
	if !ok || p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "auth required")
		return
	}
	if a.APIKeys == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "api key store not configured")
		return
	}
	orgID, err := uuid.Parse(p.OrganizationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_org", "invalid organization")
		return
	}
	keys, err := a.APIKeys.List(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, keys)
}

// createAPIKey POST /api/v1/api-keys
func (a *API) createAPIKey(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(r)
	if !ok || p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "auth required")
		return
	}
	if a.APIKeys == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "api key store not configured")
		return
	}
	var b createAPIKeyBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "JSON invalido")
		return
	}
	if b.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "name requerido")
		return
	}
	env := b.Env
	if env == "" {
		env = "live"
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	userID, _ := uuid.Parse(p.UserID)
	plaintext, keyID, err := a.APIKeys.Issue(r.Context(), orgID, userID, b.Name, env)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "issue_failed", err.Error())
		return
	}
	writeData(w, http.StatusCreated, map[string]any{
		"api_key":    plaintext,
		"api_key_id": keyID,
		"note":       "guarda la API key — solo se muestra UNA vez",
	})
}

// revokeAPIKey DELETE /api/v1/api-keys/{id}
func (a *API) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(r)
	if !ok || p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "auth required")
		return
	}
	if a.APIKeys == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "api key store not configured")
		return
	}
	keyID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid key id")
		return
	}
	if err := a.APIKeys.Revoke(r.Context(), keyID); err != nil {
		if errors.Is(err, apikey.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "key not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "revoke_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

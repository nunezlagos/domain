package handler

import (
	"encoding/json"
	"net/http"

	"nunezlagos/domain/internal/runtimeconfig"
)

type runtimeConfigUpdateBody struct {
	Value json.RawMessage `json:"value"`
}

// GET /api/v1/admin/runtime-configs/{key} — issue-27.3
func (a *API) getRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.RuntimeConfigRegistry == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime_config_not_configured", "")
		return
	}
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing_key", "")
		return
	}
	snap := a.RuntimeConfigRegistry.Current()
	val, err := runtimeconfig.ValueJSON(snap, key)
	if err != nil {
		writeError(w, http.StatusNotFound, "unknown_key", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"key":           key,
		"value":         val,
		"hot_reloadable": runtimeconfig.HotReloadable[key],
	})
}

// POST /api/v1/admin/runtime-configs/{key} — issue-27.3
func (a *API) updateRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.RuntimeConfigRegistry == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime_config_not_configured", "")
		return
	}
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing_key", "")
		return
	}
	if !runtimeconfig.HotReloadable[key] {
		writeError(w, http.StatusConflict, "not_hot_reloadable",
			"this key requires a server restart to change")
		return
	}
	var in runtimeConfigUpdateBody
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	snap := runtimeconfig.Defaults()
	if err := runtimeconfig.ApplyValue(snap, key, in.Value); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_value", err.Error())
		return
	}
	if err := a.RuntimeConfigRegistry.Update(r.Context(), key, in.Value); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	if err := a.RuntimeConfigRegistry.Refresh(r.Context()); err != nil {
		a.RuntimeConfigRegistry.Logger.Warn("runtime config refresh after update failed",
			"key", key, "error", err)
	}
	writeData(w, http.StatusOK, map[string]any{
		"key":   key,
		"value": in.Value,
	})
}

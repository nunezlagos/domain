package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/auth/apikey"
)

// promptRouterRequest body de POST /api/v1/prompt
type promptRouterRequest struct {
	RawText string `json:"raw_text"`
}

// routePrompt — endpoint HTTP alternativo al MCP tool domain_prompt (HU-12.7).
// Útil para clientes no-MCP (web UI, scripts, curl, tests E2E).
//
// POST /api/v1/prompt
//
//	{"raw_text": "el botón no funciona ya pasé screenshot"}
//
// Response 200:
//
//	{
//	  "data": {
//	    "outcome": "wizard_started",
//	    "intent": "fix",
//	    "confidence": 0.75,
//	    "intake_id": "...",
//	    "draft_id": "...",
//	    "next_question": {...},
//	    "reasoning": "..."
//	  }
//	}
func (a *API) routePrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	if a.PromptRouter == nil {
		writeError(w, http.StatusServiceUnavailable, "prompt_router_unavailable",
			"prompt router not configured in this binary")
		return
	}

	var req promptRouterRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	var createdBy *uuid.UUID
	if p, ok := apikey.FromContext(r.Context()); ok && p != nil {
		if u, err := uuid.Parse(p.UserID); err == nil {
			createdBy = &u
		}
	}

	resp, err := a.PromptRouter.Route(r.Context(), req.RawText, createdBy)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "route_failed", err.Error())
		return
	}

	writeData(w, http.StatusOK, resp)
}

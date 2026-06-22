package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/service/promptrouter"
)

// promptRouterRequest body de POST /api/v1/prompt
type promptRouterRequest struct {
	RawText string `json:"raw_text"`
	// Intent opcional: si el cliente ya clasificó (vía prompt 'triage'),
	// se usa directo y se saltea la clasificación del servidor.
	Intent string `json:"intent,omitempty"`
	// ProjectID opcional: scopea el intake/triage al proyecto.
	ProjectID string `json:"project_id,omitempty"`
}

// routePrompt — endpoint HTTP alternativo al MCP tool domain_prompt (issue-12.7).
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
	var orgID *uuid.UUID
	if p, ok := apikey.FromContext(r.Context()); ok && p != nil {
		if u, err := uuid.Parse(p.UserID); err == nil {
			createdBy = &u
		}
		if o, err := uuid.Parse(p.OrganizationID); err == nil {
			orgID = &o
		}
	}

	intentOverride := promptrouter.ParseIntent(req.Intent)
	var projectID *uuid.UUID
	if req.ProjectID != "" {
		if p, perr := uuid.Parse(req.ProjectID); perr == nil {
			projectID = &p
		} else {
			writeError(w, http.StatusBadRequest, "invalid_project_id", perr.Error())
			return
		}
	}
	resp, err := a.PromptRouter.RouteWithIntent(r.Context(), req.RawText, createdBy, orgID, projectID, intentOverride)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "route_failed", err.Error())
		return
	}

	writeData(w, http.StatusOK, resp)
}

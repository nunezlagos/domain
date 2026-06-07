// Package handler — HTTP handlers REST /api/v1/*.
//
// Router minimalista usando net/http patterns Go 1.22+ (method + path).
// Middleware stack: requestID → recover → metrics → auth (skip allowlist) → handler.
//
// Convenciones (rules/api.md):
//   - JSON requests/responses, error shape {"error":{"code","message",...}}
//   - 201 con Location, 204 en DELETE, 422 validation, 401 unauthorized
//   - X-Request-Id correlación, X-Rate-Limit-* cuando aplica
package handler

import (
	"encoding/json"
	"net/http"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/auth/otp"
	agentrunner "nunezlagos/domain/internal/runner/agent"
	agentsvc "nunezlagos/domain/internal/service/agent"
	"nunezlagos/domain/internal/service/invite"
	"nunezlagos/domain/internal/service/knowledge"
	"nunezlagos/domain/internal/service/observation"
	orgsvc "nunezlagos/domain/internal/service/org"
	projsvc "nunezlagos/domain/internal/service/project"
	promptsvc "nunezlagos/domain/internal/service/prompt"
	searchsvc "nunezlagos/domain/internal/service/search"
	sesssvc "nunezlagos/domain/internal/service/session"
	skillsvc "nunezlagos/domain/internal/service/skill"
	timelinesvc "nunezlagos/domain/internal/service/timeline"
)

// API agrupa todas las dependencias y monta el router /api/v1/*.
type API struct {
	OrgService     *orgsvc.Service
	ProjectService *projsvc.Service
	ObsService     *observation.Service
	InviteService  *invite.Service
	SessionService  *sesssvc.Service
	PromptService   *promptsvc.Service
	TimelineService  *timelinesvc.Service
	SearchService    *searchsvc.Service
	KnowledgeService *knowledge.Service
	SkillService     *skillsvc.Service
	AgentService     *agentsvc.Service
	AgentRunner      *agentrunner.Runner
	OTPService     *otp.Service
	APIKeys        *apikey.PGStore
}

// Router devuelve un http.Handler montado en /api/v1/*.
func (a *API) Router() http.Handler {
	mux := http.NewServeMux()

	// Auth (sin Bearer requerido)
	mux.HandleFunc("POST /api/v1/auth/request-otp", a.requestOTP)
	mux.HandleFunc("POST /api/v1/auth/verify-otp", a.verifyOTP)

	// Organizaciones (require auth)
	mux.HandleFunc("POST /api/v1/organizations", a.createOrg)
	mux.HandleFunc("GET /api/v1/organizations/{id}", a.getOrg)
	mux.HandleFunc("PATCH /api/v1/organizations/{id}", a.updateOrg)
	mux.HandleFunc("DELETE /api/v1/organizations/{id}", a.deleteOrg)
	mux.HandleFunc("GET /api/v1/organizations/{id}/members", a.listMembers)
	mux.HandleFunc("POST /api/v1/organizations/{id}/transfer-ownership", a.transferOwnership)

	// Invitations
	mux.HandleFunc("POST /api/v1/organizations/{id}/invitations", a.createInvite)
	mux.HandleFunc("GET /api/v1/organizations/{id}/invitations", a.listInvites)
	mux.HandleFunc("POST /api/v1/invitations/{token}/accept", a.acceptInvite)
	mux.HandleFunc("POST /api/v1/invitations/{token}/decline", a.declineInvite)
	mux.HandleFunc("POST /api/v1/invitations/{id}/revoke", a.revokeInvite)

	// Projects (scoped by org via ?organization_id= o derivado del principal)
	mux.HandleFunc("POST /api/v1/projects", a.createProject)
	mux.HandleFunc("GET /api/v1/projects", a.listProjects)
	mux.HandleFunc("GET /api/v1/projects/{slug}", a.getProject)
	mux.HandleFunc("PATCH /api/v1/projects/{slug}", a.updateProject)
	mux.HandleFunc("DELETE /api/v1/projects/{slug}", a.deleteProject)

	// Observations
	mux.HandleFunc("POST /api/v1/observations", a.saveObservation)
	mux.HandleFunc("GET /api/v1/observations/{id}", a.getObservation)
	mux.HandleFunc("DELETE /api/v1/observations/{id}", a.deleteObservation)
	mux.HandleFunc("GET /api/v1/observations", a.listObservations) // ?project_slug=
	mux.HandleFunc("GET /api/v1/search", a.searchObservations)     // ?q=

	// Sessions
	mux.HandleFunc("POST /api/v1/sessions", a.startSession)
	mux.HandleFunc("GET /api/v1/sessions", a.listSessions)
	mux.HandleFunc("GET /api/v1/sessions/active", a.activeSession) // ?project_slug=
	mux.HandleFunc("GET /api/v1/sessions/{id}", a.getSession)
	mux.HandleFunc("POST /api/v1/sessions/{id}/end", a.endSession)

	// Agents
	mux.HandleFunc("POST /api/v1/agents", a.createAgent)
	mux.HandleFunc("GET /api/v1/agents", a.listAgents)
	mux.HandleFunc("GET /api/v1/agents/{id}", a.getAgent)
	mux.HandleFunc("PATCH /api/v1/agents/{id}", a.updateAgent)
	mux.HandleFunc("DELETE /api/v1/agents/{id}", a.deleteAgent)
	mux.HandleFunc("POST /api/v1/agents/{id}/run", a.runAgent)

	// Skills
	mux.HandleFunc("POST /api/v1/skills", a.createSkill)
	mux.HandleFunc("GET /api/v1/skills", a.listSkills)       // ?type= &tag= &limit=
	mux.HandleFunc("GET /api/v1/skills/search", a.searchSkills) // ?q=
	mux.HandleFunc("GET /api/v1/skills/{id}", a.getSkill)
	mux.HandleFunc("PATCH /api/v1/skills/{id}", a.updateSkill)
	mux.HandleFunc("DELETE /api/v1/skills/{id}", a.deleteSkill)

	// Knowledge documents
	mux.HandleFunc("POST /api/v1/knowledge", a.saveKnowledge)
	mux.HandleFunc("GET /api/v1/knowledge", a.listKnowledge)        // ?project_slug=
	mux.HandleFunc("GET /api/v1/knowledge/search", a.searchKnowledge) // ?q=
	mux.HandleFunc("GET /api/v1/knowledge/{id}", a.getKnowledge)
	mux.HandleFunc("DELETE /api/v1/knowledge/{id}", a.deleteKnowledge)

	// Context + Timeline (cross-entity feeds)
	mux.HandleFunc("GET /api/v1/context", a.getContext)                          // ?project_slug=
	mux.HandleFunc("GET /api/v1/observations/{id}/timeline", a.getTimeline)      // ?before= &after=

	// Prompts (templates versionados)
	mux.HandleFunc("POST /api/v1/prompts", a.createPrompt)
	mux.HandleFunc("GET /api/v1/prompts/{id}", a.getPrompt)
	mux.HandleFunc("POST /api/v1/prompts/{id}/activate", a.setActivePrompt)
	mux.HandleFunc("DELETE /api/v1/prompts/{id}", a.deletePrompt)
	mux.HandleFunc("GET /api/v1/prompts/by-slug/{slug}/versions", a.listPromptVersions)
	mux.HandleFunc("GET /api/v1/prompts/search", a.searchPrompts)

	return mux
}

// Allowlist paths que skipean auth (definida en uno solo lugar para evitar drift).
func AuthAllowlist() []string {
	return []string{
		"/health",
		"/health/ready",
		"/health/startup",
		"/api/v1/auth/request-otp",
		"/api/v1/auth/verify-otp",
		"/metrics",
	}
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{"code": code, "message": msg},
	})
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, map[string]any{"data": data})
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

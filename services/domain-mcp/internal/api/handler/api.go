// Package handler — HTTP handlers REST /api/v1/*.
//
// Router minimalista usando net/http patterns Go 1.22+ (method + path).
// Middleware stack: requestID → recover → metrics → auth (skip allowlist) → handler.
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"reflect"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/auth/bootstrap"
	"nunezlagos/domain/internal/auth/ratelimit"
	"nunezlagos/domain/internal/auth/session"
	"nunezlagos/domain/internal/dispatch"
	enrollsvc "nunezlagos/domain/internal/service/enrollment"
	feedbacksvc "nunezlagos/domain/internal/service/feedback"
	skillsvc "nunezlagos/domain/internal/service/skill"
	skillabtestsvc "nunezlagos/domain/internal/service/skill_ab_test"
	skillmetricssvc "nunezlagos/domain/internal/service/skill_metrics"
	skillsuggestionssvc "nunezlagos/domain/internal/service/skill_suggestions"
	webhooksvc "nunezlagos/domain/internal/service/webhook"
)

// API agrupa las dependencias del router /api/v1/*.
type API struct {
	APIKeys            *apikey.PGStore
	AuthSessionService *session.Service
	Bootstrap          *bootstrap.Service
	Enrollment         *enrollsvc.Service
	WebhookService     *webhooksvc.Service
	WebhookDispatcher  *WebhookDispatcher
	Dispatcher         *dispatch.Dispatcher

	// Feedback — HU-52.1: user feedback loop (👍/👎) del chat IA.
	Feedback *feedbacksvc.Service
	// FeedbackLimiter — rate limit dedicado por user_email (30/min, anti-spam).
	FeedbackLimiter *ratelimit.Limiter

	// SkillMetrics — HU-52.2: lectura de skill success rate tracking.
	SkillMetrics *skillmetricssvc.Service
	// Skills — usado por SkillMetrics para resolver slug -> skill_id.
	Skills *skillsvc.Service

	// SkillSuggestions — HU-52.3: LLM-as-judge (list/get/approve/reject/apply).
	SkillSuggestions *skillsuggestionssvc.Service
	// SkillJudge — el aggregator del judge, para el run manual del batch.
	SkillJudge *skillsuggestionssvc.Aggregator

	// SkillABTest — HU-52.4: A/B testing de prompts (create/results/stop).
	SkillABTest *skillabtestsvc.Service
}

// Router devuelve un http.Handler montado en /api/v1/*.
func (a *API) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/auth/login", a.authLogin)
	mux.HandleFunc("GET /api/v1/auth/first-run", a.authFirstRun)
	mux.HandleFunc("POST /api/v1/auth/bootstrap", a.authBootstrap)
	mux.HandleFunc("POST /api/v1/auth/enroll", a.enrollSelf)
	mux.HandleFunc("POST /api/v1/webhooks/{slug}/receive", a.receiveWebhook)

	// HU-52.1 — feedback loop. CSRF-exempt (Bearer auth, sin cookies); rate
	// limit 30/min por user_email se aplica dentro de createFeedback.
	mux.HandleFunc("POST /api/v1/feedback", a.createFeedback)
	mux.HandleFunc("GET /api/v1/feedback", a.listFeedback)

	// HU-52.2 — skill success rate tracking (lectura). Rutas mas especificas
	// (top-failed/slowest) antes de la parametrizada {slug}.
	mux.HandleFunc("GET /api/v1/skills/metrics/top-failed", a.skillMetricsTopFailed)
	mux.HandleFunc("GET /api/v1/skills/metrics/slowest", a.skillMetricsSlowest)
	mux.HandleFunc("GET /api/v1/skills/{slug}/metrics", a.skillMetricsShow)

	// HU-52.3 — LLM-as-judge (human-in-the-loop). Ruta /run antes de /{id}
	// para que el path literal gane a la parametrizada. approve/reject/apply
	// son acciones humanas separadas (regla dura 6: approve NO aplica).
	mux.HandleFunc("GET /api/v1/skill-suggestions", a.skillSuggestionsList)
	mux.HandleFunc("POST /api/v1/skill-suggestions/run", a.skillSuggestionsRun)
	mux.HandleFunc("GET /api/v1/skill-suggestions/{id}", a.skillSuggestionsShow)
	mux.HandleFunc("POST /api/v1/skill-suggestions/{id}/approve", a.skillSuggestionsApprove)
	mux.HandleFunc("POST /api/v1/skill-suggestions/{id}/reject", a.skillSuggestionsReject)
	mux.HandleFunc("POST /api/v1/skill-suggestions/{id}/apply", a.skillSuggestionsApply)

	// HU-52.4 — A/B testing de prompts. start (create+start), results, stop.
	// El opt-in se deriva de la existencia de un test 'running' para el slug
	// (sin columna ab_test en skills). created_by sale del Principal.
	mux.HandleFunc("POST /api/v1/skill-ab-tests", a.skillABTestCreate)
	mux.HandleFunc("GET /api/v1/skill-ab-tests/{id}/results", a.skillABTestResults)
	mux.HandleFunc("POST /api/v1/skill-ab-tests/{id}/stop", a.skillABTestStop)

	return mux
}

// AuthAllowlist paths que skipean auth (definida en un solo lugar para evitar drift).
func AuthAllowlist() []string {
	return []string{
		"/health",
		"/healthz",
		"/health/ready",
		"/health/startup",
		"/api/v1/auth/login",
		"/api/v1/auth/first-run",
		"/api/v1/auth/enroll", // issue-37.1: gating por X-Enrollment-Token, no Bearer
		"/api/v1/webhooks/*",  // webhooks usan HMAC, no Bearer
		"/metrics",
	}
}

func ensureJSONSlice(data any) any {
	if data == nil {
		return []any{}
	}
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Slice && v.IsNil() {
		return reflect.MakeSlice(v.Type(), 0, 0).Interface()
	}
	return data
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("failed to encode response", "error", err, "status", status)
	}
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{"code": code, "message": msg},
	})
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, map[string]any{"data": ensureJSONSlice(data)})
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

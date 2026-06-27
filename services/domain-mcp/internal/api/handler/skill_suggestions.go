package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	skillsuggestionssvc "nunezlagos/domain/internal/service/skill_suggestions"
)

// HU-52.3 — handlers HTTP del LLM-as-judge (human-in-the-loop).
//
// Superficie REST:
//   - GET  /api/v1/skill-suggestions            list (filtros skill/kind/status)
//   - GET  /api/v1/skill-suggestions/{id}       detalle
//   - POST /api/v1/skill-suggestions/{id}/approve   pending -> approved
//   - POST /api/v1/skill-suggestions/{id}/reject    pending -> rejected
//   - POST /api/v1/skill-suggestions/{id}/apply     approved -> applied (muta skills)
//   - POST /api/v1/skill-suggestions/run        corre el batch del judge (manual)
//
// Regla dura 6: approve SOLO marca approved (no aplica). El apply es un paso
// explicito y separado (la mutacion de `skills` exige una segunda accion humana).
// reviewed_by sale del Principal (no del body) para no poder falsearlo.

// skillSuggestionsList maneja GET /api/v1/skill-suggestions.
func (a *API) skillSuggestionsList(w http.ResponseWriter, r *http.Request) {
	if a.SkillSuggestions == nil {
		writeError(w, http.StatusServiceUnavailable, "suggestions_disabled", "")
		return
	}
	q := r.URL.Query()
	f := skillsuggestionssvc.ListFilter{
		SkillSlug: strings.TrimSpace(q.Get("skill_slug")),
		Kind:      strings.TrimSpace(q.Get("kind")),
		Status:    strings.TrimSpace(q.Get("status")),
		Limit:     parseIntDefault(q.Get("limit"), 50),
		Offset:    parseIntDefault(q.Get("offset"), 0),
	}
	items, err := a.SkillSuggestions.List(r.Context(), f)
	switch {
	case errors.Is(err, skillsuggestionssvc.ErrInvalidKind):
		writeError(w, http.StatusBadRequest, "invalid_kind", "kind invalido (split|merge|refine|archive)")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, items)
}

// skillSuggestionsShow maneja GET /api/v1/skill-suggestions/{id}.
func (a *API) skillSuggestionsShow(w http.ResponseWriter, r *http.Request) {
	if a.SkillSuggestions == nil {
		writeError(w, http.StatusServiceUnavailable, "suggestions_disabled", "")
		return
	}
	id, ok := parseSuggestionID(w, r)
	if !ok {
		return
	}
	sug, err := a.SkillSuggestions.Get(r.Context(), id)
	switch {
	case errors.Is(err, skillsuggestionssvc.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "sugerencia no encontrada")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, sug)
}

// skillSuggestionsApprove maneja POST /api/v1/skill-suggestions/{id}/approve.
// Transiciona pending -> approved. NO aplica (regla dura 6).
func (a *API) skillSuggestionsApprove(w http.ResponseWriter, r *http.Request) {
	a.suggestionTransition(w, r, func(id uuid.UUID, reviewer *uuid.UUID) (*skillsuggestionssvc.Suggestion, error) {
		return a.SkillSuggestions.Approve(r.Context(), id, reviewer)
	})
}

// skillSuggestionsReject maneja POST /api/v1/skill-suggestions/{id}/reject.
func (a *API) skillSuggestionsReject(w http.ResponseWriter, r *http.Request) {
	a.suggestionTransition(w, r, func(id uuid.UUID, reviewer *uuid.UUID) (*skillsuggestionssvc.Suggestion, error) {
		return a.SkillSuggestions.Reject(r.Context(), id, reviewer)
	})
}

// suggestionTransition centraliza approve/reject: Principal -> reviewer, parse
// id, ejecuta la transicion y mapea errores tipados a HTTP.
func (a *API) suggestionTransition(w http.ResponseWriter, r *http.Request, fn func(uuid.UUID, *uuid.UUID) (*skillsuggestionssvc.Suggestion, error)) {
	if a.SkillSuggestions == nil {
		writeError(w, http.StatusServiceUnavailable, "suggestions_disabled", "")
		return
	}
	id, ok := parseSuggestionID(w, r)
	if !ok {
		return
	}
	reviewer, ok := suggestionReviewer(w, r)
	if !ok {
		return
	}
	sug, err := fn(id, reviewer)
	switch {
	case errors.Is(err, skillsuggestionssvc.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "sugerencia no encontrada")
		return
	case errors.Is(err, skillsuggestionssvc.ErrNotPending):
		writeError(w, http.StatusConflict, "not_pending", "la sugerencia ya fue revisada")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "transition_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, sug)
}

// skillSuggestionsApply maneja POST /api/v1/skill-suggestions/{id}/apply.
// Ejecuta la mutacion sobre `skills` (split/merge/refine/archive). Paso humano
// separado de approve. Degrada con ErrApplyUnavailable si refine/split necesita
// LLM y no hay (regla dura 7).
func (a *API) skillSuggestionsApply(w http.ResponseWriter, r *http.Request) {
	if a.SkillSuggestions == nil {
		writeError(w, http.StatusServiceUnavailable, "suggestions_disabled", "")
		return
	}
	id, ok := parseSuggestionID(w, r)
	if !ok {
		return
	}
	reviewer, ok := suggestionReviewer(w, r)
	if !ok {
		return
	}
	sug, result, err := a.SkillSuggestions.Apply(r.Context(), id, reviewer)
	switch {
	case errors.Is(err, skillsuggestionssvc.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "sugerencia no encontrada")
		return
	case errors.Is(err, skillsuggestionssvc.ErrNotApproved):
		writeError(w, http.StatusConflict, "not_approved", "la sugerencia debe estar approved para aplicarse")
		return
	case errors.Is(err, skillsuggestionssvc.ErrAlreadyApplied):
		writeError(w, http.StatusConflict, "already_applied", "la sugerencia ya fue aplicada")
		return
	case errors.Is(err, skillsuggestionssvc.ErrSeedManaged):
		writeError(w, http.StatusUnprocessableEntity, "seed_managed", "no se puede archivar/dividir un skill seed_managed")
		return
	case errors.Is(err, skillsuggestionssvc.ErrSkillNotFound):
		writeError(w, http.StatusUnprocessableEntity, "skill_not_found", "skill objetivo no encontrado o ya borrado")
		return
	case errors.Is(err, skillsuggestionssvc.ErrApplyUnavailable):
		writeError(w, http.StatusServiceUnavailable, "apply_unavailable", "apply de refine/split requiere LLM o content en payload")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "apply_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{"suggestion": sug, "result": result})
}

// skillSuggestionsRun maneja POST /api/v1/skill-suggestions/run.
// Corre el batch del judge manualmente (mismo Aggregator que el cron semanal).
// SOLO persiste pending (jamas aplica). Degrada con ErrJudgeUnavailable.
func (a *API) skillSuggestionsRun(w http.ResponseWriter, r *http.Request) {
	if a.SkillJudge == nil {
		writeError(w, http.StatusServiceUnavailable, "judge_disabled", "judge no configurado")
		return
	}
	res, err := a.SkillJudge.Run(r.Context())
	switch {
	case errors.Is(err, skillsuggestionssvc.ErrJudgeUnavailable):
		writeError(w, http.StatusServiceUnavailable, "judge_unavailable", "judge LLM requiere MINIMAX_API_KEY")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "run_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, res)
}

// parseSuggestionID extrae y valida el {id} de la ruta.
func parseSuggestionID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := strings.TrimSpace(r.PathValue("id"))
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id invalido")
		return uuid.Nil, false
	}
	return id, true
}

// suggestionReviewer resuelve el reviewer del Principal (no del body). Auth ya
// garantizo el Bearer; aca exigimos un user id valido para auditar quien reviso.
func suggestionReviewer(w http.ResponseWriter, r *http.Request) (*uuid.UUID, bool) {
	p, ok := principal(r)
	if !ok || p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return nil, false
	}
	uid, err := uuid.Parse(p.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid actor id")
		return nil, false
	}
	return &uid, true
}

package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	skillabtestsvc "nunezlagos/domain/internal/service/skill_ab_test"
)

// HU-52.4 — handlers HTTP del A/B testing de prompts.
//
// Superficie REST:
//   - POST /api/v1/skill-ab-tests              crea + arranca un experimento (start_now)
//   - GET  /api/v1/skill-ab-tests/{id}/results agregados de ambas variantes (+ test)
//   - POST /api/v1/skill-ab-tests/{id}/stop    cancela el experimento (status cancelled)
//
// Single-tenant (regla dura 1): NADA de organization_id. created_by sale del
// Principal (request-time), nunca del body, para no poder falsearlo.

// abTestCreateReq es el body del POST /api/v1/skill-ab-tests.
type abTestCreateReq struct {
	SkillSlug       string  `json:"skill_slug"`
	VersionA        int     `json:"version_a"`
	VersionB        int     `json:"version_b"`
	TrafficSplitA   float64 `json:"traffic_split_a"`
	MinInvocations  int     `json:"min_invocations"`
	AutoApplyWinner bool    `json:"auto_apply_winner"`
	StartNow        bool    `json:"start_now"`
}

// skillABTestCreate maneja POST /api/v1/skill-ab-tests.
//
// Crea (y arranca, si start_now) un experimento A/B. El service valida versiones
// distintas, slug requerido y unicidad del running por slug (opt-in unico).
func (a *API) skillABTestCreate(w http.ResponseWriter, r *http.Request) {
	if a.SkillABTest == nil {
		writeError(w, http.StatusServiceUnavailable, "ab_test_disabled", "")
		return
	}
	var in abTestCreateReq
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "body invalido")
		return
	}
	creator := abTestCreator(r)
	t, err := a.SkillABTest.Create(r.Context(), skillabtestsvc.CreateParams{
		SkillSlug:       strings.TrimSpace(in.SkillSlug),
		VersionA:        in.VersionA,
		VersionB:        in.VersionB,
		TrafficSplitA:   in.TrafficSplitA,
		MinInvocations:  in.MinInvocations,
		AutoApplyWinner: in.AutoApplyWinner,
		StartNow:        in.StartNow,
		CreatedBy:       creator,
	})
	switch {
	case errors.Is(err, skillabtestsvc.ErrEmptySlug):
		writeError(w, http.StatusBadRequest, "invalid_slug", "skill_slug requerido")
		return
	case errors.Is(err, skillabtestsvc.ErrInvalidVersions):
		writeError(w, http.StatusBadRequest, "invalid_versions", "version_a y version_b deben ser distintas")
		return
	case errors.Is(err, skillabtestsvc.ErrAlreadyRunning):
		writeError(w, http.StatusConflict, "already_running", "ya hay un A/B test running para este skill")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	writeData(w, http.StatusCreated, t)
}

// skillABTestResults maneja GET /api/v1/skill-ab-tests/{id}/results.
// Devuelve el test + los agregados de ambas variantes.
func (a *API) skillABTestResults(w http.ResponseWriter, r *http.Request) {
	if a.SkillABTest == nil {
		writeError(w, http.StatusServiceUnavailable, "ab_test_disabled", "")
		return
	}
	id, ok := parseABTestID(w, r)
	if !ok {
		return
	}
	t, err := a.SkillABTest.Get(r.Context(), id)
	switch {
	case errors.Is(err, skillabtestsvc.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "A/B test no encontrado")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	results, err := a.SkillABTest.GetResults(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "results_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{"ab_test": t, "results": results})
}

// skillABTestStop maneja POST /api/v1/skill-ab-tests/{id}/stop.
// Cancela el experimento (status 'cancelled').
func (a *API) skillABTestStop(w http.ResponseWriter, r *http.Request) {
	if a.SkillABTest == nil {
		writeError(w, http.StatusServiceUnavailable, "ab_test_disabled", "")
		return
	}
	id, ok := parseABTestID(w, r)
	if !ok {
		return
	}
	// Confirma existencia para devolver 404 explicito (Cancel es idempotente en DB).
	if _, err := a.SkillABTest.Get(r.Context(), id); err != nil {
		if errors.Is(err, skillabtestsvc.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "A/B test no encontrado")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	if err := a.SkillABTest.Cancel(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "stop_failed", err.Error())
		return
	}
	t, err := a.SkillABTest.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, t)
}

// parseABTestID extrae y valida el {id} de la ruta.
func parseABTestID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := strings.TrimSpace(r.PathValue("id"))
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id invalido")
		return uuid.Nil, false
	}
	return id, true
}

// abTestCreator resuelve el created_by del Principal (no del body). Si el actor
// no trae un user id valido, devuelve nil (created_by es opcional en la tabla).
func abTestCreator(r *http.Request) *uuid.UUID {
	p, ok := principal(r)
	if !ok || p == nil {
		return nil
	}
	uid, err := uuid.Parse(p.UserID)
	if err != nil {
		return nil
	}
	return &uid
}

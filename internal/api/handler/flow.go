package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/api/backpressure"
	"nunezlagos/domain/internal/audit"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	"nunezlagos/domain/internal/service/flow"
)

type createFlowBody struct {
	Slug                string    `json:"slug"`
	Name                string    `json:"name"`
	Description         string    `json:"description,omitempty"`
	Spec                flow.Spec `json:"spec"`
	DeterministicReplay bool      `json:"deterministic_replay,omitempty"`
}

func (a *API) createFlow(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	actorID, _ := uuid.Parse(p.UserID)
	var b createFlowBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	out, err := a.FlowService.Create(r.Context(), flow.CreateInput{
		OrganizationID:      orgID,
		Slug:                b.Slug,
		Name:                b.Name,
		Description:         b.Description,
		Spec:                b.Spec,
		DeterministicReplay: b.DeterministicReplay,
		ActorID:             actorID,
	})
	if err != nil {
		switch {
		case errors.Is(err, flow.ErrSlugInvalid),
			errors.Is(err, flow.ErrNameRequired),
			errors.Is(err, flow.ErrSpecInvalid):
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		case errors.Is(err, flow.ErrSlugTaken):
			writeError(w, http.StatusConflict, "slug_taken", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create", err.Error())
		}
		return
	}
	w.Header().Set("Location", "/api/v1/flows/"+out.ID.String())
	writeData(w, http.StatusCreated, out)
}

func (a *API) listFlows(w http.ResponseWriter, r *http.Request) {
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := a.FlowService.List(r.Context(), orgID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list", err.Error())
		return
	}
	writeData(w, http.StatusOK, list)
}

func (a *API) getFlow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	out, err := a.FlowService.GetByID(r.Context(), id)
	if errors.Is(err, flow.ErrNotFound) || (err == nil && out.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get", err.Error())
		return
	}
	writeData(w, http.StatusOK, out)
}

func (a *API) deleteFlow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	prev, err := a.FlowService.GetByID(r.Context(), id)
	if errors.Is(err, flow.ErrNotFound) || (err == nil && prev.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	actorID, _ := uuid.Parse(p.UserID)
	if err := a.FlowService.SoftDelete(r.Context(), id, actorID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type runFlowBody struct {
	Inputs map[string]any `json:"inputs,omitempty"`
}

// POST /api/v1/flows/{id}/run
func (a *API) runFlow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	prev, err := a.FlowService.GetByID(r.Context(), id)
	if errors.Is(err, flow.ErrNotFound) || (err == nil && prev.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	userID, _ := uuid.Parse(p.UserID)
	// issue-26.6 backpressure
	if a.Backpressure != nil {
		orgID, _ := uuid.Parse(p.OrganizationID)
		if err := a.Backpressure.CheckQueue(r.Context(),
			backpressure.PredefinedQueues["flow_runs"], orgID); err != nil {
			if errors.Is(err, backpressure.ErrQueueFull) || errors.Is(err, backpressure.ErrOrgQuotaExceeded) {
				retry := backpressure.RetryAfterSeconds(err)
				w.Header().Set("Retry-After", strconv.Itoa(retry))
				code := "queue_full"
				if errors.Is(err, backpressure.ErrOrgQuotaExceeded) {
					code = "org_queue_limit_exceeded"
				}
				writeError(w, http.StatusTooManyRequests, code,
					fmt.Sprintf("retry after %d seconds", retry))
				return
			}
		}
	}
	var b runFlowBody
	_ = decodeJSON(r, &b)
	res, runErr := a.FlowRunner.Run(r.Context(), flowrunner.RunInput{
		FlowID: id, TriggeredBy: &userID, TriggerType: "manual",
		Inputs: b.Inputs,
	})
	if res == nil {
		writeError(w, http.StatusInternalServerError, "run", runErr.Error())
		return
	}
	status := http.StatusOK
	if res.Status == flowrunner.StatusFailed {
		status = http.StatusUnprocessableEntity
	}
	writeData(w, status, map[string]any{
		"run_id":      res.RunID,
		"status":      res.Status,
		"outputs":     res.Outputs,
		"error":       res.Error,
		"started_at":  res.StartedAt,
		"finished_at": res.FinishedAt,
	})
}

// POST /api/v1/flows/{id}/dry-run — issue-09.12
func (a *API) dryRunFlow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.FlowRunner == nil {
		writeError(w, http.StatusServiceUnavailable, "runner_disabled", "")
		return
	}
	// Cross-org guard
	f, err := a.FlowService.GetByID(r.Context(), id)
	if errors.Is(err, flow.ErrNotFound) || (err == nil && f.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	var b runFlowBody
	_ = decodeJSON(r, &b)
	plan, err := a.FlowRunner.DryRun(r.Context(), id, b.Inputs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "dryrun", err.Error())
		return
	}
	writeData(w, http.StatusOK, plan)
}

type updateFlowBody struct {
	Spec *flow.Spec `json:"spec"`
	Note string     `json:"note,omitempty"`
}

// PATCH /api/v1/flows/{id} — issue-09.7 (fv-002)
// Crea una nueva flow_version en draft con el spec propuesto. NO muta la
// definition current del flow: el draft se activa recién al publish.
func (a *API) updateFlow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	prev, err := a.FlowService.GetByID(r.Context(), id)
	if errors.Is(err, flow.ErrNotFound) || (err == nil && prev.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}

	var b updateFlowBody
	if err := decodeJSON(r, &b); err != nil || b.Spec == nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "spec required")
		return
	}
	if err := b.Spec.Validate(); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	}

	def, err := json.Marshal(b.Spec)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	}
	sum := sha256.Sum256(def)
	hash := hex.EncodeToString(sum[:])

	actorID, _ := uuid.Parse(p.UserID)
	vs := &flow.VersioningStore{Pool: a.FlowService.Pool}
	v, err := vs.NewVersion(r.Context(), id, def, hash, b.Note, &actorID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "version_create", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"flow_id":    v.FlowID,
		"version":    v.Version,
		"version_id": v.ID,
		"status":     "draft",
		"hash":       v.Hash,
		"created_at": v.CreatedAt,
	})
}

type signalRunBody struct {
	Name    string          `json:"name"`
	StepKey string          `json:"step_key,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// POST /api/v1/runs/{id}/signals — issue-09.8 (sig-003)
// Entrega una señal externa a un flow_run en paused_awaiting_signal.
// 409 si el run no tiene expectativa pendiente para ese nombre.
func (a *API) signalFlowRun(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	run, err := a.FlowService.GetRun(r.Context(), id)
	if errors.Is(err, flow.ErrRunNotFound) || (err == nil && run.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}

	var b signalRunBody
	if err := decodeJSON(r, &b); err != nil || b.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "name required")
		return
	}

	signals := &flow.SignalStore{Pool: a.FlowService.Pool}
	pending, err := signals.HasPendingExpectation(r.Context(), id, b.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "pending_check", err.Error())
		return
	}
	if !pending {
		writeError(w, http.StatusConflict, "no_pending_signal",
			"no pending signal of that name for this run")
		return
	}

	var stepKey *string
	if b.StepKey != "" {
		stepKey = &b.StepKey
	}
	sig, err := signals.Send(r.Context(), id, stepKey, b.Name, b.Payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "signal_send", err.Error())
		return
	}
	if a.Audit != nil {
		actorID, _ := uuid.Parse(p.UserID)
		orgID, _ := uuid.Parse(p.OrganizationID)
		_ = a.Audit.Record(r.Context(), audit.Event{
			OrganizationID: &orgID, ActorID: &actorID,
			Action: "flow.signal_delivered", EntityType: "flow_run", EntityID: &id,
		})
	}
	writeData(w, http.StatusAccepted, map[string]any{
		"signal_id":   sig.ID,
		"flow_run_id": sig.FlowRunID,
		"name":        sig.Name,
		"created_at":  sig.CreatedAt,
	})
}

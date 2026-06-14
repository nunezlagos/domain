package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

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
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	actorID := a.userID(ctx)
	var b createFlowBody
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	out, err := a.FlowService.Create(ctx, flow.CreateInput{
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
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := a.FlowService.List(ctx, orgID, limit)
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
	ctx := r.Context()
	if a.orgID(ctx) == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	out, err := a.FlowService.GetByID(ctx, id)
	if errors.Is(err, flow.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get", err.Error())
		return
	}
	if err := a.authorizeOrg(ctx, out.OrganizationID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
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
	ctx := r.Context()
	if a.orgID(ctx) == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	prev, err := a.FlowService.GetByID(ctx, id)
	if errors.Is(err, flow.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	if err := a.authorizeOrg(ctx, prev.OrganizationID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	actorID := a.userID(ctx)
	if err := a.FlowService.SoftDelete(ctx, id, actorID); err != nil {
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
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	prev, err := a.FlowService.GetByID(ctx, id)
	if errors.Is(err, flow.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	if err := a.authorizeOrg(ctx, prev.OrganizationID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	userID := a.userID(ctx)
	// issue-26.6 backpressure
	if a.Backpressure != nil {
		if err := a.Backpressure.CheckQueue(ctx,
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
	res, runErr := a.FlowRunner.Run(ctx, flowrunner.RunInput{
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
	ctx := r.Context()
	if a.orgID(ctx) == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	if a.FlowRunner == nil {
		writeError(w, http.StatusServiceUnavailable, "runner_disabled", "")
		return
	}
	f, err := a.FlowService.GetByID(ctx, id)
	if errors.Is(err, flow.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	if err := a.authorizeOrg(ctx, f.OrganizationID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	var b runFlowBody
	_ = decodeJSON(r, &b)
	plan, err := a.FlowRunner.DryRun(ctx, id, b.Inputs)
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
	ctx := r.Context()
	if a.orgID(ctx) == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	prev, err := a.FlowService.GetByID(ctx, id)
	if errors.Is(err, flow.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	if err := a.authorizeOrg(ctx, prev.OrganizationID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
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

	actorID := a.userID(ctx)
	vs := &flow.VersioningStore{Pool: a.FlowService.Pool}
	v, err := vs.NewVersion(ctx, id, def, hash, b.Note, &actorID)
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

// PUT /api/v1/flows/{id} — issue-09.1: full update del flow current con
// optimistic locking opcional vía header If-Unmodified-Since (RFC 7232).
// Mismatch → 412 Precondition Failed.
func (a *API) replaceFlow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	ctx := r.Context()
	if a.orgID(ctx) == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	prev, err := a.FlowService.GetByID(ctx, id)
	if errors.Is(err, flow.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	if err := a.authorizeOrg(ctx, prev.OrganizationID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}

	var b createFlowBody
	if err := decodeJSON(r, &b); err != nil || b.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "name and spec required")
		return
	}
	actorID := a.userID(ctx)
	in := flow.UpdateInput{
		Name: &b.Name, Description: &b.Description, Spec: &b.Spec, ActorID: actorID,
	}
	if ius := r.Header.Get("If-Unmodified-Since"); ius != "" {
		ts, err := http.ParseTime(ius)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", "invalid If-Unmodified-Since")
			return
		}
		// Comparación a resolución de segundo (HTTP date no tiene sub-segundo):
		// usamos el updated_at real si coincide truncado.
		if prev.UpdatedAt.UTC().Truncate(time.Second).Equal(ts.UTC().Truncate(time.Second)) {
			expected := prev.UpdatedAt
			in.ExpectedUpdatedAt = &expected
		} else {
			writeError(w, http.StatusPreconditionFailed, "precondition_failed",
				"flow was modified since the provided timestamp")
			return
		}
	}
	f, err := a.FlowService.Update(ctx, id, in)
	if errors.Is(err, flow.ErrUpdateConflict) {
		writeError(w, http.StatusPreconditionFailed, "precondition_failed", "concurrent modification")
		return
	}
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, f)
}

type flowExport struct {
	Slug        string    `json:"slug" yaml:"slug"`
	Name        string    `json:"name" yaml:"name"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
	Spec        flow.Spec `json:"spec" yaml:"spec"`
}

// response-shape-lint:allow export endpoint — responde el documento crudo (json/yaml), no envelope
//
// GET /api/v1/flows/{id}/export?format=yaml|json — issue-09.1
func (a *API) exportFlow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	ctx := r.Context()
	if a.orgID(ctx) == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	f, err := a.FlowService.GetByID(ctx, id)
	if errors.Is(err, flow.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	if err := a.authorizeOrg(ctx, f.OrganizationID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	doc := flowExport{Slug: f.Slug, Name: f.Name, Description: f.Description, Spec: f.Spec}
	switch r.URL.Query().Get("format") {
	case "yaml":
		raw, err := yaml.Marshal(doc)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "export", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/yaml")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.yaml", f.Slug))
		// HU-28.5: best effort; headers ya escritos. Loggeamos write failures.
		if _, err := w.Write(raw); err != nil {
			slog.Warn("flow export write failed", "error", err, "format", "yaml", "slug", f.Slug)
		}
	case "", "json":
		raw, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "export", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.json", f.Slug))
		if _, err := w.Write(raw); err != nil {
			slog.Warn("flow export write failed", "error", err, "format", "json", "slug", f.Slug)
		}
	default:
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "format must be yaml or json")
	}
}

// GET /api/v1/flows/{id}/parents — issue-09.5: flows que usan este flow
// como sub_flow.
func (a *API) listFlowParents(w http.ResponseWriter, r *http.Request) {
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
	f, err := a.FlowService.GetByID(r.Context(), id)
	if errors.Is(err, flow.ErrNotFound) || (err == nil && f.OrganizationID.String() != p.OrganizationID) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	orgID, _ := uuid.Parse(p.OrganizationID)
	parents, err := a.FlowService.ListParents(r.Context(), orgID, f.Slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "parents", err.Error())
		return
	}
	out := make([]map[string]any, 0, len(parents))
	for _, pf := range parents {
		out = append(out, map[string]any{"id": pf.ID, "slug": pf.Slug, "name": pf.Name})
	}
	writeData(w, http.StatusOK, out)
}

const maxImportBytes = 1 << 20 // 1MB (issue-09.1)

// POST /api/v1/flows/import — issue-09.1. Acepta JSON o YAML según
// Content-Type; slug duplicado → 409.
func (a *API) importFlow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	actorID := a.userID(ctx)
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxImportBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_body", err.Error())
		return
	}
	if len(raw) > maxImportBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "import max 1MB")
		return
	}

	var doc flowExport
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "yaml") {
		err = yaml.Unmarshal(raw, &doc)
	} else {
		err = json.Unmarshal(raw, &doc)
	}
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "invalid document: "+err.Error())
		return
	}

	f, err := a.FlowService.Create(ctx, flow.CreateInput{
		OrganizationID: orgID, Slug: doc.Slug, Name: doc.Name,
		Description: doc.Description, Spec: doc.Spec, ActorID: actorID,
	})
	if errors.Is(err, flow.ErrSlugTaken) {
		writeError(w, http.StatusConflict, "slug_taken", "a flow with that slug already exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	}
	w.Header().Set("Location", "/api/v1/flows/"+f.ID.String())
	writeData(w, http.StatusCreated, f)
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
	ctx := r.Context()
	orgID := a.orgID(ctx)
	if orgID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	run, err := a.FlowService.GetRun(ctx, id)
	if errors.Is(err, flow.ErrRunNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return
	}
	if err := a.authorizeOrg(ctx, run.OrganizationID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}

	var b signalRunBody
	if err := decodeJSON(r, &b); err != nil || b.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "name required")
		return
	}

	signals := &flow.SignalStore{Pool: a.FlowService.Pool}
	pending, err := signals.HasPendingExpectation(ctx, id, b.Name)
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
	sig, err := signals.Send(ctx, id, stepKey, b.Name, b.Payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "signal_send", err.Error())
		return
	}
	if a.Audit != nil {
		actorID := a.userID(ctx)
		audit.RecordOrLog(ctx, a.Audit, audit.Event{
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

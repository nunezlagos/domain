// Handlers de flow runs (issue-09.3): consulta, control de lifecycle y
// stream SSE de progreso (issue-09.10 hb-004 consumidor).
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/flow"
)

// context5s acota cada WaitForNotification para intercalar snapshots de status.
func context5s(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 5*time.Second)
}

// loadRunForOrg carga el run con guard anti-enumeration por org.
func (a *API) loadRunForOrg(w http.ResponseWriter, r *http.Request) (*flow.RunRow, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "")
		return nil, false
	}
	p, _ := principal(r)
	if p == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return nil, false
	}
	run, err := a.FlowService.GetRun(r.Context(), id)
	if errors.Is(err, flow.ErrRunNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return nil, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup", err.Error())
		return nil, false
	}
	return run, true
}

// GET /api/v1/flow-runs/{id} — estado del run + steps con progreso.
func (a *API) getFlowRun(w http.ResponseWriter, r *http.Request) {
	run, ok := a.loadRunForOrg(w, r)
	if !ok {
		return
	}
	steps, err := a.FlowService.GetRunSteps(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "steps", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"id":           run.ID,
		"flow_id":      run.FlowID,
		"status":       run.Status,
		"error":        run.Error,
		"trigger_type": run.TriggerType,
		"started_at":   run.StartedAt,
		"finished_at":  run.FinishedAt,
		"steps":        steps,
	})
}

// POST /api/v1/flow-runs/{id}/pause
func (a *API) pauseFlowRun(w http.ResponseWriter, r *http.Request) {
	run, ok := a.loadRunForOrg(w, r)
	if !ok {
		return
	}
	if err := a.FlowService.PauseRun(r.Context(), run.ID); err != nil {
		writeError(w, http.StatusConflict, "invalid_transition", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{"id": run.ID, "status": "paused"})
}

// POST /api/v1/flow-runs/{id}/resume
func (a *API) resumeFlowRun(w http.ResponseWriter, r *http.Request) {
	run, ok := a.loadRunForOrg(w, r)
	if !ok {
		return
	}
	if err := a.FlowService.ResumeRun(r.Context(), run.ID); err != nil {
		writeError(w, http.StatusConflict, "invalid_transition", err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]any{"id": run.ID, "status": "running"})
}

// POST /api/v1/flow-runs/{id}/cancel
func (a *API) cancelFlowRun(w http.ResponseWriter, r *http.Request) {
	run, ok := a.loadRunForOrg(w, r)
	if !ok {
		return
	}
	if err := a.FlowService.CancelRun(r.Context(), run.ID); err != nil {
		writeError(w, http.StatusConflict, "invalid_transition", err.Error())
		return
	}

	if a.FlowRunner != nil {
		_ = a.FlowRunner.CancelRun(run.ID)
	}
	writeData(w, http.StatusOK, map[string]any{"id": run.ID, "status": "cancelled"})
}

// response-shape-lint:allow SSE stream — escribe text/event-stream, no JSON envelope
//
// GET /api/v1/flow-runs/{id}/stream — SSE con eventos de progreso
// (flow_step_progress NOTIFY, issue-09.10) + snapshot de status. Cierra al
// llegar el run a estado terminal o al desconectar el cliente.
func (a *API) streamFlowRun(w http.ResponseWriter, r *http.Request) {
	run, ok := a.loadRunForOrg(w, r)
	if !ok {
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream_unsupported", "")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx := r.Context()
	send := func(event string, payload any) {
		raw, _ := json.Marshal(payload)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw)
		fl.Flush()
	}
	send("status", map[string]any{"id": run.ID, "status": run.Status})

	terminal := map[string]bool{"completed": true, "failed": true, "cancelled": true}
	if terminal[run.Status] {
		return
	}



	conn, err := a.FlowService.Pool.Acquire(ctx)
	if err != nil {
		send("error", map[string]any{"code": "listen_unavailable"})
		return
	}
	defer func() {
		_ = conn.Conn().Close(r.Context())
		conn.Release()
	}()
	if _, err := conn.Exec(ctx, "LISTEN "+flow.ProgressChannel); err != nil {
		send("error", map[string]any{"code": "listen_failed"})
		return
	}

	for {
		nctx, cancel := context5s(ctx)
		n, werr := conn.Conn().WaitForNotification(nctx)
		cancel()
		if ctx.Err() != nil {
			return // cliente desconectado
		}
		if werr == nil {
			var ev flow.ProgressEvent
			if json.Unmarshal([]byte(n.Payload), &ev) == nil && ev.FlowRunID == run.ID {
				send("progress", ev)
			}
			continue
		}

		cur, err := a.FlowService.GetRun(ctx, run.ID)
		if err != nil {
			return
		}
		send("status", map[string]any{"id": cur.ID, "status": cur.Status, "error": cur.Error})
		if terminal[cur.Status] {
			return
		}
	}
}

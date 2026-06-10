// HU-16.2 web-run-visualization — página de visualización de runs con
// timeline + streaming live de chunks vía SSE.
//
// Pattern: server-rendered HTML + EventSource del browser conectado al
// endpoint streaming del runner (HU-11.3). Sin SPA, sin react-flow.
package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RunViz maneja /admin/runs*.
type RunViz struct {
	Pool      *pgxpool.Pool
	AuthCheck func(*http.Request) bool
}

// Register monta las rutas.
func (a *RunViz) Register(mux *http.ServeMux) {
	mux.HandleFunc("/admin/runs", a.list)
	mux.HandleFunc("/admin/runs/", a.list) // detail uses ?id=
	mux.HandleFunc("/admin/api/runs", a.apiList)
	mux.HandleFunc("/admin/api/runs/", a.apiDetail)
}

func (a *RunViz) checkAuth(r *http.Request) bool {
	if a.AuthCheck == nil {
		return true
	}
	return a.AuthCheck(r)
}

func (a *RunViz) list(w http.ResponseWriter, r *http.Request) {
	if !a.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	data, err := assets.ReadFile("assets/runs.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// RunRow lista compacta.
type RunRow struct {
	Type      string     `json:"type"`
	ID        uuid.UUID  `json:"id"`
	Status    string     `json:"status"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

func (a *RunViz) apiList(w http.ResponseWriter, r *http.Request) {
	if !a.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := a.Pool.Query(ctx, `
		SELECT type, id, status, started_at, ended_at FROM (
		  SELECT 'agent' AS type, id, status, started_at, ended_at FROM agent_runs
		  UNION ALL
		  SELECT 'flow', id, status, started_at, ended_at FROM flow_runs
		) u
		ORDER BY started_at DESC LIMIT 100`)
	if err != nil {
		http.Error(w, "list: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	out := make([]RunRow, 0, 100)
	for rows.Next() {
		var r RunRow
		if err := rows.Scan(&r.Type, &r.ID, &r.Status, &r.StartedAt, &r.EndedAt); err != nil {
			continue
		}
		out = append(out, r)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// apiDetail: /admin/api/runs/{type}/{id}
// Para runs flow incluye steps + snapshots (HU-09.11) + heartbeats (HU-09.10).
func (a *RunViz) apiDetail(w http.ResponseWriter, r *http.Request) {
	if !a.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	path := r.URL.Path[len("/admin/api/runs/"):]
	parts := splitPath(path)
	if len(parts) < 2 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	kind, idStr := parts[0], parts[1]
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid uuid", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if kind == "flow" {
		out, err := a.flowDetail(ctx, id)
		if err != nil {
			http.Error(w, "detail: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
		return
	}
	// agent: timeline de logs
	logs := make([]map[string]any, 0)
	rows, err := a.Pool.Query(ctx, `
		SELECT level, message, COALESCE(metadata, '{}'::jsonb), created_at
		FROM agent_run_logs WHERE agent_run_id = $1 ORDER BY created_at ASC LIMIT 500`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var level, msg string
			var meta string
			var at time.Time
			if err := rows.Scan(&level, &msg, &meta, &at); err == nil {
				logs = append(logs, map[string]any{
					"level": level, "message": msg, "metadata": meta, "at": at,
				})
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "agent", "id": id, "logs": logs,
	})
}

type FlowStepDetail struct {
	ID          uuid.UUID  `json:"id"`
	StepKey     string     `json:"step_key"`
	Status      string     `json:"status"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Heartbeat   *time.Time `json:"last_heartbeat,omitempty"`
	Error       *string    `json:"error,omitempty"`
}

type FlowDetail struct {
	Type  string           `json:"type"`
	ID    uuid.UUID        `json:"id"`
	Steps []FlowStepDetail `json:"steps"`
}

func (a *RunViz) flowDetail(ctx context.Context, runID uuid.UUID) (*FlowDetail, error) {
	out := &FlowDetail{Type: "flow", ID: runID}
	rows, err := a.Pool.Query(ctx, `
		SELECT id, COALESCE(step_key, ''), status, started_at, completed_at,
		       last_heartbeat_at, error
		FROM flow_run_steps WHERE flow_run_id = $1
		ORDER BY COALESCE(started_at, created_at) ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("steps: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s FlowStepDetail
		if err := rows.Scan(&s.ID, &s.StepKey, &s.Status, &s.StartedAt,
			&s.CompletedAt, &s.Heartbeat, &s.Error); err != nil {
			continue
		}
		out.Steps = append(out.Steps, s)
	}
	return out, nil
}

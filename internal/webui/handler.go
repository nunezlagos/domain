// Package webui — HU-16.1 web dashboard MVP.
//
// Sirve HTML estático embedded + endpoints JSON que la página consume vía
// fetch. NO es una SPA completa: es un dashboard read-only minimal con
// vanilla JS que muestra:
//   - Counts de entidades (orgs, projects, observations, agents, flows)
//   - Últimas 20 runs en cualquier estado
//   - Tail de errors recientes
//
// Para SPA full (run viz interactiva, flow editor visual, admin CRUD de
// skills/memories) ver HU-16.2 a HU-16.5 — esas requieren stack frontend
// (React/Vue + build pipeline) fuera del alcance de este package Go.
package webui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed assets/*
var assets embed.FS

// Handler sirve / y /api/dashboard/* endpoints.
type Handler struct {
	Pool *pgxpool.Pool
	// AuthCheck opcional: si seteado, retorna true si la request tiene auth válida.
	// Si nil, NO autentica (asume reverse-proxy o uso dev local).
	AuthCheck func(*http.Request) bool
}

// Register monta las rutas en el mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/admin/", h.serveAdmin)
	mux.HandleFunc("/admin/api/stats", h.serveStats)
	mux.HandleFunc("/admin/api/recent-runs", h.serveRecentRuns)
}

func (h *Handler) serveAdmin(w http.ResponseWriter, r *http.Request) {
	if !h.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/admin")
	if path == "" || path == "/" {
		path = "/index.html"
	}
	data, err := assets.ReadFile("assets" + path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := "text/html; charset=utf-8"
	switch {
	case strings.HasSuffix(path, ".js"):
		contentType = "application/javascript"
	case strings.HasSuffix(path, ".css"):
		contentType = "text/css"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

// Stats: counts agregados.
type Stats struct {
	Orgs            int `json:"orgs"`
	Projects        int `json:"projects"`
	Users           int `json:"users"`
	Observations    int `json:"observations"`
	KnowledgeDocs   int `json:"knowledge_docs"`
	Agents          int `json:"agents"`
	Flows           int `json:"flows"`
	Skills          int `json:"skills"`
	AgentRunsToday  int `json:"agent_runs_today"`
	FlowRunsToday   int `json:"flow_runs_today"`
}

func (h *Handler) serveStats(w http.ResponseWriter, r *http.Request) {
	if !h.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	s, err := h.gatherStats(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("stats: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s)
}

func (h *Handler) gatherStats(ctx context.Context) (*Stats, error) {
	var s Stats
	queries := []struct {
		dest *int
		sql  string
	}{
		{&s.Orgs, `SELECT COUNT(*) FROM organizations`},
		{&s.Projects, `SELECT COUNT(*) FROM projects WHERE deleted_at IS NULL`},
		{&s.Users, `SELECT COUNT(*) FROM users`},
		{&s.Observations, `SELECT COUNT(*) FROM observations WHERE deleted_at IS NULL`},
		{&s.KnowledgeDocs, `SELECT COUNT(*) FROM knowledge_docs WHERE deleted_at IS NULL`},
		{&s.Agents, `SELECT COUNT(*) FROM agents`},
		{&s.Flows, `SELECT COUNT(*) FROM flows`},
		{&s.Skills, `SELECT COUNT(*) FROM skills`},
		{&s.AgentRunsToday, `SELECT COUNT(*) FROM agent_runs WHERE created_at >= CURRENT_DATE`},
		{&s.FlowRunsToday, `SELECT COUNT(*) FROM flow_runs WHERE created_at >= CURRENT_DATE`},
	}
	for _, q := range queries {
		if err := h.Pool.QueryRow(ctx, q.sql).Scan(q.dest); err != nil {
			// silencioso por query — tabla puede no existir en ambiente parcial
			*q.dest = -1
		}
	}
	return &s, nil
}

type RecentRun struct {
	Type      string    `json:"type"`       // agent | flow
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	Duration  string    `json:"duration,omitempty"`
}

func (h *Handler) serveRecentRuns(w http.ResponseWriter, r *http.Request) {
	if !h.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.Pool.Query(ctx, `
		SELECT type, id, status, started_at, COALESCE(ended_at - started_at, INTERVAL '0')
		FROM (
		  SELECT 'agent' AS type, id::text, status, started_at, ended_at FROM agent_runs
		  UNION ALL
		  SELECT 'flow', id::text, status, started_at, ended_at FROM flow_runs
		) u
		ORDER BY started_at DESC LIMIT 20`)
	if err != nil {
		http.Error(w, fmt.Sprintf("runs: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var out []RecentRun
	for rows.Next() {
		var rr RecentRun
		var dur time.Duration
		if err := rows.Scan(&rr.Type, &rr.ID, &rr.Status, &rr.StartedAt, &dur); err != nil {
			continue
		}
		if dur > 0 {
			rr.Duration = dur.String()
		}
		out = append(out, rr)
	}
	writeJSON(w, out)
}

func (h *Handler) checkAuth(r *http.Request) bool {
	if h.AuthCheck == nil {
		return true
	}
	return h.AuthCheck(r)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

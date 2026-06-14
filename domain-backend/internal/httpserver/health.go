// Package httpserver — handlers HTTP del server (issue-01.3 health-version).
// Movido fuera de cmd/domain para ser testeable y reusable.
package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ShuttingDown es un flag global que se setea en true al recibir SIGTERM.
// Una vez true, ReadyHandler responde 503 para que ELB/K8s deje de rutear
// nuevos requests (issue-26.4 escenario 5).
var ShuttingDown atomic.Bool

// VersionInfo build-time metadata.
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"built"`
}

// HealthHandler responde /health.
// issue-01.3: status + version + uptime. DB ping en /health/ready.
type HealthHandler struct {
	Info      VersionInfo
	StartedAt time.Time
	Pool      *pgxpool.Pool // opcional; si nil, ready siempre 200
}

// ServeHTTP responde JSON con status liveness.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	uptime := time.Since(h.StartedAt).Round(time.Second).String()
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"version": h.Info.Version,
		"commit":  h.Info.Commit,
		"built":   h.Info.BuildTime,
		"uptime":  uptime,
	})
}

// ReadyHandler responde /health/ready con DB ping.
type ReadyHandler struct {
	Pool *pgxpool.Pool
}

// ServeHTTP — 200 si pool nil o ping OK; 503 si ping falla o ShuttingDown=true.
func (h *ReadyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if ShuttingDown.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{"ready": false, "reason": "shutting_down"})
		return
	}
	if h.Pool == nil {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"ready": true, "db": "skipped"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.Pool.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{"ready": false, "db": "ping_failed"})
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"ready": true, "db": "ok"})
}

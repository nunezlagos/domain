// Package runtimeconfig — HU-27.3 hot-reload de configs sin restart.
//
// La fuente de verdad es la tabla runtime_configs (Postgres). En memoria
// se mantiene un atomic snapshot. Refresh por SIGHUP o cron 30s.
//
// Subset de configs hot-reloadables (whitelist en HotReloadable map). El
// resto requiere restart (database_url, s3_endpoint, etc.).
package runtimeconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HotReloadable whitelist — solo keys aquí se pueden cambiar sin restart.
// Modificar requiere bump version + audit.
var HotReloadable = map[string]bool{
	"log_level":                       true,
	"http_request_timeout_seconds":    true,
	"llm_default_timeout_seconds":     true,
	"otel_sample_ratio":               true,
	"outbound_dispatcher_batch_size":  true,
	"metrics_enabled":                 true,
}

// Snapshot inmutable de configs aplicada.
type Snapshot struct {
	LogLevel                       string
	HTTPRequestTimeoutSeconds      int
	LLMDefaultTimeoutSeconds       int
	OTELSampleRatio                float64
	OutboundDispatcherBatchSize    int
	MetricsEnabled                 bool
	UpdatedAt                      time.Time
}

// Registry mantiene Snapshot atómicamente accesible cross-goroutine.
type Registry struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
	snap   atomic.Pointer[Snapshot]
}

// Update persiste un valor en la tabla y refresca el snapshot en memoria.
func (r *Registry) Update(ctx context.Context, key string, value json.RawMessage) error {
	tag, err := r.Pool.Exec(ctx,
		`UPDATE runtime_configs SET value = $2, updated_at = NOW() WHERE key = $1`, key, value)
	if err != nil {
		return fmt.Errorf("update runtime_config %s: %w", key, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("key %q not found", key)
	}
	return nil
}

// ValueJSON retorna el valor de una key como tipo json-friendly.
func ValueJSON(s *Snapshot, key string) (any, error) {
	switch key {
	case "log_level":
		return s.LogLevel, nil
	case "http_request_timeout_seconds":
		return s.HTTPRequestTimeoutSeconds, nil
	case "llm_default_timeout_seconds":
		return s.LLMDefaultTimeoutSeconds, nil
	case "otel_sample_ratio":
		return s.OTELSampleRatio, nil
	case "outbound_dispatcher_batch_size":
		return s.OutboundDispatcherBatchSize, nil
	case "metrics_enabled":
		return s.MetricsEnabled, nil
	default:
		return nil, fmt.Errorf("unknown key: %s", key)
	}
}

// Defaults retorna un snapshot con valores por defecto (si la DB falla).
func Defaults() *Snapshot {
	return &Snapshot{
		LogLevel:                    "info",
		HTTPRequestTimeoutSeconds:   30,
		LLMDefaultTimeoutSeconds:    60,
		OTELSampleRatio:             0.1,
		OutboundDispatcherBatchSize: 50,
		MetricsEnabled:              true,
	}
}

// Current retorna el último snapshot aplicado. Nunca nil.
func (r *Registry) Current() *Snapshot {
	s := r.snap.Load()
	if s == nil {
		return Defaults()
	}
	return s
}

// Refresh consulta la tabla y aplica los valores al snapshot atómico.
// Se llama al boot, por SIGHUP, o por cron.
func (r *Registry) Refresh(ctx context.Context) error {
	rows, err := r.Pool.Query(ctx,
		`SELECT key, value FROM runtime_configs WHERE is_hot_reloadable = TRUE`)
	if err != nil {
		return fmt.Errorf("query runtime_configs: %w", err)
	}
	defer rows.Close()
	new := *Defaults()
	new.UpdatedAt = time.Now()
	for rows.Next() {
		var key string
		var raw []byte
		if err := rows.Scan(&key, &raw); err != nil {
			return err
		}
		if err := ApplyValue(&new, key, raw); err != nil && r.Logger != nil {
			r.Logger.WarnContext(ctx, "runtime_config invalid value",
				slog.String("key", key), slog.String("error", err.Error()))
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	r.snap.Store(&new)
	if r.Logger != nil {
		r.Logger.InfoContext(ctx, "runtime config refreshed",
			slog.String("log_level", new.LogLevel),
			slog.Int("http_timeout", new.HTTPRequestTimeoutSeconds))
	}
	return nil
}

// ApplyValue aplica un valor a la struct según el key.
func ApplyValue(s *Snapshot, key string, raw []byte) error {
	switch key {
	case "log_level":
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		if v != "debug" && v != "info" && v != "warn" && v != "error" {
			return errors.New("log_level must be debug|info|warn|error")
		}
		s.LogLevel = v
	case "http_request_timeout_seconds":
		var v int
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		if v < 1 || v > 600 {
			return errors.New("http_request_timeout_seconds out of range [1,600]")
		}
		s.HTTPRequestTimeoutSeconds = v
	case "llm_default_timeout_seconds":
		var v int
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		if v < 1 || v > 600 {
			return errors.New("out of range")
		}
		s.LLMDefaultTimeoutSeconds = v
	case "otel_sample_ratio":
		var v float64
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		if v < 0 || v > 1 {
			return errors.New("otel_sample_ratio must be in [0,1]")
		}
		s.OTELSampleRatio = v
	case "outbound_dispatcher_batch_size":
		var v int
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		if v < 1 || v > 10000 {
			return errors.New("out of range")
		}
		s.OutboundDispatcherBatchSize = v
	case "metrics_enabled":
		var v bool
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		s.MetricsEnabled = v
	default:
		return fmt.Errorf("unknown key: %s", key)
	}
	return nil
}

// RunPolling refresca cada interval hasta ctx.Done().
// Es un fallback al SIGHUP — el ideal es push via LISTEN/NOTIFY pero
// incompatible con PgBouncer transaction-pool.
func (r *Registry) RunPolling(ctx context.Context, interval time.Duration) {
	if interval == 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.Refresh(ctx); err != nil && r.Logger != nil {
				r.Logger.WarnContext(ctx, "runtime config refresh failed",
					slog.String("error", err.Error()))
			}
		}
	}
}

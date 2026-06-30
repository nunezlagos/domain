// Package observability: este archivo cubre el self-healing acotado. Si un
// error matchea un known_error recoverable con auto_heal_action != none,
// ejecuta la accion con backoff (1s/5s/30s) hasta 3 intentos.
//
// issue-53.9 early-error-reporting.
package observability

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HealAction son las acciones de auto-recuperacion soportadas.
const (
	HealRetry         = "retry"
	HealClearCache    = "clear_cache"
	HealRestartWorker = "restart_worker"
	HealNone          = "none"
)

// selfHealMaxAttempts acota los reintentos para evitar loops infinitos.
const selfHealMaxAttempts = 3

// KnownError es la entrada de known_errors con su remediacion.
type KnownError struct {
	Fingerprint    []byte
	Name           string
	Recoverable    bool
	AutoHealAction string
	ActionParams   map[string]string
}

// KnownErrorStore resuelve un fingerprint contra known_errors.
type KnownErrorStore interface {
	LookupKnownError(ctx context.Context, fingerprint []byte) (*KnownError, bool, error)
}

// HealFunc ejecuta una accion de recuperacion concreta.
type HealFunc func(ctx context.Context, params map[string]string) error

// SelfHealer dispara acciones de recuperacion para errores conocidos.
type SelfHealer struct {
	store   KnownErrorStore
	actions map[string]HealFunc
	logger  *slog.Logger
	sleep   func(time.Duration)
}

// NewSelfHealer construye el healer con el registry de acciones vacio.
// El wiring registra las HealFunc reales (retry/clear_cache/restart_worker).
func NewSelfHealer(store KnownErrorStore, logger *slog.Logger) *SelfHealer {
	if logger == nil {
		logger = slog.Default()
	}
	return &SelfHealer{
		store:   store,
		actions: make(map[string]HealFunc),
		logger:  logger,
		sleep:   time.Sleep,
	}
}

// Register asocia una accion a su implementacion.
func (h *SelfHealer) Register(action string, fn HealFunc) { h.actions[action] = fn }

// Heal intenta recuperar el evento si matchea un known_error accionable.
// No-op si el fingerprint no es conocido, no es recoverable o accion=none.
func (h *SelfHealer) Heal(ctx context.Context, e ErrorEvent) {
	ke, ok, err := h.store.LookupKnownError(ctx, e.Fingerprint)
	if err != nil {
		h.logger.Warn("known error lookup failed", slog.String("error", err.Error()))
		return
	}
	if !ok || ke == nil || !ke.Recoverable || ke.AutoHealAction == HealNone {
		return
	}
	fn := h.actions[ke.AutoHealAction]
	if fn == nil {
		h.logger.Warn("no heal action registered", slog.String("action", ke.AutoHealAction))
		return
	}
	h.runWithBackoff(ctx, ke, fn)
}

// runWithBackoff ejecuta fn hasta selfHealMaxAttempts; para al primer exito.
func (h *SelfHealer) runWithBackoff(ctx context.Context, ke *KnownError, fn HealFunc) {
	for attempt := 1; attempt <= selfHealMaxAttempts; attempt++ {
		if attempt > 1 {
			h.sleep(backoffFor(attempt - 1))
		}
		if err := fn(ctx, ke.ActionParams); err == nil {
			h.logger.Info("self-heal succeeded",
				slog.String("known_error", ke.Name),
				slog.String("action", ke.AutoHealAction),
				slog.Int("attempt", attempt))
			return
		}
	}
	h.logger.Error("self-heal failed after max attempts",
		slog.String("known_error", ke.Name),
		slog.String("action", ke.AutoHealAction),
		slog.Int("attempts", selfHealMaxAttempts))
}

// PGKnownErrorStore resuelve known_errors por fingerprint. Pool nullable; setear via SetPool.
type PGKnownErrorStore struct {
	Pool *pgxpool.Pool
}

// SetPool setea el pool (post-init).
func (s *PGKnownErrorStore) SetPool(p *pgxpool.Pool) { s.Pool = p }

// LookupKnownError busca el fingerprint. (nil,false,nil) si no existe.
func (s *PGKnownErrorStore) LookupKnownError(ctx context.Context, fingerprint []byte) (*KnownError, bool, error) {
	if s.Pool == nil {
		return nil, false, ErrStoreNotReady
	}
	var ke KnownError
	var params []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT fingerprint, name, recoverable, auto_heal_action, action_params
		FROM known_errors WHERE fingerprint = $1
	`, fingerprint).Scan(&ke.Fingerprint, &ke.Name, &ke.Recoverable, &ke.AutoHealAction, &params)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &ke.ActionParams)
	}
	return &ke, true, nil
}

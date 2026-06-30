// Package observability: este archivo cubre el motor de alertas. Evalua cada
// ErrorEvent contra los AlertConfig cargados y, si matchea categoria + severidad
// minima y no esta throttled, dispara el canal correspondiente.
//
// issue-53.9 early-error-reporting.
package observability

import (
	"context"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"
)

// AlertConfig describe cuando y por donde alertar. Espejo de la tabla alert_configs.
type AlertConfig struct {
	Category        Category
	SeverityMin     string
	Channel         string
	ChannelConfig   map[string]string
	ThrottleSeconds int
}

// Notifier envia una alerta por un canal concreto (webhook, ntfy, email...).
type Notifier interface {
	Send(ctx context.Context, e ErrorEvent, cfg AlertConfig) error
}

// severityRank ordena las severidades para comparar contra severity_min.
var severityRank = map[string]int{
	"debug": 0, "info": 1, "warn": 2, "error": 3, "critical": 4,
}

// AlertEngine evalua eventos contra configs y dispara canales con throttle.
type AlertEngine struct {
	configs   []AlertConfig
	notifiers map[string]Notifier
	logger    *slog.Logger
	now       func() time.Time

	mu       sync.Mutex
	lastSent map[string]time.Time
}

// NewAlertEngine construye el motor. logger nil -> slog.Default().
func NewAlertEngine(configs []AlertConfig, notifiers map[string]Notifier, logger *slog.Logger) *AlertEngine {
	if logger == nil {
		logger = slog.Default()
	}
	return &AlertEngine{
		configs:   configs,
		notifiers: notifiers,
		logger:    logger,
		now:       time.Now,
		lastSent:  make(map[string]time.Time),
	}
}

// Evaluate dispara las alertas que matchean el evento. Apta como AlertHook.
func (ae *AlertEngine) Evaluate(ctx context.Context, e ErrorEvent) {
	for _, cfg := range ae.configs {
		if !matchesConfig(cfg, e) || ae.throttled(cfg, e) {
			continue
		}
		n := ae.notifiers[cfg.Channel]
		if n == nil {
			continue
		}
		if err := n.Send(ctx, e, cfg); err != nil {
			ae.logger.Warn("alert send failed",
				slog.String("channel", cfg.Channel),
				slog.String("category", string(e.Category)),
				slog.String("error", err.Error()))
			continue
		}
		ae.markSent(cfg, e)
	}
}

// matchesConfig: misma categoria y severidad del evento >= severity_min.
func matchesConfig(cfg AlertConfig, e ErrorEvent) bool {
	return cfg.Category == e.Category && severityRank[e.Severity] >= severityRank[cfg.SeverityMin]
}

// throttleKey aisla el throttle por canal + fingerprint del evento.
func throttleKey(cfg AlertConfig, e ErrorEvent) string {
	return cfg.Channel + ":" + hex.EncodeToString(e.Fingerprint)
}

func (ae *AlertEngine) throttled(cfg AlertConfig, e ErrorEvent) bool {
	if cfg.ThrottleSeconds <= 0 {
		return false
	}
	ae.mu.Lock()
	defer ae.mu.Unlock()
	last, ok := ae.lastSent[throttleKey(cfg, e)]
	if !ok {
		return false
	}
	return ae.now().Sub(last) < time.Duration(cfg.ThrottleSeconds)*time.Second
}

func (ae *AlertEngine) markSent(cfg AlertConfig, e ErrorEvent) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.lastSent[throttleKey(cfg, e)] = ae.now()
}

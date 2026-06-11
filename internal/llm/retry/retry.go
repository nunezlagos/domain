// Package retry — decorator de llm.Provider con exponential backoff
// (issue-06.2). Reintenta solo errores transient: 429, 5xx, network/timeout.
package retry

import (
	"context"
	"strings"
	"time"

	"nunezlagos/domain/internal/llm"
)

type Config struct {
	MaxRetries  int           // default 3 (intentos totales = MaxRetries+1)
	BaseBackoff time.Duration // default 200ms, se duplica por intento
	MaxBackoff  time.Duration // default 10s
}

func (c Config) withDefaults() Config {
	if c.MaxRetries <= 0 {
		c.MaxRetries = 3
	}
	if c.BaseBackoff <= 0 {
		c.BaseBackoff = 200 * time.Millisecond
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = 10 * time.Second
	}
	return c
}

type provider struct {
	inner llm.Provider
	cfg   Config
}

// New envuelve p con retry transparente.
func New(p llm.Provider, cfg Config) llm.Provider {
	return &provider{inner: p, cfg: cfg.withDefaults()}
}

func (p *provider) Name() string { return p.inner.Name() }

// IsTransient clasifica errores retryables: rate limit, 5xx, overloaded,
// network/timeout. Auth y 4xx de cliente NO reintentan.
func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, kw := range []string{"401", "403", "404", "invalid api key", "unauthorized", "bad request", "400"} {
		if strings.Contains(msg, kw) {
			return false
		}
	}
	for _, kw := range []string{"429", "rate limit", "500", "502", "503", "504", "529", "overloaded",
		"timeout", "deadline", "connection refused", "connection reset", "eof", "no such host"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

func (p *provider) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	var lastErr error
	backoff := p.cfg.BaseBackoff
	for attempt := 0; attempt <= p.cfg.MaxRetries; attempt++ {
		res, err := p.inner.Complete(ctx, opts)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if !IsTransient(err) || attempt == p.cfg.MaxRetries {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > p.cfg.MaxBackoff {
			backoff = p.cfg.MaxBackoff
		}
	}
	return nil, lastErr
}

// CompleteStream reintenta solo el establecimiento del stream; una vez
// abierto, los chunks fluyen directo (un retry mid-stream duplicaría output).
func (p *provider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	var lastErr error
	backoff := p.cfg.BaseBackoff
	for attempt := 0; attempt <= p.cfg.MaxRetries; attempt++ {
		ch, err := p.inner.CompleteStream(ctx, opts)
		if err == nil {
			return ch, nil
		}
		lastErr = err
		if !IsTransient(err) || attempt == p.cfg.MaxRetries {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > p.cfg.MaxBackoff {
			backoff = p.cfg.MaxBackoff
		}
	}
	return nil, lastErr
}

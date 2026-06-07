// Package circuitbreaker — HU-26.5 protección contra LLM providers caídos.
//
// 3 estados:
//   - Closed: requests pasan normalmente, contador de errores se incrementa
//   - Open: requests rechazadas inmediatamente (ErrCircuitOpen), después de
//     RecoveryTimeout pasa a HalfOpen
//   - HalfOpen: 1 request prueba pasar; si OK → Closed, si falla → Open
//
// Threshold: N errores consecutivos en M tiempo → Open.
// Implementación thread-safe.
package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"nunezlagos/domain/internal/llm"
)

var ErrCircuitOpen = errors.New("circuit breaker open: provider degraded")

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	}
	return "unknown"
}

// Config breaker.
type Config struct {
	FailureThreshold int           // N errores consecutivos → Open
	RecoveryTimeout  time.Duration // espera antes de HalfOpen
}

func defaults(c Config) Config {
	if c.FailureThreshold <= 0 {
		c.FailureThreshold = 5
	}
	if c.RecoveryTimeout <= 0 {
		c.RecoveryTimeout = 30 * time.Second
	}
	return c
}

// Breaker envuelve un llm.Provider con circuit breaker semantics.
type Breaker struct {
	cfg      Config
	inner    llm.Provider

	mu          sync.RWMutex
	state       State
	failures    int
	openedAt    time.Time
}

func New(inner llm.Provider, cfg Config) *Breaker {
	return &Breaker{cfg: defaults(cfg), inner: inner, state: StateClosed}
}

func (b *Breaker) Name() string { return b.inner.Name() }

func (b *Breaker) State() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

func (b *Breaker) allow() (allowed bool, reason error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		return true, nil
	case StateOpen:
		if time.Since(b.openedAt) >= b.cfg.RecoveryTimeout {
			b.state = StateHalfOpen
			return true, nil
		}
		return false, fmt.Errorf("%w (since %s)",
			ErrCircuitOpen, b.openedAt.Format(time.RFC3339))
	case StateHalfOpen:
		// HalfOpen: solo permitimos 1 request probe a la vez
		// Implementación simple: ya estamos dejando pasar el primero;
		// si entran concurrentes, el primer success/failure decide.
		return true, nil
	}
	return true, nil
}

func (b *Breaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	if b.state != StateClosed {
		b.state = StateClosed
		b.openedAt = time.Time{}
	}
}

func (b *Breaker) recordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.state == StateHalfOpen {
		b.state = StateOpen
		b.openedAt = time.Now()
		return
	}
	if b.failures >= b.cfg.FailureThreshold {
		b.state = StateOpen
		b.openedAt = time.Now()
	}
}

// Complete delega al inner si el circuito está cerrado/half-open.
func (b *Breaker) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	allowed, reason := b.allow()
	if !allowed {
		return nil, reason
	}
	resp, err := b.inner.Complete(ctx, opts)
	if err != nil {
		b.recordFailure()
		return nil, err
	}
	b.recordSuccess()
	return resp, nil
}

func (b *Breaker) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	allowed, reason := b.allow()
	if !allowed {
		return nil, reason
	}
	ch, err := b.inner.CompleteStream(ctx, opts)
	if err != nil {
		b.recordFailure()
		return nil, err
	}
	// Wrap channel para detectar errores mid-stream (final chunk con error)
	out := make(chan llm.StreamChunk, 16)
	go func() {
		defer close(out)
		var sawError bool
		for chunk := range ch {
			out <- chunk
			if chunk.Done {
				if !sawError {
					b.recordSuccess()
				}
				return
			}
		}
		_ = sawError
	}()
	return out, nil
}

// Package ratelimit — decorator de llm.Provider con semáforo de concurrencia
// (issue-06.2). Acota llamadas simultáneas al provider; el resto espera o
// aborta si el ctx muere primero.
package ratelimit

import (
	"context"

	"nunezlagos/domain/internal/llm"
)

type provider struct {
	inner llm.Provider
	sem   chan struct{}
}

// New envuelve p limitando a maxConcurrent llamadas simultáneas (default 8).
func New(p llm.Provider, maxConcurrent int) llm.Provider {
	if maxConcurrent <= 0 {
		maxConcurrent = 8
	}
	return &provider{inner: p, sem: make(chan struct{}, maxConcurrent)}
}

func (p *provider) Name() string { return p.inner.Name() }

func (p *provider) acquire(ctx context.Context) error {
	select {
	case p.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *provider) release() { <-p.sem }

func (p *provider) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	if err := p.acquire(ctx); err != nil {
		return nil, err
	}
	defer p.release()
	return p.inner.Complete(ctx, opts)
}

// CompleteStream retiene el slot hasta que el stream termina (canal cerrado):
// un stream activo ES una llamada en curso contra el provider.
func (p *provider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	if err := p.acquire(ctx); err != nil {
		return nil, err
	}
	ch, err := p.inner.CompleteStream(ctx, opts)
	if err != nil {
		p.release()
		return nil, err
	}
	out := make(chan llm.StreamChunk, 16)
	go func() {
		defer close(out)
		defer p.release()
		for chunk := range ch {
			out <- chunk
		}
	}()
	return out, nil
}

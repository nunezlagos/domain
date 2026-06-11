// Package budget — decorator de llm.Provider que enforcea TokenBudgetManager
// (issue-07.4): Check pre-llamada, Track post-respuesta y corte graceful de
// streams en modo truncate.
package budget

import (
	"context"
	"errors"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/tokens"
)

type provider struct {
	inner llm.Provider
	mgr   *tokens.TokenBudgetManager
}

// New envuelve p con budget enforcement. mgr nil → passthrough.
func New(p llm.Provider, mgr *tokens.TokenBudgetManager) llm.Provider {
	if mgr == nil {
		return p
	}
	return &provider{inner: p, mgr: mgr}
}

func (p *provider) Name() string { return p.inner.Name() }

// Complete: Check pre-llamada bloquea si el budget ya está agotado; el
// consumo real se trackea post-respuesta (la siguiente llamada bloquea).
func (p *provider) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	if err := p.mgr.Check(); err != nil {
		return nil, err
	}
	resp, err := p.inner.Complete(ctx, opts)
	if err != nil {
		return nil, err
	}
	// La respuesta ya se consumió: se entrega siempre; si esto agota el
	// budget, la SIGUIENTE llamada bloquea en Check.
	_ = p.mgr.Track(resp.Usage.TotalTokens)
	return resp, nil
}

// CompleteStream: trackea tokens estimados por chunk; en modo truncate corta
// el stream graceful (Done=true) al alcanzar el hard limit.
func (p *provider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	if err := p.mgr.Check(); err != nil {
		return nil, err
	}
	inner, err := p.inner.CompleteStream(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := make(chan llm.StreamChunk, 16)
	go func() {
		defer close(out)
		for chunk := range inner {
			if chunk.Done {
				if chunk.Usage != nil {
					// Ajuste final con el conteo real del provider.
					_ = p.mgr.Track(0)
				}
				out <- chunk
				return
			}
			trackErr := p.mgr.Track(tokens.Estimate(chunk.Delta))
			if errors.Is(trackErr, tokens.ErrBudgetTruncated) {
				// Corte graceful: entregar el chunk en curso + Done.
				out <- chunk
				st := p.mgr.State()
				out <- llm.StreamChunk{Done: true, Usage: &llm.Usage{
					CompletionTokens: st.TokensUsed, TotalTokens: st.TokensUsed,
				}}
				go drain(inner)
				return
			}
			if errors.Is(trackErr, tokens.ErrBudgetExceeded) {
				// Modo error: cerrar el stream sin Done limpio (el caller
				// detecta el corte por ausencia de Done + Check posterior).
				go drain(inner)
				return
			}
			out <- chunk
		}
	}()
	return out, nil
}

// drain evita goroutine leak del producer interno tras un corte.
func drain(ch <-chan llm.StreamChunk) {
	for range ch {
	}
}

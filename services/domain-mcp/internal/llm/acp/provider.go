// Package acp expone un llm.Provider respaldado por un agente ACP (opencode):
// cada Complete corre una sesión ACP one-shot vía internal/agentbridge/acp. Es
// el cerebro server-side por default del epic DOMAINSERV-62.
package acp

import (
	"context"
	"fmt"
	"strings"

	"nunezlagos/domain/internal/llm"
)

// runner corre un prompt one-shot y libera recursos. Lo satisface el Process de
// internal/agentbridge/acp; se inyecta para testear sin subproceso
type runner interface {
	Prompt(ctx context.Context, text string) (string, error)
	Close() error
}

type spawnFunc func(ctx context.Context) (runner, error)

// spawnHomeFunc spawnea el agente con un HOME especifico (selecciona el modelo
// via el opencode.json de ese HOME). Lo usa el path rolling.
type spawnHomeFunc func(ctx context.Context, home string) (runner, error)

// Provider es un llm.Provider respaldado por un agente ACP
type Provider struct {
	name      string
	spawn     spawnFunc
	roll      *roller
	spawnHome spawnHomeFunc
}

func (p *Provider) Name() string { return p.name }

// Complete levanta una sesión ACP, corre el prompt compuesto y devuelve el texto
func (p *Provider) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	if p.roll != nil {
		return p.completeRolling(ctx, opts)
	}
	r, err := p.spawn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acp spawn: %w", err)
	}
	defer func() { _ = r.Close() }()

	text, err := r.Prompt(ctx, composePrompt(opts))
	if err != nil {
		return nil, err
	}
	return &llm.Response{Content: text, Model: opts.Model, FinishReason: "stop"}, nil
}

// completeRolling elige el siguiente free model del roster y, ante error, lo
// pone en cooldown y reintenta con el siguiente (acotado a maxAttempts).
func (p *Provider) completeRolling(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	const maxAttempts = 2
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		model, home, ok := p.roll.next()
		if !ok {
			return nil, fmt.Errorf("acp rolling: roster de modelos free vacio")
		}
		r, err := p.spawnHome(ctx, home)
		if err != nil {
			p.roll.cooldownModel(model)
			lastErr = err
			continue
		}
		text, err := r.Prompt(ctx, composePrompt(opts))
		_ = r.Close()
		if err != nil {
			p.roll.cooldownModel(model)
			lastErr = err
			continue
		}
		return &llm.Response{Content: text, Model: model, FinishReason: "stop"}, nil
	}
	return nil, lastErr
}

// CompleteStream degrada a Complete y emite el resultado en un único chunk: el
// agente ACP one-shot no expone streaming incremental a este nivel
func (p *Provider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	resp, err := p.Complete(ctx, opts)
	if err != nil {
		return nil, err
	}
	ch := make(chan llm.StreamChunk, 1)
	ch <- llm.StreamChunk{Delta: resp.Content, Done: true, Usage: &resp.Usage}
	close(ch)
	return ch, nil
}

// composePrompt aplana SystemPrompt + Messages en un único texto para la sesión
func composePrompt(opts llm.CompletionOptions) string {
	var b strings.Builder
	if s := strings.TrimSpace(opts.SystemPrompt); s != "" {
		b.WriteString(s)
		b.WriteString("\n\n")
	}
	for _, m := range opts.Messages {
		if c := strings.TrimSpace(m.Content); c != "" {
			b.WriteString(c)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

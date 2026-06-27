// Package registry — pricing de modelos LLM (issue-06.4).
//
// REQ-42.3: la tabla model_registry fue dropeada. El catálogo de pricing
// (input/output USD por 1M tokens) ahora vive como CONSTANTES en código —
// fuente única, sin dependencia de DB ni cache con TTL. El catálogo coincide
// con el seeder previo (model_registry_seeder). Modelos no listados devuelven
// ErrModelNotFound (costo 0 en el hot path del runner).
package registry

import (
	"context"
	"errors"
	"fmt"

	"nunezlagos/domain/internal/llm"
)

var ErrModelNotFound = errors.New("model not found in registry")

type Model struct {
	Provider            string
	Model               string
	DisplayName         string
	Modality            string
	ContextSize         *int
	InputPerMillion     *float64
	OutputPerMillion    *float64
	EmbeddingDimensions *int
	IsActive            bool
}

func intPtr(v int) *int         { return &v }
func f64Ptr(v float64) *float64 { return &v }

// catalog es el pricing canónico en código (REQ-42.3, ex-model_registry_seeder).
var catalog = buildCatalog()

func buildCatalog() map[string]*Model {
	type entry struct {
		provider, model, display, modality string
		ctxSize                            int
		inPer1M, outPer1M                  float64
		embDim                             int
	}
	entries := []entry{

		{"anthropic", "claude-opus-4-7", "Claude Opus 4.7", "completion", 1_000_000, 15.0, 75.0, 0},
		{"anthropic", "claude-sonnet-4-6", "Claude Sonnet 4.6", "completion", 200_000, 3.0, 15.0, 0},
		{"anthropic", "claude-haiku-4-5-20251001", "Claude Haiku 4.5", "completion", 200_000, 0.8, 4.0, 0},

		{"openai", "gpt-4o", "GPT-4o", "completion", 128_000, 2.5, 10.0, 0},
		{"openai", "gpt-4o-mini", "GPT-4o mini", "completion", 128_000, 0.15, 0.6, 0},
		{"openai", "gpt-5", "GPT-5", "completion", 200_000, 10.0, 30.0, 0},
		{"openai", "text-embedding-3-small", "Embedding 3 Small", "embedding", 8192, 0.02, 0, 1536},
		{"openai", "text-embedding-3-large", "Embedding 3 Large", "embedding", 8192, 0.13, 0, 3072},

		{"voyage", "voyage-3", "Voyage 3", "embedding", 32000, 0.06, 0, 1024},
		{"voyage", "voyage-code-3", "Voyage Code 3", "embedding", 32000, 0.18, 0, 1024},

		{"ollama", "llama3.3:70b", "Llama 3.3 70B (local)", "completion", 128_000, 0, 0, 0},
		{"ollama", "llama3.2:3b", "Llama 3.2 3B (local)", "completion", 128_000, 0, 0, 0},
		{"ollama", "nomic-embed-text", "Nomic Embed Text (local)", "embedding", 8192, 0, 0, 768},

		{"google", "gemini-1.5-pro", "Gemini 1.5 Pro", "completion", 2_000_000, 1.25, 5.0, 0},
		{"google", "gemini-1.5-flash", "Gemini 1.5 Flash", "completion", 1_000_000, 0.075, 0.3, 0},

		// MiniMax-M3 vía endpoint anthropic-compatible. Tarifas pendientes de
		// confirmar: 0/0 es seguro (no rompe el hot path, solo no contabiliza costo).
		{"minimax", "MiniMax-M3", "MiniMax M3", "completion", 1_000_000, 0, 0, 0},
	}
	out := make(map[string]*Model, len(entries))
	for _, e := range entries {
		m := &Model{
			Provider:    e.provider,
			Model:       e.model,
			DisplayName: e.display,
			Modality:    e.modality,
			IsActive:    true,
		}
		if e.ctxSize > 0 {
			m.ContextSize = intPtr(e.ctxSize)
		}
		if e.inPer1M > 0 {
			m.InputPerMillion = f64Ptr(e.inPer1M)
		}
		if e.outPer1M > 0 {
			m.OutputPerMillion = f64Ptr(e.outPer1M)
		}
		if e.embDim > 0 {
			m.EmbeddingDimensions = intPtr(e.embDim)
		}
		out[e.provider+":"+e.model] = m
	}
	return out
}

// Registry expone el catálogo de pricing in-code. Mantiene la API previa para
// no romper callers; ya no requiere *pgxpool.Pool ni cache con TTL.
type Registry struct{}

// New construye un Registry. REQ-42.3: sin dependencias (catálogo en código).
func New() *Registry { return &Registry{} }

// Get retorna el Model por (provider, model).
func (r *Registry) Get(ctx context.Context, provider, model string) (*Model, error) {
	_ = ctx
	if m, ok := catalog[provider+":"+model]; ok {
		return m, nil
	}
	return nil, fmt.Errorf("%w: %s/%s", ErrModelNotFound, provider, model)
}

// CostUSD calcula el costo en USD del Usage para el (provider, model).
func (r *Registry) CostUSD(ctx context.Context, provider, model string, usage llm.Usage) (float64, error) {
	m, err := r.Get(ctx, provider, model)
	if err != nil {
		return 0, err
	}
	var cost float64
	if m.InputPerMillion != nil {
		cost += float64(usage.PromptTokens) * (*m.InputPerMillion) / 1_000_000
	}
	if m.OutputPerMillion != nil {
		cost += float64(usage.CompletionTokens) * (*m.OutputPerMillion) / 1_000_000
	}
	return cost, nil
}

// List devuelve todos los modelos activos del catálogo.
func (r *Registry) List(ctx context.Context) ([]Model, error) {
	_ = ctx
	out := make([]Model, 0, len(catalog))
	for _, m := range catalog {
		if m.IsActive {
			out = append(out, *m)
		}
	}
	return out, nil
}

// Refresh es un no-op (el catálogo es estático en código). Se conserva para
// compatibilidad con callers previos.
func (r *Registry) Refresh(ctx context.Context) error {
	_ = ctx
	return nil
}

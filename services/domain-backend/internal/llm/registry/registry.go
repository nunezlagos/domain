// Package registry — issue-06.4 model_registry con costos por modelo.
//
// Consulta la tabla model_registry para calcular CostUSD de un Usage.
// Cache en memoria con TTL (default 5 min) para evitar query DB en hot path.
package registry

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/llm"
)

var ErrModelNotFound = errors.New("model not found in registry")

type Model struct {
	Provider             string
	Model                string
	DisplayName          string
	Modality             string
	ContextSize          *int
	InputPerMillion      *float64
	OutputPerMillion     *float64
	EmbeddingDimensions  *int
	IsActive             bool
}

// Registry cachea entries de la tabla model_registry.
type Registry struct {
	Pool *pgxpool.Pool
	TTL  time.Duration // default 5 min

	mu        sync.RWMutex
	cache     map[string]*Model // key = provider+":"+model
	expiresAt time.Time
}

// Get retorna el Model por (provider, model). Carga cache si expirado.
func (r *Registry) Get(ctx context.Context, provider, model string) (*Model, error) {
	if err := r.refreshIfNeeded(ctx); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if m, ok := r.cache[provider+":"+model]; ok {
		return m, nil
	}
	return nil, fmt.Errorf("%w: %s/%s", ErrModelNotFound, provider, model)
}

// CostUSD calcula el costo en USD del Usage para el (provider, model).
// input * input_per_million / 1M + output * output_per_million / 1M.
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

// List devuelve todos los modelos activos.
func (r *Registry) List(ctx context.Context) ([]Model, error) {
	if err := r.refreshIfNeeded(ctx); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Model, 0, len(r.cache))
	for _, m := range r.cache {
		if m.IsActive {
			out = append(out, *m)
		}
	}
	return out, nil
}

// Refresh fuerza recarga inmediata.
func (r *Registry) Refresh(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.loadLocked(ctx)
}

func (r *Registry) refreshIfNeeded(ctx context.Context) error {
	ttl := r.TTL
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	r.mu.RLock()
	expired := time.Now().After(r.expiresAt) || r.cache == nil
	r.mu.RUnlock()
	if !expired {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// re-check después del lock por race
	if !time.Now().After(r.expiresAt) && r.cache != nil {
		return nil
	}
	return r.loadLocked(ctx)
}

func (r *Registry) loadLocked(ctx context.Context) error {
	rows, err := r.Pool.Query(ctx,
		`SELECT provider, model, display_name, modality, context_size,
		        input_per_million, output_per_million, embedding_dimensions, is_active
		 FROM model_registry WHERE is_active = true`)
	if err != nil {
		return fmt.Errorf("load model_registry: %w", err)
	}
	defer rows.Close()
	cache := map[string]*Model{}
	for rows.Next() {
		var m Model
		if err := rows.Scan(&m.Provider, &m.Model, &m.DisplayName, &m.Modality,
			&m.ContextSize, &m.InputPerMillion, &m.OutputPerMillion,
			&m.EmbeddingDimensions, &m.IsActive); err != nil {
			return err
		}
		cache[m.Provider+":"+m.Model] = &m
	}
	if err := rows.Err(); err != nil {
		return err
	}
	r.cache = cache
	ttl := r.TTL
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	r.expiresAt = time.Now().Add(ttl)
	return nil
}

// ensure pgx import
var _ = pgx.ErrNoRows

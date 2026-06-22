// Resilience helpers para MCP tools — issue-12.6.
//
// Funcionalidad:
//   - Rate limiter per-tool (token bucket simple in-memory): cap de calls/min
//     por tool name. Protege contra agent que se enloquece y spammea tools.
//   - Retry transient errors (deadline exceeded, connection reset): hasta N
//     reintentos con backoff exponencial 100ms → 200ms → 400ms.
//   - WithBudget(tool, rate) wraps handler con enforcement.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"
)

// ToolBudget configura limite por tool (defaults razonables si zero).
type ToolBudget struct {
	CallsPerMinute int           // 0 = unlimited
	MaxRetries     int           // default 0 (sin retry)
	RetryBackoff   time.Duration // default 100ms
	CBThreshold    int           // fallos consecutivos para abrir el breaker; 0 = sin CB
	CBCooldown     time.Duration // tiempo abierto antes de half-open; default 30s
}

// rateState tracking interno per-tool.
type rateState struct {
	mu     sync.Mutex
	window []time.Time // timestamps de las ultimas N calls
}

func (s *rateState) allow(maxPerMin int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-1 * time.Minute)
	// Compactar window: quitar timestamps < cutoff
	kept := s.window[:0]
	for _, t := range s.window {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	s.window = kept
	if len(s.window) >= maxPerMin {
		return false
	}
	s.window = append(s.window, now)
	return true
}

// cbState circuit breaker per-tool: tras CBThreshold fallos consecutivos
// se abre por CBCooldown. Pasado el cooldown entra en half-open implicito:
// la siguiente call pasa; si falla re-abre de inmediato, si funciona resetea.
type cbState struct {
	mu          sync.Mutex
	consecutive int
	openUntil   time.Time
}

func (s *cbState) allow(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !now.Before(s.openUntil)
}

func (s *cbState) record(failure bool, threshold int, cooldown time.Duration, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !failure {
		s.consecutive = 0
		s.openUntil = time.Time{}
		return
	}
	s.consecutive++
	// No se resetea consecutive al abrir: un fallo en half-open re-abre directo.
	if s.consecutive >= threshold {
		s.openUntil = now.Add(cooldown)
	}
}

// ResilientWrapper agrega budget + retry + circuit breaker a un mcpgo.ToolHandlerFunc.
//
// REQ-67 tambien encapsula un query cache opcional (cacheLRU). Tools
// READ marcados via SetCacheable(name, ttl) cachean su resultado por
// (org_id, tool, args_hash). Tools WRITE marcados via SetInvalidating
// (name) limpian el cache del org tras handler exitoso.
type ResilientWrapper struct {
	mu       sync.Mutex
	states   map[string]*rateState
	cbs      map[string]*cbState
	budgets  map[string]ToolBudget
	defaults ToolBudget
	now      func() time.Time

	// REQ-67 cache
	cacheLRU    CacheStore
	cacheTTLs   map[string]time.Duration // tool -> ttl si es cacheable
	invalidates map[string]bool          // tool -> true si invalida en escritura
	orgIDFn     func() string            // accessor del orgID del principal vigente

	// REQ-70 metricas. Hooks no-op si nil — el wrap sigue funcionando.
	metricsOnCall  func(tool, status string, durationSeconds float64)
	metricsOnCacheHit  func()
	metricsOnCacheMiss func()
}

// CacheStore abstrae el LRU (interface para poder mockear en tests).
// Exportado para que el wireup principal (cmd/domain) pueda inyectar
// una implementacion (cache.LRU del package internal/cache).
type CacheStore interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte, ttl time.Duration)
	FlushPrefix(prefix string) int
}

func NewResilientWrapper(defaults ToolBudget) *ResilientWrapper {
	return &ResilientWrapper{
		states:      map[string]*rateState{},
		cbs:         map[string]*cbState{},
		budgets:     map[string]ToolBudget{},
		defaults:    defaults,
		now:         time.Now,
		cacheTTLs:   map[string]time.Duration{},
		invalidates: map[string]bool{},
	}
}

// SetCache activa el cache LRU. Llamar una vez en bootstrap. Si no se
// llama, los hooks de cacheable/invalidating son no-op.
func (r *ResilientWrapper) SetCache(store CacheStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheLRU = store
}

// SetCacheable marca un tool como READ-cacheable con TTL especifico.
func (r *ResilientWrapper) SetCacheable(toolName string, ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheTTLs[toolName] = ttl
}

// SetInvalidating marca un tool como WRITE que invalida el cache del
// org tras ejecutar exitosamente.
func (r *ResilientWrapper) SetInvalidating(toolName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.invalidates[toolName] = true
}

// SetMetricsHooks (REQ-70) inyecta callbacks para emitir metricas.
// El wrapper no conoce de Prometheus; quien crea el wrapper (server.Tools)
// pasa los hooks que tocan los Counter/Histogram del Registry.
func (r *ResilientWrapper) SetMetricsHooks(
	onCall func(tool, status string, dur float64),
	onCacheHit func(),
	onCacheMiss func(),
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metricsOnCall = onCall
	r.metricsOnCacheHit = onCacheHit
	r.metricsOnCacheMiss = onCacheMiss
}

// SetOrgIDAccessor inyecta un closure que devuelve el orgID del
// principal vigente. Necesario porque el wrap no tiene acceso directo
// a Deps; quien crea el wrapper (server.Tools) si.
func (r *ResilientWrapper) SetOrgIDAccessor(fn func() string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orgIDFn = fn
}

func (r *ResilientWrapper) cacheFor(toolName string) (CacheStore, time.Duration, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cacheLRU == nil {
		return nil, 0, false
	}
	ttl, ok := r.cacheTTLs[toolName]
	if !ok {
		return nil, 0, false
	}
	return r.cacheLRU, ttl, true
}

func (r *ResilientWrapper) isInvalidating(toolName string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.invalidates[toolName]
}

func (r *ResilientWrapper) invalidateOrg(orgID string) {
	r.mu.Lock()
	store := r.cacheLRU
	r.mu.Unlock()
	if store == nil || orgID == "" {
		return
	}
	store.FlushPrefix(orgID + ":")
}

// SetBudget configura budget especifico para un tool.
func (r *ResilientWrapper) SetBudget(toolName string, b ToolBudget) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.budgets[toolName] = b
}

func (r *ResilientWrapper) state(toolName string) *rateState {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.states[toolName]; ok {
		return s
	}
	s := &rateState{}
	r.states[toolName] = s
	return s
}

func (r *ResilientWrapper) budget(toolName string) ToolBudget {
	r.mu.Lock()
	defer r.mu.Unlock()
	if b, ok := r.budgets[toolName]; ok {
		return b
	}
	return r.defaults
}

func (r *ResilientWrapper) breaker(toolName string) *cbState {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.cbs[toolName]; ok {
		return s
	}
	s := &cbState{}
	r.cbs[toolName] = s
	return s
}

// Wrap envuelve un handler con rate limiting + retry + cache + metricas.
func (r *ResilientWrapper) Wrap(toolName string, handler mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := r.now()
		b := r.budget(toolName)

		// Circuit breaker check (fail-fast sin invocar handler)
		var cb *cbState
		if b.CBThreshold > 0 {
			cb = r.breaker(toolName)
			if !cb.allow(r.now()) {
				return mcp.NewToolResultError(
					fmt.Sprintf("circuit open for tool '%s': too many consecutive failures, retry later",
						toolName)), nil
			}
		}

		// Rate limit check
		if b.CallsPerMinute > 0 {
			if !r.state(toolName).allow(b.CallsPerMinute) {
				return mcp.NewToolResultError(
					fmt.Sprintf("rate limit exceeded for tool '%s': %d calls/min",
						toolName, b.CallsPerMinute)), nil
			}
		}

		// REQ-67 cache lookup (solo si el tool esta marcado cacheable).
		r.mu.Lock()
		orgIDFn := r.orgIDFn
		r.mu.Unlock()
		orgID := ""
		if orgIDFn != nil {
			orgID = orgIDFn()
		}
		store, ttl, cacheable := r.cacheFor(toolName)
		var cacheKey string
		if cacheable && orgID != "" {
			cacheKey = buildCacheKey(orgID, toolName, req)
			if cached, hit := store.Get(cacheKey); hit {
				r.mu.Lock()
				h := r.metricsOnCacheHit
				oc := r.metricsOnCall
				r.mu.Unlock()
				if h != nil {
					h()
				}
				if oc != nil {
					oc(toolName, "cache_hit", time.Since(start).Seconds())
				}
				return decodeCachedResult(cached), nil
			}
			r.mu.Lock()
			m := r.metricsOnCacheMiss
			r.mu.Unlock()
			if m != nil {
				m()
			}
		}

		result, err := execWithRetry(ctx, b, handler, req)

		if cb != nil {
			cooldown := b.CBCooldown
			if cooldown == 0 {
				cooldown = 30 * time.Second
			}
			failure := err != nil || (result != nil && result.IsError)
			cb.record(failure, b.CBThreshold, cooldown, r.now())
		}

		// REQ-67 cache write / invalidation tras exito del handler.
		success := err == nil && (result == nil || !result.IsError)
		if success {
			if cacheable && orgID != "" && cacheKey != "" {
				if enc, encErr := encodeCachedResult(result); encErr == nil {
					store.Set(cacheKey, enc, ttl)
				}
			}
			if r.isInvalidating(toolName) {
				r.invalidateOrg(orgID)
			}
		}
		// REQ-70 emit metrica del call (excepto cache_hits que ya
		// se emitieron arriba con return).
		r.mu.Lock()
		oc := r.metricsOnCall
		r.mu.Unlock()
		if oc != nil {
			status := "ok"
			if err != nil || (result != nil && result.IsError) {
				status = "error"
			}
			oc(toolName, status, time.Since(start).Seconds())
		}
		return result, err
	}
}

// execWithRetry corre el handler con retry + backoff exponencial para
// errores transitorios.
func execWithRetry(ctx context.Context, b ToolBudget, handler mcpgo.ToolHandlerFunc, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	backoff := b.RetryBackoff
	if backoff == 0 {
		backoff = 100 * time.Millisecond
	}

	var lastResult *mcp.CallToolResult
	var lastErr error
	for attempt := 0; attempt <= b.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return mcp.NewToolResultError("canceled"), nil
			case <-time.After(backoff):
			}
			backoff *= 2
		}
		result, err := handler(ctx, req)
		lastResult, lastErr = result, err
		if err == nil && (result == nil || !result.IsError) {
			return result, nil
		}
		// Transient error? (connection reset, deadline, timeout)
		if err != nil && !isTransient(err) {
			return result, err
		}
		if result != nil && result.IsError && !isTransientResult(result) {
			return result, err
		}
	}
	return lastResult, lastErr
}

// isTransient detecta errores que tiene sentido reintentar.
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{"connection reset", "broken pipe", "i/o timeout", "temporary failure"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func isTransientResult(result *mcp.CallToolResult) bool {
	if result == nil || !result.IsError || len(result.Content) == 0 {
		return false
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		return false
	}
	msg := strings.ToLower(tc.Text)
	for _, marker := range []string{"timeout", "temporarily", "service unavailable", "503"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

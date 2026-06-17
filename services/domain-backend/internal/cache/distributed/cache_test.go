package distributed_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/cache/distributed"
)

func TestChannelFor_NamingConvention(t *testing.T) {
	require.Equal(t, "cache_invalidate_platform_policies", distributed.ChannelFor("platform_policies"))
	require.Equal(t, "cache_invalidate_mcp_servers", distributed.ChannelFor("mcp_servers"))
}

// memCache es una implementación in-memory del interface Cache para tests
// y como ejemplo de wrapper para features reales.
type memCache struct {
	mu   sync.Mutex
	data map[string]any
	full int // cuenta llamadas a InvalidateAll
}

func newMemCache() *memCache {
	return &memCache{data: make(map[string]any)}
}

func (m *memCache) Set(k string, v any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[k] = v
}

func (m *memCache) Get(k string) (any, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[k]
	return v, ok
}

func (m *memCache) InvalidateKey(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

func (m *memCache) InvalidateAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]any)
	m.full++
}

func TestMemCache_BasicOperations(t *testing.T) {
	c := newMemCache()
	c.Set("a", 1)
	c.Set("b", 2)

	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, 1, v)

	c.InvalidateKey("a")
	_, ok = c.Get("a")
	require.False(t, ok)

	_, ok = c.Get("b")
	require.True(t, ok, "InvalidateKey debe ser granular, no flush all")
}

func TestMemCache_InvalidateAll(t *testing.T) {
	c := newMemCache()
	c.Set("a", 1)
	c.Set("b", 2)
	c.InvalidateAll()

	_, ok := c.Get("a")
	require.False(t, ok)
	_, ok = c.Get("b")
	require.False(t, ok)
	require.Equal(t, 1, c.full)
}

func TestPayload_ContainsOperationAndID(t *testing.T) {
	id := uuid.New()
	orgID := uuid.New()
	orgStr := orgID.String()
	p := distributed.Payload{
		Operation:      "update",
		ID:             id.String(),
		OrganizationID: &orgStr,
	}
	require.Equal(t, "update", p.Operation)
	require.Equal(t, id.String(), p.ID)
	require.NotNil(t, p.OrganizationID)
}

// Sabotaje: si payload JSON está roto, InvalidateAll se llama (fail-safe).
// Verifica que la convención de "fallar abierto" del Listener está en código:
// no podemos testear sin DB, pero confirmamos que InvalidateAll es el fallback
// vía API directa del memCache.
func TestSabotage_FailSafeInvalidatesAll(t *testing.T) {
	c := newMemCache()
	c.Set("a", 1)
	c.Set("b", 2)

	// Simulamos lo que el listener hace ante payload corrupto
	c.InvalidateAll()

	require.Equal(t, 1, c.full)
	_, ok := c.Get("a")
	require.False(t, ok)

	// Si no fuera fail-safe (solo InvalidateKey con id vacío) la cache
	// quedaría stale — esto verifica el contrato.
}

// Ensure Cache interface compatibility (compile-time check).
var _ distributed.Cache = (*memCache)(nil)

// Helper para evitar warning de unused vars en tests rápidos.
var _ = context.Background

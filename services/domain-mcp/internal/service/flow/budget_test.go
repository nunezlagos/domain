package flow

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBudgetCache_DefaultDuration(t *testing.T) {
	cache := NewBudgetCache(nil)
	dur := cache.defaultDuration()
	require.Equal(t, 5*time.Minute, dur, "default must be 5min")
}

func TestBudgetCache_Invalidate(t *testing.T) {
	orgID := uuid.New()
	cache := NewBudgetCache(nil)

	cache.mu.Lock()
	cache.entries[orgID] = durationEntry{
		duration:  60 * time.Second,
		expiresAt: time.Now().Add(30 * time.Second),
	}
	cache.mu.Unlock()

	cache.Invalidate(orgID)

	cache.mu.RLock()
	_, ok := cache.entries[orgID]
	cache.mu.RUnlock()
	require.False(t, ok, "entry should be removed after invalidate")
}

func TestBudgetCache_CacheExpiry(t *testing.T) {
	orgID := uuid.New()
	cache := NewBudgetCache(nil)

	cache.mu.Lock()
	cache.entries[orgID] = durationEntry{
		duration:  60 * time.Second,
		expiresAt: time.Now().Add(-1 * time.Second),
	}
	cache.mu.Unlock()

	cache.mu.RLock()
	entry, ok := cache.entries[orgID]
	cache.mu.RUnlock()
	require.True(t, ok, "entry should exist in map")
	require.True(t, time.Now().After(entry.expiresAt), "entry should be expired")
}

// DOMAINSERV-234: acá vivía TestGetMaxDuration_IntegrationSkips, cuyo cuerpo era solo
// t.Skip("requires integration database") — y cuyo NOMBRE anunciaba el problema: un test que
// existe para saltearse. Sin build tag, estaba muerto en los dos jobs del CI.
//
// Se eliminó y no se reemplazó porque era REDUNDANTE, medido: GetMaxDuration ya tiene cobertura
// real en budget_integration_test.go, que la ejerce en 4 lugares con base de verdad. El
// placeholder no tapaba un hueco, solo inflaba el conteo de tests con uno que no podía fallar.

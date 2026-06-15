package main

import (
	"os"

	"nunezlagos/domain/internal/cache"
)

// mcpQueryCache devuelve un LRU singleton (4096 entries) o nil si está
// desactivado por env. Usado por el MCP HTTP wireup (REQ-67) y por el
// reporter de métricas (REQ-70) para emitir gauges.
//
// Devuelvo *cache.LRU (no la interface CacheStore) para que el wireup
// pueda leer Stats(). El campo Deps.SharedCache acepta la interface,
// así que el upcast es implícito.
//
// DOMAIN_DISABLE_CACHE=1 → nil (cache off, comportamiento legacy).
func mcpQueryCache() *cache.LRU {
	if os.Getenv("DOMAIN_DISABLE_CACHE") == "1" {
		return nil
	}
	return cache.New(4096)
}

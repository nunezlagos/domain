# Proposal: HU-08.9-agent-hierarchical-context

## Intención

KV store scoped + lifecycle (run / agent / project / organization) accesible desde el ExecContext con read-only por defecto, upstream writes explícitos y RBAC enforcement, para gestionar memoria entre agentes padre↔hijo sin context bloat ni leak entre orgs.

## Scope

**Incluye:**
- Tabla `agent_memory_kv` con scope, key, value JSONB, ttl, owner refs
- API `ExecContext.MemoryGet/Set/Delete(scope, key)` + variant con TTL
- Inheritance read-only padre→hijo
- `upstream_keys` declarados por agent para merge al terminar
- Tool sintético `parent_memory_get(key)` para acceso explícito
- RBAC scope enforcement
- TTL background cleaner

**No incluye:**
- Pub/sub entre runs (futuro)
- Estructuras complejas (queues, sets) — solo KV simple
- Sincronización cross-region (single-region)

## Enfoque técnico

1. Tabla con PK compuesta (scope, scope_id, key); índice por TTL
2. MemoryGet usa cache LRU en memoria con invalidación por LISTEN/NOTIFY
3. Upstream merge en tx al finalizar sub-run
4. Cleaner cada 5min: DELETE WHERE expires_at < now()
5. RBAC: validar scope_id ↔ user/org/project access

## Riesgos

- Race en upstream merge concurrente: row-level locks
- Performance: cache + batch reads
- KV bloat: warning si scope >10MB
- Consistency en cancel: upstream merge NO ocurre si sub-run cancelled

## Testing

- Set/Get/Delete por scope
- Read-only inheritance hijo
- Upstream merge solo keys declarados
- TTL expira en background
- RBAC cross-org → 403
- Conflict resolution en parallel fanout
- Cancel: upstream NO se mergea
- Performance: 10k gets/s con cache

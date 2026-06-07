# Design: HU-08.9-agent-hierarchical-context

## Decisión arquitectónica

**Storage:** Postgres con cache in-memory LRU + LISTEN/NOTIFY invalidation.
**Lifecycle:** TTL configurable + background cleaner.
**Inheritance:** read-only por default; writes via `upstream_keys` declarados.

## Schema

```sql
CREATE TABLE agent_memory_kv (
  scope VARCHAR(20) NOT NULL,           -- run | agent | project | organization
  scope_id UUID NOT NULL,
  key VARCHAR(255) NOT NULL,
  value JSONB NOT NULL,
  organization_id UUID NOT NULL REFERENCES organizations(id),
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW(),
  PRIMARY KEY (scope, scope_id, key)
);
CREATE INDEX ON agent_memory_kv (expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX ON agent_memory_kv (organization_id);

ALTER TABLE agents
  ADD COLUMN upstream_keys TEXT[] DEFAULT '{}';
```

## API

```go
type ExecContext interface {
  // ...
  MemoryGet(scope Scope, key string) (json.RawMessage, error)
  MemorySet(scope Scope, key string, value any, opts ...MemoryOpt) error
  MemoryDelete(scope Scope, key string) error
}

type Scope string
const (
  ScopeRun          Scope = "run"
  ScopeAgent        Scope = "agent"
  ScopeProject      Scope = "project"
  ScopeOrganization Scope = "organization"
)
```

## Inheritance rules

```
Child read:
  ctx.MemoryGet("run", k) → checks own run_id first, then parent's run_id chain
Child write:
  ctx.MemorySet("run", k, v) → writes to own run_id ONLY
On child success completion:
  for k in child.agent.upstream_keys:
    val = MemoryGet(child.run, k)
    if val != nil: MemorySet(parent.run, k, val)
On child cancel/fail:
  NO upstream merge
```

## Tools sintéticos

```
parent_memory_get(key string) → value
parent_memory_list() → []string (keys)  -- requiere agent declarar `can_list_parent: true`
```

## TDD plan

1. Set/Get/Delete por scope
2. Inheritance read child
3. Upstream merge solo declarados
4. TTL expira (fake clock)
5. RBAC cross-org → 403
6. Concurrent upstream merge: lock evita lost updates
7. Cancel: NO mergea
8. Performance: 10k gets/s cache hit
9. Sabotaje: child intenta MemorySet("run", k) en parent scope_id → 403

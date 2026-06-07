# HU-08.9-agent-hierarchical-context

**Origen:** `REQ-08-agent-system`
**Persona:** dx-engineer
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** developer de agentes que coordinan sub-agentes
**Quiero** una API explícita para compartir/aislar memoria y contexto entre padre e hijo
**Para** evitar context bloat o leak no intencional entre agentes

## Modelo

- **Read-only inheritance**: hijo puede leer KV scoped del padre, pero no escribir
- **Write isolation**: hijo escribe en su propio scope (no contamina al padre)
- **Explicit upstream write**: hijo puede declarar `upstream_keys` que el padre absorbe al recibir tool_result
- **Memory scopes**: `run`, `agent`, `project`, `organization` (cada uno con TTL)

## Criterios de aceptación

### Escenario 1: KV scoped por run

```gherkin
Dado que existe API `ctx.MemorySet(scope, key, value)` y `MemoryGet(scope, key)`
Cuando supervisor hace `ctx.MemorySet("run", "topic", "postgres-migration")`
Y delega a sub-agente con `context_keys: ["topic"]`
Entonces el hijo accede a `ctx.MemoryGet("run", "topic") → "postgres-migration"` (read-only)
Y `ctx.MemorySet("run", "topic", "X")` en el hijo NO afecta al padre (write isolated en sub-scope)
```

### Escenario 2: Upstream write explícito

```gherkin
Dado que sub-agente quiere reportar conclusión al padre
Cuando hijo declara `upstream_keys: ["conclusion", "confidence"]` en su definition
Y hace `ctx.MemorySet("run", "conclusion", "...")` antes de terminar
Entonces al completarse sub-run, esos keys se mergean al run del padre
Y otros keys (no en upstream_keys) NO se propagan
```

### Escenario 3: Scopes con TTL

```gherkin
Dado que `ctx.MemorySet("project", "user_preference", "es-ES")`
Cuando se ejecuta otro agent_run del mismo project después de horas
Entonces el valor sigue accesible (scope project no expira por run)
Cuando `ctx.MemorySet("run", "tmp", "x")`
Entonces tmp solo vive el run actual
Cuando `ctx.MemorySet("agent", "cache", v, ttl=1h)`
Entonces cache vive 1h cross-runs del mismo agent
```

### Escenario 4: Context bloat protection

```gherkin
Dado que el supervisor tiene 50 keys en memory run
Cuando delega y NO pasa context_keys explícitos
Entonces el motor inyecta SOLO un placeholder `"Use ctx.MemoryGet('run', key) to access parent state"`
Y el hijo debe pedir keys específicos a través de tool `parent_memory_get(key)`
Y se logean los gets para auditoría
```

### Escenario 5: Conflict detection

```gherkin
Dado que sub-agente A escribe upstream `"conclusion": "X"` 
Y sub-agente B (paralelo) escribe upstream `"conclusion": "Y"`
Cuando ambos terminan en parallel_fanout
Entonces el merge_strategy decide cuál prevalece (built-in: array, custom: reduce_skill)
Y NO se sobrescribe silenciosamente
```

### Escenario 6: Aislamiento entre orgs

```gherkin
Dado que un agent_run intenta `ctx.MemoryGet("project", "X", project_id=Y)` con Y de otra org
Cuando RBAC valida
Entonces 403 + audit log "memory.unauthorized_cross_org_access"
```

## Análisis breve

- **Qué pide:** KV store scoped + lifecycle por scope + upstream merging + RBAC por scope + bloat prevention
- **Esfuerzo:** M
- **Riesgos:** race en upstream merge; performance KV reads frecuentes; consistency en cancel
